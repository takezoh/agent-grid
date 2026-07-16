---
id: adr-20260624-0047-palette-disabledreason-single-source
kind: adr
title: ADR 0047 — push 可否判定と送信前再検証は ToolDef.disabledReason(daemonSnapshot) の 1 関数に集約する
status: accepted
created: '2026-06-24'
updated: '2026-07-04'
tags:
- adr
- legacy-import
owners: []
relations:
- {type: references, target: change-20260624-2026-06-24-web-ui-command-palette}
source_paths:
- src/server/web/
decision_makers:
- unknown
summary: ScopeSegment の disabled 表示と submit pre-check で同じ判定ロジック (activeSessionID 有無
  + ActiveOccupant === 'frame') を 2 箇所に書くと DRY が崩れ、片方更新で挙動差が出る。
---

<!-- migrated_from: docs/adr/0047-palette-disabledreason-single-source.md -->

# ADR 0047 — push 可否判定と送信前再検証は ToolDef.disabledReason(daemonSnapshot) の 1 関数に集約する

Status: Accepted

Related: [spec](../specs/2026-06-24-web-ui-command-palette/spec.md), [plan](../specs/2026-06-24-web-ui-command-palette/plan.md)
Related requirements: FR-004, FR-005, FR-006, FR-023

## Context

ScopeSegment の disabled 表示と submit pre-check で同じ判定ロジック (activeSessionID 有無 + ActiveOccupant === 'frame') を 2 箇所に書くと DRY が崩れ、片方更新で挙動差が出る。

## Decision

ToolRegistry の各 ToolDef に disabledReason(daemonSnapshot): string | null を持たせ、ScopeSegment は null/非null で disabled + サブテキストを描画、ToolDef.submit のラッパ (palette store の submit action) は HTTP 発行直前に同関数を再呼び出して null でなければエラー toast + close する。

## Consequences

- **positive**: 失効再検証ロジックを 1 箇所に集約 (DRY)
- **positive**: テストが disabledReason 関数の単体 1 本で carry-over される
- **positive**: 将来 scope を追加する際もこの規約に乗るだけで UI を触らなくてよい
- neutral: daemonSnapshot 型を ToolDef に晒すため store/daemon の export 整理が必要

## Alternatives Considered

### ScopeSegment と submit pre-check で別個にチェック

却下: DRY 違反と分散更新リスク

### ScopeSegment subscribe で常時監視し失効瞬間にパレットを閉じる

却下: user 入力途中で勝手に閉じる UX 退行 (Q7 検討時に却下済)
