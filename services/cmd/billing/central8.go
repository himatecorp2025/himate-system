package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"himate.local/services/internal/common"
)

func central8BillingMigration() common.Migration {
	return common.Migration{
		Version: 19,
		Name:    "central-8-package-canonical-baseline-and-analytics",
		Statements: []string{
			`UPDATE billing.subscription_plans SET
				display_name='Starter',monthly_price=990,annual_list_price=11880,annual_price=11880,
				module_limit=10,selection_mode='FIXED',customer_selectable=TRUE,sort_order=10,updated_at=NOW()
				WHERE plan_key='STARTER'`,
			`UPDATE billing.subscription_plans SET
				display_name='Business',monthly_price=1490,annual_list_price=17880,annual_price=16390,
				module_limit=20,selection_mode='FIXED',customer_selectable=TRUE,sort_order=20,updated_at=NOW()
				WHERE plan_key='BUSINESS'`,
			`UPDATE billing.subscription_plans SET
				display_name='Premium',monthly_price=2490,annual_list_price=29880,annual_price=22410,
				module_limit=0,selection_mode='UNLIMITED',customer_selectable=TRUE,sort_order=30,updated_at=NOW()
				WHERE plan_key='FLEX'`,
			`CREATE INDEX IF NOT EXISTS billing_partner_plan_active_analytics_idx
				ON billing.partner_plan_subscriptions(plan_key,status,billing_frequency)`,
		},
	}
}

type central8PackagePartner struct {
	PartnerID                string
	DisplayName              string
	LegalName                string
	Country                  string
	PlanKey                  string
	PlanName                 string
	BillingFrequency         string
	Status                   string
	CurrentPeriodStart       time.Time
	CurrentPeriodEnd         time.Time
	NextBillingAt            time.Time
	Classification           string
	OnboardingState          string
	QuoteReference           string
	MinimumMonthlyCommitment float64
	ModuleEvents7D           int64
	ModuleEvents30D          int64
	FirstModuleUse           sql.NullTime
	LastModuleUse            sql.NullTime
	PortalActiveBuckets30D   int64
	PortalRequests30D        int64
	PortalFirstSeen30D       sql.NullTime
	PortalLastSeen30D        sql.NullTime
}

func central8PlanName(key string) string {
	switch strings.ToUpper(strings.TrimSpace(key)) {
	case "STARTER":
		return "Starter"
	case "BUSINESS":
		return "Business"
	case "FLEX":
		return "Premium"
	default:
		return strings.TrimSpace(key)
	}
}

func central8NullTime(v sql.NullTime) any {
	if !v.Valid {
		return nil
	}
	return v.Time.UTC()
}

