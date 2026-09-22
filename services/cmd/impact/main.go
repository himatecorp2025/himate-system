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
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type app struct {
	db *sql.DB
	token string
	evidenceHost string
	client *http.Client
}

var metricKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_.]{2,127}$`)
var provenanceValues = map[string]bool{
	"SYSTEM": true, "MANUAL": true, "PARTNER_DECLARED": true, "VERIFIED_DOCUMENT": true,
}
var aggregationValues = map[string]bool{"SUM": true, "LATEST": true, "AVERAGE": true}
var scopeValues = map[string]bool{"GLOBAL": true, "PARTNER": true, "BOTH": true}

func adminProvenanceAllowed(v string) bool {
	v = strings.ToUpper(strings.TrimSpace(v))
	return v == "MANUAL" || v == "VERIFIED_DOCUMENT"
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
	a := &app{
		db: db,
		token: os.Getenv("HIMATE_INTERNAL_TOKEN"),
		evidenceHost: os.Getenv("EVIDENCE_HOSTPORT"),
		client: &http.Client{Timeout: 5 * time.Second},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := a.migrate(ctx); err != nil {
		log.Error("migration", "error", err)
		os.Exit(1)
	}
	go a.start22RetentionLoop()
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		common.JSON(w, 200, map[string]any{"status": "ok", "service": "impact", "time": time.Now().UTC()})
	})
	mux.HandleFunc("/api/v1/impact/definitions", a.definitions)
	mux.HandleFunc("/api/v1/impact/values", a.values)
	mux.HandleFunc("/api/v1/impact/baselines", a.baselines)
	mux.HandleFunc("/api/v1/impact/summary", a.summary)
	mux.HandleFunc("/internal/v1/impact/definitions/ensure", a.ensureSystemDefinition)
	mux.HandleFunc("/internal/v1/impact/retention", a.connectorRetention)
	mux.HandleFunc("/internal/v1/impact/ingest", a.ingest)
	mux.HandleFunc("/internal/v1/impact/summary", a.summary)
	mux.HandleFunc("/internal/v1/impact/dashboard", a.dashboardImpact)
	common.Run(log, "impact", common.Env("PORT", "10000"), common.InternalAuth(os.Getenv("HIMATE_INTERNAL_TOKEN"), mux))
}

func (a *app) start22RetentionLoop() {
	run:=func(){
		ctx,cancel:=context.WithTimeout(context.Background(),30*time.Second)
		defer cancel()
		_,_=a.db.ExecContext(ctx,`DELETE FROM impact.metric_values
			WHERE retention_policy='HIMATE_7Y' AND retain_until<=NOW() AND legal_hold=FALSE`)
	}
	run()
	ticker:=time.NewTicker(24*time.Hour)
	defer ticker.Stop()
	for range ticker.C { run() }
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
		{Version: 2, Name: "impact-baselines", Statements: []string{
			`CREATE TABLE IF NOT EXISTS impact.metric_baselines(
				partner_id TEXT NOT NULL DEFAULT '',
				metric_key TEXT NOT NULL REFERENCES impact.metric_definitions(metric_key),
				period_start DATE NOT NULL,
				period_end DATE NOT NULL,
				numeric_value NUMERIC(18,4),
				text_value TEXT NOT NULL DEFAULT '',
				provenance TEXT NOT NULL DEFAULT 'MANUAL',
				source_ref TEXT NOT NULL DEFAULT '',
				recorded_by TEXT NOT NULL DEFAULT '',
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				PRIMARY KEY(partner_id,metric_key),
				CHECK(period_end >= period_start)
			)`,
		}},
		{Version: 3, Name: "verified-evidence-links", Statements: []string{
			`ALTER TABLE impact.metric_values ADD COLUMN IF NOT EXISTS evidence_id TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE impact.metric_baselines ADD COLUMN IF NOT EXISTS evidence_id TEXT NOT NULL DEFAULT ''`,
			`CREATE INDEX IF NOT EXISTS impact_values_evidence_idx ON impact.metric_values(evidence_id) WHERE evidence_id<>''`,
		}},
		{Version: 4, Name: "start-22-connector-retention", Statements: []string{
			`ALTER TABLE impact.metric_values ADD COLUMN IF NOT EXISTS retention_policy TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE impact.metric_values ADD COLUMN IF NOT EXISTS retain_until TIMESTAMPTZ`,
			`ALTER TABLE impact.metric_values ADD COLUMN IF NOT EXISTS legal_hold BOOLEAN NOT NULL DEFAULT FALSE`,
			`ALTER TABLE impact.metric_values ADD COLUMN IF NOT EXISTS privacy_delete_requested BOOLEAN NOT NULL DEFAULT FALSE`,
			`CREATE INDEX IF NOT EXISTS impact_values_retention_idx ON impact.metric_values(retain_until) WHERE retain_until IS NOT NULL AND legal_hold=FALSE`,
			`CREATE INDEX IF NOT EXISTS impact_values_source_ref_idx ON impact.metric_values(source_ref) WHERE source_ref<>''`,
		}},
		{Version: 5, Name: "start-23-5-bilingual-impact-definitions", Statements: []string{
			`ALTER TABLE impact.metric_definitions ADD COLUMN IF NOT EXISTS label_en TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE impact.metric_definitions ADD COLUMN IF NOT EXISTS label_hu TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE impact.metric_definitions ADD COLUMN IF NOT EXISTS description_en TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE impact.metric_definitions ADD COLUMN IF NOT EXISTS description_hu TEXT NOT NULL DEFAULT ''`,
			`UPDATE impact.metric_definitions SET label_en=label WHERE label_en=''`,
			`UPDATE impact.metric_definitions SET label_hu=label WHERE label_hu=''`,
			`UPDATE impact.metric_definitions SET description_en=description WHERE description_en=''`,
			`UPDATE impact.metric_definitions SET description_hu=description WHERE description_hu=''`,
		}},
	})
}

func (a *app) ensureSystemDefinition(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use POST")
		return
	}
	var in struct {
		MetricKey   string `json:"metric_key"`
		Label       string `json:"label"`
		LabelEN     string `json:"label_en"`
		LabelHU     string `json:"label_hu"`
		Description string `json:"description"`
		DescriptionEN string `json:"description_en"`
		DescriptionHU string `json:"description_hu"`
		Unit        string `json:"unit"`
		Aggregation string `json:"aggregation"`
		Scope       string `json:"scope"`
	}
	if common.Decode(r, &in) != nil {
		common.APIError(w, http.StatusBadRequest, "JSON", "Invalid metric definition")
		return
	}
	in.MetricKey = strings.TrimSpace(in.MetricKey)
	in.Label = strings.TrimSpace(in.Label)
	in.LabelEN = strings.TrimSpace(in.LabelEN)
	in.LabelHU = strings.TrimSpace(in.LabelHU)
	in.Description = strings.TrimSpace(in.Description)
	in.DescriptionEN = strings.TrimSpace(in.DescriptionEN)
	in.DescriptionHU = strings.TrimSpace(in.DescriptionHU)
	if in.LabelEN=="" { in.LabelEN=in.Label }
	if in.LabelHU=="" { in.LabelHU=in.Label }
	if in.DescriptionEN=="" { in.DescriptionEN=in.Description }
	if in.DescriptionHU=="" { in.DescriptionHU=in.Description }
	in.Unit = strings.TrimSpace(in.Unit)
	in.Aggregation = strings.ToUpper(strings.TrimSpace(in.Aggregation))
	in.Scope = strings.ToUpper(strings.TrimSpace(in.Scope))
	if in.Unit == "" { in.Unit = "count" }
	if in.Aggregation == "" { in.Aggregation = "LATEST" }
	if in.Scope == "" { in.Scope = "PARTNER" }
	if !metricKeyPattern.MatchString(in.MetricKey) || in.LabelEN == "" || in.LabelHU == "" ||
		!aggregationValues[in.Aggregation] || !scopeValues[in.Scope] {
		common.APIError(w, http.StatusBadRequest, "VALIDATION", "Invalid system metric definition")
		return
	}
	_, err := a.db.Exec(`INSERT INTO impact.metric_definitions(metric_key,label,label_en,label_hu,description,description_en,description_hu,unit,aggregation,scope,active,system)
		VALUES($1,$2,$2,$3,$4,$4,$5,$6,$7,$8,TRUE,TRUE)
		ON CONFLICT(metric_key) DO UPDATE SET
			label=EXCLUDED.label,label_en=EXCLUDED.label_en,label_hu=EXCLUDED.label_hu,
			description=EXCLUDED.description,description_en=EXCLUDED.description_en,description_hu=EXCLUDED.description_hu,
			unit=EXCLUDED.unit,aggregation=EXCLUDED.aggregation,scope=EXCLUDED.scope,active=TRUE,system=TRUE,updated_at=NOW()`,
		in.MetricKey,in.LabelEN,in.LabelHU,in.DescriptionEN,in.DescriptionHU,in.Unit,in.Aggregation,in.Scope)
	if err != nil {
		common.APIError(w, http.StatusInternalServerError, "DB", "Could not ensure system metric definition")
		return
	}
	common.JSON(w,http.StatusOK,map[string]any{
		"metric_key":in.MetricKey,"label":common.Localized(in.LabelEN,in.LabelHU,common.RequestLocale(r)),"label_en":in.LabelEN,"label_hu":in.LabelHU,"description_en":in.DescriptionEN,"description_hu":in.DescriptionHU,"unit":in.Unit,
		"aggregation":in.Aggregation,"scope":in.Scope,"active":true,"system":true,
	})
}

func (a *app) definitions(w http.ResponseWriter, r *http.Request) {
	locale:=common.RequestLocale(r)
	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query(`SELECT metric_key,label_en,label_hu,description_en,description_hu,unit,aggregation,scope,active,system,created_at,updated_at FROM impact.metric_definitions ORDER BY lower(label_en),metric_key`)
		if err != nil { common.APIError(w,500,"DB","Could not load metric definitions"); return }
		defer rows.Close()
		items:=[]map[string]any{}
		for rows.Next() {
			var key,labelEN,labelHU,descEN,descHU,unit,agg,scope string
			var active,system bool
			var created,updated time.Time
			if rows.Scan(&key,&labelEN,&labelHU,&descEN,&descHU,&unit,&agg,&scope,&active,&system,&created,&updated)==nil {
				items=append(items,map[string]any{
					"metric_key":key,"label":common.Localized(labelEN,labelHU,locale),"label_en":labelEN,"label_hu":labelHU,
					"description":common.Localized(descEN,descHU,locale),"description_en":descEN,"description_hu":descHU,
					"unit":unit,"aggregation":agg,"scope":scope,"active":active,"system":system,"created_at":created,"updated_at":updated,
				})
			}
		}
		common.JSON(w,200,map[string]any{"items":items,"count":len(items),"locale":locale})
	case http.MethodPost:
		var in struct {
			MetricKey string `json:"metric_key"`
			Label string `json:"label"`
			LabelEN string `json:"label_en"`
			LabelHU string `json:"label_hu"`
			Description string `json:"description"`
			DescriptionEN string `json:"description_en"`
			DescriptionHU string `json:"description_hu"`
			Unit string `json:"unit"`
			Aggregation string `json:"aggregation"`
			Scope string `json:"scope"`
		}
		if common.Decode(r,&in)!=nil { common.APIError(w,400,"JSON","Invalid request"); return }
		in.MetricKey=strings.TrimSpace(in.MetricKey)
		labelEN:=strings.TrimSpace(in.LabelEN); labelHU:=strings.TrimSpace(in.LabelHU); legacyLabel:=strings.TrimSpace(in.Label)
		if labelEN==""{labelEN=legacyLabel}; if labelHU==""{labelHU=legacyLabel}
		descEN:=strings.TrimSpace(in.DescriptionEN); descHU:=strings.TrimSpace(in.DescriptionHU); legacyDesc:=strings.TrimSpace(in.Description)
		if descEN==""{descEN=legacyDesc}; if descHU==""{descHU=legacyDesc}
		in.Aggregation=strings.ToUpper(strings.TrimSpace(in.Aggregation))
		in.Scope=strings.ToUpper(strings.TrimSpace(in.Scope))
		if in.Unit=="" { in.Unit="count" }
		if in.Aggregation=="" { in.Aggregation="SUM" }
		if in.Scope=="" { in.Scope="PARTNER" }
		if !metricKeyPattern.MatchString(in.MetricKey) || labelEN=="" || labelHU=="" || !aggregationValues[in.Aggregation] || !scopeValues[in.Scope] {
			common.APIError(w,400,"VALIDATION","Invalid bilingual metric definition")
			return
		}
		_,err:=a.db.Exec(`INSERT INTO impact.metric_definitions(metric_key,label,label_en,label_hu,description,description_en,description_hu,unit,aggregation,scope,system)
			VALUES($1,$2,$2,$3,$4,$4,$5,$6,$7,$8,FALSE)`,in.MetricKey,labelEN,labelHU,descEN,descHU,strings.TrimSpace(in.Unit),in.Aggregation,in.Scope)
		if err!=nil { common.APIError(w,409,"CONFLICT","Metric key already exists"); return }
		common.JSON(w,201,map[string]any{
			"metric_key":in.MetricKey,"label":common.Localized(labelEN,labelHU,locale),"label_en":labelEN,"label_hu":labelHU,
			"description":common.Localized(descEN,descHU,locale),"description_en":descEN,"description_hu":descHU,
			"unit":strings.TrimSpace(in.Unit),"aggregation":in.Aggregation,"scope":in.Scope,"active":true,"system":false,
		})
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
	EvidenceID string `json:"evidence_id"`
	RetentionPolicy string `json:"retention_policy"`
	RetainUntil string `json:"retain_until"`
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
		periodStart:=strings.TrimSpace(r.URL.Query().Get("period_start"))
		periodEnd:=strings.TrimSpace(r.URL.Query().Get("period_end"))
		limit:=100
		if raw:=r.URL.Query().Get("limit");raw!="" {
			if v,err:=strconv.Atoi(raw);err==nil && v>0 && v<=500 { limit=v }
		}
		where:=[]string{"1=1"}
		args:=[]any{}
		if partnerID!="" { args=append(args,partnerID); where=append(where,fmt.Sprintf("v.partner_id=$%d",len(args))) }
		if metricKey!="" { args=append(args,metricKey); where=append(where,fmt.Sprintf("v.metric_key=$%d",len(args))) }
		if periodStart!="" {
			start,err:=time.Parse("2006-01-02",periodStart)
			if err!=nil { common.APIError(w,400,"VALIDATION","period_start must be YYYY-MM-DD"); return }
			args=append(args,start)
			where=append(where,fmt.Sprintf("v.period_end >= $%d",len(args)))
		}
		if periodEnd!="" {
			end,err:=time.Parse("2006-01-02",periodEnd)
			if err!=nil { common.APIError(w,400,"VALIDATION","period_end must be YYYY-MM-DD"); return }
			args=append(args,end)
			where=append(where,fmt.Sprintf("v.period_start <= $%d",len(args)))
		}
		args=append(args,limit)
		q:=`SELECT v.id,v.partner_id,v.metric_key,d.label,d.unit,v.period_start,v.period_end,v.numeric_value,v.text_value,v.provenance,v.source_ref,v.evidence_id,v.recorded_by,v.recorded_at,v.metadata
			FROM impact.metric_values v JOIN impact.metric_definitions d ON d.metric_key=v.metric_key WHERE `+strings.Join(where," AND ")+`
			ORDER BY v.period_end DESC,v.id DESC LIMIT $`+strconv.Itoa(len(args))
		rows,err:=a.db.Query(q,args...)
		if err!=nil { common.APIError(w,500,"DB","Could not load metric values"); return }
		defer rows.Close()
		items:=[]map[string]any{}
		for rows.Next() {
			var id int64
			var partner,key,label,unit,textValue,prov,source,evidenceID,recordedBy string
			var start,end,recorded time.Time
			var numeric sql.NullFloat64
			var meta []byte
			if rows.Scan(&id,&partner,&key,&label,&unit,&start,&end,&numeric,&textValue,&prov,&source,&evidenceID,&recordedBy,&recorded,&meta)==nil {
				var num any
				if numeric.Valid { num=numeric.Float64 }
				items=append(items,map[string]any{"id":id,"partner_id":partner,"metric_key":key,"label":label,"unit":unit,"period_start":start.Format("2006-01-02"),"period_end":end.Format("2006-01-02"),"numeric_value":num,"text_value":textValue,"provenance":prov,"source_ref":source,"evidence_id":evidenceID,"recorded_by":recordedBy,"recorded_at":recorded,"metadata":common.JSONRawOrEmpty(meta)})
			}
		}
		common.JSON(w,200,map[string]any{"items":items,"count":len(items)})
	case http.MethodPost:
		var in metricInput
		if common.Decode(r,&in)!=nil { common.APIError(w,400,"JSON","Invalid request"); return }
		if in.Provenance=="" { in.Provenance="MANUAL" }
		in.RetentionPolicy=""
		in.RetainUntil=""
		if !adminProvenanceAllowed(in.Provenance) {
			common.APIError(w,400,"PROVENANCE_BOUNDARY","Administrator-entered values may use only MANUAL or VERIFIED_DOCUMENT provenance")
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
		EvidenceID string `json:"evidence_id"`
		RetentionPolicy string `json:"retention_policy"`
		RetainUntil string `json:"retain_until"`
	}{
		PartnerID:strings.TrimSpace(in.PartnerID),MetricKey:in.MetricKey,
		PeriodStart:start.Format("2006-01-02"),PeriodEnd:end.Format("2006-01-02"),
		NumericValue:in.NumericValue,TextValue:strings.TrimSpace(in.TextValue),
		Provenance:in.Provenance,SourceRef:strings.TrimSpace(in.SourceRef),Metadata:in.Metadata,EvidenceID:strings.TrimSpace(in.EvidenceID),
		RetentionPolicy:strings.TrimSpace(in.RetentionPolicy),RetainUntil:strings.TrimSpace(in.RetainUntil),
	}
	raw,_:=json.Marshal(payload)
	sum:=sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func (a *app) recordValue(w http.ResponseWriter,r *http.Request,in metricInput){
	in,start,end,err:=parseMetricInput(in)
	if err!=nil { common.APIError(w,400,"VALIDATION",err.Error()); return }
	if in.Provenance=="VERIFIED_DOCUMENT" {
		in.EvidenceID=strings.TrimSpace(in.EvidenceID)
		if in.EvidenceID=="" { common.APIError(w,400,"EVIDENCE_REQUIRED","VERIFIED_DOCUMENT provenance requires evidence_id"); return }
		if err:=a.validateEvidence(r.Context(),in.EvidenceID,strings.TrimSpace(in.PartnerID),in.MetricKey);err!=nil {
			common.APIError(w,409,"EVIDENCE_INVALID",err.Error())
			return
		}
		in.SourceRef=in.EvidenceID
	} else if strings.TrimSpace(in.EvidenceID)!="" {
		common.APIError(w,400,"EVIDENCE_BOUNDARY","evidence_id may only be supplied with VERIFIED_DOCUMENT provenance")
		return
	}
	var exists bool
	if err=a.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM impact.metric_definitions WHERE metric_key=$1 AND active=TRUE)`,in.MetricKey).Scan(&exists);err!=nil || !exists {
		common.APIError(w,409,"UNKNOWN_METRIC","Metric definition is not active")
		return
	}
	meta,_:=common.MarshalJSON(in.Metadata)
	var retainUntil any
	in.RetentionPolicy=strings.TrimSpace(in.RetentionPolicy)
	in.RetainUntil=strings.TrimSpace(in.RetainUntil)
	if in.RetentionPolicy!="" || in.RetainUntil!="" {
		if in.RetentionPolicy!="HIMATE_7Y" || in.RetainUntil=="" {
			common.APIError(w,400,"RETENTION_POLICY","Connector retention must use HIMATE_7Y with retain_until")
			return
		}
		parsed,parseErr:=time.Parse(time.RFC3339,in.RetainUntil)
		if parseErr!=nil || !parsed.After(time.Now().UTC()) {
			common.APIError(w,400,"RETENTION_POLICY","retain_until must be a future RFC3339 timestamp")
			return
		}
		retainUntil=parsed.UTC()
	}
	recordedBy:=strings.TrimSpace(r.Header.Get("X-Himate-User-ID"))
	if recordedBy=="" { recordedBy="connector" }
	idempotencyKey:=strings.TrimSpace(in.IdempotencyKey)
	payloadHash:=metricPayloadHash(in,start,end)
	var id int64
	err=a.db.QueryRow(`INSERT INTO impact.metric_values(idempotency_key,payload_hash,partner_id,metric_key,period_start,period_end,numeric_value,text_value,provenance,source_ref,evidence_id,recorded_by,metadata,retention_policy,retain_until)
		VALUES(NULLIF($1,''),$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13::jsonb,$14,$15)
		ON CONFLICT(idempotency_key) DO NOTHING
		RETURNING id`,
		idempotencyKey,payloadHash,strings.TrimSpace(in.PartnerID),in.MetricKey,start,end,in.NumericValue,strings.TrimSpace(in.TextValue),in.Provenance,strings.TrimSpace(in.SourceRef),strings.TrimSpace(in.EvidenceID),recordedBy,string(meta),in.RetentionPolicy,retainUntil).Scan(&id)
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

