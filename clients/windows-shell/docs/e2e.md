# Windows Shell e2e (Phase 2)

## Principle

| Layer | Owns | Does not own |
|---|---|---|
| **WSL `make run-dev`** | Full stack fixture (`scripts/run-dev.sh`: server+web, `-no-auth` loopback) | Shell UI |
| **Windows Shell client** | Connect (REST/WS), panel, toast, tray | Starting the gateway |
| **e2e harness** | Opt-in T3 tests; optional **backend** fixture with the same flags as run-dev | Product client code paths |

The Shell **client** never starts or stops `server`; product-time server lifecycle is external.

For **tests**, prefer a live `make run-dev`. The harness `--start-run-dev` starts only the **gateway binary** with the same `-insecure -no-auth -addr 127.0.0.1:8443 -data-dir .run-dev/server` posture as `scripts/run-dev.sh` (web/`npm` not required for Shell Core e2e).
## Stack

```text
WSL:   make run-dev
         → ./server -insecure -no-auth -addr 127.0.0.1:8443 -data-dir $ROOT/.run-dev/server
         → ./web    -insecure -addr 127.0.0.1:8080 -server http://127.0.0.1:8443

Win:   AG_E2E_RUN_DEV=1  → xUnit T3 facts (probe / ticket+WS / composition)
       + WinUI self-contained layout assert + launch smoke (default)
       + AG_E2E_WINUI_UI=1 → FlaUI/UIA panel automation (AutomationId contract)
       scripts/dev-up.ps1 → optional longer manual UI against same gateway
```

## Commands

### A. Two-terminal (recommended)

```sh
# Terminal A — WSL
make run-dev

# Terminal B — full e2e (Core/Platform + RunDev facts + WinUI layout/smoke/UI)
./clients/windows-shell/scripts/e2e.sh
# gateway facts only (no WinUI rebuild):
./clients/windows-shell/scripts/e2e.sh --skip-unit --skip-winui
# layout + smoke but no UIA-driven panel tests:
./clients/windows-shell/scripts/e2e.sh --skip-ui
```

### B. One-shot harness

```sh
make test-windows-shell-e2e
# = ./clients/windows-shell/scripts/e2e.sh --start-run-dev
# stages: backend fixture → xUnit → WinUI layout + launch smoke → FlaUI UI automation
```

When a full `make run-dev` is already on `:8443`, omit `--start-run-dev` and the harness attaches to it.

### Windows .NET SDK prerequisite and diagnosis

When invoked from WSL on a Windows host, the harness runs the tests with
Windows `dotnet.exe` and copies the Shell source to a local Windows directory
before building. It resolves `dotnet.exe` in this order:

1. `%LOCALAPPDATA%\Microsoft\dotnet\dotnet.exe`
2. `C:\Program Files\dotnet\dotnet.exe`

Do not diagnose the environment from `C:\Program Files\dotnet` alone. That
installation may contain only the runtime while the SDK used by the harness is
installed under `%LOCALAPPDATA%`. Conversely, the existence of `dotnet.exe`
does not prove that an SDK is installed. Check the exact executable selected by
the harness:

```powershell
$dotnet = Join-Path $env:LOCALAPPDATA 'Microsoft\dotnet\dotnet.exe'
if (-not (Test-Path $dotnet)) {
  $dotnet = 'C:\Program Files\dotnet\dotnet.exe'
}
Write-Host "Windows Shell E2E dotnet: $dotnet"
& $dotnet --list-sdks
& $dotnet --info
```

At least one .NET 8 SDK must appear under `.NET SDKs installed`. A host/runtime
entry under `.NET runtimes installed` is insufficient for `dotnet test` and
`dotnet build`. This prerequisite is independent of Codex filesystem
sandboxing: run the command above on Windows before attributing a failure to
the sandbox.

### C. From Windows PowerShell

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File `
  ((wsl wslpath -w /workspace/agent-grid) + '\clients\windows-shell\scripts\e2e.ps1') -StartRunDev

# Skip UI binary stage:
...\e2e.ps1 -StartRunDev -SkipWinUi
```

### D. Manual UI (beyond 5s smoke)

```sh
make run-dev
powershell.exe -NoProfile -ExecutionPolicy Bypass -File \
  "$(wslpath -w /workspace/agent-grid)/clients/windows-shell/scripts/dev-up.ps1"
```

## Test inventory (T3, opt-in)

| Stage | Asserts |
|---|---|
| `Probe_sessions_against_run_dev_no_auth` | `GET /api/sessions` via `ShellGatewayClient` + `NoAuthTokenSource` |
| `Mint_ticket_and_open_websocket` | `POST /api/ws-ticket` + `ClientWebSocket` open |
| `assert-winui-layout.ps1` | Bootstrap + `Microsoft.ui.xaml.dll` next to EXE (prevents MSIX Runtime 1.6 dialog) |
| `launch-smoke.ps1` | process stays up; on fail dumps `winui-startup-error.txt` (structured SoT) |
| `WasdkBootstrapErrorsTests` | bootstrap failure report text is stable / includes Version 1.6 + MSIX hint |
| `Panel_exposes_expanded_chrome_via_automation_ids` | UIA tree has the unconditional expanded-chrome `AutomationId` contract |
| `Fake_gateway_snapshot_renders_status_and_healthy_state_is_silent` | deterministic WS snapshot replaces initial status; healthy state has no offline badge |
| `Fake_gateway_requests_render_pending_and_engage_sections` | scripted approval/question frames render `PendingList`, actions, and `EngageBox` |
| `Session_rows_fill_the_shell_content_width_when_present` | two scripted sessions render full-width rows with Working and Action-required status icons |
| `Island_collapses_to_compact_bar_and_expands_back` | notch morph swaps compact/expanded UIA trees and back |

