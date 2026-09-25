package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"himate.local/services/internal/common"
	"net/http"
	"math"
	"sort"
	"strings"
	"time"
)

type subscriptionPlan struct {
	Key                string
	Name               string
	Currency           string
	MonthlyPrice       float64
	AnnualListPrice    float64
	AnnualPrice        float64
	AnnualIncreasePercent float64
	PriceEffectiveFrom time.Time
	AnnualFreeMonths   int
	ModuleLimit        int
	SelectionMode      string
	CustomerSelectable bool
	Active             bool
	SortOrder          int
}

type partnerPlanSubscription struct {
	PartnerID               string
	PlanKey                 string
	BillingFrequency        string
	Status                  string
	CurrentPeriodStart      time.Time
	CurrentPeriodEnd        time.Time
	NextBillingAt           time.Time
	MonthlyPriceSnapshot    float64
	AnnualListPriceSnapshot float64
	AnnualPriceSnapshot     float64
	NextPlanKey             string
	NextBillingFrequency    string
	ChangeEffectiveAt       sql.NullTime
	CustomMonthlyPrice      float64
	CustomAnnualListPrice   float64
	CustomAnnualPrice       float64
	CustomModuleLimit       int
	CustomSelectionMode     string
}

func start23112PlanBillingMigration() common.Migration {
	return common.Migration{
		Version: 11,
		Name: "start-23-11-2-subscription-plan-recurring-billing",
		AllowDestructiveSchema: true,
		Statements: []string{
			`ALTER TABLE billing.partner_terms ADD COLUMN IF NOT EXISTS billing_cycle_model TEXT NOT NULL DEFAULT 'PLAN_BASED'`,
			`UPDATE billing.partner_terms SET billing_cycle_model='PLAN_BASED'`,
			`CREATE TABLE IF NOT EXISTS billing.subscription_plans(
				plan_key TEXT PRIMARY KEY,
				display_name TEXT NOT NULL,
				currency TEXT NOT NULL DEFAULT 'USD',
				monthly_price NUMERIC(12,2) NOT NULL,
				annual_list_price NUMERIC(12,2) NOT NULL,
				annual_price NUMERIC(12,2) NOT NULL,
				annual_free_months INT NOT NULL DEFAULT 0,
				module_limit INT NOT NULL,
				selection_mode TEXT NOT NULL,
				customer_selectable BOOLEAN NOT NULL DEFAULT TRUE,
				active BOOLEAN NOT NULL DEFAULT TRUE,
				sort_order INT NOT NULL DEFAULT 0,
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				CHECK(selection_mode IN ('FIXED','SELECTABLE','CUSTOM')),
				CHECK(module_limit>=0)
			)`,
			`INSERT INTO billing.subscription_plans(plan_key,display_name,currency,monthly_price,annual_list_price,annual_price,annual_free_months,module_limit,selection_mode,customer_selectable,sort_order)
				VALUES
				('STARTER','Starter','USD',500,6000,6000,0,3,'FIXED',TRUE,10),
				('BUSINESS','Business','USD',1500,18000,16500,1,10,'FIXED',TRUE,20),
				('FLEX','Flex','USD',2500,30000,22500,3,15,'SELECTABLE',TRUE,30),
				('CUSTOM','Custom','USD',0,0,0,0,0,'CUSTOM',FALSE,100)
				ON CONFLICT(plan_key) DO UPDATE SET
					display_name=EXCLUDED.display_name,currency=EXCLUDED.currency,
					monthly_price=EXCLUDED.monthly_price,annual_list_price=EXCLUDED.annual_list_price,
					annual_price=EXCLUDED.annual_price,annual_free_months=EXCLUDED.annual_free_months,
					module_limit=EXCLUDED.module_limit,selection_mode=EXCLUDED.selection_mode,
					customer_selectable=EXCLUDED.customer_selectable,sort_order=EXCLUDED.sort_order,updated_at=NOW()`,
			`CREATE TABLE IF NOT EXISTS billing.subscription_plan_modules(
				id BIGSERIAL PRIMARY KEY,
				plan_key TEXT NOT NULL REFERENCES billing.subscription_plans(plan_key),
				module_key TEXT NOT NULL,
				position INT NOT NULL DEFAULT 0,
				effective_from DATE NOT NULL,
				effective_to DATE,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				UNIQUE(plan_key,module_key,effective_from)
			)`,
			`CREATE INDEX IF NOT EXISTS billing_plan_modules_effective_idx
				ON billing.subscription_plan_modules(plan_key,effective_from,effective_to,position)`,
			`CREATE TABLE IF NOT EXISTS billing.partner_plan_subscriptions(
				partner_id TEXT PRIMARY KEY,
				plan_key TEXT NOT NULL REFERENCES billing.subscription_plans(plan_key),
				billing_frequency TEXT NOT NULL DEFAULT 'MONTHLY',
				status TEXT NOT NULL DEFAULT 'ACTIVE',
				current_period_start DATE NOT NULL,
				current_period_end DATE NOT NULL,
				next_billing_at DATE NOT NULL,
				monthly_price_snapshot NUMERIC(12,2) NOT NULL DEFAULT 0,
				annual_list_price_snapshot NUMERIC(12,2) NOT NULL DEFAULT 0,
				annual_price_snapshot NUMERIC(12,2) NOT NULL DEFAULT 0,
				next_plan_key TEXT NOT NULL DEFAULT '',
				next_billing_frequency TEXT NOT NULL DEFAULT '',
				change_effective_at DATE,
				custom_monthly_price NUMERIC(12,2) NOT NULL DEFAULT 0,
				custom_annual_list_price NUMERIC(12,2) NOT NULL DEFAULT 0,
				custom_annual_price NUMERIC(12,2) NOT NULL DEFAULT 0,
				custom_module_limit INT NOT NULL DEFAULT 0,
				custom_selection_mode TEXT NOT NULL DEFAULT 'CUSTOM',
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				CHECK(billing_frequency IN ('MONTHLY','ANNUAL')),
				CHECK(status IN ('ACTIVE','PAST_DUE','SUSPENDED','CANCELLED'))
			)`,
			`CREATE INDEX IF NOT EXISTS billing_partner_plan_due_idx
				ON billing.partner_plan_subscriptions(status,next_billing_at,partner_id)`,
			`CREATE TABLE IF NOT EXISTS billing.partner_plan_module_selections(
				id BIGSERIAL PRIMARY KEY,
				partner_id TEXT NOT NULL,
				module_key TEXT NOT NULL,
				effective_from DATE NOT NULL,
				effective_to DATE,
				source TEXT NOT NULL DEFAULT 'PARTNER_SELECTION',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				UNIQUE(partner_id,module_key,effective_from)
			)`,
			`CREATE INDEX IF NOT EXISTS billing_partner_plan_selection_effective_idx
				ON billing.partner_plan_module_selections(partner_id,effective_from,effective_to)`,
			`CREATE TABLE IF NOT EXISTS billing.plan_change_history(
				id BIGSERIAL PRIMARY KEY,
				partner_id TEXT NOT NULL,
				old_plan_key TEXT NOT NULL DEFAULT '',
				new_plan_key TEXT NOT NULL,
				old_billing_frequency TEXT NOT NULL DEFAULT '',
				new_billing_frequency TEXT NOT NULL,
				change_type TEXT NOT NULL,
				effective_at DATE NOT NULL,
				upgrade_charge NUMERIC(12,2) NOT NULL DEFAULT 0,
				actor TEXT NOT NULL DEFAULT '',
				reason TEXT NOT NULL DEFAULT '',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE INDEX IF NOT EXISTS billing_plan_change_partner_idx
				ON billing.plan_change_history(partner_id,created_at DESC,id DESC)`,
			`ALTER TABLE billing.invoices ADD COLUMN IF NOT EXISTS invoice_key TEXT`,
			`ALTER TABLE billing.invoices ADD COLUMN IF NOT EXISTS plan_key TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE billing.invoices ADD COLUMN IF NOT EXISTS billing_frequency TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE billing.invoices ADD COLUMN IF NOT EXISTS charge_type TEXT NOT NULL DEFAULT 'LEGACY'`,
			`ALTER TABLE billing.invoices ADD COLUMN IF NOT EXISTS list_price NUMERIC(12,2) NOT NULL DEFAULT 0`,
			`ALTER TABLE billing.invoices ADD COLUMN IF NOT EXISTS discount_amount NUMERIC(12,2) NOT NULL DEFAULT 0`,
			`ALTER TABLE billing.invoices ADD COLUMN IF NOT EXISTS minimum_commitment_adjustment NUMERIC(12,2) NOT NULL DEFAULT 0`,
			`ALTER TABLE billing.invoices ADD COLUMN IF NOT EXISTS billing_model TEXT NOT NULL DEFAULT 'LEGACY_MODULE'`,
			`ALTER TABLE billing.invoice_items ADD COLUMN IF NOT EXISTS billing_model TEXT NOT NULL DEFAULT 'LEGACY_MODULE'`,
			`ALTER TABLE billing.invoices ALTER COLUMN billing_model SET DEFAULT 'LEGACY_MODULE'`,
			`ALTER TABLE billing.invoice_items ALTER COLUMN billing_model SET DEFAULT 'LEGACY_MODULE'`,
			`ALTER TABLE billing.invoices DROP CONSTRAINT IF EXISTS billing_invoices_partner_id_invoice_date_key`,
			`ALTER TABLE billing.invoices DROP CONSTRAINT IF EXISTS invoices_partner_id_invoice_date_key`,
			`DROP INDEX IF EXISTS billing.billing_invoice_period_unique`,
			`DROP INDEX IF EXISTS billing.billing_invoice_date_model_unique`,
			`DROP INDEX IF EXISTS billing.billing_invoice_period_model_unique`,
			`CREATE UNIQUE INDEX IF NOT EXISTS billing_invoice_key_unique ON billing.invoices(invoice_key)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS billing_invoice_period_model_unique
				ON billing.invoices(partner_id,service_period_start,service_period_end,billing_model,charge_type,plan_key)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS billing_invoice_legacy_period_unique
				ON billing.invoices(partner_id,service_period_start,service_period_end) WHERE billing_model='LEGACY_MODULE'`,
			`INSERT INTO billing.partner_plan_subscriptions(
				partner_id,plan_key,billing_frequency,status,current_period_start,current_period_end,next_billing_at,
				monthly_price_snapshot,annual_list_price_snapshot,annual_price_snapshot,
				custom_monthly_price,custom_annual_list_price,custom_annual_price,custom_module_limit,custom_selection_mode
			)
			SELECT 'ptr_000001','CUSTOM','MONTHLY','ACTIVE',CURRENT_DATE,
				(date_trunc('month',CURRENT_DATE)+interval '1 month')::date,
				(date_trunc('month',CURRENT_DATE)+interval '1 month')::date,
				2000,24000,24000,2000,24000,24000,0,'CUSTOM'
			WHERE EXISTS(SELECT 1 FROM billing.partner_terms WHERE partner_id='ptr_000001')
			ON CONFLICT(partner_id) DO NOTHING`,
		},
	}
}

