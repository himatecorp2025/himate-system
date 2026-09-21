package main

import (
	"context"
	"strings"
	"testing"
)

func TestPreviewTokenHashBoundary(t *testing.T) {
	raw := randomToken()
	if raw == "" {
		t.Fatal("preview token must be generated")
	}
	stored := tokenHash(raw)
	if stored == raw || len(stored) != 64 {
		t.Fatal("preview token must be stored only as a SHA-256 hash")
	}
	if !tokenMatches(raw, stored) {
		t.Fatal("valid preview token was rejected")
	}
	if tokenMatches(raw+"x", stored) {
		t.Fatal("invalid preview token was accepted")
	}
}

func TestSafeCTA(t *testing.T) {
	for _, value := range []string{"/contact", "https://example.com/path", "http://localhost:8080/test", ""} {
		if !safeCTA(value) {
			t.Fatalf("expected safe CTA %q", value)
		}
	}
	for _, value := range []string{"javascript:alert(1)", "//evil.example", "data:text/html,test"} {
		if safeCTA(value) {
			t.Fatalf("unsafe CTA accepted: %q", value)
		}
	}
}

func TestCanonicalValidation(t *testing.T) {
	if !validCanonical("https://www.himate.com/platform") {
		t.Fatal("valid HTTPS canonical rejected")
	}
	for _, value := range []string{"http://himate.com/", "/platform", "https://himate.com/#fragment", "javascript:bad"} {
		if validCanonical(value) {
			t.Fatalf("invalid canonical accepted: %q", value)
		}
	}
}

func TestNormalizeVersionSortsSections(t *testing.T) {
	in := versionInput{
		Slug: " Landing ",
		Sections: []sectionInput{
			{ID: "second", ComponentType: "text", Visible: true, SortOrder: 20},
			{ID: "first", ComponentType: "hero", Visible: true, SortOrder: 10},
		},
	}
	out := normalizedInput(in)
	if out.Slug != "landing" {
		t.Fatalf("unexpected normalized slug %q", out.Slug)
	}
	if out.Sections[0].ID != "first" || out.Sections[0].ComponentType != "HERO" {
		t.Fatalf("sections were not normalized/sorted: %#v", out.Sections)
	}
}

func TestCMSMediaMimePolicy(t *testing.T) {
	for _, value := range []string{"image/png", "image/jpeg", "image/webp", "video/mp4", "video/webm"} {
		if !cmsMimeAllowed(value) {
			t.Fatalf("allowed media rejected: %s", value)
		}
	}
	for _, value := range []string{"image/svg+xml", "text/html", "application/pdf"} {
		if cmsMimeAllowed(value) {
			t.Fatalf("unsafe CMS media accepted: %s", value)
		}
	}
}

func TestMediaIDsAreUniqueAndSorted(t *testing.T) {
	in := versionInput{
		SEO: seoInput{OGImageAssetID: "cms_media_b"},
		Sections: []sectionInput{
			{MediaAssetID: "cms_media_a"},
			{MediaAssetID: "cms_media_b"},
		},
	}
	ids := mediaIDs(in)
	if len(ids) != 2 || ids[0] != "cms_media_a" || ids[1] != "cms_media_b" {
		t.Fatalf("unexpected media IDs: %#v", ids)
	}
}


func TestCMSContentBounds(t *testing.T) {
	a := &app{}
	tooLongBody := versionInput{
		Slug: "bounded-page",
		Sections: []sectionInput{
			{ID: "hero", ComponentType: "HERO", Heading: "Heading", Body: strings.Repeat("x", 20001), Visible: true, SortOrder: 10, Settings: map[string]any{}},
		},
	}
	if err := a.validateContent(context.Background(), "", tooLongBody, false); err == nil {
		t.Fatal("oversized section body must be rejected")
	}

	tooMany := versionInput{Slug: "many-sections"}
	for i := 0; i < 101; i++ {
		tooMany.Sections = append(tooMany.Sections, sectionInput{
			ID: "section-" + strings.Repeat("a", i%5+1),
			ComponentType: "TEXT",
			Heading: "Heading",
			Visible: true,
			SortOrder: i,
			Settings: map[string]any{},
		})
	}
	if err := a.validateContent(context.Background(), "", tooMany, false); err == nil {
		t.Fatal("more than 100 sections must be rejected")
	}
}

