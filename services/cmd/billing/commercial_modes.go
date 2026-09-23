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
	billingModePaid          = "PAID"
	billingModeComplimentary = "COMPLIMENTARY"
	billingModeCharity       = "CHARITY"

	charityNotRequested = "NOT_REQUESTED"
	charityPending      = "PENDING"
	charityApproved     = "APPROVED"
	charityRejected     = "REJECTED"
)

type partnerCommercialMode struct {
	PartnerID         string
	BillingMode       string
	CharityStatus     string
	CharityRequested  sql.NullTime
	CharityReviewed   sql.NullTime
	CharityReviewedBy string
	Reason            string
	UpdatedAt         time.Time
}

func start23113kCommercialModeMigration() common.Migration {
	return common.Migration{
		Version: 16,
		Name:    "start-23-11-3k-commercial-mode-charity",
		Statements: []string{
			`CREATE TABLE IF NOT EXISTS billing.partner_commercial_modes(
				partner_id TEXT PRIMARY KEY,
				billing_mode TEXT NOT NULL DEFAULT 'PAID',
				charity_status TEXT NOT NULL DEFAULT 'NOT_REQUESTED',
				charity_requested_at TIMESTAMPTZ,
				charity_reviewed_at TIMESTAMPTZ,
				charity_reviewed_by TEXT NOT NULL DEFAULT '',
				reason TEXT NOT NULL DEFAULT '',
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				CHECK(billing_mode IN ('PAID','COMPLIMENTARY','CHARITY')),
				CHECK(charity_status IN ('NOT_REQUESTED','PENDING','APPROVED','REJECTED')),
				CHECK(billing_mode<>'CHARITY' OR charity_status='APPROVED')
			)`,
			`CREATE TABLE IF NOT EXISTS billing.partner_commercial_mode_history(
				id BIGSERIAL PRIMARY KEY,
				partner_id TEXT NOT NULL,
				old_billing_mode TEXT NOT NULL,
				new_billing_mode TEXT NOT NULL,
				old_charity_status TEXT NOT NULL,
				new_charity_status TEXT NOT NULL,
				actor TEXT NOT NULL DEFAULT '',
				reason TEXT NOT NULL DEFAULT '',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE INDEX IF NOT EXISTS billing_commercial_mode_history_partner_idx
				ON billing.partner_commercial_mode_history(partner_id,created_at DESC,id DESC)`,
			`CREATE TABLE IF NOT EXISTS billing.partner_charity_module_selections(
				id BIGSERIAL PRIMARY KEY,
				partner_id TEXT NOT NULL,
				module_key TEXT NOT NULL,
				effective_from DATE NOT NULL,
				effective_to DATE,
				source TEXT NOT NULL DEFAULT 'PARTNER_SELECTION',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				UNIQUE(partner_id,module_key,effective_from)
			)`,
			`CREATE INDEX IF NOT EXISTS billing_charity_module_selection_idx
				ON billing.partner_charity_module_selections(partner_id,effective_from,effective_to,module_key)`,
			`ALTER TABLE billing.partner_terms ALTER COLUMN minimum_monthly_commitment SET DEFAULT 0`,
			`ALTER TABLE billing.partner_terms ALTER COLUMN annual_increase_percent SET DEFAULT 5`,
			`UPDATE billing.partner_terms SET annual_increase_percent=5`,
			`ALTER TABLE billing.subscription_plans ADD COLUMN IF NOT EXISTS annual_increase_percent NUMERIC(6,2) NOT NULL DEFAULT 5`,
			`UPDATE billing.subscription_plans SET annual_increase_percent=5 WHERE plan_key IN ('STARTER','BUSINESS','FLEX')`,
			`CREATE TABLE IF NOT EXISTS billing.subscription_plan_price_history(
				id BIGSERIAL PRIMARY KEY,
				plan_key TEXT NOT NULL REFERENCES billing.subscription_plans(plan_key),
				currency TEXT NOT NULL DEFAULT 'USD',
				monthly_price NUMERIC(12,2) NOT NULL,
				annual_list_price NUMERIC(12,2) NOT NULL,
				annual_price NUMERIC(12,2) NOT NULL,
				effective_from DATE NOT NULL,
				change_type TEXT NOT NULL,
				annual_increase_percent NUMERIC(6,2) NOT NULL DEFAULT 5,
				actor TEXT NOT NULL DEFAULT '',
				reason TEXT NOT NULL DEFAULT '',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				CHECK(monthly_price>=0 AND annual_list_price>=0 AND annual_price>=0),
				CHECK(annual_increase_percent>=0)
			)`,
			`CREATE INDEX IF NOT EXISTS billing_plan_price_history_lookup
				ON billing.subscription_plan_price_history(plan_key,effective_from DESC,id DESC)`,
			`INSERT INTO billing.subscription_plan_price_history(
				plan_key,currency,monthly_price,annual_list_price,annual_price,effective_from,change_type,annual_increase_percent,actor,reason)
				SELECT p.plan_key,p.currency,p.monthly_price,p.annual_list_price,p.annual_price,CURRENT_DATE,'BASELINE',
					p.annual_increase_percent,'migration','START-23.11.3k package pricing baseline'
				FROM billing.subscription_plans p
				WHERE NOT EXISTS(SELECT 1 FROM billing.subscription_plan_price_history h WHERE h.plan_key=p.plan_key)`,
		},
	}
}

