package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"himate.local/services/internal/common"
)

var designColorPattern = regexp.MustCompile("^#[0-9A-Fa-f]{6}$")
var designFonts = map[string]bool{
	"Cormorant Garamond": true,
	"Inter":              true,
	"Georgia":            true,
	"Arial":              true,
}

func defaultSiteDesign() siteDesign {
	return siteDesign{
		Navy:         "#06172C",
		Gold:         "#D7AE62",
		Background:   "#F8F9FB",
		TextColor:    "#1F2937",
		HeadingFont:  "Cormorant Garamond",
		BodyFont:     "Inter",
		ButtonRadius: 6,
		Navigation: []navigationItem{
			{LabelEN: "Platform", LabelHU: "Platform", URL: "/platform", Visible: true, SortOrder: 10},
			{LabelEN: "Modules", LabelHU: "Modulok", URL: "/modules", Visible: true, SortOrder: 20},
			{LabelEN: "Programs", LabelHU: "Programok", URL: "/programs", Visible: true, SortOrder: 30},
			{LabelEN: "Impact", LabelHU: "Hatás", URL: "/impact", Visible: true, SortOrder: 40},
			{LabelEN: "Partners", LabelHU: "Partnerek", URL: "/partners", Visible: true, SortOrder: 50},
			{LabelEN: "Contact", LabelHU: "Kapcsolat", URL: "/contact", Visible: true, SortOrder: 60},
		},
	}
}

func normalizeSiteDesign(in siteDesign) siteDesign {
	in.LogoMediaAssetID = strings.TrimSpace(in.LogoMediaAssetID)
	in.Navy = strings.ToUpper(strings.TrimSpace(in.Navy))
	in.Gold = strings.ToUpper(strings.TrimSpace(in.Gold))
	in.Background = strings.ToUpper(strings.TrimSpace(in.Background))
	in.TextColor = strings.ToUpper(strings.TrimSpace(in.TextColor))
	in.HeadingFont = strings.TrimSpace(in.HeadingFont)
	in.BodyFont = strings.TrimSpace(in.BodyFont)
	for i := range in.Navigation {
		item := &in.Navigation[i]
		item.LabelEN = strings.TrimSpace(item.LabelEN)
		item.LabelHU = strings.TrimSpace(item.LabelHU)
		item.URL = strings.TrimSpace(item.URL)
	}
	sort.SliceStable(in.Navigation, func(i, j int) bool {
		return in.Navigation[i].SortOrder < in.Navigation[j].SortOrder
	})
	return in
}

func (a *app) validateSiteDesign(ctx context.Context, in siteDesign) error {
	in = normalizeSiteDesign(in)
	if !designColorPattern.MatchString(in.Navy) ||
		!designColorPattern.MatchString(in.Gold) ||
		!designColorPattern.MatchString(in.Background) ||
		!designColorPattern.MatchString(in.TextColor) {
		return fmt.Errorf("brand colors must use six-digit hexadecimal values")
	}
	if !designFonts[in.HeadingFont] || !designFonts[in.BodyFont] {
		return fmt.Errorf("unsupported design font")
	}
	if in.ButtonRadius < 0 || in.ButtonRadius > 40 {
		return fmt.Errorf("button radius must be between 0 and 40")
	}
	if in.LogoMediaAssetID != "" && !a.mediaExists(ctx, in.LogoMediaAssetID) {
		return fmt.Errorf("logo media asset does not exist")
	}
	if len(in.Navigation) > 12 {
		return fmt.Errorf("navigation supports at most 12 items")
	}
	seen := map[int]bool{}
	for i, item := range in.Navigation {
		if item.LabelEN == "" || item.LabelHU == "" {
			return fmt.Errorf("navigation item %d requires English and Hungarian labels", i+1)
		}
		if len(item.LabelEN) > 80 || len(item.LabelHU) > 80 {
			return fmt.Errorf("navigation labels are too long")
		}
		if item.URL == "" || !safeCTA(item.URL) {
			return fmt.Errorf("navigation item %d has an invalid URL", i+1)
		}
		if item.SortOrder < 0 || seen[item.SortOrder] {
			return fmt.Errorf("navigation sort_order values must be unique and non-negative")
		}
		seen[item.SortOrder] = true
	}
	return nil
}

func (a *app) readSiteDesign() (siteDesign, siteDesign, int, string, time.Time, sql.NullTime, error) {
	// Keep draft and published defaults fully independent. siteDesign contains a
	// Navigation slice, so copying one default struct into both states would
	// alias the same backing array and allow one JSON unmarshal to overwrite
	// the other state's navigation.
	draft := defaultSiteDesign()
	published := defaultSiteDesign()
	var draftRaw, publishedRaw []byte
	var version int
	var updatedBy string
	var updatedAt time.Time
	var publishedAt sql.NullTime
	err := a.db.QueryRow("SELECT draft,published,version,updated_by,updated_at,published_at FROM cms.site_design WHERE id=1").
		Scan(&draftRaw, &publishedRaw, &version, &updatedBy, &updatedAt, &publishedAt)
	if err == sql.ErrNoRows {
		return draft, published, 0, "", time.Time{}, sql.NullTime{}, nil
	}
	if err != nil {
		return draft, published, 0, "", time.Time{}, sql.NullTime{}, err
	}
	if len(draftRaw) > 2 {
		_ = json.Unmarshal(draftRaw, &draft)
	}
	if len(publishedRaw) > 2 {
		_ = json.Unmarshal(publishedRaw, &published)
	}
	return normalizeSiteDesign(draft), normalizeSiteDesign(published), version, updatedBy, updatedAt, publishedAt, nil
}

