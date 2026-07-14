---
id: adr-20260714-wsviewer-workspace-root-handle
kind: adr
title: Drawer-scoped WorkspaceRootHandle pins workspace root for drawer lifetime
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
summary: Drawer-scoped WorkspaceRootHandle pins workspace root for drawer lifetime
---

## Context

ux assumes 1 session = 1 workspace directory, but Session.Frames []SessionFrame lets push-driver create multiple frames with different LaunchPlan.StartDir values. Per-request resolution against the live frame stack (as drafted) contradicts the promised drawer-lifetime consistency observable (critique issue-workspace-root-drawer-lifetime-consistency). The SSOT triple (LaunchPlan.StartDir / DEvWorktreeResolved.WorktreeStartDir / Session.Project) had to be spelled out (critique issue-workspace-root-field-imprecise).

## Decision

On drawer open, the browser calls a new REST endpoint that resolves the workspace root once using the SSOT triple: prefer DEvWorktreeResolved.WorktreeStartDir for the currently-active frame's LaunchPlan when a managed worktree exists, else LaunchPlan.StartDir, else Session.Project. The response carries (session_id, frame_generation, resolved_root_path) which becomes a client-owned WorkspaceRootHandle. All subsequent tree/file/diff requests from the same drawer session send that handle; server refuses if frame_generation is stale (the drawer surfaces the workspace-torn-down banner rather than resolving against the new root). Head-frame drift after drawer open never re-resolves the drawer's root.

## Consequences

- Contract-workspace-root-resolution's drawer-lifetime observable is achievable: all requests share the same snapshotted root.
- Frame push/pop while a drawer is open produces a visible 'root changed' degradation rather than silently mixing roots across requests.
- One additional round-trip at drawer open; the extra request is a single GET returning a small handle payload.

## Alternatives

- **却下: Resolve per-request against the current head frame** — Silently switches root under an open drawer; contradicts the contract's expected observable.
- **却下: Root-frame StartDir always** — Ignores push-driver semantics; the pushed frame might legitimately be a different workspace (e.g. sub-agent worktree).

## Trace

- Requirements: FR-028, FR-018
- Implementation contracts: contract-workspace-root-resolution
