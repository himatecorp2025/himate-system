package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"

	"himate.local/services/internal/common"
)

type siteSEOSettings struct {
	GlobalKeywordsEN      []string `json:"global_keywords_en"`
	GlobalKeywordsHU      []string `json:"global_keywords_hu"`
	OrganizationName      string   `json:"organization_name"`
	OrganizationURL       string   `json:"organization_url"`
	DefaultOGImageAssetID string   `json:"default_og_image_asset_id"`
}

type seoAuditIssue struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

func defaultSiteSEO() siteSEOSettings {
	return siteSEOSettings{
		GlobalKeywordsEN: []string{"arts", "culture", "cultural organizations", "HIMATE"},
		GlobalKeywordsHU: []string{"muvészet", "kultura", "kulturalis szervezetek", "HIMATE"},
		OrganizationName: "HIMATE System",
		OrganizationURL:  "https://www.himate.com",
	}
}

func normalizeKeywords(values []string, max int) []string {
	if max < 1 {
		max = 1
	}
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, raw := range values {
		value := strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
		if len(out) >= max {
			break
		}
	}
	return out
}

func validateKeywords(values []string, max int) error {
	if len(values) > max {
		return fmt.Errorf("at most %d keywords are allowed", max)
	}
	for _, value := range values {
		if len([]rune(strings.TrimSpace(value))) > 80 {
			return fmt.Errorf("keywords must be at most 80 characters")
		}
	}
	return nil
}

func normalizeSiteSEO(in siteSEOSettings) siteSEOSettings {
	in.GlobalKeywordsEN = normalizeKeywords(in.GlobalKeywordsEN, 30)
	in.GlobalKeywordsHU = normalizeKeywords(in.GlobalKeywordsHU, 30)
	in.OrganizationName = strings.TrimSpace(in.OrganizationName)
	in.OrganizationURL = strings.TrimSpace(in.OrganizationURL)
	in.DefaultOGImageAssetID = strings.TrimSpace(in.DefaultOGImageAssetID)
	return in
}

func (a *app) validateSiteSEO(ctx context.Context, in siteSEOSettings) error {
	in = normalizeSiteSEO(in)
	if err := validateKeywords(in.GlobalKeywordsEN, 30); err != nil {
		return fmt.Errorf("English global keywords: %w", err)
	}
	if err := validateKeywords(in.GlobalKeywordsHU, 30); err != nil {
		return fmt.Errorf("Hungarian global keywords: %w", err)
	}
	if len([]rune(in.OrganizationName)) > 160 {
		return fmt.Errorf("organization name is too long")
	}
	if in.OrganizationURL != "" {
		u, err := url.Parse(in.OrganizationURL)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.Fragment != "" {
			return fmt.Errorf("organization URL must be an absolute HTTPS URL")
		}
	}
	if in.DefaultOGImageAssetID != "" && !a.mediaExists(ctx, in.DefaultOGImageAssetID) {
		return fmt.Errorf("default Open Graph image does not exist")
	}
	return nil
}

func (a *app) readSiteSEO() (siteSEOSettings, siteSEOSettings, int, string, time.Time, sql.NullTime, error) {
	draft := defaultSiteSEO()
	published := defaultSiteSEO()
	var draftRaw, publishedRaw []byte
	var version int
	var updatedBy string
	var updatedAt time.Time
	var publishedAt sql.NullTime
	err := a.db.QueryRow("SELECT draft,published,version,updated_by,updated_at,published_at FROM cms.seo_settings WHERE id=1").
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
	return normalizeSiteSEO(draft), normalizeSiteSEO(published), version, updatedBy, updatedAt, publishedAt, nil
}

func seoSettingsPayload(draft, published siteSEOSettings, version int, updatedBy string, updatedAt time.Time, publishedAt sql.NullTime) map[string]any {
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

func (a *app) seoSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use GET")
		return
	}
	draft, published, version, updatedBy, updatedAt, publishedAt, err := a.readSiteSEO()
	if err != nil {
		common.APIError(w, http.StatusInternalServerError, "DB", "Could not load SEO settings")
		return
	}
	common.JSON(w, http.StatusOK, seoSettingsPayload(draft, published, version, updatedBy, updatedAt, publishedAt))
}