func designPayload(draft, published siteDesign, version int, updatedBy string, updatedAt time.Time, publishedAt sql.NullTime) map[string]any {
	var publishedValue any
	if publishedAt.Valid {
		publishedValue = publishedAt.Time.UTC()
	}
	return map[string]any{
		"draft":        draft,
		"published":    published,
		"version":      version,
		"updated_by":   updatedBy,
		"updated_at":   updatedAt,
		"published_at": publishedValue,
	}
}

func (a *app) design(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use GET")
		return
	}
	draft, published, version, updatedBy, updatedAt, publishedAt, err := a.readSiteDesign()
	if err != nil {
		common.APIError(w, http.StatusInternalServerError, "DB", "Could not load site design")
		return
	}
	common.JSON(w, http.StatusOK, designPayload(draft, published, version, updatedBy, updatedAt, publishedAt))
}

func (a *app) designAction(w http.ResponseWriter, r *http.Request) {
	action := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/cms/design/"), "/")
	switch action {
	case "draft":
		if r.Method != http.MethodPut {
			common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use PUT")
			return
		}
		var in siteDesign
		if common.Decode(r, &in) != nil {
			common.APIError(w, http.StatusBadRequest, "JSON", "Invalid request")
			return
		}
		in = normalizeSiteDesign(in)
		if err := a.validateSiteDesign(r.Context(), in); err != nil {
			common.APIError(w, http.StatusBadRequest, "VALIDATION", err.Error())
			return
		}
		oldDraft, oldPublished, version, _, _, _, err := a.readSiteDesign()
		if err != nil {
			common.APIError(w, http.StatusInternalServerError, "DB", "Could not load site design")
			return
		}
		query := "INSERT INTO cms.site_design(id,draft,published,version,updated_by,updated_at) " +
			"VALUES(1,$1::jsonb,$2::jsonb,$3,$4,NOW()) " +
			"ON CONFLICT(id) DO UPDATE SET draft=EXCLUDED.draft,updated_by=EXCLUDED.updated_by,updated_at=NOW()"
		_, err = a.db.ExecContext(r.Context(), query, string(jsonBytes(in)), string(jsonBytes(oldPublished)), version, actor(r))
		if err != nil {
			common.APIError(w, http.StatusInternalServerError, "DB", "Could not save design draft")
			return
		}
		_ = a.audit(r.Context(), "", "", "DESIGN_DRAFT_SAVED", actor(r), correlationID(r), oldDraft, in)
		draft, published, nextVersion, updatedBy, updatedAt, publishedAt, _ := a.readSiteDesign()
		common.JSON(w, http.StatusOK, designPayload(draft, published, nextVersion, updatedBy, updatedAt, publishedAt))
	case "publish":
		if r.Method != http.MethodPost {
			common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use POST")
			return
		}
		draft, published, version, _, _, _, err := a.readSiteDesign()
		if err != nil {
			common.APIError(w, http.StatusInternalServerError, "DB", "Could not load site design")
			return
		}
		if err := a.validateSiteDesign(r.Context(), draft); err != nil {
			common.APIError(w, http.StatusBadRequest, "VALIDATION", err.Error())
			return
		}
		query := "INSERT INTO cms.site_design(id,draft,published,version,updated_by,updated_at,published_at) " +
			"VALUES(1,$1::jsonb,$1::jsonb,$2,$3,NOW(),NOW()) " +
			"ON CONFLICT(id) DO UPDATE SET published=EXCLUDED.published,version=EXCLUDED.version," +
			"updated_by=EXCLUDED.updated_by,updated_at=NOW(),published_at=NOW()"
		_, err = a.db.ExecContext(r.Context(), query, string(jsonBytes(draft)), version+1, actor(r))
		if err != nil {
			common.APIError(w, http.StatusInternalServerError, "DB", "Could not publish site design")
			return
		}
		_ = a.audit(r.Context(), "", "", "DESIGN_PUBLISHED", actor(r), correlationID(r), published, draft)
		nextDraft, nextPublished, storedVersion, updatedBy, updatedAt, publishedAt, _ := a.readSiteDesign()
		common.JSON(w, http.StatusOK, designPayload(nextDraft, nextPublished, storedVersion, updatedBy, updatedAt, publishedAt))
	default:
		common.APIError(w, http.StatusNotFound, "NOT_FOUND", "Design action not found")
	}
}

func (a *app) publicDesign(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use GET or HEAD")
		return
	}
	_, published, version, _, _, publishedAt, err := a.readSiteDesign()
	if err != nil {
		common.APIError(w, http.StatusInternalServerError, "DB", "Could not load public site design")
		return
	}
	var publishedValue any
	if publishedAt.Valid {
		publishedValue = publishedAt.Time.UTC()
	}
	w.Header().Set("Cache-Control", "public, max-age=60, stale-while-revalidate=300")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	common.JSON(w, http.StatusOK, map[string]any{"version": version, "design": published, "published_at": publishedValue})
}
