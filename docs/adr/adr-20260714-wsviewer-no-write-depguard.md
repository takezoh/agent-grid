---
id: adr-20260714-wsviewer-no-write-depguard
kind: adr
title: depguard rule structurally forbids fs-mutating calls in the workspace-viewer
  handler package
status: superseded
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
summary: depguard rule structurally forbids fs-mutating calls in the workspace-viewer
  handler package
updated: '2026-07-14'
---

## Context

cc-no-write must be structural (not a UI toggle); the draft relied on grep-review at PR time (critique issue-no-write-boundary-not-structural). AGENTS.md rules require lint/compile-time enforcement. Integrator judgment (per user guidance): standalone ADR analogous to adr-20260624-0016.

## Decision

Add a depguard rule in src/.golangci.yml scoped to the workspace-viewer handler package (path: src/server/web/workspace*.go) that denies imports of os.WriteFile, os.Create, os.OpenFile with any write flag, os.Remove, os.RemoveAll, os.Rename, os.Chmod, os.Mkdir, os.MkdirAll, and io.Copy/io.CopyBuffer whose destination is not restricted to an internal buffer type. The rule ships together with the handlers in the same milestone. A synthetic-mutation regression test (a temporary CI check that patches in a forbidden call and asserts `make lint` fails) validates enforcement.

## Consequences

- A future PR introducing os.WriteFile in workspace.go fails `make lint`; cc-no-write is enforced at build time.
- Refactors that legitimately need any of the listed calls in the workspace-viewer package require the depguard exception to be explicitly narrowed — the review conversation cannot happen silently.

## Alternatives

- **却下: Manual grep review at PR time (draft position)** — cc-no-write must be structural per constraint definition; manual review is not a structural gate.
- **却下: Type-level 'read-only fs handle' wrapper** — Requires new abstraction across a package boundary for a property depguard already enforces.

## Trace

- Requirements: FR-026
- Implementation contracts: contract-no-write-boundary
