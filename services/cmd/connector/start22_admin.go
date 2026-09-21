package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"himate.local/services/internal/common"
)

func (a *app) start22Mapping(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use GET")
		return
	}
	common.JSON(w, http.StatusOK, map[string]any{
		"protocol_version": start22ProtocolVersion,
		"source_system": start22SourceSystem,
		"module_count": len(start22DatasetRegistry),
		"retention_policy": "HIMATE_7Y",
		"retention_years": start22RetentionYears,
		"items": start22RegistryPayload(),
	})
}

func (a *app) start22Summary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use GET")
		return
	}
	partnerID := strings.TrimSpace(r.URL.Query().Get("partner_id"))
	environment := normalizeEnvironment(r.URL.Query().Get("environment"))
	where := []string{"1=1"}
	args := []any{}
	if partnerID != "" {
		args = append(args, partnerID)
		where = append(where, "partner_id=$"+strconv.Itoa(len(args)))
	}
	if environment != "" {
		args = append(args, environment)
		where = append(where, "environment=$"+strconv.Itoa(len(args)))
	}
	whereSQL := strings.Join(where, " AND ")

	var batches, records, routeErrors, legalHolds, privacyDeletes, modules, datasets int
	var oldest, nextPurge, latest sql.NullTime
	_ = a.db.QueryRow("SELECT COUNT(*),COALESCE(SUM(route_error_count),0),MAX(received_at) FROM connector.data_batches WHERE "+whereSQL, args...).
		Scan(&batches, &routeErrors, &latest)
	_ = a.db.QueryRow("SELECT COUNT(*),COUNT(DISTINCT module_key),COUNT(DISTINCT dataset_key),COUNT(*) FILTER(WHERE legal_hold=TRUE),COUNT(*) FILTER(WHERE privacy_delete_requested=TRUE),MIN(received_at),MIN(retain_until) FROM connector.data_records WHERE "+whereSQL, args...).
		Scan(&records, &modules, &datasets, &legalHolds, &privacyDeletes, &oldest, &nextPurge)

	states := []map[string]any{}
	stateWhere := []string{"1=1"}
	stateArgs := []any{}
	if partnerID != "" {
		stateArgs = append(stateArgs, partnerID)
		stateWhere = append(stateWhere, "partner_id=$"+strconv.Itoa(len(stateArgs)))
	}
	if environment != "" {
		stateArgs = append(stateArgs, environment)
		stateWhere = append(stateWhere, "environment=$"+strconv.Itoa(len(stateArgs)))
	}
	rows, err := a.db.Query(`SELECT partner_id,environment,reported_version,health,protocol_version,sync_status,
		last_seen_at,last_metric_sync_at,last_data_sync_at,last_reconciliation_at,updated_at
		FROM connector.partner_state WHERE `+strings.Join(stateWhere," AND ")+` ORDER BY partner_id,environment`, stateArgs...)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var p,e,version,health,protocol,sync string
			var seen,metrics,dataSync,reconciled sql.NullTime
			var updated time.Time
			if rows.Scan(&p,&e,&version,&health,&protocol,&sync,&seen,&metrics,&dataSync,&reconciled,&updated)==nil {
				state := map[string]any{
					"partner_id":p,"environment":e,"reported_version":version,"health":health,
					"protocol_version":protocol,"sync_status":sync,"updated_at":updated.UTC(),
				}
				if seen.Valid { state["last_seen_at"]=seen.Time.UTC() }
				if metrics.Valid { state["last_metric_sync_at"]=metrics.Time.UTC() }
				if dataSync.Valid { state["last_data_sync_at"]=dataSync.Time.UTC() }
				if reconciled.Valid { state["last_reconciliation_at"]=reconciled.Time.UTC() }
				states=append(states,state)
			}
		}
	}

	var oldestValue,nextPurgeValue,latestValue any
	if oldest.Valid { oldestValue=oldest.Time.UTC() }
	if nextPurge.Valid { nextPurgeValue=nextPurge.Time.UTC() }
	if latest.Valid { latestValue=latest.Time.UTC() }

	common.JSON(w,http.StatusOK,map[string]any{
		"protocol_version":start22ProtocolVersion,
		"source_system":start22SourceSystem,
		"registry_modules":len(start22DatasetRegistry),
		"partner_id":partnerID,
		"environment":environment,
		"batches":batches,
		"records":records,
		"module_coverage":modules,
		"dataset_coverage":datasets,
		"route_errors":routeErrors,
		"legal_holds":legalHolds,
		"privacy_delete_requests":privacyDeletes,
		"oldest_record_at":oldestValue,
		"latest_batch_at":latestValue,
		"next_retention_expiry":nextPurgeValue,
		"retention_policy":"HIMATE_7Y",
		"retention_years":start22RetentionYears,
		"states":states,
	})
}

