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
	central10FreshTTL   = 30 * time.Second
	central10StaleTTL   = 10 * time.Minute
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

func (a *app) invalidateCentral10Caches(path string) {
	path = strings.ToLower(strings.TrimSpace(path))
	invalidatePartners := strings.Contains(path, "partner")
	invalidateConnections := invalidatePartners || strings.Contains(path, "connector")
	invalidateAdministration := invalidatePartners ||
		strings.Contains(path, "billing") ||
		strings.Contains(path, "invoice") ||
		strings.Contains(path, "backup") ||
		strings.Contains(path, "admin")
	invalidateWorkspaceModules := invalidatePartners ||
		strings.Contains(path, "module") ||
		strings.Contains(path, "billing") ||
		strings.Contains(path, "subscription")
	invalidateWebsite := strings.Contains(path, "cms") ||
		strings.Contains(path, "environment") ||
		strings.Contains(path, "connector")
	invalidateSystem := strings.Contains(path, "system-health") ||
		strings.Contains(path, "provision") ||
		strings.Contains(path, "environment") ||
		strings.Contains(path, "connector") ||
		strings.Contains(path, "backup")

	central10ReadCache.Lock()
	for key := range central10ReadCache.items {
		lowerKey := strings.ToLower(key)
		remove := false
		if invalidatePartners && strings.Contains(lowerKey, "/api/v1/central/partners?") {
			remove = true
		}
		if invalidateWorkspaceModules &&
			strings.Contains(lowerKey, "/api/v1/central/partners/") {
			remove = true
		}
		if invalidateConnections && strings.Contains(lowerKey, "/api/v1/central/connections?") {
			remove = true
		}
		if invalidateAdministration && strings.Contains(lowerKey, "/api/v1/central/administration?") {
			remove = true
		}
		if invalidateWebsite && strings.Contains(lowerKey, "/api/v1/central/website?") {
			remove = true
		}
		if invalidateSystem && strings.Contains(lowerKey, "/api/v1/central/system?") {
			remove = true
		}
		if remove {
			delete(central10ReadCache.items, key)
		}
	}
	central10ReadCache.Unlock()

	// Materialized screen snapshots are never destructively flushed. Relevant
	// mutations only queue recomputation while last-known-good data remains hot.
	switch {
	case strings.Contains(path, "partner"):
		a.requestDashboardRefresh()
		a.requestCentralStep3Refresh()
		a.requestCentralStep4Refresh()
	case strings.Contains(path, "module"), strings.Contains(path, "catalog"):
		a.requestDashboardRefresh()
		a.requestCentralStep3Refresh()
		// Partners materialization contains catalog-derived module/commercial
		// enrichment, so module changes must refresh that screen snapshot too.
		a.requestCentralStep4Refresh()
	case strings.Contains(path, "billing"), strings.Contains(path, "invoice"), strings.Contains(path, "subscription"):
		a.requestDashboardRefresh()
		a.requestCentralStep3Refresh()
		a.requestCentralStep4Refresh()
	case strings.Contains(path, "impact"), strings.Contains(path, "evidence"), strings.Contains(path, "report"):
		a.requestDashboardRefresh()
		a.requestCentralStep4Refresh()
	}

	if invalidateWorkspaceModules ||
		strings.Contains(path, "environment") ||
		strings.Contains(path, "provision") ||
		strings.Contains(path, "connector") ||
		strings.Contains(path, "impact") ||
		strings.Contains(path, "evidence") ||
		strings.Contains(path, "report") ||
		strings.Contains(path, "cms") ||
		strings.Contains(path, "payment") {
		a.requestAllCentralPartnerWorkspaceRefreshes()
	}
}

func central10MergeModuleSnapshot(snapshot, state map[string]any) (map[string]any, bool) {
	if snapshot == nil || state == nil {
		return snapshot, false
	}
	key := central10String(state["key"])
	if key == "" {
		return snapshot, false
	}
	rows := anyItems(snapshot["modules"])
	found := false
	for i, row := range rows {
		if central10String(row["key"]) != key {
			continue
		}
		merged := central10CopyMap(row)
		for field, value := range state {
			merged[field] = value
		}
		rows[i] = merged
		found = true
		break
	}
	if !found {
		return snapshot, false
	}
	out := central10CopyMap(snapshot)
	out["modules"] = rows
	return out, true
}

