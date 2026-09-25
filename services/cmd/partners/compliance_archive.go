package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"himate.local/services/internal/common"
)

const complianceRetentionYears = 7

type complianceSource struct {
	Key       string
	Schema    string
	Table     string
	PartnerID string
}

var complianceSources = []complianceSource{
	{Key: "partner", Schema: "partners", Table: "partners", PartnerID: "id"},
	{Key: "lifecycle_history", Schema: "partners", Table: "lifecycle_history", PartnerID: "partner_id"},
	{Key: "administrative_audit", Schema: "identity", Table: "audit_events", PartnerID: "partner_id"},
	{Key: "commercial_terms", Schema: "billing", Table: "partner_terms", PartnerID: "partner_id"},
	{Key: "initial_license", Schema: "billing", Table: "initial_licenses", PartnerID: "partner_id"},
	{Key: "base_fee_history", Schema: "billing", Table: "base_fee_history", PartnerID: "partner_id"},
	{Key: "module_subscriptions", Schema: "billing", Table: "module_subscriptions", PartnerID: "partner_id"},
	{Key: "subscription_history", Schema: "billing", Table: "subscription_history", PartnerID: "partner_id"},
	{Key: "billing_documents", Schema: "billing", Table: "documents", PartnerID: "partner_id"},
	{Key: "platform_invoices", Schema: "billing", Table: "invoices", PartnerID: "partner_id"},
	{Key: "plan_subscriptions", Schema: "billing", Table: "partner_plan_subscriptions", PartnerID: "partner_id"},
	{Key: "plan_module_selections", Schema: "billing", Table: "partner_plan_module_selections", PartnerID: "partner_id"},
	{Key: "plan_change_history", Schema: "billing", Table: "plan_change_history", PartnerID: "partner_id"},
	{Key: "payment_attempts", Schema: "payments", Table: "attempts", PartnerID: "partner_id"},
	{Key: "evidence", Schema: "evidence", Table: "items", PartnerID: "partner_id"},
	{Key: "tenant_finance_policy", Schema: "tenant_finance", Table: "policies", PartnerID: "partner_id"},
	{Key: "tenant_invoices", Schema: "tenant_finance", Table: "invoices", PartnerID: "partner_id"},
	{Key: "tenant_invoice_items", Schema: "tenant_finance", Table: "invoice_items", PartnerID: "partner_id"},
	{Key: "tenant_invoice_events", Schema: "tenant_finance", Table: "invoice_events", PartnerID: "partner_id"},
}

type complianceArchive struct {
	PartnerID     string
	DisplayName   string
	LegalName     string
	ArchiveReason string
	CreatedBy     string
	ArchivedAt    time.Time
	RetainUntil   time.Time
	Payload       json.RawMessage
	PayloadSHA256 string
	CreatedAt     time.Time
}

func complianceArchiveMigration() common.Migration {
	return common.Migration{
		Version: 9,
		Name:    "start-23-12-phase5-compliance-vault",
		Statements: []string{
			`CREATE SCHEMA IF NOT EXISTS compliance`,
			`CREATE TABLE IF NOT EXISTS compliance.partner_archives(
				partner_id TEXT PRIMARY KEY,
				display_name TEXT NOT NULL DEFAULT '',
				legal_name TEXT NOT NULL DEFAULT '',
				archive_reason TEXT NOT NULL DEFAULT '',
				created_by TEXT NOT NULL DEFAULT '',
				archived_at TIMESTAMPTZ NOT NULL,
				retain_until TIMESTAMPTZ NOT NULL,
				payload JSONB NOT NULL,
				canonical_payload TEXT NOT NULL,
				payload_sha256 TEXT NOT NULL CHECK(payload_sha256 ~ '^[0-9a-f]{64}$'),
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				CHECK(retain_until >= archived_at + INTERVAL '7 years')
			)`,
			`CREATE INDEX IF NOT EXISTS compliance_partner_archived_idx
				ON compliance.partner_archives(archived_at DESC,partner_id)`,
			`CREATE INDEX IF NOT EXISTS compliance_partner_retain_idx
				ON compliance.partner_archives(retain_until)`,
			`CREATE OR REPLACE FUNCTION compliance.reject_archive_mutation() RETURNS trigger AS $$
				BEGIN
					RAISE EXCEPTION 'compliance.partner_archives is immutable during its retention period';
				END;
			$$ LANGUAGE plpgsql`,
			`DROP TRIGGER IF EXISTS compliance_partner_archives_immutable ON compliance.partner_archives`,
			`CREATE TRIGGER compliance_partner_archives_immutable
				BEFORE UPDATE OR DELETE ON compliance.partner_archives
				FOR EACH ROW EXECUTE FUNCTION compliance.reject_archive_mutation()`,
		},
	}
}

func validComplianceIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return true
}

func complianceRowsTx(ctx context.Context, tx *sql.Tx, source complianceSource, partnerID string) (json.RawMessage, error) {
	if !validComplianceIdentifier(source.Schema) || !validComplianceIdentifier(source.Table) || !validComplianceIdentifier(source.PartnerID) {
		return nil, fmt.Errorf("invalid compliance source identifier")
	}
	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1
		FROM information_schema.columns
		WHERE table_schema=$1 AND table_name=$2 AND column_name=$3
	)`, source.Schema, source.Table, source.PartnerID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return json.RawMessage("[]"), nil
	}
	query := fmt.Sprintf(
		`SELECT COALESCE(jsonb_agg(to_jsonb(t)), '[]'::jsonb)
		 FROM %s.%s AS t WHERE %s=$1`,
		source.Schema, source.Table, source.PartnerID,
	)
	var raw []byte
	if err := tx.QueryRowContext(ctx, query, partnerID).Scan(&raw); err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		raw = []byte("[]")
	}
	return json.RawMessage(raw), nil
}

func compliancePayloadHash(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func (a *app) archiveComplianceTx(ctx context.Context, tx *sql.Tx, partnerID, actor, reason string) (complianceArchive, error) {
	partnerID = strings.TrimSpace(partnerID)
	actor = strings.TrimSpace(actor)
	reason = strings.TrimSpace(reason)
	if partnerID == "" {
		return complianceArchive{}, fmt.Errorf("partner_id is required")
	}
	if actor == "" {
		actor = "system"
	}
	if reason == "" {
		reason = "Partner lifecycle archived"
	}

	var displayName, legalName string
	if err := tx.QueryRowContext(ctx,
		`SELECT display_name,legal_name FROM partners.partners WHERE id=$1`, partnerID).
		Scan(&displayName, &legalName); err != nil {
		return complianceArchive{}, err
	}

	payload := map[string]any{
		"schema_version": 1,
		"partner_id":     partnerID,
		"retention_years": complianceRetentionYears,
		"excluded_sensitive_domains": []string{
			"identity.users",
			"identity.partner_users",
			"identity.sessions",
			"identity.mfa_challenges",
			"identity.platform_secrets",
			"payments.partner_profiles",
		},
	}
	for _, source := range complianceSources {
		rows, err := complianceRowsTx(ctx, tx, source, partnerID)
		if err != nil {
			return complianceArchive{}, fmt.Errorf("archive %s: %w", source.Key, err)
		}
		var decoded any
		if err := json.Unmarshal(rows, &decoded); err != nil {
			return complianceArchive{}, fmt.Errorf("archive %s decode: %w", source.Key, err)
		}
		payload[source.Key] = decoded
	}
	canonical, err := json.Marshal(payload)
	if err != nil {
		return complianceArchive{}, err
	}
	hash := compliancePayloadHash(canonical)

	_, err = tx.ExecContext(ctx, `INSERT INTO compliance.partner_archives(
			partner_id,display_name,legal_name,archive_reason,created_by,archived_at,retain_until,payload,canonical_payload,payload_sha256
		) VALUES($1,$2,$3,$4,$5,NOW(),NOW()+INTERVAL '7 years',$6::jsonb,$7,$8)
		ON CONFLICT(partner_id) DO NOTHING`,
		partnerID, displayName, legalName, reason, actor, string(canonical), string(canonical), hash)
	if err != nil {
		return complianceArchive{}, err
	}
	return a.loadComplianceArchiveTx(ctx, tx, partnerID)
}

func scanComplianceArchive(s interface{ Scan(...any) error }) (complianceArchive, error) {
	var rec complianceArchive
	var canonical string
	err := s.Scan(
		&rec.PartnerID, &rec.DisplayName, &rec.LegalName, &rec.ArchiveReason, &rec.CreatedBy,
		&rec.ArchivedAt, &rec.RetainUntil, &canonical, &rec.PayloadSHA256, &rec.CreatedAt,
	)
	rec.Payload = json.RawMessage([]byte(canonical))
	return rec, err
}

const complianceArchiveSelect = `SELECT
	partner_id,display_name,legal_name,archive_reason,created_by,archived_at,retain_until,
	canonical_payload,payload_sha256,created_at
	FROM compliance.partner_archives`

func (a *app) loadComplianceArchiveTx(ctx context.Context, tx *sql.Tx, partnerID string) (complianceArchive, error) {
	return scanComplianceArchive(tx.QueryRowContext(ctx, complianceArchiveSelect+` WHERE partner_id=$1`, strings.TrimSpace(partnerID)))
}

func (a *app) loadComplianceArchive(ctx context.Context, partnerID string) (complianceArchive, error) {
	return scanComplianceArchive(a.db.QueryRowContext(ctx, complianceArchiveSelect+` WHERE partner_id=$1`, strings.TrimSpace(partnerID)))
}

func complianceArchiveMap(rec complianceArchive, includePayload bool) map[string]any {
	integrity := "MISMATCH"
	if compliancePayloadHash(rec.Payload) == rec.PayloadSHA256 {
		integrity = "VERIFIED"
	}
	out := map[string]any{
		"partner_id":       rec.PartnerID,
		"display_name":     rec.DisplayName,
		"legal_name":       rec.LegalName,
		"archive_reason":   rec.ArchiveReason,
		"created_by":       rec.CreatedBy,
		"archived_at":      rec.ArchivedAt.UTC(),
		"retain_until":     rec.RetainUntil.UTC(),
		"retention_years":  complianceRetentionYears,
		"payload_sha256":   rec.PayloadSHA256,
		"integrity_status": integrity,
		"created_at":       rec.CreatedAt.UTC(),
		"read_only":        true,
	}
	if includePayload {
		var payload any
		if json.Unmarshal(rec.Payload, &payload) == nil {
			out["payload"] = payload
		}
	}
	return out
}

func (a *app) archives(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.APIError(w, http.StatusMethodNotAllowed, "READ_ONLY", "Compliance Archives are read-only")
		return
	}
	limit, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
	offset, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("offset")))
	if limit < 1 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	args := []any{}
	where := "TRUE"
	if q != "" {
		args = append(args, "%"+q+"%")
		where = "(partner_id ILIKE $1 OR display_name ILIKE $1 OR legal_name ILIKE $1)"
	}
	var total int
	if err := a.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM compliance.partner_archives WHERE "+where, args...).Scan(&total); err != nil {
		common.APIError(w, 500, "DB", "Could not load archive count")
		return
	}
	args = append(args, limit, offset)
	limitArg := len(args) - 1
	offsetArg := len(args)
	query := `SELECT partner_id,display_name,legal_name,archive_reason,created_by,archived_at,retain_until,
		canonical_payload,payload_sha256,created_at
		FROM compliance.partner_archives WHERE ` + where +
		" ORDER BY archived_at DESC,partner_id LIMIT $" + strconv.Itoa(limitArg) + " OFFSET $" + strconv.Itoa(offsetArg)
	rows, err := a.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		common.APIError(w, 500, "DB", "Could not load Compliance Archives")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		rec, scanErr := scanComplianceArchive(rows)
		if scanErr != nil {
			common.APIError(w, 500, "DB", "Could not read Compliance Archives")
			return
		}
		items = append(items, complianceArchiveMap(rec, false))
	}
	common.JSON(w, 200, map[string]any{"items": items, "count": len(items), "total": total, "offset": offset, "limit": limit})
}

func (a *app) archiveByPartner(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.APIError(w, http.StatusMethodNotAllowed, "READ_ONLY", "Compliance Archives are read-only")
		return
	}
	partnerID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/internal/v1/archives/"), "/")
	if partnerID == "" || strings.Contains(partnerID, "/") {
		common.APIError(w, 404, "NOT_FOUND", "Compliance Archive not found")
		return
	}
	rec, err := a.loadComplianceArchive(r.Context(), partnerID)
	if err != nil {
		if err == sql.ErrNoRows {
			common.APIError(w, 404, "NOT_FOUND", "Compliance Archive not found")
			return
		}
		common.APIError(w, 500, "DB", "Could not load Compliance Archive")
		return
	}
	common.JSON(w, 200, complianceArchiveMap(rec, true))
}
