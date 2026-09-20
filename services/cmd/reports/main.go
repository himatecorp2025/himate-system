package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"himate.local/services/internal/common"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var reportTypes=map[string]bool{"PARTNER_IMPACT":true,"MULTI_PARTNER":true,"HIMATE_GLOBAL":true}

type app struct{
	db *sql.DB
	token string
	impactHost string
	evidenceHost string
	partnersHost string
	storageHost string
	client *http.Client
}

type reportRecord struct{
	ID,ReportType,Title,Status,TemplateVersion,SnapshotSHA256,PDFNamespace,PDFObjectKey,PDFSHA256,RequestedBy,LastError string
	PartnerIDs,Snapshot,EvidenceIDs []byte
	PeriodStart,PeriodEnd time.Time
	PDFSizeBytes int64
	StartedAt,CompletedAt sql.NullTime
	CreatedAt,UpdatedAt time.Time
}

func main(){
	log:=common.Logger()
	db,err:=common.OpenDB();if err!=nil{log.Error("database","error",err);os.Exit(1)};defer db.Close()
	a:=&app{
		db:db,token:os.Getenv("HIMATE_INTERNAL_TOKEN"),impactHost:os.Getenv("IMPACT_HOSTPORT"),
		evidenceHost:os.Getenv("EVIDENCE_HOSTPORT"),partnersHost:os.Getenv("PARTNERS_HOSTPORT"),
		storageHost:os.Getenv("STORAGE_HOSTPORT"),client:&http.Client{Timeout:20*time.Second},
	}
	ctx,cancel:=context.WithTimeout(context.Background(),30*time.Second);defer cancel()
	if err:=a.migrate(ctx);err!=nil{log.Error("migration","error",err);os.Exit(1)}
	_,_=a.db.Exec(`UPDATE reports.jobs SET status='QUEUED',last_error='Recovered after service restart',updated_at=NOW() WHERE status='RUNNING'`)

	mux:=http.NewServeMux()
	mux.HandleFunc("/health",func(w http.ResponseWriter,r *http.Request){common.JSON(w,200,map[string]any{"status":"ok","service":"reports","time":time.Now().UTC()})})
	mux.HandleFunc("/api/v1/reports",a.collection)
	mux.HandleFunc("/api/v1/reports/",a.item)
	go a.worker()
	common.Run(log,"reports",common.Env("PORT","10000"),common.InternalAuth(a.token,mux))
}

func (a *app)migrate(ctx context.Context)error{
	return common.ApplyMigrations(ctx,a.db,"reports",[]common.Migration{
		{Version:1,Name:"pdf-report-jobs",Statements:[]string{
			`CREATE SCHEMA IF NOT EXISTS reports`,
			`CREATE TABLE IF NOT EXISTS reports.jobs(
				id TEXT PRIMARY KEY,
				report_type TEXT NOT NULL,
				title TEXT NOT NULL,
				partner_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
				period_start DATE NOT NULL,
				period_end DATE NOT NULL,
				status TEXT NOT NULL DEFAULT 'QUEUED',
				snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
				evidence_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
				pdf_namespace TEXT NOT NULL DEFAULT '',
				pdf_object_key TEXT NOT NULL DEFAULT '',
				pdf_sha256 TEXT NOT NULL DEFAULT '',
				pdf_size_bytes BIGINT NOT NULL DEFAULT 0,
				requested_by TEXT NOT NULL DEFAULT '',
				last_error TEXT NOT NULL DEFAULT '',
				started_at TIMESTAMPTZ,
				completed_at TIMESTAMPTZ,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				CHECK(period_end >= period_start)
			)`,
			`CREATE INDEX IF NOT EXISTS reports_status_idx ON reports.jobs(status,created_at)`,
			`CREATE INDEX IF NOT EXISTS reports_period_idx ON reports.jobs(period_start,period_end)`,
		}},
		{Version:2,Name:"report-template-snapshot-reference",Statements:[]string{
			`ALTER TABLE reports.jobs ADD COLUMN IF NOT EXISTS template_version TEXT NOT NULL DEFAULT 'impact-v1'`,
			`ALTER TABLE reports.jobs ADD COLUMN IF NOT EXISTS snapshot_sha256 TEXT NOT NULL DEFAULT ''`,
		}},
	})
}

