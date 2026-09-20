package main

import "testing"

func TestConnectorTokenHashIsStableAndOneWay(t *testing.T) {
	token:="hmc_crd_abc_secret"
	a:=tokenHash(token)
	b:=tokenHash(token)
	if a!=b || a==token { t.Fatal("connector token hash contract failed") }
	if normalizeEnvironment("staging")!="STAGING" || normalizeEnvironment("invalid")!="" { t.Fatal("environment normalization failed") }
}
