package main

import (
	"database/sql"
	"fmt"
	"himate.local/services/internal/common"
	"net/http"
	"strings"
	"time"
)

func start23112CatalogPlanMigration() common.Migration {
	return common.Migration{
		Version: 8,
		Name: "start-23-11-2-plan-entitlement-source",
		Statements: []string{
			`ALTER TABLE catalog.partner_modules ADD COLUMN IF NOT EXISTS entitlement_source TEXT NOT NULL DEFAULT 'MANUAL'`,
			`ALTER TABLE catalog.partner_modules ADD COLUMN IF NOT EXISTS plan_key TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE catalog.partner_modules ADD COLUMN IF NOT EXISTS plan_effective_at TIMESTAMPTZ`,
			`CREATE INDEX IF NOT EXISTS partner_modules_plan_idx ON catalog.partner_modules(partner_id,entitlement_source,plan_key)`,
		},
	}
}

func uniquePlanKeys(values []string) ([]string,error) {
	seen:=map[string]bool{}
	out:=[]string{}
	for _,raw:=range values{
		key:=strings.TrimSpace(raw)
		if key==""{continue}
		if seen[key]{return nil,fmt.Errorf("duplicate module %s",key)}
		seen[key]=true
		out=append(out,key)
	}
	return out,nil
}

