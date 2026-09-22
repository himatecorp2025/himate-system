package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"himate.local/services/internal/common"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type app struct {
	db            *sql.DB
	token         string
	billingHost   string
	provider      string
	stripeKey     string
	webhookSecret string
	stripeAPIBase string
	client        *http.Client
}

type paymentProfile struct {
	PartnerID          string
	Provider           string
	ProviderCustomerID string
	PaymentMethodID    string
	AutopayEnabled     bool
	Status             string
	UpdatedAt          time.Time
}

type chargeRequest struct {
	PartnerID      string  `json:"partner_id"`
	InvoiceID      string  `json:"invoice_id"`
	Purpose        string  `json:"purpose"`
	Amount         float64 `json:"amount"`
	Currency       string  `json:"currency"`
	IdempotencyKey string  `json:"idempotency_key"`
}

type attempt struct {
	ID                string
	PartnerID         string
	InvoiceID         string
	Purpose           string
	Amount            float64
	Currency          string
	Provider          string
	ProviderPaymentID string
	Status            string
	FailureCode       string
	FailureMessage    string
	IdempotencyKey    string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func main() {
	log := common.Logger()
	db, err := common.OpenDB()
	if err != nil { log.Error("database", "error", err); os.Exit(1) }
	defer db.Close()

	a := &app{
		db: db,
		token: strings.TrimSpace(os.Getenv("HIMATE_INTERNAL_TOKEN")),
		billingHost: strings.TrimSpace(os.Getenv("BILLING_HOSTPORT")),
		provider: strings.ToLower(common.Env("PAYMENT_PROVIDER", "mock")),
		stripeKey: strings.TrimSpace(os.Getenv("STRIPE_SECRET_KEY")),
		webhookSecret: strings.TrimSpace(os.Getenv("STRIPE_WEBHOOK_SECRET")),
		stripeAPIBase: strings.TrimRight(common.Env("STRIPE_API_BASE", "https://api.stripe.com"), "/"),
		client: &http.Client{Timeout: 15 * time.Second},
	}
	if a.provider != "mock" && a.provider != "stripe" {
		log.Error("unsupported payment provider", "provider", a.provider)
		os.Exit(1)
	}
	if a.provider == "stripe" && (a.stripeKey == "" || a.webhookSecret == "") {
		log.Error("Stripe provider requires STRIPE_SECRET_KEY and STRIPE_WEBHOOK_SECRET")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := a.migrate(ctx); err != nil { log.Error("migration", "error", err); os.Exit(1) }

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		common.JSON(w, 200, map[string]any{
			"status": "ok", "service": "payments", "provider": a.provider,
			"provider_configured": a.provider == "mock" || (a.stripeKey != "" && a.webhookSecret != ""),
			"time": time.Now().UTC(),
		})
	})
	mux.HandleFunc("/webhooks/stripe", a.stripeWebhook)
	protected := http.NewServeMux()
	protected.HandleFunc("/api/v1/payments/partners/", a.partnerRoutes)
	protected.HandleFunc("/internal/v1/charges", a.createCharge)
	mux.Handle("/api/", common.InternalAuth(a.token, protected))
	mux.Handle("/internal/", common.InternalAuth(a.token, protected))
	common.Run(log, "payments", common.Env("PORT", "10000"), mux)
}

