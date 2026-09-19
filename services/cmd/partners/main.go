package main

import (
	"context"
	"database/sql"
	"fmt"
	"himate.local/services/internal/common"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

type app struct{ db *sql.DB }

type partner struct {
	ID, Slug, DisplayName, LegalName, CategoryID, CategoryName string
	Lifecycle, PrimaryDomain, StagingDomain                    string
	ContactName, ContactEmail, Country, Notes                  string
	ExistingPartner, ReferencePartner                          bool
	CreatedAt, UpdatedAt                                       time.Time
}

var lifecycleValues = map[string]bool{
	"PROSPECT": true, "LICENSE_PENDING": true, "READY_TO_PROVISION": true,
	"PROVISIONING": true, "CONFIGURATION": true, "TESTING": true,
	"READY_FOR_LAUNCH": true, "LIVE": true, "SUSPENDED": true, "ARCHIVED": true,
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

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
		common.JSON(w, 200, map[string]any{"status": "ok", "service": "partners"})
	})
	mux.HandleFunc("/api/v1/partner-categories", a.categories)
	mux.HandleFunc("/api/v1/partners", a.partners)
	mux.HandleFunc("/api/v1/partners/", a.partnerByID)
	common.Run(log, "partners", common.Env("PORT", "10000"), common.InternalAuth(os.Getenv("HIMATE_INTERNAL_TOKEN"), mux))
}

func (a *app) migrate(ctx context.Context) error {
	if err := common.ExecStatements(ctx, a.db,
		`CREATE SCHEMA IF NOT EXISTS partners`,
		`CREATE TABLE IF NOT EXISTS partners.categories(
            id TEXT PRIMARY KEY,
            name TEXT UNIQUE NOT NULL,
            slug TEXT UNIQUE NOT NULL,
            system BOOLEAN NOT NULL DEFAULT FALSE
        )`,
		`CREATE TABLE IF NOT EXISTS partners.partners(
            id TEXT PRIMARY KEY,
            slug TEXT UNIQUE NOT NULL,
            display_name TEXT NOT NULL,
            legal_name TEXT NOT NULL,
            category_id TEXT REFERENCES partners.categories(id),
            lifecycle TEXT NOT NULL,
            existing_partner BOOLEAN NOT NULL DEFAULT FALSE,
            reference_partner BOOLEAN NOT NULL DEFAULT FALSE,
            primary_domain TEXT NOT NULL DEFAULT '',
            staging_domain TEXT NOT NULL DEFAULT '',
            contact_name TEXT NOT NULL DEFAULT '',
            contact_email TEXT NOT NULL DEFAULT '',
            country TEXT NOT NULL DEFAULT '',
            notes TEXT NOT NULL DEFAULT '',
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        )`,
		`CREATE SEQUENCE IF NOT EXISTS partners.partner_seq START 2`,
	); err != nil {
		return err
	}

	defaults := []string{"Classical Music", "Fine Art", "Gallery", "Theatre", "Cultural Organization", "Other"}
	for i, name := range defaults {
		if _, err := a.db.ExecContext(ctx,
			`INSERT INTO partners.categories(id,name,slug,system) VALUES($1,$2,$3,TRUE)
             ON CONFLICT(id) DO UPDATE SET name=EXCLUDED.name,slug=EXCLUDED.slug,system=TRUE`,
			fmt.Sprintf("cat_%03d", i+1), name, slugify(name)); err != nil {
			return err
		}
	}
	_, err := a.db.ExecContext(ctx,
		`INSERT INTO partners.partners(id,slug,display_name,legal_name,category_id,lifecycle,existing_partner,reference_partner,primary_domain,country,notes)
         VALUES('ptr_000001','klavierhaus','Klavierhaus','Klavierhaus','cat_001','LIVE',TRUE,TRUE,'klavierhaus.com','United States','Reference partner; activation fee not applicable.')
         ON CONFLICT(id) DO UPDATE SET reference_partner=TRUE,existing_partner=TRUE`)
	return err
}