func normalizeBillingMode(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	switch value {
	case billingModePaid, billingModeComplimentary, billingModeCharity:
		return value
	default:
		return ""
	}
}

func normalizeCharityStatus(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	switch value {
	case charityNotRequested, charityPending, charityApproved, charityRejected:
		return value
	default:
		return ""
	}
}

func (a *app) ensureCommercialMode(ctx context.Context, partnerID string) (partnerCommercialMode, error) {
	if _, err := a.db.ExecContext(ctx, `INSERT INTO billing.partner_commercial_modes(partner_id)
		VALUES($1) ON CONFLICT(partner_id) DO NOTHING`, partnerID); err != nil {
		return partnerCommercialMode{}, err
	}
	var state partnerCommercialMode
	err := a.db.QueryRowContext(ctx, `SELECT partner_id,billing_mode,charity_status,charity_requested_at,
		charity_reviewed_at,charity_reviewed_by,reason,updated_at
		FROM billing.partner_commercial_modes WHERE partner_id=$1`, partnerID).
		Scan(&state.PartnerID,&state.BillingMode,&state.CharityStatus,&state.CharityRequested,
			&state.CharityReviewed,&state.CharityReviewedBy,&state.Reason,&state.UpdatedAt)
	return state, err
}

func commercialModeMap(state partnerCommercialMode) map[string]any {
	var requestedAt, reviewedAt any
	if state.CharityRequested.Valid { requestedAt = state.CharityRequested.Time.UTC() }
	if state.CharityReviewed.Valid { reviewedAt = state.CharityReviewed.Time.UTC() }
	return map[string]any{
		"partner_id": state.PartnerID,
		"billing_mode": state.BillingMode,
		"charity_status": state.CharityStatus,
		"charity_requested_at": requestedAt,
		"charity_reviewed_at": reviewedAt,
		"charity_reviewed_by": state.CharityReviewedBy,
		"reason": state.Reason,
		"recurring_charge_enabled": state.BillingMode == billingModePaid,
		"invoice_generation_enabled": state.BillingMode == billingModePaid,
		"charity_module_selection_enabled": state.BillingMode == billingModeCharity && state.CharityStatus == charityApproved,
		"updated_at": state.UpdatedAt,
	}
}

