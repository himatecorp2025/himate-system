package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"himate.local/services/internal/common"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type providerDeploy struct {
	ID     string
	Status string
}

type providerRequest struct {
	ServiceID  string
	CommitID   string
	ClearCache bool
}

type deploymentProvider interface {
	Name() string
	Trigger(context.Context, providerRequest) (providerDeploy, error)
	Status(context.Context, string, string) (providerDeploy, error)
}

type localProvider struct{}

func (localProvider) Name() string { return "local" }

func (localProvider) Trigger(_ context.Context, _ providerRequest) (providerDeploy, error) {
	return providerDeploy{
		ID:     fmt.Sprintf("local_%d", time.Now().UTC().UnixNano()),
		Status: "live",
	}, nil
}

func (localProvider) Status(_ context.Context, _ string, deployID string) (providerDeploy, error) {
	return providerDeploy{ID: deployID, Status: "live"}, nil
}

type renderProvider struct {
	apiBase string
	apiKey  string
	client  *http.Client
}

func (renderProvider) Name() string { return "render" }

func (p renderProvider) do(ctx context.Context, method, path string, body any) (map[string]any, error) {
	if strings.TrimSpace(p.apiKey) == "" {
		return nil, fmt.Errorf("Render provider is selected but RENDER_API_KEY is not configured")
	}
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(p.apiBase, "/")+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	out := map[string]any{}
	if resp.Body != nil {
		_ = json.NewDecoder(resp.Body).Decode(&out)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return out, fmt.Errorf("Render API status %d", resp.StatusCode)
	}
	return out, nil
}

func renderDeployFields(out map[string]any) providerDeploy {
	if nested, ok := out["deploy"].(map[string]any); ok {
		out = nested
	}
	return providerDeploy{
		ID:     strings.TrimSpace(fmt.Sprint(out["id"])),
		Status: strings.TrimSpace(fmt.Sprint(out["status"])),
	}
}

func (p renderProvider) Trigger(ctx context.Context, in providerRequest) (providerDeploy, error) {
	serviceID := strings.TrimSpace(in.ServiceID)
	if serviceID == "" {
		return providerDeploy{}, fmt.Errorf("render_service_id is required for Render deployments")
	}
	body := map[string]any{"clearCache": "do_not_clear"}
	if in.ClearCache {
		body["clearCache"] = "clear"
	}
	if commitID := strings.TrimSpace(in.CommitID); commitID != "" {
		body["commitId"] = commitID
	}
	out, err := p.do(ctx, http.MethodPost, "/services/"+url.PathEscape(serviceID)+"/deploys", body)
	if err != nil {
		return providerDeploy{}, err
	}
	deploy := renderDeployFields(out)
	if deploy.ID == "" {
		return providerDeploy{}, fmt.Errorf("Render API returned a deploy without an id")
	}
	return deploy, nil
}

func (p renderProvider) Status(ctx context.Context, serviceID, deployID string) (providerDeploy, error) {
	serviceID = strings.TrimSpace(serviceID)
	deployID = strings.TrimSpace(deployID)
	if serviceID == "" || deployID == "" {
		return providerDeploy{}, fmt.Errorf("Render service and deploy IDs are required")
	}
	out, err := p.do(ctx, http.MethodGet, "/services/"+url.PathEscape(serviceID)+"/deploys/"+url.PathEscape(deployID), nil)
	if err != nil {
		return providerDeploy{}, err
	}
	deploy := renderDeployFields(out)
	if deploy.ID == "" {
		deploy.ID = deployID
	}
	return deploy, nil
}

type app struct {
	db              *sql.DB
	defaultProvider string
	defaultRenderID string
	providers       map[string]deploymentProvider
}

