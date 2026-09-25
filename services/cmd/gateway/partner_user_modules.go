package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"himate.local/services/internal/common"
)

const (
	partnerModuleAccessAllOwned = "ALL_OWNED"
	partnerModuleAccessSelected = "SELECTED"
	partnerInvoiceModuleKey     = "invoice_documents"
)

func partnerUserModulePermissionsMigration() common.Migration {
	return common.Migration{
		Version: 13,
		Name:    "start-23-11-5-user-module-permissions",
		Statements: []string{
			`ALTER TABLE identity.partner_users ADD COLUMN IF NOT EXISTS module_access_mode TEXT NOT NULL DEFAULT 'ALL_OWNED'`,
			`UPDATE identity.partner_users SET module_access_mode='ALL_OWNED' WHERE module_access_mode NOT IN ('ALL_OWNED','SELECTED') OR module_access_mode=''`,
			`CREATE TABLE IF NOT EXISTS identity.partner_user_modules(
				partner_id TEXT NOT NULL,
				user_id TEXT NOT NULL REFERENCES identity.partner_users(id) ON DELETE CASCADE,
				module_key TEXT NOT NULL,
				granted_by TEXT NOT NULL DEFAULT '',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				PRIMARY KEY(user_id,module_key)
			)`,
			`CREATE INDEX IF NOT EXISTS identity_partner_user_modules_partner_idx ON identity.partner_user_modules(partner_id,user_id,module_key)`,
		},
	}
}

type partnerUserModulePolicy struct {
	Mode        string
	Selected    map[string]bool
	Owned       map[string]map[string]any
	OwnedKeys   []string
	Effective   []string
	Stale       []string
}

func normalizePartnerModuleAccessMode(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == partnerModuleAccessSelected {
		return partnerModuleAccessSelected
	}
	return partnerModuleAccessAllOwned
}

func (a *app) partnerUserModuleSelection(ctx context.Context, partnerID, userID string) (string, map[string]bool, error) {
	var mode string
	err := a.db.QueryRowContext(ctx,
		`SELECT module_access_mode FROM identity.partner_users WHERE id=$1 AND partner_id=$2`,
		userID, partnerID,
	).Scan(&mode)
	if err != nil {
		return "", nil, err
	}
	selected := map[string]bool{}
	rows, err := a.db.QueryContext(ctx,
		`SELECT module_key FROM identity.partner_user_modules WHERE partner_id=$1 AND user_id=$2 ORDER BY module_key`,
		partnerID, userID,
	)
	if err != nil {
		return "", nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return "", nil, err
		}
		key = strings.TrimSpace(key)
		if key != "" {
			selected[key] = true
		}
	}
	return normalizePartnerModuleAccessMode(mode), selected, rows.Err()
}

