---
change: change-20260726-deterministic-codex-status-fidelity
role: verification
---

<!-- lifecycle is owned by change.md -->

# Verification

## Required gates

```sh
cd src
GOCACHE=/tmp/gocache-agent-grid go test -run TestStreamServerRequestWiredDelivery ./host/runtime/subsystem/stream
GOCACHE=/tmp/gocache-agent-grid go test ./host/runtime/subsystem/stream/... ./platform/agent/fakecodex/...
GOCACHE=/tmp/gocache-agent-grid go test -tags e2e -run 'TestE2E_(AppServerInit|ThreadTurnLifecycle|FakeVsRealMethods)' ./platform/agent/fakecodex
python3 <dev-docs>/scripts/docs_cli.py lint --conformance
```

The real Codex commands remain opt-in through `AG_E2E_CODEX_BIN`; an unset
binary skips rather than fabricating fidelity.

## Result

- PASS: focused wired approval delivery, routing isolation, and status reducer
  tests.
- PASS: complete stream fake, stream backend, fakecodex, and server API test
  packages.
- PASS: e2e-tag compile/skip gate; the removed live-model approval test is no
  longer selected.
- PASS: `PW_USE_SYSTEM_CHROME=1 npm run test:e2e:real`; the fake app-server
  drove the real server, real uihost, browser, and production status DOM.
- EXPECTED ENVIRONMENT SKIP: real Codex lifecycle T3 requires
  `AG_E2E_CODEX_BIN` plus a readable authenticated developer home. This change
  does not replace that deterministic fidelity with a model-behavior test.
