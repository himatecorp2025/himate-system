package common

import (
	"net/http"
	"strings"
)

func NormalizeLocale(raw string) string {
	v := strings.TrimSpace(strings.ReplaceAll(raw, "-", "_"))
	switch strings.ToLower(v) {
	case "hu", "hu_hu":
		return "hu_HU"
	default:
		return "en_US"
	}
}

func RequestLocale(r *http.Request) string {
	if r == nil {
		return "en_US"
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("locale")); raw != "" {
		return NormalizeLocale(raw)
	}
	if raw := strings.TrimSpace(r.Header.Get("X-Himate-Locale")); raw != "" {
		return NormalizeLocale(raw)
	}
	if raw := strings.TrimSpace(r.Header.Get("Accept-Language")); raw != "" {
		first := strings.TrimSpace(strings.Split(raw, ",")[0])
		if i := strings.Index(first, ";"); i >= 0 {
			first = first[:i]
		}
		return NormalizeLocale(first)
	}
	return "en_US"
}

func Localized(en, hu, locale string) string {
	en = strings.TrimSpace(en)
	hu = strings.TrimSpace(hu)
	if NormalizeLocale(locale) == "hu_HU" {
		if hu != "" {
			return hu
		}
		return en
	}
	if en != "" {
		return en
	}
	return hu
}
