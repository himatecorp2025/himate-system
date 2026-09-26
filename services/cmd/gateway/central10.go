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

const (
	central10ReadBudget = 650 * time.Millisecond
	central10FreshTTL   = 5 * time.Second
	central10StaleTTL   = 45 * time.Second
)

type central10CacheEntry struct {
	payload    map[string]any
	expiresAt  time.Time
	staleUntil time.Time
}

var central10ReadCache = struct {
	sync.RWMutex
	items map[string]central10CacheEntry
}{items: map[string]central10CacheEntry{}}

func (a *app) invalidateCentral10Caches() {
	central10ReadCache.Lock()
	central10ReadCache.items = map[string]central10CacheEntry{}
	central10ReadCache.Unlock()
	a.dashboardMu.Lock()
	a.dashboardPayload = nil
	a.dashboardExpires = time.Time{}
	a.dashboardMu.Unlock()
}

func central10CacheKey(actor user, r *http.Request) string {
	locale := common.RequestLocale(r)
	return actor.ID + "|" + locale + "|" + r.URL.Path + "?" + r.URL.RawQuery
}

func central10Cached(key string, freshOnly bool) (map[string]any, bool, bool) {
	now := time.Now()
	central10ReadCache.RLock()
	entry, ok := central10ReadCache.items[key]
	central10ReadCache.RUnlock()
	if !ok {
		return nil, false, false
	}
	if now.Before(entry.expiresAt) {
		return entry.payload, true, false
	}
	if !freshOnly && now.Before(entry.staleUntil) {
		return entry.payload, true, true
	}
	return nil, false, false
}

func central10Store(key string, payload map[string]any) {
	now := time.Now()
	central10ReadCache.Lock()
	central10ReadCache.items[key] = central10CacheEntry{
		payload: payload, expiresAt: now.Add(central10FreshTTL), staleUntil: now.Add(central10StaleTTL),
	}
	central10ReadCache.Unlock()
}

func central10Meta(started time.Time, status string, unavailable []string) map[string]any {
	sort.Strings(unavailable)
	return map[string]any{
		"architecture": "GO_BACKEND_READ_MODEL",
		"frontend_role": "PRESENTATION_ONLY",
		"target_first_usable_data_ms": 800,
		"backend_budget_ms": central10ReadBudget.Milliseconds(),
		"duration_ms": time.Since(started).Milliseconds(),
		"status": status,
		"unavailable": unavailable,
		"generated_at": time.Now().UTC(),
	}
}

func central10Int(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	case jsonNumberLike:
		n, _ := strconv.Atoi(v.String())
		return n
	default:
		n, _ := strconv.Atoi(strings.TrimSpace(fmt.Sprint(value)))
		return n
	}
}

type jsonNumberLike interface{ String() string }

func central10Float(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	default:
		n, _ := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(value)), 64)
		return n
	}
}

func central10String(value any) string {
	v := strings.TrimSpace(fmt.Sprint(value))
	if v == "<nil>" {
		return ""
	}
	return v
}

func central10CopyMap(src map[string]any) map[string]any {
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func central10QueryLimit(raw string, fallback, max int) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n < 1 {
		return fallback
	}
	if n > max {
		return max
	}
	return n
}

func central10PositiveInt(raw string, fallback int) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n < 1 {
		return fallback
	}
	return n
}

type central10ItemsPage struct {
	Items   []map[string]any `json:"items"`
	Count   int              `json:"count"`
	Total   int              `json:"total"`
	Limit   int              `json:"limit"`
	Offset  int              `json:"offset"`
	HasMore bool             `json:"has_more"`
}

func (a *app) central10AllPartners(ctx context.Context) ([]map[string]any, error) {
	const pageSize = 200
	var first central10ItemsPage
	if err := a.internalGET(
		ctx,
		a.hosts["partners"],
		"/api/v1/partners?limit=200&offset=0&include_archived=false&include_stats=true",
		&first,
	); err != nil {
		return nil, err
	}
	if !first.HasMore || first.Total <= len(first.Items) {
		return first.Items, nil
	}

	pageCount := (first.Total + pageSize - 1) / pageSize
	pages := make([][]map[string]any, pageCount)
	pages[0] = first.Items
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	var errMu sync.Mutex
	var firstErr error

	for pageIndex := 1; pageIndex < pageCount; pageIndex++ {
		pageIndex := pageIndex
		offset := pageIndex * pageSize
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				errMu.Lock()
				if firstErr == nil { firstErr = ctx.Err() }
				errMu.Unlock()
				return
			}
			var page central10ItemsPage
			path := fmt.Sprintf(
				"/api/v1/partners?limit=%d&offset=%d&include_archived=false&include_stats=false",
				pageSize,
				offset,
			)
			if err := a.internalGET(ctx, a.hosts["partners"], path, &page); err != nil {
				errMu.Lock()
				if firstErr == nil { firstErr = err }
				errMu.Unlock()
				return
			}
			pages[pageIndex] = page.Items
		}()
	}
	wg.Wait()

	items := make([]map[string]any, 0, first.Total)
	for _, page := range pages {
		items = append(items, page...)
	}
	if firstErr != nil {
		return items, firstErr
	}
	return items, nil
}

func central10StringChunks(values []string, size int) [][]string {
	if size < 1 { size = 1 }
	chunks := make([][]string, 0, (len(values)+size-1)/size)
	for start := 0; start < len(values); start += size {
		end := start + size
		if end > len(values) { end = len(values) }
		chunks = append(chunks, values[start:end])
	}
	return chunks
}

