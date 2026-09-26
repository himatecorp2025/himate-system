package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"himate.local/services/internal/common"
	"himate.local/services/internal/partnerdb"
)

type restoreJob struct {
	ID                   string
	RestorePointID       string
	PartnerID            string
	Status               string
	SafetyRestorePointID string
	DatabaseOK           bool
	MediaOK              bool
	ConfigOK             bool
	Reason               string
	CreatedBy            string
	Error                string
	CreatedAt            time.Time
	StartedAt            sql.NullTime
	CompletedAt          sql.NullTime
	DurationMS           int64
}

const restoreJobSelect = `SELECT id,restore_point_id,partner_id,status,safety_restore_point_id,database_ok,media_ok,config_ok,reason,created_by,error,created_at,started_at,completed_at,duration_ms FROM backups.restore_jobs`

func scanRestoreJob(s interface{ Scan(...any) error }) (restoreJob, error) {
	var job restoreJob
	err := s.Scan(&job.ID,&job.RestorePointID,&job.PartnerID,&job.Status,&job.SafetyRestorePointID,
		&job.DatabaseOK,&job.MediaOK,&job.ConfigOK,&job.Reason,&job.CreatedBy,&job.Error,
		&job.CreatedAt,&job.StartedAt,&job.CompletedAt,&job.DurationMS)
	return job, err
}

func mapRestoreJob(job restoreJob) map[string]any {
	var started, completed any
	if job.StartedAt.Valid { started = job.StartedAt.Time.UTC() }
	if job.CompletedAt.Valid { completed = job.CompletedAt.Time.UTC() }
	return map[string]any{
		"id":job.ID,"restore_point_id":job.RestorePointID,"partner_id":job.PartnerID,
		"status":job.Status,"safety_restore_point_id":job.SafetyRestorePointID,
		"database_ok":job.DatabaseOK,"media_ok":job.MediaOK,"config_ok":job.ConfigOK,
		"reason":job.Reason,"created_by":job.CreatedBy,"error":job.Error,
		"created_at":job.CreatedAt.UTC(),"started_at":started,"completed_at":completed,"duration_ms":job.DurationMS,
	}
}

func (a *app) verifiedRestorePoint(point restorePoint) error {
	if point.Status != "READY" { return fmt.Errorf("restore point must be READY") }
	var passed bool
	err := a.db.QueryRow(`SELECT EXISTS(
		SELECT 1 FROM backups.restore_tests
		WHERE restore_point_id=$1 AND partner_id=$2 AND status='PASSED'
	)`, point.ID, point.PartnerID).Scan(&passed)
	if err != nil { return err }
	if !passed { return fmt.Errorf("restore point must have a PASSED restore test before production restore") }
	return nil
}

func (a *app) partnerRestoreAllowed(ctx context.Context, partnerID string) error {
	if strings.TrimSpace(partnerID)=="_platform" {
		return fmt.Errorf("platform production restore is maintenance-only and cannot run from the live backup service")
	}
	var partner map[string]any
	if err := a.internalJSON(ctx,http.MethodGet,a.partnersHost,"/api/v1/partners/"+url.PathEscape(partnerID),nil,&partner,false); err != nil {
		return fmt.Errorf("load partner restore state: %w",err)
	}
	lifecycle := strings.ToUpper(strings.TrimSpace(fmt.Sprint(partner["lifecycle"])))
	testPartner, _ := partner["test_partner"].(bool)
	if !testPartner && lifecycle != "SUSPENDED" {
		return fmt.Errorf("production restore requires the partner lifecycle to be SUSPENDED")
	}
	return nil
}

