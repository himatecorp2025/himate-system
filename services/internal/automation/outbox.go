package automation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"himate.local/services/internal/common"
	"strings"
	"time"
)

type OutboxEvent struct {
	ProducerService string
	EventKey string
	EventType string
	EventVersion int
	PartnerID string
	ModuleKey string
	CorrelationID string
	CausationID string
	SubjectType string
	SubjectID string
	Payload map[string]any
	OccurredAt time.Time
	AvailableAt time.Time
}

type OutboxRecord struct {
	ID int64
	OutboxEvent
	Attempts int
}

func OutboxMigration(version int) common.Migration {
	return common.Migration{Version:version,Name:"durable-domain-automation-outbox",Statements:[]string{
		`CREATE SCHEMA IF NOT EXISTS automation_outbox`,
		`CREATE TABLE IF NOT EXISTS automation_outbox.events(
			id BIGSERIAL PRIMARY KEY,
			producer_service TEXT NOT NULL,
			event_key TEXT NOT NULL,
			event_type TEXT NOT NULL,
			event_version INT NOT NULL DEFAULT 1,
			partner_id TEXT NOT NULL DEFAULT '',
			module_key TEXT NOT NULL DEFAULT '',
			correlation_id TEXT NOT NULL DEFAULT '',
			causation_id TEXT NOT NULL DEFAULT '',
			subject_type TEXT NOT NULL DEFAULT '',
			subject_id TEXT NOT NULL DEFAULT '',
			payload JSONB NOT NULL DEFAULT '{}'::jsonb,
			occurred_at TIMESTAMPTZ NOT NULL,
			available_at TIMESTAMPTZ NOT NULL,
			status TEXT NOT NULL DEFAULT 'PENDING' CHECK(status IN ('PENDING','PROCESSING','PUBLISHED')),
			attempts INT NOT NULL DEFAULT 0,
			lease_until TIMESTAMPTZ,
			last_error TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			published_at TIMESTAMPTZ,
			UNIQUE(producer_service,event_key)
		)`,
		`CREATE INDEX IF NOT EXISTS automation_outbox_pending_idx ON automation_outbox.events(producer_service,status,available_at,lease_until,id)`,
	}}
}

func EnqueueTx(ctx context.Context,tx *sql.Tx,event OutboxEvent)error{
	if tx==nil{return errors.New("automation outbox requires the caller business transaction")}
	event.ProducerService=strings.TrimSpace(event.ProducerService);event.EventKey=strings.TrimSpace(event.EventKey);event.EventType=strings.TrimSpace(event.EventType)
	if event.ProducerService==""||event.EventKey==""||event.EventType==""{return errors.New("producer_service, event_key and event_type are required")}
	if event.EventVersion<=0{event.EventVersion=1}
	if event.OccurredAt.IsZero(){event.OccurredAt=time.Now().UTC()}
	if event.AvailableAt.IsZero(){event.AvailableAt=event.OccurredAt}
	if event.Payload==nil{event.Payload=map[string]any{}}
	raw,err:=json.Marshal(event.Payload);if err!=nil{return err}
	_,err=tx.ExecContext(ctx,`INSERT INTO automation_outbox.events(
		producer_service,event_key,event_type,event_version,partner_id,module_key,correlation_id,causation_id,subject_type,subject_id,payload,occurred_at,available_at
	) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::jsonb,$12,$13)
	ON CONFLICT(producer_service,event_key) DO NOTHING`,
		event.ProducerService,event.EventKey,event.EventType,event.EventVersion,strings.TrimSpace(event.PartnerID),strings.TrimSpace(event.ModuleKey),
		strings.TrimSpace(event.CorrelationID),strings.TrimSpace(event.CausationID),strings.TrimSpace(event.SubjectType),strings.TrimSpace(event.SubjectID),
		string(raw),event.OccurredAt.UTC(),event.AvailableAt.UTC())
	return err
}

func ClaimOutbox(ctx context.Context,db *sql.DB,producer string,limit int,lease time.Duration)([]OutboxRecord,error){
	producer=strings.TrimSpace(producer);if producer==""{return nil,errors.New("producer is required")}
	if limit<=0{limit=20};if limit>200{limit=200};if lease<=0{lease=60*time.Second}
	tx,err:=db.BeginTx(ctx,nil);if err!=nil{return nil,err};defer tx.Rollback()
	rows,err:=tx.QueryContext(ctx,`WITH picked AS (
		SELECT id FROM automation_outbox.events WHERE producer_service=$1 AND available_at<=NOW()
		AND (status='PENDING' OR (status='PROCESSING' AND lease_until<NOW()))
		ORDER BY available_at,id FOR UPDATE SKIP LOCKED LIMIT $2
	)
	UPDATE automation_outbox.events e SET status='PROCESSING',attempts=e.attempts+1,lease_until=NOW()+($3*INTERVAL '1 millisecond'),updated_at=NOW()
	FROM picked p WHERE e.id=p.id
	RETURNING e.id,e.producer_service,e.event_key,e.event_type,e.event_version,e.partner_id,e.module_key,e.correlation_id,e.causation_id,e.subject_type,e.subject_id,e.payload,e.occurred_at,e.available_at,e.attempts`,
		producer,limit,lease.Milliseconds())
	if err!=nil{return nil,err};defer rows.Close()
	out:=[]OutboxRecord{}
	for rows.Next(){
		var x OutboxRecord;var raw []byte
		if err:=rows.Scan(&x.ID,&x.ProducerService,&x.EventKey,&x.EventType,&x.EventVersion,&x.PartnerID,&x.ModuleKey,&x.CorrelationID,&x.CausationID,&x.SubjectType,&x.SubjectID,&raw,&x.OccurredAt,&x.AvailableAt,&x.Attempts);err!=nil{return nil,err}
		x.Payload=map[string]any{};_ = json.Unmarshal(raw,&x.Payload);out=append(out,x)
	}
	if err=tx.Commit();err!=nil{return nil,err}
	return out,nil
}

func MarkOutboxPublished(ctx context.Context,db *sql.DB,id int64)error{
	res,err:=db.ExecContext(ctx,`UPDATE automation_outbox.events SET status='PUBLISHED',published_at=NOW(),lease_until=NULL,last_error='',updated_at=NOW() WHERE id=$1 AND status='PROCESSING'`,id)
	if err!=nil{return err};n,_:=res.RowsAffected();if n!=1{return errors.New("automation outbox record is not processing")};return nil
}

func FailOutbox(ctx context.Context,db *sql.DB,id int64,message string,retryAfter time.Duration)error{
	if retryAfter<5*time.Second{retryAfter=5*time.Second}
	_,err:=db.ExecContext(ctx,`UPDATE automation_outbox.events SET status='PENDING',lease_until=NULL,last_error=$2,available_at=NOW()+($3*INTERVAL '1 millisecond'),updated_at=NOW() WHERE id=$1 AND status='PROCESSING'`,
		id,strings.TrimSpace(message),retryAfter.Milliseconds())
	return err
}
