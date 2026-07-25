---
id: adr-20260725-vibe-island-asis-contract-pin
kind: adr
title: Vibe Island human-input 契約は検証層のみで pin する
status: accepted
created: '2026-07-25'
summary: as-is 体験契約の固定は production code を変更せず、EARS + implementation contract + 既存テストへの
  witness 帰着 (欠落 assert の追加のみ) で行う
decision_makers:
- agent-grid-maintainers
tags:
- native-clients
- windows-shell
- vibe-island
owners:
- agent-grid-maintainers
relations:
- {type: originatedFrom, target: change-20260725-vibe-island-human-input-contract}
source_paths: []
consequences:
  positive:
  - 後続の pending replay / 選択肢質問の実装が、退行を機械検出できる契約ベースラインの上で行える
  - blast radius がテストと docs に閉じ、契約化そのものにリスクがない
  negative:
  - 既存実装に設計上の歪みがあってもこの change では直せず、契約が現状を固定化する
  neutral:
  - 契約の変更は後続 change が EARS/witness を明示的に書き換えることで行う (暗黙の挙動変更は witness が検出する)
confirmation: git diff で production code (clients/windows-shell の非 Tests プロジェクト・src/)
  に変更が無いこと。変更が必要になった場合は本 ADR の前提が崩れるため change を中断して再設計する
updated: '2026-07-25'
---

# Vibe Island human-input 契約は検証層のみで pin する

## Context

実装調査 (2026-07-25) で、Vibe Island の承認/質問ハンドリング (gateway 往復・自動展開・楽観反映・単一解決・トースト・静穏) は一貫した挙動を持つことを確認したが、その体験は暗黙契約であり、退行を機械検出できる正本が無かった。ux.md (change-20260725-vibe-island-human-input-contract, specify-r1) が体験契約を 14 UAC + counterexample で固定したため、これを技術契約へ翻訳する必要がある。同時に、pending replay と選択肢付き質問の導入が user consultation で決定済み (slice-gap-closure) であり、大きな実装変更が近い。

## Decision

本 change では production code を一切変更せず、(1) EARS 機能要件、(2) 3 つの implementation contract (往復 / 注意観察 / カードライフサイクル) による decision closure、(3) witness の既存テスト資産への帰着と欠落 assert のテスト追加のみで as-is 契約を pin する。テスト有効化のための seam 追加が必要になった時点で本 change を中断し、再設計する。

## Consequences

frontmatter `consequences` (positive / negative / neutral) が SoT。

## Alternatives

- **replay/選択肢の実装と同時に契約化する** — 却下: 実装変更と契約 pin が混ざると、テストの失敗が退行なのか意図変更なのか判別できず、as-is ベースラインの意味が失われる。
- **UI e2e の全面新設で pin する** — 却下: 契約の大半は既に T0 純粋層テストで判別可能に固定されており、e2e 全面新設は重複・flake・実行コストを増やすだけで判別力を追加しない。