func (a *app) applyCentralModuleMutationSnapshot(path string, state map[string]any) {
	normalized := strings.Trim(strings.ToLower(strings.TrimSpace(path)), "/")
	const prefix = "api/v1/modules/"
	if !strings.HasPrefix(normalized, prefix) {
		return
	}
	tail := strings.TrimPrefix(normalized, prefix)
	if tail == "" || strings.Contains(tail, "/") {
		return
	}
	snapshot, _, ok := centralStep3SnapshotGet(centralStep3RegistryKey)
	if !ok {
		return
	}
	merged, changed := central10MergeModuleSnapshot(snapshot, state)
	if !changed {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	a.centralStep3Store(ctx, centralStep3RegistryKey, merged)
}

func central10PackageEligibleModule(module map[string]any) bool {
	return strings.EqualFold(central10String(module["availability"]), "ACTIVE") &&
		strings.EqualFold(central10String(module["publication_status"]), "PUBLISHED") &&
		strings.EqualFold(central10String(module["implementation_state"]), "READY")
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
	// CENTRAL-21: every browser read reaches the persistent materialized
	// projection. The legacy process-memory response cache may only remain as
	// an implementation detail for non-screen code; it must never bypass the
	// indexed DB-first read path.
	key := central10CacheKey(actor, r)
	switch {
	case r.URL.Path == "/api/v1/central/partners":
		a.central10Partners(w, r, actor, key)
	case strings.HasPrefix(r.URL.Path, "/api/v1/central/partners/") && strings.HasSuffix(r.URL.Path, "/modules"):
		a.central10PartnerModules(w, r, actor, key)
	case strings.HasPrefix(r.URL.Path, "/api/v1/central/partners/"):
		a.central10PartnerWorkspace(w, r, actor, key)
	case r.URL.Path == "/api/v1/central/modules/commercial":
		a.central10ModulesCommercial(w, r, actor)
	case r.URL.Path == "/api/v1/central/modules":
		a.central10Modules(w, r, actor, key)
	case r.URL.Path == "/api/v1/central/packages/supplementary":
		a.central10PackagesSupplementary(w, r, actor)
	case r.URL.Path == "/api/v1/central/packages":
		a.central10Packages(w, r, actor, key)
	case r.URL.Path == "/api/v1/central/finance":
		a.central10Finance(w, r, actor, key)
	case r.URL.Path == "/api/v1/central/impact":
		a.central10Impact(w, r, actor, key)
	case r.URL.Path == "/api/v1/central/website":
		a.central17Website(w, r, actor, key)
	case r.URL.Path == "/api/v1/central/system":
		a.central17System(w, r, actor, key)
	default:
		common.APIError(w, http.StatusNotFound, "CENTRAL_READ_MODEL_NOT_FOUND", "Central read model endpoint not found")
	}
}

func central10PartnerCategories(locale string, remote []map[string]any) []map[string]any {
	type categorySeed struct {
		id, en, hu, slug string
	}
	seeds := []categorySeed{
		{"cat_001", "Classical Music", "Klasszikus zene", "classical-music"},
		{"cat_002", "Fine Art", "Képzőművészet", "fine-art"},
		{"cat_003", "Gallery", "Galéria", "gallery"},
		{"cat_004", "Theatre", "Színház", "theatre"},
		{"cat_005", "Cultural Organization", "Kulturális szervezet", "cultural-organization"},
		{"cat_006", "Other", "Egyéb", "other"},
	}
	hu := strings.HasPrefix(strings.ToLower(strings.TrimSpace(locale)), "hu")
	byID := make(map[string]map[string]any, len(seeds)+len(remote))
	for _, seed := range seeds {
		name := seed.en
		if hu { name = seed.hu }
		byID[seed.id] = map[string]any{
			"id": seed.id, "name": name, "name_en": seed.en, "name_hu": seed.hu,
			"slug": seed.slug, "system": true,
		}
	}
	for _, raw := range remote {
		id := central10String(raw["id"])
		if id == "" { continue }
		row := central10CopyMap(raw)
		if central10String(row["name"]) == "" {
			if hu {
				row["name"] = row["name_hu"]
			} else {
				row["name"] = row["name_en"]
			}
		}
		byID[id] = row
	}
	out := make([]map[string]any, 0, len(byID))
	for _, seed := range seeds {
		out = append(out, byID[seed.id])
		delete(byID, seed.id)
	}
	extra := make([]map[string]any, 0, len(byID))
	for _, row := range byID { extra = append(extra, row) }
	sort.Slice(extra, func(i, j int) bool {
		return strings.ToLower(central10String(extra[i]["name"])) < strings.ToLower(central10String(extra[j]["name"]))
	})
	out = append(out, extra...)
	return out
}

func (a *app) central10Partners(w http.ResponseWriter, r *http.Request, actor user, cacheKey string) {
	started := time.Now()
	snapshot, updatedAt, ok := a.centralSnapshotForRead(r.Context(), centralStep4PartnersKey)
	if !ok {
		a.readModelInvariantFailure(w, centralStep4PartnersKey)
		return
	}

	q := r.URL.Query()
	rawCategories := step4Items(snapshot["categories_raw"])
	snapshotItems := step4Items(snapshot["items"])
	needle := strings.ToLower(strings.TrimSpace(q.Get("q")))
	category := strings.TrimSpace(q.Get("category"))
	lifecycle := strings.ToUpper(strings.TrimSpace(q.Get("lifecycle")))
	health := strings.ToUpper(strings.TrimSpace(q.Get("health")))
	referenceOnly := strings.EqualFold(strings.TrimSpace(q.Get("reference")), "true")
	limit := central10QueryLimit(q.Get("limit"), 24, 200)
	offset := 0
	if parsed, err := strconv.Atoi(strings.TrimSpace(q.Get("offset"))); err == nil && parsed > 0 {
		offset = parsed
	}

	filtered := make([]map[string]any, 0, len(snapshotItems))
	for _, raw := range snapshotItems {
		if needle != "" && !searchContains(
			needle,
			raw["id"], raw["display_name"], raw["legal_name"], raw["brand_name"],
			raw["category_id"], raw["category_key"], raw["category_name"],
			raw["country"], raw["state_region"], raw["city"],
			raw["contact_name"], raw["contact_email"],
		) {
			continue
		}
		if category != "" && category != "ALL" &&
			!strings.EqualFold(category, central10String(raw["category_id"])) &&
			!strings.EqualFold(category, central10String(raw["category_key"])) {
			continue
		}
		if lifecycle != "" && lifecycle != "ALL" &&
			!strings.EqualFold(lifecycle, central10String(raw["lifecycle"])) {
			continue
		}
		if health != "" && health != "ALL" {
			rowHealth := strings.ToUpper(central10String(raw["system_health"]))
			if rowHealth == "" {
				rowHealth = "UNKNOWN"
			}
			if rowHealth != health {
				continue
			}
		}
		if referenceOnly && raw["reference_partner"] != true {
			continue
		}
		row := central10CopyMap(raw)
		if !a.hasPermission(actor, "catalog.read") {
			delete(row, "active_modules")
			delete(row, "extra_module_fee")
		}
		if !a.hasPermission(actor, "billing.read") {
			delete(row, "base_service_fee")
			delete(row, "service_value_30d")
			delete(row, "currency")
		}
		if !a.hasPermission(actor, "health.read") {
			delete(row, "system_health")
			delete(row, "platform_version")
			delete(row, "connector_health")
			delete(row, "environment_status")
			delete(row, "provisioning_status")
		}
		filtered = append(filtered, row)
	}

	total := len(filtered)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	visibleItems := filtered[offset:end]
	payload := map[string]any{
		"ready":      true,
		"items":      visibleItems,
		"categories": central10PartnerCategories(common.RequestLocale(r), rawCategories),
		"pagination": map[string]any{
			"count": len(visibleItems), "total": total, "limit": limit,
			"offset": offset, "has_more": end < total,
		},
		"kpis": step4Map(snapshot["kpis"]),
		"meta": centralStep4Meta(started, centralStep4PartnersKey, updatedAt, "healthy", []string{}),
	}
	w.Header().Set("X-Himate-Cache", "hot-snapshot")
	w.Header().Set("Server-Timing", fmt.Sprintf("central-partners-snapshot;dur=%d", time.Since(started).Milliseconds()))
	common.JSON(w, http.StatusOK, payload)
}

func central10GroupedInt(value int64) string {
	raw := strconv.FormatInt(value, 10)
	if len(raw) <= 3 {
		return raw
	}
	first := len(raw) % 3
	if first == 0 {
		first = 3
	}
	var b strings.Builder
	b.WriteString(raw[:first])
	for index := first; index < len(raw); index += 3 {
		b.WriteByte(',')
		b.WriteString(raw[index:index+3])
	}
	return b.String()
}

func central10PlanDisplayPrice(currency string, amount float64) string {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	prefix := currency + " "
	switch currency {
	case "USD":
		prefix = "$"
	case "EUR":
		prefix = "€"
	case "GBP":
		prefix = "£"
	}
	var value string
	if amount == float64(int64(amount)) {
		value = central10GroupedInt(int64(amount))
	} else {
		value = fmt.Sprintf("%.2f", amount)
	}
	return prefix + value + " + VAT"
}

func central10CanonicalPlan(plan map[string]any, moduleByKey map[string]map[string]any) map[string]any {
	out := central10CopyMap(plan)
	key := strings.ToUpper(central10String(plan["plan_key"]))
	switch key {
	case "STARTER":
		out["display_name"] = "Starter"
		out["module_limit"] = 10
		out["selection_mode"] = "FIXED"
		out["entitlement"] = "10 modules"
	case "BUSINESS":
		out["display_name"] = "Business"
		out["module_limit"] = 20
		out["selection_mode"] = "FIXED"
		out["entitlement"] = "20 modules"
	case "FLEX", "PREMIUM":
		out["plan_key"] = "FLEX"
		out["display_name"] = "Premium"
		out["module_limit"] = nil
		out["selection_mode"] = "UNLIMITED"
		out["entitlement"] = "Unlimited"
	}
	out["display_price"] = central10PlanDisplayPrice(
		central10String(plan["currency"]),
		central10Float(plan["monthly_price"]),
	)
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
	snapshot, updatedAt, ok := a.centralSnapshotForRead(r.Context(), centralStep3RegistryKey)
	if !ok {
		a.readModelInvariantFailure(w, centralStep3RegistryKey)
		return
	}

	modules := anyItems(snapshot["modules"])
	groups := anyItems(snapshot["groups"])
	trend := anyItems(snapshot["trend"])

	registryQ := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("registry_q")))
	registryGroup := strings.TrimSpace(r.URL.Query().Get("registry_group"))
	registryType := strings.TrimSpace(r.URL.Query().Get("registry_type"))
	registryPreset := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("registry_preset")))
	filteredModules := make([]map[string]any, 0, len(modules))
	for _, module := range modules {
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

	topics := make([]map[string]any, 0, len(groups))
	for _, group := range groups {
		groupKey := central10String(group["group_key"])
		if group["is_primary_navigation"] != true { continue }
		count, liveReady, inDevelopment, assignments := 0, 0, 0, 0
		for _, module := range modules {
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

	liveReady, sourceLinked, relationshipCount, activeAssignments := 0, 0, 0, 0
	for _, module := range modules {
		if central10String(module["availability"]) == "ACTIVE" && central10String(module["publication_status"]) == "PUBLISHED" && central10String(module["implementation_state"]) == "READY" { liveReady++ }
		if central10String(module["source_repository"]) != "" { sourceLinked++ }
		relationshipCount += central10Int(module["relationship_count"])
		activeAssignments += central10Int(module["active_partner_count"])
	}

	payload := map[string]any{
		"ready": true,
		"module_options": modules,
		"registry": map[string]any{
			"modules": filteredModules,
			"groups": groups,
			"topics": topics,
			"trend": trend,
			"kpis": map[string]any{
				"module_registry": len(modules), "active_modules": liveReady, "source_linked": sourceLinked,
				"relationships": relationshipCount, "active_partner_assignments": activeAssignments,
			},
		},
		"meta": centralStep3Meta(started, centralStep3RegistryKey, updatedAt, "healthy", []string{}),
	}
	w.Header().Set("X-Himate-Cache", "hot-snapshot")
	w.Header().Set("Server-Timing", fmt.Sprintf("central-modules-registry;dur=%d", time.Since(started).Milliseconds()))
	common.JSON(w, http.StatusOK, payload)
}

func (a *app) central10ModulesCommercial(w http.ResponseWriter, r *http.Request, actor user) {
	started := time.Now()
	commercialSnapshot, commercialUpdatedAt, commercialOK := a.centralSnapshotForRead(r.Context(), centralStep3CommercialKey)
	if !commercialOK {
		a.readModelInvariantFailure(w, centralStep3CommercialKey)
		return
	}

	modules := anyItems(commercialSnapshot["modules"])
	partners := []map[string]any{}
	matrixItems := []map[string]any{}
	subscriptionItems := []map[string]any{}
	if a.hasPermission(actor, "partners.read") {
		partners = anyItems(commercialSnapshot["partners"])
		matrixItems = anyItems(commercialSnapshot["matrix_items"])
		if a.hasPermission(actor, "billing.read") {
			subscriptionItems = anyItems(commercialSnapshot["subscription_items"])
		}
	}

	partnerName := map[string]string{}
	for _, p := range partners {
		id := central10String(p["id"])
		if id == "" { continue }
		name := central10String(p["display_name"])
		if name == "" { name = id }
		partnerName[id] = name
	}

	subByKey := map[string]map[string]any{}
	for _, item := range subscriptionItems {
		key := central10String(item["partner_id"]) + "|" + central10String(item["module_key"])
		subByKey[key] = item
	}
	moduleByKey := map[string]map[string]any{}
	for _, module := range modules {
		moduleByKey[central10String(module["key"])] = module
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
		if sub := subByKey[partnerID+"|"+moduleKey]; sub != nil { row["subscription"] = sub }
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

	canonicalPlans := []map[string]any{}
	if a.hasPermission(actor, "billing.read") {
		for _, plan := range anyItems(commercialSnapshot["plans"]) {
			key := strings.ToUpper(central10String(plan["plan_key"]))
			if key == "STARTER" || key == "BUSINESS" || key == "FLEX" || key == "PREMIUM" {
				canonicalPlans = append(canonicalPlans, central10CanonicalPlan(plan, moduleByKey))
			}
		}
	}

	payload := map[string]any{
		"ready": true,
		"partners": partners,
		"commercial": map[string]any{
			"available": true,
			"perspective": perspective,
			"groups": grouped,
			"group_count": totalGroups,
			"assignment_count": len(filteredAssignments),
		},
		"plans": canonicalPlans,
		"meta": centralStep3Meta(started, centralStep3CommercialKey, commercialUpdatedAt, "healthy", []string{}),
	}
	w.Header().Set("X-Himate-Cache", "hot-snapshot")
	w.Header().Set("Server-Timing", fmt.Sprintf("central-modules-commercial;dur=%d", time.Since(started).Milliseconds()))
	common.JSON(w, http.StatusOK, payload)
}

func (a *app) central10Packages(w http.ResponseWriter, r *http.Request, actor user, cacheKey string) {
	started := time.Now()
	plansSnapshot, updatedAt, ok := a.centralSnapshotForRead(r.Context(), centralStep3PlansKey)
	if !ok {
		a.readModelInvariantFailure(w, centralStep3PlansKey)
		return
	}

	moduleByKey := map[string]map[string]any{}
	for _, module := range anyItems(plansSnapshot["modules"]) {
		moduleByKey[central10String(module["key"])] = module
	}
	canonical := []map[string]any{}
	for _, plan := range anyItems(plansSnapshot["plans"]) {
		key := strings.ToUpper(central10String(plan["plan_key"]))
		if key == "STARTER" || key == "BUSINESS" || key == "FLEX" || key == "PREMIUM" {
			canonical = append(canonical, central10CanonicalPlan(plan, moduleByKey))
		}
	}
	sort.Slice(canonical, func(i, j int) bool {
		order := map[string]int{"STARTER": 1, "BUSINESS": 2, "FLEX": 3}
		return order[central10String(canonical[i]["plan_key"])] < order[central10String(canonical[j]["plan_key"])]
	})
	payload := map[string]any{
		"ready": true,
		"plans": canonical,
		"meta": centralStep3Meta(started, centralStep3PlansKey, updatedAt, "healthy", []string{}),
	}
	w.Header().Set("X-Himate-Cache", "hot-snapshot")
	w.Header().Set("Server-Timing", fmt.Sprintf("central-packages-plans;dur=%d", time.Since(started).Milliseconds()))
	common.JSON(w, http.StatusOK, payload)
}

func (a *app) central10PackagesSupplementary(w http.ResponseWriter, r *http.Request, actor user) {
	started := time.Now()
	analyticsSnapshot, analyticsUpdatedAt, analyticsOK := a.centralSnapshotForRead(r.Context(), centralStep3AnalyticsKey)
	if !analyticsOK {
		a.readModelInvariantFailure(w, "packages_supplementary")
		return
	}

	eligibleModules := []map[string]any{}
	if a.hasPermission(actor, "catalog.read") {
		for _, module := range anyItems(analyticsSnapshot["modules"]) {
			if central10PackageEligibleModule(module) {
				eligibleModules = append(eligibleModules, module)
			}
		}
	}
	analytics := map[string]any{}
	if raw, ok := analyticsSnapshot["analytics"].(map[string]any); ok {
		analytics = central10CopyMap(raw)
	}
	updatedAt := analyticsUpdatedAt

	payload := map[string]any{
		"ready": true,
		"modules_ready": true,
		"analytics_ready": true,
		"modules": eligibleModules,
		"analytics": analytics,
		"meta": centralStep3Meta(started, "packages_supplementary", updatedAt, "healthy", []string{}),
	}
	w.Header().Set("X-Himate-Cache", "hot-snapshot")
	w.Header().Set("Server-Timing", fmt.Sprintf("central-packages-supplementary;dur=%d", time.Since(started).Milliseconds()))
	common.JSON(w, http.StatusOK, payload)
}

func central10MoneyLabel(rows []map[string]any, key string) string {
	nonZero := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if central10Float(row[key]) != 0 {
			nonZero = append(nonZero, row)
		}
	}
	if len(nonZero) == 0 {
		return "$0.00"
	}
	if len(nonZero) == 1 {
		return fmt.Sprintf("%s %.2f", central10String(nonZero[0]["currency"]), central10Float(nonZero[0][key]))
	}
	return fmt.Sprintf("%d currencies", len(nonZero))
}

func central10FinanceChart(overview map[string]any, period, planKey, requestedCurrency string) map[string]any {
	period = strings.ToUpper(strings.TrimSpace(period))
	if period != "WEEKLY" {
		period = "MONTHLY"
	}
	planKey = strings.ToUpper(strings.TrimSpace(planKey))
	switch planKey {
	case "STARTER", "BUSINESS", "FLEX":
	default:
		planKey = "ALL"
	}

	currencyRows := anyItems(overview["currencies"])
	currency := strings.ToUpper(strings.TrimSpace(requestedCurrency))
	if currency == "" && len(currencyRows) > 0 {
		currency = strings.ToUpper(central10String(currencyRows[0]["currency"]))
	}
	if currency == "" {
		currency = "USD"
	}

	byPlan := planKey != "ALL"
	key := "monthly_paid"
	if period == "WEEKLY" {
		key = "weekly_paid"
	}
	if byPlan {
		if period == "WEEKLY" {
			key = "weekly_paid_by_plan"
		} else {
			key = "monthly_paid_by_plan"
		}
	}

	rawRows := anyItems(overview[key])
	rows := make([]map[string]any, 0, len(rawRows))
	maxPaid := 0.0
	for _, raw := range rawRows {
		if strings.ToUpper(central10String(raw["currency"])) != currency {
			continue
		}
		if byPlan && strings.ToUpper(central10String(raw["plan_key"])) != planKey {
			continue
		}
		row := central10CopyMap(raw)
		if central10String(row["period"]) == "" {
			row["period"] = row["month"]
		}
		paid := central10Float(row["paid"])
		if paid > maxPaid {
			maxPaid = paid
		}
		rows = append(rows, row)
	}
	return map[string]any{
		"period": period,
		"plan_key": planKey,
		"currency": currency,
		"rows": rows,
		"max_paid": mathRound2(maxPaid),
	}
}

func central10Step4Unavailable(raw any) []string {
	values := []string{}
	switch items := raw.(type) {
	case []string:
		values = append(values, items...)
	case []any:
		for _, item := range items {
			if value := central10String(item); value != "" {
				values = append(values, value)
			}
		}
	}
	sort.Strings(values)
	return values
}

func (a *app) central10Finance(w http.ResponseWriter, r *http.Request, actor user, cacheKey string) {
	started := time.Now()
	snapshot, updatedAt, ok := a.centralSnapshotForRead(r.Context(), centralStep4FinanceKey)
	if !ok {
		a.readModelInvariantFailure(w, centralStep4FinanceKey)
		return
	}

	profile := step4Map(snapshot["profile"])
	overview := step4Map(snapshot["overview"])
	invoices := step4Items(snapshot["invoices"])
	partners := step4Items(snapshot["partners"])

	currencyRows := anyItems(overview["currencies"])
	kpis := map[string]any{"draft": 0, "approved": 0, "sent": 0, "paid": 0, "cancelled": 0}
	if len(currencyRows) > 0 {
		outstanding := make([]map[string]any, 0, len(currencyRows))
		paidYTD := make([]map[string]any, 0, len(currencyRows))
		for _, row := range currencyRows {
			for _, key := range []string{"draft", "approved", "sent", "paid", "cancelled"} {
				kpis[key] = central10Int(kpis[key]) + central10Int(row[key])
			}
			outstanding = append(outstanding, map[string]any{"currency": row["currency"], "amount": row["outstanding"]})
			paidYTD = append(paidYTD, map[string]any{"currency": row["currency"], "amount": row["paid_ytd"]})
		}
		kpis["outstanding"] = outstanding
		kpis["paid_ytd"] = paidYTD
	}
	kpis["outstanding_label"] = central10MoneyLabel(currencyRows, "outstanding")
	kpis["paid_ytd_label"] = central10MoneyLabel(currencyRows, "paid_ytd")
	kpis["outstanding_invoice_count"] = central10Int(kpis["approved"]) + central10Int(kpis["sent"])

	onboarding := map[string]any{}
	if raw, ok := overview["onboarding"].(map[string]any); ok {
		onboarding = central10CopyMap(raw)
	}
	onboardingItems := anyItems(onboarding["items"])
	kpis["pending_onboarding"] = central10Int(onboarding["pending"])

	partnerNames := make(map[string]string, len(partners))
	for _, partner := range partners {
		id := central10String(partner["id"])
		name := central10String(partner["display_name"])
		if name == "" {
			name = id
		}
		if id != "" {
			partnerNames[id] = name
		}
	}

	invoiceStatus := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("invoice_status")))
	switch invoiceStatus {
	case "DRAFT", "APPROVED", "SENT", "PAID", "CANCELLED":
	default:
		invoiceStatus = "ALL"
	}
	filteredInvoices := make([]map[string]any, 0, len(invoices))
	for _, raw := range invoices {
		row := central10CopyMap(raw)
		partnerID := central10String(row["partner_id"])
		name := partnerNames[partnerID]
		if name == "" {
			name = partnerID
		}
		row["partner_name"] = name
		status := strings.ToUpper(central10String(row["workflow_status"]))
		if status == "" {
			status = strings.ToUpper(central10String(row["status"]))
		}
		if invoiceStatus != "ALL" && status != invoiceStatus {
			continue
		}
		filteredInvoices = append(filteredInvoices, row)
	}

	chart := central10FinanceChart(
		overview,
		r.URL.Query().Get("revenue_period"),
		r.URL.Query().Get("revenue_plan"),
		r.URL.Query().Get("currency"),
	)

	status := "healthy"
	payload := map[string]any{
		"ready": true,
		"profile": profile,
		"overview": overview,
		"invoices": filteredInvoices,
		"invoice_filter": invoiceStatus,
		"partners": partners,
		"onboarding": onboardingItems,
		"onboarding_summary": onboarding,
		"chart": chart,
		"kpis": kpis,
		"meta": centralStep4Meta(started, centralStep4FinanceKey, updatedAt, status, []string{}),
	}
	w.Header().Set("X-Himate-Cache", "hot-snapshot")
	w.Header().Set("Server-Timing", fmt.Sprintf("central-finance-snapshot;dur=%d", time.Since(started).Milliseconds()))
	common.JSON(w, http.StatusOK, payload)
}

