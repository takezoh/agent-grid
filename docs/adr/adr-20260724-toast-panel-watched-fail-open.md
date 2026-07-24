---
id: adr-20260724-toast-panel-watched-fail-open
kind: adr
title: 'Panel-watched predicate: panel-open > DND/lock precedence; query-unavailable
  fails open toward unwatched'
status: accepted
created: '2026-07-24'
summary: 'Panel-watched predicate: panel-open > DND/lock precedence; query-unavailable
  fails open toward unwatched'
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
  - Deterministic outcome across repeated trials on the same (panel-open, DND, locked)
    tuple.
  - On query-failure, users still see the notification instead of a silently dropped
    approval.
  negative:
  - One extra toast on the rare query-failure path; acceptable per the tradeoff.
  neutral:
  - goal-supervision-toast-budget is preserved on the common path (query succeeds).
---
# Panel-watched predicate: panel-open > DND/lock precedence; query-unavailable fails open toward unwatched

## Context

{% context %}
F-007's OPT-PANEL-UNWATCHED-ONLY trigger condition needs a deterministic resolution of (panel-open, DND, locked) signals and of OS-query failure. Two goals conflict on the rare query-failure path: goal-toast-fallback-recovery (don't miss it) vs goal-supervision-toast-budget (don't spam).
{% /context %}

## Decision

{% decision %}
Panel-open takes precedence over DND/lock when signals conflict (user is looking at the panel). OS DND/lock-state query failure fails open toward unwatched (toast fires). Both resolutions are documented in contract-toast-panel-watched-detection's outcome partition.
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- Deterministic outcome across repeated trials on the same (panel-open, DND, locked) tuple.
- On query-failure, users still see the notification instead of a silently dropped approval.
{% /consequence %}

{% consequence kind="negative" %}
- One extra toast on the rare query-failure path; acceptable per the tradeoff.
{% /consequence %}

{% consequence kind="neutral" %}
- goal-supervision-toast-budget is preserved on the common path (query succeeds).
{% /consequence %}

## Alternatives

- **Fail closed (suppress toast) on query failure** — Risks silently missing an approval while the panel is truly unwatched — directly contradicts goal-toast-fallback-recovery's desired_outcome.

## Related

- decision inputs: `decision-input-appnotification-inline-textbox`
- requirements: `FR-TOAST-01`
- contracts: `contract-toast-panel-watched-detection`
- change: `change-20260723-windows-shell-phase2`
