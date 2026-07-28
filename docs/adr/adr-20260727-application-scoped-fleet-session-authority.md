---
id: adr-20260727-application-scoped-fleet-session-authority
kind: adr
title: Application-scoped fleet session authority
status: accepted
created: '2026-07-27'
decision_makers:
- unknown
tags:
- fleet-runtime
- client-host
owners: []
relations:
- {type: originatedFrom, target: change-20260727-multi-server-session-palette}
- {type: references, target: adr-20260727-host-neutral-fleet-provider}
- {type: references, target: adr-20260727-server-qualified-identity-partial-failure}
- {type: references, target: adr-20260714-workspace-session-switch-lifecycle}
source_paths:
- clients/ui
- clients/workspace
- clients/web-host
summary: One host runtime owns all server session membership and publishes the same
  fleet to every view.
consequences:
  positive:
  - Every view receives one authoritative cross-server fleet and create visibility
    no longer depends on the selected server.
  negative:
  - Both hosts and all raw-ID consumers must migrate in one flag-day integration,
    with bounded publication and lifecycle tests.
  neutral:
  - Workspace and Web retain different delivery transports; profile/palette and dirty-switch
    semantics remain separately governed.
confirmation: Run the shared two-server conformance trace against Web and Workspace;
  open two views, create on B from A, and require deep-equal fleet revisions with
  no additional runtime/window.
---

# Application-scoped fleet session authority

## Context

The proven A/B defect is a responsibility gap: host-level provider code owns configured multi-server discovery and create, but each renderer constructs one `Connection` from `currentServerId` and replaces the global session store from that server. The accepted host-neutral provider ADR forbids host-kind cardinality, and the accepted Workspace switch ADR forbids mixing old dirty Workspace state with a new active session.

## Decision

Each Web or Workspace host process owns exactly one configured `FleetRuntime` for all enabled servers. It is the only writer of fleet membership, member generation/freshness, create correlation and subscriber publication state. It publishes one credential-free, monotonically revisioned immutable full `FleetSnapshot` to every live view. Views own only `activeSessionRef` and use the accepted guarded `selectSession` transaction; `serverId` is an internal routing identity and never creates a process, BrowserWindow, renderer partition, fleet store or implicit default server.

Workspace Electron main and Web host independently realize the same protocol; only preload/IPC versus same-origin HTTP/WS differs. High-frequency terminal/transcript traffic uses a separately routed active-session channel. One failed member retains only an explicitly stale last-known snapshot, and confirmed create requires authoritative same-or-newer lifecycle observation.

## Consequences

{% consequence kind="positive" %}
All views observe one fleet and cross-server create/selection outcomes become structurally testable.
{% /consequence %}

{% consequence kind="negative" %}
Provider, both hosts, SPA consumers and Workspace window lifecycle must migrate together; bounded mailbox and teardown logic become explicit responsibilities.
{% /consequence %}

{% consequence kind="neutral" %}
Existing typed launch profiles, palette form, server-owned argv materialization, dirty-switch confirmation and host-specific delivery transports remain unchanged.
{% /consequence %}

## Alternatives

- Per-renderer or per-window aggregation was rejected because it recreates split authority and divergent revisions.
- Renderer-owned credentialed fan-out was rejected because it violates the accepted trust boundary.
- Appending POST create responses directly to the list was rejected because it invents a second membership authority.

## Confirmation

Run the same deterministic two-server fixture through Web and Workspace with two views, A/A control, A/B create, collision, partial failure, dirty switch and slow subscriber; normalized snapshots/outcomes must match and no second runtime/window may be constructed.