func (a *app) migrate(ctx context.Context) error {
	return common.ApplyMigrations(ctx, a.db, "payments", []common.Migration{
		{Version: 1, Name: "start-23-4-payment-provider", Statements: []string{
			`CREATE SCHEMA IF NOT EXISTS payments`,
			`CREATE TABLE IF NOT EXISTS payments.partner_profiles(
				partner_id TEXT PRIMARY KEY,
				provider TEXT NOT NULL DEFAULT 'stripe',
				provider_customer_id TEXT NOT NULL DEFAULT '',
				payment_method_id TEXT NOT NULL DEFAULT '',
				autopay_enabled BOOLEAN NOT NULL DEFAULT FALSE,
				status TEXT NOT NULL DEFAULT 'UNCONFIGURED',
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE TABLE IF NOT EXISTS payments.attempts(
				id TEXT PRIMARY KEY,
				partner_id TEXT NOT NULL,
				invoice_id TEXT NOT NULL DEFAULT '',
				purpose TEXT NOT NULL,
				amount NUMERIC(12,2) NOT NULL,
				currency TEXT NOT NULL,
				provider TEXT NOT NULL,
				provider_payment_id TEXT NOT NULL DEFAULT '',
				status TEXT NOT NULL DEFAULT 'PENDING',
				failure_code TEXT NOT NULL DEFAULT '',
				failure_message TEXT NOT NULL DEFAULT '',
				idempotency_key TEXT NOT NULL UNIQUE,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE INDEX IF NOT EXISTS payment_attempts_partner_idx ON payments.attempts(partner_id,created_at DESC)`,
			`CREATE INDEX IF NOT EXISTS payment_attempts_invoice_idx ON payments.attempts(invoice_id) WHERE invoice_id<>''`,
			`CREATE TABLE IF NOT EXISTS payments.webhook_events(
				provider_event_id TEXT PRIMARY KEY,
				event_type TEXT NOT NULL,
				payload_sha256 TEXT NOT NULL,
				status TEXT NOT NULL DEFAULT 'RECEIVED',
				received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				processed_at TIMESTAMPTZ
			)`,
		}},
	})
}

func (a *app) partnerRoutes(w http.ResponseWriter, r *http.Request) {
	raw := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/payments/partners/"), "/")
	parts := strings.Split(raw, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] != "profile" {
		common.APIError(w, 404, "NOT_FOUND", "Payment route not found")
		return
	}
	partnerID := parts[0]
	switch r.Method {
	case http.MethodGet:
		p, err := a.ensureProfile(partnerID)
		if err != nil { common.APIError(w, 500, "DB", "Could not load payment profile"); return }
		common.JSON(w, 200, profileMap(p))
	case http.MethodPut:
		var in struct {
			ProviderCustomerID *string `json:"provider_customer_id"`
			PaymentMethodID    *string `json:"payment_method_id"`
			AutopayEnabled     *bool   `json:"autopay_enabled"`
		}
		if common.Decode(r, &in) != nil { common.APIError(w, 400, "JSON", "Invalid request"); return }
		p, err := a.ensureProfile(partnerID)
		if err != nil { common.APIError(w, 500, "DB", "Could not load payment profile"); return }
		if in.ProviderCustomerID != nil { p.ProviderCustomerID = strings.TrimSpace(*in.ProviderCustomerID) }
		if in.PaymentMethodID != nil { p.PaymentMethodID = strings.TrimSpace(*in.PaymentMethodID) }
		if in.AutopayEnabled != nil { p.AutopayEnabled = *in.AutopayEnabled }
		p.Provider = a.provider
		p.Status = "UNCONFIGURED"
		if p.ProviderCustomerID != "" && p.PaymentMethodID != "" { p.Status = "READY" }
		if p.AutopayEnabled && p.Status != "READY" {
			common.APIError(w, 409, "PAYMENT_METHOD_REQUIRED", "Autopay requires a provider customer and payment method")
			return
		}
		_, err = a.db.Exec(`INSERT INTO payments.partner_profiles(partner_id,provider,provider_customer_id,payment_method_id,autopay_enabled,status,updated_at)
			VALUES($1,$2,$3,$4,$5,$6,NOW())
			ON CONFLICT(partner_id) DO UPDATE SET provider=EXCLUDED.provider,provider_customer_id=EXCLUDED.provider_customer_id,
			payment_method_id=EXCLUDED.payment_method_id,autopay_enabled=EXCLUDED.autopay_enabled,status=EXCLUDED.status,updated_at=NOW()`,
			partnerID, p.Provider, p.ProviderCustomerID, p.PaymentMethodID, p.AutopayEnabled, p.Status)
		if err != nil { common.APIError(w, 500, "DB", "Could not update payment profile"); return }
		p, _ = a.ensureProfile(partnerID)
		common.JSON(w, 200, profileMap(p))
	default:
		common.APIError(w, 405, "METHOD", "Use GET or PUT")
	}
}

