---
id: adr-20260624-0039-palette-focus-trap-minimal
kind: adr
title: ADR 0039 — focus trap は 30 行未満の自前フックに留め、Esc / opener 復帰は store.close() に統一する
status: accepted
created: '2026-06-24'
updated: '2026-07-04'
tags:
- adr
- legacy-import
owners: []
relations:
- {type: references, target: change-20260624-2026-06-24-web-ui-command-palette}
source_paths: []
decision_makers:
- unknown
summary: focus-trap-react など外部ライブラリは shadow DOM / portal の境界バグや bundle 増を招きやすい。一方、自前で大規模な
  trap を書くと境界バグの保守を抱える。
---

<!-- migrated_from: docs/adr/0039-palette-focus-trap-minimal.md -->

# ADR 0039 — focus trap は 30 行未満の自前フックに留め、Esc / opener 復帰は store.close() に統一する

Status: Accepted

Related: [spec](../changes/change-20260624-2026-06-24-web-ui-command-palette/requirements.md), [plan](../changes/change-20260624-2026-06-24-web-ui-command-palette/implementation.md)
Related requirements: FR-003, FR-016, FR-017, FR-018

## Context

focus-trap-react など外部ライブラリは shadow DOM / portal の境界バグや bundle 増を招きやすい。一方、自前で大規模な trap を書くと境界バグの保守を抱える。

## Decision

hooks/useFocusTrap は『modal 内の最初/最後の tabbable で Tab/Shift+Tab を循環させる』だけの極小フック (~30 行) とし、Esc 処理と opener 復帰は store.close() (CommandPalette の unmount 経由) に集約する。useFocusTrap は store を import しない。

## Consequences

- **positive**: opener 復帰経路が 1 つに収束 (× ボタン / Esc / 外側クリックの全てが close → unmount → opener.focus())
- **positive**: 依存追加ゼロ、~30 行で完結
- **negative**: shadow DOM 内コンポーネントが将来出てきた場合に追加対応が必要

## Alternatives Considered

### focus-trap-react を採用

却下: bundle 増 + 既存依存に無い

### trap を全く実装しない

却下: a11y 要件 (キーボードのみで完結) を満たせない
