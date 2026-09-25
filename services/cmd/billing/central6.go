package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"himate.local/services/internal/common"
)

const (
	onboardingRegistered     = "REGISTERED"
	onboardingPendingReview  = "PENDING_REVIEW"
	onboardingClassified     = "CLASSIFIED"
	onboardingInvoicePending = "INVOICE_PENDING"
	onboardingPaymentPending = "PAYMENT_PENDING"
	onboardingAdminApproval  = "ADMIN_APPROVAL"
	onboardingActive         = "ACTIVE"

	classificationUnclassified = "UNCLASSIFIED"
	classificationPaid         = "PAID"
	classificationCharity      = "CHARITY"
	classificationSponsored    = "SPONSORED"
	classificationComplimentary = "COMPLIMENTARY"

	invoiceDraft     = "DRAFT"
	invoiceApproved  = "APPROVED"
	invoiceSent      = "SENT"
	invoicePaid      = "PAID"
	invoiceCancelled = "CANCELLED"
)

type onboardingState struct {
	PartnerID       string
	State           string
	Classification  string
	PortalEnabled   bool
	ReviewedAt      sql.NullTime
	ReviewedBy      string
	ApprovedAt      sql.NullTime
	ApprovedBy      string
	Reason          string
	UpdatedAt       time.Time
}

func central6BillingMigration() common.Migration {
	return common.Migration{
		Version: 18,
		Name:    "central-6-finance-onboarding-invoice-approval",
		Statements: []string{
			`ALTER TABLE billing.invoices ADD COLUMN IF NOT EXISTS workflow_status TEXT NOT NULL DEFAULT 'DRAFT'`,
			`ALTER TABLE billing.invoices ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'AUTOMATED'`,
			`ALTER TABLE billing.invoices ADD COLUMN IF NOT EXISTS approved_at TIMESTAMPTZ`,
			`ALTER TABLE billing.invoices ADD COLUMN IF NOT EXISTS approved_by TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE billing.invoices ADD COLUMN IF NOT EXISTS sent_at TIMESTAMPTZ`,
			`ALTER TABLE billing.invoices ADD COLUMN IF NOT EXISTS sent_by TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE billing.invoices ADD COLUMN IF NOT EXISTS cancelled_at TIMESTAMPTZ`,
			`ALTER TABLE billing.invoices ADD COLUMN IF NOT EXISTS cancelled_by TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE billing.invoices ADD COLUMN IF NOT EXISTS cancellation_reason TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE billing.invoices ADD COLUMN IF NOT EXISTS delivery_channel TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE billing.invoices ADD COLUMN IF NOT EXISTS due_date DATE`,
			`ALTER TABLE billing.invoices ADD COLUMN IF NOT EXISTS notes TEXT NOT NULL DEFAULT ''`,
			`UPDATE billing.invoices
				SET workflow_status=CASE WHEN status='PAID' THEN 'PAID' ELSE 'SENT' END,
					sent_at=CASE WHEN status='PAID' THEN COALESCE(paid_at,created_at) ELSE COALESCE(sent_at,created_at) END
				WHERE workflow_status='DRAFT' AND created_at < NOW()`,
			`ALTER TABLE billing.invoices DROP CONSTRAINT IF EXISTS billing_invoice_workflow_status_check`,
			`ALTER TABLE billing.invoices ADD CONSTRAINT billing_invoice_workflow_status_check
				CHECK(workflow_status IN ('DRAFT','APPROVED','SENT','PAID','CANCELLED'))`,
			`CREATE INDEX IF NOT EXISTS billing_invoice_workflow_idx
				ON billing.invoices(workflow_status,invoice_date DESC,created_at DESC)`,
			`CREATE TABLE IF NOT EXISTS billing.finance_transactions(
				id BIGSERIAL PRIMARY KEY,
				partner_id TEXT NOT NULL,
				invoice_id TEXT NOT NULL DEFAULT '',
				transaction_type TEXT NOT NULL,
				status TEXT NOT NULL,
				currency TEXT NOT NULL DEFAULT 'USD',
				net_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
				tax_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
				gross_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
				source TEXT NOT NULL DEFAULT 'SYSTEM',
				actor TEXT NOT NULL DEFAULT '',
				reason TEXT NOT NULL DEFAULT '',
				occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE INDEX IF NOT EXISTS billing_finance_transactions_partner_idx
				ON billing.finance_transactions(partner_id,occurred_at DESC,id DESC)`,
			`CREATE INDEX IF NOT EXISTS billing_finance_transactions_invoice_idx
				ON billing.finance_transactions(invoice_id,occurred_at DESC,id DESC) WHERE invoice_id<>''`,
			`CREATE UNIQUE INDEX IF NOT EXISTS billing_finance_transactions_invoice_state_unique
				ON billing.finance_transactions(invoice_id,status,source) WHERE invoice_id<>''`,
			`CREATE TABLE IF NOT EXISTS billing.invoice_delivery_outbox(
				id BIGSERIAL PRIMARY KEY,
				invoice_id TEXT NOT NULL,
				partner_id TEXT NOT NULL,
				channel TEXT NOT NULL,
				status TEXT NOT NULL DEFAULT 'QUEUED',
				destination TEXT NOT NULL DEFAULT '',
				error_message TEXT NOT NULL DEFAULT '',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				delivered_at TIMESTAMPTZ,
				UNIQUE(invoice_id,channel)
			)`,
			`CREATE TABLE IF NOT EXISTS billing.partner_onboarding(
				partner_id TEXT PRIMARY KEY,
				state TEXT NOT NULL DEFAULT 'REGISTERED',
				classification TEXT NOT NULL DEFAULT 'UNCLASSIFIED',
				portal_enabled BOOLEAN NOT NULL DEFAULT FALSE,
				reviewed_at TIMESTAMPTZ,
				reviewed_by TEXT NOT NULL DEFAULT '',
				approved_at TIMESTAMPTZ,
				approved_by TEXT NOT NULL DEFAULT '',
				reason TEXT NOT NULL DEFAULT '',
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				CHECK(state IN ('REGISTERED','PENDING_REVIEW','CLASSIFIED','INVOICE_PENDING','PAYMENT_PENDING','ADMIN_APPROVAL','ACTIVE')),
				CHECK(classification IN ('UNCLASSIFIED','PAID','CHARITY','SPONSORED','COMPLIMENTARY'))
			)`,
			`CREATE TABLE IF NOT EXISTS billing.partner_onboarding_history(
				id BIGSERIAL PRIMARY KEY,
				partner_id TEXT NOT NULL,
				old_state TEXT NOT NULL,
				new_state TEXT NOT NULL,
				old_classification TEXT NOT NULL,
				new_classification TEXT NOT NULL,
				actor TEXT NOT NULL DEFAULT '',
				reason TEXT NOT NULL DEFAULT '',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE INDEX IF NOT EXISTS billing_partner_onboarding_state_idx
				ON billing.partner_onboarding(state,classification,updated_at DESC)`,
			`CREATE TABLE IF NOT EXISTS billing.partner_support_waivers(
				partner_id TEXT PRIMARY KEY,
				classification TEXT NOT NULL,
				nominal_value NUMERIC(12,2) NOT NULL DEFAULT 0,
				currency TEXT NOT NULL DEFAULT 'USD',
				reason TEXT NOT NULL,
				evidence_reference TEXT NOT NULL DEFAULT '',
				approved_by TEXT NOT NULL DEFAULT '',
				approved_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				CHECK(classification IN ('CHARITY','SPONSORED','COMPLIMENTARY'))
			)`,
			`INSERT INTO billing.partner_onboarding(partner_id,state,classification,portal_enabled,reviewed_at,reviewed_by,approved_at,approved_by,reason)
				SELECT p.id,'ACTIVE',
					CASE
						WHEN pcm.billing_mode='CHARITY' THEN 'CHARITY'
						WHEN pcm.billing_mode='COMPLIMENTARY' THEN 'COMPLIMENTARY'
						ELSE 'PAID'
					END,
					TRUE,NOW(),'central-6-migration',NOW(),'central-6-migration',
					'Existing partner grandfathered into Central-6 onboarding'
				FROM partners.partners p
				LEFT JOIN billing.partner_commercial_modes pcm ON pcm.partner_id=p.id
				ON CONFLICT(partner_id) DO NOTHING`,
		},
	}
}

