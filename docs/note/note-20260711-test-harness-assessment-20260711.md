---
id: note-20260711-test-harness-assessment-20260711
kind: note
title: Test harness assessment and roadmap — 2026-07-11
status: published
created: '2026-07-11'
tags:
- testing
- harness
- assessment
owners: []
relations:
- {type: references, target: adr-20260705-test-tier-taxonomy}
- {type: references, target: change-20260705-test-harness}
source_paths:
- docs/specs/test-harness/
- docs/note/note-20260624-agent-testing.md
- docs/note/note-20260624-technical-code-enforcement.md
- .github/workflows/ci.yml
- .github/workflows/e2e-nightly.yml
- Makefile
- scripts/
- src/gorules/
summary: 既存 test-harness の再入監査、Tier/gap matrix、north-star、AI事故防御の残ロードマップ
updated: '2026-07-24'
---

# Harness Assessment: agent-grid

## Summary

2026-07-05 の test-harness plan は全 12 milestone が実装済みで、今回の再監査でも主要 gate は実在した。
294 Go test files、86 TypeScript test filesに加え、T0 pure、T1 wired、T2 contract/fuzz、T3 fidelity の
全 Tier、共有 fake、registry conformance、race/fuzz/coverage/diff gate、nightly fidelity が揃っている。

最も成熟した模範象限は stream routing である。direct contract、wired fake、fuzz、real app-server
backstop、canonical recording が同一の routing isolation を検証する。この形を残る外部依存へ外挿する。

今回、FakeVsReal 契約が pin する Codex CLI 0.142.5 と nightly の 0.133.0 の drift を修復した。残課題は、
既存外部依存への3点セット適用範囲、3点セットの機械的な加入検査、skip/flaky/mutation、ローカル速度階層である。

## 1. プロジェクト特性

- 境界: `host/state` と `orchestrator/scheduler` の pure reducer、driver `Step` を functional core とし、
  runtime、gateway、CLI/API/pty/docker を imperative shell とする。
- 主要経路: pty→tap→OSC/prompt→runtime、app-server→stream demux→frame、DriverEvent→Step→View、
  daemon/server→WebSocket→TS codec/view、browser bootstrap→hydrate/palette/new-session、devcontainer→docker。
- 外部依存: Claude/Codex CLI・app-server、docker、real pty、Linear HTTP、git/gh CLI、browser、wall clock。
- 模範象限: `host/runtime/subsystem/stream`。次点は fakecodex/fakeclaude と devcontainer/fakedocker。
- 再入元: `plan-20260705-test-harness` は done。今回の入口は全 milestone の exit criteria 再監査である。

## 2. Tier マップ

| Tier | 該当 | 代表 / 命名 | 実行 | 観測時間 |
|---|---|---|---|---|
| T0 Pure | reducer、Driver.Step、parser、codec、registry conformance | `reduce_*_test.go`、`drivertest.Conformance` | 常時 `go test` / vitest | ms〜秒 |
| T1 Wired | `runtimetest.Harness`、stream fake、gateway scenario、browser fake backend | `runtime_harness_test.go`、`routing_wired_test.go` | 常時 CI | 秒 |
| T2 Contract | routing/fan-out/severance、wire invariant、stdlib fuzz | `*_contract_test.go`、`Fuzz*` | 常時 CI + race/fuzz job | 秒〜分 |
| T3 Fidelity | FakeVsReal Claude/Codex/docker/app-server | `//go:build e2e`、`FakeVsReal*` | opt-in + nightly | 分 |

規範は {% adr-ref id="adr-20260705-test-tier-taxonomy" /%}。real pty は安価かつ hermetic なため、
fake を持たず T2 で直接使う明文化済み例外である。

## 3. gap-matrix

| 経路 / 依存 | T0 | T1 | T2 | T3 | 備考 |
|---|---:|---:|---:|---:|---|
| state / driver→view | ◎ | ○ | ◎ | — | registry walk が新 driver を自動加入 |
| pty→tap→OSC/prompt | ○ | ◎ | ◎ | 例外 | real pty posture を ADR で許可 |
| app-server→stream routing | ○ | ◎ | ◎ | ◎ | 模範象限。recording も共有 |
| daemon/server→WS→TS wire | ○ | ◎ | ◎ | △ | fixture diff は強い。real browser/backend fidelity は非目標 |
| browser bootstrap/user flow | ○ | ◎ | ○ | ✗ | deterministic Playwright smoke。実 browser fidelity は未整備 |
| Claude/Codex CLI | ○ | ◎ | ◎ | ◎ | public fake + contract + FakeVsReal + recording |
| docker CLI | ○ | ◎ | ◎ | ◎ | PATH fake + lifecycle contract + real docker |
| Linear HTTP / git / gh / clock | △ | △ | △ | ✗ | seam/fake は一部あるが3点セットの横断監査未完 |

drift:

- 解消済み: FakeVsReal の実 trigger 0.142.5 に対し nightly が Codex 0.133.0 を使用していた。
- 残存: `check-e2e-siblings.sh` は always-on sibling の存在だけを検査し、public fake + 名指し contract +
  fidelity の3点セット自体は検査しない。
