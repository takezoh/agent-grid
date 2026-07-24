---
change: change-20260724-terminal-lifecycle-liveness
role: verification
---

# Terminal lifecycle liveness verification

## Executed in this implementation slice

- `cd src && GOCACHE=/tmp/gocache-agent-grid go test ./...` — pass.
- `cd src && GOCACHE=/tmp/gocache-agent-grid go vet ./...` — pass.
- `make lint` — pass (0 issues).
- `clients/ui`: `npm run test:unit` — 115 files / 1,783 tests pass.
- `clients/ui`: `npm run build` — pass; `npm run lint` passes all 284 files.
- Gateway HOL RED (`TestGatewayLifecycle_UnsubscribeIsNotBlockedByEarlierInboundRPC`) — pass after control/data lane separation.
- Runtime v2 admission/relay packages (`go test ./host/runtime ./host/proto ./server/api`) — pass.
- Lifecycle-focused race suites for gateway and runtime — pass.
- `go test -race ./host/runtime ./server/api -run 'TestGatewayLifecycle|TestLifecycle|Test.*Lifecycle|Test.*Surface'` — pass.
- `make test-e2e` — pass; real-agent fidelity cases skip only when their documented `AG_E2E_*` binaries are absent.
- Playwright browser smoke — 6 pass, 5 explicitly opt-in screenshot cases skipped.
- `FakeVsRealSurfaceLease*` conformance — fake and real PTY release/idempotence cases pass.
- `python3 .../docs_cli.py lint --conformance` — pass (`indexed: 277`, warnings: 0).

The combined repository race command also exercises unrelated workspace RSS
NFR coverage; that independent test is environment-sensitive and is not part
of the lifecycle-specific race gate above. No lifecycle test or production
lint gate is skipped.

## T0 — pure state and algorithm contracts

- Actor reducer covers greater, equal-identical, equal-conflicting and lower revision; accepted/waiting nonterminality; exact release; supersede; apply/deadline tie; and late-result fencing.
- Browser reducer covers every `observing` transition, including publication_replaced before N+1 enqueue, and all replacement/status/timer/close orders. Timeout stores `delivery_timeout` before close.
- Fake-clock case: actor commits applied at 3.9 seconds, delivery passes four seconds, and browser reconnects without changing the old authoritative outcome.
- Dirty-slot linearizability covers update-before-snapshot, update-during-snapshot, update-after-clear, one low-rate event, continuous burst, and final latest preservation.
- Diagnostic partition covers determinate watermark, actor-authoritative `no_output`, unknown/drop/eviction, inconclusive barrier, and conflicting evidence.
- Relay barrier covers complete forwarding, explicit `delivery_gap`, timeout, and status-lane failure.

## T1 — wired behavior

- Extend `gateway_lifecycle_blocking_test.go`: blocked input and resize cannot prevent later lifecycle control.
- `runtimetest.Harness`: blocked runtime/effect work for one owner cannot stop actor reduction for another.
- Production `Connection` tests use its real pending registry, close ordering, onOpen/onClose, and desired replay; accepted/waiting cannot clear the watchdog.
- Ticket/socket recovery failures retain reconnecting and retry without page reload or resource accumulation.
- Gateway pending N replaced by N+1 emits no status; N is already publication_replaced and its canceled watchdog cannot close the socket.
- Old status and output are tested at each browser boundary: pending promise, status projection, terminal buffer, and renderer callback.
- One low-rate output produces each actual stage watermark within 250 milliseconds.
- Sustained burst stays at most 4 Hz, remains latest-one, and eventually emits the final changed value.
- Diagnostic-lane loss is visible in the immediate status marker. Independent status-lane failure closes the socket.
- Terminal status cannot pass admitted output through `finalSequence` without forwarding or `delivery_gap`.
- Playwright switches sessions during injected data/status stalls and observes current terminal recovery without page reload.

## T2 — protocol, authority, and privacy

- Current lifecycle v2 codecs test browser-produced commands, daemon-produced authoritative status/output/diagnostics, and gateway-produced forwarding/gap/final-marker evidence separately.
- Private owner/ticket/nonce/epoch material never appears in browser frame types, state, or logs.
- Every obsolete connection generation, owner mapping, revision, effect/relay epoch, and sequence is rejected before mutation/render.
- Actor-only writer checks prohibit effect worker, gateway, or browser writes to `RevisionOutcome`.
- Current codec tests contain no endpoint-version compatibility, negotiation, rollout, or rollback readiness gate.

## T3 — SurfaceLease fidelity

The repository-required triple is present for the supported PTY surface:

- deterministic fake for cancellation, late completion, acquire, release, and physical cleanup;
- invariant-naming contract for idempotent release within 100 milliseconds,
  map/channel cleanup, and one-owner-one-lease;
- `FakeVsRealSurfaceLease*` against the supported real PTY backend.

If `FakeVsReal*` fails, fix the fake or implementation; never weaken the assertion.

## Commands

```sh
GOCACHE=/tmp/gocache-agent-grid GOLANGCI_LINT_CACHE=/tmp/golangci-lint-cache make lint
cd clients/ui && NPM_CONFIG_CACHE=/tmp/npm-cache-agent-grid npm run lint
cd src && GOCACHE=/tmp/gocache-agent-grid go test ./...
cd clients/ui && npm run test:unit
GOCACHE=/tmp/gocache-agent-grid make test-e2e
cd clients/ui && PLAYWRIGHT_BROWSERS_PATH=/tmp/ms-playwright npm run test:e2e
```

## Success condition

All applicable tiers pass; the original gateway RED cases are green; only the actor writes authoritative outcomes; accepted/waiting do not stop delivery observation; fresh replay fences every old status/output path; low-rate and burst telemetry bounds pass; diagnostic failure is self-reporting; terminal status respects `finalSequence`; `no_output` is never inferred from missing evidence; and no causal claim exceeds the available stage evidence.

Governance must also be complete before readiness: both new ADRs accepted, the old desired-reconcile ADR superseded, and conformance lint at zero errors.
