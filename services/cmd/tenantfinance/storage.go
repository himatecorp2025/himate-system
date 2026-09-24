package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"himate.local/services/internal/automation"
	"himate.local/services/internal/common"
	"himate.local/services/internal/financepolicy"
)

var (
	errInvoiceNotFound = errors.New("invoice not found")
	errInvoiceState = errors.New("invoice state does not allow this operation")
	errIdempotency = errors.New("idempotency key was already used with different invoice content")
	errSourceConflict = errors.New("source already produced a different invoice")
)

func (a *app) migrate(ctx context.Context) error {
	return common.ApplyMigrations(ctx, a.db, "tenant-finance", []common.Migration{
		{Version: 1, Name: "tenant-customer-invoicing-foundation", Statements: []string{
			`CREATE SCHEMA IF NOT EXISTS tenant_finance`,
			`CREATE TABLE IF NOT EXISTS tenant_finance.policies(
				partner_id TEXT PRIMARY KEY,
				default_payment_terms_days INT NOT NULL CHECK(default_payment_terms_days BETWEEN 0 AND 365),
				accounting_basis TEXT NOT NULL CHECK(accounting_basis IN ('CASH','ACCRUAL')),
				payment_methods JSONB NOT NULL DEFAULT '[]'::jsonb,
				default_currency TEXT NOT NULL CHECK(default_currency ~ '^[A-Z]{3}$'),
				invoice_prefix TEXT NOT NULL CHECK(invoice_prefix ~ '^[A-Z0-9][A-Z0-9_-]{0,11}$'),
				updated_by TEXT NOT NULL DEFAULT '',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE TABLE IF NOT EXISTS tenant_finance.invoice_sequences(
				partner_id TEXT NOT NULL,
				sequence_year INT NOT NULL,
				next_value BIGINT NOT NULL CHECK(next_value > 0),
				PRIMARY KEY(partner_id,sequence_year)
			)`,
			`CREATE TABLE IF NOT EXISTS tenant_finance.invoices(
				id TEXT PRIMARY KEY,
				partner_id TEXT NOT NULL,
				source_type TEXT NOT NULL CHECK(source_type IN ('MANUAL','WORKFLOW','SCHEDULE')),
				source_id TEXT NOT NULL DEFAULT '',
				request_key TEXT NOT NULL,
				draft_hash TEXT NOT NULL,
				status TEXT NOT NULL DEFAULT 'DRAFT' CHECK(status IN ('DRAFT','READY_FOR_ISSUE','ISSUED','PAID','VOID')),
				invoice_number TEXT NOT NULL DEFAULT '',
				currency TEXT NOT NULL CHECK(currency ~ '^[A-Z]{3}$'),
				payment_terms_override_days INT CHECK(payment_terms_override_days BETWEEN 0 AND 365),
				payment_terms_days INT CHECK(payment_terms_days BETWEEN 0 AND 365),
				accounting_basis TEXT NOT NULL DEFAULT '' CHECK(accounting_basis IN ('','CASH','ACCRUAL')),
				payment_methods JSONB NOT NULL DEFAULT '[]'::jsonb,
				issuer_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
				customer_snapshot JSONB NOT NULL,
				subtotal_minor BIGINT NOT NULL CHECK(subtotal_minor >= 0),
				tax_minor BIGINT NOT NULL CHECK(tax_minor >= 0),
				total_minor BIGINT NOT NULL CHECK(total_minor >= 0),
				notes TEXT NOT NULL DEFAULT '',
				created_by TEXT NOT NULL,
				finalized_by TEXT NOT NULL DEFAULT '',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				ready_at TIMESTAMPTZ,
				issued_at TIMESTAMPTZ,
				paid_at TIMESTAMPTZ,
				UNIQUE(partner_id,request_key)
			)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS tenant_finance_source_once_idx
				ON tenant_finance.invoices(partner_id,source_type,source_id)
				WHERE source_type IN ('WORKFLOW','SCHEDULE') AND source_id<>''`,
			`CREATE UNIQUE INDEX IF NOT EXISTS tenant_finance_number_once_idx
				ON tenant_finance.invoices(partner_id,invoice_number) WHERE invoice_number<>''`,
			`CREATE INDEX IF NOT EXISTS tenant_finance_partner_status_idx
				ON tenant_finance.invoices(partner_id,status,created_at DESC,id DESC)`,
			`CREATE TABLE IF NOT EXISTS tenant_finance.invoice_items(
				id BIGSERIAL PRIMARY KEY,
				invoice_id TEXT NOT NULL REFERENCES tenant_finance.invoices(id) ON DELETE RESTRICT,
				partner_id TEXT NOT NULL,
				line_no INT NOT NULL CHECK(line_no > 0),
				description TEXT NOT NULL,
				quantity_milli BIGINT NOT NULL CHECK(quantity_milli > 0),
				unit_price_minor BIGINT NOT NULL CHECK(unit_price_minor >= 0),
				discount_minor BIGINT NOT NULL DEFAULT 0 CHECK(discount_minor >= 0),
				tax_rate_bps INT NOT NULL DEFAULT 0 CHECK(tax_rate_bps BETWEEN 0 AND 10000),
				net_minor BIGINT NOT NULL CHECK(net_minor >= 0),
				tax_minor BIGINT NOT NULL CHECK(tax_minor >= 0),
				total_minor BIGINT NOT NULL CHECK(total_minor >= 0),
				UNIQUE(invoice_id,line_no)
			)`,
			`CREATE INDEX IF NOT EXISTS tenant_finance_items_partner_invoice_idx
				ON tenant_finance.invoice_items(partner_id,invoice_id,line_no)`,
			`CREATE TABLE IF NOT EXISTS tenant_finance.invoice_events(
				id BIGSERIAL PRIMARY KEY,
				invoice_id TEXT NOT NULL REFERENCES tenant_finance.invoices(id) ON DELETE RESTRICT,
				partner_id TEXT NOT NULL,
				event_type TEXT NOT NULL,
				actor_id TEXT NOT NULL DEFAULT '',
				payload JSONB NOT NULL DEFAULT '{}'::jsonb,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE INDEX IF NOT EXISTS tenant_finance_events_partner_invoice_idx
				ON tenant_finance.invoice_events(partner_id,invoice_id,created_at,id)`,
			`CREATE OR REPLACE FUNCTION tenant_finance.reject_invoice_event_mutation() RETURNS trigger AS $$
				BEGIN RAISE EXCEPTION 'tenant finance invoice events are append-only'; END;
			$$ LANGUAGE plpgsql`,
			`DROP TRIGGER IF EXISTS tenant_finance_invoice_events_append_only ON tenant_finance.invoice_events`,
			`CREATE TRIGGER tenant_finance_invoice_events_append_only
				BEFORE UPDATE OR DELETE ON tenant_finance.invoice_events
				FOR EACH ROW EXECUTE FUNCTION tenant_finance.reject_invoice_event_mutation()`,
		}},
		automation.OutboxMigration(2),
	})
}

