package main

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

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

func materializedAuditCreatedAt(raw any) (time.Time, bool) {
	switch value := raw.(type) {
	case time.Time:
		return value.UTC(), true
	case string:
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
		if err == nil {
			return parsed.UTC(), true
		}
		// PostgreSQL JSON timestamps may contain sub-second precision/offsets
		// that still parse under RFC3339Nano.
		parsed, err = time.Parse(time.RFC3339Nano, strings.TrimSpace(value))
		if err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
}

func filterMaterializedAudit(items []map[string]any, r *http.Request) ([]map[string]any, error) {
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	partnerID := strings.TrimSpace(r.URL.Query().Get("partner_id"))
	actorID := strings.TrimSpace(r.URL.Query().Get("actor_id"))
	resource := strings.TrimSpace(r.URL.Query().Get("resource"))
	action := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("action")))
	correlationID := strings.TrimSpace(r.URL.Query().Get("correlation_id"))
	method := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("method")))
	outcome := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("outcome")))

	var from, to time.Time
	if raw := strings.TrimSpace(r.URL.Query().Get("from")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return nil, fmt.Errorf("from must be RFC3339")
		}
		from = parsed.UTC()
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("to")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return nil, fmt.Errorf("to must be RFC3339")
		}
		to = parsed.UTC()
	}

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
		if correlationID != "" && central10String(row["correlation_id"]) != correlationID {
			continue
		}
		if method != "" && strings.ToUpper(central10String(row["method"])) != method {
			continue
		}
		if outcome != "" && strings.ToUpper(central10String(row["outcome"])) != outcome {
			continue
		}
		if !from.IsZero() || !to.IsZero() {
			created, ok := materializedAuditCreatedAt(row["created_at"])
			if !ok {
				continue
			}
			if !from.IsZero() && created.Before(from) {
				continue
			}
			if !to.IsZero() && created.After(to) {
				continue
			}
		}
		if q != "" && !searchContains(q,
			row["actor_name"], row["actor_id"], row["path"], row["resource"],
			row["request_id"], row["correlation_id"], row["action"], row["partner_id"],
		) {
			continue
		}
		out = append(out, central10CopyMap(row))
	}
	return out, nil
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

func materializedEvidenceList(items []map[string]any, r *http.Request) (map[string]any, error) {
	partnerID := strings.TrimSpace(r.URL.Query().Get("partner_id"))
	metricKey := strings.TrimSpace(r.URL.Query().Get("metric_key"))
	kind := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("evidence_type")))
	verification := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("verification_status")))
	reportID := strings.TrimSpace(r.URL.Query().Get("report_id"))
	periodStartRaw := strings.TrimSpace(r.URL.Query().Get("period_start"))
	periodEndRaw := strings.TrimSpace(r.URL.Query().Get("period_end"))
	search := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))

	validKinds := map[string]bool{
		"PDF": true, "IMAGE": true, "INVOICE": true, "CONTRACT": true,
		"SCREENSHOT": true, "REPORT": true, "URL": true,
		"PARTNER_DECLARATION": true, "OTHER": true,
	}
	validVerification := map[string]bool{"UNVERIFIED": true, "VERIFIED": true, "REJECTED": true}
	if kind != "" && !validKinds[kind] {
		return nil, fmt.Errorf("Invalid evidence_type")
	}
	if verification != "" && !validVerification[verification] {
		return nil, fmt.Errorf("Invalid verification_status")
	}

	var periodStart, periodEnd time.Time
	var err error
	if periodStartRaw != "" {
		periodStart, err = time.Parse("2006-01-02", periodStartRaw)
		if err != nil {
			return nil, fmt.Errorf("period_start must be YYYY-MM-DD")
		}
	}
	if periodEndRaw != "" {
		periodEnd, err = time.Parse("2006-01-02", periodEndRaw)
		if err != nil {
			return nil, fmt.Errorf("period_end must be YYYY-MM-DD")
		}
	}

	filtered := make([]map[string]any, 0, len(items))
	for _, raw := range items {
		if partnerID != "" && central10String(raw["partner_id"]) != partnerID {
			continue
		}
		if metricKey != "" && central10String(raw["metric_key"]) != metricKey {
			continue
		}
		if kind != "" && strings.ToUpper(central10String(raw["evidence_type"])) != kind {
			continue
		}
		if verification != "" && strings.ToUpper(central10String(raw["verification_status"])) != verification {
			continue
		}
		if reportID != "" && !stringSetFromAny(raw["reports"])[reportID] {
			continue
		}
		if !periodStart.IsZero() {
			rawEnd := central10String(raw["period_end"])
			if rawEnd != "" {
				end, parseErr := time.Parse("2006-01-02", rawEnd)
				if parseErr == nil && end.Before(periodStart) {
					continue
				}
			}
		}
		if !periodEnd.IsZero() {
			rawStart := central10String(raw["period_start"])
			if rawStart != "" {
				start, parseErr := time.Parse("2006-01-02", rawStart)
				if parseErr == nil && start.After(periodEnd) {
					continue
				}
			}
		}
		if search != "" && !searchContains(
			search,
			raw["title"], raw["description"], raw["original_filename"],
			raw["source_url"], raw["declaration_text"], raw["partner_id"], raw["metric_key"],
		) {
			continue
		}
		filtered = append(filtered, central10CopyMap(raw))
	}

	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, parseErr := strconv.Atoi(raw); parseErr == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	offset := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		if parsed, parseErr := strconv.Atoi(raw); parseErr == nil && parsed >= 0 {
			offset = parsed
		}
	}
	page := materializedPage(filtered, limit, offset)
	return page, nil
}

