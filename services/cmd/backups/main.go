package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"himate.local/services/internal/common"
	"himate.local/services/internal/partnerdb"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	defaultRetention = 30
	defaultMaxPoints = 30
	defaultSchedule = 24
)

type app struct {
	db            *sql.DB
	internalToken string
	client        *http.Client
	dbAdminURL    string
	storageHost   string
	envHost       string
	connectorHost string
	partnersHost  string
	workRoot      string
	key           []byte
	provider      offsiteProvider
	wake          chan struct{}
	workers       int
}

type policy struct {
	PartnerID        string
	RetentionDays    int
	MaxRestorePoints int
	ScheduleHours    int
	Enabled          bool
	UpdatedAt        time.Time
}

type restorePoint struct {
	ID               string
	PartnerID        string
	Status           string
	Provider         string
	ObjectKey        string
	CiphertextSHA256 string
	CiphertextBytes  int64
	Manifest         json.RawMessage
	CreatedBy        string
	Error            string
	CreatedAt        time.Time
	CompletedAt      sql.NullTime
	ExpiresAt        sql.NullTime
}

type restoreTest struct {
	ID             string
	RestorePointID string
	PartnerID      string
	Status         string
	DatabaseOK     bool
	MediaOK        bool
	ConfigOK       bool
	Error          string
	CreatedBy      string
	CreatedAt      time.Time
	CompletedAt    sql.NullTime
	DurationMS     int64
}

func main() {
	log := common.Logger()
	db, err := common.OpenDB()
	if err != nil { log.Error("database", "error", err); os.Exit(1) }
	defer db.Close()

	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(os.Getenv("HIMATE_BACKUP_ENCRYPTION_KEY_B64")))
	if err != nil || len(key) != 32 {
		log.Error("backup encryption key", "error", "HIMATE_BACKUP_ENCRYPTION_KEY_B64 must decode to exactly 32 bytes")
		os.Exit(1)
	}
	adminURL := strings.TrimSpace(os.Getenv("PARTNER_DATABASE_ADMIN_URL"))
	if adminURL == "" { adminURL = strings.TrimSpace(os.Getenv("DATABASE_URL")) }
	if _, err := partnerdb.AdminDSN(adminURL); err != nil {
		log.Error("partner database admin URL", "error", err)
		os.Exit(1)
	}
	for _, tool := range []string{"pg_dump","pg_restore"} {
		if _, err := exec.LookPath(tool); err != nil {
			log.Error("backup tooling", "error", tool+" is required")
			os.Exit(1)
		}
	}

	workRoot := common.Env("HIMATE_BACKUP_WORK_ROOT", "/var/lib/himate-backups/work")
	if err := os.MkdirAll(workRoot,0700); err != nil { log.Error("backup work root","error",err);os.Exit(1) }
	client := &http.Client{Timeout:30*time.Minute}
	provider, err := newOffsiteProvider(client)
	if err != nil { log.Error("backup offsite provider","error",err);os.Exit(1) }

	workers,_:=strconv.Atoi(common.Env("HIMATE_BACKUP_WORKERS","1"))
	if workers<1{workers=1};if workers>4{workers=4}
	a:=&app{
		db:db,internalToken:os.Getenv("HIMATE_INTERNAL_TOKEN"),client:client,dbAdminURL:adminURL,
		storageHost:os.Getenv("STORAGE_HOSTPORT"),envHost:os.Getenv("ENVIRONMENTS_HOSTPORT"),
		connectorHost:os.Getenv("CONNECTOR_HOSTPORT"),partnersHost:os.Getenv("PARTNERS_HOSTPORT"),
		workRoot:workRoot,key:key,provider:provider,wake:make(chan struct{},1),workers:workers,
	}
	if len(a.internalToken)<24{log.Error("backup internal token","error","HIMATE_INTERNAL_TOKEN is required");os.Exit(1)}

	ctx,cancel:=context.WithTimeout(context.Background(),30*time.Second);defer cancel()
	if err:=a.migrate(ctx);err!=nil{log.Error("migration","error",err);os.Exit(1)}
	_,_=a.db.Exec(`UPDATE backups.restore_points SET status='QUEUED',error='worker restarted before completion' WHERE status='RUNNING'`)
	_,_=a.db.Exec(`UPDATE backups.restore_tests SET status='QUEUED',error='worker restarted before completion' WHERE status='RUNNING'`)

	for i:=0;i<a.workers;i++{go a.worker(i)}
	go a.scheduler()

	mux:=http.NewServeMux()
	mux.HandleFunc("/health",a.health)
	mux.HandleFunc("/api/v1/backups",a.backups)
	mux.HandleFunc("/api/v1/backups/",a.backupRoute)
	mux.HandleFunc("/internal/v1/backups/summary",a.summary)
	common.Run(log,"backups",common.Env("PORT","10000"),common.InternalAuth(a.internalToken,mux))
}

