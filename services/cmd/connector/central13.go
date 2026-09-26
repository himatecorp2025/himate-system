package main

import (
	"database/sql"
	"net/http"
	"sort"
	"strings"
	"time"

	"himate.local/services/internal/common"
)

type central13ConnectionRow struct {
	PartnerID          string
	ConnectionStatus   string
	LastSuccessfulSync any
	LastError          string
	Integrations       []map[string]any
}

func central13LatestTime(values ...sql.NullTime) sql.NullTime {
	var latest sql.NullTime
	for _, value := range values {
		if !value.Valid {
			continue
		}
		if !latest.Valid || value.Time.After(latest.Time) {
			latest = value
		}
	}
	return latest
}

func central13ConnectionStatus(active, configured, degraded bool) string {
	switch {
	case configured && degraded:
		return "SUSPENDED"
	case active:
		return "ACTIVE"
	default:
		return "INACTIVE"
	}
}

func (a *app) central13PartnerConnections(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use GET")
		return
	}

	type aggregate struct {
		active       bool
		configured   bool
		degraded     bool
		lastSuccess  sql.NullTime
		lastError    string
		lastErrorAt  time.Time
		integrations map[string]map[string]any
	}
	byPartner := map[string]*aggregate{}
	get := func(partnerID string) *aggregate {
		row := byPartner[partnerID]
		if row == nil {
			row = &aggregate{integrations: map[string]map[string]any{}}
			byPartner[partnerID] = row
		}
		return row
	}

	credentialRows, err := a.db.Query(`SELECT partner_id,environment,active,last_used_at
		FROM connector.credentials ORDER BY partner_id,environment`)
	if err != nil {
		common.APIError(w, http.StatusInternalServerError, "DB", "Could not load connector credentials")
		return
	}
	for credentialRows.Next() {
		var partnerID, environment string
		var active bool
		var lastUsed sql.NullTime
		if credentialRows.Scan(&partnerID, &environment, &active, &lastUsed) != nil {
			continue
		}
		row := get(partnerID)
		row.configured = true
		row.active = row.active || active
		key := "DATA_CONNECTOR|" + environment
		row.integrations[key] = map[string]any{
			"type": "DATA_CONNECTOR", "label": "HIMATE Data Connector",
			"environment": environment, "enabled": active,
			"last_used_at": func() any { if lastUsed.Valid { return lastUsed.Time.UTC() }; return nil }(),
		}
	}
	credentialRows.Close()

	stateRows, err := a.db.Query(`SELECT partner_id,environment,health,sync_status,
		last_metric_sync_at,last_data_sync_at,last_reconciliation_at,last_error,updated_at
		FROM connector.partner_state ORDER BY partner_id,environment`)
	if err != nil {
		common.APIError(w, http.StatusInternalServerError, "DB", "Could not load connector states")
		return
	}
	for stateRows.Next() {
		var partnerID, environment, health, syncStatus, lastError string
		var metricSync, dataSync, reconciliation sql.NullTime
		var updatedAt time.Time
		if stateRows.Scan(&partnerID, &environment, &health, &syncStatus, &metricSync, &dataSync, &reconciliation, &lastError, &updatedAt) != nil {
			continue
		}
		row := get(partnerID)
		row.configured = true
		health = strings.ToUpper(strings.TrimSpace(health))
		syncStatus = strings.ToUpper(strings.TrimSpace(syncStatus))
		if health == "ERROR" || health == "OFFLINE" || syncStatus == "FAILED" || syncStatus == "ERROR" {
			row.degraded = true
		}
		latest := central13LatestTime(metricSync, dataSync, reconciliation)
		if latest.Valid && (!row.lastSuccess.Valid || latest.Time.After(row.lastSuccess.Time)) {
			row.lastSuccess = latest
		}
		if strings.TrimSpace(lastError) != "" && (row.lastError == "" || updatedAt.After(row.lastErrorAt)) {
			row.lastError = strings.TrimSpace(lastError)
			row.lastErrorAt = updatedAt
		}
		key := "DATA_CONNECTOR|" + environment
		integration := row.integrations[key]
		if integration == nil {
			integration = map[string]any{
				"type": "DATA_CONNECTOR", "label": "HIMATE Data Connector",
				"environment": environment, "enabled": false,
			}
			row.integrations[key] = integration
		}
		integration["health"] = health
		integration["sync_status"] = syncStatus
		if latest.Valid {
			integration["last_successful_sync"] = latest.Time.UTC()
		}
		if strings.TrimSpace(lastError) != "" {
			integration["last_error"] = strings.TrimSpace(lastError)
		}
	}
	stateRows.Close()

	adapterRows, err := a.db.Query(`SELECT partner_id,environment,adapter_type,site_base_url,enabled,updated_at
		FROM connector.website_adapters ORDER BY partner_id,environment`)
	if err != nil {
		common.APIError(w, http.StatusInternalServerError, "DB", "Could not load website adapters")
		return
	}
	for adapterRows.Next() {
		var partnerID, environment, adapterType, baseURL string
		var enabled bool
		var updatedAt time.Time
		if adapterRows.Scan(&partnerID, &environment, &adapterType, &baseURL, &enabled, &updatedAt) != nil {
			continue
		}
		row := get(partnerID)
		row.configured = true
		row.active = row.active || enabled
		adapterType = strings.ToUpper(strings.TrimSpace(adapterType))
		if adapterType == "" {
			adapterType = "GENERIC_HTTP"
		}
		key := "WEBSITE_ADAPTER|" + environment
		row.integrations[key] = map[string]any{
			"type": adapterType,
			"label": "Website Adapter",
			"environment": environment,
			"enabled": enabled,
			"site_base_url": strings.TrimSpace(baseURL),
			"updated_at": updatedAt.UTC(),
		}
	}
	adapterRows.Close()

	out := make([]map[string]any, 0, len(byPartner))
	for partnerID, row := range byPartner {
		integrations := make([]map[string]any, 0, len(row.integrations))
		types := map[string]bool{}
		for _, integration := range row.integrations {
			integrations = append(integrations, integration)
			if value := strings.TrimSpace(integration["type"].(string)); value != "" {
				types[value] = true
			}
		}
		sort.Slice(integrations, func(i, j int) bool {
			left := integrations[i]["type"].(string) + "|" + integrations[i]["environment"].(string)
			right := integrations[j]["type"].(string) + "|" + integrations[j]["environment"].(string)
			return left < right
		})
		connectionTypes := make([]string, 0, len(types))
		for value := range types {
			connectionTypes = append(connectionTypes, value)
		}
		sort.Strings(connectionTypes)
		var lastSuccess any
		if row.lastSuccess.Valid {
			lastSuccess = row.lastSuccess.Time.UTC()
		}
		out = append(out, map[string]any{
			"partner_id": partnerID,
			"connection_status": central13ConnectionStatus(row.active, row.configured, row.degraded),
			"last_successful_sync": lastSuccess,
			"last_error": row.lastError,
			"integration_count": len(integrations),
			"connection_types": connectionTypes,
			"integrations": integrations,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i]["partner_id"].(string) < out[j]["partner_id"].(string)
	})
	common.JSON(w, http.StatusOK, map[string]any{"items": out, "count": len(out)})
}