func main() {
	log := common.Logger()
	db, err := common.OpenDB()
	if err != nil {
		log.Error("database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	client := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        32,
			MaxIdleConnsPerHost: 16,
			IdleConnTimeout:     90 * time.Second,
		},
	}
	a := &app{
		db:              db,
		defaultProvider: strings.ToLower(strings.TrimSpace(common.Env("HIMATE_RUNTIME_PROVIDER", "local"))),
		defaultRenderID: strings.TrimSpace(os.Getenv("HIMATE_RENDER_SERVICE_ID")),
		providers: map[string]deploymentProvider{
			"local": localProvider{},
			"render": renderProvider{
				apiBase: common.Env("RENDER_API_BASE", "https://api.render.com/v1"),
				apiKey:  strings.TrimSpace(os.Getenv("RENDER_API_KEY")),
				client:  client,
			},
		},
	}
	if _, ok := a.providers[a.defaultProvider]; !ok {
		log.Error("runtime provider", "error", "unsupported HIMATE_RUNTIME_PROVIDER", "provider", a.defaultProvider)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := a.migrate(ctx); err != nil {
		log.Error("migration", "error", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		common.JSON(w, http.StatusOK, map[string]any{
			"status":   "ok",
			"service":  "partner-runtime",
			"provider": a.defaultProvider,
			"time":     time.Now().UTC(),
		})
	})
	mux.HandleFunc("/internal/v1/runtime/deploy", a.deploy)
	mux.HandleFunc("/internal/v1/runtime/health", a.runtimeHealth)
	mux.HandleFunc("/internal/v1/runtime/summary", a.summary)
	common.Run(log, "partner-runtime", common.Env("PORT", "10000"), common.InternalAuth(os.Getenv("HIMATE_INTERNAL_TOKEN"), mux))
}

func (a *app) migrate(ctx context.Context) error {
	return common.ApplyMigrations(ctx, a.db, "runtime", []common.Migration{
		{Version: 1, Name: "partner-runtime", Statements: []string{
			`CREATE SCHEMA IF NOT EXISTS runtime`,
			`CREATE TABLE IF NOT EXISTS runtime.deployments(
				partner_id TEXT NOT NULL,
				environment TEXT NOT NULL,
				hostname TEXT NOT NULL UNIQUE,
				release TEXT NOT NULL,
				config JSONB NOT NULL DEFAULT '{}'::jsonb,
				status TEXT NOT NULL DEFAULT 'READY',
				deployed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				PRIMARY KEY(partner_id,environment)
			)`,
		}},
		{Version: 2, Name: "provider-deployment-state", Statements: []string{
			`ALTER TABLE runtime.deployments ADD COLUMN IF NOT EXISTS provider TEXT NOT NULL DEFAULT 'local'`,
			`ALTER TABLE runtime.deployments ADD COLUMN IF NOT EXISTS provider_service_id TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE runtime.deployments ADD COLUMN IF NOT EXISTS provider_deploy_id TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE runtime.deployments ADD COLUMN IF NOT EXISTS provider_status TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE runtime.deployments ADD COLUMN IF NOT EXISTS provider_error TEXT NOT NULL DEFAULT ''`,
			`CREATE INDEX IF NOT EXISTS runtime_provider_deploy_idx ON runtime.deployments(provider,provider_deploy_id) WHERE provider_deploy_id<>''`,
			`CREATE INDEX IF NOT EXISTS runtime_status_idx ON runtime.deployments(status,updated_at DESC)`,
		}},
	})
}

func validEnvironment(v string) string {
	v = strings.ToUpper(strings.TrimSpace(v))
	if v != "STAGING" && v != "PRODUCTION" {
		return ""
	}
	return v
}

func configString(config map[string]any, key string) string {
	if config == nil {
		return ""
	}
	value := config[key]
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func configBool(config map[string]any, key string) bool {
	if config == nil {
		return false
	}
	switch value := config[key].(type) {
	case bool:
		return value
	case string:
		value = strings.TrimSpace(value)
		return strings.EqualFold(value, "true") || value == "1"
	default:
		return false
	}
}

func isGitSHA(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) < 7 || len(value) > 40 {
		return false
	}
	for _, r := range value {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

func normalizedProviderStatus(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case value == "live", value == "ready", value == "success", value == "succeeded":
		return "READY"
	case strings.Contains(value, "fail"), strings.Contains(value, "cancel"), strings.Contains(value, "deactiv"):
		return "FAILED"
	default:
		return "DEPLOYING"
	}
}

func (a *app) provider(name string) (deploymentProvider, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		name = a.defaultProvider
	}
	provider, ok := a.providers[name]
	if !ok {
		return nil, fmt.Errorf("unsupported runtime provider %q", name)
	}
	return provider, nil
}

func (a *app) deploymentProvider(config map[string]any) (deploymentProvider, string, error) {
	name := configString(config, "provider")
	provider, err := a.provider(name)
	if err != nil {
		return nil, "", err
	}
	serviceID := ""
	if provider.Name() == "render" {
		serviceID = configString(config, "render_service_id")
		if serviceID == "" {
			serviceID = a.defaultRenderID
		}
		if serviceID == "" {
			return nil, "", fmt.Errorf("render_service_id is required for Render deployments")
		}
	}
	return provider, serviceID, nil
}

func (a *app) persistDeployment(
	partnerID, environment, hostname, release string,
	config map[string]any,
	providerName, serviceID string,
	deploy providerDeploy,
	status, providerError string,
) error {
	raw, err := common.MarshalJSON(config)
	if err != nil {
		return err
	}
	_, err = a.db.Exec(`INSERT INTO runtime.deployments(
			partner_id,environment,hostname,release,config,status,
			provider,provider_service_id,provider_deploy_id,provider_status,provider_error,
			deployed_at,updated_at
		) VALUES($1,$2,$3,$4,$5::jsonb,$6,$7,$8,$9,$10,$11,NOW(),NOW())
		ON CONFLICT(partner_id,environment) DO UPDATE SET
			hostname=EXCLUDED.hostname,
			release=EXCLUDED.release,
			config=EXCLUDED.config,
			status=EXCLUDED.status,
			provider=EXCLUDED.provider,
			provider_service_id=EXCLUDED.provider_service_id,
			provider_deploy_id=EXCLUDED.provider_deploy_id,
			provider_status=EXCLUDED.provider_status,
			provider_error=EXCLUDED.provider_error,
			deployed_at=NOW(),
			updated_at=NOW()`,
		partnerID, environment, hostname, release, string(raw), status,
		providerName, serviceID, deploy.ID, deploy.Status, providerError,
	)
	return err
}

func (a *app) deploy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use POST")
		return
	}
	var in struct {
		PartnerID   string         `json:"partner_id"`
		Environment string         `json:"environment"`
		Hostname    string         `json:"hostname"`
		Release     string         `json:"release"`
		Config      map[string]any `json:"config"`
	}
	if common.Decode(r, &in) != nil || strings.TrimSpace(in.PartnerID) == "" || strings.TrimSpace(in.Hostname) == "" {
		common.APIError(w, http.StatusBadRequest, "VALIDATION", "partner_id and hostname are required")
		return
	}
	in.PartnerID = strings.TrimSpace(in.PartnerID)
	in.Hostname = strings.ToLower(strings.TrimSpace(in.Hostname))
	in.Environment = validEnvironment(in.Environment)
	if in.Environment == "" {
		common.APIError(w, http.StatusBadRequest, "VALIDATION", "STAGING or PRODUCTION environment is required")
		return
	}
	in.Release = strings.TrimSpace(in.Release)
	if in.Release == "" {
		in.Release = "current"
	}

	provider, serviceID, err := a.deploymentProvider(in.Config)
	if err != nil {
		common.APIError(w, http.StatusBadRequest, "PROVIDER_CONFIG", err.Error())
		return
	}
	// Production installations configured for a real deployment provider must fail
	// closed if an environment tries to downgrade itself to the deterministic local
	// adapter. Local remains valid when it is the process-wide provider (CI/dev).
	if in.Environment == "PRODUCTION" && a.defaultProvider != "local" && provider.Name() == "local" {
		common.APIError(w, http.StatusConflict, "PRODUCTION_PROVIDER_REQUIRED", "Production deployment cannot override the configured runtime provider with local")
		return
	}
	commitID := configString(in.Config, "render_commit_id")
	if commitID == "" && isGitSHA(in.Release) {
		commitID = in.Release
	}
	request := providerRequest{
		ServiceID:  serviceID,
		CommitID:   commitID,
		ClearCache: configBool(in.Config, "clear_build_cache"),
	}
	deploy, err := provider.Trigger(r.Context(), request)
	if err != nil {
		_ = a.persistDeployment(
			in.PartnerID, in.Environment, in.Hostname, in.Release, in.Config,
			provider.Name(), serviceID, providerDeploy{}, "FAILED", err.Error(),
		)
		common.APIError(w, http.StatusBadGateway, "PROVIDER_DEPLOY", err.Error())
		return
	}
	status := normalizedProviderStatus(deploy.Status)
	if err := a.persistDeployment(
		in.PartnerID, in.Environment, in.Hostname, in.Release, in.Config,
		provider.Name(), serviceID, deploy, status, "",
	); err != nil {
		common.APIError(w, http.StatusConflict, "CONFLICT", "Runtime deployment or hostname conflicts with an existing assignment")
		return
	}

	code := http.StatusOK
	if status == "DEPLOYING" {
		code = http.StatusAccepted
	}
	common.JSON(w, code, map[string]any{
		"partner_id":          in.PartnerID,
		"environment":         in.Environment,
		"hostname":            in.Hostname,
		"release":             in.Release,
		"status":              status,
		"provider":            provider.Name(),
		"provider_service_id": serviceID,
		"provider_deploy_id":  deploy.ID,
		"provider_status":     deploy.Status,
	})
}

