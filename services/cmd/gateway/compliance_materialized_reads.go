package main

import (
	"net/http"
	"strings"

	"himate.local/services/internal/common"
)

func (a *app) serveComplianceMaterializedGET(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}
	path := r.URL.Path
	if path != "/api/v1/archives" && !strings.HasPrefix(path, "/api/v1/archives/") {
		return false
	}
	snapshot, _, ok := a.centralSnapshotForRead(r.Context(), centralStep4ComplianceKey)
	if !ok {
		common.APIError(w, http.StatusServiceUnavailable, "READ_MODEL_NOT_READY", "Compliance read model is not ready")
		return true
	}
	w.Header().Set("X-Himate-Cache", "persistent-read-model")
	if path == "/api/v1/archives" {
		q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
		filtered := []map[string]any{}
		for _, raw := range step4Items(snapshot["items"]) {
			if q != "" && !searchContains(q, raw["partner_id"], raw["display_name"], raw["legal_name"]) {
				continue
			}
			filtered = append(filtered, central10CopyMap(raw))
		}
		limit := queryInt(r.URL.Query().Get("limit"), 50, 200)
		offset := queryInt(r.URL.Query().Get("offset"), 0, 1_000_000)
		common.JSON(w, http.StatusOK, materializedPage(filtered, limit, offset))
		return true
	}
	partnerID := strings.Trim(strings.TrimPrefix(path, "/api/v1/archives/"), "/")
	if partnerID == "" || strings.Contains(partnerID, "/") {
		common.APIError(w, http.StatusNotFound, "NOT_FOUND", "Compliance Archive not found")
		return true
	}
	details := partnerWorkspaceMap(snapshot, "details")
	detail, exists := details[partnerID]
	if !exists {
		common.APIError(w, http.StatusNotFound, "NOT_FOUND", "Compliance Archive not found")
		return true
	}
	common.JSON(w, http.StatusOK, detail)
	return true
}
