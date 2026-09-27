package main

import (
	"context"
	"time"
)

func (a *app) materializeCentralGlobalSearch(ctx context.Context) map[string]any {
	partners, _, partnersOK := a.centralSnapshotForRead(ctx, centralStep4PartnersKey)
	registry, _, registryOK := a.centralSnapshotForRead(ctx, centralStep3RegistryKey)
	website, _, websiteOK := a.centralSnapshotForRead(ctx, centralStep4WebsiteKey)
	administration, _, administrationOK := a.centralSnapshotForRead(ctx, centralStep4AdministrationKey)
	if !partnersOK || !registryOK || !websiteOK || !administrationOK {
		unavailable := []string{}
		if !partnersOK { unavailable = append(unavailable, "partners") }
		if !registryOK { unavailable = append(unavailable, "modules") }
		if !websiteOK { unavailable = append(unavailable, "website") }
		if !administrationOK { unavailable = append(unavailable, "administration") }
		a.logCentralRefreshFailure(centralStep4GlobalSearchKey, unavailable)
		return map[string]any{
			"status": "partial",
			"unavailable": unavailable,
		}
	}
	return map[string]any{
		"status": "healthy",
		"unavailable": []string{},
		"partners": step4Items(partners["items"]),
		"modules": anyItems(registry["modules"]),
		"contact_inquiries": anyItems(partnerWorkspaceMap(website, "contact_inquiries")["items"]),
		"cms_pages": step4Items(website["pages"]),
		"admin_users": anyItems(partnerWorkspaceMap(administration, "admin_users")["items"]),
		"audit_events": anyItems(partnerWorkspaceMap(administration, "audit_events")["items"]),
	}
}

func (a *app) refreshCentralStep4GlobalSearch() {
	ctx, cancel := context.WithTimeout(context.Background(), centralStep4MaterializeBudget)
	defer cancel()
	payload := a.materializeCentralGlobalSearch(ctx)
	persistCtx, persistCancel := context.WithTimeout(context.Background(), readModelPersistBudget)
	defer persistCancel()
	a.centralStep3Store(persistCtx, centralStep4GlobalSearchKey, payload)
}
