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

func (a *app) materializeCentralWebsite(ctx context.Context) map[string]any {
	var pagesPayload, mediaPayload, environmentsPayload map[string]any
	var seoPayload, seoAuditPayload, contactPayload map[string]any
	var pagesErr, mediaErr, environmentsErr, seoErr, seoAuditErr, contactErr error
	var wg sync.WaitGroup
	wg.Add(6)
	go func() {
		defer wg.Done()
		pagesErr = a.internalGET(ctx, a.hosts["cms"], "/api/v1/cms/pages", &pagesPayload)
	}()
	go func() {
		defer wg.Done()
		mediaErr = a.internalGET(ctx, a.hosts["cms"], "/api/v1/cms/media", &mediaPayload)
	}()
	go func() {
		defer wg.Done()
		environmentsErr = a.internalGET(ctx, a.hosts["environments"], "/api/v1/environments", &environmentsPayload)
	}()
	go func() {
		defer wg.Done()
		seoErr = a.internalGET(ctx, a.hosts["cms"], "/api/v1/cms/seo", &seoPayload)
	}()
	go func() {
		defer wg.Done()
		seoAuditErr = a.internalGET(ctx, a.hosts["cms"], "/api/v1/cms/seo/audit", &seoAuditPayload)
	}()
	go func() {
		defer wg.Done()
		contactErr = a.internalGET(ctx, a.hosts["contact"], "/api/v1/contact/inquiries?limit=200&offset=0", &contactPayload)
	}()
	wg.Wait()

	unavailable := []string{}
	if pagesErr != nil { unavailable = append(unavailable, "cms_pages") }
	if mediaErr != nil { unavailable = append(unavailable, "cms_media") }
	if environmentsErr != nil { unavailable = append(unavailable, "environments") }
	if seoErr != nil { unavailable = append(unavailable, "seo") }
	if seoAuditErr != nil { unavailable = append(unavailable, "seo_audit") }
	if contactErr != nil { unavailable = append(unavailable, "contact_inquiries") }

	pages := central17Items(pagesPayload)
	media := central17Items(mediaPayload)
	environments := central17Items(environmentsPayload)

	pageDetails := map[string]any{}
	pageVersions := map[string]any{}
	pageAudits := map[string]any{}
	if pagesErr == nil {
		var detailWG sync.WaitGroup
		var detailMu sync.Mutex
		for _, page := range pages {
			id := central10String(page["id"])
			if id == "" {
				continue
			}
			pageID := id
			detailWG.Add(1)
			go func() {
				defer detailWG.Done()
				var detail, versions, audit map[string]any
				detailErr := a.internalGET(ctx, a.hosts["cms"], "/api/v1/cms/pages/"+pageID, &detail)
				versionsErr := a.internalGET(ctx, a.hosts["cms"], "/api/v1/cms/pages/"+pageID+"/versions", &versions)
				auditErr := a.internalGET(ctx, a.hosts["cms"], "/api/v1/cms/pages/"+pageID+"/audit", &audit)
				if detailErr != nil || versionsErr != nil || auditErr != nil {
					detailMu.Lock()
					unavailable = append(unavailable, "cms_page_detail:"+pageID)
					detailMu.Unlock()
					return
				}
				detailMu.Lock()
				pageDetails[pageID] = detail
				pageVersions[pageID] = versions
				pageAudits[pageID] = audit
				detailMu.Unlock()
			}()
		}
		detailWG.Wait()
	}

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
	status := "healthy"
	if len(unavailable) > 0 {
		status = "partial"
	}
	return map[string]any{
		"status": status,
		"unavailable": unavailable,
		"pages": pages,
		"media": media,
		"environments": environments,
		"seo": seoPayload,
		"seo_audit": seoAuditPayload,
		"contact_inquiries": contactPayload,
		"cms_page_details": pageDetails,
		"cms_page_versions": pageVersions,
		"cms_page_audits": pageAudits,
		"kpis": map[string]any{
			"pages": len(pages),
			"published_pages": published,
			"media_assets": len(media),
			"image_assets": imageAssets,
			"environments": len(environments),
			"live_environments": liveEnvironments,
		},
	}
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

	snapshot, updatedAt, ok := a.centralSnapshotForRead(r.Context(), centralStep4WebsiteKey)
	if !ok {
		common.APIError(w, http.StatusServiceUnavailable, "READ_MODEL_NOT_READY", "Website read model is not ready")
		return
	}
	if time.Since(updatedAt) > 2*centralStep4RefreshInterval {
		a.requestCentralStep4Refresh()
	}

	pages := []map[string]any{}
	media := []map[string]any{}
	environments := []map[string]any{}
	kpis := step4Map(snapshot["kpis"])
	if canCMS {
		pages = step4Items(snapshot["pages"])
		media = step4Items(snapshot["media"])
	} else {
		delete(kpis, "pages")
		delete(kpis, "published_pages")
		delete(kpis, "media_assets")
		delete(kpis, "image_assets")
	}
	if canEnvironments {
		environments = step4Items(snapshot["environments"])
	} else {
		delete(kpis, "environments")
		delete(kpis, "live_environments")
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
		"pages": pages,
		"media": media,
		"environments": environments,
		"kpis": kpis,
		"meta": centralStep4Meta(started, centralStep4WebsiteKey, updatedAt, "healthy", []string{}),
	}
	w.Header().Set("X-Himate-Cache", "persistent-read-model")
	w.Header().Set("Server-Timing", fmt.Sprintf("central-website-read-model;dur=%d", time.Since(started).Milliseconds()))
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
