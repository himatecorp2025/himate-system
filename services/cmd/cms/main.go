package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"himate.local/services/internal/common"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const maxMediaBytes = 10 << 20

var pageKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,63}$`)
var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var sectionIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,79}$`)
var componentTypes = map[string]bool{
	"HERO":true,"MISSION":true,"INDUSTRIES":true,"TECHNOLOGY":true,
	"PARTNER_NETWORK":true,"IMPACT":true,"CASE_STUDIES":true,"CONTACT":true,
	"FEATURE":true,"CTA":true,"TEXT":true,
}

type app struct {
	db *sql.DB
	token string
	storageHost string
	client *http.Client
}

type seoInput struct {
	Title string `json:"title"`
	MetaDescription string `json:"meta_description"`
	Canonical string `json:"canonical"`
	OGTitle string `json:"og_title"`
	OGDescription string `json:"og_description"`
	OGImageAssetID string `json:"og_image_asset_id"`
	NoIndex bool `json:"noindex"`
}

type sectionInput struct {
	ID string `json:"id"`
	ComponentType string `json:"component_type"`
	Heading string `json:"heading"`
	Body string `json:"body"`
	MediaAssetID string `json:"media_asset_id"`
	CTALabel string `json:"cta_label"`
	CTAURL string `json:"cta_url"`
	Visible bool `json:"visible"`
	SortOrder int `json:"sort_order"`
	Settings map[string]any `json:"settings"`
}

type versionInput struct {
	Slug string `json:"slug"`
	SEO seoInput `json:"seo"`
	Sections []sectionInput `json:"sections"`
}

type pageRow struct {
	ID,PageKey,Name,DraftVersionID,PreviewVersionID,PublishedVersionID,PreviewTokenHash string
	PreviewTokenIssuedAt sql.NullTime
	CreatedAt,UpdatedAt time.Time
}

type versionRow struct {
	ID,PageID,State,Slug,CreatedBy,SourceVersionID,RollbackOfVersionID,PublishedBy string
	VersionNo int
	SEO,Sections []byte
	PublishedAt sql.NullTime
	CreatedAt time.Time
}

type mediaRow struct {
	ID,OriginalFilename,MimeType,ObjectNamespace,ObjectKey,SHA256,AltText,CreatedBy string
	SizeBytes int64
	CreatedAt time.Time
}

func main(){
	log:=common.Logger()
	db,err:=common.OpenDB();if err!=nil{log.Error("database","error",err);os.Exit(1)};defer db.Close()
	a:=&app{
		db:db,
		token:os.Getenv("HIMATE_INTERNAL_TOKEN"),
		storageHost:os.Getenv("STORAGE_HOSTPORT"),
		client:&http.Client{Timeout:20*time.Second},
	}
	ctx,cancel:=context.WithTimeout(context.Background(),30*time.Second);defer cancel()
	if err:=a.migrate(ctx);err!=nil{log.Error("migration","error",err);os.Exit(1)}

	mux:=http.NewServeMux()
	mux.HandleFunc("/health",func(w http.ResponseWriter,r *http.Request){common.JSON(w,200,map[string]any{"status":"ok","service":"cms","time":time.Now().UTC()})})
	mux.HandleFunc("/api/v1/cms/pages",a.pages)
	mux.HandleFunc("/api/v1/cms/pages/",a.page)
	mux.HandleFunc("/api/v1/cms/media",a.mediaCollection)
	mux.HandleFunc("/api/v1/cms/media/",a.mediaItem)
	mux.HandleFunc("/public/v1/cms/pages/",a.publicPage)
	mux.HandleFunc("/public/v1/cms/media/",a.publicMedia)
	mux.HandleFunc("/public/v1/cms/manifest",a.publicManifest)
	mux.HandleFunc("/preview/v1/cms/pages/",a.previewPage)
	mux.HandleFunc("/preview/v1/cms/media/",a.previewMedia)
	common.Run(log,"cms",common.Env("PORT","10000"),common.InternalAuth(a.token,mux))
}

func (a *app)migrate(ctx context.Context)error{
	return common.ApplyMigrations(ctx,a.db,"cms",[]common.Migration{
		{Version:1,Name:"cms-versioned-content",Statements:[]string{
			`CREATE SCHEMA IF NOT EXISTS cms`,
			`CREATE TABLE IF NOT EXISTS cms.pages(
				id TEXT PRIMARY KEY,
				page_key TEXT NOT NULL UNIQUE,
				name TEXT NOT NULL,
				draft_version_id TEXT NOT NULL DEFAULT '',
				preview_version_id TEXT NOT NULL DEFAULT '',
				published_version_id TEXT NOT NULL DEFAULT '',
				preview_token_hash TEXT NOT NULL DEFAULT '',
				preview_token_issued_at TIMESTAMPTZ,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE TABLE IF NOT EXISTS cms.versions(
				id TEXT PRIMARY KEY,
				page_id TEXT NOT NULL REFERENCES cms.pages(id) ON DELETE CASCADE,
				version_no INTEGER NOT NULL,
				state TEXT NOT NULL CHECK(state IN ('DRAFT','PREVIEW','PUBLISHED')),
				slug TEXT NOT NULL,
				seo JSONB NOT NULL DEFAULT '{}'::jsonb,
				sections JSONB NOT NULL DEFAULT '[]'::jsonb,
				created_by TEXT NOT NULL DEFAULT '',
				source_version_id TEXT NOT NULL DEFAULT '',
				rollback_of_version_id TEXT NOT NULL DEFAULT '',
				published_by TEXT NOT NULL DEFAULT '',
				published_at TIMESTAMPTZ,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				UNIQUE(page_id,version_no)
			)`,
			`CREATE INDEX IF NOT EXISTS cms_versions_page_created_idx ON cms.versions(page_id,version_no DESC)`,
			`CREATE INDEX IF NOT EXISTS cms_versions_state_idx ON cms.versions(state,created_at DESC)`,
			`CREATE TABLE IF NOT EXISTS cms.media_assets(
				id TEXT PRIMARY KEY,
				original_filename TEXT NOT NULL,
				mime_type TEXT NOT NULL,
				object_namespace TEXT NOT NULL,
				object_key TEXT NOT NULL UNIQUE,
				size_bytes BIGINT NOT NULL,
				sha256 TEXT NOT NULL,
				alt_text TEXT NOT NULL DEFAULT '',
				created_by TEXT NOT NULL DEFAULT '',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE TABLE IF NOT EXISTS cms.published_media_refs(
				page_id TEXT NOT NULL REFERENCES cms.pages(id) ON DELETE CASCADE,
				version_id TEXT NOT NULL REFERENCES cms.versions(id) ON DELETE CASCADE,
				media_id TEXT NOT NULL REFERENCES cms.media_assets(id) ON DELETE CASCADE,
				PRIMARY KEY(page_id,media_id)
			)`,
			`CREATE INDEX IF NOT EXISTS cms_published_media_idx ON cms.published_media_refs(media_id)`,
			`CREATE TABLE IF NOT EXISTS cms.audit_events(
				id TEXT PRIMARY KEY,
				page_id TEXT NOT NULL DEFAULT '',
				version_id TEXT NOT NULL DEFAULT '',
				action TEXT NOT NULL,
				actor TEXT NOT NULL DEFAULT '',
				correlation_id TEXT NOT NULL DEFAULT '',
				old_state JSONB NOT NULL DEFAULT '{}'::jsonb,
				new_state JSONB NOT NULL DEFAULT '{}'::jsonb,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE INDEX IF NOT EXISTS cms_audit_page_idx ON cms.audit_events(page_id,created_at DESC)`,
		}},
	})
}

