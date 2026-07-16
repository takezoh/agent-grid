---
id: adr-20260714-wsviewer-path-guard-symlink
kind: adr
title: Per-segment symlink evaluation for workspace path traversal defense
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
summary: Per-segment symlink evaluation for workspace path traversal defense
---

## Context

The path-guard defends the fs boundary. ADR-0026's session-ID allowlist regex applies to closed-alphabet IDs; file paths are open-alphabet. The critique attacked leaving symlink policy as an implementation detail (issue-symlink-resolution-policy-open): final-only EvalSymlinks is defeated by an intermediate directory symlink chained with ../.

## Decision

The guard MUST (a) reject any client path containing '..' or absolute segments before any fs call, (b) join the workspace root with the sanitized relative path, (c) call filepath.EvalSymlinks on the full joined path, and (d) verify the fully-resolved path is a descendant of the fully-resolved workspace root (both roots EvalSymlinks-normalized) — else reject. Rejection is uniform (404 not-found) to avoid confirming presence outside root.

## Consequences

- Intermediate-symlink escape via link-to-outside/../../etc/passwd is closed (the ../ segments are refused up front; even a bare link-to-outside is caught by the descendant check on the fully-evaluated path).
- Per-request EvalSymlinks costs one stat walk per served request; workspace-tree walks amortize this by caching the resolved root inside the WorkspaceRootHandle.

## Alternatives

- **却下: Final-only EvalSymlinks** — Defeated by an intermediate directory symlink whose target is outside root followed by a legitimate relative walk.
- **却下: Reject symlinks entirely** — Blocks common legitimate cases (e.g. node_modules with symlinked local packages) that live inside the resolved root.

## Trace

- Requirements: FR-029, FR-018
- Implementation contracts: contract-workspace-path-traversal-defense