func (a *app) central10CommercialSources(
	ctx context.Context,
	partnerIDs []string,
	includeBilling bool,
) (matrixItems []map[string]any, subscriptionItems []map[string]any, matrixErr error, subscriptionErr error) {
	if len(partnerIDs) == 0 {
		return []map[string]any{}, []map[string]any{}, nil, nil
	}

	chunks := central10StringChunks(partnerIDs, 80)
	var wg sync.WaitGroup
	var mu sync.Mutex
	sem := make(chan struct{}, 8)

	for _, ids := range chunks {
		ids := append([]string(nil), ids...)
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				mu.Lock()
				if matrixErr == nil { matrixErr = ctx.Err() }
				mu.Unlock()
				return
			}
			var page central10ItemsPage
			encoded := url.QueryEscape(strings.Join(ids, ","))
			err := a.internalGET(ctx, a.hosts["catalog"], "/api/v1/module-commercial-matrix?partner_ids="+encoded, &page)
			mu.Lock()
			if err != nil {
				if matrixErr == nil { matrixErr = err }
			} else {
				matrixItems = append(matrixItems, page.Items...)
			}
			mu.Unlock()
		}()

		if includeBilling {
			wg.Add(1)
			go func() {
				defer wg.Done()
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-ctx.Done():
					mu.Lock()
					if subscriptionErr == nil { subscriptionErr = ctx.Err() }
					mu.Unlock()
					return
				}
				var page central10ItemsPage
				encoded := url.QueryEscape(strings.Join(ids, ","))
				err := a.internalGET(ctx, a.hosts["billing"], "/api/v1/billing/subscription-matrix?partner_ids="+encoded, &page)
				mu.Lock()
				if err != nil {
					if subscriptionErr == nil { subscriptionErr = err }
				} else {
					subscriptionItems = append(subscriptionItems, page.Items...)
				}
				mu.Unlock()
			}()
		}
	}
	wg.Wait()
	return matrixItems, subscriptionItems, matrixErr, subscriptionErr
}

func (a *app) central10ReadModel(w http.ResponseWriter, r *http.Request, actor user) {
	if r.Method != http.MethodGet {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use GET")
		return
	}
	key := central10CacheKey(actor, r)
	if payload, ok, _ := central10Cached(key, true); ok {
		w.Header().Set("X-Himate-Cache", "hit")
		common.JSON(w, http.StatusOK, payload)
		return
	}
	switch {
	case r.URL.Path == "/api/v1/central/partners":
		a.central10Partners(w, r, actor, key)
	case strings.HasPrefix(r.URL.Path, "/api/v1/central/partners/"):
		a.central10PartnerWorkspace(w, r, actor, key)
	case r.URL.Path == "/api/v1/central/modules":
		a.central10Modules(w, r, actor, key)
	case r.URL.Path == "/api/v1/central/packages":
		a.central10Packages(w, r, actor, key)
	case r.URL.Path == "/api/v1/central/finance":
		a.central10Finance(w, r, actor, key)
	case r.URL.Path == "/api/v1/central/impact":
		a.central10Impact(w, r, actor, key)
	default:
		common.APIError(w, http.StatusNotFound, "CENTRAL_READ_MODEL_NOT_FOUND", "Central read model endpoint not found")
	}
}

