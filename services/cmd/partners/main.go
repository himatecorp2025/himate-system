package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"himate.local/services/internal/common"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type app struct {
	db          *sql.DB
	billingHost string
	token       string
	client      *http.Client
}

type partner struct {
	ID, Slug, DisplayName, LegalName, BrandName, CategoryID, CategoryName string
	Lifecycle, PrimaryDomain, StagingDomain, LogoURL, PlatformVersion       string
	SystemHealth, ContactName, ContactEmail, FinanceContactName             string
	FinanceContactEmail, TechnicalContactName, TechnicalContactEmail        string
	MarketingContactName, MarketingContactEmail, RegistrationNumber         string
	TaxID, Country, StateRegion, City, PostalCode, AddressLine1              string
	AddressLine2, Website, Phone, Notes                                      string
	ExistingPartner, ReferencePartner, TestPartner                                       bool
	HealthCheckedAt, LastSyncAt                                              sql.NullTime
	CreatedAt, UpdatedAt                                                     time.Time
}

var lifecycleValues = map[string]bool{
	"PROSPECT": true, "LICENSE_PENDING": true, "READY_TO_PROVISION": true,
	"PROVISIONING": true, "CONFIGURATION": true, "TESTING": true,
	"READY_FOR_LAUNCH": true, "LIVE": true, "SUSPENDED": true, "ARCHIVED": true,
}

