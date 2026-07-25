---
id: change-20260725-claude-launch-mcp-boundary
kind: change
title: Replace managed Claude HOME overlay with launch-scoped MCP configuration
status: done
created: '2026-07-25'
profile: sdd@1
intent: Preserve the caller's process HOME while exposing frame-scoped agent_frames
  tools to host Claude sessions through Claude's explicit launch configuration boundary.
outcomes:
- Managed host Claude launches preserve HOME and all HOME-relative tool behavior.
- agent_frames remains frame-scoped and available through a launch-only MCP configuration.
scope:
- Host DirectDispatcher and DirectLauncher managed frame-messaging paths
- Obsolete HOME overlay and transcript-path compensation removal
- T0/T1 launch contracts and opt-in real-Claude T3 fidelity
non_goals:
- Changing container workspace MCP overlays or Codex dynamicTools exposure
- Writing user or project Claude configuration
change_classes:
- behavior
- responsibility
- boundary
- invariant
governance:
  gate: hard
  approval_evidence: 'User directed: 境界・構造を正しく修正し、ADRで固定して (2026-07-25).'
  reasons:
  - Supersedes an accepted Phase 1 decision that rejected per-launch Claude CLI MCP
    injection.
evidence_refs:
- type: test
  ref: go:test ./...
- type: test
  ref: go:vet ./...
- type: test
  ref: make:lint
- type: test
  ref: go:test -tags e2e TestFakeVsRealManagedClaudeMCPConfig
- type: test
  ref: docs:lint --conformance
members:
- role: requirements
  path: changes/change-20260725-claude-launch-mcp-boundary/requirements.md
  required: true
- role: implementation
  path: changes/change-20260725-claude-launch-mcp-boundary/implementation.md
  required: true
- role: verification
  path: changes/change-20260725-claude-launch-mcp-boundary/verification.md
  required: true
promotion:
- action: none
  reason: The durable boundary is fixed by the accepted superseding ADR; no broader
    platform or host design invariant needs modification.
unresolved_decisions: []
tags: []
owners: []
relations:
- {type: modifies, target: design-platform}
- {type: modifies, target: design-client}
source_paths:
- src/platform/agentlaunch/
- src/host/runtime/
- src/host/event/
- test-harness/
summary: Replace process-wide HOME redirection with a launch-scoped Claude MCP configuration
  while preserving frame-scoped authority.
updated: '2026-07-25'
promotion_applied_at: '2026-07-25T16:37:57.908904+00:00'
closure:
  closed_at: '2026-07-25T16:37:58.193267+00:00'
  content_hash: sha256:8be1144d84f817baec8f821fd6869ea8f6c4db4b51cfb057450b96596c5f4626
---

## Summary

Host Claude sessions previously replaced `HOME` with an agent-grid-owned temporary
directory so that one MCP server could be added to `~/.claude/settings.json`.
That process-wide substitution changed unrelated home-relative behavior and
required transcript-path compensation. This change moves MCP exposure to the
Claude invocation boundary and removes the obsolete filesystem and event
adaptation.

## Closure Notes

The container `.mcp.json` overlay and Codex `dynamicTools` paths are unchanged.
