---
change: change-20260726-windows-shell-deterministic-e2e
role: requirements
---

<!-- lifecycle is owned by change.md -->

# Requirements

- Real gateway T3 SHALL retain `/api/sessions` and ticket-to-WebSocket fidelity.
- Supervisor and composition semantics SHALL remain in always-on fake-backed tests.
- WinUI UIA SHALL receive deterministic session, approval, and question events
  through the production Shell REST/WS client.
- Session and pending UI assertions SHALL be mandatory rather than conditional
  on incidental real-server state.
