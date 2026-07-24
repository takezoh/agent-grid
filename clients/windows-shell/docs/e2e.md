# Windows Shell e2e (Phase 2)

## Principle

| Layer | Owns | Does not own |
|---|---|---|
| **WSL `make run-dev`** | Full stack fixture (`scripts/run-dev.sh`: server+web, `-no-auth` loopback) | Shell UI |
| **Windows Shell client** | Connect (REST/WS), panel, toast, tray | Starting the gateway |
| **e2e harness** | Opt-in T3 tests; optional **backend** fixture with the same flags as run-dev | Product client code paths |

The Shell **client** never starts `server`. Product-time adopt/spawn is `DaemonSupervisor` inside the Shell process.

For **tests**, prefer a live `make run-dev`. The harness `--start-run-dev` starts only the **gateway binary** with the same `-insecure -no-auth -addr 127.0.0.1:8443 -data-dir .run-dev/server` posture as `scripts/run-dev.sh` (web/`npm` not required for Shell Core e2e).
## Stack

```text
WSL:   make run-dev
         → ./server -insecure -no-auth -addr 127.0.0.1:8443 -data-dir $ROOT/.run-dev/server
         → ./web    -insecure -addr 127.0.0.1:8080 -server http://127.0.0.1:8443

Win:   AG_E2E_RUN_DEV=1  → xUnit T3 facts (probe / ticket+WS / adopt / composition)
       scripts/dev-up.ps1 → optional manual WinUI against same gateway (AG_NO_AUTH=1)
```

## Commands

### A. Two-terminal (recommended)

```sh
# Terminal A — WSL
make run-dev

# Terminal B — WSL (tests via Windows dotnet robocopy tree when available)
./clients/windows-shell/scripts/e2e.sh
# or e2e facts only:
./clients/windows-shell/scripts/e2e.sh --skip-unit
```

### B. One-shot harness

```sh
./clients/windows-shell/scripts/e2e.sh --start-run-dev
# starts run-dev *backend* (server -no-auth) on http://127.0.0.1:18443
# (dedicated port so a stale :8443 cannot steal the bind),
# runs tests, tears down the fixture
```

When a full `make run-dev` is already on `:8443`, omit `--start-run-dev` and the harness attaches to it.

### C. From Windows PowerShell

```powershell
# With run-dev already up in WSL:
powershell.exe -NoProfile -ExecutionPolicy Bypass -File `
  ((wsl wslpath -w /workspace/agent-grid) + '\clients\windows-shell\scripts\e2e.ps1')

# Or start fixture from harness:
...\e2e.ps1 -StartRunDev
```

### D. Manual UI smoke (not automated T3)

```sh
# Terminal A
make run-dev

# Terminal B
powershell.exe -NoProfile -ExecutionPolicy Bypass -File \
  "$(wslpath -w /workspace/agent-grid)/clients/windows-shell/scripts/dev-up.ps1"
# AG_NO_AUTH=1 by default — matches run-dev
```

## Test inventory (T3, opt-in)

| Test | Asserts |
|---|---|
| `Probe_sessions_against_run_dev_no_auth` | `GET /api/sessions` via `ShellGatewayClient` + `NoAuthTokenSource` |
| `Mint_ticket_and_open_websocket` | `POST /api/ws-ticket` + `ClientWebSocket` open |
| `DaemonSupervisor_adopts_running_run_dev_without_spawn` | adopt-before-spawn: **Connected/Adopted**, spawn count **0** |
| `Composition_root_connects_with_no_auth_against_run_dev` | full composition wiring against real gateway |

Enable: `AG_E2E_RUN_DEV=1`. Skip otherwise (always-on `dotnet test` stays green without fixture).

Filter:

```sh
dotnet test --filter "FullyQualifiedName~E2E"
```

## Env

| Variable | Default | Meaning |
|---|---|---|
| `AG_E2E_RUN_DEV` | unset | `1` enables T3 facts |
| `AG_E2E_GATEWAY_URL` | `http://127.0.0.1:8443` | run-dev backend |
| `AG_NO_AUTH` | (UI) | `1` for WinUI against run-dev |

## Out of scope (other gates)

| Concern | Entry |
|---|---|
| WSL detach survival (setsid) | `docs/wsl-detach-spike-*.md`, `scripts/wsl-detach-spike.sh` |
| S3 COM / IME / Narrator | `docs/s3-prototypes-checklist.md` |
| Workspace Playwright Electron | clients/workspace (future) |
| Go gateway / fake-vs-real | `make test-e2e` |

## Relation to verification member

`docs/changes/change-20260723-windows-shell-phase2/verification.md`:

- `verify-native-ws-ticket-fidelity` → e2e `Mint_ticket_and_open_websocket`
- `verify-supervisor-partition` adopt path → e2e `DaemonSupervisor_adopts_*`
- always-on unit tests cover pure machines without run-dev
