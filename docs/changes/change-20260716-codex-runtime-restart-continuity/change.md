---
id: change-20260716-codex-runtime-restart-continuity
kind: change
title: Codex runtime restart continuity
status: draft
created: '2026-07-16'
updated: '2026-07-16'
summary: Runtime shutdown を commit-first transaction とし、意図的 teardown 後も Codex session identity と conversation locator を復元する。
profile: sdd@1
intent: Preserve Web-visible Codex session identity, status, and durable conversation locator across intentional runtime server restarts without hiding unexpected app-server failures.
outcomes:
- Intentional runtime teardown cannot delete or stop a committed Codex session.
- A subsequent boot restores the same SessionID, FrameID, and Codex thread locator and exposes the frame as nonStopped after observer Ready.
- Expected subsystem termination and unexpected process exit have one race-safe classification contract.
scope:
- Runtime shutdown transaction, acknowledged all-session Save barrier, terminal ownership, and event admission
- Subsystem StopCause and stream app-server termination linearization
- Existing Codex cold-start recovery through the session-per-file store, ThreadID/RolloutPath, RecreateAll, PrepareLaunch, and observer Ready
- T0-T3 verification, including Web gateway observation and fake-versus-real restart/resume fidelity
non_goals:
- Preserving a live PTY process across daemon replacement
- Exactly-once continuation of an in-flight Codex turn
- Automatic restart after an arbitrary app-server crash
- Changing the session-per-file store format or the per-session app-server topology
- Resolving the separate signal-time container preserve versus destroy policy in this change
change_classes:
- behavior
- responsibility
- boundary
- invariant
- internal_design
governance:
  gate: soft
  reasons:
  - Crosses runtime state, persistence, subsystem/process, coordinator, and Web-observable boundaries.
  - Three proposed ADRs require user review; signal-time container compatibility remains an explicit decision.
members:
- role: requirements
  path: changes/change-20260716-codex-runtime-restart-continuity/requirements.md
  required: true
- role: implementation
  path: changes/change-20260716-codex-runtime-restart-continuity/implementation.md
  required: true
- role: verification
  path: changes/change-20260716-codex-runtime-restart-continuity/verification.md
  required: true
promotion:
- target: design-client
  section: invariants
  action: upsert
  item:
    id: codex-runtime-restart-continuity
    statement: A successful intentional runtime shutdown completes every per-session atomic upsert in the pre-teardown all-session Save barrier; teardown events cannot mutate session identity, and the next boot reconstructs the same Codex session/frame/conversation identity.
  reason: Restart continuity is a stable client-runtime invariant after the proposed ADRs are accepted and implementation is verified.
- target: design-client
  section: failure_responsibilities
  action: upsert
  item:
    id: subsystem-terminal-observation
    statement: The subsystem boundary classifies the first terminal observation; expected StopCause is neutral and an unexpected Wait is an observable frame failure without session deletion.
  reason: Expected teardown must not be reported as an app-server failure, while real failures must remain visible.
- target: design-client
  section: compatibility_policies
  action: upsert
  item:
    id: restart-continuity-axes
    statement: Restart compatibility is evaluated separately for identity/status, container adoption, and PTY/turn continuity; this change guarantees identity/status and conversation-locator continuity only.
  reason: Promotion is deferred until the container-preserve conflict is resolved with the user.
unresolved_decisions:
- Accept, reject, or revise the three proposed ADRs after design review.
- For SIGINT/SIGTERM, retain the documented container-preserve policy or promote the current graceful-destroy behavior; restart identity continuity is required under either choice.
relations:
- {type: references, target: design-client}
- {type: references, target: adr-20260716-codex-observer-subscription-ready-ownership}
source_paths:
- src/client/state/state.go
- src/client/state/event.go
- src/client/state/reduce.go
- src/client/state/reduce_lifecycle.go
- src/client/runtime/runtime.go
- src/client/runtime/interpret.go
- src/client/runtime/persist.go
- src/client/runtime/subsystem/subsystem.go
- src/client/runtime/subsystem/stream/backend.go
- src/client/runtime/bootstrap.go
- src/client/runtime/bootstrap_coldstart.go
- src/cmd/server/coordinator.go
- src/server/web/mux_scenario_test.go
---

# Codex runtime restart continuity

This change package is the design source for the reported Web UI observation: after a runtime server restart, a previously running Codex Driver session is absent or Stopped. It defines a restart transaction and recovery contract; it does not silently redefine the unresolved signal-time container policy.
