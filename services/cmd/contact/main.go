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
	Name             string `json:"name"`
	Organization     string `json:"organization"`
	Email            string `json:"email"`
	OrganizationType string `json:"organization_type"`
	InquiryTopic     string `json:"inquiry_topic"`
	Message          string `json:"message"`
	Website          string `json:"website"`
}

type inquiryRecord struct {
	ID, Name, Organization, Email, OrganizationType, InquiryTopic, Message string
	LeadStatus, AssignedTo, AdminNote                                      string
	NotificationStatus, NotificationError                                 string
	CreatedAt, UpdatedAt                                                   time.Time
}

var leadStatuses = map[string]bool{
	"NEW": true, "IN_PROGRESS": true, "CONTACTED": true, "CLOSED": true,
}

var organizationTypes = map[string]bool{
	"CLASSICAL_MUSIC": true,
	"FINE_ART": true,
	"GALLERY": true,
	"THEATRE": true,
	"CULTURAL_ORGANIZATION": true,
	"PIANO_TECHNOLOGY": true,
	"MUSEUM": true,
	"FOUNDATION": true,
	"CREATIVE_NETWORK": true,
	"OTHER": true,
}

var inquiryTopics = map[string]bool{
	"PARTNERSHIP": true,
	"PLATFORM_DEMO": true,
	"PRICING_LICENSING": true,
	"CHARITY_SPONSORSHIP": true,
	"TECHNICAL": true,
	"OTHER": true,
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
	mux.HandleFunc("/api/v1/contact/inquiries", a.handleAdminInquiries)
	mux.HandleFunc("/api/v1/contact/inquiries/", a.handleAdminInquiry)
	common.Run(log, "contact", common.Env("PORT", "10000"), common.InternalAuth(os.Getenv("HIMATE_INTERNAL_TOKEN"), mux))
}

func (a *app) migrate(ctx context.Context) error {
	return common.ApplyMigrations(ctx, a.db, "contact", []common.Migration{
		{Version: 1, Name: "contact-base", Statements: []string{
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
		}},
		{Version: 2, Name: "contact-lead-workflow", Statements: []string{
			`ALTER TABLE contact.inquiries ADD COLUMN IF NOT EXISTS lead_status TEXT NOT NULL DEFAULT 'NEW'`,
			`ALTER TABLE contact.inquiries ADD COLUMN IF NOT EXISTS assigned_to TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE contact.inquiries ADD COLUMN IF NOT EXISTS admin_note TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE contact.inquiries ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`,
			`CREATE INDEX IF NOT EXISTS idx_contact_inquiries_status_created ON contact.inquiries(lead_status,created_at DESC)`,
		}},
		{Version: 3, Name: "central-1-contact-classification", Statements: []string{
			`ALTER TABLE contact.inquiries ADD COLUMN IF NOT EXISTS organization_type TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE contact.inquiries ADD COLUMN IF NOT EXISTS inquiry_topic TEXT NOT NULL DEFAULT ''`,
			`CREATE INDEX IF NOT EXISTS idx_contact_inquiries_organization_type ON contact.inquiries(organization_type) WHERE organization_type<>''`,
			`CREATE INDEX IF NOT EXISTS idx_contact_inquiries_topic ON contact.inquiries(inquiry_topic) WHERE inquiry_topic<>''`,
		}},
	})
}


func normalizeLeadStatus(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if leadStatuses[value] { return value }
	return ""
}

func normalizeContactClassification(value string, allowed map[string]bool) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if allowed[value] { return value }
	return ""
}

func scanInquiry(scanner interface{ Scan(...any) error }) (inquiryRecord, error) {
	var item inquiryRecord
	err := scanner.Scan(
		&item.ID, &item.Name, &item.Organization, &item.Email, &item.OrganizationType, &item.InquiryTopic, &item.Message,
		&item.LeadStatus, &item.AssignedTo, &item.AdminNote,
		&item.NotificationStatus, &item.NotificationError,
		&item.CreatedAt, &item.UpdatedAt,
	)
	return item, err
}

