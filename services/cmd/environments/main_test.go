package main

import "testing"

func TestEnvironmentIdentityIsStable(t *testing.T) {
	if got:=envID("ptr_000042","STAGING");got!="env_staging_000042"{t.Fatalf("unexpected id %s",got)}
	if got:=slug("Gallery Company NYC");got!="gallery-company-nyc"{t.Fatalf("unexpected slug %s",got)}
}
