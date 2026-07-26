---
change: change-20260727-agy-passive-status-observation
role: verification
---

<!-- lifecycle is owned by change.md -->

# Verification

## Mechanical gates

```sh
cd src
go test ./host/driver ./host/lib/agenthook ./host/event ./cmd/server ./cmd/bridge
go test ./host/... ./cmd/server/... ./cmd/bridge/...
AG_E2E_AGY_BIN="$(command -v agy)" \
AG_E2E_AGY_HOME="$HOME" \
go test -tags=e2e ./host/driver -run TestFakeVsRealAgyPassiveLifecycle

cd ..
python3 -m py_compile scripts/research/*.py
python3 scripts/research/analyze-terminal-signals.py /tmp/agy-esc-conformant.raw
python3 skills/dev-docs/scripts/docs_cli.py lint
```

Set `AG_E2E_AGY_BIN` and `AG_E2E_AGY_HOME` to opt into the real fidelity run.
The test copies only the OAuth token and applicable settings into its
`t.TempDir`; it neither writes the source home nor logs credential contents.
Without either variable the T3 test skips; fake and invariant contract tests
remain always-on.

## Required assertions

- agy is registered as `AgyDriver`;
- PreInvocation → Running;
- fully-idle non-stale Stop → Waiting;
- non-idle and stale Stop do not clear Running;
- 24 hours of ticks do not synthesize Waiting;
- managed plugin contains passive events and excludes PreToolUse;
- unmanaged plugin directories are preserved and rejected;
- host and devcontainer registration both install the managed plugin;
- the hook helper emits agy-conformant stdout;
- the real opt-in scenario observes `PreInvocation … Stop{fullyIdle:true}`.

## 2026-07-27 results

- PASS: focused host/driver, installer, event, server, and bridge tests.
- PASS: `go test ./host/... ./cmd/server/... ./cmd/bridge/...`.
- PASS: authenticated real agy 1.1.7 FakeVsReal lifecycle test using an
  isolated temporary HOME.
- PASS: test-harness dependency triple and skip inventory.
- PASS: Python research scripts compile; agy Esc capture contains no OSC/BEL.
- PASS: Web unit suite (118 files, 1766 tests) and Web lint.
- OUTSIDE CHANGE: repository-wide Go test is blocked by the existing
  `uihost.TestProxyDialRefusedMaps503WithRetryAfter` 502/503 mismatch and the
  concurrency-sensitive
  `host/runtime.TestTerminalRelay_SeversSlowSubscriberWithoutBlockingOthers`;
  every changed package and the broader `host/...` run pass independently.
- OUTSIDE CHANGE: repository-wide lint reports existing WSL supervisor and
  `cmd/server/supervisor_control_fidelity_test.go` findings. Lint over
  `host/...` and `cmd/bridge/...` reports zero issues; changed server files are
  gofmt-clean and their package tests pass.
