---
id: adr-20260814-cli-attach-via-lifecycle-multiplexed-ws
kind: adr
title: Attach the terminal CLI over the lifecycle-multiplexed WebSocket instead of
  the per-session route
status: accepted
created: '2026-08-14'
decision_makers:
- unknown
tags:
- native-clients
- cli
- websocket
owners:
- agent-grid-maintainers
relations:
- {type: originatedFrom, target: change-20260812-cli-terminal-client}
- {type: references, target: design-client}
source_paths:
- services/server/api/gateway.go
- services/host/state/reduce_surface.go
- clients/cli/internal/attach
consequences:
  positive:
  - The CLI observes lifecycle state changes, so a server-side session end can be
    told apart from a lost connection.
  - Each socket carries its own subscriber identity, so a second attach to the same
    session gets its own snapshot and one detach does not drop the other client.
  negative:
  - The CLI must demultiplex by session id on every output frame and must never present
    a non-terminal subscription outcome as established.
  - The per-connection surface-subscribe capacity is shared with the graphical clients,
    so a busy fleet can refuse a CLI attach for capacity.
  neutral:
  - This is a choice between two existing public routes; no new endpoint, authentication
    path, or frame type is introduced.
confirmation: Tests assert two concurrent attaches each receive a snapshot, that no
  code path uses the per-session route, and that accepted/waiting outcomes are never
  reported as established.
summary: Attach uses GET /ws?ticket= with a lifecycle desired-state frame rather than
  GET /ws?session=, because the per-session route cannot observe session end and collides
  on an empty subscriber id.
updated: '2026-08-14'
---

# Attach the terminal CLI over the lifecycle-multiplexed WebSocket instead of the per-session route

## Context

Two public WebSocket routes can carry a session's surface. The obvious-looking one for a CLI is the per-session route, `GET /ws?session=<id>`, whose whole job is one session. Measurement disqualifies it on two independent grounds, each sufficient on its own.

First, lifecycle blindness. `writeOutbound` (`services/server/api/gateway.go:476-520`) forwards only `EvtSurfaceOutput`, `EvtSessionFileLine` and `EvtAgentNotification`, and explicitly drops `EvtSessionsChanged`; the channel closes only when the *daemon* disconnects. If the attached session ends server-side, no frame reaches the client at all. The user keeps typing at a frozen screen, and the requirement that a departure be attributable cannot hold.

Second, subscriber collision. `DaemonAdapter.SubscribeSurface` sends an empty `SubscriberID` (`gateway.go:100`), while the daemon keys subscriptions on `(ConnID, SessionID, SubscriberID)` with a `ConnID` shared by every WebSocket client. A second per-session attach to the same session collides on that key: `reduceSurfaceSubscribe` (`services/host/state/reduce_surface.go:117-119`) returns ok without emitting `EffSurfaceSubscribeStart`, so the second client never receives its snapshot, and the first client's detach unsubscribes the shared key and drops both.

The lifecycle route assigns a distinct subscriber identity per socket (`web-<n>` for surface subscription, `owner-<random24>` for lifecycle ownership, `gateway.go:308-310`) and is the route the shipping browser and workspace clients already use.

## Decision

Attach opens `GET /ws?ticket=<t>` with no session query parameter and makes the target session's surface desired through the lifecycle desired-state frame. The per-session route is not used anywhere in the CLI.

Because the socket is multiplexed, the CLI drops every output frame whose carried session id differs from the attached session, and it treats a subscription outcome of `accepted` or `waiting` as non-terminal — never as an established subscription.

## Consequences

{% consequence kind="positive" %}
Lifecycle state changes are observable, which is what makes a server-side session end separable from a transport loss. A distinct subscriber identity per socket means concurrent attachment works: both parties get their own snapshot, and one detaching does not unsubscribe the other.
{% /consequence %}

{% consequence kind="negative" %}
The CLI carries demultiplexing responsibility: an output frame belonging to another subscribed session must never reach stdout, which is a real leak the repository has already seen once. The per-connection surface-subscribe capacity of eight is keyed on the single gateway-to-daemon `ConnID` (`reduce_surface.go:106-110`), so it is shared with the graphical clients and a sufficiently busy fleet refuses the CLI attach for capacity. That refusal is contracted as a transport failure with a capacity reason; whether it deserves a dedicated exit code is left open.
{% /consequence %}

{% consequence kind="neutral" %}
This selects between two routes that already exist. No new endpoint, authentication path or frame type is added, and the WebSocket ticket admission flow is unchanged.
{% /consequence %}

## Alternatives

- **`GET /ws?session=` alone** — rejected on the two measured counterexamples above: no lifecycle observability, and subscriber-key collision that breaks the snapshot and the detach of concurrent clients.
- **Two sockets, one per-session plus one lifecycle** — rejected: classification becomes possible, but the failure surface doubles. Which socket dropped first decides the reason presented, so the departure reason becomes non-deterministic for the same underlying event.

Confirmation: `TestConcurrentAttachEachReceivesSnapshot` (two attaches, both snapshotted, neither dropped by the other's detach), `TestAttachNeverUsesPerSessionRoute` (no code path reintroduces the per-session route), `TestNonTerminalOutcomesAreNotEstablished`, and `TestForeignSessionOutputNeverReachesStdout`.

## Related

- requirements: `FR-007`, `FR-009`, `FR-014`, `NFR-006`
- contracts: `contract-attach-transport-topology`
- references: `design-client` (governs `services/server/api/`, where every counterexample above was measured)
- open: `open-resource-exhausted-failure-class` (whether capacity gets its own exit code)
- plan: `docs/changes/change-20260812-cli-terminal-client/design-plan/design.md` anchor `adr-cli-attach-via-lifecycle-multiplexed-ws`


{% transition from="proposed" to="accepted" date="2026-08-14" %}
consultation-20260814-cli-terminal-client-design-final
{% /transition %}
