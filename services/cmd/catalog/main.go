package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"himate.local/services/internal/common"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

type app struct{ db *sql.DB }
type seedModule struct{ Key, Label, Group string }
type seedGroup struct {
	Key, Label string
	Order      int
}

var seedGroups = []seedGroup{
	{"workshop", "Workshop", 1},
	{"finance_invoicing", "Finance & Invoicing", 2},
	{"technical", "Technical Operations", 3},
	{"marketing", "Marketing", 4},
	{"website_events", "Website & Events", 5},
	{"communication", "Communication", 6},
}

var seedModules = []seedModule{
	{"finance", "Balance Sheet", "finance_invoicing"},
	{"income_statement", "Income Statement", "finance_invoicing"},
	{"invoice_documents", "Invoices Documents", "finance_invoicing"},
	{"audit_log", "Audit Log", "technical"},
	{"backups", "Backups", "technical"},
	{"pianos", "Client Piano", "technical"},
	{"contacts", "Clients", "technical"},
	{"closed_jobs", "Closed Jobs", "technical"},
	{"knowledge_base", "Company Documents Archive", "technical"},
	{"company_data", "Corporate Data", "technical"},
	{"inventory", "Inventory", "technical"},
	{"partners", "Partners", "technical"},
	{"planned_jobs", "Planned Jobs", "technical"},
	{"scheduler", "Scheduler", "technical"},
	{"website_services", "Services", "technical"},
	{"settings", "Settings", "technical"},
	{"system_integrations", "System Activation & Integrations", "technical"},
	{"users", "Users", "technical"},
	{"workshop_workflow", "Workshop Workflow", "workshop"},
	{"marketing_overview", "Campaign Overview", "marketing"},
	{"customer_inbox", "Customer Inbox", "marketing"},
	{"website_reviews", "Reviews", "marketing"},
	{"campaigns_utm", "Campaigns & UTM", "marketing"},
	{"leads", "Leads", "marketing"},
	{"tracking_cookies", "Tracking & Cookies", "marketing"},
	{"seo_keywords", "SEO & Keywords", "marketing"},
	{"heatmap", "Consent Heatmap", "marketing"},
	{"website_artists", "Artists", "website_events"},
	{"website_contacts", "Contacts", "website_events"},
	{"digital_attendance", "Digital Attendance", "website_events"},
	{"events", "Events", "website_events"},
	{"event_guest_list", "Guest Data", "website_events"},
	{"event_invitations", "Invitations", "website_events"},
	{"media_library", "Media Library", "website_events"},
	{"pages_content", "Pages & Content", "website_events"},
	{"publish_preview", "Publish & Preview", "website_events"},
	{"showroom_pianos", "Showroom Pianos", "website_events"},
	{"event_tickets", "Ticket Reservation", "website_events"},
}