func (a *app) validateEvidence(ctx context.Context,evidenceID,partnerID,metricKey string)error{
	if strings.TrimSpace(a.evidenceHost)=="" { return fmt.Errorf("evidence service is not configured") }
	q:=url.Values{}
	q.Set("evidence_id",evidenceID)
	q.Set("partner_id",partnerID)
	q.Set("metric_key",metricKey)
	req,err:=http.NewRequestWithContext(ctx,http.MethodGet,"http://"+a.evidenceHost+"/internal/v1/evidence/validate?"+q.Encode(),nil)
	if err!=nil{return err}
	req.Header.Set("X-Himate-Internal-Token",a.token)
	resp,err:=a.client.Do(req)
	if err!=nil{return fmt.Errorf("evidence validation: %w",err)}
	defer resp.Body.Close()
	if resp.StatusCode<200||resp.StatusCode>=300{return fmt.Errorf("evidence is not a verified document for this partner and metric")}
	return nil
}

func (a *app) connectorRetention(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodPost {
		common.APIError(w,http.StatusMethodNotAllowed,"METHOD","Use POST")
		return
	}
	var in struct{
		SourceRef string `json:"source_ref"`
		Action string `json:"action"`
		LegalHold *bool `json:"legal_hold"`
	}
	if common.Decode(r,&in)!=nil {
		common.APIError(w,400,"JSON","Invalid retention request")
		return
	}
	in.SourceRef=strings.TrimSpace(in.SourceRef)
	in.Action=strings.ToUpper(strings.TrimSpace(in.Action))
	if in.SourceRef=="" {
		common.APIError(w,400,"VALIDATION","source_ref is required")
		return
	}
	switch in.Action {
	case "SET_LEGAL_HOLD":
		if in.LegalHold==nil {
			common.APIError(w,400,"VALIDATION","legal_hold is required")
			return
		}
		result,err:=a.db.ExecContext(r.Context(),`UPDATE impact.metric_values SET legal_hold=$2 WHERE source_ref=$1 AND retention_policy='HIMATE_7Y'`,in.SourceRef,*in.LegalHold)
		if err!=nil { common.APIError(w,500,"DB","Could not update Impact legal hold");return }
		affected,_:=result.RowsAffected()
		common.JSON(w,200,map[string]any{"source_ref":in.SourceRef,"action":in.Action,"legal_hold":*in.LegalHold,"affected":affected})
	case "PRIVACY_DELETE":
		result,err:=a.db.ExecContext(r.Context(),`DELETE FROM impact.metric_values WHERE source_ref=$1 AND retention_policy='HIMATE_7Y' AND legal_hold=FALSE`,in.SourceRef)
		if err!=nil { common.APIError(w,500,"DB","Could not delete retained Impact observations");return }
		affected,_:=result.RowsAffected()
		common.JSON(w,200,map[string]any{"source_ref":in.SourceRef,"action":in.Action,"deleted":affected})
	case "PURGE_EXPIRED":
		result,err:=a.db.ExecContext(r.Context(),`DELETE FROM impact.metric_values WHERE retention_policy='HIMATE_7Y' AND retain_until<=NOW() AND legal_hold=FALSE`)
		if err!=nil { common.APIError(w,500,"DB","Could not purge expired Impact observations");return }
		affected,_:=result.RowsAffected()
		common.JSON(w,200,map[string]any{"action":in.Action,"deleted":affected})
	default:
		common.APIError(w,400,"VALIDATION","Unknown retention action")
	}
}

