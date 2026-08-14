---
id: adr-20260814-transport-close-reason-taxonomy
kind: adr
title: Publish a typed transport close reason on every server-initiated client socket
  close
status: accepted
created: '2026-08-14'
decision_makers:
- unknown
tags:
- protocol
- native-clients
- websocket
owners:
- agent-grid-maintainers
relations:
- {type: originatedFrom, target: change-20260812-cli-terminal-client}
- {type: references, target: adr-20260624-0011-two-step-ws-close-on-daemon-disconnect}
- {type: references, target: adr-20260727-exact-match-client-server-protocol}
- {type: references, target: change-20260814-cli-prerequisite-contracts}
source_paths:
- services/server/api/gateway.go
- services/server/api/mux.go
- services/server/api/auth.go
- services/server/api/protocol_version.go
- services/server/api/push_wire.go
- protocol/events.schema.json
consequences:
  positive:
  - Capacity, authentication, protocol, going-away, backend loss and an unattributed
    drop become six distinguishable observations from the public contract alone, where
    three of them are indistinguishable today.
  - Every client benefits from one value instead of three different guesses; the browser
    SPA cannot distinguish any of them today because it does not bind the close event
    at all.
  negative:
  - A closed vocabulary in the public contract cannot be extended without a contract
    change, and every close site in the gateway must be revisited to carry one.
  - The plain-text `unauthorized` body of a rejected ticket becomes JSON, which is
    a breaking edge on a body nothing currently parses.
  neutral:
  - The close code stays `StatusGoingAway` for every reason; the taxonomy lives in
    the reason slot so one vocabulary also covers pre-upgrade rejections, where no
    close code exists.
  - Subscribe-time refusals keep their existing `resource_exhausted` typing on the
    error frame; only connection-level departures gain a vocabulary.
confirmation: A test asserts every server-initiated close carries a reason from the
  published list, a second asserts the ticket and protocol-version rejections carry
  the same vocabulary in their JSON error body, and an enumerating gate fails when
  a close site uses a reason the schema does not list.
summary: Define a closed, published vocabulary of transport close reasons on the gateway
  socket and in pre-upgrade rejection bodies, so every client can tell capacity, authentication,
  going-away, backend loss and an unattributed drop apart.
updated: '2026-08-14'
---

# Publish a typed transport close reason on every server-initiated client socket close

## Context

A terminal client must tell its user why an attached session ended. The upstream experience contract requires explicit detach, transport loss, and a server-side session end to be told apart (UAC-018, UAC-019), and the failure-class contract requires the same input to reproduce the same exit code and the same output. Both assume that "transport loss" is a thing the client can characterise.

Measurement says it is not.

**One close code, four ad-hoc strings.** The gateway has exactly one close helper — `writeTypedClose(c, reason)` at `services/server/api/gateway.go:179-181` — and it always uses `websocket.StatusGoingAway` (1001). It carries one of four strings: `invalid-geometry` (`:192`), `subscribe-failed` (`:199-201`), `lifecycle-subscribe-failed` (`:303`), and `daemon-disconnected` (`:343`, `:352`, `:484`). `StatusPolicyViolation`, `StatusProtocolError` and `StatusTryAgainLater` are used nowhere in the repository.

**Most departures send nothing.** Every path that does not call `writeTypedClose` — context cancellation (`gateway.go:329-330`), a write error, and graceful server shutdown (`services/cmd/server/gateway.go:49`, `:95-100`) — unwinds into `defer func() { _ = c.CloseNow() }()` at `services/server/api/mux.go:613`, which closes abruptly with no close frame at all.

**Nothing is modelled.** `protocol/` holds seven `*.schema.json` files and none of them models the `{"k":"c"}` control frame that carries the application-level half of the two-step close. The only field named `reason` there is `protocol/events.schema.json:24`, a free-form justification on an approval request. The two enumerated reason fields, `ApprovalWire.resolution_reason` and `QuestionWire.resolution_reason`, describe why a human-input request resolved. There is no transport-close taxonomy anywhere.

**The one numeric slot is dead.** `controlMsg` is `{k, code int omitempty, data string}` (`services/server/api/wire.go:45-55`) and every production call site passes `0` (`gateway.go:199`, `:343`, `:352`, `:484`), so `code` is always absent from the wire and the free string `data` is the only discriminator.

