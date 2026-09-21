package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"himate.local/services/internal/common"
	"math"
	"net/http"
	"os"
	"strings"
	"time"
)

type app struct {
	db           *sql.DB
	catalogHost  string
	partnersHost string
	token        string
	client      *http.Client
}

type terms struct {
	PartnerID             string
	Currency              string
	ActivationFee         float64
	ActivationFeeWaived   bool
	ActivationFeeReason   string
	BaseMonthlyFee        float64
	AnnualIncreasePercent float64
	CycleDays             int
	InvoiceDay            int
	PriceEffectiveFrom    time.Time
	ServiceAnchorDate     time.Time
	UpdatedAt             time.Time
}

type initialLicense struct {
	PartnerID     string
	Currency      string
	Required      float64
	Paid          float64
	Status        string
	PaymentDate   sql.NullTime
	Reference     string
	VerifiedBy    string
	Note          string
	Waived        bool
	WaiverReason  string
	UpdatedAt     time.Time
}

func main() {
	log := common.Logger()
	db, err := common.OpenDB()
	if err != nil {
		log.Error("database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	a := &app{
		db: db,
		catalogHost: os.Getenv("CATALOG_HOSTPORT"),
		partnersHost: os.Getenv("PARTNERS_HOSTPORT"),
		token: os.Getenv("HIMATE_INTERNAL_TOKEN"),
		client: &http.Client{Timeout: 8 * time.Second},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := a.migrate(ctx); err != nil {
		log.Error("migration", "error", err)
		os.Exit(1)
	}
	if len(os.Args) > 1 && os.Args[1] == "--run-invoice-cycle" {
		runAt := time.Now().UTC()
		if len(os.Args) > 2 && strings.TrimSpace(os.Args[2]) != "" {
			parsed, parseErr := time.Parse("2006-01-02", strings.TrimSpace(os.Args[2]))
			if parseErr != nil {
				log.Error("invoice cycle date", "error", parseErr)
				os.Exit(1)
			}
			runAt = parsed.UTC()
		}
		if err := a.runInvoiceCycle(context.Background(), runAt); err != nil {
			log.Error("invoice cycle", "error", err)
			os.Exit(1)
		}
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		common.JSON(w, 200, map[string]any{
			"status": "ok", "service": "billing", "cycle_days": 30,
			"cycle_model": "activation-date anchored", "annual_increase_date": "January 1", "time": time.Now().UTC(),
		})
	})
	mux.HandleFunc("/api/v1/billing/profile", a.profile)
	mux.HandleFunc("/api/v1/billing/partners/", a.partnerRoutes)
	mux.HandleFunc("/internal/v1/invoices/run", a.runEndpoint)
	mux.HandleFunc("/internal/v1/portfolio", a.portfolio)
	mux.HandleFunc("/internal/v1/partners/", a.internalPartnerRoutes)
	common.Run(log, "billing", common.Env("PORT", "10000"), common.InternalAuth(a.token, mux))
}

func (a *app) migrate(ctx context.Context) error {
	if err := common.ApplyMigrations(ctx, a.db, "billing", []common.Migration{
		{Version: 1, Name: "billing-base", Statements: []string{
			`CREATE SCHEMA IF NOT EXISTS billing`,
			`CREATE TABLE IF NOT EXISTS billing.company_profile(
				id INT PRIMARY KEY DEFAULT 1 CHECK(id=1),legal_name TEXT NOT NULL DEFAULT '',address TEXT NOT NULL DEFAULT '',tax_id TEXT NOT NULL DEFAULT '',email TEXT NOT NULL DEFAULT '',
				bank_name TEXT NOT NULL DEFAULT '',bank_address TEXT NOT NULL DEFAULT '',account_number TEXT NOT NULL DEFAULT '',iban TEXT NOT NULL DEFAULT '',swift TEXT NOT NULL DEFAULT '',updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
			`INSERT INTO billing.company_profile(id) VALUES(1) ON CONFLICT(id) DO NOTHING`,
			`CREATE TABLE IF NOT EXISTS billing.partner_terms(
				partner_id TEXT PRIMARY KEY,currency TEXT NOT NULL DEFAULT 'USD',activation_fee NUMERIC(12,2) NOT NULL DEFAULT 13000,activation_fee_waived BOOLEAN NOT NULL DEFAULT FALSE,
				activation_fee_reason TEXT NOT NULL DEFAULT '',base_monthly_fee NUMERIC(12,2) NOT NULL DEFAULT 0,annual_increase_percent NUMERIC(6,2) NOT NULL DEFAULT 10,
				cycle_days INT NOT NULL DEFAULT 30,invoice_day INT NOT NULL DEFAULT 1,price_effective_from DATE NOT NULL DEFAULT CURRENT_DATE,updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
			`CREATE TABLE IF NOT EXISTS billing.documents(
				id BIGSERIAL PRIMARY KEY,partner_id TEXT NOT NULL,kind TEXT NOT NULL,name TEXT NOT NULL,storage_url TEXT NOT NULL DEFAULT '',note TEXT NOT NULL DEFAULT '',created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
			`CREATE TABLE IF NOT EXISTS billing.invoices(
				id TEXT PRIMARY KEY,partner_id TEXT NOT NULL,invoice_date DATE NOT NULL,service_period_start DATE NOT NULL,service_period_end DATE NOT NULL,currency TEXT NOT NULL,
				base_fee NUMERIC(12,2) NOT NULL,module_fee NUMERIC(12,2) NOT NULL,total NUMERIC(12,2) NOT NULL,status TEXT NOT NULL DEFAULT 'DRAFT',provider_status TEXT NOT NULL DEFAULT 'NOT_CONFIGURED',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),UNIQUE(partner_id,invoice_date))`,
		}},
		{Version: 2, Name: "license-history-and-30-day-cycles", Statements: []string{
			`ALTER TABLE billing.partner_terms ADD COLUMN IF NOT EXISTS service_anchor_date DATE NOT NULL DEFAULT CURRENT_DATE`,
			`CREATE TABLE IF NOT EXISTS billing.initial_licenses(
				partner_id TEXT PRIMARY KEY,
				currency TEXT NOT NULL DEFAULT 'USD',
				required_amount NUMERIC(12,2) NOT NULL DEFAULT 13000,
				paid_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
				status TEXT NOT NULL DEFAULT 'NOT_PAID',
				payment_date DATE,
				payment_reference TEXT NOT NULL DEFAULT '',
				verified_by TEXT NOT NULL DEFAULT '',
				note TEXT NOT NULL DEFAULT '',
				waived BOOLEAN NOT NULL DEFAULT FALSE,
				waiver_reason TEXT NOT NULL DEFAULT '',
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE TABLE IF NOT EXISTS billing.base_fee_history(
				id BIGSERIAL PRIMARY KEY,
				partner_id TEXT NOT NULL,
				currency TEXT NOT NULL,
				old_price NUMERIC(12,2),
				new_price NUMERIC(12,2) NOT NULL,
				old_effective_from DATE,
				new_effective_from DATE NOT NULL,
				actor TEXT NOT NULL DEFAULT '',
				reason TEXT NOT NULL DEFAULT '',
				changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE TABLE IF NOT EXISTS billing.module_subscriptions(
				partner_id TEXT NOT NULL,
				module_key TEXT NOT NULL,
				currency TEXT NOT NULL DEFAULT 'USD',
				activation_date DATE NOT NULL,
				period_start DATE NOT NULL,
				period_end DATE NOT NULL,
				price NUMERIC(12,2) NOT NULL DEFAULT 0,
				auto_renew BOOLEAN NOT NULL DEFAULT TRUE,
				cancel_at_period_end BOOLEAN NOT NULL DEFAULT FALSE,
				payment_status TEXT NOT NULL DEFAULT 'PENDING',
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				PRIMARY KEY(partner_id,module_key)
			)`,
			`ALTER TABLE billing.documents ADD COLUMN IF NOT EXISTS uploaded_by TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE billing.documents ADD COLUMN IF NOT EXISTS verified_by TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE billing.documents ADD COLUMN IF NOT EXISTS mime_type TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE billing.documents ADD COLUMN IF NOT EXISTS sha256 TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE billing.documents ADD COLUMN IF NOT EXISTS size_bytes BIGINT NOT NULL DEFAULT 0`,
			`CREATE INDEX IF NOT EXISTS billing_documents_partner_idx ON billing.documents(partner_id,created_at DESC)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS billing_invoice_period_unique ON billing.invoices(partner_id,service_period_start,service_period_end)`,
		}},
		{Version: 3, Name: "subscription-cancellation-history", Statements: []string{
			`CREATE TABLE IF NOT EXISTS billing.subscription_history(
				id BIGSERIAL PRIMARY KEY,
				partner_id TEXT NOT NULL,
				module_key TEXT NOT NULL,
				old_auto_renew BOOLEAN NOT NULL,
				new_auto_renew BOOLEAN NOT NULL,
				old_cancel_at_period_end BOOLEAN NOT NULL,
				new_cancel_at_period_end BOOLEAN NOT NULL,
				effective_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				actor TEXT NOT NULL DEFAULT '',
				reason TEXT NOT NULL DEFAULT ''
			)`,
			`CREATE INDEX IF NOT EXISTS billing_subscription_history_lookup
				ON billing.subscription_history(partner_id,module_key,effective_at DESC)`,
		}},
		{Version: 4, Name: "billing-profile-contact-details", Statements: []string{
			`ALTER TABLE billing.company_profile ADD COLUMN IF NOT EXISTS registration_number TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE billing.company_profile ADD COLUMN IF NOT EXISTS contact_name TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE billing.company_profile ADD COLUMN IF NOT EXISTS phone TEXT NOT NULL DEFAULT ''`,
		}},
		start223BillingMigration(),
	}); err != nil {
		return err
	}

	if _, err := a.db.ExecContext(ctx, `INSERT INTO billing.partner_terms(
		partner_id,currency,activation_fee,activation_fee_waived,activation_fee_reason,base_monthly_fee,annual_increase_percent,cycle_days,invoice_day,price_effective_from,service_anchor_date
	) VALUES('ptr_000001','USD',0,TRUE,'Existing reference partner; activation fee not applicable',2000,10,30,1,'2026-01-01','2026-01-01')
	ON CONFLICT(partner_id) DO NOTHING`); err != nil {
		return err
	}
	_, err := a.db.ExecContext(ctx, `INSERT INTO billing.initial_licenses(
		partner_id,currency,required_amount,paid_amount,status,waived,waiver_reason,verified_by,note
	) VALUES('ptr_000001','USD',0,0,'WAIVED',TRUE,'Existing reference partner; activation fee not applicable','system','Klavierhaus reference partner')
	ON CONFLICT(partner_id) DO NOTHING`)
	return err
}

func (a *app) profile(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		var legal, registration, address, taxID, contactName, email, phone, bank, bankAddr, account, iban, swift string
		err := a.db.QueryRow(`SELECT legal_name,registration_number,address,tax_id,contact_name,email,phone,bank_name,bank_address,account_number,iban,swift FROM billing.company_profile WHERE id=1`).
			Scan(&legal, &registration, &address, &taxID, &contactName, &email, &phone, &bank, &bankAddr, &account, &iban, &swift)
		if err != nil {
			common.APIError(w, 500, "DB", "Could not load billing profile")
			return
		}
		common.JSON(w, 200, map[string]any{
			"legal_name": legal, "registration_number": registration, "address": address, "tax_id": taxID,
			"contact_name": contactName, "email": email, "phone": phone,
			"bank_name": bank, "bank_address": bankAddr, "account_number": account, "iban": iban, "swift": swift,
		})
	case http.MethodPut:
		var in struct {
			LegalName          string `json:"legal_name"`
			RegistrationNumber string `json:"registration_number"`
			Address            string `json:"address"`
			TaxID              string `json:"tax_id"`
			ContactName        string `json:"contact_name"`
			Email              string `json:"email"`
			Phone              string `json:"phone"`
			BankName           string `json:"bank_name"`
			BankAddress        string `json:"bank_address"`
			AccountNumber      string `json:"account_number"`
			IBAN               string `json:"iban"`
			SWIFT              string `json:"swift"`
		}
		if common.Decode(r, &in) != nil {
			common.APIError(w, 400, "JSON", "Invalid request")
			return
		}
		_, err := a.db.Exec(`UPDATE billing.company_profile SET
			legal_name=$1,registration_number=$2,address=$3,tax_id=$4,contact_name=$5,email=$6,phone=$7,
			bank_name=$8,bank_address=$9,account_number=$10,iban=$11,swift=$12,updated_at=NOW() WHERE id=1`,
			strings.TrimSpace(in.LegalName), strings.TrimSpace(in.RegistrationNumber), strings.TrimSpace(in.Address), strings.TrimSpace(in.TaxID),
			strings.TrimSpace(in.ContactName), strings.ToLower(strings.TrimSpace(in.Email)), strings.TrimSpace(in.Phone),
			strings.TrimSpace(in.BankName), strings.TrimSpace(in.BankAddress), strings.TrimSpace(in.AccountNumber), strings.TrimSpace(in.IBAN), strings.TrimSpace(in.SWIFT))
		if err != nil {
			common.APIError(w, 500, "DB", "Could not update billing profile")
			return
		}
		a.profile(w, cloneAsGet(r))
	default:
		common.APIError(w, 405, "METHOD", "Use GET or PUT")
	}
}

func cloneAsGet(r *http.Request) *http.Request {
	cp := r.Clone(r.Context())
	cp.Method = http.MethodGet
	return cp
}

func (a *app) ensureTerms(id string) (terms, error) {
	if _, err := a.db.Exec(`INSERT INTO billing.partner_terms(partner_id,service_anchor_date) VALUES($1,CURRENT_DATE) ON CONFLICT(partner_id) DO NOTHING`, id); err != nil {
		return terms{}, err
	}
	var t terms
	err := a.db.QueryRow(`SELECT partner_id,currency,activation_fee,activation_fee_waived,activation_fee_reason,base_monthly_fee,annual_increase_percent,cycle_days,invoice_day,price_effective_from,service_anchor_date,updated_at
		FROM billing.partner_terms WHERE partner_id=$1`, id).
		Scan(&t.PartnerID, &t.Currency, &t.ActivationFee, &t.ActivationFeeWaived, &t.ActivationFeeReason, &t.BaseMonthlyFee, &t.AnnualIncreasePercent, &t.CycleDays, &t.InvoiceDay, &t.PriceEffectiveFrom, &t.ServiceAnchorDate, &t.UpdatedAt)
	return t, err
}

func (a *app) ensureLicense(id string) (initialLicense, error) {
	if _, err := a.db.Exec(`INSERT INTO billing.initial_licenses(partner_id) VALUES($1) ON CONFLICT(partner_id) DO NOTHING`, id); err != nil {
		return initialLicense{}, err
	}
	var x initialLicense
	err := a.db.QueryRow(`SELECT partner_id,currency,required_amount,paid_amount,status,payment_date,payment_reference,verified_by,note,waived,waiver_reason,updated_at
		FROM billing.initial_licenses WHERE partner_id=$1`, id).
		Scan(&x.PartnerID, &x.Currency, &x.Required, &x.Paid, &x.Status, &x.PaymentDate, &x.Reference, &x.VerifiedBy, &x.Note, &x.Waived, &x.WaiverReason, &x.UpdatedAt)
	return x, err
}

func isCommercialEvidenceKind(kind string) bool {
	switch strings.ToUpper(strings.TrimSpace(kind)) {
	case "PAYMENT_EVIDENCE", "INVOICE", "RECEIPT", "CONTRACT":
		return true
	default:
		return false
	}
}

func (a *app) commercialEvidenceCount(ctx context.Context, partnerID string) (int, error) {
	var count int
	err := a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM billing.documents
		WHERE partner_id=$1
		  AND kind IN ('PAYMENT_EVIDENCE','INVOICE','RECEIPT','CONTRACT')
		  AND NULLIF(BTRIM(storage_url),'') IS NOT NULL`, partnerID).Scan(&count)
	return count, err
}

func (a *app) referencePartner(ctx context.Context, partnerID string) (bool, error) {
	if strings.TrimSpace(a.partnersHost) == "" {
		return false, fmt.Errorf("PARTNERS_HOSTPORT is required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+a.partnersHost+"/api/v1/partners/"+partnerID, nil)
	if err != nil { return false, err }
	req.Header.Set("X-Himate-Internal-Token", a.token)
	resp, err := a.client.Do(req)
	if err != nil { return false, err }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK { return false, fmt.Errorf("partners status %d", resp.StatusCode) }
	var p struct { ReferencePartner bool `json:"reference_partner"` }
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil { return false, err }
	return p.ReferencePartner, nil
}

func (a *app) internalPartnerRoutes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.APIError(w, 405, "METHOD", "Use GET")
		return
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/internal/v1/partners/"), "/"), "/")
	if len(parts) != 2 || parts[1] != "provisioning-gate" {
		common.APIError(w, 404, "NOT_FOUND", "Route not found")
		return
	}
	id := parts[0]
	x, err := a.ensureLicense(id)
	if err != nil {
		common.APIError(w, 500, "DB", "Could not load initial license")
		return
	}
	evidenceCount, invoiceCount, paymentEvidenceCount, err := a.commercialEvidenceBreakdown(r.Context(), id)
	if err != nil {
		common.APIError(w, 500, "DB", "Could not verify commercial evidence")
		return
	}
	paid := x.Status == "PAID" && paymentEvidenceCount > 0
	agreementStatus := "DRAFT"
	_ = a.db.QueryRow(`SELECT status FROM billing.commercial_agreements WHERE partner_id=$1`, id).Scan(&agreementStatus)
	reference, referenceErr := a.referencePartner(r.Context(), id)
	if referenceErr != nil {
		common.APIError(w, 502, "PARTNER_LOOKUP", "Could not verify reference-partner waiver")
		return
	}
	waived := reference && x.Status == "WAIVED" && x.Waived && strings.TrimSpace(x.WaiverReason) != ""
	allowed := (agreementStatus == "AGREED" && invoiceCount > 0 && paid) || waived
	reason := ""
	if !allowed {
		if agreementStatus != "AGREED" && !waived {
			reason = "Commercial agreement is not confirmed"
		} else if invoiceCount == 0 && !waived {
			reason = "Activation fee invoice evidence is missing"
		} else if paymentEvidenceCount == 0 && !waived {
			reason = "Payment evidence or receipt is missing"
		} else if x.Status != "PAID" && !x.Waived {
			reason = "Initial license payment is not verified"
		} else {
			reason = "Initial license gate is incomplete"
		}
	}
	common.JSON(w, 200, map[string]any{
		"partner_id": id,
		"allowed": allowed,
		"agreement_status": agreementStatus,
		"payment_status": x.Status,
		"evidence_count": evidenceCount,
		"invoice_evidence_count": invoiceCount,
		"payment_evidence_count": paymentEvidenceCount,
		"waived": x.Waived,
		"reason": reason,
	})
}


func (a *app) partnerRoutes(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/billing/partners/"), "/"), "/")
	if len(parts) < 2 || len(parts) > 3 {
		common.APIError(w, 404, "NOT_FOUND", "Route not found")
		return
	}
	id, section := parts[0], parts[1]
	if len(parts) == 3 {
		if section == "subscriptions" {
			a.subscriptionByKey(w, r, id, parts[2])
			return
		}
		common.APIError(w, 404, "NOT_FOUND", "Route not found")
		return
	}
	switch section {
	case "terms":
		a.terms(w, r, id)
	case "license":
		a.license(w, r, id)
	case "agreement":
		a.agreement(w, r, id)
	case "commercial-status":
		a.commercialStatus(w, r, id)
	case "events":
		a.billingEvents(w, r, id)
	case "summary":
		a.summary(w, r, id)
	case "documents":
		a.documents(w, r, id)
	case "invoices":
		a.invoices(w, r, id)
	case "subscriptions":
		a.subscriptions(w, r, id)
	default:
		common.APIError(w, 404, "NOT_FOUND", "Route not found")
	}
}

func (a *app) terms(w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		t, err := a.ensureTerms(id)
		if err != nil {
			common.APIError(w, 500, "DB", "Could not load terms")
			return
		}
		common.JSON(w, 200, termsMap(t))
	case http.MethodPut:
		current, err := a.ensureTerms(id)
		if err != nil {
			common.APIError(w, 500, "DB", "Could not load terms")
			return
		}
		var in struct {
			Currency              *string  `json:"currency"`
			ActivationFee         *float64 `json:"activation_fee"`
			ActivationFeeWaived   *bool    `json:"activation_fee_waived"`
			ActivationFeeReason   *string  `json:"activation_fee_reason"`
			BaseMonthlyFee        *float64 `json:"base_monthly_fee"`
			AnnualIncreasePercent *float64 `json:"annual_increase_percent"`
			PriceEffectiveFrom    *string  `json:"price_effective_from"`
			ServiceAnchorDate     *string  `json:"service_anchor_date"`
			Reason                string   `json:"reason"`
		}
		if common.Decode(r, &in) != nil {
			common.APIError(w, 400, "JSON", "Invalid request")
			return
		}

		next := current
		if in.Currency != nil { next.Currency = strings.ToUpper(strings.TrimSpace(*in.Currency)) }
		if in.ActivationFee != nil { next.ActivationFee = *in.ActivationFee }
		if in.ActivationFeeWaived != nil { next.ActivationFeeWaived = *in.ActivationFeeWaived }
		if in.ActivationFeeReason != nil { next.ActivationFeeReason = strings.TrimSpace(*in.ActivationFeeReason) }
		if in.BaseMonthlyFee != nil { next.BaseMonthlyFee = *in.BaseMonthlyFee }
		if in.AnnualIncreasePercent != nil { next.AnnualIncreasePercent = *in.AnnualIncreasePercent }
		if in.PriceEffectiveFrom != nil {
			p, e := time.Parse("2006-01-02", strings.TrimSpace(*in.PriceEffectiveFrom))
			if e != nil { common.APIError(w, 400, "VALIDATION", "price_effective_from must be YYYY-MM-DD"); return }
			next.PriceEffectiveFrom = p
		}
		if in.ServiceAnchorDate != nil {
			p, e := time.Parse("2006-01-02", strings.TrimSpace(*in.ServiceAnchorDate))
			if e != nil { common.APIError(w, 400, "VALIDATION", "service_anchor_date must be YYYY-MM-DD"); return }
			next.ServiceAnchorDate = p
		}
		if next.Currency == "" { next.Currency = "USD" }
		if next.ActivationFee < 0 || next.BaseMonthlyFee < 0 || next.AnnualIncreasePercent < 0 {
			common.APIError(w, 400, "VALIDATION", "Commercial amounts cannot be negative")
			return
		}
		if !next.ActivationFeeWaived && next.Currency == "USD" && next.ActivationFee < 13000 {
			common.APIError(w, 400, "VALIDATION", "New partner activation fee must be at least USD 13,000 unless explicitly waived")
			return
		}

		tx, err := a.db.BeginTx(r.Context(), &sql.TxOptions{})
		if err != nil { common.APIError(w, 500, "DB", "Could not start terms update"); return }
		defer tx.Rollback()
		_, err = tx.Exec(`UPDATE billing.partner_terms SET currency=$2,activation_fee=$3,activation_fee_waived=$4,activation_fee_reason=$5,base_monthly_fee=$6,annual_increase_percent=$7,cycle_days=30,invoice_day=1,price_effective_from=$8,service_anchor_date=$9,updated_at=NOW() WHERE partner_id=$1`,
			id, next.Currency, next.ActivationFee, next.ActivationFeeWaived, next.ActivationFeeReason, next.BaseMonthlyFee, next.AnnualIncreasePercent, next.PriceEffectiveFrom, next.ServiceAnchorDate)
		if err != nil { common.APIError(w, 500, "DB", "Could not update terms"); return }

		actor := strings.TrimSpace(r.Header.Get("X-Himate-User-ID"))
		reason := strings.TrimSpace(in.Reason)
		if reason == "" { reason = "HIMATE administrator commercial update" }
		if current.BaseMonthlyFee != next.BaseMonthlyFee || !sameDate(current.PriceEffectiveFrom, next.PriceEffectiveFrom) || current.Currency != next.Currency {
			if _, err = tx.Exec(`INSERT INTO billing.base_fee_history(partner_id,currency,old_price,new_price,old_effective_from,new_effective_from,actor,reason) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`,
				id, next.Currency, current.BaseMonthlyFee, next.BaseMonthlyFee, current.PriceEffectiveFrom, next.PriceEffectiveFrom, actor, reason); err != nil {
				common.APIError(w, 500, "DB", "Could not save base fee history")
				return
			}
		}
		if _, err = tx.Exec(`INSERT INTO billing.initial_licenses(partner_id,currency,required_amount,waived,waiver_reason,status)
			VALUES($1,$2,$3,$4,$5,CASE WHEN $4 THEN 'WAIVED' ELSE 'NOT_PAID' END)
			ON CONFLICT(partner_id) DO UPDATE SET currency=EXCLUDED.currency,required_amount=EXCLUDED.required_amount,waived=EXCLUDED.waived,waiver_reason=EXCLUDED.waiver_reason,
				status=CASE WHEN EXCLUDED.waived THEN 'WAIVED' WHEN billing.initial_licenses.status='WAIVED' THEN 'NOT_PAID' ELSE billing.initial_licenses.status END,updated_at=NOW()`,
			id, next.Currency, next.ActivationFee, next.ActivationFeeWaived, next.ActivationFeeReason); err != nil {
			common.APIError(w, 500, "DB", "Could not sync initial license")
			return
		}
		termsEventAt := time.Now().UTC()
		if err = emitBillingEventTx(r.Context(), tx,
			fmt.Sprintf("COMMERCIAL_TERMS_UPDATED:%s:%d", id, termsEventAt.UnixNano()),
			id, "", "COMMERCIAL_TERMS_UPDATED", termsEventAt, map[string]any{
				"currency": next.Currency, "activation_fee": next.ActivationFee,
				"activation_fee_waived": next.ActivationFeeWaived,
				"base_30_day_fee": next.BaseMonthlyFee, "price_effective_from": next.PriceEffectiveFrom.Format("2006-01-02"),
				"actor": actor, "reason": reason,
			}); err != nil {
			common.APIError(w, 500, "DB", "Could not record commercial terms event")
			return
		}
		if err = tx.Commit(); err != nil { common.APIError(w, 500, "DB", "Could not commit terms update"); return }
		t, _ := a.ensureTerms(id)
		common.JSON(w, 200, termsMap(t))
	default:
		common.APIError(w, 405, "METHOD", "Use GET or PUT")
	}
}

func sameDate(a, b time.Time) bool { return a.Format("2006-01-02") == b.Format("2006-01-02") }

func termsMap(t terms) map[string]any {
	return map[string]any{
		"partner_id": t.PartnerID, "currency": t.Currency, "activation_fee": t.ActivationFee,
		"activation_fee_waived": t.ActivationFeeWaived, "activation_fee_reason": t.ActivationFeeReason,
		"base_monthly_fee": t.BaseMonthlyFee, "annual_increase_percent": t.AnnualIncreasePercent,
		"annual_increase_month": 1, "annual_increase_day": 1, "cycle_days": 30, "invoice_day": 1,
		"price_effective_from": t.PriceEffectiveFrom.Format("2006-01-02"), "service_anchor_date": t.ServiceAnchorDate.Format("2006-01-02"),
		"updated_at": t.UpdatedAt,
	}
}

func licenseMap(x initialLicense) map[string]any {
	var paymentDate any
	if x.PaymentDate.Valid { paymentDate = x.PaymentDate.Time.Format("2006-01-02") }
	return map[string]any{
		"partner_id": x.PartnerID, "currency": x.Currency, "required_amount": x.Required, "paid_amount": x.Paid,
		"status": x.Status, "payment_status": x.Status, "payment_date": paymentDate, "payment_reference": x.Reference, "verified_by": x.VerifiedBy,
		"note": x.Note, "waived": x.Waived, "waiver_reason": x.WaiverReason, "updated_at": x.UpdatedAt,
	}
}

func (a *app) license(w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		x, err := a.ensureLicense(id)
		if err != nil { common.APIError(w, 500, "DB", "Could not load initial license"); return }
		common.JSON(w, 200, licenseMap(x))
	case http.MethodPut:
		current, err := a.ensureLicense(id)
		if err != nil { common.APIError(w, 500, "DB", "Could not load initial license"); return }
		var in struct {
			Currency         *string  `json:"currency"`
			RequiredAmount   *float64 `json:"required_amount"`
			PaidAmount       *float64 `json:"paid_amount"`
			PaymentDate      *string  `json:"payment_date"`
			PaymentReference *string  `json:"payment_reference"`
			VerifiedBy       *string  `json:"verified_by"`
			Note             *string  `json:"note"`
			Waived           *bool    `json:"waived"`
			WaiverReason     *string  `json:"waiver_reason"`
		}
		if common.Decode(r, &in) != nil { common.APIError(w, 400, "JSON", "Invalid request"); return }
		next := current
		if in.Currency != nil { next.Currency = strings.ToUpper(strings.TrimSpace(*in.Currency)) }
		if in.RequiredAmount != nil { next.Required = *in.RequiredAmount }
		if in.PaidAmount != nil { next.Paid = *in.PaidAmount }
		if in.PaymentReference != nil { next.Reference = strings.TrimSpace(*in.PaymentReference) }
		if in.VerifiedBy != nil { next.VerifiedBy = strings.TrimSpace(*in.VerifiedBy) }
		if in.Note != nil { next.Note = strings.TrimSpace(*in.Note) }
		if in.Waived != nil { next.Waived = *in.Waived }
		if in.WaiverReason != nil { next.WaiverReason = strings.TrimSpace(*in.WaiverReason) }
		if in.PaymentDate != nil {
			if strings.TrimSpace(*in.PaymentDate) == "" {
				next.PaymentDate = sql.NullTime{}
			} else {
				p, e := time.Parse("2006-01-02", strings.TrimSpace(*in.PaymentDate))
				if e != nil { common.APIError(w, 400, "VALIDATION", "payment_date must be YYYY-MM-DD"); return }
				next.PaymentDate = sql.NullTime{Time: p, Valid: true}
			}
		}
		if next.Required < 0 || next.Paid < 0 { common.APIError(w, 400, "VALIDATION", "License amounts cannot be negative"); return }
		if !next.Waived && next.Currency == "USD" && next.Required < 13000 { common.APIError(w, 400, "VALIDATION", "Initial license must be at least USD 13,000 unless waived"); return }

		status := "NOT_PAID"
		if next.Waived {
			status = "WAIVED"
		} else if next.Paid >= next.Required && next.Required > 0 {
			status = "PAID"
		} else if next.Paid > 0 {
			status = "PARTIALLY_PAID"
		}
		if status == "PAID" {
			if !next.PaymentDate.Valid || next.Reference == "" {
				common.APIError(w, 400, "VALIDATION", "Paid license requires payment_date and payment_reference")
				return
			}
			if next.VerifiedBy == "" { next.VerifiedBy = strings.TrimSpace(r.Header.Get("X-Himate-User-ID")) }
			if next.VerifiedBy == "" { common.APIError(w, 400, "VALIDATION", "Paid license requires verification"); return }
			_, _, paymentEvidenceCount, err := a.commercialEvidenceBreakdown(r.Context(), id)
			if err != nil {
				common.APIError(w, 500, "DB", "Could not verify payment evidence")
				return
			}
			if paymentEvidenceCount == 0 {
				common.APIError(w, 409, "PAYMENT_EVIDENCE_REQUIRED", "Register payment evidence or a receipt before marking the activation license paid")
				return
			}
		}
		var payment any
		if next.PaymentDate.Valid { payment = next.PaymentDate.Time }
		tx, err := a.db.BeginTx(r.Context(), &sql.TxOptions{})
		if err != nil { common.APIError(w, 500, "DB", "Could not start activation payment update"); return }
		defer tx.Rollback()
		_, err = tx.Exec(`UPDATE billing.initial_licenses SET currency=$2,required_amount=$3,paid_amount=$4,status=$5,payment_date=$6,payment_reference=$7,verified_by=$8,note=$9,waived=$10,waiver_reason=$11,updated_at=NOW() WHERE partner_id=$1`,
			id, next.Currency, next.Required, next.Paid, status, payment, next.Reference, next.VerifiedBy, next.Note, next.Waived, next.WaiverReason)
		if err != nil { common.APIError(w, 500, "DB", "Could not update initial license"); return }
		if current.Status != status || current.Paid != next.Paid {
			eventAt := time.Now().UTC()
			eventType := "ACTIVATION_PAYMENT_UPDATED"
			if status == "PAID" { eventType = "LICENSE_PAID" }
			if status == "WAIVED" { eventType = "ACTIVATION_FEE_WAIVED" }
			if err = emitBillingEventTx(r.Context(), tx,
				fmt.Sprintf("%s:%s:%d", eventType, id, eventAt.UnixNano()),
				id, "", eventType, eventAt, map[string]any{
					"required_amount": next.Required, "paid_amount": next.Paid, "currency": next.Currency,
					"payment_reference": next.Reference, "verified_by": next.VerifiedBy, "status": status,
				}); err != nil {
				common.APIError(w, 500, "DB", "Could not record activation payment event")
				return
			}
		}
		if err = tx.Commit(); err != nil { common.APIError(w, 500, "DB", "Could not commit activation payment"); return }
		x, _ := a.ensureLicense(id)
		common.JSON(w, 200, licenseMap(x))
	default:
		common.APIError(w, 405, "METHOD", "Use GET or PUT")
	}
}

func effectiveBaseFee(t terms, at time.Time) float64 {
	value := t.BaseMonthlyFee
	for year := t.PriceEffectiveFrom.Year() + 1; year <= at.Year(); year++ {
		jan1 := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
		if !at.Before(jan1) { value *= 1 + t.AnnualIncreasePercent/100 }
	}
	return math.Round(value*100) / 100
}

func cycleWindow(anchor, at time.Time) (time.Time, time.Time) {
	anchor = dateOnly(anchor)
	at = dateOnly(at)
	if at.Before(anchor) { return anchor, anchor.AddDate(0, 0, 30) }
	days := int(at.Sub(anchor).Hours() / 24)
	index := days / 30
	start := anchor.AddDate(0, 0, index*30)
	return start, start.AddDate(0, 0, 30)
}

func dateOnly(v time.Time) time.Time { return time.Date(v.UTC().Year(), v.UTC().Month(), v.UTC().Day(), 0, 0, 0, 0, time.UTC) }

func cancellationExpired(cancelAtPeriodEnd bool, periodEnd, at time.Time) bool {
	return cancelAtPeriodEnd && !dateOnly(at).Before(dateOnly(periodEnd))
}

func moduleActivationDate(mod map[string]any, fallback time.Time) time.Time {
	raw := strings.TrimSpace(fmt.Sprint(mod["activated_at"]))
	if raw != "" && raw != "<nil>" {
		if parsed, err := time.Parse(time.RFC3339Nano, raw); err == nil {
			return dateOnly(parsed)
		}
	}
	return dateOnly(fallback)
}

func (a *app) summary(w http.ResponseWriter, r *http.Request, id string) {
	t, err := a.ensureTerms(id)
	if err != nil { common.APIError(w, 500, "DB", "Could not load terms"); return }
	_, rawMods, err := a.catalogFees(r.Context(), id)
	if err != nil { common.APIError(w, 502, "CATALOG", "Could not load billable modules"); return }
	now := time.Now().UTC()
	base := effectiveBaseFee(t, now)
	start, end := cycleWindow(t.ServiceAnchorDate, now)
	if err := a.syncSubscriptions(r.Context(), id, t.Currency, rawMods, now); err != nil {
		common.APIError(w, 500, "DB", "Could not synchronize module subscriptions")
		return
	}
	extra, mods, err := a.effectiveModuleFees(r.Context(), id, rawMods, now)
	if err != nil {
		common.APIError(w, 500, "DB", "Could not calculate effective module fees")
		return
	}
	common.JSON(w, 200, map[string]any{
		"partner_id": id, "currency": t.Currency, "effective_base_fee": base, "extra_module_fee": extra,
		"current_total": math.Round((base+extra)*100) / 100, "annual_increase_percent": t.AnnualIncreasePercent,
		"annual_increase_date": "January 1", "cycle_days": 30, "invoice_day": 1,
		"service_period": "activation-date anchored 30-day cycle",
		"current_period_start": start.Format("2006-01-02"), "current_period_end_exclusive": end.Format("2006-01-02"),
		"next_billing_date": end.Format("2006-01-02"), "modules": mods,
	})
}

func (a *app) catalogFees(ctx context.Context, id string) (float64, []map[string]any, error) {
	if strings.TrimSpace(a.catalogHost) == "" { return 0, nil, fmt.Errorf("CATALOG_HOSTPORT is required") }
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+a.catalogHost+"/internal/v1/partners/"+id+"/billable-modules", nil)
	req.Header.Set("X-Himate-Internal-Token", a.token)
	resp, err := a.client.Do(req)
	if err != nil { return 0, nil, err }
	defer resp.Body.Close()
	if resp.StatusCode != 200 { return 0, nil, fmt.Errorf("catalog status %d", resp.StatusCode) }
	var out struct {
		Extra float64          `json:"extra_monthly_total"`
		Items []map[string]any `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil { return 0, nil, err }
	return out.Extra, out.Items, nil
}

func (a *app) effectiveModuleFees(ctx context.Context, id string, mods []map[string]any, at time.Time) (float64, []map[string]any, error) {
	type subState struct {
		Cancel    bool
		PeriodEnd time.Time
		Status    string
	}
	states := map[string]subState{}
	rows, err := a.db.QueryContext(ctx, `SELECT module_key,cancel_at_period_end,period_end,payment_status
		FROM billing.module_subscriptions WHERE partner_id=$1`, id)
	if err != nil { return 0, nil, err }
	for rows.Next() {
		var key, status string
		var cancel bool
		var periodEnd time.Time
		if err := rows.Scan(&key, &cancel, &periodEnd, &status); err != nil {
			rows.Close()
			return 0, nil, err
		}
		states[key] = subState{Cancel: cancel, PeriodEnd: dateOnly(periodEnd), Status: status}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, nil, err
	}
	rows.Close()

	today := dateOnly(at)
	effective := make([]map[string]any, 0, len(mods))
	total := 0.0
	for _, mod := range mods {
		key := fmt.Sprint(mod["key"])
		if state, ok := states[key]; ok {
			if cancellationExpired(state.Cancel, state.PeriodEnd, today) {
				continue
			}
			if state.Status == "INACTIVE" && state.Cancel {
				continue
			}
		}
		effective = append(effective, mod)
		if mod["included_in_base"] == true { continue }
		if value, ok := mod["partner_price"].(float64); ok {
			total += value
		} else if value, ok := mod["partner_price"].(json.Number); ok {
			v, _ := value.Float64()
			total += v
		}
	}
	return math.Round(total*100) / 100, effective, nil
}

func (a *app) setCatalogModuleNotLicensed(ctx context.Context, partnerID, moduleKey string) error {
	if strings.TrimSpace(a.catalogHost) == "" || len(strings.TrimSpace(a.token)) < 24 {
		return fmt.Errorf("catalog service credential is not configured")
	}
	body, err := json.Marshal(map[string]any{
		"status": "NOT_LICENSED",
		"reason": "Subscription cancellation period ended",
	})
	if err != nil { return err }
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch,
		"http://"+a.catalogHost+"/internal/v1/partners/"+partnerID+"/modules/"+moduleKey,
		bytes.NewReader(body))
	if err != nil { return err }
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Himate-Internal-Token", a.token)
	req.Header.Set("X-Himate-User-ID", "billing-cycle")
	resp, err := a.client.Do(req)
	if err != nil { return err }
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("catalog entitlement update returned status %d", resp.StatusCode)
	}
	return nil
}

func (a *app) syncSubscriptions(ctx context.Context, id, currency string, mods []map[string]any, now time.Time) error {
	today := dateOnly(now)
	activeKeys := make([]string, 0, len(mods))
	for _, mod := range mods {
		key := fmt.Sprint(mod["key"])
		if key == "" { continue }
		activeKeys = append(activeKeys, key)

		catalogPrice := 0.0
		if v, ok := mod["partner_price"].(float64); ok {
			catalogPrice = v
		} else if v, ok := mod["partner_price"].(json.Number); ok {
			catalogPrice, _ = v.Float64()
		}
		included := mod["included_in_base"] == true

		var activation, existingStart, existingEnd time.Time
		var currentPrice float64
		var autoRenew, cancelAtEnd bool
		var paymentStatus string
		err := a.db.QueryRowContext(ctx, `SELECT activation_date,period_start,period_end,price,auto_renew,cancel_at_period_end,payment_status
			FROM billing.module_subscriptions WHERE partner_id=$1 AND module_key=$2`, id, key).
			Scan(&activation, &existingStart, &existingEnd, &currentPrice, &autoRenew, &cancelAtEnd, &paymentStatus)

		if err == sql.ErrNoRows {
			activation = moduleActivationDate(mod, today)
			start := dateOnly(activation)
			end := start.AddDate(0, 0, 30)
			var snapshotPrice float64
			firstPeriod := true
			for {
				_, price, _, snapErr := a.ensureModulePeriodSnapshot(ctx, id, key, currency, start, end, catalogPrice, included)
				if snapErr != nil { return fmt.Errorf("create subscription snapshot %s/%s: %w", id, key, snapErr) }
				snapshotPrice = price
				if firstPeriod {
					if err := a.emitBillingEvent(ctx,
						fmt.Sprintf("MODULE_ACTIVATED:%s:%s:%s", id, key, dateOnly(activation).Format("2006-01-02")),
						id, key, "MODULE_ACTIVATED", activation, map[string]any{
							"activation_date": dateOnly(activation).Format("2006-01-02"),
							"period_start": start.Format("2006-01-02"),
							"period_end_exclusive": end.Format("2006-01-02"),
							"price_snapshot": snapshotPrice,
						}); err != nil { return err }
					firstPeriod = false
				}
				if today.Before(end) {
					break
				}
				if err := a.closeModulePeriod(ctx, id, key, start, end, false); err != nil { return err }
				nextStart := end
				nextEnd := nextStart.AddDate(0, 0, 30)
				_, nextPrice, _, snapErr := a.ensureModulePeriodSnapshot(ctx, id, key, currency, nextStart, nextEnd, catalogPrice, included)
				if snapErr != nil { return fmt.Errorf("backfill renewal snapshot %s/%s: %w", id, key, snapErr) }
				if err := a.markModuleRenewed(ctx, id, key, nextStart, nextEnd, nextPrice); err != nil { return err }
				start, end, snapshotPrice = nextStart, nextEnd, nextPrice
				if today.Before(end) {
					break
				}
			}
			if _, err := a.db.ExecContext(ctx, `INSERT INTO billing.module_subscriptions(
					partner_id,module_key,currency,activation_date,period_start,period_end,price,auto_renew,cancel_at_period_end,payment_status
				) VALUES($1,$2,$3,$4,$5,$6,$7,TRUE,FALSE,'PENDING')`,
				id, key, currency, activation, start, end, snapshotPrice); err != nil {
				return err
			}
			continue
		}
		if err != nil { return err }

		activation = dateOnly(activation)
		existingStart = dateOnly(existingStart)
		existingEnd = dateOnly(existingEnd)

		if cancelAtEnd {
			if cancellationExpired(true, existingEnd, today) {
				if err := a.closeModulePeriod(ctx, id, key, existingStart, existingEnd, true); err != nil { return err }
				if err := a.setCatalogModuleNotLicensed(ctx, id, key); err != nil {
					return fmt.Errorf("expire subscription %s/%s: %w", id, key, err)
				}
				if _, err := a.db.ExecContext(ctx, `UPDATE billing.module_subscriptions
					SET auto_renew=FALSE,payment_status='INACTIVE',updated_at=NOW()
					WHERE partner_id=$1 AND module_key=$2`, id, key); err != nil {
					return err
				}
				continue
			}
			// The current period is immutable while cancellation is pending.
			// Catalog price changes are intentionally ignored until the next period.
			if _, _, _, err := a.ensureModulePeriodSnapshot(ctx, id, key, currency, existingStart, existingEnd, currentPrice, included); err != nil {
				return err
			}
			continue
		}

		if paymentStatus == "INACTIVE" {
			activation = moduleActivationDate(mod, today)
			start, end := cycleWindow(activation, today)
			_, snapshotPrice, _, snapErr := a.ensureModulePeriodSnapshot(ctx, id, key, currency, start, end, catalogPrice, included)
			if snapErr != nil { return fmt.Errorf("reactivation snapshot %s/%s: %w", id, key, snapErr) }
			if _, err := a.db.ExecContext(ctx, `UPDATE billing.module_subscriptions SET
					currency=$3,activation_date=$4,period_start=$5,period_end=$6,price=$7,
					auto_renew=TRUE,cancel_at_period_end=FALSE,payment_status='PENDING',updated_at=NOW()
				WHERE partner_id=$1 AND module_key=$2`,
				id, key, currency, activation, start, end, snapshotPrice); err != nil {
				return err
			}
			if err := a.emitBillingEvent(ctx,
				fmt.Sprintf("MODULE_ACTIVATED:%s:%s:%s", id, key, dateOnly(activation).Format("2006-01-02")),
				id, key, "MODULE_ACTIVATED", activation, map[string]any{
					"reactivation": true, "price_snapshot": snapshotPrice,
					"period_start": start.Format("2006-01-02"), "period_end_exclusive": end.Format("2006-01-02"),
				}); err != nil { return err }
			continue
		}

		// Backfill/lock the current period snapshot. Once it exists it is immutable.
		_, snapshotPrice, _, err := a.ensureModulePeriodSnapshot(ctx, id, key, currency, existingStart, existingEnd, currentPrice, included)
		if err != nil { return fmt.Errorf("lock current snapshot %s/%s: %w", id, key, err) }
		currentPrice = snapshotPrice

		// Roll forward every completed module period. Each new period resolves its
		// price as-of that exact period start, so missed cron runs remain reproducible.
		for autoRenew && !today.Before(existingEnd) {
			if err := a.closeModulePeriod(ctx, id, key, existingStart, existingEnd, false); err != nil { return err }
			nextStart := existingEnd
			nextEnd := nextStart.AddDate(0, 0, 30)
			_, nextPrice, _, snapErr := a.ensureModulePeriodSnapshot(ctx, id, key, currency, nextStart, nextEnd, catalogPrice, included)
			if snapErr != nil { return fmt.Errorf("renewal snapshot %s/%s: %w", id, key, snapErr) }
			if err := a.markModuleRenewed(ctx, id, key, nextStart, nextEnd, nextPrice); err != nil { return err }
			existingStart, existingEnd, currentPrice = nextStart, nextEnd, nextPrice
		}

		if _, err := a.db.ExecContext(ctx, `UPDATE billing.module_subscriptions SET
				currency=$3,activation_date=$4,period_start=$5,period_end=$6,price=$7,
				auto_renew=TRUE,cancel_at_period_end=FALSE,payment_status='PENDING',updated_at=NOW()
			WHERE partner_id=$1 AND module_key=$2`,
			id, key, currency, activation, existingStart, existingEnd, currentPrice); err != nil {
			return err
		}
	}

	if len(activeKeys) == 0 {
		_, err := a.db.ExecContext(ctx, `UPDATE billing.module_subscriptions
			SET auto_renew=FALSE,cancel_at_period_end=FALSE,payment_status='INACTIVE',updated_at=NOW()
			WHERE partner_id=$1 AND payment_status<>'INACTIVE'`, id)
		return err
	}
	args := make([]any, 0, len(activeKeys)+1)
	args = append(args, id)
	placeholders := make([]string, len(activeKeys))
	for i, key := range activeKeys {
		args = append(args, key)
		placeholders[i] = fmt.Sprintf("$%d", i+2)
	}
	_, err := a.db.ExecContext(ctx, `UPDATE billing.module_subscriptions
		SET auto_renew=FALSE,cancel_at_period_end=FALSE,payment_status='INACTIVE',updated_at=NOW()
		WHERE partner_id=$1 AND module_key NOT IN (`+strings.Join(placeholders, ",")+`) AND payment_status<>'INACTIVE'`, args...)
	return err
}

func (a *app) subscriptions(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet { common.APIError(w, 405, "METHOD", "Use GET"); return }
	rows, err := a.db.Query(`SELECT module_key,currency,activation_date,period_start,period_end,price,auto_renew,cancel_at_period_end,payment_status,updated_at FROM billing.module_subscriptions WHERE partner_id=$1 ORDER BY module_key`, id)
	if err != nil { common.APIError(w, 500, "DB", "Could not load subscriptions"); return }
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var key, currency, payment string
		var activation, start, end, updated time.Time
		var price float64
		var renew, cancel bool
		if rows.Scan(&key, &currency, &activation, &start, &end, &price, &renew, &cancel, &payment, &updated) == nil {
			items = append(items, map[string]any{
				"module_key": key, "currency": currency, "activation_date": activation.Format("2006-01-02"),
				"period_start": start.Format("2006-01-02"), "period_end_exclusive": end.Format("2006-01-02"),
				"price": price, "auto_renew": renew, "cancel_at_period_end": cancel, "payment_status": payment, "updated_at": updated,
			})
		}
	}
	common.JSON(w, 200, map[string]any{"items": items})
}

func (a *app) subscriptionByKey(w http.ResponseWriter, r *http.Request, id, moduleKey string) {
	if r.Method != http.MethodPatch {
		common.APIError(w, 405, "METHOD", "Use PATCH")
		return
	}
	moduleKey = strings.TrimSpace(moduleKey)
	if moduleKey == "" {
		common.APIError(w, 404, "NOT_FOUND", "Subscription not found")
		return
	}
	var in struct {
		CancelAtPeriodEnd *bool  `json:"cancel_at_period_end"`
		Reason            string `json:"reason"`
	}
	if common.Decode(r, &in) != nil || in.CancelAtPeriodEnd == nil {
		common.APIError(w, 400, "VALIDATION", "cancel_at_period_end is required")
		return
	}

	tx, err := a.db.BeginTx(r.Context(), &sql.TxOptions{})
	if err != nil {
		common.APIError(w, 500, "DB", "Could not start subscription update")
		return
	}
	defer tx.Rollback()

	var oldRenew, oldCancel bool
	var periodEnd time.Time
	var paymentStatus string
	if err = tx.QueryRow(`SELECT auto_renew,cancel_at_period_end,period_end,payment_status
		FROM billing.module_subscriptions WHERE partner_id=$1 AND module_key=$2 FOR UPDATE`, id, moduleKey).
		Scan(&oldRenew, &oldCancel, &periodEnd, &paymentStatus); err != nil {
		if err == sql.ErrNoRows {
			common.APIError(w, 404, "NOT_FOUND", "Subscription not found; activate the module first")
		} else {
			common.APIError(w, 500, "DB", "Could not load subscription")
		}
		return
	}

	nextCancel := *in.CancelAtPeriodEnd
	nextRenew := !nextCancel
	if paymentStatus == "INACTIVE" && !nextCancel {
		common.APIError(w, 409, "SUBSCRIPTION_INACTIVE", "Reactivate the module before enabling renewal")
		return
	}
	if oldCancel != nextCancel || oldRenew != nextRenew {
		if _, err = tx.Exec(`UPDATE billing.module_subscriptions
			SET auto_renew=$3,cancel_at_period_end=$4,updated_at=NOW()
			WHERE partner_id=$1 AND module_key=$2`, id, moduleKey, nextRenew, nextCancel); err != nil {
			common.APIError(w, 500, "DB", "Could not update subscription")
			return
		}
		actor := strings.TrimSpace(r.Header.Get("X-Himate-User-ID"))
		reason := strings.TrimSpace(in.Reason)
		if reason == "" {
			if nextCancel { reason = "Cancel at current period end" } else { reason = "Cancellation withdrawn" }
		}
		if _, err = tx.Exec(`INSERT INTO billing.subscription_history(
				partner_id,module_key,old_auto_renew,new_auto_renew,old_cancel_at_period_end,new_cancel_at_period_end,actor,reason
			) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`,
			id, moduleKey, oldRenew, nextRenew, oldCancel, nextCancel, actor, reason); err != nil {
			common.APIError(w, 500, "DB", "Could not record subscription history")
			return
		}
		eventType := "MODULE_CANCELLATION_WITHDRAWN"
		if nextCancel { eventType = "MODULE_CANCELLATION_SCHEDULED" }
		eventKey := fmt.Sprintf("%s:%s:%s:%s", eventType, id, moduleKey, dateOnly(periodEnd).Format("2006-01-02"))
		if err = emitBillingEventTx(r.Context(), tx, eventKey, id, moduleKey, eventType, time.Now().UTC(), map[string]any{
			"period_end_exclusive": dateOnly(periodEnd).Format("2006-01-02"),
			"actor": actor, "reason": reason,
		}); err != nil {
			common.APIError(w, 500, "DB", "Could not record billing event")
			return
		}
	}
	if err = tx.Commit(); err != nil {
		common.APIError(w, 500, "DB", "Could not commit subscription update")
		return
	}
	common.JSON(w, 200, map[string]any{
		"partner_id": id, "module_key": moduleKey, "auto_renew": nextRenew,
		"cancel_at_period_end": nextCancel, "period_end_exclusive": periodEnd.Format("2006-01-02"),
		"payment_status": paymentStatus,
	})
}


func (a *app) documents(w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query(`SELECT id,kind,name,storage_url,note,uploaded_by,verified_by,mime_type,sha256,size_bytes,created_at FROM billing.documents WHERE partner_id=$1 ORDER BY created_at DESC`, id)
		if err != nil { common.APIError(w, 500, "DB", "Could not load documents"); return }
		defer rows.Close()
		items := []map[string]any{}
		for rows.Next() {
			var docID, size int64
			var kind, name, url, note, uploadedBy, verifiedBy, mimeType, sha256 string
			var created time.Time
			if rows.Scan(&docID, &kind, &name, &url, &note, &uploadedBy, &verifiedBy, &mimeType, &sha256, &size, &created) == nil {
				items = append(items, map[string]any{
					"id": docID, "partner_id": id, "kind": kind, "name": name, "storage_url": url, "note": note,
					"uploaded_by": uploadedBy, "verified_by": verifiedBy, "mime_type": mimeType, "sha256": sha256, "size_bytes": size, "created_at": created,
				})
			}
		}
		common.JSON(w, 200, map[string]any{"items": items})
	case http.MethodPost:
		var in struct {
			Kind       string `json:"kind"`
			Name       string `json:"name"`
			StorageURL string `json:"storage_url"`
			Note       string `json:"note"`
			VerifiedBy string `json:"verified_by"`
			MIMEType   string `json:"mime_type"`
			SHA256     string `json:"sha256"`
			SizeBytes  int64  `json:"size_bytes"`
		}
		if common.Decode(r, &in) != nil || strings.TrimSpace(in.Kind) == "" || strings.TrimSpace(in.Name) == "" {
			common.APIError(w, 400, "VALIDATION", "Document kind and name are required")
			return
		}
		kind := strings.ToUpper(strings.TrimSpace(in.Kind))
		storageReference := strings.TrimSpace(in.StorageURL)
		if isCommercialEvidenceKind(kind) && storageReference == "" {
			common.APIError(w, 400, "EVIDENCE_REFERENCE_REQUIRED", "Commercial evidence requires an attached storage URL or persistent document reference")
			return
		}
		if len(storageReference) > 2048 {
			common.APIError(w, 400, "VALIDATION", "Document storage reference is too long")
			return
		}
		if in.SizeBytes < 0 { common.APIError(w, 400, "VALIDATION", "Document size cannot be negative"); return }
		uploadedBy := strings.TrimSpace(r.Header.Get("X-Himate-User-ID"))
		var docID int64
		var created time.Time
		tx, err := a.db.BeginTx(r.Context(), &sql.TxOptions{})
		if err != nil { common.APIError(w, 500, "DB", "Could not start document registration"); return }
		defer tx.Rollback()
		err = tx.QueryRow(`INSERT INTO billing.documents(partner_id,kind,name,storage_url,note,uploaded_by,verified_by,mime_type,sha256,size_bytes)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id,created_at`,
			id, kind, strings.TrimSpace(in.Name), storageReference, strings.TrimSpace(in.Note),
			uploadedBy, strings.TrimSpace(in.VerifiedBy), strings.TrimSpace(in.MIMEType), strings.TrimSpace(in.SHA256), in.SizeBytes).Scan(&docID, &created)
		if err != nil { common.APIError(w, 500, "DB", "Could not register document"); return }
		if isCommercialEvidenceKind(kind) {
			eventType := "COMMERCIAL_EVIDENCE_REGISTERED"
			if kind == "INVOICE" { eventType = "ACTIVATION_INVOICE_REGISTERED" }
			if kind == "PAYMENT_EVIDENCE" || kind == "RECEIPT" { eventType = "PAYMENT_EVIDENCE_REGISTERED" }
			if err = emitBillingEventTx(r.Context(), tx,
				fmt.Sprintf("%s:%s:%d", eventType, id, docID),
				id, "", eventType, created, map[string]any{
					"document_id": docID, "kind": kind, "name": strings.TrimSpace(in.Name),
					"storage_reference": storageReference, "uploaded_by": uploadedBy,
				}); err != nil {
				common.APIError(w, 500, "DB", "Could not record commercial evidence event")
				return
			}
		}
		if err = tx.Commit(); err != nil { common.APIError(w, 500, "DB", "Could not commit document registration"); return }
		common.JSON(w, 201, map[string]any{"id": docID, "partner_id": id, "kind": kind, "name": strings.TrimSpace(in.Name), "storage_url": storageReference, "note": strings.TrimSpace(in.Note), "uploaded_by": uploadedBy, "created_at": created})
	default:
		common.APIError(w, 405, "METHOD", "Use GET or POST")
	}
}

func (a *app) invoices(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet { common.APIError(w, 405, "METHOD", "Use GET"); return }
	rows, err := a.db.Query(`SELECT id,invoice_date,service_period_start,service_period_end,currency,base_fee,module_fee,total,status,provider_status,created_at FROM billing.invoices WHERE partner_id=$1 ORDER BY invoice_date DESC`, id)
	if err != nil { common.APIError(w, 500, "DB", "Could not load invoices"); return }
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var invoiceID, currency, status, provider string
		var invoiceDate, start, end, created time.Time
		var base, module, total float64
		if rows.Scan(&invoiceID, &invoiceDate, &start, &end, &currency, &base, &module, &total, &status, &provider, &created) == nil {
			items = append(items, map[string]any{
				"id": invoiceID, "invoice_date": invoiceDate, "service_period_start": start, "service_period_end_exclusive": end,
				"currency": currency, "base_fee": base, "module_fee": module, "total": total, "status": status, "provider_status": provider, "created_at": created,
				"items": a.invoiceItemsFor(invoiceID),
			})
		}
	}
	common.JSON(w, 200, map[string]any{"items": items})
}

func (a *app) runEndpoint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { common.APIError(w, 405, "METHOD", "Use POST"); return }
	var in struct { Date string `json:"date"` }
	_ = common.Decode(r, &in)
	at := time.Now().UTC()
	if in.Date != "" {
		p, err := time.Parse("2006-01-02", in.Date)
		if err != nil { common.APIError(w, 400, "VALIDATION", "date must be YYYY-MM-DD"); return }
		at = p
	}
	if err := a.runInvoiceCycle(r.Context(), at); err != nil { common.APIError(w, 500, "INVOICE", "Invoice cycle failed"); return }
	common.JSON(w, 200, map[string]any{"status": "completed", "run_date": dateOnly(at).Format("2006-01-02")})
}

func isCycleBoundary(anchor, at time.Time) bool {
	anchor, at = dateOnly(anchor), dateOnly(at)
	if !at.After(anchor) { return false }
	days := int(at.Sub(anchor).Hours() / 24)
	return days > 0 && days%30 == 0
}

func (a *app) runInvoiceCycle(ctx context.Context, at time.Time) error {
	at = dateOnly(at)
	rows, err := a.db.QueryContext(ctx, `SELECT partner_id FROM billing.partner_terms`)
	if err != nil { return err }
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil { ids = append(ids, id) }
	}
	for _, id := range ids {
		t, err := a.ensureTerms(id)
		if err != nil { return err }

		// Subscription lifecycle is evaluated every daily cron run, not only at
		// the partner base-fee boundary. This guarantees that a module whose
		// own 30-day period ends today is cancelled on time.
		_, rawMods, err := a.catalogFees(ctx, id)
		if err != nil { return err }
		if err := a.syncSubscriptions(ctx, id, t.Currency, rawMods, at); err != nil { return err }

		if !isCycleBoundary(t.ServiceAnchorDate, at) { continue }
		start := at.AddDate(0, 0, -30)
		base := effectiveBaseFee(t, start)
		invoiceID := "inv_" + strings.ReplaceAll(id, "_", "") + "_" + at.Format("20060102")
		result, err := a.db.ExecContext(ctx, `INSERT INTO billing.invoices(id,partner_id,invoice_date,service_period_start,service_period_end,currency,base_fee,module_fee,total)
			VALUES($1,$2,$3,$4,$5,$6,$7,0,$7) ON CONFLICT(partner_id,service_period_start,service_period_end) DO NOTHING`,
			invoiceID, id, at, start, at, t.Currency, base)
		if err != nil { return err }
		inserted, _ := result.RowsAffected()
		// A rerun must keep the originally invoiced base price immutable.
		if err := a.db.QueryRowContext(ctx, `SELECT id,base_fee FROM billing.invoices
			WHERE partner_id=$1 AND service_period_start=$2 AND service_period_end=$3`,
			id, start, at).Scan(&invoiceID, &base); err != nil { return err }
		moduleTotal, err := a.attachInvoiceItems(ctx, invoiceID, id, t.Currency, start, at, base)
		if err != nil { return err }
		total := math.Round((base+moduleTotal)*100)/100
		if _, err := a.db.ExecContext(ctx, `UPDATE billing.invoices SET module_fee=$2,total=$3 WHERE id=$1`,
			invoiceID, moduleTotal, total); err != nil { return err }
		if inserted > 0 {
			if err := a.emitBillingEvent(ctx, "INVOICE_GENERATED:"+invoiceID, id, "", "INVOICE_GENERATED", at, map[string]any{
				"invoice_id": invoiceID, "currency": t.Currency, "base_fee": base,
				"module_fee": moduleTotal, "total": total,
				"service_period_start": start.Format("2006-01-02"),
				"service_period_end_exclusive": at.Format("2006-01-02"),
			}); err != nil { return err }
		}
	}
	return nil
}

func (a *app) portfolio(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { common.APIError(w, 405, "METHOD", "Use GET"); return }
	query := `SELECT partner_id,currency,activation_fee,activation_fee_waived,activation_fee_reason,base_monthly_fee,annual_increase_percent,cycle_days,invoice_day,price_effective_from,service_anchor_date,updated_at FROM billing.partner_terms`
	args := []any{}
	if ids := strings.TrimSpace(r.URL.Query().Get("ids")); ids != "" {
		query += ` WHERE partner_id = ANY(string_to_array($1, ','))`
		args = append(args, ids)
	}
	query += ` ORDER BY partner_id`
	rows, err := a.db.Query(query, args...)
	if err != nil { common.APIError(w, 500, "DB", "Could not load billing portfolio"); return }
	defer rows.Close()
	items := []map[string]any{}
	now := time.Now().UTC()
	for rows.Next() {
		var t terms
		if rows.Scan(&t.PartnerID, &t.Currency, &t.ActivationFee, &t.ActivationFeeWaived, &t.ActivationFeeReason, &t.BaseMonthlyFee, &t.AnnualIncreasePercent, &t.CycleDays, &t.InvoiceDay, &t.PriceEffectiveFrom, &t.ServiceAnchorDate, &t.UpdatedAt) == nil {
			items = append(items, map[string]any{"partner_id": t.PartnerID, "effective_base_fee": effectiveBaseFee(t, now), "currency": t.Currency, "updated_at": t.UpdatedAt})
		}
	}
	common.JSON(w, 200, map[string]any{"items": items})
}
