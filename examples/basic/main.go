// Example: basic
//
// This example demonstrates the minimal integration of Trek into an HTTP server.
// It shows how to initialize Trek, wrap the slog handler, and apply the middleware.
//
// Run with:
//
//	export TREK_ORG_ID=your-org
//	export TREK_ENV=dev
//	export TREK_API_ENDPOINT=http://localhost:8080
//	export TREK_API_TOKEN=your-token
//	go run main.go
package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	trek "github.com/bold-minds/trek"
)

func main() {
	// Initialize Trek with environment variables
	// Trek will log a warning and continue in disabled mode if config is invalid
	cfg := trek.DefaultConfig()
	cfg.ServiceName = "basic-example"

	if err := trek.Init(cfg); err != nil {
		slog.Error("trek initialization failed", "error", err)
		os.Exit(1)
	}
	defer trek.Shutdown()

	// Wrap the default slog handler with Trek
	// This enables request-scoped log level elevation
	handler := trek.WrapHandler(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo, // Default level - Trek will elevate when sessions match
	}))
	slog.SetDefault(slog.New(handler))

	// Set up routes
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", handleHome)
	mux.HandleFunc("GET /api/users/{id}", handleGetUser)
	mux.HandleFunc("POST /api/orders", handleCreateOrder)
	mux.HandleFunc("GET /health", handleHealth)

	// Wrap with Trek middleware
	// This evaluates each request against active debug sessions
	wrapped := trek.HTTPMiddleware(mux)

	addr := ":8081"
	slog.Info("starting server", "addr", addr)
	if err := http.ListenAndServe(addr, wrapped); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// These debug logs only appear when a matching Trek session is active
	slog.DebugContext(ctx, "handling home request")

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintln(w, "Welcome to the Trek Basic Example!")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Try these endpoints:")
	fmt.Fprintln(w, "  GET  /api/users/{id}  - Get user by ID")
	fmt.Fprintln(w, "  POST /api/orders      - Create an order")
	fmt.Fprintln(w, "  GET  /health          - Health check")
}

func handleGetUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := r.PathValue("id")

	// Debug logs for request processing - only visible when Trek session matches
	slog.DebugContext(ctx, "fetching user", "user_id", userID)
	slog.DebugContext(ctx, "checking cache", "cache_key", "user:"+userID)
	slog.DebugContext(ctx, "cache miss, querying database")

	// Simulate some processing
	slog.InfoContext(ctx, "user retrieved", "user_id", userID)

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"id": "%s", "name": "User %s", "email": "user%s@example.com"}`, userID, userID, userID)
}

func handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Debug logs show the order creation flow
	slog.DebugContext(ctx, "parsing order request")
	slog.DebugContext(ctx, "validating order items")
	slog.DebugContext(ctx, "checking inventory")
	slog.DebugContext(ctx, "calculating totals")

	slog.InfoContext(ctx, "order created", "order_id", "ord_12345")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	fmt.Fprint(w, `{"id": "ord_12345", "status": "created"}`)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"status": "ok", "trek_enabled": `+fmt.Sprintf("%t", trek.IsEnabled())+`}`)
}
