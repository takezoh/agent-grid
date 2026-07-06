---
id: adr-20260704-cli-fake-validated-by-real-cli-e2e
kind: adr
title: Claude CLI and Codex stdio fakes are validated by opt-in real-CLI e2e
status: accepted
created: '2026-07-04'
decision_makers:
- unknown
tags:
- e2e
- fake
- fidelity
owners: []
relations:
- {type: references, target: adr-20260624-0002-optin-appserver-e2e-validates-fakes}
- {type: references, target: note-20260624-agent-testing}
- {type: implementedBy, target: component-20260704-platform-claude-fakeclaude}
- {type: implementedBy, target: component-20260704-platform-fakecodex}
- {type: referencedBy, target: adr-20260705-test-tier-taxonomy}
- {type: referencedBy, target: adr-20260705-fakedocker-path-injection}
- {type: referencedBy, target: note-20260624-agent-testing}
source_paths:
- src/platform/lib/claude/fakeclaude
- src/platform/agent/fakecodex
- src/client/lib/agenthook
updated: '2026-07-04'
---

# ADR — Claude CLI and Codex stdio fakes are validated by opt-in real-CLI e2e

Status: Proposed

## Context

[ADR 0002](adr-20260624-0002-optin-appserver-e2e-validates-fakes.md) established that
the in-process fake of the codex app-server (WebSocket transport) is validated by
an opt-in `//go:build e2e` test suite that runs the same isolation invariant
against a real app-server binary. That ADR's scope is one transport
(WebSocket-over-UDS) and one dimension (routing isolation).

Two other integration surfaces existed without the same backstop:

- **Claude CLI stream-json + hook**. `cmd/claude-app-server/shim_test.go` carried a
  private `fakeLauncherSequence` and six `line*` const string fixtures. Nothing
  guaranteed the fixtures matched the wire form real `claude -p --output-format
  stream-json` emits. The same file also implicitly asserted a hook payload
  schema (`client/driver/claude_event.go:hookPayload`) that was never crossed with
  what real Claude writes to a hook command's stdin.

- **Codex app-server stdio**. `orchestrator/agent/runner_test.go` carried a
  private `fakeServer` that replied to `initialize` / `thread/start` and drove the
  turn/started ▸ turn/completed sequence. It was never validated against a real
  `codex app-server` running over stdio — the transport the orchestrator actually
  uses.

A fake is only as good as its fidelity. If the fake drifts and only fake-based
tests exist, silent breakage in production is the outcome — exactly the class of
failure ADR 0002 was written to prevent.

## Decision

Extend ADR 0002's principle to both remaining surfaces:

1. **Publish the fakes as reusable packages** with a stable public API:
   - `platform/lib/claude/fakeclaude` — `Launcher` and `Line*` builders for
     stream-json; `HookPayload` and preset builders (SessionStart /
     UserPromptSubmit / PreToolUse / PostToolUse / Stop / SessionEnd) for hook
     JSON. Depends only on `platform/lib/claude/streamjson` and
     `platform/lib/claude/cli`.
   - `platform/agent/fakecodex` — `Server` with `Config` / `TurnHandler` /
     `TurnEmitter`; presets (`DefaultTurnHandler`, `FailingTurnHandler`,
     `TextTurnHandler`, `ToolCallHandler`, `ItemPairHandler`). Depends only on
     `platform/agent/codexclient` and `platform/agent/codexschema`. Neither fake
     imports `client/*` or `orchestrator/*` (depguard-enforced).

2. **Validate each fake with an opt-in real-CLI test**, mirroring ADR 0002:
   - `platform/lib/claude/fakeclaude/claude_cli_e2e_test.go` — real `claude`
     spawned via `cli.AppServerArgs`, stream-json parsed with the production
     parser. Asserts (a) argv is accepted, (b) `SystemInit` / `Result` events
     appear with populated fields, (c) `--resume` continues the same session,
     (d) each fake event type shows up in real output.
   - `client/lib/agenthook/hook_e2e_test.go` — `agenthook.Install` writes the
     claude settings.json, real `claude` is spawned; hook stdin payloads are
     captured and asserted to decode into `fakeclaude.HookPayload` with the
     required event names present. Lives in `client/` because it imports
     `agenthook`, which platform must not depend on.
   - `platform/agent/fakecodex/codex_appserver_e2e_test.go` — real `codex
     app-server` spawned via `codex.AppServerStdioArgs`. Asserts (a) initialize
     succeeds, (b) `thread/start` → `turn/start` yields the required event
     methods, (c) the fake's method set is a subset of the real one.

3. **Gate every e2e file with `//go:build e2e` and skip unless
   `AG_E2E_CLAUDE_BIN` / `AG_E2E_CODEX_BIN` is set** (routing
   `_e2e_test.go` pattern). CI runs `go vet -tags e2e ./...` only; execution is
   local / opt-in to keep API-key spend off the CI path.

4. **All unit tests import the published fakes**. `shim_test.go` /
   `runner_test.go` were rewritten to delegate to `fakeclaude` / `fakecodex`
   instead of carrying private clones. When a `FakeVsReal*` test fails, the
   correct response is to update the fake — not the assertion.

## Consequences

Positive:

- Fake fidelity is pinned at the wire level by explicit `FakeVsReal*` tests.
  Upstream breaking changes surface on the first opt-in run, before they reach
  the production shim / orchestrator.
- Downstream tests get a single public surface to program against. Adding a new
  scenario means composing `TurnHandler` presets, not writing another private
  mock.
- The Claude hook payload schema (`session_id`, `transcript_path`,
  `hook_event_name`, …) becomes explicit and testable, closing a schema-drift
  vector between real Claude and `client/driver/claude_event.go:hookPayload`.

Negative:

- Running the e2e suite locally requires (a) the real CLI, (b) an Anthropic /
  OpenAI account, (c) a wall of trust that the API remains behavior-stable while
  the test runs (LLM outputs are non-deterministic — assertions target event
  types and required fields, not prompt content). CI cost of the safeguard is
  zero; developer cost is the one-time login.
- Two new packages (`fakeclaude`, `fakecodex`) widen the platform surface. Kept
  in `platform/` deliberately: the wire form they model is shared by all layers,
  and depguard enforces that they never grow client/orchestrator dependencies.

## Alternatives considered

- **Extend ADR 0002 in place.** Rejected: ADR 0002 targets WebSocket routing
  isolation for a specific `bindServer` topology. Widening it to cover stream-json
  lexicon and hook payload schemas would obscure its original claim. A companion
  ADR keeps each decision auditable.
- **Move fakes to `internal/testutil`.** Rejected: the project has no shared
  testutil package by convention (see `note-20260624-agent-testing`). Each fake
  belongs next to the wire it models.
- **CI-run the e2e.** Rejected for cost and noise: the real CLI depends on an
  account, and prompt non-determinism would generate flaky red without adding
  correctness confidence beyond a nightly local run.


{% transition from="proposed" to="accepted" date="2026-07-04" %}
planned + implemented in this change set (fakeclaude + fakecodex + 3 e2e suites)
{% /transition %}
