---
id: change-20260727-multi-server-session-palette
kind: change
title: Multi-server session palette
status: active
created: '2026-07-27'
profile: sdd@1
intent: 既存のtyped launch palette/profile契約を保持しつつ、全serverの全sessionを一つのapplication-scoped fleetとして全viewへ同一に投影する。
outcomes:
- 既存のfleet project/profile paletteとserver-owned create semanticsを維持する
- Workspace/Webの全viewが全serverの同じsession fleet revisionを観測する
- New Sessionは作成先serverに依存せずauthoritative fleetへ反映される
- session選択はserverIdをwindow/process境界にせず同一window内のguarded transactionで切り替わる
scope:
- existing multi-server launch palette/profile contracts
- application-scoped Web/Workspace fleet session runtime
- compound session routing and server-neutral Workspace views
- partial failure, create observation, bounded publication and cross-host conformance
non_goals:
- arbitrary shell command editor or client interpolation
- runtime servers.json reload
- per-window or renderer-owned fleet authority
- compatibility fallback for currentServerId/raw sessionId/server-specific window routing
change_classes:
- behavior
- responsibility
- boundary
- invariant
governance:
  gate: hard
  approval_evidence: User directed「進めて」and explicitly required removal of all legacy command definitions, compatibility
    paths, and fallbacks (2026-07-27).
  reasons:
  - multi-server credential and routing authority moves to the desktop main-process boundary
  - session creation wire/config contracts are intentionally breaking
members:
- role: requirements
  path: changes/change-20260727-multi-server-session-palette/requirements.md
  required: true
- role: implementation
  path: changes/change-20260727-multi-server-session-palette/implementation.md
  required: true
- role: verification
  path: changes/change-20260727-multi-server-session-palette/verification.md
  required: true
- role: ux
  path: changes/change-20260727-multi-server-session-palette/ux.md
  required: false
promotion:
- target: design-client
  section: invariants
  action: upsert
  item:
    id: INV-MULTI-SERVER-IDENTITY
    statement: Project and session identity remains server-qualified from discovery through active context and every
      routed operation.
- target: design-client
  section: invariants
  action: upsert
  item:
    id: INV-FLEET-VIEW-CONSISTENCY
    statement: Each Web/Workspace host process has one configured fleet-session authority and every live view observes
      the same monotonic fleet revisions.
- target: design-client
  section: boundaries.provides
  action: upsert
  item:
    id: BOUNDARY-FLEET-AUTH-PROXY
    statement: Host-owned Workspace/Web runtimes retain routing credentials and realize one shared credential-free
      fleet protocol; renderer and window state are projections only.
- target: design-client
  section: capabilities
  action: upsert
  item:
    id: CAPABILITY-TYPED-LAUNCH-PROFILE
    statement: A versioned server-owned launch profile validates typed values and materializes shell-free executable/argv.
- target: design-client
  section: capabilities
  action: upsert
  item:
    id: CAPABILITY-FLEET-SESSION-RUNTIME
    statement: All configured servers contribute authoritative session lifecycle to one bounded application fleet
      with compound routing and scoped failure.
unresolved_decisions: []
tags: []
owners: []
relations:
- type: modifies
  target: design-client
- type: conformsTo
  target: design-platform
source_paths: []
methodology: sdd
summary: Typed launch palette/profileを保ち、Web/Workspaceのsession lifecycleを一つのapplication-scoped fleet authorityへ統合する。
updated: '2026-07-27'
---

# Multi-server session palette and fleet runtime

The change keeps the accepted typed launch profile, palette form and server-owned materialization outcomes. This revision closes the implementation responsibility gap proven by the A/B reproduction: fleet config/create was host-wide while live sessions still came from one renderer-owned current-server connection.

The governing revision is [design-plan.json](design-plan.json). It assigns fleet-session semantics to one shared protocol, realizes it once in each Web/Workspace host process, and makes every SPA view a credential-free projection with view-local guarded selection.
