package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"himate.local/services/internal/common"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const partnerSessionCookie = "himate_partner_session"

type partnerUser struct {
	ID              string
	PartnerID       string
	Name            string
	Email           string
	PasswordHash    string
	Role            string
	Active          bool
	PreferredLocale string
	Timezone        string
	SessionVersion  int
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type partnerClaims struct {
	Sub       string `json:"sub"`
	PartnerID string `json:"partner_id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	Version   int    `json:"v"`
	Exp       int64  `json:"exp"`
	Kind      string `json:"kind"`
}

var partnerRolePermissions = map[string][]string{
	"owner":   {"*"},
	"admin":   {"dashboard.read","company.read","company.write","modules.read","modules.write","billing.read","billing.write","impact.read","users.read","users.write","design.read","design.write","notifications.read"},
	"billing": {"dashboard.read","company.read","modules.read","modules.write","billing.read","billing.write","impact.read","design.read","notifications.read"},
	"viewer":  {"dashboard.read","company.read","modules.read","billing.read","impact.read","design.read","notifications.read"},
}

func partnerPortalMigration() common.Migration {
	return common.Migration{
		Version: 7,
		Name: "partner-portal-identities",
		Statements: []string{
			`CREATE TABLE IF NOT EXISTS identity.partner_users(
				id TEXT PRIMARY KEY,
				partner_id TEXT NOT NULL,
				name TEXT NOT NULL,
				email TEXT UNIQUE NOT NULL,
				password_hash TEXT NOT NULL,
				role_key TEXT NOT NULL DEFAULT 'viewer',
				active BOOLEAN NOT NULL DEFAULT TRUE,
				preferred_locale TEXT NOT NULL DEFAULT 'en_US',
				timezone TEXT NOT NULL DEFAULT 'UTC',
				session_version INTEGER NOT NULL DEFAULT 0,
				password_changed_at TIMESTAMPTZ,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE INDEX IF NOT EXISTS identity_partner_users_partner_idx ON identity.partner_users(partner_id,active,role_key)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS identity_partner_users_email_idx ON identity.partner_users(lower(email))`,
		},
	}
}

func retiredTestPartnerIdentityMigration() common.Migration {
	return common.Migration{
		Version: 12,
		Name:    "retire-fixed-manual-qa-partner-identity",
		AllowDestructiveSchema: true,
		Statements: []string{
			`DELETE FROM identity.partner_users
			  WHERE id='pusr_himate_test_001'
			     OR partner_id='ptr_himate_test_001'
			     OR lower(email)='test.partner@himate.test'`,
		},
	}
}

func partnerRoleValid(role string) bool {
	_, ok := partnerRolePermissions[strings.ToLower(strings.TrimSpace(role))]
	return ok
}

func partnerPermissions(role string) []string {
	values := partnerRolePermissions[strings.ToLower(strings.TrimSpace(role))]
	return append([]string(nil), values...)
}

func partnerCan(u partnerUser, permission string) bool {
	for _, p := range partnerPermissions(u.Role) {
		if p == "*" || p == permission { return true }
	}
	return false
}

func partnerUserMap(u partnerUser) map[string]any {
	return map[string]any{
		"id":u.ID,"partner_id":u.PartnerID,"name":u.Name,"email":u.Email,
		"role":u.Role,"active":u.Active,"permissions":partnerPermissions(u.Role),
		"preferred_locale":normalizedLocale(u.PreferredLocale),"timezone":normalizedTimezone(u.Timezone),
		"created_at":u.CreatedAt,"updated_at":u.UpdatedAt,
	}
}

func (a *app) findPartnerUser(field, value string) (partnerUser, error) {
	column := "id"
	if field == "email" { column = "email" }
	query := `SELECT id,partner_id,name,email,password_hash,role_key,active,preferred_locale,timezone,session_version,created_at,updated_at
		FROM identity.partner_users WHERE `+column+`=$1`
	if field == "email" {
		query = `SELECT id,partner_id,name,email,password_hash,role_key,active,preferred_locale,timezone,session_version,created_at,updated_at
			FROM identity.partner_users WHERE lower(email)=lower($1)`
	}
	var u partnerUser
	err := a.db.QueryRow(query, value).Scan(&u.ID,&u.PartnerID,&u.Name,&u.Email,&u.PasswordHash,&u.Role,&u.Active,&u.PreferredLocale,&u.Timezone,&u.SessionVersion,&u.CreatedAt,&u.UpdatedAt)
	return u, err
}

func partnerUserID() (string, error) {
	raw := make([]byte, 10)
	if _, err := rand.Read(raw); err != nil { return "", err }
	return "pusr_" + base64.RawURLEncoding.EncodeToString(raw), nil
}

func (a *app) issuePartnerSession(u partnerUser, ttl time.Duration) (string,error) {
	if ttl <= 0 { ttl = a.ttl }
	raw,_:=json.Marshal(partnerClaims{
		Sub:u.ID,PartnerID:u.PartnerID,Email:u.Email,Name:u.Name,Role:u.Role,Version:u.SessionVersion,
		Exp:time.Now().UTC().Add(ttl).Unix(),Kind:"PARTNER",
	})
	payload:=base64.RawURLEncoding.EncodeToString(raw)
	mac:=hmac.New(sha256.New,[]byte(a.secret))
	mac.Write([]byte("partner:"+payload))
	return payload+"."+base64.RawURLEncoding.EncodeToString(mac.Sum(nil)),nil
}

func (a *app) parsePartnerSession(token string)(partnerClaims,error){
	parts:=strings.Split(token,".")
	if len(parts)!=2{return partnerClaims{},fmt.Errorf("invalid partner session")}
	mac:=hmac.New(sha256.New,[]byte(a.secret));mac.Write([]byte("partner:"+parts[0]))
	sig,err:=base64.RawURLEncoding.DecodeString(parts[1])
	if err!=nil||!hmac.Equal(mac.Sum(nil),sig){return partnerClaims{},fmt.Errorf("invalid partner signature")}
	raw,err:=base64.RawURLEncoding.DecodeString(parts[0]);if err!=nil{return partnerClaims{},err}
	var c partnerClaims
	if err=json.Unmarshal(raw,&c);err!=nil||c.Kind!="PARTNER"||c.PartnerID==""||time.Now().Unix()>=c.Exp{
		return partnerClaims{},fmt.Errorf("expired partner session")
	}
	return c,nil
}

func (a *app) partnerAuth(r *http.Request)(partnerUser,error){
	cookie,err:=r.Cookie(partnerSessionCookie);if err!=nil{return partnerUser{},err}
	c,err:=a.parsePartnerSession(cookie.Value);if err!=nil{return partnerUser{},err}
	u,err:=a.findPartnerUser("id",c.Sub)
	if err!=nil||!u.Active||u.PartnerID!=c.PartnerID||u.SessionVersion!=c.Version{return partnerUser{},fmt.Errorf("inactive partner session")}
	return u,nil
}

var errPartnerPortalAccessDisabled = errors.New("partner portal access is disabled")

func (a *app) partnerAccessAllowed(ctx context.Context, partnerID string) error {
	var partner map[string]any
	if err:=a.internalGET(ctx,a.hosts["partners"],"/api/v1/partners/"+url.PathEscape(partnerID),&partner);err!=nil{return err}
	lifecycle:=strings.ToUpper(strings.TrimSpace(fmt.Sprint(partner["lifecycle"])))
	if lifecycle=="SUSPENDED"||lifecycle=="ARCHIVED"{return errPartnerPortalAccessDisabled}
	var gate map[string]any
	if err:=a.internalGET(ctx,a.hosts["billing"],"/internal/v1/partners/"+url.PathEscape(partnerID)+"/portal-gate",&gate);err!=nil{return err}
	if gate["allowed"]!=true{return errPartnerPortalAccessDisabled}
	return nil
}

func writePartnerAccessError(w http.ResponseWriter, err error) {
	if errors.Is(err, errPartnerPortalAccessDisabled) {
		common.APIError(w,http.StatusForbidden,"PARTNER_ACCESS_DISABLED","Partner Portal access is suspended for this partner")
		return
	}
	var upstream internalHTTPError
	if errors.As(err,&upstream) {
		switch upstream.Status {
		case http.StatusNotFound:
			common.APIError(w,http.StatusServiceUnavailable,"PARTNER_REGISTRY_NOT_READY","Partner record is not available in the partner registry yet")
		case http.StatusUnauthorized,http.StatusForbidden:
			common.APIError(w,http.StatusServiceUnavailable,"PARTNER_REGISTRY_AUTH_FAILED","Partner registry service authentication failed")
		default:
			common.APIError(w,http.StatusServiceUnavailable,"PARTNER_REGISTRY_UNAVAILABLE","Partner registry service is unavailable")
		}
		return
	}
	if errors.Is(err,context.DeadlineExceeded) || errors.Is(err,context.Canceled) {
		common.APIError(w,http.StatusServiceUnavailable,"PARTNER_REGISTRY_TIMEOUT","Partner registry did not respond in time")
		return
	}
	if strings.Contains(strings.ToLower(err.Error()),"host is not configured") {
		common.APIError(w,http.StatusServiceUnavailable,"PARTNER_REGISTRY_UNCONFIGURED","Partner registry service host is not configured")
		return
	}
	common.APIError(w,http.StatusServiceUnavailable,"PARTNER_REGISTRY_UNAVAILABLE","Partner registry service is unavailable")
}

func (a *app) partnerLogin(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodPost{common.APIError(w,405,"METHOD","Use POST");return}
	if !browserMutationOriginAllowed(r){common.APIError(w,403,"CSRF","Cross-site request rejected");return}
	key:="partner:"+clientKey(r);now:=time.Now().UTC()
	if !a.loginAllowed(key,now){w.Header().Set("Retry-After","900");common.APIError(w,429,"RATE_LIMITED","Too many sign-in attempts. Try again later.");return}
	var in struct{Email string `json:"email"`;Password string `json:"password"`;Remember bool `json:"remember"`}
	if common.Decode(r,&in)!=nil{common.APIError(w,400,"JSON","Invalid request");return}
	u,err:=a.findPartnerUser("email",strings.ToLower(strings.TrimSpace(in.Email)))
	valid:=err==nil&&u.Active&&verifyPassword(u.PasswordHash,in.Password)
	if err!=nil{_ = pbkdf2SHA256([]byte(in.Password),make([]byte,16),passwordIterations,32)}
	if !valid{a.recordLoginFailure(key,now);common.APIError(w,401,"INVALID_CREDENTIALS","Invalid email or password");return}
	ctx,cancel:=context.WithTimeout(r.Context(),2*time.Second);defer cancel()
	if err:=a.partnerAccessAllowed(ctx,u.PartnerID);err!=nil{writePartnerAccessError(w,err);return}
	if a.beginMFAFlow(w,r,"PARTNER",u.ID,in.Remember,partnerMFARequired(u.Role)){return}
	a.clearLoginFailures(key)
	ttl:=a.ttl;if in.Remember{ttl=a.rememberTTL}
	token,_:=a.issuePartnerSession(u,ttl)
	cookie:=&http.Cookie{Name:partnerSessionCookie,Value:token,Path:"/partner",HttpOnly:true,Secure:a.secureCookie,SameSite:http.SameSiteStrictMode}
	if in.Remember{cookie.MaxAge=int(ttl.Seconds());cookie.Expires=time.Now().UTC().Add(ttl)}
	http.SetCookie(w,cookie)
	go a.emitPartnerLoginNotification(u)
	common.JSON(w,200,partnerUserMap(u))
}

func (a *app) partnerLogout(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodPost{common.APIError(w,405,"METHOD","Use POST");return}
	if !browserMutationOriginAllowed(r){common.APIError(w,403,"CSRF","Cross-site request rejected");return}
	http.SetCookie(w,&http.Cookie{Name:partnerSessionCookie,Value:"",Path:"/partner",HttpOnly:true,Secure:a.secureCookie,SameSite:http.SameSiteStrictMode,MaxAge:-1})
	w.WriteHeader(http.StatusNoContent)
}

func (a *app) partnerMe(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodGet{common.APIError(w,405,"METHOD","Use GET");return}
	u,err:=a.partnerAuth(r);if err!=nil{common.APIError(w,401,"UNAUTHORIZED","Partner authentication required");return}
	ctx,cancel:=context.WithTimeout(r.Context(),2*time.Second)
	accessErr:=a.partnerAccessAllowed(ctx,u.PartnerID)
	cancel()
	if accessErr!=nil{writePartnerAccessError(w,accessErr);return}
	common.JSON(w,200,partnerUserMap(u))
}

type internalHTTPError struct{ Status int; Body []byte }
func (e internalHTTPError) Error() string { return fmt.Sprintf("internal service status %d",e.Status) }

func (a *app) internalJSON(ctx context.Context,method,host,path string,body any,headers map[string]string,dst any) error{
	if strings.TrimSpace(host)==""{return fmt.Errorf("private service host is not configured")}
	var reader io.Reader
	if body!=nil{raw,err:=json.Marshal(body);if err!=nil{return err};reader=bytes.NewReader(raw)}
	req,err:=http.NewRequestWithContext(ctx,method,"http://"+host+path,reader);if err!=nil{return err}
	common.BindInternalRequest(req,a.internalToken)
	if body!=nil{req.Header.Set("Content-Type","application/json")}
	for k,v:=range headers{req.Header.Set(k,v)}
	resp,err:=common.DoInternal(a.client,req);if err!=nil{return err};defer resp.Body.Close()
	raw,err:=io.ReadAll(io.LimitReader(resp.Body,1<<20));if err!=nil{return err}
	if resp.StatusCode>=300{return internalHTTPError{Status:resp.StatusCode,Body:raw}}
	gotVersion:=strings.TrimSpace(resp.Header.Get("X-Himate-App-Version"))
	if gotVersion==""||gotVersion!=a.version{
		payload,_:=json.Marshal(map[string]any{"error":map[string]string{
			"code":"RELEASE_MISMATCH",
			"message":fmt.Sprintf("Upstream service release %q does not match required release %q",gotVersion,a.version),
		}})
		return internalHTTPError{Status:http.StatusServiceUnavailable,Body:payload}
	}
	if dst!=nil&&len(bytes.TrimSpace(raw))>0{return json.Unmarshal(raw,dst)}
	return nil
}

func writeInternalError(w http.ResponseWriter,err error,fallback string){
	if typed,ok:=err.(internalHTTPError);ok{
		w.Header().Set("Content-Type","application/json")
		w.WriteHeader(typed.Status)
		if len(typed.Body)>0{_,_=w.Write(typed.Body);return}
	}
	common.APIError(w,502,"UPSTREAM",fallback)
}

func (a *app) requirePartnerPermission(w http.ResponseWriter,u partnerUser,permission string)bool{
	if partnerCan(u,permission){return true}
	common.APIError(w,403,"FORBIDDEN","Partner permission required: "+permission)
	return false
}

func partnerAuditAction(r *http.Request)string{
	path:=r.URL.Path
	switch{
	case path=="/partner/api/v1/company"&&r.Method==http.MethodPatch:return "PARTNER_COMPANY_UPDATED"
	case path=="/partner/api/v1/runtime/modules/invoice_documents/invoices"&&r.Method==http.MethodPost:return "PARTNER_MANUAL_INVOICE_DRAFT_CREATED"
	case strings.HasPrefix(path,"/partner/api/v1/runtime/modules/invoice_documents/invoices/")&&r.Method==http.MethodPut:return "PARTNER_MANUAL_INVOICE_DRAFT_UPDATED"
	case strings.HasPrefix(path,"/partner/api/v1/runtime/modules/invoice_documents/invoices/")&&strings.HasSuffix(path,"/finalize")&&r.Method==http.MethodPost:return "PARTNER_MANUAL_INVOICE_FINALIZED"
	case path=="/partner/api/v1/runtime/modules/invoice_documents/policy"&&r.Method==http.MethodPut:return "PARTNER_TENANT_FINANCE_POLICY_UPDATED"
	case strings.HasSuffix(path,"/activate")&&strings.HasPrefix(path,"/partner/api/v1/design/profiles/")&&r.Method==http.MethodPost:return "PARTNER_THEME_ACTIVATED"
	case strings.Contains(path,"/activate")&&r.Method==http.MethodPost:return "PARTNER_MODULE_ACTIVATED"
	case strings.Contains(path,"/subscription")&&r.Method==http.MethodPatch:return "PARTNER_SUBSCRIPTION_UPDATED"
	case path=="/partner/api/v1/users"&&r.Method==http.MethodPost:return "PARTNER_USER_CREATED"
	case strings.HasPrefix(path,"/partner/api/v1/users/")&&strings.HasSuffix(path,"/modules")&&r.Method==http.MethodPut:return "PARTNER_USER_MODULE_ACCESS_UPDATED"
	case path=="/partner/api/v1/notifications/read-all"&&r.Method==http.MethodPost:return "PARTNER_NOTIFICATIONS_READ_ALL"
	case strings.HasPrefix(path,"/partner/api/v1/notifications/")&&strings.HasSuffix(path,"/read")&&r.Method==http.MethodPost:return "PARTNER_NOTIFICATION_READ"
	case strings.HasPrefix(path,"/partner/api/v1/users/")&&r.Method==http.MethodPatch:return "PARTNER_USER_UPDATED"
	case path=="/partner/api/v1/design/media"&&r.Method==http.MethodPost:return "PARTNER_DESIGN_MEDIA_UPLOADED"
	case path=="/partner/api/v1/design/workspace"&&r.Method==http.MethodPut:return "PARTNER_WORKSPACE_PERSONALIZATION_UPDATED"
	case strings.HasPrefix(path,"/partner/api/v1/design/modules/")&&r.Method==http.MethodPut:return "PARTNER_MODULE_PRESENTATION_UPDATED"
	case strings.HasPrefix(path,"/partner/api/v1/design/modules/")&&r.Method==http.MethodDelete:return "PARTNER_MODULE_PRESENTATION_RESET"
	case path=="/partner/api/v1/design/profiles"&&r.Method==http.MethodPost:return "PARTNER_THEME_PROFILE_CREATED"
	case strings.HasPrefix(path,"/partner/api/v1/design/profiles/")&&r.Method==http.MethodPut:return "PARTNER_THEME_PROFILE_UPDATED"
	default:return "PARTNER_PORTAL_"+strings.ToUpper(r.Method)
	}
}

func (a *app) partnerAPI(w http.ResponseWriter,r *http.Request){
	stripUntrustedAuthorityHeaders(r)
	u,err:=a.partnerAuth(r);if err!=nil{common.APIError(w,401,"UNAUTHORIZED","Partner authentication required");return}
	if !browserMutationOriginAllowed(r){common.APIError(w,403,"CSRF","Cross-site request rejected");return}
	accessCtx,cancel:=context.WithTimeout(r.Context(),2*time.Second)
	accessErr:=a.partnerAccessAllowed(accessCtx,u.PartnerID)
	cancel()
	if accessErr!=nil{writePartnerAccessError(w,accessErr);return}
	mutating:=r.Method!=http.MethodGet&&r.Method!=http.MethodHead&&r.Method!=http.MethodOptions
	if mutating{
		started:=time.Now()
		requestState:=captureAuditRequest(r)
		baseEvent:=auditEvent{
			ActorID:u.ID,ActorName:u.Name,ActorRoles:[]string{"partner_"+u.Role},
			RequestID:strings.TrimSpace(r.Header.Get("X-Request-ID")),CorrelationID:strings.TrimSpace(r.Header.Get("X-Correlation-ID")),
			Action:partnerAuditAction(r),Method:r.Method,Path:r.URL.Path,Resource:"partner_portal",PartnerID:u.PartnerID,
			OldState:map[string]any{},CreatedAt:time.Now().UTC(),
		}
		intentID,intentErr:=a.createAuditIntent(r.Context(),baseEvent,requestState)
		if intentErr!=nil{common.APIError(w,http.StatusServiceUnavailable,"AUDIT_DURABILITY","Mutation blocked because the durable audit intent could not be recorded");return}
		rec:=&auditResponseWriter{ResponseWriter:w};w=rec
		defer func(){
			status:=rec.status;if status==0{status=200};outcome:="SUCCESS";if status>=400{outcome="FAILED"}
			newState:=decodeAuditState(rec.body.Bytes());if state,ok:=newState.(map[string]any);ok&&len(state)==0{newState=requestState}
			event:=baseEvent
			event.Status=status;event.Outcome=outcome;event.NewState=newState;event.DurationMS=time.Since(started).Milliseconds()
			finalizeCtx,finalizeCancel:=context.WithTimeout(context.Background(),3*time.Second)
			_ = a.finalizeAuditIntent(finalizeCtx,intentID,event)
			finalizeCancel()
		}()
	}
	r.Header.Set("X-Himate-User-ID",u.ID)
	r.Header.Set("X-Himate-Partner-ID",u.PartnerID)

	path:=strings.TrimPrefix(r.URL.Path,"/partner/api/v1")
	switch{
	case path=="/dashboard"&&r.Method==http.MethodGet:
		if a.requirePartnerPermission(w,u,"dashboard.read"){a.partnerDashboard(w,r,u)}
	case path=="/company":
		permission:="company.read";if r.Method==http.MethodPatch{permission="company.write"}
		if a.requirePartnerPermission(w,u,permission){a.partnerCompany(w,r,u)}
	case path=="/plans"&&r.Method==http.MethodGet:
		if a.requirePartnerPermission(w,u,"billing.read"){a.partnerPlans(w,r,u)}
	case path=="/plan"&&(r.Method==http.MethodGet||r.Method==http.MethodPatch||r.Method==http.MethodPut):
		permission:="billing.read";if r.Method!=http.MethodGet{permission="modules.write"}
		if a.requirePartnerPermission(w,u,permission){a.partnerPlan(w,r,u)}
	case path=="/plan/modules"&&(r.Method==http.MethodGet||r.Method==http.MethodPut):
		permission:="modules.read";if r.Method==http.MethodPut{permission="modules.write"}
		if a.requirePartnerPermission(w,u,permission){a.partnerPlanModules(w,r,u)}
	case path=="/charity"&&r.Method==http.MethodGet:
		if a.requirePartnerPermission(w,u,"billing.read"){a.partnerCharityState(w,r,u)}
	case path=="/charity/request"&&r.Method==http.MethodPost:
		if a.requirePartnerPermission(w,u,"modules.write"){a.partnerCharityRequest(w,r,u)}
	case path=="/charity/modules"&&(r.Method==http.MethodGet||r.Method==http.MethodPut):
		permission:="modules.read";if r.Method==http.MethodPut{permission="modules.write"}
		if a.requirePartnerPermission(w,u,permission){a.partnerCharityModules(w,r,u)}
	case path=="/modules"&&r.Method==http.MethodGet:
		if a.requirePartnerPermission(w,u,"modules.read"){a.partnerModulesView(w,r,u)}
	case strings.HasPrefix(path,"/runtime/modules/"):
		permission:="modules.read";if r.Method!=http.MethodGet&&r.Method!=http.MethodHead&&r.Method!=http.MethodOptions{permission="modules.write"}
		if a.requirePartnerPermission(w,u,permission){a.partnerModuleRuntime(w,r,u)}
	case strings.HasPrefix(path,"/modules/")&&strings.HasSuffix(path,"/activate")&&r.Method==http.MethodPost:
		if a.requirePartnerPermission(w,u,"modules.write"){a.partnerActivateModule(w,r,u)}
	case strings.HasPrefix(path,"/modules/")&&strings.HasSuffix(path,"/subscription")&&r.Method==http.MethodPatch:
		if a.requirePartnerPermission(w,u,"modules.write"){a.partnerSubscription(w,r,u)}
	case path=="/billing/summary"&&r.Method==http.MethodGet:
		if a.requirePartnerPermission(w,u,"billing.read"){a.partnerBillingSummary(w,r,u)}
	case path=="/billing/subscriptions"&&r.Method==http.MethodGet:
		if a.requirePartnerPermission(w,u,"billing.read"){a.partnerBillingSubscriptions(w,r,u)}
	case path=="/billing/invoices"&&r.Method==http.MethodGet:
		if a.requirePartnerPermission(w,u,"billing.read"){a.partnerBillingInvoices(w,r,u)}
	case strings.HasPrefix(path,"/billing/invoices/")&&strings.HasSuffix(path,"/pdf")&&r.Method==http.MethodGet:
		if a.requirePartnerPermission(w,u,"billing.read"){a.partnerInvoicePDF(w,r,u)}
	case path=="/impact/summary"&&r.Method==http.MethodGet:
		if a.requirePartnerPermission(w,u,"impact.read"){a.partnerImpactSummary(w,r,u)}
	case path=="/users":
		permission:="users.read";if r.Method==http.MethodPost{permission="users.write"}
		if a.requirePartnerPermission(w,u,permission){a.partnerUsers(w,r,u)}
	case strings.HasPrefix(path,"/users/")&&strings.HasSuffix(path,"/modules")&&(r.Method==http.MethodGet||r.Method==http.MethodPut):
		permission:="users.read";if r.Method==http.MethodPut{permission="users.write"}
		if a.requirePartnerPermission(w,u,permission){
			raw:=strings.Trim(strings.TrimSuffix(strings.TrimPrefix(path,"/users/"),"/modules"),"/")
			if raw==""||strings.Contains(raw,"/"){common.APIError(w,404,"NOT_FOUND","Partner user not found")}else{a.partnerUserModuleAccess(w,r,u,raw)}
		}
	case strings.HasPrefix(path,"/users/")&&r.Method==http.MethodPatch:
		if a.requirePartnerPermission(w,u,"users.write"){a.partnerUserUpdate(w,r,u)}
	case (path=="/notifications"&&r.Method==http.MethodGet)||(strings.HasPrefix(path,"/notifications/")&&r.Method==http.MethodPost):
		if a.requirePartnerPermission(w,u,"notifications.read"){a.partnerNotifications(w,r,u)}
	case path=="/design"&&r.Method==http.MethodGet:
		if a.requirePartnerPermission(w,u,"design.read"){a.partnerDesign(w,r,u)}
	case path=="/design/media"&&(r.Method==http.MethodGet||r.Method==http.MethodPost):
		permission:="design.read";if r.Method==http.MethodPost{permission="design.write"}
		if a.requirePartnerPermission(w,u,permission){a.partnerDesignMedia(w,r,u)}
	case path=="/design/workspace"&&r.Method==http.MethodPut:
		if a.requirePartnerPermission(w,u,"design.write"){a.partnerDesign(w,r,u)}
	case strings.HasPrefix(path,"/design/modules/")&&(r.Method==http.MethodPut||r.Method==http.MethodDelete):
		if a.requirePartnerPermission(w,u,"design.write"){a.partnerDesign(w,r,u)}
	case path=="/design/profiles"&&r.Method==http.MethodPost:
		if a.requirePartnerPermission(w,u,"design.write"){a.partnerDesign(w,r,u)}
	case strings.HasPrefix(path,"/design/profiles/")&&(r.Method==http.MethodPut||r.Method==http.MethodPost):
		if a.requirePartnerPermission(w,u,"design.write"){a.partnerDesign(w,r,u)}
	default:
		common.APIError(w,404,"NOT_FOUND","Partner Portal endpoint not found")
	}
}

func (a *app) partnerDashboard(w http.ResponseWriter,r *http.Request,u partnerUser){
	ctx,cancel:=context.WithTimeout(r.Context(),3*time.Second);defer cancel()
	var company,modules,billing,impact map[string]any
	var companyErr,modulesErr,billingErr,impactErr error
	var wg sync.WaitGroup;wg.Add(4)
	go func(){defer wg.Done();companyErr=a.internalGET(ctx,a.hosts["partners"],"/api/v1/partners/"+url.PathEscape(u.PartnerID),&company)}()
	go func(){defer wg.Done();modulesErr=a.internalGET(ctx,a.hosts["catalog"],"/internal/v1/partner-portal/"+url.PathEscape(u.PartnerID)+"/modules?locale="+url.QueryEscape(u.PreferredLocale),&modules)}()
	go func(){defer wg.Done();billingErr=a.internalGET(ctx,a.hosts["billing"],"/api/v1/billing/partners/"+url.PathEscape(u.PartnerID)+"/summary",&billing)}()
	go func(){defer wg.Done();impactErr=a.internalGET(ctx,a.hosts["impact"],"/api/v1/impact/summary?partner_id="+url.QueryEscape(u.PartnerID),&impact)}()
	wg.Wait()
	if companyErr!=nil{common.APIError(w,502,"PARTNER_UNAVAILABLE","Partner company record is temporarily unavailable");return}
	if modulesErr==nil{
		a.enrichPartnerMarketplace(ctx,u.PartnerID,modules)
		if err:=a.applyPartnerUserModuleAccess(ctx,u,modules);err!=nil{modulesErr=err}
	}
	degraded:=[]string{}
	if modulesErr!=nil{degraded=append(degraded,"modules")}
	if billingErr!=nil{degraded=append(degraded,"billing")}
	if impactErr!=nil{degraded=append(degraded,"impact")}
	delete(company,"notes")
	common.JSON(w,200,map[string]any{
		"partner_id":u.PartnerID,"company":company,"modules":modules,"billing":billing,"impact":impact,
		"degraded_sections":degraded,
	})
}

func (a *app) partnerCompany(w http.ResponseWriter,r *http.Request,u partnerUser){
	path:="/api/v1/partners/"+url.PathEscape(u.PartnerID)
	if r.Method==http.MethodGet{
		var out map[string]any
		if err:=a.internalGET(r.Context(),a.hosts["partners"],path,&out);err!=nil{common.APIError(w,502,"PARTNER_UNAVAILABLE","Company data is temporarily unavailable");return}
		delete(out,"notes")
		common.JSON(w,200,out);return
	}
	if r.Method!=http.MethodPatch{common.APIError(w,405,"METHOD","Use GET or PATCH");return}
	var in struct{
		DisplayName *string `json:"display_name"`;LegalName *string `json:"legal_name"`;BrandName *string `json:"brand_name"`
		RegistrationNumber *string `json:"registration_number"`;TaxID *string `json:"tax_id"`
		ContactName *string `json:"contact_name"`;ContactEmail *string `json:"contact_email"`
		FinanceContactName *string `json:"finance_contact_name"`;FinanceContactEmail *string `json:"finance_contact_email"`
		TechnicalContactName *string `json:"technical_contact_name"`;TechnicalContactEmail *string `json:"technical_contact_email"`
		MarketingContactName *string `json:"marketing_contact_name"`;MarketingContactEmail *string `json:"marketing_contact_email"`
		Country *string `json:"country"`;StateRegion *string `json:"state_region"`;City *string `json:"city"`;PostalCode *string `json:"postal_code"`
		AddressLine1 *string `json:"address_line1"`;AddressLine2 *string `json:"address_line2"`;Website *string `json:"website"`;Phone *string `json:"phone"`;LogoURL *string `json:"logo_url"`
	}
	if common.Decode(r,&in)!=nil{common.APIError(w,400,"JSON","Invalid request");return}
	payload:=map[string]any{}
	add:=func(key string,v *string){if v!=nil{payload[key]=strings.TrimSpace(*v)}}
	add("display_name",in.DisplayName);add("legal_name",in.LegalName);add("brand_name",in.BrandName);add("registration_number",in.RegistrationNumber);add("tax_id",in.TaxID)
	add("contact_name",in.ContactName);add("contact_email",in.ContactEmail);add("finance_contact_name",in.FinanceContactName);add("finance_contact_email",in.FinanceContactEmail)
	add("technical_contact_name",in.TechnicalContactName);add("technical_contact_email",in.TechnicalContactEmail);add("marketing_contact_name",in.MarketingContactName);add("marketing_contact_email",in.MarketingContactEmail)
	add("country",in.Country);add("state_region",in.StateRegion);add("city",in.City);add("postal_code",in.PostalCode);add("address_line1",in.AddressLine1);add("address_line2",in.AddressLine2)
	add("website",in.Website);add("phone",in.Phone);add("logo_url",in.LogoURL)
	var out map[string]any
	err:=a.internalJSON(r.Context(),http.MethodPatch,a.hosts["partners"],path,payload,map[string]string{"X-Himate-User-ID":u.ID},&out)
	if err!=nil{writeInternalError(w,err,"Company data could not be updated");return}
	delete(out,"notes")
	common.JSON(w,200,out)
}

func (a *app) partnerPresentationModule(ctx context.Context,u partnerUser,key string,requireActive bool) (map[string]any,error){
	var catalog map[string]any
	path:="/internal/v1/partner-portal/"+url.PathEscape(u.PartnerID)+"/modules?locale="+url.QueryEscape(u.PreferredLocale)
	if err:=a.internalGET(ctx,a.hosts["catalog"],path,&catalog);err!=nil{return nil,err}
	for _,item:=range anyItems(catalog["items"]){
		if strings.TrimSpace(fmt.Sprint(item["key"]))!=key{continue}
		if requireActive && (strings.ToUpper(strings.TrimSpace(fmt.Sprint(item["access_state"])))!="ACTIVE" || item["executable"]!=true){
			return nil,fmt.Errorf("DEFAULT_MODULE_NOT_ACTIVE")
		}
		return item,nil
	}
	return nil,fmt.Errorf("MODULE_NOT_FOUND")
}

func (a *app) partnerDesign(w http.ResponseWriter,r *http.Request,u partnerUser){
	suffix:=strings.TrimPrefix(r.URL.Path,"/partner/api/v1/design")
	upstream:="/internal/v1/cms/partner-design/"+url.PathEscape(u.PartnerID)+suffix
	if r.Method==http.MethodGet{
		var out map[string]any
		if err:=a.internalGET(r.Context(),a.hosts["cms"],upstream,&out);err!=nil{writeInternalError(w,err,"Partner design data is temporarily unavailable");return}
		common.JSON(w,200,out);return
	}
	if r.Method!=http.MethodPost&&r.Method!=http.MethodPut&&r.Method!=http.MethodDelete{common.APIError(w,405,"METHOD","Use GET, POST, PUT or DELETE");return}
	var payload map[string]any
	if r.Method!=http.MethodDelete{
		if common.Decode(r,&payload)!=nil{common.APIError(w,400,"JSON","Invalid request");return}
	}
	if suffix=="/workspace"&&r.Method==http.MethodPut{
		key:=strings.TrimSpace(fmt.Sprint(payload["default_module_key"]))
		if key!=""{
			if _,err:=a.partnerPresentationModule(r.Context(),u,key,true);err!=nil{
				if err.Error()=="DEFAULT_MODULE_NOT_ACTIVE"{common.APIError(w,409,"DEFAULT_MODULE_NOT_ACTIVE","Default module must be an ACTIVE and executable module owned by this partner");return}
				if err.Error()=="MODULE_NOT_FOUND"{common.APIError(w,404,"MODULE_NOT_FOUND","Default module is not in this partner marketplace");return}
				common.APIError(w,502,"CATALOG_UNAVAILABLE","Could not validate the default module");return
			}
		}
	}
	if strings.HasPrefix(suffix,"/modules/")&&(r.Method==http.MethodPut||r.Method==http.MethodDelete){
		key:=strings.Trim(strings.TrimPrefix(suffix,"/modules/"),"/")
		if key==""||strings.Contains(key,"/"){common.APIError(w,404,"MODULE_NOT_FOUND","Module presentation not found");return}
		if _,err:=a.partnerPresentationModule(r.Context(),u,key,false);err!=nil{
			if err.Error()=="MODULE_NOT_FOUND"{common.APIError(w,404,"MODULE_NOT_FOUND","Module is not in this partner marketplace");return}
			common.APIError(w,502,"CATALOG_UNAVAILABLE","Could not validate the module presentation");return
		}
	}
	var out map[string]any
	err:=a.internalJSON(r.Context(),r.Method,a.hosts["cms"],upstream,payload,map[string]string{
		"X-Himate-User-ID":u.ID,
		"X-Himate-Correlation-ID":strings.TrimSpace(r.Header.Get("X-Correlation-ID")),
	},&out)
	if err!=nil{writeInternalError(w,err,"Partner design could not be updated");return}
	status:=200;if r.Method==http.MethodPost&&suffix=="/profiles"{status=201}
	common.JSON(w,status,out)
}

func (a *app) partnerDesignMedia(w http.ResponseWriter,r *http.Request,u partnerUser){
	upstream:="/internal/v1/cms/partner-media/"+url.PathEscape(u.PartnerID)
	if r.Method==http.MethodGet{
		var out map[string]any
		if err:=a.internalGET(r.Context(),a.hosts["cms"],upstream,&out);err!=nil{writeInternalError(w,err,"Partner design media is temporarily unavailable");return}
		common.JSON(w,200,out);return
	}
	if r.Method!=http.MethodPost{common.APIError(w,405,"METHOD","Use GET or POST");return}
	host:=strings.TrimSpace(a.hosts["cms"]);if host==""{common.APIError(w,502,"UPSTREAM","CMS service is not configured");return}
	req,err:=http.NewRequestWithContext(r.Context(),http.MethodPost,"http://"+host+upstream,io.LimitReader(r.Body,66<<20))
	if err!=nil{common.APIError(w,500,"REQUEST","Could not prepare design media upload");return}
	common.BindInternalRequest(req,a.internalToken)
	req.Header.Set("X-Himate-User-ID",u.ID)
	req.Header.Set("X-Correlation-ID",strings.TrimSpace(r.Header.Get("X-Correlation-ID")))
	req.Header.Set("Content-Type",r.Header.Get("Content-Type"))
	resp,err:=common.DoInternal(a.client,req);if err!=nil{common.APIError(w,502,"UPSTREAM","Partner design media upload failed");return};defer resp.Body.Close()
	for _,key:=range []string{"Content-Type","Content-Length"}{if value:=resp.Header.Get(key);value!=""{w.Header().Set(key,value)}}
	w.WriteHeader(resp.StatusCode);_,_=io.Copy(w,io.LimitReader(resp.Body,2<<20))
}

func anyItems(value any) []map[string]any {
	switch raw:=value.(type){
	case []map[string]any:
		return raw
	case []any:
		out:=make([]map[string]any,0,len(raw))
		for _,item:=range raw{if mapped,ok:=item.(map[string]any);ok{out=append(out,mapped)}}
		return out
	default:
		return []map[string]any{}
	}
}

func (a *app) partnerPlans(w http.ResponseWriter,r *http.Request,u partnerUser){
	var out map[string]any
	if err:=a.internalGET(r.Context(),a.hosts["billing"],"/api/v1/billing/plans",&out);err!=nil{
		writeInternalError(w,err,"Subscription plans are temporarily unavailable");return
	}
	if raw,ok:=out["items"].([]any);ok{
		filtered:=[]any{}
		for _,item:=range raw{
			if m,ok:=item.(map[string]any);ok && m["customer_selectable"]==true && m["active"]==true{filtered=append(filtered,m)}
		}
		out["items"]=filtered;out["count"]=len(filtered)
	}
	common.JSON(w,200,out)
}

func (a *app) partnerCharityState(w http.ResponseWriter,r *http.Request,u partnerUser){
	var out map[string]any
	if err:=a.internalGET(r.Context(),a.hosts["billing"],"/api/v1/billing/partners/"+url.PathEscape(u.PartnerID)+"/commercial-mode",&out);err!=nil{
		writeInternalError(w,err,"Charity status is temporarily unavailable");return
	}
	common.JSON(w,200,out)
}

func (a *app) partnerCharityRequest(w http.ResponseWriter,r *http.Request,u partnerUser){
	var payload map[string]any
	if common.Decode(r,&payload)!=nil{common.APIError(w,400,"JSON","Invalid request");return}
	var out map[string]any
	err:=a.internalJSON(r.Context(),http.MethodPost,a.hosts["billing"],
		"/api/v1/billing/partners/"+url.PathEscape(u.PartnerID)+"/charity/request",
		payload,map[string]string{"X-Himate-User-ID":"partner:"+u.ID},&out)
	if err!=nil{writeInternalError(w,err,"Charity review request could not be submitted");return}
	common.JSON(w,202,out)
}

func (a *app) partnerCharityModules(w http.ResponseWriter,r *http.Request,u partnerUser){
	upstream:="/api/v1/billing/partners/"+url.PathEscape(u.PartnerID)+"/charity/modules"
	if r.Method==http.MethodGet{
		var out map[string]any
		if err:=a.internalGET(r.Context(),a.hosts["billing"],upstream,&out);err!=nil{
			writeInternalError(w,err,"Charity module selection is temporarily unavailable");return
		}
		common.JSON(w,200,out);return
	}
	var payload map[string]any
	if common.Decode(r,&payload)!=nil{common.APIError(w,400,"JSON","Invalid request");return}
	var out map[string]any
	err:=a.internalJSON(r.Context(),http.MethodPut,a.hosts["billing"],upstream,payload,
		map[string]string{"X-Himate-User-ID":"partner:"+u.ID},&out)
	if err!=nil{writeInternalError(w,err,"Charity module selection could not be updated");return}
	common.JSON(w,200,out)
}

func (a *app) partnerPlan(w http.ResponseWriter,r *http.Request,u partnerUser){
	upstream:="/api/v1/billing/partners/"+url.PathEscape(u.PartnerID)+"/plan"
	if r.Method==http.MethodGet{
		var out map[string]any
		if err:=a.internalGET(r.Context(),a.hosts["billing"],upstream,&out);err!=nil{writeInternalError(w,err,"Subscription plan is temporarily unavailable");return}
		common.JSON(w,200,out);return
	}
	var payload map[string]any
	if common.Decode(r,&payload)!=nil{common.APIError(w,400,"JSON","Invalid request");return}
	if strings.ToUpper(strings.TrimSpace(fmt.Sprint(payload["plan_key"])))=="CUSTOM"{
		common.APIError(w,403,"CUSTOM_ADMIN_ONLY","Custom commercial plans are assigned only by HIMATE administrators");return
	}
	var out map[string]any
	err:=a.internalJSON(r.Context(),http.MethodPatch,a.hosts["billing"],upstream,payload,map[string]string{"X-Himate-User-ID":"partner:"+u.ID},&out)
	if err!=nil{writeInternalError(w,err,"Subscription plan could not be changed");return}
	common.JSON(w,200,out)
}

func (a *app) partnerPlanModules(w http.ResponseWriter,r *http.Request,u partnerUser){
	upstream:="/api/v1/billing/partners/"+url.PathEscape(u.PartnerID)+"/plan/modules"
	if r.Method==http.MethodGet{
		var out map[string]any
		if err:=a.internalGET(r.Context(),a.hosts["billing"],upstream,&out);err!=nil{writeInternalError(w,err,"Plan modules are temporarily unavailable");return}
		common.JSON(w,200,out);return
	}
	var payload map[string]any
	if common.Decode(r,&payload)!=nil{common.APIError(w,400,"JSON","Invalid request");return}
	var out map[string]any
	err:=a.internalJSON(r.Context(),http.MethodPut,a.hosts["billing"],upstream,payload,map[string]string{"X-Himate-User-ID":"partner:"+u.ID},&out)
	if err!=nil{writeInternalError(w,err,"Package module configuration could not be updated");return}
	common.JSON(w,200,out)
}

func (a *app) partnerHasManagedPlan(ctx context.Context,partnerID string)(bool,error){
	var out map[string]any
	err:=a.internalGET(ctx,a.hosts["billing"],"/api/v1/billing/partners/"+url.PathEscape(partnerID)+"/plan",&out)
	if err!=nil{return false,err}
	configured,ok:=out["configured"].(bool)
	return ok&&configured,nil
}

func stringSetFromAny(value any) map[string]bool {
	out:=map[string]bool{}
	raw,ok:=value.([]any)
	if !ok{return out}
	for _,item:=range raw{
		key:=strings.TrimSpace(fmt.Sprint(item))
		if key!=""{out[key]=true}
	}
	return out
}

func (a *app) enrichPartnerMarketplace(ctx context.Context,partnerID string,out map[string]any) {
	var plans,current map[string]any
	var plansErr,currentErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func(){defer wg.Done();plansErr=a.internalGET(ctx,a.hosts["billing"],"/api/v1/billing/plans",&plans)}()
	go func(){defer wg.Done();currentErr=a.internalGET(ctx,a.hosts["billing"],"/api/v1/billing/partners/"+url.PathEscape(partnerID)+"/plan",&current)}()
	wg.Wait()
	if plansErr!=nil||currentErr!=nil{
		out["plan_context_available"]=false
		return
	}
	out["plan_context_available"]=true
	configured:=current["configured"]==true
	currentKey:=strings.ToUpper(strings.TrimSpace(fmt.Sprint(current["plan_key"])))
	currentModules:=stringSetFromAny(current["active_module_keys"])
	out["current_plan_key"]=currentKey
	out["current_plan_display_name"]=fmt.Sprint(current["display_name"])

	planItems:=anyItems(plans["items"])
	currentSort:=-1
	for _,p:=range planItems{
		if strings.ToUpper(strings.TrimSpace(fmt.Sprint(p["plan_key"])))==currentKey{
			if n,ok:=p["sort_order"].(float64);ok{currentSort=int(n)}
			if n,ok:=p["sort_order"].(int);ok{currentSort=n}
		}
	}

	rawItems,ok:=out["items"].([]any)
	if !ok{
		if typed,typedOK:=out["items"].([]map[string]any);typedOK{
			rawItems=make([]any,0,len(typed))
			for _,item:=range typed{rawItems=append(rawItems,item)}
		}else{
			return
		}
	}
	enriched:=make([]map[string]any,0,len(rawItems))
	for _,raw:=range rawItems{
		module,ok:=raw.(map[string]any);if !ok{continue}
		key:=strings.TrimSpace(fmt.Sprint(module["key"]))
		executable:=module["executable"]==true
		availableKeys:=[]string{}
		availableNames:=[]string{}
		upgradeKeys:=[]string{}
		upgradeNames:=[]string{}
		for _,p:=range planItems{
			if p["customer_selectable"]!=true||p["active"]!=true||p["ready"]!=true{continue}
			planKey:=strings.ToUpper(strings.TrimSpace(fmt.Sprint(p["plan_key"])))
			planName:=strings.TrimSpace(fmt.Sprint(p["display_name"]))
			mode:=strings.ToUpper(strings.TrimSpace(fmt.Sprint(p["selection_mode"])))
			inPlan:=false
			if mode=="FIXED"{
				inPlan=stringSetFromAny(p["fixed_module_keys"])[key]
			}else if mode=="SELECTABLE" || mode=="UNLIMITED"{
				inPlan=executable
			}
			if !inPlan{continue}
			availableKeys=append(availableKeys,planKey)
			availableNames=append(availableNames,planName)
			sortOrder:=-1
			if n,ok:=p["sort_order"].(float64);ok{sortOrder=int(n)}
			if n,ok:=p["sort_order"].(int);ok{sortOrder=n}
			if configured&&sortOrder>currentSort{
				upgradeKeys=append(upgradeKeys,planKey)
				upgradeNames=append(upgradeNames,planName)
			}
		}
		module["available_in_plans"]=availableKeys
		module["available_in_plan_names"]=availableNames
		module["upgrade_plan_keys"]=upgradeKeys
		module["upgrade_plan_names"]=upgradeNames
		module["in_current_plan"]=configured&&currentModules[key]
		if len(upgradeKeys)>0{
			module["recommended_upgrade_plan"]=upgradeKeys[0]
			module["recommended_upgrade_plan_name"]=upgradeNames[0]
		}
		module["current_plan_key"]=currentKey
		enriched=append(enriched,module)
	}
	out["items"]=enriched
	out["count"]=len(enriched)
}

func (a *app) partnerModulesView(w http.ResponseWriter,r *http.Request,u partnerUser){
	var out map[string]any
	if err:=a.internalGET(r.Context(),a.hosts["catalog"],"/internal/v1/partner-portal/"+url.PathEscape(u.PartnerID)+"/modules?locale="+url.QueryEscape(u.PreferredLocale),&out);err!=nil{
		common.APIError(w,502,"CATALOG_UNAVAILABLE","Module catalog is temporarily unavailable");return}
	a.enrichPartnerMarketplace(r.Context(),u.PartnerID,out)
	if err:=a.applyPartnerUserModuleAccess(r.Context(),u,out);err!=nil{
		common.APIError(w,500,"MODULE_ACCESS","User module access could not be evaluated");return
	}
	common.JSON(w,200,out)
}

func partnerModuleKey(path,suffix string)string{
	raw:=strings.Trim(strings.TrimPrefix(path,"/partner/api/v1/modules/"),"/")
	raw=strings.TrimSuffix(raw,suffix);return strings.Trim(raw,"/")
}

func (a *app) partnerActivateModule(w http.ResponseWriter,r *http.Request,u partnerUser){
	var commercial map[string]any
	if err:=a.internalGET(r.Context(),a.hosts["billing"],"/api/v1/billing/partners/"+url.PathEscape(u.PartnerID)+"/commercial-mode",&commercial);err!=nil{
		common.APIError(w,502,"BILLING_UNAVAILABLE","Billing must be available before module entitlement changes");return
	}
	if fmt.Sprint(commercial["billing_mode"])=="CHARITY" && fmt.Sprint(commercial["charity_status"])=="APPROVED"{
		common.APIError(w,409,"CHARITY_MODULE_SELECTION_REQUIRED","Approved Charity partners manage access through Charity module selection");return
	}
	managed,planErr:=a.partnerHasManagedPlan(r.Context(),u.PartnerID)
	if planErr!=nil{common.APIError(w,502,"BILLING_UNAVAILABLE","Billing must be available before module entitlement changes");return}
	if managed{
		common.APIError(w,409,"PLAN_MANAGED_MODULES","Modules are controlled by your subscription package entitlement");return
	}
	key:=partnerModuleKey(r.URL.Path,"/activate");if key==""||strings.Contains(key,"/"){common.APIError(w,404,"NOT_FOUND","Module not found");return}
	var module map[string]any
	err:=a.internalJSON(r.Context(),http.MethodPost,a.hosts["catalog"],"/internal/v1/partner-portal/"+url.PathEscape(u.PartnerID)+"/modules/"+url.PathEscape(key)+"/activate",
		map[string]any{},map[string]string{"X-Himate-User-ID":u.ID},&module)
	if err!=nil{writeInternalError(w,err,"Module could not be activated");return}
	var billing map[string]any
	syncErr:=a.internalGET(r.Context(),a.hosts["billing"],"/api/v1/billing/partners/"+url.PathEscape(u.PartnerID)+"/summary",&billing)
	syncState:="SYNCED";if syncErr!=nil{syncState="PENDING"}
	common.JSON(w,200,map[string]any{"module":module,"billing":billing,"billing_sync":syncState})
}

func (a *app) partnerSubscription(w http.ResponseWriter,r *http.Request,u partnerUser){
	var commercial map[string]any
	if err:=a.internalGET(r.Context(),a.hosts["billing"],"/api/v1/billing/partners/"+url.PathEscape(u.PartnerID)+"/commercial-mode",&commercial);err!=nil{
		common.APIError(w,502,"BILLING_UNAVAILABLE","Billing must be available before module entitlement changes");return
	}
	if fmt.Sprint(commercial["billing_mode"])=="CHARITY" && fmt.Sprint(commercial["charity_status"])=="APPROVED"{
		common.APIError(w,409,"CHARITY_MODULE_SELECTION_REQUIRED","Approved Charity partners manage access through Charity module selection");return
	}
	managed,planErr:=a.partnerHasManagedPlan(r.Context(),u.PartnerID)
	if planErr!=nil{common.APIError(w,502,"BILLING_UNAVAILABLE","Billing must be available before module entitlement changes");return}
	if managed{
		common.APIError(w,409,"PLAN_MANAGED_MODULES","Individual module cancellation is disabled for subscription-plan partners");return
	}
	key:=partnerModuleKey(r.URL.Path,"/subscription");if key==""||strings.Contains(key,"/"){common.APIError(w,404,"NOT_FOUND","Subscription not found");return}
	var catalog map[string]any
	if err:=a.internalGET(r.Context(),a.hosts["catalog"],"/internal/v1/partner-portal/"+url.PathEscape(u.PartnerID)+"/modules",&catalog);err!=nil{
		common.APIError(w,502,"CATALOG_UNAVAILABLE","Module state is temporarily unavailable");return
	}
	var target map[string]any
	for _,item:=range anyItems(catalog["items"]){if fmt.Sprint(item["key"])==key{target=item;break}}
	if target==nil{common.APIError(w,404,"NOT_FOUND","Module not found");return}
	if target["included_in_base"]==true{common.APIError(w,409,"BASE_MODULE","Base-package modules cannot be cancelled individually");return}
	// START-23.3: Billing owns subscription lifecycle. Catalog availability/state
	// must not prevent a tenant from scheduling cancellation of an existing paid period.
	var in struct{CancelAtPeriodEnd *bool `json:"cancel_at_period_end"`}
	if common.Decode(r,&in)!=nil||in.CancelAtPeriodEnd==nil{common.APIError(w,400,"VALIDATION","cancel_at_period_end is required");return}
	payload:=map[string]any{"cancel_at_period_end":*in.CancelAtPeriodEnd,"reason":"Partner Portal request"}
	var out map[string]any
	err:=a.internalJSON(r.Context(),http.MethodPatch,a.hosts["billing"],"/api/v1/billing/partners/"+url.PathEscape(u.PartnerID)+"/subscriptions/"+url.PathEscape(key),
		payload,map[string]string{"X-Himate-User-ID":u.ID},&out)
	if err!=nil{writeInternalError(w,err,"Subscription could not be updated");return}
	common.JSON(w,200,out)
}

func (a *app) partnerBillingSummary(w http.ResponseWriter,r *http.Request,u partnerUser){
	var out map[string]any
	if err:=a.internalGET(r.Context(),a.hosts["billing"],"/api/v1/billing/partners/"+url.PathEscape(u.PartnerID)+"/summary",&out);err!=nil{
		common.APIError(w,502,"BILLING_UNAVAILABLE","Billing summary is temporarily unavailable");return}
	common.JSON(w,200,out)
}

func (a *app) partnerBillingSubscriptions(w http.ResponseWriter,r *http.Request,u partnerUser){
	var out map[string]any
	if err:=a.internalGET(r.Context(),a.hosts["billing"],"/api/v1/billing/partners/"+url.PathEscape(u.PartnerID)+"/subscriptions",&out);err!=nil{
		common.APIError(w,502,"BILLING_UNAVAILABLE","Subscriptions are temporarily unavailable");return}
	common.JSON(w,200,out)
}

func (a *app) partnerBillingInvoices(w http.ResponseWriter,r *http.Request,u partnerUser){
	var out map[string]any
	if err:=a.internalGET(r.Context(),a.hosts["billing"],"/api/v1/billing/partners/"+url.PathEscape(u.PartnerID)+"/invoices?partner_visible=true",&out);err!=nil{
		common.APIError(w,502,"BILLING_UNAVAILABLE","Invoices are temporarily unavailable");return}
	common.JSON(w,200,out)
}

func (a *app) partnerInvoicePDF(w http.ResponseWriter,r *http.Request,u partnerUser){
	raw:=strings.Trim(strings.TrimSuffix(strings.TrimPrefix(r.URL.Path,"/partner/api/v1/billing/invoices/"),"/pdf"),"/")
	if raw==""||strings.Contains(raw,"/"){common.APIError(w,404,"NOT_FOUND","Invoice not found");return}
	var invoices map[string]any
	if err:=a.internalGET(r.Context(),a.hosts["billing"],"/api/v1/billing/partners/"+url.PathEscape(u.PartnerID)+"/invoices?partner_visible=true",&invoices);err!=nil{
		common.APIError(w,502,"BILLING_UNAVAILABLE","Invoices are temporarily unavailable");return
	}
	found:=false
	for _,item:=range anyItems(invoices["items"]){if fmt.Sprint(item["id"])==raw{found=true;break}}
	if !found{common.APIError(w,404,"NOT_FOUND","Invoice not found");return}
	req,err:=http.NewRequestWithContext(r.Context(),http.MethodGet,"http://"+a.hosts["billing"]+"/api/v1/billing/invoices/"+url.PathEscape(raw)+"/pdf",nil)
	if err!=nil{common.APIError(w,500,"REQUEST","Could not create invoice PDF request");return}
	common.BindInternalRequest(req,a.token)
	resp,err:=common.DoInternal(a.client,req);if err!=nil{common.APIError(w,502,"BILLING_UNAVAILABLE","Invoice PDF is temporarily unavailable");return}
	defer resp.Body.Close()
	if resp.StatusCode!=http.StatusOK{common.APIError(w,resp.StatusCode,"INVOICE_PDF","Invoice PDF is not available");return}
	w.Header().Set("Content-Type","application/pdf")
	if cd:=resp.Header.Get("Content-Disposition");cd!=""{w.Header().Set("Content-Disposition",cd)}
	w.Header().Set("Cache-Control","private, no-store")
	w.WriteHeader(http.StatusOK)
	_,_=io.Copy(w,resp.Body)
}

func (a *app) partnerImpactSummary(w http.ResponseWriter,r *http.Request,u partnerUser){
	var out map[string]any
	if err:=a.internalGET(r.Context(),a.hosts["impact"],"/api/v1/impact/summary?partner_id="+url.QueryEscape(u.PartnerID),&out);err!=nil{
		common.APIError(w,502,"IMPACT_UNAVAILABLE","Impact results are temporarily unavailable");return}
	common.JSON(w,200,out)
}

func (a *app) listPartnerUsers(partnerID string)([]map[string]any,error){
	rows,err:=a.db.Query(`SELECT pu.id,pu.partner_id,pu.name,pu.email,pu.password_hash,pu.role_key,pu.active,pu.preferred_locale,pu.timezone,pu.session_version,pu.created_at,pu.updated_at,
		pu.module_access_mode,
		COALESCE((SELECT COUNT(*) FROM identity.partner_user_modules pum WHERE pum.partner_id=pu.partner_id AND pum.user_id=pu.id),0)
		FROM identity.partner_users pu
		WHERE pu.partner_id=$1
		ORDER BY CASE pu.role_key WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 WHEN 'billing' THEN 2 ELSE 3 END,lower(pu.name)`,partnerID)
	if err!=nil{return nil,err};defer rows.Close()
	items:=[]map[string]any{}
	for rows.Next(){
		var u partnerUser
		var mode string
		var selectedCount int
		if err:=rows.Scan(&u.ID,&u.PartnerID,&u.Name,&u.Email,&u.PasswordHash,&u.Role,&u.Active,&u.PreferredLocale,&u.Timezone,&u.SessionVersion,&u.CreatedAt,&u.UpdatedAt,&mode,&selectedCount);err!=nil{return nil,err}
		item:=partnerUserMap(u)
		item["module_access_mode"]=normalizePartnerModuleAccessMode(mode)
		item["selected_module_count"]=selectedCount
		items=append(items,item)
	}
	return items,rows.Err()
}

func (a *app) validatePartnerUserEmail(email,currentID string)error{
	email=strings.ToLower(strings.TrimSpace(email))
	if !validEmail(email){return fmt.Errorf("a valid email is required")}
	var count int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM identity.users WHERE lower(email)=lower($1)`,email).Scan(&count)
	if count>0{return fmt.Errorf("email is already used by a HIMATE administrator")}
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM identity.partner_users WHERE lower(email)=lower($1) AND id<>$2`,email,currentID).Scan(&count)
	if count>0{return fmt.Errorf("email is already used by a Partner Portal user")}
	return nil
}

func (a *app) partnerUsers(w http.ResponseWriter,r *http.Request,actor partnerUser){
	switch r.Method{
	case http.MethodGet:
		items,err:=a.listPartnerUsers(actor.PartnerID);if err!=nil{common.APIError(w,500,"DB","Could not load partner users");return}
		common.JSON(w,200,map[string]any{"items":items,"count":len(items)})
	case http.MethodPost:
		var in struct{Name string `json:"name"`;Email string `json:"email"`;Password string `json:"password"`;Role string `json:"role"`}
		if common.Decode(r,&in)!=nil{common.APIError(w,400,"JSON","Invalid request");return}
		in.Name=strings.TrimSpace(in.Name);in.Email=strings.ToLower(strings.TrimSpace(in.Email));in.Role=strings.ToLower(strings.TrimSpace(in.Role))
		if len(in.Name)<2{common.APIError(w,400,"VALIDATION","Name is required");return}
		if err:=a.validatePartnerUserEmail(in.Email,"");err!=nil{common.APIError(w,409,"EMAIL",err.Error());return}
		if !partnerRoleValid(in.Role){common.APIError(w,400,"VALIDATION","Invalid Partner Portal role");return}
		if actor.Role!="owner"&&in.Role=="owner"{common.APIError(w,403,"FORBIDDEN","Only a partner owner can create another owner");return}
		if message:=passwordPolicyError(in.Password);message!=""{common.APIError(w,400,"VALIDATION",message);return}
		hash,err:=hashPassword(in.Password);if err!=nil{common.APIError(w,500,"PASSWORD","Could not secure password");return}
		id,err:=partnerUserID();if err!=nil{common.APIError(w,500,"ID","Could not allocate user ID");return}
		var u partnerUser
		err=a.db.QueryRow(`INSERT INTO identity.partner_users(id,partner_id,name,email,password_hash,role_key)
			VALUES($1,$2,$3,$4,$5,$6)
			RETURNING id,partner_id,name,email,password_hash,role_key,active,preferred_locale,timezone,session_version,created_at,updated_at`,
			id,actor.PartnerID,in.Name,in.Email,hash,in.Role).Scan(&u.ID,&u.PartnerID,&u.Name,&u.Email,&u.PasswordHash,&u.Role,&u.Active,&u.PreferredLocale,&u.Timezone,&u.SessionVersion,&u.CreatedAt,&u.UpdatedAt)
		if err!=nil{common.APIError(w,409,"CONFLICT","Partner user could not be created");return}
		common.JSON(w,201,partnerUserMap(u))
	default:common.APIError(w,405,"METHOD","Use GET or POST")
	}
}

func (a *app) lastPartnerOwner(partnerID,excludeID string)bool{
	var count int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM identity.partner_users WHERE partner_id=$1 AND role_key='owner' AND active=TRUE AND id<>$2`,partnerID,excludeID).Scan(&count)
	return count==0
}

