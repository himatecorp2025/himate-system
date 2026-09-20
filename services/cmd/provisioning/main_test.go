package main

import "testing"

func TestProvisioningIdentityAndPresetAreIdempotent(t *testing.T) {
	a:=&app{dbMasterSecret:"0123456789012345678901234567890123456789"}
	if got:=a.partnerDatabaseName("ptr_000042");got!="himate_ptr_000042"{t.Fatalf("unexpected db name %s",got)}
	if a.partnerPassword("ptr_000042")!=a.partnerPassword("ptr_000042"){t.Fatal("partner database password must be deterministic")}
	got:=uniqueStrings([]string{"a","b","a","","b"})
	if len(got)!=2 || got[0]!="a" || got[1]!="b"{t.Fatalf("unexpected preset %#v",got)}
}

func TestProvisioningStepOrder(t *testing.T){
	if stepOrder[0]!="VALIDATE_PARTNER" || stepOrder[len(stepOrder)-1]!="COMPLETE"{t.Fatal("provisioning step order changed")}
}
