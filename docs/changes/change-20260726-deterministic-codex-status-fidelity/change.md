---
id: change-20260726-deterministic-codex-status-fidelity
kind: change
title: Make Codex status fidelity deterministic
status: done
created: '2026-07-26'
profile: sdd@1
intent: Replace the nondeterministic real-model approval T3 with deterministic fake
  wiring and contract coverage while retaining real app-server lifecycle fidelity.
outcomes:
- Status-critical server requests are exercised through the real Backend UDS/WebSocket
  path without model choice.
- Real Codex fidelity remains limited to deterministic protocol and lifecycle scenarios.
scope:
- src/host/runtime/subsystem/stream routing tests
- Codex external-dependency test-tier documentation
non_goals:
- Production status semantics or UI changes
- Running a model solely to induce a particular tool choice
change_classes:
- internal_design
- implementation_only
governance:
  gate: auto
  reasons: []
evidence_refs:
- type: test
  ref: go:test ./host/runtime/subsystem/stream/... ./platform/agent/fakecodex/...
    ./server/api/...
- type: test
  ref: clients/ui:PW_USE_SYSTEM_CHROME=1 npm run test:e2e:real
- type: test
  ref: dev-docs:lint --conformance
members:
- role: requirements
  path: changes/change-20260726-deterministic-codex-status-fidelity/requirements.md
  required: true
- role: implementation
  path: changes/change-20260726-deterministic-codex-status-fidelity/implementation.md
  required: true
- role: verification
  path: changes/change-20260726-deterministic-codex-status-fidelity/verification.md
  required: true
promotion:
- target: none
  action: none
  reason: Existing testing design already assigns deterministic wiring to T1/T2 and
    real dependency drift to T3; this change corrects test placement.
unresolved_decisions: []
tags:
- testing
- codex
- status
owners: []
relations:
- {type: originatedFrom, target: change-20260726-uihost-server-status-e2e}
source_paths:
- src/host/runtime/subsystem/stream/routing_wired_test.go
- src/host/runtime/subsystem/stream/routing_e2e_test.go
- docs/note/note-20260624-agent-testing.md
summary: Replace model-driven approval T3 with deterministic wired fake coverage and
  retain real Codex protocol fidelity.
updated: '2026-07-26'
promotion_applied_at: '2026-07-26T11:24:38.995091+00:00'
closure:
  closed_at: '2026-07-26T11:24:43.259276+00:00'
  content_hash: sha256:b84a53b041db4d6be4b26c21966becf1157fccddeabb6c60b74e5f659f2557fc
---

## Summary

The removed T3 called `BindFrame` but discarded its returned launch plan, so no
Codex CLI, thread, or turn was started. Even with that omission repaired, asking
a live model to choose a shell command is not a deterministic protocol fidelity
test. The always-on wired fake owns approval delivery; existing real app-server
tests own deterministic method and lifecycle compatibility.

## Closure Notes
