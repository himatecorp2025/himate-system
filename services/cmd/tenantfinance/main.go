package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"himate.local/services/internal/automation"
	"himate.local/services/internal/common"
	"himate.local/services/internal/financepolicy"
)

type app struct {
	db             *sql.DB
	partnersHost   string
	automationHost string
	internalToken  string
	serviceKeys    map[string]string
	automationKey  string
	client         *http.Client
	platformPolicy financePolicyState
}

func main() {
	log := common.Logger()
	db, err := common.OpenDB()
	if err != nil {
		log.Error("database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	platformPolicy, err := loadPlatformFinancePolicy()
	if err != nil {
		log.Error("tenant finance configuration", "error", err)
		os.Exit(1)
	}
	keys := loadAutomationKeys()
	automationKey := strings.TrimSpace(keys[financeProducer])
	if len(automationKey) < 24 {
		log.Error("tenant finance automation identity is missing", "service_id", financeProducer)
		os.Exit(1)
	}
	a := &app{
		db:             db,
		partnersHost:   strings.TrimSpace(os.Getenv("PARTNERS_HOSTPORT")),
		automationHost: strings.TrimSpace(os.Getenv("AUTOMATION_HOSTPORT")),
		internalToken:  strings.TrimSpace(os.Getenv("HIMATE_INTERNAL_TOKEN")),
		serviceKeys:    keys,
		automationKey:  automationKey,
		client: &http.Client{
			Timeout: 6 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns: 32, MaxIdleConnsPerHost: 12, IdleConnTimeout: 90 * time.Second,
			},
		},
		platformPolicy: platformPolicy,
	}
	if a.partnersHost == "" || a.automationHost == "" || len(a.internalToken) < 24 {
		log.Error("tenant finance service dependencies are not configured")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := a.migrate(ctx); err != nil {
		cancel()
		log.Error("migration", "error", err)
		os.Exit(1)
	}
	cancel()

	publisherCtx, stopPublisher := context.WithCancel(context.Background())
	defer stopPublisher()
	go a.runOutboxPublisher(publisherCtx, log)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		common.JSON(w, http.StatusOK, map[string]any{
			"status": "ok", "service": "tenant-finance", "time": time.Now().UTC(),
			"invoice_sources": []string{sourceManual, sourceWorkflow, sourceSchedule},
			"document_renderer": "DEFERRED",
		})
	})
	mux.HandleFunc("/internal/v1/tenant-finance/policy", a.policyHandler)
	mux.HandleFunc("/internal/v1/tenant-finance/invoices", a.invoicesHandler)
	mux.HandleFunc("/internal/v1/tenant-finance/invoices/", a.invoiceByIDHandler)
	mux.HandleFunc("/internal/v1/tenant-finance/automation/invoice-intents", a.automationIntentHandler)
	common.Run(log, "tenant-finance", common.Env("PORT", "10000"), common.InternalAuth(a.internalToken, mux))
}

func loadPlatformFinancePolicy() (financePolicyState, error) {
	rawDays := strings.TrimSpace(os.Getenv("HIMATE_TENANT_FINANCE_DEFAULT_TERMS_DAYS"))
	if rawDays == "" {
		return financePolicyState{}, errors.New("HIMATE_TENANT_FINANCE_DEFAULT_TERMS_DAYS is required")
	}
	days, err := strconv.Atoi(rawDays)
	if err != nil {
		return financePolicyState{}, errors.New("HIMATE_TENANT_FINANCE_DEFAULT_TERMS_DAYS must be an integer")
	}
	methods := []string{}
	for _, method := range strings.Split(os.Getenv("HIMATE_TENANT_FINANCE_DEFAULT_PAYMENT_METHODS"), ",") {
		if method = strings.TrimSpace(method); method != "" {
			methods = append(methods, method)
		}
	}
	policy, err := financepolicy.NormalizePolicy(financepolicy.Policy{
		DefaultPaymentTermsDays: days,
		AccountingBasis: strings.TrimSpace(os.Getenv("HIMATE_TENANT_FINANCE_DEFAULT_ACCOUNTING_BASIS")),
		PaymentMethods: methods,
	})
	if err != nil {
		return financePolicyState{}, err
	}
	currency, err := normalizeCurrency(os.Getenv("HIMATE_TENANT_FINANCE_DEFAULT_CURRENCY"), "")
	if err != nil {
		return financePolicyState{}, fmt.Errorf("default currency: %w", err)
	}
	prefix, err := normalizeInvoicePrefix(os.Getenv("HIMATE_TENANT_FINANCE_DEFAULT_INVOICE_PREFIX"))
	if err != nil {
		return financePolicyState{}, fmt.Errorf("default invoice prefix: %w", err)
	}
	return financePolicyState{Policy: policy, DefaultCurrency: currency, InvoicePrefix: prefix, Source: "PLATFORM_DEFAULT"}, nil
}