func (a *app) categories(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query(`SELECT id,name,slug,system FROM partners.categories ORDER BY system DESC,name`)
		if err != nil {
			common.APIError(w, 500, "DB", "Could not load categories")
			return
		}
		defer rows.Close()
		items := []map[string]any{}
		for rows.Next() {
			var id, name, slug string
			var system bool
			if err := rows.Scan(&id, &name, &slug, &system); err != nil {
				continue
			}
			items = append(items, map[string]any{"id": id, "name": name, "slug": slug, "system": system})
		}
		common.JSON(w, 200, map[string]any{"items": items})
	case http.MethodPost:
		var in struct {
			Name string `json:"name"`
		}
		if common.Decode(r, &in) != nil || strings.TrimSpace(in.Name) == "" {
			common.APIError(w, 400, "VALIDATION", "Category name is required")
			return
		}
		name := strings.TrimSpace(in.Name)
		id := "cat_custom_" + slugify(name)
		_, err := a.db.Exec(`INSERT INTO partners.categories(id,name,slug,system) VALUES($1,$2,$3,FALSE) ON CONFLICT(name) DO NOTHING`, id, name, slugify(name))
		if err != nil {
			common.APIError(w, 409, "CONFLICT", "Category could not be created")
			return
		}
		common.JSON(w, 201, map[string]any{"id": id, "name": name, "slug": slugify(name), "system": false})
	default:
		common.APIError(w, 405, "METHOD", "Use GET or POST")
	}
}