func (a *app) central10Partners(w http.ResponseWriter, r *http.Request, actor user, cacheKey string) {
	started := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), central10ReadBudget)
	defer cancel()

	type partnerPage struct {
		Items           []map[string]any `json:"items"`
		Count           int              `json:"count"`
		Total           int              `json:"total"`
		Limit           int              `json:"limit"`
		Offset          int              `json:"offset"`
		HasMore         bool             `json:"has_more"`
		LifecycleCounts map[string]int   `json:"lifecycle_counts"`
		ReferenceCount  int              `json:"reference_count"`
	}
	type page struct {
		Items []map[string]any `json:"items"`
	}

	query := r.URL.Query()
	values := url.Values{}
	for _, key := range []string{"q", "category", "lifecycle", "health", "reference", "include_archived", "limit", "offset"} {
		if value := strings.TrimSpace(query.Get(key)); value != "" {
			values.Set(key, value)
		}
	}
	if values.Get("limit") == "" {
		values.Set("limit", "24")
	}
	if values.Get("offset") == "" {
		values.Set("offset", "0")
	}
	values.Set("include_stats", "true")
	partnerPath := "/api/v1/partners?" + values.Encode()

	var partners partnerPage
	var categories, catalogPortfolio, billingPortfolio, healthPortfolio page
	var partnerErr, categoriesErr, catalogErr, billingErr, healthErr error

	var first sync.WaitGroup
	first.Add(2)
	go func() {
		defer first.Done()
		partnerErr = a.internalGET(ctx, a.hosts["partners"], partnerPath, &partners)
	}()
	go func() {
		defer first.Done()
		categoriesErr = a.internalGET(ctx, a.hosts["partners"], "/api/v1/partner-categories", &categories)
	}()
	first.Wait()

	if partnerErr != nil {
		if stale, ok, _ := central10Cached(cacheKey, false); ok {
			stale["meta"] = central10Meta(started, "stale", []string{"partners"})
			w.Header().Set("X-Himate-Cache", "stale")
			common.JSON(w, http.StatusOK, stale)
			return
		}
		common.APIError(w, http.StatusBadGateway, "PARTNERS_UNAVAILABLE", "Partner read model is temporarily unavailable")
		return
	}

	ids := make([]string, 0, len(partners.Items))
	for _, item := range partners.Items {
		if id := central10String(item["id"]); id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) > 0 {
		filter := url.QueryEscape(strings.Join(ids, ","))
		var enrich sync.WaitGroup
		if a.hasPermission(actor, "catalog.read") {
			enrich.Add(1)
			go func() {
				defer enrich.Done()
				catalogErr = a.internalGET(ctx, a.hosts["catalog"], "/internal/v1/portfolio?ids="+filter, &catalogPortfolio)
			}()
		}
		if a.hasPermission(actor, "billing.read") {
			enrich.Add(1)
			go func() {
				defer enrich.Done()
				billingErr = a.internalGET(ctx, a.hosts["billing"], "/internal/v1/portfolio?ids="+filter, &billingPortfolio)
			}()
		}
		if a.hasPermission(actor, "health.read") {
			enrich.Add(1)
			go func() {
				defer enrich.Done()
				healthErr = a.internalGET(ctx, a.hosts["health"], "/internal/v1/system-health/partner-snapshots?ids="+filter, &healthPortfolio)
			}()
		}
		enrich.Wait()
	}

	catalogByID := map[string]map[string]any{}
	for _, item := range catalogPortfolio.Items {
		catalogByID[central10String(item["partner_id"])] = item
	}
	billingByID := map[string]map[string]any{}
	for _, item := range billingPortfolio.Items {
		billingByID[central10String(item["partner_id"])] = item
	}
	healthByID := map[string]map[string]any{}
	for _, item := range healthPortfolio.Items {
		healthByID[central10String(item["partner_id"])] = item
	}

	for _, partner := range partners.Items {
		id := central10String(partner["id"])
		if cat := catalogByID[id]; cat != nil {
			partner["active_modules"] = central10Int(cat["active_modules"])
			partner["extra_module_fee"] = central10Float(cat["extra_module_fee"])
		}
		if bill := billingByID[id]; bill != nil {
			base := central10Float(bill["effective_base_fee"])
			extra := central10Float(partner["extra_module_fee"])
			partner["base_service_fee"] = base
			partner["service_value_30d"] = mathRound2(base + extra)
			if currency := central10String(bill["currency"]); currency != "" {
				partner["currency"] = currency
			}
		}
		if health := healthByID[id]; health != nil {
			if value := central10String(health["overall_status"]); value != "" {
				partner["system_health"] = value
			}
			if value := central10String(health["platform_version"]); value != "" {
				partner["platform_version"] = value
			}
			partner["connector_health"] = health["connector_health"]
			partner["environment_status"] = health["environment_status"]
			partner["provisioning_status"] = health["provisioning_status"]
		}
	}

	unavailable := []string{}
	if categoriesErr != nil { unavailable = append(unavailable, "partner_categories") }
	if catalogErr != nil { unavailable = append(unavailable, "catalog_portfolio") }
	if billingErr != nil { unavailable = append(unavailable, "billing_portfolio") }
	if healthErr != nil { unavailable = append(unavailable, "health_portfolio") }
	status := "healthy"
	if len(unavailable) > 0 { status = "partial" }

	recordTotal := 0
	for _, value := range partners.LifecycleCounts { recordTotal += value }
	if recordTotal == 0 && partners.Total > 0 && strings.TrimSpace(r.URL.Query().Get("q")) == "" {
		recordTotal = partners.Total
	}

	payload := map[string]any{
		"items": partners.Items,
		"categories": categories.Items,
		"pagination": map[string]any{
			"count": partners.Count, "total": partners.Total, "limit": partners.Limit,
			"offset": partners.Offset, "has_more": partners.HasMore,
		},
		"kpis": map[string]any{
			"partner_records": recordTotal,
			"live_partners": partners.LifecycleCounts["LIVE"],
			"prospects": partners.LifecycleCounts["PROSPECT"],
			"reference_partners": partners.ReferenceCount,
			"lifecycle_counts": partners.LifecycleCounts,
		},
		"meta": central10Meta(started, status, unavailable),
	}
	central10Store(cacheKey, payload)
	w.Header().Set("X-Himate-Cache", "miss")
	w.Header().Set("Server-Timing", fmt.Sprintf("central-partners;dur=%d", time.Since(started).Milliseconds()))
	common.JSON(w, http.StatusOK, payload)
}

func central10CanonicalPlan(plan map[string]any, moduleByKey map[string]map[string]any) map[string]any {
	out := central10CopyMap(plan)
	key := strings.ToUpper(central10String(plan["plan_key"]))
	switch key {
	case "STARTER":
		out["display_name"] = "Starter"
		out["monthly_price"] = 990
		out["display_price"] = "$990 + VAT"
		out["module_limit"] = 10
		out["entitlement"] = "10 modules"
	case "BUSINESS":
		out["display_name"] = "Business"
		out["monthly_price"] = 1490
		out["display_price"] = "$1,490 + VAT"
		out["module_limit"] = 20
		out["entitlement"] = "20 modules"
	case "FLEX", "PREMIUM":
		out["plan_key"] = "FLEX"
		out["display_name"] = "Premium"
		out["monthly_price"] = 2490
		out["display_price"] = "$2,490 + VAT"
		out["module_limit"] = nil
		out["entitlement"] = "Unlimited"
	}
	keys := []string{}
	switch raw := plan["fixed_module_keys"].(type) {
	case []any:
		for _, value := range raw {
			if v := central10String(value); v != "" { keys = append(keys, v) }
		}
	case []string:
		keys = append(keys, raw...)
	}
	included := make([]map[string]any, 0, len(keys))
	for _, moduleKey := range keys {
		row := map[string]any{"key": moduleKey, "label": moduleKey}
		if module := moduleByKey[moduleKey]; module != nil {
			row["label"] = module["label"]
			row["group_key"] = module["group_key"]
			row["group_label"] = module["group_label"]
		}
		included = append(included, row)
	}
	out["included_modules"] = included
	return out
}