func (a *app) queueProductionRestore(ctx context.Context, pointID, actor, reason, confirmation string) (restoreJob,error) {
	point, err := a.getRestorePoint(strings.TrimSpace(pointID))
	if err != nil { return restoreJob{},fmt.Errorf("restore point not found") }
	if err := a.verifiedRestorePoint(point); err != nil { return restoreJob{},err }
	if strings.TrimSpace(confirmation) != "RESTORE "+point.PartnerID {
		return restoreJob{},fmt.Errorf("confirmation must exactly match RESTORE %s",point.PartnerID)
	}
	if strings.TrimSpace(reason) == "" { return restoreJob{},fmt.Errorf("restore reason is required") }
	if err := a.partnerRestoreAllowed(ctx,point.PartnerID); err != nil { return restoreJob{},err }
	var pending bool
	if err := a.db.QueryRowContext(ctx,`SELECT EXISTS(
		SELECT 1 FROM backups.restore_jobs WHERE partner_id=$1 AND status IN('QUEUED','RUNNING')
	)`,point.PartnerID).Scan(&pending); err != nil { return restoreJob{},err }
	if pending { return restoreJob{},fmt.Errorf("restore already queued or running for partner") }
	id := newID("rjob")
	_,err = a.db.ExecContext(ctx,`INSERT INTO backups.restore_jobs(id,restore_point_id,partner_id,status,reason,created_by)
		VALUES($1,$2,$3,'QUEUED',$4,$5)`,id,point.ID,point.PartnerID,strings.TrimSpace(reason),strings.TrimSpace(actor))
	if err != nil { return restoreJob{},err }
	job,err := scanRestoreJob(a.db.QueryRowContext(ctx,restoreJobSelect+` WHERE id=$1`,id))
	if err == nil { a.signal() }
	return job,err
}

func (a *app) claimRestoreJob(ctx context.Context) (restoreJob,bool) {
	tx,err := a.db.BeginTx(ctx,&sql.TxOptions{})
	if err != nil { return restoreJob{},false }
	defer tx.Rollback()
	job,err := scanRestoreJob(tx.QueryRowContext(ctx,restoreJobSelect+` WHERE status='QUEUED' ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1`))
	if err != nil { return restoreJob{},false }
	if _,err=tx.ExecContext(ctx,`UPDATE backups.restore_jobs SET status='RUNNING',error='',started_at=NOW() WHERE id=$1`,job.ID);err!=nil{return restoreJob{},false}
	if tx.Commit()!=nil{return restoreJob{},false}
	job.Status="RUNNING";job.StartedAt=sql.NullTime{Time:time.Now().UTC(),Valid:true}
	return job,true
}

func (a *app) createSafetyRestorePoint(ctx context.Context, partnerID, actor string) (restorePoint,error) {
	var pending bool
	if err:=a.db.QueryRowContext(ctx,`SELECT EXISTS(SELECT 1 FROM backups.restore_points WHERE partner_id=$1 AND status IN('QUEUED','RUNNING'))`,partnerID).Scan(&pending);err!=nil{return restorePoint{},err}
	if pending{return restorePoint{},fmt.Errorf("cannot create pre-restore safety point while another backup is running")}
	p,err:=a.ensurePolicy(partnerID);if err!=nil{return restorePoint{},err}
	id:=newID("bkp")
	expires:=time.Now().UTC().Add(time.Duration(p.RetentionDays)*24*time.Hour)
	_,err=a.db.ExecContext(ctx,`INSERT INTO backups.restore_points(id,partner_id,status,provider,created_by,expires_at)
		VALUES($1,$2,'RUNNING',$3,$4,$5)`,id,partnerID,a.provider.Name(),"pre-restore:"+strings.TrimSpace(actor),expires)
	if err!=nil{return restorePoint{},err}
	point,err:=a.getRestorePoint(id);if err!=nil{return restorePoint{},err}
	if err:=a.createRestorePoint(ctx,point);err!=nil{
		_,_=a.db.ExecContext(ctx,`UPDATE backups.restore_points SET status='FAILED',error=$2,completed_at=NOW() WHERE id=$1`,id,safeError(err))
		return restorePoint{},err
	}
	ready,err:=a.getRestorePoint(id)
	if err==nil{_,_=a.queueRestoreTestRecord(ready,"automatic-pre-restore")}
	return ready,err
}

