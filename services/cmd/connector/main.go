package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"himate.local/services/internal/common"
	"net/http"
	"os"
	"strings"
	"time"
)

type app struct {
	db *sql.DB
	internalToken string
	impactHost string
	client *http.Client
}

type credential struct {
	PartnerID, Environment, CredentialID, TokenHash string
	Active bool
	CreatedAt, RotatedAt time.Time
	LastUsedAt sql.NullTime
}

func main() {
	log:=common.Logger()
	db,err:=common.OpenDB()
	if err!=nil { log.Error("database","error",err); os.Exit(1) }
	defer db.Close()
	a:=&app{
		db:db,
		internalToken:os.Getenv("HIMATE_INTERNAL_TOKEN"),
		impactHost:os.Getenv("IMPACT_HOSTPORT"),
		client:&http.Client{Timeout:6*time.Second},
	}
	ctx,cancel:=context.WithTimeout(context.Background(),30*time.Second)
	defer cancel()
	if err:=a.migrate(ctx);err!=nil { log.Error("migration","error",err);os.Exit(1) }

	publicMux:=http.NewServeMux()
	publicMux.HandleFunc("/health",func(w http.ResponseWriter,r *http.Request){
		common.JSON(w,200,map[string]any{"status":"ok","service":"connector","time":time.Now().UTC()})
	})
	publicMux.HandleFunc("/connector/v1/heartbeat",a.heartbeat)
	publicMux.HandleFunc("/connector/v1/state",a.state)
	publicMux.HandleFunc("/connector/v1/metrics",a.metrics)

	privateMux:=http.NewServeMux()
	privateMux.HandleFunc("/api/v1/connectors/",a.adminConnector)
	privateMux.HandleFunc("/internal/v1/connectors/summary",a.summary)
	privateMux.HandleFunc("/internal/v1/connectors/ensure",a.ensureCredential)
	privateHandler:=common.InternalAuth(a.internalToken,privateMux)

	root:=http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
		if strings.HasPrefix(r.URL.Path,"/api/") || strings.HasPrefix(r.URL.Path,"/internal/") {
			privateHandler.ServeHTTP(w,r)
			return
		}
		publicMux.ServeHTTP(w,r)
	})
	common.Run(log,"connector",common.Env("PORT","10000"),root)
}

func (a *app) migrate(ctx context.Context) error {
	return common.ApplyMigrations(ctx,a.db,"connector",[]common.Migration{
		{Version:1,Name:"connector-protocol",Statements:[]string{
			`CREATE SCHEMA IF NOT EXISTS connector`,
			`CREATE TABLE IF NOT EXISTS connector.credentials(
				partner_id TEXT NOT NULL,
				environment TEXT NOT NULL,
				credential_id TEXT NOT NULL UNIQUE,
				token_hash TEXT NOT NULL,
				active BOOLEAN NOT NULL DEFAULT TRUE,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				rotated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				last_used_at TIMESTAMPTZ,
				PRIMARY KEY(partner_id,environment)
			)`,
			`CREATE TABLE IF NOT EXISTS connector.partner_state(
				partner_id TEXT NOT NULL,
				environment TEXT NOT NULL DEFAULT 'PRODUCTION',
				reported_version TEXT NOT NULL DEFAULT '',
				health TEXT NOT NULL DEFAULT 'UNKNOWN',
				module_state JSONB NOT NULL DEFAULT '{}'::jsonb,
				last_seen_at TIMESTAMPTZ,
				last_metric_sync_at TIMESTAMPTZ,
				last_error TEXT NOT NULL DEFAULT '',
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				PRIMARY KEY(partner_id,environment)
			)`,
			`CREATE INDEX IF NOT EXISTS connector_state_health_idx ON connector.partner_state(health,last_seen_at)`,
		}},
	})
}

