package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSTART222PartnerRoleBoundary(t *testing.T) {
	owner := partnerUser{Role:"owner"}
	admin := partnerUser{Role:"admin"}
	billing := partnerUser{Role:"billing"}
	viewer := partnerUser{Role:"viewer"}

	if !partnerCan(owner,"users.write") || !partnerCan(owner,"modules.write") {
		t.Fatal("partner owner must have full portal authority")
	}
	if !partnerCan(admin,"users.write") || !partnerCan(admin,"company.write") {
		t.Fatal("partner admin must manage company and portal users")
	}
	if partnerCan(billing,"users.write") || !partnerCan(billing,"modules.write") {
		t.Fatal("billing role boundary is incorrect")
	}
	if partnerCan(viewer,"modules.write") || !partnerCan(viewer,"impact.read") {
		t.Fatal("viewer must remain read-only")
	}
	for _, role := range []string{"owner","admin","billing","viewer"} {
		for _, permission := range partnerPermissions(role) {
			if permission == "administration.approve" || strings.HasPrefix(permission,"administration.") {
				t.Fatalf("partner role %s leaked control-plane administration permission %s",role,permission)
			}
		}
	}
}

func TestSTART222PartnerSessionRoundTrip(t *testing.T) {
	a:=&app{secret:"0123456789012345678901234567890123456789",ttl:8*time.Hour}
	u:=partnerUser{
		ID:"pusr_test",PartnerID:"ptr_test",Name:"Partner Owner",Email:"owner@example.com",
		Role:"owner",Active:true,SessionVersion:3,
	}
	token,err:=a.issuePartnerSession(u,time.Hour)
	if err!=nil{t.Fatalf("issue partner session: %v",err)}
	claims,err:=a.parsePartnerSession(token)
	if err!=nil{t.Fatalf("parse partner session: %v",err)}
	if claims.Sub!=u.ID||claims.PartnerID!=u.PartnerID||claims.Role!=u.Role||claims.Kind!="PARTNER"||claims.Version!=3{
		t.Fatalf("unexpected partner claims: %#v",claims)
	}
	if _,err:=a.parsePartnerSession(token+"tampered");err==nil{
		t.Fatal("tampered partner session must be rejected")
	}
}

func TestSTART222PartnerModulePathParser(t *testing.T) {
	if got:=partnerModuleKey("/partner/api/v1/modules/marketing.campaigns/activate","/activate");got!="marketing.campaigns"{
		t.Fatalf("activate path parsed as %q",got)
	}
	if got:=partnerModuleKey("/partner/api/v1/modules/finance/subscription","/subscription");got!="finance"{
		t.Fatalf("subscription path parsed as %q",got)
	}
}

func TestSTART222AdminPortalUserPermissionClassification(t *testing.T) {
	for _,tc:=range []struct{method,path,want string}{
		{http.MethodGet,"/api/v1/partners/ptr_1/portal-users","administration.read"},
		{http.MethodPost,"/api/v1/partners/ptr_1/portal-users","administration.approve"},
		{http.MethodPatch,"/api/v1/partners/ptr_1/portal-users/pusr_1","administration.approve"},
	}{
		req:=httptest.NewRequest(tc.method,tc.path,nil)
		if got:=requiredPermission(req);got!=tc.want{
			t.Fatalf("%s %s => %s, want %s",tc.method,tc.path,got,tc.want)
		}
	}
}


func TestSTART23113CPartnerAccessErrorClassification(t *testing.T) {
	tests := []struct{
		name string
		err error
		status int
		code string
	}{
		{name:"suspended",err:errPartnerPortalAccessDisabled,status:http.StatusForbidden,code:"PARTNER_ACCESS_DISABLED"},
		{name:"missing registry record",err:internalHTTPError{Status:http.StatusNotFound},status:http.StatusServiceUnavailable,code:"PARTNER_REGISTRY_NOT_READY"},
		{name:"registry auth",err:internalHTTPError{Status:http.StatusForbidden},status:http.StatusServiceUnavailable,code:"PARTNER_REGISTRY_AUTH_FAILED"},
		{name:"registry outage",err:internalHTTPError{Status:http.StatusBadGateway},status:http.StatusServiceUnavailable,code:"PARTNER_REGISTRY_UNAVAILABLE"},
		{name:"registry timeout",err:context.DeadlineExceeded,status:http.StatusServiceUnavailable,code:"PARTNER_REGISTRY_TIMEOUT"},
		{name:"registry unconfigured",err:errors.New("private service host is not configured"),status:http.StatusServiceUnavailable,code:"PARTNER_REGISTRY_UNCONFIGURED"},
	}
	for _,tc := range tests {
		t.Run(tc.name,func(t *testing.T){
			rec:=httptest.NewRecorder()
			writePartnerAccessError(rec,tc.err)
			if rec.Code!=tc.status { t.Fatalf("status=%d want=%d body=%s",rec.Code,tc.status,rec.Body.String()) }
			var body struct{ Error struct{ Code string `json:"code"` } `json:"error"` }
			if err:=json.Unmarshal(rec.Body.Bytes(),&body);err!=nil { t.Fatalf("decode response: %v",err) }
			if body.Error.Code!=tc.code { t.Fatalf("code=%q want=%q",body.Error.Code,tc.code) }
		})
	}
}

func TestSTART241PartnerAuthenticationStateChangeRotatesSession(t *testing.T) {
	base := partnerUser{Email:"owner@example.com",Role:"owner",Active:true}
	if partnerAuthenticationStateChanged(base, base) {
		t.Fatal("unchanged partner authentication state must not rotate session")
	}
	emailChanged := base
	emailChanged.Email = "new-owner@example.com"
	if !partnerAuthenticationStateChanged(base, emailChanged) {
		t.Fatal("partner email change must rotate session")
	}
	roleChanged := base
	roleChanged.Role = "admin"
	if !partnerAuthenticationStateChanged(base, roleChanged) {
		t.Fatal("partner role change must rotate session")
	}
	activeChanged := base
	activeChanged.Active = false
	if !partnerAuthenticationStateChanged(base, activeChanged) {
		t.Fatal("partner activation change must rotate session")
	}
}

