package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"himate.local/backend/internal/auth"
	"himate.local/backend/internal/config"
	"himate.local/backend/internal/httpapi"
	"himate.local/backend/internal/model"
	"himate.local/backend/internal/security"
	"himate.local/backend/internal/service"
	"himate.local/backend/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration error", "error", err)
		os.Exit(1)
	}

	users := store.NewMemoryUserStore()
	if err := seedBootstrapAdmins(users, cfg.BootstrapAdmins); err != nil {
		logger.Error("bootstrap admin error", "error", err)
		os.Exit(1)
	}

	sessions, err := auth.NewSessionManager(cfg.SessionSecret, cfg.SessionTTL)
	if err != nil {
		logger.Error("session configuration error", "error", err)
		os.Exit(1)
	}

	authService := service.NewAuthService(users)
	api := httpapi.NewServer(cfg, authService, sessions, logger)

	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       20 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("HIMATE API started", "port", cfg.Port, "environment", cfg.Environment, "version", cfg.AppVersion)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
}

func seedBootstrapAdmins(users *store.MemoryUserStore, admins []config.BootstrapAdmin) error {
	for i, admin := range admins {
		hash, err := security.HashPassword(admin.Password)
		if err != nil {
			return fmt.Errorf("hash bootstrap admin password: %w", err)
		}
		user := model.User{
			ID:           fmt.Sprintf("usr_bootstrap_%03d", i+1),
			Name:         strings.TrimSpace(admin.Name),
			Email:        strings.ToLower(strings.TrimSpace(admin.Email)),
			PasswordHash: hash,
			Roles:        append([]string(nil), admin.Roles...),
			Active:       true,
			CreatedAt:    time.Now().UTC(),
		}
		if err := users.CreateUser(user); err != nil {
			return err
		}
	}
	return nil
}
