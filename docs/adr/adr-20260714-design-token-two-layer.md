---
id: adr-20260714-design-token-two-layer
kind: adr
title: デザイントークンの 2 層化 (primitive + semantic)
status: proposed
created: '2026-07-14'
decision_makers:
- Takehito Gondo
tags:
- adr
- web
- ui-refresh
owners: []
relations:
- {type: references, target: change-20260714-web-ui-refresh}
source_paths:
- src/client/web/src/css/tokens.css
summary: primitive スケールを下層に敷き semantic は alias に限定。status 色 1 系統化、色リテラルは tokens.css
  以外で禁止 (静的ガード)。
---

## Context

`tokens.css` (ADR-0059 由来) は意味名付き直値の羅列 (40+ 色) で、スケールが存在しない。status 色が 2 系統 (`--status-*` / `--session-status-*`) 重複し、palette 専用 alias 群が分岐、色相も不揃い (無彩 #1e1e1e と紫寄り #1e1e2e の同居)。spacing / type / radius / motion / font のスケールも未定義で、コンポーネント CSS には色直値が漏れている (ActivityRail のライトテーマ非追従の温床)。`tokens-css-structure.test.ts` がトークン構造を検証している。

## Decision

primitive スケール (色 ramp / `--space-1..8` / `--text-xs..xl` / `--radius-1..3, full` / `--motion-1..3` / `--font-ui`, `--font-mono`) を下層に敷き、semantic 層は「primitive への alias」に限定する。既存 semantic 名は互換 alias として維持し、コンポーネント CSS の移行は m2 以降のマイルストーンで段階的に行う。status 色は `--status-*` に統合し、`--session-status-*` は消費側 (StatusIcon / SessionList) ごと同一チャンクで移行する。色リテラルは `tokens.css` 以外で禁止し、静的ガードテストで強制する (FR-006)。

## Consequences

- (+) 色・余白・字級の追加は必ず primitive 経由になり、場当たり値の再増殖を静的ガードが防ぐ。
- (+) ライトテーマ・reduced-motion の品質が「トークンを差し替えれば全コンポーネントに波及する」構造で担保される。
- (-) `tokens-css-structure.test.ts` は 2 層構造検証への書き直しが必要。`--session-status-*` 削除は breaking のため消費側移行と同一タスクで行う (handler signature 変更の単一タスク原則に準ずる)。
- (0) 既存 semantic 名の互換 alias は移行完了後に整理する余地が残る (負債として plan 側で追跡)。

## Alternatives

- **却下: フラットな直値トークン継続 + 値だけ差し替え** — 最小 diff だが、スケール不在という構造問題が残り、次の変更でまた場当たり値が増える。構造の正しさを優先する。
- **却下: Tailwind theme への移行** — スケールは得られるが全コンポーネントの class 書き換えが必要で、目的 (見た目刷新) に対して工数過大。既存テスト資産へのリスクも大きい。将来の別判断は妨げない。

## Trace

- Requirements: FR-001..FR-007, NFR-001, NFR-005
- Confirmation: `src/__tests__/no-color-literals.test.ts` (tokens.css 以外の色リテラル検出) + `tokens-css-structure.test.ts` (2 層構造検証)
