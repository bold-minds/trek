# Contributing to Trek Spec

Thank you for your interest in contributing to the Trek SDK specification!

## Overview

Trek Spec defines the behavioral contract that all Trek SDK implementations must follow. Changes to this specification affect all SDK implementations, so we follow a careful review process.

## Types of Contributions

### 1. Clarifications

If the spec is ambiguous or unclear, please open an issue describing:
- The ambiguous section
- The confusion it causes
- Your suggested clarification

### 2. New Test Fixtures

Adding test fixtures helps ensure SDK consistency. To add fixtures:

1. Create a JSON file in `fixtures/` following the existing format
2. Include a clear `name` and `description`
3. Cover edge cases not already tested
4. Ensure the expected output is correct per `semantics.md`

### 3. Spec Changes (RFC Process)

For changes to evaluation semantics:

1. **Open an RFC Issue** with:
   - Problem statement
   - Proposed solution
   - Impact on existing SDKs
   - Migration path (if breaking)

2. **Discussion Period**: Minimum 1 week for feedback

3. **Implementation**: Once approved, update:
   - `semantics.md` with new rules
   - Fixtures to cover new behavior
   - Changelog with version bump

## Fixture Format

```json
{
  "name": "descriptive-test-name",
  "description": "What this test verifies",
  "now": "2024-01-15T12:00:00Z",
  "service_name": "api-gateway",
  "request_context": {
    "user_id": "user-123",
    "tenant_id": "",
    "request_id": "",
    "route": ""
  },
  "sessions": [...],
  "expected": {
    "matched": true,
    "session_id": "sess-abc",
    "level": "debug",
    "reason": "matched"
  }
}
```

## Versioning

Trek Spec follows semantic versioning:
- **MAJOR**: Breaking changes to evaluation semantics
- **MINOR**: New selector types, new fields (backward compatible)
- **PATCH**: Clarifications, new test fixtures

## Questions?

Open an issue for any questions about the spec or contribution process.
