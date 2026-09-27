package main

import (
	"net/http"
	"strconv"
	"strings"

	"himate.local/services/internal/common"
)

func materializedPage(items []map[string]any, limit, offset int) map[string]any {
	if offset < 0 {
		offset = 0
	}
	if offset > len(items) {
		offset = len(items)
	}
	if limit < 1 {
		limit = len(items)
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return map[string]any{
		"items": items[offset:end],
		"count": end - offset,
		"total": len(items),
		"limit": limit,
		"offset": offset,
		"has_more": end < len(items),
	}
}

func queryInt(value string, fallback, maximum int) int {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || n < 0 {
		return fallback
	}
	if maximum > 0 && n > maximum {
		return maximum
	}
	return n
}

func filterMaterializedDocuments(items []map[string]any, q string) []map[string]any {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return items
	}
	out := []map[string]any{}
	for _, item := range items {
		if searchContains(q,
			item["id"], item["name"], item["title"], item["reference"],
			item["kind"], item["document_type"], item["note"], item["partner_id"],
		) {
			out = append(out, item)
		}
	}
	return out
}

func filterMaterializedAudit(items []map[string]any, r *http.Request) []map[string]any {
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	partnerID := strings.TrimSpace(r.URL.Query().Get("partner_id"))
	actorID := strings.TrimSpace(r.URL.Query().Get("actor_id"))
	resource := strings.TrimSpace(r.URL.Query().Get("resource"))
	action := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("action")))
	method := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("method")))
	outcome := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("outcome")))
	out := []map[string]any{}
	for _, row := range items {
		if partnerID != "" && central10String(row["partner_id"]) != partnerID {
			continue
		}
		if actorID != "" && central10String(row["actor_id"]) != actorID {
			continue
		}
		if resource != "" && central10String(row["resource"]) != resource {
			continue
		}
		if action != "" && strings.ToUpper(central10String(row["action"])) != action {
			continue
		}
		if method != "" && strings.ToUpper(central10String(row["method"])) != method {
			continue
		}
		if outcome != "" && strings.ToUpper(central10String(row["outcome"])) != outcome {
			continue
		}
		if q != "" && !searchContains(q,
			row["actor_name"], row["actor_id"], row["path"], row["resource"],
			row["request_id"], row["correlation_id"], row["action"], row["partner_id"],
		) {
			continue
		}
		out = append(out, central10CopyMap(row))
	}
	return out
}

func localizedAdministrationRoles(raw map[string]any, r *http.Request) map[string]any {
	locale := common.RequestLocale(r)
	items := []map[string]any{}
	for _, source := range anyItems(raw["items"]) {
		item := central10CopyMap(source)
		labelEN := central10String(item["label_en"])
		labelHU := central10String(item["label_hu"])
		descEN := central10String(item["description_en"])
		descHU := central10String(item["description_hu"])
		item["label"] = common.Localized(labelEN, labelHU, locale)
		item["description"] = common.Localized(descEN, descHU, locale)
		items = append(items, item)
	}
	return map[string]any{"items": items, "count": len(items), "locale": locale}
}

func (a *app) materializedPartnerList(r *http.Request) (map[string]any, bool) {
	snapshot, _, ok := a.centralSnapshotForRead(r.Context(), centralStep4PartnersKey)
	if !ok {
		return nil, false
	}
	all := step4Items(snapshot["items"])
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	lifecycle := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("lifecycle")))
	category := strings.TrimSpace(r.URL.Query().Get("category"))
	referenceOnly := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("reference")), "true")
	filtered := []map[string]any{}
	for _, raw := range all {
		if lifecycle != "" && lifecycle != "ALL" && !strings.EqualFold(lifecycle, central10String(raw["lifecycle"])) {
			continue
		}
		if category != "" && category != "ALL" &&
			!strings.EqualFold(category, central10String(raw["category_id"])) &&
			!strings.EqualFold(category, central10String(raw["category_key"])) {
			continue
		}
		if referenceOnly && raw["reference_partner"] != true {
			continue
		}
		if q != "" && !searchContains(q,
			raw["id"], raw["display_name"], raw["legal_name"], raw["brand_name"],
			raw["category_name"], raw["country"], raw["state_region"], raw["city"],
			raw["contact_name"], raw["contact_email"],
		) {
			continue
		}
		filtered = append(filtered, central10CopyMap(raw))
	}
	limit := queryInt(r.URL.Query().Get("limit"), 100, 200)
	offset := queryInt(r.URL.Query().Get("offset"), 0, 1_000_000)
	out := materializedPage(filtered, limit, offset)
	kpis := step4Map(snapshot["kpis"])
	out["lifecycle_counts"] = kpis["lifecycle_counts"]
	out["reference_count"] = kpis["reference_partners"]
	return out, true
}

