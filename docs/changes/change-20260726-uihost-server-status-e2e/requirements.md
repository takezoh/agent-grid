---
change: change-20260726-uihost-server-status-e2e
role: requirements
---

<!-- lifecycle is owned by change.md -->

# Requirements

## Functional requirements

- The canonical shared-client UI/UX E2E boundary shall run a real `uihost` against a real `server`.
- The fixture shall inject a deterministic fake Codex without replacing browser `fetch` or `WebSocket`.
- The status scenario shall observe spinner-only, bell-only, bell-plus-attention, and attention-only states.
- The status scenario shall prove that attention state never renders the spinner.
- Windows E2E shall remain scoped to Shell WinUI/UIA and Workspace Electron-specific behavior.