func newID(prefix string)string{raw:=make([]byte,10);if _,err:=rand.Read(raw);err!=nil{return fmt.Sprintf("%s%d",prefix,time.Now().UnixNano())};return prefix+hex.EncodeToString(raw)}
func uniqueStrings(in []string)[]string{seen:=map[string]bool{};out:=[]string{};for _,v:=range in{v=strings.TrimSpace(v);if v!=""&&!seen[v]{seen[v]=true;out=append(out,v)}};sort.Strings(out);return out}

func (a *app)collection(w http.ResponseWriter,r *http.Request){
	switch r.Method{
	case http.MethodGet:a.list(w,r)
	case http.MethodPost:a.create(w,r)
	default:common.APIError(w,405,"METHOD","Use GET or POST")
	}
}

func (a *app)create(w http.ResponseWriter,r *http.Request){
	var in struct{
		ReportType string `json:"report_type"`
		Title string `json:"title"`
		PartnerIDs []string `json:"partner_ids"`
		PeriodStart string `json:"period_start"`
		PeriodEnd string `json:"period_end"`
	}
	if common.Decode(r,&in)!=nil{common.APIError(w,400,"JSON","Invalid request");return}
	in.ReportType=strings.ToUpper(strings.TrimSpace(in.ReportType))
	in.PartnerIDs=uniqueStrings(in.PartnerIDs)
	if !reportTypes[in.ReportType]{common.APIError(w,400,"VALIDATION","Invalid report_type");return}
	start,err:=time.Parse("2006-01-02",strings.TrimSpace(in.PeriodStart));if err!=nil{common.APIError(w,400,"VALIDATION","period_start must be YYYY-MM-DD");return}
	end,err:=time.Parse("2006-01-02",strings.TrimSpace(in.PeriodEnd));if err!=nil||end.Before(start){common.APIError(w,400,"VALIDATION","Invalid period_end");return}
	switch in.ReportType{
	case"PARTNER_IMPACT":if len(in.PartnerIDs)!=1{common.APIError(w,400,"VALIDATION","PARTNER_IMPACT requires exactly one partner");return}
	case"MULTI_PARTNER":if len(in.PartnerIDs)<2{common.APIError(w,400,"VALIDATION","MULTI_PARTNER requires at least two partners");return}
	case"HIMATE_GLOBAL":in.PartnerIDs=[]string{}
	}
	title:=strings.TrimSpace(in.Title)
	if title==""{
		switch in.ReportType{case"PARTNER_IMPACT":title="Partner Impact Report";case"MULTI_PARTNER":title="Multi-Partner Impact Report";default:title="HIMATE Global Impact Report"}
	}
	if len(title)>200{common.APIError(w,400,"VALIDATION","title must be at most 200 characters");return}
	rawPartners,_:=json.Marshal(in.PartnerIDs)
	id:=newID("rpt_")
	actor:=strings.TrimSpace(r.Header.Get("X-Himate-User-ID"))
	_,err=a.db.Exec(`INSERT INTO reports.jobs(id,report_type,title,partner_ids,period_start,period_end,requested_by)
		VALUES($1,$2,$3,$4::jsonb,$5,$6,$7)`,id,in.ReportType,title,string(rawPartners),start,end,actor)
	if err!=nil{common.APIError(w,500,"DB","Could not queue report");return}
	rec,_:=a.get(id)
	common.JSON(w,202,mapReport(rec))
	go a.process(id)
}

