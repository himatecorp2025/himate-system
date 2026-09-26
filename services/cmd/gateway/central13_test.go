package main

import "testing"

func TestCentral13ConnectionStatusForPartner(t *testing.T) {
	cases:=[]struct{ lifecycle, connector, want string }{
		{"LIVE","ACTIVE","ACTIVE"},
		{"LIVE","SUSPENDED","SUSPENDED"},
		{"LIVE","","INACTIVE"},
		{"PROSPECT","UNKNOWN","INACTIVE"},
		{"ARCHIVED","ACTIVE","DELETED"},
	}
	for _,tc:=range cases{
		if got:=central13ConnectionStatusForPartner(tc.lifecycle,tc.connector);got!=tc.want{
			t.Fatalf("status(%q,%q)=%q want %q",tc.lifecycle,tc.connector,got,tc.want)
		}
	}
}

func TestCentral13IntegrationTotal(t *testing.T) {
	rows:=[]map[string]any{
		{"integration_count":2},
		{"integration_count":1.0},
		{"integration_count":0},
	}
	if got:=central13IntegrationTotal(rows);got!=3{
		t.Fatalf("integration total=%d want 3",got)
	}
}
