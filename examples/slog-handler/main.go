// Example: slog-handler
//
// This example demonstrates Trek's slog.Handler integration with redaction.
// It shows how to wrap slog handlers and use built-in or custom redaction.
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
	"strings"

	trek "github.com/bold-minds/trek"
)

func main() {
	// Configure Trek with redaction
	cfg := trek.DefaultConfig()
	cfg.ServiceName = "slog-handler-example"

	// Use the built-in common redactor for sensitive fields
	cfg.RedactAttr = trek.CommonRedactor()

	// Or create a custom redactor
	cfg.RedactAttr = customRedactor()

	if err := trek.Init(cfg); err != nil {
		slog.Error("trek initialization failed", "error", err)
		os.Exit(1)
	}
	defer trek.Shutdown()

	// Create a JSON handler with custom options
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug, // Allow all levels, Trek controls elevation
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Customize time format
			if a.Key == slog.TimeKey {
				return slog.Attr{Key: "ts", Value: a.Value}
			}
			return a
		},
	})

	// Wrap with Trek - this adds request-scoped elevation and redaction
	trekHandler := trek.WrapDefaultHandler(jsonHandler)
	slog.SetDefault(slog.New(trekHandler))

	// Set up routes
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/login", handleLogin)
	mux.HandleFunc("GET /api/profile", handleProfile)
	mux.HandleFunc("POST /api/payment", handlePayment)

	wrapped := trek.HTTPMiddleware(mux)

	addr := ":8082"
	slog.Info("starting server", "addr", addr)
	if err := http.ListenAndServe(addr, wrapped); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

// customRedactor creates a redactor that handles common sensitive fields
// plus application-specific ones.
func customRedactor() trek.RedactAttrFunc {
	sensitiveKeys := map[string]bool{
		"password":    true,
		"secret":      true,
		"token":       true,
		"api_key":     true,
		"apikey":      true,
		"credit_card": true,
		"ssn":         true,
		"auth":        true,
		"cookie":      true,
	}

	return func(key string, val any) (any, bool) {
		lowerKey := strings.ToLower(key)

		// Check exact matches
		if sensitiveKeys[lowerKey] {
			return "[REDACTED]", false
		}

		// Check if key contains sensitive words
		for sensitive := range sensitiveKeys {
			if strings.Contains(lowerKey, sensitive) {
				return "[REDACTED]", false
			}
		}

		return val, false
	}
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// These fields will be redacted in logs
	username := r.FormValue("username")
	password := r.FormValue("password")

	// Debug logs with sensitive data - password will be redacted
	slog.DebugContext(ctx, "login attempt",
		"username", username,
		"password", password, // Will appear as [REDACTED]
	)

	slog.DebugContext(ctx, "validating credentials")
	slog.DebugContext(ctx, "checking rate limits")

	// Simulate login
	slog.InfoContext(ctx, "login successful", "username", username)

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"status": "ok", "token": "tok_example"}`)
}

func handleProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check if debug logging is active for this request
	if trek.IsDebug(ctx) {
		slog.DebugContext(ctx, "debug mode active, including extra diagnostics")
	}

	// Get the effective log level
	level := trek.EffectiveLevel(ctx)
	slog.DebugContext(ctx, "fetching profile", "effective_level", level)

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"id": "user_123", "name": "Example User"}`)
}

func handlePayment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Payment processing with sensitive data
	slog.DebugContext(ctx, "processing payment",
		"amount", "99.99",
		"credit_card", "4111-1111-1111-1111", // Will be redacted
		"cvv", "123", // Will be redacted (contains sensitive word)
	)

	slog.DebugContext(ctx, "validating card")
	slog.DebugContext(ctx, "charging payment processor")
	slog.InfoContext(ctx, "payment processed", "transaction_id", "txn_abc123")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	fmt.Fprint(w, `{"transaction_id": "txn_abc123", "status": "completed"}`)
}