func (a *app) ensureProfile(partnerID string) (paymentProfile, error) {
	_, err := a.db.Exec(`INSERT INTO payments.partner_profiles(partner_id,provider) VALUES($1,$2) ON CONFLICT(partner_id) DO NOTHING`, partnerID, a.provider)
	if err != nil { return paymentProfile{}, err }
	var p paymentProfile
	err = a.db.QueryRow(`SELECT partner_id,provider,provider_customer_id,payment_method_id,autopay_enabled,status,updated_at
		FROM payments.partner_profiles WHERE partner_id=$1`, partnerID).
		Scan(&p.PartnerID, &p.Provider, &p.ProviderCustomerID, &p.PaymentMethodID, &p.AutopayEnabled, &p.Status, &p.UpdatedAt)
	return p, err
}

func profileMap(p paymentProfile) map[string]any {
	return map[string]any{
		"partner_id": p.PartnerID, "provider": p.Provider,
		"provider_customer_id": p.ProviderCustomerID, "payment_method_id": p.PaymentMethodID,
		"autopay_enabled": p.AutopayEnabled, "status": p.Status, "updated_at": p.UpdatedAt,
	}
}

func attemptID(idempotency string) string {
	sum := sha256.Sum256([]byte(idempotency))
	return "pay_" + hex.EncodeToString(sum[:10])
}

func (a *app) createCharge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { common.APIError(w, 405, "METHOD", "Use POST"); return }
	var in chargeRequest
	if common.Decode(r, &in) != nil { common.APIError(w, 400, "JSON", "Invalid request"); return }
	in.PartnerID = strings.TrimSpace(in.PartnerID)
	in.InvoiceID = strings.TrimSpace(in.InvoiceID)
	in.Purpose = strings.ToUpper(strings.TrimSpace(in.Purpose))
	in.Currency = strings.ToUpper(strings.TrimSpace(in.Currency))
	in.IdempotencyKey = strings.TrimSpace(in.IdempotencyKey)
	if in.PartnerID == "" || in.IdempotencyKey == "" || in.Amount <= 0 || len(in.Currency) != 3 {
		common.APIError(w, 400, "VALIDATION", "partner_id, positive amount, currency and idempotency_key are required")
		return
	}
	if in.Purpose != "ACTIVATION_LICENSE" && in.Purpose != "INVOICE" {
		common.APIError(w, 400, "VALIDATION", "purpose must be ACTIVATION_LICENSE or INVOICE")
		return
	}
	if in.Purpose == "INVOICE" && in.InvoiceID == "" {
		common.APIError(w, 400, "VALIDATION", "invoice_id is required for invoice collection")
		return
	}
	if existing, ok := a.attemptByIdempotency(in.IdempotencyKey); ok {
		common.JSON(w, 200, attemptMap(existing))
		return
	}
	profile, err := a.ensureProfile(in.PartnerID)
	if err != nil { common.APIError(w, 500, "DB", "Could not load payment profile"); return }
	if profile.Status != "READY" {
		common.APIError(w, 409, "PAYMENT_PROFILE_NOT_READY", "Provider customer and payment method must be configured")
		return
	}
	if in.Purpose == "INVOICE" && !profile.AutopayEnabled {
		common.APIError(w, 409, "AUTOPAY_DISABLED", "Automatic recurring collection is disabled for this partner")
		return
	}

	id := attemptID(in.IdempotencyKey)
	_, err = a.db.Exec(`INSERT INTO payments.attempts(id,partner_id,invoice_id,purpose,amount,currency,provider,idempotency_key,status)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,'PENDING') ON CONFLICT(idempotency_key) DO NOTHING`,
		id, in.PartnerID, in.InvoiceID, in.Purpose, math.Round(in.Amount*100)/100, in.Currency, a.provider, in.IdempotencyKey)
	if err != nil { common.APIError(w, 500, "DB", "Could not create payment attempt"); return }

	providerID, providerStatus, err := a.providerCharge(r.Context(), id, in, profile)
	if err != nil {
		_, _ = a.db.Exec(`UPDATE payments.attempts SET status='FAILED',failure_code='PROVIDER_REQUEST_FAILED',failure_message=$2,updated_at=NOW() WHERE id=$1`, id, trimError(err))
		common.APIError(w, 502, "PAYMENT_PROVIDER", "Payment provider request failed")
		return
	}
	status := "PROCESSING"
	switch strings.ToLower(providerStatus) {
	case "requires_action", "requires_payment_method", "requires_confirmation":
		status = "REQUIRES_ACTION"
	case "canceled":
		status = "FAILED"
	}
	_, err = a.db.Exec(`UPDATE payments.attempts SET provider_payment_id=$2,status=$3,updated_at=NOW() WHERE id=$1`, id, providerID, status)
	if err != nil { common.APIError(w, 500, "DB", "Could not update payment attempt"); return }
	got, _ := a.attemptByID(id)
	common.JSON(w, 201, attemptMap(got))
}

