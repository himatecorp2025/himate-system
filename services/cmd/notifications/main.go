package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"himate.local/services/internal/common"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type app struct{ db *sql.DB }

type notificationEvent struct {
	ID                 int64
	EventType          string
	Severity           string
	Title              string
	Message            string
	Resource           string
	PartnerID          string
	DeepLink           string
	AudiencePermission string
	DeliveryScope      string
	TargetUserID       string
	ModuleKey          string
	Category           string
	Metadata           map[string]any
	CreatedAt          time.Time
	Read               bool
}

var notificationCategories = map[string]bool{
	"WORKFLOW": true,
	"COMMENT":  true,
	"DEADLINE": true,
	"CALENDAR": true,
	"BILLING":  true,
	"SECURITY": true,
	"SYSTEM":   true,
}

func main() {
	log := common.Logger()
	db, err := common.OpenDB()
	if err != nil {
		log.Error("database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	a := &app{db: db}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := a.migrate(ctx); err != nil {
		log.Error("migration", "error", err)
		os.Exit(1)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		common.JSON(w, http.StatusOK, map[string]any{"status": "ok", "service": "notifications", "time": time.Now().UTC()})
	})
	mux.HandleFunc("/api/v1/notifications", a.feed)
	mux.HandleFunc("/api/v1/notifications/", a.notificationAction)
	mux.HandleFunc("/internal/v1/notifications/events", a.createEvent)
	common.Run(log, "notifications", common.Env("PORT", "10000"), common.InternalAuth(os.Getenv("HIMATE_INTERNAL_TOKEN"), mux))
}

func (a *app) migrate(ctx context.Context) error {
	return common.ApplyMigrations(ctx, a.db, "notifications", []common.Migration{
		{
			Version: 1,
			Name:    "notification-center",
			Statements: []string{
				`CREATE SCHEMA IF NOT EXISTS notifications`,
				`CREATE TABLE IF NOT EXISTS notifications.events(
					id BIGSERIAL PRIMARY KEY,
					event_type TEXT NOT NULL,
					severity TEXT NOT NULL DEFAULT 'INFO',
					title TEXT NOT NULL,
					message TEXT NOT NULL DEFAULT '',
					resource TEXT NOT NULL DEFAULT '',
					partner_id TEXT NOT NULL DEFAULT '',
					deep_link TEXT NOT NULL DEFAULT '',
					audience_permission TEXT NOT NULL DEFAULT '',
					metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
					created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
				)`,
				`CREATE INDEX IF NOT EXISTS notifications_events_created_idx ON notifications.events(created_at DESC,id DESC)`,
				`CREATE INDEX IF NOT EXISTS notifications_events_audience_idx ON notifications.events(audience_permission,created_at DESC)`,
				`CREATE TABLE IF NOT EXISTS notifications.read_state(
					event_id BIGINT NOT NULL REFERENCES notifications.events(id) ON DELETE CASCADE,
					user_id TEXT NOT NULL,
					read_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
					PRIMARY KEY(event_id,user_id)
				)`,
				`CREATE INDEX IF NOT EXISTS notification_read_user_idx ON notifications.read_state(user_id,read_at DESC)`,
			},
		},
		{
			Version: 2,
			Name:    "start-23-11-6-central-delivery-engine",
			Statements: []string{
				`ALTER TABLE notifications.events ADD COLUMN IF NOT EXISTS delivery_scope TEXT NOT NULL DEFAULT 'PLATFORM'`,
				`ALTER TABLE notifications.events ADD COLUMN IF NOT EXISTS target_user_id TEXT NOT NULL DEFAULT ''`,
				`ALTER TABLE notifications.events ADD COLUMN IF NOT EXISTS module_key TEXT NOT NULL DEFAULT ''`,
				`ALTER TABLE notifications.events ADD COLUMN IF NOT EXISTS category TEXT NOT NULL DEFAULT 'SYSTEM'`,
				`CREATE INDEX IF NOT EXISTS notifications_partner_delivery_idx ON notifications.events(delivery_scope,partner_id,created_at DESC,id DESC)`,
				`CREATE INDEX IF NOT EXISTS notifications_target_user_idx ON notifications.events(target_user_id,created_at DESC,id DESC) WHERE target_user_id<>''`,
				`CREATE INDEX IF NOT EXISTS notifications_module_delivery_idx ON notifications.events(module_key,created_at DESC,id DESC) WHERE module_key<>''`,
			},
		},
	})
}