func (a *app) loadPolicy(ctx context.Context, partnerID string) (financePolicyState, error) {
	state := a.platformPolicy
	state.Source = "PLATFORM_DEFAULT"
	var methodsRaw []byte
	var days int
	var basis, currency, prefix string
	err := a.db.QueryRowContext(ctx, `SELECT default_payment_terms_days,accounting_basis,payment_methods,default_currency,invoice_prefix
		FROM tenant_finance.policies WHERE partner_id=$1`, strings.TrimSpace(partnerID)).
		Scan(&days, &basis, &methodsRaw, &currency, &prefix)
	if errors.Is(err, sql.ErrNoRows) {
		return state, nil
	}
	if err != nil {
		return state, err
	}
	methods := []string{}
	_ = json.Unmarshal(methodsRaw, &methods)
	policy, err := financepolicy.NormalizePolicy(financepolicy.Policy{
		DefaultPaymentTermsDays: days,
		AccountingBasis: basis,
		PaymentMethods: methods,
	})
	if err != nil {
		return state, err
	}
	currency, err = normalizeCurrency(currency, state.DefaultCurrency)
	if err != nil {
		return state, err
	}
	prefix, err = normalizeInvoicePrefix(prefix)
	if err != nil {
		return state, err
	}
	return financePolicyState{Policy: policy, DefaultCurrency: currency, InvoicePrefix: prefix, Source: "PARTNER"}, nil
}

