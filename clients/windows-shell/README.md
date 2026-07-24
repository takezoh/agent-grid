# Agent Grid — Windows Shell (Phase 2)

Native supervision surface for Windows: always-on panel, tray, toast, deep links,
daemon supervision of the WSL-hosted `server` binary. Workspace windows are owned
by the sibling Electron app at `clients/workspace/`.

## Configuration

Shell and Workspace share the four JSON files described in
[`../desktop-config/README.md`](../desktop-config/README.md). The default
directory is `%APPDATA%\agent-grid\config`; `--config-dir <path>` replaces it
for both apps and is the required test-isolation mechanism.

Every enabled `servers.json` entry gets an independent supervisor and gateway
connection. Desktop session identity is `{serverId, sessionId}`; requests sent
on that server connection continue to carry only `sessionId`.

## Layout

```text
AgentGrid.Shell.Core/       UI-independent pure logic + thin I/O shells (xUnit target)
  DaemonSupervisor/         Boundary-3 state machine (adopt-before-spawn)
  GatewayClient/            Boundary-2 token + ticket + probe
  SupervisionState/         Pure reducer (optimistic approve + resolved-by-other)
  DeepLinkRouter/           agent-grid:// routing + Track A alias table
  WorkspaceLauncher/        Boundary-1 control envelope + launch retry
  Health/                   Tray appearance mapping (no toast dependency)
AgentGrid.Shell.Platform/   Win32 interop behind IWin32InteropService
  JumpBack/                 Staged HWND / process+title resolution
  Engage/                   Focus capture + restore
  Toast/                    Panel-watched predicate (fail-open)
AgentGrid.Shell/            UI composition (menu handlers, tray binding)
  Menu/                     Quit vs Stop-daemon structural split
  TrayIcon/                 Daemon health → tray appearance
  Panel/                    (WinUI XAML — Windows-only; not built on Linux CI)
```

## Build / test

Requires .NET 8 SDK. Pure `Core` + `Platform` (fakeable interop) run on Linux/WSL
and on Windows. The WinUI host and real `Win32InteropService` / named-pipe client
are Windows-dev concerns.

### Unit (always-on)

```sh
# WSL → Windows robocopy tree (recommended on this machine)
powershell.exe -NoProfile -ExecutionPolicy Bypass \
  -File clients/windows-shell/scripts/win-test.ps1
```

E2E facts are **skipped** unless `AG_E2E_RUN_DEV=1`.

### e2e (T3, opt-in — `make run-dev` fixture)

Canonical doc: **[docs/e2e.md](docs/e2e.md)**.

```sh
# One-shot: start run-dev, run unit+e2e, tear down
make test-windows-shell-e2e
# or:
./clients/windows-shell/scripts/e2e.sh --start-run-dev

# Two-terminal:
make run-dev                              # terminal A
./clients/windows-shell/scripts/e2e.sh    # terminal B
```

Client UI smoke against the same stack: `scripts/dev-up.ps1` (generates and
passes an isolated `--config-dir`).

### From WSL (unit details)

Windows `dotnet` cannot reliably build over `\\wsl.localhost\...` UNC paths
(ref assemblies break). Use the helper which robocopies to `%LOCALAPPDATA%\Temp`
and runs tests via PowerShell:

```sh
# WSL
powershell.exe -NoProfile -ExecutionPolicy Bypass \
  -File clients/windows-shell/scripts/win-test.ps1
```

Or pure Core tests inside WSL with a Linux SDK (no Win32 process spawn):

```sh
export DOTNET_ROOT=/tmp/dotnet DOTNET_CLI_HOME=/tmp/dotnet-home
export NUGET_PACKAGES=/tmp/nuget-packages
cd clients/windows-shell && dotnet test
```

### From Windows PowerShell / cmd

```bat
cd /d C:\path\to\agent-grid\clients\windows-shell
dotnet test
```

## Contracts (selected)

| Contract | Owner |
|---|---|
| `contract-b3-daemon-supervisor-state-machine` | `DaemonSupervisorMachine` |
| `contract-b2-token-acquisition` | `FileTokenSource` / `ShellGatewayClient` |
| `contract-approve-submission-rollback` | `SupervisionReducer` |
| `contract-resolved-by-other-display` | `SupervisionReducer` |
| `contract-deep-link-question-jump-kind-gap` | `DeepLinkRouter` + `AliasTable` |
| `contract-b1-jsonlines-envelope-shape` | `ControlEnvelope` |
| `contract-quit-vs-daemon-stop` | `ShellMenuHandlers` |
| `contract-jump-back-staged-resolution` | `JumpBackService` |
| `contract-engage-focus-return-mechanism` | `EngageFocusService` |
| `contract-toast-panel-watched-detection` | `PanelWatchedPredicate` |
| `contract-cross-flow-focus-invariant` | `FocusTransferGuard` |