func newID(prefix string)string{
	raw:=make([]byte,10)
	if _,err:=rand.Read(raw);err!=nil{return fmt.Sprintf("%s%d",prefix,time.Now().UnixNano())}
	return prefix+hex.EncodeToString(raw)
}

func randomToken()string{
	raw:=make([]byte,32)
	if _,err:=rand.Read(raw);err!=nil{return ""}
	return hex.EncodeToString(raw)
}

func tokenHash(v string)string{
	sum:=sha256.Sum256([]byte(v))
	return hex.EncodeToString(sum[:])
}

func tokenMatches(raw,stored string)bool{
	got:=tokenHash(strings.TrimSpace(raw))
	if len(got)!=len(stored)||len(stored)==0{return false}
	return subtle.ConstantTimeCompare([]byte(got),[]byte(stored))==1
}

func actor(r *http.Request)string{return strings.TrimSpace(r.Header.Get("X-Himate-User-ID"))}
func correlationID(r *http.Request)string{
	if v:=strings.TrimSpace(r.Header.Get("X-Correlation-ID"));v!=""{return v}
	return newID("corr_")
}

func jsonBytes(v any)[]byte{raw,_:=json.Marshal(v);return raw}

func (a *app)audit(ctx context.Context,pageID,versionID,action,actorID,corr string,oldState,newState any)error{
	_,err:=a.db.ExecContext(ctx,`INSERT INTO cms.audit_events(id,page_id,version_id,action,actor,correlation_id,old_state,new_state)
		VALUES($1,$2,$3,$4,$5,$6,$7::jsonb,$8::jsonb)`,
		newID("cms_evt_"),pageID,versionID,action,actorID,corr,string(jsonBytes(oldState)),string(jsonBytes(newState)))
	return err
}

func pageSelect()string{return `SELECT id,page_key,name,draft_version_id,preview_version_id,published_version_id,preview_token_hash,preview_token_issued_at,created_at,updated_at FROM cms.pages`}
type scanner interface{Scan(...any)error}
func scanPage(s scanner)(p pageRow,err error){err=s.Scan(&p.ID,&p.PageKey,&p.Name,&p.DraftVersionID,&p.PreviewVersionID,&p.PublishedVersionID,&p.PreviewTokenHash,&p.PreviewTokenIssuedAt,&p.CreatedAt,&p.UpdatedAt);return}
func (a *app)getPage(id string)(pageRow,error){return scanPage(a.db.QueryRow(pageSelect()+" WHERE id=$1",id))}

func versionSelect()string{return `SELECT id,page_id,version_no,state,slug,seo,sections,created_by,source_version_id,rollback_of_version_id,published_by,published_at,created_at FROM cms.versions`}
func scanVersion(s scanner)(v versionRow,err error){err=s.Scan(&v.ID,&v.PageID,&v.VersionNo,&v.State,&v.Slug,&v.SEO,&v.Sections,&v.CreatedBy,&v.SourceVersionID,&v.RollbackOfVersionID,&v.PublishedBy,&v.PublishedAt,&v.CreatedAt);return}
func (a *app)getVersion(id string)(versionRow,error){if strings.TrimSpace(id)==""{return versionRow{},sql.ErrNoRows};return scanVersion(a.db.QueryRow(versionSelect()+" WHERE id=$1",id))}

func decodeVersion(v versionRow)map[string]any{
	var seo any=map[string]any{};var sections any=[]any{}
	_ = json.Unmarshal(v.SEO,&seo);_ = json.Unmarshal(v.Sections,&sections)
	var published any;if v.PublishedAt.Valid{published=v.PublishedAt.Time.UTC()}
	return map[string]any{
		"id":v.ID,"page_id":v.PageID,"version_no":v.VersionNo,"state":v.State,"slug":v.Slug,
		"seo":seo,"sections":sections,"created_by":v.CreatedBy,"source_version_id":v.SourceVersionID,
		"rollback_of_version_id":v.RollbackOfVersionID,"published_by":v.PublishedBy,
		"published_at":published,"created_at":v.CreatedAt,
	}
}

func (a *app)versionNo(id string)int{
	if strings.TrimSpace(id)==""{return 0}
	var n int;_ = a.db.QueryRow(`SELECT version_no FROM cms.versions WHERE id=$1`,id).Scan(&n);return n
}

func (a *app)mapPage(p pageRow)map[string]any{
	draftNo:=a.versionNo(p.DraftVersionID);previewNo:=a.versionNo(p.PreviewVersionID);publishedNo:=a.versionNo(p.PublishedVersionID)
	state:="DRAFT"
	if publishedNo>0&&publishedNo>=draftNo&&publishedNo>=previewNo{state="PUBLISHED"}else if previewNo>0&&previewNo>=draftNo{state="PREVIEW"}
	var issued any;if p.PreviewTokenIssuedAt.Valid{issued=p.PreviewTokenIssuedAt.Time.UTC()}
	return map[string]any{
		"id":p.ID,"page_key":p.PageKey,"name":p.Name,"workflow_state":state,
		"draft_version_id":p.DraftVersionID,"draft_version_no":draftNo,
		"preview_version_id":p.PreviewVersionID,"preview_version_no":previewNo,
		"published_version_id":p.PublishedVersionID,"published_version_no":publishedNo,
		"preview_token_active":p.PreviewTokenHash!="","preview_token_issued_at":issued,
		"created_at":p.CreatedAt,"updated_at":p.UpdatedAt,
	}
}

// Placeholder only prevents accidental use of a package-level state helper.
// Page state is calculated through app.mapPage because it requires DB version numbers.
func aVersionNumberPlaceholder()(int,error){return 0,fmt.Errorf("not used")}

func normalizedInput(in versionInput)versionInput{
	in.Slug=strings.ToLower(strings.Trim(strings.TrimSpace(in.Slug),"/"))
	in.SEO.Title=strings.TrimSpace(in.SEO.Title)
	in.SEO.MetaDescription=strings.TrimSpace(in.SEO.MetaDescription)
	in.SEO.Canonical=strings.TrimSpace(in.SEO.Canonical)
	in.SEO.OGTitle=strings.TrimSpace(in.SEO.OGTitle)
	in.SEO.OGDescription=strings.TrimSpace(in.SEO.OGDescription)
	in.SEO.OGImageAssetID=strings.TrimSpace(in.SEO.OGImageAssetID)
	for i:=range in.Sections{
		s:=&in.Sections[i]
		s.ID=strings.ToLower(strings.TrimSpace(s.ID))
		s.ComponentType=strings.ToUpper(strings.TrimSpace(s.ComponentType))
		s.Heading=strings.TrimSpace(s.Heading);s.Body=strings.TrimSpace(s.Body)
		s.MediaAssetID=strings.TrimSpace(s.MediaAssetID);s.CTALabel=strings.TrimSpace(s.CTALabel);s.CTAURL=strings.TrimSpace(s.CTAURL)
		if s.Settings==nil{s.Settings=map[string]any{}}
	}
	sort.SliceStable(in.Sections,func(i,j int)bool{return in.Sections[i].SortOrder<in.Sections[j].SortOrder})
	return in
}

