---
change: change-20260727-single-user-server-auth
role: requirements
---

<!-- lifecycle is owned by change.md -->

# Requirements

## Functional requirements

- **FR-AUTH-001 (invariant):** Each enabled schema-version-2 server entry shall contain exactly one nested `auth` variant: `{"mode":"none"}` or `{"mode":"bearer","token":"<non-empty>"}`.
- **FR-AUTH-002:** When bearer is selected, both native clients shall send the configured token only in the REST `Authorization: Bearer` header, including ticket minting, and shall retain the existing single-use WebSocket ticket flow.
- **FR-AUTH-003:** When none is selected, clients shall omit Authorization. A bearer rejection or unavailable credential shall never fall back to none.
- **FR-AUTH-004:** Schema version 1, `token_path`, unknown modes, empty bearer tokens, token on none, and extra auth properties shall fail before connection with an actionable error identifying the server and showing version-2 examples. No automatic rewrite or token-file read is permitted.
- **FR-AUTH-005:** The server shall accept `-no-auth` for loopback, non-loopback, and unspecified binds. `-allow-non-loopback-no-auth` shall be unknown/removed.
- **FR-AUTH-006:** After bind succeeds, no-auth with a listener address that is non-loopback or unspecified shall emit a WARN containing a stable risk marker and stating that the unauthenticated REST/WebSocket surface is network reachable.
- **FR-AUTH-007:** Bearer server startup shall use the existing server-owned `-token-file`. A normal restart with the same file preserves identity; missing, empty, or unreadable input fails startup and never generates a replacement token.
- **FR-AUTH-008:** Explicit token change shall use prepare/validate clients → atomically replace server token → restart → reload clients → authenticated health confirmation. Any failed restart or health check requires rollback of server and prepared client values; completion is not declared until every target authenticates.
- **FR-AUTH-009:** Settings shall mask stored bearer tokens and require explicit replacement input; URLs, ordinary logs, status, and user-facing errors shall not contain token material.

## Non-functional requirements

- **NFR-AUTH-001:** JSON Schema, C#, and TypeScript shall accept and reject the same shared fixtures.
- **NFR-AUTH-002:** Network authentication failures remain observable as 401/unavailable without silent fallback.
- **NFR-AUTH-003:** The trusted-local-device assumption shall not weaken bearer enforcement across the network boundary.

## Acceptance

- A valid bearer entry survives client reload and a normal server restart with authenticated health.
- A valid none entry reaches a non-loopback listener and the startup WARN is observable.
- Every legacy or malformed configuration counterexample fails without reading a token path or modifying the file.
- A sentinel bearer value is absent from captured URLs, normal logs, status, settings display, and errors across both clients and helper writers.
