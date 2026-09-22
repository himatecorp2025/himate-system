package main

import (
	"database/sql"
	"fmt"
	"himate.local/services/internal/common"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (a *app) backups(w http.ResponseWriter,r *http.Request){
	switch r.Method{
	case http.MethodGet:
		partnerID:=strings.TrimSpace(r.URL.Query().Get("partner_id"))
		status:=strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("status")))
		limit,_:=strconv.Atoi(r.URL.Query().Get("limit"));if limit<1{limit=50};if limit>200{limit=200}
		where:=[]string{"1=1"};args:=[]any{}
		if partnerID!=""{args=append(args,partnerID);where=append(where,fmt.Sprintf("partner_id=$%d",len(args)))}
		if status!=""{args=append(args,status);where=append(where,fmt.Sprintf("status=$%d",len(args)))}
		args=append(args,limit)
		rows,err:=a.db.Query(restorePointSelect+" WHERE "+strings.Join(where," AND ")+" ORDER BY created_at DESC LIMIT $"+strconv.Itoa(len(args)),args...)
		if err!=nil{common.APIError(w,500,"DB","Could not load restore points");return}
		defer rows.Close()
		items:=[]map[string]any{}
		for rows.Next(){if point,scanErr:=scanRestorePoint(rows);scanErr==nil{items=append(items,mapRestorePoint(point))}}
		common.JSON(w,200,map[string]any{"items":items,"count":len(items)})
	case http.MethodPost:
		var in struct{PartnerID string `json:"partner_id"`}
		if common.Decode(r,&in)!=nil||strings.TrimSpace(in.PartnerID)==""{common.APIError(w,400,"VALIDATION","partner_id is required");return}
		point,err:=a.queueBackup(strings.TrimSpace(in.PartnerID),r.Header.Get("X-Himate-User-ID"))
		if err!=nil{common.APIError(w,409,"BACKUP_QUEUE",err.Error());return}
		common.JSON(w,http.StatusAccepted,mapRestorePoint(point))
	default:
		common.APIError(w,405,"METHOD","Use GET or POST")
	}
}

func (a *app) backupRoute(w http.ResponseWriter,r *http.Request){
	raw:=strings.Trim(strings.TrimPrefix(r.URL.Path,"/api/v1/backups/"),"/")
	if raw==""{common.APIError(w,404,"NOT_FOUND","Backup route not found");return}
	parts:=strings.Split(raw,"/")
	switch{
	case len(parts)==2&&parts[0]=="policies":
		a.policyRoute(w,r,parts[1])
	case len(parts)==2&&parts[0]=="restore-points":
		if r.Method!=http.MethodGet{common.APIError(w,405,"METHOD","Use GET");return}
		point,err:=a.getRestorePoint(parts[1]);if err!=nil{common.APIError(w,404,"NOT_FOUND","Restore point not found");return}
		common.JSON(w,200,mapRestorePoint(point))
	case len(parts)==3&&parts[0]=="restore-points"&&parts[2]=="restore-test":
		a.queueRestoreTest(w,r,parts[1])
	case len(parts)==2&&parts[0]=="restore-tests":
		if r.Method!=http.MethodGet{common.APIError(w,405,"METHOD","Use GET");return}
		test,err:=scanRestoreTest(a.db.QueryRow(restoreTestSelect+` WHERE id=$1`,parts[1]))
		if err!=nil{common.APIError(w,404,"NOT_FOUND","Restore test not found");return}
		common.JSON(w,200,mapRestoreTest(test))
	case len(parts)==1&&parts[0]=="restore-tests":
		a.restoreTests(w,r)
	case len(parts)==1&&parts[0]=="summary":
		a.summary(w,r)
	case len(parts)==2&&parts[0]=="scheduler"&&parts[1]=="run":
		a.schedulerRun(w,r)
	case len(parts)==1&&parts[0]=="prune":
		a.pruneRoute(w,r)
	default:
		common.APIError(w,404,"NOT_FOUND","Backup route not found")
	}
}

