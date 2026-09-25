package main

import (
	"net/http"
	"strings"
)

var untrustedAuthorityHeaders = []string{
	"X-Himate-User-ID",
	"X-Himate-Partner-ID",
	"X-Himate-Permissions",
	"X-Himate-Module-Keys",
	"X-Himate-Notification-Scope",
	"X-Himate-Caller-ID",
	"X-Himate-Caller-Timestamp",
	"X-Himate-Caller-Signature",
}

func stripUntrustedAuthorityHeaders(r *http.Request) {
	if r == nil {
		return
	}
	for _, name := range untrustedAuthorityHeaders {
		r.Header.Del(name)
	}
}

func browserMutationOriginAllowed(r *http.Request) bool {
	if r == nil {
		return false
	}
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	if !requestOriginAllowed(r) {
		return false
	}
	fetchSite := strings.ToLower(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")))
	if fetchSite != "" && fetchSite != "same-origin" && fetchSite != "same-site" && fetchSite != "none" {
		return false
	}
	return true
}
