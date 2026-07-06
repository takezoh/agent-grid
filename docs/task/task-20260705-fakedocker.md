---
id: task-20260705-fakedocker
kind: task
title: fakedocker + real-docker backstop
status: done
created: '2026-07-05'
priority: normal
effort: large
files_touched:
- src/platform/sandbox/devcontainer/fakedocker/
- src/platform/sandbox/devcontainer/manager_lifecycle_test.go
- src/platform/sandbox/devcontainer/docker_e2e_test.go
- scripts/coverage-floors.txt
pr: null
tags:
- testing
- sandbox
- fake
owners: []
relations:
- {type: partOf, target: plan-20260705-test-harness}
- {type: dependencyOf, target: task-20260705-docs-llm-constraints}
source_paths:
- src/platform/sandbox/devcontainer/docker.go
- src/platform/sandbox/devcontainer/manager_test.go
- docs/adr/adr-20260704-cli-fake-validated-by-real-cli-e2e.md
summary: PATH 注入 fake docker CLI で devcontainer lifecycle を T1 検証し、AG_E2E_DOCKER_BIN
  opt-in の FakeVsRealDocker を追加
updated: '2026-07-05'
---

# fakedocker + real-docker backstop

## 責務

外部依存 4 種のうち唯一 fake も e2e も無い docker に 3 点セット (fake / backstop / contract) を導入する
(spec FR-008 / AC-005 の一部, adr-20260705-fakedocker-path-injection)。

## 詳細手順

1. `src/platform/sandbox/devcontainer/fakedocker/` を新設する: テスト時に `go build` する小さな fake
   `docker` binary。argv を JSON Lines で記録し、subcommand (`ps` / `inspect` / `run` / `exec` /
   `rm` / `version`) ごとに canned 応答を返す。応答は table-driven に差し替え可能にする。
2. lifecycle T1 テストを新設する: `PATH` 先頭に fakedocker を置き、manager の create → hook 実行 →
   run → remove を貫通駆動する。argv 記録に対して引数組み立て (workdir 翻訳・mount・label) を assert する。
   失敗系 (docker 非ゼロ exit、途中死) の伝搬も駆動する。
3. `FakeVsRealDocker` (T3, `//go:build e2e`) を新設する: `AG_E2E_DOCKER_BIN` 設定時のみ、同一
   シナリオを real docker に流し、canned 応答 (ps line / inspect JSON の形) と real 出力の形一致を照合
   する。未設定は Skip。使用イメージは最小 (alpine 等) に固定。
4. coverage-floors.txt の `platform/sandbox/devcontainer` floor を計測に基づいて引き上げ、
   「structurally untestable」コメントを更新する。
5. Makefile `test-e2e` の対象 package に追加、CI の `go vet -tags e2e` 対象へ追加する。

## 前提

なし (独立して着手可能)。

## スコープ外

- devcontainer 機能自体の変更・Runner interface 化 (ADR で却下済み)
- docker compose 等の追加サブコマンド対応 (現行 lifecycle が使う subcommand のみ)

## 受け入れ条件

- lifecycle T1 テストが docker 非依存で green (CI 標準ジョブ)
- real docker のある環境で `make test-e2e` + `AG_E2E_DOCKER_BIN` が green
- floor 引き上げ後 `scripts/check-coverage.sh` green、`make lint` green


{% transition from="todo" to="in_progress" date="2026-07-05" %}
implementation started and completed in this change
{% /transition %}


{% transition from="in_progress" to="done" date="2026-07-05" %}
implemented: fakedocker, lifecycle T1, opt-in docker e2e, Makefile/CI/coverage updates
{% /transition %}
