package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bold-minds/trek/server/internal/api"
	"github.com/bold-minds/trek/server/internal/caps"
	"github.com/bold-minds/trek/server/internal/metrics"
	"github.com/bold-minds/trek/server/internal/notify"
	"github.com/bold-minds/trek/server/internal/scheduler"
	"github.com/bold-minds/trek/server/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Database
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://localhost:5432/trek?sslmode=disable"
	}

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return err
	}
	slog.Info("connected to database")

	st := store.NewPostgresStore(pool)

	// Redis/ValKey for global caps (optional)
	var counter *caps.Counter
	if redisURL := os.Getenv("REDIS_URL"); redisURL != "" {
		var err error
		counter, err = caps.NewCounter(redisURL)
		if err != nil {
			slog.Warn("redis connection failed, global caps disabled", "error", err)
		} else {
			slog.Info("redis connected, global caps enabled")
			defer counter.Close()
		}
	} else {
		slog.Info("REDIS_URL not set, global caps disabled (local caps only)")
	}

	// Notifications
	notifier := setupNotifier()

	// Expiry scheduler
	expiryScheduler := scheduler.NewExpiryScheduler(st, notifier, 30*time.Second)
	go expiryScheduler.Start(ctx)

	// Router with metrics middleware
	router := api.NewRouterWithConfig(api.RouterConfig{
		Store:   st,
		Counter: counter,
	})
	router.Use(metrics.FiberMetricsMiddleware)
	router.Get("/metrics", metrics.FiberHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	go func() {
		slog.Info("starting server", "port", port, "metrics", "/metrics")
		if err := router.Listen(":" + port); err != nil {
			slog.Error("server error", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server")

	// Stop scheduler
	expiryScheduler.Stop()

	return router.ShutdownWithTimeout(30 * time.Second)
}

func setupNotifier() *notify.Notifier {
	var configs []notify.NotifyConfig

	// Slack webhook
	if slackURL := os.Getenv("SLACK_WEBHOOK_URL"); slackURL != "" {
		configs = append(configs, notify.NotifyConfig{
			Type:    "slack",
			URL:     slackURL,
			Enabled: true,
		})
		slog.Info("slack notifications enabled")
	}

	// Generic webhooks (comma-separated)
	if webhookURLs := os.Getenv("WEBHOOK_URLS"); webhookURLs != "" {
		for _, url := range strings.Split(webhookURLs, ",") {
			url = strings.TrimSpace(url)
			if url != "" {
				configs = append(configs, notify.NotifyConfig{
					Type:    "webhook",
					URL:     url,
					Enabled: true,
				})
			}
		}
		slog.Info("webhook notifications enabled", "count", len(configs)-1)
	}

	return notify.NewNotifier(configs)
}
