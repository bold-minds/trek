// Demo "shop" service for Trek CLI demo
// Shows targeted log verbosity in action
package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	trek "github.com/bold-minds/trek"
)

func main() {
	// Initialize Trek SDK
	cfg := trek.DefaultConfig()
	cfg.ServiceName = "shop"

	if err := trek.Init(cfg); err != nil {
		slog.Warn("trek init failed, continuing without trek", "error", err)
	}
	defer trek.Shutdown()

	// Wrap slog handler with Trek for request-scoped log elevation
	handler := trek.WrapHandler(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo, // Default: only INFO and above
	}))
	slog.SetDefault(slog.New(handler))

	// Routes
	mux := http.NewServeMux()
	mux.HandleFunc("GET /checkout", handleCheckout)
	mux.HandleFunc("GET /health", handleHealth)

	// Wrap with Trek middleware (extracts tenant from X-Tenant header)
	wrapped := trek.HTTPMiddleware(mux)

	addr := ":9090"
	slog.Info("shop service starting", "addr", addr, "trek_enabled", trek.IsEnabled())

	if err := http.ListenAndServe(addr, wrapped); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func handleCheckout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := r.Header.Get("X-Tenant")

	// INFO log - always visible
	slog.InfoContext(ctx, "checkout request received", "tenant", tenant)

	// DEBUG logs - only visible when Trek session matches this tenant+route
	slog.DebugContext(ctx, "validating cart contents", "tenant", tenant)
	time.Sleep(10 * time.Millisecond) // simulate work

	slog.DebugContext(ctx, "checking inventory availability", "tenant", tenant)
	time.Sleep(10 * time.Millisecond)

	slog.DebugContext(ctx, "calculating shipping options", "tenant", tenant)
	time.Sleep(10 * time.Millisecond)

	slog.DebugContext(ctx, "processing payment", "tenant", tenant)
	time.Sleep(10 * time.Millisecond)

	// INFO log - always visible
	slog.InfoContext(ctx, "checkout completed", "tenant", tenant, "order_id", "ord_"+tenant+"_001")

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"ok","tenant":"%s","order_id":"ord_%s_001"}`, tenant, tenant)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"ok","trek_enabled":%t}`, trek.IsEnabled())
}