func (a *app) central10Modules(w http.ResponseWriter, r *http.Request, actor user, cacheKey string) {
	started := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), central10ReadBudget)
	defer cancel()

	type page struct{ Items []map[string]any `json:"items"`; Count int `json:"count"` }
	var modules, groups, plans page
	var partners []map[string]any
	var modulesErr, groupsErr, partnersErr, plansErr error
	var first sync.WaitGroup
	first.Add(2)
	go func(){ defer first.Done(); modulesErr = a.internalGET(ctx, a.hosts["catalog"], "/api/v1/modules", &modules) }()
	go func(){ defer first.Done(); groupsErr = a.internalGET(ctx, a.hosts["catalog"], "/api/v1/module-groups", &groups) }()
	if a.hasPermission(actor, "partners.read") {
		first.Add(1)
		go func(){ defer first.Done(); partners, partnersErr = a.central10AllPartners(ctx) }()
	}
	if a.hasPermission(actor, "billing.read") {
		first.Add(1)
		go func(){ defer first.Done(); plansErr = a.internalGET(ctx, a.hosts["billing"], "/api/v1/billing/plans", &plans) }()
	}
	first.Wait()

	if modulesErr != nil || groupsErr != nil {
		if stale, ok, _ := central10Cached(cacheKey, false); ok {
			stale["meta"] = central10Meta(started, "stale", []string{"catalog"})
			w.Header().Set("X-Himate-Cache", "stale")
			common.JSON(w, http.StatusOK, stale)
			return
		}
		common.APIError(w, http.StatusBadGateway, "CATALOG_UNAVAILABLE", "Module read model is temporarily unavailable")
		return
	}

	partnerName := map[string]string{}
	partnerIDs := []string{}
	for _, p := range partners {
		id := central10String(p["id"])
		if id == "" { continue }
		name := central10String(p["display_name"])
		if name == "" { name = id }
		partnerName[id] = name
		partnerIDs = append(partnerIDs, id)
	}

	matrixItems, subscriptionItems, matrixErr, subscriptionsErr := a.central10CommercialSources(
		ctx,
		partnerIDs,
		a.hasPermission(actor, "billing.read"),
	)

	subByKey := map[string]map[string]any{}
	for _, item := range subscriptionItems {
		key := central10String(item["partner_id"]) + "|" + central10String(item["module_key"])
		subByKey[key] = item
	}

	moduleByKey := map[string]map[string]any{}
	for _, module := range modules.Items {
		moduleByKey[central10String(module["key"])] = module
	}

	registryQ := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("registry_q")))
	registryGroup := strings.TrimSpace(r.URL.Query().Get("registry_group"))
	registryType := strings.TrimSpace(r.URL.Query().Get("registry_type"))
	registryPreset := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("registry_preset")))
	filteredModules := make([]map[string]any, 0, len(modules.Items))
	for _, module := range modules.Items {
		if registryQ != "" {
			text := strings.ToLower(strings.Join([]string{
				central10String(module["label"]), central10String(module["label_en"]), central10String(module["label_hu"]),
				central10String(module["key"]), central10String(module["group_label"]), central10String(module["source_repository"]),
			}, " "))
			if !strings.Contains(text, registryQ) { continue }
		}
		if registryGroup != "" && registryGroup != "ALL" && central10String(module["group_key"]) != registryGroup { continue }
		if registryType != "" && registryType != "ALL" && central10String(module["module_type"]) != registryType { continue }
		switch registryPreset {
		case "ACTIVE":
			if central10String(module["availability"]) != "ACTIVE" || central10String(module["publication_status"]) != "PUBLISHED" || central10String(module["implementation_state"]) != "READY" { continue }
		case "SOURCE_LINKED":
			if central10String(module["source_repository"]) == "" { continue }
		case "RELATIONSHIPS":
			if central10Int(module["relationship_count"]) <= 0 { continue }
		}
		filteredModules = append(filteredModules, module)
	}

	topics := make([]map[string]any, 0, len(groups.Items))
	for _, group := range groups.Items {
		groupKey := central10String(group["group_key"])
		if group["is_primary_navigation"] != true { continue }
		count, liveReady, inDevelopment, assignments := 0, 0, 0, 0
		for _, module := range modules.Items {
			if central10String(module["group_key"]) != groupKey { continue }
			count++
			if central10String(module["availability"]) == "ACTIVE" && central10String(module["publication_status"]) == "PUBLISHED" && central10String(module["implementation_state"]) == "READY" { liveReady++ }
			if central10String(module["implementation_state"]) == "IN_DEVELOPMENT" { inDevelopment++ }
			assignments += central10Int(module["active_partner_count"])
		}
		row := central10CopyMap(group)
		row["module_count"] = count
		row["live_ready"] = liveReady
		row["in_development"] = inDevelopment
		row["active_partner_assignments"] = assignments
		topics = append(topics, row)
	}

	commercialQ := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("commercial_q")))
	partnerFilter := strings.TrimSpace(r.URL.Query().Get("commercial_partner"))
	moduleFilter := strings.TrimSpace(r.URL.Query().Get("commercial_module"))
	statusFilter := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("commercial_status")))
	perspective := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("perspective")))
	if perspective != "MODULE" { perspective = "PARTNER" }

	filteredAssignments := make([]map[string]any, 0, len(matrixItems))
	for _, raw := range matrixItems {
		row := central10CopyMap(raw)
		partnerID := central10String(row["partner_id"])
		moduleKey := central10String(row["key"])
		name := partnerName[partnerID]
		if name == "" { name = partnerID }
		row["partner_name"] = name
		if sub := subByKey[partnerID+"|"+moduleKey]; sub != nil {
			row["subscription"] = sub
		}
		if commercialQ != "" {
			text := strings.ToLower(strings.Join([]string{name, partnerID, central10String(row["label"]), moduleKey, central10String(row["group_label"])}, " "))
			if !strings.Contains(text, commercialQ) { continue }
		}
		if partnerFilter != "" && partnerFilter != "ALL" && partnerID != partnerFilter { continue }
		if moduleFilter != "" && moduleFilter != "ALL" && moduleKey != moduleFilter { continue }
		if statusFilter != "" && statusFilter != "ALL" && strings.ToUpper(central10String(row["status"])) != statusFilter { continue }
		filteredAssignments = append(filteredAssignments, row)
	}

	grouped := []map[string]any{}
	if perspective == "MODULE" {
		byModule := map[string][]map[string]any{}
		for _, row := range filteredAssignments {
			key := central10String(row["key"])
			byModule[key] = append(byModule[key], row)
		}
		keys := make([]string, 0, len(byModule))
		for key := range byModule { keys = append(keys, key) }
		sort.Slice(keys, func(i, j int) bool {
			return strings.ToLower(central10String(moduleByKey[keys[i]]["label"])) < strings.ToLower(central10String(moduleByKey[keys[j]]["label"]))
		})
		for _, key := range keys {
			rows := byModule[key]
			sort.Slice(rows, func(i, j int) bool { return strings.ToLower(central10String(rows[i]["partner_name"])) < strings.ToLower(central10String(rows[j]["partner_name"])) })
			label := key
			if moduleByKey[key] != nil && central10String(moduleByKey[key]["label"]) != "" { label = central10String(moduleByKey[key]["label"]) }
			grouped = append(grouped, map[string]any{"module_key": key, "module_label": label, "partners": rows, "partner_count": len(rows)})
		}
	} else {
		byPartner := map[string][]map[string]any{}
		for _, row := range filteredAssignments {
			id := central10String(row["partner_id"])
			byPartner[id] = append(byPartner[id], row)
		}
		ids := make([]string, 0, len(byPartner))
		for id := range byPartner { ids = append(ids, id) }
		sort.Slice(ids, func(i, j int) bool { return strings.ToLower(partnerName[ids[i]]) < strings.ToLower(partnerName[ids[j]]) })
		for _, id := range ids {
			rows := byPartner[id]
			sort.Slice(rows, func(i, j int) bool { return strings.ToLower(central10String(rows[i]["label"])) < strings.ToLower(central10String(rows[j]["label"])) })
			grouped = append(grouped, map[string]any{"partner_id": id, "partner_name": partnerName[id], "modules": rows, "module_count": len(rows)})
		}
	}
	groupLimit := central10PositiveInt(r.URL.Query().Get("commercial_limit"), 120)
	totalGroups := len(grouped)
	if len(grouped) > groupLimit { grouped = grouped[:groupLimit] }

	liveReady, sourceLinked, relationshipCount, activeAssignments := 0, 0, 0, 0
	for _, module := range modules.Items {
		if central10String(module["availability"]) == "ACTIVE" && central10String(module["publication_status"]) == "PUBLISHED" && central10String(module["implementation_state"]) == "READY" { liveReady++ }
		if central10String(module["source_repository"]) != "" { sourceLinked++ }
		relationshipCount += central10Int(module["relationship_count"])
		activeAssignments += central10Int(module["active_partner_count"])
	}

	unavailable := []string{}
	if partnersErr != nil { unavailable = append(unavailable, "partners") }
	if plansErr != nil { unavailable = append(unavailable, "plans") }
	if matrixErr != nil { unavailable = append(unavailable, "commercial_matrix") }
	if subscriptionsErr != nil { unavailable = append(unavailable, "subscription_matrix") }
	status := "healthy"
	if len(unavailable) > 0 { status = "partial" }

	canonicalPlans := make([]map[string]any, 0, len(plans.Items))
	for _, plan := range plans.Items {
		key := strings.ToUpper(central10String(plan["plan_key"]))
		if key == "STARTER" || key == "BUSINESS" || key == "FLEX" || key == "PREMIUM" {
			canonicalPlans = append(canonicalPlans, central10CanonicalPlan(plan, moduleByKey))
		}
	}

	payload := map[string]any{
		"module_options": modules.Items,
		"registry": map[string]any{
			"modules": filteredModules,
			"groups": groups.Items,
			"topics": topics,
			"kpis": map[string]any{
				"module_registry": len(modules.Items), "active_modules": liveReady, "source_linked": sourceLinked,
				"relationships": relationshipCount, "active_partner_assignments": activeAssignments,
			},
		},
		"commercial": map[string]any{
			"perspective": perspective,
			"groups": grouped,
			"group_count": totalGroups,
			"assignment_count": len(filteredAssignments),
		},
		"partners": partners,
		"plans": canonicalPlans,
		"meta": central10Meta(started, status, unavailable),
	}
	central10Store(cacheKey, payload)
	w.Header().Set("X-Himate-Cache", "miss")
	w.Header().Set("Server-Timing", fmt.Sprintf("central-modules;dur=%d", time.Since(started).Milliseconds()))
	common.JSON(w, http.StatusOK, payload)
}

