---
id: adr-20260724-engage-focus-restore-mechanism
kind: adr
title: Engage focus-return uses AttachThreadInput(from,target,TRUE) + SetForegroundWindow
  + AttachThreadInput(from,target,FALSE)
status: accepted
created: '2026-07-24'
summary: Engage focus-return uses AttachThreadInput(from,target,TRUE) + SetForegroundWindow
  + AttachThreadInput(from,target,FALSE)
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
  - Definitive Win32 mechanism removes the plan-time uncertainty; no fallback branch
    to maintain.
  - Try/finally ensures no lingering AttachThreadInput bond leaks even under exception
    paths.
  negative:
  - Screen readers (NVDA, JAWS, Narrator) that hook input events can observe the transient
    AttachThreadInput bond during restore; this is a known interaction pattern for
    cross-process focus restore and is not expected to break assistive-tech announcements,
    but AT compatibility should be spot-verified during the S3 pre-implementation
    prototype window.
  - AttachThreadInput blocks briefly if the target thread's message loop is unresponsive;
    the restore call is issued asynchronously off the UI thread with a bounded 500ms
    timeout to avoid stalling engage UI.
  neutral:
  - 'contract-engage-focus-return-mechanism''s outcome partition unchanged: OS-denied
    restore stays a legitimate branch.'
---
# Engage focus-return uses AttachThreadInput(from,target,TRUE) + SetForegroundWindow + AttachThreadInput(from,target,FALSE)

## Context

{% context %}
DP-PANEL-ENGAGE-FOCUS-RETURN fixes OPT-CAPTURE-RESTORE at the product level. The Win32 mechanism that actually survives Windows' foreground-lock policy is undocumented product behavior. Options: AttachThreadInput-based restore vs SendInput synthetic-input trick vs AllowSetForegroundWindow (requires target-process cooperation, not usable cross-process). User consultation (2026-07-24) resolved this in favor of the AttachThreadInput pattern.
{% /context %}

## Decision

{% decision %}
EngageFocusService.Restore(targetHwnd) invokes the following Win32 pattern: (1) `fromThread = GetWindowThreadProcessId(GetForegroundWindow(), null)`; (2) `targetThread = GetWindowThreadProcessId(targetHwnd, null)`; (3) `AttachThreadInput(fromThread, targetThread, TRUE)` to share input queues with the target thread — this satisfies Windows' foreground-lock heuristic; (4) `SetForegroundWindow(targetHwnd)`; (5) `AttachThreadInput(fromThread, targetThread, FALSE)` to detach. The five-step sequence is wrapped in a try/finally so the detach in step 5 always runs even when SetForegroundWindow returns FALSE. The outcome partition in contract-engage-focus-return-mechanism still recognizes {restore-live, restore-denied, target-destroyed} branches; step 4 returning FALSE routes to restore-denied.
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- Definitive Win32 mechanism removes the plan-time uncertainty; no fallback branch to maintain.
- Try/finally ensures no lingering AttachThreadInput bond leaks even under exception paths.
{% /consequence %}

{% consequence kind="negative" %}
- Screen readers (NVDA, JAWS, Narrator) that hook input events can observe the transient AttachThreadInput bond during restore; this is a known interaction pattern for cross-process focus restore and is not expected to break assistive-tech announcements, but AT compatibility should be spot-verified during the S3 pre-implementation prototype window.
- AttachThreadInput blocks briefly if the target thread's message loop is unresponsive; the restore call is issued asynchronously off the UI thread with a bounded 500ms timeout to avoid stalling engage UI.
{% /consequence %}

{% consequence kind="neutral" %}
- contract-engage-focus-return-mechanism's outcome partition unchanged: OS-denied restore stays a legitimate branch.
{% /consequence %}

## Alternatives

- **SendInput synthetic-key trick** — Injects synthetic input that pollutes the input stream visible to keyboard-hook consumers (macros, assistive tech); the AttachThreadInput pattern is a cleaner cross-process foreground-lock workaround.
- **AllowSetForegroundWindow** — Requires cooperation from the target process; not available cross-process for arbitrary external apps.

## Related

- decision inputs: (none)
- requirements: `FR-EF-01`, `FR-EF-02`
- contracts: `contract-engage-focus-return-mechanism`
- change: `change-20260723-windows-shell-phase2`
