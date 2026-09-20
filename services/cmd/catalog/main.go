package main

import (
	"context"
	"database/sql"
	"himate.local/services/internal/common"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

type app struct{ db *sql.DB }
type seedModule struct{ Key, Label, Group string }

var seedModules = []seedModule{
	{"finance", "Balance Sheet", "finance_invoicing"},
	{"income_statement", "Income Statement", "finance_invoicing"},
	{"invoice_documents", "Invoices Documents", "finance_invoicing"},
	{"audit_log", "Audit Log", "technical"},
	{"backups", "Backups", "technical"},
	{"pianos", "Client Piano", "technical"},
	{"contacts", "Clients", "technical"},
	{"closed_jobs", "Closed Jobs", "technical"},
	{"knowledge_base", "Company Documents Archive", "technical"},
	{"company_data", "Corporate Data", "technical"},
	{"inventory", "Inventory", "technical"},
	{"partners", "Partners", "technical"},
	{"planned_jobs", "Planned Jobs", "technical"},
	{"scheduler", "Scheduler", "technical"},
	{"website_services", "Services", "technical"},
	{"settings", "Settings", "technical"},
	{"system_integrations", "System Activation & Integrations", "technical"},
	{"users", "Users", "technical"},
	{"workshop_workflow", "Workshop Workflow", "workshop"},
	{"marketing_overview", "Campaign Overview", "marketing"},
	{"customer_inbox", "Customer Inbox", "marketing"},
	{"website_reviews", "Reviews", "marketing"},
	{"campaigns_utm", "Campaigns & UTM", "marketing"},
	{"leads", "Leads", "marketing"},
	{"tracking_cookies", "Tracking & Cookies", "marketing"},
	{"seo_keywords", "SEO & Keywords", "marketing"},
	{"heatmap", "Consent Heatmap", "marketing"},
	{"website_artists", "Artists", "website_events"},
	{"website_contacts", "Contacts", "website_events"},
	{"digital_attendance", "Digital Attendance", "website_events"},
	{"events", "Events", "website_events"},
	{"event_guest_list", "Guest Data", "website_events"},
	{"event_invitations", "Invitations", "website_events"},
	{"media_library", "Media Library", "website_events"},
	{"pages_content", "Pages & Content", "website_events"},
	{"publish_preview", "Publish & Preview", "website_events"},
	{"showroom_pianos", "Showroom Pianos", "website_events"},
	{"event_tickets", "Ticket Reservation", "website_events"},
}

var moduleStates = map[string]bool{"ACTIVE": true, "NOT_LICENSED": true, "MAINTENANCE": true}
var availabilityValues = map[string]bool{"ACTIVE": true, "UNAVAILABLE": true, "DEPRECATED": true}
var moduleKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_.]{2,127}package main

import (
	"context"
	"database/sql"
	"himate.local/services/internal/common"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

type app struct{ db *sql.DB }
type seedModule struct{ Key, Label, Group string }

var seedModules = []seedModule{
	{"finance", "Balance Sheet", "finance_invoicing"},
	{"income_statement", "Income Statement", "finance_invoicing"},
	{"invoice_documents", "Invoices Documents", "finance_invoicing"},
	{"audit_log", "Audit Log", "technical"},
	{"backups", "Backups", "technical"},
	{"pianos", "Client Piano", "technical"},
	{"contacts", "Clients", "technical"},
	{"closed_jobs", "Closed Jobs", "technical"},
	{"knowledge_base", "Company Documents Archive", "technical"},
	{"company_data", "Corporate Data", "technical"},
	{"inventory", "Inventory", "technical"},
	{"partners", "Partners", "technical"},
	{"planned_jobs", "Planned Jobs", "technical"},
	{"scheduler", "Scheduler", "technical"},
	{"website_services", "Services", "technical"},
	{"settings", "Settings", "technical"},
	{"system_integrations", "System Activation & Integrations", "technical"},
	{"users", "Users", "technical"},
	{"workshop_workflow", "Workshop Workflow", "workshop"},
	{"marketing_overview", "Campaign Overview", "marketing"},
	{"customer_inbox", "Customer Inbox", "marketing"},
	{"website_reviews", "Reviews", "marketing"},
	{"campaigns_utm", "Campaigns & UTM", "marketing"},
	{"leads", "Leads", "marketing"},
	{"tracking_cookies", "Tracking & Cookies", "marketing"},
	{"seo_keywords", "SEO & Keywords", "marketing"},
	{"heatmap", "Consent Heatmap", "marketing"},
	{"website_artists", "Artists", "website_events"},
	{"website_contacts", "Contacts", "website_events"},
	{"digital_attendance", "Digital Attendance", "website_events"},
	{"events", "Events", "website_events"},
	{"event_guest_list", "Guest Data", "website_events"},
	{"event_invitations", "Invitations", "website_events"},
	{"media_library", "Media Library", "website_events"},
	{"pages_content", "Pages & Content", "website_events"},
	{"publish_preview", "Publish & Preview", "website_events"},
	{"showroom_pianos", "Showroom Pianos", "website_events"},
	{"event_tickets", "Ticket Reservation", "website_events"},
}

)

