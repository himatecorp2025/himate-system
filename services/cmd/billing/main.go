package main

import (
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
	db          *sql.DB
	catalogHost string
	token       string
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
	UpdatedAt             time.Time
}

func main() {
	log := common.Logger()
	db, err := common.OpenDB()
	if err != nil {
		log.Error("database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	a := &app{db: db, catalogHost: os.Getenv("CATALOG_HOSTPORT"), token: os.Getenv("HIMATE_INTERNAL_TOKEN"), client: &http.Client{Timeout: 10 * time.Second}}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := a.migrate(ctx); err != nil {
		log.Error("migration", "error", err)
		os.Exit(1)
	}
	if len(os.Args) > 1 && os.Args[1] == "--run-invoice-cycle" {
		if err := a.runInvoiceCycle(context.Background(), time.Now().UTC()); err != nil {
			log.Error("invoice cycle", "error", err)
			os.Exit(1)
		}
		return
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		common.JSON(w, 200, map[string]any{"status": "ok", "service": "billing", "invoice_day": 1, "cycle_days": 30, "annual_increase_date": "January 1"})
	})
	mux.HandleFunc("/api/v1/billing/profile", a.profile)
	mux.HandleFunc("/api/v1/billing/partners/", a.partnerRoutes)
	mux.HandleFunc("/internal/v1/invoices/run", a.runEndpoint)
	common.Run(log, "billing", common.Env("PORT", "10000"), common.InternalAuth(a.token, mux))
}

func (a *app) migrate(ctx context.Context) error {
	if err := common.ExecStatements(ctx, a.db,
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
	); err != nil {
		return err
	}
	_, err := a.db.ExecContext(ctx, `INSERT INTO billing.partner_terms(partner_id,currency,activation_fee,activation_fee_waived,activation_fee_reason,base_monthly_fee,annual_increase_percent,cycle_days,invoice_day,price_effective_from)
        VALUES('ptr_000001','USD',0,TRUE,'Existing reference partner; activation fee not applicable',2000,10,30,1,'2026-01-01')
        ON CONFLICT(partner_id) DO NOTHING`)
	return err
}

func (a *app) profile(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		var legal, address, taxID, email, bank, bankAddr, account, iban, swift string
		err := a.db.QueryRow(`SELECT legal_name,address,tax_id,email,bank_name,bank_address,account_number,iban,swift FROM billing.company_profile WHERE id=1`).Scan(&legal, &address, &taxID, &email, &bank, &bankAddr, &account, &iban, &swift)
		if err != nil {
			common.APIError(w, 500, "DB", "Could not load billing profile")
			return
		}
		common.JSON(w, 200, map[string]any{"legal_name": legal, "address": address, "tax_id": taxID, "email": email, "bank_name": bank, "bank_address": bankAddr, "account_number": account, "iban": iban, "swift": swift})
	case http.MethodPut:
		var in struct {
			LegalName     string `json:"legal_name"`
			Address       string `json:"address"`
			TaxID         string `json:"tax_id"`
			Email         string `json:"email"`
			BankName      string `json:"bank_name"`
			BankAddress   string `json:"bank_address"`
			AccountNumber string `json:"account_number"`
			IBAN          string `json:"iban"`
			SWIFT         string `json:"swift"`
		}
		if common.Decode(r, &in) != nil {
			common.APIError(w, 400, "JSON", "Invalid request")
			return
		}
		_, err := a.db.Exec(`UPDATE billing.company_profile SET legal_name=$1,address=$2,tax_id=$3,email=$4,bank_name=$5,bank_address=$6,account_number=$7,iban=$8,swift=$9,updated_at=NOW() WHERE id=1`, in.LegalName, in.Address, in.TaxID, in.Email, in.BankName, in.BankAddress, in.AccountNumber, in.IBAN, in.SWIFT)
		if err != nil {
			common.APIError(w, 500, "DB", "Could not update billing profile")
			return
		}
		common.JSON(w, 200, map[string]any{"legal_name": in.LegalName, "address": in.Address, "tax_id": in.TaxID, "email": in.Email, "bank_name": in.BankName, "bank_address": in.BankAddress, "account_number": in.AccountNumber, "iban": in.IBAN, "swift": in.SWIFT})
	default:
		common.APIError(w, 405, "METHOD", "Use GET or PUT")
	}
}

