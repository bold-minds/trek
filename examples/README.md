# Trek Go SDK Examples

This directory contains example applications demonstrating Trek SDK integration patterns.

## Prerequisites

Before running the examples, you need:

1. A running Trek control plane server
2. Environment variables configured:

```bash
export TREK_ORG_ID=your-org-id
export TREK_ENV=dev
export TREK_API_ENDPOINT=http://localhost:8080
export TREK_API_TOKEN=your-api-token
```

## Examples

### [basic/](basic/)

**Minimal HTTP server integration**

Shows the simplest way to integrate Trek:
- Initialize with `trek.Init()`
- Wrap slog handler with `trek.WrapHandler()`
- Apply middleware with `trek.HTTPMiddleware()`

```bash
cd basic
go run main.go
# Server runs on :8081
```

Test it:
```bash
curl localhost:8081/
curl -H "X-User-ID: user123" localhost:8081/api/users/42
```

### [slog-handler/](slog-handler/)

**slog.Handler integration with redaction**

Demonstrates:
- Custom slog handler options
- Built-in sensitive data redaction
- Custom redaction functions
- Checking if debug mode is active with `trek.IsDebug()`

```bash
cd slog-handler
go run main.go
# Server runs on :8082
```

Test it:
```bash
# Login with sensitive data (password will be redacted in logs)
curl -X POST -d "username=alice&password=secret123" localhost:8082/api/login

# Payment with credit card (will be redacted)
curl -X POST localhost:8082/api/payment
```

### [custom-extractor/](custom-extractor/)

**Custom context extraction**

Shows how to:
- Map Trek fields to your application's header conventions
- Add custom fields for selector matching
- Support non-standard header names

```bash
cd custom-extractor
go run main.go
# Server runs on :8083
```

Test it:
```bash
# Using custom headers
curl -H "X-Auth-User-ID: user123" \
     -H "X-Correlation-ID: req-abc" \
     -H "X-Client-Type: mobile" \
     localhost:8083/api/data
```

## Creating Debug Sessions

To see debug logs, create a session targeting your requests using the Trek CLI or API:

```bash
# Target a specific user
trek session create --selector user:user123 --level debug --ttl 1h

# Target a route
trek session create --selector "route:/api/users/*" --level debug --ttl 30m

# Target by custom field (if configured)
trek session create --selector "custom.client_type:mobile" --level debug --ttl 15m
```

## Running Without Trek Server

If Trek server is unavailable, the SDK operates in "disabled mode":
- No errors are thrown
- Middleware passes requests through unchanged
- Debug logs remain hidden (default slog level applies)
- Application continues normally

This safe-by-default behavior means you can deploy Trek-enabled applications without requiring the control plane to be available.
