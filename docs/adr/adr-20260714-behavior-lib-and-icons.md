---
id: adr-20260714-behavior-lib-and-icons
kind: adr
title: 挙動ライブラリの限定導入 (Radix menu/tooltip) とアイコン方針 (lucide 静的 SVG)
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
- {type: references, target: spec-20260714-web-ui-refresh}
source_paths:
- src/client/web/package.json
summary: 新規挙動 2 種のみ Radix に委譲、既存の自前挙動は温存。アイコンは lucide SVG を静的コピーし runtime 依存なし。
---

## Context

自前の挙動実装 (focus trap / listbox / dialog / tabs) はテスト済みで健在。一方、新 UI はこれまで存在しなかった DropdownMenu (ヘッダ overflow メニュー、テーマ切替の格納先) と Tooltip (アイコンボタン化の対) を必要とする。両者はポインタ/キーボード/dismiss/positioning の組み合わせで正しく作るのが最も難しい部類の挙動。AGENTS.md はライブラリ選定時の候補比較を要求する (wire/persistence の stdlib-only 制約は UI には非適用)。アイコンは現状 ☰ ▸ ⏺ 等の文字記号頼みで、線幅の揃ったセットが無い。

## Decision

新規挙動 2 種のみライブラリに委譲する: `@radix-ui/react-dropdown-menu` と `@radix-ui/react-tooltip` (いずれも headless、スタイルはトークン CSS で当てる)。既存の自前挙動は置き換えない。アイコンは lucide (ISC license) の SVG を `src/components/icons/` に静的コピーし、自前 `<Icon>` コンポーネントで統一する (runtime 依存なし、線幅 1.5 固定)。ライセンス表記は `icons/README.md` に残す。

## Consequences

- (+) メニュー/ツールチップの a11y (フォーカス管理・dismiss 規則・aria-describedby) を実績あるライブラリに委譲でき、自前実装の工数とリスクを回避。
- (+) アイコンが runtime 依存ゼロ・tree-shaking 問題なしで統一される (NFR-002: バンドル増 gzip 10KB 以下)。
- (-) runtime 依存が 2 パッケージ増える。バージョン追従の保守対象が増える。
- (0) Tooltip は `aria-label` 規約と併用する (tooltip はホバー時の視覚提供、aria-label が常時の SoT)。

## Alternatives

- **却下: メニュー/ツールチップも自前実装** — focus trap 資産はあるが、メニューの型付き positioning・サブメニュー・dismiss 規則は工数と a11y リスクが大きい。既存資産の再利用では届かない部分。
- **却下: Base UI (@base-ui-components/react)** — 設計は近代的だが 1.0 到達直後でエコシステム実績が Radix に劣る。パッケージ分割粒度も Radix が細かい。将来の再評価は可。
- **却下: react-icons / icon font** — バンドル肥大とスタイル不統一。静的 SVG コピーで足りる。

## Trace

- Requirements: FR-011, FR-012, FR-024, FR-034 (tooltip/menu 消費箇所), NFR-002
- PR 要件: AGENTS.md の Library Selection 手順に従い、本比較を導入 PR に転記する
