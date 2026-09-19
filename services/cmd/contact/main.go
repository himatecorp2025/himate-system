package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"himate.local/services/internal/common"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type app struct {
	db       *sql.DB
	limiter  *rateLimiter
	smtpHost string
	smtpPort string
	smtpUser string
	smtpPass string
	smtpFrom string
	notifyTo string
}

type rateLimiter struct {
	mu   sync.Mutex
	hits map[string][]time.Time
}

type inquiry struct {
	Name         string `json:"name"`
	Organization string `json:"organization"`
	Email        string `json:"email"`
	Message      string `json:"message"`
	Website      string `json:"website"`
}

func main() {
	log := common.Logger()
	db, err := common.OpenDB()
	if err != nil {
		log.Error("database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	a := &app{
		db:       db,
		limiter:  &rateLimiter{hits: map[string][]time.Time{}},
		smtpHost: strings.TrimSpace(os.Getenv("SMTP_HOST")),
		smtpPort: common.Env("SMTP_PORT", "587"),
		smtpUser: strings.TrimSpace(os.Getenv("SMTP_USERNAME")),
		smtpPass: os.Getenv("SMTP_PASSWORD"),
		smtpFrom: strings.TrimSpace(os.Getenv("SMTP_FROM")),
		notifyTo: strings.TrimSpace(os.Getenv("HIMATE_CONTACT_NOTIFY_TO")),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := a.migrate(ctx); err != nil {
		log.Error("migration", "error", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		common.JSON(w, http.StatusOK, map[string]any{"status": "ok", "service": "himate-contact"})
	})
	mux.HandleFunc("/api/v1/public/contact", a.handleContact)
	common.Run(log, "contact", common.Env("PORT", "10000"), common.InternalAuth(os.Getenv("HIMATE_INTERNAL_TOKEN"), mux))
}

func (a *app) migrate(ctx context.Context) error {
	return common.ExecStatements(ctx, a.db,
		`CREATE SCHEMA IF NOT EXISTS contact`,
		`CREATE TABLE IF NOT EXISTS contact.inquiries(
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			organization TEXT NOT NULL DEFAULT '',
			email TEXT NOT NULL,
			message TEXT NOT NULL,
			source_ip TEXT NOT NULL DEFAULT '',
			user_agent TEXT NOT NULL DEFAULT '',
			notification_status TEXT NOT NULL DEFAULT 'pending',
			notification_error TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_contact_inquiries_created_at ON contact.inquiries(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_contact_inquiries_email ON contact.inquiries(email)`,
	)
}

func (a *app) handleContact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use POST")
		return
	}

	ip := clientIP(r)
	if !a.limiter.allow(ip, 5, 10*time.Minute) {
		common.APIError(w, http.StatusTooManyRequests, "RATE_LIMIT", "Too many inquiries. Please try again later.")
		return
	}

	var in inquiry
	if err := common.Decode(r, &in); err != nil {
		common.APIError(w, http.StatusBadRequest, "JSON", "Invalid request")
		return
	}

	in.Name = strings.TrimSpace(in.Name)
	in.Organization = strings.TrimSpace(in.Organization)
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.Message = strings.TrimSpace(in.Message)
	in.Website = strings.TrimSpace(in.Website)

	// Honeypot: bots commonly populate hidden website fields.
	if in.Website != "" {
		common.JSON(w, http.StatusAccepted, map[string]any{"received": true})
		return
	}

	if len(in.Name) < 2 || len(in.Name) > 120 {
		common.APIError(w, http.StatusBadRequest, "NAME", "Please enter your name.")
		return
	}
	if len(in.Organization) > 180 {
		common.APIError(w, http.StatusBadRequest, "ORGANIZATION", "Organization is too long.")
		return
	}
	if len(in.Email) > 254 {
		common.APIError(w, http.StatusBadRequest, "EMAIL", "Please enter a valid email address.")
		return
	}
	addr, err := mail.ParseAddress(in.Email)
	if err != nil || !strings.EqualFold(addr.Address, in.Email) {
		common.APIError(w, http.StatusBadRequest, "EMAIL", "Please enter a valid email address.")
		return
	}
	if len(in.Message) < 10 || len(in.Message) > 5000 {
		common.APIError(w, http.StatusBadRequest, "MESSAGE", "Please enter a message between 10 and 5000 characters.")
		return
	}

	id, err := newID()
	if err != nil {
		common.APIError(w, http.StatusInternalServerError, "ID", "Unable to accept inquiry.")
		return
	}

	_, err = a.db.ExecContext(r.Context(),
		`INSERT INTO contact.inquiries(id,name,organization,email,message,source_ip,user_agent)
		 VALUES($1,$2,$3,$4,$5,$6,$7)`,
		id, in.Name, in.Organization, in.Email, in.Message, ip, truncate(r.UserAgent(), 500),
	)
	if err != nil {
		common.APIError(w, http.StatusInternalServerError, "STORE", "Unable to save inquiry.")
		return
	}

	status := "stored"
	if a.smtpConfigured() {
		if err := a.sendNotification(in, id); err != nil {
			status = "notification_failed"
			_, _ = a.db.ExecContext(r.Context(),
				`UPDATE contact.inquiries SET notification_status='failed', notification_error=$2 WHERE id=$1`,
				id, truncate(err.Error(), 800),
			)
		} else {
			status = "notified"
			_, _ = a.db.ExecContext(r.Context(),
				`UPDATE contact.inquiries SET notification_status='sent', notification_error='' WHERE id=$1`, id)
		}
	}

	common.JSON(w, http.StatusAccepted, map[string]any{
		"received": true,
		"id":       id,
		"status":   status,
	})
}

