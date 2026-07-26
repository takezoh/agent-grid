---
change: change-20260726-uihost-server-status-e2e
role: verification
---

<!-- lifecycle is owned by change.md -->

# Verification

## Required gates

```sh
cd src && GOCACHE=/tmp/gocache-agent-grid go test ./host/runtime/subsystem/stream/fake ./server/api/...
cd clients/ui && npm run lint
cd clients/ui && npm run test:e2e:real
python3 <dev-docs>/scripts/docs_cli.py lint
```

## Result

- PASS: `go test ./host/runtime/subsystem/stream/fake ./host/runtime/subsystem/stream ./server/api/...`
- PASS: `npm run test:unit` (118 files, 1765 tests).
- PASS: `npm run lint` and `npm run typecheck`.
- PASS: `npm run test:e2e:real`; Playwright built and launched the same-revision
  real `server` and real `uihost`, injected the test-support fake Codex through
  `PATH`, and passed the composite status DOM scenario.
- PASS: the same real-stack scenario with overridden ports
  (`AG_UI_E2E_WEB_ADDR=127.0.0.1:28081`,
  `AG_UI_E2E_BACKEND_ADDR=127.0.0.1:28444`).
- PASS: the scenario observed bell-only, spinner-only, bell-plus-attention,
  attention-only after reconnect replay suppression, and explicitly rejected
  spinner-plus-attention.
- PASS: fake observer server-request delivery contract and the opt-in
  `TestStreamServerRequestE2EFidelity` compile/skip gate.
- ENVIRONMENT BLOCKED: both the pre-existing real-Codex
  `TestStreamRoutingE2EIsolation` and the new server-request fidelity test stop
  before protocol execution because the installed Codex 0.145.0 cannot start
  `app-server` (`Read-only file system`). Direct `codex app-server --listen`
  reproduces the same installation failure outside Go.
- PASS: dev-docs lint and conformance.