func (a *app) loadCentral8PackagePartners(ctx context.Context) ([]central8PackagePartner, bool, error) {
	rows, err := a.db.QueryContext(ctx, `SELECT
		s.partner_id,
		COALESCE(p.display_name,s.partner_id),
		COALESCE(p.legal_name,''),
		COALESCE(p.country,''),
		s.plan_key,
		s.billing_frequency,
		s.status,
		s.current_period_start,
		s.current_period_end,
		s.next_billing_at,
		COALESCE(o.classification,'UNCLASSIFIED'),
		COALESCE(o.state,''),
		COALESCE(t.quote_reference,''),
		COALESCE(t.minimum_monthly_commitment,0),
		COALESCE(u.events_7d,0),
		COALESCE(u.events_30d,0),
		u.first_use,
		u.last_use
	FROM billing.partner_plan_subscriptions s
	LEFT JOIN partners.partners p ON p.id=s.partner_id
	LEFT JOIN billing.partner_onboarding o ON o.partner_id=s.partner_id
	LEFT JOIN billing.partner_terms t ON t.partner_id=s.partner_id
	LEFT JOIN LATERAL (
		SELECT
			COUNT(*) FILTER (WHERE occurred_at>=NOW()-INTERVAL '7 days') AS events_7d,
			COUNT(*) FILTER (WHERE occurred_at>=NOW()-INTERVAL '30 days') AS events_30d,
			MIN(occurred_at) AS first_use,
			MAX(occurred_at) AS last_use
		FROM catalog.module_usage_events
		WHERE partner_id=s.partner_id
	) u ON TRUE
	WHERE s.plan_key IN ('STARTER','BUSINESS','FLEX')
	ORDER BY lower(COALESCE(p.display_name,s.partner_id)),s.partner_id`)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	out := []central8PackagePartner{}
	for rows.Next() {
		var row central8PackagePartner
		if err := rows.Scan(
			&row.PartnerID, &row.DisplayName, &row.LegalName, &row.Country,
			&row.PlanKey, &row.BillingFrequency, &row.Status,
			&row.CurrentPeriodStart, &row.CurrentPeriodEnd, &row.NextBillingAt,
			&row.Classification, &row.OnboardingState, &row.QuoteReference,
			&row.MinimumMonthlyCommitment, &row.ModuleEvents7D, &row.ModuleEvents30D,
			&row.FirstModuleUse, &row.LastModuleUse,
		); err != nil {
			return nil, false, err
		}
		row.PlanName = central8PlanName(row.PlanKey)
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}

	var activityAvailable bool
	if err := a.db.QueryRowContext(ctx, `SELECT to_regclass('identity.partner_portal_activity_buckets') IS NOT NULL`).Scan(&activityAvailable); err != nil {
		activityAvailable = false
	}
	if !activityAvailable || len(out) == 0 {
		return out, activityAvailable, nil
	}

	activityRows, err := a.db.QueryContext(ctx, `SELECT
		partner_id,
		COUNT(DISTINCT bucket_start) FILTER (WHERE bucket_start>=NOW()-INTERVAL '30 days'),
		COALESCE(SUM(request_count) FILTER (WHERE bucket_start>=NOW()-INTERVAL '30 days'),0),
		MIN(first_seen_at) FILTER (WHERE bucket_start>=NOW()-INTERVAL '30 days'),
		MAX(last_seen_at) FILTER (WHERE bucket_start>=NOW()-INTERVAL '30 days')
	FROM identity.partner_portal_activity_buckets
	WHERE bucket_start>=NOW()-INTERVAL '30 days'
	GROUP BY partner_id`)
	if err != nil {
		return out, false, nil
	}
	defer activityRows.Close()
	activity := map[string]struct {
		Buckets, Requests int64
		First, Last       sql.NullTime
	}{}
	for activityRows.Next() {
		var id string
		var buckets, requests int64
		var first, last sql.NullTime
		if activityRows.Scan(&id, &buckets, &requests, &first, &last) == nil {
			activity[id] = struct {
				Buckets, Requests int64
				First, Last       sql.NullTime
			}{buckets, requests, first, last}
		}
	}
	for i := range out {
		if item, ok := activity[out[i].PartnerID]; ok {
			out[i].PortalActiveBuckets30D = item.Buckets
			out[i].PortalRequests30D = item.Requests
			out[i].PortalFirstSeen30D = item.First
			out[i].PortalLastSeen30D = item.Last
		}
	}
	return out, true, nil
}

func central8PackagePartnerMap(row central8PackagePartner, activityMeasured bool) map[string]any {
	return map[string]any{
		"partner_id": row.PartnerID,
		"display_name": row.DisplayName,
		"legal_name": row.LegalName,
		"country": row.Country,
		"plan_key": row.PlanKey,
		"plan_name": row.PlanName,
		"billing_frequency": row.BillingFrequency,
		"status": row.Status,
		"current_period_start": row.CurrentPeriodStart,
		"current_period_end_exclusive": row.CurrentPeriodEnd,
		"next_billing_at": row.NextBillingAt,
		"classification": row.Classification,
		"onboarding_state": row.OnboardingState,
		"quote_reference": row.QuoteReference,
		"minimum_monthly_commitment": row.MinimumMonthlyCommitment,
		"module_usage_events_7d": row.ModuleEvents7D,
		"module_usage_events_30d": row.ModuleEvents30D,
		"first_module_use": central8NullTime(row.FirstModuleUse),
		"last_module_use": central8NullTime(row.LastModuleUse),
		"portal_activity_measured": activityMeasured,
		"portal_active_hours_30d": float64(row.PortalActiveBuckets30D) * 5.0 / 60.0,
		"portal_requests_30d": row.PortalRequests30D,
		"portal_first_seen_30d": central8NullTime(row.PortalFirstSeen30D),
		"portal_last_seen_30d": central8NullTime(row.PortalLastSeen30D),
	}
}

