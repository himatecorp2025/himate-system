package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"himate.local/services/internal/common"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type app struct{ db *sql.DB }

func main() {
	log := common.Logger()
	db, err := common.OpenDB()
	if err != nil { log.Error("database","error",err); os.Exit(1) }
	defer db.Close()
	a := &app{db:db}
	ctx,cancel:=context.WithTimeout(context.Background(),20*time.Second);defer cancel()
	if err:=a.migrate(ctx);err!=nil{log.Error("migration","error",err);os.Exit(1)}
	mux:=http.NewServeMux()
	mux.HandleFunc("/health",func(w http.ResponseWriter,r *http.Request){common.JSON(w,200,map[string]any{"status":"ok","service":"notifications","time":time.Now().UTC()})})
	mux.HandleFunc("/api/v1/notifications",a.feed)
	mux.HandleFunc("/api/v1/notifications/",a.notificationAction)
	mux.HandleFunc("/internal/v1/notifications/events",a.createEvent)
	common.Run(log,"notifications",common.Env("PORT","10000"),common.InternalAuth(os.Getenv("HIMATE_INTERNAL_TOKEN"),mux))
}

func (a *app) migrate(ctx context.Context) error {
	return common.ApplyMigrations(ctx,a.db,"notifications",[]common.Migration{
		{Version:1,Name:"notification-center",Statements:[]string{
			`CREATE SCHEMA IF NOT EXISTS notifications`,
			`CREATE TABLE IF NOT EXISTS notifications.events(
				id BIGSERIAL PRIMARY KEY,
				event_type TEXT NOT NULL,
				severity TEXT NOT NULL DEFAULT 'INFO',
				title TEXT NOT NULL,
				message TEXT NOT NULL DEFAULT '',
				resource TEXT NOT NULL DEFAULT '',
				partner_id TEXT NOT NULL DEFAULT '',
				deep_link TEXT NOT NULL DEFAULT '',
				audience_permission TEXT NOT NULL DEFAULT '',
				metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE INDEX IF NOT EXISTS notifications_events_created_idx ON notifications.events(created_at DESC,id DESC)`,
			`CREATE INDEX IF NOT EXISTS notifications_events_audience_idx ON notifications.events(audience_permission,created_at DESC)`,
			`CREATE TABLE IF NOT EXISTS notifications.read_state(
				event_id BIGINT NOT NULL REFERENCES notifications.events(id) ON DELETE CASCADE,
				user_id TEXT NOT NULL,
				read_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				PRIMARY KEY(event_id,user_id)
			)`,
			`CREATE INDEX IF NOT EXISTS notification_read_user_idx ON notifications.read_state(user_id,read_at DESC)`,
		}},
	})
}

func permissionSet(header string) map[string]bool {
	out:=map[string]bool{}
	for _,raw:=range strings.Split(header,","){p:=strings.TrimSpace(raw);if p!=""{out[p]=true}}
	return out
}

func visible(permission string, set map[string]bool) bool {
	permission=strings.TrimSpace(permission)
	return permission==""||set["*"]||set[permission]
}

func limitValue(raw string) int {
	n,err:=strconv.Atoi(strings.TrimSpace(raw));if err!=nil||n<1{return 40};if n>100{return 100};return n
}

func (a *app) feed(w http.ResponseWriter,r *http.Request) {
	if r.Method!=http.MethodGet{common.APIError(w,405,"METHOD","Use GET");return}
	userID:=strings.TrimSpace(r.Header.Get("X-Himate-User-ID"))
	if userID==""{common.APIError(w,401,"USER_REQUIRED","Authenticated user identity is required");return}
	permissions:=permissionSet(r.Header.Get("X-Himate-Permissions"))
	unreadOnly:=strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("unread_only")),"true")
	rows,err:=a.db.Query(`SELECT e.id,e.event_type,e.severity,e.title,e.message,e.resource,e.partner_id,e.deep_link,e.audience_permission,e.metadata,e.created_at,
		CASE WHEN rs.event_id IS NULL THEN FALSE ELSE TRUE END
		FROM notifications.events e
		LEFT JOIN notifications.read_state rs ON rs.event_id=e.id AND rs.user_id=$1
		ORDER BY e.created_at DESC,e.id DESC LIMIT $2`,userID,limitValue(r.URL.Query().Get("limit"))*3)
	if err!=nil{common.APIError(w,500,"DB","Could not load notifications");return}
	defer rows.Close()
	items:=[]map[string]any{};unread:=0
	for rows.Next(){
		var id int64;var eventType,severity,title,message,resource,partnerID,deepLink,audience string;var metadataRaw []byte;var created time.Time;var read bool
		if rows.Scan(&id,&eventType,&severity,&title,&message,&resource,&partnerID,&deepLink,&audience,&metadataRaw,&created,&read)!=nil{continue}
		if !visible(audience,permissions){continue}
		if !read{unread++}
		if unreadOnly&&read{continue}
		metadata:=map[string]any{};_ = json.Unmarshal(metadataRaw,&metadata)
		items=append(items,map[string]any{"id":id,"event_type":eventType,"severity":severity,"title":title,"message":message,"resource":resource,
			"partner_id":partnerID,"deep_link":deepLink,"audience_permission":audience,"metadata":metadata,"read":read,"created_at":created})
		if len(items)>=limitValue(r.URL.Query().Get("limit")){break}
	}
	common.JSON(w,200,map[string]any{"items":items,"count":len(items),"unread_count":unread})
}

