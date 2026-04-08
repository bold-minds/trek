# Changelog

All notable changes to the Trek SDK Specification will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.0] - 2025-01-01

### Added
- Initial specification release
- Core evaluation semantics (`semantics.md`)
  - Request context structure (user_id, tenant_id, request_id, route)
  - Session structure with selectors, levels, caps, and TTL
  - Decision output format with reason codes
  - Evaluation rules: filtering, matching, tie-breaking
  - Safe failure behavior
  - Caps enforcement rules
  - Clock skew handling (30-second grace period)
- Conformance test fixtures
  - Basic selector matching (user, tenant, request, route)
  - Session expiration handling
  - Priority and tie-breaking
  - Caps enforcement
  - Edge cases (no sessions, no match, expired)
- Reason codes specification
  - `matched` - Session matched request
  - `no_sessions` - No active sessions
  - `no_match` - Sessions exist but none match
  - `expired` - All matching sessions expired
  - `service_mismatch` - Service scope doesn't match

### Selector Types
- `user:<id>` - Match by user ID
- `tenant:<id>` - Match by tenant ID
- `request:<id>` - Match by request ID
- `route:<path>` - Match by URL path pattern