func (a *app) partnerOwnedModuleMap(ctx context.Context, partnerID, locale string) (map[string]map[string]any, []string, error) {
	var catalog map[string]any
	path := "/internal/v1/partner-portal/" + url.PathEscape(partnerID) + "/modules?locale=" + url.QueryEscape(locale)
	if err := a.internalGET(ctx, a.hosts["catalog"], path, &catalog); err != nil {
		return nil, nil, err
	}
	owned := map[string]map[string]any{}
	keys := []string{}
	for _, item := range anyItems(catalog["items"]) {
		key := strings.TrimSpace(fmt.Sprint(item["key"]))
		if key == "" {
			continue
		}
		active := strings.ToUpper(strings.TrimSpace(fmt.Sprint(item["access_state"]))) == "ACTIVE"
		executable := item["executable"] == true
		if active && executable {
			owned[key] = item
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return owned, keys, nil
}

func (a *app) loadPartnerUserModulePolicy(ctx context.Context, partnerID, userID, locale string) (partnerUserModulePolicy, error) {
	var mode string
	var selected map[string]bool
	var owned map[string]map[string]any
	var ownedKeys []string
	var selectionErr, ownedErr error

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		mode, selected, selectionErr = a.partnerUserModuleSelection(ctx, partnerID, userID)
	}()
	go func() {
		defer wg.Done()
		owned, ownedKeys, ownedErr = a.partnerOwnedModuleMap(ctx, partnerID, locale)
	}()
	wg.Wait()
	if selectionErr != nil {
		return partnerUserModulePolicy{}, selectionErr
	}
	if ownedErr != nil {
		return partnerUserModulePolicy{}, ownedErr
	}

	effective := []string{}
	stale := []string{}
	if mode == partnerModuleAccessAllOwned {
		effective = append(effective, ownedKeys...)
	} else {
		for key := range selected {
			if _, ok := owned[key]; ok {
				effective = append(effective, key)
			} else {
				stale = append(stale, key)
			}
		}
		sort.Strings(effective)
		sort.Strings(stale)
	}
	return partnerUserModulePolicy{
		Mode: mode, Selected: selected, Owned: owned, OwnedKeys: ownedKeys, Effective: effective, Stale: stale,
	}, nil
}

func (a *app) requirePartnerModuleExecution(w http.ResponseWriter, r *http.Request, u partnerUser, moduleKey string) (map[string]any, bool) {
	moduleKey = strings.TrimSpace(moduleKey)
	if moduleKey == "" || strings.Contains(moduleKey, "/") {
		common.APIError(w, http.StatusNotFound, "MODULE_NOT_FOUND", "Module not found")
		return nil, false
	}
	policy, err := a.loadPartnerUserModulePolicy(r.Context(), u.PartnerID, u.ID, u.PreferredLocale)
	if err != nil {
		writeInternalError(w, err, "Module access could not be evaluated")
		return nil, false
	}
	module, owned := policy.Owned[moduleKey]
	if !owned {
		common.APIError(w, http.StatusForbidden, "MODULE_NOT_OWNED", "This organization does not have active executable access to this module")
		return nil, false
	}
	if policy.Mode == partnerModuleAccessSelected && !policy.Selected[moduleKey] {
		common.APIError(w, http.StatusForbidden, "MODULE_NOT_ASSIGNED", "This module is not assigned to the authenticated user")
		return nil, false
	}
	return module, true
}

func partnerRuntimeModuleKey(path string) string {
	raw := strings.Trim(strings.TrimPrefix(path, "/partner/api/v1/runtime/modules/"), "/")
	if raw == "" {
		return ""
	}
	parts := strings.Split(raw, "/")
	return strings.TrimSpace(parts[0])
}

type moduleRuntimeStatusWriter struct {
	http.ResponseWriter
	status int
}

func (w *moduleRuntimeStatusWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *moduleRuntimeStatusWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(body)
}

func (a *app) recordPartnerModuleUsage(u partnerUser, moduleKey, method, runtimePath string) {
	partnerID := strings.TrimSpace(u.PartnerID)
	userID := strings.TrimSpace(u.ID)
	moduleKey = strings.TrimSpace(moduleKey)
	method = strings.ToUpper(strings.TrimSpace(method))
	runtimePath = strings.TrimSpace(runtimePath)
	if partnerID == "" || userID == "" || moduleKey == "" || runtimePath == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 750*time.Millisecond)
		defer cancel()
		var ignored map[string]any
		_ = a.internalJSON(ctx, http.MethodPost, a.hosts["catalog"], "/internal/v1/module-usage-events", map[string]any{
			"partner_id": partnerID,
			"user_id": userID,
			"module_key": moduleKey,
			"http_method": method,
			"runtime_path": runtimePath,
		}, nil, &ignored)
	}()
}

func (a *app) partnerModuleRuntime(w http.ResponseWriter, r *http.Request, u partnerUser) {
	key := partnerRuntimeModuleKey(r.URL.Path)
	module, ok := a.requirePartnerModuleExecution(w, r, u, key)
	if !ok {
		return
	}
	rec := &moduleRuntimeStatusWriter{ResponseWriter: w}
	raw := strings.Trim(strings.TrimPrefix(r.URL.Path, "/partner/api/v1/runtime/modules/"), "/")
	parts := strings.Split(raw, "/")
	if len(parts) == 2 && parts[1] == "access" && r.Method == http.MethodGet {
		common.JSON(rec, http.StatusOK, map[string]any{
			"partner_id": u.PartnerID,
			"user_id": u.ID,
			"module_key": key,
			"module": module,
			"access_state": "GRANTED",
			"security_rule": "PARTNER_ENTITLEMENT_INTERSECT_USER_ASSIGNMENT",
		})
		a.recordPartnerModuleUsage(u, key, r.Method, r.URL.Path)
		return
	}
	if key == partnerInvoiceModuleKey {
		a.partnerInvoiceModuleRuntime(rec, r, u, parts)
		status := rec.status
		if status == 0 {
			status = http.StatusOK
		}
		if status < http.StatusBadRequest {
			a.recordPartnerModuleUsage(u, key, r.Method, r.URL.Path)
		}
		return
	}
	common.APIError(rec, http.StatusNotFound, "MODULE_RUNTIME_ROUTE_NOT_FOUND", "No runtime operation is registered for this module path")
}