func (a *app) central10Packages(w http.ResponseWriter, r *http.Request, actor user, cacheKey string) {
	started := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), central10ReadBudget)
	defer cancel()

	type page struct{ Items []map[string]any `json:"items"` }
	var plans, modules page
	var analytics map[string]any
	var plansErr, modulesErr, analyticsErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func(){ defer wg.Done(); plansErr = a.internalGET(ctx, a.hosts["billing"], "/api/v1/billing/plans", &plans) }()
	go func(){ defer wg.Done(); analyticsErr = a.internalGET(ctx, a.hosts["billing"], "/api/v1/billing/packages/analytics", &analytics) }()
	if a.hasPermission(actor, "catalog.read") {
		wg.Add(1)
		go func(){ defer wg.Done(); modulesErr = a.internalGET(ctx, a.hosts["catalog"], "/api/v1/modules", &modules) }()
	}
	wg.Wait()

	if plansErr != nil {
		if stale, ok, _ := central10Cached(cacheKey, false); ok {
			stale["meta"] = central10Meta(started, "stale", []string{"plans"})
			w.Header().Set("X-Himate-Cache", "stale")
			common.JSON(w, http.StatusOK, stale)
			return
		}
		common.APIError(w, http.StatusBadGateway, "PACKAGES_UNAVAILABLE", "Package read model is temporarily unavailable")
		return
	}
	moduleByKey := map[string]map[string]any{}
	eligibleModules := make([]map[string]any, 0, len(modules.Items))
	for _, module := range modules.Items {
		moduleByKey[central10String(module["key"])] = module
		if module["system"] == true {
			eligibleModules = append(eligibleModules, module)
		}
	}
	canonical := []map[string]any{}
	for _, plan := range plans.Items {
		key := strings.ToUpper(central10String(plan["plan_key"]))
		if key == "STARTER" || key == "BUSINESS" || key == "FLEX" || key == "PREMIUM" {
			canonical = append(canonical, central10CanonicalPlan(plan, moduleByKey))
		}
	}
	sort.Slice(canonical, func(i, j int) bool {
		order := map[string]int{"STARTER": 1, "BUSINESS": 2, "FLEX": 3}
		return order[central10String(canonical[i]["plan_key"])] < order[central10String(canonical[j]["plan_key"])]
	})
	unavailable := []string{}
	if modulesErr != nil { unavailable = append(unavailable, "catalog") }
	if analyticsErr != nil { unavailable = append(unavailable, "analytics") }
	status := "healthy"; if len(unavailable) > 0 { status = "partial" }
	if analytics == nil { analytics = map[string]any{"packages": []any{}, "partners": []any{}, "partner_count": 0} }
	payload := map[string]any{
		"plans": canonical,
		"modules": eligibleModules,
		"analytics": analytics,
		"meta": central10Meta(started, status, unavailable),
	}
	central10Store(cacheKey, payload)
	w.Header().Set("X-Himate-Cache", "miss")
	w.Header().Set("Server-Timing", fmt.Sprintf("central-packages;dur=%d", time.Since(started).Milliseconds()))
	common.JSON(w, http.StatusOK, payload)
}

