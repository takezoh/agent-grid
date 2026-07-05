---
id: task-20260705-docs-llm-constraints
kind: task
title: AGENTS.md / enforcement note / testing note へのハーネス規範反映
status: todo
created: '2026-07-05'
priority: normal
effort: small
files_touched:
- AGENTS.md
- docs/note/note-20260624-technical-code-enforcement.md
- docs/note/note-20260624-agent-testing.md
pr: null
tags:
- docs
- enforcement
owners: []
relations:
- {type: partOf, target: plan-20260705-test-harness}
- {type: dependsOn, target: task-20260705-runtimetest-harness}
- {type: dependsOn, target: task-20260705-tap-osc-contract}
- {type: dependsOn, target: task-20260705-relay-severance-contract}
- {type: dependsOn, target: task-20260705-driver-conformance-suite}
- {type: dependsOn, target: task-20260705-fakecodex-settings-updated}
- {type: dependsOn, target: task-20260705-wire-fixtures-pipeline}
- {type: dependsOn, target: task-20260705-gateway-scenario-e2e}
- {type: dependsOn, target: task-20260705-fakedocker}
- {type: dependsOn, target: task-20260705-static-enforcement}
source_paths:
- AGENTS.md
- docs/note/note-20260624-technical-code-enforcement.md
- docs/note/note-20260624-agent-testing.md
summary: 実装完了後に LLM 向け規範 4 行を AGENTS.md へ、test-pin 節を enforcement note へ、Tier 体系を
  testing note へ反映する最終ドキュメント統合
---

# AGENTS.md / enforcement note / testing note へのハーネス規範反映

## 責務

実装された仕組みを正本ドキュメントへ反映する最終統合 task。**実装が存在しない規範を先に書かない**ため、
依存 task 完了後に実施する。

## 詳細手順

1. **AGENTS.md** — 短い規範のみ追加 (lint で機械強制されるものは書かない、詳細は note へリンク):
   - 外部依存を触るテストは 3 点セット (fake + FakeVsReal e2e + contract) を伴う (ADR 参照)
   - FakeVsReal が落ちたら fake を直す。assertion を緩めない
   - 伝搬テストは `runtimetest.Harness` / `drivertest.Conformance` を使う。ad-hoc 起動を書かない
   - テストの置き場所は T0-T3 tier で判断する (testing note 参照)
2. **enforcement note** (`note-20260624-technical-code-enforcement.md`) — test-pin 節を追加:
   §8 tap OSC routing、§9 relay severance、§10 docker fake fidelity、§11 wire fixtures gate、
   §12 driver conformance / metadata source priority。§1 の depguard 表に ruleguard real-binary
   隔離 rule を追記。
3. **testing note** (`note-20260624-agent-testing.md`) — Tier 体系 (T0-T3) の節を追加し、coverage
   tier (S-D) との直交性を明記。既存の「Multiplexed-subsystem routing harness」節と同列に新 harness
   群を追記。
4. docs CLI で relations を張る (`references` : 各新 ADR ↔ 両 note)。`docs lint` zero error。
5. 完了時、plan-20260705-test-harness の milestones を done に遷移し、spec を implemented へ transition
   する (全 task 完了が条件)。

## 前提

- m1-m9 相当の task が done であること (task-20260705-runtimetest-harness,
  task-20260705-tap-osc-contract, task-20260705-relay-severance-contract,
  task-20260705-driver-conformance-suite, task-20260705-fakecodex-settings-updated,
  task-20260705-wire-fixtures-pipeline, task-20260705-gateway-scenario-e2e,
  task-20260705-fakedocker, task-20260705-static-enforcement)。m10 / m11 (fuzz-reduce /
  recorded-fake-fixtures) は could 優先度のため、未完なら該当記述を保留にして進めて良い。

## スコープ外

- CLAUDE.md (ユーザー私物) の変更
- 新規 note の作成 (既存 note への追記で足りる)

## 受け入れ条件

- `docs lint` zero error / `docs lint --drift` で新規 drift なし
- AGENTS.md の追記が 6 行以内 (規範の肥大化防止)
- enforcement note の各新節が「何を防ぐか / どこで定義 / どう強制 / 例外」の既存フォーマットに従う