func start23112PlanBillingRecoveryMigration() common.Migration {
	return common.Migration{
		Version: 12,
		Name: "start-23-11-2-plan-billing-index-recovery",
		AllowDestructiveSchema: true,
		Statements: []string{
			`DROP INDEX IF EXISTS billing.billing_invoice_key_unique`,
			`CREATE UNIQUE INDEX billing_invoice_key_unique ON billing.invoices(invoice_key)`,
		},
	}
}

func start23112InvoiceDateConstraintRecoveryMigration() common.Migration {
	return common.Migration{
		Version: 14,
		Name: "start-23-11-2-remove-legacy-one-invoice-per-day-constraint",
		AllowDestructiveSchema: true,
		Statements: []string{
			`ALTER TABLE billing.invoices DROP CONSTRAINT IF EXISTS billing_invoices_partner_id_invoice_date_key`,
			`ALTER TABLE billing.invoices DROP CONSTRAINT IF EXISTS invoices_partner_id_invoice_date_key`,
			`CREATE INDEX IF NOT EXISTS billing_invoice_partner_date_idx ON billing.invoices(partner_id,invoice_date DESC)`,
		},
	}
}

func start23112PlanLedgerImmutabilityMigration() common.Migration {
	return common.Migration{
		Version: 13,
		Name: "start-23-11-2-plan-ledger-immutability",
		Statements: []string{
			`CREATE OR REPLACE FUNCTION billing.guard_invoice_item_mutation() RETURNS trigger LANGUAGE plpgsql AS $fn$
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
					OR OLD.billing_model IS DISTINCT FROM NEW.billing_model
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
			END; $fn$`,
			`CREATE OR REPLACE FUNCTION billing.guard_plan_invoice_mutation() RETURNS trigger LANGUAGE plpgsql AS $fn$
			BEGIN
				IF TG_OP='DELETE' THEN
					IF OLD.billing_model='PLAN' THEN
						RAISE EXCEPTION 'PLAN invoices are append-only';
					END IF;
					RETURN OLD;
				END IF;
				IF OLD.billing_model IS DISTINCT FROM NEW.billing_model THEN
					RAISE EXCEPTION 'invoice billing_model is immutable';
				END IF;
				IF OLD.billing_model='PLAN' AND (
					OLD.invoice_key IS DISTINCT FROM NEW.invoice_key
					OR OLD.partner_id IS DISTINCT FROM NEW.partner_id
					OR OLD.invoice_date IS DISTINCT FROM NEW.invoice_date
					OR OLD.service_period_start IS DISTINCT FROM NEW.service_period_start
					OR OLD.service_period_end IS DISTINCT FROM NEW.service_period_end
					OR OLD.currency IS DISTINCT FROM NEW.currency
					OR OLD.base_fee IS DISTINCT FROM NEW.base_fee
					OR OLD.module_fee IS DISTINCT FROM NEW.module_fee
					OR OLD.total IS DISTINCT FROM NEW.total
					OR OLD.minimum_commitment_adjustment IS DISTINCT FROM NEW.minimum_commitment_adjustment
					OR OLD.plan_key IS DISTINCT FROM NEW.plan_key
					OR OLD.billing_frequency IS DISTINCT FROM NEW.billing_frequency
					OR OLD.charge_type IS DISTINCT FROM NEW.charge_type
					OR OLD.list_price IS DISTINCT FROM NEW.list_price
					OR OLD.discount_amount IS DISTINCT FROM NEW.discount_amount
					OR OLD.created_at IS DISTINCT FROM NEW.created_at
				) THEN
					RAISE EXCEPTION 'PLAN invoice commercial fields are immutable';
				END IF;
				RETURN NEW;
			END; $fn$`,
			`DROP TRIGGER IF EXISTS billing_plan_invoices_immutable_fields ON billing.invoices`,
			`CREATE TRIGGER billing_plan_invoices_immutable_fields
				BEFORE UPDATE OR DELETE ON billing.invoices
				FOR EACH ROW EXECUTE FUNCTION billing.guard_plan_invoice_mutation()`,
		},
	}
}

func nextMonthStart(at time.Time) time.Time {
	at = dateOnly(at)
	return time.Date(at.Year(), at.Month()+1, 1, 0, 0, 0, 0, time.UTC)
}

func normalizeBillingFrequency(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" { return "MONTHLY" }
	if value != "MONTHLY" && value != "ANNUAL" { return "" }
	return value
}

func (a *app) loadPlan(ctx context.Context, key string) (subscriptionPlan, error) {
	var p subscriptionPlan
	err := a.db.QueryRowContext(ctx, `SELECT plan_key,display_name,currency,monthly_price,annual_list_price,annual_price,
		annual_increase_percent,annual_free_months,module_limit,selection_mode,customer_selectable,active,sort_order
		FROM billing.subscription_plans WHERE plan_key=$1`, strings.ToUpper(strings.TrimSpace(key))).
		Scan(&p.Key,&p.Name,&p.Currency,&p.MonthlyPrice,&p.AnnualListPrice,&p.AnnualPrice,
			&p.AnnualIncreasePercent,&p.AnnualFreeMonths,&p.ModuleLimit,&p.SelectionMode,&p.CustomerSelectable,&p.Active,&p.SortOrder)
	return p, err
}

func roundPlanAmount(value float64) float64 {
	return math.Round(value*100) / 100
}

func standardPlanLimit(key string) (int,bool) {
	switch strings.ToUpper(strings.TrimSpace(key)) {
	case "STARTER":
		return 10,true
	case "BUSINESS":
		return 20,true
	case "FLEX":
		return 0,true
	default:
		return 0,false
	}
}

func (a *app) loadPlanAt(ctx context.Context, key string, at time.Time) (subscriptionPlan,error) {
	p,err:=a.loadPlan(ctx,key)
	if err!=nil{return p,err}
	at=dateOnly(at)
	var effective time.Time
	err=a.db.QueryRowContext(ctx,`SELECT monthly_price,annual_list_price,annual_price,effective_from
		FROM billing.subscription_plan_price_history
		WHERE plan_key=$1 AND effective_from<=$2
		ORDER BY effective_from DESC,
			CASE change_type WHEN 'MANUAL' THEN 3 WHEN 'ANNUAL_INCREASE' THEN 2 ELSE 1 END DESC,
			id DESC LIMIT 1`,p.Key,at).
		Scan(&p.MonthlyPrice,&p.AnnualListPrice,&p.AnnualPrice,&effective)
	if err==sql.ErrNoRows{return p,nil}
	if err!=nil{return p,err}
	p.PriceEffectiveFrom=dateOnly(effective)
	return p,nil
}

