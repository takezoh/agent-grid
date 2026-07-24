# S3 prototypes checklist (manual on Windows)

Gate: `chunk-s1a-s3-prototypes-gate` — run before relying on toast-from-cold-start and engage-restore on a real desktop session.

## Prerequisites

1. **Stack (WSL)** — existing repo launcher, not a Windows-side server script:

```sh
make run-dev   # scripts/run-dev.sh: server + web, -no-auth on 127.0.0.1:8443
```

2. **Client (Windows)** — connect only:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File `
  ((wsl wslpath -w /workspace/agent-grid) + '\clients\windows-shell\scripts\dev-up.ps1')
# defaults: AG_NO_AUTH=1 against run-dev
```

## Cases

| # | Assumption | Steps | Pass criteria | On fail |
|---|---|---|---|---|
| 1 | `assumption-com-background-activation-unpackaged` | Quit Shell. Fire an approval from an agent. Click **Approve** on the toast without Shell already running. | Shell starts (or COM re-activates), approval resolves, panel queue clears. | Toast actions degrade to **Open panel** only (`AppNotificationToastService` already implements this fallback when `Register()` fails). Reopen DP-SUPERVISION-PRIMARY-ENTRY if product wants toast-as-primary. |
| 2 | `assumption-appnotification-textbox-ime` | Pending **question**. Inspect toast for inline text box; type Japanese via IME; submit. | IME composition commits; answer reaches daemon. | Keep **Answer in panel** only (current default). |
| 3 | engage-restore AttachThreadInput | Open VS Code; click Engage text box in panel; type; Send/blur. | VS Code is foreground again; Narrator still announces correctly. | Prefer SendInput-synthetic-key alternative (`implementation_decisions_remaining`). |

## Record outcomes

Copy this table into a dated note under `docs/changes/change-20260723-windows-shell-phase2/` when run:

```
date:
host:
#1 com-activation: pass|fail — notes
#2 ime-textbox: pass|fail — notes
#3 engage-restore: pass|fail — notes
```