func safeCTA(v string)bool{
	v=strings.TrimSpace(v)
	if v==""{return true}
	if strings.HasPrefix(v,"/")&&!strings.HasPrefix(v,"//"){return true}
	u,err:=url.Parse(v);if err!=nil{return false}
	return (u.Scheme=="https"||u.Scheme=="http")&&u.Host!=""
}

func validCanonical(v string)bool{
	u,err:=url.Parse(strings.TrimSpace(v));if err!=nil{return false}
	return u.Scheme=="https"&&u.Host!=""&&u.Fragment==""
}

func (a *app)mediaExists(ctx context.Context,id string)bool{
	if strings.TrimSpace(id)==""{return true}
	var exists bool;_ = a.db.QueryRowContext(ctx,`SELECT EXISTS(SELECT 1 FROM cms.media_assets WHERE id=$1)`,id).Scan(&exists);return exists
}

func (a *app)validateContent(ctx context.Context,pageID string,in versionInput,forPublish bool)error{
	in=normalizedInput(in)
	if !slugPattern.MatchString(in.Slug)||len(in.Slug)>80{return fmt.Errorf("slug must be at most 80 characters and contain lowercase letters, numbers and single hyphens only")}
	if len(in.Sections)>100{return fmt.Errorf("a CMS page may contain at most 100 sections")}
	if len(in.Sections)==0&&forPublish{return fmt.Errorf("at least one section is required for publishing")}
	seenID:=map[string]bool{};seenOrder:=map[int]bool{}
	visible:=0
	for _,s:=range in.Sections{
		if !sectionIDPattern.MatchString(s.ID){return fmt.Errorf("invalid section id %q",s.ID)}
		if seenID[s.ID]{return fmt.Errorf("duplicate section id %q",s.ID)};seenID[s.ID]=true
		if !componentTypes[s.ComponentType]{return fmt.Errorf("unsupported component type %q",s.ComponentType)}
		if len(s.Heading)>240{return fmt.Errorf("section %q heading is too long",s.ID)}
		if len(s.Body)>20000{return fmt.Errorf("section %q body is too long",s.ID)}
		if len(s.CTALabel)>100{return fmt.Errorf("section %q CTA label is too long",s.ID)}
		if len(s.CTAURL)>2048{return fmt.Errorf("section %q CTA URL is too long",s.ID)}
		settingsRaw,_:=json.Marshal(s.Settings)
		if len(settingsRaw)>16384{return fmt.Errorf("section %q settings exceed 16 KiB",s.ID)}
		if s.SortOrder<0{return fmt.Errorf("sort_order cannot be negative")}
		if seenOrder[s.SortOrder]{return fmt.Errorf("duplicate sort_order %d",s.SortOrder)};seenOrder[s.SortOrder]=true
		if (s.CTALabel=="")!=(s.CTAURL==""){return fmt.Errorf("CTA label and URL must be provided together")}
		if !safeCTA(s.CTAURL){return fmt.Errorf("invalid CTA URL in section %q",s.ID)}
		if s.MediaAssetID!=""&&!a.mediaExists(ctx,s.MediaAssetID){return fmt.Errorf("media asset %q does not exist",s.MediaAssetID)}
		if s.Visible{
			visible++
			if forPublish&&s.Heading==""{return fmt.Errorf("visible section %q requires a heading",s.ID)}
		}
	}
	if forPublish&&visible==0{return fmt.Errorf("at least one visible section is required for publishing")}
	if forPublish{
		if in.SEO.Title==""||len(in.SEO.Title)>80{return fmt.Errorf("SEO title is required and must be at most 80 characters")}
		if in.SEO.MetaDescription==""||len(in.SEO.MetaDescription)>180{return fmt.Errorf("meta description is required and must be at most 180 characters")}
		if !validCanonical(in.SEO.Canonical){return fmt.Errorf("canonical must be an absolute HTTPS URL")}
	}
	if in.SEO.OGImageAssetID!=""&&!a.mediaExists(ctx,in.SEO.OGImageAssetID){return fmt.Errorf("Open Graph media asset does not exist")}
	if in.SEO.Canonical!=""&&!validCanonical(in.SEO.Canonical){return fmt.Errorf("canonical must be an absolute HTTPS URL")}
	if in.SEO.OGTitle!=""&&len(in.SEO.OGTitle)>100{return fmt.Errorf("Open Graph title is too long")}
	if in.SEO.OGDescription!=""&&len(in.SEO.OGDescription)>220{return fmt.Errorf("Open Graph description is too long")}
	return nil
}

func inputFromVersion(v versionRow)(versionInput,error){
	var in versionInput
	in.Slug=v.Slug
	if err:=json.Unmarshal(v.SEO,&in.SEO);err!=nil{return in,err}
	if err:=json.Unmarshal(v.Sections,&in.Sections);err!=nil{return in,err}
	return normalizedInput(in),nil
}

func (a *app)nextVersionNo(ctx context.Context,tx *sql.Tx,pageID string)(int,error){
	var n int
	err:=tx.QueryRowContext(ctx,`SELECT COALESCE(MAX(version_no),0)+1 FROM cms.versions WHERE page_id=$1`,pageID).Scan(&n)
	return n,err
}

func (a *app)insertVersion(ctx context.Context,tx *sql.Tx,pageID,state string,in versionInput,createdBy,sourceID,rollbackID,publishedBy string)(versionRow,error){
	in=normalizedInput(in)
	n,err:=a.nextVersionNo(ctx,tx,pageID);if err!=nil{return versionRow{},err}
	id:=newID("cms_ver_")
	seoRaw,_:=json.Marshal(in.SEO);sectionsRaw,_:=json.Marshal(in.Sections)
	var publishedAt any
	if state=="PUBLISHED"{publishedAt=time.Now().UTC()}
	_,err=tx.ExecContext(ctx,`INSERT INTO cms.versions(id,page_id,version_no,state,slug,seo,sections,created_by,source_version_id,rollback_of_version_id,published_by,published_at)
		VALUES($1,$2,$3,$4,$5,$6::jsonb,$7::jsonb,$8,$9,$10,$11,$12)`,
		id,pageID,n,state,in.Slug,string(seoRaw),string(sectionsRaw),createdBy,sourceID,rollbackID,publishedBy,publishedAt)
	if err!=nil{return versionRow{},err}
	return scanVersion(tx.QueryRowContext(ctx,versionSelect()+" WHERE id=$1",id))
}