func reportSelect()string{return `SELECT id,report_type,title,partner_ids,status,template_version,snapshot,snapshot_sha256,evidence_ids,pdf_namespace,pdf_object_key,pdf_sha256,pdf_size_bytes,requested_by,last_error,period_start,period_end,started_at,completed_at,created_at,updated_at FROM reports.jobs`}
type scanner interface{Scan(...any)error}
func scanReport(s scanner)(v reportRecord,err error){err=s.Scan(&v.ID,&v.ReportType,&v.Title,&v.PartnerIDs,&v.Status,&v.TemplateVersion,&v.Snapshot,&v.SnapshotSHA256,&v.EvidenceIDs,&v.PDFNamespace,&v.PDFObjectKey,&v.PDFSHA256,&v.PDFSizeBytes,&v.RequestedBy,&v.LastError,&v.PeriodStart,&v.PeriodEnd,&v.StartedAt,&v.CompletedAt,&v.CreatedAt,&v.UpdatedAt);return}
func (a *app)get(id string)(reportRecord,error){return scanReport(a.db.QueryRow(reportSelect()+" WHERE id=$1",id))}
func mapReport(v reportRecord)map[string]any{
	partners:=[]string{};_ = json.Unmarshal(v.PartnerIDs,&partners)
	evidenceIDs:=[]string{};_ = json.Unmarshal(v.EvidenceIDs,&evidenceIDs)
	var snapshot any=map[string]any{};_ = json.Unmarshal(v.Snapshot,&snapshot)
	var started,completed any;if v.StartedAt.Valid{started=v.StartedAt.Time.UTC()};if v.CompletedAt.Valid{completed=v.CompletedAt.Time.UTC()}
	return map[string]any{
		"id":v.ID,"report_type":v.ReportType,"title":v.Title,"partner_ids":partners,
		"period_start":v.PeriodStart.Format("2006-01-02"),"period_end":v.PeriodEnd.Format("2006-01-02"),
		"status":v.Status,"template_version":v.TemplateVersion,"snapshot":snapshot,"snapshot_sha256":v.SnapshotSHA256,"evidence_ids":evidenceIDs,
		"pdf_sha256":v.PDFSHA256,"pdf_size_bytes":v.PDFSizeBytes,"requested_by":v.RequestedBy,
		"last_error":v.LastError,"started_at":started,"completed_at":completed,"created_at":v.CreatedAt,"updated_at":v.UpdatedAt,
		"download_ready":v.Status=="READY"&&v.PDFObjectKey!="",
	}
}

func (a *app)list(w http.ResponseWriter,r *http.Request){
	status:=strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("status")))
	reportType:=strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("report_type")))
	where:=[]string{"1=1"};args:=[]any{}
	if status!=""{args=append(args,status);where=append(where,fmt.Sprintf("status=$%d",len(args)))}
	if reportType!=""{if !reportTypes[reportType]{common.APIError(w,400,"VALIDATION","Invalid report_type");return};args=append(args,reportType);where=append(where,fmt.Sprintf("report_type=$%d",len(args)))}
	rows,err:=a.db.Query(reportSelect()+" WHERE "+strings.Join(where," AND ")+" ORDER BY created_at DESC LIMIT 200",args...)
	if err!=nil{common.APIError(w,500,"DB","Could not load reports");return}
	defer rows.Close();items:=[]map[string]any{}
	for rows.Next(){if v,err:=scanReport(rows);err==nil{items=append(items,mapReport(v))}}
	common.JSON(w,200,map[string]any{"items":items,"count":len(items)})
}

func (a *app)item(w http.ResponseWriter,r *http.Request){
	raw:=strings.Trim(strings.TrimPrefix(r.URL.Path,"/api/v1/reports/"),"/")
	parts:=strings.Split(raw,"/")
	if len(parts)==0||parts[0]==""{common.APIError(w,404,"NOT_FOUND","Report not found");return}
	id:=parts[0]
	if len(parts)==2&&parts[1]=="download"{
		if r.Method!=http.MethodGet{common.APIError(w,405,"METHOD","Use GET");return}
		rec,err:=a.get(id);if err!=nil{common.APIError(w,404,"NOT_FOUND","Report not found");return}
		if rec.Status!="READY"||rec.PDFObjectKey==""{common.APIError(w,409,"REPORT_NOT_READY","Report PDF is not ready");return}
		resp,err:=a.getObject(r.Context(),rec.PDFNamespace,rec.PDFObjectKey,"application/pdf")
		if err!=nil{common.APIError(w,502,"STORAGE","Could not load report PDF");return}
		defer resp.Body.Close();if resp.StatusCode<200||resp.StatusCode>=300{common.APIError(w,502,"STORAGE","Report PDF unavailable");return}
		w.Header().Set("Content-Type","application/pdf")
		w.Header().Set("Content-Disposition",`attachment; filename="`+rec.ID+`.pdf"`)
		w.Header().Set("Cache-Control","private, no-store")
		_,_=io.Copy(w,resp.Body);return
	}
	if len(parts)==2&&parts[1]=="regenerate"{
		if r.Method!=http.MethodPost{common.APIError(w,405,"METHOD","Use POST");return}
		rec,err:=a.get(id);if err!=nil{common.APIError(w,404,"NOT_FOUND","Report not found");return}
		if len(rec.Snapshot)==0||string(rec.Snapshot)=="{}"{common.APIError(w,409,"SNAPSHOT_MISSING","Report snapshot is not available");return}
		if err:=a.renderFromStoredSnapshot(r.Context(),rec);err!=nil{common.APIError(w,500,"REPORT_RENDER",err.Error());return}
		updated,_:=a.get(id);common.JSON(w,200,mapReport(updated));return
	}
	if len(parts)!=1||r.Method!=http.MethodGet{common.APIError(w,405,"METHOD","Use GET");return}
	rec,err:=a.get(id);if err!=nil{common.APIError(w,404,"NOT_FOUND","Report not found");return}
	common.JSON(w,200,mapReport(rec))
}

