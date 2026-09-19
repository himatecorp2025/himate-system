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
	{"workshop_workflow", "Workshop Workflow", "technical"},
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
var moduleKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{2,63}$`)

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
	mux.HandleFunc("/api/v1/partners/", a.partnerModules)
	mux.HandleFunc("/internal/v1/partners/", a.partnerModules)
	common.Run(log, "catalog", common.Env("PORT", "10000"), common.InternalAuth(os.Getenv("HIMATE_INTERNAL_TOKEN"), mux))
}

func (a *app) migrate(ctx context.Context) error {
	if err := common.ExecStatements(ctx, a.db,
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
	); err != nil {
		return err
	}

	groups := []struct {
		Key, Label string
		Order      int
	}{
		{"finance_invoicing", "Finance & Invoicing", 1}, {"technical", "Technical Operation", 2},
		{"marketing", "Marketing", 3}, {"website_events", "Website & Events", 4},
	}
	for _, g := range groups {
		if _, err := a.db.ExecContext(ctx, `INSERT INTO catalog.module_groups(group_key,label,sort_order) VALUES($1,$2,$3) ON CONFLICT(group_key) DO UPDATE SET label=EXCLUDED.label,sort_order=EXCLUDED.sort_order`, g.Key, g.Label, g.Order); err != nil {
			return err
		}
	}
	for _, m := range seedModules {
		if _, err := a.db.ExecContext(ctx, `INSERT INTO catalog.modules(module_key,label,group_key,description,system) VALUES($1,$2,$3,'Klavierhaus verified reference module',TRUE) ON CONFLICT(module_key) DO UPDATE SET label=EXCLUDED.label,group_key=EXCLUDED.group_key,system=TRUE`, m.Key, m.Label, m.Group); err != nil {
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
		rows, err := a.db.Query(`SELECT m.module_key,m.label,m.group_key,g.label,m.description,m.default_monthly_price,m.currency,m.version,m.latest_version,m.last_updated_at,m.system FROM catalog.modules m JOIN catalog.module_groups g ON g.group_key=m.group_key ORDER BY g.sort_order,m.label`)
		if err != nil {
			common.APIError(w, 500, "DB", "Could not load modules")
			return
		}
		defer rows.Close()
		items := []map[string]any{}
		for rows.Next() {
			var k, l, g, gl, d, c, v, lv string
			var p float64
			var t time.Time
			var sys bool
			if rows.Scan(&k, &l, &g, &gl, &d, &p, &c, &v, &lv, &t, &sys) == nil {
				items = append(items, map[string]any{"key": k, "label": l, "group_key": g, "group_label": gl, "description": d, "default_monthly_price": p, "currency": c, "version": v, "latest_version": lv, "last_updated_at": t, "system": sys})
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
		if in.DefaultMonthlyPrice < 0 {
			common.APIError(w, 400, "VALIDATION", "Price cannot be negative")
			return
		}
		_, err := a.db.Exec(`INSERT INTO catalog.modules(module_key,label,group_key,description,default_monthly_price,currency,version,latest_version,system) VALUES($1,$2,$3,$4,$5,$6,$7,$8,FALSE)`, in.Key, strings.TrimSpace(in.Label), in.GroupKey, in.Description, in.DefaultMonthlyPrice, in.Currency, in.Version, in.LatestVersion)
		if err != nil {
			common.APIError(w, 409, "CONFLICT", "Module could not be created")
			return
		}
		common.JSON(w, 201, map[string]any{"key": in.Key, "label": in.Label, "group_key": in.GroupKey, "default_monthly_price": in.DefaultMonthlyPrice, "currency": in.Currency, "version": in.Version, "latest_version": in.LatestVersion, "system": false})
	default:
		common.APIError(w, 405, "METHOD", "Use GET or POST")
	}
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
		PartnerPrice   *float64 `json:"partner_price"`
		Reason         string   `json:"reason"`
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
	if in.Status != nil {
		if !moduleStates[*in.Status] {
			common.APIError(w, 400, "VALIDATION", "Invalid module state")
			return
		}
		if _, err = tx.Exec(`UPDATE catalog.partner_modules SET status=$3,updated_at=NOW() WHERE partner_id=$1 AND module_key=$2`, partnerID, key, *in.Status); err != nil {
			common.APIError(w, 500, "DB", "Could not update module state")
			return
		}
	}
	if in.Visible != nil {
		if _, err = tx.Exec(`UPDATE catalog.partner_modules SET visible=$3,updated_at=NOW() WHERE partner_id=$1 AND module_key=$2`, partnerID, key, *in.Visible); err != nil {
			common.APIError(w, 500, "DB", "Could not update visibility")
			return
		}
	}
	if in.IncludedInBase != nil {
		if _, err = tx.Exec(`UPDATE catalog.partner_modules SET included_in_base=$3,updated_at=NOW() WHERE partner_id=$1 AND module_key=$2`, partnerID, key, *in.IncludedInBase); err != nil {
			common.APIError(w, 500, "DB", "Could not update base package")
			return
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
		if old.Valid {
			oldValue = old.Float64
		}
		if _, err = tx.Exec(`INSERT INTO catalog.price_history(partner_id,module_key,old_price,new_price,reason) VALUES($1,$2,$3,$4,$5)`, partnerID, key, oldValue, *in.PartnerPrice, in.Reason); err != nil {
			common.APIError(w, 500, "DB", "Could not save price history")
			return
		}
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
		q += ` AND pm.status='ACTIVE'`
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

const partnerModuleSelect = `SELECT pm.partner_id,m.module_key,m.label,m.group_key,g.label,pm.status,pm.visible,pm.included_in_base,m.default_monthly_price,COALESCE(pm.price_override,m.default_monthly_price),m.currency,m.version,m.latest_version,m.last_updated_at FROM catalog.partner_modules pm JOIN catalog.modules m ON m.module_key=pm.module_key JOIN catalog.module_groups g ON g.group_key=m.group_key`

type scanner interface{ Scan(...any) error }

func scanPartnerModule(s scanner) (map[string]any, error) {
	var id, k, l, g, gl, st, c, v, lv string
	var vis, inc bool
	var def, price float64
	var t time.Time
	err := s.Scan(&id, &k, &l, &g, &gl, &st, &vis, &inc, &def, &price, &c, &v, &lv, &t)
	return map[string]any{"partner_id": id, "key": k, "label": l, "group_key": g, "group_label": gl, "status": st, "visible": vis, "included_in_base": inc, "default_monthly_price": def, "partner_price": price, "currency": c, "version": v, "latest_version": lv, "last_updated_at": t}, err
}