var moduleStates = map[string]bool{"ACTIVE": true, "NOT_LICENSED": true, "MAINTENANCE": true}
var availabilityValues = map[string]bool{"ACTIVE": true, "UNAVAILABLE": true, "DEPRECATED": true}
var moduleTypes = map[string]bool{"CORE": true, "FEATURE": true, "INTEGRATION": true, "REPORTING": true, "WEBSITE": true, "FINANCE": true, "INFRASTRUCTURE": true}
var relationshipTypes = map[string]bool{"REQUIRES": true, "OPTIONAL_DEPENDENCY": true, "INTEGRATES_WITH": true, "EXTENDS": true, "CONFLICTS_WITH": true, "REPLACES": true}
var moduleKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_.]{2,127}$`)

func main() {
	log := common.Logger()
	db, err := common.OpenDB()
	if err != nil {
		log.Error("database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	a := &app{db: db}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := a.migrate(ctx); err != nil {
		log.Error("migration", "error", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		common.JSON(w, 200, map[string]any{"status": "ok", "service": "catalog", "reference_modules": len(seedModules)})
	})
	mux.HandleFunc("/api/v1/module-groups", a.groups)
	mux.HandleFunc("/api/v1/module-groups/", a.groupByKey)
	mux.HandleFunc("/api/v1/modules", a.modules)
	mux.HandleFunc("/api/v1/modules/", a.moduleByKey)
	mux.HandleFunc("/api/v1/module-commercial-matrix", a.commercialMatrix)
	mux.HandleFunc("/api/v1/partners/", a.partnerModules)
	mux.HandleFunc("/internal/v1/partners/", a.partnerModules)
	mux.HandleFunc("/internal/v1/module-price-quotes", a.internalModulePriceQuotes)
	mux.HandleFunc("/internal/v1/partner-portal/", a.partnerPortal)
	mux.HandleFunc("/internal/v1/portfolio", a.portfolio)
	common.Run(log, "catalog", common.Env("PORT", "10000"), common.InternalAuth(os.Getenv("HIMATE_INTERNAL_TOKEN"), mux))
}

func (a *app) migrate(ctx context.Context) error {
	if err := common.ApplyMigrations(ctx, a.db, "catalog", []common.Migration{
		{Version: 1, Name: "catalog-base", Statements: []string{
			`CREATE SCHEMA IF NOT EXISTS catalog`,
			`CREATE TABLE IF NOT EXISTS catalog.module_groups(group_key TEXT PRIMARY KEY,label TEXT NOT NULL,sort_order INT NOT NULL)`,
			`CREATE TABLE IF NOT EXISTS catalog.modules(
				module_key TEXT PRIMARY KEY,label TEXT NOT NULL,group_key TEXT NOT NULL REFERENCES catalog.module_groups(group_key),
				description TEXT NOT NULL DEFAULT '',default_monthly_price NUMERIC(12,2) NOT NULL DEFAULT 0,
				currency TEXT NOT NULL DEFAULT 'USD',version TEXT NOT NULL DEFAULT '1.0.0',latest_version TEXT NOT NULL DEFAULT '1.0.0',
				last_updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),system BOOLEAN NOT NULL DEFAULT FALSE
			)`,
			`CREATE TABLE IF NOT EXISTS catalog.partner_modules(
				partner_id TEXT NOT NULL,module_key TEXT NOT NULL REFERENCES catalog.modules(module_key),
				status TEXT NOT NULL DEFAULT 'NOT_LICENSED',visible BOOLEAN NOT NULL DEFAULT FALSE,included_in_base BOOLEAN NOT NULL DEFAULT FALSE,
				price_override NUMERIC(12,2),updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),PRIMARY KEY(partner_id,module_key)
			)`,
			`CREATE TABLE IF NOT EXISTS catalog.price_history(
				id BIGSERIAL PRIMARY KEY,partner_id TEXT NOT NULL,module_key TEXT NOT NULL,old_price NUMERIC(12,2),new_price NUMERIC(12,2) NOT NULL,
				effective_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),reason TEXT NOT NULL DEFAULT ''
			)`,
		}},
		{Version: 2, Name: "availability-and-history", Statements: []string{
			`ALTER TABLE catalog.modules ADD COLUMN IF NOT EXISTS availability TEXT NOT NULL DEFAULT 'ACTIVE'`,
			`ALTER TABLE catalog.price_history ADD COLUMN IF NOT EXISTS actor TEXT NOT NULL DEFAULT ''`,
			`CREATE TABLE IF NOT EXISTS catalog.partner_module_history(
				id BIGSERIAL PRIMARY KEY,
				partner_id TEXT NOT NULL,
				module_key TEXT NOT NULL,
				field_name TEXT NOT NULL,
				old_value TEXT,
				new_value TEXT,
				effective_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				actor TEXT NOT NULL DEFAULT '',
				reason TEXT NOT NULL DEFAULT ''
			)`,
			`CREATE INDEX IF NOT EXISTS partner_module_history_lookup ON catalog.partner_module_history(partner_id,module_key,effective_at DESC)`,
			`CREATE INDEX IF NOT EXISTS partner_modules_status_idx ON catalog.partner_modules(partner_id,status)`,
		}},
		{Version: 3, Name: "module-activation-timestamp", Statements: []string{
			`ALTER TABLE catalog.partner_modules ADD COLUMN IF NOT EXISTS activated_at TIMESTAMPTZ`,
			`UPDATE catalog.partner_modules SET activated_at=updated_at WHERE status='ACTIVE' AND activated_at IS NULL`,
		}},
		{Version: 4, Name: "module-control-plane-registry", Statements: []string{
			`ALTER TABLE catalog.modules ADD COLUMN IF NOT EXISTS module_type TEXT NOT NULL DEFAULT 'FEATURE'`,
			`ALTER TABLE catalog.modules ADD COLUMN IF NOT EXISTS owner_team TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE catalog.modules ADD COLUMN IF NOT EXISTS source_repository TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE catalog.modules ADD COLUMN IF NOT EXISTS source_path TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE catalog.modules ADD COLUMN IF NOT EXISTS source_ref TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE catalog.modules ADD COLUMN IF NOT EXISTS source_commit TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE catalog.modules ADD COLUMN IF NOT EXISTS artifact_type TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE catalog.modules ADD COLUMN IF NOT EXISTS artifact_reference TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE catalog.modules ADD COLUMN IF NOT EXISTS min_platform_version TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE catalog.modules ADD COLUMN IF NOT EXISTS manifest JSONB NOT NULL DEFAULT '{}'::jsonb`,
			`CREATE TABLE IF NOT EXISTS catalog.module_relationships(
				module_key TEXT NOT NULL REFERENCES catalog.modules(module_key) ON DELETE CASCADE,
				target_module_key TEXT NOT NULL REFERENCES catalog.modules(module_key) ON DELETE CASCADE,
				relation_type TEXT NOT NULL,
				note TEXT NOT NULL DEFAULT '',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				PRIMARY KEY(module_key,target_module_key,relation_type),
				CHECK(module_key<>target_module_key)
			)`,
			`CREATE INDEX IF NOT EXISTS module_relationship_target_idx ON catalog.module_relationships(target_module_key,relation_type)`,
			`CREATE TABLE IF NOT EXISTS catalog.module_impact_metrics(
				module_key TEXT NOT NULL REFERENCES catalog.modules(module_key) ON DELETE CASCADE,
				metric_key TEXT NOT NULL,
				label TEXT NOT NULL DEFAULT '',
				PRIMARY KEY(module_key,metric_key)
			)`,
			`CREATE INDEX IF NOT EXISTS module_impact_metric_idx ON catalog.module_impact_metrics(metric_key)`,
		}},
		{Version: 5, Name: "partner-module-commercial-control", Statements: []string{
			`ALTER TABLE catalog.modules ADD COLUMN IF NOT EXISTS default_activation_fee NUMERIC(12,2) NOT NULL DEFAULT 0`,
			`ALTER TABLE catalog.partner_modules ADD COLUMN IF NOT EXISTS activation_fee_override NUMERIC(12,2)`,
			`CREATE TABLE IF NOT EXISTS catalog.activation_fee_history(
				id BIGSERIAL PRIMARY KEY,
				partner_id TEXT NOT NULL,
				module_key TEXT NOT NULL,
				old_fee NUMERIC(12,2),
				new_fee NUMERIC(12,2) NOT NULL,
				effective_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				actor TEXT NOT NULL DEFAULT '',
				reason TEXT NOT NULL DEFAULT ''
			)`,
			`CREATE INDEX IF NOT EXISTS activation_fee_history_lookup ON catalog.activation_fee_history(partner_id,module_key,effective_at DESC,id DESC)`,
		}},
		{Version: 6, Name: "start-23-5-bilingual-catalog-records", Statements: []string{
			`ALTER TABLE catalog.module_groups ADD COLUMN IF NOT EXISTS label_en TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE catalog.module_groups ADD COLUMN IF NOT EXISTS label_hu TEXT NOT NULL DEFAULT ''`,
			`UPDATE catalog.module_groups SET label_en=label WHERE label_en=''`,
			`UPDATE catalog.module_groups SET label_hu=label WHERE label_hu=''`,
			`ALTER TABLE catalog.modules ADD COLUMN IF NOT EXISTS label_en TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE catalog.modules ADD COLUMN IF NOT EXISTS label_hu TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE catalog.modules ADD COLUMN IF NOT EXISTS description_en TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE catalog.modules ADD COLUMN IF NOT EXISTS description_hu TEXT NOT NULL DEFAULT ''`,
			`UPDATE catalog.modules SET label_en=label WHERE label_en=''`,
			`UPDATE catalog.modules SET label_hu=label WHERE label_hu=''`,
			`UPDATE catalog.modules SET description_en=description WHERE description_en=''`,
			`UPDATE catalog.modules SET description_hu=description WHERE description_hu=''`,
		}},
	}); err != nil {
		return err
	}

	for _, g := range seedGroups {
		if _, err := a.db.ExecContext(ctx, `INSERT INTO catalog.module_groups(group_key,label,label_en,label_hu,sort_order) VALUES($1,$2,$2,$2,$3) ON CONFLICT(group_key) DO UPDATE SET label=EXCLUDED.label,label_en=EXCLUDED.label_en,label_hu=CASE WHEN label_hu='' THEN EXCLUDED.label_hu ELSE label_hu END,sort_order=EXCLUDED.sort_order`, g.Key, g.Label, g.Order); err != nil {
			return err
		}
	}
	for _, m := range seedModules {
		if _, err := a.db.ExecContext(ctx, `INSERT INTO catalog.modules(module_key,label,label_en,label_hu,group_key,description,description_en,description_hu,system,availability) VALUES($1,$2,$2,$2,$3,'Klavierhaus verified reference module','Klavierhaus verified reference module','Klavierhaus verified reference module',TRUE,'ACTIVE') ON CONFLICT(module_key) DO UPDATE SET label=EXCLUDED.label,label_en=EXCLUDED.label_en,label_hu=CASE WHEN label_hu='' THEN EXCLUDED.label_hu ELSE label_hu END,group_key=EXCLUDED.group_key,system=TRUE`, m.Key, m.Label, m.Group); err != nil {
			return err
		}
		if _, err := a.db.ExecContext(ctx, `INSERT INTO catalog.partner_modules(partner_id,module_key,status,visible,included_in_base,price_override,activated_at) VALUES('ptr_000001',$1,'ACTIVE',TRUE,TRUE,0,NOW()) ON CONFLICT(partner_id,module_key) DO NOTHING`, m.Key); err != nil {
			return err
		}
	}
	return nil
}

func (a *app) groups(w http.ResponseWriter, r *http.Request) {
	locale:=common.RequestLocale(r)
	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query(`SELECT group_key,label_en,label_hu,sort_order FROM catalog.module_groups ORDER BY sort_order,lower(label_en)`)
		if err != nil { common.APIError(w,500,"DB","Could not load module groups"); return }
		defer rows.Close()
		items := []map[string]any{}
		for rows.Next() {
			var k,en,hu string; var s int
			if rows.Scan(&k,&en,&hu,&s)==nil {
				items=append(items,map[string]any{"group_key":k,"label":common.Localized(en,hu,locale),"label_en":en,"label_hu":hu,"sort_order":s})
			}
		}
		common.JSON(w,200,map[string]any{"items":items,"count":len(items),"locale":locale})
	case http.MethodPost:
		var in struct {
			Key string `json:"group_key"`
			Label string `json:"label"`
			LabelEN string `json:"label_en"`
			LabelHU string `json:"label_hu"`
			SortOrder int `json:"sort_order"`
		}
		if common.Decode(r,&in)!=nil || !moduleKeyPattern.MatchString(in.Key) {
			common.APIError(w,400,"VALIDATION","Stable group key and bilingual labels are required"); return
		}
		en:=strings.TrimSpace(in.LabelEN); hu:=strings.TrimSpace(in.LabelHU); legacy:=strings.TrimSpace(in.Label)
		if en==""{en=legacy}; if hu==""{hu=legacy}
		if en==""||hu==""{common.APIError(w,400,"VALIDATION","English and Hungarian group labels are required");return}
		if in.SortOrder<=0 { _ = a.db.QueryRow(`SELECT COALESCE(MAX(sort_order),0)+1 FROM catalog.module_groups`).Scan(&in.SortOrder) }
		if _,err:=a.db.Exec(`INSERT INTO catalog.module_groups(group_key,label,label_en,label_hu,sort_order) VALUES($1,$2,$2,$3,$4)`,in.Key,en,hu,in.SortOrder);err!=nil{
			common.APIError(w,409,"CONFLICT","Module group could not be created");return
		}
		common.JSON(w,201,map[string]any{"group_key":in.Key,"label":common.Localized(en,hu,locale),"label_en":en,"label_hu":hu,"sort_order":in.SortOrder})
	default:
		common.APIError(w,405,"METHOD","Use GET or POST")
	}
}

func (a *app) groupByKey(w http.ResponseWriter, r *http.Request) {
	key:=strings.Trim(strings.TrimPrefix(r.URL.Path,"/api/v1/module-groups/"),"/")
	if key==""||strings.Contains(key,"/"){common.APIError(w,404,"NOT_FOUND","Module group not found");return}
	if r.Method!=http.MethodPatch{common.APIError(w,405,"METHOD","Use PATCH");return}
	locale:=common.RequestLocale(r)
	var in struct{Label *string `json:"label"`; LabelEN *string `json:"label_en"`; LabelHU *string `json:"label_hu"`; SortOrder *int `json:"sort_order"`}
	if common.Decode(r,&in)!=nil{common.APIError(w,400,"JSON","Invalid request");return}
	var en,hu string; var order int
	if err:=a.db.QueryRow(`SELECT label_en,label_hu,sort_order FROM catalog.module_groups WHERE group_key=$1`,key).Scan(&en,&hu,&order);err!=nil{
		common.APIError(w,404,"NOT_FOUND","Module group not found");return
	}
	if in.Label!=nil { legacy:=strings.TrimSpace(*in.Label); if in.LabelEN==nil{en=legacy}; if in.LabelHU==nil{hu=legacy} }
	if in.LabelEN!=nil{en=strings.TrimSpace(*in.LabelEN)}
	if in.LabelHU!=nil{hu=strings.TrimSpace(*in.LabelHU)}
	if in.SortOrder!=nil{order=*in.SortOrder}
	if en==""||hu==""||order<=0{common.APIError(w,400,"VALIDATION","Valid bilingual labels and sort order are required");return}
	if _,err:=a.db.Exec(`UPDATE catalog.module_groups SET label=$2,label_en=$2,label_hu=$3,sort_order=$4 WHERE group_key=$1`,key,en,hu,order);err!=nil{
		common.APIError(w,409,"CONFLICT","Module group could not be updated");return
	}
	common.JSON(w,200,map[string]any{"group_key":key,"label":common.Localized(en,hu,locale),"label_en":en,"label_hu":hu,"sort_order":order})
}

func (a *app) modules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query(`SELECT m.module_key,m.label_en,m.label_hu,m.group_key,g.label_en,g.label_hu,m.description_en,m.description_hu,m.default_monthly_price,m.default_activation_fee,m.currency,m.version,m.latest_version,
			m.last_updated_at,m.system,m.availability,m.module_type,m.owner_team,m.source_repository,m.source_path,m.source_ref,m.source_commit,
			m.artifact_type,m.artifact_reference,m.min_platform_version,m.manifest,
			(SELECT COUNT(*) FROM catalog.module_relationships mr WHERE mr.module_key=m.module_key),
			(SELECT COUNT(*) FROM catalog.partner_modules pm WHERE pm.module_key=m.module_key AND pm.status='ACTIVE'),
			(SELECT COUNT(*) FROM catalog.module_impact_metrics mm WHERE mm.module_key=m.module_key)
			FROM catalog.modules m JOIN catalog.module_groups g ON g.group_key=m.group_key ORDER BY g.sort_order,m.label`)
		if err != nil { common.APIError(w,500,"DB","Could not load modules"); return }
		defer rows.Close()
		items := []map[string]any{}
		for rows.Next() {
			var k,labelEN,labelHU,g,groupEN,groupHU,descEN,descHU,currency,v,lv,availability,moduleType,owner,repo,path,ref,commit,artifactType,artifactRef,minPlatform string
			var p,activationFee float64; var t time.Time; var sys bool; var manifestRaw []byte; var relCount,usageCount,metricCount int
			if rows.Scan(&k,&labelEN,&labelHU,&g,&groupEN,&groupHU,&descEN,&descHU,&p,&activationFee,&currency,&v,&lv,&t,&sys,&availability,&moduleType,&owner,&repo,&path,&ref,&commit,&artifactType,&artifactRef,&minPlatform,&manifestRaw,&relCount,&usageCount,&metricCount)==nil {
				manifest:=map[string]any{}; _=json.Unmarshal(manifestRaw,&manifest)
				locale:=common.RequestLocale(r)
				items=append(items,map[string]any{"key":k,"label":common.Localized(labelEN,labelHU,locale),"label_en":labelEN,"label_hu":labelHU,"group_key":g,"group_label":common.Localized(groupEN,groupHU,locale),"group_label_en":groupEN,"group_label_hu":groupHU,"description":common.Localized(descEN,descHU,locale),"description_en":descEN,"description_hu":descHU,"default_monthly_price":p,"default_activation_fee":activationFee,"currency":currency,
					"version":v,"latest_version":lv,"last_updated_at":t,"system":sys,"availability":availability,"module_type":moduleType,"owner_team":owner,
					"source_repository":repo,"source_path":path,"source_ref":ref,"source_commit":commit,"artifact_type":artifactType,"artifact_reference":artifactRef,
					"min_platform_version":minPlatform,"manifest":manifest,"relationship_count":relCount,"active_partner_count":usageCount,"impact_metric_count":metricCount})
			}
		}
		common.JSON(w,200,map[string]any{"items":items,"count":len(items)})
	case http.MethodPost:
		var in struct {
			Key string `json:"key"`
			Label string `json:"label"`
			LabelEN string `json:"label_en"`
			LabelHU string `json:"label_hu"`
			GroupKey string `json:"group_key"`
			Description string `json:"description"`
			DescriptionEN string `json:"description_en"`
			DescriptionHU string `json:"description_hu"`
			Currency string `json:"currency"`
			Version string `json:"version"`
			LatestVersion string `json:"latest_version"`
			Availability string `json:"availability"`
			ModuleType string `json:"module_type"`
			OwnerTeam string `json:"owner_team"`
			SourceRepository string `json:"source_repository"`
			SourcePath string `json:"source_path"`
			SourceRef string `json:"source_ref"`
			SourceCommit string `json:"source_commit"`
			ArtifactType string `json:"artifact_type"`
			ArtifactReference string `json:"artifact_reference"`
			MinPlatformVersion string `json:"min_platform_version"`
			DefaultMonthlyPrice float64 `json:"default_monthly_price"`
			DefaultActivationFee float64 `json:"default_activation_fee"`
			Manifest map[string]any `json:"manifest"`
		}
		if common.Decode(r,&in)!=nil || !moduleKeyPattern.MatchString(in.Key) || strings.TrimSpace(in.GroupKey)=="" {
			common.APIError(w,400,"VALIDATION","Stable key, bilingual labels and group are required");return
		}
		labelEN:=strings.TrimSpace(in.LabelEN); labelHU:=strings.TrimSpace(in.LabelHU); legacyLabel:=strings.TrimSpace(in.Label)
		if labelEN==""{labelEN=legacyLabel}; if labelHU==""{labelHU=legacyLabel}
		descEN:=strings.TrimSpace(in.DescriptionEN); descHU:=strings.TrimSpace(in.DescriptionHU); legacyDesc:=strings.TrimSpace(in.Description)
		if descEN==""{descEN=legacyDesc}; if descHU==""{descHU=legacyDesc}
		if labelEN==""||labelHU==""{common.APIError(w,400,"VALIDATION","English and Hungarian module labels are required");return}
		if in.Currency==""{in.Currency="USD"}; if in.Version==""{in.Version="1.0.0"}; if in.LatestVersion==""{in.LatestVersion=in.Version}
		if in.Availability==""{in.Availability="ACTIVE"}; in.ModuleType=strings.ToUpper(strings.TrimSpace(in.ModuleType)); if in.ModuleType==""{in.ModuleType="FEATURE"}
		if !availabilityValues[in.Availability] || !moduleTypes[in.ModuleType] || in.DefaultMonthlyPrice<0 || in.DefaultActivationFee<0 { common.APIError(w,400,"VALIDATION","Invalid module metadata");return }
		manifest,_:=json.Marshal(in.Manifest); if len(manifest)==0{manifest=[]byte("{}")}
		_,err:=a.db.Exec(`INSERT INTO catalog.modules(
			module_key,label,label_en,label_hu,group_key,description,description_en,description_hu,default_monthly_price,default_activation_fee,currency,version,latest_version,system,availability,module_type,owner_team,
			source_repository,source_path,source_ref,source_commit,artifact_type,artifact_reference,min_platform_version,manifest)
			VALUES($1,$2,$2,$3,$4,$5,$5,$6,$7,$8,$9,$10,$11,FALSE,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22::jsonb)`,
			in.Key,labelEN,labelHU,in.GroupKey,descEN,descHU,in.DefaultMonthlyPrice,in.DefaultActivationFee,in.Currency,in.Version,in.LatestVersion,
			in.Availability,in.ModuleType,strings.TrimSpace(in.OwnerTeam),strings.TrimSpace(in.SourceRepository),strings.TrimSpace(in.SourcePath),
			strings.TrimSpace(in.SourceRef),strings.TrimSpace(in.SourceCommit),strings.TrimSpace(in.ArtifactType),strings.TrimSpace(in.ArtifactReference),
			strings.TrimSpace(in.MinPlatformVersion),string(manifest))
		if err!=nil{common.APIError(w,409,"CONFLICT","Module could not be created");return}
		common.JSON(w,201,map[string]any{"key":in.Key,"label":common.Localized(labelEN,labelHU,common.RequestLocale(r)),"label_en":labelEN,"label_hu":labelHU,"description_en":descEN,"description_hu":descHU,"group_key":in.GroupKey,"default_monthly_price":in.DefaultMonthlyPrice,"default_activation_fee":in.DefaultActivationFee,
			"currency":in.Currency,"version":in.Version,"latest_version":in.LatestVersion,"availability":in.Availability,"module_type":in.ModuleType,"system":false})
	default:
		common.APIError(w,405,"METHOD","Use GET or POST")
	}
}

func (a *app) moduleByKey(w http.ResponseWriter, r *http.Request) {
	raw:=strings.Trim(strings.TrimPrefix(r.URL.Path,"/api/v1/modules/"),"/")
	parts:=strings.Split(raw,"/")
	if len(parts)==0||parts[0]==""{common.APIError(w,404,"NOT_FOUND","Module not found");return}
	key:=parts[0]
	if len(parts)>1 {
		switch parts[1] {
		case "relationships": a.moduleRelationships(w,r,key,parts[2:]); return
		case "impact-metrics": a.moduleImpactMetrics(w,r,key); return
		case "usage": a.moduleUsage(w,r,key); return
		default: common.APIError(w,404,"NOT_FOUND","Module subresource not found"); return
		}
	}
	if r.Method!=http.MethodPatch{common.APIError(w,405,"METHOD","Use PATCH");return}
	locale:=common.RequestLocale(r)
	var in struct {
		Label *string `json:"label"`
		LabelEN *string `json:"label_en"`
		LabelHU *string `json:"label_hu"`
		Description *string `json:"description"`
		DescriptionEN *string `json:"description_en"`
		DescriptionHU *string `json:"description_hu"`
		GroupKey *string `json:"group_key"`
		Availability *string `json:"availability"`
		LatestVersion *string `json:"latest_version"`
		ModuleType *string `json:"module_type"`
		OwnerTeam *string `json:"owner_team"`
		SourceRepository *string `json:"source_repository"`
		SourcePath *string `json:"source_path"`
		SourceRef *string `json:"source_ref"`
		SourceCommit *string `json:"source_commit"`
		ArtifactType *string `json:"artifact_type"`
		ArtifactReference *string `json:"artifact_reference"`
		MinPlatformVersion *string `json:"min_platform_version"`
		DefaultMonthlyPrice *float64 `json:"default_monthly_price"`
		DefaultActivationFee *float64 `json:"default_activation_fee"`
		Manifest map[string]any `json:"manifest"`
	}
	if common.Decode(r,&in)!=nil{common.APIError(w,400,"JSON","Invalid request");return}
	var labelEN,labelHU,descEN,descHU,groupKey,availability,latestVersion,moduleType,owner,repo,path,ref,commit,artifactType,artifactRef,minPlatform string
	var price,activationFee float64; var manifestRaw []byte
	if err:=a.db.QueryRow(`SELECT label_en,label_hu,description_en,description_hu,group_key,default_monthly_price,default_activation_fee,availability,latest_version,module_type,owner_team,source_repository,source_path,
		source_ref,source_commit,artifact_type,artifact_reference,min_platform_version,manifest FROM catalog.modules WHERE module_key=$1`,key).
		Scan(&labelEN,&labelHU,&descEN,&descHU,&groupKey,&price,&activationFee,&availability,&latestVersion,&moduleType,&owner,&repo,&path,&ref,&commit,&artifactType,&artifactRef,&minPlatform,&manifestRaw);err!=nil{
		common.APIError(w,404,"NOT_FOUND","Module not found");return
	}
	if in.Label!=nil{legacy:=strings.TrimSpace(*in.Label);if in.LabelEN==nil{labelEN=legacy};if in.LabelHU==nil{labelHU=legacy}}
	if in.LabelEN!=nil{labelEN=strings.TrimSpace(*in.LabelEN)}
	if in.LabelHU!=nil{labelHU=strings.TrimSpace(*in.LabelHU)}
	if in.Description!=nil{legacy:=strings.TrimSpace(*in.Description);if in.DescriptionEN==nil{descEN=legacy};if in.DescriptionHU==nil{descHU=legacy}}
	if in.DescriptionEN!=nil{descEN=strings.TrimSpace(*in.DescriptionEN)}
	if in.DescriptionHU!=nil{descHU=strings.TrimSpace(*in.DescriptionHU)}
	set:=func(dst *string,src *string){if src!=nil{*dst=strings.TrimSpace(*src)}}
	set(&groupKey,in.GroupKey);set(&availability,in.Availability);set(&latestVersion,in.LatestVersion);set(&moduleType,in.ModuleType);set(&owner,in.OwnerTeam)
	set(&repo,in.SourceRepository);set(&path,in.SourcePath);set(&ref,in.SourceRef);set(&commit,in.SourceCommit);set(&artifactType,in.ArtifactType);set(&artifactRef,in.ArtifactReference);set(&minPlatform,in.MinPlatformVersion)
	moduleType=strings.ToUpper(moduleType); if in.DefaultMonthlyPrice!=nil{price=*in.DefaultMonthlyPrice}; if in.DefaultActivationFee!=nil{activationFee=*in.DefaultActivationFee}
	if labelEN==""||labelHU==""||price<0||activationFee<0||!availabilityValues[availability]||!moduleTypes[moduleType]{common.APIError(w,400,"VALIDATION","Invalid bilingual module update");return}
	if in.Manifest!=nil{manifestRaw,_=json.Marshal(in.Manifest)}
	if _,err:=a.db.Exec(`UPDATE catalog.modules SET label=$2,label_en=$2,label_hu=$3,description=$4,description_en=$4,description_hu=$5,group_key=$6,
		default_monthly_price=$7,default_activation_fee=$8,availability=$9,latest_version=$10,module_type=$11,owner_team=$12,source_repository=$13,source_path=$14,
		source_ref=$15,source_commit=$16,artifact_type=$17,artifact_reference=$18,min_platform_version=$19,manifest=$20::jsonb,last_updated_at=NOW()
		WHERE module_key=$1`,key,labelEN,labelHU,descEN,descHU,groupKey,price,activationFee,availability,latestVersion,moduleType,owner,repo,path,ref,commit,artifactType,artifactRef,minPlatform,string(manifestRaw));err!=nil{
		common.APIError(w,409,"CONFLICT","Module could not be updated");return
	}
	common.JSON(w,200,map[string]any{
		"key":key,"label":common.Localized(labelEN,labelHU,locale),"label_en":labelEN,"label_hu":labelHU,
		"description":common.Localized(descEN,descHU,locale),"description_en":descEN,"description_hu":descHU,
		"group_key":groupKey,"default_monthly_price":price,"default_activation_fee":activationFee,"availability":availability,
		"latest_version":latestVersion,"module_type":moduleType,"owner_team":owner,"source_repository":repo,"source_path":path,"source_ref":ref,"source_commit":commit,
		"artifact_type":artifactType,"artifact_reference":artifactRef,"min_platform_version":minPlatform,
	})
}

func (a *app) moduleRelationships(w http.ResponseWriter,r *http.Request,key string,tail []string){
	if len(tail)>0 {
		if r.Method!=http.MethodDelete{common.APIError(w,405,"METHOD","Use DELETE");return}
		target:=tail[0]; relation:=strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("type")))
		if !relationshipTypes[relation]{common.APIError(w,400,"VALIDATION","Valid relationship type is required");return}
		if res,err:=a.db.Exec(`DELETE FROM catalog.module_relationships WHERE module_key=$1 AND target_module_key=$2 AND relation_type=$3`,key,target,relation);err!=nil{
			common.APIError(w,500,"DB","Could not delete relationship");return
		}else if n,_:=res.RowsAffected();n==0{common.APIError(w,404,"NOT_FOUND","Relationship not found");return}
		w.WriteHeader(http.StatusNoContent);return
	}
	switch r.Method{
	case http.MethodGet:
		rows,err:=a.db.Query(`SELECT r.target_module_key,m.label,r.relation_type,r.note,r.updated_at FROM catalog.module_relationships r
			JOIN catalog.modules m ON m.module_key=r.target_module_key WHERE r.module_key=$1 ORDER BY r.relation_type,m.label`,key)
		if err!=nil{common.APIError(w,500,"DB","Could not load module relationships");return};defer rows.Close()
		items:=[]map[string]any{};for rows.Next(){var target,label,relation,note string;var updated time.Time;if rows.Scan(&target,&label,&relation,&note,&updated)==nil{
			items=append(items,map[string]any{"target_module_key":target,"target_label":label,"relation_type":relation,"note":note,"updated_at":updated})}}
		common.JSON(w,200,map[string]any{"items":items,"count":len(items)})
	case http.MethodPost:
		var in struct{TargetModuleKey string `json:"target_module_key"`;RelationType string `json:"relation_type"`;Note string `json:"note"`}
		if common.Decode(r,&in)!=nil{common.APIError(w,400,"JSON","Invalid request");return};in.RelationType=strings.ToUpper(strings.TrimSpace(in.RelationType))
		if in.TargetModuleKey==""||in.TargetModuleKey==key||!relationshipTypes[in.RelationType]{common.APIError(w,400,"VALIDATION","Valid target and relationship type are required");return}
		if _,err:=a.db.Exec(`INSERT INTO catalog.module_relationships(module_key,target_module_key,relation_type,note) VALUES($1,$2,$3,$4)
			ON CONFLICT(module_key,target_module_key,relation_type) DO UPDATE SET note=EXCLUDED.note,updated_at=NOW()`,key,in.TargetModuleKey,in.RelationType,strings.TrimSpace(in.Note));err!=nil{
			common.APIError(w,409,"CONFLICT","Module relationship could not be saved");return}
		common.JSON(w,201,map[string]any{"module_key":key,"target_module_key":in.TargetModuleKey,"relation_type":in.RelationType,"note":strings.TrimSpace(in.Note)})
	default:common.APIError(w,405,"METHOD","Use GET or POST")
	}
}

func (a *app) moduleImpactMetrics(w http.ResponseWriter,r *http.Request,key string){
	switch r.Method{
	case http.MethodGet:
		rows,err:=a.db.Query(`SELECT metric_key,label FROM catalog.module_impact_metrics WHERE module_key=$1 ORDER BY metric_key`,key)
		if err!=nil{common.APIError(w,500,"DB","Could not load module metrics");return};defer rows.Close()
		items:=[]map[string]any{};for rows.Next(){var metric,label string;if rows.Scan(&metric,&label)==nil{items=append(items,map[string]any{"metric_key":metric,"label":label})}}
		common.JSON(w,200,map[string]any{"items":items,"count":len(items)})
	case http.MethodPut:
		var in struct{Items []struct{MetricKey string `json:"metric_key"`;Label string `json:"label"`} `json:"items"`}
		if common.Decode(r,&in)!=nil{common.APIError(w,400,"JSON","Invalid request");return}
		tx,err:=a.db.Begin();if err!=nil{common.APIError(w,500,"DB","Could not start metric update");return};defer tx.Rollback()
		if _,err=tx.Exec(`DELETE FROM catalog.module_impact_metrics WHERE module_key=$1`,key);err!=nil{common.APIError(w,500,"DB","Could not reset metric mapping");return}
		seen:=map[string]bool{};for _,item:=range in.Items{metric:=strings.TrimSpace(item.MetricKey);if metric==""||seen[metric]{continue};seen[metric]=true
			if _,err=tx.Exec(`INSERT INTO catalog.module_impact_metrics(module_key,metric_key,label) VALUES($1,$2,$3)`,key,metric,strings.TrimSpace(item.Label));err!=nil{
				common.APIError(w,500,"DB","Could not save metric mapping");return}}
		if err=tx.Commit();err!=nil{common.APIError(w,500,"DB","Could not commit metric mapping");return}
		common.JSON(w,200,map[string]any{"module_key":key,"count":len(seen)})
	default:common.APIError(w,405,"METHOD","Use GET or PUT")
	}
}

func (a *app) moduleUsage(w http.ResponseWriter,r *http.Request,key string){
	if r.Method!=http.MethodGet{common.APIError(w,405,"METHOD","Use GET");return}
	rows,err:=a.db.Query(partnerModuleSelect+` WHERE pm.module_key=$1 ORDER BY pm.partner_id`,key)
	if err!=nil{common.APIError(w,500,"DB","Could not load module usage");return}
	defer rows.Close()
	items:=[]map[string]any{}
	counts:=map[string]int{}
	for rows.Next(){
		item,scanErr:=scanPartnerModule(rows, common.RequestLocale(r))
		if scanErr!=nil{common.APIError(w,500,"DB","Could not decode module usage");return}
		counts[fmt.Sprint(item["status"])]++
		items=append(items,item)
	}
	if err:=rows.Err();err!=nil{common.APIError(w,500,"DB","Could not load complete module usage");return}
	common.JSON(w,200,map[string]any{"items":items,"count":len(items),"status_counts":counts})
}

func (a *app) ensurePartnerModules(partnerID string) error {
	_, err := a.db.Exec(`INSERT INTO catalog.partner_modules(partner_id,module_key,status,visible,included_in_base) SELECT $1,module_key,'NOT_LICENSED',FALSE,FALSE FROM catalog.modules ON CONFLICT(partner_id,module_key) DO NOTHING`, partnerID)
	return err
}

func (a *app) partnerModules(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Path
	internal := strings.HasPrefix(raw, "/internal/v1/partners/")
	if internal {
		raw = strings.TrimPrefix(raw, "/internal/v1/partners/")
	} else {
		raw = strings.TrimPrefix(raw, "/api/v1/partners/")
	}
	parts := strings.Split(strings.Trim(raw, "/"), "/")
	if len(parts) < 2 {
		common.APIError(w, 404, "NOT_FOUND", "Route not found")
		return
	}
	partnerID := parts[0]
	if err := a.ensurePartnerModules(partnerID); err != nil {
		common.APIError(w, 500, "DB", "Could not initialize partner module state")
		return
	}
	if parts[1] == "billable-modules" {
		if !internal {
			common.APIError(w, 404, "NOT_FOUND", "Route not found")
			return
		}
		a.listPartnerModules(w, partnerID, true, common.RequestLocale(r))
		return
	}
	if parts[1] != "modules" {
		common.APIError(w, 404, "NOT_FOUND", "Route not found")
		return
	}
	if internal && len(parts) == 4 && parts[3] == "price-at" {
		if r.Method != http.MethodGet {
			common.APIError(w, 405, "METHOD", "Use GET")
			return
		}
		a.partnerModulePriceAt(w, r, partnerID, parts[2])
		return
	}
	if len(parts) == 4 && parts[3] == "commercial-history" {
		if r.Method != http.MethodGet {
			common.APIError(w, 405, "METHOD", "Use GET")
			return
		}
		a.partnerModuleCommercialHistory(w, partnerID, parts[2])
		return
	}
	if len(parts) == 2 {
		if r.Method != http.MethodGet {
			common.APIError(w, 405, "METHOD", "Use GET")
			return
		}
		a.listPartnerModules(w, partnerID, false, common.RequestLocale(r))
		return
	}
	if r.Method != http.MethodPatch {
		common.APIError(w, 405, "METHOD", "Use PATCH")
		return
	}
	key := parts[2]
	var in struct {
		Status         *string  `json:"status"`
		Visible        *bool    `json:"visible"`
		IncludedInBase *bool    `json:"included_in_base"`
		PartnerPrice         *float64 `json:"partner_price"`
		PriceEffectiveAt     string   `json:"price_effective_at"`
		PartnerActivationFee *float64 `json:"partner_activation_fee"`
		ActivationFeeEffectiveAt string `json:"activation_fee_effective_at"`
		Reason           string   `json:"reason"`
	}
	if common.Decode(r, &in) != nil {
		common.APIError(w, 400, "JSON", "Invalid request")
		return
	}
	tx, err := a.db.Begin()
	if err != nil {
		common.APIError(w, 500, "DB", "Could not start update")
		return
	}
	defer tx.Rollback()
	actor := strings.TrimSpace(r.Header.Get("X-Himate-User-ID"))
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		reason = "HIMATE administrator update"
	}
	effectiveAt := time.Now().UTC()
	if strings.TrimSpace(in.PriceEffectiveAt) != "" {
		rawEffectiveAt := strings.TrimSpace(in.PriceEffectiveAt)
		parsed, parseErr := time.Parse(time.RFC3339, rawEffectiveAt)
		if parseErr != nil {
			parsed, parseErr = time.Parse("2006-01-02", rawEffectiveAt)
		}
		if parseErr != nil {
			common.APIError(w, 400, "VALIDATION", "price_effective_at must be RFC3339 or YYYY-MM-DD")
			return
		}
		effectiveAt = parsed.UTC()
	}

	recordHistory := func(field string, oldValue, newValue any, at time.Time) error {
		return func() error {
			_, e := tx.Exec(`INSERT INTO catalog.partner_module_history(partner_id,module_key,field_name,old_value,new_value,effective_at,actor,reason) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`,
				partnerID, key, field, fmt.Sprint(oldValue), fmt.Sprint(newValue), at, actor, reason)
			return e
		}()
	}

	if in.Status != nil {
		if !moduleStates[*in.Status] {
			common.APIError(w, 400, "VALIDATION", "Invalid module state")
			return
		}
		var old string
		if err = tx.QueryRow(`SELECT status FROM catalog.partner_modules WHERE partner_id=$1 AND module_key=$2`, partnerID, key).Scan(&old); err != nil {
			common.APIError(w, 404, "NOT_FOUND", "Module not found")
			return
		}
		if old != *in.Status {
			if *in.Status == "NOT_LICENSED" && old != "NOT_LICENSED" && !internal {
				common.APIError(w, 409, "BILLING_LIFECYCLE_REQUIRED", "Module deactivation is Billing-owned; schedule period-end cancellation through Billing")
				return
			}
			if _, err = tx.Exec(`UPDATE catalog.partner_modules SET
					status=$3,
					activated_at=CASE WHEN $3='ACTIVE' THEN NOW() ELSE activated_at END,
					updated_at=NOW()
				WHERE partner_id=$1 AND module_key=$2`, partnerID, key, *in.Status); err != nil {
				common.APIError(w, 500, "DB", "Could not update module state")
				return
			}
			if err = recordHistory("status", old, *in.Status, time.Now().UTC()); err != nil {
				common.APIError(w, 500, "DB", "Could not save module history")
				return
			}
		}
	}
	if in.Visible != nil {
		var old bool
		_ = tx.QueryRow(`SELECT visible FROM catalog.partner_modules WHERE partner_id=$1 AND module_key=$2`, partnerID, key).Scan(&old)
		if old != *in.Visible {
			if _, err = tx.Exec(`UPDATE catalog.partner_modules SET visible=$3,updated_at=NOW() WHERE partner_id=$1 AND module_key=$2`, partnerID, key, *in.Visible); err != nil {
				common.APIError(w, 500, "DB", "Could not update visibility")
				return
			}
			if err = recordHistory("visible", old, *in.Visible, time.Now().UTC()); err != nil { common.APIError(w, 500, "DB", "Could not save module history"); return }
		}
	}
	if in.IncludedInBase != nil {
		var old bool
		_ = tx.QueryRow(`SELECT included_in_base FROM catalog.partner_modules WHERE partner_id=$1 AND module_key=$2`, partnerID, key).Scan(&old)
		if old != *in.IncludedInBase {
			if _, err = tx.Exec(`UPDATE catalog.partner_modules SET included_in_base=$3,updated_at=NOW() WHERE partner_id=$1 AND module_key=$2`, partnerID, key, *in.IncludedInBase); err != nil {
				common.APIError(w, 500, "DB", "Could not update base package")
				return
			}
			if err = recordHistory("included_in_base", old, *in.IncludedInBase, time.Now().UTC()); err != nil { common.APIError(w, 500, "DB", "Could not save module history"); return }
		}
	}
	if in.PartnerPrice != nil {
		if *in.PartnerPrice < 0 {
			common.APIError(w, 400, "VALIDATION", "Price cannot be negative")
			return
		}
		var oldValue float64
		if err = tx.QueryRow(`
			SELECT COALESCE(
				(SELECT ph.new_price FROM catalog.price_history ph
				 WHERE ph.partner_id=pm.partner_id AND ph.module_key=pm.module_key AND ph.effective_at<=NOW()
				 ORDER BY ph.effective_at DESC,ph.id DESC LIMIT 1),
				pm.price_override,m.default_monthly_price)
			FROM catalog.partner_modules pm
			JOIN catalog.modules m ON m.module_key=pm.module_key
			WHERE pm.partner_id=$1 AND pm.module_key=$2`, partnerID, key).Scan(&oldValue); err != nil {
			common.APIError(w, 404, "NOT_FOUND", "Module not found")
			return
		}
		if !effectiveAt.After(time.Now().UTC()) {
			if _, err = tx.Exec(`UPDATE catalog.partner_modules SET price_override=$3,updated_at=NOW() WHERE partner_id=$1 AND module_key=$2`, partnerID, key, *in.PartnerPrice); err != nil {
				common.APIError(w, 500, "DB", "Could not update price")
				return
			}
		}
		if _, err = tx.Exec(`INSERT INTO catalog.price_history(partner_id,module_key,old_price,new_price,effective_at,actor,reason) VALUES($1,$2,$3,$4,$5,$6,$7)`, partnerID, key, oldValue, *in.PartnerPrice, effectiveAt, actor, reason); err != nil {
			common.APIError(w, 500, "DB", "Could not save price history")
			return
		}
		if err = recordHistory("partner_price", oldValue, *in.PartnerPrice, effectiveAt); err != nil { common.APIError(w, 500, "DB", "Could not save module history"); return }
	}
	if in.PartnerActivationFee != nil {
		if *in.PartnerActivationFee < 0 {
			common.APIError(w, 400, "VALIDATION", "Activation fee cannot be negative")
			return
		}
		activationEffectiveAt := time.Now().UTC()
		if strings.TrimSpace(in.ActivationFeeEffectiveAt) != "" {
			rawEffectiveAt := strings.TrimSpace(in.ActivationFeeEffectiveAt)
			parsed, parseErr := time.Parse(time.RFC3339, rawEffectiveAt)
			if parseErr != nil {
				parsed, parseErr = time.Parse("2006-01-02", rawEffectiveAt)
			}
			if parseErr != nil {
				common.APIError(w, 400, "VALIDATION", "activation_fee_effective_at must be RFC3339 or YYYY-MM-DD")
				return
			}
			activationEffectiveAt = parsed.UTC()
		}
		var oldFee float64
		if err = tx.QueryRow(`
			SELECT COALESCE(
				(SELECT ah.new_fee FROM catalog.activation_fee_history ah
				 WHERE ah.partner_id=pm.partner_id AND ah.module_key=pm.module_key AND ah.effective_at<=NOW()
				 ORDER BY ah.effective_at DESC,ah.id DESC LIMIT 1),
				pm.activation_fee_override,m.default_activation_fee)
			FROM catalog.partner_modules pm
			JOIN catalog.modules m ON m.module_key=pm.module_key
			WHERE pm.partner_id=$1 AND pm.module_key=$2`, partnerID, key).Scan(&oldFee); err != nil {
			common.APIError(w, 404, "NOT_FOUND", "Module not found")
			return
		}
		if !activationEffectiveAt.After(time.Now().UTC()) {
			if _, err = tx.Exec(`UPDATE catalog.partner_modules SET activation_fee_override=$3,updated_at=NOW() WHERE partner_id=$1 AND module_key=$2`, partnerID, key, *in.PartnerActivationFee); err != nil {
				common.APIError(w, 500, "DB", "Could not update activation fee")
				return
			}
		}
		if _, err = tx.Exec(`INSERT INTO catalog.activation_fee_history(partner_id,module_key,old_fee,new_fee,effective_at,actor,reason)
			VALUES($1,$2,$3,$4,$5,$6,$7)`, partnerID, key, oldFee, *in.PartnerActivationFee, activationEffectiveAt, actor, reason); err != nil {
			common.APIError(w, 500, "DB", "Could not save activation-fee history")
			return
		}
		if err = recordHistory("partner_activation_fee", oldFee, *in.PartnerActivationFee, activationEffectiveAt); err != nil {
			common.APIError(w, 500, "DB", "Could not save module history")
			return
		}
	}
	if err = tx.Commit(); err != nil {
		common.APIError(w, 500, "DB", "Could not commit module update")
		return
	}
	a.onePartnerModule(w, partnerID, key, common.RequestLocale(r))
}

func (a *app) partnerModuleCommercialHistory(w http.ResponseWriter, partnerID, key string) {
	rows, err := a.db.Query(`
		SELECT field_name,old_value,new_value,effective_at,actor,reason
		FROM catalog.partner_module_history
		WHERE partner_id=$1 AND module_key=$2
		ORDER BY effective_at DESC,id DESC
		LIMIT 250`, partnerID, key)
	if err != nil {
		common.APIError(w, 500, "DB", "Could not load partner-module commercial history")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var field string
		var oldValue, newValue sql.NullString
		var effectiveAt time.Time
		var actor, reason string
		if err := rows.Scan(&field, &oldValue, &newValue, &effectiveAt, &actor, &reason); err != nil {
			common.APIError(w, 500, "DB", "Could not decode partner-module commercial history")
			return
		}
		var oldOut, newOut any
		if oldValue.Valid { oldOut = oldValue.String }
		if newValue.Valid { newOut = newValue.String }
		items = append(items, map[string]any{
			"field": field, "old_value": oldOut, "new_value": newOut,
			"effective_at": effectiveAt.UTC(), "actor": actor, "reason": reason,
		})
	}
	if err := rows.Err(); err != nil {
		common.APIError(w, 500, "DB", "Could not load complete partner-module commercial history")
		return
	}
	common.JSON(w, 200, map[string]any{
		"partner_id": partnerID, "module_key": key, "items": items, "count": len(items),
	})
}

func (a *app) listPartnerModules(w http.ResponseWriter, partnerID string, billable bool, locale string) {
	q := partnerModuleSelect + ` WHERE pm.partner_id=$1`
	if billable {
		q += ` AND pm.status='ACTIVE' AND m.availability='ACTIVE'`
	}
	q += ` ORDER BY g.sort_order,m.label`
	rows, err := a.db.Query(q, partnerID)
	if err != nil {
		common.APIError(w, 500, "DB", "Could not load partner modules")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	extra := 0.0
	for rows.Next() {
		item, err := scanPartnerModule(rows, locale)
		if err != nil {
			continue
		}
		items = append(items, item)
		if billable && item["included_in_base"] != true {
			extra += item["partner_price"].(float64)
		}
	}
	out := map[string]any{"items": items, "count": len(items)}
	if billable {
		out["extra_monthly_total"] = extra
	}
	common.JSON(w, 200, out)
}

func (a *app) resolvePartnerModulePriceAt(ctx context.Context, partnerID, key string, at time.Time) (float64, string, bool, error) {
	var price float64
	var currency string
	var included bool
	err := a.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(
				(SELECT ph.new_price FROM catalog.price_history ph
				 WHERE ph.partner_id=pm.partner_id AND ph.module_key=pm.module_key AND ph.effective_at<=$3
				 ORDER BY ph.effective_at DESC,ph.id DESC LIMIT 1),
				(SELECT ph.old_price FROM catalog.price_history ph
				 WHERE ph.partner_id=pm.partner_id AND ph.module_key=pm.module_key AND ph.effective_at>$3 AND ph.old_price IS NOT NULL
				 ORDER BY ph.effective_at ASC,ph.id ASC LIMIT 1),
				pm.price_override,m.default_monthly_price
			),
			m.currency,
			COALESCE(
				(SELECT CASE WHEN lower(pmh.new_value)='true' THEN TRUE WHEN lower(pmh.new_value)='false' THEN FALSE END
				 FROM catalog.partner_module_history pmh
				 WHERE pmh.partner_id=pm.partner_id AND pmh.module_key=pm.module_key AND pmh.field_name='included_in_base' AND pmh.effective_at<=$3
				 ORDER BY pmh.effective_at DESC,pmh.id DESC LIMIT 1),
				(SELECT CASE WHEN lower(pmh.old_value)='true' THEN TRUE WHEN lower(pmh.old_value)='false' THEN FALSE END
				 FROM catalog.partner_module_history pmh
				 WHERE pmh.partner_id=pm.partner_id AND pmh.module_key=pm.module_key AND pmh.field_name='included_in_base' AND pmh.effective_at>$3
				 ORDER BY pmh.effective_at ASC,pmh.id ASC LIMIT 1),
				pm.included_in_base
			)
		FROM catalog.partner_modules pm
		JOIN catalog.modules m ON m.module_key=pm.module_key
		WHERE pm.partner_id=$1 AND pm.module_key=$2`, partnerID, key, at).Scan(&price, &currency, &included)
	return price, currency, included, err
}

