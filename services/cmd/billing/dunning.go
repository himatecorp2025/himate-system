package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"himate.local/services/internal/common"
	"net/http"
	"strings"
	"time"
)

const (
	dunningMaxAttempts = 3
	dunningSecondOffset = 2
	dunningThirdOffset  = 5
	dunningCureDays     = 30
)

func start23112DunningMigration() common.Migration {
	return common.Migration{
		Version: 15,
		Name: "start-23-11-2-recurring-payment-dunning",
		Statements: []string{
			`ALTER TABLE billing.invoices ADD COLUMN IF NOT EXISTS collection_attempts INT NOT NULL DEFAULT 0`,
			`ALTER TABLE billing.invoices ADD COLUMN IF NOT EXISTS last_collection_attempt_at TIMESTAMPTZ`,
			`ALTER TABLE billing.invoices ADD COLUMN IF NOT EXISTS dunning_state TEXT NOT NULL DEFAULT 'NONE'`,
			`ALTER TABLE billing.invoices ADD COLUMN IF NOT EXISTS dunning_suspended_at DATE`,
			`ALTER TABLE billing.invoices ADD COLUMN IF NOT EXISTS purge_due_at DATE`,
			`ALTER TABLE billing.invoices ADD COLUMN IF NOT EXISTS pre_suspend_partner_lifecycle TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE billing.invoices ADD COLUMN IF NOT EXISTS operational_purged_at TIMESTAMPTZ`,
			`CREATE INDEX IF NOT EXISTS billing_invoice_dunning_idx
				ON billing.invoices(dunning_state,invoice_date,collection_attempts)
				WHERE billing_model='PLAN' AND charge_type IN ('PLAN_MONTHLY','PLAN_ANNUAL_RENEWAL')`,
		},
	}
}

func dunningEligible(billingModel, chargeType string) bool {
	if !strings.EqualFold(strings.TrimSpace(billingModel), "PLAN") { return false }
	switch strings.ToUpper(strings.TrimSpace(chargeType)) {
	case "PLAN_MONTHLY", "PLAN_ANNUAL_RENEWAL":
		return true
	default:
		return false
	}
}

func collectionRetryDue(invoiceDate, at time.Time, attempts int) bool {
	invoiceDate, at = dateOnly(invoiceDate), dateOnly(at)
	switch attempts {
	case 1:
		return !at.Before(invoiceDate.AddDate(0, 0, dunningSecondOffset))
	case 2:
		return !at.Before(invoiceDate.AddDate(0, 0, dunningThirdOffset))
	default:
		return false
	}
}

func (a *app) invoiceDunningMeta(ctx context.Context, invoiceID string) (eligible bool, attempts int, invoiceDate time.Time, dunningState string, err error) {
	var billingModel, chargeType string
	err = a.db.QueryRowContext(ctx, `SELECT billing_model,charge_type,collection_attempts,invoice_date,dunning_state
		FROM billing.invoices WHERE id=$1`, invoiceID).
		Scan(&billingModel, &chargeType, &attempts, &invoiceDate, &dunningState)
	if err != nil { return false, 0, time.Time{}, "", err }
	return dunningEligible(billingModel, chargeType), attempts, invoiceDate, dunningState, nil
}

func (a *app) markCollectionAttempt(ctx context.Context, invoiceID string, attempt int, providerStatus, attemptID, providerID string) error {
	state := "COLLECTING"
	if strings.EqualFold(providerStatus, "FAILED") || strings.EqualFold(providerStatus, "REQUIRES_ACTION") {
		state = "PAST_DUE"
	}
	_, err := a.db.ExecContext(ctx, `UPDATE billing.invoices
		SET provider_status=$2,payment_attempt_id=$3,provider_payment_id=$4,
			collection_attempts=GREATEST(collection_attempts,$5),last_collection_attempt_at=NOW(),
			dunning_state=CASE WHEN dunning_state IN ('SUSPENDED','PURGED','RECOVERED') THEN dunning_state ELSE $6 END
		WHERE id=$1 AND status<>'PAID'`,
		invoiceID, providerStatus, attemptID, providerID, attempt, state)
	return err
}

func (a *app) notifyDunning(ctx context.Context, eventType, severity, title, message, partnerID string, metadata map[string]any) {
	if strings.TrimSpace(a.notificationsHost) == "" { return }
	payload := map[string]any{
		"event_type": eventType,
		"severity": severity,
		"title": title,
		"message": message,
		"resource": "billing",
		"partner_id": partnerID,
		"deep_link": "/partner/billing",
		"audience_permission": "billing.read",
		"metadata": metadata,
	}
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+a.notificationsHost+"/internal/v1/notifications/events", bytes.NewReader(raw))
	if err != nil { return }
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Himate-Internal-Token", a.token)
	resp, err := a.client.Do(req)
	if err == nil && resp != nil { resp.Body.Close() }
}

