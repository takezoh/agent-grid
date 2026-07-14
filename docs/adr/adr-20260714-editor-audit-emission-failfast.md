---
id: adr-20260714-editor-audit-emission-failfast
kind: adr
title: Audit emission failure escalates to typed 5xx (no silent under-report)
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
summary: Audit emission failure escalates to typed 5xx (no silent under-report)
updated: '2026-07-14'
---

## Context

draft は 'emission 経路失敗時は phantom row を出さないため under-report は意図的' としていたが、これは contract-write-audit-trail の completeness invariant を自壊させる (issue-audit-emission-silent-under-report)。silent 状態を残さないためには save の観察と audit の観察を一致させる必要がある。

## Decision

operator save 成功後に tool-log writer への append が失敗した場合、handler は **typed 5xx audit_emit_failed** を返す。client は dirty buffer を保持し retry surface を提供する。これは failure_semantics.requirement_effect=degrades (audit invariant を明示的に犠牲にする path) だが、silent under-report より可視化を優先する。

## Consequences

- operator は失敗を必ず知覚できる (silent 状態が存在しない)。
- disk-side は既に rename 完了しているため、client 側 retry で idempotent な二度書きが起きる可能性がある — この trade-off は clipboard export と warning UI で運用回避する。
- verify-write-audit-trail は 5xx path も含めた fixture matrix で検証される。

## Alternatives

- **却下: silent under-report を意図的 design として容認** — contract-write-audit-trail の completeness invariant を自壊させる。

## Trace

- Requirements: FR-108
- Implementation contracts: contract-write-audit-trail