func central10Step4EvidenceMatches(row map[string]any, r *http.Request) bool {
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("evidence_query")))
	if query != "" && !strings.Contains(strings.ToLower(fmt.Sprint(row)), query) {
		return false
	}
	if wanted := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("evidence_type"))); wanted != "" {
		if strings.ToUpper(central10String(row["evidence_type"])) != wanted {
			return false
		}
	}
	if wanted := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("evidence_status"))); wanted != "" {
		if strings.ToUpper(central10String(row["verification_status"])) != wanted {
			return false
		}
	}
	if start := strings.TrimSpace(r.URL.Query().Get("evidence_period_start")); start != "" {
		rowEnd := central10String(row["period_end"])
		if rowEnd == "" {
			rowEnd = central10String(row["period_start"])
		}
		if rowEnd != "" && rowEnd < start {
			return false
		}
	}
	if end := strings.TrimSpace(r.URL.Query().Get("evidence_period_end")); end != "" {
		rowStart := central10String(row["period_start"])
		if rowStart != "" && rowStart > end {
			return false
		}
	}
	return true
}

func (a *app) central10Impact(w http.ResponseWriter, r *http.Request, actor user, cacheKey string) {
	started := time.Now()
	snapshot, updatedAt, ok := a.centralSnapshotForRead(r.Context(), centralStep4ImpactKey)
	if !ok {
		a.readModelInvariantFailure(w, centralStep4ImpactKey)
		return
	}

	definitions := []map[string]any{}
	summary := []map[string]any{}
	evidenceAll := []map[string]any{}
	reports := []map[string]any{}
	analytics := map[string]any{}
	if a.hasPermission(actor, "impact.read") {
		definitions = step4Items(snapshot["definitions"])
		summary = step4Items(snapshot["summary"])
		analytics = step4Map(snapshot["analytics"])
	}
	if a.hasPermission(actor, "evidence.read") {
		evidenceAll = step4Items(snapshot["evidence"])
	}
	if a.hasPermission(actor, "reports.read") {
		reports = step4Items(snapshot["reports"])
	}

	filteredEvidence := make([]map[string]any, 0, len(evidenceAll))
	for _, row := range evidenceAll {
		if central10Step4EvidenceMatches(row, r) {
			filteredEvidence = append(filteredEvidence, row)
		}
	}
	pendingEvidence := 0
	for _, row := range evidenceAll {
		if strings.ToUpper(central10String(row["verification_status"])) == "UNVERIFIED" {
			pendingEvidence++
		}
	}
	readyReports := 0
	for _, row := range reports {
		if strings.ToUpper(central10String(row["status"])) == "READY" {
			readyReports++
		}
	}
	total := len(filteredEvidence)
	limit := central10QueryLimit(r.URL.Query().Get("evidence_limit"), 12, 100)
	offset, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("evidence_offset")))
	if err != nil || offset < 0 {
		offset = 0
	}
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	filteredEvidence = filteredEvidence[offset:end]

	status := "healthy"
	payload := map[string]any{
		"ready": true,
		"access": map[string]any{
			"impact": a.hasPermission(actor, "impact.read"),
			"impact_write": a.hasPermission(actor, "impact.write"),
			"evidence": a.hasPermission(actor, "evidence.read"),
			"evidence_write": a.hasPermission(actor, "evidence.write"),
			"reports": a.hasPermission(actor, "reports.read"),
			"reports_write": a.hasPermission(actor, "reports.write"),
		},
		"definitions": definitions,
		"summary": summary,
		"analytics": analytics,
		"evidence": filteredEvidence,
		"evidence_total": total,
		"reports": reports,
		"kpis": map[string]any{
			"active_metrics": len(definitions),
			"evidence_total": len(evidenceAll),
			"reports_total": len(reports),
			"ready_reports": readyReports,
			"pending_evidence": pendingEvidence,
		},
		"meta": centralStep4Meta(started, centralStep4ImpactKey, updatedAt, status, []string{}),
	}
	w.Header().Set("X-Himate-Cache", "hot-snapshot")
	w.Header().Set("Server-Timing", fmt.Sprintf("central-impact-snapshot;dur=%d", time.Since(started).Milliseconds()))
	common.JSON(w, http.StatusOK, payload)
}

