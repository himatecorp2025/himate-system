package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type Claims struct {
	Subject   string   `json:"sub"`
	Email     string   `json:"email"`
	Name      string   `json:"name"`
	Roles     []string `json:"roles"`
	IssuedAt  int64    `json:"iat"`
	ExpiresAt int64    `json:"exp"`
}

type SessionManager struct {
	secret []byte
	ttl    time.Duration
}

func NewSessionManager(secret string, ttl time.Duration) (*SessionManager, error) {
	if len(secret) < 32 {
		return nil, errors.New("session secret must contain at least 32 characters")
	}
	if ttl <= 0 {
		return nil, errors.New("session ttl must be positive")
	}
	return &SessionManager{secret: []byte(secret), ttl: ttl}, nil
}

func (m *SessionManager) Issue(userID, email, name string, roles []string, now time.Time) (string, error) {
	claims := Claims{
		Subject:   userID,
		Email:     email,
		Name:      name,
		Roles:     append([]string(nil), roles...),
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(m.ttl).Unix(),
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	signature := m.sign(encodedPayload)
	return encodedPayload + "." + signature, nil
}

func (m *SessionManager) Parse(token string, now time.Time) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return Claims{}, errors.New("invalid session token")
	}

	expected := m.sign(parts[0])
	if !hmac.Equal([]byte(expected), []byte(parts[1])) {
		return Claims{}, errors.New("invalid session signature")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Claims{}, errors.New("invalid session payload")
	}

	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return Claims{}, errors.New("invalid session claims")
	}
	if claims.Subject == "" || claims.ExpiresAt <= now.Unix() {
		return Claims{}, errors.New("session expired")
	}
	return claims, nil
}

func (m *SessionManager) sign(payload string) string {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
