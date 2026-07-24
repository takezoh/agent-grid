# Question contract

**Change**: change-20260723-native-clients-phase01

## Shape

Phase 0/1 free-text only: `answer` is a single string (`HumanInputRequest.free_text`). Structured answers are rejected with 400 at the wire layer.

## Lifecycle

Same as approval: pending → resolved | expired | cancelled, first-writer-wins, teardown drains held driver JSON-RPC with connection-lost.