func (a *app) ensureAnnualPlanIncreases(ctx context.Context, at time.Time) error {
	at=dateOnly(at)
	rows,err:=a.db.QueryContext(ctx,`SELECT plan_key,annual_increase_percent FROM billing.subscription_plans
		WHERE plan_key IN ('STARTER','BUSINESS','FLEX') ORDER BY sort_order,plan_key`)
	if err!=nil{return err}
	type item struct{ key string; rate float64 }
	plans:=[]item{}
	for rows.Next(){var x item;if err:=rows.Scan(&x.key,&x.rate);err!=nil{rows.Close();return err};plans=append(plans,x)}
	rows.Close()
	for _,x:=range plans{
		var first time.Time
		if err:=a.db.QueryRowContext(ctx,`SELECT MIN(effective_from) FROM billing.subscription_plan_price_history WHERE plan_key=$1`,x.key).Scan(&first);err!=nil{return err}
		for year:=first.Year()+1;year<=at.Year();year++{
			jan1:=time.Date(year,time.January,1,0,0,0,0,time.UTC)
			if at.Before(jan1){continue}
			var exists bool
			if err:=a.db.QueryRowContext(ctx,`SELECT EXISTS(
				SELECT 1 FROM billing.subscription_plan_price_history
				WHERE plan_key=$1 AND effective_from=$2 AND change_type='ANNUAL_INCREASE')`,x.key,jan1).Scan(&exists);err!=nil{return err}
			if exists{continue}
			prior,err:=a.loadPlanAt(ctx,x.key,jan1.AddDate(0,0,-1));if err!=nil{return err}
			factor:=1+x.rate/100
			monthly:=roundPlanAmount(prior.MonthlyPrice*factor)
			list:=roundPlanAmount(prior.AnnualListPrice*factor)
			annual:=roundPlanAmount(prior.AnnualPrice*factor)
			if _,err=a.db.ExecContext(ctx,`INSERT INTO billing.subscription_plan_price_history(
				plan_key,currency,monthly_price,annual_list_price,annual_price,effective_from,change_type,
				annual_increase_percent,actor,reason)
				VALUES($1,$2,$3,$4,$5,$6,'ANNUAL_INCREASE',$7,'billing-cycle',$8)`,
				x.key,prior.Currency,monthly,list,annual,jan1,x.rate,
				fmt.Sprintf("Automatic %.2f%% January 1 package increase",x.rate));err!=nil{return err}
		}
	}
	return nil
}

func (a *app) planModulesAt(ctx context.Context, planKey string, at time.Time) ([]string, error) {
	rows, err := a.db.QueryContext(ctx, `SELECT module_key FROM billing.subscription_plan_modules
		WHERE plan_key=$1 AND effective_from<=$2 AND (effective_to IS NULL OR effective_to>$2)
		ORDER BY position,module_key`, strings.ToUpper(planKey), dateOnly(at))
	if err != nil { return nil, err }
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil { return nil, err }
		out = append(out, key)
	}
	return out, rows.Err()
}

func (a *app) flexModulesAt(ctx context.Context, partnerID string, at time.Time) ([]string, error) {
	rows, err := a.db.QueryContext(ctx, `SELECT module_key FROM billing.partner_plan_module_selections
		WHERE partner_id=$1 AND effective_from<=$2 AND (effective_to IS NULL OR effective_to>$2)
		ORDER BY module_key`, partnerID, dateOnly(at))
	if err != nil { return nil, err }
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil { return nil, err }
		out = append(out, key)
	}
	return out, rows.Err()
}

func (a *app) planReady(ctx context.Context, p subscriptionPlan, at time.Time) (bool, []string, error) {
	if p.SelectionMode != "FIXED" { return true, []string{}, nil }
	keys, err := a.planModulesAt(ctx, p.Key, at)
	if err != nil { return false, nil, err }
	return len(keys) == p.ModuleLimit, keys, nil
}

func planMap(p subscriptionPlan, fixed []string, ready bool) map[string]any {
	savings := p.AnnualListPrice - p.AnnualPrice
	if savings < 0 { savings = 0 }
	var moduleLimit any = p.ModuleLimit
	if p.SelectionMode == selectionModeUnlimited { moduleLimit = nil }
	return map[string]any{
		"plan_key":p.Key,"display_name":p.Name,"currency":p.Currency,
		"monthly_price":p.MonthlyPrice,"annual_list_price":p.AnnualListPrice,"annual_price":p.AnnualPrice,
		"annual_savings":savings,"annual_free_months":p.AnnualFreeMonths,
		"annual_increase_percent":p.AnnualIncreasePercent,
		"price_effective_from":func() any { if p.PriceEffectiveFrom.IsZero(){return nil}; return p.PriceEffectiveFrom.Format("2006-01-02") }(),
		"module_limit":moduleLimit,"selection_mode":p.SelectionMode,
		"customer_selectable":p.CustomerSelectable,"active":p.Active,"sort_order":p.SortOrder,
		"fixed_module_keys":fixed,"ready":ready,"unlimited_modules":p.SelectionMode==selectionModeUnlimited,
	}
}

func (a *app) plans(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { common.APIError(w,405,"METHOD","Use GET"); return }
	if err:=a.ensureAnnualPlanIncreases(r.Context(),time.Now().UTC());err!=nil{common.APIError(w,500,"DB","Could not apply annual package pricing");return}
	rows, err := a.db.QueryContext(r.Context(), `SELECT plan_key FROM billing.subscription_plans ORDER BY sort_order,plan_key`)
	if err != nil { common.APIError(w,500,"DB","Could not load subscription plans"); return }
	defer rows.Close()
	items := []map[string]any{}
	now := time.Now().UTC()
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			common.APIError(w,500,"DB","Could not decode subscription plan"); return
		}
		p,err:=a.loadPlanAt(r.Context(),key,now)
		if err!=nil{common.APIError(w,500,"DB","Could not resolve subscription plan price");return}
		ready, fixed, err := a.planReady(r.Context(), p, now)
		if err != nil { common.APIError(w,500,"DB","Could not load plan modules"); return }
		item:=planMap(p,fixed,ready)
		if err:=a.decoratePackageMap(r.Context(),item,p);err!=nil{common.APIError(w,500,"DB","Could not load package pricing metadata");return}
		items = append(items, item)
	}
	common.JSON(w,200,map[string]any{"items":items,"count":len(items),"pricing_authority":"SUBSCRIPTION_PLAN"})
}

func (a *app) validatePublishedModuleKeys(ctx context.Context, keys []string) error {
	req, _ := http.NewRequestWithContext(ctx,http.MethodGet,"http://"+a.catalogHost+"/api/v1/modules",nil)
	common.BindInternalRequest(req,a.token)
	resp, err := common.DoInternal(a.client, req)
	if err != nil { return err }
	defer resp.Body.Close()
	if resp.StatusCode >= 300 { return fmt.Errorf("catalog returned %d",resp.StatusCode) }
	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil { return err }
	allowed := map[string]bool{}
	if raw,ok:=payload["items"].([]any);ok{
		for _,item:=range raw{
			if m,ok:=item.(map[string]any);ok{
				key:=strings.TrimSpace(fmt.Sprint(m["key"]))
				if key=="" { key=strings.TrimSpace(fmt.Sprint(m["module_key"])) }
				if strings.ToUpper(strings.TrimSpace(fmt.Sprint(m["publication_status"])))=="PUBLISHED" &&
					strings.ToUpper(strings.TrimSpace(fmt.Sprint(m["implementation_state"])))=="READY" {
					allowed[key]=true
				}
			}
		}
	}
	for _,key:=range keys{
		if !allowed[key] { return fmt.Errorf("module %s must be PUBLISHED and READY",key) }
	}
	return nil
}

func (a *app) availablePublishedModuleKeys(ctx context.Context) ([]string,error) {
	req, _ := http.NewRequestWithContext(ctx,http.MethodGet,"http://"+a.catalogHost+"/api/v1/modules",nil)
	common.BindInternalRequest(req,a.token)
	resp, err := common.DoInternal(a.client, req)
	if err != nil { return nil,err }
	defer resp.Body.Close()
	if resp.StatusCode >= 300 { return nil,fmt.Errorf("catalog returned %d",resp.StatusCode) }
	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil { return nil,err }
	keys:=[]string{}
	if raw,ok:=payload["items"].([]any);ok{
		for _,item:=range raw{
			if m,ok:=item.(map[string]any);ok{
				key:=strings.TrimSpace(fmt.Sprint(m["key"]))
				if key=="" { key=strings.TrimSpace(fmt.Sprint(m["module_key"])) }
				if key!="" &&
					strings.ToUpper(strings.TrimSpace(fmt.Sprint(m["publication_status"])))=="PUBLISHED" &&
					strings.ToUpper(strings.TrimSpace(fmt.Sprint(m["implementation_state"])))=="READY" &&
					strings.ToUpper(strings.TrimSpace(fmt.Sprint(m["availability"])))=="ACTIVE" {
					keys=append(keys,key)
				}
			}
		}
	}
	sort.Strings(keys)
	return keys,nil
}

func uniqueModuleKeys(values []string) ([]string,error) {
	seen:=map[string]bool{}
	out:=[]string{}
	for _,raw:=range values{
		key:=strings.TrimSpace(raw)
		if key=="" { continue }
		if seen[key] { return nil,fmt.Errorf("duplicate module %s",key) }
		seen[key]=true;out=append(out,key)
	}
	sort.Strings(out)
	return out,nil
}