func (a *app) ensureTerms(id string) (terms, error) {
	if _, err := a.db.Exec(`INSERT INTO billing.partner_terms(partner_id) VALUES($1) ON CONFLICT(partner_id) DO NOTHING`, id); err != nil {
		return terms{}, err
	}
	var t terms
	err := a.db.QueryRow(`SELECT partner_id,currency,activation_fee,activation_fee_waived,activation_fee_reason,base_monthly_fee,annual_increase_percent,cycle_days,invoice_day,price_effective_from,updated_at FROM billing.partner_terms WHERE partner_id=$1`, id).Scan(&t.PartnerID, &t.Currency, &t.ActivationFee, &t.ActivationFeeWaived, &t.ActivationFeeReason, &t.BaseMonthlyFee, &t.AnnualIncreasePercent, &t.CycleDays, &t.InvoiceDay, &t.PriceEffectiveFrom, &t.UpdatedAt)
	return t, err
}

func (a *app) partnerRoutes(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/billing/partners/"), "/"), "/")
	if len(parts) != 2 {
		common.APIError(w, 404, "NOT_FOUND", "Route not found")
		return
	}
	id, section := parts[0], parts[1]
	switch section {
	case "terms":
		a.terms(w, r, id)
	case "summary":
		a.summary(w, r, id)
	case "documents":
		a.documents(w, r, id)
	case "invoices":
		a.invoices(w, r, id)
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
			Currency              string  `json:"currency"`
			ActivationFee         float64 `json:"activation_fee"`
			ActivationFeeWaived   bool    `json:"activation_fee_waived"`
			ActivationFeeReason   string  `json:"activation_fee_reason"`
			BaseMonthlyFee        float64 `json:"base_monthly_fee"`
			AnnualIncreasePercent float64 `json:"annual_increase_percent"`
			PriceEffectiveFrom    string  `json:"price_effective_from"`
		}
		if common.Decode(r, &in) != nil {
			common.APIError(w, 400, "JSON", "Invalid request")
			return
		}
		if in.Currency == "" {
			in.Currency = current.Currency
		}
		if in.AnnualIncreasePercent == 0 {
			in.AnnualIncreasePercent = current.AnnualIncreasePercent
		}
		effective := current.PriceEffectiveFrom
		if strings.TrimSpace(in.PriceEffectiveFrom) != "" {
			if parsed, e := time.Parse("2006-01-02", in.PriceEffectiveFrom); e == nil {
				effective = parsed
			} else if parsed, e := time.Parse(time.RFC3339, in.PriceEffectiveFrom); e == nil {
				effective = parsed
			}
		}
		if in.ActivationFee < 0 || in.BaseMonthlyFee < 0 || in.AnnualIncreasePercent < 0 {
			common.APIError(w, 400, "VALIDATION", "Commercial amounts cannot be negative")
			return
		}
		if !in.ActivationFeeWaived && in.ActivationFee < 13000 {
			common.APIError(w, 400, "VALIDATION", "New partner activation fee must be at least USD 13,000 unless explicitly waived")
			return
		}
		_, err = a.db.Exec(`UPDATE billing.partner_terms SET currency=$2,activation_fee=$3,activation_fee_waived=$4,activation_fee_reason=$5,base_monthly_fee=$6,annual_increase_percent=$7,cycle_days=30,invoice_day=1,price_effective_from=$8,updated_at=NOW() WHERE partner_id=$1`, id, in.Currency, in.ActivationFee, in.ActivationFeeWaived, in.ActivationFeeReason, in.BaseMonthlyFee, in.AnnualIncreasePercent, effective)
		if err != nil {
			common.APIError(w, 500, "DB", "Could not update terms")
			return
		}
		t, _ := a.ensureTerms(id)
		common.JSON(w, 200, termsMap(t))
	default:
		common.APIError(w, 405, "METHOD", "Use GET or PUT")
	}
}