func loadAutomationKeys() map[string]string {
	raw := strings.TrimSpace(os.Getenv("HIMATE_AUTOMATION_SERVICE_KEYS_JSON"))
	out := map[string]string{}
	if raw == "" {
		return out
	}
	var values map[string]string
	if json.Unmarshal([]byte(raw), &values) != nil {
		return out
	}
	for key, value := range values {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key != "" && len(value) >= 24 {
			out[key] = value
		}
	}
	return out
}

func partnerHeaders(r *http.Request) (string, string, bool) {
	partnerID := strings.TrimSpace(r.Header.Get("X-Himate-Partner-ID"))
	userID := strings.TrimSpace(r.Header.Get("X-Himate-User-ID"))
	return partnerID, userID, partnerID != "" && userID != ""
}

func (a *app) policyHandler(w http.ResponseWriter, r *http.Request) {
	partnerID, userID, ok := partnerHeaders(r)
	if !ok {
		common.APIError(w, http.StatusBadRequest, "TENANT_CONTEXT_REQUIRED", "Authoritative partner and user context headers are required")
		return
	}
	switch r.Method {
	case http.MethodGet:
		state, err := a.loadPolicy(r.Context(), partnerID)
		if err != nil {
			common.APIError(w, http.StatusInternalServerError, "FINANCE_POLICY", "Could not load tenant finance policy")
			return
		}
		common.JSON(w, http.StatusOK, state)
	case http.MethodPut:
		current, err := a.loadPolicy(r.Context(), partnerID)
		if err != nil {
			common.APIError(w, http.StatusInternalServerError, "FINANCE_POLICY", "Could not load tenant finance policy")
			return
		}
		var in struct {
			DefaultPaymentTermsDays *int      `json:"default_payment_terms_days"`
			AccountingBasis         *string   `json:"accounting_basis"`
			PaymentMethods          *[]string `json:"payment_methods"`
			DefaultCurrency         *string   `json:"default_currency"`
			InvoicePrefix           *string   `json:"invoice_prefix"`
		}
		if common.Decode(r, &in) != nil {
			common.APIError(w, http.StatusBadRequest, "JSON", "Invalid tenant finance policy")
			return
		}
		if in.DefaultPaymentTermsDays != nil {
			current.Policy.DefaultPaymentTermsDays = *in.DefaultPaymentTermsDays
		}
		if in.AccountingBasis != nil {
			current.Policy.AccountingBasis = strings.TrimSpace(*in.AccountingBasis)
		}
		if in.PaymentMethods != nil {
			current.Policy.PaymentMethods = append([]string(nil), (*in.PaymentMethods)...)
		}
		if in.DefaultCurrency != nil {
			current.DefaultCurrency = strings.TrimSpace(*in.DefaultCurrency)
		}
		if in.InvoicePrefix != nil {
			current.InvoicePrefix = strings.TrimSpace(*in.InvoicePrefix)
		}
		saved, err := a.savePolicy(r.Context(), partnerID, userID, current)
		if err != nil {
			writeTenantFinanceError(w, err)
			return
		}
		common.JSON(w, http.StatusOK, saved)
	default:
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use GET or PUT")
	}
}

func (a *app) invoicesHandler(w http.ResponseWriter, r *http.Request) {
	partnerID, userID, ok := partnerHeaders(r)
	if !ok {
		common.APIError(w, http.StatusBadRequest, "TENANT_CONTEXT_REQUIRED", "Authoritative partner and user context headers are required")
		return
	}
	switch r.Method {
	case http.MethodGet:
		limit, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
		items, err := a.listInvoices(r.Context(), partnerID, limit)
		if err != nil {
			common.APIError(w, http.StatusInternalServerError, "DB", "Could not load tenant invoices")
			return
		}
		out := make([]map[string]any, 0, len(items))
		for _, item := range items {
			out = append(out, invoicePayload(item))
		}
		common.JSON(w, http.StatusOK, map[string]any{"partner_id": partnerID, "items": out, "count": len(out)})
	case http.MethodPost:
		var in invoiceInput
		if common.Decode(r, &in) != nil {
			common.APIError(w, http.StatusBadRequest, "JSON", "Invalid manual invoice request; partner_id and source_type are server-controlled")
			return
		}
		rec, duplicate, err := a.createManualDraft(r.Context(), partnerID, userID, in)
		if err != nil {
			writeTenantFinanceError(w, err)
			return
		}
		status := http.StatusCreated
		if duplicate {
			status = http.StatusOK
		}
		payload := invoicePayload(rec)
		payload["duplicate"] = duplicate
		payload["tenant_context_rule"] = "PARTNER_ID_FROM_AUTHENTICATED_GATEWAY_CONTEXT"
		common.JSON(w, status, payload)
	default:
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use GET or POST")
	}
}