- 残存: 旧 `note-20260624-technical-harness-engineering-assessment` は 2026-05-31 snapshot であり、現況の
  test-harness assessment としては本書を優先する。

## 4. AI 事故リスク評価

| 事故 | 露出 | 現在の防御 | 次の防御 |
|---|---|---|---|
| fake drift / skip-green | 中 | FakeVsReal、nightly 全 suite 必須、失敗 issue | 3点セット registry/checker、case-level skip report |
| test・assertion 弱体化 | 高 | CI 再実行、review | protected/owned files と test-diff review gate |
| rule / coverage floor tampering | 中 | checker 自体の negative test、台帳コメント | floor 引下げ・CI gate削除の diff checker |
| 勝利宣言 | 低〜中 | CI必須 gate群 | required checks の外部設定監査、任意で Stop/pre-push gate |
| overfit / 検出力不足 | 中 | fuzz/property、一部 recording | critical path の differential mutation |
| skip / flaky 放置 | 中 | race/fuzz、nightly issue、一部 skip count pin | repo-wide skip inventory/期限、変更test repeat job |

## 5. 理想形 (north-star)

全主要伝搬経路と grandfather された外部依存が、模範象限と同じく「決定的な fake を使う T1、名前付き不変条件を
共有する T2、real と照合する T3」を持つ。registry/checker が新規依存と新実装を自動加入させ、fake drift、contract
欠落、case-level skip-green を静かに通さない。保存時の静的検査、pre-push の変更範囲 T0〜T2、PR CI の全
T0〜T2/race/fuzz/coverage/diff、nightly T3 が速度階層を作る。critical path は mutation score で「実行量」ではなく
「壊したときに検知できるか」を測り、test/lint/CI/floor の弱体化は差分 gate と ownership で人間レビューへ昇格する。

## 6. ロードマップ

| M | 対策 | exit criteria | 依存 | 規模 | handoff |
|---|---|---|---|---|---|
| M1 | Codex nightly version drift 修復 + static pin | nightly が 0.142.5 を installし、static test が後退を fail | — | 小 | 不要 |
| M2 | 外部依存 registry と3点セット checker | Linear/git/gh/clock/browserを含む依存台帳があり、各行に fake/contract/fidelity/例外が機械照合される | M1 | 大 | `/design`→`/implement` |
| M3 | skip/flaky 防御 | 全 skip に分類・理由があり、変更 test repeat job と case-level nightly skip report が CI artifact になる | M2 | 中 | 実装時 `/tdd` 相当 |
| M4 | anti-tampering gate | floor低下、gate削除、test削除が理由なしでは failし、protected/owned path が reviewを要求 | M2 | 大 | `/design`→`/implement` |
| M5 | mutation pilot | stream routing・state.Reduce・wire codec の変更差分で mutation score baseline/threshold を機械取得 | M3 | 中 | 実装時 `/tdd` 相当 |
| M6 | ローカル速度階層 | save/pre-push の決定的な検査が文書化され、CI と同じ command を再利用し、任意skipを可視化 | M3 | 中 | 実装時 `/tdd` 相当 |

## 7. enforcement 計画

| 規範 | 静的機構 | LLM 規範 | ADR pin |
|---|---|---|---|
| T3 は契約対象 version を走らせる | `static_enforcement_test.go` | 不要 | 既存 FakeVsReal ADR |
| 外部依存は3点セットか明示例外 | registry/checker + CI | 新依存時に正本台帳を更新 | taxonomy ADR の後続 ADR |
| skip-green を許さない | skip inventory + nightly report | fidelity失敗時は fake を直す | taxonomy ADR |
| gate/floor/test の弱体化を昇格 | diff checker + ownership | 変更理由と証拠を提出 | 新 anti-tampering ADR |
| 検出力を測る | differential mutation | coverageだけで完了扱いしない | mutation pilot ADR |

## 8. 規模判定と handoff

- M1: 小。workflow の version drift 1件と、その後退を防ぐ static assertion 1件。今回適用済み。
- M2: 大。複数外部依存の契約・例外・CIを横断するため、次は本書の gap-matrix と exit criteria を入力に
  `/design`→`/implement` へ渡す。
- M3/M5/M6: 中。M2 の台帳と分類が確定してから、各1系統ずつ実装する。
- M4: 大。GitHub branch protection/ownership を含む可能性があり、repo内権限だけでは完結しない。
- roadmap pin: 本書。観測日 2026-07-11。再入時は M1 の version pin と各 M の exit criteria を先に照合する。

## 9. 検証証拠

- docs: `docs lint` — 181文書時点で status ok、warning 0（本書追加後に再実行する）。
- Go主要象限: `go test ./gorules ./host/driver/... ./host/runtime/... ./platform/sandbox/devcontainer/... ./server/web/...` — pass。
- Web: `npm test -- --run` — 85 files / 1472 tests pass。`npm run build` — pass。
- 補足: Web test は `act(...)` warning を出すが exit 0。flaky/silent warning の将来 inventory 入力とする。


{% transition from="draft" to="published" date="2026-07-11" %}
test-harness再監査、M1修復、残roadmapを検証済み
{% /transition %}
