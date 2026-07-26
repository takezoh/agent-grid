---
id: change-20260726-retire-nondriving-stream-t3
kind: change
title: Retire non-driving stream T3
status: done
created: '2026-07-26'
profile: sdd@1
intent: Remove the remaining stream T3 that never executes the launch plan returned
  by BindFrame.
summary: Retire a false-green stream T3 and retain real Codex lifecycle fidelity in
  fakecodex.
outcomes:
- No stream T3 claims to exercise real turns without launching the Codex CLI.
scope:
- src/host/runtime/subsystem/stream/routing_e2e_test.go
non_goals:
- Changing production routing
change_classes:
- implementation_only
governance:
  gate: auto
  reasons: []
evidence_refs:
- type: test
  ref: go:test ./host/runtime/subsystem/stream/... ./platform/agent/fakecodex/...
members:
- role: requirements
  path: changes/change-20260726-retire-nondriving-stream-t3/requirements.md
  required: true
- role: implementation
  path: changes/change-20260726-retire-nondriving-stream-t3/implementation.md
  required: true
- role: verification
  path: changes/change-20260726-retire-nondriving-stream-t3/verification.md
  required: true
promotion:
- target: none
  action: none
  reason: The durable deterministic-fidelity rule was recorded by the originating
    change.
unresolved_decisions: []
tags: []
owners: []
relations:
- {type: originatedFrom, target: change-20260726-deterministic-codex-status-fidelity}
source_paths:
- src/host/runtime/subsystem/stream/routing_e2e_test.go
updated: '2026-07-26'
promotion_applied_at: '2026-07-26T11:26:38.220991+00:00'
closure:
  closed_at: '2026-07-26T11:26:42.154361+00:00'
  content_hash: sha256:bf094e3887a0cc2527fa2cd186f09ab1e292213c56be7ebdcf59cc044d340512
---

## Summary

`TestStreamRoutingE2EIsolation` discarded `BindFrame`'s returned launch plan,
so it started neither a CLI nor a thread/turn. Real app-server initialization,
method-set, lifecycle, and recorded fixture fidelity remain owned by
`platform/agent/fakecodex`.

## Closure Notes
