package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"himate.local/services/internal/common"
)

func central17Items(payload map[string]any) []map[string]any {
	raw, ok := payload["items"].([]any)
	if !ok {
		return []map[string]any{}
	}
	out := make([]map[string]any, 0, len(raw))
	for _, value := range raw {
		if item, ok := value.(map[string]any); ok {
			out = append(out, item)
		}
	}
	return out
}

func central17Status(unavailable []string, successful int) string {
	if successful == 0 && len(unavailable) > 0 {
		return "unavailable"
	}
	if len(unavailable) > 0 {
		return "partial"
	}
	return "healthy"
}

func (a *app) central17Website(w http.ResponseWriter, r *http.Request, actor user, cacheKey string) {
	started := time.Now()
	canCMS := a.hasPermission(actor, "cms.read")
	canContact := a.hasPermission(actor, "contact.read")
	canConnections := a.hasPermission(actor, "connectors.read")
	canEnvironments := a.hasPermission(actor, "environments.read")
	if !canCMS && !canContact && !canConnections && !canEnvironments {
		common.APIError(w, http.StatusForbidden, "FORBIDDEN", "Website & Marketing read permission required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), central10ReadBudget)
	defer cancel()

	var pagesPayload, mediaPayload, environmentsPayload map[string]any
	var pagesErr, mediaErr, environmentsErr error
	var wg sync.WaitGroup
	if canCMS {
		wg.Add(2)
		go func() {
			defer wg.Done()
			pagesErr = a.internalGET(ctx, a.hosts["cms"], "/api/v1/cms/pages", &pagesPayload)
		}()
		go func() {
			defer wg.Done()
			mediaErr = a.internalGET(ctx, a.hosts["cms"], "/api/v1/cms/media", &mediaPayload)
		}()
	}
	if canEnvironments {
		wg.Add(1)
		go func() {
			defer wg.Done()
			environmentsErr = a.internalGET(ctx, a.hosts["environments"], "/api/v1/environments", &environmentsPayload)
		}()
	}
	wg.Wait()

	unavailable := []string{}
	successful := 0
	if canCMS {
		if pagesErr != nil {
			unavailable = append(unavailable, "cms_pages")
		} else {
			successful++
		}
		if mediaErr != nil {
			unavailable = append(unavailable, "cms_media")
		} else {
			successful++
		}
	}
	if canEnvironments {
		if environmentsErr != nil {
			unavailable = append(unavailable, "environments")
		} else {
			successful++
		}
	}

	if successful == 0 && len(unavailable) > 0 {
		if stale, ok, _ := central10Cached(cacheKey, false); ok {
			stale["meta"] = central10Meta(started, "stale", unavailable)
			w.Header().Set("X-Himate-Cache", "stale")
			common.JSON(w, http.StatusOK, stale)
			return
		}
	}

	pages := central17Items(pagesPayload)
	media := central17Items(mediaPayload)
	environments := central17Items(environmentsPayload)
	published := 0
	for _, page := range pages {
		status := strings.ToUpper(central10String(page["status"]))
		if status == "" {
			status = strings.ToUpper(central10String(page["publication_status"]))
		}
		if status == "PUBLISHED" || status == "ACTIVE" || central10String(page["published_version_id"]) != "" {
			published++
		}
	}
	imageAssets := 0
	for _, asset := range media {
		if strings.HasPrefix(strings.ToLower(central10String(asset["mime_type"])), "image/") {
			imageAssets++
		}
	}
	liveEnvironments := 0
	for _, environment := range environments {
		status := strings.ToUpper(central10String(environment["environment_status"]))
		if status == "" {
			status = strings.ToUpper(central10String(environment["status"]))
		}
		if status == "LIVE" || status == "READY" || status == "DEPLOYED" {
			liveEnvironments++
		}
	}

	payload := map[string]any{
		"ready": true,
		"access": map[string]any{
			"cms":                  canCMS,
			"cms_write":            a.hasPermission(actor, "cms.write"),
			"cms_approve":          a.hasPermission(actor, "cms.approve"),
			"contact":              canContact,
			"contact_write":        a.hasPermission(actor, "contact.write"),
			"connections":          canConnections,
			"connections_write":    a.hasPermission(actor, "connectors.write"),
			"connections_approve":  a.hasPermission(actor, "connectors.approve"),
			"environments":         canEnvironments,
			"environments_write":   a.hasPermission(actor, "environments.write"),
			"environments_approve": a.hasPermission(actor, "environments.approve"),
		},
		"pages":        pages,
		"media":        media,
		"environments": environments,
		"kpis": map[string]any{
			"pages":             len(pages),
			"published_pages":   published,
			"media_assets":      len(media),
			"image_assets":      imageAssets,
			"environments":      len(environments),
			"live_environments": liveEnvironments,
		},
		"meta": central10Meta(started, central17Status(unavailable, successful), unavailable),
	}
	central10Store(cacheKey, payload)
	w.Header().Set("X-Himate-Cache", "miss")
	w.Header().Set("Server-Timing", fmt.Sprintf("central-website;dur=%d", time.Since(started).Milliseconds()))
	common.JSON(w, http.StatusOK, payload)
}

func (a *app) materializeCentralSystem(ctx context.Context) map[string]any {
	var healthPayload, provisioningPayload, environmentsPayload, backupsPayload map[string]any
	var healthErr, provisioningErr, environmentsErr, backupsErr, auditErr error
	auditEvents := []map[string]any{}
	var wg sync.WaitGroup
	wg.Add(5)

	go func() {
		defer wg.Done()
		healthErr = a.internalGET(ctx, a.hosts["health"], "/api/v1/system-health/snapshot", &healthPayload)
	}()
	go func() {
		defer wg.Done()
		provisioningErr = a.internalGET(ctx, a.hosts["provisioning"], "/api/v1/provisioning/jobs", &provisioningPayload)
	}()
	go func() {
		defer wg.Done()
		environmentsErr = a.internalGET(ctx, a.hosts["environments"], "/api/v1/environments", &environmentsPayload)
	}()
	go func() {
		defer wg.Done()
		backupsErr = a.internalGET(ctx, a.hosts["backups"], "/api/v1/backups/summary", &backupsPayload)
	}()
	go func() {
		defer wg.Done()
		rows, err := a.db.QueryContext(ctx, `SELECT action,method,path,status,outcome,created_at
			FROM identity.audit_events ORDER BY id DESC LIMIT 12`)
		if err != nil {
			auditErr = err
			return
		}
		defer rows.Close()
		for rows.Next() {
			var action, method, path, outcome string
			var status int
			var createdAt time.Time
			if err := rows.Scan(&action, &method, &path, &status, &outcome, &createdAt); err != nil {
				auditErr = err
				return
			}
			auditEvents = append(auditEvents, map[string]any{
				"action": action, "method": method, "path": path,
				"status": status, "outcome": outcome, "created_at": createdAt.UTC(),
			})
		}
		if err := rows.Err(); err != nil {
			auditErr = err
		}
	}()
	wg.Wait()

	unavailable := []string{}
	if healthErr != nil {
		unavailable = append(unavailable, "system_health")
	}
	if provisioningErr != nil {
		unavailable = append(unavailable, "provisioning")
	}
	if environmentsErr != nil {
		unavailable = append(unavailable, "environments")
	}
	if backupsErr != nil {
		unavailable = append(unavailable, "backups")
	}
	if auditErr != nil {
		unavailable = append(unavailable, "audit_events")
	}

	services := central17Items(map[string]any{"items": healthPayload["services"]})
	partners := central17Items(map[string]any{"items": healthPayload["partners"]})
	provisioning := central17Items(provisioningPayload)
	environments := central17Items(environmentsPayload)
	backups := central17Items(backupsPayload)

	healthyServices := 0
	for _, service := range services {
		status := strings.ToUpper(central10String(service["status"]))
		switch status {
		case "OK", "HEALTHY", "LIVE", "READY", "ACTIVE", "DEPLOYED":
			healthyServices++
		}
	}
	degradedPartners := 0
	for _, partner := range partners {
		status := strings.ToUpper(central10String(partner["overall_status"]))
		switch status {
		case "OK", "HEALTHY", "LIVE", "READY", "ACTIVE", "DEPLOYED":
		default:
			degradedPartners++
		}
	}
	deployedEnvironments := 0
	for _, environment := range environments {
		status := strings.ToUpper(central10String(environment["deployment_status"]))
		if status == "DEPLOYED" || status == "LIVE" || status == "READY" {
			deployedEnvironments++
		}
	}
	overall := central10String(healthPayload["status"])
	if overall == "" {
		overall = "UNKNOWN"
	}
	issueCount := (len(services) - healthyServices) + degradedPartners
	status := "healthy"
	if len(unavailable) > 0 {
		status = "partial"
	}
	return map[string]any{
		"status":      status,
		"unavailable": unavailable,
		"health": map[string]any{
			"status":   overall,
			"services": services,
			"partners": partners,
		},
		"provisioning": provisioning,
		"environments": environments,
		"events":       auditEvents,
		"backups": map[string]any{
			"provider": central10String(backupsPayload["provider"]),
			"items":    backups,
		},
		"kpis": map[string]any{
			"healthy_services":      healthyServices,
			"service_count":         len(services),
			"partner_systems":       len(partners),
			"deployed_environments": deployedEnvironments,
			"environment_count":     len(environments),
			"issues":                issueCount,
			"degraded_partners":     degradedPartners,
		},
	}
}

func (a *app) central17System(w http.ResponseWriter, r *http.Request, actor user, cacheKey string) {
	started := time.Now()
	canHealth := a.hasPermission(actor, "health.read")
	canProvisioning := a.hasPermission(actor, "provisioning.read")
	canEnvironments := a.hasPermission(actor, "environments.read")
	canBackups := a.hasPermission(actor, "backups.read")
	canAudit := a.hasPermission(actor, "audit.read")
	if !canHealth && !canProvisioning && !canEnvironments && !canBackups {
		common.APIError(w, http.StatusForbidden, "FORBIDDEN", "System & Operations read permission required")
		return
	}

	snapshot, updatedAt, ok := a.centralSnapshotForRead(r.Context(), centralStep4SystemKey)
	if !ok {
		a.requestCentralStep4Refresh()
		common.JSON(w, http.StatusOK, map[string]any{
			"ready":        false,
			"access":       map[string]any{},
			"health":       map[string]any{},
			"provisioning": []map[string]any{},
			"environments": []map[string]any{},
			"events":       []map[string]any{},
			"backups":      map[string]any{"provider": "", "items": []map[string]any{}},
			"kpis":         map[string]any{},
			"meta":         centralStep4Meta(started, centralStep4SystemKey, time.Time{}, "warming", []string{}),
		})
		return
	}
	if time.Since(updatedAt) > 2*centralStep4RefreshInterval {
		a.requestCentralStep4Refresh()
	}

	health := step4Map(snapshot["health"])
	provisioning := step4Items(snapshot["provisioning"])
	environments := step4Items(snapshot["environments"])
	events := step4Items(snapshot["events"])
	backups := step4Map(snapshot["backups"])
	kpis := step4Map(snapshot["kpis"])

	if !canHealth {
		health = map[string]any{}
		for _, key := range []string{"healthy_services", "service_count", "partner_systems", "issues", "degraded_partners"} {
			delete(kpis, key)
		}
	}
	if !canProvisioning {
		provisioning = []map[string]any{}
	}
	if !canEnvironments {
		environments = []map[string]any{}
		delete(kpis, "deployed_environments")
		delete(kpis, "environment_count")
	}
	if !canBackups {
		backups = map[string]any{"provider": "", "items": []map[string]any{}}
	}
	if !canAudit {
		events = []map[string]any{}
	}

	payload := map[string]any{
		"ready": true,
		"access": map[string]any{
			"health":               canHealth,
			"provisioning":         canProvisioning,
			"provisioning_approve": a.hasPermission(actor, "provisioning.approve"),
			"environments":         canEnvironments,
			"environments_write":   a.hasPermission(actor, "environments.write"),
			"environments_approve": a.hasPermission(actor, "environments.approve"),
			"backups":              canBackups,
			"backups_write":        a.hasPermission(actor, "backups.write") || a.hasPermission(actor, "backups.approve"),
			"backups_approve":      a.hasPermission(actor, "backups.approve"),
			"audit":                canAudit,
		},
		"health":       health,
		"provisioning": provisioning,
		"environments": environments,
		"events":       events,
		"backups":      backups,
		"kpis":         kpis,
		"meta":         centralStep4Meta(started, centralStep4SystemKey, updatedAt, "healthy", []string{}),
	}
	w.Header().Set("X-Himate-Cache", "hot-snapshot")
	w.Header().Set("Server-Timing", fmt.Sprintf("central-system-snapshot;dur=%d", time.Since(started).Milliseconds()))
	common.JSON(w, http.StatusOK, payload)
}