func (a *app) providerCharge(ctx context.Context, attemptID string, in chargeRequest, p paymentProfile) (string, string, error) {
	if a.provider == "mock" {
		return "pi_mock_" + strings.TrimPrefix(attemptID, "pay_"), "processing", nil
	}
	form := url.Values{}
	form.Set("amount", strconv.FormatInt(int64(math.Round(in.Amount*100)), 10))
	form.Set("currency", strings.ToLower(in.Currency))
	form.Set("customer", p.ProviderCustomerID)
	form.Set("payment_method", p.PaymentMethodID)
	form.Set("confirm", "true")
	form.Set("off_session", "true")
	form.Set("metadata[attempt_id]", attemptID)
	form.Set("metadata[partner_id]", in.PartnerID)
	form.Set("metadata[purpose]", in.Purpose)
	if in.InvoiceID != "" { form.Set("metadata[invoice_id]", in.InvoiceID) }
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.stripeAPIBase+"/v1/payment_intents", strings.NewReader(form.Encode()))
	if err != nil { return "", "", err }
	req.SetBasicAuth(a.stripeKey, "")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Idempotency-Key", in.IdempotencyKey)
	resp, err := a.client.Do(req)
	if err != nil { return "", "", err }
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil { return "", "", err }
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("stripe status %d", resp.StatusCode)
	}
	var out struct {
		ID string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(raw, &out); err != nil { return "", "", err }
	if out.ID == "" { return "", "", errors.New("provider payment id missing") }
	return out.ID, out.Status, nil
}

func (a *app) attemptByIdempotency(key string) (attempt, bool) {
	var x attempt
	err := a.db.QueryRow(`SELECT id,partner_id,invoice_id,purpose,amount,currency,provider,provider_payment_id,status,failure_code,failure_message,idempotency_key,created_at,updated_at
		FROM payments.attempts WHERE idempotency_key=$1`, key).
		Scan(&x.ID,&x.PartnerID,&x.InvoiceID,&x.Purpose,&x.Amount,&x.Currency,&x.Provider,&x.ProviderPaymentID,&x.Status,&x.FailureCode,&x.FailureMessage,&x.IdempotencyKey,&x.CreatedAt,&x.UpdatedAt)
	return x, err == nil
}

func (a *app) attemptByID(id string) (attempt, error) {
	var x attempt
	err := a.db.QueryRow(`SELECT id,partner_id,invoice_id,purpose,amount,currency,provider,provider_payment_id,status,failure_code,failure_message,idempotency_key,created_at,updated_at
		FROM payments.attempts WHERE id=$1`, id).
		Scan(&x.ID,&x.PartnerID,&x.InvoiceID,&x.Purpose,&x.Amount,&x.Currency,&x.Provider,&x.ProviderPaymentID,&x.Status,&x.FailureCode,&x.FailureMessage,&x.IdempotencyKey,&x.CreatedAt,&x.UpdatedAt)
	return x, err
}

func attemptMap(x attempt) map[string]any {
	return map[string]any{
		"id":x.ID,"partner_id":x.PartnerID,"invoice_id":x.InvoiceID,"purpose":x.Purpose,
		"amount":x.Amount,"currency":x.Currency,"provider":x.Provider,"provider_payment_id":x.ProviderPaymentID,
		"status":x.Status,"failure_code":x.FailureCode,"failure_message":x.FailureMessage,
		"idempotency_key":x.IdempotencyKey,"created_at":x.CreatedAt,"updated_at":x.UpdatedAt,
	}
}