func (a *app) planByKey(w http.ResponseWriter, r *http.Request) {
	key := strings.ToUpper(strings.Trim(strings.TrimPrefix(r.URL.Path,"/api/v1/billing/plans/"),"/"))
	if key=="" || strings.Contains(key,"/") { common.APIError(w,404,"NOT_FOUND","Plan not found"); return }
	now:=dateOnly(time.Now().UTC())
	if err:=a.ensureAnnualPlanIncreases(r.Context(),now);err!=nil{common.APIError(w,500,"DB","Could not apply annual package pricing");return}
	p,err:=a.loadPlanAt(r.Context(),key,now)
	if err==sql.ErrNoRows { common.APIError(w,404,"NOT_FOUND","Plan not found");return }
	if err!=nil { common.APIError(w,500,"DB","Could not load plan");return }
	if r.Method==http.MethodGet{
		ready,fixed,err:=a.planReady(r.Context(),p,now)
		if err!=nil{common.APIError(w,500,"DB","Could not load plan modules");return}
		out:=planMap(p,fixed,ready)
		if err:=a.decoratePackageMap(r.Context(),out,p);err!=nil{common.APIError(w,500,"DB","Could not load package pricing metadata");return}
		common.JSON(w,200,out);return
	}
	if r.Method!=http.MethodPatch { common.APIError(w,405,"METHOD","Use GET or PATCH");return }
	var in struct{
		MonthlyPrice *float64 `json:"monthly_price"`
		AnnualListPrice *float64 `json:"annual_list_price"`
		AnnualPrice *float64 `json:"annual_price"`
		AnnualFreeMonths *int `json:"annual_free_months"`
		ModuleLimit *int `json:"module_limit"`
		Active *bool `json:"active"`
		FixedModuleKeys []string `json:"fixed_module_keys"`
		EffectiveAt string `json:"effective_at"`
		Reason string `json:"reason"`
	}
	if common.Decode(r,&in)!=nil{common.APIError(w,400,"JSON","Invalid request");return}
	effective:=now
	if strings.TrimSpace(in.EffectiveAt)!=""{
		parsed,e:=time.Parse("2006-01-02",strings.TrimSpace(in.EffectiveAt))
		if e!=nil{common.APIError(w,400,"VALIDATION","effective_at must be YYYY-MM-DD");return}
		effective=dateOnly(parsed)
	}
	if effective.Before(now){common.APIError(w,400,"VALIDATION","effective_at cannot be in the past");return}
	reason:=strings.TrimSpace(in.Reason);if reason==""{reason="HIMATE administrator package update"}
	actor:=strings.TrimSpace(r.Header.Get("X-Himate-User-ID"));if actor==""{actor="himate-admin"}

	tx,err:=a.db.BeginTx(r.Context(),nil)
	if err!=nil{common.APIError(w,500,"DB","Could not start plan update");return}
	defer tx.Rollback()

	nextMonthly:=p.MonthlyPrice
	nextList:=p.AnnualListPrice
	nextAnnual:=p.AnnualPrice
	nextFree:=p.AnnualFreeMonths
	nextLimit:=p.ModuleLimit
	nextActive:=p.Active
	if in.MonthlyPrice!=nil{nextMonthly=*in.MonthlyPrice}
	if in.AnnualFreeMonths!=nil{nextFree=*in.AnnualFreeMonths}
	if in.ModuleLimit!=nil{nextLimit=*in.ModuleLimit}
	if in.Active!=nil{nextActive=*in.Active}
	if in.MonthlyPrice!=nil || in.AnnualFreeMonths!=nil {
		if in.AnnualListPrice==nil{nextList=roundPlanAmount(nextMonthly*12)}
		if in.AnnualPrice==nil{nextAnnual=roundPlanAmount(nextMonthly*float64(12-nextFree))}
	}
	if in.AnnualListPrice!=nil{nextList=*in.AnnualListPrice}
	if in.AnnualPrice!=nil{nextAnnual=*in.AnnualPrice}
	if fixedLimit,standard:=standardPlanLimit(key);standard{
		if in.ModuleLimit!=nil && *in.ModuleLimit!=fixedLimit{
			common.APIError(w,409,"STANDARD_PACKAGE_LIMIT","Standard package module limits are fixed: Starter 10, Business 20, Premium Unlimited")
			return
		}
		nextLimit=fixedLimit
	}
	if nextMonthly<0||nextList<0||nextAnnual<0||nextAnnual>nextList||nextFree<0||nextFree>12||nextLimit<0{
		common.APIError(w,400,"VALIDATION","Invalid plan commercial values");return
	}
	priceChanged:=nextMonthly!=p.MonthlyPrice||nextList!=p.AnnualListPrice||nextAnnual!=p.AnnualPrice
	metadataChanged:=nextFree!=p.AnnualFreeMonths||nextLimit!=p.ModuleLimit||nextActive!=p.Active
	if priceChanged {
		if _,err=tx.ExecContext(r.Context(),`INSERT INTO billing.subscription_plan_price_history(
			plan_key,currency,monthly_price,annual_list_price,annual_price,effective_from,change_type,
			annual_increase_percent,actor,reason)
			VALUES($1,$2,$3,$4,$5,$6,'MANUAL',$7,$8,$9)`,
			key,p.Currency,nextMonthly,nextList,nextAnnual,effective,p.AnnualIncreasePercent,actor,reason);err!=nil{
			common.APIError(w,500,"DB","Could not save package price history");return
		}
	}
	if priceChanged||metadataChanged{
		if _,err=tx.ExecContext(r.Context(),`UPDATE billing.subscription_plans SET
			monthly_price=CASE WHEN $8<=CURRENT_DATE THEN $2 ELSE monthly_price END,
			annual_list_price=CASE WHEN $8<=CURRENT_DATE THEN $3 ELSE annual_list_price END,
			annual_price=CASE WHEN $8<=CURRENT_DATE THEN $4 ELSE annual_price END,
			annual_free_months=$5,module_limit=$6,active=$7,updated_at=NOW()
			WHERE plan_key=$1`,
			key,nextMonthly,nextList,nextAnnual,nextFree,nextLimit,nextActive,effective);err!=nil{
			common.APIError(w,500,"DB","Could not update plan");return
		}
	}

	if in.FixedModuleKeys!=nil {
		if p.SelectionMode!="FIXED"{common.APIError(w,409,"PLAN_SELECTION_MODE","Only FIXED plans have administrator-defined module sets");return}
		keys,err:=uniqueModuleKeys(in.FixedModuleKeys);if err!=nil{common.APIError(w,400,"VALIDATION",err.Error());return}
		if len(keys)!=nextLimit{common.APIError(w,400,"MODULE_LIMIT",fmt.Sprintf("%s requires exactly %d modules",p.Key,nextLimit));return}
		if err=a.validatePublishedModuleKeys(r.Context(),keys);err!=nil{common.APIError(w,409,"MODULE_NOT_READY",err.Error());return}
		moduleEffective:=effective
		if strings.TrimSpace(in.EffectiveAt)==""{
			var existing int
			_ = tx.QueryRowContext(r.Context(),`SELECT COUNT(*) FROM billing.subscription_plan_modules WHERE plan_key=$1 AND effective_to IS NULL`,key).Scan(&existing)
			if existing>0{moduleEffective=nextMonthStart(now)}
		}
		if _,err=tx.ExecContext(r.Context(),`UPDATE billing.subscription_plan_modules SET effective_to=$2
			WHERE plan_key=$1 AND effective_from<$2 AND (effective_to IS NULL OR effective_to>$2)`,key,moduleEffective);err!=nil{
			common.APIError(w,500,"DB","Could not close prior module set");return
		}
		if _,err=tx.ExecContext(r.Context(),`DELETE FROM billing.subscription_plan_modules WHERE plan_key=$1 AND effective_from=$2`,key,moduleEffective);err!=nil{
			common.APIError(w,500,"DB","Could not replace scheduled module set");return
		}
		for i,moduleKey:=range keys{
			if _,err=tx.ExecContext(r.Context(),`INSERT INTO billing.subscription_plan_modules(plan_key,module_key,position,effective_from)
				VALUES($1,$2,$3,$4) ON CONFLICT(plan_key,module_key,effective_from)
				DO UPDATE SET position=EXCLUDED.position,effective_to=NULL`,
				key,moduleKey,i,moduleEffective);err!=nil{common.APIError(w,500,"DB","Could not save plan module");return}
		}
	}
	if err=tx.Commit();err!=nil{common.APIError(w,500,"DB","Could not commit plan update");return}
	current,err:=a.loadPlanAt(r.Context(),key,now)
	if err!=nil{common.APIError(w,500,"DB","Could not reload plan");return}
	ready,fixed,_:=a.planReady(r.Context(),current,now)
	out:=planMap(current,fixed,ready)
	if err:=a.decoratePackageMap(r.Context(),out,current);err!=nil{common.APIError(w,500,"DB","Could not load package pricing metadata");return}
	if priceChanged&&effective.After(now){out["scheduled_price_effective_at"]=effective.Format("2006-01-02")}
	common.JSON(w,200,out)
}

func (a *app) loadPartnerPlan(ctx context.Context, partnerID string) (partnerPlanSubscription,error) {
	var s partnerPlanSubscription
	err:=a.db.QueryRowContext(ctx,`SELECT partner_id,plan_key,billing_frequency,status,current_period_start,current_period_end,next_billing_at,
		monthly_price_snapshot,annual_list_price_snapshot,annual_price_snapshot,next_plan_key,next_billing_frequency,change_effective_at,
		custom_monthly_price,custom_annual_list_price,custom_annual_price,custom_module_limit,custom_selection_mode
		FROM billing.partner_plan_subscriptions WHERE partner_id=$1`,partnerID).
		Scan(&s.PartnerID,&s.PlanKey,&s.BillingFrequency,&s.Status,&s.CurrentPeriodStart,&s.CurrentPeriodEnd,&s.NextBillingAt,
			&s.MonthlyPriceSnapshot,&s.AnnualListPriceSnapshot,&s.AnnualPriceSnapshot,&s.NextPlanKey,&s.NextBillingFrequency,&s.ChangeEffectiveAt,
			&s.CustomMonthlyPrice,&s.CustomAnnualListPrice,&s.CustomAnnualPrice,&s.CustomModuleLimit,&s.CustomSelectionMode)
	return s,err
}

