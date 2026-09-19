package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"himate.local/backend/internal/auth"
	"himate.local/backend/internal/config"
	"himate.local/backend/internal/model"
	"himate.local/backend/internal/service"
)

const sessionCookieName = "himate_session"

type Server struct {
	cfg      config.Config
	auth     *service.AuthService
	sessions *auth.SessionManager
	logger   *slog.Logger
	now      func() time.Time
}

func NewServer(cfg config.Config, authService *service.AuthService, sessions *auth.SessionManager, logger *slog.Logger) *Server {
	return &Server{
		cfg:      cfg,
		auth:     authService,
		sessions: sessions,
		logger:   logger,
		now:      time.Now,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/health", s.handleHealth)
	mux.HandleFunc("/api/v1/auth/login", s.handleLogin)
	mux.HandleFunc("/api/v1/auth/logout", s.handleLogout)
	mux.Handle("/api/v1/auth/me", s.requireAuth(http.HandlerFunc(s.handleMe)))
	mux.Handle("/api/v1/dashboard/summary", s.requireAuth(http.HandlerFunc(s.handleDashboardSummary)))
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "API_NOT_FOUND", "API endpoint not found")
	})
	mux.Handle("/", s.spaHandler())

	var handler http.Handler = mux
	handler = s.securityHeaders(handler)
	handler = s.cors(handler)
	handler = s.requestLogger(handler)
	return handler
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":       "ok",
		"service":      "himate",
		"environment":  s.cfg.Environment,
		"version":      s.cfg.AppVersion,
		"storage_mode": "bootstrap_memory",
		"timestamp":    s.now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}

	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r.Body, &input); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid request body")
		return
	}

	user, err := s.auth.Authenticate(input.Email, input.Password)
	if err != nil {
		s.logger.Warn("authentication failed", "email", strings.ToLower(strings.TrimSpace(input.Email)))
		writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid email or password")
		return
	}

	token, err := s.sessions.Issue(user.ID, user.Email, user.Name, user.Roles, s.now())
	if err != nil {
		s.logger.Error("issue session", "error", err)
		writeError(w, http.StatusInternalServerError, "SESSION_ERROR", "Unable to create session")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(s.cfg.SessionTTL.Seconds()),
	})

	s.logger.Info("authentication succeeded", "user_id", user.ID)
	writeJSON(w, http.StatusOK, publicUser(user))
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	claims, ok := claimsFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}
	user, err := s.auth.FindUser(claims.Subject)
	if err != nil || !user.Active {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}
	writeJSON(w, http.StatusOK, publicUser(user))
}

func (s *Server) handleDashboardSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"partners": map[string]any{
			"total":   0,
			"live":    0,
			"staging": 0,
		},
		"modules": map[string]any{
			"active":       0,
			"not_licensed": 0,
			"maintenance":  0,
		},
		"impact": map[string]any{
			"status":           "not_collected",
			"verified_metrics": 0,
		},
		"system": map[string]any{
			"status":      "healthy",
			"environment": s.cfg.Environment,
			"version":     s.cfg.AppVersion,
		},
	})
}

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
			return
		}
		claims, err := s.sessions.Parse(cookie.Value, s.now())
		if err != nil {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
			return
		}
		next.ServeHTTP(w, r.WithContext(withClaims(r.Context(), claims)))
	})
}

func (s *Server) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := s.now()
		next.ServeHTTP(w, r)
		s.logger.Info("http request", "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(start).Milliseconds())
	})
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline' 'wasm-unsafe-eval'; connect-src 'self' http://localhost:* ws://localhost:*; font-src 'self' data:; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) cors(next http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(s.cfg.AllowedOrigins))
	for _, origin := range s.cfg.AllowedOrigins {
		allowed[origin] = struct{}{}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			if _, ok := allowed[origin]; ok {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
				w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
			}
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) spaHandler() http.Handler {
	root := filepath.Clean(s.cfg.WebDistDir)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Not found")
			return
		}

		clean := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/"))
		if clean == "." {
			clean = "index.html"
		}
		candidate := filepath.Join(root, clean)
		if !strings.HasPrefix(candidate, root) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Not found")
			return
		}

		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			if contentType := mime.TypeByExtension(filepath.Ext(candidate)); contentType != "" {
				w.Header().Set("Content-Type", contentType)
			}
			http.ServeFile(w, r, candidate)
			return
		}

		index := filepath.Join(root, "index.html")
		if _, err := os.Stat(index); err != nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"service": "himate",
				"message": "Flutter web build not found. API is running.",
			})
			return
		}
		http.ServeFile(w, r, index)
	})
}

func publicUser(user model.User) map[string]any {
	return map[string]any{
		"id":    user.ID,
		"name":  user.Name,
		"email": user.Email,
		"roles": user.Roles,
	}
}

func decodeJSON(body io.Reader, dst any) error {
	decoder := json.NewDecoder(io.LimitReader(body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if decoder.More() {
		return errors.New("multiple JSON values are not allowed")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

func writeMethodNotAllowed(w http.ResponseWriter, allowed string) {
	w.Header().Set("Allow", allowed)
	writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", fmt.Sprintf("Use %s", allowed))
}