func nullableCentral6Time(v sql.NullTime) any {
	if !v.Valid {
		return nil
	}
	return v.Time.UTC()
}

func onboardingMap(x onboardingState) map[string]any {
	return map[string]any{
		"partner_id": x.PartnerID,
		"state": x.State,
		"classification": x.Classification,
		"portal_enabled": x.PortalEnabled,
		"reviewed_at": nullableCentral6Time(x.ReviewedAt),
		"reviewed_by": x.ReviewedBy,
		"approved_at": nullableCentral6Time(x.ApprovedAt),
		"approved_by": x.ApprovedBy,
		"reason": x.Reason,
		"updated_at": x.UpdatedAt,
	}
}

func (a *app) ensureOnboarding(ctx context.Context, partnerID string) (onboardingState, error) {
	if _, err := a.db.ExecContext(ctx, `INSERT INTO billing.partner_onboarding(
			partner_id,state,classification,portal_enabled,reviewed_at,reviewed_by,approved_at,approved_by,reason)
		SELECT p.id,
			CASE WHEN COALESCE(p.onboarding_request_id,'')<>'' THEN 'REGISTERED' ELSE 'ACTIVE' END,
			CASE WHEN COALESCE(p.onboarding_request_id,'')<>'' THEN 'UNCLASSIFIED' ELSE 'PAID' END,
			CASE WHEN COALESCE(p.onboarding_request_id,'')<>'' THEN FALSE ELSE TRUE END,
			CASE WHEN COALESCE(p.onboarding_request_id,'')<>'' THEN NULL ELSE NOW() END,
			CASE WHEN COALESCE(p.onboarding_request_id,'')<>'' THEN '' ELSE 'central-6-admin-default' END,
			CASE WHEN COALESCE(p.onboarding_request_id,'')<>'' THEN NULL ELSE NOW() END,
			CASE WHEN COALESCE(p.onboarding_request_id,'')<>'' THEN '' ELSE 'central-6-admin-default' END,
			CASE WHEN COALESCE(p.onboarding_request_id,'')<>'' THEN 'Registration awaiting HIMATE review' ELSE 'HIMATE-admin-created partner' END
		FROM partners.partners p WHERE p.id=$1
		ON CONFLICT(partner_id) DO NOTHING`, partnerID); err != nil {
		return onboardingState{}, err
	}
	var x onboardingState
	err := a.db.QueryRowContext(ctx, `SELECT partner_id,state,classification,portal_enabled,
		reviewed_at,reviewed_by,approved_at,approved_by,reason,updated_at
		FROM billing.partner_onboarding WHERE partner_id=$1`, partnerID).
		Scan(&x.PartnerID, &x.State, &x.Classification, &x.PortalEnabled,
			&x.ReviewedAt, &x.ReviewedBy, &x.ApprovedAt, &x.ApprovedBy, &x.Reason, &x.UpdatedAt)
	return x, err
}

func central6OnboardingTransitionAllowed(from, to string) bool {
	if from == to {
		return true
	}
	allowed := map[string]map[string]bool{
		onboardingRegistered:     {onboardingPendingReview: true},
		onboardingPendingReview:  {onboardingClassified: true},
		onboardingClassified:     {onboardingInvoicePending: true, onboardingAdminApproval: true},
		onboardingInvoicePending: {onboardingPaymentPending: true, onboardingAdminApproval: true},
		onboardingPaymentPending: {onboardingAdminApproval: true},
		onboardingAdminApproval:  {onboardingActive: true, onboardingPendingReview: true},
		onboardingActive:         {onboardingPendingReview: true},
	}
	return allowed[from][to]
}

func zeroDollarClassification(classification string) bool {
	switch classification {
	case classificationCharity, classificationSponsored, classificationComplimentary:
		return true
	default:
		return false
	}
}