func (a *app) runtimeHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use GET")
		return
	}
	partnerID := strings.TrimSpace(r.URL.Query().Get("partner_id"))
	environment := validEnvironment(r.URL.Query().Get("environment"))
	if partnerID == "" || environment == "" {
		common.APIError(w, http.StatusBadRequest, "VALIDATION", "partner_id and valid environment are required")
		return
	}

	var hostname, release, status string
	var providerName, serviceID, deployID, providerStatus, providerError string
	var deployed time.Time
	err := a.db.QueryRow(`SELECT hostname,release,status,provider,provider_service_id,provider_deploy_id,provider_status,provider_error,deployed_at
		FROM runtime.deployments WHERE partner_id=$1 AND environment=$2`, partnerID, environment).
		Scan(&hostname, &release, &status, &providerName, &serviceID, &deployID, &providerStatus, &providerError, &deployed)
	if err != nil {
		common.APIError(w, http.StatusNotFound, "NOT_DEPLOYED", "Partner runtime is not deployed")
		return
	}

	if status == "DEPLOYING" && deployID != "" {
		provider, providerErr := a.provider(providerName)
		if providerErr != nil {
			common.APIError(w, http.StatusInternalServerError, "PROVIDER_CONFIG", providerErr.Error())
			return
		}
		current, providerErr := provider.Status(r.Context(), serviceID, deployID)
		if providerErr != nil {
			providerError = providerErr.Error()
			_, _ = a.db.Exec(`UPDATE runtime.deployments SET provider_error=$3,updated_at=NOW()
				WHERE partner_id=$1 AND environment=$2`, partnerID, environment, providerError)
			common.APIError(w, http.StatusBadGateway, "PROVIDER_STATUS", providerError)
			return
		}
		providerStatus = current.Status
		status = normalizedProviderStatus(providerStatus)
		providerError = ""
		_, _ = a.db.Exec(`UPDATE runtime.deployments SET status=$3,provider_status=$4,provider_error='',updated_at=NOW()
			WHERE partner_id=$1 AND environment=$2`, partnerID, environment, status, providerStatus)
	}

	code := http.StatusOK
	if status == "DEPLOYING" {
		code = http.StatusAccepted
	}
	if status == "FAILED" {
		code = http.StatusServiceUnavailable
	}
	common.JSON(w, code, map[string]any{
		"partner_id":          partnerID,
		"environment":         environment,
		"hostname":            hostname,
		"release":             release,
		"status":              status,
		"provider":            providerName,
		"provider_service_id": serviceID,
		"provider_deploy_id":  deployID,
		"provider_status":     providerStatus,
		"provider_error":      providerError,
		"deployed_at":         deployed,
		"checked_at":          time.Now().UTC(),
	})
}

