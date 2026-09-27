package main

import (
	"context"
	"database/sql"
	"fmt"
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

type central14AuditAggregate struct {
	Count  int
	Latest any
}

func (a *app) materializeCentralAdministration(ctx context.Context) map[string]any {
	var partners []map[string]any
	var billing central14BillingAdministrationSummary
	var backups central14BackupSummary
	var profile map[string]any
	var partnerErr, billingErr, backupErr, profileErr error

	if snapshot, _, ok := centralStep3SnapshotGet(centralStep4PartnersKey); ok {
		partners = step4Items(snapshot["items"])
	}

	var wg sync.WaitGroup
	if len(partners) == 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			partners, partnerErr = a.central13AllPartners(ctx)
		}()
	}
	wg.Add(3)
	go func() {
		defer wg.Done()
		billingErr = a.internalGET(ctx, a.hosts["billing"], "/internal/v1/administration/summary", &billing)
	}()
	go func() {
		defer wg.Done()
		profileErr = a.internalGET(ctx, a.hosts["billing"], "/api/v1/billing/profile", &profile)
	}()
	go func() {
		defer wg.Done()
		backupErr = a.internalGET(ctx, a.hosts["backups"], "/internal/v1/backups/summary", &backups)
	}()
	wg.Wait()

	unavailable := []string{}
	if partnerErr != nil {
		unavailable = append(unavailable, "partners")
	}
	if billingErr != nil {
		unavailable = append(unavailable, "billing")
	}
	if profileErr != nil {
		unavailable = append(unavailable, "company_profile")
	}
	if backupErr != nil {
		unavailable = append(unavailable, "backups")
	}

	billingByPartner := mapByPartner(billing.Items)
	backupByPartner := mapByPartner(backups.Items)
	audits := map[string]central14AuditAggregate{}

	rows, err := a.db.QueryContext(ctx, `SELECT partner_id,COUNT(*),MAX(created_at)
		FROM identity.audit_events WHERE partner_id<>'' GROUP BY partner_id`)
	if err != nil {
		unavailable = append(unavailable, "partner_audit")
	} else {
		for rows.Next() {
			var id string
			var count int
			var latest time.Time
			if scanErr := rows.Scan(&id, &count, &latest); scanErr != nil {
				unavailable = append(unavailable, "partner_audit")
				break
			}
			audits[id] = central14AuditAggregate{Count: count, Latest: latest.UTC()}
		}
		if rowsErr := rows.Err(); rowsErr != nil {
			unavailable = append(unavailable, "partner_audit")
		}
		rows.Close()
	}

	var auditTotal, activeAdmins int
	var latestAudit any
	var latestAuditTime sql.NullTime
	if err := a.db.QueryRowContext(ctx, `SELECT COUNT(*),MAX(created_at) FROM identity.audit_events`).Scan(&auditTotal, &latestAuditTime); err != nil {
		unavailable = append(unavailable, "audit")
	} else if latestAuditTime.Valid {
		latestAudit = latestAuditTime.Time.UTC()
	}
	if err := a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM identity.users WHERE active=TRUE`).Scan(&activeAdmins); err != nil {
		unavailable = append(unavailable, "administrators")
	}

	rowsOut := make([]map[string]any, 0, len(partners))
	for _, partner := range partners {
		id := central10String(partner["id"])
		if id == "" {
			continue
		}
		finance := billingByPartner[id]
		recovery := backupByPartner[id]
		audit := audits[id]
		rowsOut = append(rowsOut, map[string]any{
			"partner_id":              id,
			"partner_name":            central10String(partner["display_name"]),
			"brand_name":              central10String(partner["brand_name"]),
			"legal_name":              central10String(partner["legal_name"]),
			"contact_email":           central10String(partner["contact_email"]),
			"lifecycle":               central10String(partner["lifecycle"]),
			"test_partner":            partner["test_partner"],
			"document_count":          central10Int(finance["document_count"]),
			"invoice_count":           central10Int(finance["invoice_count"]),
			"last_document_at":        finance["last_document_at"],
			"last_invoice_at":         finance["last_invoice_at"],
			"audit_event_count":       audit.Count,
			"last_audit_at":           audit.Latest,
			"backup_status":           central10String(recovery["latest_backup_status"]),
			"restore_test_status":     central10String(recovery["latest_restore_test_status"]),
			"recoverability_status":   central10String(recovery["recoverability_status"]),
			"latest_restore_point_id": central10String(recovery["latest_restore_point_id"]),
			"latest_backup_at":        recovery["latest_backup_at"],
		})
	}
	sort.SliceStable(rowsOut, func(i, j int) bool {
		left := strings.ToLower(central10String(rowsOut[i]["partner_name"]))
		right := strings.ToLower(central10String(rowsOut[j]["partner_name"]))
		if left == right {
			return central10String(rowsOut[i]["partner_id"]) < central10String(rowsOut[j]["partner_id"])
		}
		return left < right
	})

	platformBackup := backupByPartner["_platform"]
	status := "healthy"
	if len(unavailable) > 0 {
		status = "partial"
	}
	return map[string]any{
		"status":      status,
		"unavailable": unavailable,
		"company": map[string]any{
			"profile":                 profile,
			"document_count":          central10Int(billing.Company["document_count"]),
			"last_document_at":        billing.Company["last_document_at"],
			"audit_event_count":       auditTotal,
			"last_audit_at":           latestAudit,
			"active_administrators":   activeAdmins,
			"backup_status":           central10String(platformBackup["latest_backup_status"]),
			"restore_test_status":     central10String(platformBackup["latest_restore_test_status"]),
			"recoverability_status":   central10String(platformBackup["recoverability_status"]),
			"latest_restore_point_id": central10String(platformBackup["latest_restore_point_id"]),
			"latest_backup_at":        platformBackup["latest_backup_at"],
			"backup_provider":         backups.Provider,
		},
		"items": rowsOut,
		"kpis": map[string]any{
			"partners":                    len(partners),
			"verified_recovery":           central14CountStatus(rowsOut, "recoverability_status", "VERIFIED"),
			"needs_recovery_verification": central14CountNotStatus(rowsOut, "recoverability_status", "VERIFIED"),
			"documents":                   central14Sum(rowsOut, "document_count"),
			"audit_events":                central14Sum(rowsOut, "audit_event_count"),
		},
	}
}

func (a *app) central14Administration(w http.ResponseWriter, r *http.Request, actor user) {
	if r.Method != http.MethodGet {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use GET")
		return
	}
	started := time.Now()
	snapshot, updatedAt, ok := a.centralSnapshotForRead(r.Context(), centralStep4AdministrationKey)
	if !ok {
		a.requestCentralStep4Refresh()
		common.JSON(w, http.StatusOK, map[string]any{
			"ready":      false,
			"company":    map[string]any{},
			"items":      []map[string]any{},
			"pagination": map[string]any{"count": 0, "total": 0, "limit": 0, "offset": 0, "has_more": false},
			"kpis":       map[string]any{},
			"meta":       centralStep4Meta(started, centralStep4AdministrationKey, time.Time{}, "warming", []string{}),
		})
		return
	}
	if time.Since(updatedAt) > 2*centralStep4RefreshInterval {
		a.requestCentralStep4Refresh()
	}

	canPartners := a.hasPermission(actor, "partners.read")
	canBilling := a.hasPermission(actor, "billing.read")
	canBackups := a.hasPermission(actor, "backups.read")
	canAudit := a.hasPermission(actor, "audit.read")

	allRows := step4Items(snapshot["items"])
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	lifecycle := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("lifecycle")))
	rowsOut := make([]map[string]any, 0, len(allRows))
	if canPartners {
		for _, raw := range allRows {
			row := central10CopyMap(raw)
			if lifecycle != "" && lifecycle != "ALL" && strings.ToUpper(central10String(row["lifecycle"])) != lifecycle {
				continue
			}
			if q != "" {
				haystack := strings.ToLower(strings.Join([]string{
					central10String(row["partner_id"]), central10String(row["partner_name"]),
					central10String(row["brand_name"]), central10String(row["legal_name"]),
					central10String(row["contact_email"]),
				}, " "))
				if !strings.Contains(haystack, q) {
					continue
				}
			}
			if !canBilling {
				for _, field := range []string{"document_count", "invoice_count", "last_document_at", "last_invoice_at"} {
					delete(row, field)
				}
			}
			if !canBackups {
				for _, field := range []string{"backup_status", "restore_test_status", "recoverability_status", "latest_restore_point_id", "latest_backup_at"} {
					delete(row, field)
				}
			}
			if !canAudit {
				delete(row, "audit_event_count")
				delete(row, "last_audit_at")
			}
			rowsOut = append(rowsOut, row)
		}
	}

	offset := central14IntQuery(r.URL.Query().Get("offset"), 0, 1_000_000)
	limit := central14IntQuery(r.URL.Query().Get("limit"), 60, 200)
	if limit < 1 {
		limit = 60
	}
	if offset > len(rowsOut) {
		offset = len(rowsOut)
	}
	end := offset + limit
	if end > len(rowsOut) {
		end = len(rowsOut)
	}

	company := step4Map(snapshot["company"])
	if !canBilling {
		delete(company, "profile")
		delete(company, "document_count")
		delete(company, "last_document_at")
	}
	if !canBackups {
		for _, field := range []string{"backup_status", "restore_test_status", "recoverability_status", "latest_restore_point_id", "latest_backup_at", "backup_provider"} {
			delete(company, field)
		}
	}
	if !canAudit {
		delete(company, "audit_event_count")
		delete(company, "last_audit_at")
	}

	kpis := step4Map(snapshot["kpis"])
	if !canPartners {
		kpis = map[string]any{}
	}
	payload := map[string]any{
		"ready":   true,
		"company": company,
		"items":   rowsOut[offset:end],
		"pagination": map[string]any{
			"count": end - offset, "total": len(rowsOut), "limit": limit,
			"offset": offset, "has_more": end < len(rowsOut),
		},
		"kpis": kpis,
		"access": map[string]any{
			"partners": canPartners,
			"billing":  canBilling,
			"backups":  canBackups,
			"audit":    canAudit,
		},
		"meta": centralStep4Meta(started, centralStep4AdministrationKey, updatedAt, "healthy", []string{}),
	}
	w.Header().Set("X-Himate-Cache", "hot-snapshot")
	w.Header().Set("Server-Timing", fmt.Sprintf("central-administration-snapshot;dur=%d", time.Since(started).Milliseconds()))
	common.JSON(w, http.StatusOK, payload)
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
