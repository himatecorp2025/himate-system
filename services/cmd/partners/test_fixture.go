package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"himate.local/services/internal/common"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const testFixtureMarker = "HIMATE_GOLDEN_TEST_FIXTURE"

var sqlIdentifier = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
type fixtureReconcileWriter struct {
	header http.Header
	status int
}

func (w *fixtureReconcileWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *fixtureReconcileWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

func (w *fixtureReconcileWriter) Write(payload []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return len(payload), nil
}

func (a *app) reconcileGoldenTestFixtures(ctx context.Context) (int, error) {
	rows, err := a.db.QueryContext(ctx, `SELECT p.id
		FROM partners.partners p
		WHERE p.test_partner=TRUE
		  AND lower(trim(p.display_name))='test partner'
		  AND (
			(
				SELECT COUNT(DISTINCT date_trunc('month',i.service_period_start))
				FROM billing.invoices i
				WHERE i.partner_id=p.id AND i.source='TEST_FIXTURE'
			) < 6
			OR NOT EXISTS (
				SELECT 1 FROM billing.partner_plan_subscriptions s
				WHERE s.partner_id=p.id AND s.status='ACTIVE'
			)
		  )
		ORDER BY p.created_at,p.id`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	seeded := 0
	for _, id := range ids {
		seedCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		req, err := http.NewRequestWithContext(seedCtx, http.MethodPost, "http://partners.internal/seed-test-fixture", nil)
		if err != nil {
			cancel()
			return seeded, err
		}
		req.Header.Set("X-Himate-User-ID", "system:golden-fixture-reconciler")
		writer := &fixtureReconcileWriter{header: make(http.Header)}
		a.seedTestPartnerFixture(writer, req, id)
		cancel()
		if writer.status < 200 || writer.status >= 300 {
			return seeded, fmt.Errorf("golden fixture seed for %s returned HTTP %d", id, writer.status)
		}
		seeded++
	}
	return seeded, nil
}

func (a *app) runGoldenTestFixtureReconciler() {
	log := common.Logger()
	delay := 5 * time.Second
	for {
		timer := time.NewTimer(delay)
		<-timer.C

		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		seeded, err := a.reconcileGoldenTestFixtures(ctx)
		cancel()
		if err != nil {
			log.Warn("golden test fixture reconciliation", "error", err)
			delay = 30 * time.Second
			continue
		}
		if seeded > 0 {
			log.Info("golden test fixture reconciled", "partners", seeded)
		}
		delay = 5 * time.Minute
	}
}

func monthBoundsUTC(now time.Time, monthsAgo int) (time.Time, time.Time) {
	firstThisMonth := time.Date(now.UTC().Year(), now.UTC().Month(), 1, 0, 0, 0, 0, time.UTC)
	end := firstThisMonth.AddDate(0, -monthsAgo, 0)
	start := end.AddDate(0, -1, 0)
	return start, end
}

func (a *app) seedTestPartnerFixture(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use POST")
		return
	}
	p, err := a.get(id)
	if err != nil {
		common.APIError(w, http.StatusNotFound, "NOT_FOUND", "Partner not found")
		return
	}
	if !p.TestPartner {
		common.APIError(w, http.StatusForbidden, "TEST_PARTNER_ONLY", "QA fixture generation is allowed only for the Golden Test Partner")
		return
	}
	actor := strings.TrimSpace(r.Header.Get("X-Himate-User-ID"))
	if actor == "" {
		actor = "system"
	}

	tx, err := a.db.BeginTx(r.Context(), &sql.TxOptions{})
	if err != nil {
		common.APIError(w, 500, "DB", "Could not start Test Partner fixture transaction")
		return
	}
	defer tx.Rollback()

	// Keep the stable partner identity while supplying realistic, clearly fictional
	// master data for a six-month piano-service company scenario.
	_, err = tx.ExecContext(r.Context(), `UPDATE partners.partners SET
		legal_name='Test Partner Piano Services LLC',
		brand_name='Test Partner Piano Services',
		primary_domain='test-partner.example.invalid',
		staging_domain='staging.test-partner.example.invalid',
		contact_name='QA Operations',
		contact_email='qa@test-partner.example.invalid',
		finance_contact_name='QA Finance',
		finance_contact_email='finance@test-partner.example.invalid',
		technical_contact_name='QA Technical',
		technical_contact_email='technical@test-partner.example.invalid',
		marketing_contact_name='QA Marketing',
		marketing_contact_email='marketing@test-partner.example.invalid',
		registration_number='QA-TEST-000001',
		tax_id='TEST-TAX-ID',
		country='US',
		state_region='NY',
		city='New York',
		postal_code='10001',
		address_line1='100 Test Piano Avenue',
		address_line2='QA Suite',
		website='https://test-partner.example.invalid',
		phone='+1-212-555-0100',
		notes=$2,
		lifecycle='LIVE',
		system_health='HEALTHY',
		health_checked_at=NOW(),
		last_sync_at=NOW(),
		updated_at=NOW()
		WHERE id=$1 AND test_partner=TRUE`, id,
		testFixtureMarker+" · Fictional QA data only. Six-month piano-service operating history.")
	if err != nil {
		common.APIError(w, 500, "DB", "Could not seed Test Partner master data")
		return
	}

	// Golden Test Partner gets all legacy/system modules. This mirrors the catalog
	// QA entitlement rule and keeps the fixture deterministic even after a reset.
	_, err = tx.ExecContext(r.Context(), `INSERT INTO catalog.partner_modules(
		partner_id,module_key,status,visible,included_in_base,updated_at,activated_at,
		entitlement_state,commercial_configured,contract_currency,quote_reference,commercial_effective_at
	)
	SELECT $1,m.module_key,'ACTIVE',TRUE,TRUE,NOW(),COALESCE(pm.activated_at,NOW()-INTERVAL '6 months'),
		'ACTIVE',TRUE,'USD','GOLDEN-TEST-PARTNER',COALESCE(pm.commercial_effective_at,NOW()-INTERVAL '6 months')
	FROM catalog.modules m
	LEFT JOIN catalog.partner_modules pm ON pm.partner_id=$1 AND pm.module_key=m.module_key
	WHERE m.system=TRUE
	ON CONFLICT(partner_id,module_key) DO UPDATE SET
		status='ACTIVE',visible=TRUE,included_in_base=TRUE,entitlement_state='ACTIVE',
		commercial_configured=TRUE,contract_currency='USD',quote_reference='GOLDEN-TEST-PARTNER',
		activated_at=COALESCE(catalog.partner_modules.activated_at,EXCLUDED.activated_at),
		commercial_effective_at=COALESCE(catalog.partner_modules.commercial_effective_at,EXCLUDED.commercial_effective_at),
		updated_at=NOW()`, id)
	if err != nil {
		common.APIError(w, 500, "DB", "Could not seed Test Partner module entitlements")
		return
	}

	_, err = tx.ExecContext(r.Context(), `INSERT INTO billing.partner_terms(
		partner_id,currency,activation_fee,activation_fee_waived,activation_fee_reason,
		base_monthly_fee,annual_increase_percent,cycle_days,invoice_day,price_effective_from,service_anchor_date,updated_at
	) VALUES($1,'USD',13000,FALSE,'',1850,5,30,1,(CURRENT_DATE-INTERVAL '6 months')::date,(CURRENT_DATE-INTERVAL '6 months')::date,NOW())
	ON CONFLICT(partner_id) DO UPDATE SET
		currency='USD',activation_fee=13000,activation_fee_waived=FALSE,
		base_monthly_fee=1850,annual_increase_percent=5,cycle_days=30,invoice_day=1,
		price_effective_from=(CURRENT_DATE-INTERVAL '6 months')::date,
		service_anchor_date=(CURRENT_DATE-INTERVAL '6 months')::date,updated_at=NOW()`, id)
	if err != nil {
		common.APIError(w, 500, "DB", "Could not seed Test Partner commercial terms")
		return
	}

	// Keep a real recurring subscription state alongside the six completed
	// monthly billing periods. CUSTOM isolates the Golden Test Partner from the
	// production Starter/Business/Premium package analytics while exercising the
	// same subscription lifecycle and workspace read models.
	_, err = tx.ExecContext(r.Context(), `INSERT INTO billing.partner_plan_subscriptions(
		partner_id,plan_key,billing_frequency,status,current_period_start,current_period_end,next_billing_at,
		monthly_price_snapshot,annual_list_price_snapshot,annual_price_snapshot,
		next_plan_key,next_billing_frequency,change_effective_at,
		custom_monthly_price,custom_annual_list_price,custom_annual_price,custom_module_limit,custom_selection_mode,updated_at
	) VALUES(
		$1,'CUSTOM','MONTHLY','ACTIVE',
		date_trunc('month',CURRENT_DATE)::date,
		(date_trunc('month',CURRENT_DATE)+INTERVAL '1 month')::date,
		(date_trunc('month',CURRENT_DATE)+INTERVAL '1 month')::date,
		1850,22200,22200,'','',NULL,1850,22200,22200,0,'CUSTOM',NOW()
	)
	ON CONFLICT(partner_id) DO UPDATE SET
		plan_key='CUSTOM',billing_frequency='MONTHLY',status='ACTIVE',
		current_period_start=EXCLUDED.current_period_start,current_period_end=EXCLUDED.current_period_end,
		next_billing_at=EXCLUDED.next_billing_at,monthly_price_snapshot=1850,
		annual_list_price_snapshot=22200,annual_price_snapshot=22200,
		next_plan_key='',next_billing_frequency='',change_effective_at=NULL,
		custom_monthly_price=1850,custom_annual_list_price=22200,custom_annual_price=22200,
		custom_module_limit=0,custom_selection_mode='CUSTOM',updated_at=NOW()`, id)
	if err != nil {
		common.APIError(w, 500, "DB", "Could not seed Test Partner recurring subscription")
		return
	}
	_, err = tx.ExecContext(r.Context(), `INSERT INTO billing.plan_change_history(
		partner_id,old_plan_key,new_plan_key,old_billing_frequency,new_billing_frequency,
		change_type,effective_at,upgrade_charge,actor,reason
	)
	SELECT $1,'','CUSTOM','','MONTHLY','TEST_FIXTURE_STARTED',
		(CURRENT_DATE-INTERVAL '6 months')::date,0,$2,$3
	WHERE NOT EXISTS (
		SELECT 1 FROM billing.plan_change_history h
		WHERE h.partner_id=$1 AND h.change_type='TEST_FIXTURE_STARTED'
	)`, id, actor, testFixtureMarker+" · fictional subscription started six months ago")
	if err != nil {
		common.APIError(w, 500, "DB", "Could not seed Test Partner subscription history")
		return
	}

	_, err = tx.ExecContext(r.Context(), `INSERT INTO billing.initial_licenses(
		partner_id,currency,required_amount,paid_amount,status,payment_date,payment_reference,verified_by,note,waived,waiver_reason,updated_at
	) VALUES($1,'USD',13000,13000,'PAID',(CURRENT_DATE-INTERVAL '6 months')::date,'TEST-LICENSE-PAID',$2,$3,FALSE,'',NOW())
	ON CONFLICT(partner_id) DO UPDATE SET
		currency='USD',required_amount=13000,paid_amount=13000,status='PAID',
		payment_date=(CURRENT_DATE-INTERVAL '6 months')::date,payment_reference='TEST-LICENSE-PAID',
		verified_by=$2,note=$3,waived=FALSE,waiver_reason='',updated_at=NOW()`,
		id, actor, testFixtureMarker+" · fictional paid activation license")
	if err != nil {
		common.APIError(w, 500, "DB", "Could not seed Test Partner license")
		return
	}

	type monthFixture struct {
		Revenue         float64
		PianosServiced  float64
		TechnicianHours float64
		CustomerJobs    float64
	}
	fixtures := []monthFixture{
		{6120, 31, 118, 24},
		{6840, 36, 132, 28},
		{7310, 39, 146, 31},
		{7890, 43, 158, 34},
		{8425, 47, 171, 38},
		{9180, 52, 189, 42},
	}

	metrics := []struct {
		Key, LabelEN, LabelHU, DescriptionEN, DescriptionHU, Unit string
	}{
		{"qa.piano.pianos_serviced", "Pianos serviced", "Szervizelt zongorák", "Completed tuning, regulation or repair jobs.", "Befejezett hangolási, szabályozási vagy javítási munkák.", "count"},
		{"qa.piano.technician_hours", "Technician hours", "Technikusi munkaórák", "Recorded technician work hours.", "Rögzített technikusi munkaórák.", "hours"},
		{"qa.piano.customer_jobs", "Customer service jobs", "Ügyfélmunkák", "Completed customer-facing service jobs.", "Befejezett ügyféloldali szolgáltatási munkák.", "count"},
	}
	for _, metric := range metrics {
		_, err = tx.ExecContext(r.Context(), `INSERT INTO impact.metric_definitions(
			metric_key,label,label_en,label_hu,description,description_en,description_hu,unit,aggregation,scope,active,system
		) VALUES($1,$2,$2,$3,$4,$4,$5,$6,'SUM','PARTNER',TRUE,FALSE)
		ON CONFLICT(metric_key) DO UPDATE SET
			label=EXCLUDED.label,label_en=EXCLUDED.label_en,label_hu=EXCLUDED.label_hu,
			description=EXCLUDED.description,description_en=EXCLUDED.description_en,description_hu=EXCLUDED.description_hu,
			unit=EXCLUDED.unit,aggregation='SUM',scope='PARTNER',active=TRUE,updated_at=NOW()`,
			metric.Key, metric.LabelEN, metric.LabelHU, metric.DescriptionEN, metric.DescriptionHU, metric.Unit)
		if err != nil {
			common.APIError(w, 500, "DB", "Could not seed Test Partner impact definitions")
			return
		}
	}

	now := time.Now().UTC()
	for i, fixture := range fixtures {
		monthsAgo := len(fixtures) - 1 - i
		start, end := monthBoundsUTC(now, monthsAgo)
		invoiceID := fmt.Sprintf("inv_test_fixture_%s_%s", strings.ReplaceAll(id, "_", ""), start.Format("200601"))
		invoiceKey := fmt.Sprintf("TEST_FIXTURE:%s:%s", id, start.Format("2006-01"))
		_, err = tx.ExecContext(r.Context(), `INSERT INTO billing.invoices(
			id,invoice_key,partner_id,invoice_date,service_period_start,service_period_end,currency,
			base_fee,module_fee,total,status,provider_status,plan_key,billing_frequency,charge_type,
			list_price,discount_amount,minimum_commitment_adjustment,billing_model,net_total,tax_rate_percent,tax_amount,
			workflow_status,source,paid_at,provider,provider_payment_id,notes,created_at
		) VALUES($1,$2,$3,$4,$4,$5,'USD',$6,0,$6,'PAID','SUCCEEDED','TEST','MONTHLY','MANUAL',
			$6,0,0,'MANUAL',$6,0,0,'PAID','TEST_FIXTURE',$7,'TEST_FIXTURE',$2,$8,$7)
		ON CONFLICT(invoice_key) DO UPDATE SET
			total=EXCLUDED.total,base_fee=EXCLUDED.base_fee,net_total=EXCLUDED.net_total,
			status='PAID',provider_status='SUCCEEDED',workflow_status='PAID',paid_at=EXCLUDED.paid_at,
			provider='TEST_FIXTURE',provider_payment_id=EXCLUDED.provider_payment_id,notes=EXCLUDED.notes`,
			invoiceID, invoiceKey, id, start, end, fixture.Revenue, end.Add(-24*time.Hour),
			testFixtureMarker+" · fictional monthly piano-service revenue")
		if err != nil {
			common.APIError(w, 500, "DB", "Could not seed Test Partner invoice history")
			return
		}
		_, err = tx.ExecContext(r.Context(), `INSERT INTO billing.invoice_items(
			item_key,invoice_id,partner_id,item_type,description,currency,quantity,unit_price,amount,
			period_start,period_end,status,invoiced_at,billing_model
		) VALUES($1,$2,$3,'MANUAL','Fictional piano tuning, regulation and repair services','USD',1,$4,$4,$5,$6,'INVOICED',$7,'MANUAL')
		ON CONFLICT(item_key) DO UPDATE SET amount=EXCLUDED.amount,unit_price=EXCLUDED.unit_price`,
			"TEST_FIXTURE_ITEM:"+invoiceID, invoiceID, id, fixture.Revenue, start, end, end.Add(-48*time.Hour))
		if err != nil {
			common.APIError(w, 500, "DB", "Could not seed Test Partner invoice items")
			return
		}
		_, _ = tx.ExecContext(r.Context(), `INSERT INTO billing.finance_transactions(
			partner_id,invoice_id,transaction_type,status,currency,net_amount,tax_amount,gross_amount,source,actor,reason,occurred_at
		) VALUES($1,$2,'INVOICE','PAID','USD',$3,0,$3,'TEST_FIXTURE',$4,$5,$6)
		ON CONFLICT DO NOTHING`, id, invoiceID, fixture.Revenue, actor,
			testFixtureMarker+" · fictional settled monthly invoice", end.Add(-24*time.Hour))

		metricValues := []struct {
			Key string
			Val float64
		}{
			{"qa.piano.pianos_serviced", fixture.PianosServiced},
			{"qa.piano.technician_hours", fixture.TechnicianHours},
			{"qa.piano.customer_jobs", fixture.CustomerJobs},
		}
		for _, mv := range metricValues {
			key := fmt.Sprintf("TEST_FIXTURE:%s:%s:%s", id, mv.Key, start.Format("2006-01"))
			meta, _ := json.Marshal(map[string]any{
				"fixture": true,
				"scenario": "six_month_piano_service_company",
				"marker": testFixtureMarker,
			})
			_, err = tx.ExecContext(r.Context(), `INSERT INTO impact.metric_values(
				idempotency_key,payload_hash,partner_id,metric_key,period_start,period_end,numeric_value,text_value,
				provenance,source_ref,recorded_by,recorded_at,metadata
			) VALUES($1,$1,$2,$3,$4,$5,$6,'','TEST_FIXTURE',$1,$7,$8,$9::jsonb)
			ON CONFLICT(idempotency_key) DO UPDATE SET
				numeric_value=EXCLUDED.numeric_value,period_start=EXCLUDED.period_start,period_end=EXCLUDED.period_end,
				recorded_at=EXCLUDED.recorded_at,metadata=EXCLUDED.metadata`,
				key, id, mv.Key, start, end.AddDate(0, 0, -1), mv.Val, actor, end.Add(-36*time.Hour), string(meta))
			if err != nil {
				common.APIError(w, 500, "DB", "Could not seed Test Partner impact history")
				return
			}
		}

		evidenceID := fmt.Sprintf("evd_test_%s_%s", strings.ReplaceAll(id, "_", ""), start.Format("200601"))
		_, err = tx.ExecContext(r.Context(), `INSERT INTO evidence.items(
			id,partner_id,metric_key,evidence_type,title,description,period_start,period_end,
			verification_status,source_url,declaration_text,uploaded_by,verified_by,verified_at,created_at,updated_at
		) VALUES($1,$2,'qa.piano.pianos_serviced','PARTNER_DECLARATION',$3,$4,$5,$6,
			'VERIFIED','',$7,$8,$8,$9,$9,$9)
		ON CONFLICT(id) DO UPDATE SET
			title=EXCLUDED.title,description=EXCLUDED.description,period_start=EXCLUDED.period_start,
			period_end=EXCLUDED.period_end,verification_status='VERIFIED',
			declaration_text=EXCLUDED.declaration_text,verified_by=EXCLUDED.verified_by,verified_at=EXCLUDED.verified_at,updated_at=EXCLUDED.updated_at`,
			evidenceID, id,
			fmt.Sprintf("QA service completion declaration · %s", start.Format("2006-01")),
			testFixtureMarker+" · Fictional evidence for UI/report testing only.",
			start, end.AddDate(0, 0, -1),
			fmt.Sprintf("Fictional QA declaration: %.0f pianos serviced, %.0f technician hours and %.0f customer jobs during this period. Not real business evidence.",
				fixture.PianosServiced, fixture.TechnicianHours, fixture.CustomerJobs),
			actor, end.Add(-30*time.Hour))
		if err != nil {
			common.APIError(w, 500, "DB", "Could not seed Test Partner evidence")
			return
		}
	}

	// A queued report uses the normal report worker and therefore tests the real
	// Impact -> Evidence -> Reports -> Storage pipeline rather than a fake PDF.
	firstStart, _ := monthBoundsUTC(now, len(fixtures)-1)
	_, lastEnd := monthBoundsUTC(now, 0)
	reportID := "rpt_test_fixture_" + strings.ReplaceAll(id, "_", "")
	partnerIDs, _ := json.Marshal([]string{id})
	_, err = tx.ExecContext(r.Context(), `INSERT INTO reports.jobs(
		id,report_type,title,partner_ids,period_start,period_end,status,requested_by,last_error,created_at,updated_at
	) VALUES($1,'PARTNER_IMPACT','Test Partner · Six-month Piano Service Impact Report',$2::jsonb,$3,$4,'QUEUED',$5,'',NOW(),NOW())
	ON CONFLICT(id) DO UPDATE SET
		report_type='PARTNER_IMPACT',title=EXCLUDED.title,partner_ids=EXCLUDED.partner_ids,
		period_start=EXCLUDED.period_start,period_end=EXCLUDED.period_end,status='QUEUED',
		snapshot='{}'::jsonb,evidence_ids='[]'::jsonb,pdf_namespace='',pdf_object_key='',pdf_sha256='',pdf_size_bytes=0,
		requested_by=EXCLUDED.requested_by,last_error='',started_at=NULL,completed_at=NULL,updated_at=NOW()`,
		reportID, string(partnerIDs), firstStart, lastEnd.AddDate(0, 0, -1), actor)
	if err != nil {
		common.APIError(w, 500, "DB", "Could not queue Test Partner report")
		return
	}

	if err = tx.Commit(); err != nil {
		common.APIError(w, 500, "DB", "Could not commit Test Partner fixture")
		return
	}

	common.JSON(w, http.StatusOK, map[string]any{
		"partner_id": id,
		"fixture": testFixtureMarker,
		"months": len(fixtures),
		"report_id": reportID,
		"report_status": "QUEUED",
		"aggregate_isolation": true,
		"message": "Golden Test Partner fixture generated. All values are fictional QA data.",
	})
}

type purgeTable struct {
	Schema string
	Table  string
}

func quotedTable(t purgeTable) (string, error) {
	if !sqlIdentifier.MatchString(t.Schema) || !sqlIdentifier.MatchString(t.Table) {
		return "", fmt.Errorf("invalid SQL identifier")
	}
	return fmt.Sprintf(`"%s"."%s"`, t.Schema, t.Table), nil
}

func testPartnerDependentTables(ctx context.Context, tx *sql.Tx) ([]purgeTable, error) {
	rows, err := tx.QueryContext(ctx, `SELECT table_schema,table_name
		FROM information_schema.columns
		WHERE column_name='partner_id'
		  AND table_schema NOT IN ('pg_catalog','information_schema','compliance')
		  AND NOT (table_schema='partners' AND table_name='partners')
		  AND NOT (table_schema='identity' AND table_name='audit_events')
		GROUP BY table_schema,table_name
		ORDER BY table_schema,table_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []purgeTable{}
	for rows.Next() {
		var t purgeTable
		if err := rows.Scan(&t.Schema, &t.Table); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func deleteTestPartnerData(ctx context.Context, tx *sql.Tx, partnerID string) (map[string]int64, error) {
	deleted := map[string]int64{}

	// Child/link tables whose ownership is indirect.
	// automation.deliveries has no partner_id and RESTRICTs deletion of
	// automation.events, so remove partner-owned deliveries before the generic
	// partner_id sweep reaches automation.events.
	if res, err := tx.ExecContext(ctx, `DELETE FROM automation.deliveries
		WHERE event_id IN (SELECT id FROM automation.events WHERE partner_id=$1)`, partnerID); err != nil {
		return nil, err
	} else {
		n, _ := res.RowsAffected()
		deleted["automation.deliveries"] += n
	}
	if res, err := tx.ExecContext(ctx, `DELETE FROM evidence.report_links
		WHERE evidence_id IN (SELECT id FROM evidence.items WHERE partner_id=$1)
		   OR report_id IN (SELECT id FROM reports.jobs WHERE partner_ids @> $2::jsonb)`,
		partnerID, fmt.Sprintf(`["%s"]`, partnerID)); err == nil {
		n, _ := res.RowsAffected()
		deleted["evidence.report_links"] += n
	}
	if res, err := tx.ExecContext(ctx, `DELETE FROM reports.jobs WHERE partner_ids @> $1::jsonb`,
		fmt.Sprintf(`["%s"]`, partnerID)); err != nil {
		return nil, err
	} else {
		n, _ := res.RowsAffected()
		deleted["reports.jobs"] += n
	}

	tables, err := testPartnerDependentTables(ctx, tx)
	if err != nil {
		return nil, err
	}
	pending := append([]purgeTable{}, tables...)
	for pass := 0; pass < 6 && len(pending) > 0; pass++ {
		next := []purgeTable{}
		progress := false
		for i, table := range pending {
			name, err := quotedTable(table)
			if err != nil {
				return nil, err
			}
			savepoint := fmt.Sprintf("test_purge_%d_%d", pass, i)
			if _, err := tx.ExecContext(ctx, "SAVEPOINT "+savepoint); err != nil {
				return nil, err
			}
			res, deleteErr := tx.ExecContext(ctx, "DELETE FROM "+name+" WHERE partner_id=$1", partnerID)
			if deleteErr != nil {
				_, _ = tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT "+savepoint)
				_, _ = tx.ExecContext(ctx, "RELEASE SAVEPOINT "+savepoint)
				next = append(next, table)
				continue
			}
			_, _ = tx.ExecContext(ctx, "RELEASE SAVEPOINT "+savepoint)
			n, _ := res.RowsAffected()
			deleted[table.Schema+"."+table.Table] += n
			progress = true
		}
		if len(next) == 0 {
			pending = nil
			break
		}
		if !progress {
			names := make([]string, 0, len(next))
			for _, t := range next {
				names = append(names, t.Schema+"."+t.Table)
			}
			return nil, fmt.Errorf("could not purge dependent tables: %s", strings.Join(names, ", "))
		}
		pending = next
	}

	if len(pending) > 0 {
		return nil, fmt.Errorf("test partner purge did not converge")
	}
	return deleted, nil
}

func (a *app) purgeTestReportObjects(ctx context.Context, partnerID string) error {
	rows, err := a.db.QueryContext(ctx, `SELECT pdf_namespace,pdf_object_key
		FROM reports.jobs
		WHERE partner_ids @> $1::jsonb AND pdf_namespace<>'' AND pdf_object_key<>''`,
		fmt.Sprintf(`["%s"]`, partnerID))
	if err != nil {
		return err
	}
	defer rows.Close()
	type objectRef struct{ Namespace, Key string }
	objects := []objectRef{}
	for rows.Next() {
		var ref objectRef
		if err := rows.Scan(&ref.Namespace, &ref.Key); err != nil {
			return err
		}
		objects = append(objects, ref)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, ref := range objects {
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete,
			"http://"+a.storageHost+"/internal/v1/storage/objects/"+url.PathEscape(ref.Namespace)+"/"+ref.Key, nil)
		if err != nil {
			return err
		}
		common.BindInternalRequest(req, a.token)
		resp, err := common.DoInternal(a.client, req)
		if resp != nil {
			resp.Body.Close()
		}
		if err != nil {
			return err
		}
		if resp.StatusCode >= 300 && resp.StatusCode != http.StatusNotFound {
			return fmt.Errorf("report storage purge returned HTTP %d", resp.StatusCode)
		}
	}
	return nil
}

func (a *app) purgeTestStorage(ctx context.Context, partnerID string) error {
	if strings.TrimSpace(a.storageHost) == "" {
		return fmt.Errorf("storage service is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"http://"+a.storageHost+"/internal/v1/storage/partners/"+partnerID+"/purge-test", nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Himate-Test-Partner-Confirm", partnerID)
	common.BindInternalRequest(req, a.token)
	resp, err := common.DoInternal(a.client, req)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("storage purge returned HTTP %d", resp.StatusCode)
	}
	return nil
}

func (a *app) purgeTestPartner(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use POST")
		return
	}
	p, err := a.get(id)
	if err != nil {
		common.APIError(w, http.StatusNotFound, "NOT_FOUND", "Partner not found")
		return
	}
	if !p.TestPartner {
		common.APIError(w, http.StatusForbidden, "TEST_PARTNER_ONLY", "Hard purge is allowed only for a Golden Test Partner")
		return
	}
	var in struct {
		ConfirmPartnerID string `json:"confirm_partner_id"`
	}
	if common.Decode(r, &in) != nil || strings.TrimSpace(in.ConfirmPartnerID) != id {
		common.APIError(w, http.StatusBadRequest, "CONFIRMATION_REQUIRED", "confirm_partner_id must exactly match the Test Partner ID")
		return
	}

	var complianceLocked bool
	err = a.db.QueryRowContext(r.Context(), `SELECT EXISTS(
		SELECT 1 FROM compliance.partner_archives WHERE partner_id=$1 AND retain_until>NOW()
	)`, id).Scan(&complianceLocked)
	if err != nil && err != sql.ErrNoRows {
		common.APIError(w, 500, "DB", "Could not verify Compliance Archive retention")
		return
	}
	if complianceLocked {
		common.APIError(w, http.StatusLocked, "COMPLIANCE_RETENTION", "Test Partner hard purge is blocked because a legal Compliance Archive exists")
		return
	}
	var reportBusy bool
	reportArray := fmt.Sprintf(`["%s"]`, id)
	if err := a.db.QueryRowContext(r.Context(), `SELECT EXISTS(
		SELECT 1 FROM reports.jobs
		WHERE partner_ids @> $1::jsonb AND status IN ('QUEUED','RUNNING')
	)`, reportArray).Scan(&reportBusy); err != nil {
		common.APIError(w, 500, "DB", "Could not verify Test Partner report processing")
		return
	}
	if reportBusy {
		common.APIError(w, http.StatusConflict, "TEST_REPORT_IN_PROGRESS", "Wait for the Test Partner report worker to finish before factory reset")
		return
	}

	// Files live on the storage service's persistent disk and must be removed
	// before the relational identity disappears. Reports use the shared _reports
	// namespace, so remove their exact objects before deleting the report rows.
	if err := a.purgeTestReportObjects(r.Context(), id); err != nil {
		common.APIError(w, http.StatusBadGateway, "REPORT_STORAGE_PURGE", err.Error())
		return
	}
	if err := a.purgeTestStorage(r.Context(), id); err != nil {
		common.APIError(w, http.StatusBadGateway, "STORAGE_PURGE", err.Error())
		return
	}

	tx, err := a.db.BeginTx(r.Context(), &sql.TxOptions{})
	if err != nil {
		common.APIError(w, 500, "DB", "Could not start Test Partner hard purge")
		return
	}
	defer tx.Rollback()

	// Transaction-local capability consumed only by immutable-ledger triggers.
	// The endpoint has already proven that this exact partner is a Golden Test
	// Partner and that no Compliance Archive blocks deletion.
	if _, err := tx.ExecContext(r.Context(),
		`SELECT set_config('himate.test_partner_purge',$1,TRUE)`, id); err != nil {
		common.APIError(w, 500, "DB", "Could not enable transaction-scoped Test Partner purge")
		return
	}

	deleted, err := deleteTestPartnerData(r.Context(), tx, id)
	if err != nil {
		common.APIError(w, 500, "TEST_PURGE", err.Error())
		return
	}
	res, err := tx.ExecContext(r.Context(), `DELETE FROM partners.partners WHERE id=$1 AND test_partner=TRUE`, id)
	if err != nil {
		common.APIError(w, 500, "DB", "Could not delete Golden Test Partner")
		return
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		common.APIError(w, 409, "TEST_PURGE", "Golden Test Partner identity was not deleted")
		return
	}
	deleted["partners.partners"] = n

	if err = tx.Commit(); err != nil {
		common.APIError(w, 500, "DB", "Could not commit Test Partner hard purge")
		return
	}
	common.JSON(w, http.StatusOK, map[string]any{
		"partner_id": id,
		"hard_purged": true,
		"compliance_archive_created": false,
		"deleted_rows": deleted,
		"audit_receipt_preserved": true,
		"message": "Golden Test Partner and its test data were permanently removed.",
	})
}
