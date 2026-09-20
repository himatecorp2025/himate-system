package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"himate.local/services/internal/common"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type app struct{ db *sql.DB }

var metricKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_.]{2,127}$`)
var provenanceValues = map[string]bool{
	"SYSTEM": true, "MANUAL": true, "PARTNER_DECLARED": true, "VERIFIED_DOCUMENT": true,
}
var aggregationValues = map[string]bool{"SUM": true, "LATEST": true, "AVERAGE": true}
var scopeValues = map[string]bool{"GLOBAL": true, "PARTNER": true, "BOTH": true}

func adminProvenanceAllowed(v string) bool {
	return strings.ToUpper(strings.TrimSpace(v)) == "MANUAL"
}

func connectorProvenanceAllowed(v string) bool {
	v = strings.ToUpper(strings.TrimSpace(v))
	return v == "SYSTEM" || v == "PARTNER_DECLARED"
}

func main() {
	log := common.Logger()
	db, err := common.OpenDB()
	if err != nil {
		log.Error("database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	a := &app{db: db}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := a.migrate(ctx); err != nil {
		log.Error("migration", "error", err)
		os.Exit(1)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		common.JSON(w, 200, map[string]any{"status": "ok", "service": "impact", "time": time.Now().UTC()})
	})
	mux.HandleFunc("/api/v1/impact/definitions", a.definitions)
	mux.HandleFunc("/api/v1/impact/values", a.values)
	mux.HandleFunc("/api/v1/impact/summary", a.summary)
	mux.HandleFunc("/internal/v1/impact/ingest", a.ingest)
	mux.HandleFunc("/internal/v1/impact/summary", a.summary)
	common.Run(log, "impact", common.Env("PORT", "10000"), common.InternalAuth(os.Getenv("HIMATE_INTERNAL_TOKEN"), mux))
}

func (a *app) migrate(ctx context.Context) error {
	return common.ApplyMigrations(ctx, a.db, "impact", []common.Migration{
		{Version: 1, Name: "impact-metrics", Statements: []string{
			`CREATE SCHEMA IF NOT EXISTS impact`,
			`CREATE TABLE IF NOT EXISTS impact.metric_definitions(
				metric_key TEXT PRIMARY KEY,
				label TEXT NOT NULL,
				description TEXT NOT NULL DEFAULT '',
				unit TEXT NOT NULL DEFAULT 'count',
				aggregation TEXT NOT NULL DEFAULT 'SUM',
				scope TEXT NOT NULL DEFAULT 'PARTNER',
				active BOOLEAN NOT NULL DEFAULT TRUE,
				system BOOLEAN NOT NULL DEFAULT FALSE,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE TABLE IF NOT EXISTS impact.metric_values(
				id BIGSERIAL PRIMARY KEY,
				idempotency_key TEXT UNIQUE,
				payload_hash TEXT NOT NULL DEFAULT '',
				partner_id TEXT NOT NULL DEFAULT '',
				metric_key TEXT NOT NULL REFERENCES impact.metric_definitions(metric_key),
				period_start DATE NOT NULL,
				period_end DATE NOT NULL,
				numeric_value NUMERIC(18,4),
				text_value TEXT NOT NULL DEFAULT '',
				provenance TEXT NOT NULL,
				source_ref TEXT NOT NULL DEFAULT '',
				recorded_by TEXT NOT NULL DEFAULT '',
				recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
				CHECK(period_end >= period_start)
			)`,
			`CREATE INDEX IF NOT EXISTS impact_values_partner_metric_idx ON impact.metric_values(partner_id,metric_key,period_end DESC)`,
			`CREATE INDEX IF NOT EXISTS impact_values_period_idx ON impact.metric_values(period_start,period_end)`,
		}},
	})
}

func (a *app) definitions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query(`SELECT metric_key,label,description,unit,aggregation,scope,active,system,created_at,updated_at FROM impact.metric_definitions ORDER BY label`)
		if err != nil { common.APIError(w,500,"DB","Could not load metric definitions"); return }
		defer rows.Close()
		items:=[]map[string]any{}
		for rows.Next() {
			var key,label,desc,unit,agg,scope string
			var active,system bool
			var created,updated time.Time
			if rows.Scan(&key,&label,&desc,&unit,&agg,&scope,&active,&system,&created,&updated)==nil {
				items=append(items,map[string]any{"metric_key":key,"label":label,"description":desc,"unit":unit,"aggregation":agg,"scope":scope,"active":active,"system":system,"created_at":created,"updated_at":updated})
			}
		}
		common.JSON(w,200,map[string]any{"items":items,"count":len(items)})
	case http.MethodPost:
		var in struct {
			MetricKey string `json:"metric_key"`
			Label string `json:"label"`
			Description string `json:"description"`
			Unit string `json:"unit"`
			Aggregation string `json:"aggregation"`
			Scope string `json:"scope"`
		}
		if common.Decode(r,&in)!=nil { common.APIError(w,400,"JSON","Invalid request"); return }
		in.MetricKey=strings.TrimSpace(in.MetricKey)
		in.Label=strings.TrimSpace(in.Label)
		in.Aggregation=strings.ToUpper(strings.TrimSpace(in.Aggregation))
		in.Scope=strings.ToUpper(strings.TrimSpace(in.Scope))
		if in.Unit=="" { in.Unit="count" }
		if in.Aggregation=="" { in.Aggregation="SUM" }
		if in.Scope=="" { in.Scope="PARTNER" }
		if !metricKeyPattern.MatchString(in.MetricKey) || in.Label=="" || !aggregationValues[in.Aggregation] || !scopeValues[in.Scope] {
			common.APIError(w,400,"VALIDATION","Invalid metric definition")
			return
		}
		_,err:=a.db.Exec(`INSERT INTO impact.metric_definitions(metric_key,label,description,unit,aggregation,scope,system)
			VALUES($1,$2,$3,$4,$5,$6,FALSE)`,in.MetricKey,in.Label,strings.TrimSpace(in.Description),strings.TrimSpace(in.Unit),in.Aggregation,in.Scope)
		if err!=nil { common.APIError(w,409,"CONFLICT","Metric key already exists"); return }
		common.JSON(w,201,map[string]any{"metric_key":in.MetricKey,"label":in.Label,"unit":in.Unit,"aggregation":in.Aggregation,"scope":in.Scope,"active":true,"system":false})
	default:
		common.APIError(w,405,"METHOD","Use GET or POST")
	}
}

type metricInput struct {
	IdempotencyKey string `json:"idempotency_key"`
	PartnerID string `json:"partner_id"`
	MetricKey string `json:"metric_key"`
	PeriodStart string `json:"period_start"`
	PeriodEnd string `json:"period_end"`
	NumericValue *float64 `json:"numeric_value"`
	TextValue string `json:"text_value"`
	Provenance string `json:"provenance"`
	SourceRef string `json:"source_ref"`
	Metadata map[string]any `json:"metadata"`
}

func parseMetricInput(in metricInput)(metricInput,time.Time,time.Time,error){
	in.MetricKey=strings.TrimSpace(in.MetricKey)
	in.Provenance=strings.ToUpper(strings.TrimSpace(in.Provenance))
	if in.Provenance=="" { in.Provenance="MANUAL" }
	if !metricKeyPattern.MatchString(in.MetricKey) || !provenanceValues[in.Provenance] {
		return in,time.Time{},time.Time{},fmt.Errorf("invalid metric_key or provenance")
	}
	start,err:=time.Parse("2006-01-02",strings.TrimSpace(in.PeriodStart))
	if err!=nil { return in,time.Time{},time.Time{},fmt.Errorf("period_start must be YYYY-MM-DD") }
	end,err:=time.Parse("2006-01-02",strings.TrimSpace(in.PeriodEnd))
	if err!=nil || end.Before(start) { return in,time.Time{},time.Time{},fmt.Errorf("invalid period_end") }
	if in.NumericValue==nil && strings.TrimSpace(in.TextValue)=="" { return in,time.Time{},time.Time{},fmt.Errorf("metric value is required") }
	return in,start,end,nil
}

func (a *app) values(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		partnerID:=strings.TrimSpace(r.URL.Query().Get("partner_id"))
		metricKey:=strings.TrimSpace(r.URL.Query().Get("metric_key"))
		limit:=100
		if raw:=r.URL.Query().Get("limit");raw!="" {
			if v,err:=strconv.Atoi(raw);err==nil && v>0 && v<=500 { limit=v }
		}
		where:=[]string{"1=1"}
		args:=[]any{}
		if partnerID!="" { args=append(args,partnerID); where=append(where,fmt.Sprintf("v.partner_id=$%d",len(args))) }
		if metricKey!="" { args=append(args,metricKey); where=append(where,fmt.Sprintf("v.metric_key=$%d",len(args))) }
		args=append(args,limit)
		q:=`SELECT v.id,v.partner_id,v.metric_key,d.label,d.unit,v.period_start,v.period_end,v.numeric_value,v.text_value,v.provenance,v.source_ref,v.recorded_by,v.recorded_at,v.metadata
			FROM impact.metric_values v JOIN impact.metric_definitions d ON d.metric_key=v.metric_key WHERE `+strings.Join(where," AND ")+`
			ORDER BY v.period_end DESC,v.id DESC LIMIT $`+strconv.Itoa(len(args))
		rows,err:=a.db.Query(q,args...)
		if err!=nil { common.APIError(w,500,"DB","Could not load metric values"); return }
		defer rows.Close()
		items:=[]map[string]any{}
		for rows.Next() {
			var id int64
			var partner,key,label,unit,textValue,prov,source,recordedBy string
			var start,end,recorded time.Time
			var numeric sql.NullFloat64
			var meta []byte
			if rows.Scan(&id,&partner,&key,&label,&unit,&start,&end,&numeric,&textValue,&prov,&source,&recordedBy,&recorded,&meta)==nil {
				var num any
				if numeric.Valid { num=numeric.Float64 }
				items=append(items,map[string]any{"id":id,"partner_id":partner,"metric_key":key,"label":label,"unit":unit,"period_start":start.Format("2006-01-02"),"period_end":end.Format("2006-01-02"),"numeric_value":num,"text_value":textValue,"provenance":prov,"source_ref":source,"recorded_by":recordedBy,"recorded_at":recorded,"metadata":common.JSONRawOrEmpty(meta)})
			}
		}
		common.JSON(w,200,map[string]any{"items":items,"count":len(items)})
	case http.MethodPost:
		var in metricInput
		if common.Decode(r,&in)!=nil { common.APIError(w,400,"JSON","Invalid request"); return }
		if in.Provenance=="" { in.Provenance="MANUAL" }
		if !adminProvenanceAllowed(in.Provenance) {
			common.APIError(w,400,"PROVENANCE_BOUNDARY","Administrator-entered values must use MANUAL provenance")
			return
		}
		a.recordValue(w,r,in)
	default:
		common.APIError(w,405,"METHOD","Use GET or POST")
	}
}

func (a *app) ingest(w http.ResponseWriter, r *http.Request) {
	if r.Method!=http.MethodPost { common.APIError(w,405,"METHOD","Use POST"); return }
	var in metricInput
	if common.Decode(r,&in)!=nil { common.APIError(w,400,"JSON","Invalid request"); return }
	if in.Provenance=="" { in.Provenance="PARTNER_DECLARED" }
	provenance:=strings.ToUpper(strings.TrimSpace(in.Provenance))
	if !connectorProvenanceAllowed(provenance) {
		common.APIError(w,400,"PROVENANCE_BOUNDARY","Connector ingestion may use only SYSTEM or PARTNER_DECLARED provenance")
		return
	}
	in.Provenance=provenance
	a.recordValue(w,r,in)
}

func metricPayloadHash(in metricInput,start,end time.Time) string {
	payload:=struct{
		PartnerID string `json:"partner_id"`
		MetricKey string `json:"metric_key"`
		PeriodStart string `json:"period_start"`
		PeriodEnd string `json:"period_end"`
		NumericValue *float64 `json:"numeric_value"`
		TextValue string `json:"text_value"`
		Provenance string `json:"provenance"`
		SourceRef string `json:"source_ref"`
		Metadata map[string]any `json:"metadata"`
	}{
		PartnerID:strings.TrimSpace(in.PartnerID),MetricKey:in.MetricKey,
		PeriodStart:start.Format("2006-01-02"),PeriodEnd:end.Format("2006-01-02"),
		NumericValue:in.NumericValue,TextValue:strings.TrimSpace(in.TextValue),
		Provenance:in.Provenance,SourceRef:strings.TrimSpace(in.SourceRef),Metadata:in.Metadata,
	}
	raw,_:=json.Marshal(payload)
	sum:=sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func (a *app) recordValue(w http.ResponseWriter,r *http.Request,in metricInput){
	in,start,end,err:=parseMetricInput(in)
	if err!=nil { common.APIError(w,400,"VALIDATION",err.Error()); return }
	var exists bool
	if err=a.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM impact.metric_definitions WHERE metric_key=$1 AND active=TRUE)`,in.MetricKey).Scan(&exists);err!=nil || !exists {
		common.APIError(w,409,"UNKNOWN_METRIC","Metric definition is not active")
		return
	}
	meta,_:=common.MarshalJSON(in.Metadata)
	recordedBy:=strings.TrimSpace(r.Header.Get("X-Himate-User-ID"))
	if recordedBy=="" { recordedBy="connector" }
	idempotencyKey:=strings.TrimSpace(in.IdempotencyKey)
	payloadHash:=metricPayloadHash(in,start,end)
	var id int64
	err=a.db.QueryRow(`INSERT INTO impact.metric_values(idempotency_key,payload_hash,partner_id,metric_key,period_start,period_end,numeric_value,text_value,provenance,source_ref,recorded_by,metadata)
		VALUES(NULLIF($1,''),$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12::jsonb)
		ON CONFLICT(idempotency_key) DO NOTHING
		RETURNING id`,
		idempotencyKey,payloadHash,strings.TrimSpace(in.PartnerID),in.MetricKey,start,end,in.NumericValue,strings.TrimSpace(in.TextValue),in.Provenance,strings.TrimSpace(in.SourceRef),recordedBy,string(meta)).Scan(&id)
	if err==sql.ErrNoRows && idempotencyKey!="" {
		var existingID int64
		var existingHash string
		if lookupErr:=a.db.QueryRow(`SELECT id,payload_hash FROM impact.metric_values WHERE idempotency_key=$1`,idempotencyKey).Scan(&existingID,&existingHash);lookupErr!=nil {
			common.APIError(w,500,"DB","Could not resolve idempotent metric")
			return
		}
		if existingHash!=payloadHash {
			common.APIError(w,409,"IDEMPOTENCY_CONFLICT","idempotency_key was already used for different metric data")
			return
		}
		common.JSON(w,200,map[string]any{"id":existingID,"partner_id":strings.TrimSpace(in.PartnerID),"metric_key":in.MetricKey,"duplicate":true,"provenance":in.Provenance})
		return
	}
	if err!=nil { common.APIError(w,500,"DB","Could not record metric"); return }
	common.JSON(w,201,map[string]any{"id":id,"partner_id":strings.TrimSpace(in.PartnerID),"metric_key":in.MetricKey,"period_start":start.Format("2006-01-02"),"period_end":end.Format("2006-01-02"),"provenance":in.Provenance})
}