func (a *app) savePolicy(ctx context.Context, partnerID, actorID string, state financePolicyState) (financePolicyState, error) {
	policy, err := financepolicy.NormalizePolicy(state.Policy)
	if err != nil {
		return financePolicyState{}, err
	}
	currency, err := normalizeCurrency(state.DefaultCurrency, "")
	if err != nil {
		return financePolicyState{}, err
	}
	prefix, err := normalizeInvoicePrefix(state.InvoicePrefix)
	if err != nil {
		return financePolicyState{}, err
	}
	methodsRaw, _ := json.Marshal(policy.PaymentMethods)
	_, err = a.db.ExecContext(ctx, `INSERT INTO tenant_finance.policies(
			partner_id,default_payment_terms_days,accounting_basis,payment_methods,default_currency,invoice_prefix,updated_by
		) VALUES($1,$2,$3,$4::jsonb,$5,$6,$7)
		ON CONFLICT(partner_id) DO UPDATE SET
			default_payment_terms_days=EXCLUDED.default_payment_terms_days,
			accounting_basis=EXCLUDED.accounting_basis,
			payment_methods=EXCLUDED.payment_methods,
			default_currency=EXCLUDED.default_currency,
			invoice_prefix=EXCLUDED.invoice_prefix,
			updated_by=EXCLUDED.updated_by,
			updated_at=NOW()`,
		strings.TrimSpace(partnerID), policy.DefaultPaymentTermsDays, policy.AccountingBasis, string(methodsRaw), currency, prefix, strings.TrimSpace(actorID))
	if err != nil {
		return financePolicyState{}, err
	}
	return financePolicyState{Policy: policy, DefaultCurrency: currency, InvoicePrefix: prefix, Source: "PARTNER"}, nil
}

