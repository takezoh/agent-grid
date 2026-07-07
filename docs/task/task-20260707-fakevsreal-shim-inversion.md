---
id: task-20260707-fakevsreal-shim-inversion
kind: task
title: fakecodex real-cli e2e に shim を挟んだ FakeVsReal 反転 subtest 追加
status: done
created: '2026-07-07'
priority: normal
effort: small
files_touched:
- src/platform/agent/fakecodex/codex_real_cli_e2e_test.go
pr: null
tags:
- codex
- fakevsreal
- e2e
owners: []
relations:
- {type: partOf, target: plan-20260707-codexclient-jsonrpc-id-opaque}
- {type: dependsOn, target: task-20260707-shim-2way-proxy}
- {type: dependsOn, target: task-20260707-fakecodex-string-id}
source_paths:
- src/platform/agent/fakecodex
summary: codex_real_cli_e2e_test.go に //go:build e2e + AG_E2E_CODEX_BIN 相乗りの subtest
  を追加、agent-grid shim を app-server 役として起動し real codex-cli 0.142.5 を driving する反転方向で
  string id initialize が bytes-preserving に返ることを assert (AC-005)
updated: '2026-07-07'
---

## 責務

既存の real-cli e2e (`codex_real_cli_e2e_test.go`) は agent-grid が client / real binary が app-server の方向。本 task で追加する subtest は **agent-grid の shim を app-server 役として起動し、real codex-cli 0.142.5 を子プロセスとして driving する反転方向** の FakeVsReal を pin する (adr-20260707-fakevsreal-shim-inversion)。build tag と env gate は既存に相乗り、新 tag を増やさない (spec Non-Goals)。

## 詳細手順

1. `src/platform/agent/fakecodex/codex_real_cli_e2e_test.go` に `//go:build e2e` を先頭に持つ既存 file 構造をそのまま利用し、subtest (`t.Run("shim_inverted_string_id_initialize", ...)`) を追加。
2. subtest 内で:
   - `AG_E2E_CODEX_BIN` env に real codex-cli 0.142.5 の絶対パスが指定されていない場合 `t.Skip` (既存 gate に相乗り)
   - agent-grid の shim を in-process (もしくは短命プロセス) で app-server 役として起動
   - real codex-cli を子プロセスで起動し、shim 経由で `initialize` request (string id) を送出させる
   - shim を透過して real codex-app-server から成功 reply が返ること、reply の id が入力と bytes-preserving に一致することを assert (AC-005)
3. failure 時は fake ではなく shim / codexclient 側を疑うこと。assertion を弱めない (NFR-003)。
4. 通常 CI (build tag なし) では自動 skip されることを確認 (AC-008)。
5. 検証コマンド (opt-in 手動):
   ```
   cd src && AG_E2E_CODEX_BIN=$(which codex) go test -tags e2e ./platform/agent/fakecodex/...
   ```

## 前提

- `task-20260707-shim-2way-proxy` の shim 2 方向 proxy が完了している (real codex-cli を通す前提)
- `task-20260707-fakecodex-string-id` の string id 契約が fakecodex 側に入っている (shim 経由で参照される可能性)
- real codex-cli 0.142.5 が local に install されている場合のみ opt-in で走る

## スコープ外

- 新 build tag / env gate の追加 (既存 `e2e` + `AG_E2E_CODEX_BIN` に相乗り)
- shim 経由でない real cli e2e のカバレッジ拡大 (既存 subtest の維持のみ)
- CI 上での自動 opt-in 起動 (要 real binary 配布、本 PR 範囲外)

## 受け入れ条件

- `cd src && go test ./...` (build tag なし) で自動 skip される (AC-008)
- `cd src && AG_E2E_CODEX_BIN=$(which codex) go test -tags e2e ./platform/agent/fakecodex/...` の手動実行で追加 subtest が pass する (AC-005)
- 追加 subtest 名に `shim_inverted` または相当の keyword が含まれ、既存の non-inverted subtest と分離して読める
- `grep -n "AG_E2E_CODEX_BIN" src/platform/agent/fakecodex/codex_real_cli_e2e_test.go` が gate 用に参照されている
- `make lint` 緑


{% transition from="todo" to="in_progress" date="2026-07-07" %}
Workflow start
{% /transition %}


{% transition from="in_progress" to="done" date="2026-07-07" %}
merged into work branch
{% /transition %}