func (a *app) summary(w http.ResponseWriter, r *http.Request) {
	if r.Method!=http.MethodGet { common.APIError(w,405,"METHOD","Use GET"); return }
	partnerID:=strings.TrimSpace(r.URL.Query().Get("partner_id"))
	args:=[]any{}
	where:=""
	if partnerID!="" { where="WHERE v.partner_id=$1"; args=append(args,partnerID) }
	rows,err:=a.db.Query(`SELECT v.metric_key,d.label,d.unit,d.aggregation,
		CASE d.aggregation
			WHEN 'LATEST' THEN (ARRAY_AGG(v.numeric_value ORDER BY v.period_end DESC,v.id DESC))[1]
			WHEN 'AVERAGE' THEN AVG(v.numeric_value)
			ELSE SUM(v.numeric_value)
		END AS numeric_value,
		MAX(v.period_end) AS latest_period_end,
		COUNT(*) AS observations
		FROM impact.metric_values v JOIN impact.metric_definitions d ON d.metric_key=v.metric_key `+where+`
		GROUP BY v.metric_key,d.label,d.unit,d.aggregation ORDER BY d.label`,args...)
	if err!=nil { common.APIError(w,500,"DB","Could not calculate impact summary"); return }
	defer rows.Close()
	items:=[]map[string]any{}
	for rows.Next() {
		var key,label,unit,agg string
		var num sql.NullFloat64
		var end time.Time
		var count int
		if rows.Scan(&key,&label,&unit,&agg,&num,&end,&count)==nil {
			var value any
			if num.Valid { value=num.Float64 }
			items=append(items,map[string]any{"metric_key":key,"label":label,"unit":unit,"aggregation":agg,"numeric_value":value,"latest_period_end":end.Format("2006-01-02"),"observations":count})
		}
	}
	common.JSON(w,200,map[string]any{"partner_id":partnerID,"items":items,"count":len(items)})
}
