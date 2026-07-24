---
id: adr-20260724-daemon-health-toast-structural-separation
kind: adr
title: DaemonSupervisor's package must not import ToastNotifier's package; enforced
  by depguard-style analyzer
status: accepted
created: '2026-07-24'
summary: DaemonSupervisor's package must not import ToastNotifier's package; enforced
  by depguard-style analyzer
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
  - Regression that adds `if health == Degraded { toast.Show(...) }` inside DaemonSupervisor
    fails the build, not a runtime budget check after the fact.
  - 'SSOT: the state-authoring component owns the invariant, not the observing/notifying
    layer.'
  negative:
  - Adds a build-step analyzer dependency; acceptable given ARCHITECTURE.md's mechanical-enforcement
    posture for structural rules.
  neutral:
  - contract-daemon-health-toast-budget and contract-health-toast-structural-separation
    are two owner-shared contracts on this invariant.
---
# DaemonSupervisor's package must not import ToastNotifier's package; enforced by depguard-style analyzer

## Context

{% context %}
goal-supervision-toast-budget requires zero non-supervision toasts within the observation window even under health-state flapping (F-108). A runtime 'if healthy don't toast' conditional could regress under future edits; SSOT correction from critique moved contract ownership from Shell.Platform.Win32 to DaemonSupervisor (the health-state authoring component).
{% /context %}

## Decision

{% decision %}
Structural separation: DaemonSupervisor's namespace has no import or call edge into ToastNotifier's namespace. Enforced by a Roslyn analyzer (or NetArchTest rule) wired into CI, mirroring ARCHITECTURE.md's platform/host/orchestrator depguard rule.
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- Regression that adds `if health == Degraded { toast.Show(...) }` inside DaemonSupervisor fails the build, not a runtime budget check after the fact.
- SSOT: the state-authoring component owns the invariant, not the observing/notifying layer.
{% /consequence %}

{% consequence kind="negative" %}
- Adds a build-step analyzer dependency; acceptable given ARCHITECTURE.md's mechanical-enforcement posture for structural rules.
{% /consequence %}

{% consequence kind="neutral" %}
- contract-daemon-health-toast-budget and contract-health-toast-structural-separation are two owner-shared contracts on this invariant.
{% /consequence %}

## Alternatives

- **Runtime 'if healthy don't toast' guard inside DaemonSupervisor** — A future edit adding an else-branch bypasses the guard; runtime-only detection catches it after the fact.

## Related

- decision inputs: `decision-input-h-notifyicon`, `decision-input-h-notifyicon-winui`
- requirements: `FR-TOAST-02`
- contracts: `contract-daemon-health-toast-budget`, `contract-health-toast-structural-separation`
- change: `change-20260723-windows-shell-phase2`
