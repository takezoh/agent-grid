# WSL detach spike result

- **Date**: 2026-07-24
- **Gate**: `wsl-detach-survival-verification` (`contract-b3-wsl-detach-mechanism`)
- **Candidate**: `setsid nohup … &` with `-data-dir /tmp/… -token-file … -insecure`
- **Runner**: `clients/windows-shell/scripts/wsl-detach-spike.sh` (via `wsl-detach-spike.ps1` / direct bash)

## Observations

| Check | Result |
|---|---|
| Spawn returns PID | yes |
| `/api/sessions` immediately after spawn | HTTP 200 |
| Same after ≥5s (launcher already returned) | HTTP 200 |
| PID still in `/proc` | yes |

## Notes

- Default `~/.agent-grid` on this machine is not writable for `server.log` (open → EACCES despite mode 0755). Spike and `WslDaemonRunner` therefore use an explicit `-data-dir` under `/tmp/agent-grid-data` (or `/tmp/ag-spike-data` for the spike).
- PPid after setsid is not always `1` (intermediate session leader remains); survival criterion is process liveness + authenticated `/api/sessions`, not reparent-to-1 alone.
- **Verdict**: candidate **accepted** for Phase 2. systemd --user fallback not required.

## Commands re-run

```sh
# In WSL
make build-server
bash clients/windows-shell/scripts/wsl-detach-spike.sh

# From Windows PowerShell / WSL host
powershell.exe -NoProfile -ExecutionPolicy Bypass -File `
  ((wsl wslpath -w /workspace/agent-grid) + '\clients\windows-shell\scripts\wsl-detach-spike.ps1')
```
