package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"himate.local/services/internal/common"
	"himate.local/services/internal/partnerdb"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type app struct {
	db *sql.DB
	token string
	client *http.Client
	hosts map[string]string
	dbAdminURL string
	dbMasterSecret string
}

type job struct {
	ID,PartnerID,SystemName,AdminEmail,PlatformVersion,DesiredRelease,InitialEnvironment,Status,CurrentStep,LastError string
	ModulePreset []string
	CreatedAt,UpdatedAt time.Time
	StartedAt,CompletedAt sql.NullTime
}


var stepOrder=[]string{
	"VALIDATE_PARTNER",
	"VALIDATE_LICENSE",
	"MARK_PROVISIONING",
	"CREATE_DATABASE",
	"SEED_REFERENCE_TEMPLATE",
	"APPLY_MODULE_PRESET",
	"CREATE_STORAGE",
	"CREATE_STAGING_ENVIRONMENT",
	"CREATE_CONNECTOR_CREDENTIAL",
	"SYNC_DESIRED_STATE",
	"DEPLOY_STAGING",
	"STORAGE_HEALTH",
	"PARTNER_DATABASE_HEALTH",
	"STAGING_RUNTIME_HEALTH",
	"COMPLETE",
}

func main(){
	log:=common.Logger()
	db,err:=common.OpenDB()
	if err!=nil{log.Error("database","error",err);os.Exit(1)}
	defer db.Close()
	adminURL:=strings.TrimSpace(os.Getenv("PARTNER_DATABASE_ADMIN_URL"))
	if adminURL=="" { adminURL=strings.TrimSpace(os.Getenv("DATABASE_URL")) }
	a:=&app{
		db:db,token:os.Getenv("HIMATE_INTERNAL_TOKEN"),client:&http.Client{Timeout:12*time.Second},
		dbAdminURL:adminURL,dbMasterSecret:os.Getenv("PARTNER_DATABASE_MASTER_SECRET"),
		hosts:map[string]string{
			"partners":os.Getenv("PARTNERS_HOSTPORT"),
			"billing":os.Getenv("BILLING_HOSTPORT"),
			"catalog":os.Getenv("CATALOG_HOSTPORT"),
			"environments":os.Getenv("ENVIRONMENTS_HOSTPORT"),
			"connector":os.Getenv("CONNECTOR_HOSTPORT"),
			"storage":os.Getenv("STORAGE_HOSTPORT"),
		},
	}
	if len(a.token)<24 || len(a.dbMasterSecret)<32 { log.Error("required provisioning secrets are missing");os.Exit(1) }
	ctx,cancel:=context.WithTimeout(context.Background(),30*time.Second);defer cancel()
	if err:=a.migrate(ctx);err!=nil{log.Error("migration","error",err);os.Exit(1)}

	mux:=http.NewServeMux()
	mux.HandleFunc("/health",func(w http.ResponseWriter,r *http.Request){
		common.JSON(w,200,map[string]any{"status":"ok","service":"provisioning","steps":len(stepOrder),"time":time.Now().UTC()})
	})
	mux.HandleFunc("/api/v1/provisioning/jobs",a.jobs)
	mux.HandleFunc("/api/v1/provisioning/jobs/",a.jobByID)
	mux.HandleFunc("/internal/v1/provisioning/summary",a.summary)
	mux.HandleFunc("/internal/v1/provisioning/database-health",a.databaseHealth)
	common.Run(log,"provisioning",common.Env("PORT","10000"),common.InternalAuth(a.token,mux))
}