func (a *app) start22ImpactRetention(ctx context.Context,sourceRef,action string,legalHold *bool) error {
	if strings.TrimSpace(a.impactHost)=="" { return nil }
	payload:=map[string]any{"source_ref":sourceRef,"action":action}
	if legalHold!=nil { payload["legal_hold"]=*legalHold }
	raw,_:=json.Marshal(payload)
	req,err:=http.NewRequestWithContext(ctx,http.MethodPost,"http://"+a.impactHost+"/internal/v1/impact/retention",bytes.NewReader(raw))
	if err!=nil{return err}
	req.Header.Set("Content-Type","application/json")
	req.Header.Set("X-Himate-Internal-Token",a.internalToken)
	resp,err:=a.client.Do(req)
	if err!=nil{return err}
	defer resp.Body.Close()
	if resp.StatusCode<200||resp.StatusCode>=300{return fmt.Errorf("Impact retention returned %d",resp.StatusCode)}
	return nil
}

func (a *app) start22Retention(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		partnerID:=strings.TrimSpace(r.URL.Query().Get("partner_id"))
		if partnerID=="" {
			common.APIError(w,http.StatusBadRequest,"VALIDATION","partner_id is required")
			return
		}
		var total,holds,pendingDelete int
		var oldest,next sql.NullTime
		err:=a.db.QueryRow(`SELECT COUNT(*),COUNT(*) FILTER(WHERE legal_hold=TRUE),
			COUNT(*) FILTER(WHERE privacy_delete_requested=TRUE),MIN(received_at),MIN(retain_until)
			FROM connector.data_records WHERE partner_id=$1`,partnerID).
			Scan(&total,&holds,&pendingDelete,&oldest,&next)
		if err!=nil {
			common.APIError(w,http.StatusInternalServerError,"DB","Could not load retention summary")
			return
		}
		var oldestValue,nextValue any
		if oldest.Valid { oldestValue=oldest.Time.UTC() }
		if next.Valid { nextValue=next.Time.UTC() }
		common.JSON(w,http.StatusOK,map[string]any{
			"partner_id":partnerID,"records":total,"legal_holds":holds,"privacy_delete_requests":pendingDelete,
			"retention_policy":"HIMATE_7Y","retention_years":start22RetentionYears,
			"oldest_record_at":oldestValue,"next_retention_expiry":nextValue,
		})
	case http.MethodPost:
		var in struct {
			Action string `json:"action"`
			RecordID int64 `json:"record_id"`
			LegalHold *bool `json:"legal_hold"`
		}
		if common.Decode(r,&in)!=nil {
			common.APIError(w,http.StatusBadRequest,"JSON","Invalid retention action")
			return
		}
		in.Action=strings.ToUpper(strings.TrimSpace(in.Action))
		switch in.Action {
		case "PURGE_EXPIRED":
			deleted,err:=a.start22PurgeRetention(r.Context())
			if err!=nil {
				common.APIError(w,http.StatusInternalServerError,"DB","Could not apply retention purge")
				return
			}
			common.JSON(w,http.StatusOK,map[string]any{"action":in.Action,"deleted":deleted})
		case "SET_LEGAL_HOLD":
			if in.RecordID<=0 || in.LegalHold==nil {
				common.APIError(w,http.StatusBadRequest,"VALIDATION","record_id and legal_hold are required")
				return
			}
			var sourceRef string
			if err:=a.db.QueryRowContext(r.Context(),`SELECT 'connector:data_record:'||id::text FROM connector.data_records WHERE id=$1`,in.RecordID).Scan(&sourceRef);err!=nil{
				common.APIError(w,http.StatusNotFound,"NOT_FOUND","Connector data record not found")
				return
			}
			if err:=a.start22ImpactRetention(r.Context(),sourceRef,"SET_LEGAL_HOLD",in.LegalHold);err!=nil{
				common.APIError(w,http.StatusBadGateway,"IMPACT_RETENTION","Could not synchronize Impact legal hold")
				return
			}
			result,err:=a.db.ExecContext(r.Context(),`UPDATE connector.data_records SET legal_hold=$2 WHERE id=$1`,in.RecordID,*in.LegalHold)
			if err!=nil {
				common.APIError(w,http.StatusInternalServerError,"DB","Could not update legal hold")
				return
			}
			affected,_:=result.RowsAffected()
			common.JSON(w,http.StatusOK,map[string]any{"action":in.Action,"record_id":in.RecordID,"legal_hold":*in.LegalHold,"affected":affected})
		case "PRIVACY_DELETE":
			if in.RecordID<=0 {
				common.APIError(w,http.StatusBadRequest,"VALIDATION","record_id is required")
				return
			}
			var sourceRef string
			var legalHold bool
			if err:=a.db.QueryRowContext(r.Context(),`SELECT 'connector:data_record:'||id::text,legal_hold FROM connector.data_records WHERE id=$1`,in.RecordID).Scan(&sourceRef,&legalHold);err!=nil{
				common.APIError(w,http.StatusNotFound,"NOT_FOUND","Connector data record not found")
				return
			}
			if legalHold{
				common.APIError(w,http.StatusConflict,"LEGAL_HOLD","Record is under legal hold")
				return
			}
			if err:=a.start22ImpactRetention(r.Context(),sourceRef,"PRIVACY_DELETE",nil);err!=nil{
				common.APIError(w,http.StatusBadGateway,"IMPACT_RETENTION","Could not synchronize Impact privacy deletion")
				return
			}
			result,err:=a.db.ExecContext(r.Context(),`UPDATE connector.data_records SET privacy_delete_requested=TRUE WHERE id=$1 AND legal_hold=FALSE`,in.RecordID)
			if err!=nil {
				common.APIError(w,http.StatusInternalServerError,"DB","Could not request privacy deletion")
				return
			}
			affected,_:=result.RowsAffected()
			if affected!=1 {
				common.APIError(w,http.StatusConflict,"LEGAL_HOLD_OR_NOT_FOUND","Record is under legal hold or does not exist")
				return
			}
			deleted,err:=a.start22PurgeRetention(r.Context())
			if err!=nil {
				common.APIError(w,http.StatusInternalServerError,"DB","Privacy deletion was marked but purge failed")
				return
			}
			common.JSON(w,http.StatusOK,map[string]any{"action":in.Action,"record_id":in.RecordID,"purged":deleted})
		default:
			common.APIError(w,http.StatusBadRequest,"VALIDATION","Unknown retention action")
		}
	default:
		common.APIError(w,http.StatusMethodNotAllowed,"METHOD","Use GET or POST")
	}
}