func (a *app) baselines(w http.ResponseWriter,r *http.Request){
	switch r.Method{
	case http.MethodGet:
		partnerID:=strings.TrimSpace(r.URL.Query().Get("partner_id"))
		metricKey:=strings.TrimSpace(r.URL.Query().Get("metric_key"))
		where:=[]string{"1=1"};args:=[]any{}
		if partnerID!=""{args=append(args,partnerID);where=append(where,fmt.Sprintf("b.partner_id=$%d",len(args)))}
		if metricKey!=""{args=append(args,metricKey);where=append(where,fmt.Sprintf("b.metric_key=$%d",len(args)))}
		rows,err:=a.db.Query(`SELECT b.partner_id,b.metric_key,d.label,d.unit,b.period_start,b.period_end,b.numeric_value,b.text_value,b.provenance,b.source_ref,b.evidence_id,b.recorded_by,b.updated_at
			FROM impact.metric_baselines b JOIN impact.metric_definitions d ON d.metric_key=b.metric_key WHERE `+strings.Join(where," AND ")+` ORDER BY d.label`,args...)
		if err!=nil{common.APIError(w,500,"DB","Could not load metric baselines");return}
		defer rows.Close();items:=[]map[string]any{}
		for rows.Next(){
			var partner,key,label,unit,textValue,prov,source,evidenceID,recordedBy string
			var start,end,updated time.Time;var numeric sql.NullFloat64
			if rows.Scan(&partner,&key,&label,&unit,&start,&end,&numeric,&textValue,&prov,&source,&evidenceID,&recordedBy,&updated)==nil{
				var num any;if numeric.Valid{num=numeric.Float64}
				items=append(items,map[string]any{"partner_id":partner,"metric_key":key,"label":label,"unit":unit,"period_start":start.Format("2006-01-02"),"period_end":end.Format("2006-01-02"),"numeric_value":num,"text_value":textValue,"provenance":prov,"source_ref":source,"evidence_id":evidenceID,"recorded_by":recordedBy,"updated_at":updated})
			}
		}
		common.JSON(w,200,map[string]any{"items":items,"count":len(items)})
	case http.MethodPut:
		var in metricInput
		if common.Decode(r,&in)!=nil{common.APIError(w,400,"JSON","Invalid request");return}
		if in.Provenance==""{in.Provenance="MANUAL"}
		if !adminProvenanceAllowed(in.Provenance){common.APIError(w,400,"PROVENANCE_BOUNDARY","Administrator-entered baselines may use only MANUAL or VERIFIED_DOCUMENT provenance");return}
		in,start,end,err:=parseMetricInput(in)
		if err!=nil{common.APIError(w,400,"VALIDATION",err.Error());return}
		if in.Provenance=="VERIFIED_DOCUMENT" {
			in.EvidenceID=strings.TrimSpace(in.EvidenceID)
			if in.EvidenceID==""{common.APIError(w,400,"EVIDENCE_REQUIRED","VERIFIED_DOCUMENT provenance requires evidence_id");return}
			if err:=a.validateEvidence(r.Context(),in.EvidenceID,strings.TrimSpace(in.PartnerID),in.MetricKey);err!=nil{common.APIError(w,409,"EVIDENCE_INVALID",err.Error());return}
			in.SourceRef=in.EvidenceID
		} else if strings.TrimSpace(in.EvidenceID)!="" {
			common.APIError(w,400,"EVIDENCE_BOUNDARY","evidence_id may only be supplied with VERIFIED_DOCUMENT provenance");return
		}
		var exists bool
		if err=a.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM impact.metric_definitions WHERE metric_key=$1 AND active=TRUE)`,in.MetricKey).Scan(&exists);err!=nil||!exists{
			common.APIError(w,409,"UNKNOWN_METRIC","Metric definition is not active");return
		}
		recordedBy:=strings.TrimSpace(r.Header.Get("X-Himate-User-ID"))
		_,err=a.db.Exec(`INSERT INTO impact.metric_baselines(partner_id,metric_key,period_start,period_end,numeric_value,text_value,provenance,source_ref,evidence_id,recorded_by)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
			ON CONFLICT(partner_id,metric_key) DO UPDATE SET period_start=EXCLUDED.period_start,period_end=EXCLUDED.period_end,
				numeric_value=EXCLUDED.numeric_value,text_value=EXCLUDED.text_value,provenance=EXCLUDED.provenance,source_ref=EXCLUDED.source_ref,evidence_id=EXCLUDED.evidence_id,recorded_by=EXCLUDED.recorded_by,updated_at=NOW()`,
			strings.TrimSpace(in.PartnerID),in.MetricKey,start,end,in.NumericValue,strings.TrimSpace(in.TextValue),in.Provenance,strings.TrimSpace(in.SourceRef),strings.TrimSpace(in.EvidenceID),recordedBy)
		if err!=nil{common.APIError(w,500,"DB","Could not save metric baseline");return}
		common.JSON(w,200,map[string]any{"partner_id":strings.TrimSpace(in.PartnerID),"metric_key":in.MetricKey,"period_start":start.Format("2006-01-02"),"period_end":end.Format("2006-01-02"),"numeric_value":in.NumericValue,"text_value":strings.TrimSpace(in.TextValue),"provenance":in.Provenance})
	default:
		common.APIError(w,405,"METHOD","Use GET or PUT")
	}
}

func (a *app) baselineValues(partnerID string)map[string]map[string]any{
	rows,err:=a.db.Query(`SELECT metric_key,period_start,period_end,numeric_value,text_value FROM impact.metric_baselines WHERE partner_id=$1`,partnerID)
	if err!=nil{return map[string]map[string]any{}}
	defer rows.Close();out:=map[string]map[string]any{}
	for rows.Next(){
		var key,text string;var start,end time.Time;var numeric sql.NullFloat64
		if rows.Scan(&key,&start,&end,&numeric,&text)==nil{
			var num any;if numeric.Valid{num=numeric.Float64}
			out[key]=map[string]any{"baseline_period_start":start.Format("2006-01-02"),"baseline_period_end":end.Format("2006-01-02"),"baseline_numeric_value":num,"baseline_text_value":text}
		}
	}
	return out
}

func stringValue(v any)string{if v==nil{return ""};return fmt.Sprint(v)}

const dashboardPeopleMetricKey = "klavierhaus.events.attendance.attendee_count"

func (a *app) dashboardImpact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.APIError(w, http.StatusMethodNotAllowed, "METHOD", "Use GET")
		return
	}
	year := time.Now().UTC().Year()
	if raw := strings.TrimSpace(r.URL.Query().Get("year")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 2000 || parsed > 2100 {
			common.APIError(w, http.StatusBadRequest, "VALIDATION", "year must be between 2000 and 2100")
			return
		}
		year = parsed
	}

	aggregation, unit := "SUM", "count"
	var labelEN, labelHU string
	err := a.db.QueryRow(`SELECT aggregation,unit,label_en,label_hu FROM impact.metric_definitions WHERE metric_key=$1 AND active=TRUE`,
		dashboardPeopleMetricKey).Scan(&aggregation,&unit,&labelEN,&labelHU)
	if err != nil && err != sql.ErrNoRows {
		common.APIError(w, http.StatusInternalServerError, "DB", "Could not load dashboard Impact definition")
		return
	}

	var ytd sql.NullFloat64
	if err == nil {
		err = a.db.QueryRow(`SELECT
			CASE $3
				WHEN 'LATEST' THEN (ARRAY_AGG(numeric_value ORDER BY period_end DESC,id DESC))[1]
				WHEN 'AVERAGE' THEN AVG(numeric_value)
				ELSE SUM(numeric_value)
			END
		FROM impact.metric_values
		WHERE metric_key=$1 AND numeric_value IS NOT NULL
		  AND period_end >= make_date($2,1,1)
		  AND period_end < make_date($2+1,1,1)`, dashboardPeopleMetricKey, year, aggregation).Scan(&ytd)
		if err != nil {
			common.APIError(w, http.StatusInternalServerError, "DB", "Could not calculate People Reached")
			return
		}
	}

	monthly := map[int]float64{}
	if err == nil {
		rows, queryErr := a.db.Query(`SELECT EXTRACT(MONTH FROM period_end)::int AS month_no,
			CASE $3
				WHEN 'LATEST' THEN (ARRAY_AGG(numeric_value ORDER BY period_end DESC,id DESC))[1]
				WHEN 'AVERAGE' THEN AVG(numeric_value)
				ELSE SUM(numeric_value)
			END AS value
		FROM impact.metric_values
		WHERE metric_key=$1 AND numeric_value IS NOT NULL
		  AND period_end >= make_date($2,1,1)
		  AND period_end < make_date($2+1,1,1)
		GROUP BY EXTRACT(MONTH FROM period_end)
		ORDER BY month_no`, dashboardPeopleMetricKey, year, aggregation)
		if queryErr != nil {
			common.APIError(w, http.StatusInternalServerError, "DB", "Could not calculate Impact trend")
			return
		}
		defer rows.Close()
		for rows.Next() {
			var month int
			var value sql.NullFloat64
			if scanErr := rows.Scan(&month,&value); scanErr != nil {
				common.APIError(w, http.StatusInternalServerError, "DB", "Could not read Impact trend")
				return
			}
			if value.Valid { monthly[month]=value.Float64 }
		}
	}

	labels := []string{"Jan","Feb","Mar","Apr","May","Jun","Jul","Aug","Sep","Oct","Nov","Dec"}
	trend := make([]map[string]any,0,12)
	for month:=1; month<=12; month++ {
		trend=append(trend,map[string]any{
			"month":month,
			"label":labels[month-1],
			"value":monthly[month],
		})
	}
	value:=0.0
	if ytd.Valid { value=ytd.Float64 }
	common.JSON(w,http.StatusOK,map[string]any{
		"year":year,
		"metric_key":dashboardPeopleMetricKey,
		"label_en":labelEN,
		"label_hu":labelHU,
		"unit":unit,
		"aggregation":aggregation,
		"people_reached_ytd":value,
		"trend":trend,
		"source":"IMPACT_METRIC_VALUES",
	})
}

func (a *app) summary(w http.ResponseWriter, r *http.Request) {
	if r.Method!=http.MethodGet { common.APIError(w,405,"METHOD","Use GET"); return }
	partnerID:=strings.TrimSpace(r.URL.Query().Get("partner_id"))
	periodStart:=strings.TrimSpace(r.URL.Query().Get("period_start"))
	periodEnd:=strings.TrimSpace(r.URL.Query().Get("period_end"))
	args:=[]any{}
	whereParts:=[]string{}
	if partnerID!="" { args=append(args,partnerID); whereParts=append(whereParts,fmt.Sprintf("v.partner_id=$%d",len(args))) }
	if periodStart!="" {
		start,err:=time.Parse("2006-01-02",periodStart)
		if err!=nil { common.APIError(w,400,"VALIDATION","period_start must be YYYY-MM-DD"); return }
		args=append(args,start);whereParts=append(whereParts,fmt.Sprintf("v.period_end >= $%d",len(args)))
	}
	if periodEnd!="" {
		end,err:=time.Parse("2006-01-02",periodEnd)
		if err!=nil { common.APIError(w,400,"VALIDATION","period_end must be YYYY-MM-DD"); return }
		args=append(args,end);whereParts=append(whereParts,fmt.Sprintf("v.period_start <= $%d",len(args)))
	}
	where:=""
	if len(whereParts)>0 { where="WHERE "+strings.Join(whereParts," AND ") }
	locale:=common.RequestLocale(r)
	rows,err:=a.db.Query(`SELECT v.metric_key,d.label_en,d.label_hu,d.unit,d.aggregation,
		CASE d.aggregation
			WHEN 'LATEST' THEN (ARRAY_AGG(v.numeric_value ORDER BY v.period_end DESC,v.id DESC))[1]
			WHEN 'AVERAGE' THEN AVG(v.numeric_value)
			ELSE SUM(v.numeric_value)
		END AS numeric_value,
		MAX(v.period_end) AS latest_period_end,
		COUNT(*) AS observations
		FROM impact.metric_values v JOIN impact.metric_definitions d ON d.metric_key=v.metric_key `+where+`
		GROUP BY v.metric_key,d.label_en,d.label_hu,d.unit,d.aggregation ORDER BY d.label_en`,args...)
	if err!=nil { common.APIError(w,500,"DB","Could not calculate impact summary"); return }
	defer rows.Close()
	items:=[]map[string]any{}
	for rows.Next() {
		var key,labelEN,labelHU,unit,agg string
		var num sql.NullFloat64
		var end time.Time
		var count int
		if rows.Scan(&key,&labelEN,&labelHU,&unit,&agg,&num,&end,&count)==nil {
			var value any
			if num.Valid { value=num.Float64 }
			items=append(items,map[string]any{"metric_key":key,"label":common.Localized(labelEN,labelHU,locale),"label_en":labelEN,"label_hu":labelHU,"unit":unit,"aggregation":agg,"numeric_value":value,"latest_period_end":end.Format("2006-01-02"),"observations":count})
		}
	}
	baselines:=a.baselineValues(partnerID)
	for _,item:=range items{
		key:=stringValue(item["metric_key"])
		if b:=baselines[key];b!=nil{
			for k,v:=range b{item[k]=v}
			if current,ok:=item["numeric_value"].(float64);ok{
				if baseline,ok:=b["baseline_numeric_value"].(float64);ok{item["delta_from_baseline"]=current-baseline}
			}
		}
	}
	common.JSON(w,200,map[string]any{"partner_id":partnerID,"items":items,"count":len(items)})
}
