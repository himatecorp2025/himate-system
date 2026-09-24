package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"himate.local/services/internal/automation"
	"himate.local/services/internal/common"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type app struct {
	db *sql.DB
	keys map[string]string
}

var eventTypePattern=regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.:-]{2,159}$`)

func main(){
	log:=common.Logger()
	db,err:=common.OpenDB();if err!=nil{log.Error("database","error",err);os.Exit(1)};defer db.Close()
	a:=&app{db:db,keys:loadServiceKeys()}
	if len(a.keys)==0{log.Error("automation service keys are missing");os.Exit(1)}
	ctx,cancel:=context.WithTimeout(context.Background(),30*time.Second);defer cancel()
	if err:=a.migrate(ctx);err!=nil{log.Error("migration","error",err);os.Exit(1)}
	mux:=http.NewServeMux()
	mux.HandleFunc("/health",func(w http.ResponseWriter,r *http.Request){common.JSON(w,200,map[string]any{"status":"ok","service":"automation","time":time.Now().UTC()})})
	mux.HandleFunc("/internal/v1/automation/subscriptions",a.subscriptions)
	mux.HandleFunc("/internal/v1/automation/events",a.events)
	mux.HandleFunc("/internal/v1/automation/deliveries/claim",a.claim)
	mux.HandleFunc("/internal/v1/automation/deliveries/",a.deliveryAction)
	mux.HandleFunc("/internal/v1/automation/dead-letters",a.deadLetters)
	common.Run(log,"automation",common.Env("PORT","10000"),common.InternalAuth(os.Getenv("HIMATE_INTERNAL_TOKEN"),mux))
}

func loadServiceKeys()map[string]string{
	raw:=strings.TrimSpace(os.Getenv("HIMATE_AUTOMATION_SERVICE_KEYS_JSON"))
	out:=map[string]string{}
	if raw!=""{
		var source map[string]string
		if json.Unmarshal([]byte(raw),&source)==nil{
			for k,v:=range source{
				k=strings.TrimSpace(k);v=strings.TrimSpace(v)
				if k!=""&&len(v)>=24{out[k]=v}
			}
		}
	}
	// Dedicated consumer/producer credentials may be exposed separately so a
	// service can receive only its own HMAC secret instead of the full verifier keyring.
	if financeSecret:=strings.TrimSpace(os.Getenv("HIMATE_AUTOMATION_FINANCE_SECRET"));len(financeSecret)>=24{
		out["finance"]=financeSecret
	}
	return out
}

func (a *app)migrate(ctx context.Context)error{
	return common.ApplyMigrations(ctx,a.db,"automation",[]common.Migration{
		{Version:1,Name:"durable-automation-backbone",Statements:[]string{
			`CREATE SCHEMA IF NOT EXISTS automation`,
			`CREATE TABLE IF NOT EXISTS automation.subscriptions(
				id BIGSERIAL PRIMARY KEY,
				consumer_service TEXT NOT NULL,
				event_type TEXT NOT NULL,
				partner_id TEXT NOT NULL DEFAULT '',
				module_key TEXT NOT NULL DEFAULT '',
				active BOOLEAN NOT NULL DEFAULT TRUE,
				max_attempts INT NOT NULL DEFAULT 8 CHECK(max_attempts BETWEEN 1 AND 50),
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				UNIQUE(consumer_service,event_type,partner_id,module_key)
			)`,
			`CREATE INDEX IF NOT EXISTS automation_subscription_match_idx ON automation.subscriptions(event_type,active,partner_id,module_key)`,
			`CREATE TABLE IF NOT EXISTS automation.events(
				id BIGSERIAL PRIMARY KEY,
				event_key TEXT NOT NULL,
				event_type TEXT NOT NULL,
				event_version INT NOT NULL DEFAULT 1 CHECK(event_version>0),
				producer_service TEXT NOT NULL,
				partner_id TEXT NOT NULL DEFAULT '',
				module_key TEXT NOT NULL DEFAULT '',
				correlation_id TEXT NOT NULL DEFAULT '',
				causation_id TEXT NOT NULL DEFAULT '',
				subject_type TEXT NOT NULL DEFAULT '',
				subject_id TEXT NOT NULL DEFAULT '',
				payload JSONB NOT NULL DEFAULT '{}'::jsonb,
				envelope_hash TEXT NOT NULL,
				occurred_at TIMESTAMPTZ NOT NULL,
				available_at TIMESTAMPTZ NOT NULL,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				UNIQUE(producer_service,event_key)
			)`,
			`CREATE INDEX IF NOT EXISTS automation_events_tenant_idx ON automation.events(partner_id,created_at DESC,id DESC)`,
			`CREATE INDEX IF NOT EXISTS automation_events_type_idx ON automation.events(event_type,created_at DESC,id DESC)`,
			`CREATE TABLE IF NOT EXISTS automation.deliveries(
				id BIGSERIAL PRIMARY KEY,
				event_id BIGINT NOT NULL REFERENCES automation.events(id) ON DELETE RESTRICT,
				consumer_service TEXT NOT NULL,
				status TEXT NOT NULL DEFAULT 'PENDING' CHECK(status IN ('PENDING','PROCESSING','ACKED','DEAD_LETTER')),
				attempts INT NOT NULL DEFAULT 0,
				max_attempts INT NOT NULL DEFAULT 8,
				available_at TIMESTAMPTZ NOT NULL,
				lease_until TIMESTAMPTZ,
				last_error TEXT NOT NULL DEFAULT '',
				acked_at TIMESTAMPTZ,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				UNIQUE(event_id,consumer_service)
			)`,
			`CREATE INDEX IF NOT EXISTS automation_delivery_claim_idx ON automation.deliveries(consumer_service,status,available_at,lease_until,id)`,
			`CREATE OR REPLACE FUNCTION automation.reject_event_mutation() RETURNS trigger AS $$ BEGIN RAISE EXCEPTION 'automation events are immutable'; END; $$ LANGUAGE plpgsql`,
			`DROP TRIGGER IF EXISTS automation_events_no_update ON automation.events`,
			`CREATE TRIGGER automation_events_no_update BEFORE UPDATE OR DELETE ON automation.events FOR EACH ROW EXECUTE FUNCTION automation.reject_event_mutation()`,
		}},
	})
}

func (a *app)readSignedBody(w http.ResponseWriter,r *http.Request)(string,[]byte,bool){
	r.Body=http.MaxBytesReader(w,r.Body,1024*1024)
	body,err:=io.ReadAll(r.Body);if err!=nil{common.APIError(w,400,"BODY","Could not read request body");return "",nil,false}
	service,err:=automation.VerifyRequest(r,body,func(id string)(string,bool){v,ok:=a.keys[id];return v,ok},time.Now().UTC(),5*time.Minute)
	if err!=nil{common.APIError(w,403,"SERVICE_IDENTITY",err.Error());return "",nil,false}
	return service,body,true
}

func (a *app)verifySignedEmpty(w http.ResponseWriter,r *http.Request)(string,bool){
	service,err:=automation.VerifyRequest(r,nil,func(id string)(string,bool){v,ok:=a.keys[id];return v,ok},time.Now().UTC(),5*time.Minute)
	if err!=nil{common.APIError(w,403,"SERVICE_IDENTITY",err.Error());return "",false}
	return service,true
}

func hashEnvelope(v any)string{raw,_:=json.Marshal(v);sum:=sha256.Sum256(raw);return hex.EncodeToString(sum[:])}

func (a *app)subscriptions(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodPost{common.APIError(w,405,"METHOD","Use POST");return}
	service,body,ok:=a.readSignedBody(w,r);if !ok{return}
	var in struct{EventTypes []string `json:"event_types"`;PartnerID string `json:"partner_id"`;ModuleKey string `json:"module_key"`;MaxAttempts int `json:"max_attempts"`}
	if json.Unmarshal(body,&in)!=nil||len(in.EventTypes)==0{common.APIError(w,400,"VALIDATION","event_types are required");return}
	if in.MaxAttempts==0{in.MaxAttempts=8}
	if in.MaxAttempts<1||in.MaxAttempts>50{common.APIError(w,400,"VALIDATION","max_attempts must be between 1 and 50");return}
	tx,err:=a.db.BeginTx(r.Context(),nil);if err!=nil{common.APIError(w,500,"DB","Could not begin subscription transaction");return};defer tx.Rollback()
	seen:=map[string]bool{}
	for _,eventType:=range in.EventTypes{
		eventType=strings.TrimSpace(eventType)
		if !eventTypePattern.MatchString(eventType)||seen[eventType]{common.APIError(w,400,"VALIDATION","Invalid or duplicate event type");return}
		seen[eventType]=true
		if _,err=tx.ExecContext(r.Context(),`INSERT INTO automation.subscriptions(consumer_service,event_type,partner_id,module_key,max_attempts)
			VALUES($1,$2,$3,$4,$5)
			ON CONFLICT(consumer_service,event_type,partner_id,module_key) DO UPDATE SET active=TRUE,max_attempts=EXCLUDED.max_attempts,updated_at=NOW()`,
			service,eventType,strings.TrimSpace(in.PartnerID),strings.TrimSpace(in.ModuleKey),in.MaxAttempts);err!=nil{common.APIError(w,500,"DB","Could not register subscription");return}
	}
	if err=tx.Commit();err!=nil{common.APIError(w,500,"DB","Could not commit subscription");return}
	common.JSON(w,200,map[string]any{"consumer_service":service,"event_types":in.EventTypes,"partner_id":strings.TrimSpace(in.PartnerID),"module_key":strings.TrimSpace(in.ModuleKey),"max_attempts":in.MaxAttempts})
}

func (a *app)events(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodPost{common.APIError(w,405,"METHOD","Use POST");return}
	service,body,ok:=a.readSignedBody(w,r);if !ok{return}
	var in struct{
		EventKey string `json:"event_key"`;EventType string `json:"event_type"`;EventVersion int `json:"event_version"`
		PartnerID string `json:"partner_id"`;ModuleKey string `json:"module_key"`;CorrelationID string `json:"correlation_id"`;CausationID string `json:"causation_id"`
		SubjectType string `json:"subject_type"`;SubjectID string `json:"subject_id"`;Payload map[string]any `json:"payload"`;OccurredAt time.Time `json:"occurred_at"`;AvailableAt time.Time `json:"available_at"`
	}
	if json.Unmarshal(body,&in)!=nil{common.APIError(w,400,"JSON","Invalid event envelope");return}
	in.EventKey=strings.TrimSpace(in.EventKey);in.EventType=strings.TrimSpace(in.EventType)
	if in.EventKey==""||len(in.EventKey)>200||!eventTypePattern.MatchString(in.EventType){common.APIError(w,400,"VALIDATION","event_key and valid event_type are required");return}
	if in.EventVersion==0{in.EventVersion=1}
	if in.EventVersion<1{common.APIError(w,400,"VALIDATION","event_version must be positive");return}
	if in.OccurredAt.IsZero(){in.OccurredAt=time.Now().UTC()}else{in.OccurredAt=in.OccurredAt.UTC()}
	if in.AvailableAt.IsZero(){in.AvailableAt=in.OccurredAt}else{in.AvailableAt=in.AvailableAt.UTC()}
	if in.Payload==nil{in.Payload=map[string]any{}}
	correlation:=strings.TrimSpace(in.CorrelationID);if correlation==""{correlation=strings.TrimSpace(r.Header.Get(automation.HeaderCorrelationID))}
	causation:=strings.TrimSpace(in.CausationID);if causation==""{causation=strings.TrimSpace(r.Header.Get(automation.HeaderCausationID))}
	normalized:=map[string]any{"event_key":in.EventKey,"event_type":in.EventType,"event_version":in.EventVersion,"producer_service":service,"partner_id":strings.TrimSpace(in.PartnerID),"module_key":strings.TrimSpace(in.ModuleKey),"correlation_id":correlation,"causation_id":causation,"subject_type":strings.TrimSpace(in.SubjectType),"subject_id":strings.TrimSpace(in.SubjectID),"payload":in.Payload,"occurred_at":in.OccurredAt,"available_at":in.AvailableAt}
	hash:=hashEnvelope(normalized);payloadRaw,_:=json.Marshal(in.Payload)
	tx,err:=a.db.BeginTx(r.Context(),nil);if err!=nil{common.APIError(w,500,"DB","Could not begin event transaction");return};defer tx.Rollback()
	var id int64
	var existingHash string
	err=tx.QueryRowContext(r.Context(),`INSERT INTO automation.events(event_key,event_type,event_version,producer_service,partner_id,module_key,correlation_id,causation_id,subject_type,subject_id,payload,envelope_hash,occurred_at,available_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::jsonb,$12,$13,$14)
		ON CONFLICT(producer_service,event_key) DO NOTHING RETURNING id,envelope_hash`,
		in.EventKey,in.EventType,in.EventVersion,service,strings.TrimSpace(in.PartnerID),strings.TrimSpace(in.ModuleKey),correlation,causation,strings.TrimSpace(in.SubjectType),strings.TrimSpace(in.SubjectID),string(payloadRaw),hash,in.OccurredAt,in.AvailableAt).Scan(&id,&existingHash)
	duplicate:=false
	if err==sql.ErrNoRows{
		duplicate=true
		if err=tx.QueryRowContext(r.Context(),`SELECT id,envelope_hash FROM automation.events WHERE producer_service=$1 AND event_key=$2`,service,in.EventKey).Scan(&id,&existingHash);err!=nil{common.APIError(w,500,"DB","Could not read idempotent event");return}
		if existingHash!=hash{common.APIError(w,409,"EVENT_KEY_CONFLICT","event_key was already used for a different envelope");return}
	}else if err!=nil{common.APIError(w,500,"DB","Could not persist event");return}
	if !duplicate{
		if _,err=tx.ExecContext(r.Context(),`INSERT INTO automation.deliveries(event_id,consumer_service,max_attempts,available_at)
			SELECT $1,s.consumer_service,MAX(s.max_attempts),$2 FROM automation.subscriptions s
			WHERE s.active=TRUE AND s.event_type=$3 AND (s.partner_id='' OR s.partner_id=$4) AND (s.module_key='' OR s.module_key=$5)
			GROUP BY s.consumer_service ON CONFLICT(event_id,consumer_service) DO NOTHING`,
			id,in.AvailableAt,in.EventType,strings.TrimSpace(in.PartnerID),strings.TrimSpace(in.ModuleKey));err!=nil{common.APIError(w,500,"DB","Could not create event deliveries");return}
	}
	if err=tx.Commit();err!=nil{common.APIError(w,500,"DB","Could not commit event");return}
	status:=http.StatusCreated;if duplicate{status=http.StatusOK}
	common.JSON(w,status,map[string]any{"id":id,"event_key":in.EventKey,"event_type":in.EventType,"producer_service":service,"duplicate":duplicate,"available_at":in.AvailableAt})
}

func (a *app)claim(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodPost{common.APIError(w,405,"METHOD","Use POST");return}
	service,body,ok:=a.readSignedBody(w,r);if !ok{return}
	var in struct{Limit int `json:"limit"`}
	if len(strings.TrimSpace(string(body)))>0&&json.Unmarshal(body,&in)!=nil{common.APIError(w,400,"JSON","Invalid claim request");return}
	if in.Limit<=0{in.Limit=10};if in.Limit>100{in.Limit=100}
	tx,err:=a.db.BeginTx(r.Context(),nil);if err!=nil{common.APIError(w,500,"DB","Could not begin claim");return};defer tx.Rollback()
	rows,err:=tx.QueryContext(r.Context(),`WITH picked AS (
		SELECT id FROM automation.deliveries
		WHERE consumer_service=$1 AND available_at<=NOW()
		  AND (status='PENDING' OR (status='PROCESSING' AND lease_until<NOW()))
		ORDER BY available_at,id LIMIT $2 FOR UPDATE SKIP LOCKED
	)
	UPDATE automation.deliveries d SET status='PROCESSING',attempts=d.attempts+1,lease_until=NOW()+INTERVAL '60 seconds',updated_at=NOW()
	FROM picked p,automation.events e WHERE d.id=p.id AND e.id=d.event_id
	RETURNING d.id,d.attempts,d.max_attempts,e.id,e.event_key,e.event_type,e.event_version,e.producer_service,e.partner_id,e.module_key,e.correlation_id,e.causation_id,e.subject_type,e.subject_id,e.payload,e.occurred_at,e.available_at`,service,in.Limit)
	if err!=nil{common.APIError(w,500,"DB","Could not claim deliveries");return};defer rows.Close()
	items:=[]map[string]any{}
	for rows.Next(){
		var deliveryID,eventID int64;var attempts,maxAttempts,eventVersion int
		var eventKey,eventType,producer,partnerID,moduleKey,correlation,causation,subjectType,subjectID string;var payloadRaw []byte;var occurred,available time.Time
		if err=rows.Scan(&deliveryID,&attempts,&maxAttempts,&eventID,&eventKey,&eventType,&eventVersion,&producer,&partnerID,&moduleKey,&correlation,&causation,&subjectType,&subjectID,&payloadRaw,&occurred,&available);err!=nil{common.APIError(w,500,"DB","Could not read delivery");return}
		payload:=map[string]any{};_ = json.Unmarshal(payloadRaw,&payload)
		items=append(items,map[string]any{"delivery_id":deliveryID,"attempt":attempts,"max_attempts":maxAttempts,"event":map[string]any{"id":eventID,"event_key":eventKey,"event_type":eventType,"event_version":eventVersion,"producer_service":producer,"partner_id":partnerID,"module_key":moduleKey,"correlation_id":correlation,"causation_id":causation,"subject_type":subjectType,"subject_id":subjectID,"payload":payload,"occurred_at":occurred,"available_at":available}})
	}
	if err=tx.Commit();err!=nil{common.APIError(w,500,"DB","Could not commit claim");return}
	common.JSON(w,200,map[string]any{"consumer_service":service,"items":items,"count":len(items),"lease_seconds":60})
}

func (a *app)deliveryAction(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodPost{common.APIError(w,405,"METHOD","Use POST");return}
	service,body,ok:=a.readSignedBody(w,r);if !ok{return}
	raw:=strings.Trim(strings.TrimPrefix(r.URL.Path,"/internal/v1/automation/deliveries/"),"/")
	parts:=strings.Split(raw,"/");if len(parts)!=2{common.APIError(w,404,"NOT_FOUND","Delivery action not found");return}
	id,err:=strconv.ParseInt(parts[0],10,64);if err!=nil{common.APIError(w,400,"VALIDATION","Invalid delivery id");return}
	switch parts[1]{
	case "ack":
		res,err:=a.db.ExecContext(r.Context(),`UPDATE automation.deliveries SET status='ACKED',acked_at=NOW(),lease_until=NULL,last_error='',updated_at=NOW()
			WHERE id=$1 AND consumer_service=$2 AND status='PROCESSING'`,id,service)
		if err!=nil{common.APIError(w,500,"DB","Could not acknowledge delivery");return};n,_:=res.RowsAffected();if n==0{common.APIError(w,409,"DELIVERY_STATE","Delivery is not claimable by this consumer");return}
		common.JSON(w,200,map[string]any{"delivery_id":id,"status":"ACKED"});return
	case "fail":
		var in struct{Error string `json:"error"`;RetryAfterSeconds int `json:"retry_after_seconds"`}
		if json.Unmarshal(body,&in)!=nil{common.APIError(w,400,"JSON","Invalid failure request");return}
		if in.RetryAfterSeconds<5{in.RetryAfterSeconds=5};if in.RetryAfterSeconds>86400{in.RetryAfterSeconds=86400}
		var attempts,maxAttempts int
		if err=a.db.QueryRowContext(r.Context(),`SELECT attempts,max_attempts FROM automation.deliveries WHERE id=$1 AND consumer_service=$2 AND status='PROCESSING'`,id,service).Scan(&attempts,&maxAttempts);err!=nil{common.APIError(w,409,"DELIVERY_STATE","Delivery is not processing for this consumer");return}
		status:="PENDING";if attempts>=maxAttempts{status="DEAD_LETTER"}
		_,err=a.db.ExecContext(r.Context(),`UPDATE automation.deliveries SET status=$3,last_error=$4,lease_until=NULL,
			available_at=CASE WHEN $3='PENDING' THEN NOW()+($5*INTERVAL '1 second') ELSE available_at END,updated_at=NOW()
			WHERE id=$1 AND consumer_service=$2`,id,service,status,strings.TrimSpace(in.Error),in.RetryAfterSeconds)
		if err!=nil{common.APIError(w,500,"DB","Could not fail delivery");return}
		common.JSON(w,200,map[string]any{"delivery_id":id,"status":status,"attempts":attempts,"max_attempts":maxAttempts});return
	default:common.APIError(w,404,"NOT_FOUND","Delivery action not found")
	}
}

func (a *app)deadLetters(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodGet{common.APIError(w,405,"METHOD","Use GET");return}
	service,ok:=a.verifySignedEmpty(w,r);if !ok{return}
	rows,err:=a.db.QueryContext(r.Context(),`SELECT d.id,d.attempts,d.max_attempts,d.last_error,d.updated_at,e.event_key,e.event_type,e.partner_id,e.module_key
		FROM automation.deliveries d JOIN automation.events e ON e.id=d.event_id
		WHERE d.consumer_service=$1 AND d.status='DEAD_LETTER' ORDER BY d.updated_at DESC,d.id DESC LIMIT 200`,service)
	if err!=nil{common.APIError(w,500,"DB","Could not load dead letters");return};defer rows.Close()
	items:=[]map[string]any{};for rows.Next(){var id int64;var attempts,maxAttempts int;var lastError,eventKey,eventType,partnerID,moduleKey string;var updated time.Time
		if rows.Scan(&id,&attempts,&maxAttempts,&lastError,&updated,&eventKey,&eventType,&partnerID,&moduleKey)==nil{items=append(items,map[string]any{"delivery_id":id,"attempts":attempts,"max_attempts":maxAttempts,"last_error":lastError,"updated_at":updated,"event_key":eventKey,"event_type":eventType,"partner_id":partnerID,"module_key":moduleKey})}}
	common.JSON(w,200,map[string]any{"consumer_service":service,"items":items,"count":len(items)})
}

func (a *app)String()string{return fmt.Sprintf("automation keys=%d",len(a.keys))}
