package main

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"himate.local/services/internal/common"
)

func (a *app) partnerNotificationHeaders(u partnerUser) map[string]string {
	effective := []string{}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if policy, err := a.loadPartnerUserModulePolicy(ctx, u.PartnerID, u.ID, u.PreferredLocale); err == nil {
		effective = policy.Effective
	}
	return map[string]string{
		"X-Himate-User-ID":            u.ID,
		"X-Himate-Partner-ID":         u.PartnerID,
		"X-Himate-Permissions":        strings.Join(partnerPermissions(u.Role), ","),
		"X-Himate-Module-Keys":        strings.Join(effective, ","),
		"X-Himate-Notification-Scope": "PARTNER",
	}
}

func (a *app) partnerNotifications(w http.ResponseWriter, r *http.Request, u partnerUser) {
	upstream := strings.TrimPrefix(r.URL.Path, "/partner")
	if r.URL.RawQuery != "" {
		upstream += "?" + r.URL.RawQuery
	}
	var out map[string]any
	if err := a.internalJSON(r.Context(), r.Method, a.hosts["notifications"], upstream, nil, a.partnerNotificationHeaders(u), &out); err != nil {
		writeInternalError(w, err, "Notifications are temporarily unavailable")
		return
	}
	common.JSON(w, http.StatusOK, out)
}

func (a *app) emitPartnerLoginNotification(u partnerUser) {
	host := strings.TrimSpace(a.hosts["notifications"])
	if host == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	payload := map[string]any{
		"event_type":          "SECURITY_LOGIN_DETECTED",
		"category":            "SECURITY",
		"severity":            "INFO",
		"title":               "New sign-in detected",
		"message":             "A new sign-in to your HIMATE Partner Portal account was detected.",
		"resource":            "security",
		"partner_id":          u.PartnerID,
		"target_user_id":      u.ID,
		"deep_link":           "/partner/app",
		"audience_permission": "",
		"delivery_scope":      "PARTNER",
		"metadata": map[string]any{
			"user_id": u.ID,
			"role":    u.Role,
		},
	}
	var out map[string]any
	_ = a.internalJSON(ctx, http.MethodPost, host, "/internal/v1/notifications/events", payload, map[string]string{
		"X-Himate-User-ID": "partner-login-security",
	}, &out)
}

func partnerNotificationPath(base string, raw string) string {
	base = strings.TrimRight(base, "/")
	raw = strings.TrimLeft(raw, "/")
	if raw == "" {
		return base
	}
	return base + "/" + url.PathEscape(raw)
}