func (a *app) commercialMode(w http.ResponseWriter, r *http.Request, partnerID string) {
	switch r.Method {
	case http.MethodGet:
		state, err := a.ensureCommercialMode(r.Context(), partnerID)
		if err != nil { common.APIError(w,500,"DB","Could not load partner commercial mode"); return }
		common.JSON(w,200,commercialModeMap(state))
	case http.MethodPatch:
		current, err := a.ensureCommercialMode(r.Context(), partnerID)
		if err != nil { common.APIError(w,500,"DB","Could not load partner commercial mode"); return }
		var in struct {
			BillingMode   *string `json:"billing_mode"`
			CharityStatus *string `json:"charity_status"`
			Reason        string  `json:"reason"`
		}
		if common.Decode(r,&in)!=nil { common.APIError(w,400,"JSON","Invalid request"); return }
		nextMode := current.BillingMode
		nextCharity := current.CharityStatus
		if in.BillingMode != nil {
			nextMode = normalizeBillingMode(*in.BillingMode)
			if nextMode == "" { common.APIError(w,400,"VALIDATION","billing_mode must be PAID, COMPLIMENTARY or CHARITY"); return }
		}
		if in.CharityStatus != nil {
			nextCharity = normalizeCharityStatus(*in.CharityStatus)
			if nextCharity == "" { common.APIError(w,400,"VALIDATION","Invalid charity_status"); return }
		}
		if nextCharity == charityApproved {
			nextMode = billingModeCharity
		}
		if nextCharity == charityRejected && current.BillingMode == billingModeCharity && (in.BillingMode == nil || nextMode == billingModeCharity) {
			nextMode = billingModePaid
		}
		if nextMode == billingModeCharity && nextCharity != charityApproved {
			common.APIError(w,409,"CHARITY_APPROVAL_REQUIRED","CHARITY billing mode requires explicit HIMATE approval")
			return
		}
		reason := strings.TrimSpace(in.Reason)
		if reason == "" {
			reason = "HIMATE administrator commercial-mode update"
		}
		actor := strings.TrimSpace(r.Header.Get("X-Himate-User-ID"))
		if actor == "" { actor = "himate-admin" }
		if current.BillingMode == nextMode && current.CharityStatus == nextCharity {
			common.JSON(w,200,commercialModeMap(current))
			return
		}
		tx, err := a.db.BeginTx(r.Context(),&sql.TxOptions{})
		if err != nil { common.APIError(w,500,"DB","Could not start commercial-mode update"); return }
		defer tx.Rollback()
		requestedAtExpr := "charity_requested_at"
		reviewedAtExpr := "charity_reviewed_at"
		reviewedBy := current.CharityReviewedBy
		if nextCharity == charityPending && current.CharityStatus != charityPending {
			requestedAtExpr = "NOW()"
		}
		if (nextCharity == charityApproved || nextCharity == charityRejected) && current.CharityStatus != nextCharity {
			reviewedAtExpr = "NOW()"
			reviewedBy = actor
		}
		query := fmt.Sprintf(`UPDATE billing.partner_commercial_modes SET
			billing_mode=$2,charity_status=$3,charity_requested_at=%s,charity_reviewed_at=%s,
			charity_reviewed_by=$4,reason=$5,updated_at=NOW()
			WHERE partner_id=$1`, requestedAtExpr, reviewedAtExpr)
		if _, err = tx.ExecContext(r.Context(),query,partnerID,nextMode,nextCharity,reviewedBy,reason); err != nil {
			common.APIError(w,500,"DB","Could not update partner commercial mode"); return
		}
		if _, err = tx.ExecContext(r.Context(),`INSERT INTO billing.partner_commercial_mode_history(
			partner_id,old_billing_mode,new_billing_mode,old_charity_status,new_charity_status,actor,reason)
			VALUES($1,$2,$3,$4,$5,$6,$7)`,
			partnerID,current.BillingMode,nextMode,current.CharityStatus,nextCharity,actor,reason); err != nil {
			common.APIError(w,500,"DB","Could not save commercial-mode history"); return
		}
		eventType := "BILLING_MODE_UPDATED"
		if nextCharity == charityPending && current.CharityStatus != charityPending { eventType = "CHARITY_REQUESTED" }
		if nextCharity == charityApproved && current.CharityStatus != charityApproved { eventType = "CHARITY_APPROVED" }
		if nextCharity == charityRejected && current.CharityStatus != charityRejected { eventType = "CHARITY_REJECTED" }
		eventAt := time.Now().UTC()
		if err = emitBillingEventTx(r.Context(),tx,
			fmt.Sprintf("%s:%s:%d",eventType,partnerID,eventAt.UnixNano()),
			partnerID,"",eventType,eventAt,map[string]any{
				"old_billing_mode":current.BillingMode,"billing_mode":nextMode,
				"old_charity_status":current.CharityStatus,"charity_status":nextCharity,
				"actor":actor,"reason":reason,
			}); err != nil {
			common.APIError(w,500,"DB","Could not record commercial-mode event"); return
		}
		if err = tx.Commit(); err != nil { common.APIError(w,500,"DB","Could not commit commercial-mode update"); return }

		if nextMode == billingModeCharity && nextCharity == charityApproved {
			if err = a.syncCharityEntitlements(r.Context(),partnerID); err != nil {
				state,_ := a.ensureCommercialMode(r.Context(),partnerID)
				out := commercialModeMap(state)
				out["entitlement_sync_pending"] = true
				out["warning"] = "Charity approval saved; module entitlement synchronization will retry after module selection."
				common.JSON(w,202,out)
				return
			}
		} else if current.BillingMode == billingModeCharity && nextMode != billingModeCharity {
			if plan, loadErr := a.loadPartnerPlan(r.Context(),partnerID); loadErr == nil {
				_ = a.syncPlanEntitlements(r.Context(),partnerID,plan.PlanKey,time.Now().UTC())
			} else if loadErr == sql.ErrNoRows {
				_ = a.syncCatalogEntitlementSet(r.Context(),partnerID,"NO_PLAN",[]string{},"Charity mode ended without an active plan")
			}
		}
		state,_ := a.ensureCommercialMode(r.Context(),partnerID)
		common.JSON(w,200,commercialModeMap(state))
	default:
		common.APIError(w,405,"METHOD","Use GET or PATCH")
	}
}

