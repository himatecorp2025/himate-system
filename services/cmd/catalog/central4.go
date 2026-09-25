package main

import (
	"context"
	"himate.local/services/internal/common"
	"net/http"
	"strings"
	"time"
)

func central4CatalogMigration() common.Migration {
	return common.Migration{
		Version: 10,
		Name:    "central-4-topic-registry-and-module-usage",
		Statements: []string{
			`CREATE TABLE IF NOT EXISTS catalog.module_usage_events(
				id BIGSERIAL PRIMARY KEY,
				partner_id TEXT NOT NULL,
				user_id TEXT NOT NULL,
				module_key TEXT NOT NULL REFERENCES catalog.modules(module_key) ON DELETE CASCADE,
				http_method TEXT NOT NULL,
				runtime_path TEXT NOT NULL,
				occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE INDEX IF NOT EXISTS module_usage_events_module_time_idx ON catalog.module_usage_events(module_key,occurred_at DESC)`,
			`CREATE INDEX IF NOT EXISTS module_usage_events_partner_module_time_idx ON catalog.module_usage_events(partner_id,module_key,occurred_at DESC)`,
			`INSERT INTO catalog.module_groups(group_key,label,label_en,label_hu,sort_order,is_primary_navigation)
			 VALUES
			  ('client_operations','Client & Operations','Client & Operations','Ügyfél- és működéskezelés',2,TRUE),
			  ('security_system','Security & System','Security & System','Biztonság és rendszer',5,TRUE)
			 ON CONFLICT(group_key) DO UPDATE SET
			  label=EXCLUDED.label,label_en=EXCLUDED.label_en,label_hu=EXCLUDED.label_hu,
			  sort_order=EXCLUDED.sort_order,is_primary_navigation=TRUE`,
			`UPDATE catalog.module_groups SET label='Finance & Invoicing',label_en='Finance & Invoicing',label_hu='Pénzügy és számlázás',sort_order=1,is_primary_navigation=TRUE WHERE group_key='finance_invoicing'`,
			`UPDATE catalog.module_groups SET label='Marketing',label_en='Marketing',label_hu='Marketing',sort_order=3,is_primary_navigation=TRUE WHERE group_key='marketing'`,
			`UPDATE catalog.module_groups SET label='Website & Events',label_en='Website & Events',label_hu='Weboldal és események',sort_order=4,is_primary_navigation=TRUE WHERE group_key='website_events'`,
			`UPDATE catalog.module_groups SET is_primary_navigation=FALSE WHERE group_key='technical'`,
		},
	}
}

func (a *app) moduleUsageEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use POST")
		return
	}
	var in struct {
		PartnerID   string `json:"partner_id"`
		UserID      string `json:"user_id"`
		ModuleKey   string `json:"module_key"`
		HTTPMethod  string `json:"http_method"`
		RuntimePath string `json:"runtime_path"`
	}
	if common.Decode(r, &in) != nil {
		common.APIError(w, http.StatusBadRequest, "JSON", "Invalid module usage event")
		return
	}
	in.PartnerID = strings.TrimSpace(in.PartnerID)
	in.UserID = strings.TrimSpace(in.UserID)
	in.ModuleKey = strings.TrimSpace(in.ModuleKey)
	in.HTTPMethod = strings.ToUpper(strings.TrimSpace(in.HTTPMethod))
	in.RuntimePath = strings.TrimSpace(in.RuntimePath)
	if in.PartnerID == "" || in.UserID == "" || !moduleKeyPattern.MatchString(in.ModuleKey) || in.RuntimePath == "" {
		common.APIError(w, http.StatusBadRequest, "VALIDATION", "Partner, user, module and runtime path are required")
		return
	}
	switch in.HTTPMethod {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
	default:
		common.APIError(w, http.StatusBadRequest, "VALIDATION", "Unsupported runtime method")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 1500*time.Millisecond)
	defer cancel()
	if _, err := a.db.ExecContext(ctx, `INSERT INTO catalog.module_usage_events(partner_id,user_id,module_key,http_method,runtime_path)
		VALUES($1,$2,$3,$4,$5)`, in.PartnerID, in.UserID, in.ModuleKey, in.HTTPMethod, in.RuntimePath); err != nil {
		common.APIError(w, http.StatusConflict, "MODULE_USAGE_EVENT", "Module usage event could not be recorded")
		return
	}
	common.JSON(w, http.StatusAccepted, map[string]any{
		"recorded": true,
		"module_key": in.ModuleKey,
		"partner_id": in.PartnerID,
	})
}
