---
id: note-20260624-agent-testing
kind: note
title: Testing
status: published
created: '2026-06-24'
updated: '2026-07-05'
tags:
- agent
- legacy-import
owners: []
relations:
- {type: references, target: adr-20260624-0003-termvt-fanout-isolation}
- {type: references, target: adr-20260704-cli-fake-validated-by-real-cli-e2e}
- {type: references, target: adr-20260705-driver-conformance-registry-suite}
- {type: references, target: adr-20260705-eventsink-seam-tap-relay-contracts}
- {type: references, target: adr-20260705-fakedocker-path-injection}
- {type: references, target: adr-20260705-metadata-source-priority}
- {type: references, target: adr-20260705-test-tier-taxonomy}
- {type: references, target: adr-20260705-wire-fixtures-pipeline}
- {type: references, target: design-client}
- {type: references, target: design-platform}
- {type: references, target: note-20260624-technical-code-enforcement}
source_paths:
- src/orchestrator/scheduler/
- src/client/runtime/
- src/platform/termvt/
- src/server/api/
- Makefile
- scripts/check-coverage.sh
- scripts/coverage-floors.txt
- src/cmd/bridge/
topic: agent
summary: Testability is a primary design constraint, not an afterthought. When a function
  reaches for os/exec, the filesystem, a socket, or any other external dependency,
  the path that hits the dependency lives behind an
---

<!-- migrated_from: docs/agent/testing.md -->

# Testing

## Design Principle

Testability is a primary design constraint, not an afterthought. When a function reaches for `os/exec`, the filesystem, a socket, or any other external dependency, the path that hits the dependency lives behind an interface or env-var override so tests can substitute a fake. Refactoring production code to enable a test is in scope; "we can't test it" is a design defect, not a justification.

Concrete patterns in use:

- **Subprocess wrappers** expose a `Runner` interface (e.g. `lib/github.Runner`) with a `DefaultRunner` for production and a fake for tests.
- **External config paths** accept an env-var override (`GEMINI_SETTINGS_PATH`, `CODEX_CONFIG_DIR`).
- **Runtime-injected dependencies** are interfaces, not concrete types (e.g. `runtime/subsystem/stream.RuntimeHook`).
- **`net.Pipe` + fake server** stands in for Unix sockets when verifying the proto client.

## Test Tiers (T0-T3)

The suite classifies tests by **kind of evidence**, not by package criticality. The canonical taxonomy is [ADR — Test tier taxonomy (T0-T3) and the external-dependency triple](../adr/adr-20260705-test-tier-taxonomy.md).

| Tier | Kind | Typical target | Usual runtime |
|------|------|----------------|---------------|
| **T0** | Pure | `state.Reduce`, `Driver.Step`, parsers, codecs, `drivertest.Conformance` | always-on `go test` |
| **T1** | Wired | runtime loop plus fake backend / fake CLI / fake docker | always-on `go test` |
| **T2** | Contract / Fuzz | backend-independent invariants, routing, severance, fuzz | always-on `go test`, plus CI race/fuzz jobs |
| **T3** | Fidelity | fake-vs-real CLI / daemon / docker backstops | opt-in `-tags e2e` and nightly |

This is orthogonal to the S-D coverage tiers below: T0-T3 answers **what kind of test this is**, while S-D answers **how much coverage a package should carry**.

New external dependencies follow the **triple**: ship an in-process fake, a `FakeVsReal*` T3 backstop, and a T2 contract that names the invariant. If T3 fails, fix the fake rather than weakening the assertion.

## Test patterns by layer

Both decision-loop layers (`client/` and `orchestrator/scheduler`) share the Functional Core / Imperative Shell test style: the pure `Reduce` is verified by its return value with no mocks, and the shell is exercised by injecting fakes for its dependencies. `platform/`, a library layer, injects fakes through interface seams. Test files live beside the target as `*_test.go`.

- **`state.Reduce` / `scheduler.Reduce` tests** — no mocks. Pure function tests that verify the return value `(state', []Effect)` of `Reduce(state, event, …)`. No goroutine / channel / timing dependencies; time enters as a value.
- **`Driver.Step` tests** — no mocks. Directly verify the return value `(next, effects, view)` of `Step(prev, driverEvent)`.
- **shell tests** (`client/runtime`, `orchestrator/scheduler` loop) — inject fakes for backend interfaces (`runtime.Config` `noopBackend`/`noopPersist`; scheduler `Deps{ Tracker, Spawn, Clock, … }` with a fake clock). Drive events through the loop and assert the published state.

## Harness Catalog

