package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"himate.local/services/internal/common"
)

func phase2DurabilityMigration() common.Migration {
	return common.Migration{
		Version: 14,
		Name:    "start-23-12-phase2-durability",
		Statements: []string{
			`CREATE TABLE IF NOT EXISTS identity.audit_outbox(
				id BIGSERIAL PRIMARY KEY,
				actor_id TEXT NOT NULL DEFAULT '',
				actor_name TEXT NOT NULL DEFAULT '',
				actor_roles JSONB NOT NULL DEFAULT '[]'::jsonb,
				request_id TEXT NOT NULL DEFAULT '',
				correlation_id TEXT NOT NULL DEFAULT '',
				action TEXT NOT NULL DEFAULT '',
				method TEXT NOT NULL DEFAULT '',
				path TEXT NOT NULL DEFAULT '',
				resource TEXT NOT NULL DEFAULT '',
				partner_id TEXT NOT NULL DEFAULT '',
				old_state JSONB NOT NULL DEFAULT '{}'::jsonb,
				request_state JSONB NOT NULL DEFAULT '{}'::jsonb,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE INDEX IF NOT EXISTS identity_audit_outbox_created_idx ON identity.audit_outbox(created_at,id)`,
			`CREATE INDEX IF NOT EXISTS identity_audit_outbox_request_idx ON identity.audit_outbox(request_id) WHERE request_id<>''`,
			`CREATE TABLE IF NOT EXISTS identity.partner_onboarding_sagas(
				request_id TEXT PRIMARY KEY,
				actor_id TEXT NOT NULL,
				partner_id TEXT NOT NULL DEFAULT '',
				status TEXT NOT NULL DEFAULT 'PENDING',
				partner_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
				portal_owner_name TEXT NOT NULL DEFAULT '',
				portal_owner_email TEXT NOT NULL DEFAULT '',
				portal_owner_password_hash TEXT NOT NULL,
				billing_terms JSONB NOT NULL DEFAULT '{}'::jsonb,
				partner_done BOOLEAN NOT NULL DEFAULT FALSE,
				owner_done BOOLEAN NOT NULL DEFAULT FALSE,
				billing_done BOOLEAN NOT NULL DEFAULT FALSE,
				last_error TEXT NOT NULL DEFAULT '',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				completed_at TIMESTAMPTZ
			)`,
			`CREATE INDEX IF NOT EXISTS identity_partner_onboarding_status_idx ON identity.partner_onboarding_sagas(status,updated_at DESC)`,
			`CREATE INDEX IF NOT EXISTS identity_partner_onboarding_actor_idx ON identity.partner_onboarding_sagas(actor_id,updated_at DESC)`,
		},
	}
}