func (a *app) invoiceByIDHandler(w http.ResponseWriter, r *http.Request) {
	partnerID, userID, ok := partnerHeaders(r)
	if !ok {
		common.APIError(w, http.StatusBadRequest, "TENANT_CONTEXT_REQUIRED", "Authoritative partner and user context headers are required")
		return
	}
	raw := strings.Trim(strings.TrimPrefix(r.URL.Path, "/internal/v1/tenant-finance/invoices/"), "/")
	if raw == "" {
		common.APIError(w, http.StatusNotFound, "NOT_FOUND", "Invoice not found")
		return
	}
	parts := strings.Split(raw, "/")
	invoiceID := strings.TrimSpace(parts[0])
	if invoiceID == "" || len(parts) > 2 {
		common.APIError(w, http.StatusNotFound, "NOT_FOUND", "Invoice route not found")
		return
	}
	if len(parts) == 1 && r.Method == http.MethodGet {
		rec, err := a.loadInvoice(r.Context(), partnerID, invoiceID)
		if err != nil {
			writeTenantFinanceError(w, err)
			return
		}
		common.JSON(w, http.StatusOK, invoicePayload(rec))
		return
	}
	if len(parts) == 2 && parts[1] == "finalize" && r.Method == http.MethodPost {
		rec, duplicate, err := a.finalizeManual(r.Context(), partnerID, userID, invoiceID, r.Header.Get("X-Correlation-ID"))
		if err != nil {
			writeTenantFinanceError(w, err)
			return
		}
		payload := invoicePayload(rec)
		payload["duplicate"] = duplicate
		common.JSON(w, http.StatusOK, payload)
		return
	}
	common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use GET or POST /finalize")
}

func strictDecodeBytes(raw []byte, dst any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("request body must contain exactly one JSON value")
	}
	return nil
}

func (a *app) automationIntentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use POST")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		common.APIError(w, http.StatusBadRequest, "BODY", "Could not read invoice intent")
		return
	}
	serviceID, err := automation.VerifyRequest(r, body, func(id string) (string, bool) {
		value, ok := a.serviceKeys[id]
		return value, ok
	}, time.Now().UTC(), 5*time.Minute)
	if err != nil {
		common.APIError(w, http.StatusForbidden, "SERVICE_IDENTITY", err.Error())
		return
	}
	var in automatedInvoiceIntent
	if strictDecodeBytes(body, &in) != nil {
		common.APIError(w, http.StatusBadRequest, "JSON", "Invalid automated invoice intent")
		return
	}
	if err := validateSourceForService(serviceID, in.SourceType); err != nil {
		common.APIError(w, http.StatusForbidden, "SOURCE_SERVICE_MISMATCH", err.Error())
		return
	}
	if in.CorrelationID == "" {
		in.CorrelationID = strings.TrimSpace(r.Header.Get(automation.HeaderCorrelationID))
	}
	if in.CausationID == "" {
		in.CausationID = strings.TrimSpace(r.Header.Get(automation.HeaderCausationID))
	}
	rec, duplicate, err := a.createAutomatedReady(r.Context(), serviceID, "service:"+serviceID, in)
	if err != nil {
		writeTenantFinanceError(w, err)
		return
	}
	status := http.StatusCreated
	if duplicate {
		status = http.StatusOK
	}
	payload := invoicePayload(rec)
	payload["duplicate"] = duplicate
	payload["producer_service"] = serviceID
	common.JSON(w, status, payload)
}

