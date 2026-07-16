---
id: adr-20260624-0036-palette-2phase-store-architecture
kind: adr
title: ADR 0036 — コマンドパレットを Zustand 純粋 store + ToolDef 宣言レジストリで構築する
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
summary: 2 フェーズパレット (toolSelect → paramSelect) を Web に実装するにあたり、phase / scope / paramValues
  / submitting / error / opener を散らさず 1 箇所に集約する必要がある。同時に I/O (HTTP / store/daemon
  read) と UI state 遷移を混ぜると test 容易性が落ちる。
---

<!-- migrated_from: docs/adr/0036-palette-2phase-store-architecture.md -->

# ADR 0036 — コマンドパレットを Zustand 純粋 store + ToolDef 宣言レジストリで構築する

Status: Accepted

Related: [spec](../specs/2026-06-24-web-ui-command-palette/spec.md), [plan](../specs/2026-06-24-web-ui-command-palette/plan.md)
Related requirements: FR-007, FR-008, FR-009, FR-010, FR-011, FR-019, FR-020

## Context

2 フェーズパレット (toolSelect → paramSelect) を Web に実装するにあたり、phase / scope / paramValues / submitting / error / opener を散らさず 1 箇所に集約する必要がある。同時に I/O (HTTP / store/daemon read) と UI state 遷移を混ぜると test 容易性が落ちる。

## Decision

palette store は DOM 操作と HTTP 呼び出しを持たない純粋 state + actions のみとし、I/O は ToolRegistry の ToolDef.submit(ctx) に局所化する。ctx = {http, daemon snapshot, notify, store actions} を DI で渡し、test では fake ctx を注入する。CommandPalette は DOM 副作用 (opener 記録 / blur / focus 復帰) の単一所有者となる。

## Consequences

- **positive**: store のテストが jsdom 依存なしの純粋 reducer テストになる
- **positive**: ToolDef 差し替えで standard / push / 将来の新 scope を等価に検証できる
- **positive**: ADR-0030 (TerminalPane subscribe 唯一所有) を保ちやすい (store が DOM/WS を触らない)
- **negative**: ToolDef.submit に置く ctx の型設計を初手で固める必要がある

## Alternatives Considered

### store/palette に HTTP fetch を直接書く

却下: I/O 結合で test が重くなり、push 失効再検証ロジックが分散する

### ToolDef を持たず CommandPalette 内で switch (toolId) する

却下: 新 scope 追加で UI コンポーネントを毎回触ることになり高凝集を失う
