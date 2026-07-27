---
change: change-20260727-client-server-protocol-version
role: implementation
---

<!-- lifecycle is owned by change.md -->

# Implementation

## Contract owners and seams

### `contract-version-and-evolution`

`protocol/protocol-version.json` is the only token owner. Generated or verified Go, TypeScript, C#, Swift, and Kotlin projections plus OpenAPI metadata consume it. The pure validator accepts exactly one 1–64 byte `[A-Za-z0-9._-]+` value and compares exact bytes. Any public protocol change is breaking, bumps the token, and updates every consumer atomically; no compatibility or rollback path exists.

Seams: manifest/schema validator, generated-projection drift checker, pure token parser/comparator, and old/new cross-pair fixtures.

### `contract-server-admission`

The protected pipeline is `TokenAuth -> ProtocolVersionGate -> operational handler`; explicitly unauthenticated routes use `ProtocolVersionGate -> operational handler`. Invalid credentials retain 401/403 without version disclosure. Authenticated/no-auth missing, duplicate, malformed, or unequal values return typed 426. Ticket mint records the admitted token; `/ws` consumes only a matching single-use unexpired ticket before emitting frames.

Seams: injected `http.Handler` effect counter, injectable/static server-version provider, ticket-store unit boundary, and gateway scenario harness.

### `contract-client-continuity`

The protocol-policy component owns the cross-host recorded-scenario matrix. Web proxy, browser transport, Electron `fleetFetch`/IPC, and Windows `HttpClient` each consume the same immutable generated token. `incompatible_protocol` is terminal for a server and observation generation: operational calls and reconnect/backoff stay at zero until a later compatibility probe reports exact match.

Seams: reverse-proxy request capture, injected fetch/IPC, `HttpMessageHandler`, fake clock/retry counter, and host-neutral recorded fixtures.

### `contract-fleet-observability`

The shared fleet provider owns a closed per-server outcome union and generation fencing. Presentation is a separate consumer. It joins current config-label snapshots with latest-generation mismatch evidence by `serverId`, retains available client/server values or explicit missing/invalid markers, and never converts mismatch to unavailable. Healthy peers remain operable.

Seams: provider reducer/fixture, config-generation and probe-generation constructors, pure presenter, Web smoke, and Windows UIA presentation backstop.

### `contract-repository-enforcement`

A checked-in closed inventory names every public route, shipped host, decoder, retry decision site, SDK projection, fake/simulator, and fixture. Dependency/static rules permit version/capability decisions only at the generated exact-equality API and typed outcome chokepoints. Word scanning is advisory; decisive gates are inventory completeness, dependency ownership, recorded scenario coverage, projection drift, and mutation fixtures.

Seams: compatibility-profile CLI with fixture roots, simulator injection, fake-vs-real backstop, and mutation corpus.

## Implementation order

1. Add the manifest, bounded token parser/comparator, generated projections, and exact-match ADR/policy update.
2. Gate authenticated/no-auth REST and bind WebSocket tickets before adding client behavior.
3. Update Web, Workspace, and Windows carrier continuity and terminal mismatch behavior using the shared scenarios.
4. Add the closed fleet outcome and presentation projection with generation-fenced inputs.
5. Register all surfaces in conformance inventory, update SDK/simulator/fakes, and enable mutation and drift gates.

## Task-grade units

- **Define protocol token and flag-day evolution** — owns `protocol/`, projections, OpenAPI binding, old/new fixtures, and compatibility-policy replacement.
- **Gate server REST and WebSocket admission** — owns middleware order, typed 426, effect-free rejection, and version-bound tickets.
- **Propagate exact version through every client host** — owns Web, Workspace, Windows carrier injection/decoding and mismatch retry terminality.
- **Project fleet mismatch into server lists** — owns the typed per-server union, generation fencing, label/evidence join, and presentation.
- **Enforce complete protocol conformance** — owns route/client/decoder/retry inventory, SDK/simulator parity, mutation fixtures, and CI profile.

No behavior-changing design choice is delegated to implementation. Header spelling and decoder coding style may vary only behind the generated token and typed failure contracts and are not user-observable.