func (a *app) requestCharity(w http.ResponseWriter, r *http.Request, partnerID string) {
	if r.Method != http.MethodPost { common.APIError(w,405,"METHOD","Use POST"); return }
	current, err := a.ensureCommercialMode(r.Context(),partnerID)
	if err != nil { common.APIError(w,500,"DB","Could not load charity status"); return }
	if current.CharityStatus == charityApproved {
		common.JSON(w,200,commercialModeMap(current))
		return
	}
	var in struct { Reason string `json:"reason"` }
	if common.Decode(r,&in)!=nil { common.APIError(w,400,"JSON","Invalid request"); return }
	reason := strings.TrimSpace(in.Reason)
	if reason == "" { reason = "Partner requested charity eligibility review" }
	actor := strings.TrimSpace(r.Header.Get("X-Himate-User-ID"))
	if actor == "" { actor = "partner" }
	tx, err := a.db.BeginTx(r.Context(),&sql.TxOptions{})
	if err != nil { common.APIError(w,500,"DB","Could not start charity request"); return }
	defer tx.Rollback()
	if _,err=tx.ExecContext(r.Context(),`UPDATE billing.partner_commercial_modes SET
		charity_status='PENDING',charity_requested_at=NOW(),charity_reviewed_at=NULL,charity_reviewed_by='',
		reason=$2,updated_at=NOW() WHERE partner_id=$1`,partnerID,reason);err!=nil{
		common.APIError(w,500,"DB","Could not save charity request");return
	}
	if _,err=tx.ExecContext(r.Context(),`INSERT INTO billing.partner_commercial_mode_history(
		partner_id,old_billing_mode,new_billing_mode,old_charity_status,new_charity_status,actor,reason)
		VALUES($1,$2,$2,$3,'PENDING',$4,$5)`,partnerID,current.BillingMode,current.CharityStatus,actor,reason);err!=nil{
		common.APIError(w,500,"DB","Could not save charity request history");return
	}
	eventAt:=time.Now().UTC()
	if err=emitBillingEventTx(r.Context(),tx,
		fmt.Sprintf("CHARITY_REQUESTED:%s:%d",partnerID,eventAt.UnixNano()),
		partnerID,"","CHARITY_REQUESTED",eventAt,map[string]any{"actor":actor,"reason":reason});err!=nil{
		common.APIError(w,500,"DB","Could not record charity request event");return
	}
	if err=tx.Commit();err!=nil{common.APIError(w,500,"DB","Could not commit charity request");return}
	state,_:=a.ensureCommercialMode(r.Context(),partnerID)
	common.JSON(w,202,commercialModeMap(state))
}

func (a *app) charityModulesAt(ctx context.Context, partnerID string, at time.Time) ([]string,error) {
	rows,err:=a.db.QueryContext(ctx,`SELECT module_key FROM billing.partner_charity_module_selections
		WHERE partner_id=$1 AND effective_from<=$2 AND (effective_to IS NULL OR effective_to>$2)
		ORDER BY module_key`,partnerID,dateOnly(at))
	if err!=nil{return nil,err}
	defer rows.Close()
	keys:=[]string{}
	for rows.Next(){var key string;if err:=rows.Scan(&key);err!=nil{return nil,err};keys=append(keys,key)}
	return keys,rows.Err()
}

func (a *app) replaceCharitySelection(ctx context.Context, partnerID string, keys []string, actor, reason string) error {
	keys,err:=uniqueModuleKeys(keys);if err!=nil{return err}
	if err=a.validatePublishedModuleKeys(ctx,keys);err!=nil{return err}
	effective:=dateOnly(time.Now().UTC())
	tx,err:=a.db.BeginTx(ctx,&sql.TxOptions{});if err!=nil{return err};defer tx.Rollback()
	if _,err=tx.ExecContext(ctx,`UPDATE billing.partner_charity_module_selections SET effective_to=$2
		WHERE partner_id=$1 AND effective_from<$2 AND (effective_to IS NULL OR effective_to>$2)`,partnerID,effective);err!=nil{return err}
	if _,err=tx.ExecContext(ctx,`DELETE FROM billing.partner_charity_module_selections WHERE partner_id=$1 AND effective_from=$2`,partnerID,effective);err!=nil{return err}
	for _,key:=range keys{
		if _,err=tx.ExecContext(ctx,`INSERT INTO billing.partner_charity_module_selections(partner_id,module_key,effective_from,source)
			VALUES($1,$2,$3,$4)`,partnerID,key,effective,"CHARITY_SELECTION");err!=nil{return err}
	}
	eventAt:=time.Now().UTC()
	if err=emitBillingEventTx(ctx,tx,
		fmt.Sprintf("CHARITY_MODULES_UPDATED:%s:%d",partnerID,eventAt.UnixNano()),
		partnerID,"","CHARITY_MODULES_UPDATED",eventAt,map[string]any{
			"module_keys":keys,"count":len(keys),"actor":actor,"reason":reason,
		});err!=nil{return err}
	return tx.Commit()
}

