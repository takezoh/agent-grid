---
id: adr-20260714-editor-aria-live-precedence
kind: adr
title: Single aria-live slot precedence is conflict > stale > close-warning > dirty
status: accepted
created: '2026-07-14'
decision_makers:
- Takehito Gondo
tags:
- workspace-editor
- design
owners: []
relations:
- {type: partOf, target: change-20260714-agent-workspace-editor}
source_paths: []
summary: Single aria-live slot precedence is conflict > stale > close-warning > dirty
updated: '2026-07-14'
---

## Context

issue-aria-live-slot-coalescing-collision が指摘した通り、既存 single aria-live slot (adr-20260624-0057) に dirty / stale / close-warning / conflict の 4 種が同時に発火し得る。one-transition-one-announcement を保つには precedence が要る。

## Decision

single aria-live slot に対する announcement precedence を **conflict > stale > close-warning > dirty** で確定する。同一 transition で複数種が発火した場合は最上位のみを announce する。

## Consequences

- adr-20260714-wsviewer-live-transport-and-mid-turn-stale の 1-transition-1-announcement invariant が新 UI 追加後も成立する。
- 重畳時の AT announcement flood が構造的に排除される。

## Alternatives

- **却下: 全て announce** — AT announcement flood — screen reader user への実質的な障壁になる。
- **却下: 別 slot を新設** — 既存 single-slot invariant (adr-20260624-0057) を破る。

## Trace

- Requirements: FR-105, FR-107
- Implementation contracts: contract-dirty-state-visibility, contract-write-conflict-detection
