# trek-spec

Conformance fixtures and semantics documentation for Trek SDKs.

## Purpose

This repo defines the **behavioral contract** all Trek SDKs must implement. Any SDK that passes the conformance fixtures is guaranteed to produce identical decisions.

## Structure

```
trek-spec/
├── fixtures/
│   └── v1.json          # Conformance test cases (20 cases)
├── semantics.md         # Decision rules, tie-breaking, matching
└── README.md
```

## Fixture Format

Each fixture is a test case with inputs and expected output:

```json
{
  "name": "match user_id elevates to debug",
  "now": "2026-01-01T00:00:00Z",
  "service_name": "api",
  "request_context": {
    "user_id": "u123",
    "request_id": "r1",
    "tenant_id": "t1",
    "route": "/api/orders",
    "custom": {}
  },
  "sessions": [...],
  "expected": {
    "matched": true,
    "session_id": "s1",
    "effective_level": "debug",
    "reason_code": "MATCHED",
    "labels": {}
  }
}
```

## Test Categories

| Category | Description |
|----------|-------------|
| Basic matching | user_id, tenant_id, request_id matching |
| Route matching | Exact and prefix (`*`) route patterns |
| Custom fields | Custom key-value matching |
| Expiration | Expired session handling |
| Tie-breaking | Level > specificity > expiry > id |
| Service scope | Service-specific session filtering |

## Reason Codes

| Code | Meaning |
|------|---------|
| `MATCHED` | Session matched, debug enabled |
| `NO_MATCH` | No session matched the request |
| `EXPIRED` | Session(s) existed but all expired |

## Usage

SDKs fetch fixtures via HTTP:

```
https://raw.githubusercontent.com/bold-minds/trek-spec/main/fixtures/v1.json
```

## Related Repos

| Repo | Purpose |
|------|---------|
| [trek-go](https://github.com/bold-minds/trek-go) | Go SDK |
| [trek](https://github.com/bold-minds/trek) | Control plane server |
| [trek-cli](https://github.com/bold-minds/trek-cli) | CLI (`trek` command) |