var lifecycleTransitions = map[string]map[string]bool{
	"PROSPECT":            {"LICENSE_PENDING": true, "SUSPENDED": true, "ARCHIVED": true},
	"LICENSE_PENDING":     {"PROSPECT": true, "READY_TO_PROVISION": true, "SUSPENDED": true, "ARCHIVED": true},
	"READY_TO_PROVISION":  {"LICENSE_PENDING": true, "PROVISIONING": true, "SUSPENDED": true, "ARCHIVED": true},
	"PROVISIONING":        {"READY_TO_PROVISION": true, "CONFIGURATION": true, "SUSPENDED": true},
	"CONFIGURATION":       {"PROVISIONING": true, "TESTING": true, "SUSPENDED": true},
	"TESTING":             {"CONFIGURATION": true, "READY_FOR_LAUNCH": true, "SUSPENDED": true},
	"READY_FOR_LAUNCH":    {"TESTING": true, "LIVE": true, "SUSPENDED": true},
	"LIVE":                {"SUSPENDED": true, "ARCHIVED": true},
	"SUSPENDED":           {"PROSPECT": true, "LICENSE_PENDING": true, "READY_TO_PROVISION": true, "PROVISIONING": true, "CONFIGURATION": true, "TESTING": true, "READY_FOR_LAUNCH": true, "LIVE": true, "ARCHIVED": true},
	"ARCHIVED":            {},
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

	a := &app{
		db: db,
		billingHost: strings.TrimSpace(os.Getenv("BILLING_HOSTPORT")),
		token: strings.TrimSpace(os.Getenv("HIMATE_INTERNAL_TOKEN")),
		client: &http.Client{Timeout: 4 * time.Second},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := a.migrate(ctx); err != nil {
		log.Error("migration", "error", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		common.JSON(w, 200, map[string]any{"status": "ok", "service": "partners", "time": time.Now().UTC()})
	})
	mux.HandleFunc("/api/v1/partner-categories", a.categories)
	mux.HandleFunc("/api/v1/partners", a.partners)
	mux.HandleFunc("/api/v1/partners/", a.partnerByID)
	mux.HandleFunc("/api/v1/archives", a.archives)
	mux.HandleFunc("/api/v1/archives/", a.archiveByPartner)
	common.Run(log, "partners", common.Env("PORT", "10000"), common.InternalAuth(os.Getenv("HIMATE_INTERNAL_TOKEN"), mux))
}

func (a *app) migrate(ctx context.Context) error {
	if err := common.ApplyMigrations(ctx, a.db, "partners", []common.Migration{
		{Version: 1, Name: "partners-base", Statements: []string{
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
		}},
		{Version: 2, Name: "partner-company-and-portfolio-metadata", Statements: []string{
			`ALTER TABLE partners.partners ADD COLUMN IF NOT EXISTS brand_name TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE partners.partners ADD COLUMN IF NOT EXISTS registration_number TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE partners.partners ADD COLUMN IF NOT EXISTS tax_id TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE partners.partners ADD COLUMN IF NOT EXISTS state_region TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE partners.partners ADD COLUMN IF NOT EXISTS city TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE partners.partners ADD COLUMN IF NOT EXISTS postal_code TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE partners.partners ADD COLUMN IF NOT EXISTS address_line1 TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE partners.partners ADD COLUMN IF NOT EXISTS address_line2 TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE partners.partners ADD COLUMN IF NOT EXISTS website TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE partners.partners ADD COLUMN IF NOT EXISTS phone TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE partners.partners ADD COLUMN IF NOT EXISTS finance_contact_name TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE partners.partners ADD COLUMN IF NOT EXISTS finance_contact_email TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE partners.partners ADD COLUMN IF NOT EXISTS technical_contact_name TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE partners.partners ADD COLUMN IF NOT EXISTS technical_contact_email TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE partners.partners ADD COLUMN IF NOT EXISTS marketing_contact_name TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE partners.partners ADD COLUMN IF NOT EXISTS marketing_contact_email TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE partners.partners ADD COLUMN IF NOT EXISTS logo_url TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE partners.partners ADD COLUMN IF NOT EXISTS platform_version TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE partners.partners ADD COLUMN IF NOT EXISTS system_health TEXT NOT NULL DEFAULT 'UNKNOWN'`,
			`ALTER TABLE partners.partners ADD COLUMN IF NOT EXISTS health_checked_at TIMESTAMPTZ`,
			`ALTER TABLE partners.partners ADD COLUMN IF NOT EXISTS last_sync_at TIMESTAMPTZ`,
			`CREATE TABLE IF NOT EXISTS partners.lifecycle_history(
				id BIGSERIAL PRIMARY KEY,
				partner_id TEXT NOT NULL REFERENCES partners.partners(id),
				from_state TEXT NOT NULL,
				to_state TEXT NOT NULL,
				changed_by TEXT NOT NULL DEFAULT '',
				reason TEXT NOT NULL DEFAULT '',
				changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE INDEX IF NOT EXISTS partners_lifecycle_idx ON partners.partners(lifecycle)`,
			`CREATE INDEX IF NOT EXISTS partners_category_idx ON partners.partners(category_id)`,
			`CREATE INDEX IF NOT EXISTS partners_display_name_idx ON partners.partners(display_name)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS partners_primary_domain_unique ON partners.partners((lower(primary_domain))) WHERE primary_domain<>''`,
			`CREATE UNIQUE INDEX IF NOT EXISTS partners_staging_domain_unique ON partners.partners((lower(staging_domain))) WHERE staging_domain<>''`,
		}},
		{Version: 3, Name: "partner-fast-read-indexes", Statements: []string{
			`CREATE EXTENSION IF NOT EXISTS pg_trgm`,
			`CREATE INDEX IF NOT EXISTS partners_health_idx ON partners.partners(system_health)`,
			`CREATE INDEX IF NOT EXISTS partners_active_list_idx ON partners.partners(reference_partner DESC,display_name,id) WHERE lifecycle<>'ARCHIVED'`,
			`CREATE INDEX IF NOT EXISTS partners_display_name_trgm_idx ON partners.partners USING gin(display_name gin_trgm_ops)`,
			`CREATE INDEX IF NOT EXISTS partners_legal_name_trgm_idx ON partners.partners USING gin(legal_name gin_trgm_ops)`,
			`CREATE INDEX IF NOT EXISTS partners_primary_domain_trgm_idx ON partners.partners USING gin(primary_domain gin_trgm_ops)`,
		}},
		{Version: 4, Name: "start-23-5-bilingual-partner-categories", Statements: []string{
			`ALTER TABLE partners.categories ADD COLUMN IF NOT EXISTS name_en TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE partners.categories ADD COLUMN IF NOT EXISTS name_hu TEXT NOT NULL DEFAULT ''`,
			`UPDATE partners.categories SET name_en=name WHERE name_en=''`,
			`UPDATE partners.categories SET name_hu=name WHERE name_hu=''`,
			`CREATE UNIQUE INDEX IF NOT EXISTS partners_categories_name_en_unique ON partners.categories(lower(name_en)) WHERE name_en<>''`,
			`CREATE INDEX IF NOT EXISTS partners_categories_name_hu_idx ON partners.categories(lower(name_hu)) WHERE name_hu<>''`,
		}},
		{Version: 6, Name: "retire-fixed-manual-qa-partner", AllowDestructiveSchema: true, Statements: []string{
			`DELETE FROM partners.partners
			  WHERE id='ptr_himate_test_001'
			     OR slug='himate-test-partner'
			     OR lower(contact_email)='test.partner@himate.test'`,
		}},
		{Version: 7, Name: "start-23-11-3h-idempotent-partner-onboarding", Statements: []string{
			`ALTER TABLE partners.partners ADD COLUMN IF NOT EXISTS onboarding_request_id TEXT NOT NULL DEFAULT ''`,
			`CREATE UNIQUE INDEX IF NOT EXISTS partners_onboarding_request_unique
				ON partners.partners(onboarding_request_id) WHERE onboarding_request_id<>''`,
		}},
		{Version: 8, Name: "start-23-11-3j-golden-test-partner", Statements: []string{
			`ALTER TABLE partners.partners ADD COLUMN IF NOT EXISTS test_partner BOOLEAN NOT NULL DEFAULT FALSE`,
			`CREATE INDEX IF NOT EXISTS partners_test_partner_idx ON partners.partners(test_partner) WHERE test_partner=TRUE`,
		}},
		complianceArchiveMigration(),
	}); err != nil {
		return err
	}

	defaults := []struct{ EN, HU string }{
		{"Classical Music", "Klasszikus zene"},
		{"Fine Art", "Képzőművészet"},
		{"Gallery", "Galéria"},
		{"Theatre", "Színház"},
		{"Cultural Organization", "Kulturális szervezet"},
		{"Other", "Egyéb"},
	}
	for i, item := range defaults {
		if _, err := a.db.ExecContext(ctx,
			`INSERT INTO partners.categories(id,name,name_en,name_hu,slug,system) VALUES($1,$2,$2,$3,$4,TRUE)
			 ON CONFLICT(id) DO UPDATE SET name=EXCLUDED.name,name_en=EXCLUDED.name_en,name_hu=EXCLUDED.name_hu,slug=EXCLUDED.slug,system=TRUE`,
			fmt.Sprintf("cat_%03d", i+1), item.EN, item.HU, slugify(item.EN)); err != nil {
			return err
		}
	}
	_, err := a.db.ExecContext(ctx,
		`INSERT INTO partners.partners(
			id,slug,display_name,legal_name,brand_name,category_id,lifecycle,existing_partner,reference_partner,
			primary_domain,country,platform_version,system_health,notes
		)
		VALUES('ptr_000001','klavierhaus','Klavierhaus','Klavierhaus','Klavierhaus','cat_001','LIVE',TRUE,TRUE,
			'klavierhaus.com','United States','reference','UNKNOWN','Reference partner; activation fee not applicable.')
		ON CONFLICT(id) DO UPDATE SET reference_partner=TRUE,existing_partner=TRUE`)
	return err
}

func (a *app) categories(w http.ResponseWriter, r *http.Request) {
	locale := common.RequestLocale(r)
	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query(`SELECT id,name_en,name_hu,slug,system FROM partners.categories ORDER BY system DESC,lower(name_en),id`)
		if err != nil {
			common.APIError(w, 500, "DB", "Could not load categories")
			return
		}
		defer rows.Close()
		items := []map[string]any{}
		for rows.Next() {
			var id, nameEN, nameHU, slug string
			var system bool
			if err := rows.Scan(&id, &nameEN, &nameHU, &slug, &system); err != nil {
				continue
			}
			items = append(items, map[string]any{
				"id": id, "name": common.Localized(nameEN, nameHU, locale),
				"name_en": nameEN, "name_hu": nameHU, "slug": slug, "system": system,
			})
		}
		common.JSON(w, 200, map[string]any{"items": items, "locale": locale})
	case http.MethodPost:
		var in struct {
			Name   string `json:"name"`
			NameEN string `json:"name_en"`
			NameHU string `json:"name_hu"`
		}
		if common.Decode(r, &in) != nil {
			common.APIError(w, 400, "JSON", "Invalid request")
			return
		}
		nameEN := strings.TrimSpace(in.NameEN)
		nameHU := strings.TrimSpace(in.NameHU)
		legacy := strings.TrimSpace(in.Name)
		if nameEN == "" { nameEN = legacy }
		if nameHU == "" { nameHU = legacy }
		if nameEN == "" || nameHU == "" {
			common.APIError(w, 400, "VALIDATION", "English and Hungarian category names are required")
			return
		}
		id := "cat_custom_" + slugify(nameEN)
		slug := slugify(nameEN)
		res, err := a.db.Exec(`INSERT INTO partners.categories(id,name,name_en,name_hu,slug,system)
			VALUES($1,$2,$2,$3,$4,FALSE) ON CONFLICT(name) DO NOTHING`, id, nameEN, nameHU, slug)
		if err != nil {
			common.APIError(w, 409, "CONFLICT", "Category could not be created")
			return
		}
		if rows, _ := res.RowsAffected(); rows == 0 {
			common.APIError(w, 409, "CONFLICT", "Category already exists")
			return
		}
		common.JSON(w, 201, map[string]any{
			"id": id, "name": common.Localized(nameEN, nameHU, locale),
			"name_en": nameEN, "name_hu": nameHU, "slug": slug, "system": false,
		})
	default:
		common.APIError(w, 405, "METHOD", "Use GET or POST")
	}
}

