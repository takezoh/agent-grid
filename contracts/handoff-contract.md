# Handoff contract (skeleton)

**Change**: change-20260723-native-clients-phase01  
**Deep links**: `protocol/deep-links.schema.json`

## URI shapes

Adopted from `plans/remote-control-mobile-session-deep-link.md`:

- `agent-grid://session/<id>`
- `agent-grid://approval/<id>`

Generated SDKs expose typed parse/construct helpers; native shells never hand-parse URI strings (FR-P1-09).

## Phase scope

Full multi-client handoff orchestration is deferred past Phase 0/1. This document pins the URI contract only.
