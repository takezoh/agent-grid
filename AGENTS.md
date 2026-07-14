## Build & Test

```sh
make build-server            # Build Go sources under src/ → ./server (+ bridge)
make build-orchestrator      # Build → ./orchestrator
make build-claude-app-server # Build → ./claude-app-server
make build-all               # Build all 3 main binaries
make vet                     # go vet ./...
make lint                    # golangci-lint (depguard, funlen, staticcheck, etc.)
cd src && go test ./...          # Run all tests
cd src && go test ./path/to/pkg  # Run tests for a specific package
cd src && go test -run TestName ./...  # Run a specific test
make test-race               # Race detector on concurrency-sensitive subtrees
```

### E2E (three kinds — do not conflate)

| Kind | Tier | When | Entry |
|------|------|------|-------|
| Go gateway scenario | T1 | Always-on (`go test ./...`) | `src/server/web` (`mux_*_test.go`, etc.) |
| Go opt-in fidelity | T3 | Manual / nightly; subtests skip without `AG_E2E_*` | `make test-e2e` |
| Web Playwright smoke | T1 | PR CI (`npm run test:web`) | `src/client/web` `npm run test:e2e` |

**Go gateway scenario** — no extra command; included in `go test ./...`. Exercises real `server` + fake agent (`session create → WS viewUpdate`, etc.).

**Go opt-in fidelity** — `//go:build e2e` fake-vs-real backstops. Skips when real binaries are unset (expected locally).

```sh
make test-e2e
# Package list: Makefile test-e2e target. Per-binary setup: docs/component/component-20260624-client-stream-backend-e2e.md
```

**Web Playwright smoke** — browser wiring that happy-dom cannot prove; uses deterministic fake backend (`e2e/support/fake-backend.ts`). Harness overview: `docs/component/component-20260705-client-web-browser-harness.md`.

```sh
cd src/client/web
npm ci
npx playwright install chromium   # first run or after @playwright/test bump; CI uses --with-deps
npm run test:e2e                # build (tsc + vite) then playwright test
npm run test:web                # lint + unit + e2e (CI web gate)
```

If home-directory caches are not writable (sandbox agents), redirect caches:

```sh
GOCACHE=/tmp/gocache-agent-grid GOLANGCI_LINT_CACHE=/tmp/golangci-lint-cache make lint
cd src && GOCACHE=/tmp/gocache-agent-grid go test ./...
cd src/client/web
NPM_CONFIG_CACHE=/tmp/npm-cache-agent-grid npm ci
PLAYWRIGHT_BROWSERS_PATH=/tmp/ms-playwright npx playwright install chromium
PLAYWRIGHT_BROWSERS_PATH=/tmp/ms-playwright npm run test:e2e
GOCACHE=/tmp/gocache-agent-grid make test-e2e
```

Playwright escape hatches: `PLAYWRIGHT_BROWSERS_PATH` when `~/.cache/ms-playwright` is unavailable; `PW_USE_SYSTEM_CHROME=1` to use installed Chrome instead of downloaded Chromium (`playwright.config.ts`).

### Full local verification (cross-cutting changes)

```sh
GOCACHE=/tmp/gocache-agent-grid GOLANGCI_LINT_CACHE=/tmp/golangci-lint-cache make lint
cd src/client/web && NPM_CONFIG_CACHE=/tmp/npm-cache-agent-grid npm run lint
cd src && GOCACHE=/tmp/gocache-agent-grid go test ./...
cd src/client/web && npm run test:unit
GOCACHE=/tmp/gocache-agent-grid make test-e2e
cd src/client/web && PLAYWRIGHT_BROWSERS_PATH=/tmp/ms-playwright npm run test:e2e
```

Cache env vars are optional on a normal dev machine.

## Three-layer architecture

One Go module, three top-level trees under `src/` and three binaries:

