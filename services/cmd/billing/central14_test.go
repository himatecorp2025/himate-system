package main

import "testing"

func TestCentral14DocumentSearch(t *testing.T) {
	if got:=central14DocumentSearch("  Governance Policy  ");got!="%governance policy%" {
		t.Fatalf("search=%q",got)
	}
}
