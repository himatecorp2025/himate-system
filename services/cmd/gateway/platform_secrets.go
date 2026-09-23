package main

import (
	"fmt"
	"himate.local/services/internal/common"
	"net/http"
	"strings"
	"time"
)

type platformSecretDefinition struct {
	Key         string
	Environment string
	Label       string
	Provider    string
	Consumer    string
	Description string
}

var platformSecretDefinitions = []platformSecretDefinition{
	{
		Key: "stripe_secret_key", Environment: "STRIPE_SECRET_KEY",
		Label: "Stripe secret API key", Provider: "Stripe", Consumer: "Payments",
		Description: "Server-side Stripe credential used to create and collect PaymentIntents.",
	},
	{
		Key: "stripe_webhook_secret", Environment: "STRIPE_WEBHOOK_SECRET",
		Label: "Stripe webhook signing secret", Provider: "Stripe", Consumer: "Payments",
		Description: "Signing secret used to verify Stripe webhook payloads before settlement.",
	},
	{
		Key: "render_api_key", Environment: "RENDER_API_KEY",
		Label: "Render API key", Provider: "Render", Consumer: "Runtime",
		Description: "Provider credential used by HIMATE runtime operations to trigger and inspect Render deployments.",
	},
}

func platformSecretsMigration() common.Migration {
	return common.Migration{
		Version: 10,
		Name:    "platform-secrets-vault",
		Statements: []string{
			`CREATE TABLE IF NOT EXISTS identity.platform_secrets(
				secret_key TEXT PRIMARY KEY,
				encrypted_value TEXT NOT NULL,
				active BOOLEAN NOT NULL DEFAULT TRUE,
				updated_by TEXT NOT NULL DEFAULT '',
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE INDEX IF NOT EXISTS identity_platform_secrets_active_idx ON identity.platform_secrets(active,secret_key)`,
		},
	}
}

func platformSecretDefinitionByKey(key string) (platformSecretDefinition, bool) {
	for _, definition := range platformSecretDefinitions {
		if definition.Key == key {
			return definition, true
		}
	}
	return platformSecretDefinition{}, false
}

func platformSecretMetadata(definition platformSecretDefinition, configured bool, updatedBy string, updatedAt any) map[string]any {
	item := map[string]any{
		"key": definition.Key,
		"environment_key": definition.Environment,
		"label": definition.Label,
		"provider": definition.Provider,
		"consumer": definition.Consumer,
		"description": definition.Description,
		"configured": configured,
		"status": "MISSING",
		"value": nil,
	}
	if configured {
		item["status"] = "CONFIGURED"
		item["updated_by"] = updatedBy
		item["updated_at"] = updatedAt
	}
	return item
}

func (a *app) adminSecrets(w http.ResponseWriter, r *http.Request, u user) {
	raw := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/secrets"), "/")
	if raw == "" {
		if r.Method != http.MethodGet {
			common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use GET")
			return
		}
		type configuredSecret struct {
			UpdatedBy string
			UpdatedAt time.Time
		}
		configured := map[string]configuredSecret{}
		rows, err := a.db.QueryContext(r.Context(), `SELECT secret_key,updated_by,updated_at FROM identity.platform_secrets WHERE active=TRUE`)
		if err != nil {
			common.APIError(w, http.StatusInternalServerError, "SECRETS", "Could not load platform secret status")
			return
		}
		defer rows.Close()
		for rows.Next() {
			var key, actor string
			var updated time.Time
			if rows.Scan(&key, &actor, &updated) == nil {
				configured[key] = configuredSecret{UpdatedBy: actor, UpdatedAt: updated}
			}
		}
		items := make([]map[string]any, 0, len(platformSecretDefinitions))
		for _, definition := range platformSecretDefinitions {
			state, ok := configured[definition.Key]
			items = append(items, platformSecretMetadata(definition, ok, state.UpdatedBy, state.UpdatedAt))
		}
		common.JSON(w, http.StatusOK, map[string]any{
			"items": items,
			"count": len(items),
			"storage": "AES_256_GCM_ENCRYPTED",
			"values_readable": false,
		})
		return
	}

	if strings.Contains(raw, "/") {
		common.APIError(w, http.StatusNotFound, "NOT_FOUND", "Secret route not found")
		return
	}
	definition, ok := platformSecretDefinitionByKey(raw)
	if !ok {
		common.APIError(w, http.StatusNotFound, "SECRET_NOT_SUPPORTED", "This platform secret is not supported")
		return
	}
	if !u.SystemOwner {
		common.APIError(w, http.StatusForbidden, "SYSTEM_OWNER_REQUIRED", "Only the HIMATE system owner may change platform secrets")
		return
	}

	switch r.Method {
	case http.MethodPut:
		var in struct {
			Secret string `json:"secret"`
		}
		if common.Decode(r, &in) != nil {
			common.APIError(w, http.StatusBadRequest, "JSON", "Invalid request")
			return
		}
		if strings.TrimSpace(in.Secret) == "" {
			common.APIError(w, http.StatusBadRequest, "SECRET_REQUIRED", "Secret value is required")
			return
		}
		if len(in.Secret) > 8192 {
			common.APIError(w, http.StatusBadRequest, "SECRET_TOO_LARGE", "Secret value is too large")
			return
		}
		encrypted, err := common.EncryptPlatformSecret(a.internalToken, in.Secret)
		if err != nil {
			common.APIError(w, http.StatusInternalServerError, "SECRET_ENCRYPTION", "Secret could not be encrypted")
			return
		}
		_, err = a.db.ExecContext(r.Context(), `INSERT INTO identity.platform_secrets(secret_key,encrypted_value,active,updated_by,updated_at)
			VALUES($1,$2,TRUE,$3,NOW())
			ON CONFLICT(secret_key) DO UPDATE SET encrypted_value=EXCLUDED.encrypted_value,active=TRUE,updated_by=EXCLUDED.updated_by,updated_at=NOW()`,
			definition.Key, encrypted, u.ID)
		if err != nil {
			common.APIError(w, http.StatusInternalServerError, "SECRET_SAVE", "Secret could not be stored")
			return
		}
		common.JSON(w, http.StatusOK, platformSecretMetadata(definition, true, u.ID, time.Now().UTC()))
	case http.MethodDelete:
		result, err := a.db.ExecContext(r.Context(), `DELETE FROM identity.platform_secrets WHERE secret_key=$1`, definition.Key)
		if err != nil {
			common.APIError(w, http.StatusInternalServerError, "SECRET_DELETE", "Secret could not be removed")
			return
		}
		affected, _ := result.RowsAffected()
		if affected == 0 {
			common.APIError(w, http.StatusNotFound, "SECRET_NOT_CONFIGURED", "Secret is not configured")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", fmt.Sprintf("Use PUT or DELETE for %s", definition.Environment))
	}
}