func (a *app) central10Finance(w http.ResponseWriter, r *http.Request, actor user, cacheKey string) {
	started := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), central10ReadBudget)
	defer cancel()

	type page struct{ Items []map[string]any `json:"items"` }
	var profile, overview map[string]any
	var invoices page
	var partners []map[string]any
	var profileErr, overviewErr, invoicesErr, partnersErr error
	var wg sync.WaitGroup
	wg.Add(3)
	go func(){ defer wg.Done(); profileErr = a.internalGET(ctx, a.hosts["billing"], "/api/v1/billing/profile", &profile) }()
	go func(){ defer wg.Done(); overviewErr = a.internalGET(ctx, a.hosts["billing"], "/api/v1/billing/finance/overview", &overview) }()
	go func(){ defer wg.Done(); invoicesErr = a.internalGET(ctx, a.hosts["billing"], "/api/v1/billing/invoices", &invoices) }()
	if a.hasPermission(actor, "partners.read") {
		wg.Add(1)
		go func(){ defer wg.Done(); partners, partnersErr = a.central10AllPartners(ctx) }()
	}
	wg.Wait()

	if profileErr != nil && overviewErr != nil && invoicesErr != nil {
		if stale, ok, _ := central10Cached(cacheKey, false); ok {
			stale["meta"] = central10Meta(started, "stale", []string{"billing"})
			w.Header().Set("X-Himate-Cache", "stale")
			common.JSON(w, http.StatusOK, stale)
			return
		}
		common.APIError(w, http.StatusBadGateway, "FINANCE_UNAVAILABLE", "Finance read model is temporarily unavailable")
		return
	}

	kpis := map[string]any{"draft": 0, "approved": 0, "sent": 0, "paid": 0, "cancelled": 0}
	if rows := anyItems(overview["currencies"]); len(rows) > 0 {
		outstanding := make([]map[string]any, 0, len(rows))
		paidYTD := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			for _, key := range []string{"draft", "approved", "sent", "paid", "cancelled"} {
				kpis[key] = central10Int(kpis[key]) + central10Int(row[key])
			}
			outstanding = append(outstanding, map[string]any{"currency": row["currency"], "amount": row["outstanding"]})
			paidYTD = append(paidYTD, map[string]any{"currency": row["currency"], "amount": row["paid_ytd"]})
		}
		kpis["outstanding"] = outstanding
		kpis["paid_ytd"] = paidYTD
	}
	unavailable := []string{}
	if profileErr != nil { unavailable = append(unavailable, "billing_profile") }
	if overviewErr != nil { unavailable = append(unavailable, "finance_overview") }
	if invoicesErr != nil { unavailable = append(unavailable, "invoices") }
	if partnersErr != nil { unavailable = append(unavailable, "partners") }
	status := "healthy"; if len(unavailable) > 0 { status = "partial" }
	payload := map[string]any{
		"profile": profile,
		"overview": overview,
		"invoices": invoices.Items,
		"partners": partners,
		"kpis": kpis,
		"meta": central10Meta(started, status, unavailable),
	}
	central10Store(cacheKey, payload)
	w.Header().Set("X-Himate-Cache", "miss")
	w.Header().Set("Server-Timing", fmt.Sprintf("central-finance;dur=%d", time.Since(started).Milliseconds()))
	common.JSON(w, http.StatusOK, payload)
}