func central10PartnerModuleSection(module map[string]any) (string, string) {
	switch central10String(module["group_key"]) {
	case "finance_invoicing":
		return "FINANCE_INVOICING", "Finance & Invoicing"
	case "marketing":
		return "MARKETING", "Marketing"
	case "website_events":
		return "WEBSITE_EVENTS", "Website & Events"
	default:
		return "TECHNICAL_OPERATION", "Technical Operation"
	}
}

func central10PartnerModuleView(
	modules []map[string]any,
	subscriptions []map[string]any,
	query string,
	state string,
) map[string]any {
	subscriptionByKey := map[string]map[string]any{}
	for _, item := range subscriptions {
		key := central10String(item["module_key"])
		if key != "" { subscriptionByKey[key] = item }
	}

	query = strings.ToLower(strings.TrimSpace(query))
	state = strings.ToUpper(strings.TrimSpace(state))
	switch state {
	case "ACTIVE", "NOT_LICENSED", "MAINTENANCE":
	default:
		state = "ALL"
	}

	kpis := map[string]any{
		"total": len(modules),
		"active": 0,
		"maintenance": 0,
		"base_included": 0,
	}
	activeKeys := []string{}
	enriched := make([]map[string]any, 0, len(modules))
	filtered := make([]map[string]any, 0, len(modules))
	groupMap := map[string][]map[string]any{
		"FINANCE_INVOICING": {},
		"TECHNICAL_OPERATION": {},
		"MARKETING": {},
		"WEBSITE_EVENTS": {},
	}

	for _, raw := range modules {
		row := central10CopyMap(raw)
		moduleKey := central10String(row["key"])
		if subscription := subscriptionByKey[moduleKey]; subscription != nil {
			row["subscription"] = subscription
		}
		moduleState := strings.ToUpper(central10String(row["status"]))
		switch moduleState {
		case "ACTIVE":
			kpis["active"] = central10Int(kpis["active"]) + 1
			activeKeys = append(activeKeys, moduleKey)
		case "MAINTENANCE":
			kpis["maintenance"] = central10Int(kpis["maintenance"]) + 1
		}
		if row["included_in_base"] == true {
			kpis["base_included"] = central10Int(kpis["base_included"]) + 1
		}
		enriched = append(enriched, row)

		if state != "ALL" && moduleState != state {
			continue
		}
		if query != "" {
			haystack := strings.ToLower(strings.Join([]string{
				central10String(row["label"]),
				central10String(row["key"]),
				central10String(row["group_label"]),
			}, " "))
			if !strings.Contains(haystack, query) {
				continue
			}
		}
		filtered = append(filtered, row)
		sectionKey, _ := central10PartnerModuleSection(row)
		groupMap[sectionKey] = append(groupMap[sectionKey], row)
	}

	groupOrder := []string{"FINANCE_INVOICING", "TECHNICAL_OPERATION", "MARKETING", "WEBSITE_EVENTS"}
	groups := make([]map[string]any, 0, len(groupOrder))
	for _, key := range groupOrder {
		label := key
		if rows := groupMap[key]; len(rows) > 0 {
			_, label = central10PartnerModuleSection(rows[0])
		} else {
			switch key {
			case "FINANCE_INVOICING": label = "Finance & Invoicing"
			case "TECHNICAL_OPERATION": label = "Technical Operation"
			case "MARKETING": label = "Marketing"
			case "WEBSITE_EVENTS": label = "Website & Events"
			}
		}
		groups = append(groups, map[string]any{
			"key": key,
			"label": label,
			"items": groupMap[key],
			"count": len(groupMap[key]),
		})
	}

	return map[string]any{
		"items": enriched,
		"filtered_items": filtered,
		"filtered_count": len(filtered),
		"groups": groups,
		"kpis": kpis,
		"active_module_keys": activeKeys,
		"query": query,
		"state": state,
	}
}

