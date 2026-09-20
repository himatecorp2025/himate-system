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
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const maxEvidenceBytes = 20 << 20

var evidenceTypes = map[string]bool{
	"PDF": true, "IMAGE": true, "INVOICE": true, "CONTRACT": true,
	"SCREENSHOT": true, "REPORT": true, "URL": true, "PARTNER_DECLARATION": true, "OTHER": true,
}
var verificationValues = map[string]bool{"UNVERIFIED": true, "VERIFIED": true, "REJECTED": true}
var metricKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_.]{2,127}$`)

type app struct {
	db          *sql.DB
	token       string
	storageHost string
	client      *http.Client
}

type evidence struct {
	ID, PartnerID, MetricKey, EvidenceType, Title, Description string
	PeriodStart, PeriodEnd                                      sql.NullTime
	VerificationStatus, SourceURL, DeclarationText              string
	ObjectNamespace, ObjectKey, OriginalFilename, MimeType      string
	SizeBytes                                                    int64
	SHA256, UploadedBy, VerifiedBy                               string
	CreatedAt, UpdatedAt                                         time.Time
	VerifiedAt                                                   sql.NullTime
}

func main() {
	log := common.Logger()
	db, err := common.OpenDB()
	if err != nil { log.Error("database", "error", err); os.Exit(1) }
	defer db.Close()
	a := &app{
		db: db,
		token: os.Getenv("HIMATE_INTERNAL_TOKEN"),
		storageHost: os.Getenv("STORAGE_HOSTPORT"),
		client: &http.Client{Timeout: 35 * time.Second},
	}
	ctx,cancel:=context.WithTimeout(context.Background(),30*time.Second)
	defer cancel()
	if err:=a.migrate(ctx);err!=nil{log.Error("migration","error",err);os.Exit(1)}

	mux:=http.NewServeMux()
	mux.HandleFunc("/health",func(w http.ResponseWriter,r *http.Request){
		common.JSON(w,200,map[string]any{"status":"ok","service":"evidence","time":time.Now().UTC()})
	})
	mux.HandleFunc("/api/v1/evidence",a.collection)
	mux.HandleFunc("/api/v1/evidence/",a.item)
	mux.HandleFunc("/internal/v1/evidence/validate",a.validate)
	mux.HandleFunc("/internal/v1/evidence/query",a.queryInternal)
	mux.HandleFunc("/internal/v1/evidence/report-links",a.reportLinks)
	common.Run(log,"evidence",common.Env("PORT","10000"),common.InternalAuth(a.token,mux))
}

func (a *app)migrate(ctx context.Context)error{
	return common.ApplyMigrations(ctx,a.db,"evidence",[]common.Migration{
		{Version:1,Name:"evidence-library",Statements:[]string{
			`CREATE SCHEMA IF NOT EXISTS evidence`,
			`CREATE TABLE IF NOT EXISTS evidence.items(
				id TEXT PRIMARY KEY,
				partner_id TEXT NOT NULL,
				metric_key TEXT NOT NULL DEFAULT '',
				evidence_type TEXT NOT NULL,
				title TEXT NOT NULL,
				description TEXT NOT NULL DEFAULT '',
				period_start DATE,
				period_end DATE,
				verification_status TEXT NOT NULL DEFAULT 'UNVERIFIED',
				source_url TEXT NOT NULL DEFAULT '',
				declaration_text TEXT NOT NULL DEFAULT '',
				object_namespace TEXT NOT NULL DEFAULT '',
				object_key TEXT NOT NULL DEFAULT '',
				original_filename TEXT NOT NULL DEFAULT '',
				mime_type TEXT NOT NULL DEFAULT '',
				size_bytes BIGINT NOT NULL DEFAULT 0,
				sha256 TEXT NOT NULL DEFAULT '',
				uploaded_by TEXT NOT NULL DEFAULT '',
				verified_by TEXT NOT NULL DEFAULT '',
				verified_at TIMESTAMPTZ,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				CHECK(period_end IS NULL OR period_start IS NULL OR period_end >= period_start)
			)`,
			`CREATE INDEX IF NOT EXISTS evidence_partner_idx ON evidence.items(partner_id,created_at DESC)`,
			`CREATE INDEX IF NOT EXISTS evidence_metric_idx ON evidence.items(metric_key,period_end DESC)`,
			`CREATE INDEX IF NOT EXISTS evidence_verification_idx ON evidence.items(verification_status,created_at DESC)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS evidence_object_idx ON evidence.items(object_namespace,object_key) WHERE object_key<>''`,
			`CREATE TABLE IF NOT EXISTS evidence.report_links(
				report_id TEXT NOT NULL,
				evidence_id TEXT NOT NULL REFERENCES evidence.items(id) ON DELETE CASCADE,
				linked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				PRIMARY KEY(report_id,evidence_id)
			)`,
			`CREATE INDEX IF NOT EXISTS evidence_report_links_evidence_idx ON evidence.report_links(evidence_id)`,
		}},
	})
}

func newID(prefix string)string{
	raw:=make([]byte,10)
	if _,err:=rand.Read(raw);err!=nil{return fmt.Sprintf("%s%d",prefix,time.Now().UnixNano())}
	return prefix+hex.EncodeToString(raw)
}

func normalizeType(v string)string{return strings.ToUpper(strings.TrimSpace(v))}
func normalizeVerification(v string)string{
	v=strings.ToUpper(strings.TrimSpace(v))
	if v==""{return "UNVERIFIED"}
	return v
}

func parseDateOptional(raw string)(sql.NullTime,error){
	raw=strings.TrimSpace(raw)
	if raw==""{return sql.NullTime{},nil}
	t,err:=time.Parse("2006-01-02",raw)
	if err!=nil{return sql.NullTime{},fmt.Errorf("date must be YYYY-MM-DD")}
	return sql.NullTime{Time:t,Valid:true},nil
}

func dateValue(v sql.NullTime)any{
	if !v.Valid{return nil}
	return v.Time.Format("2006-01-02")
}
func timeValue(v sql.NullTime)any{
	if !v.Valid{return nil}
	return v.Time.UTC()
}

func safeFilename(name string)string{
	name=filepath.Base(strings.ReplaceAll(name,"\\","/"))
	var b strings.Builder
	for _,r:=range name{
		switch{
		case r>='a'&&r<='z',r>='A'&&r<='Z',r>='0'&&r<='9',r=='.',r=='-',r=='_':b.WriteRune(r)
		default:b.WriteByte('_')
		}
	}
	out:=strings.Trim(b.String(),"._")
	if out==""{out="evidence.bin"}
	if len(out)>180{out=out[:180]}
	return out
}

func allowedMime(kind,mime string)bool{
	mime=strings.ToLower(strings.TrimSpace(strings.Split(mime,";")[0]))
	switch kind{
	case "PDF","REPORT":
		return mime=="application/pdf"
	case "IMAGE","SCREENSHOT":
		return mime=="image/png"||mime=="image/jpeg"||mime=="image/webp"
	case "INVOICE","CONTRACT":
		return mime=="application/pdf"||mime=="image/png"||mime=="image/jpeg"||mime=="image/webp"
	case "OTHER":
		return mime=="application/pdf"||mime=="image/png"||mime=="image/jpeg"||mime=="image/webp"||mime=="text/plain"
	default:
		return false
	}
}

func (a *app)ensureStorageNamespace(ctx context.Context,namespace string)error{
	req,err:=http.NewRequestWithContext(ctx,http.MethodPost,"http://"+a.storageHost+"/internal/v1/storage/partners/"+url.PathEscape(namespace)+"/ensure",bytes.NewReader([]byte("{}")))
	if err!=nil{return err}
	req.Header.Set("X-Himate-Internal-Token",a.token)
	req.Header.Set("Content-Type","application/json")
	resp,err:=a.client.Do(req)
	if err!=nil{return err}
	defer resp.Body.Close()
	if resp.StatusCode<200||resp.StatusCode>=300{return fmt.Errorf("storage namespace status %d",resp.StatusCode)}
	return nil
}

func (a *app)putObject(ctx context.Context,namespace,key string,file *os.File,size int64)(map[string]any,error){
	if strings.TrimSpace(a.storageHost)==""{return nil,fmt.Errorf("storage service is not configured")}
	if err:=a.ensureStorageNamespace(ctx,namespace);err!=nil{return nil,err}
	if _,err:=file.Seek(0,0);err!=nil{return nil,err}
	req,err:=http.NewRequestWithContext(ctx,http.MethodPut,"http://"+a.storageHost+"/internal/v1/storage/objects/"+url.PathEscape(namespace)+"/"+key,file)
	if err!=nil{return nil,err}
	req.ContentLength=size
	req.Header.Set("X-Himate-Internal-Token",a.token)
	resp,err:=a.client.Do(req)
	if err!=nil{return nil,err}
	defer resp.Body.Close()
	if resp.StatusCode<200||resp.StatusCode>=300{return nil,fmt.Errorf("storage put status %d",resp.StatusCode)}
	var out map[string]any
	if err:=json.NewDecoder(resp.Body).Decode(&out);err!=nil{return nil,err}
	return out,nil
}

func (a *app)getObject(ctx context.Context,namespace,key,mime string)(*http.Response,error){
	path:="http://"+a.storageHost+"/internal/v1/storage/objects/"+url.PathEscape(namespace)+"/"+key+"?content_type="+url.QueryEscape(mime)
	req,err:=http.NewRequestWithContext(ctx,http.MethodGet,path,nil)
	if err!=nil{return nil,err}
	req.Header.Set("X-Himate-Internal-Token",a.token)
	return a.client.Do(req)
}

func readMultipartFile(file multipart.File)(*os.File,int64,string,error){
	tmp,err:=os.CreateTemp("","himate-evidence-*")
	if err!=nil{return nil,0,"",err}
	name:=tmp.Name()
	ok:=false
	defer func(){if !ok{tmp.Close();os.Remove(name)}}()
	h:=sha256.New()
	n,err:=io.Copy(io.MultiWriter(tmp,h),io.LimitReader(file,maxEvidenceBytes+1))
	if err!=nil{return nil,0,"",err}
	if n>maxEvidenceBytes{return nil,0,"",fmt.Errorf("file exceeds 20 MiB")}
	if _,err=tmp.Seek(0,0);err!=nil{return nil,0,"",err}
	ok=true
	return tmp,n,hex.EncodeToString(h.Sum(nil)),nil
}

func sniffMime(file *os.File) (string,error){
	if _,err:=file.Seek(0,0);err!=nil{return "",err}
	buf:=make([]byte,512)
	n,err:=file.Read(buf)
	if err!=nil&&err!=io.EOF{return "",err}
	if _,err:=file.Seek(0,0);err!=nil{return "",err}
	return http.DetectContentType(buf[:n]),nil
}

func (a *app)collection(w http.ResponseWriter,r *http.Request){
	switch r.Method{
	case http.MethodGet:
		a.list(w,r)
	case http.MethodPost:
		if strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")),"multipart/form-data"){
			a.createFileEvidence(w,r)
		}else{
			a.createReferenceEvidence(w,r)
		}
	default:
		common.APIError(w,405,"METHOD","Use GET or POST")
	}
}

func validateCommon(partnerID,metricKey,kind,title,periodStart,periodEnd string)(sql.NullTime,sql.NullTime,error){
	partnerID=strings.TrimSpace(partnerID)
	metricKey=strings.TrimSpace(metricKey)
	kind=normalizeType(kind)
	title=strings.TrimSpace(title)
	if partnerID==""{return sql.NullTime{},sql.NullTime{},fmt.Errorf("partner_id is required")}
	if metricKey!=""&&!metricKeyPattern.MatchString(metricKey){return sql.NullTime{},sql.NullTime{},fmt.Errorf("invalid metric_key")}
	if !evidenceTypes[kind]{return sql.NullTime{},sql.NullTime{},fmt.Errorf("invalid evidence_type")}
	if title==""||len(title)>200{return sql.NullTime{},sql.NullTime{},fmt.Errorf("title is required and must be at most 200 characters")}
	start,err:=parseDateOptional(periodStart);if err!=nil{return start,sql.NullTime{},err}
	end,err:=parseDateOptional(periodEnd);if err!=nil{return start,end,err}
	if start.Valid&&end.Valid&&end.Time.Before(start.Time){return start,end,fmt.Errorf("period_end cannot be before period_start")}
	return start,end,nil
}

func (a *app)createFileEvidence(w http.ResponseWriter,r *http.Request){
	if err:=r.ParseMultipartForm(maxEvidenceBytes+1<<20);err!=nil{common.APIError(w,400,"MULTIPART","Invalid multipart evidence upload");return}
	partnerID:=strings.TrimSpace(r.FormValue("partner_id"))
	metricKey:=strings.TrimSpace(r.FormValue("metric_key"))
	kind:=normalizeType(r.FormValue("evidence_type"))
	title:=strings.TrimSpace(r.FormValue("title"))
	description:=strings.TrimSpace(r.FormValue("description"))
	start,end,err:=validateCommon(partnerID,metricKey,kind,title,r.FormValue("period_start"),r.FormValue("period_end"))
	if err!=nil{common.APIError(w,400,"VALIDATION",err.Error());return}
	if kind=="URL"||kind=="PARTNER_DECLARATION"{common.APIError(w,400,"VALIDATION","URL and PARTNER_DECLARATION evidence must be JSON records");return}
	file,header,err:=r.FormFile("file")
	if err!=nil{common.APIError(w,400,"FILE_REQUIRED","file is required");return}
	defer file.Close()
	tmp,size,sum,err:=readMultipartFile(file)
	if err!=nil{
		if strings.Contains(err.Error(),"20 MiB"){common.APIError(w,413,"FILE_TOO_LARGE",err.Error())}else{common.APIError(w,400,"FILE",err.Error())}
		return
	}
	defer func(){name:=tmp.Name();tmp.Close();os.Remove(name)}()
	mime,err:=sniffMime(tmp)
	if err!=nil||!allowedMime(kind,mime){common.APIError(w,415,"FILE_TYPE","File content type is not allowed for this evidence type");return}
	id:=newID("evd_")
	filename:=safeFilename(header.Filename)
	ext:=strings.ToLower(filepath.Ext(filename))
	if ext==""{
		switch mime{case"application/pdf":ext=".pdf";case"image/png":ext=".png";case"image/jpeg":ext=".jpg";case"image/webp":ext=".webp";default:ext=".bin"}
	}
	key:="evidence/"+id+ext
	stored,err:=a.putObject(r.Context(),partnerID,key,tmp,size)
	if err!=nil{common.APIError(w,502,"STORAGE",err.Error());return}
	if got:=strings.TrimSpace(fmt.Sprint(stored["sha256"]));got!=""&&got!=sum{common.APIError(w,502,"CHECKSUM_MISMATCH","Stored evidence checksum mismatch");return}
	uploadedBy:=strings.TrimSpace(r.Header.Get("X-Himate-User-ID"))
	_,err=a.db.Exec(`INSERT INTO evidence.items(id,partner_id,metric_key,evidence_type,title,description,period_start,period_end,verification_status,object_namespace,object_key,original_filename,mime_type,size_bytes,sha256,uploaded_by)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,'UNVERIFIED',$9,$10,$11,$12,$13,$14,$15)`,
		id,partnerID,metricKey,kind,title,description,start,end,partnerID,key,filename,mime,size,sum,uploadedBy)
	if err!=nil{common.APIError(w,500,"DB","Could not record evidence metadata");return}
	item,err:=a.get(id);if err!=nil{common.APIError(w,500,"DB","Could not reload evidence");return}
	common.JSON(w,201,mapEvidence(item))
}

func (a *app)createReferenceEvidence(w http.ResponseWriter,r *http.Request){
	var in struct{
		PartnerID string `json:"partner_id"`
		MetricKey string `json:"metric_key"`
		EvidenceType string `json:"evidence_type"`
		Title string `json:"title"`
		Description string `json:"description"`
		PeriodStart string `json:"period_start"`
		PeriodEnd string `json:"period_end"`
		SourceURL string `json:"source_url"`
		DeclarationText string `json:"declaration_text"`
	}
	if common.Decode(r,&in)!=nil{common.APIError(w,400,"JSON","Invalid request");return}
	in.EvidenceType=normalizeType(in.EvidenceType)
	start,end,err:=validateCommon(in.PartnerID,in.MetricKey,in.EvidenceType,in.Title,in.PeriodStart,in.PeriodEnd)
	if err!=nil{common.APIError(w,400,"VALIDATION",err.Error());return}
	sourceURL:=strings.TrimSpace(in.SourceURL)
	declaration:=strings.TrimSpace(in.DeclarationText)
	switch in.EvidenceType{
	case"URL":
		u,err:=url.Parse(sourceURL)
		if err!=nil||u.Host==""||(u.Scheme!="https"&&u.Scheme!="http"){common.APIError(w,400,"VALIDATION","source_url must be an absolute HTTP(S) URL");return}
		if declaration!=""{common.APIError(w,400,"VALIDATION","URL evidence cannot contain declaration_text");return}
	case"PARTNER_DECLARATION":
		if declaration==""||len(declaration)>10000{common.APIError(w,400,"VALIDATION","declaration_text is required and must be at most 10000 characters");return}
		if sourceURL!=""{common.APIError(w,400,"VALIDATION","PARTNER_DECLARATION cannot contain source_url");return}
	default:
		common.APIError(w,400,"VALIDATION","This evidence type requires a multipart file upload")
		return
	}
	id:=newID("evd_")
	uploadedBy:=strings.TrimSpace(r.Header.Get("X-Himate-User-ID"))
	_,err=a.db.Exec(`INSERT INTO evidence.items(id,partner_id,metric_key,evidence_type,title,description,period_start,period_end,verification_status,source_url,declaration_text,uploaded_by)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,'UNVERIFIED',$9,$10,$11)`,
		id,strings.TrimSpace(in.PartnerID),strings.TrimSpace(in.MetricKey),in.EvidenceType,strings.TrimSpace(in.Title),strings.TrimSpace(in.Description),start,end,sourceURL,declaration,uploadedBy)
	if err!=nil{common.APIError(w,500,"DB","Could not create evidence");return}
	item,_:=a.get(id)
	common.JSON(w,201,mapEvidence(item))
}

func evidenceSelect()string{
	return `SELECT id,partner_id,metric_key,evidence_type,title,description,period_start,period_end,verification_status,source_url,declaration_text,object_namespace,object_key,original_filename,mime_type,size_bytes,sha256,uploaded_by,verified_by,verified_at,created_at,updated_at FROM evidence.items`
}
type scanner interface{Scan(...any)error}
func scanEvidence(s scanner)(e evidence,err error){
	err=s.Scan(&e.ID,&e.PartnerID,&e.MetricKey,&e.EvidenceType,&e.Title,&e.Description,&e.PeriodStart,&e.PeriodEnd,&e.VerificationStatus,&e.SourceURL,&e.DeclarationText,&e.ObjectNamespace,&e.ObjectKey,&e.OriginalFilename,&e.MimeType,&e.SizeBytes,&e.SHA256,&e.UploadedBy,&e.VerifiedBy,&e.VerifiedAt,&e.CreatedAt,&e.UpdatedAt)
	return
}
func (a *app)get(id string)(evidence,error){return scanEvidence(a.db.QueryRow(evidenceSelect()+" WHERE id=$1",id))}
func mapEvidence(e evidence)map[string]any{
	return map[string]any{
		"id":e.ID,"partner_id":e.PartnerID,"metric_key":e.MetricKey,"evidence_type":e.EvidenceType,
		"title":e.Title,"description":e.Description,"period_start":dateValue(e.PeriodStart),"period_end":dateValue(e.PeriodEnd),
		"verification_status":e.VerificationStatus,"source_url":e.SourceURL,"declaration_text":e.DeclarationText,
		"original_filename":e.OriginalFilename,"mime_type":e.MimeType,"size_bytes":e.SizeBytes,"sha256":e.SHA256,
		"uploaded_by":e.UploadedBy,"verified_by":e.VerifiedBy,"verified_at":timeValue(e.VerifiedAt),
		"has_file":e.ObjectKey!="","created_at":e.CreatedAt,"updated_at":e.UpdatedAt,
	}
}

func (a *app)list(w http.ResponseWriter,r *http.Request){
	partnerID:=strings.TrimSpace(r.URL.Query().Get("partner_id"))
	metricKey:=strings.TrimSpace(r.URL.Query().Get("metric_key"))
	kind:=normalizeType(r.URL.Query().Get("evidence_type"))
	verification:=normalizeVerification(r.URL.Query().Get("verification_status"))
	reportID:=strings.TrimSpace(r.URL.Query().Get("report_id"))
	limit:=100
	if raw:=r.URL.Query().Get("limit");raw!=""{if v,err:=strconv.Atoi(raw);err==nil&&v>0&&v<=500{limit=v}}
	where:=[]string{"1=1"};args:=[]any{}
	if partnerID!=""{args=append(args,partnerID);where=append(where,fmt.Sprintf("e.partner_id=$%d",len(args)))}
	if metricKey!=""{args=append(args,metricKey);where=append(where,fmt.Sprintf("e.metric_key=$%d",len(args)))}
	if kind!=""{if !evidenceTypes[kind]{common.APIError(w,400,"VALIDATION","Invalid evidence_type");return};args=append(args,kind);where=append(where,fmt.Sprintf("e.evidence_type=$%d",len(args)))}
	if verification!=""&&r.URL.Query().Get("verification_status")!=""{if !verificationValues[verification]{common.APIError(w,400,"VALIDATION","Invalid verification_status");return};args=append(args,verification);where=append(where,fmt.Sprintf("e.verification_status=$%d",len(args)))}
	join:=""
	if reportID!=""{join=" JOIN evidence.report_links l ON l.evidence_id=e.id ";args=append(args,reportID);where=append(where,fmt.Sprintf("l.report_id=$%d",len(args)))}
	args=append(args,limit)
	q:=strings.Replace(evidenceSelect()," FROM evidence.items"," FROM evidence.items e",1)
	q=strings.Replace(q,"SELECT id,partner_id,metric_key,evidence_type,title,description,period_start,period_end,verification_status,source_url,declaration_text,object_namespace,object_key,original_filename,mime_type,size_bytes,sha256,uploaded_by,verified_by,verified_at,created_at,updated_at",
		"SELECT e.id,e.partner_id,e.metric_key,e.evidence_type,e.title,e.description,e.period_start,e.period_end,e.verification_status,e.source_url,e.declaration_text,e.object_namespace,e.object_key,e.original_filename,e.mime_type,e.size_bytes,e.sha256,e.uploaded_by,e.verified_by,e.verified_at,e.created_at,e.updated_at",1)
	q+=join+" WHERE "+strings.Join(where," AND ")+" ORDER BY e.created_at DESC LIMIT $"+strconv.Itoa(len(args))
	rows,err:=a.db.Query(q,args...)
	if err!=nil{common.APIError(w,500,"DB","Could not load evidence");return}
	defer rows.Close()
	items:=[]map[string]any{}
	for rows.Next(){if e,err:=scanEvidence(rows);err==nil{items=append(items,mapEvidence(e))}}
	common.JSON(w,200,map[string]any{"items":items,"count":len(items)})
}

func (a *app)item(w http.ResponseWriter,r *http.Request){
	raw:=strings.Trim(strings.TrimPrefix(r.URL.Path,"/api/v1/evidence/"),"/")
	parts:=strings.Split(raw,"/")
	if len(parts)==0||parts[0]==""{common.APIError(w,404,"NOT_FOUND","Evidence not found");return}
	id:=parts[0]
	if len(parts)==2&&parts[1]=="download"{
		if r.Method!=http.MethodGet{common.APIError(w,405,"METHOD","Use GET");return}
		e,err:=a.get(id);if err!=nil{common.APIError(w,404,"NOT_FOUND","Evidence not found");return}
		if e.ObjectKey==""{common.APIError(w,409,"NO_FILE","Evidence has no stored file");return}
		resp,err:=a.getObject(r.Context(),e.ObjectNamespace,e.ObjectKey,e.MimeType)
		if err!=nil{common.APIError(w,502,"STORAGE","Could not load evidence object");return}
		defer resp.Body.Close()
		if resp.StatusCode<200||resp.StatusCode>=300{common.APIError(w,502,"STORAGE","Evidence object unavailable");return}
		w.Header().Set("Content-Type",e.MimeType)
		w.Header().Set("Content-Disposition",`attachment; filename="`+safeFilename(e.OriginalFilename)+`"`)
		w.Header().Set("Cache-Control","private, no-store")
		_,_=io.Copy(w,resp.Body)
		return
	}
	if len(parts)!=1{common.APIError(w,404,"NOT_FOUND","Evidence route not found");return}
	switch r.Method{
	case http.MethodGet:
		e,err:=a.get(id);if err!=nil{common.APIError(w,404,"NOT_FOUND","Evidence not found");return}
		common.JSON(w,200,mapEvidence(e))
	case http.MethodPatch:
		var in struct{VerificationStatus string `json:"verification_status"`}
		if common.Decode(r,&in)!=nil{common.APIError(w,400,"JSON","Invalid request");return}
		status:=normalizeVerification(in.VerificationStatus)
		if !verificationValues[status]{common.APIError(w,400,"VALIDATION","Invalid verification_status");return}
		actor:=strings.TrimSpace(r.Header.Get("X-Himate-User-ID"))
		_,err:=a.db.Exec(`UPDATE evidence.items SET verification_status=$2,verified_by=CASE WHEN $2='VERIFIED' THEN $3 ELSE '' END,verified_at=CASE WHEN $2='VERIFIED' THEN NOW() ELSE NULL END,updated_at=NOW() WHERE id=$1`,id,status,actor)
		if err!=nil{common.APIError(w,500,"DB","Could not update evidence verification");return}
		e,err:=a.get(id);if err!=nil{common.APIError(w,404,"NOT_FOUND","Evidence not found");return}
		common.JSON(w,200,mapEvidence(e))
	default:
		common.APIError(w,405,"METHOD","Use GET or PATCH")
	}
}

func (a *app)validate(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodGet{common.APIError(w,405,"METHOD","Use GET");return}
	id:=strings.TrimSpace(r.URL.Query().Get("evidence_id"))
	partnerID:=strings.TrimSpace(r.URL.Query().Get("partner_id"))
	metricKey:=strings.TrimSpace(r.URL.Query().Get("metric_key"))
	e,err:=a.get(id)
	if err!=nil{common.APIError(w,404,"EVIDENCE_NOT_FOUND","Evidence record not found");return}
	if e.VerificationStatus!="VERIFIED"{common.APIError(w,409,"EVIDENCE_NOT_VERIFIED","Evidence is not verified");return}
	if e.ObjectKey==""{common.APIError(w,409,"EVIDENCE_NOT_DOCUMENT","VERIFIED_DOCUMENT provenance requires file-backed Evidence");return}
	if partnerID!=""&&e.PartnerID!=partnerID{common.APIError(w,409,"EVIDENCE_PARTNER_MISMATCH","Evidence belongs to another partner");return}
	if metricKey!=""&&e.MetricKey!=metricKey{common.APIError(w,409,"EVIDENCE_METRIC_MISMATCH","Evidence must be linked to the same metric");return}
	common.JSON(w,200,map[string]any{"valid":true,"evidence":mapEvidence(e)})
}

func (a *app)queryInternal(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodGet{common.APIError(w,405,"METHOD","Use GET");return}
	a.list(w,r)
}

func (a *app)reportLinks(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodPost{common.APIError(w,405,"METHOD","Use POST");return}
	var in struct{ReportID string `json:"report_id"`; EvidenceIDs []string `json:"evidence_ids"`}
	if common.Decode(r,&in)!=nil||strings.TrimSpace(in.ReportID)==""{common.APIError(w,400,"VALIDATION","report_id is required");return}
	tx,err:=a.db.BeginTx(r.Context(),&sql.TxOptions{})
	if err!=nil{common.APIError(w,500,"DB","Could not start evidence link transaction");return}
	defer tx.Rollback()
	for _,id:=range in.EvidenceIDs{
		id=strings.TrimSpace(id);if id==""{continue}
		if _,err=tx.ExecContext(r.Context(),`INSERT INTO evidence.report_links(report_id,evidence_id) VALUES($1,$2) ON CONFLICT DO NOTHING`,strings.TrimSpace(in.ReportID),id);err!=nil{
			common.APIError(w,409,"EVIDENCE_LINK","Could not link report evidence");return
		}
	}
	if err=tx.Commit();err!=nil{common.APIError(w,500,"DB","Could not commit evidence links");return}
	common.JSON(w,200,map[string]any{"report_id":strings.TrimSpace(in.ReportID),"linked":len(in.EvidenceIDs)})
}