Enable gateway facts: `AG_E2E_RUN_DEV=1`. Skip otherwise (always-on `dotnet test` stays green without fixture).

Filter:

```sh
dotnet test --filter "FullyQualifiedName~RunDevGatewayE2E"
```

## UI automation (FlaUI/UIA) — Playwright equivalent for native

Authoring rules for agents changing shell UI (AutomationId contract, retry /
side-effect discipline):
[`docs/note/note-20260724-windows-shell-ui-e2e-authoring.md`](../../../docs/note/note-20260724-windows-shell-ui-e2e-authoring.md).

`AgentGrid.Shell.WinUI.UiTests` drives the **built exe black-box** over
Windows UI Automation via [FlaUI](https://github.com/FlaUI/FlaUI) (UIA3).
No project reference to the app — tests select elements by
`AutomationProperties.AutomationId` (the native `data-testid`; also the
Narrator/S3 accessibility contract, so keep ids stable and add one to every
new interactive element in `PanelWindow.xaml`).

The UIA process uses an in-process fake gateway owned by the test fixture. It
serves the Shell REST ticket/probe endpoints and emits deterministic
`h`/`ar`/`qr` WebSocket frames. This keeps native layout, accessibility, and
process wiring real while moving server state setup out of T3. The separate
Core T3 facts retain the real server fidelity checks for `/api/sessions` and
ticket-to-WebSocket upgrade.

Library choice: FlaUI over WinAppDriver (Appium server, unmaintained) and raw
`Windows.UIAutomation` COM (FlaUI is exactly that wrapper, maintained, MIT).

| Variable | Meaning |
|---|---|
| `AG_E2E_WINUI_UI=1` | enables the suite (skips otherwise; skips off-Windows) |
| `AG_E2E_WINUI_EXE` | exe under test (harness passes the smoke-stage build) |
| `AG_E2E_GATEWAY_URL` | real gateway used by Core T3 and launch smoke; UIA uses its fixture-owned fake |

Constraints: needs an interactive desktop session (fine from WSL via
`powershell.exe`; a CI runner must not be session-0). The suite kills running
`AgentGrid.Shell.WinUI` instances first (single-instance redirect would kill
the launched child). WinUI 3 populates the UIA tree asynchronously — always
poll (`Retry.WhileNull/WhileFalse`), never assert immediately after launch.
Failures drop screenshots to `%LOCALAPPDATA%\agent-grid\logs\ui-e2e-*.png`.

Interactive inspection (Playwright-inspector equivalent): **Accessibility
Insights for Windows** or `inspect.exe` (Windows SDK) shows the live UIA tree
and AutomationIds of a running shell.

```sh
# Full stage via harness (build → smoke → UI):
./clients/windows-shell/scripts/e2e.sh --start-run-dev
# Re-run only the UI suite against an already-built exe (Windows PowerShell):
#   $env:AG_E2E_WINUI_UI='1'; $env:AG_E2E_WINUI_EXE='<path>\AgentGrid.Shell.WinUI.exe'
#   dotnet test AgentGrid.Shell.WinUI.UiTests --filter FullyQualifiedName~PanelWindowUiE2E
```

## Env

| Variable | Default | Meaning |
|---|---|---|
| `AG_E2E_RUN_DEV` | unset | `1` enables T3 facts |
| `AG_E2E_GATEWAY_URL` | `http://127.0.0.1:8443` | run-dev backend |
| `--config-dir` | Shell/Workspace | test-owned configuration; user config is never touched |
| `AG_E2E_WINUI_UI` | unset | `1` enables FlaUI UI automation suite |
| `AG_E2E_WINUI_EXE` | probe launch-smoke paths | WinUI exe under UIA test |

## Startup failure capture (WASDK bootstrap)

Native Bootstrap **MessageBox** text is **not** on stdout/stderr. Tests and
diagnostics use a structured report instead:

| Artifact | Path |
|---|---|
| Error log | `%LOCALAPPDATA%\agent-grid\logs\winui-startup-error.txt` |
| Beside EXE | `winui-startup-error.txt` next to `AgentGrid.Shell.WinUI.exe` |
| OK marker | `%LOCALAPPDATA%\agent-grid\logs\winui-startup-ok.txt` |

Production path: `Program.Main` → `WasdkBootstrapHost.TryStart` →
`Bootstrap.TryInitialize(0x00010006)` → on failure `WasdkBootstrapErrors.FormatReport`
(includes classification, HRESULT, “Runtime Version 1.6 / MSIX” hint).

| Test / script | What it asserts |
|---|---|
| `WasdkBootstrapErrorsTests` (T0) | report text for known HRESULTs is stable |
| `assert-winui-layout.ps1` | self-contained output ships Bootstrap + `Microsoft.ui.xaml.dll` |
| `launch-smoke.ps1` | process stays up, or non-zero exit dumps structured log |

```powershell
# After win-build-winui.ps1:
powershell.exe -NoProfile -ExecutionPolicy Bypass `
  -File clients/windows-shell/scripts/assert-winui-layout.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass `
  -File clients/windows-shell/scripts/launch-smoke.ps1
```

Launch the **`win-x64`** folder EXE (self-contained). Copying only the `.exe`
without natives reproduces the MSIX Runtime 1.6 dialog.

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