func (a *app)pages(w http.ResponseWriter,r *http.Request){
	switch r.Method{
	case http.MethodGet:
		rows,err:=a.db.Query(pageSelect()+" ORDER BY name,page_key")
		if err!=nil{common.APIError(w,500,"DB","Could not load CMS pages");return}
		defer rows.Close();items:=[]map[string]any{}
		for rows.Next(){if p,err:=scanPage(rows);err==nil{items=append(items,a.mapPage(p))}}
		common.JSON(w,200,map[string]any{"items":items,"count":len(items)})
	case http.MethodPost:
		var in struct{
			PageKey string `json:"page_key"`
			Name string `json:"name"`
			Version versionInput `json:"version"`
		}
		if common.Decode(r,&in)!=nil{common.APIError(w,400,"JSON","Invalid request");return}
		in.PageKey=strings.ToLower(strings.TrimSpace(in.PageKey));in.Name=strings.TrimSpace(in.Name);in.Version=normalizedInput(in.Version)
		if !pageKeyPattern.MatchString(in.PageKey)||in.Name==""||len(in.Name)>120{common.APIError(w,400,"VALIDATION","page_key and a name up to 120 characters are required");return}
		if err:=a.validateContent(r.Context(),"",in.Version,false);err!=nil{common.APIError(w,400,"VALIDATION",err.Error());return}
		tx,err:=a.db.BeginTx(r.Context(),&sql.TxOptions{});if err!=nil{common.APIError(w,500,"DB","Could not start CMS transaction");return};defer tx.Rollback()
		pageID:=newID("cms_page_")
		if _,err=tx.ExecContext(r.Context(),`INSERT INTO cms.pages(id,page_key,name) VALUES($1,$2,$3)`,pageID,in.PageKey,in.Name);err!=nil{
			common.APIError(w,409,"PAGE_CONFLICT","CMS page key already exists");return
		}
		v,err:=a.insertVersion(r.Context(),tx,pageID,"DRAFT",in.Version,actor(r),"","","")
		if err!=nil{common.APIError(w,500,"DB","Could not create CMS draft");return}
		if _,err=tx.ExecContext(r.Context(),`UPDATE cms.pages SET draft_version_id=$2,updated_at=NOW() WHERE id=$1`,pageID,v.ID);err!=nil{common.APIError(w,500,"DB","Could not activate CMS draft");return}
		if err=a.auditTx(r.Context(),tx,pageID,v.ID,"PAGE_CREATED",actor(r),correlationID(r),map[string]any{},map[string]any{"page_key":in.PageKey,"name":in.Name,"version":decodeVersion(v)});err!=nil{common.APIError(w,500,"DB","Could not audit CMS page");return}
		if err=tx.Commit();err!=nil{common.APIError(w,500,"DB","Could not commit CMS page");return}
		p,_:=a.getPage(pageID);common.JSON(w,201,map[string]any{"page":a.mapPage(p),"draft":decodeVersion(v)})
	default:common.APIError(w,405,"METHOD","Use GET or POST")
	}
}

func (a *app)auditTx(ctx context.Context,tx *sql.Tx,pageID,versionID,action,actorID,corr string,oldState,newState any)error{
	_,err:=tx.ExecContext(ctx,`INSERT INTO cms.audit_events(id,page_id,version_id,action,actor,correlation_id,old_state,new_state)
		VALUES($1,$2,$3,$4,$5,$6,$7::jsonb,$8::jsonb)`,
		newID("cms_evt_"),pageID,versionID,action,actorID,corr,string(jsonBytes(oldState)),string(jsonBytes(newState)))
	return err
}

func (a *app)page(w http.ResponseWriter,r *http.Request){
	raw:=strings.Trim(strings.TrimPrefix(r.URL.Path,"/api/v1/cms/pages/"),"/")
	parts:=strings.Split(raw,"/")
	if len(parts)==0||parts[0]==""{common.APIError(w,404,"NOT_FOUND","CMS page not found");return}
	pageID:=parts[0]
	p,err:=a.getPage(pageID);if err!=nil{common.APIError(w,404,"NOT_FOUND","CMS page not found");return}
	if len(parts)==1{
		if r.Method!=http.MethodGet{common.APIError(w,405,"METHOD","Use GET");return}
		out:=a.mapPage(p)
		if v,err:=a.getVersion(p.DraftVersionID);err==nil{out["draft"]=decodeVersion(v)}
		if v,err:=a.getVersion(p.PreviewVersionID);err==nil{out["preview"]=decodeVersion(v)}
		if v,err:=a.getVersion(p.PublishedVersionID);err==nil{out["published"]=decodeVersion(v)}
		common.JSON(w,200,out);return
	}
	action:=parts[1]
	switch action{
	case"draft":a.saveDraft(w,r,p)
	case"preview":a.promotePreview(w,r,p)
	case"preview-token":a.rotatePreviewToken(w,r,p)
	case"publish":a.publish(w,r,p)
	case"rollback":a.rollback(w,r,p)
	case"versions":a.versions(w,r,p)
	case"audit":a.auditLog(w,r,p)
	default:common.APIError(w,404,"NOT_FOUND","CMS page action not found")
	}
}

func (a *app)saveDraft(w http.ResponseWriter,r *http.Request,p pageRow){
	if r.Method!=http.MethodPut{common.APIError(w,405,"METHOD","Use PUT");return}
	var in versionInput;if common.Decode(r,&in)!=nil{common.APIError(w,400,"JSON","Invalid request");return}
	in=normalizedInput(in)
	if err:=a.validateContent(r.Context(),p.ID,in,false);err!=nil{common.APIError(w,400,"VALIDATION",err.Error());return}
	old:=map[string]any{};if v,err:=a.getVersion(p.DraftVersionID);err==nil{old=decodeVersion(v)}
	tx,err:=a.db.BeginTx(r.Context(),&sql.TxOptions{});if err!=nil{common.APIError(w,500,"DB","Could not start CMS transaction");return};defer tx.Rollback()
	v,err:=a.insertVersion(r.Context(),tx,p.ID,"DRAFT",in,actor(r),p.DraftVersionID,"","")
	if err!=nil{common.APIError(w,500,"DB","Could not save CMS draft");return}
	if _,err=tx.ExecContext(r.Context(),`UPDATE cms.pages SET draft_version_id=$2,updated_at=NOW() WHERE id=$1`,p.ID,v.ID);err!=nil{common.APIError(w,500,"DB","Could not activate CMS draft");return}
	if err=a.auditTx(r.Context(),tx,p.ID,v.ID,"DRAFT_SAVED",actor(r),correlationID(r),old,decodeVersion(v));err!=nil{common.APIError(w,500,"DB","Could not audit CMS draft");return}
	if err=tx.Commit();err!=nil{common.APIError(w,500,"DB","Could not commit CMS draft");return}
	common.JSON(w,200,decodeVersion(v))
}

