# Contributing to Trek

## Module structure

This is a Go workspace with four modules:

| Directory | Module | Purpose |
|-----------|--------|---------|
| `.` (root) | `github.com/bold-minds/trek` | Go SDK |
| `server/` | `github.com/bold-minds/trek/server` | Control plane |
| `cli/` | `github.com/bold-minds/trek/cli` | CLI tool |
| `spec/` | — | Conformance fixtures and semantics doc |

## Building

```bash
# Sync the workspace
go work sync

# Build the SDK (root)
go build ./...

# Build the server
cd server && go build ./...

# Build the CLI
cd cli && go build ./...
```

## Testing

```bash
# SDK tests
go test -race ./...

# Server tests
cd server && go test ./internal/...

# CLI tests
cd cli && go test ./...
```

## SDK conformance

The SDK must pass all conformance fixtures in `spec/fixtures/v1.json`. These fixtures define the behavioral contract for session evaluation — matching, tie-breaking, expiration, and service scope. See `spec/semantics.md` for the decision rules.

To verify SDK conformance locally, run the integration tests against the fixture file:

```bash
go test -race -run TestConformance ./...
```

## Spec changes

Changes to evaluation semantics affect all SDK implementations and require an RFC. See `spec/CONTRIBUTING.md` for the RFC process, fixture format, and versioning policy.

## Before opening a PR

Run the server validation script for server-side changes:

```bash
cd server && ./scripts/validate.sh
```

For SDK or CLI changes, ensure tests pass with race detection enabled and coverage does not regress.
