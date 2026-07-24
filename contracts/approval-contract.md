# Approval contract

**Change**: change-20260723-native-clients-phase01  
**Schema**: `protocol/events.schema.json`, `protocol/commands.schema.json`, `protocol/openapi.yaml`

## Lifecycle

`pending → resolved | expired | cancelled` (forward-only).

| Transition | Trigger | Decision | Broadcast |
|---|---|---|---|
| pending→resolved | CmdApprovalRespond (first commit) | client decision | EvtApprovalResolved (`resolution_reason=client`) |
| pending→expired | tick past `expires_at` | `default_decision` captured at creation (deny for destructive kinds) | EvtApprovalResolved (`resolution_reason=expired`) |
| pending→cancelled | frame/session teardown or CmdApprovalCancel | deny + connection-lost drain | EvtApprovalResolved (`resolution_reason=cancelled`) |

## Conflict

First-writer-wins under the single-writer Reduce loop. Loser receives `resolved_by_other` with winning `decision` and `resolving_client_instance_id`. No second EvtApprovalResolved.

## Identity

`resolving_client_instance_id` is the per-WS ephemeral id minted by ticketStore (FR-P0-12). Not a durable human identity.

## Expiry

- Default TTL: 30s at creation.
- No agent-side TTL extension.
- Mid-flight driver policy mutation must not flip `default_decision`.