func (a *app) policyRoute(w http.ResponseWriter,r *http.Request,partnerID string){
	switch r.Method{
	case http.MethodGet:
		p,err:=a.ensurePolicy(partnerID);if err!=nil{common.APIError(w,500,"DB","Could not load backup policy");return}
		var last any
		if p.LastScheduledAt.Valid{last=p.LastScheduledAt.Time.UTC()}
		common.JSON(w,200,map[string]any{"partner_id":p.PartnerID,"retention_days":p.RetentionDays,"max_restore_points":p.MaxRestorePoints,"schedule_hours":p.ScheduleHours,"enabled":p.Enabled,"last_scheduled_at":last,"updated_at":p.UpdatedAt.UTC()})
	case http.MethodPut:
		var in struct{
			RetentionDays int `json:"retention_days"`
			MaxRestorePoints int `json:"max_restore_points"`
			ScheduleHours int `json:"schedule_hours"`
			Enabled *bool `json:"enabled"`
		}
		if common.Decode(r,&in)!=nil{common.APIError(w,400,"JSON","Invalid request");return}
		if in.RetentionDays<1||in.RetentionDays>3650||in.MaxRestorePoints<1||in.MaxRestorePoints>365||in.ScheduleHours<1||in.ScheduleHours>720{
			common.APIError(w,400,"VALIDATION","retention_days 1-3650, max_restore_points 1-365 and schedule_hours 1-720 are required");return
		}
		enabled:=true;if in.Enabled!=nil{enabled=*in.Enabled}
		_,err:=a.db.Exec(`INSERT INTO backups.policies(partner_id,retention_days,max_restore_points,schedule_hours,enabled)
			VALUES($1,$2,$3,$4,$5)
			ON CONFLICT(partner_id) DO UPDATE SET retention_days=EXCLUDED.retention_days,
				max_restore_points=EXCLUDED.max_restore_points,schedule_hours=EXCLUDED.schedule_hours,
				enabled=EXCLUDED.enabled,updated_at=NOW()`,
			partnerID,in.RetentionDays,in.MaxRestorePoints,in.ScheduleHours,enabled)
		if err!=nil{common.APIError(w,500,"DB","Could not update backup policy");return}
		p,_:=a.ensurePolicy(partnerID);a.signal()
		var last any
		if p.LastScheduledAt.Valid{last=p.LastScheduledAt.Time.UTC()}
		common.JSON(w,200,map[string]any{"partner_id":p.PartnerID,"retention_days":p.RetentionDays,"max_restore_points":p.MaxRestorePoints,"schedule_hours":p.ScheduleHours,"enabled":p.Enabled,"last_scheduled_at":last,"updated_at":p.UpdatedAt.UTC()})
	default:
		common.APIError(w,405,"METHOD","Use GET or PUT")
	}
}

func (a *app) queueRestoreTest(w http.ResponseWriter,r *http.Request,restorePointID string){
	if r.Method!=http.MethodPost{common.APIError(w,405,"METHOD","Use POST");return}
	point,err:=a.getRestorePoint(restorePointID)
	if err!=nil{common.APIError(w,404,"NOT_FOUND","Restore point not found");return}
	test,err:=a.queueRestoreTestRecord(point,strings.TrimSpace(r.Header.Get("X-Himate-User-ID")))
	if err!=nil{common.APIError(w,409,"RESTORE_TEST_QUEUE",err.Error());return}
	common.JSON(w,http.StatusAccepted,mapRestoreTest(test))
}

func (a *app) restoreTests(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodGet{common.APIError(w,405,"METHOD","Use GET");return}
	partnerID:=strings.TrimSpace(r.URL.Query().Get("partner_id"))
	pointID:=strings.TrimSpace(r.URL.Query().Get("restore_point_id"))
	limit,_:=strconv.Atoi(r.URL.Query().Get("limit"));if limit<1{limit=50};if limit>200{limit=200}
	where:=[]string{"1=1"};args:=[]any{}
	if partnerID!=""{args=append(args,partnerID);where=append(where,fmt.Sprintf("partner_id=$%d",len(args)))}
	if pointID!=""{args=append(args,pointID);where=append(where,fmt.Sprintf("restore_point_id=$%d",len(args)))}
	args=append(args,limit)
	rows,err:=a.db.Query(restoreTestSelect+" WHERE "+strings.Join(where," AND ")+" ORDER BY created_at DESC LIMIT $"+strconv.Itoa(len(args)),args...)
	if err!=nil{common.APIError(w,500,"DB","Could not load restore tests");return}
	defer rows.Close()
	items:=[]map[string]any{}
	for rows.Next(){if test,scanErr:=scanRestoreTest(rows);scanErr==nil{items=append(items,mapRestoreTest(test))}}
	common.JSON(w,200,map[string]any{"items":items,"count":len(items)})
}

