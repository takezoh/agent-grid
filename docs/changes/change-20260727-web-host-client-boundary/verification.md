---
change: change-20260727-web-host-client-boundary
role: verification
---

# Verification

## T0 — Pure contracts

- Shared `servers.json` v2 fixtures: canonical URL/serverId, duplicate/invalid/auth/version matrix, credential-free projection parity.
- Startup outcome partition: valid, missing/unreadable, malformed/unsupported; exactly one stable code and no implicit fallback.
- Static audit rejects host-kind/cardinality branches, token-bearing projection fields, services-module edges from clients/web-host, root Go packages, active `src/`, and active `clients/desktop-config` references.

## T1 — Wired modules and browser transport

- Services module: `cd services && GOWORK=off go test ./...`, `go vet ./...`, lint with `services/.golangci.yml`, and exact module path audit.
- Nested Web host: `cd clients/web-host && GOWORK=off go test ./...`; inspect `go list -m -json all` and imports for zero services-module edge.
- Web httptest: embed/static/CSP/security headers, qualified REST/WS routing, same-origin rejection, unknown server, redirect rejection, credential injection only upstream, retryable 503 + `Retry-After: 1`.
- Web Playwright smoke: shared UI consumes `/host/config` and qualified routes without an implicit default server.

## T2 — Cross-host contracts

- Run the same FleetProvider conformance suite against Web HTTP and Workspace preload/IPC adapters.
- With two servers, create/list/retry targets the explicit second `serverId`; one failed server does not erase healthy state.
- Capture every UI/HTTP projection and log: no bearer bytes. Capture upstream requests: Authorization appears only on the configured canonical target.
- Verify accepted compound identity, generation fencing, redirect rejection, cancellation/cleanup and typed retry remain intact for both hosts.

## T3 — Fidelity backstops

- `clients/ui` real-stack E2E with real Web host plus deterministic fake multi-server gateways.
- Workspace Electron real runtime with fake-vs-real fleet gateway, using the same parity fixture.
- Existing Web behavior tests move with the module; they are not weakened or replaced by path-only assertions.

## Migration and rollback gates

- Before cutover, record the tracked move manifest, active-reference inventory, and content hashes for done/abandoned change packages.
- After cutover, assert `src/` and `clients/desktop-config` absent; root `.claude` untouched; services and Web-host gates pass independently; allowed top-level ownership holds; active consumers contain no stale paths.
- Completed/abandoned change hashes must match baseline. Historical references are not failures.
- Run docs lint and conformance after reconciling `change-20260727-multi-server-session-palette`, `design-clients-ui`, and `design-workspace-desktop`.
- A failed gate rolls back the whole flag-day commit. No compatibility path may be retained as recovery.

## Evidence — 2026-07-27

- PASS: `make build-all`; outputs are under `dist/bin`.
- PASS: `make test`; both `services` and the nested `clients/web-host` module pass independently with `GOWORK=off`.
- PASS: `make vet`.
- PASS: `clients/ui` typecheck/build, Biome lint, coverage thresholds, and 120 unit suites / 1759 tests.
- PASS: `clients/workspace` typecheck and 12 suites / 80 tests.
- PASS: harness dependency admission, skip inventory, `git diff --check`, active stale-path audit, and absence of `src/` / `clients/desktop-config`.
- PASS: Web-host module graph and source audit contain no services-module dependency or replace directive.
- OPEN (pre-existing baseline debt): `make lint` reports 15 findings in unchanged service bodies (existing exhaustive/funlen/file-length/gofmt/staticcheck/unparam findings). The module-path-only import edits do not cause those findings; this change does not weaken their rules.
- PASS: Web Playwright smoke uses an explicit preview lifecycle and completed with 6 passed / 5 opt-in skipped. Its default config excludes the separately owned real-stack tree.
- PASS: Web UI/UX fidelity E2E completed with 1 passed against a real `server`, real independently built `web-host`, generated client `servers.json`, and deterministic fake Codex. All REST and WS traffic used server-qualified `/host/servers/{serverId}/...` routes.
- NOT RUN (outside this Web-host completion): Windows Shell fidelity E2E.
- TOOLING BLOCKED: post-implementation docs CLI rerun could not bootstrap because the dev-skills virtualenv lock is outside the writable workspace. Pre-implementation acceptance/conformance passed at revision `rev-20260727-03`.