func (a *app) smtpConfigured() bool {
	return a.smtpHost != "" && a.smtpPort != "" && a.smtpFrom != "" && a.notifyTo != ""
}

func (a *app) sendNotification(in inquiry, id string) error {
	hostPort := net.JoinHostPort(a.smtpHost, a.smtpPort)
	var auth smtp.Auth
	if a.smtpUser != "" {
		auth = smtp.PlainAuth("", a.smtpUser, a.smtpPass, a.smtpHost)
	}
	subject := "New HIMATE website inquiry - " + in.Name
	body := fmt.Sprintf(
		"New HIMATE website inquiry\r\n\r\nInquiry ID: %s\r\nName: %s\r\nOrganization: %s\r\nEmail: %s\r\n\r\nMessage:\r\n%s\r\n",
		id, in.Name, in.Organization, in.Email, in.Message,
	)
	msg := []byte(
		"From: " + a.smtpFrom + "\r\n" +
			"To: " + a.notifyTo + "\r\n" +
			"Reply-To: " + in.Email + "\r\n" +
			"Subject: " + subject + "\r\n" +
			"MIME-Version: 1.0\r\n" +
			"Content-Type: text/plain; charset=UTF-8\r\n\r\n" +
			body,
	)
	return smtp.SendMail(hostPort, auth, a.smtpFrom, []string{a.notifyTo}, msg)
}

func (l *rateLimiter) allow(key string, limit int, window time.Duration) bool {
	now := time.Now()
	cutoff := now.Add(-window)
	l.mu.Lock()
	defer l.mu.Unlock()
	hits := l.hits[key]
	kept := hits[:0]
	for _, t := range hits {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= limit {
		l.hits[key] = kept
		return false
	}
	l.hits[key] = append(kept, now)
	return true
}

func clientIP(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		if comma := strings.IndexByte(forwarded, ','); comma >= 0 {
			forwarded = forwarded[:comma]
		}
		return strings.TrimSpace(forwarded)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func newID() (string, error) {
	raw := make([]byte, 12)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "inq_" + hex.EncodeToString(raw), nil
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}
