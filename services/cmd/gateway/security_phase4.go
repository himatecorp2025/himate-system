package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"himate.local/services/internal/common"
	"net/http"
	"net/url"
	"strings"
	"time"
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
	if r == nil { return }
	for _, name := range untrustedAuthorityHeaders { r.Header.Del(name) }
}

func browserMutationOriginAllowed(r *http.Request) bool {
	if r == nil { return false }
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	if !requestOriginAllowed(r) { return false }
	fetchSite := strings.ToLower(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")))
	return fetchSite == "" || fetchSite == "same-origin" || fetchSite == "same-site" || fetchSite == "none"
}

func phase4MFAMigration() common.Migration {
	return common.Migration{
		Version: 15,
		Name: "start-23-12-phase4-mfa",
		Statements: []string{
			`ALTER TABLE identity.users ADD COLUMN IF NOT EXISTS mfa_enabled BOOLEAN NOT NULL DEFAULT FALSE`,
			`ALTER TABLE identity.users ADD COLUMN IF NOT EXISTS mfa_secret_ciphertext TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE identity.users ADD COLUMN IF NOT EXISTS mfa_enrolled_at TIMESTAMPTZ`,
			`ALTER TABLE identity.partner_users ADD COLUMN IF NOT EXISTS mfa_enabled BOOLEAN NOT NULL DEFAULT FALSE`,
			`ALTER TABLE identity.partner_users ADD COLUMN IF NOT EXISTS mfa_secret_ciphertext TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE identity.partner_users ADD COLUMN IF NOT EXISTS mfa_enrolled_at TIMESTAMPTZ`,
			`CREATE TABLE IF NOT EXISTS identity.mfa_challenges(
				id TEXT PRIMARY KEY,
				identity_kind TEXT NOT NULL,
				user_id TEXT NOT NULL,
				purpose TEXT NOT NULL,
				pending_secret_ciphertext TEXT NOT NULL DEFAULT '',
				remember BOOLEAN NOT NULL DEFAULT FALSE,
				attempts INTEGER NOT NULL DEFAULT 0,
				expires_at TIMESTAMPTZ NOT NULL,
				used_at TIMESTAMPTZ,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				CHECK(identity_kind IN ('ADMIN','PARTNER')),
				CHECK(purpose IN ('SETUP','VERIFY')),
				CHECK(attempts BETWEEN 0 AND 5)
			)`,
			`CREATE INDEX IF NOT EXISTS identity_mfa_challenges_active_idx
				ON identity.mfa_challenges(identity_kind,user_id,expires_at) WHERE used_at IS NULL`,
		},
	}
}

func phase4RandomToken(size int) (string, error) {
	if size < 16 { size = 16 }
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil { return "", err }
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func (a *app) mfaAEAD() (cipher.AEAD, error) {
	key := sha256.Sum256([]byte("himate-phase4-mfa|" + a.secret))
	block, err := aes.NewCipher(key[:])
	if err != nil { return nil, err }
	return cipher.NewGCM(block)
}

func (a *app) encryptMFASecret(secret string) (string, error) {
	aead, err := a.mfaAEAD()
	if err != nil { return "", err }
	nonce := make([]byte, aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil { return "", err }
	sealed := aead.Seal(nonce, nonce, []byte(secret), []byte("himate-phase4-mfa"))
	return base64.RawStdEncoding.EncodeToString(sealed), nil
}

func (a *app) decryptMFASecret(encoded string) (string, error) {
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil { return "", err }
	aead, err := a.mfaAEAD()
	if err != nil { return "", err }
	if len(raw) < aead.NonceSize() { return "", errors.New("invalid mfa secret") }
	plain, err := aead.Open(nil, raw[:aead.NonceSize()], raw[aead.NonceSize():], []byte("himate-phase4-mfa"))
	if err != nil { return "", err }
	return string(plain), nil
}

func newTOTPSecret() (string, error) {
	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil { return "", err }
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw), nil
}

func totpCode(secret string, at time.Time) (string, error) {
	raw, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(strings.TrimSpace(secret)))
	if err != nil || len(raw) < 10 { return "", errors.New("invalid totp secret") }
	var counter [8]byte
	binary.BigEndian.PutUint64(counter[:], uint64(at.UTC().Unix()/30))
	mac := hmac.New(sha1.New, raw)
	_, _ = mac.Write(counter[:])
	sum := mac.Sum(nil)
	offset := int(sum[len(sum)-1] & 0x0f)
	value := (uint32(sum[offset])&0x7f)<<24 | uint32(sum[offset+1])<<16 | uint32(sum[offset+2])<<8 | uint32(sum[offset+3])
	return fmt.Sprintf("%06d", value%1000000), nil
}

func verifyTOTP(secret, code string, at time.Time) bool {
	code = strings.TrimSpace(code)
	if len(code) != 6 { return false }
	for offset := -1; offset <= 1; offset++ {
		expected, err := totpCode(secret, at.Add(time.Duration(offset)*30*time.Second))
		if err == nil && subtle.ConstantTimeCompare([]byte(expected), []byte(code)) == 1 { return true }
	}
	return false
}

func (a *app) mfaState(ctx context.Context, kind, userID string) (bool, string, error) {
	var enabled bool
	var encrypted string
	var err error
	switch kind {
	case "ADMIN":
		err = a.db.QueryRowContext(ctx, `SELECT mfa_enabled,mfa_secret_ciphertext FROM identity.users WHERE id=$1 AND active=TRUE`, userID).Scan(&enabled, &encrypted)
	case "PARTNER":
		err = a.db.QueryRowContext(ctx, `SELECT mfa_enabled,mfa_secret_ciphertext FROM identity.partner_users WHERE id=$1 AND active=TRUE`, userID).Scan(&enabled, &encrypted)
	default:
		err = errors.New("invalid identity kind")
	}
	return enabled, encrypted, err
}

func (a *app) beginMFAFlow(w http.ResponseWriter, r *http.Request, kind, userID string, remember, required bool) bool {
	if !a.mfaRequired || !required { return false }
	enabled, _, err := a.mfaState(r.Context(), kind, userID)
	if err != nil {
		common.APIError(w, http.StatusInternalServerError, "MFA", "Could not initialize multi-factor authentication")
		return true
	}
	challengeID, err := phase4RandomToken(24)
	if err != nil {
		common.APIError(w, http.StatusInternalServerError, "MFA", "Could not initialize multi-factor authentication")
		return true
	}
	purpose, pending, secret := "VERIFY", "", ""
	if !enabled {
		purpose = "SETUP"
		secret, err = newTOTPSecret()
		if err == nil { pending, err = a.encryptMFASecret(secret) }
		if err != nil {
			common.APIError(w, http.StatusInternalServerError, "MFA", "Could not initialize multi-factor authentication")
			return true
		}
	}
	_, _ = a.db.ExecContext(r.Context(), `UPDATE identity.mfa_challenges SET used_at=COALESCE(used_at,NOW())
		WHERE identity_kind=$1 AND user_id=$2 AND used_at IS NULL`, kind, userID)
	_, err = a.db.ExecContext(r.Context(), `INSERT INTO identity.mfa_challenges(
		id,identity_kind,user_id,purpose,pending_secret_ciphertext,remember,expires_at
	) VALUES($1,$2,$3,$4,$5,$6,NOW()+INTERVAL '10 minutes')`, challengeID, kind, userID, purpose, pending, remember)
	if err != nil {
		common.APIError(w, http.StatusInternalServerError, "MFA", "Could not initialize multi-factor authentication")
		return true
	}
	out := map[string]any{
		"mfa_required": true,
		"mfa_setup": !enabled,
		"challenge_id": challengeID,
		"expires_in_seconds": 600,
	}
	if !enabled {
		label := url.QueryEscape("HIMATE " + kind + " " + userID)
		out["secret"] = secret
		out["otpauth_uri"] = "otpauth://totp/" + label + "?secret=" + url.QueryEscape(secret) + "&issuer=HIMATE&algorithm=SHA1&digits=6&period=30"
	}
	common.JSON(w, http.StatusAccepted, out)
	return true
}

func (a *app) completeMFAFlow(ctx context.Context, kind, challengeID, code string) (string, bool, error) {
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil { return "", false, err }
	defer tx.Rollback()

	var userID, purpose, pending string
	var remember bool
	var attempts int
	var expires time.Time
	err = tx.QueryRowContext(ctx, `SELECT user_id,purpose,pending_secret_ciphertext,remember,attempts,expires_at
		FROM identity.mfa_challenges
		WHERE id=$1 AND identity_kind=$2 AND used_at IS NULL
		FOR UPDATE`, strings.TrimSpace(challengeID), kind).
		Scan(&userID, &purpose, &pending, &remember, &attempts, &expires)
	if err != nil || time.Now().UTC().After(expires) || attempts >= 5 {
		return "", false, errors.New("invalid mfa challenge")
	}

	encrypted := pending
	if purpose == "VERIFY" {
		var enabled bool
		switch kind {
		case "ADMIN":
			err = tx.QueryRowContext(ctx, `SELECT mfa_enabled,mfa_secret_ciphertext FROM identity.users WHERE id=$1 AND active=TRUE`, userID).Scan(&enabled, &encrypted)
		case "PARTNER":
			err = tx.QueryRowContext(ctx, `SELECT mfa_enabled,mfa_secret_ciphertext FROM identity.partner_users WHERE id=$1 AND active=TRUE`, userID).Scan(&enabled, &encrypted)
		default:
			err = errors.New("invalid identity kind")
		}
		if err != nil || !enabled || strings.TrimSpace(encrypted) == "" { return "", false, errors.New("invalid mfa challenge") }
	}

	secret, err := a.decryptMFASecret(encrypted)
	if err != nil || !verifyTOTP(secret, code, time.Now().UTC()) {
		attempts++
		_, _ = tx.ExecContext(ctx, `UPDATE identity.mfa_challenges
			SET attempts=$2,used_at=CASE WHEN $2>=5 THEN NOW() ELSE used_at END WHERE id=$1`, challengeID, attempts)
		_ = tx.Commit()
		return "", false, errors.New("invalid mfa code")
	}

	if purpose == "SETUP" {
		switch kind {
		case "ADMIN":
			_, err = tx.ExecContext(ctx, `UPDATE identity.users
				SET mfa_enabled=TRUE,mfa_secret_ciphertext=$2,mfa_enrolled_at=NOW(),updated_at=NOW()
				WHERE id=$1 AND active=TRUE`, userID, encrypted)
		case "PARTNER":
			_, err = tx.ExecContext(ctx, `UPDATE identity.partner_users
				SET mfa_enabled=TRUE,mfa_secret_ciphertext=$2,mfa_enrolled_at=NOW(),updated_at=NOW()
				WHERE id=$1 AND active=TRUE`, userID, encrypted)
		}
		if err != nil { return "", false, err }
	}
	if _, err = tx.ExecContext(ctx, `UPDATE identity.mfa_challenges SET used_at=NOW() WHERE id=$1`, challengeID); err != nil {
		return "", false, err
	}
	if err = tx.Commit(); err != nil { return "", false, err }
	return userID, remember, nil
}

func (a *app) adminMFAVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { common.APIError(w, 405, "METHOD", "Use POST"); return }
	if !browserMutationOriginAllowed(r) { common.APIError(w, 403, "CSRF", "Cross-site request rejected"); return }
	var in struct {
		ChallengeID string `json:"challenge_id"`
		Code string `json:"code"`
	}
	if common.Decode(r, &in) != nil { common.APIError(w, 400, "JSON", "Invalid request"); return }
	userID, remember, err := a.completeMFAFlow(r.Context(), "ADMIN", in.ChallengeID, in.Code)
	if err != nil { common.APIError(w, 401, "MFA_INVALID", "Invalid or expired authentication code"); return }
	u, err := a.findUser("id", userID)
	if err != nil || !u.Active { common.APIError(w, 401, "UNAUTHORIZED", "Authentication required"); return }
	ttl := a.ttl; if remember { ttl = a.rememberTTL }
	token, err := a.issueSession(u, ttl)
	if err != nil { common.APIError(w, 500, "SESSION", "Could not create session"); return }
	cookie := &http.Cookie{Name:sessionCookie,Value:token,Path:"/",HttpOnly:true,Secure:a.secureCookie,SameSite:http.SameSiteStrictMode}
	if remember { cookie.MaxAge=int(ttl.Seconds()); cookie.Expires=time.Now().UTC().Add(ttl) }
	http.SetCookie(w,cookie)
	common.JSON(w,200,a.publicUser(u))
}

