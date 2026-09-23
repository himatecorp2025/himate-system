package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"himate.local/services/internal/common"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func start223BillingMigration() common.Migration {
	return common.Migration{
		Version: 5,
		Name: "start-22-3-commercial-automation",
		Statements: []string{
			`CREATE TABLE IF NOT EXISTS billing.commercial_agreements(
				partner_id TEXT PRIMARY KEY,
				status TEXT NOT NULL DEFAULT 'DRAFT',
				agreement_reference TEXT NOT NULL DEFAULT '',
				note TEXT NOT NULL DEFAULT '',
				agreed_at TIMESTAMPTZ,
				agreed_by TEXT NOT NULL DEFAULT '',
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE TABLE IF NOT EXISTS billing.module_period_snapshots(
				id BIGSERIAL PRIMARY KEY,
				partner_id TEXT NOT NULL,
				module_key TEXT NOT NULL,
				period_start DATE NOT NULL,
				period_end DATE NOT NULL,
				currency TEXT NOT NULL,
				price_snapshot NUMERIC(12,2) NOT NULL,
				included_in_base BOOLEAN NOT NULL DEFAULT FALSE,
				source TEXT NOT NULL DEFAULT 'CATALOG_EFFECTIVE_PRICE_HISTORY',
				captured_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				UNIQUE(partner_id,module_key,period_start)
			)`,
			`CREATE INDEX IF NOT EXISTS billing_module_period_partner_idx
				ON billing.module_period_snapshots(partner_id,period_start DESC,module_key)`,
			`CREATE TABLE IF NOT EXISTS billing.invoice_items(
				id BIGSERIAL PRIMARY KEY,
				item_key TEXT NOT NULL UNIQUE,
				invoice_id TEXT,
				partner_id TEXT NOT NULL,
				module_key TEXT,
				item_type TEXT NOT NULL,
				description TEXT NOT NULL,
				currency TEXT NOT NULL,
				quantity NUMERIC(12,4) NOT NULL DEFAULT 1,
				unit_price NUMERIC(12,2) NOT NULL,
				amount NUMERIC(12,2) NOT NULL,
				period_start DATE NOT NULL,
				period_end DATE NOT NULL,
				snapshot_id BIGINT REFERENCES billing.module_period_snapshots(id),
				status TEXT NOT NULL DEFAULT 'PENDING',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				invoiced_at TIMESTAMPTZ
			)`,
			`CREATE INDEX IF NOT EXISTS billing_invoice_items_partner_pending_idx
				ON billing.invoice_items(partner_id,status,period_start) WHERE invoice_id IS NULL`,
			`CREATE INDEX IF NOT EXISTS billing_invoice_items_invoice_idx
				ON billing.invoice_items(invoice_id,id)`,
			`CREATE TABLE IF NOT EXISTS billing.billing_events(
				id BIGSERIAL PRIMARY KEY,
				event_key TEXT NOT NULL UNIQUE,
				partner_id TEXT NOT NULL,
				module_key TEXT,
				event_type TEXT NOT NULL,
				effective_at TIMESTAMPTZ NOT NULL,
				payload JSONB NOT NULL DEFAULT '{}'::jsonb,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE INDEX IF NOT EXISTS billing_events_partner_idx
				ON billing.billing_events(partner_id,effective_at DESC,id DESC)`,
		},
	}
}

