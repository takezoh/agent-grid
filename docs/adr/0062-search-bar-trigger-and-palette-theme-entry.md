# ADR 0062 — Search-bar 風 palette trigger に New Session と Theme 選択を吸収 (プログレッシブ開示)

Status: Proposed

Related: [spec](../specs/2026-06-25-web-ui-redesign/spec.md), [plan](../specs/2026-06-25-web-ui-redesign/plan.md), [ux](../specs/2026-06-25-web-ui-redesign/ux.md)
Related requirements: FR-PALETTE-TRIGGER-001, FR-THEME-001, FR-THEME-002, FR-THEME-007

## Context

既存 header は Command (⌘K) + New Session の 2 ボタン構成で、テーマトグルは存在しない (ダーク固定)。ux.md は Raycast/Linear/Spotlight 風の search-bar-as-entry-point を modeled_on とし、New Session を palette 内 suggested action として吸収する案 (Open Q4) と、テーマトグルのスマホ配置を palette 内 entry に吸収する案 (Open Q3 の選択肢 C) を並べた。最適化役は『header の button 数を 1 にし breakpoint 跨ぎの UI 変動を最小化』『bottom-sheet 新規 UI を導入しない』を推奨した。

## Decision

(1) header に CommandSearchTrigger 1 個のみを置く (虫眼鏡 icon + 'Search commands…' placeholder + ⌘K/Ctrl+K hint badge を isMacPlatform で出し分け)。
(2) New Session を palette 内の suggested action (最上位 entry) として完全吸収し、header から独立 button を消す。
(3) ThemeSegmentedControl は >=768px では header 末尾に配置 (segmented control を表示)、<768px では palette 内 suggested action 'Theme: System / Light / Dark' に吸収し header からは消す。スマホ専用 bottom-sheet UI は新設しない。
(4) palette の hotkey (Cmd/Ctrl+K capture phase) と起動経路 (CommandSearchTrigger click) は同一 PaletteStoreSlice を経由するため等価。

## Consequences

- positive: header の button 数が breakpoint 不変で 1 個に固定され UI 変動が最小化。
- positive: bottom-sheet 等の新規 UI が不要で実装表面積が削減。
- positive: ux.md Open Q3 と Q4 が同時に決着し、Open Questions リストが圧縮される。
- negative: 新規ユーザーは New Session を発見しづらい (palette を開く 1 手数が必要) → empty state ヒント (『Sessions are in the menu ☰』『Press ⌘K to start a new session』) で補強する。
- negative: スマホでテーマを切替えるには palette を 1 回開く必要があり、PC/タブレットより手数が増える。

## Alternatives Considered

### header に Command + New Session の 2 ボタンを残す (legacy 継承)

breakpoint 跨ぎで UI 変動が大きく、palette を全画面の正典 entry にする reference_ux と矛盾。

### search-bar 右端に '+' icon で New Session を分離

視覚的に search-bar の純度が落ち、palette 内 suggested action と二重導線になる。

### テーマトグルのスマホ配置で bottom-sheet 新規 UI を導入 (Open Q3 案 B)

新規 UI の実装/テスト負荷 + palette との視覚的一貫性から外れる。

### テーマトグルを header に常時 segmented control (Open Q3 案 A)

スマホ狭幅で header 横幅を圧迫し CommandSearchTrigger の幅が削られる。
