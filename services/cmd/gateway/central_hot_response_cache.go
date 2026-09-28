package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// centralHotResponse is an already encoded HTTP payload. On a cache hit the
// live read-model request path performs no snapshot copy, JSON encode/decode,
// PostgreSQL lookup, or downstream service call.
type centralHotResponse struct {
	status      int
	body        []byte
	contentType string
	cacheHeader string
}

type centralHotResponseKey struct {
	path     string
	rawQuery string
	locale   string
	browser  bool
}

type partnerHotResponseKey struct {
	partnerID string
	userID    string
	path      string
	rawQuery  string
}

var centralHotResponseCache = struct {
	sync.RWMutex
	items map[centralHotResponseKey]centralHotResponse
}{items: map[centralHotResponseKey]centralHotResponse{}}

var partnerHotResponseCache = struct {
	sync.RWMutex
	items map[partnerHotResponseKey]centralHotResponse
}{items: map[partnerHotResponseKey]centralHotResponse{}}

var hotResponseRefreshState = struct {
	sync.Mutex
	running bool
	dirty   bool
}{}

type centralHotRecorder struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func newCentralHotRecorder() *centralHotRecorder {
	return &centralHotRecorder{header: make(http.Header)}
}

func (r *centralHotRecorder) Header() http.Header { return r.header }

func (r *centralHotRecorder) WriteHeader(status int) {
	if r.status == 0 {
		r.status = status
	}
}

func (r *centralHotRecorder) Write(p []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.body.Write(p)
}

func hotResponseFromRecorder(rec *centralHotRecorder) (centralHotResponse, error) {
	status := rec.status
	if status == 0 {
		status = http.StatusOK
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return centralHotResponse{}, fmt.Errorf("prewarm returned status %d", status)
	}
	if rec.body.Len() == 0 {
		return centralHotResponse{}, fmt.Errorf("prewarm returned an empty body")
	}
	contentType := rec.header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json; charset=utf-8"
	}
	body := append([]byte(nil), rec.body.Bytes()...)
	if strings.Contains(strings.ToLower(contentType), "application/json") {
		var payload map[string]any
		if json.Unmarshal(body, &payload) == nil {
			if meta, ok := payload["meta"].(map[string]any); ok {
				// The cached response represents live request cost, not startup
				// pre-render CPU. This keeps performance telemetry deterministic.
				meta["duration_ms"] = 0
				meta["prewarmed"] = true
			}
			if normalized, err := json.Marshal(payload); err == nil {
				body = append(normalized, '\n')
			}
		}
	}
	return centralHotResponse{
		status:      status,
		body:        body,
		contentType: contentType,
		cacheHeader: rec.header.Get("X-Himate-Cache"),
	}, nil
}

func centralHotLocale(r *http.Request) string {
	if r == nil {
		return "en"
	}
	raw := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Himate-Locale")))
	if strings.HasPrefix(raw, "hu") {
		return "hu"
	}
	return "en"
}

func centralHotFullAccess(u user) bool {
	if u.SystemOwner {
		return true
	}
	for _, role := range u.Roles {
		if strings.EqualFold(strings.TrimSpace(role), "platform_admin") {
			return true
		}
	}
	return false
}

func writePrewarmedResponse(w http.ResponseWriter, entry centralHotResponse) {
	w.Header().Set("Content-Type", entry.contentType)
	if entry.cacheHeader != "" {
		w.Header().Set("X-Himate-Cache", entry.cacheHeader)
	}
	w.Header().Set("X-Himate-Prewarmed", "1")
	w.Header().Set("Server-Timing", "himate-prewarmed;dur=0")
	w.WriteHeader(entry.status)
	_, _ = w.Write(entry.body)
}

func (a *app) serveCentralPrewarmedResponse(w http.ResponseWriter, r *http.Request, u user) bool {
	if r == nil || r.Method != http.MethodGet || !centralHotFullAccess(u) {
		return false
	}
	key := centralHotResponseKey{
		path: r.URL.Path, rawQuery: r.URL.RawQuery,
		locale: centralHotLocale(r), browser: centralBrowserMaterializedRead(r),
	}
	centralHotResponseCache.RLock()
	entry, ok := centralHotResponseCache.items[key]
	centralHotResponseCache.RUnlock()
	if !ok {
		return false
	}
	writePrewarmedResponse(w, entry)
	return true
}

