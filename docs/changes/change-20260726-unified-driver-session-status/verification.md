---
change: change-20260726-unified-driver-session-status
role: verification
---

<!-- lifecycle is owned by change.md -->

# Verification

## Success conditions

- T0 reducer/property tests cover valid combinations, zero/active+failure rejection, multiple pending IDs, spinner/! exclusion, bell+!, epoch changes, reconnects, and race schedules.
- T0/T1 Driver tables cover Codex, Claude, Gemini, Grok, and Shell; Codex OSC has no state effect while Claude/Gemini OSC title handling and Shell OSC 133 remain.
- T2 Codex contract tests ship fake + invariant-naming inventory test; T3 `FakeVsReal*` detects protocol drift without weakening assertions.
- T1 gateway/schema tests accept only the composite shape and reject the removed scalar field and legacy event fallback.
- SDK regeneration cleanliness, repository-wide zero-legacy searches, and builds/tests for Web, Workspace, and Windows Shell run against the same revision; any stale SDK or partial client migration fails the cutover.
- Driver/runtime tests prove Codex ignores OSC status input while Claude/Gemini title transitions and Shell OSC 133 prompt semantics still work.
- Web DOM, Workspace accessibility, and Windows UIA tests consume the same display fixture and assert semantic key, animation role, and accessible label parity.

## Commands

```sh
cd src && GOCACHE=/tmp/gocache-agent-grid go test ./host/... ./server/api/...
cd clients/ui && npm run test:unit
GOCACHE=/tmp/gocache-agent-grid make test-e2e
cd clients/ui && PLAYWRIGHT_BROWSERS_PATH=/tmp/ms-playwright npm run test:e2e
make test-windows-shell-e2e
```

The opt-in real Codex backstop may skip when its binary/version inputs are unavailable; fake and invariant contract tests remain mandatory.

## 2026-07-26 results

- PASS: Codex 0.145.0 schema drift check (`make codex-schema-check`).
- PASS: generated TypeScript/C#/Kotlin/Swift SDK compatibility (`scripts/check-protocol-compatibility.sh`).
- PASS: Go tests for `host`, `server/api`, `gorules`, and Codex schema/router packages.
- PASS: Web unit suite (1764 tests), lint, and TypeScript typecheck.
- PASS: dev-docs lint and change conformance.
- PASS: repository search finds no Codex `DEvFrameOsc`, `handleWindowTitle`, or `LastWindowTitle` consumer; Claude/Gemini OSC tests remain.
- NOT RUN: Windows Shell .NET tests because neither WSL nor Windows has a .NET SDK installed in this environment.
- OUTSIDE CHANGE: repository-wide `go test ./...` is blocked only by existing `uihost.TestProxyDialRefusedMaps503WithRetryAfter` returning 502 after a 30-second loopback timeout; the changed Go package set passes.
- OUTSIDE CHANGE: repository-wide lint still reports pre-existing `platform/wslhost` and WSL-supervisor findings; lint over all changed Go package trees reports zero issues.
