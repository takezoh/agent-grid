---
change: change-20260706-frame-messaging
role: implementation
---

# Implementation

## Legacy Source (verbatim)

````markdown
---
id: plan-20260706-frame-messaging
kind: plan
title: Same-session frame messaging implementation plan
status: draft
created: '2026-07-06'
goal: spec-20260706-frame-messaging を、Phase 1 inbox-only の安全な frame messaging から
  prompt delivery 拡張へ段階的に実装する
scope_in:
- Phase 1 inbox-only broker/store/audit/read/reply
- Claude `.mcp.json` overlay / mcp-exec alias での tool exposure
- Codex app-server dynamicTools での tool exposure
- Web UI の MESSAGES surface と messages API
- Phase 2 以降の Codex prompt_start / Claude gated PTY delivery の設計順序
scope_out:
- Cross Session communication
- broadcast / swarm coordination
- arbitrary terminal injection
- Phase 1 での prompt delivery / waitForResponse
- Phase 1 での Web UI 送信 form
milestones:
- id: m1
  title: Phase 1 store, broker, gate, and audit
  status: done
- id: m2
  title: Phase 1 MCP tool exposure
  status: done
- id: m3
  title: Phase 1 Web MESSAGES surface
  status: done
- id: m4
  title: Phase 1 tests and restart recovery
  status: done
- id: m5
  title: Phase 2 Codex prompt_start delivery
  status: todo
- id: m6
  title: Phase 3 Claude gated PTY delivery
  status: todo
- id: m7
  title: Phase 4 policy and approval UI
  status: todo
contracts:
- 'same-session gate: target session must equal source session'
- 'source identity: daemon token/binding is authoritative; tool input cannot spoof
  source'
- 'inbox-only Phase 1: no prompt delivery, turn/start, PTY submit, or waitForResponse'
- 'audit: metadata plus body hash by default; no transcript in read API'
tags:
- mcp
- frame-messaging
owners: []
relations:
- {type: implements, target: spec-20260706-frame-messaging}
- {type: hasPart, target: task-20260706-frame-messaging-store-broker}
- {type: hasPart, target: task-20260706-frame-messaging-web-surface}
- {type: hasPart, target: task-20260706-frame-messaging-phase1-tests}
- {type: hasPart, target: task-20260706-frame-messaging-mcp-tools}
- {type: hasPart, target: adr-20260706-frame-messaging-structured-response-sources}
- {type: hasPart, target: adr-20260706-frame-messaging-managed-tool-exposure}
- {type: hasPart, target: adr-20260706-frame-messaging-daemon-broker}
- {type: referencedBy, target: note-20260706-inter-session-mcp-original-plan}
source_paths:
- src/client/state/
- src/client/runtime/
- src/client/runtime/subsystem/stream/
- src/platform/mcpproxy/
- src/server/web/
- src/client/web/
summary: Phase 1 inbox-only から Codex prompt_start、Claude gated PTY、UI/policy 管理へ段階的に進める実装計画
---

# Plan — Same-session frame messaging

## Goal

spec-20260706-frame-messaging を、最初に安全な inbox-only communication として実装する。Phase 1 の価値は、agent が同一 session 内の sibling frame に依頼を残し、返信を受け取り、人間が Web UI で確認できることにある。

prompt delivery は Phase 1 の完了条件ではない。Codex app-server `turn/start`、Claude PTY submit、response correlation、human approval は後続 Phase に分け、inbox/reply/audit の土台を先に pin する。

## Implementation Sequence

Phase 1 は次の依存順で進める。

```text
m1 -> m2
m1 -> m3
m1,m2,m3 -> m4
```

Phase 2 以降は Phase 1 の broker/store/read/reply を再利用する。

```text
m4 -> m5 -> m6 -> m7
```

{% milestone id="m1" %}
**Phase 1 store, broker, gate, and audit** — `FrameMessage` / `FrameReply` / `FrameMessagingSummary`、session-scoped `messages.jsonl`、same-session/self-target gate、body hash audit を実装する。詳細: {% task-ref id="task-20260706-frame-messaging-store-broker" /%}
{% /milestone %}

{% milestone id="m2" %}
**Phase 1 MCP tool exposure** — `agent_frames.list` / `send_message` / `read` / `reply` を Claude overlay / `mcp-exec` と Codex `dynamicTools` の入口に接続する。source identity は input ではなく daemon binding から解決する。詳細: {% task-ref id="task-20260706-frame-messaging-mcp-tools" /%}
{% /milestone %}

{% milestone id="m3" %}
**Phase 1 Web MESSAGES surface** — View summary と `GET /api/sessions/{sessionId}/messages` を追加し、session view に `MESSAGES` tab/panel を出す。送信 UI は作らない。詳細: {% task-ref id="task-20260706-frame-messaging-web-surface" /%}
{% /milestone %}

{% milestone id="m4" %}
**Phase 1 tests and restart recovery** — gate matrix、spoofing rejection、MCP schema、audit、messages replay、Web surface regression を Phase 1 の完了条件として pin する。詳細: {% task-ref id="task-20260706-frame-messaging-phase1-tests" /%}
{% /milestone %}

{% milestone id="m5" %}
**Phase 2 Codex prompt_start delivery** — idle Codex frame だけに app-server `turn/start` で prompt を配送する。`turn/steer`、Claude PTY、Cross Session は含めない。response は `turn/completed` の `phase=final_answer` を優先し、なければ heuristic として保存する。
{% /milestone %}

{% milestone id="m6" %}
**Phase 3 Claude gated PTY delivery** — hook/OSC derived idle gate、sanitization、human approval policy を通した上で Claude PTY surface input に submit する。transcript fallback は `confidence=heuristic` に限定し、VT snapshot から成功推測しない。
{% /milestone %}

{% milestone id="m7" %}
**Phase 4 policy and approval UI** — delivery status、audit viewer、pending approval UI を整える。Cross Session allow/deny UI はこの plan では扱わない。
{% /milestone %}

## Targets

- `src/client/state/`: View summary と wire model の追加。
- `src/client/runtime/`: frame messaging broker、store、gate、audit。
- `src/platform/mcpproxy/`: Claude overlay / `mcp-exec` alias exposure。
- `src/client/runtime/subsystem/stream/`: Codex dynamicTools exposure、Phase 2 の app-server delivery seam。
- `src/server/web/`: messages API と session view payload。
- `src/client/web/`: `MESSAGES` tab/panel。

## Verification

Phase 1 完了は次で判定する。

- `cd src && go test ./...` が green。
- Phase 1 task の受け入れ条件がすべて test で pin されている。
- `docs lint` が warning/error なし。
- Web UI regression は `TERMINAL` と既存 `LogTabs` の表示を壊していないことを含む。

````
