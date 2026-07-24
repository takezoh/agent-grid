---
id: adr-20260724-boundary-3-adopt-before-spawn
kind: adr
title: DaemonSupervisor uses adopt-first-then-spawn lifecycle ordering; at most one
  spawn per boot
status: accepted
created: '2026-07-24'
summary: DaemonSupervisor uses adopt-first-then-spawn lifecycle ordering; at most
  one spawn per boot
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
  - Avoids dual-run health flicker that UAC-001 CE names as a failure mode.
  - Existing daemon from a prior Shell session is found, not replaced.
  negative:
  - One extra sub-second round trip on every cold start; acceptable against loopback.
  neutral:
  - FR-B3-02 and FR-B3-03 are enforceable by the state-machine outcome partition.
---
# DaemonSupervisor uses adopt-first-then-spawn lifecycle ordering; at most one spawn per boot

## Context

{% context %}
Boundary 3 must handle both the 'existing daemon' and 'no daemon yet' cases without dual-run flicker (UAC-001 CE). Options: adopt-first-then-spawn vs spawn-first-then-adopt-on-conflict.
{% /context %}

## Decision

{% decision %}
Adopt-first-then-spawn: DaemonSupervisor probes the configured port with the UNC-fresh token first; only if the probe fails does it spawn wsl.exe -d <distro> -- <path>/server. At most one spawn per boot; a spawned-then-crashed process transitions to explicit Degraded/failed rather than a second spawn.
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- Avoids dual-run health flicker that UAC-001 CE names as a failure mode.
- Existing daemon from a prior Shell session is found, not replaced.
{% /consequence %}

{% consequence kind="negative" %}
- One extra sub-second round trip on every cold start; acceptable against loopback.
{% /consequence %}

{% consequence kind="neutral" %}
- FR-B3-02 and FR-B3-03 are enforceable by the state-machine outcome partition.
{% /consequence %}

## Alternatives

- **Spawn-first-then-adopt-on-conflict** — Transient dual-process window is directly user-visible as flicker; changes failure behavior.

## Related

- decision inputs: `decision-input-capability-negotiation-axis`
- requirements: `FR-B3-01`, `FR-B3-02`, `FR-B3-03`
- contracts: `contract-b3-daemon-supervisor-state-machine`
- change: `change-20260723-windows-shell-phase2`