func main() {
	log := common.Logger()
	db, err := common.OpenDB()
	if err != nil {
		log.Error("database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	a := &app{db: db}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := a.migrate(ctx); err != nil {
		log.Error("migration", "error", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		common.JSON(w, 200, map[string]any{"status": "ok", "service": "catalog", "reference_modules": len(seedModules)})
	})
	mux.HandleFunc("/api/v1/module-groups", a.groups)
	mux.HandleFunc("/api/v1/modules", a.modules)
	mux.HandleFunc("/api/v1/modules/", a.moduleByKey)
	mux.HandleFunc("/api/v1/partners/", a.partnerModules)
	mux.HandleFunc("/internal/v1/partners/", a.partnerModules)
	mux.HandleFunc("/internal/v1/portfolio", a.portfolio)
	common.Run(log, "catalog", common.Env("PORT", "10000"), common.InternalAuth(os.Getenv("HIMATE_INTERNAL_TOKEN"), mux))
}

func (a *app) migrate(ctx context.Context) error {
	if err := common.ApplyMigrations(ctx, a.db, "catalog", []common.Migration{
		{Version: 1, Name: "catalog-base", Statements: []string{
			`CREATE SCHEMA IF NOT EXISTS catalog`,
			`CREATE TABLE IF NOT EXISTS catalog.module_groups(group_key TEXT PRIMARY KEY,label TEXT NOT NULL,sort_order INT NOT NULL)`,
			`CREATE TABLE IF NOT EXISTS catalog.modules(
				module_key TEXT PRIMARY KEY,label TEXT NOT NULL,group_key TEXT NOT NULL REFERENCES catalog.module_groups(group_key),
				description TEXT NOT NULL DEFAULT '',default_monthly_price NUMERIC(12,2) NOT NULL DEFAULT 0,
				currency TEXT NOT NULL DEFAULT 'USD',version TEXT NOT NULL DEFAULT '1.0.0',latest_version TEXT NOT NULL DEFAULT '1.0.0',
				last_updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),system BOOLEAN NOT NULL DEFAULT FALSE
			)`,
			`CREATE TABLE IF NOT EXISTS catalog.partner_modules(
				partner_id TEXT NOT NULL,module_key TEXT NOT NULL REFERENCES catalog.modules(module_key),
				status TEXT NOT NULL DEFAULT 'NOT_LICENSED',visible BOOLEAN NOT NULL DEFAULT FALSE,included_in_base BOOLEAN NOT NULL DEFAULT FALSE,
				price_override NUMERIC(12,2),updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),PRIMARY KEY(partner_id,module_key)
			)`,
			`CREATE TABLE IF NOT EXISTS catalog.price_history(
				id BIGSERIAL PRIMARY KEY,partner_id TEXT NOT NULL,module_key TEXT NOT NULL,old_price NUMERIC(12,2),new_price NUMERIC(12,2) NOT NULL,
				effective_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),reason TEXT NOT NULL DEFAULT ''
			)`,
		}},
		{Version: 2, Name: "availability-and-history", Statements: []string{
			`ALTER TABLE catalog.modules ADD COLUMN IF NOT EXISTS availability TEXT NOT NULL DEFAULT 'ACTIVE'`,
			`ALTER TABLE catalog.price_history ADD COLUMN IF NOT EXISTS actor TEXT NOT NULL DEFAULT ''`,
			`CREATE TABLE IF NOT EXISTS catalog.partner_module_history(
				id BIGSERIAL PRIMARY KEY,
				partner_id TEXT NOT NULL,
				module_key TEXT NOT NULL,
				field_name TEXT NOT NULL,
				old_value TEXT,
				new_value TEXT,
				effective_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				actor TEXT NOT NULL DEFAULT '',
				reason TEXT NOT NULL DEFAULT ''
			)`,
			`CREATE INDEX IF NOT EXISTS partner_module_history_lookup ON catalog.partner_module_history(partner_id,module_key,effective_at DESC)`,
			`CREATE INDEX IF NOT EXISTS partner_modules_status_idx ON catalog.partner_modules(partner_id,status)`,
		}},
	}); err != nil {
		return err
	}

	groups := []struct {
		Key, Label string
		Order      int
	}{
		{"workshop", "Workshop", 1}, {"finance_invoicing", "Finance & Invoicing", 2}, {"technical", "Technical Operations", 3},
		{"marketing", "Marketing", 4}, {"website_events", "Website & Events", 5}, {"communication", "Communication", 6},
	}
	for _, g := range groups {
		if _, err := a.db.ExecContext(ctx, `INSERT INTO catalog.module_groups(group_key,label,sort_order) VALUES($1,$2,$3) ON CONFLICT(group_key) DO UPDATE SET label=EXCLUDED.label,sort_order=EXCLUDED.sort_order`, g.Key, g.Label, g.Order); err != nil {
			return err
		}
	}
	for _, m := range seedModules {
		if _, err := a.db.ExecContext(ctx, `INSERT INTO catalog.modules(module_key,label,group_key,description,system,availability) VALUES($1,$2,$3,'Klavierhaus verified reference module',TRUE,'ACTIVE') ON CONFLICT(module_key) DO UPDATE SET label=EXCLUDED.label,group_key=EXCLUDED.group_key,system=TRUE`, m.Key, m.Label, m.Group); err != nil {
			return err
		}
		if _, err := a.db.ExecContext(ctx, `INSERT INTO catalog.partner_modules(partner_id,module_key,status,visible,included_in_base,price_override) VALUES('ptr_000001',$1,'ACTIVE',TRUE,TRUE,0) ON CONFLICT(partner_id,module_key) DO NOTHING`, m.Key); err != nil {
			return err
		}
	}
	return nil
}

func (a *app) groups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.APIError(w, 405, "METHOD", "Use GET")
		return
	}
	rows, err := a.db.Query(`SELECT group_key,label,sort_order FROM catalog.module_groups ORDER BY sort_order`)
	if err != nil {
		common.APIError(w, 500, "DB", "Could not load module groups")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var k, l string
		var s int
		if rows.Scan(&k, &l, &s) == nil {
			items = append(items, map[string]any{"group_key": k, "label": l, "sort_order": s})
		}
	}
	common.JSON(w, 200, map[string]any{"items": items})
}

