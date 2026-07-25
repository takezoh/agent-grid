---
id: adr-20260725-wsl-supervisor-direct-foreground-exec
kind: adr
title: Launch the host server by direct foreground wsl.exe --exec
status: accepted
created: '2026-07-25'
summary: Launch the host server by direct foreground wsl.exe --exec
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
  - The process tree and exit code have one owner and UAC-003 is directly testable.
  negative:
  - Future environment setup must be represented as configuration or reconsider this
    ADR.
  neutral:
  - Graceful stop uses the separate Linux control command, not Windows signal propagation
    through wsl.exe.
updated: '2026-07-25'
---

# Launch the host server by direct foreground wsl.exe --exec

## Context

{% context %}
Foreground supervision requires child exit ownership and no residual shell. The original plan names direct exec as the primary form.

Provenance: confirmed constraints come from user consultation and the original plan; unresolved Explore handoffs are provisional seeds resolved by this proposed ADR. This ADR does not change `ux.md` from Explore to accepted fact.
{% /context %}

## Decision

{% decision %}
Launch wsl.exe --distribution <bound> --user <bound> --exec <server> with explicit host flags as the long-lived child. Do not insert cmd.exe, powershell, sh -c, a background process, or a launcher. Environment-dependent setup must become explicit argv/config, not a shell.
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- The process tree and exit code have one owner and UAC-003 is directly testable.
{% /consequence %}

{% consequence kind="negative" %}
- Future environment setup must be represented as configuration or reconsider this ADR.
{% /consequence %}

{% consequence kind="neutral" %}
- Graceful stop uses the separate Linux control command, not Windows signal propagation through wsl.exe.
{% /consequence %}

## Alternatives

- **Launcher plus exec** — Unneeded for the confirmed explicit argv/config surface and creates another deployed artifact.
- **Detached/background launch** — Destroys exit ownership and causes false restart loops.

## Related

- decision inputs: `decision-input-launch-topology`
- requirements: `FR-WSL-002`, `FR-WSL-003`
- contracts: `contract-control-flow`
- existing ADR posture: `adr-20260724-boundary-3-adopt-before-spawn` applies to probe ordering; `adr-20260724-boundary-3-wsl-detach-spike` is explicitly non-applicable to the foreground service unit; `adr-20260724-daemon-health-toast-structural-separation` remains accepted but is non-applicable because GUI notification is out of scope.
- change: `change-20260725-windows-service-wsl-supervision`