func materializedModuleByKey(snapshot map[string]any, key string) (map[string]any, bool) {
	for _, module := range anyItems(snapshot["modules"]) {
		if central10String(module["key"]) == key {
			return central10CopyMap(module), true
		}
	}
	return nil, false
}

// serveCentralMaterializedGET keeps the established REST paths but replaces
// synchronous service proxies/DB aggregation with persistent CQRS projections.
// It is called only for GETs; writes retain their authoritative owner service.
func (a *app) serveCentralMaterializedGET(w http.ResponseWriter, r *http.Request, actor user) bool {
	if r.Method != http.MethodGet {
		return false
	}
	path := r.URL.Path

	switch {
	case path == "/api/v1/partners" || path == "/api/v1/partners/portfolio":
		out, ok := a.materializedPartnerList(r)
		if !ok {
			common.APIError(w, http.StatusServiceUnavailable, "READ_MODEL_NOT_READY", "Partner read model is not ready")
			return true
		}
		w.Header().Set("X-Himate-Cache", "persistent-read-model")
		common.JSON(w, http.StatusOK, out)
		return true

	case strings.HasPrefix(path, "/api/v1/partners/") && !strings.Contains(strings.TrimPrefix(path, "/api/v1/partners/"), "/"):
		partnerID := strings.Trim(strings.TrimPrefix(path, "/api/v1/partners/"), "/")
		snapshot, _, ok := a.partnerWorkspaceForRead(r.Context(), partnerID)
		if !ok {
			common.APIError(w, http.StatusNotFound, "PARTNER_NOT_FOUND", "Partner not found")
			return true
		}
		w.Header().Set("X-Himate-Cache", "persistent-tenant-read-model")
		common.JSON(w, http.StatusOK, partnerWorkspaceMap(snapshot, "partner"))
		return true

	case path == "/api/v1/modules":
		snapshot, _, ok := a.centralSnapshotForRead(r.Context(), centralStep3RegistryKey)
		if !ok {
			common.APIError(w, http.StatusServiceUnavailable, "READ_MODEL_NOT_READY", "Module registry is not ready")
			return true
		}
		items := step4Items(snapshot["modules"])
		out := materializedPage(items, len(items), 0)
		w.Header().Set("X-Himate-Cache", "persistent-read-model")
		common.JSON(w, http.StatusOK, out)
		return true

	case path == "/api/v1/module-groups":
		snapshot, _, ok := a.centralSnapshotForRead(r.Context(), centralStep3RegistryKey)
		if !ok {
			common.APIError(w, http.StatusServiceUnavailable, "READ_MODEL_NOT_READY", "Module registry is not ready")
			return true
		}
		items := step4Items(snapshot["groups"])
		w.Header().Set("X-Himate-Cache", "persistent-read-model")
		common.JSON(w, http.StatusOK, map[string]any{"items": items, "count": len(items)})
		return true

	case strings.HasPrefix(path, "/api/v1/modules/") && !strings.Contains(strings.TrimPrefix(path, "/api/v1/modules/"), "/"):
		key := strings.Trim(strings.TrimPrefix(path, "/api/v1/modules/"), "/")
		snapshot, _, ok := a.centralSnapshotForRead(r.Context(), centralStep3RegistryKey)
		if !ok {
			common.APIError(w, http.StatusServiceUnavailable, "READ_MODEL_NOT_READY", "Module registry is not ready")
			return true
		}
		item, found := materializedModuleByKey(snapshot, key)
		if !found {
			common.APIError(w, http.StatusNotFound, "NOT_FOUND", "Module not found")
			return true
		}
		w.Header().Set("X-Himate-Cache", "persistent-read-model")
		common.JSON(w, http.StatusOK, item)
		return true

	case path == "/api/v1/module-commercial-matrix":
		snapshot, _, ok := a.centralSnapshotForRead(r.Context(), centralStep3CommercialKey)
		if !ok {
			common.APIError(w, http.StatusServiceUnavailable, "READ_MODEL_NOT_READY", "Commercial matrix is not ready")
			return true
		}
		items := step4Items(snapshot["matrix_items"])
		w.Header().Set("X-Himate-Cache", "persistent-read-model")
		common.JSON(w, http.StatusOK, map[string]any{"items": items, "count": len(items)})
		return true
	}

	administrationNeeded := path == "/api/v1/admin/roles" ||
		path == "/api/v1/admin/users" ||
		path == "/api/v1/admin/secrets" ||
		path == "/api/v1/audit/events" ||
		path == "/api/v1/billing/profile" ||
		path == "/api/v1/billing/invoices" ||
		path == "/api/v1/billing/company/documents" ||
		path == "/api/v1/backups/summary" ||
		(strings.HasPrefix(path, "/api/v1/billing/partners/") && strings.HasSuffix(path, "/documents"))
	if administrationNeeded {
		snapshot, _, ok := a.centralSnapshotForRead(r.Context(), centralStep4AdministrationKey)
		if !ok {
			common.APIError(w, http.StatusServiceUnavailable, "READ_MODEL_NOT_READY", "Administration read model is not ready")
			return true
		}
		w.Header().Set("X-Himate-Cache", "persistent-read-model")
		switch {
		case path == "/api/v1/admin/roles":
			common.JSON(w, http.StatusOK, localizedAdministrationRoles(partnerWorkspaceMap(snapshot, "admin_roles"), r))
		case path == "/api/v1/admin/users":
			if !ownerRequired(w, actor) {
				return true
			}
			common.JSON(w, http.StatusOK, partnerWorkspaceMap(snapshot, "admin_users"))
		case path == "/api/v1/admin/secrets":
			common.JSON(w, http.StatusOK, partnerWorkspaceMap(snapshot, "admin_secrets"))
		case path == "/api/v1/audit/events":
			audit := partnerWorkspaceMap(snapshot, "audit_events")
			filtered := filterMaterializedAudit(anyItems(audit["items"]), r)
			limit := queryInt(r.URL.Query().Get("limit"), 50, 200)
			offset := queryInt(r.URL.Query().Get("offset"), 0, 1_000_000)
			common.JSON(w, http.StatusOK, materializedPage(filtered, limit, offset))
		case path == "/api/v1/billing/profile":
			company := partnerWorkspaceMap(snapshot, "company")
			common.JSON(w, http.StatusOK, partnerWorkspaceMap(company, "profile"))
		case path == "/api/v1/billing/invoices":
			raw := partnerWorkspaceMap(snapshot, "invoice_register")
			items := anyItems(raw["items"])
			if partnerID := strings.TrimSpace(r.URL.Query().Get("partner_id")); partnerID != "" {
				filtered := []map[string]any{}
				for _, item := range items {
					if central10String(item["partner_id"]) == partnerID {
						filtered = append(filtered, item)
					}
				}
				items = filtered
			}
			out := central10CopyMap(raw)
			if out == nil { out = map[string]any{} }
			out["items"] = items
			out["count"] = len(items)
			common.JSON(w, http.StatusOK, out)
		case path == "/api/v1/billing/company/documents":
			raw := partnerWorkspaceMap(snapshot, "company_documents")
			items := filterMaterializedDocuments(anyItems(raw["items"]), r.URL.Query().Get("q"))
			out := central10CopyMap(raw)
			if out == nil { out = map[string]any{} }
			out["items"] = items
			out["count"] = len(items)
			common.JSON(w, http.StatusOK, out)
		case strings.HasPrefix(path, "/api/v1/billing/partners/") && strings.HasSuffix(path, "/documents"):
			rawID := strings.TrimSuffix(strings.TrimPrefix(path, "/api/v1/billing/partners/"), "/documents")
			partnerID := strings.Trim(rawID, "/")
			tenant, _, tenantOK := a.partnerWorkspaceForRead(r.Context(), partnerID)
			if !tenantOK {
				common.APIError(w, http.StatusNotFound, "PARTNER_NOT_FOUND", "Partner not found")
				return true
			}
			items := filterMaterializedDocuments(partnerWorkspaceItems(tenant, "documents"), r.URL.Query().Get("q"))
			common.JSON(w, http.StatusOK, map[string]any{"items": items, "count": len(items)})
		case path == "/api/v1/backups/summary":
			common.JSON(w, http.StatusOK, partnerWorkspaceMap(snapshot, "backup_api"))
		}
		return true
	}

	websiteNeeded := path == "/api/v1/cms/pages" ||
		path == "/api/v1/cms/media" ||
		path == "/api/v1/cms/seo" ||
		path == "/api/v1/cms/seo/audit" ||
		path == "/api/v1/contact/inquiries" ||
		path == "/api/v1/environments" ||
		strings.HasPrefix(path, "/api/v1/cms/pages/") ||
		strings.HasPrefix(path, "/api/v1/environments/")
	if websiteNeeded {
		snapshot, _, ok := a.centralSnapshotForRead(r.Context(), centralStep4WebsiteKey)
		if !ok {
			common.APIError(w, http.StatusServiceUnavailable, "READ_MODEL_NOT_READY", "Website read model is not ready")
			return true
		}
		w.Header().Set("X-Himate-Cache", "persistent-read-model")
		switch {
		case path == "/api/v1/cms/pages":
			items := step4Items(snapshot["pages"])
			common.JSON(w, http.StatusOK, map[string]any{"items": items, "count": len(items)})
		case path == "/api/v1/cms/media":
			items := step4Items(snapshot["media"])
			common.JSON(w, http.StatusOK, map[string]any{"items": items, "count": len(items)})
		case path == "/api/v1/cms/seo":
			common.JSON(w, http.StatusOK, partnerWorkspaceMap(snapshot, "seo"))
		case path == "/api/v1/cms/seo/audit":
			common.JSON(w, http.StatusOK, partnerWorkspaceMap(snapshot, "seo_audit"))
		case strings.HasPrefix(path, "/api/v1/cms/pages/"):
			raw := strings.Trim(strings.TrimPrefix(path, "/api/v1/cms/pages/"), "/")
			parts := strings.Split(raw, "/")
			pageID := parts[0]
			key := "cms_page_details"
			if len(parts) == 2 && parts[1] == "versions" { key = "cms_page_versions" }
			if len(parts) == 2 && parts[1] == "audit" { key = "cms_page_audits" }
			if len(parts) > 2 || (len(parts) == 2 && parts[1] != "versions" && parts[1] != "audit") {
				return false
			}
			root := partnerWorkspaceMap(snapshot, key)
			item, exists := root[pageID]
			if !exists {
				common.APIError(w, http.StatusNotFound, "NOT_FOUND", "CMS page not found")
				return true
			}
			common.JSON(w, http.StatusOK, item)
		case path == "/api/v1/contact/inquiries":
			raw := partnerWorkspaceMap(snapshot, "contact_inquiries")
			items := anyItems(raw["items"])
			q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
			status := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("status")))
			filtered := []map[string]any{}
			for _, item := range items {
				if status != "" && status != "ALL" && strings.ToUpper(central10String(item["lead_status"])) != status {
					continue
				}
				if q != "" && !searchContains(q,
					item["name"], item["organization"], item["email"], item["message"],
					item["assigned_to"], item["admin_note"], item["organization_type"], item["inquiry_topic"],
				) {
					continue
				}
				filtered = append(filtered, item)
			}
			limit := queryInt(r.URL.Query().Get("limit"), 100, 200)
			offset := queryInt(r.URL.Query().Get("offset"), 0, 1_000_000)
			common.JSON(w, http.StatusOK, materializedPage(filtered, limit, offset))
		case path == "/api/v1/environments":
			items := step4Items(snapshot["environments"])
			if partnerID := strings.TrimSpace(r.URL.Query().Get("partner_id")); partnerID != "" {
				filtered := []map[string]any{}
				for _, item := range items {
					if central10String(item["partner_id"]) == partnerID {
						filtered = append(filtered, item)
					}
				}
				items = filtered
			}
			common.JSON(w, http.StatusOK, map[string]any{"items": items, "count": len(items)})
		case strings.HasPrefix(path, "/api/v1/environments/"):
			id := strings.Trim(strings.TrimPrefix(path, "/api/v1/environments/"), "/")
			for _, item := range step4Items(snapshot["environments"]) {
				if central10String(item["id"]) == id {
					common.JSON(w, http.StatusOK, item)
					return true
				}
			}
			common.APIError(w, http.StatusNotFound, "NOT_FOUND", "Environment not found")
		}
		return true
	}

	return false
}