func boundedInt(raw string, fallback, min, max int) int {
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func (a *app) loadPartnerStats(whereSQL string, args []any) (int, map[string]int, int, error) {
	var total, referenceCount int
	var lifecycleJSON []byte
	query := `SELECT
		(SELECT COUNT(*) FROM partners.partners p WHERE ` + whereSQL + `) AS filtered_total,
		(SELECT COUNT(*) FROM partners.partners WHERE reference_partner=TRUE) AS reference_count,
		COALESCE((
			SELECT jsonb_object_agg(s.lifecycle,s.cnt)
			FROM (SELECT lifecycle,COUNT(*) AS cnt FROM partners.partners GROUP BY lifecycle) s
		),'{}'::jsonb) AS lifecycle_counts`
	if err := a.db.QueryRow(query, args...).Scan(&total, &referenceCount, &lifecycleJSON); err != nil {
		return 0, nil, 0, err
	}
	lifecycleCounts := map[string]int{}
	if len(lifecycleJSON) > 0 {
		_ = json.Unmarshal(lifecycleJSON, &lifecycleCounts)
	}
	return total, lifecycleCounts, referenceCount, nil
}

func (a *app) partners(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		limit := boundedInt(r.URL.Query().Get("limit"), 100, 1, 200)
		offset := boundedInt(r.URL.Query().Get("offset"), 0, 0, 1_000_000)
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		category := strings.TrimSpace(r.URL.Query().Get("category"))
		lifecycle := strings.TrimSpace(r.URL.Query().Get("lifecycle"))
		health := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("health")))
		includeArchived, _ := strconv.ParseBool(r.URL.Query().Get("include_archived"))
		includeStats := true
		if raw := strings.TrimSpace(r.URL.Query().Get("include_stats")); raw != "" {
			if parsed, err := strconv.ParseBool(raw); err == nil { includeStats = parsed }
		}
		statsOnly, _ := strconv.ParseBool(r.URL.Query().Get("stats_only"))

		where := []string{"1=1"}
		args := []any{}
		add := func(clause string, value any) {
			args = append(args, value)
			where = append(where, fmt.Sprintf(clause, len(args)))
		}
		if q != "" {
			args = append(args, "%"+q+"%")
			n := len(args)
			where = append(where, fmt.Sprintf(`(p.display_name ILIKE $%d OR p.legal_name ILIKE $%d OR p.id ILIKE $%d OR p.primary_domain ILIKE $%d)`, n, n, n, n))
		}
		if category != "" && category != "ALL" { add(`p.category_id=$%d`, category) }
		if lifecycle != "" && lifecycle != "ALL" {
			if !lifecycleValues[lifecycle] {
				common.APIError(w, 400, "VALIDATION", "Invalid lifecycle filter")
				return
			}
			add(`p.lifecycle=$%d`, lifecycle)
		} else if !includeArchived {
			where = append(where, `p.lifecycle<>'ARCHIVED'`)
		}
		if health != "" && health != "ALL" {
			switch health {
			case "HEALTHY", "WARNING", "OFFLINE", "UNKNOWN":
				add(`p.system_health=$%d`, health)
			default:
				common.APIError(w, 400, "VALIDATION", "Invalid health filter")
				return
			}
		}

		whereSQL := strings.Join(where, " AND ")
		if statsOnly {
			total, lifecycleCounts, referenceCount, err := a.loadPartnerStats(whereSQL, args)
			if err != nil {
				common.APIError(w, 500, "DB", "Could not load partner statistics")
				return
			}
			common.JSON(w, 200, map[string]any{
				"items": []map[string]any{}, "count": 0, "total": total, "limit": limit, "offset": offset,
				"has_more": false, "lifecycle_counts": lifecycleCounts, "reference_count": referenceCount,
			})
			return
		}

		fetchLimit := limit
		if !includeStats { fetchLimit = limit + 1 }
		queryArgs := append(append([]any{}, args...), fetchLimit, offset)
		rows, err := a.db.Query(
			selectPartner+` WHERE `+whereSQL+` ORDER BY p.reference_partner DESC,p.display_name,p.id LIMIT $`+strconv.Itoa(len(args)+1)+` OFFSET $`+strconv.Itoa(len(args)+2),
			queryArgs...,
		)
		if err != nil {
			common.APIError(w, 500, "DB", "Could not load partners")
			return
		}
		defer rows.Close()
		items := []map[string]any{}
		for rows.Next() {
			if p, err := scanPartner(rows); err == nil { items = append(items, partnerMap(p)) }
		}
		hasMore := false
		if !includeStats && len(items) > limit {
			hasMore = true
			items = items[:limit]
		}
		total := offset + len(items)
		lifecycleCounts := map[string]int{}
		referenceCount := 0
		if includeStats {
			var statsErr error
			total, lifecycleCounts, referenceCount, statsErr = a.loadPartnerStats(whereSQL, args)
			if statsErr != nil {
				common.APIError(w, 500, "DB", "Could not load partner statistics")
				return
			}
			hasMore = offset+len(items) < total
		} else if hasMore {
			total++
		}
		common.JSON(w, 200, map[string]any{
			"items": items, "count": len(items), "total": total, "limit": limit, "offset": offset,
			"has_more": hasMore, "lifecycle_counts": lifecycleCounts, "reference_count": referenceCount,
		})
	case http.MethodPost:
		var in struct {
			DisplayName           string `json:"display_name"`
			LegalName             string `json:"legal_name"`
			BrandName             string `json:"brand_name"`
			CategoryID            string `json:"category_id"`
			Lifecycle             string `json:"lifecycle"`
			PrimaryDomain         string `json:"primary_domain"`
			ContactName           string `json:"contact_name"`
			ContactEmail          string `json:"contact_email"`
			FinanceContactName    string `json:"finance_contact_name"`
			FinanceContactEmail   string `json:"finance_contact_email"`
			TechnicalContactName  string `json:"technical_contact_name"`
			TechnicalContactEmail string `json:"technical_contact_email"`
			MarketingContactName  string `json:"marketing_contact_name"`
			MarketingContactEmail string `json:"marketing_contact_email"`
			RegistrationNumber    string `json:"registration_number"`
			TaxID                 string `json:"tax_id"`
			Country               string `json:"country"`
			StateRegion           string `json:"state_region"`
			City                  string `json:"city"`
			PostalCode            string `json:"postal_code"`
			AddressLine1          string `json:"address_line1"`
			AddressLine2          string `json:"address_line2"`
			Website               string `json:"website"`
			Phone                 string `json:"phone"`
			Notes                 string `json:"notes"`
			OnboardingRequestID   string `json:"onboarding_request_id"`
		}
		if err := common.Decode(r, &in); err != nil {
			common.APIError(w, 400, "JSON", "Invalid partner request: "+err.Error())
			return
		}
		in.DisplayName = strings.TrimSpace(in.DisplayName)
		if in.DisplayName == "" {
			common.APIError(w, 400, "VALIDATION", "Display name is required")
			return
		}
		in.OnboardingRequestID = strings.TrimSpace(in.OnboardingRequestID)
		if len(in.OnboardingRequestID) > 160 {
			common.APIError(w, 400, "VALIDATION", "Onboarding request ID is too long")
			return
		}
		if in.OnboardingRequestID != "" {
			var existingID string
			err := a.db.QueryRow(`SELECT id FROM partners.partners WHERE onboarding_request_id=$1`, in.OnboardingRequestID).Scan(&existingID)
			if err == nil {
				p, getErr := a.get(existingID)
				if getErr != nil {
					common.APIError(w, 500, "DB", "Could not recover existing partner onboarding request")
					return
				}
				common.JSON(w, 200, partnerMap(p))
				return
			}
			if err != sql.ErrNoRows {
				common.APIError(w, 500, "DB", "Could not validate onboarding request")
				return
			}
		}
		if strings.TrimSpace(in.LegalName) == "" {
			in.LegalName = in.DisplayName
		}
		if strings.TrimSpace(in.BrandName) == "" {
			in.BrandName = in.DisplayName
		}
		if in.CategoryID == "" {
			in.CategoryID = "cat_006"
		}
		if in.Lifecycle == "" {
			in.Lifecycle = "PROSPECT"
		}
		if !lifecycleValues[in.Lifecycle] || in.Lifecycle != "PROSPECT" {
			common.APIError(w, 400, "VALIDATION", "New partners must begin in PROSPECT")
			return
		}
		var seq int64
		if err := a.db.QueryRow(`SELECT nextval('partners.partner_seq')`).Scan(&seq); err != nil {
			common.APIError(w, 500, "DB", "Could not allocate partner ID")
			return
		}
		id := fmt.Sprintf("ptr_%06d", seq)
		slug := partnerTechnicalSlug(in.DisplayName, id)
		primaryDomain := strings.TrimSpace(in.PrimaryDomain)
		if primaryDomain != "" {
			var domainExists bool
			if err := a.db.QueryRow(`SELECT EXISTS(
				SELECT 1 FROM partners.partners WHERE lower(primary_domain)=lower($1) AND primary_domain<>''
			)`, primaryDomain).Scan(&domainExists); err != nil {
				common.APIError(w, 500, "DB", "Could not validate primary domain")
				return
			}
			if domainExists {
				common.APIError(w, 409, "PRIMARY_DOMAIN_EXISTS", "Primary domain is already assigned to another partner")
				return
			}
		}
		_, err := a.db.Exec(`INSERT INTO partners.partners(
				id,slug,display_name,legal_name,brand_name,category_id,lifecycle,primary_domain,
				contact_name,contact_email,finance_contact_name,finance_contact_email,
				technical_contact_name,technical_contact_email,marketing_contact_name,marketing_contact_email,
				registration_number,tax_id,country,state_region,city,postal_code,address_line1,address_line2,website,phone,notes,onboarding_request_id
			) VALUES(
				$1,$2,$3,$4,$5,$6,$7,$8,
				$9,$10,$11,$12,$13,$14,$15,$16,
				$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28
			)`,
			id, slug, in.DisplayName, strings.TrimSpace(in.LegalName), strings.TrimSpace(in.BrandName),
			in.CategoryID, in.Lifecycle, primaryDomain,
			strings.TrimSpace(in.ContactName), strings.ToLower(strings.TrimSpace(in.ContactEmail)),
			strings.TrimSpace(in.FinanceContactName), strings.ToLower(strings.TrimSpace(in.FinanceContactEmail)),
			strings.TrimSpace(in.TechnicalContactName), strings.ToLower(strings.TrimSpace(in.TechnicalContactEmail)),
			strings.TrimSpace(in.MarketingContactName), strings.ToLower(strings.TrimSpace(in.MarketingContactEmail)),
			strings.TrimSpace(in.RegistrationNumber), strings.TrimSpace(in.TaxID), strings.TrimSpace(in.Country),
			strings.TrimSpace(in.StateRegion), strings.TrimSpace(in.City), strings.TrimSpace(in.PostalCode),
			strings.TrimSpace(in.AddressLine1), strings.TrimSpace(in.AddressLine2), strings.TrimSpace(in.Website),
			strings.TrimSpace(in.Phone), strings.TrimSpace(in.Notes), in.OnboardingRequestID)
		if err != nil {
			if in.OnboardingRequestID != "" {
				var existingID string
				if lookupErr := a.db.QueryRow(`SELECT id FROM partners.partners WHERE onboarding_request_id=$1`, in.OnboardingRequestID).Scan(&existingID); lookupErr == nil {
					if p, getErr := a.get(existingID); getErr == nil {
						common.JSON(w, 200, partnerMap(p))
						return
					}
				}
			}
			common.APIError(w, 500, "DB", "Could not create partner")
			return
		}
		p, _ := a.get(id)
		common.JSON(w, 201, partnerMap(p))
	default:
		common.APIError(w, 405, "METHOD", "Use GET or POST")
	}
}