func termsMap(t terms) map[string]any {
	return map[string]any{"partner_id": t.PartnerID, "currency": t.Currency, "activation_fee": t.ActivationFee, "activation_fee_waived": t.ActivationFeeWaived, "activation_fee_reason": t.ActivationFeeReason, "base_monthly_fee": t.BaseMonthlyFee, "annual_increase_percent": t.AnnualIncreasePercent, "annual_increase_month": 1, "annual_increase_day": 1, "cycle_days": 30, "invoice_day": 1, "price_effective_from": t.PriceEffectiveFrom.Format("2006-01-02"), "updated_at": t.UpdatedAt}
}

func effectiveBaseFee(t terms, at time.Time) float64 {
	value := t.BaseMonthlyFee
	for year := t.PriceEffectiveFrom.Year() + 1; year <= at.Year(); year++ {
		jan1 := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
		if !at.Before(jan1) {
			value *= 1 + t.AnnualIncreasePercent/100
		}
	}
	return math.Round(value*100) / 100
}

func (a *app) summary(w http.ResponseWriter, r *http.Request, id string) {
	t, err := a.ensureTerms(id)
	if err != nil {
		common.APIError(w, 500, "DB", "Could not load terms")
		return
	}
	extra, mods, err := a.catalogFees(r.Context(), id)
	if err != nil {
		common.APIError(w, 502, "CATALOG", "Could not load billable modules")
		return
	}
	base := effectiveBaseFee(t, time.Now().UTC())
	common.JSON(w, 200, map[string]any{"partner_id": id, "currency": t.Currency, "effective_base_fee": base, "extra_module_fee": extra, "current_total": math.Round((base+extra)*100) / 100, "annual_increase_percent": t.AnnualIncreasePercent, "annual_increase_date": "January 1", "cycle_days": 30, "invoice_day": 1, "service_period": "preceding 30 days", "modules": mods})
}