func partnerMFARequired(role string) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "owner", "admin", "billing":
		return true
	default:
		return false
	}
}

func (a *app) partnerMFAVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { common.APIError(w, 405, "METHOD", "Use POST"); return }
	if !browserMutationOriginAllowed(r) { common.APIError(w, 403, "CSRF", "Cross-site request rejected"); return }
	var in struct {
		ChallengeID string `json:"challenge_id"`
		Code string `json:"code"`
	}
	if common.Decode(r, &in) != nil { common.APIError(w, 400, "JSON", "Invalid request"); return }
	userID, remember, err := a.completeMFAFlow(r.Context(), "PARTNER", in.ChallengeID, in.Code)
	if err != nil { common.APIError(w, 401, "MFA_INVALID", "Invalid or expired authentication code"); return }
	u, err := a.findPartnerUser("id", userID)
	if err != nil || !u.Active { common.APIError(w, 401, "UNAUTHORIZED", "Partner authentication required"); return }
	ctx,cancel:=context.WithTimeout(r.Context(),2*time.Second); accessErr:=a.partnerAccessAllowed(ctx,u.PartnerID); cancel()
	if accessErr!=nil { writePartnerAccessError(w,accessErr); return }
	ttl:=a.ttl; if remember { ttl=a.rememberTTL }
	token,err:=a.issuePartnerSession(u,ttl)
	if err!=nil { common.APIError(w,500,"SESSION","Could not create session");return }
	cookie:=&http.Cookie{Name:partnerSessionCookie,Value:token,Path:"/partner",HttpOnly:true,Secure:a.secureCookie,SameSite:http.SameSiteStrictMode}
	if remember { cookie.MaxAge=int(ttl.Seconds());cookie.Expires=time.Now().UTC().Add(ttl) }
	http.SetCookie(w,cookie)
	go a.emitPartnerLoginNotification(u)
	common.JSON(w,200,partnerUserMap(u))
}