func canTransition(from, to string) bool {
	if from == to {
		return true
	}
	return lifecycleTransitions[from][to]
}

func requiresProvisioningGate(from, to string) bool {
	return from != to && to == "PROVISIONING"
}

func (a *app) provisioningAllowed(ctx context.Context, partnerID string) (bool, string, error) {
	if a.billingHost == "" || len(a.token) < 24 {
		return false, "", fmt.Errorf("billing service credential is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"http://"+a.billingHost+"/internal/v1/partners/"+partnerID+"/provisioning-gate", nil)
	if err != nil { return false, "", err }
	common.BindInternalRequest(req, a.token)
	resp, err := common.DoInternal(a.client, req)
	if err != nil { return false, "", err }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, "", fmt.Errorf("billing gate returned status %d", resp.StatusCode)
	}
	var gate struct {
		Allowed bool   `json:"allowed"`
		Reason  string `json:"reason"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&gate); err != nil {
		return false, "", err
	}
	return gate.Allowed, gate.Reason, nil
}

func (a *app) partnerByID(w http.ResponseWriter, r *http.Request) {
	raw := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/partners/"), "/")
	parts := strings.Split(raw, "/")
	if len(parts) == 2 && parts[0] != "" && parts[1] == "purge-operational" {
		a.purgeOperationalPartner(w, r, parts[0])
		return
	}
	if len(parts) != 1 || parts[0] == "" {
		common.APIError(w, 404, "NOT_FOUND", "Partner not found")
		return
	}
	id := parts[0]
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
		var in struct {
			DisplayName           *string `json:"display_name"`
			LegalName             *string `json:"legal_name"`
			BrandName             *string `json:"brand_name"`
			CategoryID            *string `json:"category_id"`
			Lifecycle             *string `json:"lifecycle"`
			PrimaryDomain         *string `json:"primary_domain"`
			StagingDomain         *string `json:"staging_domain"`
			ContactName           *string `json:"contact_name"`
			ContactEmail          *string `json:"contact_email"`
			FinanceContactName    *string `json:"finance_contact_name"`
			FinanceContactEmail   *string `json:"finance_contact_email"`
			TechnicalContactName  *string `json:"technical_contact_name"`
			TechnicalContactEmail *string `json:"technical_contact_email"`
			MarketingContactName  *string `json:"marketing_contact_name"`
			MarketingContactEmail *string `json:"marketing_contact_email"`
			RegistrationNumber    *string `json:"registration_number"`
			TaxID                 *string `json:"tax_id"`
			Country               *string `json:"country"`
			StateRegion           *string `json:"state_region"`
			City                  *string `json:"city"`
			PostalCode            *string `json:"postal_code"`
			AddressLine1          *string `json:"address_line1"`
			AddressLine2          *string `json:"address_line2"`
			Website               *string `json:"website"`
			Phone                 *string `json:"phone"`
			LogoURL               *string `json:"logo_url"`
			Notes                 *string `json:"notes"`
			TestPartner           *bool   `json:"test_partner"`
			Reason                string  `json:"reason"`
		}
		if common.Decode(r, &in) != nil {
			common.APIError(w, 400, "JSON", "Invalid request")
			return
		}
		set := func(src *string, dst *string) {
			if src != nil {
				*dst = strings.TrimSpace(*src)
			}
		}
		oldLifecycle := p.Lifecycle
		oldTestPartner := p.TestPartner
		set(in.DisplayName, &p.DisplayName)
		set(in.LegalName, &p.LegalName)
		set(in.BrandName, &p.BrandName)
		set(in.CategoryID, &p.CategoryID)
		set(in.Lifecycle, &p.Lifecycle)
		set(in.PrimaryDomain, &p.PrimaryDomain)
		set(in.StagingDomain, &p.StagingDomain)
		set(in.ContactName, &p.ContactName)
		set(in.ContactEmail, &p.ContactEmail)
		set(in.FinanceContactName, &p.FinanceContactName)
		set(in.FinanceContactEmail, &p.FinanceContactEmail)
		set(in.TechnicalContactName, &p.TechnicalContactName)
		set(in.TechnicalContactEmail, &p.TechnicalContactEmail)
		set(in.MarketingContactName, &p.MarketingContactName)
		set(in.MarketingContactEmail, &p.MarketingContactEmail)
		set(in.RegistrationNumber, &p.RegistrationNumber)
		set(in.TaxID, &p.TaxID)
		set(in.Country, &p.Country)
		set(in.StateRegion, &p.StateRegion)
		set(in.City, &p.City)
		set(in.PostalCode, &p.PostalCode)
		set(in.AddressLine1, &p.AddressLine1)
		set(in.AddressLine2, &p.AddressLine2)
		set(in.Website, &p.Website)
		set(in.Phone, &p.Phone)
		set(in.LogoURL, &p.LogoURL)
		set(in.Notes, &p.Notes)
		if in.TestPartner != nil {
			if oldTestPartner && !*in.TestPartner {
				common.APIError(w, 409, "GOLDEN_TEST_PARTNER_IMMUTABLE", "Golden Test Partner mode can only be removed by the dedicated test-tenant purge workflow")
				return
			}
			p.TestPartner = *in.TestPartner
			if p.TestPartner && !oldTestPartner {
				p.Lifecycle = "LIVE"
			}
		}
		if p.DisplayName == "" {
			common.APIError(w, 400, "VALIDATION", "Display name is required")
			return
		}
		p.ContactEmail = strings.ToLower(p.ContactEmail)
		p.FinanceContactEmail = strings.ToLower(p.FinanceContactEmail)
		p.TechnicalContactEmail = strings.ToLower(p.TechnicalContactEmail)
		p.MarketingContactEmail = strings.ToLower(p.MarketingContactEmail)

		if !lifecycleValues[p.Lifecycle] {
			common.APIError(w, 400, "VALIDATION", "Invalid lifecycle")
			return
		}
		goldenActivation := !oldTestPartner && p.TestPartner
		if !goldenActivation && !canTransition(oldLifecycle, p.Lifecycle) {
			common.APIError(w, 409, "INVALID_LIFECYCLE_TRANSITION", "Lifecycle transition is not allowed")
			return
		}
		if !goldenActivation && requiresProvisioningGate(oldLifecycle, p.Lifecycle) {
			allowed, reason, gateErr := a.provisioningAllowed(r.Context(), id)
			if gateErr != nil {
				common.APIError(w, 503, "BILLING_GATE_UNAVAILABLE", "Provisioning cannot start while license verification is unavailable")
				return
			}
			if !allowed {
				if strings.TrimSpace(reason) == "" { reason = "Initial license payment and evidence are required before provisioning" }
				common.APIError(w, 409, "INITIAL_LICENSE_REQUIRED", reason)
				return
			}
		}

		tx, err := a.db.BeginTx(r.Context(), &sql.TxOptions{})
		if err != nil {
			common.APIError(w, 500, "DB", "Could not start partner update")
			return
		}
		defer tx.Rollback()
		_, err = tx.Exec(`UPDATE partners.partners SET
			display_name=$2,legal_name=$3,brand_name=$4,category_id=$5,lifecycle=$6,primary_domain=$7,staging_domain=$8,
			contact_name=$9,contact_email=$10,finance_contact_name=$11,finance_contact_email=$12,
			technical_contact_name=$13,technical_contact_email=$14,marketing_contact_name=$15,marketing_contact_email=$16,
			registration_number=$17,tax_id=$18,country=$19,state_region=$20,city=$21,postal_code=$22,address_line1=$23,address_line2=$24,
			website=$25,phone=$26,logo_url=$27,notes=$28,test_partner=$29,updated_at=NOW()
			WHERE id=$1`,
			id, p.DisplayName, p.LegalName, p.BrandName, p.CategoryID, p.Lifecycle, p.PrimaryDomain, p.StagingDomain,
			p.ContactName, p.ContactEmail, p.FinanceContactName, p.FinanceContactEmail,
			p.TechnicalContactName, p.TechnicalContactEmail, p.MarketingContactName, p.MarketingContactEmail,
			p.RegistrationNumber, p.TaxID, p.Country, p.StateRegion, p.City, p.PostalCode, p.AddressLine1, p.AddressLine2,
			p.Website, p.Phone, p.LogoURL, p.Notes, p.TestPartner)
		if err != nil {
			common.APIError(w, 409, "CONFLICT", "Partner could not be updated; verify domain and category uniqueness")
			return
		}
		if oldLifecycle != p.Lifecycle {
			actor := strings.TrimSpace(r.Header.Get("X-Himate-User-ID"))
			reason := strings.TrimSpace(in.Reason)
			if reason == "" {
				if goldenActivation {
					reason = "Golden Test Partner activation"
				} else {
					reason = "HIMATE administrator lifecycle update"
				}
			}
			if _, err = tx.Exec(`INSERT INTO partners.lifecycle_history(partner_id,from_state,to_state,changed_by,reason) VALUES($1,$2,$3,$4,$5)`,
				id, oldLifecycle, p.Lifecycle, actor, reason); err != nil {
				common.APIError(w, 500, "DB", "Could not record lifecycle transition")
				return
			}
			if p.Lifecycle == "ARCHIVED" {
				if _, err = a.archiveComplianceTx(r.Context(), tx, id, actor, reason); err != nil {
					common.APIError(w, 500, "COMPLIANCE_ARCHIVE", "Could not create the immutable seven-year Compliance Archive")
					return
				}
			}
		}
		if err = tx.Commit(); err != nil {
			common.APIError(w, 500, "DB", "Could not commit partner update")
			return
		}
		p, _ = a.get(id)
		common.JSON(w, 200, partnerMap(p))
	default:
		common.APIError(w, 405, "METHOD", "Use GET or PATCH")
	}
}

func (a *app) purgeOperationalPartner(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		common.APIError(w, 405, "METHOD", "Use POST")
		return
	}
	p, err := a.get(id)
	if err != nil {
		common.APIError(w, 404, "NOT_FOUND", "Partner not found")
		return
	}
	if p.Lifecycle != "SUSPENDED" && p.Lifecycle != "ARCHIVED" {
		common.APIError(w, 409, "PARTNER_NOT_SUSPENDED", "Operational purge requires a suspended or archived partner")
		return
	}
	var in struct{ Reason string `json:"reason"` }
	_ = common.Decode(r, &in)
	reason := strings.TrimSpace(in.Reason)
	if reason == "" { reason = "Operational account purge" }
	actor := strings.TrimSpace(r.Header.Get("X-Himate-User-ID"))
	if actor == "" { actor = "system" }

	tx, err := a.db.BeginTx(r.Context(), &sql.TxOptions{})
	if err != nil {
		common.APIError(w, 500, "DB", "Could not start operational purge")
		return
	}
	defer tx.Rollback()

	deletedUsers := int64(0)
	var identityExists bool
	if err = tx.QueryRow(`SELECT to_regclass('identity.partner_users') IS NOT NULL`).Scan(&identityExists); err != nil {
		common.APIError(w, 500, "DB", "Could not inspect partner identity storage")
		return
	}
	if identityExists {
		res, deleteErr := tx.Exec(`DELETE FROM identity.partner_users WHERE partner_id=$1`, id)
		if deleteErr != nil {
			common.APIError(w, 500, "DB", "Could not purge partner login identities")
			return
		}
		deletedUsers, _ = res.RowsAffected()
	}
	if p.Lifecycle != "ARCHIVED" {
		if _, err = tx.Exec(`UPDATE partners.partners SET lifecycle='ARCHIVED',system_health='ARCHIVED',updated_at=NOW() WHERE id=$1`, id); err != nil {
			common.APIError(w, 500, "DB", "Could not archive partner")
			return
		}
		if _, err = tx.Exec(`INSERT INTO partners.lifecycle_history(partner_id,from_state,to_state,changed_by,reason)
			VALUES($1,$2,'ARCHIVED',$3,$4)`, id, p.Lifecycle, actor, reason); err != nil {
			common.APIError(w, 500, "DB", "Could not record operational purge lifecycle")
			return
		}
	}
	archive, archiveErr := a.archiveComplianceTx(r.Context(), tx, id, actor, reason)
	if archiveErr != nil {
		common.APIError(w, 500, "COMPLIANCE_ARCHIVE", "Operational purge was blocked because the seven-year Compliance Archive could not be secured")
		return
	}
	if err = tx.Commit(); err != nil {
		common.APIError(w, 500, "DB", "Could not commit operational purge")
		return
	}
	common.JSON(w, 200, map[string]any{
		"partner_id": id,
		"lifecycle": "ARCHIVED",
		"operational_identity_records_deleted": deletedUsers,
		"legal_financial_records_retained": true,
		"compliance_archive": complianceArchiveMap(archive, false),
	})
}

const selectPartner = `SELECT
	p.id,p.slug,p.display_name,p.legal_name,p.brand_name,COALESCE(p.category_id,''),COALESCE(c.name,''),p.lifecycle,
	p.existing_partner,p.reference_partner,p.test_partner,p.primary_domain,p.staging_domain,p.logo_url,p.platform_version,p.system_health,
	p.contact_name,p.contact_email,p.finance_contact_name,p.finance_contact_email,p.technical_contact_name,p.technical_contact_email,
	p.marketing_contact_name,p.marketing_contact_email,p.registration_number,p.tax_id,p.country,p.state_region,p.city,p.postal_code,
	p.address_line1,p.address_line2,p.website,p.phone,p.notes,p.health_checked_at,p.last_sync_at,p.created_at,p.updated_at
	FROM partners.partners p LEFT JOIN partners.categories c ON c.id=p.category_id`

type scanner interface{ Scan(...any) error }

func scanPartner(s scanner) (partner, error) {
	var p partner
	err := s.Scan(
		&p.ID, &p.Slug, &p.DisplayName, &p.LegalName, &p.BrandName, &p.CategoryID, &p.CategoryName, &p.Lifecycle,
		&p.ExistingPartner, &p.ReferencePartner, &p.TestPartner, &p.PrimaryDomain, &p.StagingDomain, &p.LogoURL, &p.PlatformVersion, &p.SystemHealth,
		&p.ContactName, &p.ContactEmail, &p.FinanceContactName, &p.FinanceContactEmail, &p.TechnicalContactName, &p.TechnicalContactEmail,
		&p.MarketingContactName, &p.MarketingContactEmail, &p.RegistrationNumber, &p.TaxID, &p.Country, &p.StateRegion, &p.City, &p.PostalCode,
		&p.AddressLine1, &p.AddressLine2, &p.Website, &p.Phone, &p.Notes, &p.HealthCheckedAt, &p.LastSyncAt, &p.CreatedAt, &p.UpdatedAt,
	)
	return p, err
}

func (a *app) get(id string) (partner, error) {
	return scanPartner(a.db.QueryRow(selectPartner+` WHERE p.id=$1`, id))
}

func nullableTime(v sql.NullTime) any {
	if !v.Valid {
		return nil
	}
	return v.Time.UTC()
}

func partnerMap(p partner) map[string]any {
	return map[string]any{
		"id": p.ID, "slug": p.Slug, "display_name": p.DisplayName, "legal_name": p.LegalName, "brand_name": p.BrandName,
		"category_id": p.CategoryID, "category_name": p.CategoryName, "lifecycle": p.Lifecycle,
		"existing_partner": p.ExistingPartner, "reference_partner": p.ReferencePartner, "test_partner": p.TestPartner,
		"primary_domain": p.PrimaryDomain, "staging_domain": p.StagingDomain, "logo_url": p.LogoURL,
		"platform_version": p.PlatformVersion, "system_health": p.SystemHealth,
		"health_checked_at": nullableTime(p.HealthCheckedAt), "last_sync_at": nullableTime(p.LastSyncAt),
		"contact_name": p.ContactName, "contact_email": p.ContactEmail,
		"finance_contact_name": p.FinanceContactName, "finance_contact_email": p.FinanceContactEmail,
		"technical_contact_name": p.TechnicalContactName, "technical_contact_email": p.TechnicalContactEmail,
		"marketing_contact_name": p.MarketingContactName, "marketing_contact_email": p.MarketingContactEmail,
		"registration_number": p.RegistrationNumber, "tax_id": p.TaxID, "country": p.Country, "state_region": p.StateRegion,
		"city": p.City, "postal_code": p.PostalCode, "address_line1": p.AddressLine1, "address_line2": p.AddressLine2,
		"website": p.Website, "phone": p.Phone, "notes": p.Notes, "created_at": p.CreatedAt, "updated_at": p.UpdatedAt,
	}
}

func slugify(v string) string {
	s := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(strings.TrimSpace(v)), "-"), "-")
	if s == "" {
		return "custom"
	}
	return s
}

func partnerTechnicalSlug(displayName, partnerID string) string {
	suffix := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(partnerID)), "ptr_")
	suffix = strings.Trim(nonSlug.ReplaceAllString(suffix, "-"), "-")
	if suffix == "" {
		suffix = "partner"
	}
	return slugify(displayName) + "-" + suffix
}
