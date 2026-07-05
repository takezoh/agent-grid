---
id: spec-20260705-test-harness
kind: spec
title: Robust test harness for event propagation, drivers, external-platform fidelity,
  and server-to-view
status: draft
created: '2026-07-05'
tags:
- testing
- harness
owners: []
functional_requirements:
- id: FR-001
  statement: システムのテストスイートは、すべてのテストを T0 (pure) / T1 (wired) / T2 (contract・fuzz)
    / T3 (fidelity) のいずれかの tier に分類できる状態を常に維持しなければならない
  priority: must
  rationale: 暗黙に存在する 4 層構造を命名・規範化し、新規テストの置き場所判断を機械化する
- id: FR-002
  statement: 新しい外部プラットフォーム依存 (CLI / daemon / protocol) が導入されたとき、システムは in-process
    fake・FakeVsReal e2e backstop・不変条件を名指しする contract test の 3 点を同時に備えなければならない
  priority: must
  rationale: adr-20260704-cli-fake-validated-by-real-cli-e2e の原則を claude/codex 固有から一般規範へ昇格する
- id: FR-003
  statement: frame の pty 出力に OSC / prompt シーケンスが含まれるとき、システムは当該 frame の FrameID
    を持つ EvFrameOsc / EvFramePrompt のみを runtime へ enqueue しなければならない
  priority: must
  rationale: 経路 A (pty→tap→OSC) は現状 end-to-end を駆動する口が無く、routing 正しさが pin されていない
- id: FR-004
  statement: surface subscriber が配送に追従できない間、システムは当該 subscriber を sever し、他の subscriber
    および他 session への配送順序を維持しなければならない
  priority: must
  rationale: TerminalRelay の severance 分岐は state.Reduce を通らない internal 経路で、現状駆動困難
- id: FR-005
  statement: システムは state.Register 済みのすべての driver に共通 conformance 契約 (Step 純粋性 /
    DriverEvent totality / Persist-Restore round-trip / metadata source priority) を適用しなければならない
  priority: must
  rationale: driver 個別テストは厚いが共通契約が暗黙で、新 driver への波及が保証されない
- id: FR-006
  statement: Go 側 wire encoder と TypeScript 側 codec は同一の golden fixture を消費しなければならない。もし
    fixture 再生成で差分が生じたなら、CI は fail しなければならない
  priority: must
  rationale: ADR 0021 の fixture 機構は未実装のまま手動同期が常態化しており、直近の model/effort 追加でも手動同期が発生した
- id: FR-007
  statement: システムは fake CLI を agent とした server→view 貫通シナリオ (session create → WS
    viewUpdate 受信) を常時 CI で検証しなければならない
  priority: must
  rationale: server=>view の伝搬を wire レベルで貫く常時テストが存在しない
- id: FR-008
  statement: devcontainer lifecycle は fake docker CLI に対して常時検証され、opt-in の real docker
    e2e が fake の忠実性を保証しなければならない
  priority: should
  rationale: docker は外部依存 4 種 (claude/codex/pty/docker) のうち唯一 fake も e2e も持たない
- id: FR-009
  statement: もし *_e2e_test.go および platform/lib 以外のコードが real CLI (claude / codex /
    docker) を exec するなら、lint は fail しなければならない
  priority: must
  rationale: 「T1/T2 は fake のみで走る」を review 依存でなくコンパイル時制約にする
- id: FR-010
  statement: T3 fidelity スイートは nightly で定期実行され、失敗は issue として可視化されなければならない
  priority: should
  rationale: fake PASS は real binary で verify しない限り証拠にならない。opt-in のみでは実行されない期間が生じる
- id: FR-011
  statement: T3 実行時に real CLI のイベント列を録音する場合、システムはその録音を fake preset との照合および T0 replay
    に再利用できなければならない
  priority: could
  rationale: fake 忠実性の保証を「人がテストを書く」から「記録が契約になる」へ移行する
- id: FR-012
  statement: 任意の event 列に対して、state.Reduce は panic せず大域不変条件 (HeadFrameID 整合 / MRU
    整合 / 入力 State の不変) を維持しなければならない
  priority: could
  rationale: 個別 reducer テストが到達しない event 順序の組合せを fuzz で覆う
non_functional_requirements:
- id: NFR-001
  type: maintainability
  criteria: 新 driver 追加時、conformance suite への加入に手作業を要しない (registry 走査で自動加入)
  measurement: driver 追加 PR に conformance 加入用の追加コードが不要であること
- id: NFR-002
  type: performance
  criteria: T0–T2 は network / API key / docker なしで `cd src && go test ./...` により完走する
  measurement: CI 標準 test ジョブ (API secret 非注入) の green
- id: NFR-003
  type: reliability
  criteria: T1/T2 の並行系 (client/runtime, platform/termvt, runtime/subsystem/stream)
    は -race で green
  measurement: CI race ジョブ
