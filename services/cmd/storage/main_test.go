package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestArchiveNamespaceProducesReadableTarGzip(t *testing.T) {
	root := t.TempDir()
	partnerID := "ptr_000001"
	namespace := filepath.Join(root, partnerID)
	if err := os.MkdirAll(filepath.Join(namespace, "media"), 0700); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(namespace, ".himate-storage"), []byte(partnerID), 0600); err != nil { t.Fatal(err) }
	want := []byte("START-21-readable-media")
	if err := os.WriteFile(filepath.Join(namespace, "media", "marker.txt"), want, 0600); err != nil { t.Fatal(err) }

	a := &app{root: root}
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/storage/partners/"+partnerID+"/archive", nil)
	rec := httptest.NewRecorder()
	a.archiveNamespace(rec, req, partnerID)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("X-Himate-Archive-Files"); got != "1" {
		t.Fatalf("expected one archived file, got %q", got)
	}

	gz, err := gzip.NewReader(bytes.NewReader(rec.Body.Bytes()))
	if err != nil { t.Fatalf("gzip reader: %v", err) }
	defer gz.Close()
	tr := tar.NewReader(gz)
	found := false
	for {
		header, err := tr.Next()
		if err == io.EOF { break }
		if err != nil { t.Fatalf("tar reader: %v", err) }
		if header.Name == ".himate-storage" {
			t.Fatal("storage namespace marker leaked into media archive")
		}
		if header.Name == "media/marker.txt" {
			body, err := io.ReadAll(tr)
			if err != nil { t.Fatal(err) }
			if !bytes.Equal(body, want) {
				t.Fatalf("unexpected archived media %q", string(body))
			}
			found = true
		}
	}
	if !found { t.Fatal("media marker missing from archive") }
}

func TestArchiveNamespaceRejectsUnreadableMediaBeforeStreaming(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission semantics are not meaningful as root")
	}
	root := t.TempDir()
	partnerID := "ptr_000002"
	namespace := filepath.Join(root, partnerID)
	if err := os.MkdirAll(namespace, 0700); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(namespace, ".himate-storage"), []byte(partnerID), 0600); err != nil { t.Fatal(err) }
	media := filepath.Join(namespace, "private.bin")
	if err := os.WriteFile(media, []byte("secret"), 0600); err != nil { t.Fatal(err) }
	if err := os.Chmod(media, 0000); err != nil { t.Fatal(err) }
	defer os.Chmod(media, 0600)

	a := &app{root: root}
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/storage/partners/"+partnerID+"/archive", nil)
	rec := httptest.NewRecorder()
	a.archiveNamespace(rec, req, partnerID)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected unreadable media to fail before streaming, got %d", rec.Code)
	}
}