func start223BillingImmutabilityMigration() common.Migration {
	return common.Migration{
		Version: 6,
		Name: "start-22-3-immutable-ledger-guards",
		Statements: []string{
			`CREATE OR REPLACE FUNCTION billing.reject_module_snapshot_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
			BEGIN
				RAISE EXCEPTION 'billing.module_period_snapshots is immutable';
				RETURN OLD;
			END; $$`,
			`DROP TRIGGER IF EXISTS billing_module_period_snapshots_immutable ON billing.module_period_snapshots`,
			`CREATE TRIGGER billing_module_period_snapshots_immutable
				BEFORE UPDATE OR DELETE ON billing.module_period_snapshots
				FOR EACH ROW EXECUTE FUNCTION billing.reject_module_snapshot_mutation()`,
			`CREATE OR REPLACE FUNCTION billing.reject_billing_event_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
			BEGIN
				RAISE EXCEPTION 'billing.billing_events is append-only';
				RETURN OLD;
			END; $$`,
			`DROP TRIGGER IF EXISTS billing_billing_events_append_only ON billing.billing_events`,
			`CREATE TRIGGER billing_billing_events_append_only
				BEFORE UPDATE OR DELETE ON billing.billing_events
				FOR EACH ROW EXECUTE FUNCTION billing.reject_billing_event_mutation()`,
			`CREATE OR REPLACE FUNCTION billing.guard_invoice_item_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
			BEGIN
				IF TG_OP='DELETE' THEN
					RAISE EXCEPTION 'billing.invoice_items is append-only';
				END IF;
				IF OLD.item_key IS DISTINCT FROM NEW.item_key
					OR OLD.partner_id IS DISTINCT FROM NEW.partner_id
					OR OLD.module_key IS DISTINCT FROM NEW.module_key
					OR OLD.item_type IS DISTINCT FROM NEW.item_type
					OR OLD.description IS DISTINCT FROM NEW.description
					OR OLD.currency IS DISTINCT FROM NEW.currency
					OR OLD.quantity IS DISTINCT FROM NEW.quantity
					OR OLD.unit_price IS DISTINCT FROM NEW.unit_price
					OR OLD.amount IS DISTINCT FROM NEW.amount
					OR OLD.period_start IS DISTINCT FROM NEW.period_start
					OR OLD.period_end IS DISTINCT FROM NEW.period_end
					OR OLD.snapshot_id IS DISTINCT FROM NEW.snapshot_id
					OR OLD.created_at IS DISTINCT FROM NEW.created_at THEN
					RAISE EXCEPTION 'billing.invoice_items commercial fields are immutable';
				END IF;
				IF OLD.invoice_id IS NOT NULL AND OLD.invoice_id IS DISTINCT FROM NEW.invoice_id THEN
					RAISE EXCEPTION 'billing.invoice_items invoice assignment is immutable once set';
				END IF;
				IF OLD.status='INVOICED' AND NEW.status IS DISTINCT FROM OLD.status THEN
					RAISE EXCEPTION 'billing.invoice_items invoiced status cannot be reversed';
				END IF;
				IF OLD.invoiced_at IS NOT NULL AND OLD.invoiced_at IS DISTINCT FROM NEW.invoiced_at THEN
					RAISE EXCEPTION 'billing.invoice_items invoiced_at is immutable once set';
				END IF;
				RETURN NEW;
			END; $$`,
			`DROP TRIGGER IF EXISTS billing_invoice_items_immutable_fields ON billing.invoice_items`,
			`CREATE TRIGGER billing_invoice_items_immutable_fields
				BEFORE UPDATE OR DELETE ON billing.invoice_items
				FOR EACH ROW EXECUTE FUNCTION billing.guard_invoice_item_mutation()`,
		},
	}
}

func start233BillingLifecycleMigration() common.Migration {
	return common.Migration{
		Version: 7,
		Name: "start-23-3-authoritative-subscription-lifecycle",
		Statements: []string{
			`ALTER TABLE billing.module_subscriptions ADD COLUMN IF NOT EXISTS lifecycle_state TEXT NOT NULL DEFAULT 'ACTIVE'`,
			`ALTER TABLE billing.module_subscriptions ADD COLUMN IF NOT EXISTS cancellation_requested_at TIMESTAMPTZ`,
			`ALTER TABLE billing.module_subscriptions ADD COLUMN IF NOT EXISTS cancellation_effective_at DATE`,
			`ALTER TABLE billing.module_subscriptions ADD COLUMN IF NOT EXISTS cancellation_requested_by TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE billing.module_subscriptions ADD COLUMN IF NOT EXISTS cancellation_reason TEXT NOT NULL DEFAULT ''`,
			`UPDATE billing.module_subscriptions
				SET lifecycle_state=CASE
					WHEN payment_status='INACTIVE' THEN 'INACTIVE'
					WHEN cancel_at_period_end THEN 'CANCEL_PENDING'
					ELSE 'ACTIVE'
				END
				WHERE lifecycle_state NOT IN ('ACTIVE','CANCEL_PENDING','INACTIVE') OR lifecycle_state IS NULL`,
			`UPDATE billing.module_subscriptions
				SET lifecycle_state='INACTIVE'
				WHERE payment_status='INACTIVE' AND lifecycle_state<>'INACTIVE'`,
			`UPDATE billing.module_subscriptions
				SET lifecycle_state='CANCEL_PENDING',cancellation_effective_at=period_end
				WHERE payment_status<>'INACTIVE' AND cancel_at_period_end=TRUE`,
			`ALTER TABLE billing.subscription_history ADD COLUMN IF NOT EXISTS old_lifecycle_state TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE billing.subscription_history ADD COLUMN IF NOT EXISTS new_lifecycle_state TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE billing.subscription_history ADD COLUMN IF NOT EXISTS period_end DATE`,
			`CREATE INDEX IF NOT EXISTS billing_subscription_lifecycle_idx
				ON billing.module_subscriptions(lifecycle_state,period_end,partner_id)`,
		},
	}
}

type billingEventExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func emitBillingEventWith(ctx context.Context, execer billingEventExecer, eventKey, partnerID, moduleKey, eventType string, effectiveAt time.Time, payload map[string]any) error {
	raw, _ := json.Marshal(payload)
	_, err := execer.ExecContext(ctx, `INSERT INTO billing.billing_events(event_key,partner_id,module_key,event_type,effective_at,payload)
		VALUES($1,$2,NULLIF($3,''),$4,$5,$6::jsonb)
		ON CONFLICT(event_key) DO NOTHING`,
		eventKey, partnerID, moduleKey, eventType, effectiveAt.UTC(), string(raw))
	return err
}

func (a *app) emitBillingEvent(ctx context.Context, eventKey, partnerID, moduleKey, eventType string, effectiveAt time.Time, payload map[string]any) error {
	return emitBillingEventWith(ctx, a.db, eventKey, partnerID, moduleKey, eventType, effectiveAt, payload)
}

func emitBillingEventTx(ctx context.Context, tx *sql.Tx, eventKey, partnerID, moduleKey, eventType string, effectiveAt time.Time, payload map[string]any) error {
	return emitBillingEventWith(ctx, tx, eventKey, partnerID, moduleKey, eventType, effectiveAt, payload)
}

func (a *app) modulePriceAt(ctx context.Context, partnerID, moduleKey string, at time.Time, fallbackPrice float64, fallbackIncluded bool, fallbackCurrency string) (float64, bool, string, error) {
	if strings.TrimSpace(a.catalogHost) == "" {
		return fallbackPrice, fallbackIncluded, fallbackCurrency, fmt.Errorf("CATALOG_HOSTPORT is required")
	}
	path := "/internal/v1/partners/" + url.PathEscape(partnerID) + "/modules/" + url.PathEscape(moduleKey) +
		"/price-at?at=" + url.QueryEscape(dateOnly(at).Format("2006-01-02"))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+a.catalogHost+path, nil)
	if err != nil { return fallbackPrice, fallbackIncluded, fallbackCurrency, err }
	common.BindInternalRequest(req, a.token)
	resp, err := a.client.Do(req)
	if err != nil { return fallbackPrice, fallbackIncluded, fallbackCurrency, err }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fallbackPrice, fallbackIncluded, fallbackCurrency, fmt.Errorf("catalog price-at status %d", resp.StatusCode)
	}
	var out struct {
		Price float64 `json:"price"`
		Currency string `json:"currency"`
		Included bool `json:"included_in_base"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return fallbackPrice, fallbackIncluded, fallbackCurrency, err
	}
	if strings.TrimSpace(out.Currency) == "" { out.Currency = fallbackCurrency }
	return out.Price, out.Included, out.Currency, nil
}

func (a *app) ensureModulePeriodSnapshot(
	ctx context.Context,
	partnerID, moduleKey, billingCurrency string,
	periodStart, periodEnd time.Time,
	fallbackPrice float64,
	fallbackIncluded bool,
) (int64, float64, bool, error) {
	periodStart = dateOnly(periodStart)
	periodEnd = dateOnly(periodEnd)
	var id int64
	var price float64
	var included bool
	var currency string
	err := a.db.QueryRowContext(ctx, `SELECT id,price_snapshot,included_in_base,currency
		FROM billing.module_period_snapshots
		WHERE partner_id=$1 AND module_key=$2 AND period_start=$3`,
		partnerID, moduleKey, periodStart).Scan(&id, &price, &included, &currency)
	if err == nil {
		if currency != billingCurrency {
			return 0, 0, false, fmt.Errorf("snapshot currency mismatch for %s/%s: %s != %s", partnerID, moduleKey, currency, billingCurrency)
		}
		return id, price, included, nil
	}
	if err != sql.ErrNoRows { return 0, 0, false, err }

	price, included, catalogCurrency, err := a.modulePriceAt(ctx, partnerID, moduleKey, periodStart, fallbackPrice, fallbackIncluded, billingCurrency)
	if err != nil { return 0, 0, false, err }
	if catalogCurrency != "" && billingCurrency != "" && !strings.EqualFold(catalogCurrency, billingCurrency) {
		return 0, 0, false, fmt.Errorf("module %s currency %s does not match partner billing currency %s", moduleKey, catalogCurrency, billingCurrency)
	}

	_, err = a.db.ExecContext(ctx, `INSERT INTO billing.module_period_snapshots(
			partner_id,module_key,period_start,period_end,currency,price_snapshot,included_in_base
		) VALUES($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT(partner_id,module_key,period_start) DO NOTHING`,
		partnerID, moduleKey, periodStart, periodEnd, billingCurrency, price, included)
	if err != nil { return 0, 0, false, err }
	if err = a.db.QueryRowContext(ctx, `SELECT id,price_snapshot,included_in_base,currency
		FROM billing.module_period_snapshots
		WHERE partner_id=$1 AND module_key=$2 AND period_start=$3`,
		partnerID, moduleKey, periodStart).Scan(&id, &price, &included, &currency); err != nil {
		return 0, 0, false, err
	}
	if currency != billingCurrency {
		return 0, 0, false, fmt.Errorf("snapshot currency mismatch for %s/%s: %s != %s", partnerID, moduleKey, currency, billingCurrency)
	}

	eventKey := fmt.Sprintf("MODULE_PERIOD_STARTED:%s:%s:%s", partnerID, moduleKey, periodStart.Format("2006-01-02"))
	if err := a.emitBillingEvent(ctx, eventKey, partnerID, moduleKey, "MODULE_PERIOD_STARTED", periodStart, map[string]any{
		"period_start": periodStart.Format("2006-01-02"),
		"period_end_exclusive": periodEnd.Format("2006-01-02"),
		"currency": billingCurrency,
		"price_snapshot": price,
		"included_in_base": included,
	}); err != nil { return 0, 0, false, err }

	if !included && price > 0 {
		itemKey := fmt.Sprintf("MODULE:%s:%s:%s", partnerID, moduleKey, periodStart.Format("2006-01-02"))
		result, err := a.db.ExecContext(ctx, `INSERT INTO billing.invoice_items(
				item_key,partner_id,module_key,item_type,description,currency,quantity,unit_price,amount,period_start,period_end,snapshot_id
			) VALUES($1,$2,$3,'MODULE',$4,$5,1,$6,$6,$7,$8,$9)
			ON CONFLICT(item_key) DO NOTHING`,
			itemKey, partnerID, moduleKey, "Module "+moduleKey+" · 30-day service", billingCurrency, price, periodStart, periodEnd, id)
		if err != nil { return 0, 0, false, err }
		if rows, _ := result.RowsAffected(); rows > 0 {
			if err := a.emitBillingEvent(ctx, "INVOICE_ITEM_CREATED:"+itemKey, partnerID, moduleKey, "INVOICE_ITEM_CREATED", time.Now().UTC(), map[string]any{
				"item_key": itemKey,
				"item_type": "MODULE",
				"amount": price,
				"currency": billingCurrency,
				"period_start": periodStart.Format("2006-01-02"),
				"period_end_exclusive": periodEnd.Format("2006-01-02"),
			}); err != nil { return 0, 0, false, err }
		}
	}
	return id, price, included, nil
}

func (a *app) closeModulePeriod(ctx context.Context, partnerID, moduleKey string, periodStart, periodEnd time.Time, cancelled bool) error {
	eventKey := fmt.Sprintf("MODULE_PERIOD_ENDED:%s:%s:%s", partnerID, moduleKey, dateOnly(periodEnd).Format("2006-01-02"))
	return a.emitBillingEvent(ctx, eventKey, partnerID, moduleKey, "MODULE_PERIOD_ENDED", dateOnly(periodEnd), map[string]any{
		"period_start": dateOnly(periodStart).Format("2006-01-02"),
		"period_end_exclusive": dateOnly(periodEnd).Format("2006-01-02"),
		"cancelled": cancelled,
	})
}

func (a *app) markModuleRenewed(ctx context.Context, partnerID, moduleKey string, start, end time.Time, price float64) error {
	key := fmt.Sprintf("MODULE_RENEWED:%s:%s:%s", partnerID, moduleKey, dateOnly(start).Format("2006-01-02"))
	return a.emitBillingEvent(ctx, key, partnerID, moduleKey, "MODULE_RENEWED", dateOnly(start), map[string]any{
		"period_start": dateOnly(start).Format("2006-01-02"),
		"period_end_exclusive": dateOnly(end).Format("2006-01-02"),
		"price_snapshot": price,
	})
}

func (a *app) agreement(w http.ResponseWriter, r *http.Request, partnerID string) {
	switch r.Method {
	case http.MethodGet:
		var status, reference, note, agreedBy string
		var agreedAt sql.NullTime
		var updated time.Time
		err := a.db.QueryRow(`SELECT status,agreement_reference,note,agreed_at,agreed_by,updated_at
			FROM billing.commercial_agreements WHERE partner_id=$1`, partnerID).
			Scan(&status, &reference, &note, &agreedAt, &agreedBy, &updated)
		if err == sql.ErrNoRows {
			common.JSON(w, 200, map[string]any{"partner_id": partnerID, "status": "DRAFT", "agreement_reference": "", "note": ""})
			return
		}
		if err != nil { common.APIError(w, 500, "DB", "Could not load commercial agreement"); return }
		var agreed any
		if agreedAt.Valid { agreed = agreedAt.Time.UTC() }
		common.JSON(w, 200, map[string]any{
			"partner_id": partnerID, "status": status, "agreement_reference": reference,
			"note": note, "agreed_at": agreed, "agreed_by": agreedBy, "updated_at": updated,
		})
	case http.MethodPut:
		var in struct {
			Status string `json:"status"`
			AgreementReference string `json:"agreement_reference"`
			Note string `json:"note"`
		}
		if common.Decode(r, &in) != nil { common.APIError(w, 400, "JSON", "Invalid request"); return }
		status := strings.ToUpper(strings.TrimSpace(in.Status))
		if status != "DRAFT" && status != "AGREED" {
			common.APIError(w, 400, "VALIDATION", "status must be DRAFT or AGREED")
			return
		}
		reference := strings.TrimSpace(in.AgreementReference)
		if status == "AGREED" && reference == "" {
			common.APIError(w, 400, "VALIDATION", "AGREED status requires agreement_reference")
			return
		}
		actor := strings.TrimSpace(r.Header.Get("X-Himate-User-ID"))
		eventAt := time.Now().UTC()
		var agreedAt any
		agreedBy := ""
		if status == "AGREED" {
			agreedAt = eventAt
			agreedBy = actor
		}

		tx, err := a.db.BeginTx(r.Context(), &sql.TxOptions{})
		if err != nil { common.APIError(w, 500, "DB", "Could not start commercial agreement update"); return }
		defer tx.Rollback()

		oldStatus := "DRAFT"
		oldReference := ""
		var oldNote string
		err = tx.QueryRowContext(r.Context(), `SELECT status,agreement_reference,note
			FROM billing.commercial_agreements WHERE partner_id=$1 FOR UPDATE`, partnerID).
			Scan(&oldStatus, &oldReference, &oldNote)
		existing := err == nil
		if err != nil && err != sql.ErrNoRows {
			common.APIError(w, 500, "DB", "Could not load commercial agreement")
			return
		}

		_, err = tx.ExecContext(r.Context(), `INSERT INTO billing.commercial_agreements(
				partner_id,status,agreement_reference,note,agreed_at,agreed_by
			) VALUES($1,$2,$3,$4,$5,$6)
			ON CONFLICT(partner_id) DO UPDATE SET
				status=EXCLUDED.status,agreement_reference=EXCLUDED.agreement_reference,note=EXCLUDED.note,
				agreed_at=EXCLUDED.agreed_at,agreed_by=EXCLUDED.agreed_by,updated_at=NOW()`,
			partnerID, status, reference, strings.TrimSpace(in.Note), agreedAt, agreedBy)
		if err != nil { common.APIError(w, 500, "DB", "Could not save commercial agreement"); return }

		eventType := "COMMERCIAL_AGREEMENT_UPDATED"
		switch {
		case status == "AGREED" && (oldStatus != "AGREED" || oldReference != reference):
			eventType = "COMMERCIAL_AGREEMENT_CONFIRMED"
		case status == "DRAFT" && oldStatus == "AGREED":
			eventType = "COMMERCIAL_AGREEMENT_DRAFTED"
		}
		if !existing || oldStatus != status || oldReference != reference || oldNote != strings.TrimSpace(in.Note) {
			if err = emitBillingEventTx(r.Context(), tx,
				fmt.Sprintf("%s:%s:%d", eventType, partnerID, eventAt.UnixNano()),
				partnerID, "", eventType, eventAt, map[string]any{
					"status": status, "agreement_reference": reference, "actor": actor,
				}); err != nil {
				common.APIError(w, 500, "DB", "Could not record commercial agreement event")
				return
			}
		}
		if err = tx.Commit(); err != nil {
			common.APIError(w, 500, "DB", "Could not commit commercial agreement update")
			return
		}
		a.agreement(w, cloneAsGet(r), partnerID)
	default:
		common.APIError(w, 405, "METHOD", "Use GET or PUT")
	}
}

func (a *app) billingEvents(w http.ResponseWriter, r *http.Request, partnerID string) {
	if r.Method != http.MethodGet { common.APIError(w, 405, "METHOD", "Use GET"); return }
	limit := 100
	rows, err := a.db.Query(`SELECT id,event_key,module_key,event_type,effective_at,payload,created_at
		FROM billing.billing_events WHERE partner_id=$1 ORDER BY effective_at DESC,id DESC LIMIT $2`, partnerID, limit)
	if err != nil { common.APIError(w, 500, "DB", "Could not load billing events"); return }
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id int64
		var eventKey, eventType string
		var moduleKey sql.NullString
		var effectiveAt, created time.Time
		var payload []byte
		if rows.Scan(&id, &eventKey, &moduleKey, &eventType, &effectiveAt, &payload, &created) == nil {
			var module any
			if moduleKey.Valid { module = moduleKey.String }
			items = append(items, map[string]any{
				"id": id, "event_key": eventKey, "partner_id": partnerID, "module_key": module,
				"event_type": eventType, "effective_at": effectiveAt, "payload": common.JSONRawOrEmpty(payload), "created_at": created,
			})
		}
	}
	common.JSON(w, 200, map[string]any{"items": items, "count": len(items)})
}

func (a *app) commercialEvidenceBreakdown(ctx context.Context, partnerID string) (commercial, invoices, paymentEvidence int, err error) {
	err = a.db.QueryRowContext(ctx, `SELECT
		COUNT(*) FILTER (WHERE kind IN ('PAYMENT_EVIDENCE','INVOICE','RECEIPT','CONTRACT') AND NULLIF(BTRIM(storage_url),'') IS NOT NULL),
		COUNT(*) FILTER (WHERE kind='INVOICE' AND NULLIF(BTRIM(storage_url),'') IS NOT NULL),
		COUNT(*) FILTER (WHERE kind IN ('PAYMENT_EVIDENCE','RECEIPT') AND NULLIF(BTRIM(storage_url),'') IS NOT NULL)
		FROM billing.documents WHERE partner_id=$1`, partnerID).
		Scan(&commercial, &invoices, &paymentEvidence)
	return
}

func (a *app) commercialStatus(w http.ResponseWriter, r *http.Request, partnerID string) {
	if r.Method != http.MethodGet { common.APIError(w, 405, "METHOD", "Use GET"); return }
	t, err := a.ensureTerms(partnerID)
	if err != nil { common.APIError(w, 500, "DB", "Could not load commercial terms"); return }
	license, err := a.ensureLicense(partnerID)
	if err != nil { common.APIError(w, 500, "DB", "Could not load activation license"); return }

	agreementStatus := "DRAFT"
	agreementReference := ""
	var agreedAt sql.NullTime
	_ = a.db.QueryRow(`SELECT status,agreement_reference,agreed_at FROM billing.commercial_agreements WHERE partner_id=$1`, partnerID).
		Scan(&agreementStatus, &agreementReference, &agreedAt)

	evidenceCount, invoiceCount, paymentEvidenceCount, evidenceErr := a.commercialEvidenceBreakdown(r.Context(), partnerID)
	if evidenceErr != nil { common.APIError(w, 500, "DB", "Could not resolve commercial evidence"); return }

	referencePartner, _ := a.referencePartner(r.Context(), partnerID)
	paymentVerified := license.Status == "PAID" && paymentEvidenceCount > 0
	waived := referencePartner && license.Status == "WAIVED" && license.Waived && strings.TrimSpace(license.WaiverReason) != ""
	provisioningAllowed := (agreementStatus == "AGREED" && invoiceCount > 0 && paymentVerified) || waived
	nextAction := "CONFIRM_COMMERCIAL_AGREEMENT"
	switch {
	case waived:
		nextAction = "READY_FOR_PROVISIONING"
	case agreementStatus != "AGREED":
		nextAction = "CONFIRM_COMMERCIAL_AGREEMENT"
	case invoiceCount == 0:
		nextAction = "REGISTER_ACTIVATION_INVOICE"
	case paymentEvidenceCount == 0:
		nextAction = "REGISTER_PAYMENT_EVIDENCE"
	case license.Status != "PAID":
		nextAction = "VERIFY_ACTIVATION_PAYMENT"
	case provisioningAllowed:
		nextAction = "READY_FOR_PROVISIONING"
	}

	var agreementAt any
	if agreedAt.Valid { agreementAt = agreedAt.Time.UTC() }
	common.JSON(w, 200, map[string]any{
		"partner_id": partnerID,
		"agreement": map[string]any{
			"status": agreementStatus, "reference": agreementReference, "agreed_at": agreementAt,
		},
		"activation_fee": map[string]any{
			"currency": t.Currency, "required_amount": license.Required, "paid_amount": license.Paid,
			"waived": license.Waived, "waiver_reason": license.WaiverReason,
		},
		"evidence": map[string]any{
			"commercial_count": evidenceCount, "invoice_count": invoiceCount, "payment_evidence_count": paymentEvidenceCount,
		},
		"payment": map[string]any{
			"status": license.Status, "verified": paymentVerified, "payment_reference": license.Reference, "verified_by": license.VerifiedBy,
		},
		"provisioning_allowed": provisioningAllowed,
		"next_action": nextAction,
		"workflow": []map[string]any{
			{"key":"COMMERCIAL_AGREEMENT","complete":agreementStatus=="AGREED" || waived},
			{"key":"ACTIVATION_FEE_INVOICE","complete":invoiceCount>0 || waived},
			{"key":"PAYMENT_EVIDENCE","complete":paymentEvidenceCount>0 || waived},
			{"key":"PAYMENT_VERIFIED","complete":paymentVerified || waived},
			{"key":"LICENSE_PAID","complete":license.Status=="PAID" || waived},
			{"key":"PROVISIONING_ALLOWED","complete":provisioningAllowed},
		},
	})
}

func (a *app) invoiceItemsFor(invoiceID string) []map[string]any {
	rows, err := a.db.Query(`SELECT id,item_key,module_key,item_type,description,currency,quantity,unit_price,amount,period_start,period_end,status,created_at,invoiced_at
		FROM billing.invoice_items WHERE invoice_id=$1 ORDER BY id`, invoiceID)
	if err != nil { return []map[string]any{} }
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id int64
		var itemKey, itemType, description, currency, status string
		var moduleKey sql.NullString
		var quantity, unitPrice, amount float64
		var start, end, created time.Time
		var invoiced sql.NullTime
		if rows.Scan(&id,&itemKey,&moduleKey,&itemType,&description,&currency,&quantity,&unitPrice,&amount,&start,&end,&status,&created,&invoiced)==nil {
			var module, invoicedAt any
			if moduleKey.Valid { module = moduleKey.String }
			if invoiced.Valid { invoicedAt = invoiced.Time.UTC() }
			items = append(items,map[string]any{
				"id":id,"item_key":itemKey,"module_key":module,"item_type":itemType,"description":description,
				"currency":currency,"quantity":quantity,"unit_price":unitPrice,"amount":amount,
				"period_start":start.Format("2006-01-02"),"period_end_exclusive":end.Format("2006-01-02"),
				"status":status,"created_at":created,"invoiced_at":invoicedAt,
			})
		}
	}
	return items
}

func (a *app) attachInvoiceItems(ctx context.Context, invoiceID, partnerID, currency string, serviceStart, serviceEnd time.Time, base float64) (float64, error) {
	baseKey := fmt.Sprintf("BASE:%s:%s", partnerID, dateOnly(serviceStart).Format("2006-01-02"))
	result, err := a.db.ExecContext(ctx, `INSERT INTO billing.invoice_items(
			item_key,invoice_id,partner_id,item_type,description,currency,quantity,unit_price,amount,period_start,period_end,status,invoiced_at
		) VALUES($1,$2,$3,'BASE_SERVICE','Base 30-day service',$4,1,$5,$5,$6,$7,'INVOICED',NOW())
		ON CONFLICT(item_key) DO NOTHING`,
		baseKey, invoiceID, partnerID, currency, base, dateOnly(serviceStart), dateOnly(serviceEnd))
	if err != nil { return 0, err }
	if rows, _ := result.RowsAffected(); rows > 0 {
		_ = a.emitBillingEvent(ctx, "INVOICE_ITEM_CREATED:"+baseKey, partnerID, "", "INVOICE_ITEM_CREATED", time.Now().UTC(), map[string]any{
			"item_key":baseKey,"item_type":"BASE_SERVICE","amount":base,"currency":currency,
			"period_start":dateOnly(serviceStart).Format("2006-01-02"),"period_end_exclusive":dateOnly(serviceEnd).Format("2006-01-02"),
		})
	}

	if _, err = a.db.ExecContext(ctx, `UPDATE billing.invoice_items
		SET invoice_id=$2,status='INVOICED',invoiced_at=NOW()
		WHERE partner_id=$1 AND item_type='MODULE' AND invoice_id IS NULL AND period_start<$3`,
		partnerID, invoiceID, dateOnly(serviceEnd)); err != nil {
		return 0, err
	}
	var moduleTotal float64
	if err = a.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount),0) FROM billing.invoice_items
		WHERE invoice_id=$1 AND item_type='MODULE'`, invoiceID).Scan(&moduleTotal); err != nil {
		return 0, err
	}
	return moduleTotal, nil
}


