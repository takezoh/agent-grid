---
id: adr-20260711-test-harness-dependency-admission
kind: adr
title: Define external dependency admission as executable triples
status: accepted
created: '2026-07-11'
decision_makers:
- unknown
tags: []
owners: []
relations:
- {type: partOf, target: plan-20260711-test-harness-north-star}
- {type: references, target: spec-20260711-test-harness-north-star}
source_paths: []
summary: JSON registryとGo validatorで外部依存tripleを意味契約まで機械検証する
updated: '2026-07-11'
---

## Context

既存 taxonomy は外部依存に public in-process fake、名前付き T2 invariant contract、同一入力を real と比較する T3 fidelity を要求し、real pty だけを例外とする。既存 checker は sibling の存在を確認するが、空 fake や観測不能な contract でも名前だけ整えば通り得る。

## Decision

stdlib で読める checked-in JSON registry を SSOT とし、Go validator が各 row を意味契約まで検査することにする。pty 以外は triple 必須とし、例外機構を一般化しない。将来 pty 以外を免除する場合は taxonomy ADR を supersede する別 ADR を先に承認する。

## Consequences

### Positive
{% consequence kind="positive" %}新外部依存が自動加入し、空 fake、観測不能 contract、same-input 比較のない fidelity を negative fixture で拒否できる。pure validator は T0、filesystem 結線は T1、負例は T2、既存 FakeVsReal は T3 で検証できる。{% /consequence %}

### Negative
{% consequence kind="negative" %}外部依存追加時に registry と triple を同時更新するコストが発生する。{% /consequence %}

### Neutral
{% consequence kind="neutral" %}既存 fake/FakeVsReal/contract は再実装せず参照する。wire 形式は JSON、validator は Go stdlib のみを使う。{% /consequence %}

## Alternatives

**Go source registry。** 型安全だが非 Go tooling から読みにくいため却下する。

**TSV + shell。** 例外と evidence の構造、精密な診断、負例 fixture が弱いため却下する。

**一般的な期限付き例外。** accepted taxonomy と衝突し triple 規律を希釈するため却下する。


{% transition from="proposed" to="accepted" date="2026-07-11" %}
独立Verify 3項目pass、実装指示に基づき承認
{% /transition %}