func (a *app) start22PurgeRetention(ctx context.Context)(int64,error) {
	tx,err:=a.db.BeginTx(ctx,&sql.TxOptions{})
	if err!=nil{return 0,err}
	defer tx.Rollback()
	result,err:=tx.ExecContext(ctx,`DELETE FROM connector.data_records
		WHERE legal_hold=FALSE AND (privacy_delete_requested=TRUE OR retain_until<=NOW())`)
	if err!=nil{return 0,err}
	deleted,_:=result.RowsAffected()
	_,err=tx.ExecContext(ctx,`DELETE FROM connector.data_batches WHERE retain_until<=NOW()
		AND NOT EXISTS(SELECT 1 FROM connector.data_records r
			WHERE r.partner_id=connector.data_batches.partner_id
			AND r.environment=connector.data_batches.environment
			AND r.batch_id=connector.data_batches.batch_id)`)
	if err!=nil{return 0,err}
	_,err=tx.ExecContext(ctx,`DELETE FROM connector.reconciliations WHERE retain_until<=NOW()`)
	if err!=nil{return 0,err}
	if err:=tx.Commit();err!=nil{return 0,err}
	return deleted,nil
}

func (a *app) start22RetentionLoop() {
	run:=func(){
		ctx,cancel:=context.WithTimeout(context.Background(),30*time.Second)
		defer cancel()
		_,_=a.start22PurgeRetention(ctx)
		_,_=a.db.ExecContext(ctx,`DELETE FROM connector.replay_nonces WHERE expires_at<NOW()`)
	}
	run()
	ticker:=time.NewTicker(24*time.Hour)
	defer ticker.Stop()
	for range ticker.C { run() }
}