func (a *app) unpackRestorePoint(ctx context.Context, point restorePoint, prefix string) (string,error) {
	work,err:=os.MkdirTemp(a.workRoot,prefix+"-")
	if err!=nil{return "",err}
	fail:=func(e error)(string,error){_ = os.RemoveAll(work);return "",e}
	encrypted:=filepath.Join(work,"offsite.hmbk")
	reader,err:=a.provider.Open(ctx,point.ObjectKey);if err!=nil{return fail(fmt.Errorf("open offsite restore point: %w",err))}
	out,err:=os.OpenFile(encrypted,os.O_CREATE|os.O_TRUNC|os.O_WRONLY,0600)
	if err!=nil{reader.Close();return fail(err)}
	_,copyErr:=io.Copy(out,reader);reader.Close();closeErr:=out.Close()
	if copyErr!=nil{return fail(copyErr)};if closeErr!=nil{return fail(closeErr)}
	cipherSHA,_,err:=shaFile(encrypted);if err!=nil{return fail(err)}
	if !strings.EqualFold(cipherSHA,point.CiphertextSHA256){return fail(fmt.Errorf("offsite ciphertext checksum mismatch"))}
	archive:=filepath.Join(work,"restore.tar.gz")
	encReader,err:=os.Open(encrypted);if err!=nil{return fail(err)}
	err=decryptStreamToFile(encReader,archive,a.key);encReader.Close();if err!=nil{return fail(err)}
	extracted:=filepath.Join(work,"components")
	if err:=os.MkdirAll(extracted,0700);err!=nil{return fail(err)}
	if err:=extractTarGz(archive,extracted);err!=nil{return fail(fmt.Errorf("extract restore point: %w",err))}
	manifestRaw,err:=os.ReadFile(filepath.Join(extracted,"manifest.json"));if err!=nil{return fail(err)}
	var manifest map[string]any;if err:=json.Unmarshal(manifestRaw,&manifest);err!=nil{return fail(err)}
	if fmt.Sprint(manifest["partner_id"])!=point.PartnerID{return fail(fmt.Errorf("restore manifest partner mismatch"))}
	for _,component:=range []string{"database","media","config"}{
		want,err:=componentSHA(manifest,component);if err!=nil{return fail(err)}
		name:=map[string]string{"database":"database.dump","media":"media.tar.gz","config":"config.json"}[component]
		got,_,err:=shaFile(filepath.Join(extracted,name));if err!=nil{return fail(err)}
		if !strings.EqualFold(got,want){return fail(fmt.Errorf("%s component checksum mismatch",component))}
	}
	return work,nil
}

func shortDBName(base,suffix string) string {
	value:=base+"_"+suffix
	if len(value)<=63{return value}
	max:=63-len(suffix)-1
	if max<1{return suffix}
	return base[:max]+"_"+suffix
}

func (a *app) restoreLiveDatabase(ctx context.Context,dumpPath,partnerID,jobID string) error {
	suffix:=strings.ReplaceAll(strings.TrimPrefix(jobID,"rjob_"),"-","")
	if len(suffix)>10{suffix=suffix[:10]}
	target:=partnerdb.DatabaseName(partnerID);if target==""{return fmt.Errorf("invalid partner id")}
	scratch:=shortDBName("himate_restore_live",suffix)
	if err:=a.createScratchDatabase(ctx,scratch);err!=nil{return fmt.Errorf("create restore database: %w",err)}
	cleanupScratch:=true
	defer func(){if cleanupScratch{a.dropScratchDatabase(context.Background(),scratch)}}()
	env,err:=a.pgEnvironment(scratch);if err!=nil{return err}
	if err:=runCommand(ctx,env,"pg_restore","--no-owner","--no-privileges","--exit-on-error","--dbname",scratch,dumpPath);err!=nil{return err}
	u,err:=url.Parse(a.dbAdminURL);if err!=nil{return err};u.Path="/"+scratch
	verifyDB,err:=sql.Open("pgx",u.String());if err!=nil{return err}
	var restoredPartner string
	verifyErr:=verifyDB.QueryRowContext(ctx,`SELECT value FROM partner_core.system_meta WHERE key='partner_id'`).Scan(&restoredPartner)
	_ = verifyDB.Close()
	if verifyErr!=nil{return fmt.Errorf("verify restored partner identity: %w",verifyErr)}
	if restoredPartner!=partnerID{return fmt.Errorf("restored partner identity mismatch")}

	adminDSN,err:=partnerdb.AdminDSN(a.dbAdminURL);if err!=nil{return err}
	admin,err:=sql.Open("pgx",adminDSN);if err!=nil{return err};defer admin.Close()
	old:=shortDBName(target,"pre_"+suffix)
	_,_=admin.ExecContext(ctx,`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname=$1 AND pid<>pg_backend_pid()`,old)
	_,_=admin.ExecContext(ctx,"DROP DATABASE IF EXISTS "+quoteIdent(old))
	var targetExists bool
	if err:=admin.QueryRowContext(ctx,`SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname=$1)`,target).Scan(&targetExists);err!=nil{return err}
	if targetExists{
		_,_=admin.ExecContext(ctx,`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname=$1 AND pid<>pg_backend_pid()`,target)
		if _,err:=admin.ExecContext(ctx,"ALTER DATABASE "+quoteIdent(target)+" RENAME TO "+quoteIdent(old));err!=nil{return fmt.Errorf("stage current partner database: %w",err)}
	}
	if _,err:=admin.ExecContext(ctx,"ALTER DATABASE "+quoteIdent(scratch)+" RENAME TO "+quoteIdent(target));err!=nil{
		if targetExists{_,_=admin.ExecContext(context.Background(),"ALTER DATABASE "+quoteIdent(old)+" RENAME TO "+quoteIdent(target))}
		return fmt.Errorf("activate restored partner database: %w",err)
	}
	cleanupScratch=false
	if targetExists{
		_,_=admin.ExecContext(ctx,`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname=$1 AND pid<>pg_backend_pid()`,old)
		if _,err:=admin.ExecContext(ctx,"DROP DATABASE "+quoteIdent(old));err!=nil{return fmt.Errorf("retire previous partner database: %w",err)}
	}
	return nil
}

