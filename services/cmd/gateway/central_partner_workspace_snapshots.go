package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	centralPartnerWorkspacePrefix              = "partner_workspace:"
	centralPartnerWorkspaceRefreshInterval     = 30 * time.Second
	centralPartnerWorkspaceMaterializeBudget   = 6 * time.Second
	centralPartnerWorkspaceStartupBudget       = 12 * time.Second
	centralPartnerWorkspaceMaterializeWorkers  = 6
)

func centralPartnerWorkspaceKey(partnerID string) string {
	return centralPartnerWorkspacePrefix + strings.TrimSpace(partnerID)
}

func centralPartnerWorkspaceID(key string) string {
	return strings.TrimSpace(strings.TrimPrefix(key, centralPartnerWorkspacePrefix))
}

func (a *app) centralPartnerIDsForMaterialization(ctx context.Context) []string {
	seen := map[string]bool{}
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id != "" {
			seen[id] = true
		}
	}

	// Active/current tenants come from the already materialized Partners screen.
	if snapshot, _, ok := centralStep3SnapshotGet(centralStep4PartnersKey); ok {
		for _, partner := range step4Items(snapshot["items"]) {
			add(central10String(partner["id"]))
		}
	}

	// Persisted workspaces keep historical/archived tenants in the refresh set
	// even if they no longer appear in the active Central portfolio.
	if rows, err := a.db.QueryContext(ctx, `SELECT partner_id FROM identity.partner_workspace_snapshots ORDER BY partner_id`); err == nil {
		for rows.Next() {
			var id string
			if rows.Scan(&id) == nil {
				add(id)
			}
		}
		rows.Close()
	}

	// During background materialization (never a browser read path), reconcile
	// the directory against the authoritative Partners owner including archives.
	// A dependency failure only skips discovery; existing LKG tenant rows remain.
	const pageSize = 200
	for offset := 0; ; offset += pageSize {
		var page central10ItemsPage
		path := fmt.Sprintf(
			"/api/v1/partners?limit=%d&offset=%d&include_archived=true&include_stats=false",
			pageSize, offset,
		)
		if err := a.internalGET(ctx, a.hosts["partners"], path, &page); err != nil {
			if a.log != nil {
				a.log.Warn("tenant materialization directory reconciliation failed; retaining persisted IDs", "error", err)
			}
			break
		}
		for _, partner := range page.Items {
			add(central10String(partner["id"]))
		}
		if !page.HasMore || len(page.Items) == 0 || (page.Total > 0 && offset+len(page.Items) >= page.Total) {
			break
		}
	}

	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (a *app) partnerWorkspaceBasePartner(partnerID string) map[string]any {
	snapshot, _, ok := centralStep3SnapshotGet(centralStep4PartnersKey)
	if !ok {
		return nil
	}
	for _, item := range step4Items(snapshot["items"]) {
		if central10String(item["id"]) == partnerID {
			return central10CopyMap(item)
		}
	}
	return nil
}