func inquiryJSON(item inquiryRecord) map[string]any {
	return map[string]any{
		"id": item.ID,
		"name": item.Name,
		"organization": item.Organization,
		"email": item.Email,
		"organization_type": item.OrganizationType,
		"inquiry_topic": item.InquiryTopic,
		"message": item.Message,
		"lead_status": item.LeadStatus,
		"assigned_to": item.AssignedTo,
		"admin_note": item.AdminNote,
		"notification_status": item.NotificationStatus,
		"notification_error": item.NotificationError,
		"created_at": item.CreatedAt.UTC(),
		"updated_at": item.UpdatedAt.UTC(),
	}
}

const inquirySelect = `SELECT id,name,organization,email,organization_type,inquiry_topic,message,lead_status,assigned_to,admin_note,
notification_status,notification_error,created_at,updated_at FROM contact.inquiries`

func (a *app) handleAdminInquiries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use GET")
		return
	}

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	status := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("status")))
	if status != "" && status != "ALL" && !leadStatuses[status] {
		common.APIError(w, http.StatusBadRequest, "STATUS", "Invalid lead status")
		return
	}
	limit := envIntValue(r.URL.Query().Get("limit"), 50, 1, 200)
	offset := envIntValue(r.URL.Query().Get("offset"), 0, 0, 1000000)

	where := []string{"1=1"}
	args := []any{}
	if q != "" {
		args = append(args, "%"+q+"%")
		pos := len(args)
		where = append(where, fmt.Sprintf("(name ILIKE $%d OR organization ILIKE $%d OR email ILIKE $%d OR organization_type ILIKE $%d OR inquiry_topic ILIKE $%d OR message ILIKE $%d)", pos, pos, pos, pos, pos, pos))
	}
	if status != "" && status != "ALL" {
		args = append(args, status)
		where = append(where, fmt.Sprintf("lead_status=$%d", len(args)))
	}
	whereSQL := strings.Join(where, " AND ")

	var total int
	if err := a.db.QueryRow("SELECT COUNT(*) FROM contact.inquiries WHERE "+whereSQL, args...).Scan(&total); err != nil {
		common.APIError(w, http.StatusInternalServerError, "DB", "Unable to load contact leads")
		return
	}

	queryArgs := append(append([]any{}, args...), limit, offset)
	rows, err := a.db.Query(
		inquirySelect+" WHERE "+whereSQL+" ORDER BY created_at DESC,id DESC LIMIT $"+strconv.Itoa(len(args)+1)+" OFFSET $"+strconv.Itoa(len(args)+2),
		queryArgs...,
	)
	if err != nil {
		common.APIError(w, http.StatusInternalServerError, "DB", "Unable to load contact leads")
		return
	}
	defer rows.Close()

	items := []map[string]any{}
	for rows.Next() {
		item, err := scanInquiry(rows)
		if err != nil { continue }
		items = append(items, inquiryJSON(item))
	}
	common.JSON(w, http.StatusOK, map[string]any{
		"items": items,
		"total": total,
		"limit": limit,
		"offset": offset,
		"has_more": offset+len(items) < total,
	})
}

