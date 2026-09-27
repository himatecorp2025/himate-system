package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"himate.local/services/internal/common"
)

type central13PartnerPage struct {
	Items   []map[string]any `json:"items"`
	Count   int              `json:"count"`
	Total   int              `json:"total"`
	Limit   int              `json:"limit"`
	Offset  int              `json:"offset"`
	HasMore bool             `json:"has_more"`
}

func (a *app) central13AllPartners(ctx context.Context) ([]map[string]any, error) {
	const pageSize = 200
	var first central13PartnerPage
	if err := a.internalGET(ctx, a.hosts["partners"],
		"/api/v1/partners?limit=200&offset=0&include_archived=true&include_stats=false", &first); err != nil {
		return nil, err
	}
	if !first.HasMore || first.Total <= len(first.Items) {
		return first.Items, nil
	}
	pageCount := (first.Total + pageSize - 1) / pageSize
	pages := make([][]map[string]any, pageCount)
	pages[0] = first.Items
	var wg sync.WaitGroup
	var errMu sync.Mutex
	var firstErr error
	sem := make(chan struct{}, 6)
	for page := 1; page < pageCount; page++ {
		page := page
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			var next central13PartnerPage
			path := fmt.Sprintf("/api/v1/partners?limit=%d&offset=%d&include_archived=true&include_stats=false", pageSize, page*pageSize)
			if err := a.internalGET(ctx, a.hosts["partners"], path, &next); err != nil {
				errMu.Lock()
				if firstErr == nil { firstErr = err }
				errMu.Unlock()
				return
			}
			pages[page] = next.Items
		}()
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	all := make([]map[string]any, 0, first.Total)
	for _, page := range pages { all = append(all, page...) }
	return all, nil
}

func central13ConnectionStatusForPartner(lifecycle, connectorStatus string) string {
	if strings.EqualFold(strings.TrimSpace(lifecycle), "ARCHIVED") {
		return "DELETED"
	}
	switch strings.ToUpper(strings.TrimSpace(connectorStatus)) {
	case "ACTIVE", "SUSPENDED":
		return strings.ToUpper(strings.TrimSpace(connectorStatus))
	default:
		return "INACTIVE"
	}
}

func central13BoundedInt(raw string, fallback, max int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value < 0 { return fallback }
	if value > max { return max }
	return value
}

func (a *app) materializeCentralConnections(ctx context.Context) map[string]any {
	partners := []map[string]any{}
	var partnerErr error
	if snapshot, _, ok := centralStep3SnapshotGet(centralStep4PartnersKey); ok {
		partners = step4Items(snapshot["items"])
	} else {
		partners, partnerErr = a.central13AllPartners(ctx)
	}

	var connections central10ItemsPage
	var start22Mapping, start22SummaryAll, start22SummaryProduction, start22SummaryStaging map[string]any
	var connectionErr, mappingErr, summaryErr, summaryProductionErr, summaryStagingErr error
	var wg sync.WaitGroup
	wg.Add(5)
	go func() {
		defer wg.Done()
		connectionErr = a.internalGET(ctx, a.hosts["connector"], "/internal/v1/partner-connections", &connections)
	}()
	go func() {
		defer wg.Done()
		mappingErr = a.internalGET(ctx, a.hosts["connector"], "/api/v1/connectors/start22/mapping", &start22Mapping)
	}()
	go func() {
		defer wg.Done()
		summaryErr = a.internalGET(ctx, a.hosts["connector"], "/api/v1/connectors/start22/summary", &start22SummaryAll)
	}()
	go func() {
		defer wg.Done()
		summaryProductionErr = a.internalGET(ctx, a.hosts["connector"], "/api/v1/connectors/start22/summary?environment=PRODUCTION", &start22SummaryProduction)
	}()
	go func() {
		defer wg.Done()
		summaryStagingErr = a.internalGET(ctx, a.hosts["connector"], "/api/v1/connectors/start22/summary?environment=STAGING", &start22SummaryStaging)
	}()
	wg.Wait()
	unavailable := []string{}
	if partnerErr != nil {
		unavailable = append(unavailable, "partners")
	}
	if connectionErr != nil {
		unavailable = append(unavailable, "connector_runtime")
	}
	if mappingErr != nil {
		unavailable = append(unavailable, "start22_mapping")
	}
	if summaryErr != nil || summaryProductionErr != nil || summaryStagingErr != nil {
		unavailable = append(unavailable, "start22_summary")
	}

	connectionByPartner := map[string]map[string]any{}
	for _, item := range connections.Items {
		if id := central10String(item["partner_id"]); id != "" {
			connectionByPartner[id] = item
		}
	}
	statusCounts := map[string]int{"ACTIVE": 0, "INACTIVE": 0, "SUSPENDED": 0, "DELETED": 0}
	all := make([]map[string]any, 0, len(partners))
	for _, partner := range partners {
		id := central10String(partner["id"])
		if id == "" {
			continue
		}
		runtime := connectionByPartner[id]
		status := central13ConnectionStatusForPartner(central10String(partner["lifecycle"]), central10String(runtime["connection_status"]))
		statusCounts[status]++
		all = append(all, map[string]any{
			"partner_id": id,
			"partner_name": central10String(partner["display_name"]),
			"brand_name": central10String(partner["brand_name"]),
			"lifecycle": central10String(partner["lifecycle"]),
			"connection_status": status,
			"last_successful_sync": runtime["last_successful_sync"],
			"last_error": central10String(runtime["last_error"]),
			"integration_count": central10Int(runtime["integration_count"]),
			"connection_types": runtime["connection_types"],
			"integrations": runtime["integrations"],
		})
	}
	sort.SliceStable(all, func(i, j int) bool {
		left := strings.ToLower(central10String(all[i]["partner_name"]))
		right := strings.ToLower(central10String(all[j]["partner_name"]))
		if left == right {
			return central10String(all[i]["partner_id"]) < central10String(all[j]["partner_id"])
		}
		return left < right
	})

	status := "healthy"
	if len(unavailable) > 0 {
		status = "partial"
	}
	return map[string]any{
		"status": status,
		"unavailable": unavailable,
		"items": all,
		"start22_mapping": start22Mapping,
		"start22_summary": map[string]any{
			"ALL": start22SummaryAll,
			"PRODUCTION": start22SummaryProduction,
			"STAGING": start22SummaryStaging,
		},
		"kpis": map[string]any{
			"partner_count": len(all),
			"active": statusCounts["ACTIVE"],
			"inactive": statusCounts["INACTIVE"],
			"suspended": statusCounts["SUSPENDED"],
			"deleted": statusCounts["DELETED"],
			"integration_count": central13IntegrationTotal(all),
		},
	}
}

