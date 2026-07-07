---
id: task-20260707-fake-appserver-string-id
kind: task
title: stream/fake/appserver.go の string id 受理 + Handler I/F 追従
status: done
created: '2026-07-07'
priority: high
effort: small
files_touched:
- src/client/runtime/subsystem/stream/fake/appserver.go
- src/client/runtime/subsystem/stream/fake/appserver_test.go
- src/client/runtime/subsystem/stream/fake/cli.go
pr: null
tags:
- codex
- fake
- string-id
owners: []
relations:
- {type: partOf, target: plan-20260707-codexclient-jsonrpc-id-opaque}
- {type: dependsOn, target: task-20260707-codexclient-ssot-t0}
source_paths:
- src/client/runtime/subsystem/stream/fake
summary: client/runtime/subsystem/stream/fake/appserver.go の serverConn.OnServerRequest
  を codexclient.RequestID signature に追従させ、string id を受理して同 id bytes を Reply に echo
  する契約を実装 + appserver_test.go を追従
updated: '2026-07-07'
---

## 責務

stream backend が直接触る fake app-server の serverConn を、新 Handler I/F に追従させると同時に **string id を含む request を受理して同 bytes を Reply に echo** する契約を実装する (FR-007)。fakecodex 側と挙動を揃えないと stream backend 系と orchestrator/agent 系の挙動が食い違う。

## 詳細手順

1. `src/client/runtime/subsystem/stream/fake/appserver.go` の `serverConn.OnServerRequest(id int64, ...)` を `OnServerRequest(id codexclient.RequestID, ...)` に置換。
2. Reply に元 id を bytes-preserving に echo する経路を実装 (`c.Reply(id, result)` — id は受信した RequestID をそのまま渡す)。既存で `Reply(int64Id, ...)` を組み立てていた場合は撤去。
3. `appserver_test.go` に string id (`"initialize"`) を含む request 送信 → 同 bytes での reply を assert する pin を追加。既存の int64 id パスも並行して緑を保つ。
4. cli.go 側で Handler を wrap している場合は必要最小限のみ signature 追従 (files_touched には含めるが responsibility 追加は行わない)。cli.go は Handler impl でなければ触らない。
5. 検証: `cd src && go build ./client/runtime/subsystem/stream/fake/... && go test ./client/runtime/subsystem/stream/fake/...` が緑。

## 前提

- `task-20260707-codexclient-ssot-t0` で `codexclient.RequestID` が公開されている
- 同時進行の `fakecodex-string-id` と挙動契約 (string id echo) を対称に実装する必要がある

## スコープ外

- fakecodex/fakecodex.go の追従 (別 task)
- shim の 2 方向 proxy 実装 (別 task)
- stream backend の Handler 実装追従 (別 task)

## 受け入れ条件

- `cd src && go build ./client/runtime/subsystem/stream/fake/...` が非ゼロにならない
- `cd src && go test ./client/runtime/subsystem/stream/fake/...` が緑
- appserver_test.go に string id request → 同 bytes での reply echo を pin する case が追加されている (FR-007)
- `grep -n "id int64" src/client/runtime/subsystem/stream/fake/appserver.go` が Handler 系で 0 件
- `make lint` 緑


{% transition from="todo" to="in_progress" date="2026-07-07" %}
Workflow start
{% /transition %}


{% transition from="in_progress" to="done" date="2026-07-07" %}
merged into work branch
{% /transition %}