func start23111BillingCommercialModelMigration() common.Migration {
	return common.Migration{
		Version: 9,
		Name: "start-23-11-1-individual-commercial-terms",
		Statements: []string{
			`ALTER TABLE billing.partner_terms ADD COLUMN IF NOT EXISTS minimum_monthly_commitment NUMERIC(12,2) NOT NULL DEFAULT 1500`,
			`ALTER TABLE billing.partner_terms ADD COLUMN IF NOT EXISTS quote_reference TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE billing.partner_terms ADD COLUMN IF NOT EXISTS commercial_configured BOOLEAN NOT NULL DEFAULT FALSE`,
			`ALTER TABLE billing.partner_terms ADD COLUMN IF NOT EXISTS terms_version INTEGER NOT NULL DEFAULT 1`,
			`ALTER TABLE billing.partner_terms ADD COLUMN IF NOT EXISTS contracted_at TIMESTAMPTZ`,
			`ALTER TABLE billing.partner_terms ADD COLUMN IF NOT EXISTS pricing_model TEXT NOT NULL DEFAULT 'INDIVIDUAL_QUOTE'`,
			`ALTER TABLE billing.partner_terms ALTER COLUMN activation_fee SET DEFAULT 0`,
			`ALTER TABLE billing.initial_licenses ALTER COLUMN required_amount SET DEFAULT 0`,
			`CREATE TABLE IF NOT EXISTS billing.partner_terms_history(
				id BIGSERIAL PRIMARY KEY,
				partner_id TEXT NOT NULL,
				terms_version INTEGER NOT NULL,
				currency TEXT NOT NULL,
				activation_fee NUMERIC(12,2) NOT NULL,
				activation_fee_waived BOOLEAN NOT NULL,
				base_monthly_fee NUMERIC(12,2) NOT NULL,
				minimum_monthly_commitment NUMERIC(12,2) NOT NULL,
				annual_increase_percent NUMERIC(6,2) NOT NULL,
				price_effective_from DATE NOT NULL,
				service_anchor_date DATE NOT NULL,
				quote_reference TEXT NOT NULL DEFAULT '',
				pricing_model TEXT NOT NULL DEFAULT 'INDIVIDUAL_QUOTE',
				actor TEXT NOT NULL DEFAULT '',
				reason TEXT NOT NULL DEFAULT '',
				changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				UNIQUE(partner_id,terms_version)
			)`,
			`CREATE INDEX IF NOT EXISTS billing_partner_terms_history_lookup ON billing.partner_terms_history(partner_id,terms_version DESC)`,
			`CREATE OR REPLACE FUNCTION billing.reject_partner_terms_history_mutation() RETURNS trigger LANGUAGE plpgsql AS $fn$
			BEGIN
				RAISE EXCEPTION 'billing.partner_terms_history is append-only';
				RETURN OLD;
			END; $fn$`,
			`DROP TRIGGER IF EXISTS billing_partner_terms_history_append_only ON billing.partner_terms_history`,
			`CREATE TRIGGER billing_partner_terms_history_append_only
				BEFORE UPDATE OR DELETE ON billing.partner_terms_history
				FOR EACH ROW EXECUTE FUNCTION billing.reject_partner_terms_history_mutation()`,
			`UPDATE billing.partner_terms SET minimum_monthly_commitment=1500 WHERE minimum_monthly_commitment<1500 AND currency='USD'`,
			`UPDATE billing.partner_terms SET commercial_configured=TRUE,quote_reference=CASE WHEN quote_reference='' THEN 'REFERENCE-PARTNER' ELSE quote_reference END,contracted_at=COALESCE(contracted_at,updated_at) WHERE partner_id='ptr_000001'`,
		},
	}
}