func (a *app) fetchIssuer(ctx context.Context, partnerID string) (issuerSnapshot, error) {
	if strings.TrimSpace(a.partnersHost) == "" {
		return issuerSnapshot{}, errors.New("partners service is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"http://"+a.partnersHost+"/api/v1/partners/"+url.PathEscape(strings.TrimSpace(partnerID)), nil)
	if err != nil {
		return issuerSnapshot{}, err
	}
	common.BindInternalRequest(req, a.internalToken)
	resp, err := common.DoInternal(a.client, req)
	if err != nil {
		return issuerSnapshot{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return issuerSnapshot{}, errors.New("partner company profile was not found")
	}
	if resp.StatusCode >= 300 {
		return issuerSnapshot{}, fmt.Errorf("partners service returned status %d", resp.StatusCode)
	}
	var raw map[string]any
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&raw); err != nil {
		return issuerSnapshot{}, err
	}
	issuer, err := normalizeIssuer(partnerID, raw)
	if err != nil {
		return issuerSnapshot{}, fmt.Errorf("ISSUER_PROFILE_INCOMPLETE: %w", err)
	}
	return issuer, nil
}

func writeTenantFinanceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errInvoiceNotFound):
		common.APIError(w, http.StatusNotFound, "INVOICE_NOT_FOUND", "Invoice not found for the authenticated partner")
	case errors.Is(err, errInvoiceState):
		common.APIError(w, http.StatusConflict, "INVOICE_STATE", "Invoice state does not allow this operation")
	case errors.Is(err, errIdempotency):
		common.APIError(w, http.StatusConflict, "REQUEST_KEY_CONFLICT", err.Error())
	case errors.Is(err, errSourceConflict):
		common.APIError(w, http.StatusConflict, "SOURCE_INVOICE_CONFLICT", err.Error())
	case strings.HasPrefix(err.Error(), "ISSUER_PROFILE_INCOMPLETE:"):
		common.APIError(w, http.StatusConflict, "ISSUER_PROFILE_INCOMPLETE", strings.TrimSpace(strings.TrimPrefix(err.Error(), "ISSUER_PROFILE_INCOMPLETE:")))
	case strings.Contains(err.Error(), "required") ||
		strings.Contains(err.Error(), "must ") ||
		strings.Contains(err.Error(), "outside the supported range") ||
		strings.Contains(err.Error(), "at most") ||
		strings.Contains(err.Error(), "exceeds") ||
		strings.Contains(err.Error(), "too large") ||
		strings.Contains(err.Error(), "payment terms") ||
		strings.Contains(err.Error(), "accounting basis") ||
		strings.Contains(err.Error(), "currency") ||
		strings.Contains(err.Error(), "invoice_prefix"):
		common.APIError(w, http.StatusBadRequest, "VALIDATION", err.Error())
	default:
		common.APIError(w, http.StatusInternalServerError, "TENANT_FINANCE", "Tenant finance operation failed")
	}
}

func (a *app) runOutboxPublisher(ctx context.Context, log interface{ Error(string, ...any) }) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		if err := a.publishOutboxOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("tenant finance outbox publisher", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (a *app) publishOutboxOnce(ctx context.Context) error {
	records, err := automation.ClaimOutbox(ctx, a.db, financeProducer, 25, 60*time.Second)
	if err != nil {
		return err
	}
	for _, record := range records {
		payload := map[string]any{
			"event_key": record.EventKey, "event_type": record.EventType, "event_version": record.EventVersion,
			"partner_id": record.PartnerID, "module_key": record.ModuleKey, "correlation_id": record.CorrelationID,
			"causation_id": record.CausationID, "subject_type": record.SubjectType, "subject_id": record.SubjectID,
			"payload": record.Payload, "occurred_at": record.OccurredAt, "available_at": record.AvailableAt,
		}
		body, _ := json.Marshal(payload)
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost,
			"http://"+a.automationHost+"/internal/v1/automation/events", bytes.NewReader(body))
		if reqErr != nil {
			_ = automation.FailOutbox(context.Background(), a.db, record.ID, reqErr.Error(), 15*time.Second)
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		common.BindInternalRequest(req, a.internalToken)
		if signErr := automation.SignRequest(req, body, financeProducer, a.automationKey, time.Now().UTC()); signErr != nil {
			_ = automation.FailOutbox(context.Background(), a.db, record.ID, signErr.Error(), 15*time.Second)
			continue
		}
		resp, sendErr := common.DoInternal(a.client, req)
		if sendErr != nil {
			_ = automation.FailOutbox(context.Background(), a.db, record.ID, sendErr.Error(), 15*time.Second)
			continue
		}
		status := resp.StatusCode
		resp.Body.Close()
		if status < 200 || status >= 300 {
			_ = automation.FailOutbox(context.Background(), a.db, record.ID, fmt.Sprintf("automation status %d", status), 15*time.Second)
			continue
		}
		if err := automation.MarkOutboxPublished(context.Background(), a.db, record.ID); err != nil {
			return err
		}
	}
	return nil
}
