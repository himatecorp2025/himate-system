package main

import "testing"

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
	for _, value := range []string{"SYSTEM", "PARTNER_DECLARED", "VERIFIED_DOCUMENT"} {
		if adminProvenanceAllowed(value) {
			t.Fatalf("admin must not be able to self-assert %s provenance", value)
		}
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
