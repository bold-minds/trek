# Trek Evaluation Semantics

This document defines the decision rules that all Trek SDKs must implement. Behavior must match exactly across languages.

## Core Function

```
decide(now, service_name, request_context, sessions) -> decision
```

### Inputs

- **now**: Current timestamp (ISO 8601)
- **service_name**: Identity of the service running the SDK (string)
- **request_context**: Extracted from the incoming request
- **sessions**: List of active sessions from cache

### Output

- **decision**: Result of evaluation

## Types

### RequestContext

| Field | Type | Description |
|-------|------|-------------|
| `user_id` | string | User identifier (nullable) |
| `request_id` | string | Request/correlation ID (nullable) |
| `tenant_id` | string | Tenant/org identifier (nullable) |
| `route` | string | Request path/route (required) |
| `custom` | map[string]string | Custom fields extracted by host app |

### Session

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique session identifier |
| `selector` | Selector | Match conditions |
| `level` | string | Log level: `debug` or `trace` |
| `expires_at` | timestamp | Expiration time (ISO 8601) |
| `caps` | Caps | Rate limiting configuration |
| `labels` | map[string]string | Arbitrary metadata |
| `service_scope` | []string | Services this session applies to (null = all) |

### Selector

| Field | Type | Description |
|-------|------|-------------|
| `user_id` | string | Match user ID (nullable) |
| `request_id` | string | Match request ID (nullable) |
| `tenant_id` | string | Match tenant ID (nullable) |
| `route` | string | Match route (exact or prefix with `*`) (nullable) |
| `custom` | map[string]string | Match custom fields (nullable) |

### Caps

| Field | Type | Description |
|-------|------|-------------|
| `max_debug_events_per_request` | int | Max debug log events per request |
| `max_debug_events_per_session` | int | Max debug log events per session (best-effort) |

### Decision

| Field | Type | Description |
|-------|------|-------------|
| `matched` | bool | Whether a session matched |
| `session_id` | string | ID of matched session (null if no match) |
| `effective_level` | string | `info`, `debug`, or `trace` |
| `reason_code` | string | Why this decision was made |
| `labels` | map[string]string | Labels from matched session |

### Reason Codes

| Code | Meaning |
|------|---------|
| `MATCHED` | Session matched, debug enabled |
| `NO_MATCH` | No session matched |
| `EXPIRED` | Session(s) existed but all expired |

## Evaluation Rules

### 1. Filter Applicable Sessions

For each session, check:

1. **Not expired**: `session.expires_at > now`
2. **Service scope matches**: 
   - If `session.service_scope` is `null` → applies to all services
   - If `session.service_scope` is `[]` (empty array) → applies to no services (skip)
   - Otherwise → current `service_name` must be in the list

### 2. Match Selector

A session matches if **ALL** specified selector fields match the request context.

#### Field Matching Rules

| Field | Rule |
|-------|------|
| `user_id` | Exact string equality |
| `request_id` | Exact string equality |
| `tenant_id` | Exact string equality |
| `route` | Exact match OR prefix match if selector ends with `*` |
| `custom` | All specified custom keys must exist in request with exact value match |

#### Route Matching Examples

| Selector Route | Request Route | Match? |
|---------------|---------------|--------|
| `/api/orders` | `/api/orders` | ✅ |
| `/api/orders` | `/api/orders/123` | ❌ |
| `/api/orders*` | `/api/orders` | ✅ |
| `/api/orders*` | `/api/orders/123` | ✅ |
| `/api/orders*` | `/api/order` | ❌ |

#### Custom Field Matching

- Selector custom: `{"plan": "enterprise"}`
- Request custom: `{"plan": "enterprise", "region": "us-west"}`
- Result: ✅ Match (all selector keys present with matching values)

### 3. Tie-Breaking (Multiple Matches)

When multiple sessions match, select ONE using these rules in order:

1. **Highest verbosity**: `trace` > `debug` > `info`
2. **Most specific selector**: Count of non-null selector fields (custom counts as 1 regardless of keys)
3. **Earliest expiration**: Smaller `expires_at` wins
4. **Lexicographic session ID**: Smallest `id` string wins

### 4. Build Decision

If a session was selected:
```
matched: true
session_id: <selected session id>
effective_level: <session level>
reason_code: "MATCHED"
labels: <session labels>
```

If no session matched:
```
matched: false
session_id: null
effective_level: "info"
reason_code: "NO_MATCH" or "EXPIRED"
labels: {}
```

Use `EXPIRED` if at least one session would have matched but was expired. Otherwise use `NO_MATCH`.

## Safe Failure Behavior

If the evaluator encounters an error (malformed session, etc.):

- Skip the problematic session
- Continue evaluating remaining sessions
- If all sessions fail, return `NO_MATCH`
- **Never elevate logging due to an error**

## Caps Enforcement

Caps are **not part of the evaluator**. They are enforced by the logging wrapper after the decision.

- Per-request cap: Stop emitting debug logs after N events in one request
- Per-session cap: Best-effort local tracking per instance (v1)

When cap is reached:
- Stop emitting debug/trace logs for remainder of request
- Continue emitting info/warn/error normally
- Emit one `TREK_CAP_REACHED` marker event

## Clock Handling

- SDK should use server-provided `server_time` to compute clock offset
- If unavailable, use local system time
- Session is expired if `expires_at <= now` (no grace period in evaluator; grace is for cache staleness)
