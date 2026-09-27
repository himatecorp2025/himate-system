package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"himate.local/services/internal/common"
)

const centralNotificationRefreshInterval = 15 * time.Second

func centralUserReadModelMigration() common.Migration {
	return common.Migration{
		Version: 20,
		Name:    "central-21-user-read-models",
		Statements: []string{
			`CREATE TABLE IF NOT EXISTS identity.central_user_read_models(
				user_id TEXT PRIMARY KEY REFERENCES identity.users(id) ON DELETE CASCADE,
				notifications JSONB NOT NULL,
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE INDEX IF NOT EXISTS identity_central_user_read_models_updated_idx
			  ON identity.central_user_read_models(updated_at DESC)`,
		},
	}
}

func centralNotificationSnapshotValid(payload map[string]any) bool {
	if payload == nil {
		return false
	}
	if strings.ToUpper(central10String(payload["delivery_scope"])) != "PLATFORM" {
		return false
	}
	if _, ok := payload["items"]; !ok {
		return false
	}
	if _, ok := payload["unread_count"]; !ok {
		return false
	}
	return true
}

func (a *app) materializeCentralUserNotifications(ctx context.Context, userID string, roles []string) (map[string]any, error) {
	var feed map[string]any
	headers := map[string]string{
		"X-Himate-User-ID":            userID,
		"X-Himate-Permissions":        strings.Join(a.permissionsForRoles(roles), ","),
		"X-Himate-Notification-Scope": "PLATFORM",
	}
	if err := a.internalGETWithHeaders(
		ctx,
		a.hosts["notifications"],
		"/api/v1/notifications?limit=100",
		headers,
		&feed,
	); err != nil {
		return nil, err
	}
	if !centralNotificationSnapshotValid(feed) {
		return nil, fmt.Errorf("notification feed failed projection validation")
	}
	return feed, nil
}

func (a *app) persistCentralUserNotifications(ctx context.Context, userID string, feed map[string]any) bool {
	if strings.TrimSpace(userID) == "" || !centralNotificationSnapshotValid(feed) {
		return false
	}
	raw, err := json.Marshal(feed)
	if err != nil {
		if a.log != nil {
			a.log.Error("Central notification projection marshal failed", "user_id", userID, "error", err)
		}
		return false
	}
	if _, err := a.db.ExecContext(ctx,
		`INSERT INTO identity.central_user_read_models(user_id,notifications,updated_at)
		 VALUES($1,$2::jsonb,NOW())
		 ON CONFLICT(user_id) DO UPDATE
		 SET notifications=EXCLUDED.notifications,updated_at=EXCLUDED.updated_at`,
		userID, string(raw),
	); err != nil {
		if a.log != nil {
			a.log.Error("Central notification projection persistence failed", "user_id", userID, "error", err)
		}
		return false
	}
	return true
}

func (a *app) refreshCentralUserNotifications(ctx context.Context, userID string, roles []string) bool {
	feed, err := a.materializeCentralUserNotifications(ctx, userID, roles)
	if err != nil {
		if a.log != nil {
			a.log.Warn("Central notification refresh failed; retaining LKG", "user_id", userID, "error", err)
		}
		return false
	}
	return a.persistCentralUserNotifications(ctx, userID, feed)
}

func (a *app) centralUsersForReadModels(ctx context.Context) ([]struct {
	ID    string
	Roles []string
}, error) {
	rows, err := a.db.QueryContext(ctx,
		`SELECT id,roles FROM identity.users WHERE active=TRUE ORDER BY id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := []struct {
		ID    string
		Roles []string
	}{}
	for rows.Next() {
		var id string
		var raw []byte
		if err := rows.Scan(&id, &raw); err != nil {
			return nil, err
		}
		roles := []string{}
		_ = json.Unmarshal(raw, &roles)
		users = append(users, struct {
			ID    string
			Roles []string
		}{ID: id, Roles: roles})
	}
	return users, rows.Err()
}

func (a *app) refreshCentralUserNotificationSnapshots(ctx context.Context) bool {
	users, err := a.centralUsersForReadModels(ctx)
	if err != nil {
		if a.log != nil {
			a.log.Error("Central notification user scan failed", "error", err)
		}
		return false
	}
	ok := true
	var wg sync.WaitGroup
	var mu sync.Mutex
	sem := make(chan struct{}, 6)
	for _, raw := range users {
		u := raw
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				mu.Lock()
				ok = false
				mu.Unlock()
				return
			}
			if !a.refreshCentralUserNotifications(ctx, u.ID, u.Roles) {
				mu.Lock()
				ok = false
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return ok
}

func (a *app) loadCentralUserNotifications(ctx context.Context, userID string) (map[string]any, time.Time, error) {
	started := time.Now()
	var raw []byte
	var updated time.Time
	if err := a.db.QueryRowContext(ctx,
		`SELECT notifications,updated_at FROM identity.central_user_read_models WHERE user_id=$1`,
		userID,
	).Scan(&raw, &updated); err != nil {
		return nil, time.Time{}, err
	}
	var feed map[string]any
	if err := json.Unmarshal(raw, &feed); err != nil {
		return nil, time.Time{}, err
	}
	if !centralNotificationSnapshotValid(feed) {
		return nil, time.Time{}, fmt.Errorf("Central notification projection is invalid")
	}
	if elapsed := time.Since(started); elapsed > readModelTargetLatency && a.log != nil {
		a.log.Warn("Central notification materialized read exceeded target", "user_id", userID, "duration_ms", elapsed.Milliseconds())
	}
	return feed, updated.UTC(), nil
}

func (a *app) ensureCentralUserNotificationReadModelsReady(ctx context.Context) error {
	users, err := a.centralUsersForReadModels(ctx)
	if err != nil {
		return err
	}
	missing := []string{}
	for _, u := range users {
		if _, _, err := a.loadCentralUserNotifications(ctx, u.ID); err == nil {
			continue
		}
		refreshCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
		ok := a.refreshCentralUserNotifications(refreshCtx, u.ID, u.Roles)
		cancel()
		if !ok {
			missing = append(missing, u.ID)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("Central user notification projections not ready: %s", strings.Join(missing, ","))
	}
	return nil
}

func centralNotificationFeedForRequest(feed map[string]any, r *http.Request) map[string]any {
	unreadOnly := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("unread_only")), "true")
	limit := queryInt(r.URL.Query().Get("limit"), 40, 100)
	items := []map[string]any{}
	unread := 0
	for _, raw := range anyItems(feed["items"]) {
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
		"items": items,
		"count": len(items),
		"unread_count": unread,
		"delivery_scope": "PLATFORM",
	}
}

func (a *app) serveCentralNotificationGET(w http.ResponseWriter, r *http.Request, actor user) bool {
	if r.Method != http.MethodGet || r.URL.Path != "/api/v1/notifications" {
		return false
	}
	feed, _, err := a.loadCentralUserNotifications(r.Context(), actor.ID)
	if err != nil {
		common.APIError(w, http.StatusServiceUnavailable, "READ_MODEL_NOT_READY", "Notification read model is not ready")
		return true
	}
	w.Header().Set("X-Himate-Cache", "persistent-user-read-model")
	common.JSON(w, http.StatusOK, centralNotificationFeedForRequest(feed, r))
	return true
}

func (a *app) runCentralUserNotificationMaterializer() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	a.refreshCentralUserNotificationSnapshots(ctx)
	cancel()
	ticker := time.NewTicker(centralNotificationRefreshInterval)
	defer ticker.Stop()
	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		a.refreshCentralUserNotificationSnapshots(ctx)
		cancel()
	}
}
