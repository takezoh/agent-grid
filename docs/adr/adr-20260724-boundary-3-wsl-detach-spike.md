---
id: adr-20260724-boundary-3-wsl-detach-spike
kind: adr
title: WSL-side daemon detach uses setsid + nohup wrapping wsl.exe -d <distro> --
  setsid nohup <path>/server ...
status: accepted
created: '2026-07-24'
summary: WSL-side daemon detach uses setsid + nohup wrapping wsl.exe -d <distro> --
  setsid nohup <path>/server ...
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
  - Zero additional install-time artifacts (no unit file to author/install/keep in
    sync).
  - Single-line argv change to Runner.cs invocation makes the mechanism auditable
    at one call site.
  - Survival verification (contract-b3-wsl-detach-mechanism) is the sole ongoing gate;
    no mechanism-selection decision remains open.
  negative:
  - If a future WSL2 update changes pid-1 reparenting behavior such that setsid+nohup
    no longer survives, this ADR is superseded by a new one selecting systemd --user;
    the survival test in verify-wsl-detach-fidelity is the regression detector.
  neutral:
  - adr-20260716-restart-continuity-compatibility-axes's per-server restart semantics
    are unaffected — the daemon's own restart flow is orthogonal to detach mechanism
    selection.
---
# WSL-side daemon detach uses setsid + nohup wrapping wsl.exe -d <distro> -- setsid nohup <path>/server ...

## Context

{% context %}
'Shell closes ≠ daemon dies' (FR-B3-05) requires the WSL-side server to survive the wsl.exe launcher exiting. Options considered: setsid/nohup wrapper vs systemd --user service unit. WSL2's minimal init behavior can differ from a normal Linux session; user consultation (2026-07-24) resolved the choice in favor of setsid + nohup based on WSL2's actual pid-1 reparenting behavior. Plan §9's S1 gate now becomes a narrower survival verification rather than a mechanism-selection spike.
{% /context %}

## Decision

{% decision %}
DaemonSupervisor spawns the WSL-side daemon via `wsl.exe -d <distro> -- setsid nohup <path>/server ...` (setsid detaches the process from the wsl.exe controlling terminal / session; nohup ignores SIGHUP when the wsl.exe launcher exits). The S1 spike is narrowed to a survival verification: forcibly kill the Windows-side wsl.exe launcher (taskkill /F on wsl.exe PID) and assert that GET /api/sessions on 127.0.0.1:<port> continues to respond after ≥5s. systemd --user is explicitly rejected as unnecessary for the single-user personal-use scope (cc-single-user-personal).
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- Zero additional install-time artifacts (no unit file to author/install/keep in sync).
- Single-line argv change to Runner.cs invocation makes the mechanism auditable at one call site.
- Survival verification (contract-b3-wsl-detach-mechanism) is the sole ongoing gate; no mechanism-selection decision remains open.
{% /consequence %}

{% consequence kind="negative" %}
- If a future WSL2 update changes pid-1 reparenting behavior such that setsid+nohup no longer survives, this ADR is superseded by a new one selecting systemd --user; the survival test in verify-wsl-detach-fidelity is the regression detector.
{% /consequence %}

{% consequence kind="neutral" %}
- adr-20260716-restart-continuity-compatibility-axes's per-server restart semantics are unaffected — the daemon's own restart flow is orthogonal to detach mechanism selection.
{% /consequence %}

## Alternatives

- **systemd --user service unit** — Adds a unit file to author/install/keep in sync per-distro; unnecessary for single-user personal-use scope where setsid+nohup verifiably survives per the S1 survival test.
- **Background inside wsl.exe process group without detaching** — Explicitly fails the adversarial case in contract-b3-wsl-detach-mechanism (launcher kill terminates the whole group).

## Related

- decision inputs: (none)
- requirements: `FR-B3-05`
- contracts: `contract-b3-wsl-detach-mechanism`
- change: `change-20260723-windows-shell-phase2`
