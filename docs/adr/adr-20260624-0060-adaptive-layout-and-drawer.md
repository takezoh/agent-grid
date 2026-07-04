---
id: adr-20260624-0060-adaptive-layout-and-drawer
kind: adr
title: ADR 0060 — 適応レイアウト (named grid areas + dvh + safe-area) + SessionDrawer 三重 guard
status: proposed
created: '2026-06-24'
updated: '2026-07-04'
tags:
- adr
- legacy-import
owners: []
relations: []
source_paths: []
decision_makers:
- unknown
---

<!-- migrated_from: docs/adr/0060-adaptive-layout-and-drawer.md -->

# ADR 0060 — 適応レイアウト (named grid areas + dvh + safe-area) + SessionDrawer 三重 guard

Status: Proposed

Related: [spec](../specs/2026-06-25-web-ui-redesign/spec.md), [plan](../specs/2026-06-25-web-ui-redesign/plan.md), [ux](../specs/2026-06-25-web-ui-redesign/ux.md), extends [0029](../adr/adr-20260624-0029-terminal-host-flex-height.md)
Related requirements: FR-LAYOUT-001, FR-LAYOUT-002, FR-LAYOUT-003, FR-LAYOUT-004, FR-DRAWER-001, FR-DRAWER-002, FR-DRAWER-003, FR-DRAWER-004, FR-DRAWER-005, FR-DRAWER-006, FR-DRAWER-007, FR-TERMINAL-001, FR-TERMINAL-002, FR-RUNSTATE-001

## Context

既存 App.tsx は grid-template-columns: 280px 1fr auto + auto-placement で、メディアクエリ 0 件・100vh 依存。.app-header セレクタは app.css に未定義で grid auto-placement 任せ。ux.md UAC-001/002/004/013/015 は『375px で sidebar off-canvas + ハンバーガー + 横スクロールなし + dvh + safe-area + scrim 配下 inert/aria-hidden で背後到達しない modal』を must-fail で要求。否定役は『inert または aria-hidden の OR では aria-hidden 単独実装が pointer-event 越しタップで通り抜ける』『breakpoint 跨ぎの連続リサイズで二重発火しうる』を指摘した。本 ADR は drawer を独立 ADR にせず適応レイアウトの帰結として内包する (最適化役案: drawer-適応レイアウトは同根判断)。

## Decision

(1) AppShell の grid-template-areas を 3 breakpoint で切替: <768 = 'banner' 'header' 'main' / 768-1024 = sidebar 折り畳み可 (icon-rail) / >=1024 = sidebar 常設。
(2) AppShell root を 100dvh + env(safe-area-inset-*) で算出 (100vh は使わない、body overflow は hidden に固定して二重スクロール無発生 invariant)。
(3) SessionDrawer の背後不可視化は『inert + aria-hidden='true' + pointer-events:none』の三重 guard を必須とする (OR 緩和を許さない)。inert は AT/keyboard、aria-hidden は VoiceOver rotor、pointer-events は scrim 越しタップを各々遮断する直交責務。
(4) drawer の取消クローズ経路は scrim click / Esc / 左→右スワイプ (touch 限定、SwipeHandler util で水平>=50 垂直<30) の 3 経路を全て同じ close ハンドラに流し、行 tap (= 選択クローズ) は previousActiveSessionId を渡す別ハンドラに流す。
(5) breakpoint 跨ぎ時の drawer close + focus 復帰は 50ms debounce で idempotent 化し、focus 復帰先は HamburgerToggle が visible ならそれ、hidden なら AppShell header 内の最初の focusable へ。
(6) drawer.open / previousActiveSessionId は Zustand store ではなく AppShell の useState で UI-local 保持 (store pure 不変条件維持)。

## Consequences

- positive: UAC-001/002/004/013/015 の must-fail counterexample を構造的に排除 (三重 guard / dvh / safe-area / 取消経路の 3 経路集約)。
- positive: store slice が 1 件削減 (DrawerStoreSlice 不要)。
- positive: breakpoint 跨ぎの連続リサイズ idempotency が debounce で保証される。
- negative: ADR-0029 (terminal-host flex height) との関係: 100vh → 100dvh の差替えは 0029 の本質 (flex:1 1 0 + min-height:0) を保つが、root 高さの計算式は変わる。本 ADR は 0029 を extends する位置づけ (frontmatter relations で明示)。
- negative: inert 属性は happy-dom が完全エミュレートしない可能性 (テストでは属性付与の有無 + aria-hidden + pointer-events を並列観察で代替)。
- neutral: ADR-0030 (keyed remount) / ADR-0034 (rAF coalesce refit) は dvh 移行で破れない (ResizeObserver が dvh 値変化を pick up し rAF で coalesce する経路は不変)。

## Alternatives Considered

### drawer を独立 ADR (0065 等) に分離

適応レイアウトの帰結であり 1 ADR の粒度として自然。最適化役案を採用。

### feature flag で drawer を段階導入

単一 chunk + CSP 制約と摩擦。token 置換 + Build sequence 順による visual 退行ゼロで十分。

### inert を採らず focus trap + aria-hidden のみ

UAC-002 counterexample (背後到達) + scrim 越しタップ を排除できない (否定役 blocker)。

### 100svh / 100lvh の単独使用

100dvh の連続値変動の方が ResizeObserver/rAF refit と整合する。100svh は最小固定で iOS Safari ツールバー出現時に terminal-host が縮まらない (UAC-015 拡大方向 fail の counterexample に近い)。