func jsonForAudit(value any) string {
	raw, err := json.Marshal(sanitizeAuditValue(value))
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func (a *app) createAuditIntent(ctx context.Context, event auditEvent, requestState any) (int64, error) {
	roles, _ := json.Marshal(event.ActorRoles)
	var id int64
	err := a.db.QueryRowContext(ctx, `INSERT INTO identity.audit_outbox(
		actor_id,actor_name,actor_roles,request_id,correlation_id,action,method,path,resource,partner_id,old_state,request_state,created_at
	) VALUES($1,$2,$3::jsonb,$4,$5,$6,$7,$8,$9,$10,$11::jsonb,$12::jsonb,$13) RETURNING id`,
		event.ActorID,event.ActorName,string(roles),event.RequestID,event.CorrelationID,event.Action,event.Method,event.Path,
		event.Resource,event.PartnerID,jsonForAudit(event.OldState),jsonForAudit(requestState),event.CreatedAt,
	).Scan(&id)
	return id, err
}

func (a *app) finalizeAuditIntent(ctx context.Context, intentID int64, event auditEvent) error {
	if intentID == 0 {
		return fmt.Errorf("audit intent id is required")
	}
	tx, err := a.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	roles, _ := json.Marshal(event.ActorRoles)
	oldState, _ := json.Marshal(sanitizeAuditValue(event.OldState))
	newState, _ := json.Marshal(sanitizeAuditValue(event.NewState))
	if _, err = tx.ExecContext(ctx, `INSERT INTO identity.audit_events(
		actor_id,actor_name,actor_roles,request_id,correlation_id,action,method,path,resource,partner_id,status,outcome,old_state,new_state,duration_ms,created_at
	) VALUES($1,$2,$3::jsonb,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13::jsonb,$14::jsonb,$15,$16)`,
		event.ActorID,event.ActorName,string(roles),event.RequestID,event.CorrelationID,event.Action,event.Method,event.Path,event.Resource,
		event.PartnerID,event.Status,event.Outcome,string(oldState),string(newState),event.DurationMS,event.CreatedAt,
	); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM identity.audit_outbox WHERE id=$1`, intentID); err != nil {
		return err
	}
	return tx.Commit()
}

func (a *app) recoverAuditOutbox(ctx context.Context) error {
	rows, err := a.db.QueryContext(ctx, `SELECT id,actor_id,actor_name,actor_roles,request_id,correlation_id,action,method,path,resource,partner_id,old_state,request_state,created_at
		FROM identity.audit_outbox ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type pending struct {
		id int64
		event auditEvent
		requestState any
	}
	items := []pending{}
	for rows.Next() {
		var x pending
		var rolesRaw, oldRaw, requestRaw []byte
		if err := rows.Scan(&x.id,&x.event.ActorID,&x.event.ActorName,&rolesRaw,&x.event.RequestID,&x.event.CorrelationID,
			&x.event.Action,&x.event.Method,&x.event.Path,&x.event.Resource,&x.event.PartnerID,&oldRaw,&requestRaw,&x.event.CreatedAt); err != nil {
			return err
		}
		_ = json.Unmarshal(rolesRaw,&x.event.ActorRoles)
		_ = json.Unmarshal(oldRaw,&x.event.OldState)
		_ = json.Unmarshal(requestRaw,&x.requestState)
		items = append(items,x)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, x := range items {
		x.event.Action += "_INTERRUPTED"
		x.event.Status = 0
		x.event.Outcome = "INTERRUPTED"
		x.event.NewState = map[string]any{
			"recovery": "gateway restarted before the mutation result was durably audited",
			"request_state": sanitizeAuditValue(x.requestState),
		}
		if err := a.finalizeAuditIntent(ctx,x.id,x.event); err != nil {
			return err
		}
	}
	return nil
}

type partnerOnboardingSaga struct {
	RequestID string
	ActorID string
	PartnerID string
	Status string
	PartnerPayload map[string]any
	OwnerName string
	OwnerEmail string
	OwnerPasswordHash string
	BillingTerms map[string]any
	PartnerDone bool
	OwnerDone bool
	BillingDone bool
	LastError string
	CreatedAt time.Time
	UpdatedAt time.Time
	CompletedAt sql.NullTime
}

func scanPartnerOnboardingSaga(row *sql.Row) (partnerOnboardingSaga,error) {
	var s partnerOnboardingSaga
	var partnerRaw,billingRaw []byte
	err:=row.Scan(&s.RequestID,&s.ActorID,&s.PartnerID,&s.Status,&partnerRaw,&s.OwnerName,&s.OwnerEmail,&s.OwnerPasswordHash,
		&billingRaw,&s.PartnerDone,&s.OwnerDone,&s.BillingDone,&s.LastError,&s.CreatedAt,&s.UpdatedAt,&s.CompletedAt)
	_ = json.Unmarshal(partnerRaw,&s.PartnerPayload)
	_ = json.Unmarshal(billingRaw,&s.BillingTerms)
	return s,err
}

func (a *app) loadPartnerOnboardingSaga(ctx context.Context,requestID string)(partnerOnboardingSaga,error){
	return scanPartnerOnboardingSaga(a.db.QueryRowContext(ctx,`SELECT request_id,actor_id,partner_id,status,partner_payload,portal_owner_name,
		portal_owner_email,portal_owner_password_hash,billing_terms,partner_done,owner_done,billing_done,last_error,created_at,updated_at,completed_at
		FROM identity.partner_onboarding_sagas WHERE request_id=$1`,requestID))
}

func onboardingSagaMap(s partnerOnboardingSaga, partner map[string]any) map[string]any {
	out:=map[string]any{
		"request_id":s.RequestID,"partner_id":s.PartnerID,"status":s.Status,
		"partner_done":s.PartnerDone,"owner_done":s.OwnerDone,"billing_done":s.BillingDone,
		"last_error":s.LastError,"created_at":s.CreatedAt.UTC(),"updated_at":s.UpdatedAt.UTC(),
	}
	if s.CompletedAt.Valid { out["completed_at"]=s.CompletedAt.Time.UTC() }
	if partner!=nil { out["partner"]=partner }
	return out
}

func (a *app) ensureOnboardingOwner(ctx context.Context,s partnerOnboardingSaga) error {
	var existingID,existingPartner,existingRole string
	var existingActive bool
	err:=a.db.QueryRowContext(ctx,`SELECT id,partner_id,role_key,active FROM identity.partner_users WHERE lower(email)=lower($1)`,s.OwnerEmail).
		Scan(&existingID,&existingPartner,&existingRole,&existingActive)
	if err==nil {
		if existingPartner==s.PartnerID && existingRole=="owner" && existingActive { return nil }
		return fmt.Errorf("Partner Portal owner email is already assigned to another identity")
	}
	if err!=sql.ErrNoRows { return err }
	var adminCollision bool
	if err:=a.db.QueryRowContext(ctx,`SELECT EXISTS(SELECT 1 FROM identity.users WHERE lower(email)=lower($1))`,s.OwnerEmail).Scan(&adminCollision);err!=nil{return err}
	if adminCollision{return fmt.Errorf("Partner Portal owner email is already assigned to a HIMATE administrator")}
	id,err:=partnerUserID();if err!=nil{return err}
	_,err=a.db.ExecContext(ctx,`INSERT INTO identity.partner_users(id,partner_id,name,email,password_hash,role_key)
		VALUES($1,$2,$3,$4,$5,'owner')`,id,s.PartnerID,s.OwnerName,s.OwnerEmail,s.OwnerPasswordHash)
	return err
}

func onboardingTermsMatch(current, desired map[string]any) bool {
	keys:=[]string{
		"currency","activation_fee","activation_fee_waived","activation_fee_reason",
		"base_monthly_fee","minimum_monthly_commitment","quote_reference",
		"annual_increase_percent","price_effective_from","service_anchor_date",
	}
	for _,key:=range keys{
		want,ok:=desired[key]
		if !ok{continue}
		got,exists:=current[key]
		if !exists{return false}
		if fmt.Sprint(got)!=fmt.Sprint(want){return false}
	}
	return true
}

func (a *app) runPartnerOnboardingSaga(ctx context.Context,requestID string,actor user)(partnerOnboardingSaga,map[string]any,error){
	s,err:=a.loadPartnerOnboardingSaga(ctx,requestID)
	if err!=nil{return s,nil,err}
	if s.ActorID!=actor.ID && !actor.SystemOwner{return s,nil,fmt.Errorf("onboarding request belongs to another administrator")}

	var partner map[string]any
	fail:=func(stepErr error)(partnerOnboardingSaga,map[string]any,error){
		_,_=a.db.ExecContext(context.Background(),`UPDATE identity.partner_onboarding_sagas SET status='FAILED',last_error=$2,updated_at=NOW() WHERE request_id=$1`,requestID,stepErr.Error())
		latest,_:=a.loadPartnerOnboardingSaga(context.Background(),requestID)
		return latest,partner,stepErr
	}

	if !s.PartnerDone {
		payload:=map[string]any{}
		for k,v:=range s.PartnerPayload{payload[k]=v}
		payload["onboarding_request_id"]=s.RequestID
		if err:=a.internalJSON(ctx,http.MethodPost,a.hosts["partners"],"/api/v1/partners",payload,map[string]string{"X-Himate-User-ID":actor.ID},&partner);err!=nil{
			return fail(fmt.Errorf("partner master record: %w",err))
		}
		s.PartnerID=strings.TrimSpace(fmt.Sprint(partner["id"]))
		if s.PartnerID==""{return fail(fmt.Errorf("partner service returned no partner id"))}
		if _,err:=a.db.ExecContext(ctx,`UPDATE identity.partner_onboarding_sagas SET partner_id=$2,partner_done=TRUE,status='RUNNING',last_error='',updated_at=NOW() WHERE request_id=$1`,s.RequestID,s.PartnerID);err!=nil{
			return fail(err)
		}
		s.PartnerDone=true
	} else {
		if err:=a.internalJSON(ctx,http.MethodGet,a.hosts["partners"],"/api/v1/partners/"+url.PathEscape(s.PartnerID),nil,nil,&partner);err!=nil{
			return fail(fmt.Errorf("partner master readback: %w",err))
		}
	}

	if !s.OwnerDone {
		if err:=a.ensureOnboardingOwner(ctx,s);err!=nil{return fail(fmt.Errorf("Partner Portal owner: %w",err))}
		if _,err:=a.db.ExecContext(ctx,`UPDATE identity.partner_onboarding_sagas SET owner_done=TRUE,status='RUNNING',last_error='',updated_at=NOW() WHERE request_id=$1`,s.RequestID);err!=nil{
			return fail(err)
		}
		s.OwnerDone=true
	}

	if !s.BillingDone {
		var currentTerms map[string]any
		termsPath:="/api/v1/billing/partners/"+url.PathEscape(s.PartnerID)+"/terms"
		if err:=a.internalJSON(ctx,http.MethodGet,a.hosts["billing"],termsPath,nil,map[string]string{"X-Himate-User-ID":actor.ID},&currentTerms);err!=nil{
			return fail(fmt.Errorf("billing terms readback: %w",err))
		}
		if !onboardingTermsMatch(currentTerms,s.BillingTerms){
			if err:=a.internalJSON(ctx,http.MethodPut,a.hosts["billing"],termsPath,
				s.BillingTerms,map[string]string{"X-Himate-User-ID":actor.ID},nil);err!=nil{
				return fail(fmt.Errorf("billing terms: %w",err))
			}
		}
		if _,err:=a.db.ExecContext(ctx,`UPDATE identity.partner_onboarding_sagas SET billing_done=TRUE,status='COMPLETE',last_error='',completed_at=NOW(),updated_at=NOW() WHERE request_id=$1`,s.RequestID);err!=nil{
			return fail(err)
		}
		s.BillingDone=true
	}
	latest,err:=a.loadPartnerOnboardingSaga(ctx,requestID)
	if err!=nil{return s,partner,err}
	return latest,partner,nil
}

func (a *app) partnerOnboarding(w http.ResponseWriter,r *http.Request,actor user){
	if !actor.SystemOwner{common.APIError(w,http.StatusForbidden,"OWNER_REQUIRED","Only the HIMATE system owner can run partner onboarding");return}
	raw:=strings.Trim(strings.TrimPrefix(r.URL.Path,"/api/v1/partner-onboarding"),"/")
	if raw!=""{
		parts:=strings.Split(raw,"/")
		requestID:=parts[0]
		if requestID==""{common.APIError(w,404,"NOT_FOUND","Onboarding request not found");return}
		if len(parts)==1&&r.Method==http.MethodGet{
			s,err:=a.loadPartnerOnboardingSaga(r.Context(),requestID);if err!=nil{common.APIError(w,404,"NOT_FOUND","Onboarding request not found");return}
			if s.ActorID!=actor.ID&&!actor.SystemOwner{common.APIError(w,403,"FORBIDDEN","Onboarding request belongs to another administrator");return}
			var partner map[string]any
			if s.PartnerID!=""{_ = a.internalJSON(r.Context(),http.MethodGet,a.hosts["partners"],"/api/v1/partners/"+url.PathEscape(s.PartnerID),nil,nil,&partner)}
			common.JSON(w,200,onboardingSagaMap(s,partner));return
		}
		if len(parts)==2&&parts[1]=="resume"&&r.Method==http.MethodPost{
			s,partner,err:=a.runPartnerOnboardingSaga(r.Context(),requestID,actor)
			if err!=nil{common.JSON(w,http.StatusConflict,map[string]any{"onboarding":onboardingSagaMap(s,partner),"error":map[string]any{"code":"ONBOARDING_INCOMPLETE","message":err.Error()}});return}
			common.JSON(w,200,onboardingSagaMap(s,partner));return
		}
		common.APIError(w,405,"METHOD","Use GET or POST /resume");return
	}

	if r.Method!=http.MethodPost{common.APIError(w,405,"METHOD","Use POST");return}
	var in struct{
		RequestID string `json:"request_id"`
		Partner map[string]any `json:"partner"`
		PortalOwner struct{Name string `json:"name"`;Email string `json:"email"`;Password string `json:"password"`} `json:"portal_owner"`
		BillingTerms map[string]any `json:"billing_terms"`
	}
	if common.Decode(r,&in)!=nil{common.APIError(w,400,"JSON","Invalid onboarding request");return}
	in.RequestID=strings.TrimSpace(in.RequestID)
	in.PortalOwner.Name=strings.TrimSpace(in.PortalOwner.Name)
	in.PortalOwner.Email=strings.ToLower(strings.TrimSpace(in.PortalOwner.Email))
	if in.RequestID==""||len(in.RequestID)>160{common.APIError(w,400,"VALIDATION","request_id is required and must be at most 160 characters");return}
	if strings.TrimSpace(fmt.Sprint(in.Partner["display_name"]))==""{common.APIError(w,400,"VALIDATION","Partner display name is required");return}
	if len(in.PortalOwner.Name)<2||!validEmail(in.PortalOwner.Email){common.APIError(w,400,"VALIDATION","Valid Partner Portal owner name and email are required");return}
	if message:=passwordPolicyError(in.PortalOwner.Password);message!=""{common.APIError(w,400,"VALIDATION",message);return}
	if len(in.BillingTerms)==0{common.APIError(w,400,"VALIDATION","billing_terms are required");return}

	if existing,err:=a.loadPartnerOnboardingSaga(r.Context(),in.RequestID);err==nil{
		if existing.ActorID!=actor.ID&&!actor.SystemOwner{common.APIError(w,403,"FORBIDDEN","Onboarding request belongs to another administrator");return}
		s,partner,runErr:=a.runPartnerOnboardingSaga(r.Context(),in.RequestID,actor)
		if runErr!=nil{common.JSON(w,http.StatusConflict,map[string]any{"onboarding":onboardingSagaMap(s,partner),"error":map[string]any{"code":"ONBOARDING_INCOMPLETE","message":runErr.Error()}});return}
		common.JSON(w,200,onboardingSagaMap(s,partner));return
	}

	hash,err:=hashPassword(in.PortalOwner.Password);if err!=nil{common.APIError(w,500,"PASSWORD","Could not secure Partner Portal password");return}
	partnerRaw,_:=json.Marshal(in.Partner);billingRaw,_:=json.Marshal(in.BillingTerms)
	_,err=a.db.ExecContext(r.Context(),`INSERT INTO identity.partner_onboarding_sagas(
		request_id,actor_id,status,partner_payload,portal_owner_name,portal_owner_email,portal_owner_password_hash,billing_terms
	) VALUES($1,$2,'PENDING',$3::jsonb,$4,$5,$6,$7::jsonb)`,
		in.RequestID,actor.ID,string(partnerRaw),in.PortalOwner.Name,in.PortalOwner.Email,hash,string(billingRaw))
	if err!=nil{common.APIError(w,409,"CONFLICT","Onboarding request could not be created");return}

	s,partner,runErr:=a.runPartnerOnboardingSaga(r.Context(),in.RequestID,actor)
	if runErr!=nil{common.JSON(w,http.StatusConflict,map[string]any{"onboarding":onboardingSagaMap(s,partner),"error":map[string]any{"code":"ONBOARDING_INCOMPLETE","message":runErr.Error()}});return}
	common.JSON(w,http.StatusCreated,onboardingSagaMap(s,partner))
}
