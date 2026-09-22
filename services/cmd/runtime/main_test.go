package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizedProviderStatus(t *testing.T) {
	cases := map[string]string{
		"live":              "READY",
		"build_in_progress": "DEPLOYING",
		"queued":            "DEPLOYING",
		"build_failed":      "FAILED",
		"canceled":          "FAILED",
	}
	for input, want := range cases {
		if got := normalizedProviderStatus(input); got != want {
			t.Fatalf("%s: expected %s, got %s", input, want, got)
		}
	}
}

func TestRenderProviderTriggerAndStatus(t *testing.T) {
	var triggerSeen bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-render-key" {
			t.Fatalf("missing Render authorization header")
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/services/srv_123/deploys":
			triggerSeen = true
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["clearCache"] != "clear" || body["commitId"] != "abcdef1" {
				t.Fatalf("unexpected trigger body: %#v", body)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"dep_123","status":"build_in_progress"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/services/srv_123/deploys/dep_123":
			_, _ = w.Write([]byte(`{"id":"dep_123","status":"live"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	provider := renderProvider{
		apiBase: server.URL + "/v1",
		apiKey:  "test-render-key",
		client:  server.Client(),
	}
	deploy, err := provider.Trigger(context.Background(), providerRequest{
		ServiceID: "srv_123", CommitID: "abcdef1", ClearCache: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !triggerSeen || deploy.ID != "dep_123" || normalizedProviderStatus(deploy.Status) != "DEPLOYING" {
		t.Fatalf("unexpected trigger deploy: %#v", deploy)
	}
	current, err := provider.Status(context.Background(), "srv_123", "dep_123")
	if err != nil {
		t.Fatal(err)
	}
	if current.ID != "dep_123" || normalizedProviderStatus(current.Status) != "READY" {
		t.Fatalf("unexpected provider status: %#v", current)
	}
}

func TestRenderProviderRejectsMissingServiceID(t *testing.T) {
	provider := renderProvider{
		apiBase: "https://api.render.invalid/v1",
		apiKey:  "key",
		client:  http.DefaultClient,
	}
	_, err := provider.Trigger(context.Background(), providerRequest{})
	if err == nil || !strings.Contains(err.Error(), "render_service_id") {
		t.Fatalf("expected missing service ID error, got %v", err)
	}
}


func TestValidateEnvironmentProviderProductionRejectsLocalDowngrade(t *testing.T) {
	a := &app{defaultProvider: "render"}
	if err := a.validateEnvironmentProvider("PRODUCTION", localProvider{}); err == nil {
		t.Fatal("expected production local-provider downgrade to be rejected")
	}
}

func TestValidateEnvironmentProviderLocalProcessAllowsLocalForCI(t *testing.T) {
	a := &app{defaultProvider: "local"}
	if err := a.validateEnvironmentProvider("PRODUCTION", localProvider{}); err != nil {
		t.Fatalf("local CI provider should remain valid: %v", err)
	}
}

func TestDeploymentProviderUsesRenderDefaultAndServiceID(t *testing.T) {
	a := &app{
		defaultProvider: "render",
		defaultRenderID: "srv_default",
		providers: map[string]deploymentProvider{
			"local": localProvider{},
			"render": renderProvider{},
		},
	}
	provider, serviceID, err := a.deploymentProvider(map[string]any{})
	if err != nil {
		t.Fatalf("deployment provider: %v", err)
	}
	if provider.Name() != "render" {
		t.Fatalf("expected render provider, got %s", provider.Name())
	}
	if serviceID != "srv_default" {
		t.Fatalf("expected default Render service id, got %q", serviceID)
	}
}
