---
id: adr-20260725-wsl-supervisor-registration-bound-json-config
kind: adr
title: Bind WSL supervisor target at registration in an atomic JSON snapshot
status: accepted
created: '2026-07-25'
summary: Bind WSL supervisor target at registration in an atomic JSON snapshot
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
  - One immutable generation controls launch, health, stop, and records.
  negative:
  - Target changes require an explicit stop/apply/start operation.
  neutral:
  - Existing host flags remain unchanged and are projected from the JSON snapshot.
updated: '2026-07-25'
---

# Bind WSL supervisor target at registration in an atomic JSON snapshot

## Context

{% context %}
The UX consultation confirmed registration-time binding. Runtime default lookup would redirect launch and terminate authority. TOML is only an Explore candidate and persistence types must remain stdlib-only.

Provenance: confirmed constraints come from user consultation and the original plan; unresolved Explore handoffs are provisional seeds resolved by this proposed ADR. This ADR does not change `ux.md` from Explore to accepted fact.
{% /context %}

## Decision

{% decision %}
Store versioned JSON under %ProgramData%\AgentGrid\wsl-host-supervisor.json using temp-file, fsync, rename, ACL validation, and generation fingerprint. Explicit identity/distro override defaults; omitted values are resolved once during registration. Apply requires stop, validate, atomic replace, then start.
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- One immutable generation controls launch, health, stop, and records.
{% /consequence %}

{% consequence kind="negative" %}
- Target changes require an explicit stop/apply/start operation.
{% /consequence %}

{% consequence kind="neutral" %}
- Existing host flags remain unchanged and are projected from the JSON snapshot.
{% /consequence %}

## Alternatives

- **TOML supervisor config** — Adds a third-party codec and duplicates host launch values.
- **Runtime default lookup** — Breaks registration-bound target and stop authority.

## Related

- decision inputs: `decision-input-registration-binding`
- requirements: `FR-WSL-018`
- contracts: `contract-data-model`, `contract-security-boundary`, `contract-migration-compatibility`
- existing ADR posture: `adr-20260724-boundary-3-adopt-before-spawn` applies to probe ordering; `adr-20260724-boundary-3-wsl-detach-spike` is explicitly non-applicable to the foreground service unit; `adr-20260724-daemon-health-toast-structural-separation` remains accepted but is non-applicable because GUI notification is out of scope.
- change: `change-20260725-windows-service-wsl-supervision`
