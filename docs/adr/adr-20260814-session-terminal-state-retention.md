---
id: adr-20260814-session-terminal-state-retention
kind: adr
title: Retain an ended session with a terminal lifecycle state and publish the transition
status: accepted
created: '2026-08-14'
decision_makers:
- unknown
tags:
- server
- session-lifecycle
- protocol
owners:
- agent-grid-maintainers
relations:
- {type: originatedFrom, target: change-20260812-cli-terminal-client}
- {type: references, target: design-client}
- {type: references, target: change-20260814-cli-prerequisite-contracts}
source_paths:
- services/host/state/reduce_session.go
- services/host/state/reduce_frame_evict.go
- services/host/state/reduce_tick.go
- services/host/state/view/status.go
- services/server/api/gateway.go
consequences:
  positive:
  - The stopped lifecycle state that the protocol already declares and every client
    already renders becomes reachable instead of unreachable on three of four termination
    paths.
  - One change closes post-termination observability, ended-session listing after
    a self-exit, and the missing discriminator for a server-side session end during
    attach.
  negative:
  - Retained rows accumulate, so a retention window and a reclamation owner must be
    decided, and consumers that treated absence as "ended" now see a row.
  neutral:
  - Frame and process teardown are unchanged; only the fate of the session record
    and what is published change, and no responsibility moves into a client.
confirmation: Four tests that are red today — a terminal row survives explicit termination,
  survives a clean or signalled self-exit and a vanished frame, the transition is
  published on the lifecycle socket, and an ended session is distinguishable from
  an identifier that never existed.
summary: Keep an ended session observable with a terminal lifecycle state on all four
  termination paths and publish the transition, because the public contract declares
  a stopped state the execution plane deletes rather than publishes.
updated: '2026-08-14'
---

# Retain an ended session with a terminal lifecycle state and publish the transition

## Context

The public protocol declares a terminal session state and every shipping client renders it. `protocol/session-status.schema.json` defines `lifecycle ∈ {active, idle, stopped}`; `protocol/session-status-display.json` gives `stopped` the label `Stopped`, a glyph and its own precedence slot; `services/host/state/view/status.go:112,140-150` maps `StatusStopped` to `LifecycleStopped`.

That mapping can only run while the session record still exists, and measurement shows the record is deleted on three of the four ways a session ends.

| Path | Reducer | Today |
|---|---|---|
| explicit terminate | `reduceStopSession` | `delete(s.Sessions, sid)`; durable snapshot deleted (`reduce_session.go:313-329`, `runtime.go:789-792`) |
| self-exit, code ∈ {0,129,130,137,143} | `reduceFrameCommandExited` → `evictFrame(…, true)` | session deleted by `evictRootFrame` (`reduce_tick.go:75-115`, `reduce_frame_evict.go:29-32`) |
| backend frame vanished | `reduceFrameVanished` → `evictFrame(…, false)` | session deleted (`reduce_tick.go:49-57`) |
| self-exit, other exit codes | `reduceFrameCommandExited` | frame retained with driver `Stopped`; **row survives** |

So a `stopped` row is reachable on exactly one path in four, and that one is the crash path — the state is effectively an accident rather than a contract. A client cannot distinguish "this session ended" from "this identifier never existed", because after deletion the only observable is absence.

### Why this is a permitted server change

The source requirement permits a server change only when all three of its conditions hold, and forbids CLI-motivated server changes otherwise.

1. **The capability is absent from the execution plane.** Measured in the table above: three of four paths delete the record, and the fourth retains it as a side effect of crash handling rather than as a contract.
2. **It cannot be expressed through the current public client contract.** Once the record is gone, `GET /api/sessions` and the lifecycle view carry only absence. No public field separates ended from never-existed, and no ordering of public calls recovers it.
3. **The browser and workspace clients benefit from the same fix.** This is the condition that must be evidenced rather than asserted, and it is: the protocol schema, the shared display matrix, the server's own status mapping, and the rendering code in both the SPA and the C# panel all carry presentation for `stopped`. Every shipping client already presents a state the plane can essentially never publish — the SPA shows a row disappearing where the contract says it should show `Stopped`. The defect is shared, and the fix is a public-contract fix, not a CLI accommodation.

