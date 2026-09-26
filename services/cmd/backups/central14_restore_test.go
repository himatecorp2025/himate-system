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

func TestCentral14JSONEquivalentIgnoresMapKeyOrder(t *testing.T) {
	left:=map[string]any{"provider":"local","nested":map[string]any{"b":2,"a":1}}
	right:=map[string]any{"nested":map[string]any{"a":1,"b":2},"provider":"local"}
	if !jsonEquivalent(left,right){t.Fatalf("equivalent JSON maps must compare equal: %#v %#v",left,right)}
	right["provider"]="render"
	if jsonEquivalent(left,right){t.Fatalf("different runtime config must not be reusable: %#v %#v",left,right)}
}

func TestCentral14MapSubset(t *testing.T) {
	source:=map[string]any{"a":1,"b":2,"secret":"do-not-copy"}
	got:=mapSubset(source,"a","b")
	if len(got)!=2 || got["a"]!=1 || got["b"]!=2 { t.Fatalf("subset=%v",got) }
	if _,ok:=got["secret"];ok { t.Fatalf("unexpected secret in subset: %v",got) }
}
