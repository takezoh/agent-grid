---
change: change-20260723-windows-shell-phase2
role: consultation
---

# S3 prototypes run log

Template for manual outcomes. Copy a new section per host run.

## Run — 2026-07-24 (client smoke)

- **Host**: DESKTOP-PJNFA6R / Windows 11 + WSL2 Ubuntu-22.04
- **Boundary**: client connects only; gateway was already up in WSL (not started by the client script).
- **WinUI**: process launched; `agent-grid://` registered to WinUI exe
- **Unit tests**: Core 86 + Platform 23 = **109 passed** (at last full run)

| # | Assumption | Result | Notes |
|---|---|---|---|
| 1 | COM background activation unpackaged | _pending eyes-on_ | fallback is Open-panel |
| 2 | AppNotification textbox IME | _pending eyes-on_ | Default Answer-in-panel |
| 3 | engage-restore AttachThreadInput | _unit green_ | Narrator still manual |

e2e stack is **`make run-dev` (or harness backend fixture) + Shell.Core T3 facts**.  
Canonical doc: `clients/windows-shell/docs/e2e.md`.  
UI smoke: `dev-up.ps1` with `AG_NO_AUTH=1`.

### Re-run automated e2e

```sh
make test-windows-shell-e2e
# or: ./clients/windows-shell/scripts/e2e.sh --start-run-dev
# two-terminal: make run-dev  +  ./clients/windows-shell/scripts/e2e.sh
```