func partnerPortalModulesWithPlanContext(source, plans, current map[string]any) map[string]any {
	out := central10CopyMap(source)
	if out == nil {
		out = map[string]any{}
	}
	configured := current["configured"] == true
	currentKey := strings.ToUpper(central10String(current["plan_key"]))
	currentModules := stringSetFromAny(current["active_module_keys"])
	planItems := anyItems(plans["items"])
	currentSort := -1
	for _, plan := range planItems {
		if strings.EqualFold(central10String(plan["plan_key"]), currentKey) {
			currentSort = central10Int(plan["sort_order"])
		}
	}

	enriched := make([]map[string]any, 0, len(anyItems(source["items"])))
	for _, raw := range anyItems(source["items"]) {
		module := central10CopyMap(raw)
		key := central10String(module["key"])
		executable := module["executable"] == true
		availableKeys := []string{}
		availableNames := []string{}
		upgradeKeys := []string{}
		upgradeNames := []string{}
		for _, plan := range planItems {
			if plan["customer_selectable"] != true || plan["active"] != true || plan["ready"] != true {
				continue
			}
			planKey := strings.ToUpper(central10String(plan["plan_key"]))
			planName := central10String(plan["display_name"])
			mode := strings.ToUpper(central10String(plan["selection_mode"]))
			inPlan := false
			switch mode {
			case "FIXED":
				inPlan = stringSetFromAny(plan["fixed_module_keys"])[key]
			case "SELECTABLE", "UNLIMITED":
				inPlan = executable
			}
			if !inPlan {
				continue
			}
			availableKeys = append(availableKeys, planKey)
			availableNames = append(availableNames, planName)
			if configured && central10Int(plan["sort_order"]) > currentSort {
				upgradeKeys = append(upgradeKeys, planKey)
				upgradeNames = append(upgradeNames, planName)
			}
		}
		module["available_in_plans"] = availableKeys
		module["available_in_plan_names"] = availableNames
		module["upgrade_plan_keys"] = upgradeKeys
		module["upgrade_plan_names"] = upgradeNames
		module["in_current_plan"] = configured && currentModules[key]
		if len(upgradeKeys) > 0 {
			module["recommended_upgrade_plan"] = upgradeKeys[0]
			module["recommended_upgrade_plan_name"] = upgradeNames[0]
		}
		module["current_plan_key"] = currentKey
		enriched = append(enriched, module)
	}
	out["items"] = enriched
	out["count"] = len(enriched)
	out["plan_context_available"] = true
	out["current_plan_key"] = currentKey
	out["current_plan_display_name"] = current["display_name"]
	return out
}

func partnerPortalPlanModulesFromPlan(partnerID string, current map[string]any) map[string]any {
	configured := current["configured"] == true
	keys := []string{}
	if configured {
		for key := range stringSetFromAny(current["active_module_keys"]) {
			if strings.TrimSpace(key) != "" {
				keys = append(keys, key)
			}
		}
		sort.Strings(keys)
	}
	selectionMode := strings.ToUpper(central10String(current["selection_mode"]))
	entitlementMode := strings.ToUpper(central10String(current["entitlement_mode"]))
	if entitlementMode == "" {
		entitlementMode = selectionMode
	}
	var moduleLimit any
	if configured {
		moduleLimit = current["module_limit"]
	}
	return map[string]any{
		"partner_id":        partnerID,
		"plan_key":          central10String(current["plan_key"]),
		"selection_mode":    selectionMode,
		"entitlement_mode":  entitlementMode,
		"module_limit":      moduleLimit,
		"module_keys":       keys,
		"count":             len(keys),
		"unlimited_modules": configured && current["unlimited_modules"] == true,
	}
}

func partnerPortalSelectablePlans(source map[string]any) map[string]any {
	out := central10CopyMap(source)
	if out == nil {
		out = map[string]any{}
	}
	filtered := []map[string]any{}
	for _, item := range anyItems(source["items"]) {
		if item["customer_selectable"] == true && item["active"] == true {
			filtered = append(filtered, central10CopyMap(item))
		}
	}
	out["items"] = filtered
	out["count"] = len(filtered)
	return out
}

func (a *app) materializePartnerUserPolicies(ctx context.Context, partnerID string, users []map[string]any, modules map[string]any) (map[string]any, map[string]partnerUserModulePolicy, error) {
	owned := map[string]map[string]any{}
	ownedKeys := []string{}
	for _, item := range anyItems(modules["items"]) {
		key := central10String(item["key"])
		if key == "" {
			continue
		}
		if strings.EqualFold(central10String(item["access_state"]), "ACTIVE") && item["executable"] == true {
			owned[key] = item
			ownedKeys = append(ownedKeys, key)
		}
	}
	sort.Strings(ownedKeys)

	selectedByUser := map[string]map[string]bool{}
	rows, err := a.db.QueryContext(ctx,
		`SELECT user_id,module_key
		 FROM identity.partner_user_modules
		 WHERE partner_id=$1
		 ORDER BY user_id,module_key`,
		partnerID,
	)
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var userID, moduleKey string
		if err := rows.Scan(&userID, &moduleKey); err != nil {
			rows.Close()
			return nil, nil, err
		}
		if selectedByUser[userID] == nil {
			selectedByUser[userID] = map[string]bool{}
		}
		selectedByUser[userID][moduleKey] = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, nil, err
	}
	rows.Close()

	serialized := map[string]any{}
	runtime := map[string]partnerUserModulePolicy{}
	for _, user := range users {
		userID := central10String(user["id"])
		if userID == "" {
			continue
		}
		mode := normalizePartnerModuleAccessMode(central10String(user["module_access_mode"]))
		selected := selectedByUser[userID]
		if selected == nil {
			selected = map[string]bool{}
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
		policy := partnerUserModulePolicy{
			Mode: mode, Selected: selected, Owned: owned, OwnedKeys: ownedKeys,
			Effective: effective, Stale: stale,
		}
		runtime[userID] = policy
		serialized[userID] = partnerUserModulePolicyMap(policy)
	}
	return serialized, runtime, nil
}

