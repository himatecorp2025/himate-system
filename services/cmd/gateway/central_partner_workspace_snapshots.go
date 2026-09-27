package main

import (
	"context"
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

func centralPartnerIDsFromSnapshot() []string {
	snapshot, _, ok := centralStep3SnapshotGet(centralStep4PartnersKey)
	if !ok {
		return nil
	}
	ids := make([]string, 0, len(step4Items(snapshot["items"])))
	seen := map[string]bool{}
	for _, partner := range step4Items(snapshot["items"]) {
		id := central10String(partner["id"])
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
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

func (a *app) materializeCentralPartnerWorkspace(ctx context.Context, partnerID string) map[string]any {
	type page struct {
		Items []map[string]any `json:"items"`
	}

	partner := a.partnerWorkspaceBasePartner(partnerID)
	var livePartner, billing, terms, license, agreement, commercialStatus map[string]any
	var paymentProfile, websiteAdapter, partnerDesign map[string]any
	var modules, documents, invoices, subscriptions, environments, provisioningJobs page
	var impactSummary, evidenceItems, connectorCredentials, billingEvents page
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
	runPage := func(name, service, path string, dst *page) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mark(name, a.internalGET(ctx, a.hosts[service], path, dst))
		}()
	}

	escapedID := url.PathEscape(partnerID)
	runMap("partner", "partners", "/api/v1/partners/"+escapedID, &livePartner)
	runPage("modules", "catalog", "/api/v1/partners/"+escapedID+"/modules", &modules)

	base := "/api/v1/billing/partners/" + escapedID
	runMap("billing_summary", "billing", base+"/summary", &billing)
	runMap("billing_terms", "billing", base+"/terms", &terms)
	runMap("license", "billing", base+"/license", &license)
	runPage("documents", "billing", base+"/documents", &documents)
	runPage("invoices", "billing", base+"/invoices", &invoices)
	runPage("subscriptions", "billing", base+"/subscriptions", &subscriptions)
	runMap("agreement", "billing", base+"/agreement", &agreement)
	runMap("commercial_status", "billing", base+"/commercial-status", &commercialStatus)
	runPage("billing_events", "billing", base+"/events", &billingEvents)

	runPage("environments", "environments", "/api/v1/environments?partner_id="+url.QueryEscape(partnerID), &environments)
	runPage("provisioning", "provisioning", "/api/v1/provisioning/jobs?partner_id="+url.QueryEscape(partnerID), &provisioningJobs)
	runPage("impact", "impact", "/api/v1/impact/summary?partner_id="+url.QueryEscape(partnerID), &impactSummary)
	runPage("evidence", "evidence", "/api/v1/evidence?partner_id="+url.QueryEscape(partnerID)+"&limit=50&offset=0", &evidenceItems)
	runPage("connector_credentials", "connector", "/api/v1/connectors/"+escapedID+"/credential", &connectorCredentials)
	runMap("website_adapter", "connector", "/api/v1/connectors/"+escapedID+"/website-adapter?environment=PRODUCTION", &websiteAdapter)
	runMap("partner_design", "cms", "/internal/v1/cms/partner-design/"+escapedID, &partnerDesign)
	runMap("payment_profile", "payments", "/api/v1/payments/partners/"+escapedID+"/profile", &paymentProfile)

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
		// The complete Partners screen is itself LKG. A transient detail-read
		// miss must not invalidate a usable base partner record.
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

	moduleView := central10PartnerModuleView(modules.Items, subscriptions.Items, "", "ALL")
	productionEnvironment := central10ProductionEnvironment(environments.Items)
	preferredConnectorEnvironment := "STAGING"
	if productionEnvironment != nil {
		preferredConnectorEnvironment = "PRODUCTION"
	}

	status := "healthy"
	if len(unavailable) > 0 {
		status = "partial"
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
		"terms":                           terms,
		"license":                         license,
		"documents":                       documents.Items,
		"invoices":                        invoices.Items,
		"subscriptions":                   subscriptions.Items,
		"environments":                    environments.Items,
		"provisioning_jobs":               provisioningJobs.Items,
		"impact_summary":                  impactSummary.Items,
		"evidence":                        evidenceItems.Items,
		"connector_credentials":           connectorCredentials.Items,
		"portal_users":                    portalUsers,
		"agreement":                       agreement,
		"commercial_status":               commercialStatus,
		"billing_events":                  billingEvents.Items,
		"website_adapter":                 websiteAdapter,
		"partner_design":                  partnerDesign,
		"payment_profile":                 paymentProfile,
	}
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

	payload := a.materializeCentralPartnerWorkspace(ctx, partnerID)
	persistCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	a.centralStep3Store(persistCtx, key, payload)
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
	ids := centralPartnerIDsFromSnapshot()
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
