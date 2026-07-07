---
id: task-20260707-handler-follow-claude-app-server
kind: task
title: cmd/claude-app-server の Handler I/F 追従
status: done
created: '2026-07-07'
priority: high
effort: small
files_touched:
- src/cmd/claude-app-server/server.go
- src/cmd/claude-app-server/shim_test.go
- src/cmd/claude-app-server/toolbridge_test.go
pr: null
tags:
- codex
- handler
- follow-through
- claude-app-server
owners: []
relations:
- {type: partOf, target: plan-20260707-codexclient-jsonrpc-id-opaque}
- {type: dependsOn, target: task-20260707-codexclient-ssot-t0}
source_paths:
- src/cmd/claude-app-server
summary: cmd/claude-app-server 配下の Handler 実装 caller (server.go の toolbridge / shim
  系および *_test.go) を codexclient.RequestID signature に追従させる
updated: '2026-07-07'
---

## 責務

claude-app-server binary (codex app-server stdio shim for Claude) が持つ Handler 実装 / mock を新 `RequestID` signature に追従させる。server.go, shim_test.go, toolbridge_test.go が対象。

## 詳細手順

1. `src/cmd/claude-app-server/server.go` の Handler 実装 (OnServerRequest / OnNotification / Reply / ReplyError 呼び出し) を新 signature に追従。
2. `src/cmd/claude-app-server/shim_test.go` / `toolbridge_test.go` の mock Handler / assertion を追従。
3. `main.go` / `launch.go` / `dynamictool.go` / `turn.go` / `toolbridge.go` にも Handler impl / caller があれば同様に追従 (grep で洗い出し)。
4. 検証: `cd src && go build ./cmd/claude-app-server/... && go test ./cmd/claude-app-server/...` が緑。conformance_test.go も緑 (既存 conformance 契約は変えない)。

## 前提

- `task-20260707-codexclient-ssot-t0` で Handler I/F が新 signature に切り替わっている
- codex-cli 0.142.5 の string id 問題は shim 側 (別 task) が請け負う。claude-app-server 側は int64 のまま扱っても差し支えない場面が多いが、received id を bytes-preserving に扱う (自 signature に沿って RequestID を素通しする) こと

## スコープ外

- claude-app-server 側で string id 加工を新規に導入すること (shim 側の責務)
- CLI 側の Codex.command 切り替え運用の変更 (WORKFLOW.md 参照)

## 受け入れ条件

- `cd src && go build ./cmd/claude-app-server/...` が非ゼロにならない
- `cd src && go test ./cmd/claude-app-server/...` が緑
- `grep -rn "id int64" src/cmd/claude-app-server/` が Handler / Reply / ReplyError 呼び出しで 0 件
- `make lint` 緑


{% transition from="todo" to="in_progress" date="2026-07-07" %}
Workflow start
{% /transition %}


{% transition from="in_progress" to="done" date="2026-07-07" %}
merged into work branch
{% /transition %}