acceptance:
- id: AC-001
  given: state.Register 済みの全 driver
  when: drivertest.Conformance を registry 走査テストで実行する
  then: 全 driver が共通契約を pass する
  requirement_refs: [FR-005, NFR-001]
- id: AC-002
  given: Go wire encoder の型変更 (フィールド追加など)
  when: fixture を再生成せずに CI を走らせる
  then: fixtures diff gate が fail する
  requirement_refs: [FR-006]
- id: AC-003
  given: fakeclaude を agent CLI とした real server binary
  when: REST で session を create し WS で subscribe する
  then: driver View と一致する viewUpdate frame が subscriber に届く
  requirement_refs: [FR-007]
- id: AC-004
  given: real pty 上の frame
  when: OSC 0/2/9/133 シーケンスを pty へ書き込む
  then: 当該 FrameID を持つ EvFrameOsc / EvFramePrompt のみが enqueue され、event loop は生存し続ける
  requirement_refs: [FR-003]
- id: AC-005
  given: e2e build tag 外の Go ファイル
  when: exec.Command("docker") を追加して make lint を実行する
  then: lint が fail する
  requirement_refs: [FR-009]
- id: AC-006
  given: 容量 1 に注入した subscriber channel と受信停止した subscriber
  when: relay が fan-out を継続する
  then: 停止 subscriber は sever され、他 subscriber は全 event を順序どおり受信する
  requirement_refs: [FR-004]
relations:
- {type: implementedBy, target: plan-20260705-test-harness}
source_paths:
- src/client/runtime/
- src/client/driver/
- src/client/state/
- src/platform/agent/fakecodex/
- src/platform/lib/claude/fakeclaude/
- src/platform/sandbox/devcontainer/
- src/server/web/
- src/client/web/src/wire/
- src/gorules/
- scripts/check-coverage.sh
summary: 4 tier のテスト体系 (pure/wired/contract/fidelity) を正式化し、pty→OSC・surface relay・driver
  conformance・docker・Go↔TS wire の欠落象限を埋める
---

# Spec — Robust test harness

## Overview

agent-grid には既に一貫した世界観のテスト資産 — pure core テスト (`state.Reduce` / `Driver.Step`)、fake 注入
shell テスト、invariant contract (routing isolation / fan-out isolation)、fake-vs-real e2e backstop
(fakeclaude / fakecodex) — が存在する。本 spec はこの暗黙の 4 層を **T0 pure / T1 wired / T2 contract /
T3 fidelity** として正式化し、パターンから漏れている 4 つの象限を同じ構造で埋める:

1. **経路 A (pty → tap → OSC → EvFrameOsc)** — end-to-end を駆動する口が無い
2. **surface relay (TerminalRelay)** — severance 分岐が飽和を要して駆動困難
3. **docker (devcontainer)** — fake も real e2e も無い唯一の外部依存
4. **Go↔TS wire** — ADR 0021 の fixture 機構が未実装のまま手動同期が常態化

また保証構造を「fake は契約 (T2) で保証され、契約は e2e (T3) で保証される」という二層として静的制約 (lint / CI /
coverage floor) と LLM 制約 (AGENTS.md / enforcement note) の両方に pin する。

## Requirements

{% req id="FR-001" %}
**Tier 体系の常時維持** — テストスイートは全テストを T0 (pure: `Reduce`/`Step`/codec の純関数) / T1 (wired:
runtime loop + fake backend) / T2 (contract・fuzz: backend 非依存の不変条件 pin) / T3 (fidelity: fake vs
real、opt-in + nightly) のいずれかに分類できる状態を維持しなければならない。分類の正本は
adr-20260705-test-tier-taxonomy。
{% /req %}

{% req id="FR-002" %}
**外部依存 3 点セット** — 新しい外部プラットフォーム依存が導入されたとき、(a) public package 化した in-process
fake、(b) `FakeVsReal*` e2e backstop、(c) 不変条件を名指しする contract test、の 3 点を同時に備えなければならない。既存の
`thread/settings/updated` のように「production 処理はあるが fake が emit しない」状態はこの要件の違反例である。
{% /req %}

{% req id="FR-003" %}
**pty→OSC routing** — frame の pty 出力に OSC / prompt シーケンスが含まれるとき、当該 frame の FrameID
を持つ `EvFrameOsc` / `EvFramePrompt` のみを enqueue しなければならない。VT emulator の panic は封じ込め、event
loop を停止させてはならない。
{% /req %}

{% req id="FR-004" %}
**relay severance** — surface subscriber が配送に追従できない間、当該 subscriber を sever し、他 subscriber
と他 session への配送 (順序含む) を維持しなければならない。termvt の fan-out isolation と対になる relay 層の不変条件。
{% /req %}