func (a *app) modules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query(`SELECT m.module_key,m.label,m.group_key,g.label,m.description,m.default_monthly_price,m.currency,m.version,m.latest_version,m.last_updated_at,m.system,m.availability FROM catalog.modules m JOIN catalog.module_groups g ON g.group_key=m.group_key ORDER BY g.sort_order,m.label`)
		if err != nil {
			common.APIError(w, 500, "DB", "Could not load modules")
			return
		}
		defer rows.Close()
		items := []map[string]any{}
		for rows.Next() {
			var k, l, g, gl, d, currency, v, lv, availability string
			var p float64
			var t time.Time
			var sys bool
			if rows.Scan(&k, &l, &g, &gl, &d, &p, &currency, &v, &lv, &t, &sys, &availability) == nil {
				items = append(items, map[string]any{"key": k, "label": l, "group_key": g, "group_label": gl, "description": d, "default_monthly_price": p, "currency": currency, "version": v, "latest_version": lv, "last_updated_at": t, "system": sys, "availability": availability})
			}
		}
		common.JSON(w, 200, map[string]any{"items": items, "count": len(items)})
	case http.MethodPost:
		var in struct {
			Key                 string  `json:"key"`
			Label               string  `json:"label"`
			GroupKey            string  `json:"group_key"`
			Description         string  `json:"description"`
			Currency            string  `json:"currency"`
			Version             string  `json:"version"`
			LatestVersion       string  `json:"latest_version"`
			DefaultMonthlyPrice float64 `json:"default_monthly_price"`
			Availability        string  `json:"availability"`
		}
		if common.Decode(r, &in) != nil || !moduleKeyPattern.MatchString(in.Key) || strings.TrimSpace(in.Label) == "" || strings.TrimSpace(in.GroupKey) == "" {
			common.APIError(w, 400, "VALIDATION", "Stable key, label and group are required")
			return
		}
		if in.Currency == "" {
			in.Currency = "USD"
		}
		if in.Version == "" {
			in.Version = "1.0.0"
		}
		if in.LatestVersion == "" {
			in.LatestVersion = in.Version
		}
		if in.Availability == "" {
			in.Availability = "ACTIVE"
		}
		if !availabilityValues[in.Availability] {
			common.APIError(w, 400, "VALIDATION", "Invalid module availability")
			return
		}
		if in.DefaultMonthlyPrice < 0 {
			common.APIError(w, 400, "VALIDATION", "Price cannot be negative")
			return
		}
		_, err := a.db.Exec(`INSERT INTO catalog.modules(module_key,label,group_key,description,default_monthly_price,currency,version,latest_version,system,availability) VALUES($1,$2,$3,$4,$5,$6,$7,$8,FALSE,$9)`, in.Key, strings.TrimSpace(in.Label), in.GroupKey, in.Description, in.DefaultMonthlyPrice, in.Currency, in.Version, in.LatestVersion, in.Availability)
		if err != nil {
			common.APIError(w, 409, "CONFLICT", "Module could not be created")
			return
		}
		common.JSON(w, 201, map[string]any{"key": in.Key, "label": in.Label, "group_key": in.GroupKey, "default_monthly_price": in.DefaultMonthlyPrice, "currency": in.Currency, "version": in.Version, "latest_version": in.LatestVersion, "availability": in.Availability, "system": false})
	default:
		common.APIError(w, 405, "METHOD", "Use GET or POST")
	}
}