func csvSet(header string) map[string]bool {
	out := map[string]bool{}
	for _, raw := range strings.Split(header, ",") {
		value := strings.TrimSpace(raw)
		if value != "" {
			out[value] = true
		}
	}
	return out
}

func permissionVisible(permission string, set map[string]bool) bool {
	permission = strings.TrimSpace(permission)
	return permission == "" || set["*"] || set[permission]
}

func normalizeScope(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "PARTNER" {
		return "PARTNER"
	}
	return "PLATFORM"
}

func notificationCategory(eventType, resource, explicit string) string {
	explicit = strings.ToUpper(strings.TrimSpace(explicit))
	if notificationCategories[explicit] {
		return explicit
	}
	eventType = strings.ToUpper(strings.TrimSpace(eventType))
	resource = strings.ToUpper(strings.TrimSpace(resource))
	switch {
	case strings.Contains(eventType, "COMMENT"):
		return "COMMENT"
	case strings.Contains(eventType, "DEADLINE") || strings.Contains(eventType, "DUE_"):
		return "DEADLINE"
	case strings.Contains(eventType, "CALENDAR") || strings.Contains(eventType, "EVENT_"):
		return "CALENDAR"
	case strings.Contains(eventType, "WORKFLOW") || strings.Contains(eventType, "TASK_"):
		return "WORKFLOW"
	case strings.Contains(eventType, "LOGIN") || strings.Contains(eventType, "SECURITY") || resource == "SECURITY":
		return "SECURITY"
	case strings.Contains(eventType, "PAYMENT") || strings.Contains(eventType, "BILLING") || strings.Contains(eventType, "SUSPEND") || resource == "BILLING":
		return "BILLING"
	default:
		return "SYSTEM"
	}
}

func limitValue(raw string) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n < 1 {
		return 40
	}
	if n > 100 {
		return 100
	}
	return n
}

func eventVisible(event notificationEvent, scope, partnerID, userID string, permissions, modules map[string]bool) bool {
	if normalizeScope(event.DeliveryScope) != normalizeScope(scope) {
		return false
	}
	if normalizeScope(scope) == "PARTNER" {
		if strings.TrimSpace(partnerID) == "" || strings.TrimSpace(event.PartnerID) != strings.TrimSpace(partnerID) {
			return false
		}
	}
	if event.TargetUserID != "" && event.TargetUserID != userID {
		return false
	}
	if !permissionVisible(event.AudiencePermission, permissions) {
		return false
	}
	if event.ModuleKey != "" && !modules[event.ModuleKey] && !modules["*"] {
		return false
	}
	return true
}

func (a *app) queryEvents(r *http.Request, userID string) ([]notificationEvent, error) {
	scope := normalizeScope(r.Header.Get("X-Himate-Notification-Scope"))
	partnerID := strings.TrimSpace(r.Header.Get("X-Himate-Partner-ID"))
	query := `SELECT e.id,e.event_type,e.severity,e.title,e.message,e.resource,e.partner_id,e.deep_link,e.audience_permission,
		e.delivery_scope,e.target_user_id,e.module_key,e.category,e.metadata,e.created_at,
		CASE WHEN rs.event_id IS NULL THEN FALSE ELSE TRUE END
		FROM notifications.events e
		LEFT JOIN notifications.read_state rs ON rs.event_id=e.id AND rs.user_id=$1
		WHERE e.delivery_scope=$2
		  AND (e.target_user_id='' OR e.target_user_id=$1)`
	args := []any{userID, scope}
	if scope == "PARTNER" {
		query += ` AND e.partner_id=$3`
		args = append(args, partnerID)
	}
	query += ` ORDER BY e.created_at DESC,e.id DESC LIMIT 500`
	rows, err := a.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []notificationEvent{}
	for rows.Next() {
		var event notificationEvent
		var metadataRaw []byte
		if err := rows.Scan(
			&event.ID, &event.EventType, &event.Severity, &event.Title, &event.Message, &event.Resource,
			&event.PartnerID, &event.DeepLink, &event.AudiencePermission, &event.DeliveryScope,
			&event.TargetUserID, &event.ModuleKey, &event.Category, &metadataRaw, &event.CreatedAt, &event.Read,
		); err != nil {
			continue
		}
		event.Metadata = map[string]any{}
		_ = json.Unmarshal(metadataRaw, &event.Metadata)
		out = append(out, event)
	}
	return out, rows.Err()
}