func (a *app)migrate(ctx context.Context)error{
	return common.ApplyMigrations(ctx,a.db,"provisioning",[]common.Migration{
		{Version:1,Name:"provisioning-engine",Statements:[]string{
			`CREATE SCHEMA IF NOT EXISTS provisioning`,
			`CREATE TABLE IF NOT EXISTS provisioning.jobs(
				id TEXT PRIMARY KEY,
				partner_id TEXT NOT NULL UNIQUE,
				system_name TEXT NOT NULL,
				admin_email TEXT NOT NULL DEFAULT '',
				platform_version TEXT NOT NULL DEFAULT '',
				desired_release TEXT NOT NULL DEFAULT '',
				module_preset JSONB NOT NULL DEFAULT '[]'::jsonb,
				status TEXT NOT NULL DEFAULT 'READY',
				current_step TEXT NOT NULL DEFAULT '',
				last_error TEXT NOT NULL DEFAULT '',
				started_at TIMESTAMPTZ,
				completed_at TIMESTAMPTZ,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE TABLE IF NOT EXISTS provisioning.steps(
				job_id TEXT NOT NULL REFERENCES provisioning.jobs(id) ON DELETE CASCADE,
				step_key TEXT NOT NULL,
				status TEXT NOT NULL DEFAULT 'PENDING',
				attempts INT NOT NULL DEFAULT 0,
				last_error TEXT NOT NULL DEFAULT '',
				started_at TIMESTAMPTZ,
				completed_at TIMESTAMPTZ,
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				PRIMARY KEY(job_id,step_key)
			)`,
			`CREATE INDEX IF NOT EXISTS provisioning_jobs_status_idx ON provisioning.jobs(status,updated_at DESC)`,
		}},
		{Version:2,Name:"provisioning-plan",Statements:[]string{
			`ALTER TABLE provisioning.jobs ADD COLUMN IF NOT EXISTS initial_environment TEXT NOT NULL DEFAULT 'STAGING'`,
		}},
	})
}

func jobID(partnerID string)string{return "prv_"+strings.TrimPrefix(strings.ToLower(strings.TrimSpace(partnerID)),"ptr_")}

func (a *app)jobs(w http.ResponseWriter,r *http.Request){
	switch r.Method{
	case http.MethodGet:
		partnerID:=strings.TrimSpace(r.URL.Query().Get("partner_id"))
		q:=`SELECT id,partner_id,system_name,admin_email,platform_version,desired_release,initial_environment,module_preset,status,current_step,last_error,started_at,completed_at,created_at,updated_at FROM provisioning.jobs`
		args:=[]any{}
		if partnerID!=""{q+=" WHERE partner_id=$1";args=append(args,partnerID)}
		q+=" ORDER BY updated_at DESC"
		rows,err:=a.db.Query(q,args...)
		if err!=nil{common.APIError(w,500,"DB","Could not load provisioning jobs");return}
		defer rows.Close()
		items:=[]map[string]any{}
		for rows.Next(){if j,err:=scanJob(rows);err==nil{items=append(items,mapJob(j))}}
		common.JSON(w,200,map[string]any{"items":items,"count":len(items)})
	case http.MethodPost:
		var in struct{
			PartnerID string `json:"partner_id"`
			SystemName string `json:"system_name"`
			AdminEmail string `json:"admin_email"`
			PlatformVersion string `json:"platform_version"`
			DesiredRelease string `json:"desired_release"`
			Environment string `json:"environment"`
			ModulePreset []string `json:"module_preset"`
			PrepareOnly bool `json:"prepare_only"`
		}
		if common.Decode(r,&in)!=nil || strings.TrimSpace(in.PartnerID)==""{common.APIError(w,400,"VALIDATION","partner_id is required");return}
		in.PartnerID=strings.TrimSpace(in.PartnerID)
		if strings.TrimSpace(in.SystemName)==""{in.SystemName=in.PartnerID}
		in.Environment=strings.ToUpper(strings.TrimSpace(in.Environment))
		if in.Environment==""{in.Environment="STAGING"}
		if in.Environment!="STAGING"{common.APIError(w,400,"VALIDATION","Initial provisioning environment must be STAGING");return}
		raw,_:=json.Marshal(uniqueStrings(in.ModulePreset))
		id:=jobID(in.PartnerID)
		_,err:=a.db.Exec(`INSERT INTO provisioning.jobs(id,partner_id,system_name,admin_email,platform_version,desired_release,initial_environment,module_preset,status)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8::jsonb,'READY')
			ON CONFLICT(partner_id) DO UPDATE SET
				system_name=CASE WHEN provisioning.jobs.status IN ('COMPLETED','CONFIGURATION_REQUIRED') THEN provisioning.jobs.system_name ELSE EXCLUDED.system_name END,
				admin_email=CASE WHEN EXCLUDED.admin_email<>'' THEN EXCLUDED.admin_email ELSE provisioning.jobs.admin_email END,
				platform_version=CASE WHEN EXCLUDED.platform_version<>'' THEN EXCLUDED.platform_version ELSE provisioning.jobs.platform_version END,
				desired_release=CASE WHEN EXCLUDED.desired_release<>'' THEN EXCLUDED.desired_release ELSE provisioning.jobs.desired_release END,
				initial_environment=CASE WHEN provisioning.jobs.status IN ('COMPLETED','CONFIGURATION_REQUIRED') THEN provisioning.jobs.initial_environment ELSE EXCLUDED.initial_environment END,
				module_preset=CASE WHEN EXCLUDED.module_preset<>'[]'::jsonb THEN EXCLUDED.module_preset ELSE provisioning.jobs.module_preset END,
				updated_at=NOW()`,
			id,in.PartnerID,strings.TrimSpace(in.SystemName),strings.ToLower(strings.TrimSpace(in.AdminEmail)),strings.TrimSpace(in.PlatformVersion),strings.TrimSpace(in.DesiredRelease),in.Environment,string(raw))
		if err!=nil{common.APIError(w,500,"DB","Could not create provisioning job");return}
		for _,step:=range stepOrder{
			_,_ = a.db.Exec(`INSERT INTO provisioning.steps(job_id,step_key) VALUES($1,$2) ON CONFLICT(job_id,step_key) DO NOTHING`,id,step)
		}
		if in.PrepareOnly{
			j,_:=a.getJob(id)
			common.JSON(w,202,mapJob(j))
			return
		}
		if err:=a.runJob(r.Context(),id,strings.TrimSpace(r.Header.Get("X-Himate-User-ID")));err!=nil{
			j,_:=a.getJob(id)
			common.JSON(w,409,map[string]any{"job":mapJob(j),"error":map[string]string{"code":"PROVISIONING_INCOMPLETE","message":err.Error()}})
			return
		}
		j,_:=a.getJob(id)
		common.JSON(w,202,mapJob(j))
	default:common.APIError(w,405,"METHOD","Use GET or POST")
	}
}