func (a *app) moduleByKey(w http.ResponseWriter, r *http.Request) {
	key := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/modules/"), "/")
	if key == "" || strings.Contains(key, "/") {
		common.APIError(w, 404, "NOT_FOUND", "Module not found")
		return
	}
	if r.Method != http.MethodPatch {
		common.APIError(w, 405, "METHOD", "Use PATCH")
		return
	}
	var in struct {
		Label               *string  `json:"label"`
		Description         *string  `json:"description"`
		GroupKey            *string  `json:"group_key"`
		DefaultMonthlyPrice *float64 `json:"default_monthly_price"`
		Availability        *string  `json:"availability"`
		LatestVersion       *string  `json:"latest_version"`
	}
	if common.Decode(r, &in) != nil {
		common.APIError(w, 400, "JSON", "Invalid request")
		return
	}
	var label, description, groupKey, availability, latestVersion string
	var price float64
	if err := a.db.QueryRow(`SELECT label,description,group_key,default_monthly_price,availability,latest_version FROM catalog.modules WHERE module_key=$1`, key).
		Scan(&label, &description, &groupKey, &price, &availability, &latestVersion); err != nil {
		common.APIError(w, 404, "NOT_FOUND", "Module not found")
		return
	}
	if in.Label != nil { label = strings.TrimSpace(*in.Label) }
	if in.Description != nil { description = strings.TrimSpace(*in.Description) }
	if in.GroupKey != nil { groupKey = strings.TrimSpace(*in.GroupKey) }
	if in.DefaultMonthlyPrice != nil { price = *in.DefaultMonthlyPrice }
	if in.Availability != nil { availability = strings.TrimSpace(*in.Availability) }
	if in.LatestVersion != nil { latestVersion = strings.TrimSpace(*in.LatestVersion) }
	if label == "" || price < 0 || !availabilityValues[availability] {
		common.APIError(w, 400, "VALIDATION", "Invalid module update")
		return
	}
	if _, err := a.db.Exec(`UPDATE catalog.modules SET label=$2,description=$3,group_key=$4,default_monthly_price=$5,availability=$6,latest_version=$7,last_updated_at=NOW() WHERE module_key=$1`,
		key, label, description, groupKey, price, availability, latestVersion); err != nil {
		common.APIError(w, 409, "CONFLICT", "Module could not be updated")
		return
	}
	common.JSON(w, 200, map[string]any{"key": key, "label": label, "description": description, "group_key": groupKey, "default_monthly_price": price, "availability": availability, "latest_version": latestVersion})
}


