# Trek

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://golang.org/doc/go1.25)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Targeted debug logging for production — elevate log verbosity for specific users, requests, or tenants without changing global log levels. Sessions are time-bounded with TTL enforcement, per-request caps, and a full audit trail.

## How it works

The control plane (`server/`) manages debug sessions and propagates them to all service instances. The Go SDK evaluates session matching locally using cached session data, so there is zero hot-path latency — no network call happens per request. Operators create and manage sessions using the CLI.

```
operator
   |
   v
 trek CLI ──► Trek server (control plane)
                    |
                    | polling (background)
                    v
              Go SDK (in-process)
                    |
                    | local decision
                    v
              slog handler (elevates level per-request)
```

## Quick start — SDK

```bash
go get github.com/bold-minds/trek
```

```go
package main

import (
    "log/slog"
    "net/http"
    "os"

    "github.com/bold-minds/trek"
)

func main() {
    // Initialize Trek
    err := trek.Init(trek.Config{
        ServiceName: "api",
        OrgID:       os.Getenv("TREK_ORG_ID"),
        Env:         os.Getenv("TREK_ENV"),
        APIEndpoint: os.Getenv("TREK_API_ENDPOINT"),
        APIToken:    os.Getenv("TREK_API_TOKEN"),
    })
    if err != nil {
        slog.Warn("trek init failed, continuing without trek", "error", err)
    }
    defer trek.Shutdown()

    // Wrap your slog handler
    handler := trek.WrapHandler(slog.NewJSONHandler(os.Stdout, nil))
    slog.SetDefault(slog.New(handler))

    // Use Trek middleware
    mux := http.NewServeMux()
    mux.HandleFunc("/api/orders", handleOrders)

    http.ListenAndServe(":8080", trek.HTTPMiddleware(mux))
}

func handleOrders(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // Debug logs appear only when a matching session is active
    slog.DebugContext(ctx, "processing order", "step", "validation")

    // ... handler logic
}
```

If Trek is unavailable, logging behavior is unchanged — the SDK is safe by default.

## Quick start — Server

```bash
# Start Postgres, Redis, and Trek server
./server/scripts/docker-up.sh

# Run migrations and seed demo data
./server/scripts/migrate.sh
./server/scripts/seed.sh

# Verify it's running
curl http://localhost:8080/health
```

## Quick start — CLI

```bash
go install github.com/bold-minds/trek/cli@latest
```

```bash
# Authenticate
trek auth login --clerk-domain your-domain.clerk.accounts.dev --client-id <client-id>

# Start a debug session for a specific user
trek start --user u123 --ttl 15m --level debug --reason "investigating order issue"

# Debug a route
trek start --route "/api/orders*" --ttl 10m --level trace

# List active sessions
trek list

# Stop a session
trek stop --session s_abc123
```

| Command | Description |
|---------|-------------|
| `trek auth login` | Authenticate via Clerk |
| `trek auth whoami` | Show auth status |
| `trek start` | Create a debug session |
| `trek stop` | Revoke a session |
| `trek list` | List sessions |
| `trek inspect` | Test request matching locally |
| `trek tokens create` | Create service token |
| `trek tokens list` | List tokens |
| `trek tokens revoke` | Revoke a token |

## Repository structure

| Directory | Description |
|-----------|-------------|
| `.` (root) | Go SDK — integrate Trek into your services |
| `server/` | Control plane server — manages debug sessions |
| `cli/` | CLI tool — create and manage sessions from terminal |
| `spec/` | Conformance specification and test fixtures |
| `examples/` | Example integrations and demo scripts |

## Conformance spec

`spec/` defines the behavioral contract all Trek SDKs must implement. Any SDK that passes the conformance fixtures is guaranteed to produce identical session-matching decisions across implementations. See `spec/semantics.md` for the decision rules.

## License

MIT — see [LICENSE](LICENSE).
