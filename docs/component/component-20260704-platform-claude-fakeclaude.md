---
id: component-20260704-platform-claude-fakeclaude
kind: component
title: fakeclaude — Claude CLI wire fake
status: active
created: '2026-07-04'
tags:
- fake
- claude
- stream-json
- hook
owners: []
provides: []
source_paths:
- src/platform/lib/claude/fakeclaude
relations:
- {type: implements, target: adr-20260704-cli-fake-validated-by-real-cli-e2e}
summary: 'Reusable in-memory fake for the two wire surfaces the Claude CLI exposes:
  the -p --output-format stream-json output stream, and the JSON payload Claude writes
  to hook stdin. Made public as part of the decision in'
---

## Overview

Reusable in-memory fake for the two wire surfaces the Claude CLI exposes:
the `-p --output-format stream-json` output stream, and the JSON payload
Claude writes to hook stdin. Made public as part of the decision in
[adr-20260704-cli-fake-validated-by-real-cli-e2e](../adr/adr-20260704-cli-fake-validated-by-real-cli-e2e.md).

## Public API

### Launcher — stream-json driver

```go
type LauncherFunc func(
    ctx context.Context,
    cwd, resumeSessionID, appendSystemPrompt, prompt string,
    extraEnv []string,
) (io.ReadCloser, func() error, error)

func NewLauncher(sequences ...[]string) (LauncherFunc, *CallLog)
func NewProgrammableLauncher(fn func(LaunchArgs) LaunchResponse) (LauncherFunc, *CallLog)

type CallLog struct { /* ... */ }
func (l *CallLog) Calls() []LaunchCall
func (l *CallLog) Last() LaunchCall
func (l *CallLog) Len() int
```

`LauncherFunc` is byte-for-byte identical to the private `claudeLauncher` type
declared in `cmd/claude-app-server/launch.go`; a `LauncherFunc` value is
directly assignable to that private type.

`NewLauncher` returns each provided sequence in turn; the last sequence sticks.
This covers every shim scenario historically served by the private
`fakeLauncherSequence`.

`NewProgrammableLauncher` computes the response from the actual call args —
used when a test needs to inspect `extraEnv` (e.g. TOOL_BRIDGE_SOCKET) or block
until context cancellation.

### Line builders — stream-json event fixtures

Six top-level constants match what the shim historically hard-coded:

- `LineSystemInit` (`{"type":"system","subtype":"init","session_id":"claude-sess-1"}`)
- `LineAssistant`
- `LineToolUse`, `LineToolResult`
- `LineResultOK`, `LineResultFail`

Plus parameterised builders:

- `SystemInit(sessionID string) string`
- `AssistantText(text string) string`
- `ToolUse(id, name string, input any) string`
- `ToolResult(toolUseID, content string, isError bool) string`
- `ResultOK(text string, usage streamjson.Usage) string`
- `ResultFail(errText string, usage streamjson.Usage) string`

Every builder's output round-trips through
`platform/lib/claude/streamjson.Parse` back to the expected typed `Event`
(pinned by `TestLines_RoundTrip`).

### HookPayload — hook stdin JSON

```go
type HookPayload struct {
    SessionID        string
    HookEventName    string
    Prompt           string
    TranscriptPath   string
    NotificationType string
    ToolName         string
    ToolInput        map[string]any
    Source           string
    ToolUseID        string
    PermissionMode   string
    Error            string
    IsInterrupt      bool
}
func Marshal(p HookPayload) []byte
```

The 15 hook event names are fixed by `agenthook.Claude.Events`. Tests build
`HookPayload{HookEventName: "SessionStart", ...}` literals directly — there
are no per-event constructors because every event's required field set is
different and driven by the test scenario.

```


Fields mirror `client/driver/claude_event.go:hookPayload`. The duplication is
deliberate — the driver's private struct cannot be re-exported without
crossing the platform → client depguard rule.

## Coverage matrix

| Wire | Real Claude emits | fakeclaude covers | e2e pin |
|---|---|---|---|
| stream-json `system.init` | ✓ | `LineSystemInit`, `SystemInit(id)` | `TestE2E_StreamJSONLexicon`, `TestE2E_FakeVsRealShape` |
| stream-json `assistant.text` | ✓ | `LineAssistant`, `AssistantText(s)` | `TestE2E_FakeVsRealShape` |
| stream-json `assistant.tool_use` | ✓ | `LineToolUse`, `ToolUse(id,name,input)` | `TestE2E_ToolUseLexicon` |
| stream-json `user.tool_result` | ✓ | `LineToolResult`, `ToolResult(...)` | `TestE2E_ToolUseLexicon` |
| stream-json `result` (success / error) | ✓ | `LineResultOK`, `LineResultFail`, `ResultOK/Fail(...)` | `TestE2E_StreamJSONLexicon` |
| hook `SessionStart` / `UserPromptSubmit` / `Stop` / `SessionEnd` | ✓ | `HookPayload{HookEventName: "…", …}` literal | `TestE2E_HookPayloadSchema` |
| hook `PreToolUse` / `PostToolUse` | ✓ | `HookPayload` literal with `ToolUseID`/`ToolName`/`ToolInput` | `TestE2E_HookPayloadKeySubset` |
| hook 15 events (full `agenthook.Claude.Events`) | ✓ | `HookPayload` struct fields | `TestE2E_HookPayloadKeySubset` |

The 15-event breadth is checked at the schema level rather than one test per
event — Claude does not deterministically fire every event on a short prompt.

## Consumer sites

- `src/cmd/claude-app-server/shim_test.go` — the `fakeLauncherSequence`
  helper is a thin wrapper delegating to `fakeclaude.NewLauncher`; the six
  `line*` const are aliases of `fakeclaude.Line*`
- `src/cmd/claude-app-server/conformance_test.go`
- `src/cmd/claude-app-server/toolbridge_test.go`
- `src/cmd/claude-app-server/main_test.go`
- `src/client/lib/agenthook/hook_e2e_test.go` (build tag `e2e`)

Do not clone fixtures into new tests — import them.

## Import rules

- `fakeclaude` imports **only** `platform/lib/claude/streamjson` and
  `platform/lib/claude/cli`. `client/*` and `orchestrator/*` are forbidden by
  depguard `platform-no-client-or-orchestrator`.
- Consumers may live in any layer.
- The e2e tests under this package are `//go:build e2e`; skipped when
  `AG_E2E_CLAUDE_BIN` is unset. Hook e2e lives under
  `client/lib/agenthook/` because it imports `agenthook`.

## Recording refresh

When the real Claude CLI wire contract changes, refresh the committed
recordings before updating `lines.go`:

```sh
cd src
AG_E2E_CLAUDE_BIN=claude go test -tags e2e ./platform/lib/claude/fakeclaude -run 'Recorded.*Fixture' -record
go test ./platform/lib/claude/fakeclaude
```

The `-record` run rewrites `testdata/recordings/*.jsonl` with normalized
values (`session_id`, paths, timestamps, tokens). The non-e2e package tests
then compare the committed recordings against the builder contracts in
`lines.go`.

## Parts
