---
id: adr-20260624-0013-attacher-interface-and-protofake
kind: adr
title: ADR 0013 — Keep the `Attacher` interface; satisfy it with both `DaemonAdapter`
  and `protofake`
status: accepted
created: '2026-06-24'
updated: '2026-07-04'
tags:
- adr
- legacy-import
owners: []
relations:
- {type: referencedBy, target: adr-20260624-0066-terminal-scrollback-via-vt-buffer}
source_paths:
- src/server/web/gateway.go
- src/client/proto/
- src/client/proto/protofake/
- src/server/web/
decision_makers:
- unknown
summary: 'server/web/gateway.go currently exposes Attacher as the seam between the
  WebSocket handler and the session backend. Three options were considered for A1-α:'
---

<!-- migrated_from: docs/adr/0013-attacher-interface-and-protofake.md -->

# ADR 0013 — Keep the `Attacher` interface; satisfy it with both `DaemonAdapter` and `protofake`

Status: Accepted

## Context

`server/web/gateway.go` currently exposes `Attacher` as the seam between the
WebSocket handler and the session backend. Three options were considered for
A1-α:

- Remove `Attacher` and let `AttachWS` consume `proto.Client` directly.
- Keep `Attacher`, implement it with a `DaemonAdapter`, and provide a
  `protofake` test double that satisfies the same interface.
- Move `Attacher` into `gateway`'s caller (mux).

Without the interface, `gateway_test.go` would have to mock `proto.Client`
internals — couples tests to wire-encoding details that are tested separately
in `client/proto`.

## Decision

Keep `Attacher` as the seam. Provide:

- `DaemonAdapter` (production impl) — wraps `daemon_client` and translates
  WebSocket frames ↔ proto commands.
- `client/proto/protofake` (test impl) — `net.Pipe` + ndjson encoder,
  public API limited to `NewPair() (*ClientSide, *ServerSide)` and `Close()`.

Split `AttachWS` into three functions, each ≤80 lines:

- `readInbound(ctx, conn, adapter)` — reads browser frames, translates to
  proto commands.
- `writeOutbound(ctx, conn, adapter, sessID)` — reads proto events from the
  adapter, encodes them as `controlMsg` / asciicast, writes to the WS.
- `subscribeLifecycle(ctx, adapter, sessID)` — sends the initial
  `CmdSurfaceSubscribe`, handles `RespOK` / `RespErr`, ensures the matching
  `CmdSurfaceUnsubscribe` runs on teardown.

`writeTypedClose(reason)` centralises typed close writes.

## Consequences

- `gateway_test.go` exercises subscribe → output → unsubscribe end-to-end via
  `protofake` without depending on `proto.Client` internals.
- File / function length constraints (500 / 80 lines) stay satisfied.
- The `Attacher` interface stays small; the only risk — interface drift
  between adapter and fake — is mitigated by limiting the fake to two public
  functions.

## Alternatives

- **Remove `Attacher`** — rejected; tests would mock proto internals.
- **Define `Attacher` in `gateway`'s caller** — rejected; complicates the
  import direction without functional benefit.

## Related requirements

- FR-003, FR-006, FR-007, FR-008
