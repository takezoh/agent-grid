# Agent Grid — Windows Shell (Phase 2)

Native supervision surface for Windows: always-on panel, tray, toast, deep links,
daemon supervision of the WSL-hosted `server` binary. Workspace windows are owned
by the sibling Electron app at `clients/workspace/`.

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

Requires .NET 8 SDK. On Linux CI only `Core` + `Platform` (fakeable interop) + their
xUnit projects are expected to pass; the WinUI host is a Windows-dev concern.

```sh
export DOTNET_ROOT=...   # if needed
cd clients/windows-shell
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

## Deep-link alias (Track A)

`protocol/deep-links.schema.json` currently accepts only `session` and `approval`.
Phase 2 routes `agent-grid://question/<id>` and `agent-grid://session/<id>/jump`
via a documented client-side alias table
(`adr-20260724-deep-link-schema-additive-extension`). The alias is removed once the
upstream additive schema PR lands.