func (a *app) central10Impact(w http.ResponseWriter, r *http.Request, actor user, cacheKey string) {
	started := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), central10ReadBudget)
	defer cancel()

	type page struct{ Items []map[string]any `json:"items"`; Total int `json:"total"` }
	var definitions, summary, evidence, reports page
	var definitionsErr, summaryErr, evidenceErr, reportsErr error
	var wg sync.WaitGroup
	if a.hasPermission(actor, "impact.read") {
		wg.Add(2)
		go func(){ defer wg.Done(); definitionsErr = a.internalGET(ctx, a.hosts["impact"], "/api/v1/impact/definitions", &definitions) }()
		go func(){ defer wg.Done(); summaryErr = a.internalGET(ctx, a.hosts["impact"], "/api/v1/impact/summary", &summary) }()
	}
	if a.hasPermission(actor, "evidence.read") {
		wg.Add(1)
		evidenceQuery := url.Values{}
		evidenceQuery.Set("limit", strconv.Itoa(central10QueryLimit(r.URL.Query().Get("evidence_limit"), 12, 100)))
		if raw := strings.TrimSpace(r.URL.Query().Get("evidence_offset")); raw != "" {
			evidenceQuery.Set("offset", raw)
		} else {
			evidenceQuery.Set("offset", "0")
		}
		if raw := strings.TrimSpace(r.URL.Query().Get("evidence_query")); raw != "" {
			evidenceQuery.Set("q", raw)
		}
		for _, pair := range [][2]string{
			{"evidence_type", "evidence_type"},
			{"evidence_status", "verification_status"},
			{"evidence_period_start", "period_start"},
			{"evidence_period_end", "period_end"},
		} {
			if raw := strings.TrimSpace(r.URL.Query().Get(pair[0])); raw != "" {
				evidenceQuery.Set(pair[1], raw)
			}
		}
		evidencePath := "/api/v1/evidence?" + evidenceQuery.Encode()
		go func(){ defer wg.Done(); evidenceErr = a.internalGET(ctx, a.hosts["evidence"], evidencePath, &evidence) }()
	}
	if a.hasPermission(actor, "reports.read") {
		wg.Add(1)
		go func(){ defer wg.Done(); reportsErr = a.internalGET(ctx, a.hosts["reports"], "/api/v1/reports", &reports) }()
	}
	wg.Wait()

	unavailable := []string{}
	if definitionsErr != nil { unavailable = append(unavailable, "impact_definitions") }
	if summaryErr != nil { unavailable = append(unavailable, "impact_summary") }
	if evidenceErr != nil { unavailable = append(unavailable, "evidence") }
	if reportsErr != nil { unavailable = append(unavailable, "reports") }
	status := "healthy"; if len(unavailable) > 0 { status = "partial" }
	if len(unavailable) > 0 && len(definitions.Items) == 0 && len(summary.Items) == 0 && len(evidence.Items) == 0 && len(reports.Items) == 0 {
		if stale, ok, _ := central10Cached(cacheKey, false); ok {
			stale["meta"] = central10Meta(started, "stale", unavailable)
			w.Header().Set("X-Himate-Cache", "stale")
			common.JSON(w, http.StatusOK, stale)
			return
		}
	}
	payload := map[string]any{
		"definitions": definitions.Items,
		"summary": summary.Items,
		"evidence": evidence.Items,
		"evidence_total": evidence.Total,
		"reports": reports.Items,
		"meta": central10Meta(started, status, unavailable),
	}
	central10Store(cacheKey, payload)
	w.Header().Set("X-Himate-Cache", "miss")
	w.Header().Set("Server-Timing", fmt.Sprintf("central-impact;dur=%d", time.Since(started).Milliseconds()))
	common.JSON(w, http.StatusOK, payload)
}

