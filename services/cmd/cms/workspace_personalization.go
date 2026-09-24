package main

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"himate.local/services/internal/common"
)

type workspaceSettings struct {
	PartnerID        string
	WorkspaceName    string
	LogoMediaID      sql.NullString
	PrimaryColor     string
	SidebarColor     string
	BackgroundColor  string
	AccentColor      string
	TextColor        string
	DefaultModuleKey string
	UpdatedBy        string
}

type modulePresentation struct {
	PartnerID         string
	ModuleKey         string
	DisplayName       string
	Description       string
	IconKey           string
	CustomIconMediaID sql.NullString
	CardColor         string
	UpdatedBy         string
}

var workspaceModuleKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_.]{2,127}$`)

var workspaceIconLibrary = []map[string]string{
	{"key":"default","label":"Default"},
	{"key":"finance","label":"Finance"},
	{"key":"workflow","label":"Workflow"},
	{"key":"inventory","label":"Inventory"},
	{"key":"calendar","label":"Calendar"},
	{"key":"crm","label":"CRM / Clients"},
	{"key":"marketing","label":"Marketing"},
	{"key":"events","label":"Events"},
	{"key":"analytics","label":"Analytics"},
	{"key":"documents","label":"Documents"},
	{"key":"settings","label":"Settings"},
	{"key":"integrations","label":"Integrations"},
	{"key":"users","label":"Users"},
	{"key":"support","label":"Support"},
}

func defaultWorkspaceSettings(partnerID string) workspaceSettings {
	return workspaceSettings{
		PartnerID: partnerID,
		PrimaryColor: "#0B1F3B",
		SidebarColor: "#06172C",
		BackgroundColor: "#F8F9FB",
		AccentColor: "#D4AF6B",
		TextColor: "#1F2937",
	}
}

func (a *app) getPartnerWorkspaceSettings(ctx context.Context, partnerID string) (workspaceSettings, error) {
	out:=defaultWorkspaceSettings(partnerID)
	err:=a.db.QueryRowContext(ctx,`SELECT partner_id,workspace_name,logo_media_id,primary_color,sidebar_color,background_color,accent_color,text_color,default_module_key,updated_by
		FROM cms.partner_workspace_settings WHERE partner_id=$1`,partnerID).
		Scan(&out.PartnerID,&out.WorkspaceName,&out.LogoMediaID,&out.PrimaryColor,&out.SidebarColor,&out.BackgroundColor,&out.AccentColor,&out.TextColor,&out.DefaultModuleKey,&out.UpdatedBy)
	if err==sql.ErrNoRows{return out,nil}
	return out,err
}

func workspaceSettingsMap(row workspaceSettings) map[string]any {
	logoID:=""
	if row.LogoMediaID.Valid{logoID=row.LogoMediaID.String}
	logoURL:=""
	if logoID!=""{logoURL="/public/v1/cms/media/"+logoID}
	return map[string]any{
		"partner_id":row.PartnerID,
		"workspace_name":row.WorkspaceName,
		"logo_media_id":logoID,
		"logo_url":logoURL,
		"primary_color":row.PrimaryColor,
		"sidebar_color":row.SidebarColor,
		"background_color":row.BackgroundColor,
		"accent_color":row.AccentColor,
		"text_color":row.TextColor,
		"default_module_key":row.DefaultModuleKey,
		"presentation_only":true,
	}
}

func (a *app) listPartnerModulePresentations(ctx context.Context, partnerID string) ([]map[string]any,error) {
	rows,err:=a.db.QueryContext(ctx,`SELECT partner_id,module_key,display_name,description,icon_key,custom_icon_media_id,card_color,updated_by
		FROM cms.partner_module_presentations WHERE partner_id=$1 ORDER BY module_key`,partnerID)
	if err!=nil{return nil,err}
	defer rows.Close()
	out:=[]map[string]any{}
	for rows.Next(){
		var row modulePresentation
		if err:=rows.Scan(&row.PartnerID,&row.ModuleKey,&row.DisplayName,&row.Description,&row.IconKey,&row.CustomIconMediaID,&row.CardColor,&row.UpdatedBy);err!=nil{return nil,err}
		mediaID:=""
		if row.CustomIconMediaID.Valid{mediaID=row.CustomIconMediaID.String}
		iconURL:=""
		if mediaID!=""{iconURL="/public/v1/cms/media/"+mediaID}
		out=append(out,map[string]any{
			"partner_id":row.PartnerID,
			"module_key":row.ModuleKey,
			"display_name":row.DisplayName,
			"description":row.Description,
			"icon_key":row.IconKey,
			"custom_icon_media_id":mediaID,
			"custom_icon_url":iconURL,
			"card_color":row.CardColor,
			"presentation_only":true,
		})
	}
	return out,rows.Err()
}

func parseWorkspaceHex(value string) (int64,error) {
	value=strings.TrimPrefix(strings.TrimSpace(value),"#")
	if len(value)!=6{return 0,fmt.Errorf("color must be a six-digit hexadecimal value")}
	return strconv.ParseInt(value,16,64)
}

func workspaceChannel(v float64) float64 {
	v/=255
	if v<=0.04045{return v/12.92}
	return math.Pow((v+0.055)/1.055,2.4)
}

func workspaceLuminance(color string) (float64,error) {
	raw,err:=parseWorkspaceHex(color);if err!=nil{return 0,err}
	r:=workspaceChannel(float64((raw>>16)&255))
	g:=workspaceChannel(float64((raw>>8)&255))
	b:=workspaceChannel(float64(raw&255))
	return .2126*r+.7152*g+.0722*b,nil
}

func workspaceContrast(a,b string) (float64,error) {
	la,err:=workspaceLuminance(a);if err!=nil{return 0,err}
	lb,err:=workspaceLuminance(b);if err!=nil{return 0,err}
	if la<lb{la,lb=lb,la}
	return (la+.05)/(lb+.05),nil
}

func validateWorkspaceColors(primary,sidebar,background,accent,text string) error {
	for name,value:=range map[string]string{
		"primary_color":primary,"sidebar_color":sidebar,"background_color":background,"accent_color":accent,"text_color":text,
	}{
		if !designColorPattern.MatchString(strings.ToUpper(strings.TrimSpace(value))){
			return fmt.Errorf("%s must use #RRGGBB",name)
		}
	}
	ratio,err:=workspaceContrast(background,text);if err!=nil{return err}
	if ratio<4.5{return fmt.Errorf("text_color and background_color must have at least 4.5:1 contrast")}
	return nil
}

func workspaceIconAllowed(value string) bool {
	if strings.TrimSpace(value)==""{return true}
	for _,item:=range workspaceIconLibrary{if item["key"]==value{return true}}
	return false
}

func (a *app) partnerWorkspaceInternal(w http.ResponseWriter,r *http.Request,partnerID string) {
	if r.Method!=http.MethodPut{common.APIError(w,http.StatusMethodNotAllowed,"METHOD","Use PUT");return}
	var in struct{
		WorkspaceName string `json:"workspace_name"`
		LogoMediaID string `json:"logo_media_id"`
		PrimaryColor string `json:"primary_color"`
		SidebarColor string `json:"sidebar_color"`
		BackgroundColor string `json:"background_color"`
		AccentColor string `json:"accent_color"`
		TextColor string `json:"text_color"`
		DefaultModuleKey string `json:"default_module_key"`
	}
	if common.Decode(r,&in)!=nil{common.APIError(w,http.StatusBadRequest,"JSON","Invalid workspace personalization request");return}
	in.WorkspaceName=strings.TrimSpace(in.WorkspaceName)
	in.LogoMediaID=strings.TrimSpace(in.LogoMediaID)
	in.PrimaryColor=strings.ToUpper(strings.TrimSpace(in.PrimaryColor))
	in.SidebarColor=strings.ToUpper(strings.TrimSpace(in.SidebarColor))
	in.BackgroundColor=strings.ToUpper(strings.TrimSpace(in.BackgroundColor))
	in.AccentColor=strings.ToUpper(strings.TrimSpace(in.AccentColor))
	in.TextColor=strings.ToUpper(strings.TrimSpace(in.TextColor))
	in.DefaultModuleKey=strings.TrimSpace(in.DefaultModuleKey)
	if len(in.WorkspaceName)>100{common.APIError(w,400,"VALIDATION","workspace_name must be at most 100 characters");return}
	if in.DefaultModuleKey!=""&&!workspaceModuleKeyPattern.MatchString(in.DefaultModuleKey){common.APIError(w,400,"VALIDATION","default_module_key is invalid");return}
	if in.LogoMediaID!=""&&!a.partnerOwnsMedia(r.Context(),partnerID,in.LogoMediaID){common.APIError(w,403,"MEDIA_SCOPE","Workspace logo must belong to this partner");return}
	if err:=validateWorkspaceColors(in.PrimaryColor,in.SidebarColor,in.BackgroundColor,in.AccentColor,in.TextColor);err!=nil{common.APIError(w,400,"CONTRAST",err.Error());return}
	old,_:=a.getPartnerWorkspaceSettings(r.Context(),partnerID)
	_,err:=a.db.ExecContext(r.Context(),`INSERT INTO cms.partner_workspace_settings(
		partner_id,workspace_name,logo_media_id,primary_color,sidebar_color,background_color,accent_color,text_color,default_module_key,updated_by,updated_at
	) VALUES($1,$2,NULLIF($3,''),$4,$5,$6,$7,$8,$9,$10,NOW())
	ON CONFLICT(partner_id) DO UPDATE SET
		workspace_name=EXCLUDED.workspace_name,logo_media_id=EXCLUDED.logo_media_id,primary_color=EXCLUDED.primary_color,
		sidebar_color=EXCLUDED.sidebar_color,background_color=EXCLUDED.background_color,accent_color=EXCLUDED.accent_color,
		text_color=EXCLUDED.text_color,default_module_key=EXCLUDED.default_module_key,updated_by=EXCLUDED.updated_by,updated_at=NOW()`,
		partnerID,in.WorkspaceName,in.LogoMediaID,in.PrimaryColor,in.SidebarColor,in.BackgroundColor,in.AccentColor,in.TextColor,in.DefaultModuleKey,actor(r))
	if err!=nil{common.APIError(w,500,"DB","Could not save workspace personalization");return}
	next,_:=a.getPartnerWorkspaceSettings(r.Context(),partnerID)
	_ = a.audit(r.Context(),"",partnerID,"PARTNER_WORKSPACE_PERSONALIZATION_UPDATED",actor(r),correlationID(r),workspaceSettingsMap(old),workspaceSettingsMap(next))
	common.JSON(w,200,workspaceSettingsMap(next))
}

func (a *app) partnerModulePresentationInternal(w http.ResponseWriter,r *http.Request,partnerID,moduleKey string) {
	moduleKey=strings.TrimSpace(moduleKey)
	if !workspaceModuleKeyPattern.MatchString(moduleKey){common.APIError(w,404,"NOT_FOUND","Module presentation not found");return}
	if r.Method==http.MethodDelete{
		var old map[string]any
		rows,_:=a.listPartnerModulePresentations(r.Context(),partnerID)
		for _,item:=range rows{if item["module_key"]==moduleKey{old=item;break}}
		_,err:=a.db.ExecContext(r.Context(),"DELETE FROM cms.partner_module_presentations WHERE partner_id=$1 AND module_key=$2",partnerID,moduleKey)
		if err!=nil{common.APIError(w,500,"DB","Could not reset module presentation");return}
		_ = a.audit(r.Context(),"",moduleKey,"PARTNER_MODULE_PRESENTATION_RESET",actor(r),correlationID(r),old,map[string]any{"module_key":moduleKey,"reset":true})
		common.JSON(w,200,map[string]any{"partner_id":partnerID,"module_key":moduleKey,"reset":true})
		return
	}
	if r.Method!=http.MethodPut{common.APIError(w,http.StatusMethodNotAllowed,"METHOD","Use PUT or DELETE");return}
	var in struct{
		DisplayName string `json:"display_name"`
		Description string `json:"description"`
		IconKey string `json:"icon_key"`
		CustomIconMediaID string `json:"custom_icon_media_id"`
		CardColor string `json:"card_color"`
	}
	if common.Decode(r,&in)!=nil{common.APIError(w,400,"JSON","Invalid module presentation request");return}
	in.DisplayName=strings.TrimSpace(in.DisplayName)
	in.Description=strings.TrimSpace(in.Description)
	in.IconKey=strings.TrimSpace(in.IconKey)
	in.CustomIconMediaID=strings.TrimSpace(in.CustomIconMediaID)
	in.CardColor=strings.ToUpper(strings.TrimSpace(in.CardColor))
	if len(in.DisplayName)>100||len(in.Description)>800{common.APIError(w,400,"VALIDATION","Display name must be at most 100 characters and description at most 800 characters");return}
	if !workspaceIconAllowed(in.IconKey){common.APIError(w,400,"VALIDATION","Unsupported HIMATE icon-library key");return}
	if in.IconKey!=""&&in.CustomIconMediaID!=""{common.APIError(w,400,"VALIDATION","Choose either an icon-library icon or a custom icon asset");return}
	if in.CustomIconMediaID!=""&&!a.partnerOwnsMedia(r.Context(),partnerID,in.CustomIconMediaID){common.APIError(w,403,"MEDIA_SCOPE","Custom module icon must belong to this partner");return}
	if in.CardColor!=""&&!designColorPattern.MatchString(in.CardColor){common.APIError(w,400,"VALIDATION","card_color must use #RRGGBB");return}
	oldRows,_:=a.listPartnerModulePresentations(r.Context(),partnerID)
	var old map[string]any
	for _,item:=range oldRows{if item["module_key"]==moduleKey{old=item;break}}
	_,err:=a.db.ExecContext(r.Context(),`INSERT INTO cms.partner_module_presentations(
		partner_id,module_key,display_name,description,icon_key,custom_icon_media_id,card_color,updated_by,updated_at
	) VALUES($1,$2,$3,$4,$5,NULLIF($6,''),$7,$8,NOW())
	ON CONFLICT(partner_id,module_key) DO UPDATE SET
		display_name=EXCLUDED.display_name,description=EXCLUDED.description,icon_key=EXCLUDED.icon_key,
		custom_icon_media_id=EXCLUDED.custom_icon_media_id,card_color=EXCLUDED.card_color,updated_by=EXCLUDED.updated_by,updated_at=NOW()`,
		partnerID,moduleKey,in.DisplayName,in.Description,in.IconKey,in.CustomIconMediaID,in.CardColor,actor(r))
	if err!=nil{common.APIError(w,500,"DB","Could not save module presentation");return}
	nextRows,_:=a.listPartnerModulePresentations(r.Context(),partnerID)
	var next map[string]any
	for _,item:=range nextRows{if item["module_key"]==moduleKey{next=item;break}}
	_ = a.audit(r.Context(),"",moduleKey,"PARTNER_MODULE_PRESENTATION_UPDATED",actor(r),correlationID(r),old,next)
	common.JSON(w,200,next)
}
