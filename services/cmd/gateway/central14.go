package main

import (
	"context"
	"database/sql"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"himate.local/services/internal/common"
)

type central14BillingAdministrationSummary struct {
	Items   []map[string]any
	Company map[string]any
	Source  string
}

type central14BackupSummary struct {
	Items    []map[string]any
	Provider string
	Workers  int
}

func central14IntQuery(raw string, fallback, max int) int {
	value,err:=strconv.Atoi(strings.TrimSpace(raw))
	if err!=nil||value<0{return fallback}
	if value>max{return max}
	return value
}

func mapByPartner(items []map[string]any) map[string]map[string]any {
	out:=map[string]map[string]any{}
	for _,item:=range items{
		id:=central10String(item["partner_id"])
		if id!=""{out[id]=item}
	}
	return out
}

func (a *app) central14Administration(w http.ResponseWriter,r *http.Request,actor user){
	if r.Method!=http.MethodGet{
		common.APIError(w,http.StatusMethodNotAllowed,"METHOD","Use GET")
		return
	}
	started:=time.Now()
	cacheKey:=central10CacheKey(actor,r)
	if payload,ok,_:=central10Cached(cacheKey,true);ok{
		w.Header().Set("X-Himate-Cache","hit")
		common.JSON(w,http.StatusOK,payload)
		return
	}

	ctx,cancel:=context.WithTimeout(r.Context(),2*time.Second)
	defer cancel()

	var partners []map[string]any
	var billing central14BillingAdministrationSummary
	var backups central14BackupSummary
	var profile map[string]any
	var partnerErr,billingErr,backupErr,profileErr error
	var wg sync.WaitGroup
	wg.Add(4)
	go func(){defer wg.Done();partners,partnerErr=a.central13AllPartners(ctx)}()
	go func(){defer wg.Done();billingErr=a.internalGET(ctx,a.hosts["billing"],"/internal/v1/administration/summary",&billing)}()
	go func(){defer wg.Done();backupErr=a.internalGET(ctx,a.hosts["backups"],"/internal/v1/backups/summary",&backups)}()
	go func(){defer wg.Done();profileErr=a.internalGET(ctx,a.hosts["billing"],"/api/v1/billing/profile",&profile)}()
	wg.Wait()
	if partnerErr!=nil{
		common.APIError(w,http.StatusBadGateway,"ADMINISTRATION_UNAVAILABLE","Partner registry is temporarily unavailable")
		return
	}

	unavailable:=[]string{}
	if billingErr!=nil{unavailable=append(unavailable,"billing")}
	if backupErr!=nil{unavailable=append(unavailable,"backups")}
	if profileErr!=nil{unavailable=append(unavailable,"company_profile")}

	billingByPartner:=mapByPartner(billing.Items)
	backupByPartner:=mapByPartner(backups.Items)

	type auditAggregate struct{Count int;Latest any}
	audits:=map[string]auditAggregate{}
	rows,err:=a.db.QueryContext(ctx,`SELECT partner_id,COUNT(*),MAX(created_at)
		FROM identity.audit_events WHERE partner_id<>'' GROUP BY partner_id`)
	if err==nil{
		defer rows.Close()
		for rows.Next(){
			var id string
			var count int
			var latest time.Time
			if rows.Scan(&id,&count,&latest)==nil{audits[id]=auditAggregate{Count:count,Latest:latest.UTC()}}
		}
	}else{
		unavailable=append(unavailable,"partner_audit")
	}

	var auditTotal,activeAdmins int
	var latestAudit any
	var latestAuditTime sql.NullTime
	if err:=a.db.QueryRowContext(ctx,`SELECT COUNT(*),MAX(created_at) FROM identity.audit_events`).Scan(&auditTotal,&latestAuditTime);err==nil&&latestAuditTime.Valid{
		latestAudit=latestAuditTime.Time.UTC()
	}
	_ = a.db.QueryRowContext(ctx,`SELECT COUNT(*) FROM identity.users WHERE active=TRUE`).Scan(&activeAdmins)

	q:=strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	lifecycle:=strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("lifecycle")))
	rowsOut:=make([]map[string]any,0,len(partners))
	for _,partner:=range partners{
		id:=central10String(partner["id"])
		if id==""{continue}
		if lifecycle!=""&&lifecycle!="ALL"&&strings.ToUpper(central10String(partner["lifecycle"]))!=lifecycle{continue}
		if q!=""{
			haystack:=strings.ToLower(strings.Join([]string{
				id,central10String(partner["display_name"]),central10String(partner["brand_name"]),
				central10String(partner["legal_name"]),central10String(partner["contact_email"]),
			}," "))
			if !strings.Contains(haystack,q){continue}
		}
		finance:=billingByPartner[id]
		recovery:=backupByPartner[id]
		audit:=audits[id]
		rowsOut=append(rowsOut,map[string]any{
			"partner_id":id,
			"partner_name":central10String(partner["display_name"]),
			"brand_name":central10String(partner["brand_name"]),
			"legal_name":central10String(partner["legal_name"]),
			"lifecycle":central10String(partner["lifecycle"]),
			"test_partner":partner["test_partner"],
			"document_count":central10Int(finance["document_count"]),
			"invoice_count":central10Int(finance["invoice_count"]),
			"last_document_at":finance["last_document_at"],
			"last_invoice_at":finance["last_invoice_at"],
			"audit_event_count":audit.Count,
			"last_audit_at":audit.Latest,
			"backup_status":central10String(recovery["latest_backup_status"]),
			"restore_test_status":central10String(recovery["latest_restore_test_status"]),
			"recoverability_status":central10String(recovery["recoverability_status"]),
			"latest_restore_point_id":central10String(recovery["latest_restore_point_id"]),
			"latest_backup_at":recovery["latest_backup_at"],
		})
	}
	sort.SliceStable(rowsOut,func(i,j int)bool{
		left:=strings.ToLower(central10String(rowsOut[i]["partner_name"]))
		right:=strings.ToLower(central10String(rowsOut[j]["partner_name"]))
		if left==right{return central10String(rowsOut[i]["partner_id"])<central10String(rowsOut[j]["partner_id"])}
		return left<right
	})

	offset:=central14IntQuery(r.URL.Query().Get("offset"),0,1_000_000)
	limit:=central14IntQuery(r.URL.Query().Get("limit"),60,200)
	if limit<1{limit=60}
	if offset>len(rowsOut){offset=len(rowsOut)}
	end:=offset+limit;if end>len(rowsOut){end=len(rowsOut)}

	platformBackup:=backupByPartner["_platform"]
	status:="healthy";if len(unavailable)>0{status="partial"}
	payload:=map[string]any{
		"company":map[string]any{
			"profile":profile,
			"document_count":central10Int(billing.Company["document_count"]),
			"last_document_at":billing.Company["last_document_at"],
			"audit_event_count":auditTotal,
			"last_audit_at":latestAudit,
			"active_administrators":activeAdmins,
			"backup_status":central10String(platformBackup["latest_backup_status"]),
			"restore_test_status":central10String(platformBackup["latest_restore_test_status"]),
			"recoverability_status":central10String(platformBackup["recoverability_status"]),
			"latest_restore_point_id":central10String(platformBackup["latest_restore_point_id"]),
			"latest_backup_at":platformBackup["latest_backup_at"],
			"backup_provider":backups.Provider,
		},
		"items":rowsOut[offset:end],
		"pagination":map[string]any{"count":end-offset,"total":len(rowsOut),"limit":limit,"offset":offset,"has_more":end<len(rowsOut)},
		"kpis":map[string]any{
			"partners":len(partners),
			"verified_recovery":central14CountStatus(rowsOut,"recoverability_status","VERIFIED"),
			"needs_recovery_verification":central14CountNotStatus(rowsOut,"recoverability_status","VERIFIED"),
			"documents":central14Sum(rowsOut,"document_count"),
			"audit_events":central14Sum(rowsOut,"audit_event_count"),
		},
		"meta":map[string]any{
			"architecture":"GO_BACKEND_READ_MODEL",
			"frontend_role":"PRESENTATION_ONLY",
			"status":status,"unavailable":unavailable,
			"duration_ms":time.Since(started).Milliseconds(),
			"generated_at":time.Now().UTC(),
		},
	}
	central10Store(cacheKey,payload)
	common.JSON(w,http.StatusOK,payload)
}

func central14Sum(items []map[string]any,key string) int{
	total:=0
	for _,item:=range items{total+=central10Int(item[key])}
	return total
}
func central14CountStatus(items []map[string]any,key,want string) int{
	count:=0
	for _,item:=range items{if strings.EqualFold(central10String(item[key]),want){count++}}
	return count
}
func central14CountNotStatus(items []map[string]any,key,want string) int{
	count:=0
	for _,item:=range items{if !strings.EqualFold(central10String(item[key]),want){count++}}
	return count
}
