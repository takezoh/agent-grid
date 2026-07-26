---
change: change-20260726-deterministic-codex-status-fidelity
role: requirements
---

<!-- lifecycle is owned by change.md -->

# Requirements

## Functional requirements

- **FR-001:** A scripted app-server approval request SHALL traverse the real
  UDS/WebSocket connection, Backend observer, thread-to-frame routing, and
  `SubsystemApprovalRequested` emission in an always-on test.
- **FR-002:** UI status E2E SHALL use the deterministic fake app-server while
  retaining the real `server`, `uihost`, browser, and production DOM.
- **FR-003:** Real Codex T3 SHALL be reserved for deterministic app-server
  initialization, method-set, recorded lifecycle, and protocol compatibility.
- **FR-004:** No T3 SHALL require a model to choose a specific tool or produce
  an approval request.

## Acceptance

- The wired request-delivery test passes without authentication or network.
- The model-driven `TestStreamServerRequestE2EFidelity` no longer exists.
- Existing fake observer-routing contract and real app-server FakeVsReal tests
  remain available in their respective tiers.