func (a *app) partnerInvoiceModuleRuntime(w http.ResponseWriter, r *http.Request, u partnerUser, parts []string) {
	if len(parts) < 2 {
		common.APIError(w, http.StatusNotFound, "MODULE_RUNTIME_ROUTE_NOT_FOUND", "Invoice runtime route not found")
		return
	}
	readOnly := r.Method == http.MethodGet || r.Method == http.MethodHead
	permission := "billing.read"
	if !readOnly {
		permission = "billing.write"
	}
	if !a.requirePartnerPermission(w, u, permission) {
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodPost && r.Method != http.MethodPut {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Unsupported tenant invoice operation")
		return
	}
	suffixParts := parts[1:]
	if suffixParts[0] != "invoices" && suffixParts[0] != "policy" {
		common.APIError(w, http.StatusNotFound, "MODULE_RUNTIME_ROUTE_NOT_FOUND", "Invoice runtime route not found")
		return
	}
	if suffixParts[0] == "policy" && len(suffixParts) != 1 {
		common.APIError(w, http.StatusNotFound, "MODULE_RUNTIME_ROUTE_NOT_FOUND", "Finance policy route not found")
		return
	}
	if suffixParts[0] == "invoices" && len(suffixParts) > 3 {
		common.APIError(w, http.StatusNotFound, "MODULE_RUNTIME_ROUTE_NOT_FOUND", "Invoice route not found")
		return
	}
	if suffixParts[0] == "invoices" && len(suffixParts) == 3 && suffixParts[2] != "finalize" {
		common.APIError(w, http.StatusNotFound, "MODULE_RUNTIME_ROUTE_NOT_FOUND", "Invoice action not found")
		return
	}

	upstream := "/internal/v1/tenant-finance/" + strings.Join(suffixParts, "/")
	if r.Method == http.MethodGet && strings.TrimSpace(r.URL.RawQuery) != "" {
		upstream += "?" + r.URL.RawQuery
	}
	var body any
	if r.Method == http.MethodPost || r.Method == http.MethodPut {
		payload := map[string]any{}
		if r.ContentLength != 0 {
			if common.Decode(r, &payload) != nil {
				common.APIError(w, http.StatusBadRequest, "JSON", "Invalid tenant invoice request")
				return
			}
		}
		body = payload
	}
	var out map[string]any
	err := a.internalJSON(r.Context(), r.Method, a.hosts["tenant-finance"], upstream, body, map[string]string{
		"X-Himate-Partner-ID": u.PartnerID,
		"X-Himate-User-ID": u.ID,
		"X-Correlation-ID": strings.TrimSpace(r.Header.Get("X-Correlation-ID")),
	}, &out)
	if err != nil {
		writeInternalError(w, err, "Tenant invoice operation could not be completed")
		return
	}
	out["gateway_tenant_context"] = "AUTHENTICATED_PARTNER_SESSION"
	status := http.StatusOK
	if r.Method == http.MethodPost && len(suffixParts) == 1 && suffixParts[0] == "invoices" && out["duplicate"] != true {
		status = http.StatusCreated
	}
	common.JSON(w, status, out)
}

func partnerUserModulePolicyMap(policy partnerUserModulePolicy) map[string]any {
	ownedModules := make([]map[string]any, 0, len(policy.OwnedKeys))
	for _, key := range policy.OwnedKeys {
		item := policy.Owned[key]
		ownedModules = append(ownedModules, map[string]any{
			"key": key,
			"label": item["label"],
			"group_key": item["group_key"],
			"group_label": item["group_label"],
			"access_state": item["access_state"],
			"executable": item["executable"],
		})
	}
	selected := make([]string, 0, len(policy.Selected))
	for key := range policy.Selected {
		selected = append(selected, key)
	}
	sort.Strings(selected)
	return map[string]any{
		"access_mode": policy.Mode,
		"selected_module_keys": selected,
		"effective_module_keys": policy.Effective,
		"owned_module_keys": policy.OwnedKeys,
		"stale_module_keys": policy.Stale,
		"owned_modules": ownedModules,
		"effective_count": len(policy.Effective),
		"owned_count": len(policy.OwnedKeys),
		"security_rule": "PARTNER_ENTITLEMENT_INTERSECT_USER_ASSIGNMENT",
	}
}

func (a *app) applyPartnerUserModuleAccess(ctx context.Context, u partnerUser, payload map[string]any) error {
	mode, selected, err := a.partnerUserModuleSelection(ctx, u.PartnerID, u.ID)
	if err != nil {
		return err
	}
	for _, item := range anyItems(payload["items"]) {
		key := strings.TrimSpace(fmt.Sprint(item["key"]))
		orgOwned := strings.ToUpper(strings.TrimSpace(fmt.Sprint(item["access_state"]))) == "ACTIVE" && item["executable"] == true
		granted := orgOwned && (mode == partnerModuleAccessAllOwned || selected[key])
		switch {
		case !orgOwned:
			item["user_access_state"] = "ORGANIZATION_LOCKED"
		case granted:
			item["user_access_state"] = "GRANTED"
		default:
			item["user_access_state"] = "NOT_ASSIGNED"
		}
		item["user_executable"] = granted
	}
	payload["user_module_access"] = map[string]any{
		"access_mode": mode,
		"security_rule": "PARTNER_ENTITLEMENT_INTERSECT_USER_ASSIGNMENT",
	}
	return nil
}

func (a *app) partnerUserModuleAccess(w http.ResponseWriter, r *http.Request, actor partnerUser, targetID string) {
	var targetRole string
	if err := a.db.QueryRow(
		`SELECT role_key FROM identity.partner_users WHERE id=$1 AND partner_id=$2`,
		targetID, actor.PartnerID,
	).Scan(&targetRole); err != nil {
		if err == sql.ErrNoRows {
			common.APIError(w, http.StatusNotFound, "NOT_FOUND", "Partner user not found")
			return
		}
		common.APIError(w, http.StatusInternalServerError, "DB", "Could not load partner user")
		return
	}
	if actor.Role != "owner" && targetRole == "owner" {
		common.APIError(w, http.StatusForbidden, "FORBIDDEN", "Only a partner owner can modify an owner")
		return
	}

	switch r.Method {
	case http.MethodGet:
		policy, err := a.loadPartnerUserModulePolicy(r.Context(), actor.PartnerID, targetID, actor.PreferredLocale)
		if err != nil {
			writeInternalError(w, err, "User module access could not be loaded")
			return
		}
		out := partnerUserModulePolicyMap(policy)
		out["user_id"] = targetID
		out["partner_id"] = actor.PartnerID
		common.JSON(w, http.StatusOK, out)
	case http.MethodPut:
		var in struct {
			AccessMode string   `json:"access_mode"`
			ModuleKeys []string `json:"module_keys"`
		}
		if common.Decode(r, &in) != nil {
			common.APIError(w, http.StatusBadRequest, "JSON", "Invalid module access request")
			return
		}
		mode := strings.ToUpper(strings.TrimSpace(in.AccessMode))
		if mode != partnerModuleAccessAllOwned && mode != partnerModuleAccessSelected {
			common.APIError(w, http.StatusBadRequest, "VALIDATION", "access_mode must be ALL_OWNED or SELECTED")
			return
		}
		owned, _, err := a.partnerOwnedModuleMap(r.Context(), actor.PartnerID, actor.PreferredLocale)
		if err != nil {
			writeInternalError(w, err, "Partner module entitlement could not be validated")
			return
		}
		unique := map[string]bool{}
		for _, raw := range in.ModuleKeys {
			key := strings.TrimSpace(raw)
			if key == "" || unique[key] {
				continue
			}
			unique[key] = true
			if _, ok := owned[key]; !ok {
				common.APIError(w, http.StatusConflict, "MODULE_NOT_OWNED", "A user cannot be assigned a module that is not ACTIVE and executable for the partner")
				return
			}
		}
		tx, err := a.db.BeginTx(r.Context(), nil)
		if err != nil {
			common.APIError(w, http.StatusInternalServerError, "DB", "Could not update user module access")
			return
		}
		defer tx.Rollback()
		if _, err = tx.ExecContext(r.Context(),
			`UPDATE identity.partner_users SET module_access_mode=$3,updated_at=NOW() WHERE id=$1 AND partner_id=$2`,
			targetID, actor.PartnerID, mode,
		); err != nil {
			common.APIError(w, http.StatusInternalServerError, "DB", "Could not update user module access")
			return
		}
		if _, err = tx.ExecContext(r.Context(),
			`DELETE FROM identity.partner_user_modules WHERE partner_id=$1 AND user_id=$2`,
			actor.PartnerID, targetID,
		); err != nil {
			common.APIError(w, http.StatusInternalServerError, "DB", "Could not replace user module grants")
			return
		}
		if mode == partnerModuleAccessSelected {
			keys := make([]string, 0, len(unique))
			for key := range unique {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				if _, err = tx.ExecContext(r.Context(),
					`INSERT INTO identity.partner_user_modules(partner_id,user_id,module_key,granted_by)
					 VALUES($1,$2,$3,$4)`,
					actor.PartnerID, targetID, key, actor.ID,
				); err != nil {
					common.APIError(w, http.StatusInternalServerError, "DB", "Could not save user module grants")
					return
				}
			}
		}
		if err = tx.Commit(); err != nil {
			common.APIError(w, http.StatusInternalServerError, "DB", "Could not commit user module access")
			return
		}
		policy, err := a.loadPartnerUserModulePolicy(r.Context(), actor.PartnerID, targetID, actor.PreferredLocale)
		if err != nil {
			common.APIError(w, http.StatusInternalServerError, "DB", "User module access was saved but could not be reloaded")
			return
		}
		out := partnerUserModulePolicyMap(policy)
		out["user_id"] = targetID
		out["partner_id"] = actor.PartnerID
		common.JSON(w, http.StatusOK, out)
	default:
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use GET or PUT")
	}
}
