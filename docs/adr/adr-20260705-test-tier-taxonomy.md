---
id: adr-20260705-test-tier-taxonomy
kind: adr
title: Test tier taxonomy (T0-T3) and the external-dependency triple
status: proposed
created: '2026-07-05'
decision_makers:
- Takehito Gondo
tags:
- testing
- harness
owners: []
relations:
- {type: partOf, target: plan-20260705-test-harness}
- {type: references, target: adr-20260704-cli-fake-validated-by-real-cli-e2e}
- {type: referencedBy, target: note-20260624-agent-testing}
- {type: referencedBy, target: note-20260624-technical-code-enforcement}
- {type: references, target: note-20260624-agent-testing}
- {type: references, target: note-20260624-technical-code-enforcement}
source_paths:
- docs/note/note-20260624-agent-testing.md
- src/platform/lib/claude/fakeclaude/
- src/platform/agent/fakecodex/
- Makefile
- .github/workflows/ci.yml
summary: テストを T0 pure / T1 wired / T2 contract / T3 fidelity に正式階層化し、外部依存導入時の fake
  + FakeVsReal e2e + contract の 3 点セットを規範化する
---

# Test tier taxonomy (T0-T3) and the external-dependency triple

## Context

{% context %}
リポジトリのテスト資産は既に一貫した 4 層 — 純関数テスト、fake 注入 shell テスト、不変条件 contract
(routing isolation / fan-out isolation)、fake-vs-real e2e backstop — を形成しているが、この構造は
命名されておらず、新しいテストの置き場所・新しい外部依存に要求される保証セットが暗黙である。実害として、
`thread/settings/updated` (commit 71d05a4) は production 処理と in-package unit test を持つ一方、
fakecodex / stream fake のどちらも emit せず、fidelity backstop の保証外に置かれた。

adr-20260624-0002 と adr-20260704-cli-fake-validated-by-real-cli-e2e は「fake の忠実性は opt-in
real e2e が保証する」原則を stream backend / claude / codex に個別確立したが、外部依存一般への規範には
なっていない。coverage tier (S〜D, note-20260624-agent-testing) は「どれだけ覆うか」を規定するが、
「どの種類のテストで覆うか」は規定しない。
{% /context %}

## Decision

{% decision %}
テストを次の 4 tier に正式分類し、以後この語彙を docs / レビュー / AGENTS.md で用いることにする:

| Tier | 名称 | 対象 | 実行 |
|---|---|---|---|
| T0 | Pure | `state.Reduce` / `Driver.Step` / `View` / codec 等の純関数 | 常時 (`go test`) |
| T1 | Wired | runtime loop + fake backend / subsystem の伝搬 | 常時 |
| T2 | Contract / Fuzz | backend 非依存の不変条件 pin | 常時 + CI fuzz job |
| T3 | Fidelity | fake vs real (claude / codex / pty / docker) | opt-in (`-tags e2e`) + nightly |

あわせて以下を規範とする:

1. **外部依存 3 点セット** — 新しい外部プラットフォーム依存の導入 PR は (a) public package の
   in-process fake、(b) `FakeVsReal*` e2e backstop、(c) 不変条件を名指しする contract test を同梱する。
   既存依存へ新しいイベント / メソッドの処理を足す変更も、fake の emit 能力と backstop の照合対象を同時に拡張する。
2. **fix the fake** — T3 が落ちたら直すのは fake であって assertion ではない (adr-20260704 の原則を全依存へ拡張)。
3. **T1/T2 は fake のみで走る** — real binary の exec は `platform/lib` / fake package /
   `*_e2e_test.go` に隔離し、ruleguard で強制する。
4. **T3 の実行 posture** — PR CI には含めず (API spend / 環境依存の隔離)、`go vet -tags e2e` の
   compile check + nightly workflow での定期実行 + 失敗の issue 起票とする。
5. **pty 例外** — termvt は real pty が安価かつ hermetic なため fake を持たず、T2 contract を real pty
   で走らせる。3 点セットの (a)(b) を免除される唯一の外部依存であり、免除には本 ADR のような明文を要する。
{% /decision %}

## Consequences

{% consequence kind="positive" %}
テストの置き場所と保証セットの判断が機械化され、fake drift (settings/updated 型の欠落) が構造的に検出される。
レビューと LLM への指示が「T2 に置け」「3 点セットが無い」という短い語彙で成立する。
{% /consequence %}

{% consequence kind="negative" %}
外部依存の導入コストが上がる (fake + backstop + contract の同梱)。小さな依存追加でも 3 点セットの
免除判断を ADR で明文化する手間が生じる。
{% /consequence %}

{% consequence kind="neutral" %}
coverage tier (S〜D) とは直交する分類として併存する。floors は「量」、本 taxonomy は「種類」を規定する。
{% /consequence %}

## Alternatives

- **coverage floor のみで品質を担保する** — 却下。floor は fake に対する PASS も同価に数えるため、
  「fake が本物に似ているか」という直交する欠陥を検出できない。
- **T3 を PR CI で必須にする** — 却下。API spend・credential・外部サービス可用性が PR velocity に
  直結し、flaky gate 化する。nightly + issue 起票で「定期的に真実と突き合わせる」を達成する。
- **mock ライブラリ (gomock 等) による per-test mock** — 却下。プロジェクトは手書きの高忠実 fake +
  実プロトコル round-trip を採用済みで、per-test mock は wire 忠実性の保証と両立しない。
