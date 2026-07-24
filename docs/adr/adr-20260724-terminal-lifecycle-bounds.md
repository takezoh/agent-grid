---
id: adr-20260724-terminal-lifecycle-bounds
kind: adr
title: Bind terminal recovery, correlation, teardown and diagnostics to fixed structural
  bounds
status: accepted
created: '2026-07-24'
decision_makers:
- unknown
tags:
- terminal
- liveness
- lease
- correlation
- observability
owners:
- server/api
- host/runtime
- clients/ui
relations:
- {type: references, target: adr-20260711-keep-single-ipc-connection-topology}
- {type: references, target: adr-20260711-server-initiated-severance-signal}
- {type: references, target: adr-20260711-extend-sever-not-drop-shared-ipc-hops}
- {type: references, target: adr-20260711-priority-lane-interactive-vs-bulk}
- {type: references, target: adr-20260715-geometry-bearing-terminal-attach}
- {type: originatedFrom, target: change-20260724-terminal-lifecycle-liveness}
source_paths:
- src/server/api
- src/host/runtime
- src/platform/termvt
- clients/ui/src/socket
summary: Separate browser delivery observation from remote truth, use fresh correlation
  on recovery, and bind stage diagnostics and terminal ordering to fixed bounds.
consequences:
  positive:
  - Delayed status recovers without rewriting remote truth, and old delivery cannot
    affect the replacement.
  negative:
  - Low-rate and burst diagnostics are bounded and diagnostic failure remains observable.
  neutral:
  - Additional public correlation, relay barrier, retry and producer-slot tests are
    required.
confirmation: Fake-clock Connection, stale-fence, SurfaceLease, low-rate/burst dirty-slot,
  diagnostic failure, finalSequence barrier, gateway, and Playwright tests enforce
  the fixed contracts.
updated: '2026-07-24'
---

# Context

Daemon authority does not by itself answer whether an outcome reached the browser, prevent old output from rendering after reconnect, physically reclaim a subscriber, or keep rare-failure diagnostics observable when their own lane fails.

# Decision

Browser publication owns an exclusive connection-attempt state machine:

```text
observing -> observed_remote | delivery_timeout | socket_closed
```

Only a matching authoritative terminal outcome yields `observed_remote`; accepted/waiting keep the watchdog active. Before N+1 enqueue, the browser CASes observing N to local `publication_replaced`, cancels only N's watchdog, and keeps the socket open; gateway pending-slot disposal emits no status. At four seconds, deadline-first compare-and-set records `delivery_timeout` before closing. An external close can instead select `socket_closed`. Both recovery states increment connection generation and obtain a fresh ticket, private owner mapping, and client revision before replay. Ticket/socket failure remains visibly reconnecting and retries at capped intervals.

Browser-visible correlation is `{clientInstanceID, connectionGeneration, clientRevision}`. Gateway maps it to a private owner derived from ticket, nonce, and daemon IPC generation. Private authority never reaches the browser. Every command, effect, status, output, release, and severance is fenced at its consumer by the correlation dimensions available there.

Telemetry has one single-writer dirty/latest slot per actual producer: daemon for accepted/applied/output, gateway for forwarded/delivery gap, browser for received/rendered. A 250-millisecond monotonic tick atomically snapshots dirty state. Concurrent/later updates remain dirty. Emission is maximum 4 Hz, with no sequence-count prerequisite.

Terminal and close markers carry `finalSequence`. Gateway sends them only after all admitted output through that sequence is forwarded or each gap is explicitly attributed. The marker immediately includes final watermark and local diagnostic drop counters. Diagnostic-lane loss therefore cannot hide itself; barrier timeout or status-lane failure closes the socket. `no_output` derives only from actor-applied `outputSeq=0` and is exclusive with drop/unknown.

Surface acquisition is context-aware; idempotent release physically removes subscriber map/channel/fanout within 100 milliseconds while preserving the PTY. Renewal is four seconds, expiry twelve, owners eight per IPC generation, and healthy shared IPC is not recycled for one owner.

# Alternatives

**Browser-local degraded RevisionOutcome.** Rejected because it duplicates daemon authority.

**Output-count plus time threshold.** Rejected because it can make the first low-rate event invisible.

**Terminal marker on an independent lane without a relay barrier.** Rejected because it can overtake already admitted output.

**Report diagnostic loss only on the diagnostic lane.** Rejected because the failure can suppress its own evidence.

# Consequences

{% consequence kind="positive" %}Delivery recovery, stale isolation, teardown, low-rate diagnosis, and terminal/output order become falsifiable.{% /consequence %}

{% consequence kind="negative" %}Public correlation, retry reduction, producer slots, relay barriers, acknowledgements, and tests add implementation surface.{% /consequence %}

{% consequence kind="neutral" %}Browser delivery deadline, daemon apply deadline, owner lease expiry, and SurfaceLease cleanup remain distinct clocks and responsibilities.{% /consequence %}


{% transition from="proposed" to="accepted" date="2026-07-24" %}
ユーザーが全設計を承認し、transport recovery と telemetry bounds を実装開始
{% /transition %}
