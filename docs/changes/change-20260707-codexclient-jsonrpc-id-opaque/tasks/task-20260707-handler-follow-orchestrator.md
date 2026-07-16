---
id: task-20260707-handler-follow-orchestrator
kind: task
title: orchestrator/agent Handler の I/F 追従
status: done
created: '2026-07-07'
priority: high
effort: small
files_touched:
- src/orchestrator/agent/handler.go
- src/orchestrator/agent/runner_events_test.go
- src/orchestrator/agent/runner_loop_test.go
- src/orchestrator/agent/handler_test.go
pr: null
tags:
- codex
- handler
- follow-through
- orchestrator
owners: []
relations:
- {type: dependsOn, target: task-20260707-codexclient-ssot-t0}
- {type: partOf, target: change-20260707-codexclient-jsonrpc-id-opaque}
source_paths:
- src/orchestrator/agent
summary: codexclient.Handler / Reply / ReplyError の signature 変更に orchestrator/agent/handler.go
  と関連テスト (runner_events_test.go / runner_loop_test.go / handler_test.go) の Handler
  実装 caller を追従させる
updated: '2026-07-07'
change: change-20260707-codexclient-jsonrpc-id-opaque
---

## 責務

`codexclient.Handler` の I/F 破壊的更新に伴う orchestrator 側の caller 追従。orchestrator/agent 内で Handler を実装している型と、その mock を持つ 3 本の test の signature を機械的に更新する。挙動追加 (id 加工 / event 追加) は本 task では行わない。

## 詳細手順

1. `src/orchestrator/agent/handler.go` の Handler 実装 (`OnServerRequest` の signature、Reply / ReplyError 呼び出し) を新 `RequestID` に追従。
2. `runner_events_test.go` / `runner_loop_test.go` / `handler_test.go` の Handler mock (fakeCodexHandler など) の signature を追従。既存 test の期待挙動は変えない。
3. depguard 境界 (orchestrator/* が client/* を import しない) を維持したまま追従することを確認。
4. 検証: `cd src && go build ./orchestrator/... && go test ./orchestrator/...` が緑。

## 前提

- `task-20260707-codexclient-ssot-t0` で Handler I/F が新 signature に切り替わっている
- 現行 orchestrator/agent/handler.go は 283 行、80 行/func 制約は追従で崩さない

## スコープ外

- orchestrator 側の event / log 追加
- shim / fake / backend の追従 (それぞれ別 task)

## 受け入れ条件

- `cd src && go build ./orchestrator/...` が非ゼロにならない
- `cd src && go test ./orchestrator/...` が緑
- `grep -n "id int64" src/orchestrator/agent/handler.go` が Handler 系で 0 件
- `make lint` (depguard 含む) 緑 (NFR-007)


{% transition from="todo" to="in_progress" date="2026-07-07" %}
Workflow start
{% /transition %}


{% transition from="in_progress" to="done" date="2026-07-07" %}
merged into work branch
{% /transition %}