func (a *app) ensurePartnerModules(partnerID string) error {
	_, err := a.db.Exec(`INSERT INTO catalog.partner_modules(partner_id,module_key,status,visible,included_in_base) SELECT $1,module_key,'NOT_LICENSED',FALSE,FALSE FROM catalog.modules ON CONFLICT(partner_id,module_key) DO NOTHING`, partnerID)
	return err
}

func (a *app) partnerModules(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Path
	internal := strings.HasPrefix(raw, "/internal/v1/partners/")
	if internal {
		raw = strings.TrimPrefix(raw, "/internal/v1/partners/")
	} else {
		raw = strings.TrimPrefix(raw, "/api/v1/partners/")
	}
	parts := strings.Split(strings.Trim(raw, "/"), "/")
	if len(parts) < 2 {
		common.APIError(w, 404, "NOT_FOUND", "Route not found")
		return
	}
	partnerID := parts[0]
	if err := a.ensurePartnerModules(partnerID); err != nil {
		common.APIError(w, 500, "DB", "Could not initialize partner module state")
		return
	}
	if parts[1] == "billable-modules" {
		if !internal {
			common.APIError(w, 404, "NOT_FOUND", "Route not found")
			return
		}
		a.listPartnerModules(w, partnerID, true)
		return
	}
	if parts[1] != "modules" {
		common.APIError(w, 404, "NOT_FOUND", "Route not found")
		return
	}
	if len(parts) == 2 {
		if r.Method != http.MethodGet {
			common.APIError(w, 405, "METHOD", "Use GET")
			return
		}
		a.listPartnerModules(w, partnerID, false)
		return
	}
	if r.Method != http.MethodPatch {
		common.APIError(w, 405, "METHOD", "Use PATCH")
		return
	}
	key := parts[2]
	var in struct {
		Status         *string  `json:"status"`
		Visible        *bool    `json:"visible"`
		IncludedInBase *bool    `json:"included_in_base"`
		PartnerPrice     *float64 `json:"partner_price"`
		PriceEffectiveAt string   `json:"price_effective_at"`
		Reason           string   `json:"reason"`
	}
	if common.Decode(r, &in) != nil {
		common.APIError(w, 400, "JSON", "Invalid request")
		return
	}
	tx, err := a.db.Begin()
	if err != nil {
		common.APIError(w, 500, "DB", "Could not start update")
		return
	}
	defer tx.Rollback()
	actor := strings.TrimSpace(r.Header.Get("X-Himate-User-ID"))
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		reason = "HIMATE administrator update"
	}
	effectiveAt := time.Now().UTC()
	if strings.TrimSpace(in.PriceEffectiveAt) != "" {
		parsed, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(in.PriceEffectiveAt))
		if parseErr != nil {
			common.APIError(w, 400, "VALIDATION", "price_effective_at must be RFC3339")
			return
		}
		effectiveAt = parsed.UTC()
	}

	recordHistory := func(field string, oldValue, newValue any, at time.Time) error {
		return func() error {
			_, e := tx.Exec(`INSERT INTO catalog.partner_module_history(partner_id,module_key,field_name,old_value,new_value,effective_at,actor,reason) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`,
				partnerID, key, field, fmt.Sprint(oldValue), fmt.Sprint(newValue), at, actor, reason)
			return e
		}()
	}

	if in.Status != nil {
		if !moduleStates[*in.Status] {
			common.APIError(w, 400, "VALIDATION", "Invalid module state")
			return
		}
		var old string
		if err = tx.QueryRow(`SELECT status FROM catalog.partner_modules WHERE partner_id=$1 AND module_key=$2`, partnerID, key).Scan(&old); err != nil {
			common.APIError(w, 404, "NOT_FOUND", "Module not found")
			return
		}
		if old != *in.Status {
			if _, err = tx.Exec(`UPDATE catalog.partner_modules SET status=$3,updated_at=NOW() WHERE partner_id=$1 AND module_key=$2`, partnerID, key, *in.Status); err != nil {
				common.APIError(w, 500, "DB", "Could not update module state")
				return
			}
			if err = recordHistory("status", old, *in.Status, time.Now().UTC()); err != nil {
				common.APIError(w, 500, "DB", "Could not save module history")
				return
			}
		}
	}
	if in.Visible != nil {
		var old bool
		_ = tx.QueryRow(`SELECT visible FROM catalog.partner_modules WHERE partner_id=$1 AND module_key=$2`, partnerID, key).Scan(&old)
		if old != *in.Visible {
			if _, err = tx.Exec(`UPDATE catalog.partner_modules SET visible=$3,updated_at=NOW() WHERE partner_id=$1 AND module_key=$2`, partnerID, key, *in.Visible); err != nil {
				common.APIError(w, 500, "DB", "Could not update visibility")
				return
			}
			if err = recordHistory("visible", old, *in.Visible, time.Now().UTC()); err != nil { common.APIError(w, 500, "DB", "Could not save module history"); return }
		}
	}
	if in.IncludedInBase != nil {
		var old bool
		_ = tx.QueryRow(`SELECT included_in_base FROM catalog.partner_modules WHERE partner_id=$1 AND module_key=$2`, partnerID, key).Scan(&old)
		if old != *in.IncludedInBase {
			if _, err = tx.Exec(`UPDATE catalog.partner_modules SET included_in_base=$3,updated_at=NOW() WHERE partner_id=$1 AND module_key=$2`, partnerID, key, *in.IncludedInBase); err != nil {
				common.APIError(w, 500, "DB", "Could not update base package")
				return
			}
			if err = recordHistory("included_in_base", old, *in.IncludedInBase, time.Now().UTC()); err != nil { common.APIError(w, 500, "DB", "Could not save module history"); return }
		}
	}
	if in.PartnerPrice != nil {
		if *in.PartnerPrice < 0 {
			common.APIError(w, 400, "VALIDATION", "Price cannot be negative")
			return
		}
		var old sql.NullFloat64
		_ = tx.QueryRow(`SELECT price_override FROM catalog.partner_modules WHERE partner_id=$1 AND module_key=$2`, partnerID, key).Scan(&old)
		if _, err = tx.Exec(`UPDATE catalog.partner_modules SET price_override=$3,updated_at=NOW() WHERE partner_id=$1 AND module_key=$2`, partnerID, key, *in.PartnerPrice); err != nil {
			common.APIError(w, 500, "DB", "Could not update price")
			return
		}
		var oldValue any
		if old.Valid { oldValue = old.Float64 }
		if _, err = tx.Exec(`INSERT INTO catalog.price_history(partner_id,module_key,old_price,new_price,effective_at,actor,reason) VALUES($1,$2,$3,$4,$5,$6,$7)`, partnerID, key, oldValue, *in.PartnerPrice, effectiveAt, actor, reason); err != nil {
			common.APIError(w, 500, "DB", "Could not save price history")
			return
		}
		if err = recordHistory("partner_price", oldValue, *in.PartnerPrice, effectiveAt); err != nil { common.APIError(w, 500, "DB", "Could not save module history"); return }
	}
	if err = tx.Commit(); err != nil {
		common.APIError(w, 500, "DB", "Could not commit module update")
		return
	}
	a.onePartnerModule(w, partnerID, key)
}

