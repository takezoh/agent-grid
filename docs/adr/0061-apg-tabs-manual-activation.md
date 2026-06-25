# ADR 0061 — MainTabs を WAI-ARIA APG Tabs Pattern (roving tabindex + manual activation) で再実装

Status: Proposed

Related: [spec](../specs/2026-06-25-web-ui-redesign/spec.md), [plan](../specs/2026-06-25-web-ui-redesign/plan.md), [ux](../specs/2026-06-25-web-ui-redesign/ux.md)
Related requirements: FR-TABS-001, FR-TABS-002, FR-TABS-003

## Context

既存 MainTabs は role='tab'/tablist/tabpanel を持つが onKeyDown 0 件 / roving tabindex 未実装で、矢印キーでのタブ移動が不可 (UAC-014 vs_legacy: must-fail)。terminal は常時 mount + CSS 可視切替で scrollback を保持する (legacy で成立)。ux.md UAC-014 の When は『ArrowRight で TRANSCRIPT へ移し Space/Enter で activate』と明示で manual activation を指す。否定役は『automatic activation だと Space/Enter が冗長 + TERMINAL タブが focus 移動だけで onChange 系副作用が走る』を指摘した。

## Decision

(1) MainTabs を APG Tabs Pattern の manual activation 流儀で再実装する。ArrowLeft/Right で focus 移動のみ、Space/Enter で activate (aria-selected 遷移)、Home/End で先頭/末尾 focus 移動。
(2) roving tabindex: aria-selected='true' の tab だけ tabindex='0'、他は '-1'。
(3) terminal の常時 mount + CSS 可視切替 (display または visibility) は維持し scrollback 保持 (FR-TABS-003)。
(4) palette の hotkey 規律 (Cmd/Ctrl+K capture phase) と直交する (tablist focus 時のみ Arrow 系を消費)。

## Consequences

- positive: UAC-014 counterexample (onKeyDown 0 件 legacy) を構造的に fail させる。
- positive: keyboard 一貫性が palette と tablist で統一される (manual activation 流儀)。
- positive: terminal の常時 mount は変わらず ADR-0030 (keyed remount) と整合。
- negative: 既存テスト (MainTabs.test.tsx) の onClick assertion を Space/Enter assertion に更新する必要がある。
- neutral: tabpanel の aria-labelledby は既存実装で成立しているため変更不要。

## Alternatives Considered

### automatic activation (focus 移動 = activate)

UAC-014 の When が Space/Enter で activate を明示しており literal 不整合。TERMINAL タブで focus 移動だけで subscribe 副作用が走るリスク。

### focus 移動と activate を Tab/Space 両方で

APG 推奨流儀から外れ、テストの判別性が落ちる。
