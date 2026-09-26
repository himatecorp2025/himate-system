package main

import "testing"

func TestCentral14IntQuery(t *testing.T) {
	if got:=central14IntQuery("",60,200);got!=60 { t.Fatalf("default=%d",got) }
	if got:=central14IntQuery("500",60,200);got!=200 { t.Fatalf("bounded=%d",got) }
	if got:=central14IntQuery("-1",60,200);got!=60 { t.Fatalf("negative fallback=%d",got) }
}

func TestCentral14AdministrationAggregates(t *testing.T) {
	rows:=[]map[string]any{
		{"document_count":2,"recoverability_status":"VERIFIED"},
		{"document_count":3,"recoverability_status":"FAILED"},
		{"document_count":0,"recoverability_status":"UNVERIFIED"},
	}
	if got:=central14Sum(rows,"document_count");got!=5 { t.Fatalf("sum=%d",got) }
	if got:=central14CountStatus(rows,"recoverability_status","VERIFIED");got!=1 { t.Fatalf("verified=%d",got) }
	if got:=central14CountNotStatus(rows,"recoverability_status","VERIFIED");got!=2 { t.Fatalf("not verified=%d",got) }
}
