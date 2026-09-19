package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type BootstrapAdmin struct {
	Name     string   `json:"name"`
	Email    string   `json:"email"`
	Password string   `json:"password"`
	Roles    []string `json:"roles"`
}

type Config struct {
	Port              string
	Environment       string
	AppVersion        string
	WebDistDir        string
	SessionSecret     string
	SessionTTL        time.Duration
	CookieSecure      bool
	AllowedOrigins    []string
	BootstrapAdmins   []BootstrapAdmin
	DatabaseURL       string
	RenderServiceName string
}

func Load() (Config, error) {
	cfg := Config{
		Port:              getenv("PORT", "10000"),
		Environment:       getenv("HIMATE_ENV", "development"),
		AppVersion:        getenv("HIMATE_APP_VERSION", "0.1.0-start-01-03"),
		WebDistDir:        getenv("WEB_DIST_DIR", "../frontend/build/web"),
		SessionSecret:     os.Getenv("HIMATE_SESSION_SECRET"),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		RenderServiceName: getenv("RENDER_SERVICE_NAME", "himate"),
	}

	if cfg.SessionSecret == "" {
		return Config{}, errors.New("HIMATE_SESSION_SECRET is required")
	}

	ttlHours, err := strconv.Atoi(getenv("HIMATE_SESSION_TTL_HOURS", "8"))
	if err != nil || ttlHours < 1 || ttlHours > 168 {
		return Config{}, errors.New("HIMATE_SESSION_TTL_HOURS must be between 1 and 168")
	}
	cfg.SessionTTL = time.Duration(ttlHours) * time.Hour

	secure, err := strconv.ParseBool(getenv("COOKIE_SECURE", "false"))
	if err != nil {
		return Config{}, errors.New("COOKIE_SECURE must be true or false")
	}
	cfg.CookieSecure = secure

	if raw := strings.TrimSpace(os.Getenv("ALLOWED_ORIGINS")); raw != "" {
		for _, value := range strings.Split(raw, ",") {
			if origin := strings.TrimSpace(value); origin != "" {
				cfg.AllowedOrigins = append(cfg.AllowedOrigins, origin)
			}
		}
	}

	admins, err := loadBootstrapAdmins()
	if err != nil {
		return Config{}, err
	}
	if len(admins) == 0 {
		return Config{}, errors.New("at least one bootstrap HIMATE administrator is required")
	}
	cfg.BootstrapAdmins = admins

	return cfg, nil
}

func loadBootstrapAdmins() ([]BootstrapAdmin, error) {
	if raw := strings.TrimSpace(os.Getenv("HIMATE_BOOTSTRAP_ADMINS_JSON")); raw != "" {
		var admins []BootstrapAdmin
		if err := json.Unmarshal([]byte(raw), &admins); err != nil {
			return nil, fmt.Errorf("parse HIMATE_BOOTSTRAP_ADMINS_JSON: %w", err)
		}
		for i := range admins {
			if err := validateAdmin(admins[i]); err != nil {
				return nil, fmt.Errorf("bootstrap admin %d: %w", i+1, err)
			}
			if len(admins[i].Roles) == 0 {
				admins[i].Roles = []string{"platform_admin"}
			}
		}
		return admins, nil
	}

	email := strings.TrimSpace(os.Getenv("HIMATE_BOOTSTRAP_ADMIN_EMAIL"))
	password := os.Getenv("HIMATE_BOOTSTRAP_ADMIN_PASSWORD")
	if email == "" && password == "" {
		return nil, nil
	}
	admin := BootstrapAdmin{
		Name:     getenv("HIMATE_BOOTSTRAP_ADMIN_NAME", "HIMATE Administrator"),
		Email:    email,
		Password: password,
		Roles:    []string{"platform_admin"},
	}
	if err := validateAdmin(admin); err != nil {
		return nil, err
	}
	return []BootstrapAdmin{admin}, nil
}

func validateAdmin(admin BootstrapAdmin) error {
	if strings.TrimSpace(admin.Name) == "" {
		return errors.New("name is required")
	}
	if !strings.Contains(admin.Email, "@") {
		return errors.New("valid email is required")
	}
	if len(admin.Password) < 12 {
		return errors.New("password must contain at least 12 characters")
	}
	return nil
}

func getenv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}