func (a *app)internalGET(ctx context.Context,host,path string,dst any)error{
	if strings.TrimSpace(host)==""{return fmt.Errorf("private service host is not configured")}
	req,err:=http.NewRequestWithContext(ctx,http.MethodGet,"http://"+host+path,nil);if err!=nil{return err}
	req.Header.Set("X-Himate-Internal-Token",a.token)
	resp,err:=a.client.Do(req);if err!=nil{return err};defer resp.Body.Close()
	if resp.StatusCode<200||resp.StatusCode>=300{return fmt.Errorf("status %d",resp.StatusCode)}
	return json.NewDecoder(resp.Body).Decode(dst)
}
func (a *app)internalPOST(ctx context.Context,host,path string,payload any,dst any)error{
	raw,_:=json.Marshal(payload);req,err:=http.NewRequestWithContext(ctx,http.MethodPost,"http://"+host+path,bytes.NewReader(raw));if err!=nil{return err}
	req.Header.Set("X-Himate-Internal-Token",a.token);req.Header.Set("Content-Type","application/json")
	resp,err:=a.client.Do(req);if err!=nil{return err};defer resp.Body.Close()
	if resp.StatusCode<200||resp.StatusCode>=300{return fmt.Errorf("status %d",resp.StatusCode)}
	if dst!=nil{return json.NewDecoder(resp.Body).Decode(dst)}
	return nil
}

func overlapsEvidence(item map[string]any,start,end time.Time)bool{
	ps,pe:=fmt.Sprint(item["period_start"]),fmt.Sprint(item["period_end"])
	if ps==""||ps=="<nil>"||pe==""||pe=="<nil>"{return true}
	s,err1:=time.Parse("2006-01-02",ps);e,err2:=time.Parse("2006-01-02",pe)
	if err1!=nil||err2!=nil{return true}
	return !e.Before(start)&&!s.After(end)
}

