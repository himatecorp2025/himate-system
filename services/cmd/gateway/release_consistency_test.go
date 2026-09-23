package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func releaseHealthServer(t *testing.T, version string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			http.NotFound(w, r)
			return
		}
		if version != "" {
			w.Header().Set("X-Himate-App-Version", version)
		}
		w.WriteHeader(http.StatusOK)
	}))
}

func TestServiceReleaseAcceptsMatchingMicroservice(t *testing.T) {
	server := releaseHealthServer(t, "0.8.26-start-23.11.3i")
	defer server.Close()

	a := &app{
		version: "0.8.26-start-23.11.3i",
		internalToken: "local-development-internal-token-123456789",
		client: &http.Client{Timeout: time.Second},
		hosts: map[string]string{"partners": strings.TrimPrefix(server.URL, "http://")},
	}
	got, err := a.serviceRelease(context.Background(), "partners")
	if err != nil {
		t.Fatal(err)
	}
	if got != a.version {
		t.Fatalf("got %q want %q", got, a.version)
	}
}

func TestServiceReleaseRejectsOldOrUnversionedMicroservice(t *testing.T) {
	for name, version := range map[string]string{
		"old": "0.8.25-start-23.11.3h",
		"missing": "",
	} {
		t.Run(name, func(t *testing.T) {
			server := releaseHealthServer(t, version)
			defer server.Close()
			a := &app{
				version: "0.8.26-start-23.11.3i",
				internalToken: "local-development-internal-token-123456789",
				client: &http.Client{Timeout: time.Second},
				hosts: map[string]string{"partners": strings.TrimPrefix(server.URL, "http://")},
			}
			if _, err := a.serviceRelease(context.Background(), "partners"); err == nil {
				t.Fatal("expected release mismatch to be rejected")
			}
		})
	}
}