func (a *app) seoAction(w http.ResponseWriter, r *http.Request) {
	action := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/cms/seo/"), "/")
	switch action {
	case "draft":
		if r.Method != http.MethodPut {
			common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use PUT")
			return
		}
		var in siteSEOSettings
		if common.Decode(r, &in) != nil {
			common.APIError(w, http.StatusBadRequest, "JSON", "Invalid request")
			return
		}
		in = normalizeSiteSEO(in)
		if err := a.validateSiteSEO(r.Context(), in); err != nil {
			common.APIError(w, http.StatusBadRequest, "VALIDATION", err.Error())
			return
		}
		oldDraft, oldPublished, version, _, _, _, err := a.readSiteSEO()
		if err != nil {
			common.APIError(w, http.StatusInternalServerError, "DB", "Could not load SEO settings")
			return
		}
		query := "INSERT INTO cms.seo_settings(id,draft,published,version,updated_by,updated_at) " +
			"VALUES(1,$1::jsonb,$2::jsonb,$3,$4,NOW()) " +
			"ON CONFLICT(id) DO UPDATE SET draft=EXCLUDED.draft,updated_by=EXCLUDED.updated_by,updated_at=NOW()"
		if _, err = a.db.ExecContext(r.Context(), query, string(jsonBytes(in)), string(jsonBytes(oldPublished)), version, actor(r)); err != nil {
			common.APIError(w, http.StatusInternalServerError, "DB", "Could not save SEO draft")
			return
		}
		_ = a.audit(r.Context(), "", "", "SEO_DRAFT_SAVED", actor(r), correlationID(r), oldDraft, in)
		draft, published, nextVersion, updatedBy, updatedAt, publishedAt, _ := a.readSiteSEO()
		common.JSON(w, http.StatusOK, seoSettingsPayload(draft, published, nextVersion, updatedBy, updatedAt, publishedAt))
	case "publish":
		if r.Method != http.MethodPost {
			common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use POST")
			return
		}
		draft, published, version, _, _, _, err := a.readSiteSEO()
		if err != nil {
			common.APIError(w, http.StatusInternalServerError, "DB", "Could not load SEO settings")
			return
		}
		if err := a.validateSiteSEO(r.Context(), draft); err != nil {
			common.APIError(w, http.StatusBadRequest, "VALIDATION", err.Error())
			return
		}
		query := "INSERT INTO cms.seo_settings(id,draft,published,version,updated_by,updated_at,published_at) " +
			"VALUES(1,$1::jsonb,$1::jsonb,$2,$3,NOW(),NOW()) " +
			"ON CONFLICT(id) DO UPDATE SET published=EXCLUDED.published,version=EXCLUDED.version," +
			"updated_by=EXCLUDED.updated_by,updated_at=NOW(),published_at=NOW()"
		if _, err = a.db.ExecContext(r.Context(), query, string(jsonBytes(draft)), version+1, actor(r)); err != nil {
			common.APIError(w, http.StatusInternalServerError, "DB", "Could not publish SEO settings")
			return
		}
		_ = a.audit(r.Context(), "", "", "SEO_PUBLISHED", actor(r), correlationID(r), published, draft)
		nextDraft, nextPublished, storedVersion, updatedBy, updatedAt, publishedAt, _ := a.readSiteSEO()
		common.JSON(w, http.StatusOK, seoSettingsPayload(nextDraft, nextPublished, storedVersion, updatedBy, updatedAt, publishedAt))
	case "audit":
		a.seoAudit(w, r)
	default:
		common.APIError(w, http.StatusNotFound, "NOT_FOUND", "SEO action not found")
	}
}

