---
change: change-20260726-uihost-server-status-e2e
role: implementation
---

<!-- lifecycle is owned by change.md -->

# Implementation

## Changes

- Add a separate Playwright real-stack project and process fixture.
- Extend the existing high-fidelity fake app-server with opt-in prompt markers for a delayed active turn and a pending approval request.
- Exercise session creation and terminal input through real HTTP/WebSocket routes and assert the resulting production DOM.
- Persist the UI/UX and Windows E2E ownership boundary in host design, testing guidance, and repository commands.