func (a *app) central13Connections(w http.ResponseWriter, r *http.Request, actor user) {
	if r.Method != http.MethodGet {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use GET")
		return
	}
	started := time.Now()
	snapshot, updatedAt, ok := a.centralSnapshotForRead(r.Context(), centralStep4ConnectionsKey)
	if !ok {
		common.APIError(w, http.StatusServiceUnavailable, "READ_MODEL_NOT_READY", "Connections read model is not ready")
		return
	}
	if time.Since(updatedAt) > 2*centralStep4RefreshInterval {
		a.requestCentralStep4Refresh()
	}

	all := step4Items(snapshot["items"])
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	statusFilter := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("status")))
	filtered := make([]map[string]any, 0, len(all))
	for _, row := range all {
		if statusFilter != "" && statusFilter != "ALL" && central10String(row["connection_status"]) != statusFilter {
			continue
		}
		if query != "" {
			haystack := strings.ToLower(strings.Join([]string{
				central10String(row["partner_name"]), central10String(row["brand_name"]),
				central10String(row["partner_id"]), strings.Join(central13Strings(row["connection_types"]), " "),
			}, " "))
			if !strings.Contains(haystack, query) {
				continue
			}
		}
		filtered = append(filtered, central10CopyMap(row))
	}
	offset := central13BoundedInt(r.URL.Query().Get("offset"), 0, 1_000_000)
	limit := central13BoundedInt(r.URL.Query().Get("limit"), 60, 200)
	if limit < 1 {
		limit = 60
	}
	if offset > len(filtered) {
		offset = len(filtered)
	}
	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}

	payload := map[string]any{
		"items": filtered[offset:end],
		"kpis": step4Map(snapshot["kpis"]),
		"pagination": map[string]any{
			"count": end - offset, "total": len(filtered), "limit": limit,
			"offset": offset, "has_more": end < len(filtered),
		},
		"meta": map[string]any{
			"architecture": "MATERIALIZED_READ_MODEL",
			"frontend_role": "PRESENTATION_ONLY",
			"source": "PERSISTED_CONNECTIONS_SCREEN",
			"duration_ms": time.Since(started).Milliseconds(),
			"generated_at": updatedAt.UTC(),
		},
	}
	w.Header().Set("X-Himate-Cache", "persistent-read-model")
	w.Header().Set("Server-Timing", fmt.Sprintf("central-connections-read-model;dur=%d", time.Since(started).Milliseconds()))
	common.JSON(w, http.StatusOK, payload)
}

func central13Strings(value any) []string {
	raw, ok := value.([]any)
	if ok {
		out := make([]string, 0, len(raw))
		for _, item := range raw {
			if text := strings.TrimSpace(fmt.Sprint(item)); text != "" { out = append(out, text) }
		}
		return out
	}
	if typed, ok := value.([]string); ok { return typed }
	return nil
}

func central13IntegrationTotal(items []map[string]any) int {
	total := 0
	for _, item := range items { total += central10Int(item["integration_count"]) }
	return total
}

func central13ConnectionPath(q, status string, limit, offset int) string {
	values := url.Values{}
	if strings.TrimSpace(q) != "" { values.Set("q", strings.TrimSpace(q)) }
	if strings.TrimSpace(status) != "" && strings.ToUpper(strings.TrimSpace(status)) != "ALL" {
		values.Set("status", strings.ToUpper(strings.TrimSpace(status)))
	}
	values.Set("limit", strconv.Itoa(limit))
	values.Set("offset", strconv.Itoa(offset))
	return "/api/v1/central/connections?" + values.Encode()
}
