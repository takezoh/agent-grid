# S3 prototypes gate (`chunk-s1a-s3-prototypes-gate`)

Conditional regression detector before entering panel-glance / approval-round-trip UI work.

| Assumption | What to prototype on a Windows machine | On fail |
|---|---|---|
| `assumption-com-background-activation-unpackaged` | AppNotification COM background activation works unpackaged | Reopen `DP-SUPERVISION-PRIMARY-ENTRY`; toast actions fall back to panel-expand only |
| `assumption-appnotification-textbox-ime` | Toast inline textbox accepts IME input for question answers | Questions engage via panel only |
| engage-restore AttachThreadInput + screen-reader | Confirm/cancel restores prior HWND without breaking Narrator | Prefer SendInput-synthetic-key alternative (`implementation_decisions_remaining`) |

This gate does **not** block S1 entry. Record outcomes under
`docs/changes/change-20260723-windows-shell-phase2/` when prototypes run.

## Running from this WSL host

Windows tooling is reached via PowerShell:

```powershell
# Shell unit tests (robocopy + local disk)
powershell.exe -NoProfile -ExecutionPolicy Bypass -File `
  ((wsl wslpath -w /workspace/agent-grid) + '\clients\windows-shell\scripts\win-test.ps1')

# WinUI panel + AppNotification build
powershell.exe -NoProfile -ExecutionPolicy Bypass -File `
  ((wsl wslpath -w /workspace/agent-grid) + '\clients\windows-shell\scripts\win-build-winui.ps1')
```

Manual S3 checklist: [s3-prototypes-checklist.md](./s3-prototypes-checklist.md).

WSL detach spike (T3) — accepted 2026-07-24; see [wsl-detach-spike-result.md](./wsl-detach-spike-result.md).
