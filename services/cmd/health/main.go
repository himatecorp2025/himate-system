package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"himate.local/services/internal/common"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type app struct {
	db *sql.DB
	token string
	client *http.Client
	hosts map[string]string
}

type serviceResult struct {
	Name string `json:"name"`
	Status string `json:"status"`
	LatencyMS int64 `json:"latency_ms"`
	Error string `json:"error,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
}

func main(){
	log:=common.Logger()
	db,err:=common.OpenDB();if err!=nil{log.Error("database","error",err);os.Exit(1)};defer db.Close()
	a:=&app{
		db:db,token:os.Getenv("HIMATE_INTERNAL_TOKEN"),client:&http.Client{Timeout:3*time.Second},
		hosts:map[string]string{
			"partners":os.Getenv("PARTNERS_HOSTPORT"),
			"catalog":os.Getenv("CATALOG_HOSTPORT"),
			"billing":os.Getenv("BILLING_HOSTPORT"),
			"contact":os.Getenv("CONTACT_HOSTPORT"),
			"provisioning":os.Getenv("PROVISIONING_HOSTPORT"),
			"environments":os.Getenv("ENVIRONMENTS_HOSTPORT"),
			"connector":os.Getenv("CONNECTOR_HOSTPORT"),
			"impact":os.Getenv("IMPACT_HOSTPORT"),
		},
	}
	ctx,cancel:=context.WithTimeout(context.Background(),30*time.Second);defer cancel()
	if err:=a.migrate(ctx);err!=nil{log.Error("migration","error",err);os.Exit(1)}
	mux:=http.NewServeMux()
	mux.HandleFunc("/health",func(w http.ResponseWriter,r *http.Request){common.JSON(w,200,map[string]any{"status":"ok","service":"health","time":time.Now().UTC()})})
	mux.HandleFunc("/api/v1/system-health",a.systemHealth)
	mux.HandleFunc("/internal/v1/system-health/summary",a.systemHealth)
	common.Run(log,"health",common.Env("PORT","10000"),common.InternalAuth(a.token,mux))
}

func (a *app)migrate(ctx context.Context)error{
	return common.ApplyMigrations(ctx,a.db,"health",[]common.Migration{
		{Version:1,Name:"health-snapshots",Statements:[]string{
			`CREATE SCHEMA IF NOT EXISTS health`,
			`CREATE TABLE IF NOT EXISTS health.service_snapshots(
				service_name TEXT PRIMARY KEY,status TEXT NOT NULL,latency_ms BIGINT NOT NULL DEFAULT 0,last_error TEXT NOT NULL DEFAULT '',checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE TABLE IF NOT EXISTS health.partner_snapshots(
				partner_id TEXT PRIMARY KEY,overall_status TEXT NOT NULL DEFAULT 'UNKNOWN',platform_version TEXT NOT NULL DEFAULT '',
				connector_health TEXT NOT NULL DEFAULT 'UNKNOWN',environment_status TEXT NOT NULL DEFAULT 'UNKNOWN',
				provisioning_status TEXT NOT NULL DEFAULT 'UNKNOWN',last_seen_at TIMESTAMPTZ,checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
		}},
	})
}

func (a *app)checkServices(ctx context.Context)[]serviceResult{
	results:=make([]serviceResult,0,len(a.hosts)+1)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for name,host:=range a.hosts{
		name,host:=name,host
		wg.Add(1)
		go func(){
			defer wg.Done()
			start:=time.Now();result:=serviceResult{Name:name,Status:"UNAVAILABLE",CheckedAt:start.UTC()}
			if strings.TrimSpace(host)==""{result.Error="host not configured"}else{
				req,_:=http.NewRequestWithContext(ctx,http.MethodGet,"http://"+host+"/health",nil)
				resp,err:=a.client.Do(req)
				result.LatencyMS=time.Since(start).Milliseconds()
				if err!=nil{result.Error=err.Error()}else{
					resp.Body.Close()
					if resp.StatusCode>=200&&resp.StatusCode<300{result.Status="OK"}else{result.Error=fmt.Sprintf("status %d",resp.StatusCode)}
				}
			}
			_,_ = a.db.ExecContext(context.Background(),`INSERT INTO health.service_snapshots(service_name,status,latency_ms,last_error,checked_at)
				VALUES($1,$2,$3,$4,$5) ON CONFLICT(service_name) DO UPDATE SET status=EXCLUDED.status,latency_ms=EXCLUDED.latency_ms,last_error=EXCLUDED.last_error,checked_at=EXCLUDED.checked_at`,
				result.Name,result.Status,result.LatencyMS,result.Error,result.CheckedAt)
			mu.Lock();results=append(results,result);mu.Unlock()
		}()
	}
	wg.Wait()
	dbResult:=serviceResult{Name:"postgres",Status:"OK",CheckedAt:time.Now().UTC()}
	start:=time.Now()
	if err:=a.db.PingContext(ctx);err!=nil{dbResult.Status="UNAVAILABLE";dbResult.Error=err.Error()}
	dbResult.LatencyMS=time.Since(start).Milliseconds()
	results=append(results,dbResult)
	return results
}