func (a *app)buildSnapshot(ctx context.Context,rec reportRecord)(map[string]any,[]string,error){
	partnerIDs:=[]string{};_ = json.Unmarshal(rec.PartnerIDs,&partnerIDs)
	type itemsResponse struct{Items []map[string]any `json:"items"`}
	partnerData:=[]map[string]any{}
	metricSections:=[]map[string]any{}
	dataSources:=[]map[string]any{}
	evidenceItems:=[]map[string]any{}
	evidenceSet:=map[string]bool{}

	if rec.ReportType=="HIMATE_GLOBAL"{
		var summary itemsResponse
		if err:=a.internalGET(ctx,a.impactHost,"/internal/v1/impact/summary?period_start="+url.QueryEscape(rec.PeriodStart.Format("2006-01-02"))+"&period_end="+url.QueryEscape(rec.PeriodEnd.Format("2006-01-02")),&summary);err!=nil{return nil,nil,fmt.Errorf("global impact summary: %w",err)}
		metricSections=append(metricSections,map[string]any{"scope":"GLOBAL","partner_id":"","metrics":summary.Items})
		var values itemsResponse
		if err:=a.internalGET(ctx,a.impactHost,"/api/v1/impact/values?limit=500&period_start="+url.QueryEscape(rec.PeriodStart.Format("2006-01-02"))+"&period_end="+url.QueryEscape(rec.PeriodEnd.Format("2006-01-02")),&values);err==nil{
			for _,v:=range values.Items{dataSources=append(dataSources,v);if id:=strings.TrimSpace(fmt.Sprint(v["partner_id"]));id!=""&&!contains(partnerIDs,id){partnerIDs=append(partnerIDs,id)}}
		}
		sort.Strings(partnerIDs)
	}

	for _,partnerID:=range partnerIDs{
		var partner map[string]any
		if err:=a.internalGET(ctx,a.partnersHost,"/api/v1/partners/"+url.PathEscape(partnerID),&partner);err!=nil{
			partner=map[string]any{"id":partnerID,"display_name":partnerID}
		}
		partnerData=append(partnerData,partner)
		var summary itemsResponse
		if err:=a.internalGET(ctx,a.impactHost,"/internal/v1/impact/summary?partner_id="+url.QueryEscape(partnerID)+"&period_start="+url.QueryEscape(rec.PeriodStart.Format("2006-01-02"))+"&period_end="+url.QueryEscape(rec.PeriodEnd.Format("2006-01-02")),&summary);err!=nil{return nil,nil,fmt.Errorf("impact summary for %s: %w",partnerID,err)}
		metricSections=append(metricSections,map[string]any{"scope":"PARTNER","partner_id":partnerID,"metrics":summary.Items})
		var values itemsResponse
		if err:=a.internalGET(ctx,a.impactHost,"/api/v1/impact/values?partner_id="+url.QueryEscape(partnerID)+"&limit=500&period_start="+url.QueryEscape(rec.PeriodStart.Format("2006-01-02"))+"&period_end="+url.QueryEscape(rec.PeriodEnd.Format("2006-01-02")),&values);err==nil{
			for _,v:=range values.Items{
				ps,_:=time.Parse("2006-01-02",fmt.Sprint(v["period_start"]));pe,_:=time.Parse("2006-01-02",fmt.Sprint(v["period_end"]))
				if !pe.Before(rec.PeriodStart)&&!ps.After(rec.PeriodEnd){dataSources=append(dataSources,v)}
			}
		}
		var ev itemsResponse
		if err:=a.internalGET(ctx,a.evidenceHost,"/internal/v1/evidence/query?partner_id="+url.QueryEscape(partnerID)+"&limit=500",&ev);err==nil{
			for _,item:=range ev.Items{
				if !overlapsEvidence(item,rec.PeriodStart,rec.PeriodEnd){continue}
				id:=strings.TrimSpace(fmt.Sprint(item["id"]));if id==""||evidenceSet[id]{continue}
				evidenceSet[id]=true;evidenceItems=append(evidenceItems,item)
			}
		}
	}
	evidenceIDs:=make([]string,0,len(evidenceSet));for id:=range evidenceSet{evidenceIDs=append(evidenceIDs,id)};sort.Strings(evidenceIDs)
	snapshot:=map[string]any{
		"schema_version":1,"report_id":rec.ID,"report_type":rec.ReportType,"template_version":rec.TemplateVersion,"title":rec.Title,
		"partner_ids":partnerIDs,"partners":partnerData,
		"period_start":rec.PeriodStart.Format("2006-01-02"),"period_end":rec.PeriodEnd.Format("2006-01-02"),
		"metric_sections":metricSections,"data_sources":dataSources,"evidence":evidenceItems,
		"snapshot_created_at":time.Now().UTC(),
		"disclaimer":"HIMATE reports document auditable business and industry data and do not constitute an automatic legal determination.",
	}
	return snapshot,evidenceIDs,nil
}

func contains(values []string,target string)bool{for _,v:=range values{if v==target{return true}};return false}

func (a *app)process(id string){
	ctx,cancel:=context.WithTimeout(context.Background(),90*time.Second);defer cancel()
	rec,err:=a.get(id);if err!=nil{return}
	res,err:=a.db.ExecContext(ctx,`UPDATE reports.jobs SET status='RUNNING',started_at=COALESCE(started_at,NOW()),last_error='',updated_at=NOW() WHERE id=$1 AND status='QUEUED'`,id)
	if err!=nil{return};n,_:=res.RowsAffected();if n==0{return}
	snapshot,evidenceIDs,err:=a.buildSnapshot(ctx,rec)
	if err!=nil{a.fail(id,err);return}
	rawSnapshot,_:=json.Marshal(snapshot);rawEvidence,_:=json.Marshal(evidenceIDs)
	snapshotSum:=sha256.Sum256(rawSnapshot);snapshotSHA:=hex.EncodeToString(snapshotSum[:])
	if _,err=a.db.ExecContext(ctx,`UPDATE reports.jobs SET snapshot=$2::jsonb,snapshot_sha256=$3,evidence_ids=$4::jsonb,updated_at=NOW() WHERE id=$1`,id,string(rawSnapshot),snapshotSHA,string(rawEvidence));err!=nil{a.fail(id,err);return}
	rec.Snapshot=rawSnapshot;rec.SnapshotSHA256=snapshotSHA;rec.EvidenceIDs=rawEvidence
	if err=a.renderFromStoredSnapshot(ctx,rec);err!=nil{a.fail(id,err);return}
	if len(evidenceIDs)>0{
		_ = a.internalPOST(ctx,a.evidenceHost,"/internal/v1/evidence/report-links",map[string]any{"report_id":id,"evidence_ids":evidenceIDs},nil)
	}
}