func (a *app)promotePreview(w http.ResponseWriter,r *http.Request,p pageRow){
	if r.Method!=http.MethodPost{common.APIError(w,405,"METHOD","Use POST");return}
	draft,err:=a.getVersion(p.DraftVersionID);if err!=nil{common.APIError(w,409,"DRAFT_REQUIRED","No draft is available");return}
	in,err:=inputFromVersion(draft);if err!=nil{common.APIError(w,500,"CMS_DATA","Could not decode draft");return}
	if err=a.validateContent(r.Context(),p.ID,in,false);err!=nil{common.APIError(w,400,"VALIDATION",err.Error());return}
	rawToken:=randomToken();if rawToken==""{common.APIError(w,500,"TOKEN","Could not generate preview token");return}
	tx,err:=a.db.BeginTx(r.Context(),&sql.TxOptions{});if err!=nil{common.APIError(w,500,"DB","Could not start CMS transaction");return};defer tx.Rollback()
	v,err:=a.insertVersion(r.Context(),tx,p.ID,"PREVIEW",in,actor(r),draft.ID,"","")
	if err!=nil{common.APIError(w,500,"DB","Could not create preview version");return}
	if _,err=tx.ExecContext(r.Context(),`UPDATE cms.pages SET preview_version_id=$2,preview_token_hash=$3,preview_token_issued_at=NOW(),updated_at=NOW() WHERE id=$1`,p.ID,v.ID,tokenHash(rawToken));err!=nil{common.APIError(w,500,"DB","Could not activate preview version");return}
	if err=a.auditTx(r.Context(),tx,p.ID,v.ID,"PREVIEW_CREATED",actor(r),correlationID(r),map[string]any{"draft_version_id":draft.ID},decodeVersion(v));err!=nil{common.APIError(w,500,"DB","Could not audit CMS preview");return}
	if err=tx.Commit();err!=nil{common.APIError(w,500,"DB","Could not commit CMS preview");return}
	common.JSON(w,201,map[string]any{"preview":decodeVersion(v),"preview_token":rawToken,"preview_path":"/preview/v1/cms/pages/"+url.PathEscape(v.Slug)+"?token="+url.QueryEscape(rawToken)})
}

func (a *app)rotatePreviewToken(w http.ResponseWriter,r *http.Request,p pageRow){
	if r.Method!=http.MethodPost{common.APIError(w,405,"METHOD","Use POST");return}
	if strings.TrimSpace(p.PreviewVersionID)==""{common.APIError(w,409,"PREVIEW_REQUIRED","Create a preview before rotating its token");return}
	rawToken:=randomToken();if rawToken==""{common.APIError(w,500,"TOKEN","Could not generate preview token");return}
	if _,err:=a.db.ExecContext(r.Context(),`UPDATE cms.pages SET preview_token_hash=$2,preview_token_issued_at=NOW(),updated_at=NOW() WHERE id=$1`,p.ID,tokenHash(rawToken));err!=nil{common.APIError(w,500,"DB","Could not rotate preview token");return}
	_ = a.audit(r.Context(),p.ID,p.PreviewVersionID,"PREVIEW_TOKEN_ROTATED",actor(r),correlationID(r),map[string]any{},map[string]any{"preview_version_id":p.PreviewVersionID})
	v,_:=a.getVersion(p.PreviewVersionID)
	common.JSON(w,200,map[string]any{"preview_token":rawToken,"preview_path":"/preview/v1/cms/pages/"+url.PathEscape(v.Slug)+"?token="+url.QueryEscape(rawToken)})
}

func (a *app)publishedConflict(ctx context.Context,pageID string,in versionInput)error{
	var id string
	err:=a.db.QueryRowContext(ctx,`SELECT p.id FROM cms.pages p JOIN cms.versions v ON v.id=p.published_version_id
		WHERE p.id<>$1 AND (lower(v.slug)=lower($2) OR lower(v.seo->>'canonical')=lower($3)) LIMIT 1`,pageID,in.Slug,in.SEO.Canonical).Scan(&id)
	if err==nil{return fmt.Errorf("published slug or canonical conflicts with another page")}
	if err==sql.ErrNoRows{return nil}
	return err
}

func mediaIDs(in versionInput)[]string{
	set:=map[string]bool{}
	if v:=strings.TrimSpace(in.SEO.OGImageAssetID);v!=""{set[v]=true}
	for _,s:=range in.Sections{if v:=strings.TrimSpace(s.MediaAssetID);v!=""{set[v]=true}}
	out:=make([]string,0,len(set));for id:=range set{out=append(out,id)};sort.Strings(out);return out
}

func (a *app)replacePublishedMedia(ctx context.Context,tx *sql.Tx,pageID,versionID string,in versionInput)error{
	if _,err:=tx.ExecContext(ctx,`DELETE FROM cms.published_media_refs WHERE page_id=$1`,pageID);err!=nil{return err}
	for _,id:=range mediaIDs(in){
		if _,err:=tx.ExecContext(ctx,`INSERT INTO cms.published_media_refs(page_id,version_id,media_id) VALUES($1,$2,$3)`,pageID,versionID,id);err!=nil{return err}
	}
	return nil
}

func (a *app)publish(w http.ResponseWriter,r *http.Request,p pageRow){
	if r.Method!=http.MethodPost{common.APIError(w,405,"METHOD","Use POST");return}
	preview,err:=a.getVersion(p.PreviewVersionID);if err!=nil{common.APIError(w,409,"PREVIEW_REQUIRED","A preview version is required before publishing");return}
	if preview.SourceVersionID!=p.DraftVersionID{common.APIError(w,409,"PREVIEW_STALE","The draft changed after preview. Create a fresh preview before publishing.");return}
	in,err:=inputFromVersion(preview);if err!=nil{common.APIError(w,500,"CMS_DATA","Could not decode preview");return}
	if err=a.validateContent(r.Context(),p.ID,in,true);err!=nil{common.APIError(w,400,"PUBLISH_VALIDATION",err.Error());return}
	if err=a.publishedConflict(r.Context(),p.ID,in);err!=nil{common.APIError(w,409,"SEO_CONFLICT",err.Error());return}
	old:=map[string]any{};if v,err:=a.getVersion(p.PublishedVersionID);err==nil{old=decodeVersion(v)}
	tx,err:=a.db.BeginTx(r.Context(),&sql.TxOptions{});if err!=nil{common.APIError(w,500,"DB","Could not start CMS transaction");return};defer tx.Rollback()
	v,err:=a.insertVersion(r.Context(),tx,p.ID,"PUBLISHED",in,actor(r),preview.ID,"",actor(r))
	if err!=nil{common.APIError(w,500,"DB","Could not create published version");return}
	if _,err=tx.ExecContext(r.Context(),`UPDATE cms.pages SET published_version_id=$2,updated_at=NOW() WHERE id=$1`,p.ID,v.ID);err!=nil{common.APIError(w,500,"DB","Could not activate published version");return}
	if err=a.replacePublishedMedia(r.Context(),tx,p.ID,v.ID,in);err!=nil{common.APIError(w,500,"DB","Could not publish CMS media references");return}
	if err=a.auditTx(r.Context(),tx,p.ID,v.ID,"PUBLISHED",actor(r),correlationID(r),old,decodeVersion(v));err!=nil{common.APIError(w,500,"DB","Could not audit publish");return}
	if err=tx.Commit();err!=nil{common.APIError(w,500,"DB","Could not commit publish");return}
	common.JSON(w,201,decodeVersion(v))
}