func (a *app) partnerLifecycle(ctx context.Context, partnerID string) (string, error) {
	if strings.TrimSpace(a.partnersHost) == "" { return "", fmt.Errorf("PARTNERS_HOSTPORT is required") }
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+a.partnersHost+"/api/v1/partners/"+partnerID, nil)
	if err != nil { return "", err }
	req.Header.Set("X-Himate-Internal-Token", a.token)
	resp, err := a.client.Do(req)
	if err != nil { return "", err }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK { return "", fmt.Errorf("partners status %d", resp.StatusCode) }
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil { return "", err }
	return strings.ToUpper(strings.TrimSpace(fmt.Sprint(out["lifecycle"]))), nil
}

func (a *app) setPartnerLifecycle(ctx context.Context, partnerID, lifecycle, reason string) error {
	payload := map[string]any{"lifecycle": lifecycle, "reason": reason}
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, "http://"+a.partnersHost+"/api/v1/partners/"+partnerID, bytes.NewReader(raw))
	if err != nil { return err }
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Himate-Internal-Token", a.token)
	req.Header.Set("X-Himate-User-ID", "billing-dunning-engine")
	resp, err := a.client.Do(req)
	if err != nil { return err }
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 { return fmt.Errorf("partners lifecycle status %d", resp.StatusCode) }
	return nil
}

func (a *app) purgePartnerOperationalAccess(ctx context.Context, partnerID, reason string) error {
	payload := map[string]any{"reason": reason}
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+a.partnersHost+"/api/v1/partners/"+partnerID+"/purge-operational", bytes.NewReader(raw))
	if err != nil { return err }
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Himate-Internal-Token", a.token)
	req.Header.Set("X-Himate-User-ID", "billing-dunning-engine")
	resp, err := a.client.Do(req)
	if err != nil { return err }
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 { return fmt.Errorf("partners operational purge status %d", resp.StatusCode) }
	return nil
}

func (a *app) setPlanEntitlements(ctx context.Context, partnerID, planKey string, moduleKeys []string, reason string) error {
	payload := map[string]any{"plan_key": planKey, "module_keys": moduleKeys, "reason": reason}
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, "http://"+a.catalogHost+"/internal/v1/partners/"+partnerID+"/plan-entitlements", bytes.NewReader(raw))
	if err != nil { return err }
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Himate-Internal-Token", a.token)
	req.Header.Set("X-Himate-User-ID", "billing-dunning-engine")
	resp, err := a.client.Do(req)
	if err != nil { return err }
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 { return fmt.Errorf("catalog entitlement status %d", resp.StatusCode) }
	return nil
}

func (a *app) markPastDue(ctx context.Context, invoiceID, partnerID string, attempts int, invoiceDate time.Time) error {
	if attempts <= 0 || attempts >= dunningMaxAttempts { return nil }
	if _, err := a.db.ExecContext(ctx, `UPDATE billing.invoices SET dunning_state='PAST_DUE'
		WHERE id=$1 AND status<>'PAID' AND dunning_state NOT IN ('SUSPENDED','PURGED','RECOVERED')`, invoiceID); err != nil {
		return err
	}
	if _, err := a.db.ExecContext(ctx, `UPDATE billing.partner_plan_subscriptions SET status='PAST_DUE',updated_at=NOW()
		WHERE partner_id=$1 AND status='ACTIVE'`, partnerID); err != nil {
		return err
	}
	next := invoiceDate.AddDate(0, 0, dunningSecondOffset)
	if attempts == 2 { next = invoiceDate.AddDate(0, 0, dunningThirdOffset) }
	a.notifyDunning(ctx, "PAYMENT_RETRY_SCHEDULED", "WARNING", "Payment retry scheduled",
		fmt.Sprintf("Recurring payment attempt %d of %d failed. The next automatic attempt is scheduled for %s.",
			attempts, dunningMaxAttempts, dateOnly(next).Format("2006-01-02")),
		partnerID, map[string]any{"invoice_id": invoiceID, "attempt": attempts, "next_attempt_at": dateOnly(next).Format("2006-01-02")})
	return nil
}