func (a *app) central10PartnerWorkspace(w http.ResponseWriter, r *http.Request, actor user, cacheKey string) {
	started := time.Now()
	raw := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/central/partners/"), "/")
	if raw == "" || strings.Contains(raw, "/") {
		common.APIError(w, http.StatusNotFound, "PARTNER_NOT_FOUND", "Partner workspace not found")
		return
	}
	partnerID := raw
	ctx, cancel := context.WithTimeout(r.Context(), central10ReadBudget)
	defer cancel()

	type page struct{ Items []map[string]any `json:"items"` }
	var partner, billing, terms, license, agreement, commercialStatus, paymentProfile, websiteAdapter map[string]any
	var modules, documents, invoices, subscriptions, environments, provisioningJobs, impactSummary, connectorCredentials, billingEvents page
	portalUsers := []map[string]any{}
	unavailable := []string{}
	var unavailableMu sync.Mutex
	mark := func(name string, err error) {
		if err == nil { return }
		unavailableMu.Lock(); unavailable = append(unavailable, name); unavailableMu.Unlock()
	}

	var wg sync.WaitGroup
	runMap := func(name, service, path string, dst *map[string]any) {
		wg.Add(1)
		go func(){ defer wg.Done(); err := a.internalGET(ctx, a.hosts[service], path, dst); mark(name, err) }()
	}
	runPage := func(name, service, path string, dst *page) {
		wg.Add(1)
		go func(){ defer wg.Done(); err := a.internalGET(ctx, a.hosts[service], path, dst); mark(name, err) }()
	}
	runMap("partner", "partners", "/api/v1/partners/"+url.PathEscape(partnerID), &partner)
	if a.hasPermission(actor, "catalog.read") {
		runPage("modules", "catalog", "/api/v1/partners/"+url.PathEscape(partnerID)+"/modules", &modules)
	}
	if a.hasPermission(actor, "billing.read") {
		base := "/api/v1/billing/partners/" + url.PathEscape(partnerID)
		runMap("billing_summary", "billing", base+"/summary", &billing)
		runMap("billing_terms", "billing", base+"/terms", &terms)
		runMap("license", "billing", base+"/license", &license)
		runPage("documents", "billing", base+"/documents", &documents)
		runPage("invoices", "billing", base+"/invoices", &invoices)
		runPage("subscriptions", "billing", base+"/subscriptions", &subscriptions)
		runMap("agreement", "billing", base+"/agreement", &agreement)
		runMap("commercial_status", "billing", base+"/commercial-status", &commercialStatus)
		runPage("billing_events", "billing", base+"/events", &billingEvents)
	}
	if a.hasPermission(actor, "environments.read") {
		runPage("environments", "environments", "/api/v1/environments?partner_id="+url.QueryEscape(partnerID), &environments)
	}
	if a.hasPermission(actor, "provisioning.read") {
		runPage("provisioning", "provisioning", "/api/v1/provisioning/jobs?partner_id="+url.QueryEscape(partnerID), &provisioningJobs)
	}
	if a.hasPermission(actor, "impact.read") {
		runPage("impact", "impact", "/api/v1/impact/summary?partner_id="+url.QueryEscape(partnerID), &impactSummary)
	}
	if a.hasPermission(actor, "connectors.read") {
		runPage("connector_credentials", "connector", "/api/v1/connectors/"+url.PathEscape(partnerID)+"/credential", &connectorCredentials)
		runMap("website_adapter", "connector", "/api/v1/connectors/"+url.PathEscape(partnerID)+"/website-adapter?environment=PRODUCTION", &websiteAdapter)
	}
	if a.hasPermission(actor, "billing.read") {
		runMap("payment_profile", "payments", "/api/v1/payments/partners/"+url.PathEscape(partnerID)+"/profile", &paymentProfile)
	}
	if actor.SystemOwner && a.hasPermission(actor, "administration.read") {
		wg.Add(1)
		go func(){
			defer wg.Done()
			items, err := a.listPartnerUsers(partnerID)
			if err != nil { mark("portal_users", err); return }
			portalUsers = items
		}()
	}
	wg.Wait()

	if partner == nil {
		if stale, ok, _ := central10Cached(cacheKey, false); ok {
			stale["meta"] = central10Meta(started, "stale", unavailable)
			w.Header().Set("X-Himate-Cache", "stale")
			common.JSON(w, http.StatusOK, stale)
			return
		}
		common.APIError(w, http.StatusBadGateway, "PARTNER_WORKSPACE_UNAVAILABLE", "Partner workspace is temporarily unavailable")
		return
	}

	status := "healthy"; if len(unavailable) > 0 { status = "partial" }
	payload := map[string]any{
		"partner": partner,
		"modules": modules.Items,
		"billing": billing,
		"terms": terms,
		"license": license,
		"documents": documents.Items,
		"invoices": invoices.Items,
		"subscriptions": subscriptions.Items,
		"environments": environments.Items,
		"provisioning_jobs": provisioningJobs.Items,
		"impact_summary": impactSummary.Items,
		"connector_credentials": connectorCredentials.Items,
		"portal_users": portalUsers,
		"agreement": agreement,
		"commercial_status": commercialStatus,
		"billing_events": billingEvents.Items,
		"website_adapter": websiteAdapter,
		"payment_profile": paymentProfile,
		"meta": central10Meta(started, status, unavailable),
	}
	central10Store(cacheKey, payload)
	w.Header().Set("X-Himate-Cache", "miss")
	w.Header().Set("Server-Timing", fmt.Sprintf("central-partner-workspace;dur=%d", time.Since(started).Milliseconds()))
	common.JSON(w, http.StatusOK, payload)
}

func central10NormalizeDashboardImpact(impact map[string]any) map[string]any {
	if impact == nil {
		return map[string]any{"people_reached_ytd": 0, "trend": []any{}, "weekly_trend": []any{}, "has_data": false}
	}
	out := central10CopyMap(impact)
	hasData := impact["has_data"] == true || central10Int(impact["observation_count"]) > 0
	if !hasData {
		out["trend"] = []any{}
		out["weekly_trend"] = []any{}
		out["has_data"] = false
		return out
	}
	now := time.Now().UTC()
	startOfCurrentWeek := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -(int(now.Weekday())+6)%7)
	weekly := anyItems(impact["weekly_trend"])
	elapsed := make([]map[string]any, 0, len(weekly))
	for _, row := range weekly {
		parsed, err := time.Parse("2006-01-02", central10String(row["week_start"]))
		if err != nil || parsed.After(startOfCurrentWeek) { continue }
		copyRow := central10CopyMap(row)
		copyRow["label"] = fmt.Sprintf("%d/%d", int(parsed.Month()), parsed.Day())
		elapsed = append(elapsed, copyRow)
	}
	sort.Slice(elapsed, func(i, j int) bool { return central10String(elapsed[i]["week_start"]) < central10String(elapsed[j]["week_start"]) })
	if len(elapsed) > 4 { elapsed = elapsed[len(elapsed)-4:] }
	out["weekly_trend"] = elapsed
	out["has_data"] = true
	return out
}
