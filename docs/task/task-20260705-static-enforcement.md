---
id: task-20260705-static-enforcement
kind: task
title: '静的 enforcement: real-binary 隔離 lint + e2e sibling check + nightly e2e + floors
  修繕'
status: todo
created: '2026-07-05'
priority: high
effort: medium
files_touched:
- src/gorules/
- src/.golangci.yml
- scripts/check-e2e-siblings.sh
- .github/workflows/e2e-nightly.yml
- scripts/coverage-floors.txt
pr: null
tags:
- testing
- enforcement
owners: []
relations:
- {type: partOf, target: plan-20260705-test-harness}
- {type: dependencyOf, target: task-20260705-docs-llm-constraints}
source_paths:
- src/gorules/purecore.go
- src/.golangci.yml
- .github/workflows/ci.yml
- scripts/check-coverage.sh
summary: ruleguard で real CLI exec を e2e/lib 外から排除、//go:build e2e の sibling fake テスト存在
  check、nightly fidelity workflow、coverage-floors の fakecodex 行補修
---

# 静的 enforcement 一式 + nightly e2e

## 責務

「T1/T2 は fake のみで走る」「fake は定期的に real と突き合わせる」を review 依存から機械制約へ移す
(spec FR-009 / FR-010 / AC-005, adr-20260705-test-tier-taxonomy の Decision 3-4)。

## 詳細手順

1. **ruleguard: real-binary 隔離** — `src/gorules/` に追加: `exec.Command` /
   `exec.CommandContext` の第 1 引数 (または command 名) が `"claude"` / `"codex"` / `"docker"` の
   literal である呼び出しを、`platform/lib/**`・fake package (`fakeclaude` / `fakecodex` /
   `fakedocker`)・`*_e2e_test.go` 以外で禁止する。`REACTOR_E2E_*` env の参照も同様に e2e ファイル外で
   禁止する。既存コードが rule に通ることを確認し、正当な例外は `.golangci.yml` の exclusions に path
   pattern で宣言する (in-code annotation は使わない — 既存方針)。
2. **e2e sibling check** — `scripts/check-e2e-siblings.sh`: `//go:build e2e` を含む各 package に
   常時実行のテストファイル (fake ベース) が同居することを検査し、CI step に追加する。
3. **nightly fidelity workflow** — `.github/workflows/e2e-nightly.yml`: schedule (cron) で
   `make test-e2e` を実行する。real CLI の install は codex (schema-drift job と同じ pin 方式) +
   claude。credential は repo secrets から注入し、secrets 不在時は該当 suite を Skip としてジョブ自体は
   成功させる。失敗時は既存 issue 起票 action (auto-fix-ci.yml のパターン) で可視化する。PR CI には
   追加しない。
4. **floors 修繕** — `scripts/coverage-floors.txt` に `platform/agent/fakecodex` 行を追加 (UNKNOWN
   判定リスクの解消)。fakeclaude 側の floor 妥当性も確認する。

## 前提

なし (独立して着手可能)。fakedocker rule 対象は task-20260705-fakedocker と入れ違いでも
package 名を先に予約して問題ない。

## スコープ外

- AGENTS.md / note 群への規範記載 (task-20260705-docs-llm-constraints — 実装が揃ってから)
- nightly の結果を behavioral eval に拡張すること

## 受け入れ条件

- e2e ファイル外に `exec.Command("docker")` を足す negative テストで `make lint` が fail する (AC-005)
- 既存コードベース全体が新 rule / check で green
- nightly workflow が手動 dispatch (workflow_dispatch) で end-to-end 成功する