func (a *app) packageAnalytics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use GET")
		return
	}
	partners, activityMeasured, err := a.loadCentral8PackagePartners(r.Context())
	if err != nil {
		common.APIError(w, 500, "DB", "Could not load package analytics")
		return
	}
	type aggregate struct {
		PlanKey, PlanName string
		ActivePartners    int
		MonthlyPartners   int
		AnnualPartners    int
		ModuleEvents30D   int64
		PortalHours30D    float64
	}
	byPlan := map[string]*aggregate{
		"STARTER":  {PlanKey: "STARTER", PlanName: "Starter"},
		"BUSINESS": {PlanKey: "BUSINESS", PlanName: "Business"},
		"FLEX":     {PlanKey: "FLEX", PlanName: "Premium"},
	}
	partnerItems := make([]map[string]any, 0, len(partners))
	for _, row := range partners {
		partnerItems = append(partnerItems, central8PackagePartnerMap(row, activityMeasured))
		aRow := byPlan[row.PlanKey]
		if aRow == nil {
			continue
		}
		if row.Status == "ACTIVE" {
			aRow.ActivePartners++
		}
		if row.BillingFrequency == "MONTHLY" {
			aRow.MonthlyPartners++
		}
		if row.BillingFrequency == "ANNUAL" {
			aRow.AnnualPartners++
		}
		aRow.ModuleEvents30D += row.ModuleEvents30D
		aRow.PortalHours30D += float64(row.PortalActiveBuckets30D) * 5.0 / 60.0
	}
	packageItems := []map[string]any{}
	for _, key := range []string{"STARTER", "BUSINESS", "FLEX"} {
		row := byPlan[key]
		packageItems = append(packageItems, map[string]any{
			"plan_key": row.PlanKey,
			"display_name": row.PlanName,
			"active_partner_count": row.ActivePartners,
			"monthly_partner_count": row.MonthlyPartners,
			"annual_partner_count": row.AnnualPartners,
			"module_usage_events_30d": row.ModuleEvents30D,
			"portal_active_hours_30d": row.PortalHours30D,
		})
	}
	common.JSON(w, 200, map[string]any{
		"packages": packageItems,
		"partners": partnerItems,
		"partner_count": len(partnerItems),
		"portal_activity_measured": activityMeasured,
		"portal_activity_definition": "5-minute buckets with at least one authenticated Partner Portal request",
		"source": "BILLING_SUBSCRIPTIONS_CATALOG_USAGE_PORTAL_ACTIVITY",
	})
}

