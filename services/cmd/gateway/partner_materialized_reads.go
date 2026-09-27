package main

import (
	"net/http"
	"strconv"
	"strings"

	"himate.local/services/internal/common"
)

func partnerReadLimit(raw string, fallback, maximum int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value < 1 {
		return fallback
	}
	if value > maximum {
		return maximum
	}
	return value
}

func partnerSnapshotModule(snapshot map[string]any, u partnerUser, moduleKey string) (map[string]any, bool) {
	modules := partnerWorkspaceModulesForUser(snapshot, u)
	for _, item := range anyItems(modules["items"]) {
		if central10String(item["key"]) != moduleKey {
			continue
		}
		if item["user_executable"] != true {
			return nil, false
		}
		return central10CopyMap(item), true
	}
	return nil, false
}

func partnerNotificationFeedFromSnapshot(snapshot map[string]any, u partnerUser, r *http.Request) map[string]any {
	root := partnerWorkspaceMap(snapshot, "portal_notifications")
	stored, _ := root[u.ID].(map[string]any)
	if stored == nil {
		return map[string]any{
			"items": []map[string]any{}, "count": 0, "unread_count": 0, "delivery_scope": "PARTNER",
		}
	}
	unreadOnly := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("unread_only")), "true")
	limit := partnerReadLimit(r.URL.Query().Get("limit"), 40, 100)
	items := []map[string]any{}
	unread := 0
	for _, raw := range anyItems(stored["items"]) {
		item := central10CopyMap(raw)
		if item["read"] != true {
			unread++
		}
		if unreadOnly && item["read"] == true {
			continue
		}
		if len(items) < limit {
			items = append(items, item)
		}
	}
	return map[string]any{
		"items": items, "count": len(items), "unread_count": unread, "delivery_scope": "PARTNER",
	}
}

func partnerAuditFeedFromSnapshot(snapshot map[string]any, r *http.Request) map[string]any {
	all := partnerWorkspaceItems(snapshot, "partner_audit_events")
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	resource := strings.TrimSpace(r.URL.Query().Get("resource"))
	action := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("action")))
	method := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("method")))
	outcome := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("outcome")))
	limit := partnerReadLimit(r.URL.Query().Get("limit"), 50, 200)
	offset, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("offset")))
	if offset < 0 {
		offset = 0
	}
	filtered := []map[string]any{}
	for _, row := range all {
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
		if query != "" && !searchContains(query,
			row["actor_name"], row["actor_id"], row["path"], row["resource"],
			row["request_id"], row["correlation_id"], row["action"], row["partner_id"],
		) {
			continue
		}
		filtered = append(filtered, row)
	}
	if offset > len(filtered) {
		offset = len(filtered)
	}
	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}
	return map[string]any{
		"items": filtered[offset:end], "count": end - offset, "total": len(filtered),
		"limit": limit, "offset": offset, "has_more": end < len(filtered),
	}
}