func central10ProductionEnvironment(environments []map[string]any) map[string]any {
	for _, environment := range environments {
		if strings.ToUpper(central10String(environment["kind"])) == "PRODUCTION" {
			return environment
		}
	}
	return nil
}

func (a *app) central10PartnerModules(w http.ResponseWriter, r *http.Request, actor user, cacheKey string) {
	started := time.Now()
	if !a.hasPermission(actor, "catalog.read") {
		common.APIError(w, http.StatusForbidden, "FORBIDDEN", "Catalog read permission required")
		return
	}
	raw := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/central/partners/"), "/")
	parts := strings.Split(raw, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] != "modules" {
		common.APIError(w, http.StatusNotFound, "PARTNER_MODULE_VIEW_NOT_FOUND", "Partner module read model not found")
		return
	}
	partnerID := parts[0]
	key := centralPartnerWorkspaceKey(partnerID)
	snapshot, updatedAt, ok := a.partnerWorkspaceForRead(r.Context(), partnerID)
	if !ok {
		a.readModelInvariantFailure(w, key)
		return
	}

	modules := step4Items(snapshot["modules"])
	subscriptions := []map[string]any{}
	if a.hasPermission(actor, "billing.read") {
		subscriptions = step4Items(snapshot["subscriptions"])
	}
	view := central10PartnerModuleView(modules, subscriptions, r.URL.Query().Get("q"), r.URL.Query().Get("state"))
	view["ready"] = true
	view["meta"] = centralStep4Meta(started, key, updatedAt, "healthy", []string{})
	w.Header().Set("X-Himate-Cache", "hot-snapshot")
	w.Header().Set("Server-Timing", fmt.Sprintf("central-partner-modules-snapshot;dur=%d", time.Since(started).Milliseconds()))
	common.JSON(w, http.StatusOK, view)
}