func (a *app) queryEventByIDInScope(r *http.Request, userID string, id int64) (*notificationEvent, error) {
	scope := normalizeScope(r.Header.Get("X-Himate-Notification-Scope"))
	partnerID := strings.TrimSpace(r.Header.Get("X-Himate-Partner-ID"))
	query := `SELECT e.id,e.event_type,e.severity,e.title,e.message,e.resource,e.partner_id,e.deep_link,e.audience_permission,
		e.delivery_scope,e.target_user_id,e.module_key,e.category,e.metadata,e.created_at,
		CASE WHEN rs.event_id IS NULL THEN FALSE ELSE TRUE END
		FROM notifications.events e
		LEFT JOIN notifications.read_state rs ON rs.event_id=e.id AND rs.user_id=$1
		WHERE e.id=$2 AND e.delivery_scope=$3`
	args := []any{userID, id, scope}
	if scope == "PARTNER" {
		query += ` AND e.partner_id=$4`
		args = append(args, partnerID)
	}
	var event notificationEvent
	var metadataRaw []byte
	err := a.db.QueryRow(query, args...).Scan(
		&event.ID, &event.EventType, &event.Severity, &event.Title, &event.Message, &event.Resource,
		&event.PartnerID, &event.DeepLink, &event.AudiencePermission, &event.DeliveryScope,
		&event.TargetUserID, &event.ModuleKey, &event.Category, &metadataRaw, &event.CreatedAt, &event.Read,
	)
	if err != nil {
		return nil, err
	}
	event.Metadata = map[string]any{}
	_ = json.Unmarshal(metadataRaw, &event.Metadata)
	return &event, nil
}

func eventMap(event notificationEvent) map[string]any {
	return map[string]any{
		"id":                  event.ID,
		"event_type":          event.EventType,
		"category":            event.Category,
		"severity":            event.Severity,
		"title":               event.Title,
		"message":             event.Message,
		"resource":            event.Resource,
		"partner_id":          event.PartnerID,
		"deep_link":           event.DeepLink,
		"audience_permission": event.AudiencePermission,
		"module_key":          event.ModuleKey,
		"metadata":            event.Metadata,
		"read":                event.Read,
		"created_at":          event.CreatedAt,
	}
}

func (a *app) feed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use GET")
		return
	}
	userID := strings.TrimSpace(r.Header.Get("X-Himate-User-ID"))
	if userID == "" {
		common.APIError(w, http.StatusUnauthorized, "USER_REQUIRED", "Authenticated user identity is required")
		return
	}
	scope := normalizeScope(r.Header.Get("X-Himate-Notification-Scope"))
	partnerID := strings.TrimSpace(r.Header.Get("X-Himate-Partner-ID"))
	if scope == "PARTNER" && partnerID == "" {
		common.APIError(w, http.StatusForbidden, "PARTNER_SCOPE_REQUIRED", "Partner notification scope requires a partner identity")
		return
	}
	permissions := csvSet(r.Header.Get("X-Himate-Permissions"))
	modules := csvSet(r.Header.Get("X-Himate-Module-Keys"))
	unreadOnly := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("unread_only")), "true")
	limit := limitValue(r.URL.Query().Get("limit"))
	events, err := a.queryEvents(r, userID)
	if err != nil {
		common.APIError(w, http.StatusInternalServerError, "DB", "Could not load notifications")
		return
	}
	items := []map[string]any{}
	unread := 0
	for _, event := range events {
		if !eventVisible(event, scope, partnerID, userID, permissions, modules) {
			continue
		}
		if !event.Read {
			unread++
		}
		if unreadOnly && event.Read {
			continue
		}
		if len(items) < limit {
			items = append(items, eventMap(event))
		}
	}
	common.JSON(w, http.StatusOK, map[string]any{
		"items": items, "count": len(items), "unread_count": unread, "delivery_scope": scope,
	})
}

