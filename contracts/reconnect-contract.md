# Reconnect contract

**Change**: change-20260723-native-clients-phase01  
**Extends**: ADR-0025 (REST backfill + WS tail), ADR-0011 (two-step close), ADR-0022 (subscribe retry)

## Additive extension (FR-P0-08 / FR-P1-11)

On resubscribe/hello, the authoritative pending ApprovalRequest and QuestionRequest set for the session is included. Clients do **not** require a full event-log replay for pending human-input state.

## Unchanged

- ADR-0025 REST transcript backfill then WS tail
- ADR-0011 two-step WS close on daemon disconnect
- ADR-0022 subscribe retry in the socket layer
- ADR-0023 / ADR-20260705 viewUpdate sessions-only + discriminated-union `k`

## Convergence

Reconnecting client snapshot + still-connected client broadcasts must converge to host/state's pending set.
