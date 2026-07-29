package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/verdantflarehub/verdantflare-login/internal/auth"
	"github.com/verdantflarehub/verdantflare-login/internal/config"
	"github.com/verdantflarehub/verdantflare-login/internal/httpapi"
	"github.com/verdantflarehub/verdantflare-login/internal/mailer"
	"github.com/verdantflarehub/verdantflare-login/internal/store/memory"
	postgresstore "github.com/verdantflarehub/verdantflare-login/internal/store/postgres"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration invalid", "error", err)
		os.Exit(1)
	}

	var store auth.Store
	var closeStore func() error
	storeName := "memory"
	if cfg.DatabaseURL != "" {
		postgresStore, err := postgresstore.Open(cfg.DatabaseURL, cfg.DatabaseMaxOpen, cfg.DatabaseMaxIdle)
		if err != nil {
			logger.Error("database initialization failed", "error", err)
			os.Exit(1)
		}
		store = postgresStore
		closeStore = postgresStore.Close
		storeName = "postgres"
	} else {
		store = memory.New()
	}
	if closeStore != nil {
		defer func() {
			if err := closeStore(); err != nil {
				logger.Error("database close failed", "error", err)
			}
		}()
	}
	service, err := auth.NewService(store, mailer.LogMailer{Logger: logger}, auth.ServiceConfig{
		HubURL: cfg.HubURL, PublicLoginURL: cfg.PublicLoginURL,
		SessionTTL: cfg.SessionTTL, VerificationTTL: cfg.VerificationTTL,
		PasswordResetTTL: cfg.PasswordResetTTL, TokenPepper: cfg.TokenPepper,
		ExposeDebugCodes: cfg.ExposeDebugCodes,
	})
	if err != nil {
		logger.Error("authentication service initialization failed", "error", err)
		os.Exit(1)
	}

	handler := httpapi.New(service, httpapi.Config{
		CookieName: cfg.CookieName, CookieDomain: cfg.CookieDomain,
		CookieSecure: cfg.CookieSecure, SessionTTL: cfg.SessionTTL,
	}, logger)
	server := &http.Server{
		Addr: cfg.Address, Handler: handler,
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second,
	}

	shutdownSignals := make(chan os.Signal, 1)
	signal.Notify(shutdownSignals, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-shutdownSignals
		shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
		}
	}()

	logger.Info("login server started", "address", cfg.Address, "environment", cfg.Environment, "store", storeName)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("login server stopped unexpectedly", "error", err)
		os.Exit(1)
	}
}