func (a *app) suspendForNonPayment(ctx context.Context, invoiceID, partnerID string, at time.Time) error {
	at = dateOnly(at)
	var state, planKey, previous string
	var purgeDue sql.NullTime
	if err := a.db.QueryRowContext(ctx, `SELECT dunning_state,plan_key,pre_suspend_partner_lifecycle,purge_due_at
		FROM billing.invoices WHERE id=$1`, invoiceID).Scan(&state, &planKey, &previous, &purgeDue); err != nil {
		return err
	}
	if state == "PURGED" || state == "RECOVERED" { return nil }
	if state != "SUSPENDED" {
		lifecycle, err := a.partnerLifecycle(ctx, partnerID)
		if err != nil { return err }
		if lifecycle != "SUSPENDED" && lifecycle != "ARCHIVED" {
			previous = lifecycle
			if err := a.setPartnerLifecycle(ctx, partnerID, "SUSPENDED", "Three recurring payment attempts failed"); err != nil { return err }
		}
		if !strings.EqualFold(planKey, "CUSTOM") {
			if err := a.setPlanEntitlements(ctx, partnerID, planKey, []string{}, "Recurring payment suspended after three failed attempts"); err != nil { return err }
		}
		due := at.AddDate(0, 0, dunningCureDays)
		if _, err := a.db.ExecContext(ctx, `UPDATE billing.invoices SET dunning_state='SUSPENDED',dunning_suspended_at=$2,purge_due_at=$3,
			pre_suspend_partner_lifecycle=CASE WHEN pre_suspend_partner_lifecycle='' THEN $4 ELSE pre_suspend_partner_lifecycle END
			WHERE id=$1`, invoiceID, at, due, previous); err != nil { return err }
		if _, err := a.db.ExecContext(ctx, `UPDATE billing.partner_plan_subscriptions SET status='SUSPENDED',updated_at=NOW()
			WHERE partner_id=$1 AND status<>'CANCELLED'`, partnerID); err != nil { return err }
		purgeDue = sql.NullTime{Time: due, Valid: true}
	}
	if purgeDue.Valid {
		a.notifyDunning(ctx, "PARTNER_SUSPENDED_NONPAYMENT", "CRITICAL", "Service suspended for non-payment",
			fmt.Sprintf("Three automatic payment attempts failed. Service is inactive. Resolve payment by %s to restore access; after that date the operational account will be closed and purged while legally required billing/audit records are retained.", dateOnly(purgeDue.Time).Format("2006-01-02")),
			partnerID, map[string]any{"invoice_id": invoiceID, "suspended_at": at.Format("2006-01-02"), "purge_due_at": dateOnly(purgeDue.Time).Format("2006-01-02")})
	}
	return nil
}

func (a *app) recoverDunningPayment(ctx context.Context, invoiceID, partnerID string, at time.Time) error {
	at = dateOnly(at)
	var state, planKey, previous string
	var purgeDue sql.NullTime
	if err := a.db.QueryRowContext(ctx, `SELECT dunning_state,plan_key,pre_suspend_partner_lifecycle,purge_due_at
		FROM billing.invoices WHERE id=$1`, invoiceID).Scan(&state, &planKey, &previous, &purgeDue); err != nil {
		return err
	}
	if state == "PURGED" { return nil }
	if purgeDue.Valid && at.After(dateOnly(purgeDue.Time)) { return nil }

	if state == "SUSPENDED" {
		if strings.TrimSpace(previous) != "" && previous != "SUSPENDED" && previous != "ARCHIVED" {
			if err := a.setPartnerLifecycle(ctx, partnerID, previous, "Recurring payment recovered within cure window"); err != nil { return err }
		}
		if err := a.syncPlanEntitlements(ctx, partnerID, planKey, at); err != nil { return err }
	}
	if _, err := a.db.ExecContext(ctx, `UPDATE billing.partner_plan_subscriptions SET status='ACTIVE',updated_at=NOW()
		WHERE partner_id=$1 AND status IN ('PAST_DUE','SUSPENDED')`, partnerID); err != nil { return err }
	if _, err := a.db.ExecContext(ctx, `UPDATE billing.invoices SET dunning_state='RECOVERED' WHERE id=$1 AND dunning_state<>'PURGED'`, invoiceID); err != nil { return err }
	a.notifyDunning(ctx, "PAYMENT_RECOVERED", "INFO", "Payment recovered",
		"Recurring payment was recovered and the subscription is active again.", partnerID,
		map[string]any{"invoice_id": invoiceID, "recovered_at": at.Format("2006-01-02")})
	return nil
}

