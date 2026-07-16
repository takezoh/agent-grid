---
id: task-20260706-frame-messaging-store-broker
kind: task
title: Frame messaging store, broker, and same-session gate
status: done
created: '2026-07-06'
priority: normal
effort: medium
files_touched: []
pr: null
tags:
- mcp
- frame-messaging
- phase-1
owners: []
relations:
- {type: partOf, target: change-20260706-frame-messaging}
source_paths:
- src/client/state/
- src/client/runtime/
summary: Phase 1 inbox-only の message/reply 型、append-only store、same-session/self-target
  gate、audit emission を実装する
updated: '2026-07-06'
change: change-20260706-frame-messaging
---

# Frame messaging store, broker, and same-session gate

## 責務

Phase 1 inbox-only の中核として、message/reply の durable store、daemon broker、same-session/self-target gate、audit emission を実装する。prompt delivery、turn 操作、PTY submit はこの task に含めない。

## 詳細手順

1. `FrameMessage`、`FrameReply`、`FrameMessagingSummary`、priority/resolution/answerSource/confidence の wire 型を追加する。persistence 型は stdlib-only を維持する。
2. session-scoped store を追加する。正本は append-only `messages.jsonl` とし、`message_state.json` 相当の compaction は最適化扱いにする。
3. broker API を追加する: `ListFrames(source)`, `SendMessage(source, targetFrameID, topic, body, priority)`, `Read(source, filter)`, `Reply(source, messageID, body, resolution)`。
4. hard gate を broker で強制する。source は daemon が解決した frame identity のみを使い、self-target と session 外 target を reject する。
5. audit append-only record を追加する。本文は保存せず、body hash、tool name、source/target、decision、reason を記録する。
6. read state は frame 単位で管理する。Web 既読と agent 既読の分離は Phase 1 では行わない。

## 前提

- 対象 frame は agent-grid daemon 管理下の同一 session frame。
- 同一 session 内 inbox/read/reply に allow/deny policy は入れない。

## スコープ外

- `agent_frames.deliver_prompt`
- Codex `turn/start` / `turn/steer`
- Claude PTY submit
- Cross Session policy
- Web UI 表示

## 受け入れ条件

- 同一 session target への `SendMessage` で message が保存される。
- self-target と session 外 target は reject され、audit reason が残る。
- `Reply` は message に紐付く `FrameReply` を作り、source/target が `Read` で確認できる。
- daemon restart 相当の store reload で `messages.jsonl` から message/reply/read state が復元される。
- audit record は body hash を持ち、本文を既定で保存しない。


{% transition from="todo" to="in_progress" date="2026-07-06" %}
Started Phase 1 broker/store implementation.
{% /transition %}


{% transition from="in_progress" to="done" date="2026-07-06" %}
Implemented Phase 1 broker/store/audit/replay and verified with tests plus lint.
{% /transition %}
