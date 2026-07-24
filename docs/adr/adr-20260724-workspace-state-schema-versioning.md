---
id: adr-20260724-workspace-state-schema-versioning
kind: adr
title: workspace-state.json carries schema_version; readers handle old/new versions
  safely, never silent corruption
status: accepted
created: '2026-07-24'
summary: workspace-state.json carries schema_version; readers handle old/new versions
  safely, never silent corruption
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
  - Future window-registry field additions have an explicit backward-compat obligation.
  - Downgrade or partial-deploy scenarios don't silently corrupt live state.
  negative:
  - Reader must implement a small documented upgrade path per known-older version.
  neutral:
  - FR-WS-STATE-SCHEMA and the safe-fallback verification are gated on this ADR.
---
# workspace-state.json carries schema_version; readers handle old/new versions safely, never silent corruption

## Context

{% context %}
Critique minor: draft-1's input-workspace-window-state-file has snapshot preservation but no schema_version field; a future window-registry change adding new fields (e.g. per-monitor DPI) will silently corrupt or discard persisted state on downgrade/partial deploy.
{% /context %}

## Decision

{% decision %}
Every persisted workspace-state.json record carries a schema_version field. Readers handle (current, older-known, newer-unknown, corrupt, missing) via the documented outcome partition; a newer-unknown file yields the same empty-map startup as missing/corrupt (contract-workspace-state-schema-evolution).
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- Future window-registry field additions have an explicit backward-compat obligation.
- Downgrade or partial-deploy scenarios don't silently corrupt live state.
{% /consequence %}

{% consequence kind="negative" %}
- Reader must implement a small documented upgrade path per known-older version.
{% /consequence %}

{% consequence kind="neutral" %}
- FR-WS-STATE-SCHEMA and the safe-fallback verification are gated on this ADR.
{% /consequence %}

## Alternatives

- **No schema_version; rely on future field additions being backward-compatible by convention** — Convention is not enforceable; silent corruption on downgrade is the exact failure mode critique flagged.

## Related

- decision inputs: (none)
- requirements: `FR-WS-STATE-SCHEMA`
- contracts: `contract-workspace-state-schema-evolution`
- change: `change-20260723-windows-shell-phase2`