func (a *app)jobByID(w http.ResponseWriter,r *http.Request){
	raw:=strings.Trim(strings.TrimPrefix(r.URL.Path,"/api/v1/provisioning/jobs/"),"/")
	parts:=strings.Split(raw,"/")
	id:=parts[0]
	if id==""{common.APIError(w,404,"NOT_FOUND","Provisioning job not found");return}
	if len(parts)==1 && r.Method==http.MethodGet{
		j,err:=a.getJob(id);if err!=nil{common.APIError(w,404,"NOT_FOUND","Provisioning job not found");return}
		out:=mapJob(j);out["steps"]=a.steps(id);common.JSON(w,200,out);return
	}
	if len(parts)==2 && parts[1]=="run" && r.Method==http.MethodPost{
		if err:=a.runJob(r.Context(),id,strings.TrimSpace(r.Header.Get("X-Himate-User-ID")));err!=nil{
			j,_:=a.getJob(id);common.JSON(w,409,map[string]any{"job":mapJob(j),"error":map[string]string{"code":"PROVISIONING_INCOMPLETE","message":err.Error()}});return
		}
		j,_:=a.getJob(id);out:=mapJob(j);out["steps"]=a.steps(id);common.JSON(w,200,out);return
	}
	common.APIError(w,405,"METHOD","Use GET or POST /run")
}

func (a *app)runJob(ctx context.Context,id,actor string)error{
	j,err:=a.getJob(id);if err!=nil{return err}
	if j.Status=="COMPLETED" || j.Status=="CONFIGURATION_REQUIRED"{return nil}
	_,_ = a.db.ExecContext(ctx,`UPDATE provisioning.jobs SET status='RUNNING',last_error='',started_at=COALESCE(started_at,NOW()),updated_at=NOW() WHERE id=$1`,id)
	for _,step:=range stepOrder{
		if a.stepSucceeded(id,step){continue}
		_,_ = a.db.ExecContext(ctx,`UPDATE provisioning.jobs SET current_step=$2,updated_at=NOW() WHERE id=$1`,id,step)
		_,_ = a.db.ExecContext(ctx,`UPDATE provisioning.steps SET status='RUNNING',attempts=attempts+1,last_error='',started_at=NOW(),updated_at=NOW() WHERE job_id=$1 AND step_key=$2`,id,step)
		err=a.executeStep(ctx,j,step,actor)
		if err!=nil{
			status:="FAILED"
			if errors.Is(err,errLicenseBlocked){status="BLOCKED_LICENSE"}
			_,_ = a.db.ExecContext(ctx,`UPDATE provisioning.steps SET status=$3,last_error=$4,updated_at=NOW() WHERE job_id=$1 AND step_key=$2`,id,step,status,err.Error())
			_,_ = a.db.ExecContext(ctx,`UPDATE provisioning.jobs SET status=$2,last_error=$3,updated_at=NOW() WHERE id=$1`,id,status,err.Error())
			return err
		}
		_,_ = a.db.ExecContext(ctx,`UPDATE provisioning.steps SET status='SUCCESS',last_error='',completed_at=NOW(),updated_at=NOW() WHERE job_id=$1 AND step_key=$2`,id,step)
	}
	_,_ = a.db.ExecContext(ctx,`UPDATE provisioning.jobs SET status='CONFIGURATION_REQUIRED',current_step='COMPLETE',last_error='',completed_at=NOW(),updated_at=NOW() WHERE id=$1`,id)
	return nil
}