func (a *app)rollback(w http.ResponseWriter,r *http.Request,p pageRow){
	if r.Method!=http.MethodPost{common.APIError(w,405,"METHOD","Use POST");return}
	var in struct{VersionID string `json:"version_id"`}
	if common.Decode(r,&in)!=nil||strings.TrimSpace(in.VersionID)==""{common.APIError(w,400,"VALIDATION","version_id is required");return}
	target,err:=a.getVersion(strings.TrimSpace(in.VersionID))
	if err!=nil||target.PageID!=p.ID||target.State!="PUBLISHED"{common.APIError(w,409,"ROLLBACK_TARGET","Rollback target must be a previously published version of this page");return}
	payload,err:=inputFromVersion(target);if err!=nil{common.APIError(w,500,"CMS_DATA","Could not decode rollback target");return}
	if err=a.validateContent(r.Context(),p.ID,payload,true);err!=nil{common.APIError(w,400,"PUBLISH_VALIDATION",err.Error());return}
	if err=a.publishedConflict(r.Context(),p.ID,payload);err!=nil{common.APIError(w,409,"SEO_CONFLICT",err.Error());return}
	old:=map[string]any{};if v,err:=a.getVersion(p.PublishedVersionID);err==nil{old=decodeVersion(v)}
	tx,err:=a.db.BeginTx(r.Context(),&sql.TxOptions{});if err!=nil{common.APIError(w,500,"DB","Could not start rollback transaction");return};defer tx.Rollback()
	v,err:=a.insertVersion(r.Context(),tx,p.ID,"PUBLISHED",payload,actor(r),target.ID,target.ID,actor(r))
	if err!=nil{common.APIError(w,500,"DB","Could not create rollback version");return}
	if _,err=tx.ExecContext(r.Context(),`UPDATE cms.pages SET published_version_id=$2,updated_at=NOW() WHERE id=$1`,p.ID,v.ID);err!=nil{common.APIError(w,500,"DB","Could not activate rollback");return}
	if err=a.replacePublishedMedia(r.Context(),tx,p.ID,v.ID,payload);err!=nil{common.APIError(w,500,"DB","Could not restore published media references");return}
	if err=a.auditTx(r.Context(),tx,p.ID,v.ID,"ROLLBACK_PUBLISHED",actor(r),correlationID(r),old,map[string]any{"active_version":decodeVersion(v),"rollback_of_version_id":target.ID});err!=nil{common.APIError(w,500,"DB","Could not audit rollback");return}
	if err=tx.Commit();err!=nil{common.APIError(w,500,"DB","Could not commit rollback");return}
	common.JSON(w,201,map[string]any{"active_version":decodeVersion(v),"rollback_of_version_id":target.ID})
}

func (a *app)versions(w http.ResponseWriter,r *http.Request,p pageRow){
	if r.Method!=http.MethodGet{common.APIError(w,405,"METHOD","Use GET");return}
	rows,err:=a.db.Query(versionSelect()+" WHERE page_id=$1 ORDER BY version_no DESC",p.ID)
	if err!=nil{common.APIError(w,500,"DB","Could not load CMS versions");return};defer rows.Close()
	items:=[]map[string]any{};for rows.Next(){if v,err:=scanVersion(rows);err==nil{items=append(items,decodeVersion(v))}}
	common.JSON(w,200,map[string]any{"items":items,"count":len(items)})
}

func (a *app)auditLog(w http.ResponseWriter,r *http.Request,p pageRow){
	if r.Method!=http.MethodGet{common.APIError(w,405,"METHOD","Use GET");return}
	rows,err:=a.db.Query(`SELECT id,version_id,action,actor,correlation_id,old_state,new_state,created_at FROM cms.audit_events WHERE page_id=$1 ORDER BY created_at DESC LIMIT 200`,p.ID)
	if err!=nil{common.APIError(w,500,"DB","Could not load CMS audit");return};defer rows.Close()
	items:=[]map[string]any{}
	for rows.Next(){
		var id,versionID,action,actorID,corr string;var oldRaw,newRaw []byte;var created time.Time
		if rows.Scan(&id,&versionID,&action,&actorID,&corr,&oldRaw,&newRaw,&created)==nil{
			var oldState,newState any;_ = json.Unmarshal(oldRaw,&oldState);_ = json.Unmarshal(newRaw,&newState)
			items=append(items,map[string]any{"id":id,"version_id":versionID,"action":action,"actor":actorID,"correlation_id":corr,"old_state":oldState,"new_state":newState,"created_at":created})
		}
	}
	common.JSON(w,200,map[string]any{"items":items,"count":len(items)})
}

func publicVersion(v versionRow,visibleOnly bool)map[string]any{
	var seo any=map[string]any{}
	_ = json.Unmarshal(v.SEO,&seo)
	var sections []sectionInput
	_ = json.Unmarshal(v.Sections,&sections)
	if visibleOnly{
		filtered:=[]sectionInput{}
		for _,s:=range sections{if s.Visible{filtered=append(filtered,s)}}
		sections=filtered
	}
	var publishedAt any
	if v.PublishedAt.Valid{publishedAt=v.PublishedAt.Time.UTC()}
	return map[string]any{
		"content_model_version":1,
		"state":v.State,
		"version_no":v.VersionNo,
		"slug":v.Slug,
		"seo":seo,
		"sections":sections,
		"published_at":publishedAt,
	}
}

func (a *app)publicPage(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodGet&&r.Method!=http.MethodHead{common.APIError(w,405,"METHOD","Use GET or HEAD");return}
	slug:=strings.ToLower(strings.Trim(strings.TrimPrefix(r.URL.Path,"/public/v1/cms/pages/"),"/"))
	if slug==""{common.APIError(w,404,"NOT_FOUND","Published page not found");return}
	var versionID string
	err:=a.db.QueryRow(`SELECT p.published_version_id FROM cms.pages p JOIN cms.versions v ON v.id=p.published_version_id WHERE lower(v.slug)=lower($1)`,slug).Scan(&versionID)
	if err!=nil{common.APIError(w,404,"NOT_FOUND","Published page not found");return}
	v,err:=a.getVersion(versionID);if err!=nil||v.State!="PUBLISHED"{common.APIError(w,404,"NOT_FOUND","Published page not found");return}
	w.Header().Set("Cache-Control","public, max-age=60, stale-while-revalidate=300")
	common.JSON(w,200,publicVersion(v,true))
}