func (a *app) notificationAction(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.Header.Get("X-Himate-User-ID"))
	if userID == "" {
		common.APIError(w, http.StatusUnauthorized, "USER_REQUIRED", "Authenticated user identity is required")
		return
	}
	scope := normalizeScope(r.Header.Get("X-Himate-Notification-Scope"))
	partnerID := strings.TrimSpace(r.Header.Get("X-Himate-Partner-ID"))
	if scope == "PARTNER" && partnerID == "" {
		common.APIError(w, http.StatusForbidden, "PARTNER_SCOPE_REQUIRED", "Partner notification scope requires a partner identity")
		return
	}
	permissions := csvSet(r.Header.Get("X-Himate-Permissions"))
	modules := csvSet(r.Header.Get("X-Himate-Module-Keys"))
	raw := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/notifications/"), "/")

	events, err := a.queryEvents(r, userID)
	if err != nil {
		common.APIError(w, http.StatusInternalServerError, "DB", "Could not load notification state")
		return
	}

	if raw == "read-all" {
		if r.Method != http.MethodPost {
			common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use POST")
			return
		}
		ids := []int64{}
		for _, event := range events {
			if eventVisible(event, scope, partnerID, userID, permissions, modules) {
				ids = append(ids, event.ID)
			}
		}
		tx, err := a.db.Begin()
		if err != nil {
			common.APIError(w, http.StatusInternalServerError, "DB", "Could not update notifications")
			return
		}
		defer tx.Rollback()
		if len(ids) > 0 {
			values := make([]string, 0, len(ids))
			args := make([]any, 0, len(ids)+1)
			args = append(args, userID)
			for i, id := range ids {
				values = append(values, fmt.Sprintf("($%d,$1)", i+2))
				args = append(args, id)
			}
			query := `INSERT INTO notifications.read_state(event_id,user_id) VALUES ` + strings.Join(values, ",") +
				` ON CONFLICT(event_id,user_id) DO UPDATE SET read_at=NOW()`
			if _, err = tx.Exec(query, args...); err != nil {
				common.APIError(w, http.StatusInternalServerError, "DB", "Could not update notifications")
				return
			}
		}
		if err = tx.Commit(); err != nil {
			common.APIError(w, http.StatusInternalServerError, "DB", "Could not commit notification state")
			return
		}
		common.JSON(w, http.StatusOK, map[string]any{"status": "read", "count": len(ids)})
		return
	}

	parts := strings.Split(raw, "/")
	if len(parts) != 2 || parts[1] != "read" {
		common.APIError(w, http.StatusNotFound, "NOT_FOUND", "Notification action not found")
		return
	}
	if r.Method != http.MethodPost {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use POST")
		return
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id < 1 {
		common.APIError(w, http.StatusNotFound, "NOT_FOUND", "Notification not found")
		return
	}
	var selected *notificationEvent
	for i := range events {
		if events[i].ID == id {
			selected = &events[i]
			break
		}
	}
	if selected == nil {
		selected, err = a.queryEventByIDInScope(r, userID, id)
		if err == sql.ErrNoRows {
			common.APIError(w, http.StatusNotFound, "NOT_FOUND", "Notification not found")
			return
		}
		if err != nil {
			common.APIError(w, http.StatusInternalServerError, "DB", "Could not load notification state")
			return
		}
	}
	if !eventVisible(*selected, scope, partnerID, userID, permissions, modules) {
		common.APIError(w, http.StatusForbidden, "FORBIDDEN", "Notification is outside your delivery scope")
		return
	}
	if _, err = a.db.Exec(`INSERT INTO notifications.read_state(event_id,user_id) VALUES($1,$2)
		ON CONFLICT(event_id,user_id) DO UPDATE SET read_at=NOW()`, id, userID); err != nil {
		common.APIError(w, http.StatusInternalServerError, "DB", "Could not update notification")
		return
	}
	common.JSON(w, http.StatusOK, map[string]any{"id": id, "read": true})
}