func (a *app) listPartnerModules(w http.ResponseWriter, partnerID string, billable bool) {
	q := partnerModuleSelect + ` WHERE pm.partner_id=$1`
	if billable {
		q += ` AND pm.status='ACTIVE' AND m.availability='ACTIVE'`
	}
	q += ` ORDER BY g.sort_order,m.label`
	rows, err := a.db.Query(q, partnerID)
	if err != nil {
		common.APIError(w, 500, "DB", "Could not load partner modules")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	extra := 0.0
	for rows.Next() {
		item, err := scanPartnerModule(rows)
		if err != nil {
			continue
		}
		items = append(items, item)
		if billable && item["included_in_base"] != true {
			extra += item["partner_price"].(float64)
		}
	}
	out := map[string]any{"items": items, "count": len(items)}
	if billable {
		out["extra_monthly_total"] = extra
	}
	common.JSON(w, 200, out)
}

func (a *app) onePartnerModule(w http.ResponseWriter, partnerID, key string) {
	item, err := scanPartnerModule(a.db.QueryRow(partnerModuleSelect+` WHERE pm.partner_id=$1 AND pm.module_key=$2`, partnerID, key))
	if err != nil {
		common.APIError(w, 404, "NOT_FOUND", "Module not found")
		return
	}
	common.JSON(w, 200, item)
}

func (a *app) portfolio(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.APIError(w, 405, "METHOD", "Use GET")
		return
	}
	rows, err := a.db.Query(`
		SELECT pm.partner_id,
			COUNT(*) FILTER (WHERE pm.status='ACTIVE' AND m.availability='ACTIVE'),
			COALESCE(SUM(CASE WHEN pm.status='ACTIVE' AND m.availability='ACTIVE' AND pm.included_in_base=FALSE THEN COALESCE(pm.price_override,m.default_monthly_price) ELSE 0 END),0),
			MAX(pm.updated_at)
		FROM catalog.partner_modules pm
		JOIN catalog.modules m ON m.module_key=pm.module_key
		GROUP BY pm.partner_id
		ORDER BY pm.partner_id`)
	if err != nil {
		common.APIError(w, 500, "DB", "Could not load partner module portfolio")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id string
		var active int
		var extra float64
		var updated time.Time
		if rows.Scan(&id, &active, &extra, &updated) == nil {
			items = append(items, map[string]any{"partner_id": id, "active_modules": active, "extra_module_fee": extra, "updated_at": updated})
		}
	}
	common.JSON(w, 200, map[string]any{"items": items})
}


const partnerModuleSelect = `SELECT pm.partner_id,m.module_key,m.label,m.group_key,g.label,pm.status,pm.visible,pm.included_in_base,m.default_monthly_price,COALESCE(pm.price_override,m.default_monthly_price),m.currency,m.version,m.latest_version,m.last_updated_at,m.availability FROM catalog.partner_modules pm JOIN catalog.modules m ON m.module_key=pm.module_key JOIN catalog.module_groups g ON g.group_key=m.group_key`

type scanner interface{ Scan(...any) error }

func scanPartnerModule(s scanner) (map[string]any, error) {
	var id, k, l, g, gl, st, currency, v, lv, availability string
	var vis, inc bool
	var def, price float64
	var t time.Time
	err := s.Scan(&id, &k, &l, &g, &gl, &st, &vis, &inc, &def, &price, &currency, &v, &lv, &t, &availability)
	return map[string]any{"partner_id": id, "key": k, "label": l, "group_key": g, "group_label": gl, "status": st, "visible": vis, "included_in_base": inc, "default_monthly_price": def, "partner_price": price, "currency": currency, "version": v, "latest_version": lv, "last_updated_at": t, "availability": availability}, err
}