var errLicenseBlocked=errors.New("initial license gate is not satisfied")

func (a *app)executeStep(ctx context.Context,j job,step,actor string)error{
	switch step{
	case "VALIDATE_PARTNER":
		var p map[string]any
		if err:=a.internalJSON(ctx,http.MethodGet,a.hosts["partners"],"/api/v1/partners/"+j.PartnerID,nil,&p,actor);err!=nil{return fmt.Errorf("partner validation: %w",err)}
		lifecycle:=fmt.Sprint(p["lifecycle"])
		if lifecycle!="READY_TO_PROVISION" && lifecycle!="PROVISIONING" && lifecycle!="CONFIGURATION"{
			return fmt.Errorf("partner lifecycle must be READY_TO_PROVISION before provisioning; current=%s",lifecycle)
		}
	case "VALIDATE_LICENSE":
		var gate struct{
			Allowed bool `json:"allowed"`
			Reason string `json:"reason"`
			PaymentStatus string `json:"payment_status"`
			EvidenceCount int `json:"evidence_count"`
		}
		if err:=a.internalJSON(ctx,http.MethodGet,a.hosts["billing"],"/internal/v1/partners/"+j.PartnerID+"/provisioning-gate",nil,&gate,actor);err!=nil{return fmt.Errorf("license gate: %w",err)}
		if !gate.Allowed{return fmt.Errorf("%w: %s",errLicenseBlocked,gate.Reason)}
	case "MARK_PROVISIONING":
		var p map[string]any
		if err:=a.internalJSON(ctx,http.MethodGet,a.hosts["partners"],"/api/v1/partners/"+j.PartnerID,nil,&p,actor);err!=nil{return err}
		if fmt.Sprint(p["lifecycle"])=="READY_TO_PROVISION"{
			if err:=a.patchPartnerLifecycle(ctx,j.PartnerID,"PROVISIONING",actor,"Initial license gate passed; Provisioning Engine started");err!=nil{return err}
		}
	case "CREATE_DATABASE":
		if err:=a.ensurePartnerDatabase(ctx,j.PartnerID);err!=nil{return err}
	case "SEED_REFERENCE_TEMPLATE":
		if err:=a.seedPartnerTemplate(ctx,j);err!=nil{return err}
	case "APPLY_MODULE_PRESET":
		if err:=a.applyModulePreset(ctx,j,actor);err!=nil{return err}
	case "CREATE_STORAGE":
		var out map[string]any
		if err:=a.internalJSON(ctx,http.MethodPost,a.hosts["storage"],"/internal/v1/storage/partners/"+j.PartnerID+"/ensure",map[string]any{},&out,actor);err!=nil{return fmt.Errorf("partner storage: %w",err)}
	case "CREATE_STAGING_ENVIRONMENT":
		payload:=map[string]any{"partner_id":j.PartnerID,"system_name":j.SystemName,"platform_version":j.PlatformVersion,"desired_release":j.DesiredRelease,"config":map[string]any{"partner_id":j.PartnerID,"database":a.partnerDatabaseName(j.PartnerID)}}
		var out map[string]any
		if err:=a.internalJSON(ctx,http.MethodPost,a.hosts["environments"],"/internal/v1/environments/ensure-staging",payload,&out,actor);err!=nil{return fmt.Errorf("staging environment: %w",err)}
	case "CREATE_CONNECTOR_CREDENTIAL":
		payload:=map[string]any{"PartnerID":j.PartnerID,"Environment":"STAGING"}
		var out map[string]any
		if err:=a.internalJSON(ctx,http.MethodPost,a.hosts["connector"],"/internal/v1/connectors/ensure",payload,&out,actor);err!=nil{return fmt.Errorf("connector credential: %w",err)}
	case "SYNC_DESIRED_STATE":
		entitlements:=map[string]any{}
		for _,key:=range uniqueStrings(j.ModulePreset){entitlements[key]="ACTIVE"}
		payload:=map[string]any{
			"environment":"STAGING",
			"entitlements":entitlements,
			"maintenance":map[string]any{"status":"ACTIVE"},
			"config":map[string]any{"partner_id":j.PartnerID,"system_name":j.SystemName,"platform_version":j.PlatformVersion,"desired_release":j.DesiredRelease},
		}
		var out map[string]any
		if err:=a.internalJSON(ctx,http.MethodPut,a.hosts["connector"],"/api/v1/connectors/"+j.PartnerID+"/desired-state",payload,&out,actor);err!=nil{return fmt.Errorf("connector desired state: %w",err)}
	case "DEPLOY_STAGING":
		payload:=map[string]any{"partner_id":j.PartnerID,"release":j.DesiredRelease}
		var out map[string]any
		if err:=a.internalJSON(ctx,http.MethodPost,a.hosts["environments"],"/internal/v1/environments/deploy-staging",payload,&out,actor);err!=nil{return fmt.Errorf("staging deployment: %w",err)}
	case "STORAGE_HEALTH":
		var out map[string]any
		if err:=a.internalJSON(ctx,http.MethodGet,a.hosts["storage"],"/internal/v1/storage/partners/"+j.PartnerID+"/health",nil,&out,actor);err!=nil{return fmt.Errorf("partner storage health: %w",err)}
	case "PARTNER_DATABASE_HEALTH":
		if err:=a.checkPartnerDatabase(ctx,j.PartnerID);err!=nil{return err}
	case "STAGING_RUNTIME_HEALTH":
		var out map[string]any
		if err:=a.internalJSON(ctx,http.MethodGet,a.hosts["environments"],"/internal/v1/environments/runtime-health?partner_id="+url.QueryEscape(j.PartnerID)+"&environment=STAGING",nil,&out,actor);err!=nil{return fmt.Errorf("staging runtime health: %w",err)}
	case "COMPLETE":
		var p map[string]any
		if err:=a.internalJSON(ctx,http.MethodGet,a.hosts["partners"],"/api/v1/partners/"+j.PartnerID,nil,&p,actor);err!=nil{return err}
		if fmt.Sprint(p["lifecycle"])=="PROVISIONING"{
			if err:=a.patchPartnerLifecycle(ctx,j.PartnerID,"CONFIGURATION",actor,"Provisioning completed; partner configuration required");err!=nil{return err}
		}
	default:return fmt.Errorf("unknown provisioning step %s",step)
	}
	return nil
}