func partnerAuthenticationStateChanged(current,next partnerUser)bool{
	return current.Active!=next.Active||current.Role!=next.Role||!strings.EqualFold(current.Email,next.Email)
}

func (a *app) updatePartnerUserRecord(w http.ResponseWriter,r *http.Request,actorRole,partnerID,targetID string){
	var current partnerUser
	err:=a.db.QueryRow(`SELECT id,partner_id,name,email,password_hash,role_key,active,preferred_locale,timezone,session_version,created_at,updated_at
		FROM identity.partner_users WHERE id=$1 AND partner_id=$2`,targetID,partnerID).
		Scan(&current.ID,&current.PartnerID,&current.Name,&current.Email,&current.PasswordHash,&current.Role,&current.Active,&current.PreferredLocale,&current.Timezone,&current.SessionVersion,&current.CreatedAt,&current.UpdatedAt)
	if err!=nil{common.APIError(w,404,"NOT_FOUND","Partner user not found");return}
	if actorRole!="owner"&&current.Role=="owner"{common.APIError(w,403,"FORBIDDEN","Only a partner owner can modify an owner");return}
	var in struct{Name *string `json:"name"`;Email *string `json:"email"`;Password *string `json:"password"`;Role *string `json:"role"`;Active *bool `json:"active"`}
	if common.Decode(r,&in)!=nil{common.APIError(w,400,"JSON","Invalid request");return}
	next:=current
	if in.Name!=nil{next.Name=strings.TrimSpace(*in.Name)}
	if in.Email!=nil{next.Email=strings.ToLower(strings.TrimSpace(*in.Email))}
	if in.Role!=nil{next.Role=strings.ToLower(strings.TrimSpace(*in.Role))}
	if in.Active!=nil{next.Active=*in.Active}
	if len(next.Name)<2{common.APIError(w,400,"VALIDATION","Name is required");return}
	if err:=a.validatePartnerUserEmail(next.Email,current.ID);err!=nil{common.APIError(w,409,"EMAIL",err.Error());return}
	if !partnerRoleValid(next.Role){common.APIError(w,400,"VALIDATION","Invalid Partner Portal role");return}
	if actorRole!="owner"&&next.Role=="owner"{common.APIError(w,403,"FORBIDDEN","Only a partner owner can assign the owner role");return}
	if current.Role=="owner"&&current.Active&&(next.Role!="owner"||!next.Active)&&a.lastPartnerOwner(partnerID,current.ID){
		common.APIError(w,409,"LAST_OWNER","A partner must always have at least one active owner");return}
	hash:=current.PasswordHash;sessionVersion:=current.SessionVersion
	passwordChanged:=false
	if in.Password!=nil&&strings.TrimSpace(*in.Password)!=""{
		if message:=passwordPolicyError(*in.Password);message!=""{common.APIError(w,400,"VALIDATION",message);return}
		hash,err=hashPassword(*in.Password);if err!=nil{common.APIError(w,500,"PASSWORD","Could not secure password");return}
		sessionVersion++;passwordChanged=true
	}
	if partnerAuthenticationStateChanged(current,next){sessionVersion++}
	err=a.db.QueryRow(`UPDATE identity.partner_users SET name=$3,email=$4,password_hash=$5,role_key=$6,active=$7,session_version=$8,
		password_changed_at=CASE WHEN $9 THEN NOW() ELSE password_changed_at END,updated_at=NOW()
		WHERE id=$1 AND partner_id=$2
		RETURNING id,partner_id,name,email,password_hash,role_key,active,preferred_locale,timezone,session_version,created_at,updated_at`,
		current.ID,partnerID,next.Name,next.Email,hash,next.Role,next.Active,sessionVersion,passwordChanged).
		Scan(&next.ID,&next.PartnerID,&next.Name,&next.Email,&next.PasswordHash,&next.Role,&next.Active,&next.PreferredLocale,&next.Timezone,&next.SessionVersion,&next.CreatedAt,&next.UpdatedAt)
	if err!=nil{common.APIError(w,500,"DB","Could not update partner user");return}
	common.JSON(w,200,partnerUserMap(next))
}