func (a *app) handleAdminInquiry(w http.ResponseWriter, r *http.Request) {
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/contact/inquiries/"), "/")
	if id == "" || strings.Contains(id, "/") {
		common.APIError(w, http.StatusNotFound, "NOT_FOUND", "Contact lead not found")
		return
	}

	switch r.Method {
	case http.MethodGet:
		item, err := scanInquiry(a.db.QueryRow(inquirySelect+" WHERE id=$1", id))
		if err == sql.ErrNoRows {
			common.APIError(w, http.StatusNotFound, "NOT_FOUND", "Contact lead not found")
			return
		}
		if err != nil {
			common.APIError(w, http.StatusInternalServerError, "DB", "Unable to load contact lead")
			return
		}
		common.JSON(w, http.StatusOK, inquiryJSON(item))
	case http.MethodPatch:
		var in struct {
			LeadStatus *string `json:"lead_status"`
			AssignedTo *string `json:"assigned_to"`
			AdminNote  *string `json:"admin_note"`
		}
		if err := common.Decode(r, &in); err != nil {
			common.APIError(w, http.StatusBadRequest, "JSON", "Invalid request")
			return
		}
		current, err := scanInquiry(a.db.QueryRow(inquirySelect+" WHERE id=$1", id))
		if err == sql.ErrNoRows {
			common.APIError(w, http.StatusNotFound, "NOT_FOUND", "Contact lead not found")
			return
		}
		if err != nil {
			common.APIError(w, http.StatusInternalServerError, "DB", "Unable to load contact lead")
			return
		}
		if in.LeadStatus != nil {
			status := normalizeLeadStatus(*in.LeadStatus)
			if status == "" {
				common.APIError(w, http.StatusBadRequest, "STATUS", "Invalid lead status")
				return
			}
			current.LeadStatus = status
		}
		if in.AssignedTo != nil {
			current.AssignedTo = strings.TrimSpace(*in.AssignedTo)
			if len(current.AssignedTo) > 160 {
				common.APIError(w, http.StatusBadRequest, "ASSIGNEE", "Assigned-to value is too long")
				return
			}
		}
		if in.AdminNote != nil {
			current.AdminNote = strings.TrimSpace(*in.AdminNote)
			if len(current.AdminNote) > 4000 {
				common.APIError(w, http.StatusBadRequest, "NOTE", "Admin note is too long")
				return
			}
		}
		_, err = a.db.Exec(
			`UPDATE contact.inquiries SET lead_status=$2,assigned_to=$3,admin_note=$4,updated_at=NOW() WHERE id=$1`,
			id, current.LeadStatus, current.AssignedTo, current.AdminNote,
		)
		if err != nil {
			common.APIError(w, http.StatusInternalServerError, "DB", "Unable to update contact lead")
			return
		}
		item, err := scanInquiry(a.db.QueryRow(inquirySelect+" WHERE id=$1", id))
		if err != nil {
			common.APIError(w, http.StatusInternalServerError, "DB", "Unable to reload contact lead")
			return
		}
		common.JSON(w, http.StatusOK, inquiryJSON(item))
	default:
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use GET or PATCH")
	}
}

func envIntValue(value string, fallback, min, max int) int {
	value = strings.TrimSpace(value)
	if value == "" { return fallback }
	n, err := strconv.Atoi(value)
	if err != nil { return fallback }
	if n < min { return min }
	if n > max { return max }
	return n
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
	in.OrganizationType = normalizeContactClassification(in.OrganizationType, organizationTypes)
	in.InquiryTopic = normalizeContactClassification(in.InquiryTopic, inquiryTopics)
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
	if in.OrganizationType == "" {
		common.APIError(w, http.StatusBadRequest, "ORGANIZATION_TYPE", "Please select your organization type.")
		return
	}
	if in.InquiryTopic == "" {
		common.APIError(w, http.StatusBadRequest, "INQUIRY_TOPIC", "Please select an inquiry topic.")
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
		`INSERT INTO contact.inquiries(id,name,organization,email,organization_type,inquiry_topic,message,source_ip,user_agent)
		 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		id, in.Name, in.Organization, in.Email, in.OrganizationType, in.InquiryTopic, in.Message, ip, truncate(r.UserAgent(), 500),
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
		"New HIMATE website inquiry\r\n\r\nInquiry ID: %s\r\nName: %s\r\nOrganization: %s\r\nEmail: %s\r\nOrganization type: %s\r\nInquiry topic: %s\r\n\r\nMessage:\r\n%s\r\n",
		id, in.Name, in.Organization, in.Email, in.OrganizationType, in.InquiryTopic, in.Message,
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
