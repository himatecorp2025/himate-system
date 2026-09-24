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

	"himate.local/services/internal/common"
)

const (
	partnerModuleAccessAllOwned = "ALL_OWNED"
	partnerModuleAccessSelected = "SELECTED"
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