func (a *app) partners(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query(selectPartner + ` ORDER BY p.reference_partner DESC,p.display_name`)
		if err != nil {
			common.APIError(w, 500, "DB", "Could not load partners")
			return
		}
		defer rows.Close()
		items := []map[string]any{}
		for rows.Next() {
			if p, err := scanPartner(rows); err == nil {
				items = append(items, partnerMap(p))
			}
		}
		common.JSON(w, 200, map[string]any{"items": items, "count": len(items)})
	case http.MethodPost:
		var in struct {
			DisplayName   string `json:"display_name"`
			LegalName     string `json:"legal_name"`
			CategoryID    string `json:"category_id"`
			Lifecycle     string `json:"lifecycle"`
			PrimaryDomain string `json:"primary_domain"`
			ContactName   string `json:"contact_name"`
			ContactEmail  string `json:"contact_email"`
			Country       string `json:"country"`
		}
		if common.Decode(r, &in) != nil || strings.TrimSpace(in.DisplayName) == "" {
			common.APIError(w, 400, "VALIDATION", "Display name is required")
			return
		}
		if in.LegalName == "" {
			in.LegalName = in.DisplayName
		}
		if in.CategoryID == "" {
			in.CategoryID = "cat_006"
		}
		if in.Lifecycle == "" {
			in.Lifecycle = "PROSPECT"
		}
		if !lifecycleValues[in.Lifecycle] {
			common.APIError(w, 400, "VALIDATION", "Invalid lifecycle")
			return
		}
		var seq int64
		if err := a.db.QueryRow(`SELECT nextval('partners.partner_seq')`).Scan(&seq); err != nil {
			common.APIError(w, 500, "DB", "Could not allocate partner ID")
			return
		}
		id := fmt.Sprintf("ptr_%06d", seq)
		_, err := a.db.Exec(`INSERT INTO partners.partners(id,slug,display_name,legal_name,category_id,lifecycle,primary_domain,contact_name,contact_email,country)
            VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, id, slugify(in.DisplayName), in.DisplayName, in.LegalName, in.CategoryID, in.Lifecycle, in.PrimaryDomain, in.ContactName, in.ContactEmail, in.Country)
		if err != nil {
			common.APIError(w, 409, "CONFLICT", "Partner could not be created")
			return
		}
		p, _ := a.get(id)
		common.JSON(w, 201, partnerMap(p))
	default:
		common.APIError(w, 405, "METHOD", "Use GET or POST")
	}
}

func (a *app) partnerByID(w http.ResponseWriter, r *http.Request) {
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/partners/"), "/")
	if id == "" {
		common.APIError(w, 404, "NOT_FOUND", "Partner not found")
		return
	}
	switch r.Method {
	case http.MethodGet:
		p, err := a.get(id)
		if err != nil {
			common.APIError(w, 404, "NOT_FOUND", "Partner not found")
			return
		}
		common.JSON(w, 200, partnerMap(p))
	case http.MethodPatch:
		p, err := a.get(id)
		if err != nil {
			common.APIError(w, 404, "NOT_FOUND", "Partner not found")
			return
		}
		var in map[string]any
		if common.Decode(r, &in) != nil {
			common.APIError(w, 400, "JSON", "Invalid request")
			return
		}
		set := func(key string, dst *string) {
			if v, ok := in[key].(string); ok {
				*dst = strings.TrimSpace(v)
			}
		}
		set("display_name", &p.DisplayName)
		set("legal_name", &p.LegalName)
		set("category_id", &p.CategoryID)
		set("lifecycle", &p.Lifecycle)
		set("primary_domain", &p.PrimaryDomain)
		set("staging_domain", &p.StagingDomain)
		set("contact_name", &p.ContactName)
		set("contact_email", &p.ContactEmail)
		set("country", &p.Country)
		set("notes", &p.Notes)
		if !lifecycleValues[p.Lifecycle] {
			common.APIError(w, 400, "VALIDATION", "Invalid lifecycle")
			return
		}
		_, err = a.db.Exec(`UPDATE partners.partners SET display_name=$2,legal_name=$3,category_id=$4,lifecycle=$5,primary_domain=$6,staging_domain=$7,contact_name=$8,contact_email=$9,country=$10,notes=$11,updated_at=NOW() WHERE id=$1`, id, p.DisplayName, p.LegalName, p.CategoryID, p.Lifecycle, p.PrimaryDomain, p.StagingDomain, p.ContactName, p.ContactEmail, p.Country, p.Notes)
		if err != nil {
			common.APIError(w, 500, "DB", "Partner could not be updated")
			return
		}
		p, _ = a.get(id)
		common.JSON(w, 200, partnerMap(p))
	default:
		common.APIError(w, 405, "METHOD", "Use GET or PATCH")
	}
}

const selectPartner = `SELECT p.id,p.slug,p.display_name,p.legal_name,p.category_id,COALESCE(c.name,''),p.lifecycle,p.existing_partner,p.reference_partner,p.primary_domain,p.staging_domain,p.contact_name,p.contact_email,p.country,p.notes,p.created_at,p.updated_at FROM partners.partners p LEFT JOIN partners.categories c ON c.id=p.category_id`

type scanner interface{ Scan(...any) error }

func scanPartner(s scanner) (partner, error) {
	var p partner
	err := s.Scan(&p.ID, &p.Slug, &p.DisplayName, &p.LegalName, &p.CategoryID, &p.CategoryName, &p.Lifecycle, &p.ExistingPartner, &p.ReferencePartner, &p.PrimaryDomain, &p.StagingDomain, &p.ContactName, &p.ContactEmail, &p.Country, &p.Notes, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}
func (a *app) get(id string) (partner, error) {
	return scanPartner(a.db.QueryRow(selectPartner+` WHERE p.id=$1`, id))
}
func partnerMap(p partner) map[string]any {
	return map[string]any{"id": p.ID, "slug": p.Slug, "display_name": p.DisplayName, "legal_name": p.LegalName, "category_id": p.CategoryID, "category_name": p.CategoryName, "lifecycle": p.Lifecycle, "existing_partner": p.ExistingPartner, "reference_partner": p.ReferencePartner, "primary_domain": p.PrimaryDomain, "staging_domain": p.StagingDomain, "contact_name": p.ContactName, "contact_email": p.ContactEmail, "country": p.Country, "notes": p.Notes, "created_at": p.CreatedAt, "updated_at": p.UpdatedAt}
}
func slugify(v string) string {
	s := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(strings.TrimSpace(v)), "-"), "-")
	if s == "" {
		return "custom"
	}
	return s
}
