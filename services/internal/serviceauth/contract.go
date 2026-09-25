package serviceauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	HeaderCallerID        = "X-Himate-Caller-ID"
	HeaderCallerTimestamp = "X-Himate-Caller-Timestamp"
	HeaderCallerSignature = "X-Himate-Caller-Signature"
)

var allowedCallers = map[string]bool{
	"gateway": true,
	"partners": true,
	"catalog": true,
	"billing": true,
	"payments": true,
	"contact": true,
	"provisioning": true,
	"environments": true,
	"connector": true,
	"health": true,
	"backups": true,
	"impact": true,
	"evidence": true,
	"reports": true,
	"cms": true,
	"notifications": true,
	"runtime": true,
	"tenantfinance": true,
	"automation": true,
	"storage": true,
	"workshop": true,
	"scheduler": true,
	"client-piano": true,
}

func KnownCaller(caller string) bool {
	return allowedCallers[strings.ToLower(strings.TrimSpace(caller))]
}

func canonical(req *http.Request, caller, timestamp string) string {
	if req == nil {
		return ""
	}
	path := "/"
	query := ""
	if req.URL != nil {
		if escaped := req.URL.EscapedPath(); escaped != "" {
			path = escaped
		}
		query = req.URL.RawQuery
	}
	fields := []string{
		strings.ToUpper(strings.TrimSpace(req.Method)),
		path,
		query,
		strings.TrimSpace(caller),
		strings.TrimSpace(timestamp),
		strings.TrimSpace(req.Header.Get("X-Himate-Partner-ID")),
		strings.TrimSpace(req.Header.Get("X-Himate-User-ID")),
		strings.TrimSpace(req.Header.Get("X-Himate-Permissions")),
		strings.TrimSpace(req.Header.Get("X-Himate-Notification-Scope")),
		strings.TrimSpace(req.Header.Get("X-Himate-Module-Keys")),
	}
	return strings.Join(fields, "\n")
}

func callerKey(token, caller string) []byte {
	root := hmac.New(sha256.New, []byte(token))
	_, _ = root.Write([]byte("himate-service-v1|" + strings.ToLower(strings.TrimSpace(caller))))
	return root.Sum(nil)
}

func Sign(req *http.Request, token, caller string, at time.Time) error {
	if req == nil {
		return errors.New("request is required")
	}
	token = strings.TrimSpace(token)
	caller = strings.ToLower(strings.TrimSpace(caller))
	if len(token) < 24 {
		return errors.New("internal service credential is not configured")
	}
	if !KnownCaller(caller) {
		return fmt.Errorf("unknown internal caller %q", caller)
	}
	timestamp := strconv.FormatInt(at.UTC().Unix(), 10)
	req.Header.Set(HeaderCallerID, caller)
	req.Header.Set(HeaderCallerTimestamp, timestamp)
	req.Header.Del(HeaderCallerSignature)
	mac := hmac.New(sha256.New, callerKey(token, caller))
	_, _ = mac.Write([]byte(canonical(req, caller, timestamp)))
	req.Header.Set(HeaderCallerSignature, base64.RawURLEncoding.EncodeToString(mac.Sum(nil)))
	return nil
}

func Verify(req *http.Request, token string, now time.Time, maxSkew time.Duration) (string, error) {
	if req == nil {
		return "", errors.New("request is required")
	}
	token = strings.TrimSpace(token)
	if len(token) < 24 {
		return "", errors.New("internal service credential is not configured")
	}
	caller := strings.ToLower(strings.TrimSpace(req.Header.Get(HeaderCallerID)))
	if !KnownCaller(caller) {
		return "", errors.New("unknown internal caller")
	}
	timestamp := strings.TrimSpace(req.Header.Get(HeaderCallerTimestamp))
	signature := strings.TrimSpace(req.Header.Get(HeaderCallerSignature))
	if timestamp == "" || signature == "" {
		return "", errors.New("missing internal service signature")
	}
	unixTime, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return "", errors.New("invalid internal service timestamp")
	}
	signedAt := time.Unix(unixTime, 0).UTC()
	if maxSkew <= 0 {
		maxSkew = 5 * time.Minute
	}
	delta := now.UTC().Sub(signedAt)
	if delta < 0 {
		delta = -delta
	}
	if delta > maxSkew {
		return "", errors.New("internal service signature is stale")
	}
	got, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		return "", errors.New("invalid internal service signature")
	}
	mac := hmac.New(sha256.New, callerKey(token, caller))
	_, _ = mac.Write([]byte(canonical(req, caller, timestamp)))
	expected := mac.Sum(nil)
	if len(got) != len(expected) || subtle.ConstantTimeCompare(got, expected) != 1 {
		return "", errors.New("invalid internal service signature")
	}
	return caller, nil
}