**Rejections happen before a socket exists.** A bad or expired ticket is answered with HTTP 401 and the plain-text body `unauthorized` before `websocket.Accept` is reached (`mux.go:198-204`, `auth.go:20-23`), and `ticketStore.consume` collapses empty, unknown, already-used and expired into a single boolean (`ticket.go:66-81`). A protocol-version mismatch is answered with HTTP 426 and `{"code":"incompatible_protocol",…}` on REST (`protocol_version.go:39-50`) — and, measured separately, the **authenticated** WebSocket upgrade performs no version check at all (`mux.go:198-204`); only the no-auth branch checks.

Put together, the client-observable result is:

| Departure | What a client can observe today |
|---|---|
| daemon dies | `{"k":"c","data":"daemon-disconnected"}` then close 1001 with the same reason — **distinguishable** |
| server shuts down gracefully | nothing; code 1006, empty reason |
| ticket rejected or expired | nothing on the socket; a failed handshake, code 1006 |
| capacity exhausted (subscribe) | typed: `{"k":"e",…,"code":"resource_exhausted",…}` on the lifecycle route |
| capacity exhausted (connection) | nothing distinct |
| network drops | nothing; code 1006, empty reason |

Three of these are mutually indistinguishable, and the one that is distinguishable is so only through an application frame no schema describes.

### Why this is a permitted server-side contract change

The source requirement admits a prerequisite change on three conditions.

1. **The capability is absent from the execution plane.** Measured above: one close code, four unmodelled strings, an always-absent numeric slot, and most departures sending nothing.

2. **It cannot be expressed through the current public client contract.** A client that wants to separate a rejected credential from a pulled cable has nothing to read. No CLI-side rule manufactures the distinction; a heuristic over timing and in-flight operation would be a guess presented as a fact, and would be least reliable for the case that matters most.

3. **The browser and workspace clients benefit from the same fix.** This is the condition worth stating carefully, because it is the one an author is tempted to assert. The SPA does not merely ignore the close reason — it does not bind the event: `this.ws.onclose = () => this.handleClose(auth);` (`clients/ui/src/socket/connection.ts:400`), so `code` and `reason` are structurally unreachable, and a repository-wide search for `CloseEvent` across `clients/` finds nothing. Its `handleControl` recognises exactly two `data` literals and silently drops every other, including `resource_exhausted` (`connection.ts:586-595`). The Workspace renderer relay publishes `close` with no payload, discarding both fields (`clients/workspace/src/main/electron-entry.ts:488-491`), and its fleet prober classifies failures with regular expressions over `Error.message` strings (`clients/workspace/src/main/server-health.ts:126-131`) — a technique that exists precisely because there is no typed value to read. All three clients would read one value instead of guessing three different ways.

**Execution responsibility is not moved into the CLI.** The CLI publishes nothing and infers nothing; it reads the value the plane published and maps it to an exit class.

## Decision

**A closed, published vocabulary.** `going_away`, `backend_unavailable`, `unauthenticated`, `incompatible_protocol`, `capacity`, `invalid_request`, `internal_error`. It is published under `protocol/` together with the `{"k":"c"}` control-frame shape, which no schema models today.

**Every server-initiated departure carries exactly one of them**, in the WebSocket close frame's reason slot and — whenever the connection is still writable — in a control frame first, preserving the two-step shape `adr-20260624-0011-two-step-ws-close-on-daemon-disconnect` established. This includes the departures that send nothing today.

**Pre-upgrade rejections carry the same vocabulary** in their JSON error body's `code` field, because a rejection that happens before `websocket.Accept` cannot carry a close reason. One vocabulary therefore spans both sides of the upgrade.

**The close code stays 1001.** The taxonomy lives in the reason, not in a numeric code.

**A departure carrying no reason is the only case a client classifies locally**, and it is presented as an unattributed loss — never dressed as one of the named reasons. The sentinel is deliberately not a server-sendable value.

**A registration gate.** An enumerating repository check fails when a close site uses a reason the published list does not contain, so a fifth ad-hoc string cannot join the four that exist today.

**Out of scope.** Changing the browser or Workspace clients to consume the value. They gain the capability here and adopt it in their own change.

## Consequences