func (a *app) central10PartnerWorkspace(w http.ResponseWriter, r *http.Request, actor user, cacheKey string) {
	started := time.Now()
	raw := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/central/partners/"), "/")
	if raw == "" || strings.Contains(raw, "/") {
		common.APIError(w, http.StatusNotFound, "PARTNER_NOT_FOUND", "Partner workspace not found")
		return
	}
	partnerID := raw
	key := centralPartnerWorkspaceKey(partnerID)
	snapshot, updatedAt, ok := a.partnerWorkspaceForRead(r.Context(), partnerID)
	if !ok {
		a.readModelInvariantFailure(w, key)
		return
	}

	partner := step4Map(snapshot["partner"])
	modules := step4Items(snapshot["modules"])
	moduleView := step4Map(snapshot["module_view"])
	billing := step4Map(snapshot["billing"])
	terms := step4Map(snapshot["terms"])
	license := step4Map(snapshot["license"])
	documents := step4Items(snapshot["documents"])
	invoices := step4Items(snapshot["invoices"])
	subscriptions := step4Items(snapshot["subscriptions"])
	environments := step4Items(snapshot["environments"])
	provisioningJobs := step4Items(snapshot["provisioning_jobs"])
	impactSummary := step4Items(snapshot["impact_summary"])
	evidence := step4Items(snapshot["evidence"])
	connectorCredentials := step4Items(snapshot["connector_credentials"])
	portalUsers := step4Items(snapshot["portal_users"])
	agreement := step4Map(snapshot["agreement"])
	commercialStatus := step4Map(snapshot["commercial_status"])
	billingEvents := step4Items(snapshot["billing_events"])
	websiteAdapter := step4Map(snapshot["website_adapter"])
	partnerDesign := step4Map(snapshot["partner_design"])
	paymentProfile := step4Map(snapshot["payment_profile"])
	productionEnvironment := snapshot["production_environment"]

	if !a.hasPermission(actor, "catalog.read") {
		modules = []map[string]any{}
		moduleView = central10PartnerModuleView(nil, nil, "", "ALL")
	}
	if !a.hasPermission(actor, "billing.read") {
		billing = map[string]any{}
		terms = map[string]any{}
		license = map[string]any{}
		documents = []map[string]any{}
		invoices = []map[string]any{}
		subscriptions = []map[string]any{}
		agreement = map[string]any{}
		commercialStatus = map[string]any{}
		billingEvents = []map[string]any{}
		paymentProfile = map[string]any{}
	}
	if !a.hasPermission(actor, "environments.read") {
		environments = []map[string]any{}
		productionEnvironment = nil
	}
	if !a.hasPermission(actor, "provisioning.read") {
		provisioningJobs = []map[string]any{}
	}
	if !a.hasPermission(actor, "impact.read") {
		impactSummary = []map[string]any{}
	}
	if !a.hasPermission(actor, "evidence.read") {
		evidence = []map[string]any{}
	}
	if !a.hasPermission(actor, "connectors.read") {
		connectorCredentials = []map[string]any{}
		websiteAdapter = map[string]any{}
	}
	if !a.hasPermission(actor, "cms.read") {
		partnerDesign = map[string]any{}
	}
	if !actor.SystemOwner || !a.hasPermission(actor, "administration.read") {
		portalUsers = []map[string]any{}
	}

	payload := map[string]any{
		"ready":                           true,
		"partner":                         partner,
		"modules":                         modules,
		"module_view":                     moduleView,
		"production_environment":          productionEnvironment,
		"preferred_connector_environment": snapshot["preferred_connector_environment"],
		"billing":                         billing,
		"terms":                           terms,
		"license":                         license,
		"documents":                       documents,
		"invoices":                        invoices,
		"subscriptions":                   subscriptions,
		"environments":                    environments,
		"provisioning_jobs":               provisioningJobs,
		"impact_summary":                  impactSummary,
		"evidence":                        evidence,
		"connector_credentials":           connectorCredentials,
		"portal_users":                    portalUsers,
		"agreement":                       agreement,
		"commercial_status":               commercialStatus,
		"billing_events":                  billingEvents,
		"website_adapter":                 websiteAdapter,
		"partner_design":                  partnerDesign,
		"payment_profile":                 paymentProfile,
		"meta":                            centralStep4Meta(started, key, updatedAt, "healthy", []string{}),
	}
	w.Header().Set("X-Himate-Cache", "hot-snapshot")
	w.Header().Set("Server-Timing", fmt.Sprintf("central-partner-workspace-snapshot;dur=%d", time.Since(started).Milliseconds()))
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