## Decision

A session that ends — by explicit termination, by a clean or signalled self-exit, or by its backend frame vanishing — is retained with a terminal lifecycle state instead of being deleted from the session map, and the transition is published on the lifecycle socket.

Frame teardown, process teardown and resource release are unchanged. Only the fate of the session record and what is published change.

**Execution responsibility stays with the execution plane.** No client keeps a private record of terminated sessions to compensate. A client-side store would violate the rule that the execution plane owns session state, and it would make two clients disagree about which sessions ever existed — one remembering what the other never saw.

The retention window and what reclaims a terminal row are deliberately **not** decided here. That choice is product-visible (it decides what a listing shows an hour later) and belongs with the durable-snapshot owner.

## Consequences

{% consequence kind="positive" %}
The `stopped` state becomes reachable, so the protocol, the display matrix and three clients' rendering code stop describing something that cannot happen. One change closes three separate gaps: an explicit termination is observable afterwards; a session that exited on its own is still listed with a state change instead of vanishing; and an attached client gains the observation that attributes a departure to a session end rather than to a lost connection — which removes the risk that the three-way departure distinction is permanently unsatisfiable.
{% /consequence %}

{% consequence kind="negative" %}
Terminal rows accumulate, so a retention window and a reclamation owner have to be decided before this ships; without them the session map grows without bound. Consumers that implicitly treated absence as "ended" now see a row and must render it, which is the intended change but is still a behaviour change for the SPA and the panel.
{% /consequence %}

{% consequence kind="neutral" %}
Additive for readers: no field changes shape, and a previously unreachable enum value becomes reachable. Sessions that ended before the change are already gone and are not reconstructed. Rollback is restoring the deletes, with no data migration. The reducer change and the publication must land together — a retained record that is not published would leave an attached client no better off.
{% /consequence %}

## Alternatives

- **Have each client remember the sessions it terminated** — rejected: it moves execution-plane responsibility into clients, violates the no-local-authority rule, and produces clients that disagree about history depending on which one issued the command.
- **Report success on the acknowledgement and say nothing about afterwards** — this is what the CLI does *while this ADR is unresolved*, and it is exactly the degraded state described below. It is a fallback, not a solution.
- **Retain only on explicit termination** — rejected: it fixes one of four paths and leaves the self-exit paths, where a session ends without anyone asking, still vanishing; the self-exit case is the one a user is most likely to be surprised by.
- **Add a separate "recently ended" query instead of retaining the row** — rejected: it creates a second listing surface with its own consistency rules, where the existing listing already has a state column built for exactly this.

### If this is not accepted

The CLI ships with these observable gaps, all recorded rather than papered over:

- An explicit termination reports only that the request was acknowledged as applied. The following listing shows the target **absent**, not ended, so the upstream acceptance scenario's second half is unmet.
- A session that exits cleanly, is killed by a standard signal, or whose backend frame vanishes disappears from the listing rather than changing state, so the upstream ended-sessions-remain-listed scenario holds only for abnormal exit codes.
- The departure classification degrades from three classes to two: explicit detach versus an undifferentiated remote departure. Transport loss and a server-side session end become indistinguishable from the client's side.

None of these is worked around client-side, because every available workaround is a local record of session history, which is forbidden.

Confirmation: Four tests, all red today by construction: `TestStopSessionRetainsATerminalRow`, `TestIntentionalExitAndFrameVanishRetainATerminalRow`, `TestLifecycleSocketPublishesTerminalTransition`, and `TestEndedSessionIsDistinguishableFromUnknownIdentifier`.

## Related

- requirements: `FR-026`, `FR-015`, `FR-014`, `FR-023`
- contracts: `contract-session-terminal-state-retention`, `contract-explicit-terminate`, `contract-exit-classification`
- open: `open-terminate-observability` (accept the prerequisite, or accept the degraded v1; and if accepted, the retention window and its reclamation owner)
- plan: `docs/changes/change-20260812-cli-terminal-client/design-plan/design.md` anchor `adr-cli-session-terminal-state-retention-prerequisite`


{% transition from="proposed" to="accepted" date="2026-08-14" %}
consultation-20260814-cli-terminal-client-design-final
{% /transition %}
