---
id: adr-20260624-0006-surface-namespace-for-new-proto-commands
kind: adr
title: ADR 0006 — Group new proto commands/events under the `Surface` namespace
status: accepted
created: '2026-06-24'
updated: '2026-07-04'
tags:
- adr
- legacy-import
owners: []
relations:
- {type: referencedBy, target: note-20260624-technical-web-gateway}
source_paths:
- src/client/proto/
decision_makers:
- unknown
---

<!-- migrated_from: docs/adr/0006-surface-namespace-for-new-proto-commands.md -->

# ADR 0006 — Group new proto commands/events under the `Surface` namespace

Status: Accepted

## Context

A1-α adds four new commands (subscribe / unsubscribe / resize / write-raw) and
two new events (terminal output / prompt event) to `client/proto`. The existing
surface family already includes `CmdSurfaceSendText` and `CmdSurfaceSendKey`,
which model "the visible 1-frame I/O surface a user interacts with". Earlier
drafts proposed `CmdSubscribeTerminal` / `CmdResizeFrame` etc., which would
have introduced a second namespace (Terminal / Frame) overlapping with Surface.

## Decision

Name the new commands `CmdSurfaceSubscribe` / `CmdSurfaceUnsubscribe` /
`CmdSurfaceResize` / `CmdSurfaceWriteRaw` and the new events
`EvtSurfaceOutput` / `EvtPromptEvent`. The Terminal / Frame prefixes proposed
in the initial draft are discarded.

## Consequences

- `codec.go` switch becomes coherent: all Surface commands group together,
  matching the existing SendText / SendKey decoders.
- Documentation does not bifurcate "Terminal vs Surface vs Frame".
- The fuzz corpus discriminates on a single concept (Surface), simplifying
  invariant statements.
- Future additions to the same plane (e.g. cursor / selection commands) have
  a natural home.

## Alternatives

- **Separate Terminal / Frame prefixes** — rejected because they would create
  parallel vocabularies for the same domain concept.
- **Fold subscribe into a generic Filter mechanism** — rejected as YAGNI for
  α; can be revisited when driver-side filters need richer semantics.

## Related requirements

- FR-003, FR-004, FR-005, FR-006, FR-011, FR-021
