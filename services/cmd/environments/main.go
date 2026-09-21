package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"fmt"
	"himate.local/services/internal/common"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

type app struct {
	db           *sql.DB
	token        string
	runtimeHost  string
	partnersHost string
	client       *http.Client
}

var kindValues = map[string]bool{"STAGING": true, "PRODUCTION": true}
var deploymentValues = map[string]bool{"NOT_DEPLOYED": true, "QUEUED": true, "DEPLOYING": true, "DEPLOYED": true, "FAILED": true}
var environmentValues = map[string]bool{"CREATING": true, "CONFIGURATION_REQUIRED": true, "TESTING": true, "READY": true, "READY_FOR_LAUNCH": true, "LIVE": true, "SUSPENDED": true, "FAILED": true}
var hostPart = regexp.MustCompile(`[^a-z0-9-]+`)
var publicHostname = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`)

type environment struct {
	ID, PartnerID, Kind, Hostname, PlatformVersion string
	DeploymentStatus, EnvironmentStatus            string
	DesiredRelease, ActiveRelease                  string
	ConfigJSON                                     []byte
	RuntimeStatus                                  string
	RuntimeLatencyMS                               int64
	LastHealthCheck                                sql.NullTime
	DNSStatus, TLSStatus, DomainStatus              string
	DomainError, LaunchActor                        string
	LastDomainCheck, LaunchedAt                     sql.NullTime
	CreatedAt, UpdatedAt                            time.Time
}

func main() {
	log := common.Logger()
	db, err := common.OpenDB()
	if err != nil { log.Error("database", "error", err); os.Exit(1) }
	defer db.Close()
	a := &app{
		db: db,
		token: os.Getenv("HIMATE_INTERNAL_TOKEN"),
		runtimeHost: os.Getenv("PARTNER_RUNTIME_HOSTPORT"),
		partnersHost: os.Getenv("PARTNERS_HOSTPORT"),
		client: &http.Client{Timeout: 20 * time.Second},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := a.migrate(ctx); err != nil { log.Error("migration", "error", err); os.Exit(1) }

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		common.JSON(w, 200, map[string]any{"status": "ok", "service": "environments", "time": time.Now().UTC()})
	})
	mux.HandleFunc("/api/v1/environments", a.environments)
	mux.HandleFunc("/api/v1/environments/", a.environmentByID)
	mux.HandleFunc("/internal/v1/environments/ensure-staging", a.ensureStaging)
	mux.HandleFunc("/internal/v1/environments/deploy-staging", a.deployStaging)
	mux.HandleFunc("/internal/v1/environments/runtime-health", a.runtimeHealth)
	mux.HandleFunc("/internal/v1/environments/summary", a.summary)
	common.Run(log, "environments", common.Env("PORT", "10000"), common.InternalAuth(a.token, mux))
}

func (a *app) migrate(ctx context.Context) error {
	return common.ApplyMigrations(ctx, a.db, "environments", []common.Migration{
		{Version: 1, Name: "partner-environments", Statements: []string{
			`CREATE SCHEMA IF NOT EXISTS environments`,
			`CREATE TABLE IF NOT EXISTS environments.partner_environments(
				id TEXT PRIMARY KEY,
				partner_id TEXT NOT NULL,
				kind TEXT NOT NULL,
				hostname TEXT NOT NULL,
				platform_version TEXT NOT NULL DEFAULT '',
				config JSONB NOT NULL DEFAULT '{}'::jsonb,
				deployment_status TEXT NOT NULL DEFAULT 'NOT_DEPLOYED',
				environment_status TEXT NOT NULL DEFAULT 'CREATING',
				desired_release TEXT NOT NULL DEFAULT '',
				active_release TEXT NOT NULL DEFAULT '',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				UNIQUE(partner_id,kind),
				UNIQUE(hostname)
			)`,
			`CREATE INDEX IF NOT EXISTS environments_partner_idx ON environments.partner_environments(partner_id)`,
			`CREATE INDEX IF NOT EXISTS environments_status_idx ON environments.partner_environments(environment_status,deployment_status)`,
		}},
		{Version: 2, Name: "runtime-health-state", Statements: []string{
			`ALTER TABLE environments.partner_environments ADD COLUMN IF NOT EXISTS runtime_status TEXT NOT NULL DEFAULT 'UNKNOWN'`,
			`ALTER TABLE environments.partner_environments ADD COLUMN IF NOT EXISTS runtime_latency_ms BIGINT NOT NULL DEFAULT 0`,
			`ALTER TABLE environments.partner_environments ADD COLUMN IF NOT EXISTS last_health_check TIMESTAMPTZ`,
		}},
		{Version: 3, Name: "domains-deployments-launch-gate", Statements: []string{
			`ALTER TABLE environments.partner_environments ADD COLUMN IF NOT EXISTS dns_status TEXT NOT NULL DEFAULT 'UNKNOWN'`,
			`ALTER TABLE environments.partner_environments ADD COLUMN IF NOT EXISTS tls_status TEXT NOT NULL DEFAULT 'UNKNOWN'`,
			`ALTER TABLE environments.partner_environments ADD COLUMN IF NOT EXISTS domain_status TEXT NOT NULL DEFAULT 'UNVERIFIED'`,
			`ALTER TABLE environments.partner_environments ADD COLUMN IF NOT EXISTS domain_error TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE environments.partner_environments ADD COLUMN IF NOT EXISTS last_domain_check TIMESTAMPTZ`,
			`ALTER TABLE environments.partner_environments ADD COLUMN IF NOT EXISTS launch_actor TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE environments.partner_environments ADD COLUMN IF NOT EXISTS launched_at TIMESTAMPTZ`,
			`CREATE INDEX IF NOT EXISTS environments_launch_idx ON environments.partner_environments(kind,environment_status,domain_status,deployment_status)`,
		}},
	})
}

func slug(v string) string {
	s := strings.Trim(hostPart.ReplaceAllString(strings.ToLower(strings.TrimSpace(v)), "-"), "-")
	if s == "" { return "partner" }
	return s
}

func envID(partnerID, kind string) string {
	return "env_" + strings.ToLower(kind) + "_" + strings.TrimPrefix(strings.ToLower(partnerID), "ptr_")
}

func nullableTime(v sql.NullTime) any {
	if !v.Valid { return nil }
	return v.Time.UTC()
}

func isInternalHostname(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return strings.HasSuffix(host, ".local") || host == "localhost"
}

func validPublicHostname(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return !isInternalHostname(host) && len(host) <= 253 && publicHostname.MatchString(host)
}

func launchReadiness(e environment) []string {
	reasons := []string{}
	if e.Kind != "PRODUCTION" { reasons = append(reasons, "environment must be PRODUCTION") }
	if e.DeploymentStatus != "DEPLOYED" { reasons = append(reasons, "production deployment is not DEPLOYED") }
	if e.RuntimeStatus != "OK" { reasons = append(reasons, "runtime health is not OK") }
	if strings.TrimSpace(e.ActiveRelease) == "" { reasons = append(reasons, "active release is missing") }
	if e.DomainStatus != "VERIFIED" { reasons = append(reasons, "domain is not VERIFIED") }
	if e.DNSStatus != "VERIFIED" { reasons = append(reasons, "DNS is not VERIFIED") }
	if e.TLSStatus != "VERIFIED" { reasons = append(reasons, "TLS is not VERIFIED") }
	return reasons
}

func mapEnvironment(e environment) map[string]any {
	return map[string]any{
		"id": e.ID, "partner_id": e.PartnerID, "kind": e.Kind, "hostname": e.Hostname,
		"platform_version": e.PlatformVersion, "config": common.JSONRawOrEmpty(e.ConfigJSON),
		"deployment_status": e.DeploymentStatus, "environment_status": e.EnvironmentStatus,
		"desired_release": e.DesiredRelease, "active_release": e.ActiveRelease,
		"runtime_status": e.RuntimeStatus, "runtime_latency_ms": e.RuntimeLatencyMS,
		"last_health_check": nullableTime(e.LastHealthCheck),
		"dns_status": e.DNSStatus, "tls_status": e.TLSStatus, "domain_status": e.DomainStatus,
		"domain_error": e.DomainError, "last_domain_check": nullableTime(e.LastDomainCheck),
		"launch_actor": e.LaunchActor, "launched_at": nullableTime(e.LaunchedAt),
		"launch_ready": len(launchReadiness(e)) == 0, "launch_blockers": launchReadiness(e),
		"created_at": e.CreatedAt, "updated_at": e.UpdatedAt,
	}
}

type scanner interface{ Scan(...any) error }

const envSelect = `SELECT id,partner_id,kind,hostname,platform_version,config,deployment_status,environment_status,desired_release,active_release,runtime_status,runtime_latency_ms,last_health_check,dns_status,tls_status,domain_status,domain_error,last_domain_check,launch_actor,launched_at,created_at,updated_at FROM environments.partner_environments`

func scanEnvironment(s scanner) (environment, error) {
	var e environment
	err := s.Scan(&e.ID,&e.PartnerID,&e.Kind,&e.Hostname,&e.PlatformVersion,&e.ConfigJSON,&e.DeploymentStatus,&e.EnvironmentStatus,&e.DesiredRelease,&e.ActiveRelease,&e.RuntimeStatus,&e.RuntimeLatencyMS,&e.LastHealthCheck,&e.DNSStatus,&e.TLSStatus,&e.DomainStatus,&e.DomainError,&e.LastDomainCheck,&e.LaunchActor,&e.LaunchedAt,&e.CreatedAt,&e.UpdatedAt)
	return e, err
}

func (a *app) environments(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		partnerID := strings.TrimSpace(r.URL.Query().Get("partner_id"))
		q := envSelect
		args := []any{}
		if partnerID != "" { q += " WHERE partner_id=$1"; args = append(args, partnerID) }
		q += " ORDER BY partner_id,kind"
		rows, err := a.db.Query(q,args...)
		if err != nil { common.APIError(w,500,"DB","Could not load environments"); return }
		defer rows.Close()
		items := []map[string]any{}
		for rows.Next() { if e, err := scanEnvironment(rows); err == nil { items = append(items,mapEnvironment(e)) } }
		common.JSON(w,200,map[string]any{"items":items,"count":len(items)})
	case http.MethodPost:
		var in struct {
			PartnerID string `json:"partner_id"`
			Kind string `json:"kind"`
			Hostname string `json:"hostname"`
			PlatformVersion string `json:"platform_version"`
			DesiredRelease string `json:"desired_release"`
			Config map[string]any `json:"config"`
		}
		if common.Decode(r,&in)!=nil || strings.TrimSpace(in.PartnerID)=="" { common.APIError(w,400,"VALIDATION","partner_id is required"); return }
		in.Kind = strings.ToUpper(strings.TrimSpace(in.Kind))
		if in.Kind=="" { in.Kind="STAGING" }
		if !kindValues[in.Kind] { common.APIError(w,400,"VALIDATION","Invalid environment kind"); return }
		if strings.TrimSpace(in.Hostname)=="" {
			if in.Kind=="PRODUCTION" { common.APIError(w,400,"VALIDATION","Production hostname is required"); return }
			in.Hostname = slug(in.PartnerID)+"-"+strings.ToLower(in.Kind)+".himate.local"
		}
		in.Hostname = strings.ToLower(strings.TrimSpace(in.Hostname))
		if in.Kind=="PRODUCTION" && !validPublicHostname(in.Hostname) {
			common.APIError(w,400,"VALIDATION","Production hostname must be a public DNS hostname"); return
		}
		raw, err := common.MarshalJSON(in.Config)
		if err != nil { common.APIError(w,400,"VALIDATION","Invalid config"); return }
		e := environment{ID:envID(in.PartnerID,in.Kind),PartnerID:in.PartnerID,Kind:in.Kind,Hostname:in.Hostname,PlatformVersion:strings.TrimSpace(in.PlatformVersion),DesiredRelease:strings.TrimSpace(in.DesiredRelease),ConfigJSON:raw}
		_, err = a.db.Exec(`INSERT INTO environments.partner_environments(id,partner_id,kind,hostname,platform_version,config,desired_release)
			VALUES($1,$2,$3,$4,$5,$6::jsonb,$7)`, e.ID,e.PartnerID,e.Kind,e.Hostname,e.PlatformVersion,string(e.ConfigJSON),e.DesiredRelease)
		if err != nil { common.APIError(w,409,"CONFLICT","Environment already exists or hostname is in use"); return }
		stored,_:=a.get(e.ID)
		common.JSON(w,201,mapEnvironment(stored))
	default:
		common.APIError(w,405,"METHOD","Use GET or POST")
	}
}

func (a *app) environmentByID(w http.ResponseWriter, r *http.Request) {
	rawPath := strings.Trim(strings.TrimPrefix(r.URL.Path,"/api/v1/environments/"),"/")
	parts := strings.Split(rawPath,"/")
	id := strings.TrimSpace(parts[0])
	if id=="" { common.APIError(w,404,"NOT_FOUND","Environment not found"); return }
	action := ""
	if len(parts)>1 { action = strings.TrimSpace(parts[1]) }
	if len(parts)>2 { common.APIError(w,404,"NOT_FOUND","Environment action not found"); return }

	switch action {
	case "verify-domain":
		a.verifyDomain(w,r,id); return
	case "deploy":
		a.deployEnvironment(w,r,id); return
	case "launch":
		a.launchEnvironment(w,r,id); return
	case "":
	default:
		common.APIError(w,404,"NOT_FOUND","Environment action not found"); return
	}

	if r.Method!=http.MethodPatch { common.APIError(w,405,"METHOD","Use PATCH"); return }
	e, err:=a.get(id)
	if err!=nil { common.APIError(w,404,"NOT_FOUND","Environment not found"); return }
	var in struct {
		Hostname *string `json:"hostname"`
		PlatformVersion *string `json:"platform_version"`
		DeploymentStatus *string `json:"deployment_status"`
		EnvironmentStatus *string `json:"environment_status"`
		DesiredRelease *string `json:"desired_release"`
		ActiveRelease *string `json:"active_release"`
		Config map[string]any `json:"config"`
	}
	if common.Decode(r,&in)!=nil { common.APIError(w,400,"JSON","Invalid request"); return }
	if in.Hostname!=nil {
		next:=strings.ToLower(strings.TrimSpace(*in.Hostname))
		if e.Kind=="PRODUCTION" && !validPublicHostname(next) {
			common.APIError(w,400,"VALIDATION","Production hostname must be a public DNS hostname"); return
		}
		if next!=e.Hostname {
			if e.Kind=="PRODUCTION" && e.EnvironmentStatus=="LIVE" {
				common.APIError(w,409,"LIVE_DOMAIN_LOCK","Suspend or move production out of LIVE before changing its hostname"); return
			}
			e.Hostname=next
			e.DNSStatus="UNKNOWN";e.TLSStatus="UNKNOWN";e.DomainStatus="UNVERIFIED";e.DomainError=""
			e.LastDomainCheck=sql.NullTime{}
			if e.Kind=="PRODUCTION" && e.EnvironmentStatus=="READY_FOR_LAUNCH" { e.EnvironmentStatus="CONFIGURATION_REQUIRED" }
		}
	}
	if in.PlatformVersion!=nil { e.PlatformVersion=strings.TrimSpace(*in.PlatformVersion) }
	if in.DeploymentStatus!=nil {
		v:=strings.ToUpper(strings.TrimSpace(*in.DeploymentStatus))
		if !deploymentValues[v] { common.APIError(w,400,"VALIDATION","Invalid deployment_status"); return }
		e.DeploymentStatus=v
	}
	if in.EnvironmentStatus!=nil {
		v:=strings.ToUpper(strings.TrimSpace(*in.EnvironmentStatus))
		if !environmentValues[v] { common.APIError(w,400,"VALIDATION","Invalid environment_status"); return }
		if v=="LIVE" { common.APIError(w,409,"LAUNCH_GATE","Use the launch action to move a production environment LIVE"); return }
		e.EnvironmentStatus=v
	}
	if in.DesiredRelease!=nil { e.DesiredRelease=strings.TrimSpace(*in.DesiredRelease) }
	if in.ActiveRelease!=nil { e.ActiveRelease=strings.TrimSpace(*in.ActiveRelease) }
	raw:=e.ConfigJSON
	if in.Config!=nil {
		raw,err=common.MarshalJSON(in.Config)
		if err!=nil { common.APIError(w,400,"VALIDATION","Invalid config"); return }
	}
	_,err=a.db.Exec(`UPDATE environments.partner_environments SET hostname=$2,platform_version=$3,config=$4::jsonb,deployment_status=$5,environment_status=$6,desired_release=$7,active_release=$8,dns_status=$9,tls_status=$10,domain_status=$11,domain_error=$12,last_domain_check=$13,updated_at=NOW() WHERE id=$1`,
		id,e.Hostname,e.PlatformVersion,string(raw),e.DeploymentStatus,e.EnvironmentStatus,e.DesiredRelease,e.ActiveRelease,e.DNSStatus,e.TLSStatus,e.DomainStatus,e.DomainError,e.LastDomainCheck)
	if err!=nil { common.APIError(w,409,"CONFLICT","Environment update failed"); return }
	e,_=a.get(id)
	common.JSON(w,200,mapEnvironment(e))
}

func (a *app) checkDomain(ctx context.Context, e environment) environment {
	now:=time.Now().UTC()
	e.LastDomainCheck=sql.NullTime{Time:now,Valid:true}
	e.DomainError=""

	if isInternalHostname(e.Hostname) {
		e.DNSStatus="NOT_APPLICABLE"
		e.TLSStatus="NOT_APPLICABLE"
		e.DomainStatus="INTERNAL"
		return e
	}

	dnsCtx,cancel:=context.WithTimeout(ctx,3*time.Second)
	defer cancel()
	if _,err:=net.DefaultResolver.LookupHost(dnsCtx,e.Hostname);err!=nil {
		e.DNSStatus="FAILED";e.TLSStatus="SKIPPED";e.DomainStatus="FAILED";e.DomainError="DNS: "+err.Error()
		return e
	}
	e.DNSStatus="VERIFIED"

	dialer:=&net.Dialer{Timeout:4*time.Second}
	conn,err:=tls.DialWithDialer(dialer,"tcp",net.JoinHostPort(e.Hostname,"443"),&tls.Config{
		ServerName:e.Hostname,MinVersion:tls.VersionTLS12,
	})
	if err!=nil {
		e.TLSStatus="FAILED";e.DomainStatus="FAILED";e.DomainError="TLS: "+err.Error()
		return e
	}
	defer conn.Close()
	state:=conn.ConnectionState()
	if len(state.PeerCertificates)==0 || time.Now().After(state.PeerCertificates[0].NotAfter) {
		e.TLSStatus="FAILED";e.DomainStatus="FAILED";e.DomainError="TLS certificate is missing or expired"
		return e
	}
	e.TLSStatus="VERIFIED"
	e.DomainStatus="VERIFIED"
	return e
}

func (a *app) persistDomainState(e environment) error {
	_,err:=a.db.Exec(`UPDATE environments.partner_environments
		SET dns_status=$2,tls_status=$3,domain_status=$4,domain_error=$5,last_domain_check=$6,environment_status=$7,updated_at=NOW()
		WHERE id=$1`,e.ID,e.DNSStatus,e.TLSStatus,e.DomainStatus,e.DomainError,e.LastDomainCheck,e.EnvironmentStatus)
	return err
}

func (a *app) verifyDomain(w http.ResponseWriter,r *http.Request,id string){
	if r.Method!=http.MethodPost { common.APIError(w,405,"METHOD","Use POST");return }
	e,err:=a.get(id)
	if err!=nil { common.APIError(w,404,"NOT_FOUND","Environment not found");return }
	e=a.checkDomain(r.Context(),e)
	if err:=a.persistDomainState(e);err!=nil { common.APIError(w,500,"DB","Could not persist domain verification");return }
	e,_=a.get(id)
	if e.Kind=="PRODUCTION" && e.EnvironmentStatus!="LIVE" && len(launchReadiness(e))==0 {
		actor:=strings.TrimSpace(r.Header.Get("X-Himate-User-ID"))
		if err:=a.transitionPartnerLifecycle(r.Context(),e.PartnerID,"READY_FOR_LAUNCH",actor,"Production DNS and TLS verification completed all launch gates");err!=nil {
			common.JSON(w,http.StatusConflict,map[string]any{"error":"PARTNER_LIFECYCLE","message":err.Error(),"environment":mapEnvironment(e)})
			return
		}
		_,_ = a.db.Exec(`UPDATE environments.partner_environments SET environment_status='READY_FOR_LAUNCH',updated_at=NOW() WHERE id=$1`,e.ID)
		e,_=a.get(id)
	}
	common.JSON(w,http.StatusOK,mapEnvironment(e))
}

func deploymentInProgressStatus(e environment) string {
	if e.Kind == "PRODUCTION" && e.EnvironmentStatus == "LIVE" {
		return "LIVE"
	}
	return "TESTING"
}

func runtimeProviderState(out map[string]any) string {
	return strings.ToUpper(strings.TrimSpace(fmt.Sprint(out["status"])))
}

func (a *app) finalizeDeployment(ctx context.Context, e environment, release, actor string, latency int64) (environment, error) {
	wasLive := e.Kind == "PRODUCTION" && e.EnvironmentStatus == "LIVE"
	nextStatus := "READY"
	if e.Kind == "PRODUCTION" {
		if wasLive {
			nextStatus = "LIVE"
		} else {
			nextStatus = "CONFIGURATION_REQUIRED"
		}
	}
	_, err := a.db.Exec(`UPDATE environments.partner_environments
		SET deployment_status='DEPLOYED',environment_status=$2,active_release=$3,
		    runtime_status='OK',runtime_latency_ms=$4,last_health_check=NOW(),updated_at=NOW()
		WHERE id=$1`, e.ID, nextStatus, release, latency)
	if err != nil {
		return e, err
	}
	e, err = a.get(e.ID)
	if err != nil {
		return e, err
	}
	if e.Kind == "PRODUCTION" && !wasLive {
		if err := a.transitionPartnerLifecycle(ctx, e.PartnerID, "TESTING", actor, "Production provider deployment completed; launch testing started"); err != nil {
			return e, fmt.Errorf("partner lifecycle TESTING: %w", err)
		}
		if len(launchReadiness(e)) == 0 {
			if err := a.transitionPartnerLifecycle(ctx, e.PartnerID, "READY_FOR_LAUNCH", actor, "Production deployment, runtime, DNS and TLS gates passed"); err != nil {
				return e, fmt.Errorf("partner lifecycle READY_FOR_LAUNCH: %w", err)
			}
			_, _ = a.db.Exec(`UPDATE environments.partner_environments
				SET environment_status='READY_FOR_LAUNCH',updated_at=NOW() WHERE id=$1`, e.ID)
			e, _ = a.get(e.ID)
		}
	}
	return e, nil
}

func (a *app) deployRecord(ctx context.Context, e environment, release, actor string) (environment, error) {
	wasLive := e.Kind == "PRODUCTION" && e.EnvironmentStatus == "LIVE"
	release = strings.TrimSpace(release)
	if release == "" { release = e.DesiredRelease }
	if release == "" { release = e.PlatformVersion }
	if release == "" { release = "current" }

	inProgressStatus := deploymentInProgressStatus(e)
	_, _ = a.db.Exec(`UPDATE environments.partner_environments
		SET deployment_status='DEPLOYING',environment_status=$2,runtime_status='CHECKING',updated_at=NOW()
		WHERE id=$1`, e.ID, inProgressStatus)

	var out map[string]any
	latency, err := a.runtimeRequest(ctx, http.MethodPost, "/internal/v1/runtime/deploy", map[string]any{
		"partner_id": e.PartnerID,
		"environment": e.Kind,
		"hostname": e.Hostname,
		"release": release,
		"config": common.JSONRawOrEmpty(e.ConfigJSON),
	}, &out)
	if err != nil {
		failureStatus := "FAILED"
		if wasLive { failureStatus = "LIVE" }
		_, _ = a.db.Exec(`UPDATE environments.partner_environments
			SET deployment_status='FAILED',environment_status=$2,runtime_status='ERROR',
			    runtime_latency_ms=$3,last_health_check=NOW(),updated_at=NOW()
			WHERE id=$1`, e.ID, failureStatus, latency)
		failed, _ := a.get(e.ID)
		return failed, err
	}

	switch runtimeProviderState(out) {
	case "READY":
		return a.finalizeDeployment(ctx, e, release, actor, latency)
	case "DEPLOYING":
		_, _ = a.db.Exec(`UPDATE environments.partner_environments
			SET deployment_status='DEPLOYING',runtime_status='DEPLOYING',
			    runtime_latency_ms=$2,last_health_check=NOW(),updated_at=NOW()
			WHERE id=$1`, e.ID, latency)
		pending, _ := a.get(e.ID)
		return pending, nil
	default:
		failureStatus := "FAILED"
		if wasLive { failureStatus = "LIVE" }
		_, _ = a.db.Exec(`UPDATE environments.partner_environments
			SET deployment_status='FAILED',environment_status=$2,runtime_status='ERROR',
			    runtime_latency_ms=$3,last_health_check=NOW(),updated_at=NOW()
			WHERE id=$1`, e.ID, failureStatus, latency)
		failed, _ := a.get(e.ID)
		return failed, fmt.Errorf("provider deployment returned unexpected state %q", runtimeProviderState(out))
	}
}

func (a *app) deployEnvironment(w http.ResponseWriter,r *http.Request,id string){
	if r.Method!=http.MethodPost { common.APIError(w,405,"METHOD","Use POST");return }
	e,err:=a.get(id)
	if err!=nil { common.APIError(w,404,"NOT_FOUND","Environment not found");return }
	var in struct{ Release string `json:"release"` }
	if r.Body!=nil && r.ContentLength!=0 {
		if common.Decode(r,&in)!=nil { common.APIError(w,400,"JSON","Invalid request");return }
	}
	e,err=a.deployRecord(r.Context(),e,in.Release,strings.TrimSpace(r.Header.Get("X-Himate-User-ID")))
	if err!=nil { common.APIError(w,502,"RUNTIME_DEPLOY",err.Error());return }
	common.JSON(w,200,mapEnvironment(e))
}

func (a *app) launchEnvironment(w http.ResponseWriter,r *http.Request,id string){
	if r.Method!=http.MethodPost { common.APIError(w,405,"METHOD","Use POST");return }
	e,err:=a.get(id)
	if err!=nil { common.APIError(w,404,"NOT_FOUND","Environment not found");return }
	if e.Kind!="PRODUCTION" { common.APIError(w,409,"LAUNCH_GATE","Only PRODUCTION environments can go LIVE");return }
	actor:=strings.TrimSpace(r.Header.Get("X-Himate-User-ID"))
	e,probeErr:=a.probeRuntime(r.Context(),e,actor)
	if probeErr!=nil { common.APIError(w,409,"LAUNCH_GATE","Production runtime health check failed");return }
	blockers:=launchReadiness(e)
	if len(blockers)>0 {
		common.JSON(w,http.StatusConflict,map[string]any{"error":"LAUNCH_GATE","message":"Production is not ready for launch","blockers":blockers,"environment":mapEnvironment(e)})
		return
	}
	if err:=a.transitionPartnerLifecycle(r.Context(),e.PartnerID,"LIVE",actor,"Production launch gate approved; environment is LIVE");err!=nil {
		common.JSON(w,http.StatusConflict,map[string]any{"error":"PARTNER_LIFECYCLE","message":err.Error(),"environment":mapEnvironment(e)})
		return
	}
	_,err=a.db.Exec(`UPDATE environments.partner_environments SET environment_status='LIVE',launch_actor=$2,launched_at=NOW(),updated_at=NOW() WHERE id=$1`,id,actor)
	if err!=nil { common.APIError(w,500,"DB","Could not finalize production launch");return }
	e,_=a.get(id)
	common.JSON(w,200,mapEnvironment(e))
}

func (a *app) ensureStaging(w http.ResponseWriter, r *http.Request) {
	if r.Method!=http.MethodPost { common.APIError(w,405,"METHOD","Use POST"); return }
	var in struct {
		PartnerID string `json:"partner_id"`
		SystemName string `json:"system_name"`
		PlatformVersion string `json:"platform_version"`
		DesiredRelease string `json:"desired_release"`
		Config map[string]any `json:"config"`
	}
	if common.Decode(r,&in)!=nil || strings.TrimSpace(in.PartnerID)=="" { common.APIError(w,400,"VALIDATION","partner_id is required"); return }
	host := slug(in.SystemName)
	if host=="partner" { host=slug(in.PartnerID) }
	partnerSuffix := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(in.PartnerID)), "ptr_")
	if partnerSuffix != "" && !strings.HasSuffix(host, "-"+partnerSuffix) { host += "-" + partnerSuffix }
	host += ".himate-staging.local"
	raw,_:=common.MarshalJSON(in.Config)
	id:=envID(in.PartnerID,"STAGING")
	_,err:=a.db.Exec(`INSERT INTO environments.partner_environments(id,partner_id,kind,hostname,platform_version,config,deployment_status,environment_status,desired_release)
		VALUES($1,$2,'STAGING',$3,$4,$5::jsonb,'NOT_DEPLOYED','CONFIGURATION_REQUIRED',$6)
		ON CONFLICT(partner_id,kind) DO UPDATE SET
			platform_version=CASE WHEN EXCLUDED.platform_version<>'' THEN EXCLUDED.platform_version ELSE environments.partner_environments.platform_version END,
			config=CASE WHEN EXCLUDED.config<>'{}'::jsonb THEN EXCLUDED.config ELSE environments.partner_environments.config END,
			desired_release=CASE WHEN EXCLUDED.desired_release<>'' THEN EXCLUDED.desired_release ELSE environments.partner_environments.desired_release END,
			updated_at=NOW()`,
		id,in.PartnerID,host,strings.TrimSpace(in.PlatformVersion),string(raw),strings.TrimSpace(in.DesiredRelease))
	if err!=nil { common.APIError(w,500,"DB","Could not ensure staging environment"); return }
	e,_:=a.get(id)
	common.JSON(w,200,mapEnvironment(e))
}

func (a *app) runtimeRequest(ctx context.Context, method, path string, payload any, dst any) (int64,error) {
	if strings.TrimSpace(a.runtimeHost)=="" { return 0,fmt.Errorf("partner runtime is not configured") }
	var body *bytes.Reader
	if payload==nil { body=bytes.NewReader(nil) } else {
		raw,err:=json.Marshal(payload); if err!=nil{return 0,err}; body=bytes.NewReader(raw)
	}
	req,err:=http.NewRequestWithContext(ctx,method,"http://"+a.runtimeHost+path,body)
	if err!=nil{return 0,err}
	req.Header.Set("X-Himate-Internal-Token",a.token)
	if payload!=nil{req.Header.Set("Content-Type","application/json")}
	started:=time.Now()
	resp,err:=a.client.Do(req)
	latency:=time.Since(started).Milliseconds()
	if err!=nil{return latency,err}
	defer resp.Body.Close()
	if resp.StatusCode<200||resp.StatusCode>=300{return latency,fmt.Errorf("runtime status %d",resp.StatusCode)}
	if dst!=nil{return latency,json.NewDecoder(resp.Body).Decode(dst)}
	return latency,nil
}

func (a *app) partnerJSON(ctx context.Context,method,path string,payload any,actor string,dst any) error {
	if strings.TrimSpace(a.partnersHost)=="" { return fmt.Errorf("partners service is not configured") }
	var body *bytes.Reader
	if payload==nil { body=bytes.NewReader(nil) } else {
		raw,err:=json.Marshal(payload); if err!=nil{return err}; body=bytes.NewReader(raw)
	}
	req,err:=http.NewRequestWithContext(ctx,method,"http://"+a.partnersHost+path,body)
	if err!=nil{return err}
	req.Header.Set("X-Himate-Internal-Token",a.token)
	if strings.TrimSpace(actor)!="" { req.Header.Set("X-Himate-User-ID",strings.TrimSpace(actor)) }
	if payload!=nil { req.Header.Set("Content-Type","application/json") }
	resp,err:=a.client.Do(req)
	if err!=nil{return err}
	defer resp.Body.Close()
	if resp.StatusCode<200||resp.StatusCode>=300 {
		var apiErr map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		return fmt.Errorf("partners status %d: %v",resp.StatusCode,apiErr)
	}
	if dst!=nil{return json.NewDecoder(resp.Body).Decode(dst)}
	return nil
}

func (a *app) partnerLifecycle(ctx context.Context,partnerID string)(string,error){
	var out map[string]any
	if err:=a.partnerJSON(ctx,http.MethodGet,"/api/v1/partners/"+url.PathEscape(partnerID),nil,"",&out);err!=nil{return "",err}
	return strings.ToUpper(strings.TrimSpace(fmt.Sprint(out["lifecycle"]))),nil
}

func (a *app) transitionPartnerLifecycle(ctx context.Context,partnerID,target,actor,reason string) error {
	target=strings.ToUpper(strings.TrimSpace(target))
	current,err:=a.partnerLifecycle(ctx,partnerID)
	if err!=nil{return err}
	if current==target{return nil}

	allowed:=false
	switch target {
	case "TESTING":
		allowed=current=="CONFIGURATION"
		if current=="READY_FOR_LAUNCH"||current=="LIVE"{return nil}
	case "READY_FOR_LAUNCH":
		allowed=current=="TESTING"
		if current=="LIVE"{return nil}
	case "LIVE":
		allowed=current=="READY_FOR_LAUNCH"
	default:
		return fmt.Errorf("unsupported lifecycle target %s",target)
	}
	if !allowed{return fmt.Errorf("partner lifecycle %s cannot transition to %s",current,target)}

	payload:=map[string]any{"lifecycle":target,"reason":reason}
	var out map[string]any
	if err:=a.partnerJSON(ctx,http.MethodPatch,"/api/v1/partners/"+url.PathEscape(partnerID),payload,actor,&out);err!=nil{
		// The PATCH may have committed before a transport failure. Re-read once
		// and treat the transition as successful when the target is already live.
		if state,readErr:=a.partnerLifecycle(ctx,partnerID);readErr==nil&&state==target{return nil}
		return err
	}
	return nil
}

func (a *app) deployStaging(w http.ResponseWriter, r *http.Request) {
	if r.Method!=http.MethodPost { common.APIError(w,405,"METHOD","Use POST"); return }
	var in struct {
		PartnerID string `json:"partner_id"`
		Release string `json:"release"`
	}
	if common.Decode(r,&in)!=nil || strings.TrimSpace(in.PartnerID)=="" { common.APIError(w,400,"VALIDATION","partner_id is required"); return }
	id:=envID(in.PartnerID,"STAGING")
	e,err:=a.get(id)
	if err!=nil { common.APIError(w,404,"NOT_FOUND","Staging environment not found"); return }
	release:=strings.TrimSpace(in.Release)
	if release==""{release=e.DesiredRelease}
	if release==""{release=e.PlatformVersion}
	if release==""{release="current"}
	e,err=a.deployRecord(r.Context(),e,release,strings.TrimSpace(r.Header.Get("X-Himate-User-ID")))
	if err!=nil{common.APIError(w,502,"RUNTIME_DEPLOY",err.Error());return}
	common.JSON(w,200,mapEnvironment(e))
}

func (a *app) probeRuntime(ctx context.Context, e environment, actor string) (environment, error) {
	path := "/internal/v1/runtime/health?partner_id="+url.QueryEscape(e.PartnerID)+"&environment="+url.QueryEscape(e.Kind)
	var out map[string]any
	latency, probeErr := a.runtimeRequest(ctx, http.MethodGet, path, nil, &out)
	if probeErr != nil {
		_, _ = a.db.Exec(`UPDATE environments.partner_environments
			SET runtime_status='ERROR',runtime_latency_ms=$2,last_health_check=NOW(),updated_at=NOW()
			WHERE id=$1`, e.ID, latency)
		e.RuntimeStatus = "ERROR"
		e.RuntimeLatencyMS = latency
		e.LastHealthCheck = sql.NullTime{Time: time.Now().UTC(), Valid: true}
		return e, probeErr
	}

	switch runtimeProviderState(out) {
	case "DEPLOYING":
		_, _ = a.db.Exec(`UPDATE environments.partner_environments
			SET deployment_status='DEPLOYING',runtime_status='DEPLOYING',
			    runtime_latency_ms=$2,last_health_check=NOW(),updated_at=NOW()
			WHERE id=$1`, e.ID, latency)
		pending, _ := a.get(e.ID)
		return pending, nil
	case "READY":
		release := strings.TrimSpace(fmt.Sprint(out["release"]))
		if release == "" { release = e.DesiredRelease }
		if release == "" { release = e.PlatformVersion }
		if release == "" { release = "current" }
		if e.DeploymentStatus != "DEPLOYED" || e.RuntimeStatus != "OK" || strings.TrimSpace(e.ActiveRelease) != release {
			return a.finalizeDeployment(ctx, e, release, actor, latency)
		}
		_, _ = a.db.Exec(`UPDATE environments.partner_environments
			SET runtime_status='OK',runtime_latency_ms=$2,last_health_check=NOW(),updated_at=NOW()
			WHERE id=$1`, e.ID, latency)
		ready, _ := a.get(e.ID)
		return ready, nil
	default:
		_, _ = a.db.Exec(`UPDATE environments.partner_environments
			SET deployment_status='FAILED',runtime_status='ERROR',
			    runtime_latency_ms=$2,last_health_check=NOW(),updated_at=NOW()
			WHERE id=$1`, e.ID, latency)
		failed, _ := a.get(e.ID)
		return failed, fmt.Errorf("provider runtime returned unexpected state %q", runtimeProviderState(out))
	}
}

func (a *app) runtimeHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method!=http.MethodGet { common.APIError(w,405,"METHOD","Use GET"); return }
	partnerID:=strings.TrimSpace(r.URL.Query().Get("partner_id"))
	kind:=strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("environment")))
	if !kindValues[kind] { common.APIError(w,400,"VALIDATION","environment must be STAGING or PRODUCTION");return }
	e,err:=a.get(envID(partnerID,kind))
	if err!=nil{common.APIError(w,404,"NOT_FOUND","Environment not found");return}
	e,probeErr:=a.probeRuntime(r.Context(),e,strings.TrimSpace(r.Header.Get("X-Himate-User-ID")))
	if probeErr!=nil{common.JSON(w,503,map[string]any{"partner_id":partnerID,"environment":kind,"hostname":e.Hostname,"status":"ERROR","latency_ms":e.RuntimeLatencyMS,"error":probeErr.Error()});return}
	if e.RuntimeStatus=="DEPLOYING"{common.JSON(w,http.StatusAccepted,map[string]any{"partner_id":partnerID,"environment":kind,"hostname":e.Hostname,"status":"DEPLOYING","latency_ms":e.RuntimeLatencyMS,"active_release":e.ActiveRelease,"checked_at":time.Now().UTC()});return}
	common.JSON(w,200,map[string]any{"partner_id":partnerID,"environment":kind,"hostname":e.Hostname,"status":"OK","hostname_status":"REACHABLE","latency_ms":e.RuntimeLatencyMS,"active_release":e.ActiveRelease,"checked_at":time.Now().UTC()})
}

func (a *app) summary(w http.ResponseWriter, r *http.Request) {
	if r.Method!=http.MethodGet { common.APIError(w,405,"METHOD","Use GET"); return }
	rows,err:=a.db.Query(envSelect+" ORDER BY partner_id,kind")
	if err!=nil { common.APIError(w,500,"DB","Could not load environment summary"); return }
	defer rows.Close()
	items:=[]map[string]any{}
	for rows.Next() {
		if e,err:=scanEnvironment(rows);err==nil {
			items=append(items,map[string]any{
				"id":e.ID,"partner_id":e.PartnerID,"kind":e.Kind,"hostname":e.Hostname,
				"platform_version":e.PlatformVersion,"deployment_status":e.DeploymentStatus,
				"environment_status":e.EnvironmentStatus,"active_release":e.ActiveRelease,
				"runtime_status":e.RuntimeStatus,"runtime_latency_ms":e.RuntimeLatencyMS,
				"dns_status":e.DNSStatus,"tls_status":e.TLSStatus,"domain_status":e.DomainStatus,
				"launch_ready":len(launchReadiness(e))==0,
				"last_health_check":nullableTime(e.LastHealthCheck),"last_domain_check":nullableTime(e.LastDomainCheck),"updated_at":e.UpdatedAt,
			})
		}
	}
	common.JSON(w,200,map[string]any{"items":items})
}

func (a *app) get(id string)(environment,error){
	return scanEnvironment(a.db.QueryRow(envSelect+" WHERE id=$1",id))
}