func (a *app) partnerModulePriceAt(w http.ResponseWriter, r *http.Request, partnerID, key string) {
	rawAt := strings.TrimSpace(r.URL.Query().Get("at"))
	at := time.Now().UTC()
	if rawAt != "" {
		parsed, err := time.Parse("2006-01-02", rawAt)
		if err != nil {
			common.APIError(w, 400, "VALIDATION", "at must be YYYY-MM-DD")
			return
		}
		at = parsed.UTC()
	}
	price, currency, included, err := a.resolvePartnerModulePriceAt(r.Context(), partnerID, key, at)
	if err != nil {
		if err == sql.ErrNoRows {
			common.APIError(w, 404, "NOT_FOUND", "Partner module not found")
		} else {
			common.APIError(w, 500, "DB", "Could not resolve historical partner module price")
		}
		return
	}
	common.JSON(w, 200, map[string]any{
		"partner_id": partnerID, "module_key": key, "at": at.Format("2006-01-02"),
		"price": price, "currency": currency, "included_in_base": included,
		"source": "CATALOG_EFFECTIVE_PRICE_HISTORY",
	})
}

func (a *app) internalModulePriceQuotes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		common.APIError(w, 405, "METHOD", "Use POST")
		return
	}
	var in struct {
		Items []struct {
			PartnerID string `json:"partner_id"`
			ModuleKey string `json:"module_key"`
			At        string `json:"at"`
		} `json:"items"`
	}
	if common.Decode(r, &in) != nil || len(in.Items) == 0 || len(in.Items) > 2000 {
		common.APIError(w, 400, "VALIDATION", "One to 2000 price quote items are required")
		return
	}
	items := make([]map[string]any, 0, len(in.Items))
	for _, item := range in.Items {
		partnerID := strings.TrimSpace(item.PartnerID)
		moduleKey := strings.TrimSpace(item.ModuleKey)
		at, err := time.Parse("2006-01-02", strings.TrimSpace(item.At))
		if partnerID == "" || moduleKey == "" || err != nil {
			common.APIError(w, 400, "VALIDATION", "Each quote requires partner_id, module_key and YYYY-MM-DD at")
			return
		}
		price, currency, included, err := a.resolvePartnerModulePriceAt(r.Context(), partnerID, moduleKey, at.UTC())
		if err != nil {
			if err == sql.ErrNoRows {
				common.APIError(w, 404, "NOT_FOUND", "Partner-module quote target not found")
			} else {
				common.APIError(w, 500, "DB", "Could not resolve partner-module price quote")
			}
			return
		}
		items = append(items, map[string]any{
			"partner_id": partnerID, "module_key": moduleKey, "at": at.Format("2006-01-02"),
			"price": price, "currency": currency, "included_in_base": included,
			"source": "CATALOG_EFFECTIVE_PRICE_HISTORY",
		})
	}
	common.JSON(w, 200, map[string]any{"items": items, "count": len(items)})
}

