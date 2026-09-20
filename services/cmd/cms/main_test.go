package main

import "testing"

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
	for _, value := range []string{"image/png", "image/jpeg", "image/webp"} {
		if !cmsMimeAllowed(value) {
			t.Fatalf("allowed media rejected: %s", value)
		}
	}
	for _, value := range []string{"image/svg+xml", "text/html", "application/pdf"} {
		if cmsMimeAllowed(value) {
			t.Fatalf("unsafe/non-image CMS media accepted: %s", value)
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