func (a *app)patchPartnerLifecycle(ctx context.Context,partnerID,state,actor,reason string)error{
	payload:=map[string]any{"lifecycle":state,"reason":reason}
	var out map[string]any
	return a.internalJSON(ctx,http.MethodPatch,a.hosts["partners"],"/api/v1/partners/"+partnerID,payload,&out,actor)
}

func uniqueStrings(values []string)[]string{
	seen:=map[string]bool{};out:=[]string{}
	for _,v:=range values{v=strings.TrimSpace(v);if v!=""&&!seen[v]{seen[v]=true;out=append(out,v)}}
	return out
}

func (a *app)applyModulePreset(ctx context.Context,j job,actor string)error{
	var list map[string]any
	if err:=a.internalJSON(ctx,http.MethodGet,a.hosts["catalog"],"/internal/v1/partners/"+j.PartnerID+"/modules",nil,&list,actor);err!=nil{return fmt.Errorf("initialize module catalog: %w",err)}
	for _,key:=range uniqueStrings(j.ModulePreset){
		payload:=map[string]any{"status":"ACTIVE","visible":true,"included_in_base":true,"reason":"Provisioning module preset"}
		var out map[string]any
		if err:=a.internalJSON(ctx,http.MethodPatch,a.hosts["catalog"],"/internal/v1/partners/"+j.PartnerID+"/modules/"+url.PathEscape(key),payload,&out,actor);err!=nil{return fmt.Errorf("module preset %s: %w",key,err)}
	}
	return nil
}