func (a *app) summary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use GET")
		return
	}
	rows, err := a.db.Query(`SELECT partner_id,environment,hostname,release,status,provider,
			provider_service_id,provider_deploy_id,provider_status,provider_error,deployed_at,updated_at
		FROM runtime.deployments ORDER BY partner_id,environment`)
	if err != nil {
		common.APIError(w, http.StatusInternalServerError, "DB", "Could not load runtime summary")
		return
	}
	defer rows.Close()

	items := []map[string]any{}
	for rows.Next() {
		var partnerID, environment, hostname, release, status string
		var providerName, serviceID, deployID, providerStatus, providerError string
		var deployed, updated time.Time
		if rows.Scan(
			&partnerID, &environment, &hostname, &release, &status,
			&providerName, &serviceID, &deployID, &providerStatus, &providerError,
			&deployed, &updated,
		) == nil {
			items = append(items, map[string]any{
				"partner_id": partnerID, "environment": environment, "hostname": hostname,
				"release": release, "status": status, "provider": providerName,
				"provider_service_id": serviceID, "provider_deploy_id": deployID,
				"provider_status": providerStatus, "provider_error": providerError,
				"deployed_at": deployed, "updated_at": updated,
			})
		}
	}
	common.JSON(w, http.StatusOK, map[string]any{"items": items})
}