func insertItemsTx(ctx context.Context, tx *sql.Tx, invoiceID, partnerID string, items []calculatedItem) error {
	for _, item := range items {
		if _, err := tx.ExecContext(ctx, `INSERT INTO tenant_finance.invoice_items(
				invoice_id,partner_id,line_no,description,quantity_milli,unit_price_minor,discount_minor,tax_rate_bps,net_minor,tax_minor,total_minor
			) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
			invoiceID, partnerID, item.LineNo, item.Description, item.QuantityMilli, item.UnitPriceMinor,
			item.DiscountMinor, item.TaxRateBPS, item.NetMinor, item.TaxMinor, item.TotalMinor); err != nil {
			return err
		}
	}
	return nil
}

func appendInvoiceEventTx(ctx context.Context, tx *sql.Tx, rec invoiceRecord, eventType, actorID string, payload map[string]any) error {
	raw, _ := json.Marshal(payload)
	_, err := tx.ExecContext(ctx, `INSERT INTO tenant_finance.invoice_events(invoice_id,partner_id,event_type,actor_id,payload)
		VALUES($1,$2,$3,$4,$5::jsonb)`, rec.ID, rec.PartnerID, eventType, strings.TrimSpace(actorID), string(raw))
	return err
}

func (a *app) createManualDraft(ctx context.Context, partnerID, actorID string, in invoiceInput) (invoiceRecord, bool, error) {
	partnerID = strings.TrimSpace(partnerID)
	actorID = strings.TrimSpace(actorID)
	in.RequestKey = strings.TrimSpace(in.RequestKey)
	if partnerID == "" || actorID == "" || in.RequestKey == "" || len(in.RequestKey) > 200 {
		return invoiceRecord{}, false, errors.New("partner, actor and request_key are required")
	}
	policy, err := a.loadPolicy(ctx, partnerID)
	if err != nil {
		return invoiceRecord{}, false, err
	}
	currency, err := normalizeCurrency(in.Currency, policy.DefaultCurrency)
	if err != nil {
		return invoiceRecord{}, false, err
	}
	customer, err := normalizeCustomer(in.Customer, false)
	if err != nil {
		return invoiceRecord{}, false, err
	}
	if in.PaymentTermsDays != nil {
		if err := financepolicy.ValidateDays(*in.PaymentTermsDays); err != nil {
			return invoiceRecord{}, false, err
		}
	}
	items, subtotal, taxTotal, total, err := calculateItems(in.Items)
	if err != nil {
		return invoiceRecord{}, false, err
	}
	in.Notes = strings.TrimSpace(in.Notes)
	requestCurrency := strings.ToUpper(strings.TrimSpace(in.Currency))
	hash := draftEnvelopeHash(sourceManual, "", requestCurrency, customer, items, in.PaymentTermsDays, in.Notes)
	customerRaw, _ := json.Marshal(customer)
	id := newID("tin")
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return invoiceRecord{}, false, err
	}
	defer tx.Rollback()
	var inserted string
	err = tx.QueryRowContext(ctx, `INSERT INTO tenant_finance.invoices(
			id,partner_id,source_type,source_id,request_key,draft_hash,status,currency,payment_terms_override_days,
			customer_snapshot,subtotal_minor,tax_minor,total_minor,notes,created_by
		) VALUES($1,$2,'MANUAL','',$3,$4,'DRAFT',$5,$6,$7::jsonb,$8,$9,$10,$11,$12)
		ON CONFLICT(partner_id,request_key) DO NOTHING RETURNING id`,
		id, partnerID, in.RequestKey, hash, currency, nullableInt(in.PaymentTermsDays), string(customerRaw),
		subtotal, taxTotal, total, in.Notes, actorID).Scan(&inserted)
	if errors.Is(err, sql.ErrNoRows) {
		var existingID, existingHash string
		if err = tx.QueryRowContext(ctx, `SELECT id,draft_hash FROM tenant_finance.invoices WHERE partner_id=$1 AND request_key=$2`,
			partnerID, in.RequestKey).Scan(&existingID, &existingHash); err != nil {
			return invoiceRecord{}, false, err
		}
		if existingHash != hash {
			return invoiceRecord{}, false, errIdempotency
		}
		_ = tx.Rollback()
		rec, loadErr := a.loadInvoice(ctx, partnerID, existingID)
		return rec, true, loadErr
	}
	if err != nil {
		return invoiceRecord{}, false, err
	}
	if err = insertItemsTx(ctx, tx, id, partnerID, items); err != nil {
		return invoiceRecord{}, false, err
	}
	rec := invoiceRecord{ID: id, PartnerID: partnerID}
	if err = appendInvoiceEventTx(ctx, tx, rec, "MANUAL_DRAFT_CREATED", actorID, map[string]any{
		"request_key": in.RequestKey, "currency": currency, "total_minor": total,
	}); err != nil {
		return invoiceRecord{}, false, err
	}
	if err = tx.Commit(); err != nil {
		return invoiceRecord{}, false, err
	}
	rec, err = a.loadInvoice(ctx, partnerID, id)
	return rec, false, err
}

func nullableInt(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

func (a *app) nextInvoiceNumberTx(ctx context.Context, tx *sql.Tx, partnerID, prefix string, now time.Time) (string, error) {
	var sequence int64
	year := now.UTC().Year()
	err := tx.QueryRowContext(ctx, `INSERT INTO tenant_finance.invoice_sequences(partner_id,sequence_year,next_value)
			VALUES($1,$2,2)
			ON CONFLICT(partner_id,sequence_year) DO UPDATE
			SET next_value=tenant_finance.invoice_sequences.next_value+1
			RETURNING next_value-1`,
		partnerID, year).Scan(&sequence)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%04d-%06d", prefix, year, sequence), nil
}

func resolvedTerms(policy financePolicyState, override *int) (int, error) {
	return financepolicy.ResolvePaymentTerms(
		policy.Policy.DefaultPaymentTermsDays,
		nil,
		nil,
		override,
	)
}

func (a *app) finalizeManual(ctx context.Context, partnerID, actorID, invoiceID, correlationID string) (invoiceRecord, bool, error) {
	pre, err := a.loadInvoice(ctx, partnerID, invoiceID)
	if err != nil {
		return invoiceRecord{}, false, err
	}
	if pre.SourceType != sourceManual {
		return invoiceRecord{}, false, errInvoiceState
	}
	if pre.Status == statusReadyForIssue {
		return pre, true, nil
	}
	if pre.Status != statusDraft {
		return invoiceRecord{}, false, errInvoiceState
	}
	issuer, err := a.fetchIssuer(ctx, partnerID)
	if err != nil {
		return invoiceRecord{}, false, err
	}
	policy, err := a.loadPolicy(ctx, partnerID)
	if err != nil {
		return invoiceRecord{}, false, err
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return invoiceRecord{}, false, err
	}
	defer tx.Rollback()
	var status, sourceType string
	var customerRaw []byte
	var terms sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT status,source_type,customer_snapshot,payment_terms_override_days
		FROM tenant_finance.invoices WHERE id=$1 AND partner_id=$2 FOR UPDATE`,
		invoiceID, partnerID).Scan(&status, &sourceType, &customerRaw, &terms)
	if errors.Is(err, sql.ErrNoRows) {
		return invoiceRecord{}, false, errInvoiceNotFound
	}
	if err != nil {
		return invoiceRecord{}, false, err
	}
	if sourceType != sourceManual {
		return invoiceRecord{}, false, errInvoiceState
	}
	if status == statusReadyForIssue {
		_ = tx.Rollback()
		rec, loadErr := a.loadInvoice(ctx, partnerID, invoiceID)
		return rec, true, loadErr
	}
	if status != statusDraft {
		return invoiceRecord{}, false, errInvoiceState
	}
	customer := customerSnapshot{}
	if json.Unmarshal(customerRaw, &customer) != nil {
		return invoiceRecord{}, false, errors.New("stored customer snapshot is invalid")
	}
	customer, err = normalizeCustomer(customer, true)
	if err != nil {
		return invoiceRecord{}, false, err
	}
	var override *int
	if terms.Valid {
		v := int(terms.Int64)
		override = &v
	}
	days, err := resolvedTerms(policy, override)
	if err != nil {
		return invoiceRecord{}, false, err
	}
	now := time.Now().UTC()
	number, err := a.nextInvoiceNumberTx(ctx, tx, partnerID, policy.InvoicePrefix, now)
	if err != nil {
		return invoiceRecord{}, false, err
	}
	issuerRaw, _ := json.Marshal(issuer)
	methodsRaw, _ := json.Marshal(policy.Policy.PaymentMethods)
	customerFinalRaw, _ := json.Marshal(customer)
	_, err = tx.ExecContext(ctx, `UPDATE tenant_finance.invoices SET
			status='READY_FOR_ISSUE',invoice_number=$3,payment_terms_days=$4,accounting_basis=$5,payment_methods=$6::jsonb,
			issuer_snapshot=$7::jsonb,customer_snapshot=$8::jsonb,finalized_by=$9,ready_at=$10,updated_at=$10
		WHERE id=$1 AND partner_id=$2`,
		invoiceID, partnerID, number, days, policy.Policy.AccountingBasis, string(methodsRaw), string(issuerRaw), string(customerFinalRaw), actorID, now)
	if err != nil {
		return invoiceRecord{}, false, err
	}
	rec := invoiceRecord{ID: invoiceID, PartnerID: partnerID}
	if err = appendInvoiceEventTx(ctx, tx, rec, "READY_FOR_ISSUE", actorID, map[string]any{
		"invoice_number": number, "payment_terms_days": days, "accounting_basis": policy.Policy.AccountingBasis,
		"issuer_profile_source": "PARTNERS_SERVICE_CURRENT_SNAPSHOT",
	}); err != nil {
		return invoiceRecord{}, false, err
	}
	if err = automation.EnqueueTx(ctx, tx, automation.OutboxEvent{
		ProducerService: financeProducer,
		EventKey: "invoice-ready:" + invoiceID,
		EventType: "tenant_invoice.ready_for_issue.v1",
		EventVersion: 1,
		PartnerID: partnerID,
		ModuleKey: invoiceModuleKey,
		CorrelationID: strings.TrimSpace(correlationID),
		SubjectType: "tenant_invoice",
		SubjectID: invoiceID,
		Payload: map[string]any{
			"invoice_id": invoiceID, "invoice_number": number, "source_type": sourceManual,
			"status": statusReadyForIssue, "document_renderer": "DEFERRED",
		},
		OccurredAt: now,
		AvailableAt: now,
	}); err != nil {
		return invoiceRecord{}, false, err
	}
	if err = tx.Commit(); err != nil {
		return invoiceRecord{}, false, err
	}
	rec, err = a.loadInvoice(ctx, partnerID, invoiceID)
	return rec, false, err
}

func (a *app) createAutomatedReady(ctx context.Context, serviceID, actorID string, in automatedInvoiceIntent) (invoiceRecord, bool, error) {
	in.PartnerID = strings.TrimSpace(in.PartnerID)
	in.SourceType = strings.ToUpper(strings.TrimSpace(in.SourceType))
	in.SourceID = strings.TrimSpace(in.SourceID)
	in.Notes = strings.TrimSpace(in.Notes)
	if in.PartnerID == "" || in.SourceID == "" || len(in.SourceID) > 240 {
		return invoiceRecord{}, false, errors.New("partner_id and source_id are required")
	}
	if err := validateSourceForService(serviceID, in.SourceType); err != nil {
		return invoiceRecord{}, false, err
	}
	customer, err := normalizeCustomer(in.Customer, true)
	if err != nil {
		return invoiceRecord{}, false, err
	}
	if in.PaymentTermsDays != nil {
		if err := financepolicy.ValidateDays(*in.PaymentTermsDays); err != nil {
			return invoiceRecord{}, false, err
		}
	}
	items, subtotal, taxTotal, total, err := calculateItems(in.Items)
	if err != nil {
		return invoiceRecord{}, false, err
	}
	requestCurrency := strings.ToUpper(strings.TrimSpace(in.Currency))
	hash := draftEnvelopeHash(in.SourceType, in.SourceID, requestCurrency, customer, items, in.PaymentTermsDays, in.Notes)
	var existingID, existingHash string
	err = a.db.QueryRowContext(ctx, `SELECT id,draft_hash FROM tenant_finance.invoices
		WHERE partner_id=$1 AND source_type=$2 AND source_id=$3`,
		in.PartnerID, in.SourceType, in.SourceID).Scan(&existingID, &existingHash)
	if err == nil {
		if existingHash != hash {
			return invoiceRecord{}, false, errSourceConflict
		}
		rec, loadErr := a.loadInvoice(ctx, in.PartnerID, existingID)
		return rec, true, loadErr
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return invoiceRecord{}, false, err
	}
	issuer, err := a.fetchIssuer(ctx, in.PartnerID)
	if err != nil {
		return invoiceRecord{}, false, err
	}
	policy, err := a.loadPolicy(ctx, in.PartnerID)
	if err != nil {
		return invoiceRecord{}, false, err
	}
	currency, err := normalizeCurrency(in.Currency, policy.DefaultCurrency)
	if err != nil {
		return invoiceRecord{}, false, err
	}
	days, err := resolvedTerms(policy, in.PaymentTermsDays)
	if err != nil {
		return invoiceRecord{}, false, err
	}
	requestKey := "AUTO:" + in.SourceType + ":" + in.SourceID
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return invoiceRecord{}, false, err
	}
	defer tx.Rollback()
	existingID, existingHash = "", ""
	err = tx.QueryRowContext(ctx, `SELECT id,draft_hash FROM tenant_finance.invoices
		WHERE partner_id=$1 AND source_type=$2 AND source_id=$3 FOR UPDATE`,
		in.PartnerID, in.SourceType, in.SourceID).Scan(&existingID, &existingHash)
	if err == nil {
		if existingHash != hash {
			return invoiceRecord{}, false, errSourceConflict
		}
		_ = tx.Rollback()
		rec, loadErr := a.loadInvoice(ctx, in.PartnerID, existingID)
		return rec, true, loadErr
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return invoiceRecord{}, false, err
	}
	now := time.Now().UTC()
	number, err := a.nextInvoiceNumberTx(ctx, tx, in.PartnerID, policy.InvoicePrefix, now)
	if err != nil {
		return invoiceRecord{}, false, err
	}
	id := newID("tin")
	issuerRaw, _ := json.Marshal(issuer)
	customerRaw, _ := json.Marshal(customer)
	methodsRaw, _ := json.Marshal(policy.Policy.PaymentMethods)
	_, err = tx.ExecContext(ctx, `INSERT INTO tenant_finance.invoices(
			id,partner_id,source_type,source_id,request_key,draft_hash,status,invoice_number,currency,
			payment_terms_override_days,payment_terms_days,accounting_basis,payment_methods,issuer_snapshot,customer_snapshot,
			subtotal_minor,tax_minor,total_minor,notes,created_by,finalized_by,ready_at
		) VALUES($1,$2,$3,$4,$5,$6,'READY_FOR_ISSUE',$7,$8,$9,$10,$11,$12::jsonb,$13::jsonb,$14::jsonb,$15,$16,$17,$18,$19,$19,$20)`,
		id, in.PartnerID, in.SourceType, in.SourceID, requestKey, hash, number, currency,
		nullableInt(in.PaymentTermsDays), days, policy.Policy.AccountingBasis, string(methodsRaw), string(issuerRaw), string(customerRaw),
		subtotal, taxTotal, total, strings.TrimSpace(in.Notes), actorID, now)
	if err != nil {
		return invoiceRecord{}, false, err
	}
	if err = insertItemsTx(ctx, tx, id, in.PartnerID, items); err != nil {
		return invoiceRecord{}, false, err
	}
	rec := invoiceRecord{ID: id, PartnerID: in.PartnerID}
	if err = appendInvoiceEventTx(ctx, tx, rec, "AUTOMATED_READY_FOR_ISSUE", actorID, map[string]any{
		"producer_service": serviceID, "source_type": in.SourceType, "source_id": in.SourceID,
		"invoice_number": number, "total_minor": total,
	}); err != nil {
		return invoiceRecord{}, false, err
	}
	if err = automation.EnqueueTx(ctx, tx, automation.OutboxEvent{
		ProducerService: financeProducer,
		EventKey: "invoice-ready:" + id,
		EventType: "tenant_invoice.ready_for_issue.v1",
		EventVersion: 1,
		PartnerID: in.PartnerID,
		ModuleKey: invoiceModuleKey,
		CorrelationID: strings.TrimSpace(in.CorrelationID),
		CausationID: strings.TrimSpace(in.CausationID),
		SubjectType: "tenant_invoice",
		SubjectID: id,
		Payload: map[string]any{
			"invoice_id": id, "invoice_number": number, "source_type": in.SourceType,
			"source_id": in.SourceID, "status": statusReadyForIssue, "document_renderer": "DEFERRED",
		},
		OccurredAt: now,
		AvailableAt: now,
	}); err != nil {
		return invoiceRecord{}, false, err
	}
	if err = tx.Commit(); err != nil {
		return invoiceRecord{}, false, err
	}
	rec, err = a.loadInvoice(ctx, in.PartnerID, id)
	return rec, false, err
}

