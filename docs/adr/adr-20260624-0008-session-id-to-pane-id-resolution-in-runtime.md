---
id: adr-20260624-0008-session-id-to-pane-id-resolution-in-runtime
kind: adr
title: ADR 0008 — Resolve `SessionID` to backend handle in runtime
status: superseded
created: '2026-06-24'
updated: '2026-07-04'
tags:
- adr
- legacy-import
owners: []
relations: []
source_paths: []
decision_makers:
- unknown
summary: The historical "paneID" abstraction described in earlier revisions of this
  ADR no longer exists. termvt.Manager keys on string(FrameID) directly, and there
  is no separate physical-handle namespace inside the backend.
---

<!-- migrated_from: docs/adr/0008-session-id-to-pane-id-resolution-in-runtime.md -->

# ADR 0008 — Resolve `SessionID` to backend handle in runtime

Status: Superseded by ADR 0004.

The historical "paneID" abstraction described in earlier revisions of this
ADR no longer exists. `termvt.Manager` keys on `string(FrameID)` directly,
and there is no separate physical-handle namespace inside the backend. The
wire, runtime, and state layers all share the same `FrameID`; no translation
step exists. This ADR is retained only so its number is not reused.

## Related requirements

- FR-014, FR-015, FR-016
