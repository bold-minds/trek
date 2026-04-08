// Example: custom-extractor
//
// This example demonstrates how to configure a custom context extractor
// to match your application's header conventions and add custom fields.
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
	cfg := trek.DefaultConfig()
	cfg.ServiceName = "custom-extractor-example"

	if err := trek.Init(cfg); err != nil {
		slog.Error("trek initialization failed", "error", err)
		os.Exit(1)
	}
	defer trek.Shutdown()

	// Configure a custom extractor that matches your application's conventions
	customExtractor := trek.ContextExtractor{
		// Map Trek fields to your application's headers
		UserIDHeader:    "X-Auth-User-ID",    // Your auth system's user ID header
		RequestIDHeader: "X-Correlation-ID",  // Distributed tracing correlation ID
		TenantIDHeader:  "X-Organization-ID", // Multi-tenant org identifier

		// Add custom fields for additional matching
		CustomHeaders: map[string]string{
			"feature_flag": "X-Feature-Flag", // Match requests with specific feature flags
			"client_type":  "X-Client-Type",  // Match by client type (mobile, web, api)
			"api_version":  "X-API-Version",  // Match by API version
		},
	}

	// Set the custom extractor globally
	trek.SetExtractor(customExtractor)

	// Wrap slog handler
	handler := trek.WrapHandler(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(slog.New(handler))

	// Set up routes
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/data", handleData)
	mux.HandleFunc("GET /api/v2/data", handleDataV2)

	wrapped := trek.HTTPMiddleware(mux)

	addr := ":8083"
	slog.Info("starting server", "addr", addr,
		"user_header", customExtractor.UserIDHeader,
		"request_header", customExtractor.RequestIDHeader,
		"tenant_header", customExtractor.TenantIDHeader,
	)

	fmt.Println("\nCustom Extractor Example")
	fmt.Println("========================")
	fmt.Println("This example uses custom header mappings:")
	fmt.Printf("  User ID:    %s\n", customExtractor.UserIDHeader)
	fmt.Printf("  Request ID: %s\n", customExtractor.RequestIDHeader)
	fmt.Printf("  Tenant ID:  %s\n", customExtractor.TenantIDHeader)
	fmt.Println("\nCustom fields:")
	for k, v := range customExtractor.CustomHeaders {
		fmt.Printf("  %s: %s\n", k, v)
	}
	fmt.Println("\nTry:")
	fmt.Println(`  curl -H "X-Auth-User-ID: user123" -H "X-Client-Type: mobile" localhost:8083/api/data`)
	fmt.Println()

	if err := http.ListenAndServe(addr, wrapped); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func handleData(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Log request details - these debug logs only appear when session matches
	slog.DebugContext(ctx, "processing data request",
		"path", r.URL.Path,
		"user_id_header", r.Header.Get("X-Auth-User-ID"),
		"correlation_id", r.Header.Get("X-Correlation-ID"),
		"client_type", r.Header.Get("X-Client-Type"),
	)

	slog.DebugContext(ctx, "loading data from database")
	slog.DebugContext(ctx, "applying transformations")

	slog.InfoContext(ctx, "data retrieved")

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"version": "v1", "data": [1, 2, 3]}`)
}

func handleDataV2(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// This endpoint can be targeted by route:* selectors or api_version custom field
	slog.DebugContext(ctx, "processing v2 data request",
		"api_version", r.Header.Get("X-API-Version"),
	)

	slog.DebugContext(ctx, "using v2 data pipeline")
	slog.DebugContext(ctx, "applying v2 transformations")

	slog.InfoContext(ctx, "v2 data retrieved")

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"version": "v2", "data": {"items": [1, 2, 3], "meta": {}}}`)
}