func (a *app)fail(id string,err error){_,_=a.db.Exec(`UPDATE reports.jobs SET status='FAILED',last_error=$2,completed_at=NOW(),updated_at=NOW() WHERE id=$1`,id,truncate(err.Error(),500))}

func (a *app)renderFromStoredSnapshot(ctx context.Context,rec reportRecord)error{
	var snapshot map[string]any
	if err:=json.Unmarshal(rec.Snapshot,&snapshot);err!=nil{return fmt.Errorf("snapshot: %w",err)}
	pdf:=renderPDF(snapshot)
	sum:=sha256.Sum256(pdf);checksum:=hex.EncodeToString(sum[:])
	namespace:="_reports";key:="reports/"+rec.ID+".pdf"
	if err:=a.ensureStorageNamespace(ctx,namespace);err!=nil{return err}
	stored,err:=a.putObject(ctx,namespace,key,pdf);if err!=nil{return err}
	if got:=strings.TrimSpace(fmt.Sprint(stored["sha256"]));got!=""&&got!=checksum{return fmt.Errorf("stored report checksum mismatch")}
	_,err=a.db.ExecContext(ctx,`UPDATE reports.jobs SET status='READY',pdf_namespace=$2,pdf_object_key=$3,pdf_sha256=$4,pdf_size_bytes=$5,last_error='',completed_at=NOW(),updated_at=NOW() WHERE id=$1`,rec.ID,namespace,key,checksum,len(pdf))
	return err
}

func truncate(v string,n int)string{if len(v)<=n{return v};return v[:n]}
func (a *app)ensureStorageNamespace(ctx context.Context,namespace string)error{
	req,err:=http.NewRequestWithContext(ctx,http.MethodPost,"http://"+a.storageHost+"/internal/v1/storage/partners/"+url.PathEscape(namespace)+"/ensure",bytes.NewReader([]byte("{}")));if err!=nil{return err}
	req.Header.Set("X-Himate-Internal-Token",a.token);req.Header.Set("Content-Type","application/json")
	resp,err:=a.client.Do(req);if err!=nil{return err};defer resp.Body.Close();if resp.StatusCode<200||resp.StatusCode>=300{return fmt.Errorf("storage namespace status %d",resp.StatusCode)};return nil
}
func (a *app)putObject(ctx context.Context,namespace,key string,data []byte)(map[string]any,error){
	req,err:=http.NewRequestWithContext(ctx,http.MethodPut,"http://"+a.storageHost+"/internal/v1/storage/objects/"+url.PathEscape(namespace)+"/"+key,bytes.NewReader(data));if err!=nil{return nil,err}
	req.ContentLength=int64(len(data));req.Header.Set("X-Himate-Internal-Token",a.token)
	resp,err:=a.client.Do(req);if err!=nil{return nil,err};defer resp.Body.Close();if resp.StatusCode<200||resp.StatusCode>=300{return nil,fmt.Errorf("storage put status %d",resp.StatusCode)}
	var out map[string]any;if err:=json.NewDecoder(resp.Body).Decode(&out);err!=nil{return nil,err};return out,nil
}
func (a *app)getObject(ctx context.Context,namespace,key,mime string)(*http.Response,error){
	req,err:=http.NewRequestWithContext(ctx,http.MethodGet,"http://"+a.storageHost+"/internal/v1/storage/objects/"+url.PathEscape(namespace)+"/"+key+"?content_type="+url.QueryEscape(mime),nil);if err!=nil{return nil,err}
	req.Header.Set("X-Himate-Internal-Token",a.token);return a.client.Do(req)
}

