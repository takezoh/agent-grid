---
change: change-20260727-single-user-server-auth
role: implementation
---

<!-- lifecycle is owned by change.md -->

# Implementation

## Fixed decisions

- Persisted client wire format is `schema_version: 2` with nested `auth` discriminated union. There is no compatibility reader or automatic migration.
- `clients/desktop-config` owns the validated auth value and its network mapping. Windows Shell and Workspace are typed consumers of that single contract.
- Server bearer persistence remains the existing server-owned `-token-file`; client `token_path` is deleted. Server startup requires explicit `-token`, `-token-file`, or `-no-auth`, so random-on-restart identity is impossible.
- Listener risk classification uses the successful `net.Listener.Addr()`, not the requested bind string. Loopback is safe for this warning; non-loopback and unspecified addresses warn.
- The existing REST bearer and single-use WebSocket-ticket protocol is unchanged.

## Implementation units

1. **Schema and migration boundary (implemented):** schema/README use version 2 and both clients reject v1 without mutation.
2. **Native client consumers (implemented):** C# and TypeScript consume the validated direct value; settings, health, preload/composition, defaults, scripts, and e2e writers use the same contract, with masked presentation.
3. **Server lifecycle and warning (implemented):** the escape-hatch flag/guard and random fallback are removed, `-token-file` is strict, and the bound listener selects the stable WARN.
4. **Operations and authority (implemented):** systemd installation provisions the initial token once, operating guidance describes coordinated replacement/rollback, and the former LAN escape-hatch behavior is superseded.

## Test seams

- T0: JSON fixtures, auth mapping, listener-address classifier, warning formatter, token redaction/masking.
- T1: native client config/health/settings/component tests and Go gateway scenarios.
- T2: persisted schema consumer contract shared across both clients; server startup/restart contract.
- T3: Windows Shell and web real-stack checks only where native/runtime wiring cannot be proved below.

## Failure and rollback

Internal invalid variants fail fast. External filesystem or network failures surface as typed unavailable/startup errors. Token change is a controlled operational transaction: retain old files until all health checks pass, and restore both sides plus restart on failure. No component independently rotates or retries with no auth.
