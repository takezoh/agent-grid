---
id: adr-20260714-wsviewer-appshell-composition
kind: adr
title: Workspace viewer composes via wrapping container preserving ADR-0065 terminal-slot
  rect
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
summary: Workspace viewer composes via wrapping container preserving ADR-0065 terminal-slot
  rect
---

## Context

cc-appshell-preserve forbids changing AppShell's named-grid-area layout or MainTabs' exclusive-tab paradigm. adr-20260624-0065 makes MainTabs' terminal slot an absolute-positioned overlay whose containing block is the region MainTabs itself provides. A sibling ActivityRail placed next to MainTabs inside the main grid area would change the terminal slot's containing block sizing (critique issue-appshell-composition-vs-adr-0065).

## Decision

Introduce a wrapping container inside the existing main grid area that hosts ActivityRail (fixed-width flex child) plus MainTabs (flex child providing the same-sized region for the ADR-0065 terminal-slot absolute overlay). MainTabs receives no props changes; the terminal-slot's rendered rect must remain identical whether ActivityRail is mounted or unmounted. A T1 non-regression test measures the terminal-slot bounding rect with and without ActivityRail mounted and asserts equality.

## Consequences

- cc-appshell-preserve is now structurally verifiable via a bounding-rect regression test.
- ActivityRail composition is fixed at the wrapping-container placement; no per-viewport branching adds structural variance.
- Future rail-adjacent controls plug into the same wrapping container without touching AppShell or MainTabs.

## Alternatives

- **却下: Sibling of MainTabs directly under the main grid area** — Changes MainTabs' containing block size, breaking ADR-0065 terminal-slot geometry.
- **却下: Nest ActivityRail inside MainTabs as a new tab** — Violates exp-live-background (rail must remain visible while a drawer covers viewer content) and abuses MainTabs' exclusive-tab paradigm.

## Trace

- Requirements: FR-027
- Implementation contracts: contract-appshell-preserve
