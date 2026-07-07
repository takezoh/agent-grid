---
id: task-20260707-codexclient-observability-log
kind: task
title: codexclient.Conn.Run silent drop 経路 3 種を構造化 log 化
status: done
created: '2026-07-07'
priority: high
effort: small
files_touched:
- src/platform/agent/codexclient/conn.go
- src/platform/agent/codexclient/conn_test.go
pr: null
tags:
- codex
- observability
- logging
owners: []
relations:
- {type: partOf, target: plan-20260707-codexclient-jsonrpc-id-opaque}
- {type: dependsOn, target: task-20260707-codexclient-ssot-t0}
source_paths:
- src/platform/agent/codexclient
summary: codexclient.Conn.Run の json.Unmarshal 失敗 / id parse 失敗 / pending miss の 3
  経路を slog で構造化 log に落とし、transport は close せず後続 message を継続処理する変更と T0 log capture
  test
updated: '2026-07-07'
---

## 責務

現行 `Conn.Run` は 3 種の silent drop 経路 (JSON decode 失敗 / id 型不正 / pending miss) を `continue` だけで呑んでおり、bug 事案の初動 `/debug` で server.log を grep しても跡が残らない。この observability blind spot を構造化 log で埋め、事案再発時に log grep で 1 手で捕捉できる状態にする。transport は close せず、per-message skip で継続処理する (案 A)。

## 詳細手順

1. `src/platform/agent/codexclient/conn.go` の `Conn.Run` に slog を差し込む (第一候補は `slog.Default()`。Conn に logger を注入する I/F は本 PR では追加しない — plan.md Open Questions 参照)。
2. 3 経路それぞれに以下の event 名で `slog.Warn` (もしくは `slog.Error`) を出す:
   - `codexclient.decode_error` — `json.Unmarshal(data, &msg)` 失敗時。attribute: `raw` (先頭 256 bytes に truncate), `err`
   - `codexclient.invalid_id` — wire id が JSON-RPC 2.0 で禁じられた型 (object / array) または int64 parse 不能 numeric。attribute: `raw_id` (bytes), `method`, `err`
   - `codexclient.pending_miss` — id は parse できたが pending map に対応 request が居ない。attribute: `raw_id`, `method`, `result_len`, `error_len`
3. 3 経路とも `continue` で後続 message 処理を継続する (transport は close しない)。
4. FR-006 (method 空 + id=null 経路) は `codexclient.pending_miss` の一種として同 event で扱い、attribute で method="" を明示する。
5. `src/platform/agent/codexclient/conn_test.go` に log capture テストを追加:
   - `slog.NewJSONHandler` で bytes.Buffer を仕込み、`slog.SetDefault` して invalid envelope 3 種 (id=object / int64 overflow / method 空 + id=null) を注入
   - buffer に `codexclient.decode_error` / `codexclient.invalid_id` / `codexclient.pending_miss` の entry が現れることを assert
   - transport が close されず、その後の valid message の処理継続を assert (AC-006)
6. 検証: `cd src && go test ./platform/agent/codexclient/... -run TestConnRunLog` が緑。log rate limit は本 PR では入れない (plan.md Open Questions)。

## 前提

- `task-20260707-codexclient-ssot-t0` で SSOT 変更が完了しており、rpcMessage.ID の bytes-preserving 型が確立している
- 出力先 slog は `slog.Default()` (500 行 target と 80 行 func 制約を優先し I/F 注入は避ける)

## スコープ外

- log rate limit / sampling (別 PR で扱う)
- Conn に logger を注入する I/F 追加 (現行 default で足りる)
- 他パッケージへの log event 波及 (backend / orchestrator など)

## 受け入れ条件

- `cd src && go test ./platform/agent/codexclient/...` が緑
- T0 unit が `codexclient.decode_error` / `codexclient.invalid_id` / `codexclient.pending_miss` の 3 event を capture できる
- invalid envelope 注入後も transport は close されず、後続の valid message は正常処理される (AC-006)
- `grep -n "codexclient.decode_error\|codexclient.invalid_id\|codexclient.pending_miss" src/platform/agent/codexclient/conn.go` に 3 event 名がすべて存在する
- `make lint` (funlen) 緑 — Conn.Run が 80 行を超えた場合は helper 関数を切り出す


{% transition from="todo" to="in_progress" date="2026-07-07" %}
Workflow start
{% /transition %}


{% transition from="in_progress" to="done" date="2026-07-07" %}
merged into work branch
{% /transition %}