func (a *app) catalogFees(ctx context.Context, id string) (float64, []map[string]any, error) {
	if strings.TrimSpace(a.catalogHost) == "" {
		return 0, nil, fmt.Errorf("CATALOG_HOSTPORT is required")
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+a.catalogHost+"/internal/v1/partners/"+id+"/billable-modules", nil)
	req.Header.Set("X-Himate-Internal-Token", a.token)
	resp, err := a.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return 0, nil, fmt.Errorf("catalog status %d", resp.StatusCode)
	}
	var out struct {
		Extra float64          `json:"extra_monthly_total"`
		Items []map[string]any `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0, nil, err
	}
	return out.Extra, out.Items, nil
}

func (a *app) documents(w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query(`SELECT id,kind,name,storage_url,note,created_at FROM billing.documents WHERE partner_id=$1 ORDER BY created_at DESC`, id)
		if err != nil {
			common.APIError(w, 500, "DB", "Could not load documents")
			return
		}
		defer rows.Close()
		items := []map[string]any{}
		for rows.Next() {
			var docID int64
			var kind, name, url, note string
			var created time.Time
			if rows.Scan(&docID, &kind, &name, &url, &note, &created) == nil {
				items = append(items, map[string]any{"id": docID, "partner_id": id, "kind": kind, "name": name, "storage_url": url, "note": note, "created_at": created})
			}
		}
		common.JSON(w, 200, map[string]any{"items": items})
	case http.MethodPost:
		var in struct {
			Kind       string `json:"kind"`
			Name       string `json:"name"`
			StorageURL string `json:"storage_url"`
			Note       string `json:"note"`
		}
		if common.Decode(r, &in) != nil || strings.TrimSpace(in.Kind) == "" || strings.TrimSpace(in.Name) == "" {
			common.APIError(w, 400, "VALIDATION", "Document kind and name are required")
			return
		}
		var docID int64
		var created time.Time
		err := a.db.QueryRow(`INSERT INTO billing.documents(partner_id,kind,name,storage_url,note) VALUES($1,$2,$3,$4,$5) RETURNING id,created_at`, id, in.Kind, in.Name, in.StorageURL, in.Note).Scan(&docID, &created)
		if err != nil {
			common.APIError(w, 500, "DB", "Could not register document")
			return
		}
		common.JSON(w, 201, map[string]any{"id": docID, "partner_id": id, "kind": in.Kind, "name": in.Name, "storage_url": in.StorageURL, "note": in.Note, "created_at": created})
	default:
		common.APIError(w, 405, "METHOD", "Use GET or POST")
	}
}

func (a *app) invoices(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		common.APIError(w, 405, "METHOD", "Use GET")
		return
	}
	rows, err := a.db.Query(`SELECT id,invoice_date,service_period_start,service_period_end,currency,base_fee,module_fee,total,status,provider_status,created_at FROM billing.invoices WHERE partner_id=$1 ORDER BY invoice_date DESC`, id)
	if err != nil {
		common.APIError(w, 500, "DB", "Could not load invoices")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var invoiceID, currency, status, provider string
		var invoiceDate, start, end, created time.Time
		var base, module, total float64
		if rows.Scan(&invoiceID, &invoiceDate, &start, &end, &currency, &base, &module, &total, &status, &provider, &created) == nil {
			items = append(items, map[string]any{"id": invoiceID, "invoice_date": invoiceDate, "service_period_start": start, "service_period_end": end, "currency": currency, "base_fee": base, "module_fee": module, "total": total, "status": status, "provider_status": provider, "created_at": created})
		}
	}
	common.JSON(w, 200, map[string]any{"items": items})
}

func (a *app) runEndpoint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		common.APIError(w, 405, "METHOD", "Use POST")
		return
	}
	var in struct {
		Date string `json:"date"`
	}
	_ = common.Decode(r, &in)
	at := time.Now().UTC()
	if in.Date != "" {
		if p, err := time.Parse("2006-01-02", in.Date); err == nil {
			at = p
		}
	}
	if err := a.runInvoiceCycle(r.Context(), at); err != nil {
		common.APIError(w, 500, "INVOICE", "Invoice cycle failed")
		return
	}
	common.JSON(w, 200, map[string]any{"status": "completed", "invoice_date": at.Format("2006-01-02")})
}

func (a *app) runInvoiceCycle(ctx context.Context, at time.Time) error {
	if at.Day() != 1 {
		return nil
	}
	rows, err := a.db.QueryContext(ctx, `SELECT partner_id FROM billing.partner_terms WHERE invoice_day=1`)
	if err != nil {
		return err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	for _, id := range ids {
		t, err := a.ensureTerms(id)
		if err != nil {
			return err
		}
		extra, _, err := a.catalogFees(ctx, id)
		if err != nil {
			return err
		}
		base := effectiveBaseFee(t, at)
		invoiceDate := time.Date(at.Year(), at.Month(), 1, 0, 0, 0, 0, time.UTC)
		start := invoiceDate.AddDate(0, 0, -30)
		invoiceID := "inv_" + strings.ReplaceAll(id, "_", "") + "_" + invoiceDate.Format("20060102")
		_, err = a.db.ExecContext(ctx, `INSERT INTO billing.invoices(id,partner_id,invoice_date,service_period_start,service_period_end,currency,base_fee,module_fee,total) VALUES($1,$2,$3,$4,$3,$5,$6,$7,$8) ON CONFLICT(partner_id,invoice_date) DO NOTHING`, invoiceID, id, invoiceDate, start, t.Currency, base, extra, math.Round((base+extra)*100)/100)
		if err != nil {
			return err
		}
	}
	return nil
}
