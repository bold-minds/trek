package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bold-minds/trek"
)

func main() {
	logger := slog.New(trek.WrapHandler(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))
	slog.SetDefault(logger)

	if err := trek.Init(trek.Config{
		ServiceName:  getEnv("TREK_SERVICE_NAME", "demo-api"),
		OrgID:        getEnv("TREK_ORG_ID", "org_demo"),
		Env:          getEnv("TREK_ENV", "dev"),
		APIEndpoint:  getEnv("TREK_API_ENDPOINT", "http://localhost:8080"),
		APIToken:     getEnv("TREK_API_TOKEN", "demo-token"),
		PollInterval: 5 * time.Second,
	}); err != nil {
		slog.Warn("trek init failed, continuing without trek", "error", err)
	}
	defer trek.Shutdown()

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/orders", handleListOrders)
	mux.HandleFunc("GET /api/orders/{id}", handleGetOrder)
	mux.HandleFunc("POST /api/orders", handleCreateOrder)
	mux.HandleFunc("GET /health", handleHealth)

	handler := trek.HTTPMiddleware(mux)

	server := &http.Server{
		Addr:    ":3000",
		Handler: handler,
	}

	go func() {
		slog.Info("demo server starting", "port", 3000)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.Shutdown(ctx)
}

func handleListOrders(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	slog.InfoContext(ctx, "listing orders")
	slog.DebugContext(ctx, "fetching orders from database", "limit", 100)

	time.Sleep(50 * time.Millisecond)

	slog.DebugContext(ctx, "database query completed", "count", 42)

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"orders": [{"id": "ord_1", "status": "pending"}, {"id": "ord_2", "status": "completed"}]}`))
}

func handleGetOrder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orderID := r.PathValue("id")

	slog.InfoContext(ctx, "getting order", "order_id", orderID)
	slog.DebugContext(ctx, "looking up order in cache", "order_id", orderID)
	slog.DebugContext(ctx, "cache miss, querying database", "order_id", orderID)

	time.Sleep(30 * time.Millisecond)

	slog.DebugContext(ctx, "order found", "order_id", orderID, "status", "pending", "items", 3)

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"id": "%s", "status": "pending", "items": 3, "total": 99.99}`, orderID)
}

func handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	slog.InfoContext(ctx, "creating order")
	slog.DebugContext(ctx, "validating order payload")
	slog.DebugContext(ctx, "checking inventory availability")

	time.Sleep(100 * time.Millisecond)

	slog.DebugContext(ctx, "inventory reserved", "items", 2)
	slog.DebugContext(ctx, "order persisted", "order_id", "ord_new_123")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(`{"id": "ord_new_123", "status": "pending"}`))
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status": "ok"}`))
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
