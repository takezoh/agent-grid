---
id: adr-20260724-boundary-1-named-pipe-jsonlines
kind: adr
title: Shell↔Workspace boundary uses \\.\pipe\agent-grid-workspace named pipe with
  a closed {op,id} JSON Lines envelope
status: accepted
created: '2026-07-24'
summary: Shell↔Workspace boundary uses \\.\pipe\agent-grid-workspace named pipe with
  a closed {op,id} JSON Lines envelope
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
  - OS-native single-user ACL is granted for free (matches cc-single-user-personal).
  - No port-discovery/coordination code needed.
  negative:
  - Windows-only; acceptable because macOS/Linux clients are out of scope.
  neutral:
  - Additive-only schema evolution follows the same additive-only rule as adr-20260724-deep-link-shape-adopts-remote-control-plan.
---
# Shell↔Workspace boundary uses \\.\pipe\agent-grid-workspace named pipe with a closed {op,id} JSON Lines envelope

## Context

{% context %}
Boundary 1 needs a low-latency single-user control channel carrying only control envelopes (no session domain data). Options: named pipe + JSON Lines, TCP loopback ephemeral port, or a full IPC framework (gRPC).
{% /context %}

## Decision

{% decision %}
Adopt a Windows named pipe at \\.\pipe\agent-grid-workspace with a closed {op,id} JSON Lines envelope (contract-b1-jsonlines-envelope-shape). The envelope carries an additive schema_version for future evolution; any additional field is rejected.
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- OS-native single-user ACL is granted for free (matches cc-single-user-personal).
- No port-discovery/coordination code needed.
{% /consequence %}

{% consequence kind="negative" %}
- Windows-only; acceptable because macOS/Linux clients are out of scope.
{% /consequence %}

{% consequence kind="neutral" %}
- Additive-only schema evolution follows the same additive-only rule as adr-20260724-deep-link-shape-adopts-remote-control-plan.
{% /consequence %}

## Alternatives

- **TCP loopback ephemeral port** — Reintroduces port discovery/coordination for a same-machine channel that named pipes already cover with zero dependency.
- **gRPC / other IPC framework** — Disproportionate for a two-op control channel; adds a codegen dependency the minimal envelope avoids.

## Related

- decision inputs: `decision-input-named-pipe-jsonlines`, `decision-input-named-pipe-jsonlines-ipc`
- requirements: `FR-B1-01`, `FR-B1-02`, `FR-B1-03`
- contracts: `contract-b1-jsonlines-envelope-shape`
- change: `change-20260723-windows-shell-phase2`
