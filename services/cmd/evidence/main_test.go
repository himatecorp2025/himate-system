package main

import (
	"strings"
	"testing"
)

func TestEvidenceFileTypeBoundary(t *testing.T) {
	if !allowedMime("PDF", "application/pdf") {
		t.Fatal("PDF evidence must accept application/pdf")
	}
	if allowedMime("PDF", "text/html") {
		t.Fatal("PDF evidence must reject HTML")
	}
	if !allowedMime("SCREENSHOT", "image/png") {
		t.Fatal("screenshot evidence must accept PNG")
	}
	if allowedMime("SCREENSHOT", "image/svg+xml") {
		t.Fatal("SVG is intentionally rejected from evidence uploads")
	}
}

func TestEvidenceCommonValidation(t *testing.T) {
	if _, _, err := validateCommon("", "culture.events", "PDF", "Evidence", "2026-01-01", "2026-01-31"); err == nil {
		t.Fatal("partner_id must be required")
	}
	if _, _, err := validateCommon("ptr_1", "bad key", "PDF", "Evidence", "2026-01-01", "2026-01-31"); err == nil {
		t.Fatal("invalid metric key must be rejected")
	}
	if _, _, err := validateCommon("ptr_1", "culture.events", "PDF", "Evidence", "2026-02-01", "2026-01-01"); err == nil {
		t.Fatal("reversed period must be rejected")
	}
}

func TestSafeFilenameRemovesTraversalAndControlCharacters(t *testing.T) {
	got := safeFilename("../../secret\nreport.pdf")
	if strings.Contains(got, "..") || strings.ContainsAny(got, "/\\\n\r") {
		t.Fatalf("unsafe filename %q", got)
	}
}