func (a *app) schedulerRun(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodPost{common.APIError(w,405,"METHOD","Use POST");return}
	queued:=a.runSchedulerOnce()
	common.JSON(w,200,map[string]any{"status":"ok","queued":queued,"ran_at":time.Now().UTC()})
}

func (a *app) pruneRoute(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodPost{common.APIError(w,405,"METHOD","Use POST");return}
	partnerID:=strings.TrimSpace(r.URL.Query().Get("partner_id"))
	if partnerID==""{common.APIError(w,400,"VALIDATION","partner_id is required");return}
	count,err:=a.prunePartner(r.Context(),partnerID)
	if err!=nil{common.APIError(w,502,"OFFSITE_DELETE",err.Error());return}
	common.JSON(w,200,map[string]any{"partner_id":partnerID,"expired":count})
}

func (a *app) summary(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodGet{common.APIError(w,405,"METHOD","Use GET");return}
	rows,err:=a.db.Query(`SELECT p.partner_id,p.retention_days,p.max_restore_points,p.schedule_hours,p.enabled,
		COALESCE(r.id,''),COALESCE(r.status,'NEVER'),r.completed_at,
		COALESCE(t.restore_point_id,''),COALESCE(t.status,'NEVER'),t.completed_at
		FROM backups.policies p
		LEFT JOIN LATERAL(
			SELECT id,status,completed_at FROM backups.restore_points WHERE partner_id=p.partner_id ORDER BY created_at DESC LIMIT 1
		) r ON TRUE
		LEFT JOIN LATERAL(
			SELECT restore_point_id,status,completed_at FROM backups.restore_tests WHERE partner_id=p.partner_id ORDER BY created_at DESC LIMIT 1
		) t ON TRUE
		ORDER BY p.partner_id`)
	if err!=nil{common.APIError(w,500,"DB","Could not load backup summary");return}
	defer rows.Close()
	items:=[]map[string]any{}
	for rows.Next(){
		var partnerID,pointID,pointStatus,testPointID,testStatus string
		var retention,maxPoints,schedule int;var enabled bool
		var pointCompleted,testCompleted sql.NullTime
		if rows.Scan(&partnerID,&retention,&maxPoints,&schedule,&enabled,&pointID,&pointStatus,&pointCompleted,&testPointID,&testStatus,&testCompleted)==nil{
			var pc,tc any;if pointCompleted.Valid{pc=pointCompleted.Time.UTC()};if testCompleted.Valid{tc=testCompleted.Time.UTC()}
			recoverability:="UNVERIFIED"
			if pointStatus=="READY"&&testPointID==pointID&&testStatus=="PASSED"{recoverability="VERIFIED"}
			if pointStatus=="FAILED"||testStatus=="FAILED"{recoverability="FAILED"}
			items=append(items,map[string]any{
				"partner_id":partnerID,"retention_days":retention,"max_restore_points":maxPoints,"schedule_hours":schedule,"enabled":enabled,
				"latest_restore_point_id":pointID,"latest_backup_status":pointStatus,"latest_backup_at":pc,
				"latest_restore_test_status":testStatus,"latest_restore_test_at":tc,
				"recoverability_status":recoverability,"provider":a.provider.Name(),
			})
		}
	}
	common.JSON(w,200,map[string]any{"items":items,"provider":a.provider.Name(),"workers":a.workers})
}

func (a *app) health(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodGet{common.APIError(w,405,"METHOD","Use GET");return}
	var queued,running,failed int
	_ = a.db.QueryRow(`SELECT
		COUNT(*) FILTER(WHERE status='QUEUED'),
		COUNT(*) FILTER(WHERE status='RUNNING'),
		COUNT(*) FILTER(WHERE status='FAILED')
		FROM backups.restore_points`).Scan(&queued,&running,&failed)
	common.JSON(w,200,map[string]any{
		"status":"ok","service":"backups","provider":a.provider.Name(),
		"queued":queued,"running":running,"failed":failed,"time":time.Now().UTC(),
	})
}
