# Agent Grid — Workspace (Electron)

On-demand session windows for the Windows desktop slice (Phase 2).
Each session maps to exactly one `BrowserWindow`; Shell drives open/focus via a
JSON Lines control channel (named pipe on Windows).

## Layout

```text
src/main/
  window-registry.ts    sole BrowserWindow creation point
  control-endpoint.ts   named-pipe / Unix-socket JSON Lines server
  desktop-config.ts     shared config load/create + validation
  daemon-config.ts      per-server fresh token resolve + hosted URL
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
| `contract-b2-token-acquisition` | `DaemonConfigResolver.resolve` |

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
Playwright-for-Electron e2e is the T1/T3 fidelity path on Windows CI.

On WSL, Windows-side Electron checks can use:

```sh
powershell.exe -NoProfile -Command "cd (wsl wslpath -w /workspace/agent-grid/clients/workspace); npm test"
```

## Lint invariant

`new BrowserWindow` must only appear in `electron-window-factory.ts`
(the sole production creation site owned by `window-registry`). Enforced by
`npm run lint:browserwindow` (`scripts/check-browserwindow.mjs`).
