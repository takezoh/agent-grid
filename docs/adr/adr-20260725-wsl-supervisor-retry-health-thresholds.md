---
id: adr-20260725-wsl-supervisor-retry-health-thresholds
kind: adr
title: Use typed failure classes and fixed two-stage health and terminal thresholds
status: accepted
created: '2026-07-25'
summary: Use typed failure classes and fixed two-stage health and terminal thresholds
decision_makers:
- agent-grid-maintainers
consulted:
- windows-service-operators
informed:
- agent-grid-users
tags:
- windows
- wsl
- supervision
owners:
- agent-grid-maintainers
relations:
- {type: originatedFrom, target: change-20260725-windows-service-wsl-supervision}
source_paths: []
consequences:
  positive:
  - Recovery and terminal time are bounded and fake-clock testable.
  negative:
  - Numerical policy changes require a new design decision and operational retest.
  neutral:
  - SCM Running does not encode Ready/Backoff/Failed; health plus records remain the
    read model.
updated: '2026-07-25'
---

# Use typed failure classes and fixed two-stage health and terminal thresholds

## Context

{% context %}
The UX requires configuration, transient, already-running, and unknown outcomes to remain discriminating and requires different restart and terminal thresholds.

Provenance: confirmed constraints come from user consultation and the original plan; unresolved Explore handoffs are provisional seeds resolved by this proposed ADR. This ADR does not change `ux.md` from Explore to accepted fact.
{% /context %}

## Decision

{% decision %}
Use health URL and TLS mode projected from the same bound addr/config. Probe timeout 2s, cadence 5s, startup readiness 60s, resume probe start <=10s, health restart threshold 3, configuration terminal threshold 5, transient/unknown terminal threshold 10, and retry schedule 1,2,5,15,60 seconds capped at 60. Ready resets both counters. status ok alone is healthy; daemon-unavailable is unhealthy.
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- Recovery and terminal time are bounded and fake-clock testable.
{% /consequence %}

{% consequence kind="negative" %}
- Numerical policy changes require a new design decision and operational retest.
{% /consequence %}

{% consequence kind="neutral" %}
- SCM Running does not encode Ready/Backoff/Failed; health plus records remain the read model.
{% /consequence %}

## Alternatives

- **One threshold for every failure** — Fails UAC-017 and hides classifier errors.
- **Unbounded retry** — Hides permanent configuration failure.
- **Fixed supervisor-only port** — Can drift from the server addr.

## Related

- decision inputs: `decision-input-health-config`
- requirements: `FR-WSL-001`, `FR-WSL-004`, `FR-WSL-005`, `FR-WSL-011`, `FR-WSL-012`, `FR-WSL-015`, `FR-WSL-017`
- contracts: `contract-state-lifecycle`, `contract-performance-budget`
- existing ADR posture: `adr-20260724-boundary-3-adopt-before-spawn` applies to probe ordering; `adr-20260724-boundary-3-wsl-detach-spike` is explicitly non-applicable to the foreground service unit; `adr-20260724-daemon-health-toast-structural-separation` remains accepted but is non-applicable because GUI notification is out of scope.
- change: `change-20260725-windows-service-wsl-supervision`
