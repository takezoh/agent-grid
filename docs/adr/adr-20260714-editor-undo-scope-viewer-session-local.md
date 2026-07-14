---
id: adr-20260714-editor-undo-scope-viewer-session-local
kind: adr
title: Undo/redo scope is viewer-session-local (strict clear on reload/conflict accept)
status: accepted
created: '2026-07-14'
decision_makers:
- Takehito Gondo
tags:
- workspace-editor
- design
owners: []
relations:
- {type: partOf, target: plan-20260714-agent-workspace-editor}
source_paths: []
summary: Undo/redo scope is viewer-session-local (strict clear on reload/conflict
  accept)
updated: '2026-07-14'
---

## Context

decision-point 'undo/redo history scope' は viewer-session-local / session-global / persistent の 3 option で observable 挙動が根本的に変わる (reload 後の u の意味など)。issue-undo-scope-classification-contradiction により、これは implementation_detail から design_choice に格上げされる。

## Decision

**viewer-session-local**: undo/redo stack は現在の buffer instance に完結し、buffer close と reload / conflict-resolution acceptance で strict にクリアされる。reload 後の u は空 stack で no-op になる (silent replay を排除する)。unnamed register は drawer-session-local (同一 drawer 内で複数 buffer 共有、drawer close で破棄)。

## Consequences

- vim 標準の per-buffer undo model に忠実で最も単純。
- reload/conflict accept 後の silent replay failure mode が構造的に排除される。
- cross-tab / persistent 拡張は future-work として open questions に残す (現段階では要求が無い)。

## Alternatives

- **却下: session-global** — 要求されていない utility のために複雑度を増す。
- **却下: persistent** — client storage と鮮度問題を持ち込むが requirement に無い。

## Trace

- Requirements: FR-104
- Implementation contracts: contract-vim-undo-locality
