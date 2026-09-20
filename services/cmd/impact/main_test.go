package main

import (
	"testing"
	"time"
)

func TestMetricProvenanceContract(t *testing.T) {
	for _,v:=range []string{"SYSTEM","MANUAL","PARTNER_DECLARED","VERIFIED_DOCUMENT"}{
		if !provenanceValues[v]{t.Fatalf("missing provenance %s",v)}
	}
}

func TestMetricInputValidation(t *testing.T) {
	n:=12.0
	in:=metricInput{MetricKey:"culture.events",PeriodStart:"2026-01-01",PeriodEnd:"2026-01-31",NumericValue:&n,Provenance:"manual"}
	out,_,_,err:=parseMetricInput(in)
	if err!=nil{t.Fatal(err)}
	if out.Provenance!="MANUAL"{t.Fatalf("unexpected provenance %s",out.Provenance)}
}

func TestMetricProvenanceTrustBoundaries(t *testing.T) {
	if !adminProvenanceAllowed("manual") {
		t.Fatal("admin MANUAL provenance must be accepted")
	}
	for _, value := range []string{"SYSTEM", "PARTNER_DECLARED"} {
		if adminProvenanceAllowed(value) {
			t.Fatalf("admin must not be able to self-assert %s provenance", value)
		}
	}
	if !adminProvenanceAllowed("VERIFIED_DOCUMENT") {
		t.Fatal("admin VERIFIED_DOCUMENT must be admitted to the evidence-validation gate")
	}
	for _, value := range []string{"SYSTEM", "PARTNER_DECLARED"} {
		if !connectorProvenanceAllowed(value) {
			t.Fatalf("connector should accept %s", value)
		}
	}
	for _, value := range []string{"MANUAL", "VERIFIED_DOCUMENT"} {
		if connectorProvenanceAllowed(value) {
			t.Fatalf("connector must not assert %s provenance", value)
		}
	}
}

func TestMetricPayloadHashChangesWithPayload(t *testing.T) {
	n1, n2 := 7.0, 8.0
	start, _ := time.Parse("2006-01-02", "2026-09-01")
	end, _ := time.Parse("2006-01-02", "2026-09-20")
	a := metricInput{PartnerID:"ptr_000002",MetricKey:"culture.events",NumericValue:&n1,Provenance:"PARTNER_DECLARED"}
	b := metricInput{PartnerID:"ptr_000002",MetricKey:"culture.events",NumericValue:&n2,Provenance:"PARTNER_DECLARED"}
	if metricPayloadHash(a,start,end)==metricPayloadHash(b,start,end) {
		t.Fatal("different metric payloads must not share a payload hash")
	}
}
