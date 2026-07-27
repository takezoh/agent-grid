---
id: note-20260726-codex-0-145-0-terminal-status-patterns-f
kind: note
title: Codex 0.145.0 terminal status patterns from app-server events
status: published
created: '2026-07-26'
tags:
- codex
- app-server
- session-status
- terminal
owners: []
relations:
- {type: references, target: change-20260726-unified-driver-session-status}
source_paths:
- services/host/runtime/subsystem/stream
- services/host/driver/codex.go
summary: Codex 0.145.0 の app-server event、TUI terminal-title、BEL/OSC9 notification 実装を突合し、spinner・Action Required・bell の意味と許可される組合せを固定する調査記録。
updated: '2026-07-26'
---

## Summary

Codex CLI 0.145.0 (`rust-v0.145.0`, commit
`25af12f7e61572b0bc18ddb1008be543b91519b0`) の app-server protocol と TUI
実装を突合した。terminal tab の spinner と `[ ! ] Action Required` は OSC 0
terminal-title の排他的な2表示であり、bell は app-server event そのものではなく、
TUI が完了・承認・elicitation等から派生させる一過性の notification side effect
(BELまたはOSC 9)である。

観測された `spinnerのみ`、`!のみ`、`bellのみ`、`bell + !` は実装から説明できる。
`spinner + !` は TUI が明示的に禁止する。Agent Grid は OSC/BELを状態検出に使わず、
app-serverのturn lifecycle、server request、`serverRequest/resolved`から同じ意味を
再構築する。

## Notes

### Evidence baseline

- 実行バイナリ: `codex-cli 0.145.0`
- upstream tag: `rust-v0.145.0`
- upstream commit: `25af12f7e61572b0bc18ddb1008be543b91519b0`
- protocol正本:
  `codex-rs/app-server/README.md`,
  `codex-rs/app-server-protocol/src/protocol/`
- TUI正本:
  `codex-rs/tui/src/chatwidget/status_surfaces.rs`,
  `turn_runtime.rs`, `tool_requests.rs`, `notifications.rs`,
  `codex-rs/tui/src/notifications/`

### Event to visible pattern

| app-server / TUI event | Derived meaning | Terminal title | Notification |
|---|---|---|---|
| `turn/started` | active progress | 100ms Braille spinner | none |
| `turn/completed` with no immediate follow-up/goal continuation | agent is waiting for user | spinner removed | turn-complete BEL/OSC9 when enabled and allowed by focus condition |
| command/file/permissions approval server request | unresolved human input | blinking `[ ! ]` / `[ . ] Action Required` | approval BEL/OSC9 |
| `item/tool/requestUserInput` | unresolved human input | Action Required | plan-mode-prompt notification |
| `mcpServer/elicitation/request` including tool install/enable suggestion | unresolved human input | Action Required | elicitation BEL/OSC9 |
| `serverRequest/resolved` | request cleared | modal dismissed; spinner resumes when turn remains active | none |

`status_surfaces.rs::terminal_title_has_active_progress` returns false whenever
Action Required is visible. The Action Required title builder also excludes the
spinner item. Therefore `spinner + !` is not a supported stable pattern.

### Bell is a signal, not a state

`ChatWidget::notify` coalesces a pending notification, then `Tui::notify` applies
the focus condition and configured backend. `notification_method=auto` chooses
OSC 9 for Ghostty/iTerm2/Kitty/Warp/WezTerm and BEL for other detected terminals.
BEL writes the single byte `0x07`. Default focus policy can suppress the signal,
so the same unresolved request may appear as `!のみ` or `bell + !`.

The semantic contract is consequently:

- spinner: active progress
- `!`: unresolved human-input request
- bell: transient notification signal
- spinner and `!`: mutually exclusive
- bell and `!`: may coexist

### Driver implications

Codex Driver must consume at least:

- `turn/started`, `turn/completed`, interruption/error lifecycle
- command execution, file change, and permissions approval requests
- `item/tool/requestUserInput`
- `mcpServer/elicitation/request`, including tool suggestions
- `serverRequest/resolved`
- connection loss and unknown method diagnostics

OSC 0/2 and BEL/OSC9 remain terminal presentation mechanisms for the Codex TUI;
they are not authoritative Agent Grid status inputs. Shell OSC 133 prompt
semantics are a separate shell integration and are outside this finding.
