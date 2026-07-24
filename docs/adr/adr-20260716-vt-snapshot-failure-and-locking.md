---
id: adr-20260716-vt-snapshot-failure-and-locking
kind: adr
title: VT snapshot failure and locking contract
status: proposed
created: '2026-07-16'
decision_makers:
- unknown
tags:
- terminal
- snapshot
- concurrency
owners: []
relations:
- {type: references, target: adr-20260715-terminal-semantic-history-reattach}
- {type: references, target: adr-20260715-geometry-bearing-terminal-attach}
source_paths:
- src/go.mod
- src/platform/termvt
methodology: sdd
summary: Expose concrete typed snapshot failure and synchronize SafeEmulator snapshots
  without widening Terminal.
consequences:
  positive:
  - Snapshot success is complete opaque ANSI from one state instant and failure cannot
    leak partial seed bytes.
  - Existing external Terminal implementers do not break when the new capability is
    added.
  negative:
  - Callers of the draft ReattachSnapshot signature must migrate to handle an error.
  - SafeEmulator holds its read lock through serialization, increasing contention
    during bounded whole-state snapshots.
  neutral:
  - Agent-grid retains opaque ANSI as its outbound format and keeps failure policy
    in the actor owner.
confirmation: Race tests run concurrent Write, Resize, and snapshot; agent-grid wired
  tests assert zero seed publication and unusable-session transition on typed failure.
---

## Context

PR #919 adds `ReattachSnapshot` only to concrete `Emulator`. Embedding promotes it through `SafeEmulator` without taking the mutex, and the race detector observes concurrent cell, scrollback, cursor, and provenance access. The draft API also cannot report invariant failure without returning guessed or partial ANSI. Adding it to the existing `Terminal` interface would break every external implementer even though the capability is new.

The accepted geometry-bearing attach ADR requires snapshot generation inside the actor commit, success publication as the linearization point, and session fail-fast for impossible VT failure.

## Decision

{% decision %}Define `(*Emulator).ReattachSnapshot() ([]byte, error)` and an explicit same-signature method on `*SafeEmulator`. The safe method holds the same state lock used by Write and Resize until immutable view validation and ANSI serialization finish. Do not add the method to x/vt's existing `Terminal` interface. Agent-grid declares its own narrow `Snapshotter` consumer interface and accepts only opaque bytes plus error.{% /decision %}

Invariant failure is an exported typed `SnapshotError` discoverable with `errors.As`; it contains diagnostic classification but no partial ANSI. The result is either complete bytes or an error with zero bytes. Illegal internal combinations are eliminated by semantic values and operation-local transitions; snapshot view construction performs the bounded whole-state validation needed to defend the public boundary without a full-history scan on every write.

During agent-grid attach commit, success publishes exactly one seed before live output. Typed failure publishes nothing, marks the session unusable, and severs subscribers. No old serializer or inferred-wrap fallback is permitted.

## Consequences

### Positive

{% consequence kind="positive" %}The public result is all-or-nothing, SafeEmulator cannot expose an unlocked promoted path, and agent-grid can test failure without learning semantic row types.{% /consequence %}

### Negative

{% consequence kind="negative" %}The new draft signature changes and serialization occurs under a read lock. A corrupted internal state intentionally loses the session instead of degrading display.{% /consequence %}

### Neutral

{% consequence kind="neutral" %}The released Terminal interface and external implementers remain unchanged. Outbound terminal data remains ANSI bytes and actor mailbox ordering remains the attach linearization mechanism.{% /consequence %}

## Alternatives

**Add ReattachSnapshot to Terminal.** Rejected because it creates an unnecessary breaking change for all existing implementers; the concrete x/vt method and consumer-owned narrow interface express the new capability.

**Copy state under lock and serialize after unlock.** Rejected as the initial contract because proving a complete deep copy for cells, styles, cursors, history, and provenance recreates alias risk; a later optimization requires its own evidence.

**Return best-effort ANSI on invariant failure.** Rejected because it makes silent terminal corruption a successful result and violates the accepted no-fallback decision.

## Confirmation

`go test -race` must report zero races for concurrent Write/Resize/Snapshot, typed-failure tests must observe zero returned bytes, and agent-grid actor tests must observe seed count zero plus unusable session state.
