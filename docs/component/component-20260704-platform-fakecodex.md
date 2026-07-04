---
id: component-20260704-platform-fakecodex
kind: component
title: fakecodex — Codex app-server wire fake (stdio)
status: active
created: '2026-07-04'
tags:
- fake
- codex
- app-server
- stdio
owners: []
provides: []
source_paths:
- src/platform/agent/fakecodex
relations:
- {type: implements, target: adr-20260704-cli-fake-validated-by-real-cli-e2e}
- {type: references, target: adr-20260624-0002-optin-appserver-e2e-validates-fakes}
---

## Overview

Reusable in-memory fake of the Codex app-server (stdio JSON-RPC v2).
Made public as part of the decision in
[adr-20260704-cli-fake-validated-by-real-cli-e2e](../adr/adr-20260704-cli-fake-validated-by-real-cli-e2e.md).

Complements `client/runtime/subsystem/stream/fake` (WebSocket transport,
validated by [ADR 0002](../adr/adr-20260624-0002-optin-appserver-e2e-validates-fakes.md)):
this package models the same protocol over the stdio transport
`orchestrator/agent` uses directly.

## Public API

```go
type Server struct { /* ... */ }
func New(cfg Config) *Server
func (s *Server) Serve(ctx context.Context, transport codexclient.Transport) error
func (s *Server) Attach(ctx context.Context, r io.Reader, w io.WriteCloser) (stop func())

// Observation.
func (s *Server) LastThreadParams() json.RawMessage
func (s *Server) LastTurnParams() json.RawMessage
func (s *Server) LastResumeParams() json.RawMessage
func (s *Server) LastCWD() string
func (s *Server) LastMessage() string
func (s *Server) ToolReplies() []json.RawMessage
```

Config:

```go
type Config struct {
    ThreadID   string          // default DefaultThreadID
    TurnID     string          // default DefaultTurnID
    FailInit   bool            // reject initialize with a JSON-RPC error
    HangTurn   bool            // start the session but never resolve the turn
    Handler    TurnHandler     // custom per-turn logic; nil ⇒ DefaultTurnHandler
    TokenUsage TokenUsageSpec
}

type TurnRequest struct { ThreadID, CWD, Message string; Raw json.RawMessage }
type TurnEmitter interface {
    AgentDelta(delta string) error
    ItemStarted(item map[string]any) error
    ItemCompleted(item map[string]any) error
    ToolCallRequest(tool string, arguments any, callID string) (json.RawMessage, error)
}
type TurnHandler func(ctx context.Context, req TurnRequest, e TurnEmitter) (text string, err error)
```

Presets in `presets.go`:

- `DefaultTurnHandler` — immediate `turn/completed` with text `"done"`.
- `FailingTurnHandler(msg)` — emit `error` instead of `turn/completed`.
- `TextTurnHandler(delta, completedText)` — one `item/agentMessage/delta` then complete.
- `ToolCallHandler(toolName, arguments, completedText)` — issue one `item/tool/call`
  request; capture the reply into `ToolReplies()`.
- `ItemPairHandler(started, completed, completedText)` — emit `item/started` +
  `item/completed`, mirrors the shim's tool-use / tool-result pair.

## Method coverage

Every notification method the fake can emit corresponds to a real codex
app-server method. Union set:

| Method | Direction | Fake emits | Notes |
|---|---|---|---|
| `initialize` | client → fake | reply `{}` or JSON-RPC error | `FailInit` gates the error |
| `initialized` | client → fake | (no-op) | client notif |
| `thread/start` | client → fake | reply `{thread:{id:...}}` | `LastThreadParams()` records params |
| `thread/resume` | client → fake | reply `{thread:{id:...}}` | `LastResumeParams()` records params |
| `turn/start` | client → fake | (drives the sequence below) | `LastTurnParams()` / `LastCWD()` / `LastMessage()` record |
| `thread/started` | fake → client | ✓ | on every turn/start |
| `turn/started` | fake → client | ✓ | on every turn/start |
| `item/agentMessage/delta` | fake → client | via `TurnEmitter.AgentDelta` |  |
| `item/started` | fake → client | via `TurnEmitter.ItemStarted` |  |
| `item/completed` | fake → client | via `TurnEmitter.ItemCompleted` |  |
| `item/tool/call` | fake → client | via `TurnEmitter.ToolCallRequest` | request/response round-trip |
| `thread/tokenUsage/updated` | fake → client | ✓ | on turn completion; `TokenUsageSpec` controls last/total/modelContextWindow |
| `turn/completed` | fake → client | ✓ | on turn success |
| `error` | fake → client | on turn failure |  |

## Role split with `stream/fake`

| Concern | `client/runtime/subsystem/stream/fake` | `platform/agent/fakecodex` |
|---|---|---|
| Transport | WebSocket-over-UDS | stdio |
| ADR | [0002](../adr/adr-20260624-0002-optin-appserver-e2e-validates-fakes.md) | [this](../adr/adr-20260704-cli-fake-validated-by-real-cli-e2e.md) |
| Primary consumer | `client/runtime/subsystem/stream` routing tests | `orchestrator/agent` runner tests |
| Fidelity backstop | `routing_e2e_test.go` (WS backend) | `codex_appserver_e2e_test.go` (stdio backend) |
| Multi-thread | broadcast, rollout files, active/idle status | single-thread, per-turn |

The two fakes intentionally cover different scopes. Do not merge them: routing
fidelity needs multi-frame broadcast, while orchestrator's stdio needs the
whole turn lifecycle over one pipe.

## Consumer sites

- `src/orchestrator/agent/runner_test.go` — the `fakeServer` struct is a thin
  adapter wrapping `fakecodex.Server`.
- `src/orchestrator/agent/handler_test.go`
- `src/orchestrator/agent/runner_events_test.go`
- `src/orchestrator/agent/runner_loop_test.go`
- `src/orchestrator/agent/worker_test.go`
- `src/platform/agent/fakecodex/codex_appserver_e2e_test.go` (build tag `e2e`)

## Import rules

- `fakecodex` imports **only** `platform/agent/codexclient` and
  `platform/agent/codexschema`. `client/*` / `orchestrator/*` are forbidden by
  depguard.
- Consumers live in any layer.
- The e2e test is `//go:build e2e`; skipped when `REACTOR_E2E_CODEX_BIN` is
  unset.

## Parts