func seoLocaleKeywords(settings siteSEOSettings, locale string) []string {
	if normalizeLocale(locale) == "hu_HU" {
		return append([]string(nil), settings.GlobalKeywordsHU...)
	}
	return append([]string(nil), settings.GlobalKeywordsEN...)
}

func combineKeywords(primary, secondary []string, max int) []string {
	combined := append(append([]string(nil), primary...), secondary...)
	return normalizeKeywords(combined, max)
}

var seoStopWords = map[string]bool{
	"about": true, "after": true, "also": true, "and": true, "are": true, "been": true, "being": true,
	"from": true, "have": true, "into": true, "more": true, "our": true, "that": true, "the": true, "their": true,
	"this": true, "through": true, "with": true, "your": true, "you": true, "for": true, "can": true, "will": true,
	"egy": true, "es": true, "az": true, "hogy": true, "mint": true, "vagy": true, "ami": true, "ahol": true,
	"minden": true, "utan": true, "elott": true, "kozott": true, "szamara": true, "pedig": true, "mar": true,
	"himate": false,
}

func seoWords(value string) []string {
	value = strings.ToLower(value)
	var out []string
	var current strings.Builder
	flush := func() {
		word := current.String()
		current.Reset()
		if len([]rune(word)) < 4 || seoStopWords[word] {
			return
		}
		out = append(out, word)
	}
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return out
}

func suggestSEOKeywords(text string, existing []string, limit int) []string {
	counts := map[string]int{}
	blocked := map[string]bool{}
	for _, value := range existing {
		blocked[strings.ToLower(strings.TrimSpace(value))] = true
	}
	for _, word := range seoWords(text) {
		if !blocked[word] {
			counts[word]++
		}
	}
	type candidate struct {
		word  string
		count int
	}
	values := make([]candidate, 0, len(counts))
	for word, count := range counts {
		values = append(values, candidate{word: word, count: count})
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].count == values[j].count {
			return values[i].word < values[j].word
		}
		return values[i].count > values[j].count
	})
	if limit > len(values) {
		limit = len(values)
	}
	out := make([]string, 0, limit)
	for _, item := range values[:limit] {
		out = append(out, item.word)
	}
	return out
}

