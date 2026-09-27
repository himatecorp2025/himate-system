package main

import (
	"context"
	"encoding/json"
	"time"
)

func (a *app) materializeAdministrationRoles(ctx context.Context) (map[string]any, error) {
	items := make([]map[string]any, 0, len(roleDefinitions)+8)
	for _, role := range roleDefinitions {
		items = append(items, map[string]any{
			"key": role.Key,
			"label_en": role.Label, "label_hu": role.Label,
			"description_en": role.Description, "description_hu": role.Description,
			"permissions": role.Permissions, "system": true, "active": true,
		})
	}
	rows, err := a.db.QueryContext(ctx,
		`SELECT role_key,label_en,label_hu,description_en,description_hu,permissions,active,created_by,created_at,updated_at
		 FROM identity.custom_roles
		 ORDER BY active DESC,lower(label_en),role_key`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var key, labelEN, labelHU, descEN, descHU, createdBy string
		var raw []byte
		var active bool
		var created, updated time.Time
		if err := rows.Scan(&key, &labelEN, &labelHU, &descEN, &descHU, &raw, &active, &createdBy, &created, &updated); err != nil {
			return nil, err
		}
		permissions := []string{}
		_ = json.Unmarshal(raw, &permissions)
		items = append(items, map[string]any{
			"key": key, "label_en": labelEN, "label_hu": labelHU,
			"description_en": descEN, "description_hu": descHU,
			"permissions": permissions, "system": false, "active": active,
			"created_by": createdBy, "created_at": created.UTC(), "updated_at": updated.UTC(),
		})
	}
	return map[string]any{"items": items, "count": len(items)}, rows.Err()
}

func (a *app) materializeAdministrationUsers(ctx context.Context) (map[string]any, error) {
	rows, err := a.db.QueryContext(ctx,
		`SELECT id,name,email,password_hash,roles,active,system_owner,preferred_locale,timezone,job_title,phone,session_version,created_at,updated_at
		 FROM identity.users
		 ORDER BY system_owner DESC,active DESC,lower(name),lower(email)`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var u user
		var rolesRaw []byte
		var createdAt, updatedAt time.Time
		if err := rows.Scan(
			&u.ID, &u.Name, &u.Email, &u.PasswordHash, &rolesRaw, &u.Active, &u.SystemOwner,
			&u.PreferredLocale, &u.Timezone, &u.JobTitle, &u.Phone, &u.SessionVersion,
			&createdAt, &updatedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(rolesRaw, &u.Roles)
		items = append(items, a.adminUserMap(u, createdAt, updatedAt))
	}
	return map[string]any{"items": items, "count": len(items)}, rows.Err()
}

func (a *app) materializeAdministrationSecrets(ctx context.Context) (map[string]any, error) {
	type configuredSecret struct {
		UpdatedBy string
		UpdatedAt time.Time
	}
	configured := map[string]configuredSecret{}
	rows, err := a.db.QueryContext(ctx,
		`SELECT secret_key,updated_by,updated_at
		 FROM identity.platform_secrets
		 WHERE active=TRUE`,
	)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var key, actor string
		var updated time.Time
		if err := rows.Scan(&key, &actor, &updated); err != nil {
			rows.Close()
			return nil, err
		}
		configured[key] = configuredSecret{UpdatedBy: actor, UpdatedAt: updated.UTC()}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	items := make([]map[string]any, 0, len(platformSecretDefinitions))
	for _, definition := range platformSecretDefinitions {
		state, ok := configured[definition.Key]
		items = append(items, platformSecretMetadata(definition, ok, state.UpdatedBy, state.UpdatedAt))
	}
	return map[string]any{
		"items": items, "count": len(items),
		"storage": "AES_256_GCM_ENCRYPTED", "values_readable": false,
	}, nil
}

func (a *app) materializeAdministrationAudit(ctx context.Context) (map[string]any, error) {
	rows, err := a.db.QueryContext(ctx,
		`SELECT id,actor_id,actor_name,actor_roles,request_id,correlation_id,action,method,path,resource,partner_id,status,outcome,old_state,new_state,duration_ms,created_at
		 FROM identity.audit_events
		 ORDER BY created_at DESC,id DESC
		 LIMIT 5000`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id int64
		var actorID, actorName, requestID, correlationID, action, method, path, resource, partnerID, outcome string
		var rolesRaw, oldRaw, newRaw []byte
		var status int
		var duration int64
		var created time.Time
		if err := rows.Scan(
			&id, &actorID, &actorName, &rolesRaw, &requestID, &correlationID, &action,
			&method, &path, &resource, &partnerID, &status, &outcome, &oldRaw, &newRaw,
			&duration, &created,
		); err != nil {
			return nil, err
		}
		var roles []string
		var oldState, newState any
		_ = json.Unmarshal(rolesRaw, &roles)
		_ = json.Unmarshal(oldRaw, &oldState)
		_ = json.Unmarshal(newRaw, &newState)
		items = append(items, map[string]any{
			"id": id, "actor_id": actorID, "actor_name": actorName, "actor_roles": roles,
			"request_id": requestID, "correlation_id": correlationID, "action": action,
			"method": method, "path": path, "resource": resource, "partner_id": partnerID,
			"status": status, "outcome": outcome, "old_state": oldState, "new_state": newState,
			"duration_ms": duration, "created_at": created.UTC(),
		})
	}
	return map[string]any{"items": items, "count": len(items), "total": len(items)}, rows.Err()
}