func (a *app)worker(){
	t:=time.NewTicker(5*time.Second);defer t.Stop()
	for{
		var id string
		err:=a.db.QueryRow(`SELECT id FROM reports.jobs WHERE status='QUEUED' ORDER BY created_at LIMIT 1`).Scan(&id)
		if err==nil{a.process(id)}
		<-t.C
	}
}

type pdfMetric struct{Label,Unit string;Value float64;HasValue bool}
type pdfSection struct{Title string;Metrics []pdfMetric}

func snapshotSections(snapshot map[string]any)[]pdfSection{
	sections:=[]pdfSection{}
	raw,ok:=snapshot["metric_sections"].([]any);if !ok{return sections}
	for _,entry:=range raw{
		m,ok:=entry.(map[string]any);if !ok{continue}
		title:="Global metrics";if p:=strings.TrimSpace(fmt.Sprint(m["partner_id"]));p!=""{title="Partner "+p}
		s:=pdfSection{Title:title}
		if arr,ok:=m["metrics"].([]any);ok{
			for _,rv:=range arr{
				x,ok:=rv.(map[string]any);if !ok{continue}
				pm:=pdfMetric{Label:fmt.Sprint(x["label"]),Unit:fmt.Sprint(x["unit"])}
				switch v:=x["numeric_value"].(type){case float64:pm.Value=v;pm.HasValue=true;case json.Number:if n,e:=v.Float64();e==nil{pm.Value=n;pm.HasValue=true}}
				s.Metrics=append(s.Metrics,pm)
			}
		}
		sections=append(sections,s)
	}
	return sections
}

func pdfEscape(v string)string{
	v=strings.ReplaceAll(v,"\\","\\\\");v=strings.ReplaceAll(v,"(","\\(");v=strings.ReplaceAll(v,")","\\)")
	var b strings.Builder
	for _,r:=range v{if r>=32&&r<=126{b.WriteRune(r)}else{b.WriteByte('?')}}
	return b.String()
}

func renderPDF(snapshot map[string]any)[]byte{
	title:=fmt.Sprint(snapshot["title"]);reportID:=fmt.Sprint(snapshot["report_id"])
	templateVersion:=fmt.Sprint(snapshot["template_version"])
	period:=fmt.Sprintf("%v to %v",snapshot["period_start"],snapshot["period_end"])
	snapshotRaw,_:=json.Marshal(snapshot);snapshotSum:=sha256.Sum256(snapshotRaw);snapshotRef:=hex.EncodeToString(snapshotSum[:])
	disclaimer:=fmt.Sprint(snapshot["disclaimer"])
	sections:=snapshotSections(snapshot)
	evidence:=[]any{};if v,ok:=snapshot["evidence"].([]any);ok{evidence=v}
	sources:=[]any{};if v,ok:=snapshot["data_sources"].([]any);ok{sources=v}

	pages:=[][]string{}
	generated:=fmt.Sprint(snapshot["snapshot_created_at"])
	scope:="Scope: HIMATE Global"
	if raw,ok:=snapshot["partners"].([]any);ok && len(raw)>0{
		names:=[]string{}
		for _,entry:=range raw{
			if m,ok:=entry.(map[string]any);ok{
				name:=strings.TrimSpace(fmt.Sprint(m["display_name"]))
				if name==""||name=="<nil>"{name=strings.TrimSpace(fmt.Sprint(m["id"]))}
				if name!=""&&name!="<nil>"{names=append(names,name)}
			}
		}
		if len(names)==1{scope="Partner: "+names[0]}else if len(names)>1{scope="Partners: "+strings.Join(names,", ")}
	}
	current:=[]string{"HIMATE","IMPACT REPORT",title,"Report ID: "+reportID,"Template: "+templateVersion,"Snapshot: "+snapshotRef,scope,"Period: "+period,"Generated: "+generated,""}
	for _,s:=range sections{
		if len(current)>35{pages=append(pages,current);current=[]string{"HIMATE · IMPACT REPORT (continued)",""}}
		current=append(current,s.Title)
		max:=0.0;for _,m:=range s.Metrics{if m.HasValue&&math.Abs(m.Value)>max{max=math.Abs(m.Value)}}
		for _,m:=range s.Metrics{
			value:="—";if m.HasValue{value=strconv.FormatFloat(m.Value,'f',2,64)}
			bar:=0
			if max>0&&m.HasValue{bar=int(math.Round(math.Abs(m.Value)/max*20))}
			current=append(current,fmt.Sprintf("%s: %s %s  %s",m.Label,value,m.Unit,strings.Repeat("=",bar)))
			if len(current)>42{pages=append(pages,current);current=[]string{"HIMATE · IMPACT REPORT (continued)",""}}
		}
		current=append(current,"")
	}
	current=append(current,"EVIDENCE & DOCUMENTS")
	for _,raw:=range evidence{
		if m,ok:=raw.(map[string]any);ok{
			current=append(current,fmt.Sprintf("%v · %v · %v · %v",m["id"],m["evidence_type"],m["verification_status"],m["title"]))
			if len(current)>42{pages=append(pages,current);current=[]string{"HIMATE · IMPACT REPORT (continued)",""}}
		}
	}
	current=append(current,"","DATA SOURCES")
	for _,raw:=range sources{
		if m,ok:=raw.(map[string]any);ok{
			current=append(current,fmt.Sprintf("%v · %v=%v · %v to %v · %v · %v",m["id"],m["metric_key"],m["numeric_value"],m["period_start"],m["period_end"],m["provenance"],m["source_ref"]))
			if len(current)>42{pages=append(pages,current);current=[]string{"HIMATE · IMPACT REPORT (continued)",""}}
		}
	}
	current=append(current,"","Generated from immutable snapshot.","",disclaimer)
	pages=append(pages,current)
	return buildPDFPages(pages)
}

