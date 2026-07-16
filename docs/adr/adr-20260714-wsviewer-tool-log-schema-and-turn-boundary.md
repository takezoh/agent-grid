---
id: adr-20260714-wsviewer-tool-log-schema-and-turn-boundary
kind: adr
title: Tool-log schema versioning, reader classification, and Claude Stop-family turn
  boundary
status: accepted
created: '2026-07-14'
decision_makers:
- Takehito Gondo
tags:
- workspace-viewer
- design
owners: []
relations:
- {type: partOf, target: change-20260714-agent-workspace-viewer}
source_paths: []
summary: Tool-log schema versioning, reader classification, and Claude Stop-family
  turn boundary
---

## Context

Draft misclassified the tool-log reader as an existing component (critique issue-tool-log-reader-misclassified-existing). The schema extension (turn_id + normalized file-event kind) must land beside on-disk lines already written by shipped daemons without the extended fields (critique issue-tool-log-schema-migration-unaddressed). The Claude driver has no native turn id; Stop, StopFailure, SubagentStart, SubagentStop hooks each carry different semantics (critique issue-claude-turn-boundary-stop-hook-ambiguity), and overlapping PreToolUse spans could misgroup.

## Decision

(1) Split component-tool-log-activity-source into component-tool-log-writer (existing sink) and component-tool-log-reader (new tailer/classifier). (2) Bump the JSONL schema with a top-level schema_version field (int, starts at 2; legacy entries lack it). The reader treats missing/lower schema_version as 'legacy': it emits neither an activity row nor a mid-turn signal for legacy entries and increments a named diagnostic counter (tool_log_legacy_skipped_total). Silent field synthesis is forbidden. (3) Claude turn boundary: increment the synthesized turn counter on Stop and StopFailure (both terminate a turn; StopFailure additionally marks the turn's terminal aggregated row as failure-terminated). SubagentStart begins a nested sub-turn (rows for the sub-agent's tool calls belong to a distinct nested turn id derived from the parent); SubagentStop closes the sub-turn. User-cancel maps to StopFailure. Overlapping PreToolUse spans within one turn belong to that same turn; a PreToolUse arriving after Stop but before the next PreToolUse of the new turn belongs to the new turn.

## Consequences

- Reader is a first-class component with owner, test seams, and its own contract obligations.
- Mixed old/new JSONL on first upgrade is deterministic (skip legacy with counter, never fabricate).
- Claude turn aggregation matches Codex semantics for the common case; every Stop-family variant has a defined behavior; UAC-002 can be asserted per variant.
- SubagentStart/Stop pairs let sub-agent rows drill-down without leaking into the parent turn's row count.

## Alternatives

- **却下: Silent default classification of legacy lines** — Fabricates fields the SSOT does not have — exactly what ARCHITECTURE.md forbids as 'no fabricated fallbacks'.
- **却下: Increment turn counter only on Stop, ignore StopFailure/SubagentStop** — Leaves failure-terminated turns visually indistinguishable from successful ones and misgroups sub-agent rows into the parent turn.
- **却下: Every PostToolUse is its own row for Claude** — Structurally breaks UAC-002 turn aggregation for one of the two supported agents.

## Trace

- Requirements: FR-002, FR-003, FR-015
- Implementation contracts: contract-activity-event-source, contract-turn-aggregation