func (a *app) migrate(ctx context.Context) error {
	return common.ApplyMigrations(ctx,a.db,"backups",[]common.Migration{
		{Version:1,Name:"restore-points",Statements:[]string{
			`CREATE SCHEMA IF NOT EXISTS backups`,
			`CREATE TABLE IF NOT EXISTS backups.policies(
				partner_id TEXT PRIMARY KEY,
				retention_days INTEGER NOT NULL DEFAULT 30,
				max_restore_points INTEGER NOT NULL DEFAULT 30,
				schedule_hours INTEGER NOT NULL DEFAULT 24,
				enabled BOOLEAN NOT NULL DEFAULT TRUE,
				last_scheduled_at TIMESTAMPTZ,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				CHECK(retention_days BETWEEN 1 AND 3650),
				CHECK(max_restore_points BETWEEN 1 AND 365),
				CHECK(schedule_hours BETWEEN 1 AND 720)
			)`,
			`CREATE TABLE IF NOT EXISTS backups.restore_points(
				id TEXT PRIMARY KEY,
				partner_id TEXT NOT NULL,
				status TEXT NOT NULL DEFAULT 'QUEUED',
				provider TEXT NOT NULL DEFAULT '',
				object_key TEXT NOT NULL DEFAULT '',
				ciphertext_sha256 TEXT NOT NULL DEFAULT '',
				ciphertext_bytes BIGINT NOT NULL DEFAULT 0,
				manifest JSONB NOT NULL DEFAULT '{}'::jsonb,
				created_by TEXT NOT NULL DEFAULT '',
				error TEXT NOT NULL DEFAULT '',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				completed_at TIMESTAMPTZ,
				expires_at TIMESTAMPTZ
			)`,
			`CREATE INDEX IF NOT EXISTS backups_restore_partner_idx ON backups.restore_points(partner_id,created_at DESC)`,
			`CREATE INDEX IF NOT EXISTS backups_restore_status_idx ON backups.restore_points(status,created_at)`,
			`CREATE TABLE IF NOT EXISTS backups.restore_tests(
				id TEXT PRIMARY KEY,
				restore_point_id TEXT NOT NULL REFERENCES backups.restore_points(id),
				partner_id TEXT NOT NULL,
				status TEXT NOT NULL DEFAULT 'QUEUED',
				database_ok BOOLEAN NOT NULL DEFAULT FALSE,
				media_ok BOOLEAN NOT NULL DEFAULT FALSE,
				config_ok BOOLEAN NOT NULL DEFAULT FALSE,
				error TEXT NOT NULL DEFAULT '',
				created_by TEXT NOT NULL DEFAULT '',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				completed_at TIMESTAMPTZ,
				duration_ms BIGINT NOT NULL DEFAULT 0
			)`,
			`CREATE INDEX IF NOT EXISTS backups_tests_point_idx ON backups.restore_tests(restore_point_id,created_at DESC)`,
			`CREATE INDEX IF NOT EXISTS backups_tests_status_idx ON backups.restore_tests(status,created_at)`,
		}},
	})
}

func newID(prefix string) string {
	raw:=make([]byte,12)
	if _,err:=rand.Read(raw);err!=nil{return fmt.Sprintf("%s_%d",prefix,time.Now().UTC().UnixNano())}
	return prefix+"_"+hex.EncodeToString(raw)
}

func safeError(err error) string {
	if err==nil{return ""}
	value:=err.Error();if len(value)>1000{value=value[:1000]};return value
}

func (a *app) signal(){select{case a.wake<-struct{}{}:default:}}

func (a *app) ensurePolicy(partnerID string)(policy,error){
	partnerID=strings.TrimSpace(partnerID);if partnerID==""{return policy{},fmt.Errorf("partner_id is required")}
	_,err:=a.db.Exec(`INSERT INTO backups.policies(partner_id) VALUES($1) ON CONFLICT(partner_id) DO NOTHING`,partnerID)
	if err!=nil{return policy{},err}
	var p policy
	err=a.db.QueryRow(`SELECT partner_id,retention_days,max_restore_points,schedule_hours,enabled,updated_at FROM backups.policies WHERE partner_id=$1`,partnerID).
		Scan(&p.PartnerID,&p.RetentionDays,&p.MaxRestorePoints,&p.ScheduleHours,&p.Enabled,&p.UpdatedAt)
	return p,err
}

