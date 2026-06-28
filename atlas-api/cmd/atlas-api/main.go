package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	"github.com/atlas/atlas-api/internal/auth"
	"github.com/atlas/atlas-api/internal/config"
	db "github.com/atlas/atlas-api/internal/db/sqlc"
	"github.com/atlas/atlas-api/internal/httpapi"
	"github.com/atlas/atlas-api/internal/observability"
	_ "github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(1)
	}

	logger, err := newLogger(cfg.LogLevel, cfg.Env)
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = logger.Sync()
	}()

	otelShutdown, err := observability.InitOTel(context.Background(), logger, cfg.ServiceName, cfg.Env)
	if err != nil {
		logger.Fatal("failed initializing OpenTelemetry", zap.Error(err))
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if shutdownErr := otelShutdown(shutdownCtx); shutdownErr != nil {
			logger.Warn("failed shutting down OpenTelemetry", zap.Error(shutdownErr))
		}
	}()

	database, err := sql.Open("pgx", cfg.PostgresURL)
	if err != nil {
		logger.Fatal("failed opening postgres connection", zap.Error(err))
	}
	defer database.Close()

	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()
	if err := database.PingContext(pingCtx); err != nil {
		logger.Fatal("failed pinging postgres", zap.Error(err), zap.String("postgres_url", redactDSN(cfg.PostgresURL)))
	}

	tokenSvc := auth.NewTokenService(cfg.JWTSecret, cfg.AccessTokenTTL(), cfg.RefreshTokenTTL(), time.Now)
	queries := db.New(database)
	router := httpapi.NewRouter(logger, cfg, queries, tokenSvc)

	srv := &http.Server{
		Addr:              cfg.ServerAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("starting API server", zap.String("addr", cfg.ServerAddr), zap.String("env", cfg.Env))
		if serveErr := srv.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			logger.Fatal("server exited with error", zap.Error(serveErr))
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info("shutdown signal received")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if shutdownErr := srv.Shutdown(shutdownCtx); shutdownErr != nil {
		logger.Error("graceful shutdown failed", zap.Error(shutdownErr))
	}
	logger.Info("server stopped")
}

func newLogger(level, env string) (*zap.Logger, error) {
	cfg := zap.NewProductionConfig()
	if env == config.EnvLocal {
		cfg = zap.NewDevelopmentConfig()
	}

	var parsedLevel zapcore.Level
	if err := parsedLevel.UnmarshalText([]byte(level)); err != nil {
		return nil, err
	}
	cfg.Level = zap.NewAtomicLevelAt(parsedLevel)

	return cfg.Build()
}

func redactDSN(dsn string) string {
	if dsn == "" {
		return dsn
	}

	parsed, err := url.Parse(dsn)
	if err == nil && parsed.Scheme != "" {
		if parsed.User != nil {
			username := parsed.User.Username()
			if _, hasPassword := parsed.User.Password(); hasPassword {
				parsed.User = url.UserPassword(username, "xxxxx")
			}
		}

		query := parsed.Query()
		for _, key := range []string{"password", "pass", "pwd"} {
			if _, ok := query[key]; ok {
				query.Set(key, "xxxxx")
			}
		}
		parsed.RawQuery = query.Encode()
		return parsed.String()
	}

	passwordKVPattern := regexp.MustCompile(`(?i)(password=)([^\s]+)`)
	return passwordKVPattern.ReplaceAllString(dsn, "${1}xxxxx")
}
