package common

import (
	"net/http"
	"testing"
)

func TestNormalizeLocale(t *testing.T) {
	cases:=map[string]string{
		"hu":"hu_HU","hu-HU":"hu_HU","hu_HU":"hu_HU",
		"en":"en_US","en-US":"en_US","en_US":"en_US","":"en_US","de-DE":"en_US",
	}
	for in,want:=range cases{
		if got:=NormalizeLocale(in);got!=want{t.Fatalf("NormalizeLocale(%q)=%q want %q",in,got,want)}
	}
}

func TestRequestLocalePriority(t *testing.T) {
	r,_:=http.NewRequest(http.MethodGet,"http://example.test/api?locale=hu_HU",nil)
	r.Header.Set("X-Himate-Locale","en_US")
	r.Header.Set("Accept-Language","en-US,en;q=0.9")
	if got:=RequestLocale(r);got!="hu_HU"{t.Fatalf("query locale priority failed: %s",got)}

	r,_=http.NewRequest(http.MethodGet,"http://example.test/api",nil)
	r.Header.Set("X-Himate-Locale","hu_HU")
	r.Header.Set("Accept-Language","en-US,en;q=0.9")
	if got:=RequestLocale(r);got!="hu_HU"{t.Fatalf("header locale priority failed: %s",got)}

	r,_=http.NewRequest(http.MethodGet,"http://example.test/api",nil)
	r.Header.Set("Accept-Language","hu-HU,hu;q=0.9,en;q=0.8")
	if got:=RequestLocale(r);got!="hu_HU"{t.Fatalf("accept-language fallback failed: %s",got)}
}

func TestLocalizedFallback(t *testing.T) {
	if got:=Localized("English","Magyar","hu_HU");got!="Magyar"{t.Fatalf("HU=%q",got)}
	if got:=Localized("English","Magyar","en_US");got!="English"{t.Fatalf("EN=%q",got)}
	if got:=Localized("English","","hu_HU");got!="English"{t.Fatalf("HU fallback=%q",got)}
	if got:=Localized("","Magyar","en_US");got!="Magyar"{t.Fatalf("EN fallback=%q",got)}
}