func (a *app) applyPlanEntitlements(w http.ResponseWriter,r *http.Request,partnerID string){
	if r.Method!=http.MethodPut{common.APIError(w,405,"METHOD","Use PUT");return}
	var in struct{
		PlanKey string `json:"plan_key"`
		ModuleKeys []string `json:"module_keys"`
		Reason string `json:"reason"`
	}
	if common.Decode(r,&in)!=nil{common.APIError(w,400,"JSON","Invalid request");return}
	planKey:=strings.ToUpper(strings.TrimSpace(in.PlanKey))
	if planKey==""{common.APIError(w,400,"VALIDATION","plan_key is required");return}
	keys,err:=uniquePlanKeys(in.ModuleKeys);if err!=nil{common.APIError(w,400,"VALIDATION",err.Error());return}
	var testPartner bool
	if err:=a.db.QueryRowContext(r.Context(),`SELECT test_partner FROM partners.partners WHERE id=$1`,partnerID).Scan(&testPartner);err!=nil && err!=sql.ErrNoRows{
		common.APIError(w,500,"DB","Could not verify Golden Test Partner state");return
	}
	if testPartner{
		if err:=a.ensurePartnerModules(partnerID);err!=nil{common.APIError(w,500,"DB","Could not preserve Golden Test entitlements");return}
		rows,err:=a.db.QueryContext(r.Context(),`SELECT pm.module_key FROM catalog.partner_modules pm
			JOIN catalog.modules m ON m.module_key=pm.module_key
			WHERE pm.partner_id=$1 AND m.system=TRUE AND pm.status='ACTIVE' AND pm.entitlement_state='ACTIVE'
			ORDER BY pm.module_key`,partnerID)
		if err!=nil{common.APIError(w,500,"DB","Could not load Golden Test entitlements");return}
		defer rows.Close()
		golden:=[]string{}
		for rows.Next(){var key string;if err:=rows.Scan(&key);err!=nil{common.APIError(w,500,"DB","Could not decode Golden Test entitlement");return};golden=append(golden,key)}
		common.JSON(w,200,map[string]any{"partner_id":partnerID,"plan_key":"GOLDEN_TEST","module_keys":golden,"count":len(golden),"entitlement_source":"TEST"})
		return
	}
	target:=map[string]bool{};for _,key:=range keys{target[key]=true}
	entitlementSource:="PLAN"
	quoteReference:="PLAN:"+planKey
	if planKey=="CHARITY"{entitlementSource="CHARITY";quoteReference="CHARITY"}

	tx,err:=a.db.BeginTx(r.Context(),&sql.TxOptions{});if err!=nil{common.APIError(w,500,"DB","Could not start entitlement sync");return};defer tx.Rollback()
	for _,key:=range keys{
		var publication,implementation string
		if err=tx.QueryRowContext(r.Context(),`SELECT publication_status,implementation_state FROM catalog.modules WHERE module_key=$1`,key).Scan(&publication,&implementation);err!=nil{
			if err==sql.ErrNoRows{common.APIError(w,404,"MODULE_NOT_FOUND","Module "+key+" not found");return}
			common.APIError(w,500,"DB","Could not validate plan module");return
		}
		if publication!="PUBLISHED"||implementation!="READY"{common.APIError(w,409,"MODULE_NOT_READY","Plan modules must be PUBLISHED and READY");return}
	}
	rows,err:=tx.QueryContext(r.Context(),`SELECT module_key,status,entitlement_state,entitlement_source FROM catalog.partner_modules
		WHERE partner_id=$1`,partnerID)
	if err!=nil{common.APIError(w,500,"DB","Could not load current partner entitlements");return}
	type state struct{key,status,entitlement,source string};current:=[]state{}
	for rows.Next(){var s state;if err:=rows.Scan(&s.key,&s.status,&s.entitlement,&s.source);err!=nil{rows.Close();common.APIError(w,500,"DB","Could not decode partner entitlement");return};current=append(current,s)}
	rows.Close()
	now:=time.Now().UTC();actor:=strings.TrimSpace(r.Header.Get("X-Himate-User-ID"));if actor==""{actor="billing-plan-engine"}
	reason:=strings.TrimSpace(in.Reason);if reason==""{reason="Subscription plan entitlement synchronization"}

	for _,s:=range current{
		if target[s.key]{continue}
		if s.status!="ACTIVE" && s.entitlement!="ACTIVE" && s.source!="PLAN" && s.source!="CHARITY"{continue}
		if _,err=tx.ExecContext(r.Context(),`UPDATE catalog.partner_modules SET status='NOT_LICENSED',entitlement_state='INACTIVE',visible=FALSE,
			included_in_base=FALSE,entitlement_source=$4,plan_key='',plan_effective_at=$3,updated_at=NOW()
			WHERE partner_id=$1 AND module_key=$2`,partnerID,s.key,now,entitlementSource);err!=nil{common.APIError(w,500,"DB","Could not enforce target plan entitlement set");return}
		if _,err=tx.ExecContext(r.Context(),`INSERT INTO catalog.partner_module_history(partner_id,module_key,field_name,old_value,new_value,effective_at,actor,reason)
			VALUES($1,$2,'plan_entitlement',$3,'INACTIVE',$4,$5,$6)`,partnerID,s.key,s.entitlement,now,actor,reason);err!=nil{common.APIError(w,500,"DB","Could not record plan entitlement history");return}
	}
	for _,key:=range keys{
		var oldStatus,oldEntitlement string
		if err=tx.QueryRowContext(r.Context(),`SELECT status,entitlement_state FROM catalog.partner_modules WHERE partner_id=$1 AND module_key=$2 FOR UPDATE`,partnerID,key).
			Scan(&oldStatus,&oldEntitlement);err!=nil{common.APIError(w,500,"DB","Could not load target plan entitlement");return}
		if _,err=tx.ExecContext(r.Context(),`UPDATE catalog.partner_modules SET status='ACTIVE',entitlement_state='ACTIVE',visible=TRUE,included_in_base=TRUE,
			commercial_configured=TRUE,contract_currency='USD',quote_reference=$3,entitlement_source=$6,plan_key=$4,
			plan_effective_at=$5,activated_at=COALESCE(activated_at,$5),updated_at=NOW()
			WHERE partner_id=$1 AND module_key=$2`,partnerID,key,quoteReference,planKey,now,entitlementSource);err!=nil{common.APIError(w,500,"DB","Could not apply plan entitlement");return}
		if oldStatus!="ACTIVE"||oldEntitlement!="ACTIVE"{
			if _,err=tx.ExecContext(r.Context(),`INSERT INTO catalog.partner_module_history(partner_id,module_key,field_name,old_value,new_value,effective_at,actor,reason)
				VALUES($1,$2,'plan_entitlement',$3,'ACTIVE',$4,$5,$6)`,partnerID,key,oldEntitlement,now,actor,reason);err!=nil{common.APIError(w,500,"DB","Could not record plan entitlement history");return}
		}
	}
	if err=tx.Commit();err!=nil{common.APIError(w,500,"DB","Could not commit plan entitlements");return}
	common.JSON(w,200,map[string]any{"partner_id":partnerID,"plan_key":planKey,"module_keys":keys,"count":len(keys),"entitlement_source":entitlementSource})
}