func (a *app) onePartnerModule(w http.ResponseWriter, partnerID, key, locale string) {
	item, err := scanPartnerModule(a.db.QueryRow(partnerModuleSelect+` WHERE pm.partner_id=$1 AND pm.module_key=$2`, partnerID, key), locale)
	if err != nil {
		common.APIError(w, 404, "NOT_FOUND", "Module not found")
		return
	}
	common.JSON(w, 200, item)
}

func splitPartnerIDs(raw string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range strings.Split(raw, ",") {
		id := strings.TrimSpace(value)
		if id == "" || seen[id] { continue }
		seen[id] = true
		out = append(out, id)
		if len(out) >= 200 { break }
	}
	return out
}

func (a *app) commercialMatrix(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.APIError(w, 405, "METHOD", "Use GET")
		return
	}
	ids := splitPartnerIDs(r.URL.Query().Get("partner_ids"))
	if len(ids) == 0 {
		common.JSON(w, 200, map[string]any{"items": []map[string]any{}, "count": 0})
		return
	}
	for _, id := range ids {
		if err := a.ensurePartnerModules(id); err != nil {
			common.APIError(w, 500, "DB", "Could not initialize partner module matrix")
			return
		}
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	query := partnerModuleSelect + ` WHERE pm.partner_id IN (` + strings.Join(placeholders, ",") + `) ORDER BY pm.partner_id,g.sort_order,m.label`
	rows, err := a.db.Query(query, args...)
	if err != nil {
		common.APIError(w, 500, "DB", "Could not load partner-module commercial matrix")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		item, scanErr := scanPartnerModule(rows, common.RequestLocale(r))
		if scanErr != nil {
			common.APIError(w, 500, "DB", "Could not decode partner-module commercial matrix")
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		common.APIError(w, 500, "DB", "Could not load complete partner-module commercial matrix")
		return
	}
	common.JSON(w, 200, map[string]any{"items": items, "count": len(items)})
}

func (a *app) portfolio(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.APIError(w, 405, "METHOD", "Use GET")
		return
	}
	query := `
		SELECT pm.partner_id,
			COUNT(*) FILTER (WHERE pm.status='ACTIVE' AND m.availability='ACTIVE'),
			COALESCE(SUM(CASE WHEN pm.status='ACTIVE' AND m.availability='ACTIVE' AND pm.included_in_base=FALSE
				THEN COALESCE(ep.new_price,pm.price_override,m.default_monthly_price) ELSE 0 END),0),
			MAX(pm.updated_at)
		FROM catalog.partner_modules pm
		JOIN catalog.modules m ON m.module_key=pm.module_key
		LEFT JOIN LATERAL (
			SELECT ph.new_price
			FROM catalog.price_history ph
			WHERE ph.partner_id=pm.partner_id AND ph.module_key=pm.module_key AND ph.effective_at<=NOW()
			ORDER BY ph.effective_at DESC,ph.id DESC
			LIMIT 1
		) ep ON TRUE`
	args := []any{}
	if ids := strings.TrimSpace(r.URL.Query().Get("ids")); ids != "" {
		query += ` WHERE pm.partner_id = ANY(string_to_array($1, ','))`
		args = append(args, ids)
	}
	query += ` GROUP BY pm.partner_id ORDER BY pm.partner_id`
	rows, err := a.db.Query(query, args...)
	if err != nil {
		common.APIError(w, 500, "DB", "Could not load partner module portfolio")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id string
		var active int
		var extra float64
		var updated time.Time
		if rows.Scan(&id, &active, &extra, &updated) == nil {
			items = append(items, map[string]any{"partner_id": id, "active_modules": active, "extra_module_fee": extra, "updated_at": updated})
		}
	}
	common.JSON(w, 200, map[string]any{"items": items})
}