func tenantFinanceMaterializedGET(snapshot map[string]any, path string, u partnerUser) (map[string]any, int, bool) {
	const prefix = "/runtime/modules/invoice_documents/"
	if !strings.HasPrefix(path, prefix) {
		return nil, 0, false
	}
	module, granted := partnerSnapshotModule(snapshot, u, partnerInvoiceModuleKey)
	if !granted {
		return map[string]any{
			"code": "MODULE_NOT_ASSIGNED", "message": "This module is not assigned to the authenticated user",
		}, http.StatusForbidden, true
	}
	suffix := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	if suffix == "access" {
		return map[string]any{
			"partner_id": u.PartnerID, "user_id": u.ID, "module_key": partnerInvoiceModuleKey,
			"module": module, "access_state": "GRANTED",
			"security_rule": "PARTNER_ENTITLEMENT_INTERSECT_USER_ASSIGNMENT",
		}, http.StatusOK, true
	}
	finance := partnerWorkspaceMap(snapshot, "tenant_finance")
	switch suffix {
	case "policy":
		out := partnerWorkspaceMap(finance, "policy")
		out["gateway_tenant_context"] = "AUTHENTICATED_PARTNER_SESSION"
		return out, http.StatusOK, true
	case "invoices":
		out := partnerWorkspaceMap(finance, "invoices")
		out["gateway_tenant_context"] = "AUTHENTICATED_PARTNER_SESSION"
		return out, http.StatusOK, true
	default:
		if strings.HasPrefix(suffix, "invoices/") {
			id := strings.Trim(strings.TrimPrefix(suffix, "invoices/"), "/")
			if id == "" || strings.Contains(id, "/") {
				return nil, 0, false
			}
			for _, invoice := range anyItems(partnerWorkspaceMap(finance, "invoices")["items"]) {
				if central10String(invoice["id"]) == id {
					out := central10CopyMap(invoice)
					out["gateway_tenant_context"] = "AUTHENTICATED_PARTNER_SESSION"
					return out, http.StatusOK, true
				}
			}
			return map[string]any{"code": "NOT_FOUND", "message": "Invoice not found"}, http.StatusNotFound, true
		}
	}
	return nil, 0, false
}

