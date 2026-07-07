---
id: task-20260706-frame-messaging-phase1-tests
kind: task
title: Phase 1 frame messaging tests and restart recovery
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
- testing
owners: []
relations:
- {type: partOf, target: plan-20260706-frame-messaging}
- {type: dependsOn, target: task-20260706-frame-messaging-store-broker}
- {type: dependsOn, target: task-20260706-frame-messaging-mcp-tools}
- {type: dependsOn, target: task-20260706-frame-messaging-web-surface}
source_paths:
- src/client/runtime/
- src/server/web/
- src/client/web/
summary: gate matrix、MCP schema、audit、messages.jsonl replay、Web surface regression
  を Phase 1 の完了条件として pin する
updated: '2026-07-06'
---

# Phase 1 frame messaging tests and restart recovery

## 責務

Phase 1 を「実装済み」と判定するための test coverage を pin する。broker/store/MCP/Web の happy path だけでなく、scope gate、source spoofing、audit privacy、restart recovery、prompt delivery 非公開を検証する。

## 詳細手順

1. broker gate matrix test を追加する: same-session allow、self-target reject、session 外 reject、non-agent frame reject。
2. source spoofing rejection test を追加する: tool input に source identity 相当 field があっても schema/broker が採用しない。
3. store replay test を追加する: `messages.jsonl` から message/reply/read state を復元できる。
4. audit test を追加する: decision/reason/body hash は残り、本文と raw transcript は既定で保存されない。
5. MCP schema/invocation test を追加する: `list` / `send_message` / `read` / `reply` が broker を通り、`deliver_prompt` は存在しない。
6. Web surface regression test を追加する: `TERMINAL` と既存 `LogTabs` を維持し、`MESSAGES` が summary/API から描画される。
7. Phase 1 の verification command を plan に合わせ、`cd src && go test ./...` と必要な client/web test を明記する。

## 前提

- Phase 1 の store/broker、MCP tools、Web surface が実装済み。

## スコープ外

- Codex app-server prompt delivery tests
- Claude PTY delivery tests
- real CLI fidelity tests
- Cross Session policy tests

## 受け入れ条件

- `cd src && go test ./...` が green。
- broker gate、spoofing rejection、audit privacy、restart recovery のテストが存在する。
- MCP tool set に `deliver_prompt` が含まれないことをテストで確認する。
- Web regression test が `TERMINAL`、既存 `LogTabs`、`MESSAGES` の共存を確認する。
- Phase 1 の acceptance AC-001〜AC-008 に対応するテストが trace できる。


{% transition from="todo" to="in_progress" date="2026-07-06" %}
Started Phase 1 regression coverage work.
{% /transition %}


{% transition from="in_progress" to="done" date="2026-07-06" %}
Added broker/MCP/replay/privacy regression coverage and verified with go test ./... plus lint.
{% /transition %}