func buildPDFPages(pages [][]string)[]byte{
	objects:=map[int][]byte{}
	objects[1]=[]byte("<< /Type /Catalog /Pages 2 0 R >>")
	objects[3]=[]byte("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	objects[4]=[]byte("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold >>")
	kids:=[]string{}
	for i,lines:=range pages{
		pageID:=5+i*2;contentID:=pageID+1;kids=append(kids,fmt.Sprintf("%d 0 R",pageID))
		var stream strings.Builder
		stream.WriteString("0.043 0.122 0.231 rg\n")
		stream.WriteString("36 792 540 18 re f\n")
		y:=775.0
		for idx,line:=range lines{
			size:=10.0;font:="F1"
			if idx==0{size=16;font="F2"}else if idx==1{size=11;font="F2"}else if strings.HasPrefix(line,"EVIDENCE")||strings.HasPrefix(line,"DATA SOURCES")||strings.HasPrefix(line,"Partner ")||line=="Global metrics"{size=11;font="F2"}
			stream.WriteString("BT 0.043 0.122 0.231 rg /"+font+" "+strconv.FormatFloat(size,'f',1,64)+" Tf 54 "+strconv.FormatFloat(y,'f',1,64)+" Td ("+pdfEscape(line)+") Tj ET\n")
			if strings.Contains(line,"="){
				count:=strings.Count(line,"=");if count>0{w:=float64(count)*6;stream.WriteString("0.76 0.61 0.22 rg 54 "+strconv.FormatFloat(y-5,'f',1,64)+" "+strconv.FormatFloat(w,'f',1,64)+" 2 re f\n")}
			}
			y-=16
		}
		body:=[]byte(stream.String())
		objects[contentID]=[]byte(fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream",len(body),body))
		objects[pageID]=[]byte(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 828] /Resources << /Font << /F1 3 0 R /F2 4 0 R >> >> /Contents %d 0 R >>",contentID))
	}
	objects[2]=[]byte("<< /Type /Pages /Kids ["+strings.Join(kids," ")+"] /Count "+strconv.Itoa(len(pages))+" >>")
	maxID:=4+len(pages)*2
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n%HIMATE\n")
	offsets:=make([]int,maxID+1)
	for id:=1;id<=maxID;id++{
		offsets[id]=out.Len()
		out.WriteString(fmt.Sprintf("%d 0 obj\n",id));out.Write(objects[id]);out.WriteString("\nendobj\n")
	}
	xref:=out.Len();out.WriteString(fmt.Sprintf("xref\n0 %d\n",maxID+1));out.WriteString("0000000000 65535 f \n")
	for id:=1;id<=maxID;id++{out.WriteString(fmt.Sprintf("%010d 00000 n \n",offsets[id]))}
	out.WriteString(fmt.Sprintf("trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n",maxID+1,xref))
	return out.Bytes()
}