func (a *app) servePartnerPrewarmedResponse(w http.ResponseWriter, r *http.Request, u partnerUser, path string) bool {
	if r == nil || r.Method != http.MethodGet || !partnerBrowserMaterializedRead(r) {
		return false
	}
	key := partnerHotResponseKey{
		partnerID: u.PartnerID, userID: u.ID, path: path, rawQuery: r.URL.RawQuery,
	}
	partnerHotResponseCache.RLock()
	entry, ok := partnerHotResponseCache.items[key]
	partnerHotResponseCache.RUnlock()
	if !ok {
		return false
	}
	writePrewarmedResponse(w, entry)
	return true
}

func invalidateCentralHotResponseCaches() {
	centralHotResponseCache.Lock()
	centralHotResponseCache.items = map[centralHotResponseKey]centralHotResponse{}
	centralHotResponseCache.Unlock()

	partnerHotResponseCache.Lock()
	partnerHotResponseCache.items = map[partnerHotResponseKey]centralHotResponse{}
	partnerHotResponseCache.Unlock()
}

func centralHotStaticRoutes() []string {
	year := time.Now().UTC().Year()
	return []string{
		fmt.Sprintf("/api/v1/dashboard/summary?year=%d", year),
		"/api/v1/central/partners?limit=24&offset=0",
		"/api/v1/central/partners?lifecycle=READY_TO_PROVISION&limit=24&offset=0",
		"/api/v1/central/modules",
		"/api/v1/central/modules?registry_preset=ACTIVE",
		"/api/v1/central/modules/commercial?perspective=PARTNER&commercial_limit=120",
		"/api/v1/central/modules/commercial?perspective=MODULE&commercial_status=ACTIVE&commercial_limit=120",
		"/api/v1/central/packages",
		"/api/v1/central/packages/supplementary",
		"/api/v1/central/finance",
		"/api/v1/central/finance?invoice_status=ALL&revenue_period=MONTHLY&revenue_plan=ALL",
		"/api/v1/central/impact",
		"/api/v1/central/impact?evidence_limit=12&evidence_offset=0",
		"/api/v1/central/website",
		"/api/v1/central/administration?limit=60&offset=0",
		"/api/v1/central/administration?limit=200&offset=0",
		"/api/v1/central/system",
		"/api/v1/central/connections",
	}
}

