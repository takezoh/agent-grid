---
id: adr-20260725-wsl-supervisor-shutdown-budget-escalation
kind: adr
title: Use a 15-second graceful budget before registered-distro-only escalation
status: accepted
created: '2026-07-25'
summary: Use a 15-second graceful budget before registered-distro-only escalation
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
  - The budget is derived from existing shutdown behavior and shared by both paths.
  negative:
  - A stuck recovery can disrupt every process in the bound distribution.
  neutral:
  - The service reports SCM stop-pending checkpoints while the bounded sequence runs.
updated: '2026-07-25'
---

# Use a 15-second graceful budget before registered-distro-only escalation

## Context

{% context %}
UX handoff DP-SHUTDOWN-TIMEOUT-PROVENANCE leaves the numeric source to design and recommends OPT-DERIVE-FROM-SERVER-BUDGET. The server drains for 8 seconds and then its gateway has a 5-second grace; the ungrounded 30-second proposal is not adopted.

Provenance: confirmed constraints come from user consultation and the original plan; unresolved Explore handoffs are provisional seeds resolved by this proposed ADR. This ADR does not change `ux.md` from Explore to accepted fact.
{% /context %}

## Decision

{% decision %}
Use one 15-second graceful shutdown budget for SCM stop and recovery restart: 8s drain + 5s gateway grace + 2s supervised-unit exit overhead. SCM stop then terminates the owned child, waits 2s, and conditionally terminates the bound distro. Recovery restart skips child-only kill after 15s and directly terminates the bound distro, matching UAC-013. Never call wsl --shutdown.
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- The budget is derived from existing shutdown behavior and shared by both paths.
{% /consequence %}

{% consequence kind="negative" %}
- A stuck recovery can disrupt every process in the bound distribution.
{% /consequence %}

{% consequence kind="neutral" %}
- The service reports SCM stop-pending checkpoints while the bounded sequence runs.
{% /consequence %}

## Alternatives

- **Thirty seconds** — Has no repository derivation and needlessly delays SCM.
- **Eight seconds** — Interrupts gateway grace and unit exit.
- **Always child-kill before distro terminate** — Contradicts the confirmed recovery escalation scenario.

## Related

- decision inputs: `decision-input-shutdown-timeout-provenance` (`OPT-DERIVE-FROM-SERVER-BUDGET` adopted)
- requirements: `FR-WSL-008`, `FR-WSL-009`, `FR-WSL-010`, `FR-WSL-013`
- contracts: `contract-failure-recovery`, `contract-performance-budget`
- existing ADR posture: `adr-20260724-boundary-3-adopt-before-spawn` applies to probe ordering; `adr-20260724-boundary-3-wsl-detach-spike` is explicitly non-applicable to the foreground service unit; `adr-20260724-daemon-health-toast-structural-separation` remains accepted but is non-applicable because GUI notification is out of scope.
- change: `change-20260725-windows-service-wsl-supervision`