func (a *app) syncCatalogEntitlementSet(ctx context.Context, partnerID, planKey string, keys []string, reason string) error {
	payload:=map[string]any{"plan_key":planKey,"module_keys":keys,"reason":reason}
	raw,_:=json.Marshal(payload)
	req,err:=http.NewRequestWithContext(ctx,http.MethodPut,
		"http://"+a.catalogHost+"/internal/v1/partners/"+partnerID+"/plan-entitlements",bytes.NewReader(raw))
	if err!=nil{return err}
	req.Header.Set("Content-Type","application/json")
	common.BindInternalRequest(req,a.token)
	req.Header.Set("X-Himate-User-ID","billing-commercial-mode")
	resp,err:=common.DoInternal(a.client,req);if err!=nil{return err}
	defer resp.Body.Close()
	if resp.StatusCode>=300{return fmt.Errorf("catalog entitlement sync returned %d",resp.StatusCode)}
	return nil
}

func (a *app) syncCharityEntitlements(ctx context.Context, partnerID string) error {
	state,err:=a.ensureCommercialMode(ctx,partnerID);if err!=nil{return err}
	if state.BillingMode!=billingModeCharity||state.CharityStatus!=charityApproved{return nil}
	keys,err:=a.charityModulesAt(ctx,partnerID,time.Now().UTC());if err!=nil{return err}
	return a.syncCatalogEntitlementSet(ctx,partnerID,"CHARITY",keys,"Approved Charity module entitlement synchronization")
}

func (a *app) charityModules(w http.ResponseWriter,r *http.Request,partnerID string) {
	state,err:=a.ensureCommercialMode(r.Context(),partnerID)
	if err!=nil{common.APIError(w,500,"DB","Could not load charity status");return}
	if r.Method==http.MethodGet{
		keys,err:=a.charityModulesAt(r.Context(),partnerID,time.Now().UTC())
		if err!=nil{common.APIError(w,500,"DB","Could not load Charity module selection");return}
		common.JSON(w,200,map[string]any{
			"partner_id":partnerID,"billing_mode":state.BillingMode,"charity_status":state.CharityStatus,
			"approved":state.BillingMode==billingModeCharity&&state.CharityStatus==charityApproved,
			"module_keys":keys,"count":len(keys),"module_limit":nil,
		})
		return
	}
	if r.Method!=http.MethodPut{common.APIError(w,405,"METHOD","Use GET or PUT");return}
	if state.BillingMode!=billingModeCharity||state.CharityStatus!=charityApproved{
		common.APIError(w,409,"CHARITY_APPROVAL_REQUIRED","Charity modules can be selected only after HIMATE approval")
		return
	}
	var in struct{
		ModuleKeys []string `json:"module_keys"`
		Reason string `json:"reason"`
	}
	if common.Decode(r,&in)!=nil{common.APIError(w,400,"JSON","Invalid request");return}
	keys,err:=uniqueModuleKeys(in.ModuleKeys);if err!=nil{common.APIError(w,400,"VALIDATION",err.Error());return}
	actor:=strings.TrimSpace(r.Header.Get("X-Himate-User-ID"));if actor==""{actor="partner"}
	reason:=strings.TrimSpace(in.Reason);if reason==""{reason="Approved Charity module selection"}
	if err=a.replaceCharitySelection(r.Context(),partnerID,keys,actor,reason);err!=nil{
		common.APIError(w,409,"CHARITY_MODULE_SELECTION",err.Error());return
	}
	if err=a.syncCharityEntitlements(r.Context(),partnerID);err!=nil{
		common.APIError(w,502,"ENTITLEMENT_SYNC","Charity module selection saved but entitlement synchronization failed");return
	}
	common.JSON(w,200,map[string]any{
		"partner_id":partnerID,"billing_mode":billingModeCharity,"charity_status":charityApproved,
		"approved":true,"module_keys":keys,"count":len(keys),"module_limit":nil,
	})
}