func (a *app) materializeCentralPartnerWorkspace(ctx context.Context, partnerID string) map[string]any {
	partner := a.partnerWorkspaceBasePartner(partnerID)
	var livePartner, billing, terms, license, agreement, commercialStatus map[string]any
	var companyProfile, paymentProfile, websiteAdapter, partnerDesign map[string]any
	var modules, documents, invoices, subscriptions, environments, provisioningJobs map[string]any
	var impactSummary, evidenceItems, connectorCredentials, billingEvents map[string]any
	var moduleCommercialHistory map[string]any
	var start22SummaryAll, start22SummaryProduction, start22SummaryStaging, start22Retention map[string]any
	var portalGate, portalModulesEN, portalModulesHU, portalPlansRaw, portalPlan map[string]any
	var portalCharity, portalCharityModules, portalDesignMedia map[string]any
	var portalInvoices map[string]any
	var tenantFinancePolicy, tenantFinanceInvoices map[string]any
	portalUsers := []map[string]any{}

	unavailable := []string{}
	var unavailableMu sync.Mutex
	mark := func(name string, err error) {
		if err == nil {
			return
		}
		unavailableMu.Lock()
		unavailable = append(unavailable, name)
		unavailableMu.Unlock()
	}

	var wg sync.WaitGroup
	runMap := func(name, service, path string, dst *map[string]any) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mark(name, a.internalGET(ctx, a.hosts[service], path, dst))
		}()
	}
	runTenantMap := func(name, service, path string, headers map[string]string, dst *map[string]any) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mark(name, a.internalGETWithHeaders(ctx, a.hosts[service], path, headers, dst))
		}()
	}

	escapedID := url.PathEscape(partnerID)
	runMap("partner", "partners", "/api/v1/partners/"+escapedID, &livePartner)
	runMap("modules", "catalog", "/api/v1/partners/"+escapedID+"/modules", &modules)
	runMap("module_commercial_history", "catalog", "/internal/v1/read-model/partner-module-history/"+escapedID, &moduleCommercialHistory)

	base := "/api/v1/billing/partners/" + escapedID
	runMap("billing_summary", "billing", base+"/summary", &billing)
	runMap("company_profile", "billing", "/api/v1/billing/profile", &companyProfile)
	runMap("billing_terms", "billing", base+"/terms", &terms)
	runMap("license", "billing", base+"/license", &license)
	runMap("documents", "billing", base+"/documents", &documents)
	runMap("invoices", "billing", base+"/invoices", &invoices)
	runMap("subscriptions", "billing", base+"/subscriptions", &subscriptions)
	runMap("agreement", "billing", base+"/agreement", &agreement)
	runMap("commercial_status", "billing", base+"/commercial-status", &commercialStatus)
	runMap("billing_events", "billing", base+"/events", &billingEvents)
	runMap("portal_gate", "billing", "/internal/v1/partners/"+escapedID+"/portal-gate", &portalGate)
	runMap("portal_plans", "billing", "/api/v1/billing/plans", &portalPlansRaw)
	runMap("portal_plan", "billing", base+"/plan", &portalPlan)
	runMap("portal_charity", "billing", base+"/commercial-mode", &portalCharity)
	runMap("portal_charity_modules", "billing", base+"/charity/modules", &portalCharityModules)
	runMap("portal_invoices", "billing", base+"/invoices?partner_visible=true", &portalInvoices)

	runMap("environments", "environments", "/api/v1/environments?partner_id="+url.QueryEscape(partnerID), &environments)
	runMap("provisioning", "provisioning", "/api/v1/provisioning/jobs?partner_id="+url.QueryEscape(partnerID), &provisioningJobs)
	runMap("impact", "impact", "/api/v1/impact/summary?partner_id="+url.QueryEscape(partnerID), &impactSummary)
	runMap("evidence", "evidence", "/api/v1/evidence?partner_id="+url.QueryEscape(partnerID)+"&limit=200&offset=0", &evidenceItems)
	runMap("connector_credentials", "connector", "/api/v1/connectors/"+escapedID+"/credential", &connectorCredentials)
	runMap("website_adapter", "connector", "/api/v1/connectors/"+escapedID+"/website-adapter?environment=PRODUCTION", &websiteAdapter)
	runMap("start22_summary", "connector", "/api/v1/connectors/start22/summary?partner_id="+url.QueryEscape(partnerID), &start22SummaryAll)
	runMap("start22_summary_production", "connector", "/api/v1/connectors/start22/summary?partner_id="+url.QueryEscape(partnerID)+"&environment=PRODUCTION", &start22SummaryProduction)
	runMap("start22_summary_staging", "connector", "/api/v1/connectors/start22/summary?partner_id="+url.QueryEscape(partnerID)+"&environment=STAGING", &start22SummaryStaging)
	runMap("start22_retention", "connector", "/api/v1/connectors/start22/retention?partner_id="+url.QueryEscape(partnerID), &start22Retention)
	runMap("partner_design", "cms", "/internal/v1/cms/partner-design/"+escapedID, &partnerDesign)
	runMap("portal_design_media", "cms", "/internal/v1/cms/partner-media/"+escapedID, &portalDesignMedia)
	runMap("payment_profile", "payments", "/api/v1/payments/partners/"+escapedID+"/profile", &paymentProfile)
	runMap("portal_modules_en", "catalog", "/internal/v1/partner-portal/"+escapedID+"/modules?locale=en_US", &portalModulesEN)
	runMap("portal_modules_hu", "catalog", "/internal/v1/partner-portal/"+escapedID+"/modules?locale=hu_HU", &portalModulesHU)

	tenantHeaders := map[string]string{
		"X-Himate-Partner-ID": partnerID,
		"X-Himate-User-ID":    "read-model-worker",
	}
	runTenantMap("tenant_finance_policy", "tenant-finance", "/internal/v1/tenant-finance/policy", tenantHeaders, &tenantFinancePolicy)
	runTenantMap("tenant_finance_invoices", "tenant-finance", "/internal/v1/tenant-finance/invoices", tenantHeaders, &tenantFinanceInvoices)

	wg.Add(1)
	go func() {
		defer wg.Done()
		items, err := a.listPartnerUsers(partnerID)
		if err != nil {
			mark("portal_users", err)
			return
		}
		portalUsers = items
	}()
	wg.Wait()

	if livePartner != nil {
		partner = livePartner
	} else if partner != nil {
		filtered := unavailable[:0]
		for _, name := range unavailable {
			if name != "partner" {
				filtered = append(filtered, name)
			}
		}
		unavailable = filtered
	}
	if partner == nil {
		unavailable = append(unavailable, "partner")
	}

	// /plan already carries the authoritative entitlement set. Build the
	// historical /plan/modules REST shape locally instead of adding a second
	// Billing dependency that can turn an otherwise complete tenant snapshot
	// partial during startup.
	portalPlanModules := partnerPortalPlanModulesFromPlan(partnerID, portalPlan)

	portalModulesEN = partnerPortalModulesWithPlanContext(portalModulesEN, portalPlansRaw, portalPlan)
	portalModulesHU = partnerPortalModulesWithPlanContext(portalModulesHU, portalPlansRaw, portalPlan)
	portalPlans := partnerPortalSelectablePlans(portalPlansRaw)

	userPolicies, runtimePolicies, policyErr := a.materializePartnerUserPolicies(ctx, partnerID, portalUsers, portalModulesEN)
	mark("portal_user_module_policies", policyErr)

	portalNotifications := map[string]any{}
	if policyErr == nil {
		var notificationsMu sync.Mutex
		var notificationWG sync.WaitGroup
		for _, rawUser := range portalUsers {
			user := rawUser
			userID := central10String(user["id"])
			if userID == "" || user["active"] != true {
				continue
			}
			policy := runtimePolicies[userID]
			notificationWG.Add(1)
			go func() {
				defer notificationWG.Done()
				var out map[string]any
				headers := map[string]string{
					"X-Himate-User-ID":            userID,
					"X-Himate-Partner-ID":         partnerID,
					"X-Himate-Permissions":        strings.Join(partnerPermissions(central10String(user["role"])), ","),
					"X-Himate-Module-Keys":        strings.Join(policy.Effective, ","),
					"X-Himate-Notification-Scope": "PARTNER",
				}
				err := a.internalGETWithHeaders(ctx, a.hosts["notifications"], "/api/v1/notifications?limit=100", headers, &out)
				if err != nil {
					mark("portal_notifications:"+userID, err)
					return
				}
				notificationsMu.Lock()
				portalNotifications[userID] = out
				notificationsMu.Unlock()
			}()
		}
		notificationWG.Wait()
	}

	partnerAuditEvents := []map[string]any{}
	rows, auditErr := a.db.QueryContext(ctx,
		`SELECT id,actor_id,actor_name,actor_roles,request_id,correlation_id,action,method,path,resource,partner_id,status,outcome,old_state,new_state,duration_ms,created_at
		 FROM identity.audit_events
		 WHERE partner_id=$1
		 ORDER BY created_at DESC,id DESC
		 LIMIT 500`,
		partnerID,
	)
	if auditErr == nil {
		for rows.Next() {
			var id int64
			var actorID, actorName, requestID, correlationID, action, method, path, resource, rowPartnerID, outcome string
			var rolesRaw, oldRaw, newRaw []byte
			var statusCode int
			var duration int64
			var created time.Time
			if err := rows.Scan(&id, &actorID, &actorName, &rolesRaw, &requestID, &correlationID, &action, &method, &path, &resource, &rowPartnerID, &statusCode, &outcome, &oldRaw, &newRaw, &duration, &created); err != nil {
				auditErr = err
				break
			}
			var roles []string
			var oldState, newState any
			_ = json.Unmarshal(rolesRaw, &roles)
			_ = json.Unmarshal(oldRaw, &oldState)
			_ = json.Unmarshal(newRaw, &newState)
			partnerAuditEvents = append(partnerAuditEvents, map[string]any{
				"id": id, "actor_id": actorID, "actor_name": actorName, "actor_roles": roles,
				"request_id": requestID, "correlation_id": correlationID, "action": action,
				"method": method, "path": path, "resource": resource, "partner_id": rowPartnerID,
				"status": statusCode, "outcome": outcome, "old_state": oldState, "new_state": newState,
				"duration_ms": duration, "created_at": created.UTC(),
			})
		}
		if err := rows.Err(); err != nil && auditErr == nil {
			auditErr = err
		}
		rows.Close()
	}
	mark("partner_audit_events", auditErr)

	moduleView := central10PartnerModuleView(anyItems(modules["items"]), anyItems(subscriptions["items"]), "", "ALL")
	productionEnvironment := central10ProductionEnvironment(anyItems(environments["items"]))
	preferredConnectorEnvironment := "STAGING"
	if productionEnvironment != nil {
		preferredConnectorEnvironment = "PRODUCTION"
	}

	status := "healthy"
	if len(unavailable) > 0 {
		status = "partial"
	}
	contacts := []map[string]any{}
	if partner != nil {
		primary := map[string]any{
			"kind": "PRIMARY",
			"name": central10String(partner["contact_name"]),
			"email": central10String(partner["contact_email"]),
			"phone": central10String(partner["contact_phone"]),
		}
		if primary["name"] != "" || primary["email"] != "" || primary["phone"] != "" {
			contacts = append(contacts, primary)
		}
	}
	for _, portalUser := range portalUsers {
		contacts = append(contacts, map[string]any{
			"kind": "PORTAL_USER",
			"user_id": portalUser["id"],
			"name": portalUser["name"],
			"email": portalUser["email"],
			"role": portalUser["role"],
			"active": portalUser["active"],
		})
	}

	return map[string]any{
		"status":                          status,
		"unavailable":                     unavailable,
		"partner":                         partner,
		"modules":                         moduleView["items"],
		"module_view":                     moduleView,
		"production_environment":          productionEnvironment,
		"preferred_connector_environment": preferredConnectorEnvironment,
		"billing":                         billing,
		"company_profile":                 companyProfile,
		"terms":                           terms,
		"license":                         license,
		"documents":                       anyItems(documents["items"]),
		"invoices":                        anyItems(invoices["items"]),
		"subscriptions":                   anyItems(subscriptions["items"]),
		"environments":                    anyItems(environments["items"]),
		"provisioning_jobs":               anyItems(provisioningJobs["items"]),
		"impact_summary":                  anyItems(impactSummary["items"]),
		"evidence":                        anyItems(evidenceItems["items"]),
		"connector_credentials":           anyItems(connectorCredentials["items"]),
		"portal_users":                    portalUsers,
		"agreement":                       agreement,
		"commercial_status":               commercialStatus,
		"billing_events":                  anyItems(billingEvents["items"]),
		"website_adapter":                 websiteAdapter,
		"partner_design":                  partnerDesign,
		"payment_profile":                 paymentProfile,
		"catalog_modules_api":             modules,
		"environments_api":                environments,
		"provisioning_api":                provisioningJobs,
		"impact_api":                      impactSummary,
		"evidence_api":                    evidenceItems,
		"connector_credentials_api":       connectorCredentials,
		"portal_users_api": map[string]any{
			"partner_id": partnerID,
			"items":      portalUsers,
			"count":      len(portalUsers),
		},
		"portal_gate":                     portalGate,
		"portal_modules":                  map[string]any{"en_US": portalModulesEN, "hu_HU": portalModulesHU},
		"portal_plans":                    portalPlans,
		"portal_plan":                     portalPlan,
		"portal_plan_modules":             portalPlanModules,
		"portal_charity":                  portalCharity,
		"portal_charity_modules":          portalCharityModules,
		"portal_design_media":             portalDesignMedia,
		"portal_billing_subscriptions":    subscriptions,
		"portal_user_module_policies":     userPolicies,
		"portal_notifications":            portalNotifications,
		"portal_billing_invoices":         portalInvoices,
		"portal_impact":                   impactSummary,
		"tenant_finance": map[string]any{
			"policy":   tenantFinancePolicy,
			"invoices": tenantFinanceInvoices,
		},
		"partner_audit_events": partnerAuditEvents,
		"partner_contacts": contacts,
		"partner_domains_deployments": map[string]any{
			"items": anyItems(environments["items"]),
			"count": len(anyItems(environments["items"])),
		},
		"partner_permissions": map[string]any{
			"users": portalUsers,
			"module_policies": userPolicies,
		},
		"module_commercial_history": moduleCommercialHistory,
		"start22_summary": map[string]any{
			"ALL": start22SummaryAll,
			"PRODUCTION": start22SummaryProduction,
			"STAGING": start22SummaryStaging,
		},
		"start22_retention": start22Retention,
	}
}