func (a *app) queueBackup(partnerID,actor string)(restorePoint,error){
	p,err:=a.ensurePolicy(partnerID);if err!=nil{return restorePoint{},err}
	if !p.Enabled{return restorePoint{},fmt.Errorf("backup policy is disabled")}
	var pending bool
	if err:=a.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM backups.restore_points WHERE partner_id=$1 AND status IN('QUEUED','RUNNING'))`,partnerID).Scan(&pending);err!=nil{return restorePoint{},err}
	if pending{return restorePoint{},fmt.Errorf("backup already queued or running for partner")}
	id:=newID("bkp");expires:=time.Now().UTC().Add(time.Duration(p.RetentionDays)*24*time.Hour)
	_,err=a.db.Exec(`INSERT INTO backups.restore_points(id,partner_id,status,provider,created_by,expires_at) VALUES($1,$2,'QUEUED',$3,$4,$5)`,
		id,partnerID,a.provider.Name(),strings.TrimSpace(actor),expires)
	if err!=nil{return restorePoint{},err}
	point,err:=a.getRestorePoint(id);if err==nil{a.signal()};return point,err
}

func scanRestorePoint(s interface{Scan(...any)error})(restorePoint,error){
	var p restorePoint
	err:=s.Scan(&p.ID,&p.PartnerID,&p.Status,&p.Provider,&p.ObjectKey,&p.CiphertextSHA256,&p.CiphertextBytes,&p.Manifest,&p.CreatedBy,&p.Error,&p.CreatedAt,&p.CompletedAt,&p.ExpiresAt)
	return p,err
}

const restorePointSelect=`SELECT id,partner_id,status,provider,object_key,ciphertext_sha256,ciphertext_bytes,manifest,created_by,error,created_at,completed_at,expires_at FROM backups.restore_points`

func (a *app) getRestorePoint(id string)(restorePoint,error){return scanRestorePoint(a.db.QueryRow(restorePointSelect+` WHERE id=$1`,id))}

func mapRestorePoint(p restorePoint)map[string]any{
	var manifest any=map[string]any{};_ = json.Unmarshal(p.Manifest,&manifest)
	var completed,expires any;if p.CompletedAt.Valid{completed=p.CompletedAt.Time.UTC()};if p.ExpiresAt.Valid{expires=p.ExpiresAt.Time.UTC()}
	return map[string]any{
		"id":p.ID,"partner_id":p.PartnerID,"status":p.Status,"provider":p.Provider,"object_key":p.ObjectKey,
		"ciphertext_sha256":p.CiphertextSHA256,"ciphertext_bytes":p.CiphertextBytes,"manifest":manifest,
		"created_by":p.CreatedBy,"error":p.Error,"created_at":p.CreatedAt.UTC(),"completed_at":completed,"expires_at":expires,
	}
}

func scanRestoreTest(s interface{Scan(...any)error})(restoreTest,error){
	var t restoreTest
	err:=s.Scan(&t.ID,&t.RestorePointID,&t.PartnerID,&t.Status,&t.DatabaseOK,&t.MediaOK,&t.ConfigOK,&t.Error,&t.CreatedBy,&t.CreatedAt,&t.CompletedAt,&t.DurationMS)
	return t,err
}

const restoreTestSelect=`SELECT id,restore_point_id,partner_id,status,database_ok,media_ok,config_ok,error,created_by,created_at,completed_at,duration_ms FROM backups.restore_tests`

func mapRestoreTest(t restoreTest)map[string]any{
	var completed any;if t.CompletedAt.Valid{completed=t.CompletedAt.Time.UTC()}
	return map[string]any{
		"id":t.ID,"restore_point_id":t.RestorePointID,"partner_id":t.PartnerID,"status":t.Status,
		"database_ok":t.DatabaseOK,"media_ok":t.MediaOK,"config_ok":t.ConfigOK,"error":t.Error,
		"created_by":t.CreatedBy,"created_at":t.CreatedAt.UTC(),"completed_at":completed,"duration_ms":t.DurationMS,
	}
}

func validateS3Endpoint(raw string)(*url.URL,error){
	u,err:=url.Parse(strings.TrimSpace(raw))
	if err!=nil||u.Scheme!="https"||u.Host==""{return nil,fmt.Errorf("HIMATE_BACKUP_S3_ENDPOINT must be an HTTPS URL")}
	return u,nil
}