{% req id="FR-005" %}
**driver conformance** — `state.Register` 済みの全 driver は共通契約 — Step 純粋性 (同一入力同一出力・prev
不変)、全 DriverEvent 種への totality、Persist/Restore round-trip、metadata source priority
(adr-20260705-metadata-source-priority: authoritative 確定後は fallback が上書き不可、tri-state 表現) —
を pass しなければならない。
{% /req %}

{% req id="FR-006" %}
**cross-language wire fixtures** — Go 側 wire encoder と TS 側 codec は同一の golden fixture
ファイルを消費しなければならない。fixture 再生成で差分が生じたなら CI は fail しなければならない (codex-schema-check と同方式)。
{% /req %}

{% req id="FR-007" %}
**server→view 貫通** — fake CLI (fakeclaude / fakecodex) を agent とした real server binary
に対する「session create → WS viewUpdate 受信」シナリオが常時 CI で検証されなければならない。
{% /req %}

{% req id="FR-008" %}
**docker fake + backstop** — devcontainer lifecycle (create → hook → run → remove) は PATH 注入の
fake docker CLI に対して常時検証され、opt-in real docker e2e が fake の canned 出力の忠実性を保証しなければならない。
{% /req %}

{% req id="FR-009" %}
**real binary の隔離** — `*_e2e_test.go` および `platform/lib`・fake package 以外のコードが real CLI
(claude / codex / docker) を exec するなら lint が fail しなければならない (ruleguard)。
{% /req %}

{% req id="FR-010" %}
**nightly fidelity** — T3 スイートは nightly workflow で定期実行され、失敗は issue 起票で可視化されなければならない。PR
CI には含めない (API spend の隔離、adr-20260704 の posture 維持)。
{% /req %}

{% req id="FR-011" %}
**録音駆動 fake** — T3 実行時に real CLI のイベント列を testdata へ録音し、fake preset との照合および T0 golden
replay に再利用できる。
{% /req %}

{% req id="FR-012" %}
**Reduce 大域不変条件の fuzz** — 任意の event 列に対して `state.Reduce` は panic せず、HeadFrameID
整合 / MRU 整合 / 入力 State の不変を維持しなければならない (stdlib fuzz)。
{% /req %}

{% req id="NFR-001" %}
新 driver 追加時、conformance suite への加入は registry 走査により自動で、手作業を要しない。
{% /req %}

{% req id="NFR-002" %}
T0–T2 は network / API key / docker なしで `cd src && go test ./...` により完走する。
{% /req %}

{% req id="NFR-003" %}
T1/T2 の並行系 (client/runtime, platform/termvt, runtime/subsystem/stream) は -race で green を維持する。
{% /req %}

## Acceptance Criteria

{% acceptance id="AC-001" %}
Given: `state.Register` 済みの全 driver。When: `drivertest.Conformance` を registry 走査テストで実行。Then:
全 driver が共通契約を pass する。
{% /acceptance %}

{% acceptance id="AC-002" %}
Given: Go wire encoder の型変更 (フィールド追加など)。When: fixture 未再生成のまま CI。Then: fixtures diff
gate が fail する。
{% /acceptance %}

{% acceptance id="AC-003" %}
Given: fakeclaude を agent CLI とした real server binary。When: REST で session create し WS で
subscribe。Then: driver View と一致する viewUpdate frame が届く。
{% /acceptance %}

{% acceptance id="AC-004" %}
Given: real pty 上の frame。When: OSC 0/2/9/133 を pty へ書き込む。Then: 当該 FrameID の `EvFrameOsc` /
`EvFramePrompt` のみが enqueue され、event loop は生存し続ける (`-race` 下)。
{% /acceptance %}

{% acceptance id="AC-005" %}
Given: e2e build tag 外の Go ファイル。When: `exec.Command("docker")` を追加して `make lint`。Then:
lint が fail する。
{% /acceptance %}

{% acceptance id="AC-006" %}
Given: 容量 1 に注入した subscriber channel と受信停止 subscriber。When: relay の fan-out 継続。Then:
停止 subscriber は sever され、他 subscriber は全 event を順序どおり受信する。
{% /acceptance %}

## Out of Scope

- **behavioral eval (T4)** — 駆動されたエージェントがタスクを完遂するかの成果 eval
  (note-20260624-technical-harness-engineering-assessment P1)。本 spec の T3 録音基盤 (FR-011) の上に将来構築できるが、別議題。
- **termvt への fake 導入** — real pty が安価かつ hermetic なため、termvt は fake を持たず T2 を real pty
  で走らせる現行 posture を維持する (adr-20260705-test-tier-taxonomy に明文化)。
- **playwright 等ブラウザ e2e の導入** — vitest + happy-dom で wire〜store まで届いており、現段階では費用対効果が薄い。
- **orchestrator 層の scheduler 系テスト強化** — 既に symphony-conformance で SPEC ↔ test 対応が正本化されており、本
  spec の対象外。