See `docs/changes/change-20260723-windows-shell-phase2/` and the 14 ADRs under
`docs/adr/adr-20260724-*.md`.

## Headless host + WinUI + scripts (Windows via PowerShell)

| Script | Purpose |
|---|---|
| `scripts/win-test.ps1` | robocopy + `dotnet test` on local disk |
| `scripts/win-build-winui.ps1` | self-contained WinUI build + layout assert + launch smoke |
| `scripts/assert-winui-layout.ps1` | Bootstrap / ui.xaml が EXE 横にあること |
| `scripts/launch-smoke.ps1` | 起動スモーク；失敗時は structured startup log をダンプ |
| `scripts/wsl-detach-spike.ps1` | T3 detach survival (calls in-distro `.sh`) |
| `scripts/register-deep-link.ps1` | HKCU `agent-grid://` → host exe |
| `scripts/install-local.ps1` | publish Host to `%LOCALAPPDATA%\agent-grid` |

```sh
# Unit tests
powershell.exe -NoProfile -ExecutionPolicy Bypass -File \
  "$(wslpath -w /workspace/agent-grid)/clients/windows-shell/scripts/win-test.ps1"

# WinUI (self-contained; launch the win-x64 folder)
powershell.exe -NoProfile -ExecutionPolicy Bypass -File \
  "$(wslpath -w /workspace/agent-grid)/clients/windows-shell/scripts/win-build-winui.ps1"
# EXE:
# %LOCALAPPDATA%\Temp\ag-shell-src\AgentGrid.Shell.WinUI\bin\x64\Debug\net8.0-windows10.0.19041.0\win-x64\AgentGrid.Shell.WinUI.exe
# Startup errors (not MessageBox OCR): %LOCALAPPDATA%\agent-grid\logs\winui-startup-error.txt
```

| Project | Role |
|---|---|
| `AgentGrid.Shell.Core` | Pure machines / reducers / pipe client |
| `AgentGrid.Shell.Platform` | Win32 + toast decision + engage AttachThreadInput |
| `AgentGrid.Shell` | Composition root + panel presenter |
| `AgentGrid.Shell.Host` | Headless console host |
| `AgentGrid.Shell.WinUI` | Tray + panel (WS_EX_NOACTIVATE) + AppNotification |

Spike result: `docs/wsl-detach-spike-result.md` (PASS).  
S3 manual gate: `docs/s3-prototypes-checklist.md`.  
Run log: `docs/changes/change-20260723-windows-shell-phase2/s3-prototypes-run-log.md`.

### e2e: `make run-dev` (WSL) + client launch (Windows)

The Windows Shell is a **client**. For local e2e use the existing repo stack
in WSL — do not invent a Windows-side server launcher.

| Side | Command | Role |
|---|---|---|
| WSL | `make run-dev` → `scripts/run-dev.sh` | gateway + web on loopback, **`-no-auth`** |
| Windows | `scripts/dev-up.ps1` | build/register/launch WinUI with an isolated test config |

```sh
# Terminal A — WSL
make run-dev
# backend http://127.0.0.1:8443  web http://127.0.0.1:8080

# Terminal B — Windows client (from WSL host shell is fine)
powershell.exe -NoProfile -ExecutionPolicy Bypass -File \
  "$(wslpath -w /workspace/agent-grid)/clients/windows-shell/scripts/dev-up.ps1"
```

Auth-enabled gateway (non-e2e):  
`dev-up.ps1 -NoAuth:$false -TokenPath 'C:\path\to\gateway-token'`.

Product-time adopt/spawn of the daemon is **`DaemonSupervisor`**, not these scripts.  
Detach spike remains `docs/wsl-detach-spike-*.md` (server binary fidelity), separate from e2e.

## Deep-link alias (Track A)

`protocol/deep-links.schema.json` currently accepts only `session` and `approval`.
Phase 2 routes `agent-grid://server/<serverId>/question/<id>` and
`agent-grid://server/<serverId>/session/<id>/jump`
via a documented client-side alias table
(`adr-20260724-deep-link-schema-additive-extension`). The alias is removed once the
upstream additive schema PR lands.