- **`runtimetest.Harness` (T1)** — boots a real `client/runtime` loop with injected fakes at `New(...)` time and provides `Runtime`, `Enqueue`, `WaitFor`, and `Quiesce` so propagation tests do not need ad-hoc runtime startup code.
- **`drivertest.Conformance` (T0)** — runs the common `state.Driver` contract over every registered driver: Step purity, DriverEvent totality, View/Status totality, and Persist/Restore round-trip. The Step-purity check snapshots the pre-Step state with a JSON clone, so driver state must stay JSON-round-trippable by value; this matches the same `Persist`/`Restore` contract the suite asserts immediately afterward.
- **`drivertest.MetadataSourcePriority` (T0)** — applies the authoritative-vs-fallback metadata contract to a driver-specific scenario; this is separate from registry conformance and must be invoked explicitly where the driver carries metadata state.

## Multiplexed-subsystem routing harness

The stream subsystem multiplexes many frames over one codex app-server
connection; its safety-critical property is **routing isolation** (an event
reaches only the frame that owns its thread). The demux binds each thread
synchronously at creation/resume, so same-cwd frames get distinct ids and cannot
cross-talk by construction. It is pinned by a dedicated harness — direct-drive
contract, a wired fake app-server exercised under `-race`, a stdlib
`FuzzStreamRouting`, and an opt-in real app-server fidelity backstop
([setup](../design/design-client.md#legacy-source-component-20260624-client-stream-backend-e2e)). Full guide:
[stream backend testing](../design/design-client.md#legacy-source-component-20260624-client-stream-backend-testing). This is
the test-pinned enforcement catalogued in
[code-enforcement.md §6](note-20260624-technical-code-enforcement.md).

## Propagation and fidelity harnesses

- **pty tap contract (T2)** — the tap path writes real OSC 0/2/9/133 sequences through a real pty and asserts that only the owning frame receives `EvFrameOsc` / `EvFramePrompt`, while malformed input is contained without killing the loop. Enforcement is catalogued in [code-enforcement.md §8](note-20260624-technical-code-enforcement.md).
- **relay severance contract (T2)** — `TerminalRelay` is driven with a deterministically saturated subscriber to prove that only the slow consumer is severed and all other subscribers keep ordered delivery. Enforcement is catalogued in [code-enforcement.md §9](note-20260624-technical-code-enforcement.md).
- **wire fixtures pipeline (T1 + CI gate)** — Go generates canonical wire JSON fixtures, vitest consumes the same files, and CI fails on regeneration drift. Enforcement is catalogued in [code-enforcement.md §11](note-20260624-technical-code-enforcement.md).
- **gateway scenario e2e (T1)** — a real-`server` + fake-agent scenario now verifies `session create → WS viewUpdate` in the always-on Go suite for the server→view path.
- **clients/ui browser smoke (T1)** — Playwright runs a deterministic fake-backend browser harness for session hydrate, command palette open, and new-session submit. This covers browser wiring that happy-dom cannot prove, while keeping real soft keyboard / assistive-tech flows on the manual-device checklist.
- **fakedocker + `FakeVsRealDocker` (T1 + T3)** — devcontainer lifecycle tests run against PATH-injected `fakedocker`, and an opt-in real-docker backstop verifies the fake's output shape. Enforcement is catalogued in [code-enforcement.md §10](note-20260624-technical-code-enforcement.md).

Local e2e run commands live in [AGENTS.md](../../AGENTS.md) (Build & Test → E2E).

## Fan-out isolation harness (termvt multiplexer)

The backend's `platform/termvt` is the same shape — one source
(a pty) multiplexed to many subscribers — so it carries the analogous
safety-critical property: **fan-out isolation** (every event reaches exactly the
live subscribers of its own session, in order; a slow subscriber is severed, not
allowed to block or corrupt the others). It is pinned by a real-pty contract
(`fanout_contract_test.go`: multi-subscriber delivery, `Manager` cross-talk,
slow-vs-fast containment, control-before-output ordering) run under `-race`, plus
a `server/api` `FuzzApplyInboundProto` over the untrusted client→server frame decode.
Unlike the stream subsystem, termvt has no in-process fake — its only backend is
a real pty — so there is no opt-in e2e tier. Full guide:
[termvt multiplexer testing](../design/design-platform.md#legacy-source-component-20260624-platform-termvt-multiplexer-testing);
rationale: [ADR 0003](../adr/adr-20260624-0003-termvt-fanout-isolation.md);
enforcement: [code-enforcement.md §7](note-20260624-technical-code-enforcement.md).

## Race detector

`make test` stays non-`-race` for the full tree, but the audited
concurrency-sensitive subtrees are pinned behind a dedicated target so race
signal stays actionable instead of drowning in unrelated startup paths:

```sh
make test-race
# → cd src && go test -race -count=1 ./platform/termvt/... ./client/runtime/...
```

This is the canonical "did my concurrency refactor regress something" smoke
test. `platform/termvt` is on the list because the Session actor (single
mainLoop owner + atomic exit state) and the fanout-isolation contract live
there; `client/runtime` is on the list because the single dispatch goroutine
must remain race-free under IPC fan-out.

Adding a subtree: audit it under `-race` locally, fix anything that surfaces,
then append it to the `test-race` recipe in the Makefile in the same PR.

## Coverage Tiers

Coverage targets are tiered by architectural blast radius. A regression in `state` corrupts every session; a regression in `lib/github` typically surfaces as one connector misbehaving.

| Tier | Target | Layer | Members |
|------|--------|-------|---------|
| **S** | ≥85% | Pure domain layer & wire types | `state`, `state/view`, `proto`, `features`, `orchestrator/scheduler` (pure `Reduce` + transitions) |
| **A** | ≥75% | Core execution layer | `runtime`, `runtime/worker`, `runtime/subsystem/stream`, `driver`, `config`, `sandbox/devcontainer`, `platform/termvt`, `uihost`, `server/api` (gateway scenario + browser smoke keep this tier honest) |
| **B** | ≥60% | Infrastructure integrations | `lib/*` (except thin CLI wrappers), `proto/sessions`, `hostexec`, `mcpproxy`, `tools`, `platform/agent/fakecodex` |
| **C** | ≥40% | Thin CLI & wiring | `cmd/claude-app-server`, `cmd/orchestrator`, `runtime/subsystem/cli`, `client/lib/claude/transcript`, `client/lib/codex/transcript` |
| **D** | smoke tests minimum | Trivial packages | `event`, `internal/globutil`, `lib/wsl`, `runtime/subsystem` (shared utilities), `sandbox`, `cmd/server`, `cmd/uihost`, `cmd/bridge`, `cmd/credproxy-run`, `cmd/linear-graphql-cli` |

Tier S and A packages must not lose coverage in a PR. Tier B packages should improve over time; new B-tier code arrives with tests. Tier C packages aim for the goldenpath; full coverage isn't expected. Tier D packages need at least one test that exercises the package surface.

## Running Coverage

```sh
cd src && TMPDIR=/tmp go test -short -cover ./...
```

`TMPDIR=/tmp` is required because the sandbox blocks Unix socket creation under the default `TMPDIR`. Packages that exercise sockets (`proto`, `proto/sessions`, `mcpproxy`, etc.) will fail without it.

Per-package detail:

```sh
cd src && TMPDIR=/tmp go test -coverprofile=/tmp/c.out ./path/to/pkg
go tool cover -func=/tmp/c.out
```

## Enforcement

CI runs `scripts/check-coverage.sh` (the `coverage` step in `.github/workflows/ci.yml`), which executes the full test suite with coverage and compares each package against the floor declared in `scripts/coverage-floors.txt`. Any package below its floor — or any covered package missing from that file — fails the build.

Floors sit a few points below current measurement so legitimate variance does not break the build; the *target* in the Tier table above is the aspiration. When coverage gains stick, raise the floor in the same PR. If a floor must move down, record the measured replacement ceiling in `scripts/coverage-floors.txt` alongside the new value so the recalibration is reviewable.

The `Simplify` workflow (`.github/workflows/simplify.yml`) runs on every pull request and applies the `/simplify` skill (parallel reuse / quality / efficiency review agents) to the diff, fixing defects, leaky abstractions, narration-only comments, no-assert tests, and concrete duplication. Treat its results like any other reviewer.

## When Coverage Can't Be Reached

Some packages still can't hit their Tier target in CI because the dependency is the OS itself — `cmd/bridge` is a process entry point, and low-level helpers such as `platform/lib/tlsdev` still depend on environment-shaped behavior. `platform/sandbox/devcontainer` is no longer the canonical example here: always-on `fakedocker` coverage plus `FakeVsRealDocker` now carry that package's structural backstop. For the remaining low-ceiling packages:

1. Cover everything that doesn't require the external process (pure parsing, command-string assembly, etc.).
2. Document the structural ceiling in the package's test file.
3. Don't lower the Tier target — the gap is a real risk, just not one a unit test can close. Integration tests, not coverage adjustments, are the answer.