func auditSEOPage(page pageRow, version versionRow, settings siteSEOSettings) map[string]any {
	in, _ := inputFromVersion(version)
	globalKeywords := seoLocaleKeywords(settings, page.Locale)
	pageKeywords := normalizeKeywords(in.SEO.Keywords, 24)
	combined := combineKeywords(pageKeywords, globalKeywords, 40)

	var content strings.Builder
	content.WriteString(in.SEO.Title)
	content.WriteByte(' ')
	content.WriteString(in.SEO.MetaDescription)
	for _, section := range in.Sections {
		if !section.Visible {
			continue
		}
		content.WriteByte(' ')
		content.WriteString(section.Heading)
		content.WriteByte(' ')
		content.WriteString(section.Body)
	}
	contentText := strings.Join(strings.Fields(content.String()), " ")
	lowerContent := strings.ToLower(contentText)

	score := 100
	issues := []seoAuditIssue{}
	add := func(code, severity, message string, penalty int) {
		issues = append(issues, seoAuditIssue{Code: code, Severity: severity, Message: message})
		score -= penalty
	}

	titleLen := len([]rune(in.SEO.Title))
	switch {
	case titleLen == 0:
		add("TITLE_MISSING", "ERROR", "SEO title is missing.", 25)
	case titleLen < 30:
		add("TITLE_SHORT", "WARNING", "SEO title is shorter than 30 characters.", 6)
	case titleLen > 60:
		add("TITLE_LONG", "WARNING", "SEO title is longer than 60 characters.", 6)
	}
	metaLen := len([]rune(in.SEO.MetaDescription))
	switch {
	case metaLen == 0:
		add("META_MISSING", "ERROR", "Meta description is missing.", 20)
	case metaLen < 120:
		add("META_SHORT", "WARNING", "Meta description is shorter than 120 characters.", 6)
	case metaLen > 160:
		add("META_LONG", "WARNING", "Meta description is longer than 160 characters.", 6)
	}
	if in.SEO.Canonical == "" || !validCanonical(in.SEO.Canonical) {
		add("CANONICAL_INVALID", "ERROR", "Canonical URL must be an absolute HTTPS URL.", 15)
	}
	if len(pageKeywords) < 3 {
		add("PAGE_KEYWORDS_LOW", "WARNING", "Add at least three page-specific keywords.", 8)
	} else if len(pageKeywords) > 12 {
		add("PAGE_KEYWORDS_HIGH", "INFO", "Consider focusing page-specific keywords to twelve or fewer.", 2)
	}
	if len(globalKeywords) < 3 {
		add("GLOBAL_KEYWORDS_LOW", "WARNING", "Global keywords for this language are sparse.", 4)
	}
	if len([]rune(contentText)) < 240 {
		add("CONTENT_THIN", "WARNING", "Visible page content is thin for organic search.", 8)
	}
	if len(pageKeywords) > 0 {
		matches := 0
		for _, keyword := range pageKeywords {
			if strings.Contains(lowerContent, strings.ToLower(keyword)) {
				matches++
			}
		}
		if matches == 0 {
			add("KEYWORD_MISMATCH", "WARNING", "Page keywords are not represented in visible page content.", 10)
		}
	}
	if strings.TrimSpace(in.SEO.OGTitle) == "" {
		add("OG_TITLE_FALLBACK", "INFO", "Open Graph title will fall back to the SEO title.", 1)
	}
	if strings.TrimSpace(in.SEO.OGDescription) == "" {
		add("OG_DESCRIPTION_FALLBACK", "INFO", "Open Graph description will fall back to the meta description.", 1)
	}
	if score < 0 {
		score = 0
	}
	status := "READY"
	if score < 80 {
		status = "NEEDS_ATTENTION"
	}
	if score < 60 {
		status = "BLOCKED"
	}

	return map[string]any{
		"page_id":             page.ID,
		"page_key":            page.PageKey,
		"name":                page.Name,
		"locale":              normalizeLocale(page.Locale),
		"version_no":          version.VersionNo,
		"workflow_state":      version.State,
		"score":               score,
		"status":              status,
		"issues":              issues,
		"page_keywords":       pageKeywords,
		"global_keywords":     globalKeywords,
		"effective_keywords":  combined,
		"suggested_keywords":  suggestSEOKeywords(contentText, combined, 10),
		"title_length":         titleLen,
		"meta_length":          metaLen,
		"content_characters":  len([]rune(contentText)),
	}
}

func (a *app) seoAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use GET")
		return
	}
	localeFilter := strings.TrimSpace(r.URL.Query().Get("locale"))
	if localeFilter != "" {
		localeFilter = normalizeLocale(localeFilter)
	}
	draftSettings, _, _, _, _, _, err := a.readSiteSEO()
	if err != nil {
		common.APIError(w, http.StatusInternalServerError, "DB", "Could not load SEO settings")
		return
	}
	query := pageSelect()
	args := []any{}
	if localeFilter != "" {
		query += " WHERE locale=$1"
		args = append(args, localeFilter)
	}
	query += " ORDER BY locale,name,page_key"
	rows, err := a.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		common.APIError(w, http.StatusInternalServerError, "DB", "Could not load CMS pages for SEO audit")
		return
	}
	defer rows.Close()

	result := []map[string]any{}
	totalScore := 0
	attention := 0
	for rows.Next() {
		page, err := scanPage(rows)
		if err != nil || strings.TrimSpace(page.DraftVersionID) == "" {
			continue
		}
		version, err := a.getVersion(page.DraftVersionID)
		if err != nil {
			continue
		}
		item := auditSEOPage(page, version, draftSettings)
		score, _ := item["score"].(int)
		totalScore += score
		if item["status"] != "READY" {
			attention++
		}
		result = append(result, item)
	}
	average := 0
	if len(result) > 0 {
		average = totalScore / len(result)
	}
	common.JSON(w, http.StatusOK, map[string]any{
		"items": result,
		"summary": map[string]any{
			"pages":               len(result),
			"average_score":       average,
			"needs_attention":     attention,
			"ready":               len(result) - attention,
		},
	})
}