func tokenHash(token string) string {
	sum:=sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func randomToken(partnerID,environment string)(credentialID,token string,error error){
	idRaw:=make([]byte,9)
	secret:=make([]byte,32)
	if _,error=rand.Read(idRaw);error!=nil{return}
	if _,error=rand.Read(secret);error!=nil{return}
	credentialID="crd_"+base64.RawURLEncoding.EncodeToString(idRaw)
	token="hmc_"+credentialID+"_"+base64.RawURLEncoding.EncodeToString(secret)
	return
}

func normalizeEnvironment(v string) string {
	v=strings.ToUpper(strings.TrimSpace(v))
	if v!="STAGING" && v!="PRODUCTION" { return "" }
	return v
}

func (a *app) issueCredential(ctx context.Context,partnerID,environment string)(map[string]any,error){
	environment=normalizeEnvironment(environment)
	if strings.TrimSpace(partnerID)=="" || environment=="" { return nil,fmt.Errorf("partner_id and STAGING/PRODUCTION environment are required") }
	credentialID,token,err:=randomToken(partnerID,environment)
	if err!=nil{return nil,err}
	hash:=tokenHash(token)
	_,err=a.db.ExecContext(ctx,`INSERT INTO connector.credentials(partner_id,environment,credential_id,token_hash,active)
		VALUES($1,$2,$3,$4,TRUE)
		ON CONFLICT(partner_id,environment) DO UPDATE SET credential_id=EXCLUDED.credential_id,token_hash=EXCLUDED.token_hash,active=TRUE,rotated_at=NOW()`,
		partnerID,environment,credentialID,hash)
	if err!=nil{return nil,err}
	_,_ = a.db.ExecContext(ctx,`INSERT INTO connector.partner_state(partner_id,environment) VALUES($1,$2)
		ON CONFLICT(partner_id,environment) DO UPDATE SET updated_at=NOW()`,partnerID,environment)
	return map[string]any{
		"partner_id":partnerID,"environment":environment,"credential_id":credentialID,
		"token":token,"token_returned_once":true,"rotated_at":time.Now().UTC(),
	},nil
}

func (a *app) adminConnector(w http.ResponseWriter,r *http.Request){
	path:=strings.Trim(strings.TrimPrefix(r.URL.Path,"/api/v1/connectors/"),"/")
	parts:=strings.Split(path,"/")
	if len(parts)!=2 || parts[0]=="" || parts[1]!="credential" { common.APIError(w,404,"NOT_FOUND","Connector route not found");return }
	partnerID:=parts[0]
	switch r.Method {
	case http.MethodPost:
		var in struct{ Environment string `json:"environment"` }
		if common.Decode(r,&in)!=nil { common.APIError(w,400,"JSON","Invalid request");return }
		out,err:=a.issueCredential(r.Context(),partnerID,in.Environment)
		if err!=nil { common.APIError(w,400,"VALIDATION",err.Error());return }
		common.JSON(w,201,out)
	case http.MethodGet:
		rows,err:=a.db.Query(`SELECT partner_id,environment,credential_id,active,created_at,rotated_at,last_used_at FROM connector.credentials WHERE partner_id=$1 ORDER BY environment`,partnerID)
		if err!=nil { common.APIError(w,500,"DB","Could not load connector credentials");return }
		defer rows.Close()
		items:=[]map[string]any{}
		for rows.Next(){
			var p,e,id string
			var active bool
			var created,rotated time.Time
			var last sql.NullTime
			if rows.Scan(&p,&e,&id,&active,&created,&rotated,&last)==nil {
				var used any
				if last.Valid { used=last.Time.UTC() }
				items=append(items,map[string]any{"partner_id":p,"environment":e,"credential_id":id,"active":active,"created_at":created,"rotated_at":rotated,"last_used_at":used})
			}
		}
		common.JSON(w,200,map[string]any{"items":items})
	default:
		common.APIError(w,405,"METHOD","Use GET or POST")
	}
}

func (a *app) ensureCredential(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodPost { common.APIError(w,405,"METHOD","Use POST");return }
	var in struct{ PartnerID,Environment string }
	if common.Decode(r,&in)!=nil { common.APIError(w,400,"JSON","Invalid request");return }
	in.Environment=normalizeEnvironment(in.Environment)
	var id string
	err:=a.db.QueryRow(`SELECT credential_id FROM connector.credentials WHERE partner_id=$1 AND environment=$2 AND active=TRUE`,in.PartnerID,in.Environment).Scan(&id)
	if err==nil {
		common.JSON(w,200,map[string]any{"partner_id":in.PartnerID,"environment":in.Environment,"credential_id":id,"exists":true,"token_returned_once":false})
		return
	}
	out,err:=a.issueCredential(r.Context(),in.PartnerID,in.Environment)
	if err!=nil { common.APIError(w,400,"VALIDATION",err.Error());return }
	out["exists"]=false
	common.JSON(w,201,out)
}

func bearer(r *http.Request) string {
	value:=strings.TrimSpace(r.Header.Get("Authorization"))
	if len(value)>7 && strings.EqualFold(value[:7],"Bearer ") { return strings.TrimSpace(value[7:]) }
	return ""
}

func (a *app) authenticate(r *http.Request)(credential,error){
	token:=bearer(r)
	if token=="" { return credential{},fmt.Errorf("missing connector credential") }
	parts:=strings.Split(token,"_")
	if len(parts)<4 || parts[0]!="hmc" || parts[1]!="crd" { return credential{},fmt.Errorf("invalid connector credential") }
	credentialID:="crd_"+parts[2]
	var c credential
	err:=a.db.QueryRow(`SELECT partner_id,environment,credential_id,token_hash,active,created_at,rotated_at,last_used_at FROM connector.credentials WHERE credential_id=$1`,credentialID).
		Scan(&c.PartnerID,&c.Environment,&c.CredentialID,&c.TokenHash,&c.Active,&c.CreatedAt,&c.RotatedAt,&c.LastUsedAt)
	if err!=nil || !c.Active { return credential{},fmt.Errorf("invalid connector credential") }
	got:=tokenHash(token)
	if len(got)!=len(c.TokenHash) || subtle.ConstantTimeCompare([]byte(got),[]byte(c.TokenHash))!=1 { return credential{},fmt.Errorf("invalid connector credential") }
	_,_ = a.db.Exec(`UPDATE connector.credentials SET last_used_at=NOW() WHERE credential_id=$1`,credentialID)
	return c,nil
}

func (a *app) heartbeat(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodPost { common.APIError(w,405,"METHOD","Use POST");return }
	c,err:=a.authenticate(r)
	if err!=nil { common.APIError(w,401,"CONNECTOR_UNAUTHORIZED",err.Error());return }
	var in struct{
		Version string `json:"version"`
		Health string `json:"health"`
		Modules map[string]any `json:"modules"`
		Error string `json:"error"`
	}
	if common.Decode(r,&in)!=nil { common.APIError(w,400,"JSON","Invalid request");return }
	health:=strings.ToUpper(strings.TrimSpace(in.Health))
	if health=="" { health="OK" }
	if health!="OK" && health!="DEGRADED" && health!="OFFLINE" && health!="ERROR" { common.APIError(w,400,"VALIDATION","Invalid health value");return }
	modules,_:=common.MarshalJSON(in.Modules)
	_,err=a.db.Exec(`INSERT INTO connector.partner_state(partner_id,environment,reported_version,health,module_state,last_seen_at,last_error)
		VALUES($1,$2,$3,$4,$5::jsonb,NOW(),$6)
		ON CONFLICT(partner_id,environment) DO UPDATE SET reported_version=EXCLUDED.reported_version,
			health=EXCLUDED.health,module_state=EXCLUDED.module_state,last_seen_at=NOW(),last_error=EXCLUDED.last_error,updated_at=NOW()`,
		c.PartnerID,c.Environment,strings.TrimSpace(in.Version),health,string(modules),strings.TrimSpace(in.Error))
	if err!=nil { common.APIError(w,500,"DB","Could not record heartbeat");return }
	common.JSON(w,200,map[string]any{"status":"accepted","partner_id":c.PartnerID,"environment":c.Environment,"server_time":time.Now().UTC()})
}

func (a *app) state(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodGet { common.APIError(w,405,"METHOD","Use GET");return }
	c,err:=a.authenticate(r)
	if err!=nil { common.APIError(w,401,"CONNECTOR_UNAUTHORIZED",err.Error());return }
	var version,health string
	var modules []byte
	var seen,metrics sql.NullTime
	err=a.db.QueryRow(`SELECT reported_version,health,module_state,last_seen_at,last_metric_sync_at FROM connector.partner_state WHERE partner_id=$1 AND environment=$2`,c.PartnerID,c.Environment).
		Scan(&version,&health,&modules,&seen,&metrics)
	if err!=nil { common.APIError(w,404,"NOT_FOUND","Connector state not found");return }
	var lastSeen,lastMetrics any
	if seen.Valid { lastSeen=seen.Time.UTC() }
	if metrics.Valid { lastMetrics=metrics.Time.UTC() }
	common.JSON(w,200,map[string]any{"partner_id":c.PartnerID,"environment":c.Environment,"reported_version":version,"health":health,"modules":common.JSONRawOrEmpty(modules),"last_seen_at":lastSeen,"last_metric_sync_at":lastMetrics})
}

func (a *app) metrics(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodPost { common.APIError(w,405,"METHOD","Use POST");return }
	c,err:=a.authenticate(r)
	if err!=nil { common.APIError(w,401,"CONNECTOR_UNAUTHORIZED",err.Error());return }
	if strings.TrimSpace(a.impactHost)=="" { common.APIError(w,503,"IMPACT_UNAVAILABLE","Impact service is not configured");return }
	var in struct{ Items []map[string]any `json:"items"` }
	if common.Decode(r,&in)!=nil || len(in.Items)==0 || len(in.Items)>250 { common.APIError(w,400,"VALIDATION","1-250 metric items are required");return }
	accepted:=0
	for i,item:=range in.Items {
		item["partner_id"]=c.PartnerID
		if _,ok:=item["provenance"];!ok { item["provenance"]="PARTNER_DECLARED" }
		if strings.TrimSpace(fmt.Sprint(item["idempotency_key"]))=="" {
			common.APIError(w,400,"IDEMPOTENCY_KEY_REQUIRED",fmt.Sprintf("Metric %d requires idempotency_key",i))
			return
		}
		raw,_:=json.Marshal(item)
		req,err:=http.NewRequestWithContext(r.Context(),http.MethodPost,"http://"+a.impactHost+"/internal/v1/impact/ingest",bytes.NewReader(raw))
		if err!=nil { common.APIError(w,500,"FORWARD","Could not prepare metric sync");return }
		req.Header.Set("Content-Type","application/json")
		req.Header.Set("X-Himate-Internal-Token",a.internalToken)
		req.Header.Set("X-Himate-User-ID","connector:"+c.CredentialID)
		resp,err:=a.client.Do(req)
		if err!=nil { common.APIError(w,502,"IMPACT_UNAVAILABLE","Impact service did not respond");return }
		resp.Body.Close()
		if resp.StatusCode<200 || resp.StatusCode>=300 { common.APIError(w,409,"METRIC_REJECTED",fmt.Sprintf("Impact service rejected metric %d",i));return }
		accepted++
	}
	_,_ = a.db.Exec(`UPDATE connector.partner_state SET last_metric_sync_at=NOW(),updated_at=NOW() WHERE partner_id=$1 AND environment=$2`,c.PartnerID,c.Environment)
	common.JSON(w,202,map[string]any{"status":"accepted","partner_id":c.PartnerID,"accepted":accepted})
}

func (a *app) summary(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodGet { common.APIError(w,405,"METHOD","Use GET");return }
	rows,err:=a.db.Query(`SELECT s.partner_id,s.environment,s.reported_version,s.health,s.last_seen_at,s.last_metric_sync_at,s.last_error,
		EXISTS(SELECT 1 FROM connector.credentials c WHERE c.partner_id=s.partner_id AND c.active=TRUE)
		FROM connector.partner_state s ORDER BY s.partner_id`)
	if err!=nil { common.APIError(w,500,"DB","Could not load connector summary");return }
	defer rows.Close()
	items:=[]map[string]any{}
	for rows.Next(){
		var p,e,v,h,lastErr string
		var seen,metrics sql.NullTime
		var credentialActive bool
		if rows.Scan(&p,&e,&v,&h,&seen,&metrics,&lastErr,&credentialActive)==nil {
			var s,m any
			if seen.Valid{s=seen.Time.UTC()}
			if metrics.Valid{m=metrics.Time.UTC()}
			items=append(items,map[string]any{"partner_id":p,"environment":e,"reported_version":v,"health":h,"last_seen_at":s,"last_metric_sync_at":m,"last_error":lastErr,"credential_active":credentialActive})
		}
	}
	common.JSON(w,200,map[string]any{"items":items})
}