const invoiceColumns = `i.id,i.partner_id,i.source_type,i.source_id,i.request_key,i.draft_hash,i.status,i.invoice_number,i.currency,
	i.payment_terms_override_days,i.payment_terms_days,i.accounting_basis,i.payment_methods,i.issuer_snapshot,i.customer_snapshot,
	i.subtotal_minor,i.tax_minor,i.total_minor,i.notes,i.created_by,i.finalized_by,i.created_at,i.updated_at,i.ready_at,i.issued_at,i.paid_at`

func scanInvoice(scanner interface{ Scan(...any) error }) (invoiceRecord, error) {
	var rec invoiceRecord
	var termsOverride, terms sql.NullInt64
	var methodsRaw, issuerRaw, customerRaw []byte
	var ready, issued, paid sql.NullTime
	err := scanner.Scan(
		&rec.ID, &rec.PartnerID, &rec.SourceType, &rec.SourceID, &rec.RequestKey, &rec.DraftHash, &rec.Status, &rec.InvoiceNumber, &rec.Currency,
		&termsOverride, &terms, &rec.AccountingBasis, &methodsRaw, &issuerRaw, &customerRaw,
		&rec.SubtotalMinor, &rec.TaxMinor, &rec.TotalMinor, &rec.Notes, &rec.CreatedBy, &rec.FinalizedBy, &rec.CreatedAt, &rec.UpdatedAt,
		&ready, &issued, &paid,
	)
	if err != nil {
		return rec, err
	}
	if termsOverride.Valid {
		v := int(termsOverride.Int64)
		rec.PaymentTermsOverride = &v
	}
	if terms.Valid {
		v := int(terms.Int64)
		rec.PaymentTermsDays = &v
	}
	_ = json.Unmarshal(methodsRaw, &rec.PaymentMethods)
	_ = json.Unmarshal(issuerRaw, &rec.Issuer)
	_ = json.Unmarshal(customerRaw, &rec.Customer)
	if ready.Valid {
		v := ready.Time
		rec.ReadyAt = &v
	}
	if issued.Valid {
		v := issued.Time
		rec.IssuedAt = &v
	}
	if paid.Valid {
		v := paid.Time
		rec.PaidAt = &v
	}
	return rec, nil
}