func (a *app) restoreMedia(ctx context.Context, partnerID, archivePath string) error {
	file,err:=os.Open(archivePath);if err!=nil{return err};defer file.Close()
	req,err:=http.NewRequestWithContext(ctx,http.MethodPost,
		"http://"+a.storageHost+"/internal/v1/storage/partners/"+url.PathEscape(partnerID)+"/restore-archive",file)
	if err!=nil{return err}
	common.BindInternalRequest(req,a.internalToken)
	req.Header.Set("X-Himate-User-ID","service:backups")
	req.Header.Set("Content-Type","application/gzip")
	resp,err:=common.DoInternal(a.client,req);if err!=nil{return err}
	defer resp.Body.Close()
	if resp.StatusCode<200||resp.StatusCode>=300{return fmt.Errorf("storage restore returned status %d",resp.StatusCode)}
	return nil
}

func mapSubset(source map[string]any,keys ...string) map[string]any {
	out:=map[string]any{}
	for _,key:=range keys{if value,ok:=source[key];ok{out[key]=value}}
	return out
}

func (a *app) suspendPartnerEnvironments(ctx context.Context,partnerID string) error {
	var response map[string]any
	if err:=a.internalJSON(ctx,http.MethodGet,a.envHost,"/api/v1/environments?partner_id="+url.QueryEscape(partnerID),nil,&response,false);err!=nil{
		return fmt.Errorf("load partner environments before recovery: %w",err)
	}
	items,ok:=response["items"].([]any)
	if !ok{return nil}
	for _,value:=range items{
		env,ok:=value.(map[string]any);if !ok{continue}
		id:=strings.TrimSpace(fmt.Sprint(env["id"]));if id==""{continue}
		if err:=a.internalJSON(ctx,http.MethodPatch,a.envHost,"/api/v1/environments/"+url.PathEscape(id),
			map[string]any{"environment_status":"SUSPENDED"},nil,false);err!=nil{
			return fmt.Errorf("suspend environment %s for recovery: %w",id,err)
		}
	}
	return nil
}

