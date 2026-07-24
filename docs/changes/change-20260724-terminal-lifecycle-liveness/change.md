---
id: change-20260724-terminal-lifecycle-liveness
kind: change
title: Terminal subscription authority and liveness
status: done
created: '2026-07-24'
profile: sdd@1
intent: Replace implicit imperative terminal lifecycle authority with one daemon RevisionOutcome
  owner, disjoint browser transport recovery, exact stale fencing, and bounded self-reporting
  stage evidence.
outcomes:
- Terminal presentation recovers from stalled work or delivery without page reload
  while preserving daemon-authored remote truth.
- Fresh replay cannot be affected by status or output from a replaced connection,
  owner, revision, or relay epoch.
- Low-rate and burst diagnostics remain bounded, terminal markers cannot overtake
  output, and diagnostic loss cannot hide itself.
scope:
- Simultaneously implemented current lifecycle v2 browser, gateway and daemon
- Daemon-only RevisionOutcome authority and accepted/waiting nonterminal states
- Browser connection-attempt delivery recovery with fresh generation/ticket/private-owner/clientRevision
  replay
- Public correlation to private owner mapping and old status/output fencing
- Producer-owned <=250ms/max4Hz diagnostics, finalSequence barrier, typed no_output/unknown
- SurfaceLease teardown, fixed resource bounds and T0-T3 verification
non_goals:
- Compatibility behavior outside the simultaneously implemented current lifecycle
  v2 endpoints
- Targeted unsubscribe, absence query or gateway speculative subscription state
- A second physical daemon IPC
- ANSI rendering semantics
change_classes:
- behavior
- responsibility
- boundary
- invariant
- capability
governance:
  gate: hard
  approval_evidence: ユーザー指示『全て approve して実装して』(2026-07-24)
  reasons:
  - Supersedes accepted terminal subscription ownership and wire decisions.
evidence_refs:
- type: test
  ref: go:test ./...
- type: test
  ref: go:vet ./...
- type: test
  ref: make:lint
- type: test
  ref: ui:test:unit-and-build
- type: test
  ref: playwright:test:e2e
- type: test
  ref: surfacelease:fake-vs-real
- type: test
  ref: docs:lint --conformance
members:
- role: requirements
  path: changes/change-20260724-terminal-lifecycle-liveness/requirements.md
  required: true
- role: implementation
  path: changes/change-20260724-terminal-lifecycle-liveness/implementation.md
  required: true
- role: verification
  path: changes/change-20260724-terminal-lifecycle-liveness/verification.md
  required: true
promotion:
- target: design-client
  section: invariants
  action: upsert
  item:
    id: INV-007
    statement: Browser publishes complete terminal intent and owns only connection-attempt
      TransportObservation; TerminalLifecycleActor exclusively owns lease, desired/applied
      state and RevisionOutcome; public correlation maps to a private gateway owner
      and old delivery is fenced.
    enforcement: contract
  reason: Authority and correlation boundaries must persist after closure.
- target: design-client
  section: failure_responsibilities
  action: upsert
  item:
    id: FAILURE-004
    statement: Browser delivery timeout replaces the connection without authoring
      remote outcome; producer telemetry is bounded to 250ms/4Hz and terminal status
      is ordered after finalSequence or explicit delivery_gap.
  reason: Recovery truth and diagnostic delivery responsibilities are structurally
    distinct.
unresolved_decisions: []
tags:
- terminal
- websocket
- liveness
- authority
- observability
owners:
- server/api
- host/runtime
- clients/ui
relations:
- {type: modifies, target: design-client}
- {type: references, target: adr-20260711-keep-single-ipc-connection-topology}
- {type: references, target: adr-20260711-server-initiated-severance-signal}
- {type: references, target: adr-20260711-extend-sever-not-drop-shared-ipc-hops}
- {type: references, target: adr-20260711-priority-lane-interactive-vs-bulk}
- {type: references, target: adr-20260715-geometry-bearing-terminal-attach}
- {type: references, target: adr-20260724-terminal-lifecycle-actor-boundary}
- {type: references, target: adr-20260724-terminal-lifecycle-bounds}
source_paths:
- clients/ui/src/socket
- src/server/api
- src/host/proto
- src/host/state
- src/host/runtime
- src/platform/termvt
summary: Unify daemon lifecycle truth, separate browser transport recovery, fence
  public/private namespaces, and make terminal diagnostics bounded and order-safe.
updated: '2026-07-24'
promotion_applied_at: '2026-07-24T05:39:38.779915+00:00'
closure:
  closed_at: '2026-07-24T05:39:43.519002+00:00'
  content_hash: sha256:3e242cf9d63f9e4c2dbfd496afb4f921acf6e9bb171a081a92756a7164125484
---

# Summary

The browser and daemon now have deliberately different state domains. The daemon actor alone writes authoritative `RevisionOutcome`; the browser writes only a connection-attempt `TransportObservation` and projects reconnecting while it obtains a fresh ticket, owner mapping, connection generation, and revision. Gateway correlation connects those domains without exposing private owner authority.

Stage diagnostics are producer-owned. Their single-writer dirty slots guarantee a first low-rate event within 250 milliseconds and at most 4 Hz under burst. Terminal/close status crosses an explicit `finalSequence` barrier and carries final watermark/drop evidence on the status lane; when that lane also fails the socket closes and recovery remains transport-unknown rather than fabricating a remote result.

This design targets the current lifecycle v2 browser and server implemented together. Endpoint-version compatibility is not specified by this change.

## Governance

Both new ADRs are accepted and the incompatible desired-reconcile ADR is superseded by explicit user approval. All implementation chunks and applicable fidelity gates are complete; the package is ready for immutable closure.


{% transition from="draft" to="ready" date="2026-07-24" %}
設計承認済み。current v2 implementation slice と機械検証を反映
{% /transition %}


{% transition from="ready" to="active" date="2026-07-24" %}
実装を開始し、gateway/data control separation と v2 lifecycle foundation を反映
{% /transition %}


{% transition from="active" to="closing" date="2026-07-24" %}
All implementation, verification, and applicable fidelity gates completed.
{% /transition %}
