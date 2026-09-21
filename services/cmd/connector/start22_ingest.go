package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"himate.local/services/internal/common"
)

const start22MaxBodyBytes = 1 << 20
const start22MaxBatchItems = 250
const start22SignatureWindow = 5 * time.Minute

var start22IdentifierPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{8,180}$`)

type start22DataItem struct {
	ModuleKey      string         `json:"module_key"`
	DatasetKey     string         `json:"dataset_key"`
	SchemaVersion  int            `json:"schema_version"`
	PeriodStart    string         `json:"period_start"`
	PeriodEnd      string         `json:"period_end"`
	Aggregation    string         `json:"aggregation"`
	Data           map[string]any `json:"data"`
	SourceChecksum string         `json:"source_checksum"`
	IdempotencyKey string         `json:"idempotency_key"`
}

type start22BatchInput struct {
	ProtocolVersion string            `json:"protocol_version"`
	SourceSystem    string            `json:"source_system"`
	SourceVersion   string            `json:"source_version"`
	BatchID         string            `json:"batch_id"`
	GeneratedAt     string            `json:"generated_at"`
	Items           []start22DataItem `json:"items"`
}

type start22ReconcileDataset struct {
	DatasetKey        string `json:"dataset_key"`
	ItemCount         int    `json:"item_count"`
	AggregateChecksum string `json:"aggregate_checksum"`
}

type start22ReconcileInput struct {
	ProtocolVersion  string                    `json:"protocol_version"`
	SourceSystem     string                    `json:"source_system"`
	SourceVersion    string                    `json:"source_version"`
	ReconciliationID string                    `json:"reconciliation_id"`
	BatchID          string                    `json:"batch_id"`
	GeneratedAt      string                    `json:"generated_at"`
	Datasets         []start22ReconcileDataset `json:"datasets"`
}

type start22StoredRecord struct {
	ID             int64
	PartnerID      string
	Environment    string
	BatchID        string
	ModuleKey      string
	DatasetKey     string
	SchemaVersion  int
	PeriodStart    time.Time
	PeriodEnd      time.Time
	Aggregation    string
	Data           map[string]any
	SourceChecksum string
	IdempotencyKey string
	SourceVersion  string
	ReceivedAt     time.Time
	RetainUntil    time.Time
}

func start22SHA512Hex(raw []byte) string {
	sum := sha512.Sum512(raw)
	return hex.EncodeToString(sum[:])
}

func start22DataChecksum(data map[string]any) string {
	raw, _ := json.Marshal(data)
	return start22SHA512Hex(raw)
}

func start22AggregateChecksum(values []string) string {
	clean := append([]string(nil), values...)
	sort.Strings(clean)
	return start22SHA512Hex([]byte(strings.Join(clean, "\n")))
}

func start22ConstantHexEqual(a, b string) bool {
	ab, errA := hex.DecodeString(strings.TrimSpace(a))
	bb, errB := hex.DecodeString(strings.TrimSpace(b))
	if errA != nil || errB != nil || len(ab) != len(bb) || len(ab) == 0 {
		return false
	}
	return subtle.ConstantTimeCompare(ab, bb) == 1
}

func start22Signature(token, timestamp, nonce, bodySHA string) string {
	mac := hmac.New(sha512.New, []byte(token))
	_, _ = mac.Write([]byte(timestamp))
	_, _ = mac.Write([]byte("\n"))
	_, _ = mac.Write([]byte(nonce))
	_, _ = mac.Write([]byte("\n"))
	_, _ = mac.Write([]byte(bodySHA))
	return hex.EncodeToString(mac.Sum(nil))
}

func start22ParseDate(value string) (time.Time, error) {
	return time.Parse("2006-01-02", strings.TrimSpace(value))
}

func start22ReadJSON(raw []byte, dst any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("request body must contain exactly one JSON value")
	}
	return nil
}

func (a *app) start22VerifySignedRequest(w http.ResponseWriter, r *http.Request, credentialID string) ([]byte, bool) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, start22MaxBodyBytes+1))
	if err != nil {
		common.APIError(w, http.StatusBadRequest, "BODY", "Could not read request body")
		return nil, false
	}
	if len(raw) > start22MaxBodyBytes {
		common.APIError(w, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "Connector payload exceeds 1 MiB")
		return nil, false
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		common.APIError(w, http.StatusBadRequest, "JSON", "Request body is required")
		return nil, false
	}

	timestampRaw := strings.TrimSpace(r.Header.Get("X-Himate-Timestamp"))
	nonce := strings.TrimSpace(r.Header.Get("X-Himate-Nonce"))
	bodySHA := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Himate-Body-SHA512")))
	signature := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Himate-Signature")))
	if timestampRaw == "" || nonce == "" || bodySHA == "" || signature == "" {
		common.APIError(w, http.StatusUnauthorized, "SIGNATURE_REQUIRED", "START-22 signed connector headers are required")
		return nil, false
	}
	timestamp, err := time.Parse(time.RFC3339, timestampRaw)
	if err != nil || time.Since(timestamp.UTC()) > start22SignatureWindow || time.Until(timestamp.UTC()) > start22SignatureWindow {
		common.APIError(w, http.StatusUnauthorized, "SIGNATURE_TIMESTAMP", "Connector timestamp is outside the allowed five-minute window")
		return nil, false
	}
	if !start22IdentifierPattern.MatchString(nonce) {
		common.APIError(w, http.StatusUnauthorized, "SIGNATURE_NONCE", "Connector nonce is invalid")
		return nil, false
	}
	expectedBodySHA := start22SHA512Hex(raw)
	if !start22ConstantHexEqual(bodySHA, expectedBodySHA) {
		common.APIError(w, http.StatusUnauthorized, "BODY_CHECKSUM", "Connector body checksum does not match")
		return nil, false
	}
	token := bearer(r)
	expectedSignature := start22Signature(token, timestampRaw, nonce, expectedBodySHA)
	if !start22ConstantHexEqual(signature, expectedSignature) {
		common.APIError(w, http.StatusUnauthorized, "SIGNATURE_INVALID", "Connector request signature is invalid")
		return nil, false
	}

	nonceHash := start22SHA512Hex([]byte(nonce))
	_, _ = a.db.ExecContext(r.Context(), `DELETE FROM connector.replay_nonces WHERE expires_at < NOW()`)
	result, err := a.db.ExecContext(r.Context(), `INSERT INTO connector.replay_nonces(credential_id,nonce_hash,expires_at)
		VALUES($1,$2,NOW()+INTERVAL '10 minutes') ON CONFLICT(credential_id,nonce_hash) DO NOTHING`, credentialID, nonceHash)
	if err != nil {
		common.APIError(w, http.StatusInternalServerError, "DB", "Could not record connector nonce")
		return nil, false
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		common.APIError(w, http.StatusConflict, "REPLAY_DETECTED", "Connector nonce has already been used")
		return nil, false
	}
	return raw, true
}

func start22ValidateScalar(value any) error {
	switch typed := value.(type) {
	case nil, bool, float64:
		return nil
	case string:
		if len([]rune(typed)) > 256 {
			return errors.New("string value exceeds 256 characters")
		}
		if strings.ContainsAny(typed, "\r\n") {
			return errors.New("multiline/free-text values are not accepted")
		}
		return nil
	default:
		return fmt.Errorf("nested or array value of type %T is not accepted", value)
	}
}

func start22ValidateItem(item *start22DataItem) (start22DatasetDefinition, time.Time, time.Time, error) {
	item.ModuleKey = strings.TrimSpace(item.ModuleKey)
	item.DatasetKey = strings.TrimSpace(item.DatasetKey)
	item.Aggregation = strings.ToUpper(strings.TrimSpace(item.Aggregation))
	item.SourceChecksum = strings.ToLower(strings.TrimSpace(item.SourceChecksum))
	item.IdempotencyKey = strings.TrimSpace(item.IdempotencyKey)
	if item.SchemaVersion != 1 {
		return start22DatasetDefinition{}, time.Time{}, time.Time{}, errors.New("schema_version must be 1")
	}
	definition, ok := start22DatasetByKey[item.DatasetKey]
	if !ok || definition.ModuleKey != item.ModuleKey {
		return start22DatasetDefinition{}, time.Time{}, time.Time{}, errors.New("module_key/dataset_key is not present in the START-22 allowlist")
	}
	if item.Aggregation == "" {
		item.Aggregation = "LATEST"
	}
	if item.Aggregation != "SUM" && item.Aggregation != "LATEST" && item.Aggregation != "AVERAGE" {
		return start22DatasetDefinition{}, time.Time{}, time.Time{}, errors.New("aggregation must be SUM, LATEST or AVERAGE")
	}
	start, err := start22ParseDate(item.PeriodStart)
	if err != nil {
		return start22DatasetDefinition{}, time.Time{}, time.Time{}, errors.New("period_start must use YYYY-MM-DD")
	}
	end, err := start22ParseDate(item.PeriodEnd)
	if err != nil || end.Before(start) {
		return start22DatasetDefinition{}, time.Time{}, time.Time{}, errors.New("period_end must use YYYY-MM-DD and be on/after period_start")
	}
	if !start22IdentifierPattern.MatchString(item.IdempotencyKey) {
		return start22DatasetDefinition{}, time.Time{}, time.Time{}, errors.New("idempotency_key must be 8-180 safe identifier characters")
	}
	if len(item.Data) == 0 || len(item.Data) > len(definition.AllowedFields) {
		return start22DatasetDefinition{}, time.Time{}, time.Time{}, errors.New("data must contain approved fields")
	}
	allowed := make(map[string]bool, len(definition.AllowedFields))
	for _, field := range definition.AllowedFields {
		allowed[field] = true
	}
	for key, value := range item.Data {
		if !allowed[key] {
			return start22DatasetDefinition{}, time.Time{}, time.Time{}, fmt.Errorf("field %s is not allowed for dataset %s", key, item.DatasetKey)
		}
		lower := strings.ToLower(key)
		for _, forbidden := range []string{"password", "secret", "token", "stripe", "card_number", "cvv"} {
			if strings.Contains(lower, forbidden) {
				return start22DatasetDefinition{}, time.Time{}, time.Time{}, fmt.Errorf("field %s is prohibited", key)
			}
		}
		if err := start22ValidateScalar(value); err != nil {
			return start22DatasetDefinition{}, time.Time{}, time.Time{}, fmt.Errorf("field %s: %w", key, err)
		}
	}
	if len(item.SourceChecksum) != 128 || !start22ConstantHexEqual(item.SourceChecksum, start22DataChecksum(item.Data)) {
		return start22DatasetDefinition{}, time.Time{}, time.Time{}, errors.New("source_checksum must be the SHA-512 checksum of data")
	}
	return definition, start.UTC(), end.UTC(), nil
}

func (a *app) start22DataBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use POST")
		return
	}
	credential, err := a.authenticate(r)
	if err != nil {
		common.APIError(w, http.StatusUnauthorized, "CONNECTOR_UNAUTHORIZED", err.Error())
		return
	}
	raw, ok := a.start22VerifySignedRequest(w, r, credential.CredentialID)
	if !ok {
		return
	}
	var in start22BatchInput
	if err := start22ReadJSON(raw, &in); err != nil {
		common.APIError(w, http.StatusBadRequest, "JSON", "Invalid START-22 batch payload")
		return
	}
	in.ProtocolVersion = strings.TrimSpace(in.ProtocolVersion)
	in.SourceSystem = strings.ToUpper(strings.TrimSpace(in.SourceSystem))
	in.SourceVersion = strings.TrimSpace(in.SourceVersion)
	in.BatchID = strings.TrimSpace(in.BatchID)
	if in.ProtocolVersion != start22ProtocolVersion || in.SourceSystem != start22SourceSystem {
		common.APIError(w, http.StatusBadRequest, "PROTOCOL", "Unsupported connector protocol/source system")
		return
	}
	if !start22IdentifierPattern.MatchString(in.BatchID) || len(in.Items) == 0 || len(in.Items) > start22MaxBatchItems {
		common.APIError(w, http.StatusBadRequest, "VALIDATION", "batch_id and 1-250 items are required")
		return
	}
	generatedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(in.GeneratedAt))
	if err != nil {
		common.APIError(w, http.StatusBadRequest, "VALIDATION", "generated_at must be RFC3339")
		return
	}
	requestSHA := start22SHA512Hex(raw)

	var existingSHA string
	var existingAccepted, existingDuplicate int
	err = a.db.QueryRowContext(r.Context(), `SELECT request_sha512,accepted_count,duplicate_count
		FROM connector.data_batches WHERE partner_id=$1 AND environment=$2 AND batch_id=$3`,
		credential.PartnerID, credential.Environment, in.BatchID).Scan(&existingSHA, &existingAccepted, &existingDuplicate)
	if err == nil {
		if !start22ConstantHexEqual(existingSHA, requestSHA) {
			common.APIError(w, http.StatusConflict, "BATCH_IDEMPOTENCY_CONFLICT", "batch_id was already used for a different payload")
			return
		}
		common.JSON(w, http.StatusOK, map[string]any{
			"status": "ALREADY_PROCESSED", "duplicate": true, "partner_id": credential.PartnerID,
			"environment": credential.Environment, "batch_id": in.BatchID,
			"accepted": existingAccepted, "duplicates": existingDuplicate,
		})
		return
	}
	if err != nil && err != sql.ErrNoRows {
		common.APIError(w, http.StatusInternalServerError, "DB", "Could not check batch idempotency")
		return
	}

	validatedDefs := make([]start22DatasetDefinition, len(in.Items))
	periodStarts := make([]time.Time, len(in.Items))
	periodEnds := make([]time.Time, len(in.Items))
	for i := range in.Items {
		definition, start, end, validationErr := start22ValidateItem(&in.Items[i])
		if validationErr != nil {
			common.APIError(w, http.StatusBadRequest, "DATASET_VALIDATION", fmt.Sprintf("item %d: %v", i, validationErr))
			return
		}
		validatedDefs[i], periodStarts[i], periodEnds[i] = definition, start, end
	}

	tx, err := a.db.BeginTx(r.Context(), &sql.TxOptions{})
	if err != nil {
		common.APIError(w, http.StatusInternalServerError, "DB", "Could not begin START-22 batch transaction")
		return
	}
	defer tx.Rollback()

	accepted := 0
	duplicates := 0
	records := make([]start22StoredRecord, 0, len(in.Items))
	for i, item := range in.Items {
		dataRaw, _ := json.Marshal(item.Data)
		var id int64
		var receivedAt, retainUntil time.Time
		err = tx.QueryRowContext(r.Context(), `INSERT INTO connector.data_records(
				partner_id,environment,batch_id,module_key,dataset_key,schema_version,period_start,period_end,aggregation,
				data,source_checksum,idempotency_key,source_version,retain_until)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb,$11,$12,$13,NOW()+INTERVAL '7 years')
			ON CONFLICT(partner_id,environment,dataset_key,idempotency_key) DO NOTHING
			RETURNING id,received_at,retain_until`,
			credential.PartnerID, credential.Environment, in.BatchID, item.ModuleKey, item.DatasetKey, item.SchemaVersion,
			periodStarts[i], periodEnds[i], item.Aggregation, string(dataRaw), item.SourceChecksum, item.IdempotencyKey, in.SourceVersion).
			Scan(&id, &receivedAt, &retainUntil)
		if err == sql.ErrNoRows {
			var existingChecksum string
			if lookupErr := tx.QueryRowContext(r.Context(), `SELECT source_checksum FROM connector.data_records
				WHERE partner_id=$1 AND environment=$2 AND dataset_key=$3 AND idempotency_key=$4`,
				credential.PartnerID, credential.Environment, item.DatasetKey, item.IdempotencyKey).Scan(&existingChecksum); lookupErr != nil {
				common.APIError(w, http.StatusInternalServerError, "DB", "Could not resolve idempotent data item")
				return
			}
			if !start22ConstantHexEqual(existingChecksum, item.SourceChecksum) {
				common.APIError(w, http.StatusConflict, "ITEM_IDEMPOTENCY_CONFLICT", fmt.Sprintf("item %d idempotency_key was already used for different data", i))
				return
			}
			duplicates++
			continue
		}
		if err != nil {
			common.APIError(w, http.StatusInternalServerError, "DB", "Could not store START-22 data item")
			return
		}
		accepted++
		records = append(records, start22StoredRecord{
			ID:id, PartnerID:credential.PartnerID, Environment:credential.Environment, BatchID:in.BatchID,
			ModuleKey:item.ModuleKey, DatasetKey:item.DatasetKey, SchemaVersion:item.SchemaVersion,
			PeriodStart:periodStarts[i], PeriodEnd:periodEnds[i], Aggregation:item.Aggregation, Data:item.Data,
			SourceChecksum:item.SourceChecksum, IdempotencyKey:item.IdempotencyKey, SourceVersion:in.SourceVersion,
			ReceivedAt:receivedAt.UTC(), RetainUntil:retainUntil.UTC(),
		})
		_ = validatedDefs[i]
	}
	_, err = tx.ExecContext(r.Context(), `INSERT INTO connector.data_batches(
		partner_id,environment,batch_id,protocol_version,source_system,source_version,generated_at,request_sha512,
		item_count,accepted_count,duplicate_count,signature_verified,status)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,TRUE,'ACCEPTED')`,
		credential.PartnerID, credential.Environment, in.BatchID, in.ProtocolVersion, in.SourceSystem, in.SourceVersion,
		generatedAt.UTC(), requestSHA, len(in.Items), accepted, duplicates)
	if err != nil {
		common.APIError(w, http.StatusInternalServerError, "DB", "Could not record START-22 batch")
		return
	}
	_, err = tx.ExecContext(r.Context(), `UPDATE connector.partner_state SET protocol_version=$3,last_data_sync_at=NOW(),
		sync_status='RECEIVED',updated_at=NOW() WHERE partner_id=$1 AND environment=$2`,
		credential.PartnerID, credential.Environment, in.ProtocolVersion)
	if err != nil {
		common.APIError(w, http.StatusInternalServerError, "DB", "Could not update connector sync state")
		return
	}
	if err := tx.Commit(); err != nil {
		common.APIError(w, http.StatusInternalServerError, "DB", "Could not commit START-22 batch")
		return
	}

	routed := 0
	routeErrors := 0
	for _, record := range records {
		success, routeErr := a.start22RouteImpactRecord(r.Context(), record)
		if routeErr != nil {
			routeErrors++
			_, _ = a.db.ExecContext(r.Context(), `UPDATE connector.data_records SET route_status='ERROR',route_error=$2 WHERE id=$1`, record.ID, routeErr.Error())
			continue
		}
		if success {
			routed++
			_, _ = a.db.ExecContext(r.Context(), `UPDATE connector.data_records SET route_status='ROUTED',route_error='' WHERE id=$1`, record.ID)
		} else {
			_, _ = a.db.ExecContext(r.Context(), `UPDATE connector.data_records SET route_status='STORED',route_error='' WHERE id=$1`, record.ID)
		}
	}
	status := "ACCEPTED"
	if routeErrors > 0 {
		status = "ACCEPTED_WITH_ROUTE_ERRORS"
	}
	_, _ = a.db.ExecContext(r.Context(), `UPDATE connector.data_batches SET status=$4,routed_count=$5,route_error_count=$6
		WHERE partner_id=$1 AND environment=$2 AND batch_id=$3`,
		credential.PartnerID, credential.Environment, in.BatchID, status, routed, routeErrors)

	common.JSON(w, http.StatusAccepted, map[string]any{
		"status": status, "partner_id": credential.PartnerID, "environment": credential.Environment,
		"batch_id": in.BatchID, "protocol_version": in.ProtocolVersion, "accepted": accepted,
		"duplicates": duplicates, "routed_metrics": routed, "route_errors": routeErrors,
		"retention_policy": "HIMATE_7Y", "retention_years": start22RetentionYears,
	})
}

func start22MetricUnit(field string) string {
	switch {
	case strings.HasSuffix(field, "_usd"):
		return "USD"
	case strings.HasSuffix(field, "_rate"):
		return "ratio"
	case strings.HasSuffix(field, "_hours"):
		return "hours"
	case strings.HasSuffix(field, "_minutes"):
		return "minutes"
	case strings.HasSuffix(field, "_bytes"):
		return "bytes"
	default:
		return "count"
	}
}

func start22MetricLabel(dataset, field string) string {
	value := strings.ReplaceAll(dataset+" "+field, ".", " ")
	value = strings.ReplaceAll(value, "_", " ")
	words := strings.Fields(value)
	for i, word := range words {
		if strings.EqualFold(word, "usd") {
			words[i] = "USD"
		} else {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	return strings.Join(words, " ")
}

func (a *app) start22EnsureImpactMetric(ctx context.Context, key, label, unit, aggregation string) error {
	if strings.TrimSpace(a.impactHost) == "" {
		return errors.New("Impact service is not configured")
	}
	payload := map[string]any{
		"metric_key": key, "label": label, "unit": unit, "aggregation": aggregation,
		"scope": "PARTNER",
	}
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+a.impactHost+"/internal/v1/impact/definitions/ensure", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Himate-Internal-Token", a.internalToken)
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Impact definition ensure returned %d", resp.StatusCode)
	}
	return nil
}

func (a *app) start22RouteImpactRecord(ctx context.Context, record start22StoredRecord) (bool, error) {
	definition := start22DatasetByKey[record.DatasetKey]
	if definition.TargetService != "impact" {
		return false, nil
	}
	numericFields := 0
	for field, rawValue := range record.Data {
		value, ok := rawValue.(float64)
		if !ok {
			continue
		}
		numericFields++
		metricKey := "klavierhaus." + record.DatasetKey + "." + field
		if err := a.start22EnsureImpactMetric(ctx, metricKey, start22MetricLabel(record.DatasetKey, field), start22MetricUnit(field), record.Aggregation); err != nil {
			return false, err
		}
		metricPayload := map[string]any{
			"idempotency_key": "start22:" + record.PartnerID + ":" + record.DatasetKey + ":" + record.IdempotencyKey + ":" + field,
			"partner_id": record.PartnerID,
			"metric_key": metricKey,
			"period_start": record.PeriodStart.Format("2006-01-02"),
			"period_end": record.PeriodEnd.Format("2006-01-02"),
			"numeric_value": value,
			"provenance": "PARTNER_DECLARED",
			"source_ref": "connector:data_record:" + strconv.FormatInt(record.ID, 10),
			"retention_policy": "HIMATE_7Y",
			"retain_until": record.RetainUntil.Format(time.RFC3339),
			"metadata": map[string]any{
				"source_system": start22SourceSystem,
				"source_version": record.SourceVersion,
				"module_key": record.ModuleKey,
				"dataset_key": record.DatasetKey,
				"batch_id": record.BatchID,
				"source_checksum_sha512": record.SourceChecksum,
				"retention_policy": "HIMATE_7Y",
				"retain_until": record.RetainUntil.Format(time.RFC3339),
			},
		}
		raw, _ := json.Marshal(metricPayload)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+a.impactHost+"/internal/v1/impact/ingest", bytes.NewReader(raw))
		if err != nil {
			return false, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Himate-Internal-Token", a.internalToken)
		req.Header.Set("X-Himate-User-ID", "connector:start22")
		resp, err := a.client.Do(req)
		if err != nil {
			return false, err
		}
		resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return false, fmt.Errorf("Impact ingestion returned %d for %s", resp.StatusCode, metricKey)
		}
	}
	return numericFields > 0, nil
}

func (a *app) start22Reconcile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use POST")
		return
	}
	credential, err := a.authenticate(r)
	if err != nil {
		common.APIError(w, http.StatusUnauthorized, "CONNECTOR_UNAUTHORIZED", err.Error())
		return
	}
	raw, ok := a.start22VerifySignedRequest(w, r, credential.CredentialID)
	if !ok {
		return
	}
	var in start22ReconcileInput
	if err := start22ReadJSON(raw, &in); err != nil {
		common.APIError(w, http.StatusBadRequest, "JSON", "Invalid reconciliation payload")
		return
	}
	in.ProtocolVersion = strings.TrimSpace(in.ProtocolVersion)
	in.SourceSystem = strings.ToUpper(strings.TrimSpace(in.SourceSystem))
	in.SourceVersion = strings.TrimSpace(in.SourceVersion)
	in.ReconciliationID = strings.TrimSpace(in.ReconciliationID)
	in.BatchID = strings.TrimSpace(in.BatchID)
	if in.ProtocolVersion != start22ProtocolVersion || in.SourceSystem != start22SourceSystem ||
		!start22IdentifierPattern.MatchString(in.ReconciliationID) || !start22IdentifierPattern.MatchString(in.BatchID) {
		common.APIError(w, http.StatusBadRequest, "VALIDATION", "Invalid reconciliation identity or protocol")
		return
	}
	if _, err := time.Parse(time.RFC3339, strings.TrimSpace(in.GeneratedAt)); err != nil {
		common.APIError(w, http.StatusBadRequest, "VALIDATION", "generated_at must be RFC3339")
		return
	}
	if len(in.Datasets) == 0 || len(in.Datasets) > len(start22DatasetRegistry) {
		common.APIError(w, http.StatusBadRequest, "VALIDATION", "Reconciliation datasets are required")
		return
	}

	details := make([]map[string]any, 0, len(in.Datasets))
	status := "SYNCED"
	for _, dataset := range in.Datasets {
		if _, exists := start22DatasetByKey[dataset.DatasetKey]; !exists || dataset.ItemCount < 0 || len(strings.TrimSpace(dataset.AggregateChecksum)) != 128 {
			common.APIError(w, http.StatusBadRequest, "VALIDATION", "Invalid reconciliation dataset")
			return
		}
		rows, err := a.db.QueryContext(r.Context(), `SELECT source_checksum FROM connector.data_records
			WHERE partner_id=$1 AND environment=$2 AND batch_id=$3 AND dataset_key=$4 ORDER BY source_checksum`,
			credential.PartnerID, credential.Environment, in.BatchID, dataset.DatasetKey)
		if err != nil {
			common.APIError(w, http.StatusInternalServerError, "DB", "Could not reconcile connector dataset")
			return
		}
		checksums := []string{}
		for rows.Next() {
			var checksum string
			if rows.Scan(&checksum) == nil {
				checksums = append(checksums, checksum)
			}
		}
		rows.Close()
		actualChecksum := start22AggregateChecksum(checksums)
		match := len(checksums) == dataset.ItemCount && start22ConstantHexEqual(actualChecksum, dataset.AggregateChecksum)
		if !match {
			status = "OUT_OF_SYNC"
		}
		details = append(details, map[string]any{
			"dataset_key": dataset.DatasetKey, "expected_count": dataset.ItemCount, "actual_count": len(checksums),
			"expected_checksum": dataset.AggregateChecksum, "actual_checksum": actualChecksum, "match": match,
		})
	}
	detailsRaw, _ := json.Marshal(details)
	_, err = a.db.ExecContext(r.Context(), `INSERT INTO connector.reconciliations(
		partner_id,environment,reconciliation_id,batch_id,source_version,status,details)
		VALUES($1,$2,$3,$4,$5,$6,$7::jsonb)
		ON CONFLICT(partner_id,environment,reconciliation_id) DO UPDATE SET
			status=EXCLUDED.status,details=EXCLUDED.details,checked_at=NOW()`,
		credential.PartnerID, credential.Environment, in.ReconciliationID, in.BatchID, in.SourceVersion, status, string(detailsRaw))
	if err != nil {
		common.APIError(w, http.StatusInternalServerError, "DB", "Could not persist connector reconciliation")
		return
	}
	_, _ = a.db.ExecContext(r.Context(), `UPDATE connector.partner_state SET last_reconciliation_at=NOW(),sync_status=$3,
		protocol_version=$4,updated_at=NOW() WHERE partner_id=$1 AND environment=$2`,
		credential.PartnerID, credential.Environment, status, in.ProtocolVersion)
	code := http.StatusOK
	if status == "OUT_OF_SYNC" {
		code = http.StatusConflict
	}
	common.JSON(w, code, map[string]any{
		"status": status, "partner_id": credential.PartnerID, "environment": credential.Environment,
		"reconciliation_id": in.ReconciliationID, "batch_id": in.BatchID, "datasets": details,
	})
}