func TestLocaleNormalizationAndDefaultDesign(t *testing.T) {
	if got := normalizeLocale("hu_HU"); got != "hu_HU" { t.Fatalf("hungarian locale: %q", got) }
	for _, value := range []string{"", "en_US", "de_DE", "hu"} {
		if got := normalizeLocale(value); got != "en_US" { t.Fatalf("locale %q normalized to %q", value, got) }
	}
	design := defaultSiteDesign()
	if design.Navy != "#06172C" || design.Gold != "#D7AE62" { t.Fatalf("unexpected brand defaults: %+v", design) }
	if len(design.Navigation) != 6 { t.Fatalf("expected six default navigation items, got %d", len(design.Navigation)) }

	other := defaultSiteDesign()
	design.Navigation[2].LabelHU = "Módosított"
	if other.Navigation[2].LabelHU != "Programok" {
		t.Fatal("default Design Guide states must not share navigation backing storage")
	}
}

func TestDesignValidationWithoutMediaLookup(t *testing.T) {
	a := &app{}
	valid := defaultSiteDesign()
	if err := a.validateSiteDesign(context.Background(), valid); err != nil {
		t.Fatalf("default design must validate: %v", err)
	}
	badColor := valid
	badColor.Navy = "navy"
	if err := a.validateSiteDesign(context.Background(), badColor); err == nil {
		t.Fatal("invalid color should fail")
	}
	badFont := valid
	badFont.HeadingFont = "Untrusted Remote Font"
	if err := a.validateSiteDesign(context.Background(), badFont); err == nil {
		t.Fatal("unsupported font should fail")
	}
	badURL := valid
	badURL.Navigation = append([]navigationItem(nil), valid.Navigation...)
	badURL.Navigation[0].URL = "javascript:alert(1)"
	if err := a.validateSiteDesign(context.Background(), badURL); err == nil {
		t.Fatal("unsafe navigation URL should fail")
	}
}

func TestSEOKeywordNormalizationAndAudit(t *testing.T) {
	keywords := normalizeKeywords([]string{" Culture ", "culture", "Arts", "digital platform"}, 30)
	if len(keywords) != 3 || keywords[0] != "Culture" || keywords[1] != "Arts" {
		t.Fatalf("unexpected normalized keywords: %#v", keywords)
	}
	if err := validateKeywords([]string{"arts", "culture"}, 24); err != nil {
		t.Fatalf("valid keywords rejected: %v", err)
	}

	page := pageRow{ID: "page_1", PageKey: "landing", Name: "Landing", Locale: "en_US"}
	version := versionRow{
		VersionNo: 2,
		State: "DRAFT",
		SEO: jsonBytes(seoInput{
			Title: "HIMATE culture platform for arts organizations",
			MetaDescription: "HIMATE connects arts and cultural organizations with a secure digital platform for programs, partnerships, evidence, reporting and measurable community impact.",
			Canonical: "https://www.himate.com/landing",
			Keywords: []string{"culture", "arts", "digital platform"},
		}),
		Sections: jsonBytes([]sectionInput{
			{ID: "hero", ComponentType: "HERO", Heading: "Culture connects people", Body: "Arts organizations use the HIMATE digital platform to connect programs, partners, communities and evidence for measurable cultural impact.", Visible: true, SortOrder: 10, Settings: map[string]any{}},
		}),
	}
	result := auditSEOPage(page, version, defaultSiteSEO())
	score, ok := result["score"].(int)
	if !ok || score <= 0 || score > 100 {
		t.Fatalf("unexpected SEO score: %#v", result["score"])
	}
	effective, ok := result["effective_keywords"].([]string)
	if !ok || len(effective) < 3 {
		t.Fatalf("expected combined SEO keywords: %#v", result["effective_keywords"])
	}
	if result["locale"] != "en_US" {
		t.Fatalf("unexpected locale: %#v", result["locale"])
	}
}

func TestSiteSEOValidationAndLanguageSelection(t *testing.T) {
	a := &app{}
	settings := defaultSiteSEO()
	settings.GlobalKeywordsEN = []string{"arts", "culture", "HIMATE"}
	settings.GlobalKeywordsHU = []string{"művészet", "kultúra", "HIMATE"}
	if err := a.validateSiteSEO(context.Background(), settings); err != nil {
		t.Fatalf("default SEO settings must validate: %v", err)
	}
	if got := seoLocaleKeywords(settings, "hu_HU"); len(got) != 3 || got[0] != "művészet" {
		t.Fatalf("unexpected Hungarian keywords: %#v", got)
	}
	settings.OrganizationURL = "http://example.com"
	if err := a.validateSiteSEO(context.Background(), settings); err == nil {
		t.Fatal("non-HTTPS organization URL must be rejected")
	}
}