func (a *app) refreshCentralPartnerWorkspaceLocked(ctx context.Context, partnerID string) {
	payload := a.materializeCentralPartnerWorkspace(ctx, partnerID)
	persistCtx, cancel := context.WithTimeout(context.Background(), readModelPersistBudget)
	defer cancel()
	a.centralStep3Store(persistCtx, centralPartnerWorkspaceKey(partnerID), payload)
}

func (a *app) refreshCentralPartnerWorkspace(ctx context.Context, partnerID string) {
	partnerID = strings.TrimSpace(partnerID)
	if partnerID == "" {
		return
	}
	key := centralPartnerWorkspaceKey(partnerID)
	if !centralStep3BeginRefresh(key) {
		return
	}
	defer centralStep3EndRefresh(key)
	a.refreshCentralPartnerWorkspaceLocked(ctx, partnerID)
}

func (a *app) writeThroughCentralPartnerWorkspace(partnerID string) {
	partnerID = strings.TrimSpace(partnerID)
	if partnerID == "" {
		return
	}
	key := centralPartnerWorkspaceKey(partnerID)
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 2*centralPartnerWorkspaceMaterializeBudget)
	defer waitCancel()
	if !centralStep3WaitBeginRefresh(waitCtx, key) {
		if a.log != nil {
			a.log.Error("tenant write-through could not acquire projection lock", "partner_id", partnerID)
		}
		return
	}
	defer centralStep3EndRefresh(key)

	refreshCtx, refreshCancel := context.WithTimeout(context.Background(), centralPartnerWorkspaceMaterializeBudget)
	defer refreshCancel()
	a.refreshCentralPartnerWorkspaceLocked(refreshCtx, partnerID)
}

