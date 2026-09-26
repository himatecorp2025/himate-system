package main

import (
	"strings"
	"testing"
)

func TestCentral14ShortDBName(t *testing.T) {
	got:=shortDBName(strings.Repeat("x",70),"rollback123")
	if len(got)>63 { t.Fatalf("database name length=%d want <=63: %q",len(got),got) }
	if !strings.HasSuffix(got,"_rollback123") { t.Fatalf("database name=%q missing suffix",got) }
}

func TestCentral14MapSubset(t *testing.T) {
	source:=map[string]any{"a":1,"b":2,"secret":"do-not-copy"}
	got:=mapSubset(source,"a","b")
	if len(got)!=2 || got["a"]!=1 || got["b"]!=2 { t.Fatalf("subset=%v",got) }
	if _,ok:=got["secret"];ok { t.Fatalf("unexpected secret in subset: %v",got) }
}
