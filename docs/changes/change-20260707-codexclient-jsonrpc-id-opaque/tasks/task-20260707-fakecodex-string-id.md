---
id: task-20260707-fakecodex-string-id
kind: task
title: platform/agent/fakecodex/fakecodex.go の string id 受理 + Handler I/F 追従
status: done
created: '2026-07-07'
priority: high
effort: small
files_touched:
- src/platform/agent/fakecodex/fakecodex.go
- src/platform/agent/fakecodex/fakecodex_test.go
pr: null
tags:
- codex
- fake
- string-id
owners: []
relations:
- {type: dependsOn, target: task-20260707-codexclient-ssot-t0}
- {type: partOf, target: change-20260707-codexclient-jsonrpc-id-opaque}
source_paths:
- src/platform/agent/fakecodex
summary: fakecodex.Server.OnServerRequest を codexclient.RequestID signature に追従させ、string
  id 受理と同 id bytes echo を実装 + fakecodex_test.go を追従。stream fake と挙動を同時更新 (FR-007)
updated: '2026-07-07'
change: change-20260707-codexclient-jsonrpc-id-opaque
---

## 責務

orchestrator / real-cli e2e が直接触る `fakecodex.Server` を、新 Handler I/F に追従させると同時に **string id を含む request を受理して同 bytes を Reply に echo** する契約を実装する (FR-007)。stream/fake/appserver.go 側と挙動を対称に更新することが要件。

## 詳細手順

1. `src/platform/agent/fakecodex/fakecodex.go` の `Server.OnServerRequest(id int64, ...)` を `OnServerRequest(id codexclient.RequestID, ...)` に置換。
2. Reply / ReplyError も同 signature に追従し、受信した RequestID を bytes-preserving にそのまま echo する経路を実装する。
3. `fakecodex_test.go` に string id (`"initialize"`) の request → 同 bytes reply を assert する case を追加。既存の numeric id パスも並行して緑を保つ。
4. `default_turn_contract.go` / `presets.go` / `recordings_test.go` は Handler 実装ではないため触らないが、fake の挙動契約に string id echo が含まれることをコメントで明示する余地があれば追記 (必須ではない)。
5. 検証: `cd src && go build ./platform/agent/fakecodex/... && go test ./platform/agent/fakecodex/...` が緑 (build tag `e2e` なし条件)。

## 前提

- `task-20260707-codexclient-ssot-t0` で `codexclient.RequestID` が公開されている
- `fake-appserver-string-id` と挙動契約 (string id echo) を対称に実装する

## スコープ外

- codex_real_cli_e2e_test.go の FakeVsReal 反転 subtest 追加 (別 task `fakevsreal-shim-inversion`)
- fakecodex_test.go への codex-cli 0.142.5 特有の挙動追加 (real cli テストは build tag 経路で扱う)
- stream/fake/appserver.go の追従 (別 task)

## 受け入れ条件

- `cd src && go build ./platform/agent/fakecodex/...` が非ゼロにならない
- `cd src && go test ./platform/agent/fakecodex/...` が緑 (build tag なし)
- fakecodex_test.go に string id request → 同 bytes での reply echo を pin する case が追加されている (FR-007)
- `grep -n "id int64" src/platform/agent/fakecodex/fakecodex.go` が Handler 系で 0 件
- `make lint` 緑


{% transition from="todo" to="in_progress" date="2026-07-07" %}
Workflow start
{% /transition %}


{% transition from="in_progress" to="done" date="2026-07-07" %}
merged into work branch
{% /transition %}