{% consequence kind="positive" %}
Six departure kinds become six observations. A script can tell "retry later" (`going_away`, `capacity`, `backend_unavailable`) from "retrying will never work" (`unauthenticated`, `incompatible_protocol`) without parsing prose, and the CLI's exit class follows a published value rather than a client-authored sentence. The capacity refusal stops needing an exit code of its own, because the discriminator is now in the public contract. The `{"k":"c"}` frame gains a schema, which also removes a divergence that has stood since June: `adr-20260624-0011` records the decision as `code:'daemon-disconnected'` while the implementation puts it in `data` and leaves `code` an always-omitted int, and nothing could catch that.
{% /consequence %}

{% consequence kind="negative" %}
A closed vocabulary in the public contract cannot be extended without a contract change, and every close site in the gateway must be revisited — including the paths that currently unwind into `CloseNow()` and therefore need a close frame they do not write today. The plain-text `unauthorized` body of a rejected ticket becomes JSON; nothing parses that body today, but it is a breaking edge and not an additive one. Graceful shutdown must walk live sockets to send `going_away`, which `srv.Shutdown` does not do now.
{% /consequence %}

{% consequence kind="neutral" %}
The close code stays `StatusGoingAway` for every reason, so no client that reads codes changes behaviour, and no close-code registry has to be invented or mirrored. Subscribe-time refusals keep their existing typing — the per-connection surface cap already answers `resource_exhausted` on an error frame over an open socket (`services/host/state/reduce_surface.go:108-110`) — so this change adds a vocabulary for connection-level departures rather than replacing one that works. A client that ignores the reason keeps working, because the close itself is unchanged.
{% /consequence %}

## Alternatives

- **Numeric close codes per cause** — rejected: it requires a code registry this design would have to invent and every client mirror, it creates a second vocabulary running parallel to the reason string with no mechanism keeping them in step, and it cannot cover the pre-upgrade rejections, where no close code exists at all.
- **Leave it, and let each client infer the cause from timing and from which operation was in flight** — rejected: that is a heuristic presented to the user as a fact, and it is least available exactly where it matters most. A rejected credential and a dropped packet are both "the handshake failed"; no amount of timing separates them.
- **Model only the `{"k":"c"}` control frame and leave the close reason as it is** — rejected: the control frame is best-effort and is not sent at all on the paths that unwind into `CloseNow()`, so the departures that carry no information today would still carry none.
- **Give the client a health probe to disambiguate after the fact** — rejected: it answers a different question (is the server reachable *now*) and cannot recover why *this* connection ended; a server that restarted between the drop and the probe reports healthy.
- **Extend the existing `proto.ErrCode` set and reuse it for closes** — rejected as the primary mechanism: those codes describe why a *request* failed and are already carried on the error frame, which is the right shape for a subscribe refusal and the wrong one for a departure that has no request to answer.

### If this is not accepted

The CLI's transport-loss class stays one word covering a rejected ticket, a draining server and a pulled cable, so `AC-disconnect-classes-distinct` holds only in the coarse sense that transport loss differs from detach and from session end. FR-024's capacity clause falls back to a CLI-authored reason string — deterministic within one build, but a per-implementer interface of exactly the kind the same requirement forbids for the exit codes. Automation that retries on exit `6` keeps retrying a connection that a rejected credential will never let succeed. `open-resource-exhausted-failure-class` reopens, because the argument for declining a dedicated capacity exit code is precisely that the discriminator lives in the public contract instead. The browser and Workspace clients keep their present position: one cause distinguishable out of six, through an application frame no schema describes.

Confirmation: `TestEveryServerInitiatedCloseCarriesAPublishedReason`, `TestTicketAndVersionRejectionsCarryTheSameReasonVocabulary`, `TestTransportCloseReasonVocabularyIsRegisteredAndExhaustive`, and `TestEachPublishedCloseReasonMapsToItsExitClass` on the CLI side.

## Related

- requirements: `FR-032`, `FR-014`, `FR-024`
- contracts: `contract-transport-close-taxonomy`, `contract-exit-classification`, `contract-attach-transport-topology`
- references: `adr-20260624-0011-two-step-ws-close-on-daemon-disconnect` (the two-step shape this preserves, and whose recorded field name diverges from the implementation), `adr-20260727-exact-match-client-server-protocol` (the compatibility failure this vocabulary names)
- open: `open-resource-exhausted-failure-class` (the budget-ownership half, which this does not close)
- plan: `docs/changes/change-20260812-cli-terminal-client/design-plan/design.md` anchor `adr-transport-close-reason-taxonomy`


{% transition from="proposed" to="accepted" date="2026-08-14" %}
consultation-20260814-cli-terminal-client-design-final
{% /transition %}
