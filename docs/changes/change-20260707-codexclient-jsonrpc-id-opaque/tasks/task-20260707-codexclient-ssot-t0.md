---
id: task-20260707-codexclient-ssot-t0
kind: task
title: codexclient SSOT を RequestID (opaque bytes) 化して Handler I/F を破壊的更新
status: done
created: '2026-07-07'
priority: critical
effort: medium
files_touched:
- src/platform/agent/codexclient/conn.go
- src/platform/agent/codexclient/conn_test.go
- src/platform/agent/codexclient/client.go
- src/platform/agent/codexclient/server_test.go
pr: null
tags:
- codex
- jsonrpc
- ssot
owners: []
relations:
- {type: partOf, target: change-20260707-codexclient-jsonrpc-id-opaque}
source_paths:
- src/platform/agent/codexclient
summary: rpcMessage.ID を *codexclient.RequestID (json.RawMessage named type) に置換し
  Handler / Reply / ReplyError の signature を差し替え、pending は int64 のまま strconv.ParseInt
  経路で解決する SSOT 変更と T0 unit (string/number/null/欠如 round-trip + pending 解決)
updated: '2026-07-07'
change: change-20260707-codexclient-jsonrpc-id-opaque
---

## 責務

`codexclient.Conn` の JSON-RPC envelope 型を JSON-RPC 2.0 に準拠した bytes-preserving な opaque id に単一化し、Handler I/F を新 named type `codexclient.RequestID` に破壊的置換する。この 1 task が完了した瞬間、caller 側 (backend / orchestrator / claude-app-server / shim / fake × 2) は compile error になり、後続の並列 follow-through task がそれぞれ追従して DAG が閉じる (NFR-005 の invariant)。pending map の SSOT は int64 のまま維持し、reply 到着時に wire id bytes を `strconv.ParseInt` して照合する (改善案 2)。

## 詳細手順

1. `src/platform/agent/codexclient/conn.go` に公開型 `type RequestID json.RawMessage` を追加。`bytes.Equal` 相当の等値比較 helper (`func (r RequestID) Equal(other RequestID) bool`) と `String()` (デバッグ用 raw bytes 展開) を用意する。
2. `rpcMessage.ID` を `*RequestID` (nil = id フィールド未出現 = Notification) に変更。JSON literal `null` は `RequestID([]byte("null"))` として保持する。
3. Handler I/F を以下に変更する:
   - `OnServerRequest(id RequestID, method string, params json.RawMessage)`
   - `Conn.Reply(id RequestID, result any) error`
   - `Conn.ReplyError(id RequestID, errMsg string) error`
4. `Conn.Run` の response 分岐で、wire id bytes を `strconv.ParseInt(strings.Trim(string(*msg.ID), `"`), 10, 64)` 相当で int64 化し pending map から解決する。parse 失敗経路は task `codexclient-observability-log` で log 化するので、本 task では `continue` するだけの skeleton にとどめる。
5. `client.go` の `Request` / `Notify` / `Initialize` は外部 signature を現行維持しつつ内部で自発 Request の nextID を `RequestID(strconv.AppendInt(nil, id, 10))` に normalize する。
6. `conn_test.go` に T0 unit を追加:
   - (a) string (`"initialize"`) / number (`42`) / null (`null`) / 欠如 の 4 パターンについて `rpcMessage` を Unmarshal → Marshal 往復し id フィールドの raw bytes が一致すること (`AC-002`)
   - (b) `Conn` が自発 Request を発行 → mock transport から reply を送り込み → int64 pending map から解決される経路 (`AC-003`)
7. 検証: `cd src && go build ./platform/agent/codexclient/... && go test ./platform/agent/codexclient/...` が緑になること。`go vet` / `golangci-lint` (funlen / file length) を通す (NFR-002)。他 package の compile error は本 task の scope 外 (follow-through task で解消)。

## 前提

- なし (このタスクが DAG の起点)。
- 現行 `rpcMessage.ID *int64` を 100% 置換する。int64 に依存する内部 helper は int64 のままだが、外部 signature は RequestID に載せ替える。

## スコープ外

- silent drop 3 経路の構造化 log 化 (task `codexclient-observability-log`)
- backend / orchestrator / claude-app-server / shim / fake × 2 の caller 追従 (各 follow-through task)
- shim の 2 方向 proxy 実装 (`shim-2way-proxy`)
- id 型変更に便乗した codexclient 一般化 refactor / file 分割 (spec Non-Goals)

## 受け入れ条件

- `cd src && go build ./platform/agent/codexclient/...` が非ゼロにならない
- `cd src && go test ./platform/agent/codexclient/...` が緑
- `grep -n "RequestID" src/platform/agent/codexclient/conn.go` に named type 定義がある
- `grep -n "id int64" src/platform/agent/codexclient/conn.go` が Handler / Reply / ReplyError に対して 0 件
- T0 unit で string / number / null / 欠如 4 パターンの round-trip bytes 保存が assert されている (AC-002)
- T0 unit で int64 pending map による自発 Request の解決経路が assert されている (AC-003)
- `make lint` (funlen / file length rule) が緑 (NFR-002 / NFR-007)


{% transition from="todo" to="in_progress" date="2026-07-07" %}
Workflow start
{% /transition %}


{% transition from="in_progress" to="done" date="2026-07-07" %}
merged into work branch
{% /transition %}
