package main

import (
	"database/sql"
	"testing"
	"time"
)

func TestCentral13ConnectionStatus(t *testing.T) {
	cases := []struct{
		active, configured, degraded bool
		want string
	}{
		{true,true,false,"ACTIVE"},
		{true,true,true,"SUSPENDED"},
		{false,true,true,"SUSPENDED"},
		{false,true,false,"INACTIVE"},
		{false,false,false,"INACTIVE"},
	}
	for _, tc := range cases {
		if got := central13ConnectionStatus(tc.active, tc.configured, tc.degraded); got != tc.want {
			t.Fatalf("status(%v,%v,%v)=%s want %s", tc.active,tc.configured,tc.degraded,got,tc.want)
		}
	}
}

func TestCentral13LatestTime(t *testing.T) {
	base:=time.Date(2026,9,26,12,0,0,0,time.UTC)
	got:=central13LatestTime(
		sql.NullTime{Time:base,Valid:true},
		sql.NullTime{},
		sql.NullTime{Time:base.Add(2*time.Hour),Valid:true},
	)
	if !got.Valid || !got.Time.Equal(base.Add(2*time.Hour)) {
		t.Fatalf("latest=%v",got)
	}
}