func (a *app) loadInvoice(ctx context.Context, partnerID, invoiceID string) (invoiceRecord, error) {
	rec, err := scanInvoice(a.db.QueryRowContext(ctx, `SELECT `+invoiceColumns+` FROM tenant_finance.invoices i
		WHERE i.partner_id=$1 AND i.id=$2`, strings.TrimSpace(partnerID), strings.TrimSpace(invoiceID)))
	if errors.Is(err, sql.ErrNoRows) {
		return invoiceRecord{}, errInvoiceNotFound
	}
	if err != nil {
		return invoiceRecord{}, err
	}
	rows, err := a.db.QueryContext(ctx, `SELECT line_no,description,quantity_milli,unit_price_minor,discount_minor,tax_rate_bps,net_minor,tax_minor,total_minor
		FROM tenant_finance.invoice_items WHERE partner_id=$1 AND invoice_id=$2 ORDER BY line_no`,
		rec.PartnerID, rec.ID)
	if err != nil {
		return invoiceRecord{}, err
	}
	defer rows.Close()
	rec.Items = []calculatedItem{}
	for rows.Next() {
		var item calculatedItem
		if err := rows.Scan(&item.LineNo, &item.Description, &item.QuantityMilli, &item.UnitPriceMinor, &item.DiscountMinor, &item.TaxRateBPS, &item.NetMinor, &item.TaxMinor, &item.TotalMinor); err != nil {
			return invoiceRecord{}, err
		}
		rec.Items = append(rec.Items, item)
	}
	return rec, rows.Err()
}

func (a *app) listInvoices(ctx context.Context, partnerID string, limit int) ([]invoiceRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	rows, err := a.db.QueryContext(ctx, `SELECT `+invoiceColumns+` FROM tenant_finance.invoices i
		WHERE i.partner_id=$1 ORDER BY i.created_at DESC,i.id DESC LIMIT $2`, strings.TrimSpace(partnerID), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []invoiceRecord{}
	for rows.Next() {
		rec, err := scanInvoice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}