func (a *app) requestCentralPartnerWorkspaceRefresh(partnerID string) {
	partnerID = strings.TrimSpace(partnerID)
	if partnerID == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), centralPartnerWorkspaceMaterializeBudget)
		defer cancel()
		a.refreshCentralPartnerWorkspace(ctx, partnerID)
	}()
}

func (a *app) refreshCentralPartnerWorkspaceSnapshots(ctx context.Context, missingOnly bool) {
	ids := a.centralPartnerIDsForMaterialization(ctx)
	if len(ids) == 0 {
		return
	}

	jobs := make(chan string)
	var wg sync.WaitGroup
	workers := centralPartnerWorkspaceMaterializeWorkers
	if len(ids) < workers {
		workers = len(ids)
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for partnerID := range jobs {
				if missingOnly {
					if _, _, ok := centralStep3SnapshotGet(centralPartnerWorkspaceKey(partnerID)); ok {
						continue
					}
				}
				if ctx.Err() != nil {
					return
				}
				partnerCtx, cancel := context.WithTimeout(ctx, centralPartnerWorkspaceMaterializeBudget)
				a.refreshCentralPartnerWorkspace(partnerCtx, partnerID)
				cancel()
			}
		}()
	}
	for _, partnerID := range ids {
		if ctx.Err() != nil {
			break
		}
		jobs <- partnerID
	}
	close(jobs)
	wg.Wait()
}

func (a *app) warmMissingCentralPartnerWorkspaces() {
	ctx, cancel := context.WithTimeout(context.Background(), centralPartnerWorkspaceStartupBudget)
	defer cancel()
	a.refreshCentralPartnerWorkspaceSnapshots(ctx, true)
	if ctx.Err() != nil && a.log != nil {
		a.log.Warn("central partner workspace startup warmup reached budget; background refresh will continue")
	}
}

func (a *app) requestAllCentralPartnerWorkspaceRefreshes() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), centralPartnerWorkspaceStartupBudget)
		defer cancel()
		a.refreshCentralPartnerWorkspaceSnapshots(ctx, false)
	}()
}

func (a *app) runCentralPartnerWorkspaceMaterializer() {
	ctx, cancel := context.WithTimeout(context.Background(), centralPartnerWorkspaceStartupBudget)
	a.refreshCentralPartnerWorkspaceSnapshots(ctx, false)
	cancel()

	ticker := time.NewTicker(centralPartnerWorkspaceRefreshInterval)
	defer ticker.Stop()
	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), centralPartnerWorkspaceRefreshInterval)
		a.refreshCentralPartnerWorkspaceSnapshots(ctx, false)
		cancel()
	}
}