func (a *app) effectivePlanPrices(s partnerPlanSubscription,p subscriptionPlan)(float64,float64,float64,int,string){
	if p.Key=="CUSTOM"{
		return s.CustomMonthlyPrice,s.CustomAnnualListPrice,s.CustomAnnualPrice,s.CustomModuleLimit,s.CustomSelectionMode
	}
	return p.MonthlyPrice,p.AnnualListPrice,p.AnnualPrice,p.ModuleLimit,p.SelectionMode
}

func (a *app) partnerPlanMap(ctx context.Context,s partnerPlanSubscription) (map[string]any,error) {
	p,err:=a.loadPlanAt(ctx,s.PlanKey,time.Now().UTC());if err!=nil{return nil,err}
	monthly,list,annual,limit,mode:=a.effectivePlanPrices(s,p)
	var modules []string
	if mode=="FIXED"{modules,err=a.planModulesAt(ctx,p.Key,time.Now().UTC())}else if mode=="SELECTABLE"{modules,err=a.flexModulesAt(ctx,s.PartnerID,time.Now().UTC())}else if mode==selectionModeUnlimited{modules,err=a.availablePublishedModuleKeys(ctx)}
	if err!=nil{return nil,err}
	var change any
	if s.ChangeEffectiveAt.Valid{
		change=map[string]any{"next_plan_key":s.NextPlanKey,"next_billing_frequency":s.NextBillingFrequency,"effective_at":dateOnly(s.ChangeEffectiveAt.Time).Format("2006-01-02")}
	}
	var moduleLimit any=limit
	if mode==selectionModeUnlimited{moduleLimit=nil}
	out:=map[string]any{
		"partner_id":s.PartnerID,"plan_key":s.PlanKey,"display_name":p.Name,"billing_frequency":s.BillingFrequency,"status":s.Status,
		"currency":p.Currency,"monthly_price":monthly,"annual_list_price":list,"annual_price":annual,"annual_savings":list-annual,
		"module_limit":moduleLimit,"selection_mode":mode,"entitlement_mode":mode,"unlimited_modules":mode==selectionModeUnlimited,
		"active_module_keys":modules,"available_module_count":len(modules),
		"current_period_start":dateOnly(s.CurrentPeriodStart).Format("2006-01-02"),
		"current_period_end_exclusive":dateOnly(s.CurrentPeriodEnd).Format("2006-01-02"),
		"next_billing_at":dateOnly(s.NextBillingAt).Format("2006-01-02"),"scheduled_change":change,
	}
	policy,err:=a.loadBillingTaxPolicy(ctx);if err!=nil{return nil,err}
	addTaxQuote(out,monthly,annual,policy)
	return out,nil
}

func sameModuleKeys(aKeys,bKeys []string) bool {
	if len(aKeys)!=len(bKeys){return false}
	for i:=range aKeys{if aKeys[i]!=bKeys[i]{return false}}
	return true
}

