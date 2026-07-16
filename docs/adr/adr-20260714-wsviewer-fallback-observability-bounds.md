---
id: adr-20260714-wsviewer-fallback-observability-bounds
kind: adr
title: Parse-timeout, tree-torn-down, and non-git degradation observables have closed
  epistemic partitions and numeric bounds
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
summary: Parse-timeout, tree-torn-down, and non-git degradation observables have closed
  epistemic partitions and numeric bounds
---

## Context

Three fallible observables were left with open partitions or unbounded observables in the draft: (a) parse timeout for Mermaid/JSON structured render (critique issue-parse-timeout-bound-unfixed); (b) workspace-root-disappeared tree state (critique issue-tree-refresh-workspace-torn-down-open); (c) corrupted-but-present .git vs missing .git indistinguishable (critique dimension coverage attack). All three must fix outcome partitions and bounds at plan time.

## Decision

(a) Parse timeout: Mermaid and JSON renderers abort parsing after 300 ms per file (NFR-004 measurement); on abort or parse failure, the raw-source/raw-text fallback pane renders within an additional 100 ms. (b) Tree-torn-down: the workspace-fs-api tree endpoint returns a distinct typed response 'root_unreachable' when EvalSymlinks or stat of the WorkspaceRootHandle-snapshotted root fails; WorkspaceTree renders an explicit banner + retry control; silent success is structurally impossible because the typed response is non-optional. (c) Diff base: git-helper distinguishes 'not_a_repo' (no .git), 'git_metadata_corrupted' (.git exists but git-cli errors), 'git_binary_missing' (exec.LookPath fails). DiffViewer surfaces distinct banner copy per class; the three form a closed outcome partition alongside 'ok'.

## Consequences

- 'never blank or infinite loading' becomes falsifiable via the 300 ms + 100 ms bounds.
- 'silent success on root removal' is structurally impossible.
- Non-git degradation covers three distinct causes with dedicated banner copy each, aiding operator diagnosis.

## Alternatives

- **却下: Leave numeric parse timeout to implementation** — Unfalsifiable observable — 'eventually' is not a bound.
- **却下: Single non-git degraded response covering all git failures** — Operator cannot distinguish 'missing binary' (env issue) from 'not a repo' (workspace choice) — misleads root-cause analysis.

## Trace

- Requirements: FR-010, FR-011, FR-013, FR-014, FR-019, FR-020
- Implementation contracts: contract-structured-render-fallback, contract-tree-refresh, contract-diff-base-non-git-fallback