func materializedImpactSummary(tenant map[string]any, r *http.Request) (map[string]any, error) {
	partnerID := strings.TrimSpace(r.URL.Query().Get("partner_id"))
	periodStartRaw := strings.TrimSpace(r.URL.Query().Get("period_start"))
	periodEndRaw := strings.TrimSpace(r.URL.Query().Get("period_end"))
	var periodStart, periodEnd time.Time
	var err error
	if periodStartRaw != "" {
		periodStart, err = time.Parse("2006-01-02", periodStartRaw)
		if err != nil {
			return nil, fmt.Errorf("period_start must be YYYY-MM-DD")
		}
	}
	if periodEndRaw != "" {
		periodEnd, err = time.Parse("2006-01-02", periodEndRaw)
		if err != nil {
			return nil, fmt.Errorf("period_end must be YYYY-MM-DD")
		}
	}

	metaByKey := map[string]map[string]any{}
	for _, raw := range anyItems(partnerWorkspaceMap(tenant, "impact_api")["items"]) {
		key := central10String(raw["metric_key"])
		if key != "" {
			metaByKey[key] = central10CopyMap(raw)
		}
	}

	groups := map[string][]map[string]any{}
	for _, raw := range anyItems(partnerWorkspaceMap(tenant, "impact_values_api")["items"]) {
		key := central10String(raw["metric_key"])
		if key == "" {
			continue
		}
		start, startErr := time.Parse("2006-01-02", central10String(raw["period_start"]))
		end, endErr := time.Parse("2006-01-02", central10String(raw["period_end"]))
		if startErr != nil || endErr != nil {
			continue
		}
		if !periodStart.IsZero() && end.Before(periodStart) {
			continue
		}
		if !periodEnd.IsZero() && start.After(periodEnd) {
			continue
		}
		groups[key] = append(groups[key], raw)
	}

	items := make([]map[string]any, 0, len(groups))
	for key, rows := range groups {
		meta := central10CopyMap(metaByKey[key])
		if meta == nil {
			meta = map[string]any{
				"metric_key": key,
				"label":      central10String(rows[0]["label"]),
				"label_en":   central10String(rows[0]["label"]),
				"label_hu":   central10String(rows[0]["label"]),
				"unit":       central10String(rows[0]["unit"]),
				"aggregation": "SUM",
			}
		}
		aggregation := strings.ToUpper(central10String(meta["aggregation"]))
		if aggregation == "" {
			aggregation = "SUM"
			meta["aggregation"] = aggregation
		}

		var sum float64
		numericCount := 0
		var latestNumeric any
		var latestEnd time.Time
		latestID := -1
		for _, row := range rows {
			end, err := time.Parse("2006-01-02", central10String(row["period_end"]))
			if err == nil && end.After(latestEnd) {
				latestEnd = end
			}
			numeric, ok := row["numeric_value"].(float64)
			if !ok {
				continue
			}
			sum += numeric
			numericCount++
			id := central10Int(row["id"])
			if latestNumeric == nil || end.After(latestEnd) || (end.Equal(latestEnd) && id > latestID) {
				latestNumeric = numeric
				latestID = id
			}
		}
		// LATEST follows the Impact SQL ordering by period_end DESC,id DESC.
		if aggregation == "LATEST" {
			var chosen map[string]any
			for _, row := range rows {
				if _, ok := row["numeric_value"].(float64); !ok {
					continue
				}
				if chosen == nil {
					chosen = row
					continue
				}
				rowEnd, _ := time.Parse("2006-01-02", central10String(row["period_end"]))
				chosenEnd, _ := time.Parse("2006-01-02", central10String(chosen["period_end"]))
				if rowEnd.After(chosenEnd) || (rowEnd.Equal(chosenEnd) && central10Int(row["id"]) > central10Int(chosen["id"])) {
					chosen = row
				}
			}
			if chosen != nil {
				latestNumeric = chosen["numeric_value"]
			}
		}

		var value any
		switch aggregation {
		case "LATEST":
			value = latestNumeric
		case "AVERAGE":
			if numericCount > 0 {
				value = sum / float64(numericCount)
			}
		default:
			if numericCount > 0 {
				value = sum
			}
		}
		meta["numeric_value"] = value
		meta["observations"] = len(rows)
		if !latestEnd.IsZero() {
			meta["latest_period_end"] = latestEnd.Format("2006-01-02")
		} else {
			meta["latest_period_end"] = ""
		}
		items = append(items, meta)
	}
	sort.SliceStable(items, func(i, j int) bool {
		left := strings.ToLower(central10String(items[i]["label_en"]))
		right := strings.ToLower(central10String(items[j]["label_en"]))
		if left == right {
			return central10String(items[i]["metric_key"]) < central10String(items[j]["metric_key"])
		}
		return left < right
	})
	return map[string]any{"partner_id": partnerID, "items": items, "count": len(items)}, nil
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

func materializedPartnerIDSet(raw string) map[string]bool {
	out := map[string]bool{}
	for _, value := range strings.Split(strings.TrimSpace(raw), ",") {
		id := strings.TrimSpace(value)
		if id != "" {
			out[id] = true
		}
	}
	return out
}

func filterMaterializedByPartnerIDs(items []map[string]any, raw string) []map[string]any {
	ids := materializedPartnerIDSet(raw)
	if len(ids) == 0 {
		return items
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if ids[central10String(item["partner_id"])] {
			out = append(out, central10CopyMap(item))
		}
	}
	return out
}

func materializedItemsPage(items []map[string]any, r *http.Request, fallbackLimit int) map[string]any {
	limit := queryInt(r.URL.Query().Get("limit"), fallbackLimit, 500)
	offset := queryInt(r.URL.Query().Get("offset"), 0, 1_000_000)
	return materializedPage(items, limit, offset)
}

// serveCentralMaterializedGET keeps the established REST paths but replaces
// synchronous service proxies/DB aggregation with persistent CQRS projections.
// It is called only for GETs; writes retain their authoritative owner service.
func centralBrowserMaterializedRead(r *http.Request) bool {
	if r == nil || r.Method != http.MethodGet {
		return false
	}
	// Locale is not a browser discriminator: historical localized START smokes
	// intentionally send X-Himate-Locale. Only the shipped browser client and the
	// CENTRAL-21 browser acceptance smoke send the explicit CQRS discriminator.
	if !strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Himate-Read-Model")), "browser") {
		return false
	}
	return strings.TrimSpace(r.Header.Get("X-Himate-Locale")) != ""
}

