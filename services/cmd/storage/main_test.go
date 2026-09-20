package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestObjectPathStaysInsideNamespace(t *testing.T) {
	a := &app{root: t.TempDir()}
	path, err := a.objectPath("ptr_1", "evidence/evd_1.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(path, filepath.Join("ptr_1", "evidence")) {
		t.Fatalf("unexpected object path %q", path)
	}
	if _, err := a.objectPath("ptr_1", "../escape.txt"); err == nil {
		t.Fatal("path traversal must be rejected")
	}
}


func TestObjectRouteSupportsByteRanges(t *testing.T) {
	root := t.TempDir()
	namespace := "cms_test"
	key := "media/story.mp4"
	target := filepath.Join(root, namespace, "media", "story.mp4")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil { t.Fatal(err) }
	payload := []byte("0123456789abcdef")
	if err := os.WriteFile(target, payload, 0o600); err != nil { t.Fatal(err) }

	a := &app{root: root}
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/storage/objects/"+namespace+"/"+key+"?content_type=video%2Fmp4", nil)
	req.Header.Set("Range", "bytes=4-7")
	rec := httptest.NewRecorder()
	a.objectRoute(rec, req)

	if rec.Code != http.StatusPartialContent {
		t.Fatalf("expected 206, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Range"); got != "bytes 4-7/16" {
		t.Fatalf("unexpected content range %q", got)
	}
	if got := rec.Header().Get("Accept-Ranges"); got != "bytes" {
		t.Fatalf("expected byte range support, got %q", got)
	}
	if rec.Body.String() != "4567" {
		t.Fatalf("unexpected partial body %q", rec.Body.String())
	}
}
