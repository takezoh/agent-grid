---
change: change-20260726-windows-shell-deterministic-e2e
role: verification
---

<!-- lifecycle is owned by change.md -->

# Verification

```sh
dotnet test clients/windows-shell/AgentGrid.Shell.sln
./clients/windows-shell/scripts/e2e.sh --start-run-dev
python3 <dev-docs>/scripts/docs_cli.py lint --conformance
```

## Result

- PASS: Windows SDK compile and always-on tests — Core 153 passed with two
  opt-in real-gateway facts skipped; Platform 27 passed; UIA assembly compiled.
- PASS: full out-of-sandbox Windows E2E — Core 155, Platform 27, WinUI
  self-contained layout, five-second launch smoke, and all five FlaUI/UIA
  assertions.
- PASS: real gateway fidelity is limited to sessions probe and
  ticket-to-WebSocket upgrade.
- PASS: fixture-owned fake gateway drives two session statuses, a bell,
  approval, and question through the production Shell client into the native
  UIA tree; assertions are unconditional.
