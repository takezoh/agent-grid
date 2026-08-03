---
change: change-20260803-workspace-display-selection
role: verification
---

<!-- lifecycle is owned by change.md -->

# Verification

## Evidence

- Workspace unit tests: schema v2→v3 state upgrade, last-display matching/fallback,
  centered/clamped initial bounds, display list bridge, Settings save payload, and config
  round trip.
- Shell unit tests: optional anchor config load and invalid anchor rejection.
- Windows UI code contract: initial panel work area resolves from configured anchor with
  primary fallback.
- `cd clients/workspace && npm run typecheck` — PASS.
- `cd clients/workspace && npm test` — PASS, 147 tests; the Windows-only
  `FakeVsReal workspace display projection` is correctly skipped without
  `AG_E2E_WORKSPACE_DISPLAY=1`.
- `cd clients/workspace && npm run lint:browserwindow` — PASS.
- `cd clients/workspace && npm run build` — PASS, including shared UI build and packaged
  Workspace TypeScript output.
- `powershell.exe ... clients/windows-shell/scripts/win-test.ps1` — PASS: Shell Core 164,
  Platform 27; opt-in E2E skipped.
- `powershell.exe ... clients/windows-shell/scripts/win-build-winui.ps1` — WinUI x64
  self-contained build and layout check PASS with zero warnings/errors. The script's later
  aggregate test step exits nonzero because four pre-existing shared fixture files are not
  copied into that script's temporary test output; the canonical `win-test.ps1` run above
  includes those fixtures and passes.
- `git diff --check` — PASS.

## Remaining fidelity gate

- Run `AG_E2E_WORKSPACE_DISPLAY=1 npm test -- display-placement-real` from a Windows
  Workspace checkout with Electron available to compare the fakeable projection against
  the machine's real multi-display topology.