| Binary | Source | Layer | Role |
|---|---|---|---|
| `server` | `src/cmd/server/` | `client/` | Single-process backend — pty session daemon + HTTP/WS gateway in one binary (Unix-socket IPC plus browser-facing REST/WS) |
| `orchestrator` | `src/cmd/orchestrator/` | `orchestrator/` | Symphony SPEC implementation — autonomous poll/dispatch/reconcile + observability HTTP |
| `claude-app-server` | `src/cmd/claude-app-server/` | `platform/` + `orchestrator/` | Codex app-server stdio shim for Claude; enables agent-switch via `codex.command` in WORKFLOW.md |

`platform/` is shared infrastructure; `client/` is agent-grid's session daemon and the embedded HTTP/WS gateway under `client/web/`, both shipped as the single `server` binary; `orchestrator/` is the Symphony pipeline. Import direction (enforced by `depguard`, `src/.golangci.yml`): `platform/*` imports neither `client/*` nor `orchestrator/*`; `client/*` does not import `orchestrator/*`; `orchestrator/*` does not import `client/*`. Full layer definition: [ARCHITECTURE.md](ARCHITECTURE.md).

Orchestrator-scoped test run: `cd src && go test ./orchestrator/... ./platform/tracker/... ./cmd/orchestrator/... ./cmd/claude-app-server/...`
Conformance: `docs/component/component-20260624-orchestrator-symphony-conformance.md` — SPEC §17 ↔ test 対応表と逸脱 posture の正本。

## Rules

- Follow the design principles in [ARCHITECTURE.md](ARCHITECTURE.md)
- All structural & architectural rules are enforced at lint or compile time. The comprehensive catalogue — each rule, where it is defined, and how to handle exceptions — is [docs/note/note-20260624-technical-code-enforcement.md](docs/note/note-20260624-technical-code-enforcement.md)
- Treat file/function length limits as responsibility-splitting heuristics, not goals by themselves. Default targets are files under 500 lines and functions under 80 lines; when a cohesive responsibility would be harmed by forced splitting, document and adjust the specific lint rule/path instead. State-machine reducers in `client/state/reduce_*.go` are exempt from the function-length limit — dispatch tables stay cohesive (see ARCHITECTURE.md "Layer Structure")
- Actively use libraries. Do not implement from scratch
- Do not overwrite user config files (~/.agent-grid/)
- Always write tests for new features and bug fixes. Do not consider work complete without tests
- Testability is a primary design constraint. Refactor production code (interface extraction, env-var override, dependency injection) when it's needed to enable a test. Per-package coverage targets and the Tier scheme are in `docs/note/note-20260624-agent-testing.md`
- Tests that touch an external dependency must ship the full triple: fake, `FakeVsReal*` e2e backstop, and an invariant-naming contract test (see related ADRs in `docs/adr/`)
- If `FakeVsReal*` fails, fix the fake; do not weaken the assertion
- Use `runtimetest.Harness` for runtime propagation tests instead of ad-hoc bootstraps; use `drivertest.Conformance` / `drivertest.MetadataSourcePriority` for driver contract checks
- Choose test placement by T0-T3 tier; see `docs/note/note-20260624-agent-testing.md`

## Library Selection

Before adding a third-party dependency:
1. List 2-3 candidates with their trade-offs (size, maintenance, license, API fit)
2. Justify the chosen one against the alternatives in the PR description
3. Prefer libraries already in `go.mod` when they cover the use case
4. Wire-format and persistence types must remain stdlib-only

## Documentation

Structured docs live under [`docs/`](docs/note/note-20260624-docs-overview.md): [contributing](docs/note/note-20260624-agent-contributing.md) (this file expanded), [WORKFLOW.md authoring](docs/note/note-20260624-agent-workflow-authoring.md), [testing](docs/note/note-20260624-agent-testing.md), and per-layer internals ([platform](docs/component/component-20260624-platform-overview.md) · [client](docs/component/component-20260624-client-overview.md) · [orchestrator](docs/component/component-20260624-orchestrator-overview.md)).