func (a *app) partnerUserUpdate(w http.ResponseWriter,r *http.Request,actor partnerUser){
	targetID:=strings.Trim(strings.TrimPrefix(r.URL.Path,"/partner/api/v1/users/"),"/")
	if targetID==""||strings.Contains(targetID,"/"){common.APIError(w,404,"NOT_FOUND","Partner user not found");return}
	a.updatePartnerUserRecord(w,r,actor.Role,actor.PartnerID,targetID)
}

func (a *app) adminPartnerUsers(w http.ResponseWriter,r *http.Request,actor user){
	if !ownerRequired(w,actor){return}
	raw:=strings.Trim(strings.TrimPrefix(r.URL.Path,"/api/v1/partners/"),"/")
	parts:=strings.Split(raw,"/")
	if len(parts)<2||parts[1]!="portal-users"{common.APIError(w,404,"NOT_FOUND","Partner Portal user route not found");return}
	partnerID:=parts[0]
	var partner map[string]any
	if err:=a.internalGET(r.Context(),a.hosts["partners"],"/api/v1/partners/"+url.PathEscape(partnerID),&partner);err!=nil{
		common.APIError(w,404,"PARTNER_NOT_FOUND","Partner not found");return
	}
	if len(parts)==3{
		if r.Method!=http.MethodPatch{common.APIError(w,405,"METHOD","Use PATCH");return}
		a.updatePartnerUserRecord(w,r,"owner",partnerID,parts[2]);return
	}
	switch r.Method{
	case http.MethodGet:
		items,err:=a.listPartnerUsers(partnerID);if err!=nil{common.APIError(w,500,"DB","Could not load Partner Portal users");return}
		common.JSON(w,200,map[string]any{"partner_id":partnerID,"items":items,"count":len(items)})
	case http.MethodPost:
		var in struct{Name string `json:"name"`;Email string `json:"email"`;Password string `json:"password"`;Role string `json:"role"`}
		if common.Decode(r,&in)!=nil{common.APIError(w,400,"JSON","Invalid request");return}
		in.Name=strings.TrimSpace(in.Name);in.Email=strings.ToLower(strings.TrimSpace(in.Email));in.Role=strings.ToLower(strings.TrimSpace(in.Role))
		if in.Role==""{in.Role="owner"}
		if len(in.Name)<2{common.APIError(w,400,"VALIDATION","Name is required");return}
		if err:=a.validatePartnerUserEmail(in.Email,"");err!=nil{common.APIError(w,409,"EMAIL",err.Error());return}
		if !partnerRoleValid(in.Role){common.APIError(w,400,"VALIDATION","Invalid Partner Portal role");return}
		if message:=passwordPolicyError(in.Password);message!=""{common.APIError(w,400,"VALIDATION",message);return}
		hash,err:=hashPassword(in.Password);if err!=nil{common.APIError(w,500,"PASSWORD","Could not secure password");return}
		id,err:=partnerUserID();if err!=nil{common.APIError(w,500,"ID","Could not allocate user ID");return}
		var u partnerUser
		err=a.db.QueryRow(`INSERT INTO identity.partner_users(id,partner_id,name,email,password_hash,role_key)
			VALUES($1,$2,$3,$4,$5,$6)
			RETURNING id,partner_id,name,email,password_hash,role_key,active,preferred_locale,timezone,session_version,created_at,updated_at`,
			id,partnerID,in.Name,in.Email,hash,in.Role).Scan(&u.ID,&u.PartnerID,&u.Name,&u.Email,&u.PasswordHash,&u.Role,&u.Active,&u.PreferredLocale,&u.Timezone,&u.SessionVersion,&u.CreatedAt,&u.UpdatedAt)
		if err!=nil{common.APIError(w,409,"CONFLICT","Partner Portal user could not be created");return}
		common.JSON(w,201,partnerUserMap(u))
	default:common.APIError(w,405,"METHOD","Use GET or POST")
	}
}
