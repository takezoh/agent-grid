---
id: adr-20260706-frame-messaging-daemon-broker
kind: adr
title: Frame messaging is same-session inbox-first via daemon broker
status: accepted
created: '2026-07-06'
decision_makers:
- unknown
tags:
- mcp
- frame-messaging
- architecture
owners: []
relations:
- {type: partOf, target: plan-20260706-frame-messaging}
- {type: referencedBy, target: note-20260706-inter-session-mcp-original-plan}
source_paths:
- src/client/runtime/
- src/client/state/
- src/server/web/
summary: frame messaging は外部 MCP 注入器ではなく daemon broker を authority とし、Phase 1 は same-session
  inbox/reply に限定する
updated: '2026-07-06'
---

# Frame messaging is same-session inbox-first via daemon broker

## Context

{% context %}
agent-grid の frame 間 communication は、外部 MCP server が terminal へ文字列を注入するだけでは要件を満たさない。必要なのは、source identity、target scope、message/reply の永続化、既読、audit、Web UI 表示を daemon が一貫して管理することである。

また、初期実装で session をまたぐ通信を許すと、cross-project approval、allowlist、cohort policy、audit retention の設計が同時に必要になる。これは Phase 1 の目的である「同一 session 内の sibling frame へ安全に依頼を残し、返信を読めること」より大きい。
{% /context %}

## Decision

{% decision %}
frame messaging の authority は daemon 内 broker とする。MCP tool は broker への入口であり、権限判定、same-session gate、self-target rejection、message persistence、audit emission は daemon 側で強制する。

Phase 1 は inbox-first とし、`agent_frames.list`、`agent_frames.send_message`、`agent_frames.read`、`agent_frames.reply` のみを提供する。prompt delivery は message ではなく target agent の turn を操作する強い capability なので、別 capability として default deny にする。

target は source frame と同じ session に属する `claude` / `codex` driver frame に限定する。Cross Session communication、broadcast、swarm coordination、session-level alias はこの plan から外す。
{% /decision %}

## Consequences

{% consequence kind="positive" %}
source identity spoofing と session 外操作を broker で防げる。message/reply は durable object になり、Web UI と MCP read が同じ正本を参照できる。
{% /consequence %}

{% consequence kind="negative" %}
任意の外部 terminal や既存 tmux pane への後付け入力は扱えない。Cross Session の要望が出た場合は、別 plan と policy/approval ADR が必要になる。
{% /consequence %}

{% consequence kind="neutral" %}
prompt delivery は後続 Phase で broker の capability として追加できるが、inbox message と同じ操作にはしない。
{% /consequence %}

## Alternatives

- **外部 MCP server を authority にする** — 却下。source frame 認証、target runtime への安全な access、audit は daemon state が必要であり、外部 process だけでは強制できない。
- **CLI stdin/PTY 注入を主機能にする** — 却下。依頼、応答、監査、既読を durable に管理できず、session 境界も daemon で強制できない。
- **初期実装から Cross Session を許す** — 却下。approval と allowlist の設計面が広がり、Phase 1 の inbox/reply 基盤を遅らせる。
- **同一 session 内でも allow/deny policy を入れる** — 却下。Phase 1 では同一 session を trust boundary とし、policy は prompt delivery 以降に限定する。


{% transition from="proposed" to="accepted" date="2026-07-06" %}
Decision extracted from promoted inter-session MCP plan
{% /transition %}
