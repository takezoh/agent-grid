# Compatibility policy

**Change**: change-20260727-client-server-protocol-version
**Decision**: `adr-20260727-exact-match-client-server-protocol`
**Version SSOT**: `protocol/protocol-version.json`

## Exact-match admission

Every client/server protocol boundary admits exactly one state: the client's
compiled protocol version is byte-for-byte equal to the server version.
Different, missing, malformed, or duplicate versions are
`incompatible_protocol`; operational REST handlers and WebSocket application
frames are not entered.

The server list preserves the server identity, expected client version, and
observed server version (or an explicit missing/invalid marker). A mismatching
server does not disable matching peers.

## Permanent development rule

Cross-version success is forbidden. Do not implement:

- version ranges or SemVer compatibility;
- capability-based degradation across versions;
- legacy, dual, tolerant, or fallback decoders;
- downgrade, retry-as-fallback, or user overrides;
- rolling mixed-version availability.

Every public wire change is a breaking flag-day change: update
`protocol/protocol-version.json`, the server, every shipped client and SDK,
simulators/fakes, fixtures, and tests atomically. Capability declarations are
informational within an already exact-matching version and never authorize a
compatibility path.

## Enforcement

CI must fail when a shipped projection differs from the manifest, a protected
route bypasses exact-version admission, a client performs operational work
after mismatch, or a protocol decoder accepts an alternate version shape.
