---
id: adr-20260714-editor-audit-schema-additive-actor
kind: adr
title: Operator audit uses additive optional actor field on schema_version=2 (no version
  bump)
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
summary: Operator audit uses additive optional actor field on schema_version=2 (no
  version bump)
updated: '2026-07-14'
---

## Context

decision-point 'audit routing' は tool-log schema を v3 に bump するか additive field で拡張するかで、shipped v2 reader が operator record を skip して silent 消失するリスク vs 誤帰属リスクを持つ。issue-tool-log-schema-v3-reader-behavior-undefined を閉じる必要がある。

## Decision

**schema_version は 2 を維持**し、**optional `actor` field (default=agent)** を additive に追加する。新 reader は actor=operator を operator kind へ分類、shipped v2 reader は不明 field を無視し全て agent 帰属で表示継続 (silent skip なし)。rollout は writer → reader → UI の 3 段階、rollback は writer 停止のみで完結する。

## Consequences

- shipped v2 reader が v2 fixture を byte-identical に読み続ける (baseline verification)。
- operator record は shipped v2 reader からは agent として under-report される (silent skip でない、可視だが actor 誤分類)。この trade-off は accept する — silent 行消失より許容範囲。
- rollback path (writer emission 停止) は writer 単体の revert で完結する。

## Alternatives

- **却下: schema_version=3 bump** — shipped v2 reader が新 record を legacy-skip して operator save が rail から silent に消える。rollout evidence が閉じない。
- **却下: 完全別 operator-audit log channel** — turn-aggregation / latency-bound を重複実装することになり、agent/operator 間の latency/順序保証が食い違うリスクを持ち込む。

## Trace

- Requirements: FR-108
- Implementation contracts: contract-audit-schema-migration, contract-write-audit-trail
