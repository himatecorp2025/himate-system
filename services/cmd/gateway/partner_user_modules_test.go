package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCentral4ModuleRuntimeStatusWriterTracksSuccessfulStatus(t *testing.T) {
	base := httptest.NewRecorder()
	writer := &moduleRuntimeStatusWriter{ResponseWriter: base}

	writer.WriteHeader(http.StatusAccepted)
	if writer.status != http.StatusAccepted {
		t.Fatalf("status=%d want=%d", writer.status, http.StatusAccepted)
	}
	if base.Code != http.StatusAccepted {
		t.Fatalf("underlying status=%d want=%d", base.Code, http.StatusAccepted)
	}
}

func TestCentral4ModuleRuntimeStatusWriterDefaultsWriteToOK(t *testing.T) {
	base := httptest.NewRecorder()
	writer := &moduleRuntimeStatusWriter{ResponseWriter: base}

	if _, err := writer.Write([]byte("ok")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if writer.status != http.StatusOK {
		t.Fatalf("status=%d want=%d", writer.status, http.StatusOK)
	}
}