func (a *app) publicSEOSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use GET or HEAD")
		return
	}
	locale := normalizeLocale(r.URL.Query().Get("locale"))
	_, published, version, _, _, publishedAt, err := a.readSiteSEO()
	if err != nil {
		common.APIError(w, http.StatusInternalServerError, "DB", "Could not load public SEO settings")
		return
	}
	var publishedValue any
	if publishedAt.Valid {
		publishedValue = publishedAt.Time.UTC()
	}
	out := map[string]any{
		"locale":                    locale,
		"version":                   version,
		"global_keywords":           seoLocaleKeywords(published, locale),
		"organization_name":         published.OrganizationName,
		"organization_url":          published.OrganizationURL,
		"default_og_image_asset_id": published.DefaultOGImageAssetID,
		"published_at":              publishedValue,
	}
	w.Header().Set("Cache-Control", "public, max-age=60, stale-while-revalidate=300")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	common.JSON(w, http.StatusOK, out)
}

func anyStringSlice(value any) []string {
	out := []string{}
	switch typed := value.(type) {
	case []string:
		return normalizeKeywords(typed, 40)
	case []any:
		for _, item := range typed {
			if text, ok := item.(string); ok {
				out = append(out, text)
			}
		}
	}
	return normalizeKeywords(out, 40)
}

func (a *app) enrichPublicSEO(ctx context.Context, pageKey, locale string, out map[string]any) {
	_, published, _, _, _, _, err := a.readSiteSEO()
	if err != nil {
		published = defaultSiteSEO()
	}
	seoMap, _ := out["seo"].(map[string]any)
	if seoMap == nil {
		seoMap = map[string]any{}
	}
	pageKeywords := anyStringSlice(seoMap["keywords"])
	effectiveKeywords := combineKeywords(pageKeywords, seoLocaleKeywords(published, locale), 40)
	seoMap["keywords"] = effectiveKeywords
	if strings.TrimSpace(fmt.Sprint(seoMap["og_image_asset_id"])) == "" && published.DefaultOGImageAssetID != "" {
		seoMap["og_image_asset_id"] = published.DefaultOGImageAssetID
	}

	title := strings.TrimSpace(fmt.Sprint(seoMap["title"]))
	description := strings.TrimSpace(fmt.Sprint(seoMap["meta_description"]))
	canonical := strings.TrimSpace(fmt.Sprint(seoMap["canonical"]))
	language := "en-US"
	if normalizeLocale(locale) == "hu_HU" {
		language = "hu-HU"
	}
	jsonLD := map[string]any{
		"@context":    "https://schema.org",
		"@type":       "WebPage",
		"name":        title,
		"description": description,
		"url":         canonical,
		"inLanguage":  language,
	}
	if published.OrganizationName != "" || published.OrganizationURL != "" {
		jsonLD["publisher"] = map[string]any{
			"@type": "Organization",
			"name":  published.OrganizationName,
			"url":   published.OrganizationURL,
		}
	}
	seoMap["json_ld"] = jsonLD
	out["seo"] = seoMap

	alternates := map[string]string{}
	rows, err := a.db.QueryContext(ctx, "SELECT p.locale,v.seo->>'canonical' FROM cms.pages p JOIN cms.versions v ON v.id=p.published_version_id WHERE p.page_key=$1 AND v.state='PUBLISHED' ORDER BY p.locale", pageKey)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var siblingLocale, siblingCanonical string
			if rows.Scan(&siblingLocale, &siblingCanonical) == nil && validCanonical(siblingCanonical) {
				alternates[normalizeLocale(siblingLocale)] = siblingCanonical
			}
		}
	}
	out["alternates"] = alternates
}