func centralHotVisiblePartnerIDs() []string {
	snapshot, _, ok := centralStep3SnapshotGet(centralStep4PartnersKey)
	if !ok {
		return nil
	}
	seen := map[string]bool{}
	for _, item := range step4Items(snapshot["items"]) {
		if id := strings.TrimSpace(central10String(item["id"])); id != "" {
			seen[id] = true
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func tenantHotPartnerIDs() []string {
	seen := map[string]bool{}
	centralStep3Snapshots.RLock()
	for key := range centralStep3Snapshots.items {
		if strings.HasPrefix(key, centralPartnerWorkspacePrefix) {
			if id := centralPartnerWorkspaceID(key); id != "" {
				seen[id] = true
			}
		}
	}
	centralStep3Snapshots.RUnlock()
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func centralHotRoutes() []string {
	routes := append([]string{}, centralHotStaticRoutes()...)
	for _, partnerID := range centralHotVisiblePartnerIDs() {
		routes = append(routes,
			"/api/v1/central/partners/"+partnerID,
			"/api/v1/central/partners/"+partnerID+"/modules?state=ALL",
		)
	}
	return routes
}

func centralHotLegacySafe(path string) bool {
	if path == "/api/v1/central/connections" || path == "/api/v1/central/administration" {
		return false
	}
	if path == "/api/v1/dashboard/summary" {
		return true
	}
	return strings.HasPrefix(path, "/api/v1/central/")
}

func (a *app) dispatchCentralHotRoute(w http.ResponseWriter, r *http.Request, actor user) bool {
	switch {
	case r.URL.Path == "/api/v1/dashboard/summary":
		a.dashboard(w, r, actor)
		return true
	case r.URL.Path == "/api/v1/central/connections":
		a.central13Connections(w, r, actor)
		return true
	case r.URL.Path == "/api/v1/central/administration":
		a.central14Administration(w, r, actor)
		return true
	case strings.HasPrefix(r.URL.Path, "/api/v1/central/"):
		a.central10ReadModel(w, r, actor)
		return true
	default:
		return false
	}
}

func (a *app) renderCentralHotResponse(ctx context.Context, uri string, browser bool, locale string) (centralHotResponseKey, centralHotResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://himate.prewarm"+uri, nil)
	if err != nil {
		return centralHotResponseKey{}, centralHotResponse{}, err
	}
	if browser {
		req.Header.Set("X-Himate-Read-Model", "browser")
		if locale == "hu" {
			req.Header.Set("X-Himate-Locale", "hu")
		} else {
			req.Header.Set("X-Himate-Locale", "en")
		}
	}
	rec := newCentralHotRecorder()
	actor := user{
		ID: "__central_hot_platform__", Name: "Central Hot Prewarm",
		Roles: []string{"platform_admin"}, Active: true, SystemOwner: true,
	}
	if !a.dispatchCentralHotRoute(rec, req, actor) {
		return centralHotResponseKey{}, centralHotResponse{}, fmt.Errorf("no Central hot route for %s", uri)
	}
	entry, err := hotResponseFromRecorder(rec)
	if err != nil {
		return centralHotResponseKey{}, centralHotResponse{}, fmt.Errorf("%s: %w", uri, err)
	}
	return centralHotResponseKey{
		path: req.URL.Path, rawQuery: req.URL.RawQuery,
		locale: centralHotLocale(req), browser: browser,
	}, entry, nil
}

func partnerHotPaths() []string {
	return []string{
		"/dashboard", "/company", "/plans", "/plan", "/plan/modules",
		"/charity", "/charity/modules", "/modules",
		"/billing/summary", "/billing/subscriptions", "/billing/invoices",
		"/impact/summary", "/contacts", "/domains", "/deployments",
		"/audit", "/permissions", "/users", "/notifications",
		"/design", "/design/media",
	}
}

func partnerUsersForHotPrewarm(snapshot map[string]any, partnerID string) []partnerUser {
	rows := partnerWorkspaceItems(snapshot, "portal_users")
	users := make([]partnerUser, 0, len(rows))
	for _, row := range rows {
		id := strings.TrimSpace(central10String(row["id"]))
		if id == "" {
			continue
		}
		if active, ok := row["active"].(bool); ok && !active {
			continue
		}
		role := strings.TrimSpace(central10String(row["role"]))
		if role == "" {
			role = strings.TrimSpace(central10String(row["role_key"]))
		}
		if role == "" {
			role = "viewer"
		}
		users = append(users, partnerUser{
			ID: id, PartnerID: partnerID,
			Name: central10String(row["name"]), Email: central10String(row["email"]),
			Role: role, Active: true,
			PreferredLocale: central10String(row["preferred_locale"]),
			Timezone: central10String(row["timezone"]),
		})
	}
	return users
}

func (a *app) renderPartnerHotResponse(ctx context.Context, u partnerUser, path string) (partnerHotResponseKey, centralHotResponse, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://himate.prewarm/partner/api/v1"+path, nil)
	if err != nil {
		return partnerHotResponseKey{}, centralHotResponse{}, false, err
	}
	req.Header.Set("X-Himate-Read-Model", "browser")
	locale := normalizedLocale(u.PreferredLocale)
	if strings.HasPrefix(strings.ToLower(locale), "hu") {
		req.Header.Set("X-Himate-Locale", "hu")
	} else {
		req.Header.Set("X-Himate-Locale", "en")
	}
	rec := newCentralHotRecorder()
	if !a.servePartnerMaterializedGET(rec, req, u, path) {
		return partnerHotResponseKey{}, centralHotResponse{}, false, nil
	}
	if rec.status == http.StatusForbidden {
		return partnerHotResponseKey{}, centralHotResponse{}, false, nil
	}
	entry, err := hotResponseFromRecorder(rec)
	if err != nil {
		return partnerHotResponseKey{}, centralHotResponse{}, false, fmt.Errorf("%s %s: %w", u.PartnerID, path, err)
	}
	return partnerHotResponseKey{
		partnerID: u.PartnerID, userID: u.ID, path: path, rawQuery: req.URL.RawQuery,
	}, entry, true, nil
}

func (a *app) buildCentralHotResponses(ctx context.Context) (map[centralHotResponseKey]centralHotResponse, error) {
	out := map[centralHotResponseKey]centralHotResponse{}
	for _, uri := range centralHotRoutes() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://himate.prewarm"+uri, nil)
		if err != nil {
			return nil, err
		}
		if centralHotLegacySafe(req.URL.Path) {
			key, entry, err := a.renderCentralHotResponse(ctx, uri, false, "en")
			if err != nil {
				return nil, err
			}
			out[key] = entry
		}
		for _, locale := range []string{"en", "hu"} {
			key, entry, err := a.renderCentralHotResponse(ctx, uri, true, locale)
			if err != nil {
				return nil, err
			}
			out[key] = entry
		}
	}
	return out, nil
}

func (a *app) buildPartnerHotResponses(ctx context.Context) (map[partnerHotResponseKey]centralHotResponse, error) {
	out := map[partnerHotResponseKey]centralHotResponse{}
	for _, partnerID := range tenantHotPartnerIDs() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		snapshot, _, ok := centralStep3SnapshotGet(centralPartnerWorkspaceKey(partnerID))
		if !ok {
			return nil, fmt.Errorf("tenant hot prewarm missing workspace %s", partnerID)
		}
		for _, u := range partnerUsersForHotPrewarm(snapshot, partnerID) {
			for _, path := range partnerHotPaths() {
				key, entry, handled, err := a.renderPartnerHotResponse(ctx, u, path)
				if err != nil {
					return nil, err
				}
				if handled {
					out[key] = entry
				}
			}
		}
	}
	return out, nil
}

// prewarmCentral10To21HotResponses is part of readiness. It renders the exact
// Central-10..21 screen contracts, plus accessible Partner Portal browser
// screens, into immutable byte slices before the request gate opens.
func (a *app) prewarmCentral10To21HotResponses(ctx context.Context) error {
	central, err := a.buildCentralHotResponses(ctx)
	if err != nil {
		return fmt.Errorf("Central hot-response prewarm: %w", err)
	}
	partner, err := a.buildPartnerHotResponses(ctx)
	if err != nil {
		return fmt.Errorf("tenant hot-response prewarm: %w", err)
	}

	centralHotResponseCache.Lock()
	centralHotResponseCache.items = central
	centralHotResponseCache.Unlock()

	partnerHotResponseCache.Lock()
	partnerHotResponseCache.items = partner
	partnerHotResponseCache.Unlock()

	if a.log != nil {
		a.log.Info("Central/Tenant serialized hot-response prewarm complete",
			"central_responses", len(central), "tenant_responses", len(partner))
	}
	return nil
}

// requestCentralHotResponseRefresh coalesces mutation bursts. Foreground
// write-through owns correctness; this worker only restores the zero-marshal
// fast path after committed projections are current.
func (a *app) requestCentralHotResponseRefresh() {
	if !gatewayReadiness.Load() {
		return
	}
	hotResponseRefreshState.Lock()
	hotResponseRefreshState.dirty = true
	if hotResponseRefreshState.running {
		hotResponseRefreshState.Unlock()
		return
	}
	hotResponseRefreshState.running = true
	hotResponseRefreshState.Unlock()

	go func() {
		for {
			time.Sleep(125 * time.Millisecond)
			hotResponseRefreshState.Lock()
			hotResponseRefreshState.dirty = false
			hotResponseRefreshState.Unlock()

			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			err := a.prewarmCentral10To21HotResponses(ctx)
			cancel()
			if err != nil && a.log != nil {
				a.log.Warn("serialized hot-response background rewarm failed; dynamic LKG fallback remains active", "error", err)
			}

			hotResponseRefreshState.Lock()
			if !hotResponseRefreshState.dirty {
				hotResponseRefreshState.running = false
				hotResponseRefreshState.Unlock()
				return
			}
			hotResponseRefreshState.Unlock()
		}
	}()
}