func (a *app) archiveExpiredDunning(ctx context.Context, at time.Time) error {
	at = dateOnly(at)
	rows, err := a.db.QueryContext(ctx, `SELECT id,partner_id,plan_key FROM billing.invoices
		WHERE dunning_state='SUSPENDED' AND purge_due_at IS NOT NULL AND purge_due_at<=$1 AND operational_purged_at IS NULL
		ORDER BY purge_due_at,id LIMIT 200`, at)
	if err != nil { return err }
	type item struct{ invoiceID, partnerID, planKey string }
	items := []item{}
	for rows.Next() {
		var x item
		if err := rows.Scan(&x.invoiceID, &x.partnerID, &x.planKey); err != nil { rows.Close(); return err }
		items = append(items, x)
	}
	rows.Close()

	for _, x := range items {
		if err := a.setPlanEntitlements(ctx, x.partnerID, x.planKey, []string{}, "Operational account purge after non-payment cure window"); err != nil { return err }
		if err := a.purgePartnerOperationalAccess(ctx, x.partnerID, "Non-payment cure window expired"); err != nil { return err }
		if _, err := a.db.ExecContext(ctx, `UPDATE billing.partner_plan_subscriptions SET status='CANCELLED',next_plan_key='',next_billing_frequency='',change_effective_at=NULL,updated_at=NOW()
			WHERE partner_id=$1`, x.partnerID); err != nil { return err }
		if _, err := a.db.ExecContext(ctx, `DELETE FROM billing.partner_plan_module_selections WHERE partner_id=$1`, x.partnerID); err != nil { return err }
		if _, err := a.db.ExecContext(ctx, `UPDATE billing.invoices SET dunning_state='PURGED',operational_purged_at=NOW() WHERE id=$1`, x.invoiceID); err != nil { return err }
		a.notifyDunning(ctx, "PARTNER_OPERATIONAL_ACCOUNT_PURGED", "CRITICAL", "Operational account closed",
			"The non-payment cure window expired. Partner login identities and active service entitlements were removed. Financial, contract and audit evidence remains under the legal retention policy.",
			x.partnerID, map[string]any{"invoice_id": x.invoiceID, "purged_at": at.Format("2006-01-02")})
	}
	return nil
}

func (a *app) runDunningCycle(ctx context.Context, at time.Time) error {
	at = dateOnly(at)
	rows, err := a.db.QueryContext(ctx, `SELECT id,partner_id,invoice_date,collection_attempts,provider_status,dunning_state,status
		FROM billing.invoices
		WHERE billing_model='PLAN' AND charge_type IN ('PLAN_MONTHLY','PLAN_ANNUAL_RENEWAL')
		  AND dunning_state<>'PURGED' AND operational_purged_at IS NULL
		ORDER BY invoice_date,id`)
	if err != nil { return err }
	type item struct {
		id, partnerID, providerStatus, dunningState, invoiceStatus string
		invoiceDate time.Time
		attempts int
	}
	items := []item{}
	for rows.Next() {
		var x item
		if err := rows.Scan(&x.id,&x.partnerID,&x.invoiceDate,&x.attempts,&x.providerStatus,&x.dunningState,&x.invoiceStatus); err != nil { rows.Close(); return err }
		items = append(items,x)
	}
	rows.Close()

	for _, x := range items {
		if x.invoiceStatus == "PAID" {
			if x.dunningState == "PAST_DUE" || x.dunningState == "SUSPENDED" {
				if err := a.recoverDunningPayment(ctx, x.id, x.partnerID, at); err != nil { return err }
			}
			continue
		}
		failed := strings.EqualFold(x.providerStatus, "FAILED") || strings.EqualFold(x.providerStatus, "REQUIRES_ACTION")
		if !failed { continue }
		if x.attempts >= dunningMaxAttempts {
			if err := a.suspendForNonPayment(ctx, x.id, x.partnerID, at); err != nil { return err }
			continue
		}
		if x.attempts > 0 {
			if err := a.markPastDue(ctx, x.id, x.partnerID, x.attempts, x.invoiceDate); err != nil { return err }
		}
		if collectionRetryDue(x.invoiceDate, at, x.attempts) {
			var currency string
			var total float64
			if err := a.db.QueryRowContext(ctx, `SELECT currency,total FROM billing.invoices WHERE id=$1`, x.id).Scan(&currency,&total); err != nil { return err }
			a.queueInvoiceCollection(ctx, x.id, x.partnerID, currency, total)
		}
	}
	return a.archiveExpiredDunning(ctx, at)
}
