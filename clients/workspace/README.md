# Agent Grid — Workspace (Electron)

On-demand session windows for the Windows desktop slice (Phase 2).
Each session maps to exactly one `BrowserWindow`; Shell drives open/focus via a
JSON Lines control channel (named pipe on Windows).

## Layout

```text
src/main/
  window-registry.ts    window-per-session dedup over the sole creation point
  electron-window-factory.ts  sole window/view creation point (frameless BaseWindow
                              + title-bar / SPA WebContentsViews)
  window-chrome.ts      VSCode-style title bar: colors, layout, menu-bar contract,
                        title-bar document (pure, no electron import)
  control-endpoint.ts   named-pipe / Unix-socket JSON Lines server
  desktop-config.ts     shared config load/create + validation
  daemon-config.ts      per-server persisted auth resolve + hosted URL
  index.ts              bootstrap wiring
src/preload/
  index.ts              contextBridge surface + token-not-in-URL guard
src/shared/
  control-envelope.ts   closed {op,id} schema (mirrors Shell.Core)
```

## Contracts

| Contract | Module |
|---|---|
| `contract-b1-jsonlines-envelope-shape` | `control-envelope.ts` |
| `contract-b1-window-registry-dedup` | `window-registry.ts` |
| `contract-migration-window-per-session-invariant` | `window-registry.ts` |
| `contract-window-close-not-session-stop` | `window-registry.ts` `closeSessionView` |
| `contract-workspace-state-schema-evolution` | `loadWorkspaceState` |
| `contract-b2-hosted-mode-token-injection` | `daemon-config.ts` + preload |
| persisted desktop auth | `DaemonConfigResolver.resolve` |

## Configuration

Shell and Workspace share the files documented in
[`../desktop-config/README.md`](../desktop-config/README.md). Production uses
`%APPDATA%\agent-grid\config`; pass `--config-dir <path>` to replace the whole
set. Shell forwards the argument when it launches Workspace.

Workspace window identity and persisted bounds use `{serverId, sessionId}`.
After routing to a configured server connection, hosted SPA/server calls still
receive only `sessionId`.

## Test

```sh
cd clients/workspace
npm ci
npm test
npm run lint:browserwindow
```

Electron binary is optional for unit tests (vitest + memory factory).

`npm run build` builds the shared UI and copies it into `dist/ui`. Workspace
loads `dist/ui/index.html` locally; it never obtains UI content from a Server.

## Windows build and local install

Run either the PowerShell scripts directly or their BAT wrappers:

```bat
clients\workspace\scripts\build-windows.bat
clients\workspace\scripts\install-local.bat
```

The build wrapper produces an unpacked Electron runtime under
`%LOCALAPPDATA%\Temp`. The installer publishes it to:

```text
%LOCALAPPDATA%\agent-grid\workspace\agent-grid-workspace.exe
```

The Shell's default `workspace_executable` name resolves to this install path.
Playwright-for-Electron e2e is the T1/T3 fidelity path on Windows CI.

On WSL, Windows-side Electron checks can use:

```sh
powershell.exe -NoProfile -Command "cd (wsl wslpath -w /workspace/agent-grid/clients/workspace); npm test"
```

## Lint invariant

`new BrowserWindow` / `new BaseWindow` / `new WebContentsView` must only
appear in `electron-window-factory.ts` (the sole production creation site
owned by `window-registry`). Enforced by `npm run lint:browserwindow`
(`scripts/check-browserwindow.mjs`).

## Window chrome

Windows are frameless (`titleBarStyle: "hidden"` + Window Controls Overlay —
native caption buttons with Win11 Snap Layouts). A Workspace-owned title-bar
`WebContentsView` (38px, drag region + File/Edit/View/Window menu bar) sits
above the unmodified shared SPA view; native popup menus are served by
`electron-entry.ts` over `agent-grid:titlebar-menu`. The SPA reports theme
switches (resolved value + chosen mode) via the preload `setTitleBarTheme`
bridge and the entry repaints the overlay + title-bar palette and mirrors
`nativeTheme.themeSource` so native popup menus render in the app palette
(`agent-grid:titlebar-theme`; mode "system" stays on the OS palette —
pinning it would freeze `prefers-color-scheme` for the renderers).
