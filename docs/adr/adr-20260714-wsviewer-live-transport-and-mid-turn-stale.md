---
id: adr-20260714-wsviewer-live-transport-and-mid-turn-stale
kind: adr
title: ViewUpdate WS extension carries activity rows and mid-turn stale signals with
  a 750 ms end-to-end latency bound
status: accepted
created: '2026-07-14'
decision_makers:
- Takehito Gondo
tags:
- workspace-viewer
- design
owners: []
relations:
- {type: partOf, target: plan-20260714-agent-workspace-viewer}
source_paths: []
summary: ViewUpdate WS extension carries activity rows and mid-turn stale signals
  with a 750 ms end-to-end latency bound
---

## Context

exp-live-background forbids silent stale and requires background rail updates while a drawer is open (UAC-007, UAC-009). Turn-completion-driven aggregation alone leaves a mid-turn edit invisible until Stop fires (critique issue-stale-detection-latency-vs-turn-aggregation); polling with a large interval trivially passes 'not queued' while failing UAC-009 (critique issue-live-background-transport-non-discriminating).

## Decision

Extend the existing per-session ViewUpdate WS broadcast (adr-20260624-0023 payload, adr-20260705 sessions-only scoping) with an `activity_events` array carrying two message types: (a) `turn_row` on turn completion (the aggregated row shown in the rail) and (b) `mid_turn_touch` on every PostToolUse whose classified path matches any file currently open in an active drawer of the same session. The reader emits (b) synchronously with the PostToolUse hook, without waiting for Stop. End-to-end latency (JSONL append to browser render) is bounded at 750 ms p95 (activity row) and at 500 ms p95 (mid-turn touch -> stale banner render). A polling implementation whose interval exceeds these bounds is forbidden by the contract. Store side coalesces multiple mid_turn_touch events for the same open drawer into one stale transition (respecting adr-20260624-0057 single aria-live slot); one AT announcement per transition.

## Consequences

- Mid-turn edit is visible as stale within 500 ms; no silent-stale window remains between the mutation and turn completion.
- Latency bounds turn 'not queued' into a falsifiable proposition — a naive 5 s poll fails the T1 timing test.
- Store-side coalescing prevents flooding the single aria-live slot even under rapid repeated background edits.
- Reusing ViewUpdate's session-scoping guarantee prevents cross-session leak.

## Alternatives

- **却下: Dedicated per-session WS channel** — Duplicates reconnect/backfill (adr-20260624-0025 already solved that once for transcripts) without a latency benefit.
- **却下: Short-interval REST poll** — Adds request volume proportional to open sessions and cannot beat WS latency for exp-live-background; would need per-poll cost budget.
- **却下: Wait for turn completion to signal stale** — Leaves the mid-turn edit silently stale — exp-live-background silent-stale prohibition violation.

## Trace

- Requirements: FR-005, FR-006, FR-008
- Implementation contracts: contract-live-background-transport, contract-stale-banner-presentation