func (a *app) restoreEnvironmentRelease(ctx context.Context,id,release string) error {
	release=strings.TrimSpace(release)
	if release=="" {
		return a.internalJSON(ctx,http.MethodPatch,a.envHost,"/api/v1/environments/"+url.PathEscape(id),
			map[string]any{"environment_status":"SUSPENDED"},nil,false)
	}
	var deployed map[string]any
	if err:=a.internalJSON(ctx,http.MethodPost,a.envHost,"/api/v1/environments/"+url.PathEscape(id)+"/deploy",
		map[string]any{"release":release},&deployed,false);err!=nil{
		return fmt.Errorf("deploy captured release %s: %w",release,err)
	}
	for attempt:=0;attempt<150;attempt++{
		state:=deployed
		if attempt>0{
			state=map[string]any{}
			if err:=a.internalJSON(ctx,http.MethodGet,a.envHost,"/api/v1/environments/"+url.PathEscape(id),nil,&state,false);err!=nil{
				return fmt.Errorf("poll restored environment %s: %w",id,err)
			}
		}
		deployment:=strings.ToUpper(strings.TrimSpace(fmt.Sprint(state["deployment_status"])))
		active:=strings.TrimSpace(fmt.Sprint(state["active_release"]))
		if deployment=="FAILED"{return fmt.Errorf("captured release deployment failed for environment %s",id)}
		if deployment=="DEPLOYED"&&active==release{
			return a.internalJSON(ctx,http.MethodPatch,a.envHost,"/api/v1/environments/"+url.PathEscape(id),
				map[string]any{"environment_status":"SUSPENDED"},nil,false)
		}
		select{
		case <-ctx.Done():return ctx.Err()
		case <-time.After(2*time.Second):
		}
	}
	return fmt.Errorf("captured release deployment timed out for environment %s",id)
}

func (a *app) restoreConfig(ctx context.Context,partnerID,configPath,actor,reason string) error {
	raw,err:=os.ReadFile(configPath);if err!=nil{return err}
	var config map[string]any;if err:=json.Unmarshal(raw,&config);err!=nil{return err}
	if fmt.Sprint(config["partner_id"])!=partnerID{return fmt.Errorf("restored configuration partner mismatch")}
	if partner,ok:=config["partner"].(map[string]any);ok{
		payload:=mapSubset(partner,
			"display_name","legal_name","brand_name","category_id","primary_domain","staging_domain",
			"contact_name","contact_email","finance_contact_name","finance_contact_email",
			"technical_contact_name","technical_contact_email","marketing_contact_name","marketing_contact_email",
			"registration_number","tax_id","country","state_region","city","postal_code",
			"address_line1","address_line2","website","phone","logo_url","notes")
		payload["reason"]="Restore from verified HIMATE restore point: "+strings.TrimSpace(reason)
		if err:=a.internalJSON(ctx,http.MethodPatch,a.partnersHost,"/api/v1/partners/"+url.PathEscape(partnerID),payload,nil,false);err!=nil{return fmt.Errorf("restore partner metadata: %w",err)}
	}
	if environments,ok:=config["environments"].([]any);ok{
		for _,value:=range environments{
			env,ok:=value.(map[string]any);if !ok{continue}
			id:=strings.TrimSpace(fmt.Sprint(env["id"]));if id==""{continue}
			payload:=mapSubset(env,"platform_version","desired_release","config")
			if len(payload)>0{
				if err:=a.internalJSON(ctx,http.MethodPatch,a.envHost,"/api/v1/environments/"+url.PathEscape(id),payload,nil,false);err!=nil{return fmt.Errorf("restore environment %s: %w",id,err)}
			}
			release:=strings.TrimSpace(fmt.Sprint(env["active_release"]))
			if release==""{release=strings.TrimSpace(fmt.Sprint(env["desired_release"]))}
			if err:=a.restoreEnvironmentRelease(ctx,id,release);err!=nil{return fmt.Errorf("restore environment release %s: %w",id,err)}
		}
	}
	if desired,ok:=config["connector_desired_state"].(map[string]any);ok{
		for environment,value:=range desired{
			state,ok:=value.(map[string]any);if !ok{continue}
			payload:=mapSubset(state,"entitlements","maintenance","config")
			payload["environment"]=environment
			if err:=a.internalJSON(ctx,http.MethodPut,a.connectorHost,"/api/v1/connectors/"+url.PathEscape(partnerID)+"/desired-state",payload,nil,false);err!=nil{return fmt.Errorf("restore connector desired state %s: %w",environment,err)}
		}
	}
	_ = actor
	return nil
}

