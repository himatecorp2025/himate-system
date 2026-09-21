package main

import (
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