func (a *app) start22RecordList(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodGet {
		common.APIError(w,http.StatusMethodNotAllowed,"METHOD","Use GET")
		return
	}
	partnerID:=strings.TrimSpace(r.URL.Query().Get("partner_id"))
	if partnerID=="" {
		common.APIError(w,http.StatusBadRequest,"VALIDATION","partner_id is required")
		return
	}
	limit:=50
	if raw:=strings.TrimSpace(r.URL.Query().Get("limit"));raw!=""{
		if n,err:=strconv.Atoi(raw);err==nil&&n>0&&n<=200{limit=n}
	}
	rows,err:=a.db.Query(`SELECT id,partner_id,environment,batch_id,module_key,dataset_key,schema_version,
		period_start,period_end,aggregation,data,data_ciphertext,data_nonce,wrapped_data_key,key_nonce,data_key_version,
		source_checksum,idempotency_key,source_version,route_status,route_error,received_at,retain_until,legal_hold,privacy_delete_requested
		FROM connector.data_records WHERE partner_id=$1 ORDER BY received_at DESC,id DESC LIMIT $2`,partnerID,limit)
	if err!=nil{
		common.APIError(w,http.StatusInternalServerError,"DB","Could not load connector data records")
		return
	}
	defer rows.Close()
	items:=[]map[string]any{}
	for rows.Next(){
		var id int64
		var p,e,batch,module,dataset,aggregation,checksum,idempotency,sourceVersion,routeStatus,routeError string
		var schemaVersion int
		var start,end,received,retain time.Time
		var dataRaw,ciphertext,dataNonce,wrappedKey,keyNonce []byte
		var keyVersion string
		var legalHold,privacyDelete bool
		if rows.Scan(&id,&p,&e,&batch,&module,&dataset,&schemaVersion,&start,&end,&aggregation,&dataRaw,
			&ciphertext,&dataNonce,&wrappedKey,&keyNonce,&keyVersion,
			&checksum,&idempotency,&sourceVersion,&routeStatus,&routeError,&received,&retain,&legalHold,&privacyDelete)!=nil{continue}
		var data map[string]any
		if len(ciphertext)>0 {
			decrypted,decryptErr:=a.start22DecryptData(start22EncryptedEnvelope{
				Ciphertext:ciphertext,DataNonce:dataNonce,WrappedKey:wrappedKey,KeyNonce:keyNonce,KeyVersion:keyVersion,
			},p,e,dataset,idempotency)
			if decryptErr!=nil {
				common.APIError(w,http.StatusInternalServerError,"DECRYPTION","Could not decrypt retained START-22 data")
				return
			}
			data=decrypted
		} else {
			_ = json.Unmarshal(dataRaw,&data)
		}
		items=append(items,map[string]any{
			"id":id,"partner_id":p,"environment":e,"batch_id":batch,"module_key":module,"dataset_key":dataset,
			"schema_version":schemaVersion,"period_start":start.Format("2006-01-02"),"period_end":end.Format("2006-01-02"),
			"aggregation":aggregation,"data":data,"source_checksum_sha512":checksum,"idempotency_key":idempotency,
			"source_version":sourceVersion,"route_status":routeStatus,"route_error":routeError,
			"received_at":received.UTC(),"retain_until":retain.UTC(),"retention_policy":"HIMATE_7Y",
			"data_encryption":map[bool]string{true:start22EncryptionAlgorithm,false:"LEGACY_PLAINTEXT"}[len(ciphertext)>0],
			"data_key_version":keyVersion,
			"legal_hold":legalHold,"privacy_delete_requested":privacyDelete,
		})
	}
	common.JSON(w,http.StatusOK,map[string]any{"items":items,"count":len(items),"limit":limit})
}