func (a *app) stripeWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { common.APIError(w, 405, "METHOD", "Use POST"); return }
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil { common.APIError(w, 400, "BODY", "Could not read webhook body"); return }
	if err := verifyStripeSignature(raw, r.Header.Get("Stripe-Signature"), a.webhookSecret, time.Now().UTC(), 5*time.Minute); err != nil {
		common.APIError(w, 400, "INVALID_SIGNATURE", "Invalid payment webhook signature")
		return
	}
	var event struct {
		ID string `json:"id"`
		Type string `json:"type"`
		Data struct {
			Object struct {
				ID string `json:"id"`
				Status string `json:"status"`
				AmountReceived int64 `json:"amount_received"`
				Currency string `json:"currency"`
				Customer string `json:"customer"`
				PaymentMethod string `json:"payment_method"`
				Metadata map[string]string `json:"metadata"`
				LastPaymentError *struct {
					Code string `json:"code"`
					Message string `json:"message"`
				} `json:"last_payment_error"`
			} `json:"object"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &event) != nil || strings.TrimSpace(event.ID) == "" {
		common.APIError(w, 400, "EVENT", "Invalid payment webhook event")
		return
	}
	sum := sha256.Sum256(raw)
	payloadHash := hex.EncodeToString(sum[:])
	res, err := a.db.Exec(`INSERT INTO payments.webhook_events(provider_event_id,event_type,payload_sha256,status)
		VALUES($1,$2,$3,'RECEIVED') ON CONFLICT(provider_event_id) DO NOTHING`, event.ID, event.Type, payloadHash)
	if err != nil { common.APIError(w, 500, "DB", "Could not persist webhook event"); return }
	rows, _ := res.RowsAffected()
	if rows == 0 {
		var status, existingHash string
		if err := a.db.QueryRow(`SELECT status,payload_sha256 FROM payments.webhook_events WHERE provider_event_id=$1`,event.ID).Scan(&status,&existingHash); err != nil {
			common.APIError(w,500,"DB","Could not inspect webhook retry");return
		}
		if existingHash != payloadHash {
			common.APIError(w,409,"WEBHOOK_EVENT_CONFLICT","Provider event ID was reused with a different payload");return
		}
		if status == "PROCESSED" {
			common.JSON(w, 200, map[string]any{"status":"duplicate","event_id":event.ID})
			return
		}
	}
	if event.Type != "payment_intent.succeeded" && event.Type != "payment_intent.payment_failed" {
		common.JSON(w, 200, map[string]any{"status":"ignored","event_id":event.ID})
		return
	}
	attemptID := strings.TrimSpace(event.Data.Object.Metadata["attempt_id"])
	x, err := a.attemptByID(attemptID)
	if err != nil {
		common.APIError(w, 409, "ATTEMPT_NOT_FOUND", "Webhook does not match a payment attempt")
		return
	}
	if event.Data.Object.ID != "" && x.ProviderPaymentID != "" && event.Data.Object.ID != x.ProviderPaymentID {
		common.APIError(w,409,"PAYMENT_ID_MISMATCH","Webhook payment does not match the original provider attempt")
		return
	}
	if event.Type == "payment_intent.succeeded" {
		received := float64(event.Data.Object.AmountReceived) / 100.0
		if event.Data.Object.AmountReceived <= 0 || math.Abs(received-x.Amount) > 0.005 || !strings.EqualFold(event.Data.Object.Currency,x.Currency) {
			common.APIError(w,409,"PAYMENT_AMOUNT_MISMATCH","Webhook amount or currency does not match the original attempt")
			return
		}
	}
	status := "SUCCEEDED"
	failureCode, failureMessage := "", ""
	if event.Type == "payment_intent.payment_failed" {
		status = "FAILED"
		if event.Data.Object.LastPaymentError != nil {
			failureCode = event.Data.Object.LastPaymentError.Code
			failureMessage = event.Data.Object.LastPaymentError.Message
		}
	}
	providerPaymentID := strings.TrimSpace(event.Data.Object.ID)
	if providerPaymentID == "" { providerPaymentID = x.ProviderPaymentID }
	if _, err := a.db.Exec(`UPDATE payments.attempts SET provider_payment_id=$2,status=$3,failure_code=$4,failure_message=$5,updated_at=NOW() WHERE id=$1`,
		x.ID, providerPaymentID, status, failureCode, failureMessage); err != nil {
		common.APIError(w, 500, "DB", "Could not update payment attempt")
		return
	}
	if status == "SUCCEEDED" && event.Data.Object.Customer != "" && event.Data.Object.PaymentMethod != "" {
		_, _ = a.db.Exec(`UPDATE payments.partner_profiles SET provider_customer_id=$2,payment_method_id=$3,status='READY',updated_at=NOW() WHERE partner_id=$1`,
			x.PartnerID, event.Data.Object.Customer, event.Data.Object.PaymentMethod)
	}
	if err := a.settleBilling(r.Context(), x, status, providerPaymentID, event.ID, failureCode, failureMessage); err != nil {
		common.APIError(w, 502, "BILLING_SETTLEMENT", "Payment was verified but billing settlement failed")
		return
	}
	if _,err:=a.db.Exec(`UPDATE payments.webhook_events SET status='PROCESSED',processed_at=NOW() WHERE provider_event_id=$1`,event.ID);err!=nil{
		common.APIError(w,500,"DB","Could not finalize webhook processing state");return
	}
	common.JSON(w, 200, map[string]any{"status":"processed","event_id":event.ID,"attempt_id":x.ID,"payment_status":status})
}

func verifyStripeSignature(body []byte, header, secret string, now time.Time, tolerance time.Duration) error {
	if strings.TrimSpace(secret) == "" { return errors.New("webhook secret missing") }
	var timestamp int64
	signatures := []string{}
	for _, part := range strings.Split(header, ",") {
		pair := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(pair) != 2 { continue }
		switch pair[0] {
		case "t":
			timestamp, _ = strconv.ParseInt(pair[1], 10, 64)
		case "v1":
			signatures = append(signatures, pair[1])
		}
	}
	if timestamp <= 0 || len(signatures) == 0 { return errors.New("signature fields missing") }
	eventTime := time.Unix(timestamp,0)
	if now.Sub(eventTime) > tolerance || eventTime.Sub(now) > tolerance { return errors.New("signature timestamp outside tolerance") }
	signed := strconv.FormatInt(timestamp,10) + "." + string(body)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(signed))
	expected := mac.Sum(nil)
	for _, sig := range signatures {
		got, err := hex.DecodeString(sig)
		if err == nil && hmac.Equal(expected, got) { return nil }
	}
	return errors.New("signature mismatch")
}

func (a *app) settleBilling(ctx context.Context, x attempt, status, providerPaymentID, eventID, failureCode, failureMessage string) error {
	if a.billingHost == "" { return errors.New("BILLING_HOSTPORT is required") }
	payload := map[string]any{
		"attempt_id":x.ID,"partner_id":x.PartnerID,"invoice_id":x.InvoiceID,"purpose":x.Purpose,
		"amount":x.Amount,"currency":x.Currency,"status":status,"provider":x.Provider,
		"provider_payment_id":providerPaymentID,"provider_event_id":eventID,
		"failure_code":failureCode,"failure_message":failureMessage,
	}
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx,http.MethodPost,"http://"+a.billingHost+"/internal/v1/payments/settlements",bytes.NewReader(raw))
	if err != nil { return err }
	req.Header.Set("Content-Type","application/json")
	req.Header.Set("X-Himate-Internal-Token",a.token)
	req.Header.Set("X-Himate-User-ID","payments-service")
	resp, err := a.client.Do(req)
	if err != nil { return err }
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("billing settlement status %d",resp.StatusCode)
	}
	return nil
}

func trimError(err error) string {
	if err == nil { return "" }
	s := err.Error()
	if len(s) > 500 { s = s[:500] }
	return s
}