func (a *app) notificationAction(w http.ResponseWriter,r *http.Request){
	userID:=strings.TrimSpace(r.Header.Get("X-Himate-User-ID"));if userID==""{common.APIError(w,401,"USER_REQUIRED","Authenticated user identity is required");return}
	raw:=strings.Trim(strings.TrimPrefix(r.URL.Path,"/api/v1/notifications/"),"/")
	if raw=="read-all"{
		if r.Method!=http.MethodPost{common.APIError(w,405,"METHOD","Use POST");return}
		permissions:=permissionSet(r.Header.Get("X-Himate-Permissions"))
		rows,err:=a.db.Query(`SELECT id,audience_permission FROM notifications.events ORDER BY created_at DESC LIMIT 500`);if err!=nil{common.APIError(w,500,"DB","Could not load notification state");return}
		defer rows.Close();ids:=[]int64{}
		for rows.Next(){var id int64;var audience string;if rows.Scan(&id,&audience)==nil&&visible(audience,permissions){ids=append(ids,id)}}
		tx,err:=a.db.Begin();if err!=nil{common.APIError(w,500,"DB","Could not update notifications");return};defer tx.Rollback()
		for _,id:=range ids{if _,err=tx.Exec(`INSERT INTO notifications.read_state(event_id,user_id) VALUES($1,$2) ON CONFLICT(event_id,user_id) DO UPDATE SET read_at=NOW()`,id,userID);err!=nil{common.APIError(w,500,"DB","Could not update notifications");return}}
		if err=tx.Commit();err!=nil{common.APIError(w,500,"DB","Could not commit notification state");return}
		common.JSON(w,200,map[string]any{"status":"read","count":len(ids)});return
	}
	parts:=strings.Split(raw,"/");if len(parts)!=2||parts[1]!="read"{common.APIError(w,404,"NOT_FOUND","Notification action not found");return}
	if r.Method!=http.MethodPost{common.APIError(w,405,"METHOD","Use POST");return}
	id,err:=strconv.ParseInt(parts[0],10,64);if err!=nil||id<1{common.APIError(w,404,"NOT_FOUND","Notification not found");return}
	var audience string
	if err=a.db.QueryRow(`SELECT audience_permission FROM notifications.events WHERE id=$1`,id).Scan(&audience);err!=nil{common.APIError(w,404,"NOT_FOUND","Notification not found");return}
	if !visible(audience,permissionSet(r.Header.Get("X-Himate-Permissions"))){common.APIError(w,403,"FORBIDDEN","Notification is outside your permission scope");return}
	_,err=a.db.Exec(`INSERT INTO notifications.read_state(event_id,user_id) VALUES($1,$2) ON CONFLICT(event_id,user_id) DO UPDATE SET read_at=NOW()`,id,userID)
	if err!=nil{common.APIError(w,500,"DB","Could not update notification");return}
	common.JSON(w,200,map[string]any{"id":id,"read":true})
}

func (a *app) createEvent(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodPost{common.APIError(w,405,"METHOD","Use POST");return}
	var in struct{
		EventType string `json:"event_type"`
		Severity string `json:"severity"`
		Title string `json:"title"`
		Message string `json:"message"`
		Resource string `json:"resource"`
		PartnerID string `json:"partner_id"`
		DeepLink string `json:"deep_link"`
		AudiencePermission string `json:"audience_permission"`
		Metadata map[string]any `json:"metadata"`
	}
	if common.Decode(r,&in)!=nil||strings.TrimSpace(in.EventType)==""||strings.TrimSpace(in.Title)==""{common.APIError(w,400,"VALIDATION","event_type and title are required");return}
	in.Severity=strings.ToUpper(strings.TrimSpace(in.Severity));if in.Severity==""{in.Severity="INFO"}
	if in.Severity!="INFO"&&in.Severity!="WARNING"&&in.Severity!="CRITICAL"{common.APIError(w,400,"VALIDATION","Invalid notification severity");return}
	raw,_:=json.Marshal(in.Metadata);if len(raw)==0{raw=[]byte("{}")}
	var id int64;var created time.Time
	err:=a.db.QueryRow(`INSERT INTO notifications.events(event_type,severity,title,message,resource,partner_id,deep_link,audience_permission,metadata)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb) RETURNING id,created_at`,
		strings.ToUpper(strings.TrimSpace(in.EventType)),in.Severity,strings.TrimSpace(in.Title),strings.TrimSpace(in.Message),strings.TrimSpace(in.Resource),
		strings.TrimSpace(in.PartnerID),strings.TrimSpace(in.DeepLink),strings.TrimSpace(in.AudiencePermission),string(raw)).Scan(&id,&created)
	if err!=nil{common.APIError(w,500,"DB","Could not create notification");return}
	common.JSON(w,201,map[string]any{"id":id,"created_at":created})
}