func (a *app)internalGET(ctx context.Context,host,path string,dst any)error{
	if strings.TrimSpace(host)==""{return fmt.Errorf("host not configured")}
	req,_:=http.NewRequestWithContext(ctx,http.MethodGet,"http://"+host+path,nil)
	req.Header.Set("X-Himate-Internal-Token",a.token)
	resp,err:=a.client.Do(req);if err!=nil{return err}
	defer resp.Body.Close()
	if resp.StatusCode<200||resp.StatusCode>=300{return fmt.Errorf("status %d",resp.StatusCode)}
	return json.NewDecoder(resp.Body).Decode(dst)
}

func stringValue(v any)string{if v==nil{return ""};return fmt.Sprint(v)}
func timeValue(v any)*time.Time{
	s:=stringValue(v);if s==""||s=="<nil>"{return nil}
	t,err:=time.Parse(time.RFC3339,s);if err!=nil{return nil};return &t
}

func (a *app)partnerHealth(ctx context.Context)[]map[string]any{
	type itemsResponse struct{Items []map[string]any `json:"items"`}
	var connectors,environments,provisioning itemsResponse
	var wg sync.WaitGroup
	wg.Add(3)
	go func(){defer wg.Done();_ = a.internalGET(ctx,a.hosts["connector"],"/internal/v1/connectors/summary",&connectors)}()
	go func(){defer wg.Done();_ = a.internalGET(ctx,a.hosts["environments"],"/internal/v1/environments/summary",&environments)}()
	go func(){defer wg.Done();_ = a.internalGET(ctx,a.hosts["provisioning"],"/internal/v1/provisioning/summary",&provisioning)}()
	wg.Wait()

	byID:=map[string]map[string]any{}
	ensure:=func(id string)map[string]any{if byID[id]==nil{byID[id]=map[string]any{"partner_id":id,"connector_health":"UNKNOWN","environment_status":"UNKNOWN","provisioning_status":"UNKNOWN","platform_version":"","last_seen_at":nil}};return byID[id]}
	for _,x:=range connectors.Items{
		id:=stringValue(x["partner_id"]);if id==""{continue};p:=ensure(id)
		p["connector_health"]=stringValue(x["health"]);p["platform_version"]=stringValue(x["reported_version"]);p["last_seen_at"]=x["last_seen_at"]
	}
	for _,x:=range environments.Items{
		id:=stringValue(x["partner_id"]);if id==""{continue};p:=ensure(id)
		kind:=stringValue(x["kind"])
		if kind=="PRODUCTION" || p["environment_status"]=="UNKNOWN"{p["environment_status"]=stringValue(x["environment_status"])}
		if p["platform_version"]==""{p["platform_version"]=stringValue(x["platform_version"])}
	}
	for _,x:=range provisioning.Items{
		id:=stringValue(x["partner_id"]);if id==""{continue};p:=ensure(id);p["provisioning_status"]=stringValue(x["status"])
	}
	items:=make([]map[string]any,0,len(byID))
	now:=time.Now().UTC()
	for id,p:=range byID{
		conn:=stringValue(p["connector_health"]);env:=stringValue(p["environment_status"]);prov:=stringValue(p["provisioning_status"])
		overall:="OK"
		if conn=="ERROR"||conn=="OFFLINE"||env=="FAILED"||prov=="FAILED"{overall="ERROR"}else if conn=="DEGRADED"||conn=="UNKNOWN"||env=="UNKNOWN"||prov=="BLOCKED_LICENSE"{overall="DEGRADED"}
		if seen:=timeValue(p["last_seen_at"]);seen!=nil && now.Sub(*seen)>15*time.Minute && conn!="UNKNOWN"{overall="DEGRADED"}
		p["overall_status"]=overall;p["checked_at"]=now
		var last any=p["last_seen_at"]
		_,_ = a.db.ExecContext(ctx,`INSERT INTO health.partner_snapshots(partner_id,overall_status,platform_version,connector_health,environment_status,provisioning_status,last_seen_at,checked_at)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8)
			ON CONFLICT(partner_id) DO UPDATE SET overall_status=EXCLUDED.overall_status,platform_version=EXCLUDED.platform_version,
				connector_health=EXCLUDED.connector_health,environment_status=EXCLUDED.environment_status,provisioning_status=EXCLUDED.provisioning_status,last_seen_at=EXCLUDED.last_seen_at,checked_at=EXCLUDED.checked_at`,
			id,overall,stringValue(p["platform_version"]),conn,env,prov,last,now)
		items=append(items,p)
	}
	return items
}

func (a *app)systemHealth(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodGet{common.APIError(w,405,"METHOD","Use GET");return}
	ctx,cancel:=context.WithTimeout(r.Context(),5*time.Second);defer cancel()
	services:=a.checkServices(ctx)
	partners:=a.partnerHealth(ctx)
	overall:="OK"
	for _,s:=range services{if s.Status!="OK"{overall="DEGRADED";break}}
	errorPartners:=0;degradedPartners:=0
	for _,p:=range partners{switch stringValue(p["overall_status"]){case"ERROR":errorPartners++;overall="DEGRADED";case"DEGRADED":degradedPartners++;if overall=="OK"{overall="DEGRADED"}}}
	common.JSON(w,200,map[string]any{
		"status":overall,"checked_at":time.Now().UTC(),"services":services,"partners":partners,
		"summary":map[string]any{"services":len(services),"partners":len(partners),"partner_errors":errorPartners,"partner_degraded":degradedPartners},
	})
}