func (a *app)internalJSON(ctx context.Context,method,host,path string,payload any,dst any,actor string)error{
	if strings.TrimSpace(host)==""{return fmt.Errorf("service host is not configured")}
	var body *bytes.Reader
	if payload==nil{body=bytes.NewReader(nil)}else{raw,err:=json.Marshal(payload);if err!=nil{return err};body=bytes.NewReader(raw)}
	req,err:=http.NewRequestWithContext(ctx,method,"http://"+host+path,body);if err!=nil{return err}
	req.Header.Set("X-Himate-Internal-Token",a.token)
	if actor!=""{req.Header.Set("X-Himate-User-ID",actor)}
	if payload!=nil{req.Header.Set("Content-Type","application/json")}
	resp,err:=a.client.Do(req);if err!=nil{return err}
	defer resp.Body.Close()
	if resp.StatusCode<200||resp.StatusCode>=300{
		var e map[string]any;_ = json.NewDecoder(resp.Body).Decode(&e)
		return fmt.Errorf("status %d: %v",resp.StatusCode,e)
	}
	if dst!=nil{return json.NewDecoder(resp.Body).Decode(dst)}
	return nil
}

func (a *app)partnerDatabaseName(partnerID string)string{return partnerdb.DatabaseName(partnerID)}
func (a *app)partnerRoleName(partnerID string)string{return partnerdb.RoleName(partnerID)}

func quoteIdent(v string)string{return `"`+strings.ReplaceAll(v,`"`,`""`)+`"`}

func (a *app)partnerPassword(partnerID string)string{
	m:=hmac.New(sha256.New,[]byte(a.dbMasterSecret));m.Write([]byte("partner-db:"+partnerID))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))[:40]
}

func (a *app)adminDSN()(string,error){return partnerdb.AdminDSN(a.dbAdminURL)}

func (a *app)partnerDSN(partnerID string)(string,error){
	u,err:=url.Parse(a.dbAdminURL);if err!=nil{return "",err}
	name:=a.partnerDatabaseName(partnerID);role:=a.partnerRoleName(partnerID)
	if name==""||role==""{return "",fmt.Errorf("invalid partner id")}
	u.Path="/"+name
	u.User=url.UserPassword(role,a.partnerPassword(partnerID))
	return u.String(),nil
}

func (a *app)ensurePartnerDatabase(ctx context.Context,partnerID string)error{
	adminDSN,err:=a.adminDSN();if err!=nil{return err}
	admin,err:=sql.Open("pgx",adminDSN);if err!=nil{return err}
	defer admin.Close()
	if err=admin.PingContext(ctx);err!=nil{return fmt.Errorf("partner database admin connection: %w",err)}
	dbName:=a.partnerDatabaseName(partnerID);roleName:=a.partnerRoleName(partnerID)
	if dbName==""||roleName==""{return fmt.Errorf("invalid partner id for database allocation")}
	password:=a.partnerPassword(partnerID)
	var exists bool
	if err=admin.QueryRowContext(ctx,`SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname=$1)`,roleName).Scan(&exists);err!=nil{return err}
	if !exists{
		if _,err=admin.ExecContext(ctx,"CREATE ROLE "+quoteIdent(roleName)+" LOGIN PASSWORD '"+password+"' NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT");err!=nil{return fmt.Errorf("create partner database role: %w",err)}
	}else{
		if _,err=admin.ExecContext(ctx,"ALTER ROLE "+quoteIdent(roleName)+" PASSWORD '"+password+"' NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT");err!=nil{return fmt.Errorf("refresh partner database role: %w",err)}
	}
	if err=admin.QueryRowContext(ctx,`SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname=$1)`,dbName).Scan(&exists);err!=nil{return err}
	if !exists{
		if _,err=admin.ExecContext(ctx,"CREATE DATABASE "+quoteIdent(dbName)+" OWNER "+quoteIdent(roleName));err!=nil{return fmt.Errorf("create isolated partner database: %w",err)}
	}
	if _,err=admin.ExecContext(ctx,"REVOKE ALL ON DATABASE "+quoteIdent(dbName)+" FROM PUBLIC");err!=nil{return fmt.Errorf("revoke public database access: %w",err)}
	if _,err=admin.ExecContext(ctx,"GRANT CONNECT,TEMPORARY ON DATABASE "+quoteIdent(dbName)+" TO "+quoteIdent(roleName));err!=nil{return fmt.Errorf("grant partner database access: %w",err)}
	return nil
}

