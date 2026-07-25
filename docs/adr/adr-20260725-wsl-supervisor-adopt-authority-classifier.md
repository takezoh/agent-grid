---
id: adr-20260725-wsl-supervisor-adopt-authority-classifier
kind: adr
title: Probe before spawn and adopt only a fingerprint-matched locked host instance
status: accepted
created: '2026-07-25'
summary: Probe before spawn and adopt only a fingerprint-matched locked host instance
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
- {type: references, target: adr-20260724-boundary-3-adopt-before-spawn}
source_paths: []
consequences:
  positive:
  - Adoption, spawn permission, and stop authority derive from one typed identity
    proof.
  negative:
  - Adds two additive Linux CLI operations and /proc-based fidelity requirements.
  neutral:
  - The pid stored in the lock remains human-readable; authority additionally uses
    start token and fingerprint.
updated: '2026-07-25'
---

# Probe before spawn and adopt only a fingerprint-matched locked host instance

## Context

{% context %}
The accepted adopt-before-spawn ADR forbids learning adoption from a failed spawn. Stop ownership must also be explicit.

Provenance: confirmed constraints come from user consultation and the original plan; unresolved Explore handoffs are provisional seeds resolved by this proposed ADR. This ADR does not change `ux.md` from Explore to accepted fact.
{% /context %}

## Decision

{% decision %}
Before every spawn, invoke the additive Linux supervisor-probe command for the bound data directory and health target. It returns typed JSON and exit classes after checking flock holder PID, Linux start token, executable, argv fingerprint, and health. matched_running is adopted with stop authority bound to that token; absent alone permits one spawn; mismatch is configuration failure; unavailable/unknown is transient and never permits spawn. supervisor-stop revalidates the token before SIGTERM.
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- Adoption, spawn permission, and stop authority derive from one typed identity proof.
{% /consequence %}

{% consequence kind="negative" %}
- Adds two additive Linux CLI operations and /proc-based fidelity requirements.
{% /consequence %}

{% consequence kind="neutral" %}
- The pid stored in the lock remains human-readable; authority additionally uses start token and fingerprint.
{% /consequence %}

## Alternatives

- **Parse spawn stderr for flock refusal** — Violates probe-before-spawn and is text-fragile.
- **Adopt healthy endpoint without process fingerprint** — Cannot establish data-dir identity or stop authority.
- **Never stop adopted instances** — Would make SCM Stopped while the required endpoint remains alive.

## Related

- decision inputs: `decision-input-already-running`
- requirements: `FR-WSL-006`, `FR-WSL-008`
- contracts: `contract-state-lifecycle`, `contract-integration-contract`
- existing ADR posture: `adr-20260724-boundary-3-adopt-before-spawn` applies to probe ordering; `adr-20260724-boundary-3-wsl-detach-spike` is explicitly non-applicable to the foreground service unit; `adr-20260724-daemon-health-toast-structural-separation` remains accepted but is non-applicable because GUI notification is out of scope.
- change: `change-20260725-windows-service-wsl-supervision`