func (a *app)previewPage(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodGet&&r.Method!=http.MethodHead{common.APIError(w,405,"METHOD","Use GET or HEAD");return}
	slug:=strings.ToLower(strings.Trim(strings.TrimPrefix(r.URL.Path,"/preview/v1/cms/pages/"),"/"))
	rawToken:=strings.TrimSpace(r.URL.Query().Get("token"))
	var p pageRow
	err:=a.db.QueryRow(pageSelect()+` WHERE preview_token_hash=$2 AND preview_version_id IN (SELECT id FROM cms.versions WHERE lower(slug)=lower($1) AND state='PREVIEW')`,slug,tokenHash(rawToken)).
		Scan(&p.ID,&p.PageKey,&p.Name,&p.DraftVersionID,&p.PreviewVersionID,&p.PublishedVersionID,&p.PreviewTokenHash,&p.PreviewTokenIssuedAt,&p.CreatedAt,&p.UpdatedAt)
	if err!=nil||!tokenMatches(rawToken,p.PreviewTokenHash){common.APIError(w,404,"NOT_FOUND","Preview not found");return}
	v,err:=a.getVersion(p.PreviewVersionID);if err!=nil{common.APIError(w,404,"NOT_FOUND","Preview not found");return}
	w.Header().Set("Cache-Control","private, no-store")
	w.Header().Set("X-Robots-Tag","noindex, nofollow")
	common.JSON(w,200,publicVersion(v,true))
}

func safeFilename(name string)string{
	name=filepath.Base(strings.ReplaceAll(name,"\\","/"))
	var b strings.Builder
	for _,r:=range name{
		switch{case r>='a'&&r<='z',r>='A'&&r<='Z',r>='0'&&r<='9',r=='.',r=='-',r=='_':b.WriteRune(r);default:b.WriteByte('_')}
	}
	out:=strings.Trim(b.String(),"._");if out==""{out="cms-media.bin"};if len(out)>180{out=out[:180]};return out
}

func cmsMimeAllowed(v string)bool{
	v=strings.ToLower(strings.TrimSpace(strings.Split(v,";")[0]))
	return v=="image/png"||v=="image/jpeg"||v=="image/webp"
}

func (a *app)ensureStorage(ctx context.Context)error{
	if strings.TrimSpace(a.storageHost)==""{return fmt.Errorf("storage service is not configured")}
	req,err:=http.NewRequestWithContext(ctx,http.MethodPost,"http://"+a.storageHost+"/internal/v1/storage/partners/_cms/ensure",bytes.NewReader([]byte("{}")));if err!=nil{return err}
	req.Header.Set("X-Himate-Internal-Token",a.token);req.Header.Set("Content-Type","application/json")
	resp,err:=a.client.Do(req);if err!=nil{return err};defer resp.Body.Close()
	if resp.StatusCode<200||resp.StatusCode>=300{return fmt.Errorf("storage namespace status %d",resp.StatusCode)}
	return nil
}

func (a *app)putMedia(ctx context.Context,key string,data io.Reader,size int64)(map[string]any,error){
	if err:=a.ensureStorage(ctx);err!=nil{return nil,err}
	req,err:=http.NewRequestWithContext(ctx,http.MethodPut,"http://"+a.storageHost+"/internal/v1/storage/objects/_cms/"+key,data);if err!=nil{return nil,err}
	req.ContentLength=size;req.Header.Set("X-Himate-Internal-Token",a.token)
	resp,err:=a.client.Do(req);if err!=nil{return nil,err};defer resp.Body.Close()
	if resp.StatusCode<200||resp.StatusCode>=300{return nil,fmt.Errorf("storage put status %d",resp.StatusCode)}
	var out map[string]any;if err:=json.NewDecoder(resp.Body).Decode(&out);err!=nil{return nil,err};return out,nil
}

func (a *app)getMediaObject(ctx context.Context,m mediaRow)(*http.Response,error){
	path:="http://"+a.storageHost+"/internal/v1/storage/objects/"+url.PathEscape(m.ObjectNamespace)+"/"+m.ObjectKey+"?content_type="+url.QueryEscape(m.MimeType)
	req,err:=http.NewRequestWithContext(ctx,http.MethodGet,path,nil);if err!=nil{return nil,err}
	req.Header.Set("X-Himate-Internal-Token",a.token);return a.client.Do(req)
}

func mediaSelect()string{return `SELECT id,original_filename,mime_type,object_namespace,object_key,size_bytes,sha256,alt_text,created_by,created_at FROM cms.media_assets`}
func scanMedia(s scanner)(m mediaRow,err error){err=s.Scan(&m.ID,&m.OriginalFilename,&m.MimeType,&m.ObjectNamespace,&m.ObjectKey,&m.SizeBytes,&m.SHA256,&m.AltText,&m.CreatedBy,&m.CreatedAt);return}
func (a *app)getMedia(id string)(mediaRow,error){return scanMedia(a.db.QueryRow(mediaSelect()+" WHERE id=$1",id))}
func mapMedia(m mediaRow)map[string]any{return map[string]any{"id":m.ID,"original_filename":m.OriginalFilename,"mime_type":m.MimeType,"size_bytes":m.SizeBytes,"sha256":m.SHA256,"alt_text":m.AltText,"created_by":m.CreatedBy,"created_at":m.CreatedAt}}

func (a *app)mediaCollection(w http.ResponseWriter,r *http.Request){
	switch r.Method{
	case http.MethodGet:
		rows,err:=a.db.Query(mediaSelect()+" ORDER BY created_at DESC LIMIT 500");if err!=nil{common.APIError(w,500,"DB","Could not load CMS media");return};defer rows.Close()
		items:=[]map[string]any{};for rows.Next(){if m,err:=scanMedia(rows);err==nil{items=append(items,mapMedia(m))}}
		common.JSON(w,200,map[string]any{"items":items,"count":len(items)})
	case http.MethodPost:
		r.Body=http.MaxBytesReader(w,r.Body,maxMediaBytes+(1<<20))
		if err:=r.ParseMultipartForm(maxMediaBytes+(1<<20));err!=nil{common.APIError(w,413,"MEDIA_TOO_LARGE","CMS media request exceeds 11 MiB");return}
		if r.MultipartForm!=nil{defer r.MultipartForm.RemoveAll()}
		file,header,err:=r.FormFile("file");if err!=nil{common.APIError(w,400,"FILE_REQUIRED","file is required");return};defer file.Close()
		tmp,err:=os.CreateTemp("","himate-cms-*");if err!=nil{common.APIError(w,500,"FILE","Could not prepare media");return}
		tmpName:=tmp.Name();defer func(){tmp.Close();os.Remove(tmpName)}()
		h:=sha256.New();n,err:=io.Copy(io.MultiWriter(tmp,h),io.LimitReader(file,maxMediaBytes+1))
		if err!=nil{common.APIError(w,500,"FILE","Could not read media");return};if n>maxMediaBytes{common.APIError(w,413,"MEDIA_TOO_LARGE","CMS media exceeds 10 MiB");return}
		if _,err=tmp.Seek(0,0);err!=nil{common.APIError(w,500,"FILE","Could not inspect media");return}
		buf:=make([]byte,512);readN,_:=tmp.Read(buf);mime:=http.DetectContentType(buf[:readN])
		if !cmsMimeAllowed(mime){common.APIError(w,415,"MEDIA_TYPE","CMS media must be PNG, JPEG or WebP");return}
		if _,err=tmp.Seek(0,0);err!=nil{common.APIError(w,500,"FILE","Could not store media");return}
		id:=newID("cms_media_");filename:=safeFilename(header.Filename);ext:=".bin"
		switch mime{case"image/png":ext=".png";case"image/jpeg":ext=".jpg";case"image/webp":ext=".webp"}
		key:="media/"+id+ext;sum:=hex.EncodeToString(h.Sum(nil))
		stored,err:=a.putMedia(r.Context(),key,tmp,n);if err!=nil{common.APIError(w,502,"STORAGE",err.Error());return}
		if got:=strings.TrimSpace(fmt.Sprint(stored["sha256"]));got!=""&&got!=sum{common.APIError(w,502,"CHECKSUM_MISMATCH","Stored media checksum mismatch");return}
		alt:=strings.TrimSpace(r.FormValue("alt_text"));if len(alt)>240{common.APIError(w,400,"VALIDATION","alt_text is too long");return}
		_,err=a.db.ExecContext(r.Context(),`INSERT INTO cms.media_assets(id,original_filename,mime_type,object_namespace,object_key,size_bytes,sha256,alt_text,created_by)
			VALUES($1,$2,$3,'_cms',$4,$5,$6,$7,$8)`,id,filename,mime,key,n,sum,alt,actor(r))
		if err!=nil{common.APIError(w,500,"DB","Could not record CMS media");return}
		m,_:=a.getMedia(id);_ = a.audit(r.Context(),"","","MEDIA_UPLOADED",actor(r),correlationID(r),map[string]any{},mapMedia(m))
		common.JSON(w,201,mapMedia(m))
	default:common.APIError(w,405,"METHOD","Use GET or POST")
	}
}

