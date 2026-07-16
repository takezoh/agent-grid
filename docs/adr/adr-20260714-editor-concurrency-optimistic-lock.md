---
id: adr-20260714-editor-concurrency-optimistic-lock
kind: adr
title: Concurrency policy for operator/agent write race is optimistic lock via If-Unmodified-Since
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
summary: Concurrency policy for operator/agent write race is optimistic lock via If-Unmodified-Since
updated: '2026-07-14'
---

## Context

operator dirty buffer と agent 背後 write の race は decision-input-concurrency-policy-candidates の 3 option (LWW / optimistic lock / operator-priority) で解決策が分かれる。LWW は silent overwrite の温床 (exp-write-race-recovery 単独違反)、operator-priority は agent 進行を締め出して exp-live-background と緊張。

## Decision

Concurrency policy を **optimistic lock**: buffer 開時 mtime/ETag を snapshot し、:w 時 If-Unmodified-Since header で precondition。server 側 precondition_failed → 412 を返し、client の conflict detector が 3-way (keep-mine / take-theirs / merge) UI を trigger する。

## Consequences

- silent overwrite は server-side precondition で機械的に阻止される。
- conflict UI は client 側の完全な責務として明示的な選択を強制する。
- background agent progress は保留されず、exp-live-background invariant を保つ。

## Alternatives

- **却下: last-write-wins** — conflict-detector UI と組み合わせない限り silent overwrite が構造的に発生する。
- **却下: operator-priority (dirty 中は agent write 保留)** — exp-live-background と直接緊張する; agent の可視な進行が operator の drawer にブロックされ得る。

## Trace

- Requirements: FR-105, FR-112
- Implementation contracts: contract-write-persistence-save, contract-write-conflict-detection
