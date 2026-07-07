---
id: task-20260707-handler-follow-backend
kind: task
title: stream backend / launch flow の Handler I/F 追従
status: done
created: '2026-07-07'
priority: high
effort: small
files_touched:
- src/client/runtime/subsystem/stream/backend.go
- src/client/runtime/subsystem/stream/backend_test.go
- src/client/runtime/subsystem/stream/launch_flow_test.go
pr: null
tags:
- codex
- handler
- follow-through
owners: []
relations:
- {type: partOf, target: plan-20260707-codexclient-jsonrpc-id-opaque}
- {type: dependsOn, target: task-20260707-codexclient-ssot-t0}
source_paths:
- src/client/runtime/subsystem/stream
summary: codexclient.Handler / Reply / ReplyError の signature 変更に client/runtime/subsystem/stream/backend.go
  と 関連テスト (backend_test.go / launch_flow_test.go) の Handler 実装 caller を追従させ、notification
  経路 (AC-007) の regression が無いことを assert
updated: '2026-07-07'
---

## 責務

`codexclient.Handler` の I/F が `RequestID` に置換されたことにより、`stream/backend.go` の `Backend.Start` / `applyPatchApproval` 系 Handler と mock stub がすべて compile error を起こす。これを RequestID signature に沿って追従させ、既存の notification 経路 (turn/completed / thread/status/changed / item/agentMessage/delta) を id 型変更前と等価に維持する (FR-008 / AC-007)。

## 詳細手順

1. `src/client/runtime/subsystem/stream/backend.go` の Handler 実装 (`OnServerRequest` / `OnNotification` の signature、`applyPatchApproval` 相当の Reply / ReplyError 呼び出し) を新 signature に追従。`id int64` → `id codexclient.RequestID` へ機械的に置換し、Reply/ReplyError 呼び出しも合わせる。
2. server-initiated request を受けたときの返信は「受信した RequestID をそのまま Reply に渡す」だけの pass-through で足りる (このパッケージ独自の id 加工は本 PR では入れない)。
3. `backend_test.go` / `launch_flow_test.go` の Handler mock / stub (fakeHandler / recordingHandler 等) を新 signature に追従。既存 test の期待値 (回答内容) は変えない。
4. AC-007 用の追加 regression テスト: id 型変更後の Conn を通して `turn/completed` / `thread/status/changed` / `item/agentMessage/delta` の notification が従来通り `OnNotification` に届くことを 1 case pin する (既存 test で cover 済みなら期待値のみ確認)。
5. 検証: `cd src && go build ./client/runtime/subsystem/stream/... && go test ./client/runtime/subsystem/stream/...` が緑。

## 前提

- `task-20260707-codexclient-ssot-t0` で Handler I/F が新 signature に切り替わっている
- backend.go はもともと 500 行超えの制約対象なので、追従に伴う微増は許容範囲内 (機能追加はしない)

## スコープ外

- backend への logger 注入や新 event 追加
- fake app-server / fakecodex の追従 (それぞれ別 task)
- shim / orchestrator handler の追従 (それぞれ別 task)

## 受け入れ条件

- `cd src && go build ./client/runtime/subsystem/stream/...` が非ゼロにならない
- `cd src && go test ./client/runtime/subsystem/stream/...` が緑
- `grep -n "id int64" src/client/runtime/subsystem/stream/backend.go` が Handler 系で 0 件
- notification 3 種 (turn/completed / thread/status/changed / item/agentMessage/delta) の dispatch が id 型変更前と等価であることが T0 で assert されている (AC-007)
- `make lint` (funlen / file length) 緑


{% transition from="todo" to="in_progress" date="2026-07-07" %}
Workflow start
{% /transition %}


{% transition from="in_progress" to="done" date="2026-07-07" %}
merged into work branch
{% /transition %}
