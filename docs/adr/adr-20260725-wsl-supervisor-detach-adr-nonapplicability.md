---
id: adr-20260725-wsl-supervisor-detach-adr-nonapplicability
kind: adr
title: Do not apply the WSL detach spike to the foreground supervised service unit
status: accepted
created: '2026-07-25'
summary: Do not apply the WSL detach spike to the foreground supervised service unit
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
- {type: references, target: adr-20260724-boundary-3-wsl-detach-spike}
source_paths: []
consequences:
  positive:
  - Foreground ownership is explicit without weakening the accepted ADR in its own
    scope.
  negative:
  - The service cannot reuse the Shell's detach invocation.
  neutral:
  - adr-20260724-boundary-3-adopt-before-spawn remains applicable; the toast-separation
    ADR remains outside this GUI-free component.
updated: '2026-07-25'
---

# Do not apply the WSL detach spike to the foreground supervised service unit

## Context

{% context %}
The accepted detach ADR solves a short-lived Windows launcher whose Linux daemon must outlive it. This service is deliberately a long-lived parent whose child must not outlive supervision.

Provenance: confirmed constraints come from user consultation and the original plan; unresolved Explore handoffs are provisional seeds resolved by this proposed ADR. This ADR does not change `ux.md` from Explore to accepted fact.
{% /context %}

## Decision

{% decision %}
Treat adr-20260724-boundary-3-wsl-detach-spike as non-applicable, not superseded: no setsid or nohup is used in this service topology. Its accepted decision remains valid for Windows Shell Phase 2 only.
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- Foreground ownership is explicit without weakening the accepted ADR in its own scope.
{% /consequence %}

{% consequence kind="negative" %}
- The service cannot reuse the Shell's detach invocation.
{% /consequence %}

{% consequence kind="neutral" %}
- adr-20260724-boundary-3-adopt-before-spawn remains applicable; the toast-separation ADR remains outside this GUI-free component.
{% /consequence %}

## Alternatives

- **Apply setsid plus nohup** — Would sever the lifecycle and stop ownership required here.
- **Supersede the accepted detach ADR** — Incorrect because its original boundary and evidence remain valid.

## Related

- decision inputs: `decision-input-detach-spike-nonapplicability`
- requirements: `FR-WSL-002`, `FR-WSL-003`
- contracts: `contract-control-flow`
- existing ADR posture: `adr-20260724-boundary-3-adopt-before-spawn` applies to probe ordering; `adr-20260724-boundary-3-wsl-detach-spike` is explicitly non-applicable to the foreground service unit; `adr-20260724-daemon-health-toast-structural-separation` remains accepted but is non-applicable because GUI notification is out of scope.
- change: `change-20260725-windows-service-wsl-supervision`