const partnerModuleSelect = `SELECT
	pm.partner_id,m.module_key,m.label_en,m.label_hu,m.group_key,g.label_en,g.label_hu,pm.status,pm.visible,pm.included_in_base,
	m.default_monthly_price,pm.price_override,COALESCE(ep.new_price,pm.price_override,m.default_monthly_price),
	CASE WHEN ep.new_price IS NOT NULL THEN 'PARTNER_HISTORY' WHEN pm.price_override IS NOT NULL THEN 'PARTNER_OVERRIDE' ELSE 'MODULE_DEFAULT' END,
	np.new_price,np.effective_at,
	m.default_activation_fee,pm.activation_fee_override,COALESCE(eaf.new_fee,pm.activation_fee_override,m.default_activation_fee),
	CASE WHEN eaf.new_fee IS NOT NULL THEN 'PARTNER_HISTORY' WHEN pm.activation_fee_override IS NOT NULL THEN 'PARTNER_OVERRIDE' ELSE 'MODULE_DEFAULT' END,
	naf.new_fee,naf.effective_at,
	m.currency,m.version,m.latest_version,m.last_updated_at,m.availability,pm.activated_at
	FROM catalog.partner_modules pm
	JOIN catalog.modules m ON m.module_key=pm.module_key
	JOIN catalog.module_groups g ON g.group_key=m.group_key
	LEFT JOIN LATERAL (
		SELECT ph.new_price FROM catalog.price_history ph
		WHERE ph.partner_id=pm.partner_id AND ph.module_key=pm.module_key AND ph.effective_at<=NOW()
		ORDER BY ph.effective_at DESC,ph.id DESC LIMIT 1
	) ep ON TRUE
	LEFT JOIN LATERAL (
		SELECT ph.new_price,ph.effective_at FROM catalog.price_history ph
		WHERE ph.partner_id=pm.partner_id AND ph.module_key=pm.module_key AND ph.effective_at>NOW()
		ORDER BY ph.effective_at ASC,ph.id ASC LIMIT 1
	) np ON TRUE
	LEFT JOIN LATERAL (
		SELECT ah.new_fee FROM catalog.activation_fee_history ah
		WHERE ah.partner_id=pm.partner_id AND ah.module_key=pm.module_key AND ah.effective_at<=NOW()
		ORDER BY ah.effective_at DESC,ah.id DESC LIMIT 1
	) eaf ON TRUE
	LEFT JOIN LATERAL (
		SELECT ah.new_fee,ah.effective_at FROM catalog.activation_fee_history ah
		WHERE ah.partner_id=pm.partner_id AND ah.module_key=pm.module_key AND ah.effective_at>NOW()
		ORDER BY ah.effective_at ASC,ah.id ASC LIMIT 1
	) naf ON TRUE`

