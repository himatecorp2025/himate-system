package main

import (
	"context"
	"database/sql"
	"fmt"
	"himate.local/services/internal/common"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

type app struct{ db *sql.DB }

var kindValues = map[string]bool{"STAGING": true, "PRODUCTION": true}
var deploymentValues = map[string]bool{"NOT_DEPLOYED": true, "QUEUED": true, "DEPLOYING": true, "DEPLOYED": true, "FAILED": true}
var environmentValues = map[string]bool{"CREATING": true, "CONFIGURATION_REQUIRED": true, "TESTING": true, "READY": true, "LIVE": true, "SUSPENDED": true, "FAILED": true}
var hostPart = regexp.MustCompile(`[^a-z0-9-]+`)

type environment struct {
	ID, PartnerID, Kind, Hostname, PlatformVersion string
	DeploymentStatus, EnvironmentStatus            string
	DesiredRelease, ActiveRelease                  string
	ConfigJSON                                     []byte
	CreatedAt, UpdatedAt                           time.Time
}

func main() {
	log := common.Logger()
	db, err := common.OpenDB()
	if err != nil {
		log.Error("database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	a := &app{db: db}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := a.migrate(ctx); err != nil {
		log.Error("migration", "error", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		common.JSON(w, 200, map[string]any{"status": "ok", "service": "environments", "time": time.Now().UTC()})
	})
	mux.HandleFunc("/api/v1/environments", a.environments)
	mux.HandleFunc("/api/v1/environments/", a.environmentByID)
	mux.HandleFunc("/internal/v1/environments/ensure-staging", a.ensureStaging)
	mux.HandleFunc("/internal/v1/environments/summary", a.summary)
	common.Run(log, "environments", common.Env("PORT", "10000"), common.InternalAuth(os.Getenv("HIMATE_INTERNAL_TOKEN"), mux))
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

func mapEnvironment(e environment) map[string]any {
	return map[string]any{
		"id": e.ID, "partner_id": e.PartnerID, "kind": e.Kind, "hostname": e.Hostname,
		"platform_version": e.PlatformVersion, "config": common.JSONRawOrEmpty(e.ConfigJSON),
		"deployment_status": e.DeploymentStatus, "environment_status": e.EnvironmentStatus,
		"desired_release": e.DesiredRelease, "active_release": e.ActiveRelease,
		"created_at": e.CreatedAt, "updated_at": e.UpdatedAt,
	}
}

type scanner interface{ Scan(...any) error }

const envSelect = `SELECT id,partner_id,kind,hostname,platform_version,config,deployment_status,environment_status,desired_release,active_release,created_at,updated_at FROM environments.partner_environments`

func scanEnvironment(s scanner) (environment, error) {
	var e environment
	err := s.Scan(&e.ID,&e.PartnerID,&e.Kind,&e.Hostname,&e.PlatformVersion,&e.ConfigJSON,&e.DeploymentStatus,&e.EnvironmentStatus,&e.DesiredRelease,&e.ActiveRelease,&e.CreatedAt,&e.UpdatedAt)
	return e, err
}

func (a *app) environments(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		partnerID := strings.TrimSpace(r.URL.Query().Get("partner_id"))
		q := envSelect
		args := []any{}
		if partnerID != "" {
			q += " WHERE partner_id=$1"
			args = append(args, partnerID)
		}
		q += " ORDER BY partner_id,kind"
		rows, err := a.db.Query(q,args...)
		if err != nil { common.APIError(w,500,"DB","Could not load environments"); return }
		defer rows.Close()
		items := []map[string]any{}
		for rows.Next() {
			if e, err := scanEnvironment(rows); err == nil { items = append(items,mapEnvironment(e)) }
		}
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
			in.Hostname = slug(in.PartnerID)+"-"+strings.ToLower(in.Kind)+".himate.local"
		}
		raw, err := common.MarshalJSON(in.Config)
		if err != nil { common.APIError(w,400,"VALIDATION","Invalid config"); return }
		e := environment{ID:envID(in.PartnerID,in.Kind),PartnerID:in.PartnerID,Kind:in.Kind,Hostname:strings.ToLower(strings.TrimSpace(in.Hostname)),PlatformVersion:strings.TrimSpace(in.PlatformVersion),DesiredRelease:strings.TrimSpace(in.DesiredRelease),ConfigJSON:raw}
		_, err = a.db.Exec(`INSERT INTO environments.partner_environments(id,partner_id,kind,hostname,platform_version,config,desired_release)
			VALUES($1,$2,$3,$4,$5,$6::jsonb,$7)`,
			e.ID,e.PartnerID,e.Kind,e.Hostname,e.PlatformVersion,string(e.ConfigJSON),e.DesiredRelease)
		if err != nil { common.APIError(w,409,"CONFLICT","Environment already exists or hostname is in use"); return }
		stored,_:=a.get(e.ID)
		common.JSON(w,201,mapEnvironment(stored))
	default:
		common.APIError(w,405,"METHOD","Use GET or POST")
	}
}

func (a *app) environmentByID(w http.ResponseWriter, r *http.Request) {
	id := strings.Trim(strings.TrimPrefix(r.URL.Path,"/api/v1/environments/"),"/")
	if id=="" || strings.Contains(id,"/") { common.APIError(w,404,"NOT_FOUND","Environment not found"); return }
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
	if in.Hostname!=nil { e.Hostname=strings.ToLower(strings.TrimSpace(*in.Hostname)) }
	if in.PlatformVersion!=nil { e.PlatformVersion=strings.TrimSpace(*in.PlatformVersion) }
	if in.DeploymentStatus!=nil {
		v:=strings.ToUpper(strings.TrimSpace(*in.DeploymentStatus))
		if !deploymentValues[v] { common.APIError(w,400,"VALIDATION","Invalid deployment_status"); return }
		e.DeploymentStatus=v
	}
	if in.EnvironmentStatus!=nil {
		v:=strings.ToUpper(strings.TrimSpace(*in.EnvironmentStatus))
		if !environmentValues[v] { common.APIError(w,400,"VALIDATION","Invalid environment_status"); return }
		e.EnvironmentStatus=v
	}
	if in.DesiredRelease!=nil { e.DesiredRelease=strings.TrimSpace(*in.DesiredRelease) }
	if in.ActiveRelease!=nil { e.ActiveRelease=strings.TrimSpace(*in.ActiveRelease) }
	raw:=e.ConfigJSON
	if in.Config!=nil {
		raw,err=common.MarshalJSON(in.Config)
		if err!=nil { common.APIError(w,400,"VALIDATION","Invalid config"); return }
	}
	_,err=a.db.Exec(`UPDATE environments.partner_environments SET hostname=$2,platform_version=$3,config=$4::jsonb,deployment_status=$5,environment_status=$6,desired_release=$7,active_release=$8,updated_at=NOW() WHERE id=$1`,
		id,e.Hostname,e.PlatformVersion,string(raw),e.DeploymentStatus,e.EnvironmentStatus,e.DesiredRelease,e.ActiveRelease)
	if err!=nil { common.APIError(w,409,"CONFLICT","Environment update failed"); return }
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
				"environment_status":e.EnvironmentStatus,"active_release":e.ActiveRelease,"updated_at":e.UpdatedAt,
			})
		}
	}
	common.JSON(w,200,map[string]any{"items":items})
}

func (a *app) get(id string)(environment,error){
	return scanEnvironment(a.db.QueryRow(envSelect+" WHERE id=$1",id))
}

var _ = fmt.Sprintf