// servePartnerMaterializedGET preserves the existing Partner Portal REST
// contract while making every screen-data GET a single tenant snapshot read.
// It deliberately performs no internalGET/internalJSON/proxy operation.
func (a *app) servePartnerMaterializedGET(w http.ResponseWriter, r *http.Request, u partnerUser, path string) bool {
	if r.Method != http.MethodGet {
		return false
	}

	permission := ""
	switch {
	case path == "/dashboard":
		permission = "dashboard.read"
	case path == "/company":
		permission = "company.read"
	case path == "/plans", path == "/plan", path == "/charity",
		path == "/billing/summary", path == "/billing/subscriptions", path == "/billing/invoices":
		permission = "billing.read"
	case path == "/plan/modules", path == "/charity/modules", path == "/modules",
		strings.HasPrefix(path, "/runtime/modules/"):
		permission = "modules.read"
	case path == "/impact/summary":
		permission = "impact.read"
	case path == "/users", strings.HasPrefix(path, "/users/") && strings.HasSuffix(path, "/modules"),
		path == "/audit", path == "/permissions":
		permission = "users.read"
	case path == "/contacts", path == "/domains", path == "/deployments":
		permission = "company.read"
	case path == "/notifications":
		permission = "notifications.read"
	case path == "/design", path == "/design/media":
		permission = "design.read"
	default:
		return false
	}
	if !a.requirePartnerPermission(w, u, permission) {
		return true
	}

	snapshot, ok := a.partnerWorkspaceRequest(r, u)
	if !ok {
		// Startup/readiness guarantees prevent this state from being reachable
		// on a live instance. Do not fall through to an upstream read.
		common.APIError(w, http.StatusServiceUnavailable, "READ_MODEL_NOT_READY", "Partner workspace is not ready")
		return true
	}
	w.Header().Set("X-Himate-Cache", "persistent-tenant-read-model")

	switch {
	case path == "/dashboard":
		company := partnerWorkspaceMap(snapshot, "partner")
		delete(company, "notes")
		common.JSON(w, http.StatusOK, map[string]any{
			"partner_id": u.PartnerID,
			"company": company,
			"modules": partnerWorkspaceModulesForUser(snapshot, u),
			"billing": partnerWorkspaceMap(snapshot, "billing"),
			"impact": partnerWorkspaceMap(snapshot, "portal_impact"),
			"degraded_sections": []string{},
		})
	case path == "/company":
		out := partnerWorkspaceMap(snapshot, "partner")
		delete(out, "notes")
		common.JSON(w, http.StatusOK, out)
	case path == "/plans":
		common.JSON(w, http.StatusOK, partnerWorkspaceMap(snapshot, "portal_plans"))
	case path == "/plan":
		common.JSON(w, http.StatusOK, partnerWorkspaceMap(snapshot, "portal_plan"))
	case path == "/plan/modules":
		common.JSON(w, http.StatusOK, partnerWorkspaceMap(snapshot, "portal_plan_modules"))
	case path == "/charity":
		common.JSON(w, http.StatusOK, partnerWorkspaceMap(snapshot, "portal_charity"))
	case path == "/charity/modules":
		common.JSON(w, http.StatusOK, partnerWorkspaceMap(snapshot, "portal_charity_modules"))
	case path == "/modules":
		common.JSON(w, http.StatusOK, partnerWorkspaceModulesForUser(snapshot, u))
	case path == "/billing/summary":
		common.JSON(w, http.StatusOK, partnerWorkspaceMap(snapshot, "billing"))
	case path == "/billing/subscriptions":
		common.JSON(w, http.StatusOK, partnerWorkspaceMap(snapshot, "portal_billing_subscriptions"))
	case path == "/billing/invoices":
		common.JSON(w, http.StatusOK, partnerWorkspaceMap(snapshot, "portal_billing_invoices"))
	case path == "/impact/summary":
		common.JSON(w, http.StatusOK, partnerWorkspaceMap(snapshot, "portal_impact"))
	case path == "/contacts":
		items := partnerWorkspaceItems(snapshot, "partner_contacts")
		common.JSON(w, http.StatusOK, map[string]any{"items": items, "count": len(items)})
	case path == "/domains", path == "/deployments":
		common.JSON(w, http.StatusOK, partnerWorkspaceMap(snapshot, "partner_domains_deployments"))
	case path == "/audit":
		common.JSON(w, http.StatusOK, partnerAuditFeedFromSnapshot(snapshot, r))
	case path == "/permissions":
		common.JSON(w, http.StatusOK, partnerWorkspaceMap(snapshot, "partner_permissions"))
	case path == "/users":
		items := partnerWorkspaceItems(snapshot, "portal_users")
		common.JSON(w, http.StatusOK, map[string]any{"items": items, "count": len(items)})
	case strings.HasPrefix(path, "/users/") && strings.HasSuffix(path, "/modules"):
		userID := strings.Trim(strings.TrimSuffix(strings.TrimPrefix(path, "/users/"), "/modules"), "/")
		policies := partnerWorkspaceMap(snapshot, "portal_user_module_policies")
		raw, exists := policies[userID]
		if !exists {
			common.APIError(w, http.StatusNotFound, "NOT_FOUND", "Partner user not found")
			return true
		}
		policy, _ := raw.(map[string]any)
		out := central10CopyMap(policy)
		out["user_id"] = userID
		out["partner_id"] = u.PartnerID
		common.JSON(w, http.StatusOK, out)
	case path == "/notifications":
		common.JSON(w, http.StatusOK, partnerNotificationFeedFromSnapshot(snapshot, u, r))
	case path == "/design":
		common.JSON(w, http.StatusOK, partnerWorkspaceMap(snapshot, "partner_design"))
	case path == "/design/media":
		common.JSON(w, http.StatusOK, partnerWorkspaceMap(snapshot, "portal_design_media"))
	case strings.HasPrefix(path, "/runtime/modules/"):
		out, status, handled := tenantFinanceMaterializedGET(snapshot, path, u)
		if !handled {
			common.APIError(w, http.StatusNotFound, "MODULE_RUNTIME_ROUTE_NOT_FOUND", "No runtime operation is registered for this module path")
			return true
		}
		if status >= http.StatusBadRequest {
			code := central10String(out["code"])
			message := central10String(out["message"])
			if code == "" {
				code = "MODULE_RUNTIME_ERROR"
			}
			common.APIError(w, status, code, message)
			return true
		}
		common.JSON(w, status, out)
	default:
		return false
	}
	return true
}