func (a *app) serveCentralMaterializedGET(w http.ResponseWriter, r *http.Request, actor user) bool {
	if !centralBrowserMaterializedRead(r) {
		return false
	}
	if a.serveComplianceMaterializedGET(w, r) {
		return true
	}
	path := r.URL.Path

	switch {
	case path == "/api/v1/partner-categories":
		snapshot, _, ok := a.centralSnapshotForRead(r.Context(), centralStep4PartnersKey)
		if !ok {
			common.APIError(w, http.StatusServiceUnavailable, "READ_MODEL_NOT_READY", "Partners read model is not ready")
			return true
		}
		locale := common.RequestLocale(r)
		rawItems := step4Items(snapshot["categories_raw"])
		items := make([]map[string]any, 0, len(rawItems))
		for _, raw := range rawItems {
			nameEN := central10String(raw["name_en"])
			nameHU := central10String(raw["name_hu"])
			items = append(items, map[string]any{
				"id": raw["id"],
				"name": common.Localized(nameEN, nameHU, locale),
				"name_en": nameEN,
				"name_hu": nameHU,
				"slug": raw["slug"],
				"system": raw["system"],
			})
		}
		w.Header().Set("X-Himate-Cache", "persistent-read-model")
		common.JSON(w, http.StatusOK, map[string]any{"items": items, "locale": locale})
		return true

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

	case strings.HasPrefix(path, "/api/v1/modules/"):
		raw := strings.Trim(strings.TrimPrefix(path, "/api/v1/modules/"), "/")
		parts := strings.Split(raw, "/")
		if len(parts) == 2 && parts[0] != "" {
			snapshot, _, ok := a.centralSnapshotForRead(r.Context(), centralStep3RegistryKey)
			if !ok {
				common.APIError(w, http.StatusServiceUnavailable, "READ_MODEL_NOT_READY", "Module registry is not ready")
				return true
			}
			details := partnerWorkspaceMap(snapshot, "module_details")
			rawDetail, exists := details[parts[0]]
			if !exists {
				common.APIError(w, http.StatusNotFound, "NOT_FOUND", "Module not found")
				return true
			}
			detail, _ := rawDetail.(map[string]any)
			if detail == nil {
				common.APIError(w, http.StatusNotFound, "NOT_FOUND", "Module detail projection not found")
				return true
			}
			key := ""
			switch parts[1] {
			case "relationships":
				key = "relationships"
			case "impact-metrics":
				key = "impact_metrics"
			case "usage":
				key = "usage"
			default:
				return false
			}
			w.Header().Set("X-Himate-Cache", "persistent-read-model")
			common.JSON(w, http.StatusOK, partnerWorkspaceMap(detail, key))
			return true
		}
		return false

	case path == "/api/v1/module-commercial-matrix":
		snapshot, _, ok := a.centralSnapshotForRead(r.Context(), centralStep3CommercialKey)
		if !ok {
			common.APIError(w, http.StatusServiceUnavailable, "READ_MODEL_NOT_READY", "Commercial matrix is not ready")
			return true
		}
		items := filterMaterializedByPartnerIDs(step4Items(snapshot["matrix_items"]), r.URL.Query().Get("partner_ids"))
		w.Header().Set("X-Himate-Cache", "persistent-read-model")
		common.JSON(w, http.StatusOK, map[string]any{"items": items, "count": len(items)})
		return true
	}

	// Direct Central/tenant REST reads are projected as well. This keeps
	// sub-screens and future clients on the same zero-fan-out CQRS read path.
	if path == "/api/v1/billing/plans" {
		snapshot, _, ok := a.centralSnapshotForRead(r.Context(), centralStep3PlansKey)
		if !ok { common.APIError(w,http.StatusServiceUnavailable,"READ_MODEL_NOT_READY","Billing plans read model is not ready"); return true }
		items := step4Items(snapshot["plans"])
		w.Header().Set("X-Himate-Cache","persistent-read-model")
		common.JSON(w,http.StatusOK,map[string]any{"items":items,"count":len(items),"total":len(items)})
		return true
	}
	if path == "/api/v1/billing/packages/analytics" {
		snapshot, _, ok := a.centralSnapshotForRead(r.Context(), centralStep3AnalyticsKey)
		if !ok { common.APIError(w,http.StatusServiceUnavailable,"READ_MODEL_NOT_READY","Package analytics read model is not ready"); return true }
		w.Header().Set("X-Himate-Cache","persistent-read-model")
		common.JSON(w,http.StatusOK,partnerWorkspaceMap(snapshot,"analytics"))
		return true
	}
	if path == "/api/v1/billing/subscription-matrix" {
		snapshot, _, ok := a.centralSnapshotForRead(r.Context(), centralStep3CommercialKey)
		if !ok { common.APIError(w,http.StatusServiceUnavailable,"READ_MODEL_NOT_READY","Subscription matrix read model is not ready"); return true }
		items := filterMaterializedByPartnerIDs(step4Items(snapshot["subscription_items"]), r.URL.Query().Get("partner_ids"))
		w.Header().Set("X-Himate-Cache","persistent-read-model")
		common.JSON(w,http.StatusOK,map[string]any{"items":items,"count":len(items)})
		return true
	}
	if path == "/api/v1/billing/finance/overview" {
		snapshot, _, ok := a.centralSnapshotForRead(r.Context(), centralStep4FinanceKey)
		if !ok { common.APIError(w,http.StatusServiceUnavailable,"READ_MODEL_NOT_READY","Finance read model is not ready"); return true }
		w.Header().Set("X-Himate-Cache","persistent-read-model")
		common.JSON(w,http.StatusOK,partnerWorkspaceMap(snapshot,"overview"))
		return true
	}

	if strings.HasPrefix(path, "/api/v1/partners/") && strings.HasSuffix(path, "/commercial-history") {
		raw := strings.Trim(strings.TrimPrefix(path, "/api/v1/partners/"), "/")
		parts := strings.Split(raw, "/")
		if len(parts) == 4 && parts[0] != "" && parts[1] == "modules" && parts[2] != "" && parts[3] == "commercial-history" {
			partnerID, moduleKey := parts[0], parts[2]
			tenant, _, ok := a.partnerWorkspaceForRead(r.Context(), partnerID)
			if !ok {
				common.APIError(w, http.StatusNotFound, "PARTNER_NOT_FOUND", "Partner not found")
				return true
			}
			historyRoot := partnerWorkspaceMap(tenant, "module_commercial_history")
			historyItems := partnerWorkspaceMap(historyRoot, "items")
			if rawHistory, exists := historyItems[moduleKey]; exists {
				if history, ok := rawHistory.(map[string]any); ok {
					w.Header().Set("X-Himate-Cache", "persistent-tenant-read-model")
					common.JSON(w, http.StatusOK, history)
					return true
				}
			}
			w.Header().Set("X-Himate-Cache", "persistent-tenant-read-model")
			common.JSON(w, http.StatusOK, map[string]any{
				"partner_id": partnerID, "module_key": moduleKey,
				"items": []map[string]any{}, "count": 0,
			})
			return true
		}
	}

	if strings.HasPrefix(path, "/api/v1/partners/") {
		raw := strings.Trim(strings.TrimPrefix(path, "/api/v1/partners/"), "/")
		parts := strings.Split(raw, "/")
		if len(parts) == 2 && parts[0] != "" {
			partnerID := parts[0]
			tenant, _, ok := a.partnerWorkspaceForRead(r.Context(), partnerID)
			if ok {
				w.Header().Set("X-Himate-Cache","persistent-tenant-read-model")
				switch parts[1] {
				case "portal-users":
					if !ownerRequired(w, actor) { return true }
					common.JSON(w,http.StatusOK,partnerWorkspaceMap(tenant,"portal_users_api")); return true
				case "modules":
					common.JSON(w,http.StatusOK,partnerWorkspaceMap(tenant,"catalog_modules_api")); return true
				}
			}
		}
	}

	if path == "/api/v1/provisioning/jobs" {
		if partnerID := strings.TrimSpace(r.URL.Query().Get("partner_id")); partnerID != "" {
			tenant, _, ok := a.partnerWorkspaceForRead(r.Context(), partnerID)
			if !ok { common.APIError(w,http.StatusNotFound,"PARTNER_NOT_FOUND","Partner not found"); return true }
			w.Header().Set("X-Himate-Cache","persistent-tenant-read-model")
			common.JSON(w,http.StatusOK,partnerWorkspaceMap(tenant,"provisioning_api")); return true
		}
		snapshot, _, ok := a.centralSnapshotForRead(r.Context(), centralStep4SystemKey)
		if !ok { common.APIError(w,http.StatusServiceUnavailable,"READ_MODEL_NOT_READY","System read model is not ready"); return true }
		w.Header().Set("X-Himate-Cache","persistent-read-model")
		common.JSON(w,http.StatusOK,partnerWorkspaceMap(snapshot,"provisioning_api")); return true
	}

	if path == "/api/v1/connectors/start22/mapping" {
		snapshot, _, ok := a.centralSnapshotForRead(r.Context(), centralStep4ConnectionsKey)
		if !ok {
			common.APIError(w, http.StatusServiceUnavailable, "READ_MODEL_NOT_READY", "Connections read model is not ready")
			return true
		}
		w.Header().Set("X-Himate-Cache", "persistent-read-model")
		common.JSON(w, http.StatusOK, partnerWorkspaceMap(snapshot, "start22_mapping"))
		return true
	}
	if path == "/api/v1/connectors/start22/summary" {
		partnerID := strings.TrimSpace(r.URL.Query().Get("partner_id"))
		environment := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("environment")))
		if environment != "PRODUCTION" && environment != "STAGING" { environment = "ALL" }
		if partnerID != "" {
			tenant, _, ok := a.partnerWorkspaceForRead(r.Context(), partnerID)
			if !ok {
				common.APIError(w, http.StatusNotFound, "PARTNER_NOT_FOUND", "Partner not found")
				return true
			}
			summaries := partnerWorkspaceMap(tenant, "start22_summary")
			w.Header().Set("X-Himate-Cache", "persistent-tenant-read-model")
			common.JSON(w, http.StatusOK, partnerWorkspaceMap(summaries, environment))
			return true
		}
		snapshot, _, ok := a.centralSnapshotForRead(r.Context(), centralStep4ConnectionsKey)
		if !ok {
			common.APIError(w, http.StatusServiceUnavailable, "READ_MODEL_NOT_READY", "Connections read model is not ready")
			return true
		}
		summaries := partnerWorkspaceMap(snapshot, "start22_summary")
		w.Header().Set("X-Himate-Cache", "persistent-read-model")
		common.JSON(w, http.StatusOK, partnerWorkspaceMap(summaries, environment))
		return true
	}
	if path == "/api/v1/connectors/start22/retention" {
		partnerID := strings.TrimSpace(r.URL.Query().Get("partner_id"))
		if partnerID == "" {
			common.APIError(w, http.StatusBadRequest, "VALIDATION", "partner_id is required")
			return true
		}
		tenant, _, ok := a.partnerWorkspaceForRead(r.Context(), partnerID)
		if !ok {
			common.APIError(w, http.StatusNotFound, "PARTNER_NOT_FOUND", "Partner not found")
			return true
		}
		w.Header().Set("X-Himate-Cache", "persistent-tenant-read-model")
		common.JSON(w, http.StatusOK, partnerWorkspaceMap(tenant, "start22_retention"))
		return true
	}

	if strings.HasPrefix(path, "/api/v1/connectors/") {
		raw := strings.Trim(strings.TrimPrefix(path, "/api/v1/connectors/"), "/")
		parts := strings.Split(raw, "/")
		if len(parts) == 2 && parts[0] != "" {
			tenant, _, ok := a.partnerWorkspaceForRead(r.Context(), parts[0])
			if ok {
				w.Header().Set("X-Himate-Cache","persistent-tenant-read-model")
				switch parts[1] {
				case "credential":
					common.JSON(w,http.StatusOK,partnerWorkspaceMap(tenant,"connector_credentials_api")); return true
				case "website-adapter":
					common.JSON(w,http.StatusOK,partnerWorkspaceMap(tenant,"website_adapter")); return true
				}
			}
		}
	}

	if strings.HasPrefix(path, "/api/v1/payments/partners/") && strings.HasSuffix(path, "/profile") {
		partnerID := strings.Trim(strings.TrimSuffix(strings.TrimPrefix(path, "/api/v1/payments/partners/"), "/profile"), "/")
		if partnerID != "" {
			tenant, _, ok := a.partnerWorkspaceForRead(r.Context(), partnerID)
			if !ok { common.APIError(w,http.StatusNotFound,"PARTNER_NOT_FOUND","Partner not found"); return true }
			w.Header().Set("X-Himate-Cache","persistent-tenant-read-model")
			common.JSON(w,http.StatusOK,partnerWorkspaceMap(tenant,"payment_profile")); return true
		}
	}

	if path == "/api/v1/impact/definitions" {
		snapshot, _, ok := a.centralSnapshotForRead(r.Context(), centralStep4ImpactKey)
		if !ok { common.APIError(w,http.StatusServiceUnavailable,"READ_MODEL_NOT_READY","Impact read model is not ready"); return true }
		items := step4Items(snapshot["definitions"])
		w.Header().Set("X-Himate-Cache","persistent-read-model")
		common.JSON(w,http.StatusOK,materializedItemsPage(items,r,len(items))); return true
	}
	if path == "/api/v1/impact/summary" {
		if partnerID := strings.TrimSpace(r.URL.Query().Get("partner_id")); partnerID != "" {
			tenant, _, ok := a.partnerWorkspaceForRead(r.Context(), partnerID)
			if !ok { common.APIError(w,http.StatusNotFound,"PARTNER_NOT_FOUND","Partner not found"); return true }
			out, err := materializedImpactSummary(tenant, r)
			if err != nil {
				common.APIError(w, http.StatusBadRequest, "VALIDATION", err.Error())
				return true
			}
			w.Header().Set("X-Himate-Cache","persistent-tenant-read-model")
			common.JSON(w,http.StatusOK,out); return true
		}
		snapshot, _, ok := a.centralSnapshotForRead(r.Context(), centralStep4ImpactKey)
		if !ok { common.APIError(w,http.StatusServiceUnavailable,"READ_MODEL_NOT_READY","Impact read model is not ready"); return true }
		items := step4Items(snapshot["summary"])
		w.Header().Set("X-Himate-Cache","persistent-read-model")
		common.JSON(w,http.StatusOK,materializedItemsPage(items,r,len(items))); return true
	}
	if path == "/api/v1/evidence" {
		var items []map[string]any
		cacheHeader := "persistent-read-model"
		if partnerID := strings.TrimSpace(r.URL.Query().Get("partner_id")); partnerID != "" {
			tenant, _, ok := a.partnerWorkspaceForRead(r.Context(), partnerID)
			if !ok { common.APIError(w,http.StatusNotFound,"PARTNER_NOT_FOUND","Partner not found"); return true }
			items = anyItems(partnerWorkspaceMap(tenant,"evidence_api")["items"])
			cacheHeader = "persistent-tenant-read-model"
		} else {
			snapshot, _, ok := a.centralSnapshotForRead(r.Context(), centralStep4ImpactKey)
			if !ok { common.APIError(w,http.StatusServiceUnavailable,"READ_MODEL_NOT_READY","Impact read model is not ready"); return true }
			items = step4Items(snapshot["evidence"])
		}
		out, err := materializedEvidenceList(items, r)
		if err != nil {
			common.APIError(w, http.StatusBadRequest, "VALIDATION", err.Error())
			return true
		}
		w.Header().Set("X-Himate-Cache", cacheHeader)
		common.JSON(w,http.StatusOK,out); return true
	}
	if path == "/api/v1/reports" {
		snapshot, _, ok := a.centralSnapshotForRead(r.Context(), centralStep4ImpactKey)
		if !ok { common.APIError(w,http.StatusServiceUnavailable,"READ_MODEL_NOT_READY","Impact read model is not ready"); return true }
		items := step4Items(snapshot["reports"])
		w.Header().Set("X-Himate-Cache","persistent-read-model")
		common.JSON(w,http.StatusOK,materializedItemsPage(items,r,100)); return true
	}

	if path == "/api/v1/backups" || path == "/api/v1/backups/summary" {
		snapshot, _, ok := a.centralSnapshotForRead(r.Context(), centralStep4SystemKey)
		if !ok { common.APIError(w,http.StatusServiceUnavailable,"READ_MODEL_NOT_READY","System read model is not ready"); return true }
		w.Header().Set("X-Himate-Cache","persistent-read-model")
		common.JSON(w,http.StatusOK,partnerWorkspaceMap(snapshot,"backups_api")); return true
	}

	if strings.HasPrefix(path, "/api/v1/backups/restore-points/") {
		id := strings.Trim(strings.TrimPrefix(path, "/api/v1/backups/restore-points/"), "/")
		if id == "" || strings.Contains(id, "/") {
			return false
		}
		snapshot, _, ok := a.centralSnapshotForRead(r.Context(), centralStep4SystemKey)
		if !ok {
			common.APIError(w, http.StatusServiceUnavailable, "READ_MODEL_NOT_READY", "System read model is not ready")
			return true
		}
		points := partnerWorkspaceMap(snapshot, "backup_restore_points")
		item, exists := points[id]
		if !exists {
			common.APIError(w, http.StatusNotFound, "NOT_FOUND", "Restore point not found")
			return true
		}
		w.Header().Set("X-Himate-Cache", "persistent-read-model")
		common.JSON(w, http.StatusOK, item)
		return true
	}

	if path == "/api/v1/backups/restore-tests" {
		snapshot, _, ok := a.centralSnapshotForRead(r.Context(), centralStep4SystemKey)
		if !ok {
			common.APIError(w, http.StatusServiceUnavailable, "READ_MODEL_NOT_READY", "System read model is not ready")
			return true
		}
		items := anyItems(partnerWorkspaceMap(snapshot, "backup_restore_tests_api")["items"])
		partnerID := strings.TrimSpace(r.URL.Query().Get("partner_id"))
		restorePointID := strings.TrimSpace(r.URL.Query().Get("restore_point_id"))
		filtered := make([]map[string]any, 0, len(items))
		for _, item := range items {
			if partnerID != "" && central10String(item["partner_id"]) != partnerID {
				continue
			}
			if restorePointID != "" && central10String(item["restore_point_id"]) != restorePointID {
				continue
			}
			filtered = append(filtered, central10CopyMap(item))
		}
		limit := queryInt(r.URL.Query().Get("limit"), 50, 200)
		if limit > len(filtered) {
			limit = len(filtered)
		}
		filtered = filtered[:limit]
		w.Header().Set("X-Himate-Cache", "persistent-read-model")
		common.JSON(w, http.StatusOK, map[string]any{"items": filtered, "count": len(filtered)})
		return true
	}

	if strings.HasPrefix(path, "/api/v1/backups/restore-tests/") {
		id := strings.Trim(strings.TrimPrefix(path, "/api/v1/backups/restore-tests/"), "/")
		if id == "" || strings.Contains(id, "/") {
			return false
		}
		snapshot, _, ok := a.centralSnapshotForRead(r.Context(), centralStep4SystemKey)
		if !ok {
			common.APIError(w, http.StatusServiceUnavailable, "READ_MODEL_NOT_READY", "System read model is not ready")
			return true
		}
		for _, item := range anyItems(partnerWorkspaceMap(snapshot, "backup_restore_tests_api")["items"]) {
			if central10String(item["id"]) == id {
				w.Header().Set("X-Himate-Cache", "persistent-read-model")
				common.JSON(w, http.StatusOK, item)
				return true
			}
		}
		common.APIError(w, http.StatusNotFound, "NOT_FOUND", "Restore test not found")
		return true
	}

	if path == "/api/v1/backups/restores" {
		snapshot, _, ok := a.centralSnapshotForRead(r.Context(), centralStep4SystemKey)
		if !ok {
			common.APIError(w, http.StatusServiceUnavailable, "READ_MODEL_NOT_READY", "System read model is not ready")
			return true
		}
		items := anyItems(partnerWorkspaceMap(snapshot, "backup_restore_jobs_api")["items"])
		partnerID := strings.TrimSpace(r.URL.Query().Get("partner_id"))
		filtered := make([]map[string]any, 0, len(items))
		for _, item := range items {
			if partnerID != "" && central10String(item["partner_id"]) != partnerID {
				continue
			}
			filtered = append(filtered, central10CopyMap(item))
		}
		if len(filtered) > 100 {
			filtered = filtered[:100]
		}
		w.Header().Set("X-Himate-Cache", "persistent-read-model")
		common.JSON(w, http.StatusOK, map[string]any{"items": filtered, "count": len(filtered)})
		return true
	}

	if strings.HasPrefix(path, "/api/v1/backups/restores/") {
		id := strings.Trim(strings.TrimPrefix(path, "/api/v1/backups/restores/"), "/")
		if id == "" || strings.Contains(id, "/") {
			return false
		}
		snapshot, _, ok := a.centralSnapshotForRead(r.Context(), centralStep4SystemKey)
		if !ok {
			common.APIError(w, http.StatusServiceUnavailable, "READ_MODEL_NOT_READY", "System read model is not ready")
			return true
		}
		for _, item := range anyItems(partnerWorkspaceMap(snapshot, "backup_restore_jobs_api")["items"]) {
			if central10String(item["id"]) == id {
				w.Header().Set("X-Himate-Cache", "persistent-read-model")
				common.JSON(w, http.StatusOK, item)
				return true
			}
		}
		common.APIError(w, http.StatusNotFound, "NOT_FOUND", "Restore job not found")
		return true
	}

	if strings.HasPrefix(path, "/api/v1/system-health/partner-snapshots") {
		snapshot, _, ok := a.centralSnapshotForRead(r.Context(), centralStep4PartnersKey)
		if !ok { common.APIError(w,http.StatusServiceUnavailable,"READ_MODEL_NOT_READY","Partner read model is not ready"); return true }
		ids := materializedPartnerIDSet(r.URL.Query().Get("ids"))
		items := []map[string]any{}
		for _, row := range step4Items(snapshot["items"]) {
			id := central10String(row["id"])
			if len(ids) > 0 && !ids[id] { continue }
			items = append(items,map[string]any{
				"partner_id": id,
				"overall_status": row["system_health"],
				"platform_version": row["platform_version"],
				"connector_health": row["connector_health"],
				"environment_status": row["environment_status"],
				"provisioning_status": row["provisioning_status"],
			})
		}
		w.Header().Set("X-Himate-Cache","persistent-read-model")
		common.JSON(w,http.StatusOK,map[string]any{"items":items,"count":len(items)}); return true
	}

	if strings.HasPrefix(path, "/api/v1/billing/partners/") {
		raw := strings.Trim(strings.TrimPrefix(path, "/api/v1/billing/partners/"), "/")
		parts := strings.Split(raw, "/")
		if len(parts) >= 2 {
			partnerID := parts[0]
			tenant, _, ok := a.partnerWorkspaceForRead(r.Context(), partnerID)
			if ok {
				w.Header().Set("X-Himate-Cache", "persistent-tenant-read-model")
				suffix := strings.Join(parts[1:], "/")
				switch suffix {
				case "summary":
					common.JSON(w, http.StatusOK, partnerWorkspaceMap(tenant, "billing")); return true
				case "plan":
					common.JSON(w, http.StatusOK, partnerWorkspaceMap(tenant, "portal_plan")); return true
				case "plan/modules":
					common.JSON(w, http.StatusOK, partnerWorkspaceMap(tenant, "portal_plan_modules")); return true
				case "commercial-mode":
					common.JSON(w, http.StatusOK, partnerWorkspaceMap(tenant, "portal_charity")); return true
				case "charity/modules":
					common.JSON(w, http.StatusOK, partnerWorkspaceMap(tenant, "portal_charity_modules")); return true
				case "terms":
					common.JSON(w, http.StatusOK, partnerWorkspaceMap(tenant, "terms")); return true
				case "license":
					common.JSON(w, http.StatusOK, partnerWorkspaceMap(tenant, "license")); return true
				case "documents":
					items := filterMaterializedDocuments(partnerWorkspaceItems(tenant, "documents"), r.URL.Query().Get("q"))
					common.JSON(w, http.StatusOK, map[string]any{"items": items, "count": len(items)}); return true
				case "invoices":
					if strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("partner_visible")), "true") {
						common.JSON(w, http.StatusOK, partnerWorkspaceMap(tenant, "portal_billing_invoices")); return true
					}
					items := partnerWorkspaceItems(tenant, "invoices")
					common.JSON(w, http.StatusOK, map[string]any{"items": items, "count": len(items)}); return true
				case "subscriptions":
					common.JSON(w, http.StatusOK, partnerWorkspaceMap(tenant, "portal_billing_subscriptions")); return true
				case "agreement":
					common.JSON(w, http.StatusOK, partnerWorkspaceMap(tenant, "agreement")); return true
				case "commercial-status":
					common.JSON(w, http.StatusOK, partnerWorkspaceMap(tenant, "commercial_status")); return true
				case "events":
					items := partnerWorkspaceItems(tenant, "billing_events")
					common.JSON(w, http.StatusOK, map[string]any{"items": items, "count": len(items)}); return true
				}
			}
		}
	}

	if path == "/api/v1/system-health" || path == "/api/v1/system-health/snapshot" {
		snapshot, _, ok := a.centralSnapshotForRead(r.Context(), centralStep4SystemKey)
		if !ok { common.APIError(w,http.StatusServiceUnavailable,"READ_MODEL_NOT_READY","System read model is not ready"); return true }
		w.Header().Set("X-Himate-Cache","persistent-read-model")
		common.JSON(w,http.StatusOK,partnerWorkspaceMap(snapshot,"health_api"))
		return true
	}

	if strings.HasPrefix(path, "/api/v1/reports/") && !strings.Contains(strings.TrimPrefix(path, "/api/v1/reports/"), "/") {
		id := strings.Trim(strings.TrimPrefix(path, "/api/v1/reports/"), "/")
		snapshot, _, ok := a.centralSnapshotForRead(r.Context(), centralStep4ImpactKey)
		if !ok { common.APIError(w,http.StatusServiceUnavailable,"READ_MODEL_NOT_READY","Impact read model is not ready"); return true }
		for _, item := range step4Items(snapshot["reports"]) {
			if central10String(item["id"]) == id {
				w.Header().Set("X-Himate-Cache","persistent-read-model")
				common.JSON(w,http.StatusOK,item); return true
			}
		}
		common.APIError(w,http.StatusNotFound,"NOT_FOUND","Report not found"); return true
	}

	if strings.HasPrefix(path, "/api/v1/evidence/") && strings.HasSuffix(path, "/integrity") {
		id := strings.Trim(strings.TrimSuffix(strings.TrimPrefix(path, "/api/v1/evidence/"), "/integrity"), "/")
		snapshot, _, ok := a.centralSnapshotForRead(r.Context(), centralStep4ImpactKey)
		if !ok { common.APIError(w,http.StatusServiceUnavailable,"READ_MODEL_NOT_READY","Impact read model is not ready"); return true }
		integrity := partnerWorkspaceMap(snapshot, "evidence_integrity")
		if item, exists := integrity[id]; exists {
			w.Header().Set("X-Himate-Cache","persistent-read-model")
			common.JSON(w,http.StatusOK,item); return true
		}
		common.APIError(w,http.StatusNotFound,"NOT_FOUND","Evidence integrity projection not found"); return true
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
			filtered, err := filterMaterializedAudit(anyItems(audit["items"]), r)
			if err != nil {
				common.APIError(w, http.StatusBadRequest, "VALIDATION", err.Error())
				return true
			}
			limit := auditLimit(r.URL.Query().Get("limit"), 50, 200)
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
		path == "/api/v1/cms/design" ||
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
		case path == "/api/v1/cms/design":
			common.JSON(w, http.StatusOK, partnerWorkspaceMap(snapshot, "cms_design"))
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
