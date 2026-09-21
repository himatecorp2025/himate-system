package partnerdb

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var safeID = regexp.MustCompile(`^[a-z0-9_]+$`)

func DatabaseName(partnerID string) string {
	id := strings.ToLower(strings.TrimSpace(partnerID))
	id = strings.ReplaceAll(id, "-", "_")
	if !safeID.MatchString(id) {
		return ""
	}
	return "himate_" + id
}

func RoleName(partnerID string) string {
	name := DatabaseName(partnerID)
	if name == "" {
		return ""
	}
	return name + "_app"
}

func AdminDSN(baseURL string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", err
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("partner database admin URL is invalid")
	}
	u.Path = "/postgres"
	return u.String(), nil
}

func DatabaseDSN(baseURL, partnerID string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", err
	}
	name := DatabaseName(partnerID)
	if u.Scheme == "" || u.Host == "" || name == "" {
		return "", fmt.Errorf("partner database URL or partner id is invalid")
	}
	u.Path = "/" + name
	return u.String(), nil
}
