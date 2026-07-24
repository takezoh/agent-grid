---
id: adr-20260724-cross-flow-focus-transfer-invariant
kind: adr
title: 'Cross-flow FR-FOCUS-INV: only JumpBackService and EngageFocusService may transfer
  OS foreground focus; enforced by static analyzer'
status: accepted
created: '2026-07-24'
summary: 'Cross-flow FR-FOCUS-INV: only JumpBackService and EngageFocusService may
  transfer OS foreground focus; enforced by static analyzer'
decision_makers:
- agent-grid-maintainers
consulted:
- windows-shell-maintainers
- workspace-maintainers
- server-api-maintainers
informed:
- agent-grid-users
tags:
- native-clients
- windows-shell
- phase2
owners:
- agent-grid-maintainers
relations:
- type: originatedFrom
  target: change-20260723-windows-shell-phase2
source_paths: []
consequences:
  positive:
  - A future toast-dismiss-back-to-session affordance that silently steals foreground
    now violates a named invariant + a build-step check.
  - 'SSOT: focus policy is owned by two named services, not sprinkled across the codebase.'
  negative:
  - New focus-transferring features must extend the allowlist deliberately (an ADR
    update, not a code review comment).
  neutral:
  - Aligns with adr-20260724-daemon-health-toast-structural-separation's structural-enforcement
    posture.
---
# Cross-flow FR-FOCUS-INV: only JumpBackService and EngageFocusService may transfer OS foreground focus; enforced by static analyzer

## Context

{% context %}
Draft-1 had per-flow focus-invariants (FR-JB-03 and FR-EF-02) but no cross-flow invariant excluding a third automated focus-transfer path. Critique major: any future toast-back-to-session or panel-self-activation would be ungoverned.
{% /context %}

## Decision

{% decision %}
Ubiquitous FR-FOCUS-INV lifts the two per-flow FRs to a surface-wide invariant. contract-cross-flow-focus-invariant enforces it structurally: a Roslyn analyzer (or CI ripgrep step) asserts SetForegroundWindow/AllowSetForegroundWindow call sites exist only inside JumpBackService and EngageFocusService. A third call site fails the build.
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- A future toast-dismiss-back-to-session affordance that silently steals foreground now violates a named invariant + a build-step check.
- SSOT: focus policy is owned by two named services, not sprinkled across the codebase.
{% /consequence %}

{% consequence kind="negative" %}
- New focus-transferring features must extend the allowlist deliberately (an ADR update, not a code review comment).
{% /consequence %}

{% consequence kind="neutral" %}
- Aligns with adr-20260724-daemon-health-toast-structural-separation's structural-enforcement posture.
{% /consequence %}

## Alternatives

- **Per-flow FRs only** — Any future third automated focus-transfer path escapes the two counterexamples of F-005/F-004.

## Related

- decision inputs: `decision-input-jump-back-provenance-handoff`, `decision-input-jump-back-target-provenance`
- requirements: `FR-FOCUS-INV`, `FR-JB-01`, `FR-JB-02`, `FR-JB-03`, `FR-EF-01`, `FR-EF-02`
- contracts: `contract-cross-flow-focus-invariant`, `contract-jump-back-staged-resolution`, `contract-engage-focus-return-mechanism`
- change: `change-20260723-windows-shell-phase2`