type scanner interface{ Scan(...any) error }

func nullableFloat(v sql.NullFloat64) any {
	if !v.Valid { return nil }
	return v.Float64
}

func nullableTime(v sql.NullTime) any {
	if !v.Valid { return nil }
	return v.Time.UTC()
}

func scanPartnerModule(s scanner, locale string) (map[string]any, error) {
	var id, k, labelEN, labelHU, g, groupEN, groupHU, st, currency, v, lv, availability, priceSource, activationSource string
	var vis, inc bool
	var defPrice, price, defaultActivationFee, activationFee float64
	var priceOverride, nextPrice, activationOverride, nextActivationFee sql.NullFloat64
	var nextPriceAt, nextActivationFeeAt, activated sql.NullTime
	var t time.Time
	err := s.Scan(
		&id,&k,&labelEN,&labelHU,&g,&groupEN,&groupHU,&st,&vis,&inc,
		&defPrice,&priceOverride,&price,&priceSource,&nextPrice,&nextPriceAt,
		&defaultActivationFee,&activationOverride,&activationFee,&activationSource,&nextActivationFee,&nextActivationFeeAt,
		&currency,&v,&lv,&t,&availability,&activated,
	)
	return map[string]any{
		"partner_id": id, "key": k, "label": common.Localized(labelEN,labelHU,locale), "label_en": labelEN, "label_hu": labelHU,
		"group_key": g, "group_label": common.Localized(groupEN,groupHU,locale), "group_label_en": groupEN, "group_label_hu": groupHU,
		"status": st, "visible": vis, "included_in_base": inc,
		"default_monthly_price": defPrice, "price_override": nullableFloat(priceOverride),
		"partner_price": price, "price_source": priceSource,
		"next_partner_price": nullableFloat(nextPrice), "next_price_effective_at": nullableTime(nextPriceAt),
		"default_activation_fee": defaultActivationFee, "activation_fee_override": nullableFloat(activationOverride),
		"partner_activation_fee": activationFee, "activation_fee_source": activationSource,
		"next_partner_activation_fee": nullableFloat(nextActivationFee), "next_activation_fee_effective_at": nullableTime(nextActivationFeeAt),
		"currency": currency, "version": v, "latest_version": lv, "last_updated_at": t,
		"availability": availability, "activated_at": nullableTime(activated),
	}, err
}
