---
change: change-20260726-deterministic-codex-status-fidelity
role: implementation
---

<!-- lifecycle is owned by change.md -->

# Implementation

## Changes

1. Add `TestStreamServerRequestWiredDelivery` using a scripted
   `fake.TurnHandler`. It drives `thread/start` and `turn/start` over the same
   connection types used in production and observes the Backend runtime event.
2. Remove the live-model approval T3 and its misleading fidelity claim.
3. Document that real dependency fidelity must compare deterministic
   observables; model-selected tool behavior belongs to fake-driven E2E.

No production code or wire contract changes.
