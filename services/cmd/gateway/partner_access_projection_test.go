package main

import (
	"errors"
	"testing"
)

func TestPartnerAccessStateAllowed(t *testing.T) {
	tests := []struct {
		name string
		lifecycle string
		onboardingRequestID string
		onboardingState string
		portalEnabled bool
		onboardingExists bool
		wantDisabled bool
	}{
		{name:"admin-created default is active", lifecycle:"PROSPECT"},
		{name:"registered without onboarding row fails closed", lifecycle:"PROSPECT", onboardingRequestID:"req_1", wantDisabled:true},
		{name:"active onboarding is allowed", lifecycle:"ACTIVE", onboardingState:"ACTIVE", portalEnabled:true, onboardingExists:true},
		{name:"pending onboarding is disabled", lifecycle:"ACTIVE", onboardingState:"PENDING_REVIEW", onboardingExists:true, wantDisabled:true},
		{name:"suspended partner is disabled", lifecycle:"SUSPENDED", onboardingState:"ACTIVE", portalEnabled:true, onboardingExists:true, wantDisabled:true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := partnerAccessStateAllowed(tt.lifecycle, tt.onboardingRequestID, tt.onboardingState, tt.portalEnabled, tt.onboardingExists)
			if got := errors.Is(err, errPartnerPortalAccessDisabled); got != tt.wantDisabled {
				t.Fatalf("disabled=%v want=%v err=%v", got, tt.wantDisabled, err)
			}
		})
	}
}

func TestReadModelTenantSliceScopesCoverImmediatePartnerReads(t *testing.T) {
	modules, plans, design, access, billing := readModelTenantSliceScopes("/api/v1/billing/partners/ptr_1/onboarding/audit")
	if modules || plans || design || !access || billing {
		t.Fatalf("onboarding scopes modules=%v plans=%v design=%v access=%v billing=%v", modules, plans, design, access, billing)
	}
	modules, plans, design, access, billing = readModelTenantSliceScopes("/api/v1/billing/invoices/inv_1/mark-paid/audit")
	if modules || plans || design || access || !billing {
		t.Fatalf("invoice scopes modules=%v plans=%v design=%v access=%v billing=%v", modules, plans, design, access, billing)
	}
	modules, plans, design, access, billing = readModelTenantSliceScopes("/api/v1/partners/ptr_1/audit")
	if modules || plans || design || !access || billing {
		t.Fatalf("partner lifecycle scopes modules=%v plans=%v design=%v access=%v billing=%v", modules, plans, design, access, billing)
	}
}

func TestClassifyReadModelMutationAvoidsPartnerSubstringStorms(t *testing.T) {
	portalUsers := classifyReadModelMutation("/api/v1/partners/ptr_1/portal-users/audit")
	if portalUsers.partner || !portalUsers.admin || !portalUsers.tenantOnly { t.Fatalf("portal user scope=%+v", portalUsers) }
	design := classifyReadModelMutation("/partner/api/v1/design")
	if design.partner || !design.website || !design.tenantOnly { t.Fatalf("partner design scope=%+v", design) }
	company := classifyReadModelMutation("/partner/api/v1/company")
	if !company.partner || !company.tenantOnly { t.Fatalf("partner company scope=%+v", company) }
	master := classifyReadModelMutation("/api/v1/partners/ptr_1/audit")
	if !master.partner || !master.admin || master.tenantOnly { t.Fatalf("partner master scope=%+v", master) }
}
