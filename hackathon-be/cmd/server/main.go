// Command server starts the NightWatch HTTP API and background monitor.
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

	"github.com/t-kaijifu/hackathon-be/internal/api"
	"github.com/t-kaijifu/hackathon-be/internal/config"
	"github.com/t-kaijifu/hackathon-be/internal/db"
	"github.com/t-kaijifu/hackathon-be/internal/monitor"
	"github.com/t-kaijifu/hackathon-be/internal/notify"
	"github.com/t-kaijifu/hackathon-be/internal/service"
	"github.com/t-kaijifu/hackathon-be/internal/store"
)

func main() {
	cfg := config.Load()
	logger := newLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		logger.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}
	logger.Info("migrations applied")

	st := store.New(pool)
	notifier, err := buildNotifier(cfg, st, logger)
	if err != nil {
		logger.Error("failed to build notifier", "error", err)
		os.Exit(1)
	}
	svc := service.New(st, notifier,
		service.WithLogger(logger),
		service.WithDefaultLowBattery(cfg.DefaultLowBattery),
	)

	if cfg.MonitorEnabled {
		sched := monitor.New(svc, cfg.MonitorTick, logger)
		go sched.Run(ctx)
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           api.NewServer(svc, logger).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("http server listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server error", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
}

func newLogger(level string) *slog.Logger {
	var lv slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lv = slog.LevelDebug
	case "warn":
		lv = slog.LevelWarn
	case "error":
		lv = slog.LevelError
	default:
		lv = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lv}))
}

// buildNotifier always persists+logs messages (DBNotifier). When APNs is
// enabled, it additionally pushes to Apple devices via a MultiNotifier.
func buildNotifier(cfg config.Config, st *store.PgStore, logger *slog.Logger) (notify.Notifier, error) {
	db := notify.NewDBNotifier(st, logger)
	if !cfg.APNs.Enabled {
		logger.Info("APNs push notifications disabled (set APNS_ENABLED=true to enable)")
		return db, nil
	}
	keyPEM, err := os.ReadFile(cfg.APNs.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("read APNs auth key: %w", err)
	}
	apns, err := notify.NewAPNsNotifier(notify.APNsConfig{
		KeyID:      cfg.APNs.KeyID,
		TeamID:     cfg.APNs.TeamID,
		Topic:      cfg.APNs.Topic,
		Production: cfg.APNs.Production,
	}, keyPEM, st, logger)
	if err != nil {
		return nil, err
	}
	logger.Info("APNs push notifications enabled", "topic", cfg.APNs.Topic, "production", cfg.APNs.Production)
	return notify.NewMultiNotifier(db, logger, apns), nil
}
