---
id: adr-20260714-editor-conflict-ui-nonblocking
kind: adr
title: Conflict UI is a non-blocking banner with keep-mine / take-theirs / merge
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
summary: Conflict UI is a non-blocking banner with keep-mine / take-theirs / merge
updated: '2026-07-14'
---

## Context

decision-point 'conflict UI modality' は non-blocking banner / auto-diff resolver / blocking modal の 3 option を持つ。exp-live-background との整合、および viewer の既存 stale-banner 前例からの継続性を評価する必要がある。

## Decision

**Non-blocking banner + keep-mine/take-theirs/merge 3 択ボタン** を確定する。operator は決定中も編集を継続でき、既存 stale-banner 前例と integrateし、single aria-live slot の precedence rule (conflict > stale > close-warning > dirty) 下で 1 transition 1 announcement を保つ。

## Consequences

- exp-live-background (agent の進行で operator を締め出さない) と整合する。
- auto-diff resolver 相当の判断は operator 側に委譲される (システムが賢く merge しない)。
- blocking modal による反射的 dismiss リスクを回避する。

## Alternatives

- **却下: auto-diff resolver** — '曖昧' の定義と merge algorithm を新規に導入する必要があり、exp-write-race-recovery が要求するのは operator による選択 (システムの推論ではない)。
- **却下: blocking modal** — 反射的 dismiss リスク + exp-live-background との緊張。

## Trace

- Requirements: FR-105
- Implementation contracts: contract-write-conflict-detection