func (a *app)serveMedia(w http.ResponseWriter,r *http.Request,m mediaRow,inline bool){
	resp,err:=a.getMediaObject(r.Context(),m);if err!=nil{common.APIError(w,502,"STORAGE","Could not load CMS media");return};defer resp.Body.Close()
	if resp.StatusCode==404{common.APIError(w,410,"BROKEN_MEDIA_REFERENCE","CMS media metadata exists but object is missing");return}
	if resp.StatusCode<200||resp.StatusCode>=300{common.APIError(w,502,"STORAGE","CMS media unavailable");return}
	w.Header().Set("Content-Type",m.MimeType)
	disposition:="attachment";if inline{disposition="inline"}
	w.Header().Set("Content-Disposition",disposition+`; filename="`+safeFilename(m.OriginalFilename)+`"`)
	_,_=io.Copy(w,resp.Body)
}

func (a *app)mediaItem(w http.ResponseWriter,r *http.Request){
	raw:=strings.Trim(strings.TrimPrefix(r.URL.Path,"/api/v1/cms/media/"),"/")
	parts:=strings.Split(raw,"/");if len(parts)!=2||parts[1]!="preview"{common.APIError(w,404,"NOT_FOUND","CMS media route not found");return}
	if r.Method!=http.MethodGet{common.APIError(w,405,"METHOD","Use GET");return}
	m,err:=a.getMedia(parts[0]);if err!=nil{common.APIError(w,404,"NOT_FOUND","CMS media not found");return}
	w.Header().Set("Cache-Control","private, no-store");a.serveMedia(w,r,m,true)
}

func (a *app)publicMedia(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodGet&&r.Method!=http.MethodHead{common.APIError(w,405,"METHOD","Use GET or HEAD");return}
	id:=strings.Trim(strings.TrimPrefix(r.URL.Path,"/public/v1/cms/media/"),"/")
	var exists bool;_ = a.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM cms.published_media_refs WHERE media_id=$1)`,id).Scan(&exists)
	if !exists{common.APIError(w,404,"NOT_FOUND","Published media not found");return}
	m,err:=a.getMedia(id);if err!=nil{common.APIError(w,404,"NOT_FOUND","Published media not found");return}
	w.Header().Set("Cache-Control","public, max-age=3600, immutable");a.serveMedia(w,r,m,true)
}

func versionReferencesMedia(v versionRow,mediaID string)bool{
	in,err:=inputFromVersion(v);if err!=nil{return false}
	for _,id:=range mediaIDs(in){if id==mediaID{return true}}
	return false
}

func (a *app)previewMedia(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodGet&&r.Method!=http.MethodHead{common.APIError(w,405,"METHOD","Use GET or HEAD");return}
	id:=strings.Trim(strings.TrimPrefix(r.URL.Path,"/preview/v1/cms/media/"),"/")
	pageID:=strings.TrimSpace(r.URL.Query().Get("page_id"));rawToken:=strings.TrimSpace(r.URL.Query().Get("token"))
	p,err:=a.getPage(pageID);if err!=nil||!tokenMatches(rawToken,p.PreviewTokenHash){common.APIError(w,404,"NOT_FOUND","Preview media not found");return}
	v,err:=a.getVersion(p.PreviewVersionID);if err!=nil||!versionReferencesMedia(v,id){common.APIError(w,404,"NOT_FOUND","Preview media not found");return}
	m,err:=a.getMedia(id);if err!=nil{common.APIError(w,404,"NOT_FOUND","Preview media not found");return}
	w.Header().Set("Cache-Control","private, no-store");w.Header().Set("X-Robots-Tag","noindex, nofollow");a.serveMedia(w,r,m,true)
}

func (a *app)publicManifest(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodGet{common.APIError(w,405,"METHOD","Use GET");return}
	rows,err:=a.db.Query(`SELECT p.id,p.page_key,p.name,v.id,v.page_id,v.version_no,v.state,v.slug,v.seo,v.sections,v.created_by,v.source_version_id,v.rollback_of_version_id,v.published_by,v.published_at,v.created_at
		FROM cms.pages p JOIN cms.versions v ON v.id=p.published_version_id ORDER BY v.slug`)
	if err!=nil{common.APIError(w,500,"DB","Could not load CMS manifest");return};defer rows.Close()
	items:=[]map[string]any{}
	for rows.Next(){
		var pID,key,name string;v,err:=scanVersionWithPrefix(rows,&pID,&key,&name);if err!=nil{continue}
		var seo seoInput;_ = json.Unmarshal(v.SEO,&seo)
		_ = pID; _ = key; _ = name
		items=append(items,map[string]any{"slug":v.Slug,"canonical":seo.Canonical,"title":seo.Title,"noindex":seo.NoIndex,"published_version":v.VersionNo,"published_at":timeValue(v.PublishedAt)})
	}
	w.Header().Set("Cache-Control","public, max-age=60");common.JSON(w,200,map[string]any{"items":items,"count":len(items)})
}

func scanVersionWithPrefix(s scanner,pID,key,name *string)(versionRow,error){
	var v versionRow
	err:=s.Scan(pID,key,name,&v.ID,&v.PageID,&v.VersionNo,&v.State,&v.Slug,&v.SEO,&v.Sections,&v.CreatedBy,&v.SourceVersionID,&v.RollbackOfVersionID,&v.PublishedBy,&v.PublishedAt,&v.CreatedAt)
	return v,err
}

func timeValue(v sql.NullTime)any{if !v.Valid{return nil};return v.Time.UTC()}

