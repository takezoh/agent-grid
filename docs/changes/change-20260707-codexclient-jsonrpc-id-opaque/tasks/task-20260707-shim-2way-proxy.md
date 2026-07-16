---
id: task-20260707-shim-2way-proxy
kind: task
title: cmd/bridge codex_app_server_shim の 2 方向 proxy 実装 + T2 contract test
status: done
created: '2026-07-07'
priority: critical
effort: medium
files_touched:
- src/cmd/bridge/codex_app_server_shim.go
- src/cmd/bridge/codex_app_server_shim_test.go
pr: null
tags:
- codex
- shim
- proxy
- contract-test
owners: []
relations:
- {type: dependsOn, target: task-20260707-codexclient-ssot-t0}
- {type: partOf, target: change-20260707-codexclient-jsonrpc-id-opaque}
source_paths:
- src/cmd/bridge
summary: 'codex_app_server_shim.go の codexShimSession に downstream (CLI→upstream:
  id echo) と upstream (server→downstream: 新規 numeric 採番) の 2 方向 proxy を実装、shim_test.go
  に string id initialize の in-process T2 contract を pin'
updated: '2026-07-07'
change: change-20260707-codexclient-jsonrpc-id-opaque
---

## 責務

shim の 2 本 Conn (downstream = CLI 相手 / upstream = real codex-app-server 相手) は方向で役割が反転する。それぞれの Handler 実装で受信 RequestID を捕まえ、対称構造で proxy する。ここが正しく動かないと codex-cli 0.142.5 の `initialize` が 10s timeout → exit_code=1 → 60s reap で session を停止させる根本原因になる。

## 詳細手順

1. `src/cmd/bridge/codex_app_server_shim.go` の `codexShimSession` に以下 2 方向を実装する:
   - **downstream → upstream** (CLI initiator): `downstream.Handler.OnServerRequest(id RequestID, method, params)` で受けた `id` を struct field (もしくは request-scoped ctx) に保存し、`upstream.Request(method, params)` を呼ぶ (upstream 側は Conn 内部で新規 numeric 採番)。upstream から result / error を受けたら `downstream.Reply(savedID, result)` / `downstream.ReplyError(savedID, msg)` で **元 bytes を echo**。
   - **upstream → downstream** (server-initiated, e.g. applyPatchApproval): `upstream.Handler.OnServerRequest(id RequestID, method, params)` で受けた id を保存し、`downstream.Request(method, params)` を呼ぶ (downstream 側で新規 numeric 採番)。downstream 返却後 `upstream.Reply(savedID, result)` で echo。
2. 保存領域は sync.Map[correlationKey]RequestID 相当で shim 単位で持つ。correlation key の設計は「downstream から upstream への転送時に upstream 側の新規採番 int64 を key、downstream の RequestID を value」とする。
3. `shim_test.go` に T2 contract test を追加 (in-process、real binary 不要):
   - (a) fake AppServer + shim + test client を組み立て `{"id":"initialize","method":"initialize","params":{...}}` を送信 → 10 秒以内に `{"id":"initialize","result":{...}}` が返り、reply の id bytes が入力と等価 (AC-001 / AC-004 の downstream 側)
   - (b) upstream 発信の server-initiated request (`applyPatchApproval`) を fake から発火 → downstream で新規 numeric id で転送されていること、reply が upstream の元 id で echo されることを pin (AC-004 の upstream 側)
4. shim の Handler I/F 追従 (m2 分) はこの task に統合する。追従だけの中間コミットを別立てにしない。
5. 検証: `cd src && go build ./cmd/bridge/... && go test ./cmd/bridge/...` が緑。テストは build tag `e2e` 不要 (T2 contract は in-process)。

## 前提

- `task-20260707-codexclient-ssot-t0` で `codexclient.RequestID` が公開されている
- fake app-server の string id echo が (`fake-appserver-string-id` で) 実装されるが、本 task の T2 contract は必ずしも fake の string id echo に依存しない (fake が numeric でも contract は shim 側だけで pin できる)。並列でも問題ない

## スコープ外

- FakeVsReal (real codex-cli 0.142.5) を挟んだ e2e (別 task `fakevsreal-shim-inversion`)
- shim への構造化 log 追加 (別 PR)
- upstream / downstream 双方向で id 型を混ぜる edge case (現行は片方向ずつ)

## 受け入れ条件

- `cd src && go build ./cmd/bridge/...` が非ゼロにならない
- `cd src && go test ./cmd/bridge/...` が緑
- shim_test.go に string id initialize の bytes-preserving reply を pin する T2 case (AC-001)
- shim_test.go に upstream server-initiated request が downstream で新規 numeric 採番されること + reply の upstream 側 echo を pin する T2 case (AC-004)
- `grep -n "id int64" src/cmd/bridge/codex_app_server_shim.go` が Handler 系で 0 件
- `make lint` (funlen / file length rule) 緑


{% transition from="todo" to="in_progress" date="2026-07-07" %}
Workflow start
{% /transition %}


{% transition from="in_progress" to="done" date="2026-07-07" %}
merged into work branch
{% /transition %}