func (a *app) createEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use POST")
		return
	}
	var in struct {
		EventType          string         `json:"event_type"`
		Severity           string         `json:"severity"`
		Title              string         `json:"title"`
		Message            string         `json:"message"`
		Resource           string         `json:"resource"`
		PartnerID          string         `json:"partner_id"`
		DeepLink           string         `json:"deep_link"`
		AudiencePermission string         `json:"audience_permission"`
		DeliveryScope      string         `json:"delivery_scope"`
		TargetUserID       string         `json:"target_user_id"`
		ModuleKey          string         `json:"module_key"`
		Category           string         `json:"category"`
		Metadata           map[string]any `json:"metadata"`
	}
	if common.Decode(r, &in) != nil || strings.TrimSpace(in.EventType) == "" || strings.TrimSpace(in.Title) == "" {
		common.APIError(w, http.StatusBadRequest, "VALIDATION", "event_type and title are required")
		return
	}
	in.Severity = strings.ToUpper(strings.TrimSpace(in.Severity))
	if in.Severity == "" {
		in.Severity = "INFO"
	}
	if in.Severity != "INFO" && in.Severity != "WARNING" && in.Severity != "CRITICAL" {
		common.APIError(w, http.StatusBadRequest, "VALIDATION", "Invalid notification severity")
		return
	}
	in.PartnerID = strings.TrimSpace(in.PartnerID)
	in.TargetUserID = strings.TrimSpace(in.TargetUserID)
	in.ModuleKey = strings.TrimSpace(in.ModuleKey)
	in.AudiencePermission = strings.TrimSpace(in.AudiencePermission)
	scopeRaw := strings.ToUpper(strings.TrimSpace(in.DeliveryScope))
	if scopeRaw == "" {
		if in.PartnerID != "" {
			scopeRaw = "PARTNER"
		} else {
			scopeRaw = "PLATFORM"
		}
	}
	if scopeRaw != "PLATFORM" && scopeRaw != "PARTNER" {
		common.APIError(w, http.StatusBadRequest, "VALIDATION", "delivery_scope must be PLATFORM or PARTNER")
		return
	}
	if scopeRaw == "PARTNER" && in.PartnerID == "" {
		common.APIError(w, http.StatusBadRequest, "VALIDATION", "partner_id is required for PARTNER notifications")
		return
	}
	category := notificationCategory(in.EventType, in.Resource, in.Category)
	raw, _ := json.Marshal(in.Metadata)
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	var id int64
	var created time.Time
	err := a.db.QueryRow(`INSERT INTO notifications.events(
			event_type,severity,title,message,resource,partner_id,deep_link,audience_permission,delivery_scope,target_user_id,module_key,category,metadata
		) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13::jsonb) RETURNING id,created_at`,
		strings.ToUpper(strings.TrimSpace(in.EventType)), in.Severity, strings.TrimSpace(in.Title), strings.TrimSpace(in.Message),
		strings.TrimSpace(in.Resource), in.PartnerID, strings.TrimSpace(in.DeepLink), in.AudiencePermission, scopeRaw,
		in.TargetUserID, in.ModuleKey, category, string(raw),
	).Scan(&id, &created)
	if err != nil {
		common.APIError(w, http.StatusInternalServerError, "DB", "Could not create notification")
		return
	}
	common.JSON(w, http.StatusCreated, map[string]any{
		"id": id, "created_at": created, "delivery_scope": scopeRaw, "category": category,
	})
}
