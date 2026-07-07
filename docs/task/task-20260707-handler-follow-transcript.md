---
id: task-20260707-handler-follow-transcript
kind: task
title: 'Handler follow: transcript e2e Handler I/F 追従'
status: done
created: '2026-07-07'
priority: normal
effort: medium
files_touched:
- src/client/lib/codex/transcript/transcript_e2e_test.go
pr: null
tags:
- codex
- handler-follow
owners: []
relations:
- {type: partOf, target: plan-20260707-codexclient-jsonrpc-id-opaque}
- {type: dependsOn, target: task-20260707-codexclient-ssot-t0}
source_paths:
- src/client/lib/codex/transcript
summary: transcript_e2e_test.go の Handler impl (e2eRecorder.OnServerRequest) を新 RequestID
  signature に追従する
updated: '2026-07-07'
---

# Handler follow: transcript e2e Handler I/F 追従

## 責務

`src/client/lib/codex/transcript/transcript_e2e_test.go` の `e2eRecorder.OnServerRequest(_ int64, _ string, _ json.RawMessage)` を新 `codexclient.RequestID` signature に追従する。SSOT 変更 (ssot-t0) 後の `go build ./...` が非ゼロで終了しないようにする。

## 詳細手順

1. `codexclient-ssot-t0` (blocker) がマージされた作業ブランチを取り込む。
2. `transcript_e2e_test.go:62` の `OnServerRequest(_ int64, ...)` を `OnServerRequest(_ codexclient.RequestID, ...)` に書き換える (signature のみ / test behavior 変更なし)。
3. 必要なら import に `codexclient` を追加。
4. `cd src && go build ./client/lib/codex/transcript/...` が緑になることを確認。
5. `cd src && go test ./client/lib/codex/transcript/...` が緑になることを確認。

## 前提

- `codexclient-ssot-t0` で `codexclient.Handler.OnServerRequest` の signature が `id codexclient.RequestID` に更新されている。
- `codexclient.RequestID` が `json.RawMessage` の named type として export されている。

## スコープ外

- transcript 本体 (transcript.go) のロジック変更
- e2e test の behavior 追加変更

## 受け入れ条件

- `cd src && go build ./client/lib/codex/transcript/...` が非ゼロにならない
- `cd src && go test ./client/lib/codex/transcript/...` が緑
- `grep -n "OnServerRequest(_ int64" src/client/lib/codex/transcript/transcript_e2e_test.go` が 0 件
- `make lint` 緑


{% transition from="todo" to="in_progress" date="2026-07-07" %}
Workflow start
{% /transition %}


{% transition from="in_progress" to="done" date="2026-07-07" %}
merged into work branch
{% /transition %}