func (a *app) partnerPlan(w http.ResponseWriter,r *http.Request,partnerID string){
	if r.Method==http.MethodGet{
		s,err:=a.loadPartnerPlan(r.Context(),partnerID)
		if err==sql.ErrNoRows{common.JSON(w,200,map[string]any{"partner_id":partnerID,"configured":false});return}
		if err!=nil{common.APIError(w,500,"DB","Could not load partner plan");return}
		out,err:=a.partnerPlanMap(r.Context(),s);if err!=nil{common.APIError(w,500,"DB","Could not load partner plan details");return}
		out["configured"]=true;common.JSON(w,200,out);return
	}
	if r.Method!=http.MethodPatch && r.Method!=http.MethodPut{common.APIError(w,405,"METHOD","Use GET, PUT or PATCH");return}
	var in struct{
		PlanKey string `json:"plan_key"`
		BillingFrequency string `json:"billing_frequency"`
		ModuleKeys []string `json:"module_keys"`
		CustomMonthlyPrice *float64 `json:"custom_monthly_price"`
		CustomAnnualListPrice *float64 `json:"custom_annual_list_price"`
		CustomAnnualPrice *float64 `json:"custom_annual_price"`
		CustomModuleLimit *int `json:"custom_module_limit"`
		Reason string `json:"reason"`
	}
	if common.Decode(r,&in)!=nil{common.APIError(w,400,"JSON","Invalid request");return}
	targetKey:=strings.ToUpper(strings.TrimSpace(in.PlanKey));frequency:=normalizeBillingFrequency(in.BillingFrequency)
	if targetKey==""||frequency==""{common.APIError(w,400,"VALIDATION","plan_key and MONTHLY/ANNUAL billing_frequency are required");return}
	now:=dateOnly(time.Now().UTC())
	if err:=a.ensureAnnualPlanIncreases(r.Context(),now);err!=nil{common.APIError(w,500,"DB","Could not apply annual package pricing");return}
	target,err:=a.loadPlanAt(r.Context(),targetKey,now);if err==sql.ErrNoRows{common.APIError(w,404,"PLAN_NOT_FOUND","Plan not found");return};if err!=nil{common.APIError(w,500,"DB","Could not load target plan");return}
	if !target.Active{common.APIError(w,409,"PLAN_INACTIVE","Plan is inactive");return}
	commercialMode,err:=a.ensureCommercialMode(r.Context(),partnerID)
	if err!=nil{common.APIError(w,500,"DB","Could not load partner commercial mode");return}
	if commercialMode.BillingMode==billingModeCharity && commercialMode.CharityStatus==charityApproved{
		common.APIError(w,409,"CHARITY_USES_MODULE_SELECTION","Approved Charity partners select modules directly and do not use paid subscription packages")
		return
	}
	ready,_,err:=a.planReady(r.Context(),target,now);if err!=nil{common.APIError(w,500,"DB","Could not validate target plan");return}
	if !ready{common.APIError(w,409,"PLAN_NOT_READY","Fixed plan module set is not fully configured");return}
	keys,err:=uniqueModuleKeys(in.ModuleKeys);if err!=nil{common.APIError(w,400,"VALIDATION",err.Error());return}
	if target.SelectionMode=="SELECTABLE"{
		if len(keys)>target.ModuleLimit{common.APIError(w,400,"MODULE_LIMIT",fmt.Sprintf("%s allows at most %d modules",target.Key,target.ModuleLimit));return}
		if err=a.validatePublishedModuleKeys(r.Context(),keys);err!=nil{common.APIError(w,409,"MODULE_NOT_READY",err.Error());return}
	}
	if target.SelectionMode==selectionModeUnlimited && len(keys)>0{
		common.APIError(w,409,"UNLIMITED_PLAN_MANAGED","Premium module entitlement is automatic and cannot be replaced by a partner-selected list");return
	}
	if target.Key=="CUSTOM"{
		if in.CustomMonthlyPrice==nil{common.APIError(w,400,"CUSTOM_PRICE_REQUIRED","Custom monthly price is required");return}
	}
	current,loadErr:=a.loadPartnerPlan(r.Context(),partnerID)
	if loadErr!=nil && loadErr!=sql.ErrNoRows{common.APIError(w,500,"DB","Could not load current plan");return}
	if loadErr==sql.ErrNoRows {
		license,licenseErr:=a.ensureLicense(partnerID)
		if licenseErr!=nil{common.APIError(w,500,"DB","Could not verify activation license");return}
		if license.Status!="PAID" && !license.Waived {
			common.APIError(w,409,"ACTIVATION_LICENSE_REQUIRED","Activation license must be paid or explicitly waived before a subscription plan can be activated");return
		}
		if commercialMode.BillingMode==billingModePaid{
			autopayReady,profileErr:=a.paymentProfileAutopayReady(r.Context(),partnerID)
			if profileErr!=nil{common.APIError(w,502,"PAYMENT_PROFILE_UNAVAILABLE","Payment profile could not be verified");return}
			if !autopayReady{
				common.APIError(w,409,"AUTOPAY_REQUIRED","A saved payment method with automatic recurring collection enabled is required before a paid subscription plan can be activated");return
			}
		}
	}
	actor:=strings.TrimSpace(r.Header.Get("X-Himate-User-ID"));if actor==""{actor="partner"}
	reason:=strings.TrimSpace(in.Reason);if reason==""{reason="Subscription plan selection"}
	if loadErr==sql.ErrNoRows{
		monthly,list,annual:=target.MonthlyPrice,target.AnnualListPrice,target.AnnualPrice
		customMonthly,customList,customAnnual,customLimit:=0.0,0.0,0.0,0
		customMode:="CUSTOM"
		if target.Key=="CUSTOM"{
			customMonthly=*in.CustomMonthlyPrice;monthly=customMonthly
			if in.CustomAnnualListPrice!=nil{customList=*in.CustomAnnualListPrice}else{customList=customMonthly*12}
			if in.CustomAnnualPrice!=nil{customAnnual=*in.CustomAnnualPrice}else{customAnnual=customList}
			if in.CustomModuleLimit!=nil{customLimit=*in.CustomModuleLimit}
			list,annual=customList,customAnnual
		}
		start:=now;end:=nextMonthStart(now);nextBilling:=end
		if frequency=="ANNUAL"{end=start.AddDate(1,0,0);nextBilling=end}
		_,err=a.db.ExecContext(r.Context(),`INSERT INTO billing.partner_plan_subscriptions(
			partner_id,plan_key,billing_frequency,status,current_period_start,current_period_end,next_billing_at,
			monthly_price_snapshot,annual_list_price_snapshot,annual_price_snapshot,
			custom_monthly_price,custom_annual_list_price,custom_annual_price,custom_module_limit,custom_selection_mode
		) VALUES($1,$2,$3,'ACTIVE',$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
			partnerID,target.Key,frequency,start,end,nextBilling,monthly,list,annual,customMonthly,customList,customAnnual,customLimit,customMode)
		if err!=nil{common.APIError(w,500,"DB","Could not create partner plan");return}
		if target.SelectionMode=="SELECTABLE"{
			if err=a.replaceFlexSelection(r.Context(),partnerID,keys,now,"INITIAL_SELECTION");err!=nil{common.APIError(w,500,"DB","Could not save Flex module selection");return}
		}
		if frequency=="ANNUAL"{
			actualAnnual:=annual
			if commercialMode.BillingMode!=billingModePaid{actualAnnual=0}
			invoiceID,grossCharge,err:=a.createPlanInvoice(r.Context(),partnerID,target.Key,frequency,"PLAN_ANNUAL_PREPAY",start,end,list,actualAnnual)
			if err!=nil{common.APIError(w,500,"INVOICE","Could not create annual prepayment invoice");return}
			if invoiceID!=""{a.queueInvoiceCollection(r.Context(),invoiceID,partnerID,target.Currency,grossCharge)}
		}
		_,_=a.db.ExecContext(r.Context(),`INSERT INTO billing.plan_change_history(partner_id,new_plan_key,new_billing_frequency,change_type,effective_at,actor,reason)
			VALUES($1,$2,$3,'INITIAL',$4,$5,$6)`,partnerID,target.Key,frequency,now,actor,reason)
		s,_:=a.loadPartnerPlan(r.Context(),partnerID)
		out,_:=a.partnerPlanMap(r.Context(),s)
		out["change_type"]="INITIAL"
		if err=a.syncPlanEntitlements(r.Context(),partnerID,target.Key,now);err!=nil{
			out["entitlement_sync_pending"]=true
			out["warning"]="Plan and billing state saved; module entitlement synchronization will retry automatically."
			common.JSON(w,202,out);return
		}
		common.JSON(w,201,out);return
	}
	currentPlan,err:=a.loadPlanAt(r.Context(),current.PlanKey,now);if err!=nil{common.APIError(w,500,"DB","Could not load current plan definition");return}
	if current.PlanKey=="CUSTOM" && target.Key!="CUSTOM" && !target.CustomerSelectable{
		common.APIError(w,409,"PLAN_NOT_SELECTABLE","Target plan is not customer-selectable");return
	}
	currentMonthly,_,currentAnnual,_,_:=a.effectivePlanPrices(current,currentPlan)
	targetMonthly,targetList,targetAnnual:=target.MonthlyPrice,target.AnnualListPrice,target.AnnualPrice
	if target.Key=="CUSTOM"{
		targetMonthly=*in.CustomMonthlyPrice
		if in.CustomAnnualListPrice!=nil{targetList=*in.CustomAnnualListPrice}else{targetList=targetMonthly*12}
		if in.CustomAnnualPrice!=nil{targetAnnual=*in.CustomAnnualPrice}else{targetAnnual=targetList}
	}
	if current.PlanKey==target.Key && current.BillingFrequency==frequency {
		if target.SelectionMode=="SELECTABLE" && len(keys)>0 {
			currentKeys,keyErr:=a.flexModulesAt(r.Context(),partnerID,now)
			if keyErr!=nil{common.APIError(w,500,"DB","Could not load current Flex module selection");return}
			if !sameModuleKeys(currentKeys,keys) {
				effective:=now
				changeType:="RECOVERY_SELECTION"
				if len(currentKeys)>0 {effective=nextMonthStart(now);changeType="SCHEDULED_MODULE_SET"}
				if err=a.replaceFlexSelection(r.Context(),partnerID,keys,effective,changeType);err!=nil{common.APIError(w,500,"DB","Could not update Flex module selection");return}
				if !effective.Equal(now) {
					s,_:=a.loadPartnerPlan(r.Context(),partnerID);out,_:=a.partnerPlanMap(r.Context(),s)
					out["change_type"]=changeType;out["module_change_effective_at"]=effective.Format("2006-01-02")
					common.JSON(w,200,out);return
				}
			}
		}
		syncErr:=a.syncPlanEntitlements(r.Context(),partnerID,target.Key,now)
		s,_:=a.loadPartnerPlan(r.Context(),partnerID);out,_:=a.partnerPlanMap(r.Context(),s);out["change_type"]="NO_CHANGE"
		if syncErr!=nil {
			out["entitlement_sync_pending"]=true
			out["warning"]="Billing plan is unchanged; module entitlement synchronization will retry automatically."
			common.JSON(w,202,out);return
		}
		common.JSON(w,200,out);return
	}
	if current.BillingFrequency!=frequency {
		effective:=current.NextBillingAt
		if _,err=a.db.ExecContext(r.Context(),`UPDATE billing.partner_plan_subscriptions SET next_plan_key=$2,next_billing_frequency=$3,change_effective_at=$4,updated_at=NOW() WHERE partner_id=$1`,
			partnerID,target.Key,frequency,effective);err!=nil{common.APIError(w,500,"DB","Could not schedule billing-frequency change");return}
		_,_=a.db.ExecContext(r.Context(),`INSERT INTO billing.plan_change_history(partner_id,old_plan_key,new_plan_key,old_billing_frequency,new_billing_frequency,change_type,effective_at,actor,reason)
			VALUES($1,$2,$3,$4,$5,'SCHEDULED_FREQUENCY_CHANGE',$6,$7,$8)`,partnerID,current.PlanKey,target.Key,current.BillingFrequency,frequency,effective,actor,reason)
		s,_:=a.loadPartnerPlan(r.Context(),partnerID);out,_:=a.partnerPlanMap(r.Context(),s);out["change_type"]="SCHEDULED";common.JSON(w,200,out);return
	}
	currentPrice:=currentMonthly;targetPrice:=targetMonthly
	if frequency=="ANNUAL"{currentPrice=currentAnnual;targetPrice=targetAnnual}
	if targetPrice>currentPrice{
		if commercialMode.BillingMode==billingModePaid{
			autopayReady,profileErr:=a.paymentProfileAutopayReady(r.Context(),partnerID)
			if profileErr!=nil{common.APIError(w,502,"PAYMENT_PROFILE_UNAVAILABLE","Payment profile could not be verified");return}
			if !autopayReady{
				common.APIError(w,409,"AUTOPAY_REQUIRED","A saved payment method with automatic recurring collection enabled is required before an immediate paid upgrade");return
			}
		}
		diff:=targetPrice-currentPrice
		_,err=a.db.ExecContext(r.Context(),`UPDATE billing.partner_plan_subscriptions SET
			plan_key=$2,monthly_price_snapshot=$3,annual_list_price_snapshot=$4,annual_price_snapshot=$5,
			next_plan_key='',next_billing_frequency='',change_effective_at=NULL,updated_at=NOW() WHERE partner_id=$1`,
			partnerID,target.Key,targetMonthly,targetList,targetAnnual)
		if err!=nil{common.APIError(w,500,"DB","Could not apply plan upgrade");return}
		if target.SelectionMode=="SELECTABLE"{
			if err=a.replaceFlexSelection(r.Context(),partnerID,keys,now,"UPGRADE_SELECTION");err!=nil{common.APIError(w,500,"DB","Could not save Flex module selection");return}
		}
		actualUpgradeCharge:=diff
		if commercialMode.BillingMode!=billingModePaid{actualUpgradeCharge=0}
		invoiceID,grossCharge,err:=a.createPlanInvoice(r.Context(),partnerID,target.Key,frequency,"PLAN_UPGRADE",now,current.CurrentPeriodEnd,diff,actualUpgradeCharge)
		if err!=nil{common.APIError(w,500,"INVOICE","Could not create upgrade invoice");return}
		if invoiceID!=""{a.queueInvoiceCollection(r.Context(),invoiceID,partnerID,target.Currency,grossCharge)}
		_,_=a.db.ExecContext(r.Context(),`INSERT INTO billing.plan_change_history(partner_id,old_plan_key,new_plan_key,old_billing_frequency,new_billing_frequency,change_type,effective_at,upgrade_charge,actor,reason)
			VALUES($1,$2,$3,$4,$5,'IMMEDIATE_UPGRADE',$6,$7,$8,$9)`,partnerID,current.PlanKey,target.Key,frequency,frequency,now,actualUpgradeCharge,actor,reason)
		s,_:=a.loadPartnerPlan(r.Context(),partnerID)
		out,_:=a.partnerPlanMap(r.Context(),s)
		out["change_type"]="IMMEDIATE_UPGRADE";out["upgrade_charge"]=actualUpgradeCharge
		if err=a.syncPlanEntitlements(r.Context(),partnerID,target.Key,now);err!=nil{
			out["entitlement_sync_pending"]=true
			out["warning"]="Upgrade and charge saved; module entitlement synchronization will retry automatically."
			common.JSON(w,202,out);return
		}
		common.JSON(w,200,out);return
	}
	effective:=nextMonthStart(now);if frequency=="ANNUAL"{effective=current.NextBillingAt}
	if _,err=a.db.ExecContext(r.Context(),`UPDATE billing.partner_plan_subscriptions SET next_plan_key=$2,next_billing_frequency=$3,change_effective_at=$4,updated_at=NOW() WHERE partner_id=$1`,
		partnerID,target.Key,frequency,effective);err!=nil{common.APIError(w,500,"DB","Could not schedule plan change");return}
	_,_=a.db.ExecContext(r.Context(),`INSERT INTO billing.plan_change_history(partner_id,old_plan_key,new_plan_key,old_billing_frequency,new_billing_frequency,change_type,effective_at,actor,reason)
		VALUES($1,$2,$3,$4,$5,'SCHEDULED_DOWNGRADE',$6,$7,$8)`,partnerID,current.PlanKey,target.Key,frequency,frequency,effective,actor,reason)
	s,_:=a.loadPartnerPlan(r.Context(),partnerID);out,_:=a.partnerPlanMap(r.Context(),s);out["change_type"]="SCHEDULED_DOWNGRADE";common.JSON(w,200,out)
}

func (a *app) replaceFlexSelection(ctx context.Context,partnerID string,keys []string,effective time.Time,source string) error{
	keys,err:=uniqueModuleKeys(keys);if err!=nil{return err}
	effective=dateOnly(effective)
	tx,err:=a.db.BeginTx(ctx,nil);if err!=nil{return err};defer tx.Rollback()
	if _,err=tx.ExecContext(ctx,`UPDATE billing.partner_plan_module_selections SET effective_to=$2
		WHERE partner_id=$1 AND effective_from<$2 AND (effective_to IS NULL OR effective_to>$2)`,partnerID,effective);err!=nil{return err}
	if _,err=tx.ExecContext(ctx,`DELETE FROM billing.partner_plan_module_selections WHERE partner_id=$1 AND effective_from=$2`,partnerID,effective);err!=nil{return err}
	for _,key:=range keys{
		if _,err=tx.ExecContext(ctx,`INSERT INTO billing.partner_plan_module_selections(partner_id,module_key,effective_from,source)
			VALUES($1,$2,$3,$4) ON CONFLICT(partner_id,module_key,effective_from) DO UPDATE SET effective_to=NULL,source=EXCLUDED.source`,
			partnerID,key,effective,source);err!=nil{return err}
	}
	return tx.Commit()
}

func (a *app) partnerPlanModules(w http.ResponseWriter,r *http.Request,partnerID string){
	s,err:=a.loadPartnerPlan(r.Context(),partnerID);if err==sql.ErrNoRows{common.APIError(w,409,"PLAN_REQUIRED","Choose a subscription plan first");return};if err!=nil{common.APIError(w,500,"DB","Could not load partner plan");return}
	p,err:=a.loadPlan(r.Context(),s.PlanKey);if err!=nil{common.APIError(w,500,"DB","Could not load plan");return}
	if r.Method==http.MethodGet{
		var keys []string
		if p.SelectionMode=="FIXED"{keys,err=a.planModulesAt(r.Context(),p.Key,time.Now().UTC())}else if p.SelectionMode=="SELECTABLE"{keys,err=a.flexModulesAt(r.Context(),partnerID,time.Now().UTC())}else if p.SelectionMode==selectionModeUnlimited{keys,err=a.availablePublishedModuleKeys(r.Context())}
		if err!=nil{common.APIError(w,500,"DB","Could not load plan module set");return}
		var limit any=p.ModuleLimit;if p.SelectionMode==selectionModeUnlimited{limit=nil}
		common.JSON(w,200,map[string]any{"partner_id":partnerID,"plan_key":p.Key,"selection_mode":p.SelectionMode,"entitlement_mode":p.SelectionMode,"module_limit":limit,"module_keys":keys,"count":len(keys),"unlimited_modules":p.SelectionMode==selectionModeUnlimited});return
	}
	if r.Method!=http.MethodPut{common.APIError(w,405,"METHOD","Use GET or PUT");return}
	if p.SelectionMode==selectionModeUnlimited{common.APIError(w,409,"UNLIMITED_PLAN_MANAGED","Premium includes all current and future eligible modules automatically");return}
	if p.SelectionMode!="SELECTABLE"{common.APIError(w,409,"FIXED_PLAN","Starter and Business module sets are fixed by HIMATE");return}
	var in struct{ModuleKeys []string `json:"module_keys"`; Reason string `json:"reason"`}
	if common.Decode(r,&in)!=nil{common.APIError(w,400,"JSON","Invalid request");return}
	keys,err:=uniqueModuleKeys(in.ModuleKeys);if err!=nil{common.APIError(w,400,"VALIDATION",err.Error());return}
	if len(keys)>p.ModuleLimit{common.APIError(w,400,"MODULE_LIMIT",fmt.Sprintf("Flex allows at most %d modules",p.ModuleLimit));return}
	if err=a.validatePublishedModuleKeys(r.Context(),keys);err!=nil{common.APIError(w,409,"MODULE_NOT_READY",err.Error());return}
	effective:=nextMonthStart(time.Now().UTC())
	if err=a.replaceFlexSelection(r.Context(),partnerID,keys,effective,"PARTNER_SCHEDULED_SELECTION");err!=nil{common.APIError(w,500,"DB","Could not schedule Flex module selection");return}
	common.JSON(w,200,map[string]any{"partner_id":partnerID,"plan_key":p.Key,"module_keys":keys,"effective_at":effective.Format("2006-01-02"),"change_type":"SCHEDULED_MODULE_SET"})
}

func (a *app) syncPlanEntitlements(ctx context.Context,partnerID,planKey string,at time.Time) error{
	p,err:=a.loadPlan(ctx,planKey);if err!=nil{return err}
	if p.SelectionMode=="CUSTOM"{return nil}
	var keys []string
	if p.SelectionMode=="FIXED"{keys,err=a.planModulesAt(ctx,p.Key,at)}else if p.SelectionMode=="SELECTABLE"{keys,err=a.flexModulesAt(ctx,partnerID,at)}else if p.SelectionMode==selectionModeUnlimited{keys=[]string{}}
	if err!=nil{return err}
	payload:=map[string]any{"plan_key":p.Key,"module_keys":keys,"entitlement_mode":p.SelectionMode,"reason":"Subscription plan entitlement synchronization"}
	raw,_:=json.Marshal(payload)
	req,err:=http.NewRequestWithContext(ctx,http.MethodPut,"http://"+a.catalogHost+"/internal/v1/partners/"+partnerID+"/plan-entitlements",bytes.NewReader(raw))
	if err!=nil{return err}
	req.Header.Set("Content-Type","application/json");common.BindInternalRequest(req,a.token);req.Header.Set("X-Himate-User-ID","billing-plan-engine")
	resp,err:=common.DoInternal(a.client, req);if err!=nil{return err};defer resp.Body.Close()
	if resp.StatusCode>=300{return fmt.Errorf("catalog plan entitlement sync returned %d",resp.StatusCode)}
	return nil
}

func (a *app) createPlanInvoice(ctx context.Context,partnerID,planKey,frequency,chargeType string,start,end time.Time,listPrice,amount float64)(string,float64,error){
	start,end=dateOnly(start),dateOnly(end)
	policy,err:=a.loadBillingTaxPolicy(ctx);if err!=nil{return "",0,err}
	netAmount,taxAmount,grossAmount:=applyBillingTax(amount,policy)
	key:=fmt.Sprintf("PLAN:%s:%s:%s:%s",partnerID,chargeType,start.Format("2006-01-02"),planKey)
	if netAmount <= 0 {
		eventKey:=fmt.Sprintf("ZERO_DOLLAR_BILLING_CYCLE:%s:%s:%s:%s",partnerID,chargeType,start.Format("2006-01-02"),planKey)
		if err:=a.emitBillingEvent(ctx,eventKey,partnerID,"","ZERO_DOLLAR_BILLING_CYCLE",time.Now().UTC(),map[string]any{
			"billing_model":"PLAN","plan_key":planKey,"billing_frequency":frequency,"charge_type":chargeType,
			"list_price":listPrice,"net_total":0,"tax_rate_percent":policy.RatePercent,"tax_amount":0,"total":0,
			"service_period_start":start.Format("2006-01-02"),"service_period_end_exclusive":end.Format("2006-01-02"),
		});err!=nil{return "",0,err}
		return "",0,nil
	}
	id:="inv_plan_"+strings.ReplaceAll(partnerID,"_","")+"_"+strings.ToLower(chargeType)+"_"+strings.ToLower(planKey)+"_"+start.Format("20060102")
	discount:=listPrice-netAmount;if discount<0{discount=0}
	tx,err:=a.db.BeginTx(ctx,nil);if err!=nil{return "",0,err}
	defer tx.Rollback()
	if _,err=tx.ExecContext(ctx,`INSERT INTO billing.invoices(
		id,invoice_key,partner_id,invoice_date,service_period_start,service_period_end,currency,base_fee,module_fee,total,
		minimum_commitment_adjustment,billing_model,plan_key,billing_frequency,charge_type,list_price,discount_amount,
		net_total,tax_rate_percent,tax_amount,workflow_status,source
	) VALUES($1,$2,$3,CURRENT_DATE,$4,$5,'USD',$6,0,$7,0,'PLAN',$8,$9,$10,$11,$12,$6,$13,$14,'DRAFT','AUTOMATED')
	ON CONFLICT(invoice_key) DO NOTHING`,
		id,key,partnerID,start,end,netAmount,grossAmount,planKey,frequency,chargeType,listPrice,discount,policy.RatePercent,taxAmount);err!=nil{return "",0,err}
	if err=tx.QueryRowContext(ctx,`SELECT id FROM billing.invoices WHERE invoice_key=$1 FOR UPDATE`,key).Scan(&id);err!=nil{return "",0,err}
	itemKey:="PLAN_ITEM:"+key
	if _,err=tx.ExecContext(ctx,`INSERT INTO billing.invoice_items(
		item_key,invoice_id,partner_id,item_type,description,currency,quantity,unit_price,amount,period_start,period_end,status,invoiced_at,billing_model
	) VALUES($1,$2,$3,'PLAN',$4,'USD',1,$5,$5,$6,$7,'INVOICED',NOW(),'PLAN')
	ON CONFLICT(item_key) DO NOTHING`,
		itemKey,id,partnerID,fmt.Sprintf("%s %s subscription",planKey,frequency),netAmount,start,end);err!=nil{return "",0,err}
	if taxAmount>0{
		taxItemKey:="TAX_ITEM:"+key
		if _,err=tx.ExecContext(ctx,`INSERT INTO billing.invoice_items(
			item_key,invoice_id,partner_id,item_type,description,currency,quantity,unit_price,amount,period_start,period_end,status,invoiced_at,billing_model
		) VALUES($1,$2,$3,'TAX',$4,'USD',1,$5,$5,$6,$7,'INVOICED',NOW(),'PLAN')
		ON CONFLICT(item_key) DO NOTHING`,
			taxItemKey,id,partnerID,fmt.Sprintf("%s %.2f%%",policy.Label,policy.RatePercent),taxAmount,start,end);err!=nil{return "",0,err}
	}
	var storedInvoice,storedPartner string
	var storedAmount float64
	if err=tx.QueryRowContext(ctx,`SELECT invoice_id,partner_id,amount FROM billing.invoice_items WHERE item_key=$1`,itemKey).
		Scan(&storedInvoice,&storedPartner,&storedAmount);err!=nil{return "",0,err}
	if storedInvoice!=id||storedPartner!=partnerID||math.Abs(storedAmount-netAmount)>0.005{
		return "",0,fmt.Errorf("plan invoice item idempotency conflict for %s",itemKey)
	}
	if err=a.recordFinanceTransactionTx(ctx,tx,partnerID,id,"INVOICE","DRAFT","USD",netAmount,taxAmount,grossAmount,"AUTOMATED","billing-plan-engine","Recurring package invoice draft generated");err!=nil{return "",0,err}
	if err=emitBillingEventTx(ctx,tx,"INVOICE_GENERATED:"+id,partnerID,"","INVOICE_GENERATED",time.Now().UTC(),map[string]any{
		"invoice_id":id,"billing_model":"PLAN","plan_key":planKey,"billing_frequency":frequency,"charge_type":chargeType,
		"list_price":listPrice,"discount_amount":discount,"net_total":netAmount,
		"tax_rate_percent":policy.RatePercent,"tax_amount":taxAmount,"total":grossAmount,
		"service_period_start":start.Format("2006-01-02"),"service_period_end_exclusive":end.Format("2006-01-02"),
	});err!=nil{return "",0,err}
	if err=tx.Commit();err!=nil{return "",0,err}
	a.advanceOnboardingFromInvoice(ctx,partnerID,onboardingInvoicePending,"billing-plan-engine","Recurring package invoice draft generated")
	return id,grossAmount,nil
}
func (a *app) applyDuePlanChanges(ctx context.Context,at time.Time) error{
	at=dateOnly(at)
	rows,err:=a.db.QueryContext(ctx,`SELECT partner_id,next_plan_key,next_billing_frequency FROM billing.partner_plan_subscriptions
		WHERE status='ACTIVE' AND change_effective_at IS NOT NULL AND change_effective_at<=$1 ORDER BY partner_id`,at)
	if err!=nil{return err}
	type item struct{partner,plan,freq string};items:=[]item{}
	for rows.Next(){var x item;if err:=rows.Scan(&x.partner,&x.plan,&x.freq);err!=nil{rows.Close();return err};items=append(items,x)}
	rows.Close()
	for _,x:=range items{
		p,err:=a.loadPlanAt(ctx,x.plan,at);if err!=nil{return err}
		monthly,list,annual:=p.MonthlyPrice,p.AnnualListPrice,p.AnnualPrice
		if _,err=a.db.ExecContext(ctx,`UPDATE billing.partner_plan_subscriptions SET plan_key=$2,billing_frequency=$3,
			monthly_price_snapshot=$4,annual_list_price_snapshot=$5,annual_price_snapshot=$6,
			next_plan_key='',next_billing_frequency='',change_effective_at=NULL,updated_at=NOW() WHERE partner_id=$1`,
			x.partner,p.Key,x.freq,monthly,list,annual);err!=nil{return err}
		if err=a.syncPlanEntitlements(ctx,x.partner,p.Key,at);err!=nil{return err}
	}
	return nil
}

func (a *app) runPlanBillingCycle(ctx context.Context,at time.Time) (map[string]bool,error){
	at=dateOnly(at)
	if err:=a.ensureAnnualPlanIncreases(ctx,at);err!=nil{return nil,err}
	if err:=a.applyDuePlanChanges(ctx,at);err!=nil{return nil,err}
	rows,err:=a.db.QueryContext(ctx,`SELECT partner_id,status FROM billing.partner_plan_subscriptions`)
	if err!=nil{return nil,err}
	managed:=map[string]bool{};ids:=[]string{}
	for rows.Next(){
		var id,status string
		if err:=rows.Scan(&id,&status);err!=nil{rows.Close();return nil,err}
		managed[id]=true
		if status=="ACTIVE"{ids=append(ids,id)}
	}
	rows.Close()
	for _,id:=range ids{
		s,loadErr:=a.loadPartnerPlan(ctx,id)
		if loadErr!=nil{return nil,loadErr}
		mode,modeErr:=a.ensureCommercialMode(ctx,id)
		if modeErr!=nil{return nil,modeErr}
		var syncErr error
		if mode.BillingMode==billingModeCharity && mode.CharityStatus==charityApproved{
			syncErr=a.syncCharityEntitlements(ctx,id)
		}else{
			syncErr=a.syncPlanEntitlements(ctx,id,s.PlanKey,at)
		}
		if syncErr!=nil{
			_ = a.emitBillingEvent(ctx,
				"PLAN_ENTITLEMENT_SYNC_FAILED:"+id+":"+at.Format("2006-01-02"),
				id,"","PLAN_ENTITLEMENT_SYNC_FAILED",time.Now().UTC(),
				map[string]any{"plan_key":s.PlanKey,"error":syncErr.Error()},
			)
		}
	}
	dueRows,err:=a.db.QueryContext(ctx,`SELECT partner_id FROM billing.partner_plan_subscriptions
		WHERE status='ACTIVE' AND next_billing_at<=$1 ORDER BY next_billing_at,partner_id`,at)
	if err!=nil{return nil,err}
	due:=[]string{};for dueRows.Next(){var id string;if err:=dueRows.Scan(&id);err!=nil{dueRows.Close();return nil,err};due=append(due,id)};dueRows.Close()
	for _,id:=range due{
		s,err:=a.loadPartnerPlan(ctx,id);if err!=nil{return nil,err}
		start:=dateOnly(s.NextBillingAt)
		p,err:=a.loadPlanAt(ctx,s.PlanKey,start);if err!=nil{return nil,err}
		monthly,list,annual,_,_:=a.effectivePlanPrices(s,p)
		end:=nextMonthStart(start);amount:=monthly;listPrice:=monthly;chargeType:="PLAN_MONTHLY"
		if s.BillingFrequency=="ANNUAL"{end=start.AddDate(1,0,0);amount=annual;listPrice=list;chargeType="PLAN_ANNUAL_RENEWAL"}
		mode,err:=a.ensureCommercialMode(ctx,id);if err!=nil{return nil,err}
		actualAmount:=amount
		if mode.BillingMode!=billingModePaid{actualAmount=0}
		invoiceID,grossCharge,err:=a.createPlanInvoice(ctx,id,p.Key,s.BillingFrequency,chargeType,start,end,listPrice,actualAmount);if err!=nil{return nil,err}
		if invoiceID!=""{a.queueInvoiceCollection(ctx,invoiceID,id,p.Currency,grossCharge)}
		next:=end
		if _,err=a.db.ExecContext(ctx,`UPDATE billing.partner_plan_subscriptions SET current_period_start=$2,current_period_end=$3,next_billing_at=$4,
			monthly_price_snapshot=$5,annual_list_price_snapshot=$6,annual_price_snapshot=$7,updated_at=NOW() WHERE partner_id=$1`,
			id,start,end,next,monthly,list,annual);err!=nil{return nil,err}
	}
	return managed,nil
}
