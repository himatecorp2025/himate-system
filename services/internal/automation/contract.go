package automation

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	HeaderServiceID        = "X-Himate-Service-ID"
	HeaderServiceTimestamp = "X-Himate-Service-Timestamp"
	HeaderServiceSignature = "X-Himate-Service-Signature"
	HeaderCorrelationID    = "X-Himate-Correlation-ID"
	HeaderCausationID      = "X-Himate-Causation-ID"
)

func bodyHash(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func canonical(serviceID, method, path, timestamp string, body []byte) string {
	return strings.Join([]string{timestamp, serviceID, strings.ToUpper(method), path, bodyHash(body)}, "\n")
}

func Signature(secret, serviceID, method, path, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(canonical(serviceID, method, path, timestamp, body)))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func SignRequest(req *http.Request, body []byte, serviceID, secret string, now time.Time) error {
	serviceID = strings.TrimSpace(serviceID)
	if req == nil || serviceID == "" || len(secret) < 24 {
		return errors.New("automation service identity is not configured")
	}
	ts := strconv.FormatInt(now.UTC().Unix(), 10)
	req.Header.Set(HeaderServiceID, serviceID)
	req.Header.Set(HeaderServiceTimestamp, ts)
	req.Header.Set(HeaderServiceSignature, Signature(secret, serviceID, req.Method, req.URL.Path, ts, body))
	return nil
}

func VerifyRequest(req *http.Request, body []byte, secretFor func(string) (string, bool), now time.Time, tolerance time.Duration) (string, error) {
	if req == nil || secretFor == nil {
		return "", errors.New("automation verification is not configured")
	}
	serviceID := strings.TrimSpace(req.Header.Get(HeaderServiceID))
	tsRaw := strings.TrimSpace(req.Header.Get(HeaderServiceTimestamp))
	got := strings.TrimSpace(req.Header.Get(HeaderServiceSignature))
	if serviceID == "" || tsRaw == "" || got == "" {
		return "", errors.New("service identity headers are required")
	}
	secret, ok := secretFor(serviceID)
	if !ok || len(secret) < 24 {
		return "", errors.New("unknown service identity")
	}
	ts, err := strconv.ParseInt(tsRaw, 10, 64)
	if err != nil {
		return "", errors.New("invalid service identity timestamp")
	}
	signedAt := time.Unix(ts, 0).UTC()
	if now.UTC().Sub(signedAt) > tolerance || signedAt.Sub(now.UTC()) > tolerance {
		return "", errors.New("service identity signature expired")
	}
	want := Signature(secret, serviceID, req.Method, req.URL.Path, tsRaw, body)
	if len(got) != len(want) || subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
		return "", fmt.Errorf("invalid service identity signature")
	}
	return serviceID, nil
}