func central9PlanPrice(planKey string) (string, string) {
	switch strings.ToUpper(strings.TrimSpace(planKey)) {
	case "STARTER":
		return "$990 + VAT", "10 modules"
	case "BUSINESS":
		return "$1,490 + VAT", "20 modules"
	case "FLEX":
		return "$2,490 + VAT", "Unlimited"
	default:
		return "-", "-"
	}
}
func (a *app) packageExportPDF(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use GET")
		return
	}
	rows, activityMeasured, err := a.loadCentral8PackagePartners(r.Context())
	if err != nil {
		common.APIError(w, 500, "DB", "Could not export package analytics")
		return
	}
	tableRows := make([][]string, 0, len(rows))
	for _, row := range rows {
		price, entitlement := central9PlanPrice(row.PlanKey)
		portalHours := "-"
		if activityMeasured {
			portalHours = fmt.Sprintf("%.1f h", float64(row.PortalActiveBuckets30D)*5.0/60.0)
		}
		tableRows = append(tableRows, []string{
			row.DisplayName,
			row.PlanName,
			price,
			entitlement,
			row.BillingFrequency,
			row.Status,
			fmt.Sprintf("%d", row.ModuleEvents30D),
			portalHours,
			row.NextBillingAt.Format("2006-01-02"),
		})
	}
	common.WriteBrandedTablePDF(
		w,
		"himate-package-analytics.pdf",
		"HiMate Central - Packages",
		"Package portfolio, usage and commercial analytics",
		[]string{"Partner", "Package", "Price", "Entitlement", "Billing", "Status", "Module uses / 30d", "Portal / 30d", "Next billing"},
		tableRows,
	)
}
func (a *app) financeExportPDF(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use GET")
		return
	}
	status := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("status")))
	plan := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("plan_key")))
	query := `SELECT id,partner_id,invoice_date,currency,
		COALESCE(plan_key,''),COALESCE(billing_frequency,''),COALESCE(charge_type,''),
		workflow_status,COALESCE(net_total,total),COALESCE(tax_rate_percent,0),COALESCE(tax_amount,0),total,
		paid_at
		FROM billing.invoices WHERE 1=1`
	args := []any{}
	if status != "" && status != "ALL" {
		args = append(args, status)
		query += fmt.Sprintf(" AND workflow_status=$%d", len(args))
	}
	if plan != "" && plan != "ALL" {
		args = append(args, plan)
		query += fmt.Sprintf(" AND plan_key=$%d", len(args))
	}
	query += " ORDER BY invoice_date DESC,id DESC"
	rows, err := a.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		common.APIError(w, 500, "DB", "Could not export finance data")
		return
	}
	defer rows.Close()

	tableRows := make([][]string, 0, 64)
	for rows.Next() {
		var id, partnerID, currency, planKey, frequency, chargeType, workflow string
		var invoiceDate time.Time
		var paidAt sql.NullTime
		var net, taxRate, tax, gross float64
		if rows.Scan(&id, &partnerID, &invoiceDate, &currency, &planKey, &frequency,
			&chargeType, &workflow, &net, &taxRate, &tax, &gross, &paidAt) != nil {
			continue
		}
		paid := "-"
		if paidAt.Valid {
			paid = paidAt.Time.UTC().Format("2006-01-02")
		}
		tableRows = append(tableRows, []string{
			id, partnerID, invoiceDate.Format("2006-01-02"), central8PlanName(planKey),
			chargeType, workflow, currency + " " + fmt.Sprintf("%.2f", net),
			fmt.Sprintf("%.2f%%", taxRate), currency + " " + fmt.Sprintf("%.2f", gross), paid,
		})
	}
	common.WriteBrandedTablePDF(
		w,
		"himate-finance.pdf",
		"HiMate Central - Finance",
		"Filtered invoice and paid-revenue ledger export",
		[]string{"Invoice", "Partner", "Date", "Package", "Charge", "Status", "Net", "VAT", "Gross", "Paid"},
		tableRows,
	)
}
func (a *app) central8RevenueTrends(ctx context.Context) (weekly, monthlyByPlan, weeklyByPlan []map[string]any, err error) {
	weeklyRows, err := a.db.QueryContext(ctx, `
		WITH periods AS (
			SELECT generate_series(date_trunc('week',NOW())-INTERVAL '3 weeks',date_trunc('week',NOW()),INTERVAL '1 week') AS period
		), currencies AS (
			SELECT currency FROM billing.invoices GROUP BY currency
			UNION SELECT 'USD'
		)
		SELECT to_char(p.period,'YYYY-MM-DD'),c.currency,COALESCE(SUM(i.total),0)
		FROM periods p CROSS JOIN currencies c
		LEFT JOIN billing.invoices i ON i.workflow_status='PAID' AND i.currency=c.currency
			AND date_trunc('week',COALESCE(i.paid_at,i.created_at))=p.period
		GROUP BY p.period,c.currency ORDER BY c.currency,p.period`)
	if err != nil {
		return nil, nil, nil, err
	}
	for weeklyRows.Next() {
		var period, currency string
		var amount float64
		if weeklyRows.Scan(&period, &currency, &amount) == nil {
			weekly = append(weekly, map[string]any{"period": period, "currency": currency, "paid": amount})
		}
	}
	weeklyRows.Close()

	loadByPlan := func(monthly bool) ([]map[string]any, error) {
		series := "date_trunc('month',NOW())-INTERVAL '11 months'"
		step := "1 month"
		trunc := "month"
		format := "YYYY-MM"
		if !monthly {
			series = "date_trunc('week',NOW())-INTERVAL '3 weeks'"
			step = "1 week"
			trunc = "week"
			format = "YYYY-MM-DD"
		}
		query := fmt.Sprintf(`
			WITH periods AS (
				SELECT generate_series(%s,date_trunc('%s',NOW()),INTERVAL '%s') AS period
			), plans(plan_key,display_name) AS (
				VALUES ('STARTER','Starter'),('BUSINESS','Business'),('FLEX','Premium')
			), currencies AS (
				SELECT currency FROM billing.invoices GROUP BY currency
				UNION SELECT 'USD'
			)
			SELECT to_char(p.period,'%s'),c.currency,pl.plan_key,pl.display_name,COALESCE(SUM(i.total),0)
			FROM periods p CROSS JOIN plans pl CROSS JOIN currencies c
			LEFT JOIN billing.invoices i ON i.workflow_status='PAID'
				AND i.currency=c.currency AND i.plan_key=pl.plan_key
				AND date_trunc('%s',COALESCE(i.paid_at,i.created_at))=p.period
			GROUP BY p.period,c.currency,pl.plan_key,pl.display_name
			ORDER BY c.currency,pl.plan_key,p.period`, series, trunc, step, format, trunc)
		rows, err := a.db.QueryContext(ctx, query)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var period, currency, planKey, displayName string
			var amount float64
			if rows.Scan(&period, &currency, &planKey, &displayName, &amount) == nil {
				out = append(out, map[string]any{
					"period": period, "currency": currency, "plan_key": planKey,
					"display_name": displayName, "paid": amount,
				})
			}
		}
		return out, rows.Err()
	}
	monthlyByPlan, err = loadByPlan(true)
	if err != nil {
		return nil, nil, nil, err
	}
	weeklyByPlan, err = loadByPlan(false)
	if err != nil {
		return nil, nil, nil, err
	}
	return weekly, monthlyByPlan, weeklyByPlan, nil
}