func (a *app) processProductionRestore(ctx context.Context,job restoreJob) {
	started:=time.Now()
	fail:=func(err error){
		_,_=a.db.Exec(`UPDATE backups.restore_jobs SET status='FAILED',error=$2,completed_at=NOW(),duration_ms=$3 WHERE id=$1`,job.ID,safeError(err),time.Since(started).Milliseconds())
	}
	if err:=a.partnerRestoreAllowed(ctx,job.PartnerID);err!=nil{fail(err);return}
	point,err:=a.getRestorePoint(job.RestorePointID);if err!=nil{fail(err);return}
	if err:=a.verifiedRestorePoint(point);err!=nil{fail(err);return}
	safety,err:=a.createSafetyRestorePoint(ctx,job.PartnerID,job.CreatedBy);if err!=nil{fail(fmt.Errorf("pre-restore safety backup: %w",err));return}
	_,_=a.db.Exec(`UPDATE backups.restore_jobs SET safety_restore_point_id=$2 WHERE id=$1`,job.ID,safety.ID)
	if err:=a.suspendPartnerEnvironments(ctx,job.PartnerID);err!=nil{fail(err);return}

	work,err:=a.unpackRestorePoint(ctx,point,"production-"+job.ID)
	if err!=nil{fail(err);return}
	defer os.RemoveAll(work)
	extracted:=filepath.Join(work,"components")
	if err:=a.restoreLiveDatabase(ctx,filepath.Join(extracted,"database.dump"),job.PartnerID,job.ID);err!=nil{fail(fmt.Errorf("database restore: %w",err));return}
	_,_=a.db.Exec(`UPDATE backups.restore_jobs SET database_ok=TRUE WHERE id=$1`,job.ID)
	if err:=a.restoreMedia(ctx,job.PartnerID,filepath.Join(extracted,"media.tar.gz"));err!=nil{fail(fmt.Errorf("media restore: %w",err));return}
	_,_=a.db.Exec(`UPDATE backups.restore_jobs SET media_ok=TRUE WHERE id=$1`,job.ID)
	if err:=a.restoreConfig(ctx,job.PartnerID,filepath.Join(extracted,"config.json"),job.CreatedBy,job.Reason);err!=nil{fail(fmt.Errorf("configuration restore: %w",err));return}
	_,_=a.db.Exec(`UPDATE backups.restore_jobs SET config_ok=TRUE,status='COMPLETED',error='',completed_at=NOW(),duration_ms=$2 WHERE id=$1`,job.ID,time.Since(started).Milliseconds())
}

func (a *app) productionRestoreRoute(w http.ResponseWriter,r *http.Request,pointID string){
	if r.Method!=http.MethodPost{common.APIError(w,405,"METHOD","Use POST");return}
	var in struct{Confirmation string `json:"confirmation"`;Reason string `json:"reason"`}
	if common.Decode(r,&in)!=nil{common.APIError(w,400,"JSON","Invalid request");return}
	job,err:=a.queueProductionRestore(r.Context(),pointID,r.Header.Get("X-Himate-User-ID"),in.Reason,in.Confirmation)
	if err!=nil{common.APIError(w,409,"RESTORE_QUEUE",err.Error());return}
	common.JSON(w,http.StatusAccepted,mapRestoreJob(job))
}

func (a *app) restoreJobsRoute(w http.ResponseWriter,r *http.Request,jobID string){
	if r.Method!=http.MethodGet{common.APIError(w,405,"METHOD","Use GET");return}
	if strings.TrimSpace(jobID)!=""{
		job,err:=scanRestoreJob(a.db.QueryRow(restoreJobSelect+` WHERE id=$1`,jobID))
		if err!=nil{common.APIError(w,404,"NOT_FOUND","Restore job not found");return}
		common.JSON(w,200,mapRestoreJob(job));return
	}
	partnerID:=strings.TrimSpace(r.URL.Query().Get("partner_id"))
	where:="1=1";args:=[]any{}
	if partnerID!=""{args=append(args,partnerID);where="partner_id=$1"}
	rows,err:=a.db.Query(restoreJobSelect+" WHERE "+where+" ORDER BY created_at DESC LIMIT 100",args...)
	if err!=nil{common.APIError(w,500,"DB","Could not load restore jobs");return}
	defer rows.Close()
	items:=[]map[string]any{}
	for rows.Next(){if job,scanErr:=scanRestoreJob(rows);scanErr==nil{items=append(items,mapRestoreJob(job))}}
	common.JSON(w,200,map[string]any{"items":items,"count":len(items)})
}