func (a *app)seedPartnerTemplate(ctx context.Context,j job)error{
	dsn,err:=a.partnerDSN(j.PartnerID);if err!=nil{return err}
	db,err:=sql.Open("pgx",dsn);if err!=nil{return err}
	defer db.Close()
	if err=db.PingContext(ctx);err!=nil{return fmt.Errorf("partner database connection: %w",err)}
	stmts:=[]string{
		`CREATE SCHEMA IF NOT EXISTS partner_core`,
		`CREATE TABLE IF NOT EXISTS partner_core.system_meta(key TEXT PRIMARY KEY,value TEXT NOT NULL,updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE IF NOT EXISTS partner_core.admin_invites(email TEXT PRIMARY KEY,status TEXT NOT NULL DEFAULT 'PENDING',created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE IF NOT EXISTS partner_core.module_entitlements(module_key TEXT PRIMARY KEY,enabled BOOLEAN NOT NULL DEFAULT FALSE,updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
	}
	for _,stmt:=range stmts{if _,err=db.ExecContext(ctx,stmt);err!=nil{return fmt.Errorf("seed reference template: %w",err)}}
	values:=map[string]string{
		"partner_id":j.PartnerID,"system_name":j.SystemName,"platform_version":j.PlatformVersion,
		"template":"HIMATE Partner Platform","template_data_policy":"STRUCTURE_ONLY_NO_KLAVIERHAUS_BUSINESS_DATA",
		"storage_namespace":"partner/"+j.PartnerID,
	}
	for k,v:=range values{if _,err=db.ExecContext(ctx,`INSERT INTO partner_core.system_meta(key,value) VALUES($1,$2) ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value,updated_at=NOW()`,k,v);err!=nil{return err}}
	if strings.TrimSpace(j.AdminEmail)!=""{
		if _,err=db.ExecContext(ctx,`INSERT INTO partner_core.admin_invites(email,status) VALUES($1,'PENDING') ON CONFLICT(email) DO NOTHING`,j.AdminEmail);err!=nil{return err}
	}
	for _,key:=range uniqueStrings(j.ModulePreset){if _,err=db.ExecContext(ctx,`INSERT INTO partner_core.module_entitlements(module_key,enabled) VALUES($1,TRUE) ON CONFLICT(module_key) DO UPDATE SET enabled=TRUE,updated_at=NOW()`,key);err!=nil{return err}}
	return nil
}

func (a *app)checkPartnerDatabase(ctx context.Context,partnerID string)error{
	dsn,err:=a.partnerDSN(partnerID);if err!=nil{return err}
	db,err:=sql.Open("pgx",dsn);if err!=nil{return err}
	defer db.Close()
	var partner string
	if err=db.QueryRowContext(ctx,`SELECT value FROM partner_core.system_meta WHERE key='partner_id'`).Scan(&partner);err!=nil{return fmt.Errorf("partner database readiness: %w",err)}
	if partner!=partnerID{return fmt.Errorf("partner database identity mismatch")}
	return nil
}

type scanner interface{Scan(...any)error}
func scanJob(s scanner)(job,error){
	var j job;var raw []byte
	err:=s.Scan(&j.ID,&j.PartnerID,&j.SystemName,&j.AdminEmail,&j.PlatformVersion,&j.DesiredRelease,&j.InitialEnvironment,&raw,&j.Status,&j.CurrentStep,&j.LastError,&j.StartedAt,&j.CompletedAt,&j.CreatedAt,&j.UpdatedAt)
	_ = json.Unmarshal(raw,&j.ModulePreset)
	return j,err
}
func (a *app)getJob(id string)(job,error){
	return scanJob(a.db.QueryRow(`SELECT id,partner_id,system_name,admin_email,platform_version,desired_release,initial_environment,module_preset,status,current_step,last_error,started_at,completed_at,created_at,updated_at FROM provisioning.jobs WHERE id=$1`,id))
}
func nullableTime(v sql.NullTime)any{if !v.Valid{return nil};return v.Time.UTC()}
func mapJob(j job)map[string]any{return map[string]any{"id":j.ID,"partner_id":j.PartnerID,"system_name":j.SystemName,"admin_email":j.AdminEmail,"platform_version":j.PlatformVersion,"desired_release":j.DesiredRelease,"initial_environment":j.InitialEnvironment,"module_preset":j.ModulePreset,"status":j.Status,"current_step":j.CurrentStep,"last_error":j.LastError,"started_at":nullableTime(j.StartedAt),"completed_at":nullableTime(j.CompletedAt),"created_at":j.CreatedAt,"updated_at":j.UpdatedAt}}
func (a *app)stepSucceeded(id,step string)bool{var status string;return a.db.QueryRow(`SELECT status FROM provisioning.steps WHERE job_id=$1 AND step_key=$2`,id,step).Scan(&status)==nil&&status=="SUCCESS"}
func (a *app)steps(id string)[]map[string]any{
	rows,err:=a.db.Query(`SELECT step_key,status,attempts,last_error,started_at,completed_at,updated_at FROM provisioning.steps WHERE job_id=$1
		ORDER BY CASE step_key
			WHEN 'VALIDATE_PARTNER' THEN 1 WHEN 'VALIDATE_LICENSE' THEN 2 WHEN 'MARK_PROVISIONING' THEN 3
			WHEN 'CREATE_DATABASE' THEN 4 WHEN 'SEED_REFERENCE_TEMPLATE' THEN 5 WHEN 'APPLY_MODULE_PRESET' THEN 6
			WHEN 'CREATE_STORAGE' THEN 7 WHEN 'CREATE_STAGING_ENVIRONMENT' THEN 8 WHEN 'CREATE_CONNECTOR_CREDENTIAL' THEN 9
			WHEN 'SYNC_DESIRED_STATE' THEN 10 WHEN 'DEPLOY_STAGING' THEN 11 WHEN 'STORAGE_HEALTH' THEN 12 WHEN 'PARTNER_DATABASE_HEALTH' THEN 13
			WHEN 'STAGING_RUNTIME_HEALTH' THEN 14 WHEN 'COMPLETE' THEN 15 ELSE 99 END`,id)
	if err!=nil{return []map[string]any{}}
	defer rows.Close()
	items:=[]map[string]any{}
	for rows.Next(){var key,status,lastErr string;var attempts int;var started,completed sql.NullTime;var updated time.Time;if rows.Scan(&key,&status,&attempts,&lastErr,&started,&completed,&updated)==nil{items=append(items,map[string]any{"step_key":key,"status":status,"attempts":attempts,"last_error":lastErr,"started_at":nullableTime(started),"completed_at":nullableTime(completed),"updated_at":updated})}}
	return items
}
func (a *app)summary(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodGet{common.APIError(w,405,"METHOD","Use GET");return}
	rows,err:=a.db.Query(`SELECT id,partner_id,system_name,admin_email,platform_version,desired_release,initial_environment,module_preset,status,current_step,last_error,started_at,completed_at,created_at,updated_at FROM provisioning.jobs ORDER BY updated_at DESC`)
	if err!=nil{common.APIError(w,500,"DB","Could not load provisioning summary");return}
	defer rows.Close();items:=[]map[string]any{}
	for rows.Next(){if j,err:=scanJob(rows);err==nil{items=append(items,mapJob(j))}}
	common.JSON(w,200,map[string]any{"items":items})
}

func (a *app)databaseHealth(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodGet{common.APIError(w,405,"METHOD","Use GET");return}
	rows,err:=a.db.Query(`SELECT partner_id FROM provisioning.jobs WHERE status IN ('CONFIGURATION_REQUIRED','COMPLETED') ORDER BY partner_id`)
	if err!=nil{common.APIError(w,500,"DB","Could not load provisioned partners");return}
	defer rows.Close()
	items:=[]map[string]any{}
	for rows.Next(){
		var partnerID string
		if rows.Scan(&partnerID)!=nil{continue}
		started:=time.Now()
		err:=a.checkPartnerDatabase(r.Context(),partnerID)
		status:="OK";message:=""
		if err!=nil{status="ERROR";message=err.Error()}
		items=append(items,map[string]any{"partner_id":partnerID,"status":status,"latency_ms":time.Since(started).Milliseconds(),"error":message,"checked_at":time.Now().UTC()})
	}
	common.JSON(w,200,map[string]any{"items":items})
}

var _ = strconv.Itoa