func (a *app) onboardingPrerequisite(ctx context.Context, partnerID, nextState, classification string) error {
	if nextState == onboardingClassified && classification == classificationUnclassified {
		return fmt.Errorf("commercial classification is required")
	}
	if nextState == onboardingInvoicePending && classification != classificationPaid {
		return fmt.Errorf("only PAID onboarding uses invoice/payment steps")
	}
	if nextState == onboardingPaymentPending {
		var count int
		if err := a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM billing.invoices
			WHERE partner_id=$1 AND workflow_status='SENT' AND total>0`, partnerID).Scan(&count); err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("a sent invoice is required before payment pending")
		}
	}
	if nextState == onboardingAdminApproval {
		if classification == classificationPaid {
			var count int
			if err := a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM billing.invoices
				WHERE partner_id=$1 AND workflow_status='PAID' AND total>0`, partnerID).Scan(&count); err != nil {
				return err
			}
			if count == 0 {
				return fmt.Errorf("a paid invoice is required before administrator approval")
			}
		} else if zeroDollarClassification(classification) {
			var count int
			if err := a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM billing.partner_support_waivers
				WHERE partner_id=$1`, partnerID).Scan(&count); err != nil {
				return err
			}
			if count == 0 {
				return fmt.Errorf("documented zero-dollar waiver/support approval is required")
			}
		}
	}
	if nextState == onboardingActive {
		var state string
		if err := a.db.QueryRowContext(ctx, `SELECT state FROM billing.partner_onboarding WHERE partner_id=$1`, partnerID).Scan(&state); err != nil {
			return err
		}
		if state != onboardingAdminApproval {
			return fmt.Errorf("final administrator approval is required")
		}
	}
	return nil
}

func (a *app) partnerOnboarding(w http.ResponseWriter, r *http.Request, partnerID string) {
	switch r.Method {
	case http.MethodGet:
		state, err := a.ensureOnboarding(r.Context(), partnerID)
		if err != nil {
			common.APIError(w, 500, "DB", "Could not load onboarding state")
			return
		}
		out := onboardingMap(state)
		var waiverCount int
		_ = a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM billing.partner_support_waivers WHERE partner_id=$1`, partnerID).Scan(&waiverCount)
		out["support_waiver_documented"] = waiverCount > 0
		common.JSON(w, 200, out)
	case http.MethodPatch:
		current, err := a.ensureOnboarding(r.Context(), partnerID)
		if err != nil {
			common.APIError(w, 500, "DB", "Could not load onboarding state")
			return
		}
		var in struct {
			State             string  `json:"state"`
			Classification    string  `json:"classification"`
			Reason            string  `json:"reason"`
			NominalValue      float64 `json:"nominal_value"`
			Currency          string  `json:"currency"`
			EvidenceReference string  `json:"evidence_reference"`
		}
		if common.Decode(r, &in) != nil {
			common.APIError(w, 400, "JSON", "Invalid onboarding request")
			return
		}
		nextState := strings.ToUpper(strings.TrimSpace(in.State))
		if nextState == "" {
			nextState = current.State
		}
		nextClassification := strings.ToUpper(strings.TrimSpace(in.Classification))
		if nextClassification == "" {
			nextClassification = current.Classification
		}
		validStates := map[string]bool{
			onboardingRegistered:true,onboardingPendingReview:true,onboardingClassified:true,
			onboardingInvoicePending:true,onboardingPaymentPending:true,onboardingAdminApproval:true,onboardingActive:true,
		}
		validClassifications := map[string]bool{
			classificationUnclassified:true,classificationPaid:true,classificationCharity:true,
			classificationSponsored:true,classificationComplimentary:true,
		}
		if !validStates[nextState] || !validClassifications[nextClassification] {
			common.APIError(w, 400, "VALIDATION", "Invalid onboarding state or classification")
			return
		}
		if !central6OnboardingTransitionAllowed(current.State, nextState) {
			common.APIError(w, 409, "INVALID_ONBOARDING_TRANSITION", "Onboarding transition is not allowed")
			return
		}
		reason := strings.TrimSpace(in.Reason)
		if reason == "" {
			reason = "Central-6 onboarding update"
		}
		actor := strings.TrimSpace(r.Header.Get("X-Himate-User-ID"))
		if actor == "" {
			actor = "system"
		}
		if nextState == onboardingClassified && zeroDollarClassification(nextClassification) {
			if in.Currency == "" {
				in.Currency = "USD"
			}
			if strings.TrimSpace(in.Reason) == "" {
				common.APIError(w, 400, "WAIVER_REASON_REQUIRED", "Zero-dollar charity/sponsored support requires a documented reason")
				return
			}
		}
		if err := a.onboardingPrerequisite(r.Context(), partnerID, nextState, nextClassification); err != nil {
			common.APIError(w, 409, "ONBOARDING_PREREQUISITE", err.Error())
			return
		}
		tx, err := a.db.BeginTx(r.Context(), nil)
		if err != nil {
			common.APIError(w, 500, "DB", "Could not start onboarding update")
			return
		}
		defer tx.Rollback()

		if nextState == onboardingClassified {
			mode, modeErr := a.ensureCommercialMode(r.Context(), partnerID)
			if modeErr != nil {
				common.APIError(w, 500, "DB", "Could not load commercial billing mode")
				return
			}
			nextMode := billingModePaid
			nextCharityStatus := charityNotRequested
			switch nextClassification {
			case classificationCharity:
				nextMode = billingModeCharity
				nextCharityStatus = charityApproved
			case classificationSponsored, classificationComplimentary:
				nextMode = billingModeComplimentary
				nextCharityStatus = charityNotRequested
			}
			if _, err = tx.ExecContext(r.Context(), `UPDATE billing.partner_commercial_modes SET
				billing_mode=$2,charity_status=$3,charity_reviewed_at=CASE WHEN $3='APPROVED' THEN COALESCE(charity_reviewed_at,NOW()) ELSE charity_reviewed_at END,
				charity_reviewed_by=CASE WHEN $3='APPROVED' THEN CASE WHEN charity_reviewed_by='' THEN $4 ELSE charity_reviewed_by END ELSE charity_reviewed_by END,
				reason=$5,updated_at=NOW() WHERE partner_id=$1`,
				partnerID,nextMode,nextCharityStatus,actor,reason); err != nil {
				common.APIError(w, 500, "DB", "Could not synchronize commercial billing mode")
				return
			}
			if mode.BillingMode != nextMode || mode.CharityStatus != nextCharityStatus {
				if _, err = tx.ExecContext(r.Context(), `INSERT INTO billing.partner_commercial_mode_history(
					partner_id,old_billing_mode,new_billing_mode,old_charity_status,new_charity_status,actor,reason)
					VALUES($1,$2,$3,$4,$5,$6,$7)`,
					partnerID,mode.BillingMode,nextMode,mode.CharityStatus,nextCharityStatus,actor,reason); err != nil {
					common.APIError(w, 500, "DB", "Could not record commercial mode audit history")
					return
				}
			}
		}

		if nextState == onboardingClassified && zeroDollarClassification(nextClassification) {
			currency := strings.ToUpper(strings.TrimSpace(in.Currency))
			if currency == "" {
				currency = "USD"
			}
			if _, err = tx.ExecContext(r.Context(), `INSERT INTO billing.partner_support_waivers(
				partner_id,classification,nominal_value,currency,reason,evidence_reference,approved_by,approved_at)
				VALUES($1,$2,$3,$4,$5,$6,$7,NOW())
				ON CONFLICT(partner_id) DO UPDATE SET
					classification=EXCLUDED.classification,nominal_value=EXCLUDED.nominal_value,currency=EXCLUDED.currency,
					reason=EXCLUDED.reason,evidence_reference=EXCLUDED.evidence_reference,approved_by=EXCLUDED.approved_by,approved_at=NOW()`,
				partnerID,nextClassification,math.Max(0,in.NominalValue),currency,reason,strings.TrimSpace(in.EvidenceReference),actor); err != nil {
				common.APIError(w, 500, "DB", "Could not document zero-dollar support")
				return
			}
			if _, err = tx.ExecContext(r.Context(), `INSERT INTO billing.initial_licenses(partner_id,currency,required_amount,paid_amount,status,waived,waiver_reason,note)
				VALUES($1,$2,0,0,'WAIVED',TRUE,$3,'Central-6 documented zero-dollar support')
				ON CONFLICT(partner_id) DO UPDATE SET currency=EXCLUDED.currency,required_amount=0,paid_amount=0,status='WAIVED',
					waived=TRUE,waiver_reason=EXCLUDED.waiver_reason,payment_date=NULL,payment_reference='',verified_by='',updated_at=NOW()`,
				partnerID,currency,reason); err != nil {
				common.APIError(w, 500, "DB", "Could not synchronize activation waiver")
				return
			}
		}

		portalEnabled := nextState == onboardingActive
		reviewedExpr := current.ReviewedAt
		reviewedBy := current.ReviewedBy
		if nextState == onboardingPendingReview && current.State != onboardingPendingReview {
			reviewedExpr = sql.NullTime{Time:time.Now().UTC(),Valid:true}
			reviewedBy = actor
		}
		var approvedAt any
		approvedBy := current.ApprovedBy
		if current.ApprovedAt.Valid {
			approvedAt = current.ApprovedAt.Time
		}
		if nextState == onboardingActive {
			approvedAt = time.Now().UTC()
			approvedBy = actor
		}
		if _, err = tx.ExecContext(r.Context(), `UPDATE billing.partner_onboarding SET
			state=$2,classification=$3,portal_enabled=$4,reviewed_at=$5,reviewed_by=$6,
			approved_at=$7,approved_by=$8,reason=$9,updated_at=NOW()
			WHERE partner_id=$1`, partnerID,nextState,nextClassification,portalEnabled,
			func() any { if reviewedExpr.Valid { return reviewedExpr.Time }; return nil }(),
			reviewedBy,approvedAt,approvedBy,reason); err != nil {
			common.APIError(w, 500, "DB", "Could not update onboarding state")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO billing.partner_onboarding_history(
			partner_id,old_state,new_state,old_classification,new_classification,actor,reason)
			VALUES($1,$2,$3,$4,$5,$6,$7)`,
			partnerID,current.State,nextState,current.Classification,nextClassification,actor,reason); err != nil {
			common.APIError(w, 500, "DB", "Could not record onboarding audit history")
			return
		}
		if err = tx.Commit(); err != nil {
			common.APIError(w, 500, "DB", "Could not commit onboarding update")
			return
		}
		state, _ := a.ensureOnboarding(r.Context(), partnerID)
		common.JSON(w, 200, onboardingMap(state))
	default:
		common.APIError(w, 405, "METHOD", "Use GET or PATCH")
	}
}

func (a *app) portalGate(w http.ResponseWriter, r *http.Request, partnerID string) {
	state, err := a.ensureOnboarding(r.Context(), partnerID)
	if err != nil {
		common.APIError(w, 500, "DB", "Could not load Partner Portal onboarding gate")
		return
	}
	allowed := state.State == onboardingActive && state.PortalEnabled
	reason := ""
	if !allowed {
		reason = "Partner onboarding is not fully approved"
	}
	common.JSON(w, 200, map[string]any{
		"partner_id": partnerID,
		"allowed": allowed,
		"state": state.State,
		"classification": state.Classification,
		"portal_enabled": state.PortalEnabled,
		"reason": reason,
	})
}

func (a *app) recordFinanceTransactionTx(ctx context.Context, tx *sql.Tx, partnerID, invoiceID, kind, status, currency string, net, tax, gross float64, source, actor, reason string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO billing.finance_transactions(
		partner_id,invoice_id,transaction_type,status,currency,net_amount,tax_amount,gross_amount,source,actor,reason)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		partnerID,invoiceID,kind,status,currency,net,tax,gross,source,actor,reason)
	return err
}

func (a *app) invoiceCollectionApproved(ctx context.Context, invoiceID string) bool {
	var workflow string
	if err := a.db.QueryRowContext(ctx, `SELECT workflow_status FROM billing.invoices WHERE id=$1`, invoiceID).Scan(&workflow); err != nil {
		return false
	}
	return workflow == invoiceSent
}

func (a *app) invoiceWorkflowMeta(ctx context.Context, invoiceID string) (map[string]any, error) {
	var workflow, source, approvedBy, sentBy, cancelledBy, cancellationReason, deliveryChannel, notes string
	var approvedAt, sentAt, cancelledAt, dueDate sql.NullTime
	err := a.db.QueryRowContext(ctx, `SELECT workflow_status,source,approved_at,approved_by,sent_at,sent_by,
		cancelled_at,cancelled_by,cancellation_reason,delivery_channel,due_date,notes
		FROM billing.invoices WHERE id=$1`, invoiceID).
		Scan(&workflow,&source,&approvedAt,&approvedBy,&sentAt,&sentBy,
			&cancelledAt,&cancelledBy,&cancellationReason,&deliveryChannel,&dueDate,&notes)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"workflow_status":workflow,"source":source,
		"approved_at":nullableCentral6Time(approvedAt),"approved_by":approvedBy,
		"sent_at":nullableCentral6Time(sentAt),"sent_by":sentBy,
		"cancelled_at":nullableCentral6Time(cancelledAt),"cancelled_by":cancelledBy,
		"cancellation_reason":cancellationReason,"delivery_channel":deliveryChannel,
		"due_date":func() any { if dueDate.Valid { return dueDate.Time.Format("2006-01-02") }; return nil }(),
		"notes":notes,
	}, nil
}

func (a *app) manualInvoiceCreate(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PartnerID          string  `json:"partner_id"`
		Currency           string  `json:"currency"`
		Description        string  `json:"description"`
		NetAmount          float64 `json:"net_amount"`
		TaxRatePercent     *float64 `json:"tax_rate_percent"`
		ServicePeriodStart string  `json:"service_period_start"`
		ServicePeriodEnd   string  `json:"service_period_end_exclusive"`
		DueDate            string  `json:"due_date"`
		Notes              string  `json:"notes"`
	}
	if common.Decode(r,&in)!=nil {
		common.APIError(w,400,"JSON","Invalid manual invoice request")
		return
	}
	in.PartnerID = strings.TrimSpace(in.PartnerID)
	in.Description = strings.TrimSpace(in.Description)
	if in.PartnerID=="" || in.Description=="" || in.NetAmount<=0 {
		common.APIError(w,400,"VALIDATION","partner_id, description and positive net_amount are required")
		return
	}
	onboarding,err:=a.ensureOnboarding(r.Context(),in.PartnerID)
	if err!=nil{common.APIError(w,500,"DB","Could not load onboarding state");return}
	if zeroDollarClassification(onboarding.Classification) {
		common.APIError(w,409,"ZERO_DOLLAR_NO_INVOICE","Charity/sponsored/complimentary partners use documented waiver/support and do not receive zero-dollar invoices")
		return
	}
	currency:=strings.ToUpper(strings.TrimSpace(in.Currency));if currency==""{currency="USD"}
	start:=dateOnly(time.Now().UTC());end:=start.AddDate(0,1,0)
	var parseErr error
	if in.ServicePeriodStart!="" { start,parseErr=time.Parse("2006-01-02",in.ServicePeriodStart);if parseErr!=nil{common.APIError(w,400,"VALIDATION","service_period_start must be YYYY-MM-DD");return} }
	if in.ServicePeriodEnd!="" { end,parseErr=time.Parse("2006-01-02",in.ServicePeriodEnd);if parseErr!=nil{common.APIError(w,400,"VALIDATION","service_period_end_exclusive must be YYYY-MM-DD");return} }
	if !end.After(start){common.APIError(w,400,"VALIDATION","service period end must be after start");return}
	policy,err:=a.loadBillingTaxPolicy(r.Context());if err!=nil{common.APIError(w,500,"DB","Could not load VAT policy");return}
	if in.TaxRatePercent!=nil{policy.RatePercent=*in.TaxRatePercent}
	net,tax,gross:=applyBillingTax(in.NetAmount,policy)
	id:=fmt.Sprintf("inv_manual_%s_%d",strings.ReplaceAll(in.PartnerID,"_",""),time.Now().UTC().UnixNano())
	invoiceKey:="MANUAL:"+id
	var due any
	if strings.TrimSpace(in.DueDate)!="" {
		d,err:=time.Parse("2006-01-02",strings.TrimSpace(in.DueDate));if err!=nil{common.APIError(w,400,"VALIDATION","due_date must be YYYY-MM-DD");return};due=d
	}
	actor:=strings.TrimSpace(r.Header.Get("X-Himate-User-ID"));if actor==""{actor="system"}
	tx,err:=a.db.BeginTx(r.Context(),nil);if err!=nil{common.APIError(w,500,"DB","Could not start manual invoice");return}
	defer tx.Rollback()
	_,err=tx.ExecContext(r.Context(),`INSERT INTO billing.invoices(
		id,invoice_key,partner_id,invoice_date,service_period_start,service_period_end,currency,base_fee,module_fee,total,status,provider_status,
		plan_key,billing_frequency,charge_type,list_price,discount_amount,minimum_commitment_adjustment,billing_model,
		net_total,tax_rate_percent,tax_amount,workflow_status,source,due_date,notes)
		VALUES($1,$2,$3,CURRENT_DATE,$4,$5,$6,$7,0,$8,'DRAFT','NOT_CONFIGURED',
		'','MANUAL','MANUAL',$7,0,0,'MANUAL',$7,$9,$10,'DRAFT','MANUAL',$11,$12)`,
		id,invoiceKey,in.PartnerID,start,end,currency,net,gross,policy.RatePercent,tax,due,strings.TrimSpace(in.Notes))
	if err!=nil{common.APIError(w,500,"DB","Could not create manual invoice");return}
	itemKey:="MANUAL_ITEM:"+id
	_,err=tx.ExecContext(r.Context(),`INSERT INTO billing.invoice_items(
		item_key,invoice_id,partner_id,item_type,description,currency,quantity,unit_price,amount,period_start,period_end,status,invoiced_at,billing_model)
		VALUES($1,$2,$3,'MANUAL',$4,$5,1,$6,$6,$7,$8,'INVOICED',NOW(),'MANUAL')`,
		itemKey,id,in.PartnerID,in.Description,currency,net,start,end)
	if err!=nil{common.APIError(w,500,"DB","Could not create manual invoice line");return}
	if tax>0{
		_,err=tx.ExecContext(r.Context(),`INSERT INTO billing.invoice_items(
			item_key,invoice_id,partner_id,item_type,description,currency,quantity,unit_price,amount,period_start,period_end,status,invoiced_at,billing_model)
			VALUES($1,$2,$3,'TAX',$4,$5,1,$6,$6,$7,$8,'INVOICED',NOW(),'MANUAL')`,
			"TAX_ITEM:"+id,id,in.PartnerID,fmt.Sprintf("%s %.2f%%",policy.Label,policy.RatePercent),currency,tax,start,end)
		if err!=nil{common.APIError(w,500,"DB","Could not create tax line");return}
	}
	if err=a.recordFinanceTransactionTx(r.Context(),tx,in.PartnerID,id,"INVOICE","DRAFT",currency,net,tax,gross,"MANUAL",actor,"Manual invoice draft created");err!=nil{
		common.APIError(w,500,"DB","Could not record finance transaction");return
	}
	if onboarding.State==onboardingClassified && onboarding.Classification==classificationPaid {
		_,_=tx.ExecContext(r.Context(),`UPDATE billing.partner_onboarding SET state='INVOICE_PENDING',reason='Manual invoice draft created',updated_at=NOW() WHERE partner_id=$1`,in.PartnerID)
		_,_=tx.ExecContext(r.Context(),`INSERT INTO billing.partner_onboarding_history(
			partner_id,old_state,new_state,old_classification,new_classification,actor,reason)
			VALUES($1,'CLASSIFIED','INVOICE_PENDING','PAID','PAID',$2,'Manual invoice draft created')`,in.PartnerID,actor)
	}
	if err=tx.Commit();err!=nil{common.APIError(w,500,"DB","Could not commit manual invoice");return}
	out,err:=a.invoiceDetail(r.Context(),id);if err!=nil{common.APIError(w,500,"DB","Invoice created but could not be reloaded");return}
	common.JSON(w,201,out)
}

func (a *app) invoiceDetail(ctx context.Context, invoiceID string) (map[string]any,error) {
	var id,partnerID,currency,status,providerStatus,planKey,billingFrequency,chargeType,billingModel string
	var invoiceDate,start,end,created time.Time
	var paidAt sql.NullTime
	var base,module,total,listPrice,discount,net,taxRate,tax float64
	err:=a.db.QueryRowContext(ctx,`SELECT id,partner_id,invoice_date,service_period_start,service_period_end,currency,
		base_fee,module_fee,total,status,provider_status,paid_at,created_at,COALESCE(plan_key,''),COALESCE(billing_frequency,''),
		COALESCE(charge_type,'LEGACY'),COALESCE(list_price,0),COALESCE(discount_amount,0),COALESCE(billing_model,'LEGACY_MODULE'),
		COALESCE(net_total,total),COALESCE(tax_rate_percent,0),COALESCE(tax_amount,0)
		FROM billing.invoices WHERE id=$1`,invoiceID).
		Scan(&id,&partnerID,&invoiceDate,&start,&end,&currency,&base,&module,&total,&status,&providerStatus,&paidAt,&created,
			&planKey,&billingFrequency,&chargeType,&listPrice,&discount,&billingModel,&net,&taxRate,&tax)
	if err!=nil{return nil,err}
	out:=map[string]any{
		"id":id,"partner_id":partnerID,"invoice_date":invoiceDate,"service_period_start":start,"service_period_end_exclusive":end,
		"currency":currency,"base_fee":base,"module_fee":module,"total":total,"status":status,"provider_status":providerStatus,
		"plan_key":planKey,"billing_frequency":billingFrequency,"charge_type":chargeType,"billing_model":billingModel,
		"list_price":listPrice,"discount_amount":discount,"net_total":net,"tax_rate_percent":taxRate,"tax_amount":tax,"gross_total":total,
		"paid_at":nullableCentral6Time(paidAt),"created_at":created,"items":a.invoiceItemsFor(invoiceID),
	}
	meta,err:=a.invoiceWorkflowMeta(ctx,invoiceID);if err!=nil{return nil,err}
	for k,v:=range meta{out[k]=v}
	return out,nil
}

func (a *app) invoiceCollection(w http.ResponseWriter,r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		status:=strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("status")))
		partnerID:=strings.TrimSpace(r.URL.Query().Get("partner_id"))
		query:=`SELECT id FROM billing.invoices WHERE 1=1`
		args:=[]any{}
		if status!=""{args=append(args,status);query+=fmt.Sprintf(" AND workflow_status=$%d",len(args))}
		if partnerID!=""{args=append(args,partnerID);query+=fmt.Sprintf(" AND partner_id=$%d",len(args))}
		query+=" ORDER BY invoice_date DESC,created_at DESC LIMIT 500"
		rows,err:=a.db.QueryContext(r.Context(),query,args...);if err!=nil{common.APIError(w,500,"DB","Could not load invoice queue");return}
		defer rows.Close()
		items:=[]map[string]any{}
		for rows.Next(){var id string;if rows.Scan(&id)!=nil{continue};item,err:=a.invoiceDetail(r.Context(),id);if err==nil{items=append(items,item)}}
		common.JSON(w,200,map[string]any{"items":items,"count":len(items)})
	case http.MethodPost:
		a.manualInvoiceCreate(w,r)
	default:
		common.APIError(w,405,"METHOD","Use GET or POST")
	}
}

func (a *app) invoiceByID(w http.ResponseWriter,r *http.Request) {
	raw:=strings.Trim(strings.TrimPrefix(r.URL.Path,"/api/v1/billing/invoices/"),"/")
	parts:=strings.Split(raw,"/")
	if len(parts)<1 || parts[0]==""{common.APIError(w,404,"NOT_FOUND","Invoice not found");return}
	id:=parts[0]
	if len(parts)==1 {
		if r.Method!=http.MethodGet{common.APIError(w,405,"METHOD","Use GET");return}
		out,err:=a.invoiceDetail(r.Context(),id);if err!=nil{common.APIError(w,404,"NOT_FOUND","Invoice not found");return}
		common.JSON(w,200,out);return
	}
	action:=parts[1]
	if action=="pdf" {
		if r.Method!=http.MethodGet{common.APIError(w,405,"METHOD","Use GET");return}
		a.invoicePDF(w,r,id);return
	}
	if r.Method!=http.MethodPost{common.APIError(w,405,"METHOD","Use POST");return}
	var in struct{Reason string `json:"reason"`;PaymentReference string `json:"payment_reference"`}
	_ = common.Decode(r,&in)
	reason:=strings.TrimSpace(in.Reason);if reason==""{reason="Central-6 invoice workflow update"}
	actor:=strings.TrimSpace(r.Header.Get("X-Himate-User-ID"));if actor==""{actor="system"}
	current,err:=a.invoiceDetail(r.Context(),id);if err!=nil{common.APIError(w,404,"NOT_FOUND","Invoice not found");return}
	workflow:=fmt.Sprint(current["workflow_status"])
	partnerID:=fmt.Sprint(current["partner_id"])
	currency:=fmt.Sprint(current["currency"])
	net:=current["net_total"].(float64);tax:=current["tax_amount"].(float64);gross:=current["gross_total"].(float64)
	tx,err:=a.db.BeginTx(r.Context(),nil);if err!=nil{common.APIError(w,500,"DB","Could not start invoice workflow");return}
	defer tx.Rollback()
	next:=workflow
	switch action {
	case "approve":
		if workflow!=invoiceDraft{common.APIError(w,409,"INVOICE_STATE","Only DRAFT invoices can be approved");return}
		next=invoiceApproved
		_,err=tx.ExecContext(r.Context(),`UPDATE billing.invoices SET workflow_status='APPROVED',approved_at=NOW(),approved_by=$2 WHERE id=$1`,id,actor)
	case "send":
		if workflow!=invoiceApproved{common.APIError(w,409,"INVOICE_STATE","Only APPROVED invoices can be sent");return}
		next=invoiceSent
		_,err=tx.ExecContext(r.Context(),`UPDATE billing.invoices SET workflow_status='SENT',sent_at=NOW(),sent_by=$2,
			delivery_channel='PORTAL_EMAIL' WHERE id=$1`,id,actor)
		if err==nil{
			_,err=tx.ExecContext(r.Context(),`INSERT INTO billing.invoice_delivery_outbox(invoice_id,partner_id,channel,status,destination)
				VALUES($1,$2,'PORTAL','QUEUED','Partner Portal'),($1,$2,'EMAIL','QUEUED','Finance contact')
				ON CONFLICT(invoice_id,channel) DO NOTHING`,id,partnerID)
		}
	case "mark-paid":
		if workflow!=invoiceSent && workflow!=invoiceApproved{common.APIError(w,409,"INVOICE_STATE","Only APPROVED or SENT invoices can be marked paid");return}
		next=invoicePaid
		ref:=strings.TrimSpace(in.PaymentReference);if ref==""{ref="manual:"+id}
		_,err=tx.ExecContext(r.Context(),`UPDATE billing.invoices SET workflow_status='PAID',status='PAID',provider_status='SUCCEEDED',
			provider='MANUAL',provider_payment_id=$2,paid_at=COALESCE(paid_at,NOW()) WHERE id=$1`,id,ref)
	case "cancel":
		if workflow==invoicePaid || workflow==invoiceCancelled{common.APIError(w,409,"INVOICE_STATE","Paid or already-cancelled invoice cannot be cancelled");return}
		next=invoiceCancelled
		_,err=tx.ExecContext(r.Context(),`UPDATE billing.invoices SET workflow_status='CANCELLED',status='CANCELLED',
			provider_status='CANCELLED',cancelled_at=NOW(),cancelled_by=$2,cancellation_reason=$3 WHERE id=$1`,id,actor,reason)
	default:
		common.APIError(w,404,"NOT_FOUND","Invoice action not found");return
	}
	if err!=nil{common.APIError(w,500,"DB","Could not update invoice workflow");return}
	if err=a.recordFinanceTransactionTx(r.Context(),tx,partnerID,id,"INVOICE",next,currency,net,tax,gross,"WORKFLOW",actor,reason);err!=nil{
		common.APIError(w,500,"DB","Could not record invoice workflow transaction");return
	}
	if err=tx.Commit();err!=nil{common.APIError(w,500,"DB","Could not commit invoice workflow");return}
	if action=="send" {
		a.sendInvoicePortalNotification(r.Context(),id,partnerID,gross,currency)
		a.queueInvoiceCollection(r.Context(),id,partnerID,currency,gross)
		a.advanceOnboardingFromInvoice(r.Context(),partnerID,onboardingPaymentPending,actor,"Invoice sent to partner")
	}
	if action=="mark-paid" {
		a.advanceOnboardingFromInvoice(r.Context(),partnerID,onboardingAdminApproval,actor,"Invoice payment recorded")
	}
	out,err:=a.invoiceDetail(r.Context(),id);if err!=nil{common.APIError(w,500,"DB","Invoice updated but could not be reloaded");return}
	common.JSON(w,200,out)
}

func (a *app) advanceOnboardingFromInvoice(ctx context.Context,partnerID,target,actor,reason string) {
	state,err:=a.ensureOnboarding(ctx,partnerID);if err!=nil{return}
	if state.Classification!=classificationPaid{return}
	if !central6OnboardingTransitionAllowed(state.State,target){return}
	if a.onboardingPrerequisite(ctx,partnerID,target,state.Classification)!=nil{return}
	_,_=a.db.ExecContext(ctx,`UPDATE billing.partner_onboarding SET state=$2,reason=$3,updated_at=NOW() WHERE partner_id=$1`,partnerID,target,reason)
	_,_=a.db.ExecContext(ctx,`INSERT INTO billing.partner_onboarding_history(
		partner_id,old_state,new_state,old_classification,new_classification,actor,reason)
		VALUES($1,$2,$3,$4,$4,$5,$6)`,partnerID,state.State,target,state.Classification,actor,reason)
}

func (a *app) sendInvoicePortalNotification(ctx context.Context,invoiceID,partnerID string,total float64,currency string) {
	if strings.TrimSpace(a.notificationsHost)==""{return}
	body,_:=json.Marshal(map[string]any{
		"event_type":"INVOICE_SENT","severity":"INFO","title":"New HIMATE invoice",
		"message":fmt.Sprintf("Invoice %s is available. Amount: %s %.2f",invoiceID,currency,total),
		"resource":"billing","partner_id":partnerID,"deep_link":"/partner/app/billing",
		"delivery_scope":"PARTNER","category":"BILLING","metadata":map[string]any{
			"invoice_id":invoiceID,"amount":total,"currency":currency,
			"pdf_path":"/partner/api/v1/billing/invoices/"+invoiceID+"/pdf",
		},
	})
	req,err:=http.NewRequestWithContext(ctx,http.MethodPost,"http://"+a.notificationsHost+"/internal/v1/notifications/events",bytes.NewReader(body))
	if err!=nil{return}
	req.Header.Set("Content-Type","application/json");common.BindInternalRequest(req,a.token)
	resp,err:=common.DoInternal(a.client,req);if err!=nil{return};defer resp.Body.Close()
	if resp.StatusCode>=200&&resp.StatusCode<300{
		_,_=a.db.ExecContext(ctx,`UPDATE billing.invoice_delivery_outbox SET status='DELIVERED',delivered_at=NOW()
			WHERE invoice_id=$1 AND channel='PORTAL'`,invoiceID)
	}
}

func pdfEscape(s string) string {
	s=strings.ReplaceAll(s,"\","\\")
	s=strings.ReplaceAll(s,"(","\(")
	s=strings.ReplaceAll(s,")","\)")
	return s
}

func basicPDF(lines []string) []byte {
	var stream strings.Builder
	stream.WriteString("BT /F1 12 Tf 50 790 Td ")
	for i,line:=range lines{
		if i>0{stream.WriteString("0 -18 Td ")}
		stream.WriteString("(");stream.WriteString(pdfEscape(line));stream.WriteString(") Tj ")
	}
	stream.WriteString("ET")
	content:=stream.String()
	objs:=[]string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream",len(content),content),
	}
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	offsets:=make([]int,len(objs)+1)
	for i,obj:=range objs{
		offsets[i+1]=out.Len()
		fmt.Fprintf(&out,"%d 0 obj\n%s\nendobj\n",i+1,obj)
	}
	xref:=out.Len()
	fmt.Fprintf(&out,"xref\n0 %d\n0000000000 65535 f \n",len(objs)+1)
	for i:=1;i<=len(objs);i++{fmt.Fprintf(&out,"%010d 00000 n \n",offsets[i])}
	fmt.Fprintf(&out,"trailer << /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF",len(objs)+1,xref)
	return out.Bytes()
}

func (a *app) invoicePDF(w http.ResponseWriter,r *http.Request,invoiceID string) {
	inv,err:=a.invoiceDetail(r.Context(),invoiceID);if err!=nil{common.APIError(w,404,"NOT_FOUND","Invoice not found");return}
	workflow:=fmt.Sprint(inv["workflow_status"])
	if workflow==invoiceDraft{common.APIError(w,409,"INVOICE_NOT_APPROVED","Draft invoice has no distributable PDF");return}
	var legal,address,taxID,email string
	_ = a.db.QueryRowContext(r.Context(),`SELECT legal_name,address,tax_id,email FROM billing.company_profile WHERE id=1`).Scan(&legal,&address,&taxID,&email)
	lines:=[]string{
		"HIMATE INVOICE",
		"Invoice: "+invoiceID,
		"Issuer: "+legal,
		"Address: "+address,
		"Tax ID: "+taxID,
		"Billing email: "+email,
		"Partner: "+fmt.Sprint(inv["partner_id"]),
		"Period: "+fmt.Sprint(inv["service_period_start"])+" - "+fmt.Sprint(inv["service_period_end_exclusive"]),
		fmt.Sprintf("Net: %s %.2f",fmt.Sprint(inv["currency"]),inv["net_total"].(float64)),
		fmt.Sprintf("Tax: %.2f%% = %.2f",inv["tax_rate_percent"].(float64),inv["tax_amount"].(float64)),
		fmt.Sprintf("Total: %s %.2f",fmt.Sprint(inv["currency"]),inv["gross_total"].(float64)),
		"Status: "+workflow,
	}
	pdf:=basicPDF(lines)
	w.Header().Set("Content-Type","application/pdf")
	w.Header().Set("Content-Disposition",fmt.Sprintf("inline; filename=%q",invoiceID+".pdf"))
	w.Header().Set("Cache-Control","private, no-store")
	w.WriteHeader(http.StatusOK)
	_,_=w.Write(pdf)
}

func (a *app) financeOverview(w http.ResponseWriter,r *http.Request) {
	if r.Method!=http.MethodGet{common.APIError(w,405,"METHOD","Use GET");return}
	type row struct{Currency string;Draft,Approved,Sent,Paid,Cancelled int;Outstanding,PaidYTD float64}
	rows,err:=a.db.QueryContext(r.Context(),`SELECT currency,
		COUNT(*) FILTER (WHERE workflow_status='DRAFT'),
		COUNT(*) FILTER (WHERE workflow_status='APPROVED'),
		COUNT(*) FILTER (WHERE workflow_status='SENT'),
		COUNT(*) FILTER (WHERE workflow_status='PAID'),
		COUNT(*) FILTER (WHERE workflow_status='CANCELLED'),
		COALESCE(SUM(total) FILTER (WHERE workflow_status IN ('APPROVED','SENT')),0),
		COALESCE(SUM(total) FILTER (WHERE workflow_status='PAID' AND paid_at>=date_trunc('year',NOW())),0)
		FROM billing.invoices GROUP BY currency ORDER BY currency`)
	if err!=nil{common.APIError(w,500,"DB","Could not calculate finance KPIs");return}
	defer rows.Close()
	currencies:=[]map[string]any{}
	for rows.Next(){var x row;if rows.Scan(&x.Currency,&x.Draft,&x.Approved,&x.Sent,&x.Paid,&x.Cancelled,&x.Outstanding,&x.PaidYTD)==nil{
		currencies=append(currencies,map[string]any{"currency":x.Currency,"draft":x.Draft,"approved":x.Approved,"sent":x.Sent,"paid":x.Paid,"cancelled":x.Cancelled,"outstanding":x.Outstanding,"paid_ytd":x.PaidYTD})
	}}
	monthRows,err:=a.db.QueryContext(r.Context(),`
		WITH months AS (
			SELECT generate_series(date_trunc('month',NOW())-INTERVAL '11 months',date_trunc('month',NOW()),INTERVAL '1 month') AS month
		), currencies AS (
			SELECT DISTINCT currency FROM billing.invoices
		)
		SELECT to_char(m.month,'YYYY-MM'),c.currency,COALESCE(SUM(i.total),0)
		FROM months m
		CROSS JOIN currencies c
		LEFT JOIN billing.invoices i ON i.workflow_status='PAID'
			AND i.currency=c.currency
			AND date_trunc('month',COALESCE(i.paid_at,i.created_at))=m.month
		GROUP BY m.month,c.currency ORDER BY c.currency,m.month`)
	if err!=nil{common.APIError(w,500,"DB","Could not calculate finance chart");return}
	defer monthRows.Close()
	monthly:=[]map[string]any{}
	for monthRows.Next(){var month,currency string;var amount float64;if monthRows.Scan(&month,&currency,&amount)==nil{monthly=append(monthly,map[string]any{"month":month,"currency":currency,"paid":math.Round(amount*100)/100})}}
	var pendingOnboarding,activeOnboarding,waived int
	_ = a.db.QueryRowContext(r.Context(),`SELECT
		COUNT(*) FILTER (WHERE state<>'ACTIVE'),COUNT(*) FILTER (WHERE state='ACTIVE'),
		COUNT(*) FILTER (WHERE classification IN ('CHARITY','SPONSORED','COMPLIMENTARY'))
		FROM billing.partner_onboarding`).Scan(&pendingOnboarding,&activeOnboarding,&waived)
	onRows,err:=a.db.QueryContext(r.Context(),`SELECT o.partner_id,COALESCE(p.display_name,o.partner_id),o.state,o.classification,o.portal_enabled,o.updated_at
		FROM billing.partner_onboarding o LEFT JOIN partners.partners p ON p.id=o.partner_id
		WHERE o.state<>'ACTIVE' ORDER BY o.updated_at ASC LIMIT 50`)
	if err!=nil{common.APIError(w,500,"DB","Could not load onboarding queue");return}
	defer onRows.Close()
	onboarding:=[]map[string]any{}
	for onRows.Next(){var id,name,state,classification string;var portal bool;var updated time.Time;if onRows.Scan(&id,&name,&state,&classification,&portal,&updated)==nil{
		onboarding=append(onboarding,map[string]any{"partner_id":id,"display_name":name,"state":state,"classification":classification,"portal_enabled":portal,"updated_at":updated})
	}}
	common.JSON(w,200,map[string]any{
		"currencies":currencies,"monthly_paid":monthly,
		"onboarding":map[string]any{"pending":pendingOnboarding,"active":activeOnboarding,"zero_dollar_supported":waived,"items":onboarding},
		"source":"CENTRAL_6_FINANCE_LEDGER",
	})
}
