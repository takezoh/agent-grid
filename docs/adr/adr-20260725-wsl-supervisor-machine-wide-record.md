---
id: adr-20260725-wsl-supervisor-machine-wide-record
kind: adr
title: Write structured records under ProgramData with bounded retention and SCM pre-start
  complement
status: accepted
created: '2026-07-25'
summary: Write structured records under ProgramData with bounded retention and SCM
  pre-start complement
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
- {type: references, target: adr-20260724-daemon-health-toast-structural-separation}
source_paths: []
consequences:
  positive:
  - Records are deterministic, operator-reachable, bounded, and structurally redacted.
  negative:
  - Raw child diagnostics are not retained by the Windows supervisor.
  neutral:
  - The Linux host retains its own existing logs; SCM owns failures before process
    creation.
updated: '2026-07-25'
---

# Write structured records under ProgramData with bounded retention and SCM pre-start complement

## Context

{% context %}
Consultation selected a dedicated file, but a profile-relative path is not operator-reachable and no process can log before SCM creates it.

Provenance: confirmed constraints come from user consultation and the original plan; unresolved Explore handoffs are provisional seeds resolved by this proposed ADR. This ADR does not change `ux.md` from Explore to accepted fact.
{% /context %}

## Decision

{% decision %}
Use %ProgramData%\AgentGrid\logs\wsl-host.log as primary structured JSON-lines record. ACL: SYSTEM and Administrators full control, configured service identity modify, BUILTIN\Users read on logs only. Rotate at 10 MiB, keep 5 files, delete files older than 30 days. Persist only allowlisted supervisor fields and never raw stdout/stderr, argv secrets, tokens, or environment values. Pre-process failures are read through SCM's System event record and documented sc query/event lookup.
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- Records are deterministic, operator-reachable, bounded, and structurally redacted.
{% /consequence %}

{% consequence kind="negative" %}
- Raw child diagnostics are not retained by the Windows supervisor.
{% /consequence %}

{% consequence kind="neutral" %}
- The Linux host retains its own existing logs; SCM owns failures before process creation.
{% /consequence %}

## Alternatives

- **Windows Event Log as primary sink** — Rejected by user consultation; retain only SCM's unavoidable pre-start complement.
- **Service-account LocalAppData** — Not reliably reachable from the signed-in operator profile.
- **Persist raw stdout/stderr with redaction** — Cannot prove arbitrary application output contains no secret.

## Related

- decision inputs: `decision-input-dedicated-log`
- requirements: `FR-WSL-007`, `FR-WSL-016`
- contracts: `contract-resource-management`, `contract-user-observability`, `contract-security-boundary`
- existing ADR posture: `adr-20260724-boundary-3-adopt-before-spawn` applies to probe ordering; `adr-20260724-boundary-3-wsl-detach-spike` is explicitly non-applicable to the foreground service unit; `adr-20260724-daemon-health-toast-structural-separation` remains accepted but is non-applicable because GUI notification is out of scope.
- change: `change-20260725-windows-service-wsl-supervision`
