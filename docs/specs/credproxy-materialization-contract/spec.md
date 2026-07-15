---
id: spec-20260715-credproxy-materialization-contract
kind: spec
title: credproxy credential materialization contract (Materialize CQRS split, caller-owned retry)
status: draft
created: '2026-07-15'
tags:
- credproxy
- container-provider
- materialization
- cqrs
- restart-safety
owners:
- take.gn
functional_requirements:
- {id: FR-001, statement: "credproxy の `container.Provider` interface SHALL expose an idempotent `Materialize(ctx context.Context, projectPath string) error` method that either (a) prepares any host-side credential state that provider owns for the given projectPath and returns nil, or (b) returns an error describing the specific failure. Repeated calls with the same projectPath and no external state change SHALL leave observable filesystem state unchanged.", priority: must}
- {id: FR-002, statement: "credproxy の `container.Provider.ContainerSpec(ctx, projectPath) (container.Spec, error)` SHALL remain a pure wiring query — signature unchanged, and it MUST NOT perform any credential-state write side effects (no os.WriteFile of token bytes, no directory creation for credential materialization). Wiring information (bind mounts, env, listener addresses) may still be computed.", priority: must}
- {id: FR-003, statement: "credproxy MUST NOT retry `Materialize` internally by default — the credproxy implementation's default retry count SHALL be 0 (fail fast). Retry policy (attempt cap, per-attempt timeout, cadence) is 100% owned by the caller.", priority: must}
- {id: FR-004, statement: "credproxy gcloudcli provider's per-request `/token` metadata handler at metadata.go:78 SHALL invoke `Materialize` asynchronously (fire-and-forget goroutine bound to the SpecBuilder-lifetime ctx, not the per-request ctx) after successful token fetch. HTTP response semantics SHALL remain unchanged — 200 + JSON on token-fetch success, 4xx/5xx on token-fetch failure — irrespective of the async Materialize outcome. `_ = os.WriteFile(...)` silent-swallow MUST be eliminated from the handler.", priority: must}
- {id: FR-005, statement: "The gcloudcli provider's `Materialize` implementation SHALL collapse the three legacy write sites (pre-populate in ensureMetadataServer, periodic sweep in refreshAllTokens, per-request in metadataHandler) into calls to a single internal write path; direct `os.WriteFile(tokenHostPath, ...)` from anywhere other than that single path SHALL be forbidden.", priority: must}
- {id: FR-006, statement: "The gcloudcli materialization write path (idempotency, error branches) SHALL be exercisable by T0/T1 tests that do not require a real gcloud CLI binary or a real daemon restart.", priority: must}
- {id: FR-007, statement: "The gcloud CLI external dependency SHALL ship a registered fake + `//go:build e2e` FakeVsReal backstop + invariant-naming contract test triple, with the triple registered in `test-harness/dependencies.json` under `id=\"gcloud-cli\"` and machine-checked by `src/internal/harnesspolicy`.", priority: must}
- {id: FR-008, statement: "The credproxy fork SHALL be wired into agent-grid's build via a REMOTE-fork replace directive at a content-addressed pseudo-version (matching `src/go.mod:251` and `src/go.mod:253`'s actual precedent for `x/vt` and `ultraviolet`), NOT via a repo-local `forks/` directory or an absolute dev-machine path.", priority: must}
- {id: FR-101, statement: "agent-grid's `resolveOverlaySpecs` (src/platform/agentlaunch/devcontainer.go:427-441) SHALL call both `proxy.ContainerSpec(specCtx, projectPath)` and `proxy.Materialize(specCtx, projectPath)` for each project, and SHALL implement bounded caller-side retry of Materialize failures inside the 30s specCtx envelope with attempt_cap=1 and per-attempt timeout ≤ 6s (= ceil(specCtx 30s / (coldStartContainerConcurrency 4 + 1))). All retry-cadence numbers are agent-grid concerns and live only in this component's implementation.", priority: must}
- {id: FR-102, statement: "agent-grid's `credproxy.Runner` (src/platform/credproxy/) SHALL maintain a per-project per-provider readiness aggregation map constructed *solely from the Runner's own Materialize call outcomes*, and expose it via `ReadinessSnapshot() []ProjectReadiness` (defensive copy). The Runner MUST NOT peek into any credproxy library internal state.", priority: must}
- {id: FR-103, statement: "When agent-grid's `credproxy.Runner` observes a non-nil error from its own `Materialize` call, THEN it SHALL emit exactly one `slog.Warn` line grep-distinguishable from the existing generic `credproxy: provider failed` line, carrying provider name and project path fields. This log signal is emitted by agent-grid Runner, NOT by credproxy library.", priority: must}
non_functional_requirements:
- {id: NFR-001, type: compatibility, criteria: "The `Materialize(ctx, projectPath) error` method addition to `container.Provider` SHALL be strictly additive — the existing `ContainerSpec` signature remains unchanged. All existing providers (awssso / sshagent / hostexec / mcpproxy / secretenv / gcloudcli) SHALL implement `Materialize`; for providers that own no host-side credential state today, a `return nil` no-op is sufficient.", measurement: "grep of container/provider.go confirms exactly one new method (`Materialize`) and no change to any other method signature; `go vet ./...` passes on all 6 providers after adding the no-op / real implementations."}
- {id: NFR-002, type: maintainability, criteria: "The gcloudcli T0/T1 test surface is a PATH-injected fake mirroring fakedocker (adr-20260705-fakedocker-path-injection); the T3 FakeVsReal fidelity backstop uses `//go:build e2e` + `AG_E2E_GCLOUD_BIN` gating (adr-20260704-cli-fake-validated-by-real-cli-e2e); the T2 contract test asserts fake vs real argv/stdout shape.", measurement: "harnesspolicy validator accepts the gcloud-cli triple entry; existing negative fixtures still reject weakened variants."}
- {id: NFR-003, type: performance, criteria: "The metadata handler `/token` endpoint SHALL NOT block on Materialize completion. HTTP round-trip latency for a `/token` request SHALL remain within existing bounds (goroutine spawn overhead only, no `os.WriteFile` on the response path).", measurement: "T1 test with a Materialize implementation that sleeps 500ms asserts the HTTP response returns in < 100ms; no goroutine leak (post-test goroutine count is stable)."}
acceptance:
- {id: AC-001, given: "credproxy's `container.Provider` interface after this iteration", when: "`grep -n 'Materialize(ctx' container/provider.go` runs against the fork tree", then: "exactly one `Materialize(ctx context.Context, projectPath string) error` signature is defined on the interface; `ContainerSpec` signature is byte-identical to pre-change", requirement_refs: [FR-001, FR-002, NFR-001]}
- {id: AC-002, given: "the gcloudcli provider after this iteration; a controlled test environment where `os.WriteFile(tokenHostPath, ...)` is forced to fail once then succeed", when: "the test calls `provider.Materialize(ctx, project)` twice in sequence", then: "the first call returns a non-nil error whose text identifies the write failure; the second call returns nil AND the host-side token file exists with the expected bytes; no internal retry occurred within a single Materialize call (assertable via a call-count counter on the injected gcpToken hook)", requirement_refs: [FR-001, FR-003, FR-005]}
- {id: AC-003, given: "the gcloudcli metadata handler after this iteration; a test that intercepts `/token` HTTP hits with a Materialize hook that sleeps 500ms and then errors", when: "the test issues a single HTTP `GET /token` request", then: "the response returns within < 100ms with status 200 and body containing the token bytes; a background goroutine invoked Materialize; the write-error path did NOT influence the HTTP status; no `os.WriteFile(tokenHostPath, ...)` call is made outside the Materialize implementation path (verifiable by static grep over the fork)", requirement_refs: [FR-004, FR-005, NFR-003]}
- {id: AC-004, given: "agent-grid `resolveOverlaySpecs` after this iteration; injected credproxy provider whose Materialize returns error on attempt 1 and nil on attempt 2", when: "resolveOverlaySpecs runs for a single project under a 30s specCtx", then: "at least 2 Materialize calls are issued within the specCtx envelope; the overall call returns success; attempt count for one specCtx tick SHALL NOT exceed the configured attempt_cap=1 per Materialize invocation (retry is by re-invocation, not internal)", requirement_refs: [FR-101]}
- {id: AC-005, given: "agent-grid `credproxy.Runner` after this iteration; the Runner has just observed a `Materialize` error for project P from provider gcloudcli", when: "an operator reads `Runner.ReadinessSnapshot()`", then: "the returned slice contains exactly one `ProjectReadiness{ProviderName:\"gcloudcli\", ProjectPath:P, Materialized:false, LastError:...}` entry AND a `slog.Warn` line has been emitted by agent-grid Runner with a stable message string grep-distinguishable from `credproxy.go:246` generic warning; the Runner MUST NOT have called any type-assertion to peek credproxy internal state", requirement_refs: [FR-102, FR-103]}
- {id: AC-006, given: "`src/go.mod` contains the new REMOTE-fork replace directive for github.com/takezoh/credproxy at a content-addressed pseudo-version", when: "`cd src && go build ./... && go vet ./... && go list -m github.com/takezoh/credproxy` runs", then: "the build succeeds, `go list -m` reports the replaced pseudo-version, and a T1 test asserting the new Materialize method (which does not exist in the pre-replace pinned version) passes — proving the wiring is what makes the fix real", requirement_refs: [FR-008]}
- {id: AC-007, given: "`test-harness/dependencies.json` contains a new triple entry with `id=\"gcloud-cli\"`", when: "the `src/internal/harnesspolicy` validator runs against `dependencies.json` and, separately, against each existing negative fixture (`marker-missing.json`, `missing-fidelity.json`, `name-only-fake.json`, `empty-assertion.json`) applied to the new entry", then: "the positive entry passes validation and each negative-fixture variant fails validation", requirement_refs: [FR-007, NFR-002]}
- {id: AC-008, given: "the FakeVsReal test runs against a live devcontainer with a real gcloud binary (`AG_E2E_GCLOUD_BIN` set) and the `runDir` bind-mount is provisioned via `devcontainer.go:296`", when: "the test invokes `provider.Materialize(ctx, project)` and reads the resulting token file from inside the container via docker exec / testcontainer stat, comparing bytes to host-side content", then: "the byte contents match, proving host-visible ⇒ container-visible for anything written by Materialize under `runDir`", requirement_refs: [FR-001, FR-007]}
relations:
- {type: implementedBy, target: plan-20260715-credproxy-materialization-contract}
- {type: references, target: adr-20260715-credproxy-materialize-method}
- {type: references, target: adr-20260715-credproxy-retry-owner-caller-side}
- {type: references, target: adr-20260715-credproxy-metadata-handler-async-materialize}
- {type: references, target: adr-20260715-credproxy-fork-remote-replace}
- {type: references, target: adr-20260715-credproxy-recovery-lever-accepts-degraded-window}
- {type: references, target: adr-20260715-credproxy-runner-readonly-aggregation}
source_paths:
- /home/dev/dev/credproxy/providers/gcloudcli/spec.go
- /home/dev/dev/credproxy/providers/gcloudcli/metadata.go
- /home/dev/dev/credproxy/container/provider.go
- src/platform/credproxy/credproxy.go
- src/platform/agentlaunch/devcontainer.go
- src/go.mod
- test-harness/dependencies.json
summary: credproxy container.Provider に Materialize(ctx, projectPath) error を additive 追加し credential 書き込み SSOT を分離。ContainerSpec 不変。default retry = 0。retry cadence と readiness aggregation は agent-grid caller 側に閉じる。credproxy library は generalized のまま。
---

## Goal

credproxy `container.Provider` interface に、credential 状態書き込みの SSOT command として `Materialize(ctx, projectPath) error` を additive に追加する。`ContainerSpec` は pure wiring query として signature 不変。credproxy 側は default retry = 0 (fail fast) とし、agent-grid caller が specCtx envelope 内で retry cadence を所有する。gcloudcli の 3 legacy write site (pre-populate / periodic / metadata /token) を Materialize に集約し、`metadata.go:78` の silent-swallow を async Materialize invocation に refactor して既存 backstop 挙動を保存する。同時に fork wiring は src/go.mod:251,253 の remote-fork replace precedent と一致させ、fake + FakeVsReal + contract triple を test-harness/dependencies.json に登録する。

## Scope

**In scope**:

- credproxy fork 側: (i) `container.Provider` interface に `Materialize(ctx, projectPath) error` を additive 追加; (ii) 5 no-op provider (awssso/sshagent/hostexec/mcpproxy/secretenv) の Materialize 実装 (`return nil`); (iii) gcloudcli の real Materialize 実装 + 3 legacy write site の集約; (iv) metadata.go:78 の async Materialize refactor (silent-swallow 廃止); (v) credproxy 内部 default retry = 0 の pin.
- agent-grid caller 側: (vi) `resolveOverlaySpecs` が Materialize を呼び retry cadence を specCtx envelope 内で持つ; (vii) `credproxy.Runner.ReadinessSnapshot()` (caller 自身の Materialize 呼び出し outcome から構築); (viii) distinguishable slog.Warn signal (agent-grid Runner が emit).
- fork wiring / harness: (ix) src/go.mod への REMOTE-fork replace; (x) fakegcloud + FakeVsReal + contract test triple; (xi) test-harness/dependencies.json の gcloud-cli triple エントリ。

**Out of scope**:

- 単一実測 incident の trigger 特定 (confidence LOW; fix は trigger-independent)。
- awssso / sshagent の実 credential materialize 実装 (0 same-defect hits; no-op で足りる)。
- spec.go:121 の unrelated ctx-ignore smell (別 issue tracked)。
- Runner-centered generic rehydrate-and-verify layer (cut-point evidence 不足)。
- 新 admin HTTP surface / SIGHUP repurpose (adr-20260715-credproxy-recovery-lever-accepts-degraded-window で defer)。
- readiness snapshot の HTTP 公開 (Go API のみ)。
- credproxy に `RetryPolicy` interface / struct を持ち込む (境界越え)。
- credproxy に MaterializationReporter interface を持ち込む (境界越え; caller-side aggregation で代替)。

## Requirements

{% req id="FR-001" %}
credproxy の `container.Provider` interface SHALL expose an idempotent `Materialize(ctx context.Context, projectPath string) error` method that either (a) prepares any host-side credential state that provider owns for the given projectPath and returns nil, or (b) returns an error describing the specific failure. Repeated calls with the same projectPath and no external state change SHALL leave observable filesystem state unchanged.
{% /req %}

{% req id="FR-002" %}
credproxy の `container.Provider.ContainerSpec(ctx, projectPath) (container.Spec, error)` SHALL remain a pure wiring query — signature unchanged, and it MUST NOT perform any credential-state write side effects (no `os.WriteFile` of token bytes, no directory creation for credential materialization). Wiring information (bind mounts, env, listener addresses) may still be computed.
{% /req %}

{% req id="FR-003" %}
credproxy MUST NOT retry `Materialize` internally by default — the credproxy implementation's default retry count SHALL be 0 (fail fast). Retry policy (attempt cap, per-attempt timeout, cadence) is 100% owned by the caller.
{% /req %}

{% req id="FR-004" %}
credproxy gcloudcli provider's per-request `/token` metadata handler at `metadata.go:78` SHALL invoke `Materialize` asynchronously (fire-and-forget goroutine bound to the SpecBuilder-lifetime ctx, not the per-request ctx) after successful token fetch. HTTP response semantics SHALL remain unchanged — 200 + JSON on token-fetch success, 4xx/5xx on token-fetch failure — irrespective of the async Materialize outcome. `_ = os.WriteFile(...)` silent-swallow MUST be eliminated from the handler.
{% /req %}

{% req id="FR-005" %}
The gcloudcli provider's `Materialize` implementation SHALL collapse the three legacy write sites (pre-populate in `ensureMetadataServer`, periodic sweep in `refreshAllTokens`, per-request in `metadataHandler`) into calls to a single internal write path; direct `os.WriteFile(tokenHostPath, ...)` from anywhere other than that single path SHALL be forbidden.
{% /req %}

{% req id="FR-006" %}
The gcloudcli materialization write path (idempotency, error branches) SHALL be exercisable by T0/T1 tests that do not require a real gcloud CLI binary or a real daemon restart.
{% /req %}

{% req id="FR-007" %}
The gcloud CLI external dependency SHALL ship a registered fake + `//go:build e2e` FakeVsReal backstop + invariant-naming contract test triple, with the triple registered in `test-harness/dependencies.json` under `id="gcloud-cli"` and machine-checked by `src/internal/harnesspolicy`.
{% /req %}

{% req id="FR-008" %}
The credproxy fork SHALL be wired into agent-grid's build via a REMOTE-fork replace directive at a content-addressed pseudo-version (matching `src/go.mod:251` and `src/go.mod:253`'s actual precedent for `x/vt` and `ultraviolet`), NOT via a repo-local `forks/` directory or an absolute dev-machine path.
{% /req %}

{% req id="FR-101" %}
agent-grid's `resolveOverlaySpecs` (`src/platform/agentlaunch/devcontainer.go:427-441`) SHALL call both `proxy.ContainerSpec(specCtx, projectPath)` and `proxy.Materialize(specCtx, projectPath)` for each project, and SHALL implement bounded caller-side retry of Materialize failures inside the 30s specCtx envelope with `attempt_cap=1` per Materialize invocation and per-attempt timeout ≤ 6s (= `ceil(specCtx 30s / (coldStartContainerConcurrency 4 + 1))`). All retry-cadence numbers are agent-grid concerns and live only in this component's implementation.
{% /req %}

{% req id="FR-102" %}
agent-grid's `credproxy.Runner` (`src/platform/credproxy/`) SHALL maintain a per-project per-provider readiness aggregation map constructed *solely from the Runner's own Materialize call outcomes*, and expose it via `ReadinessSnapshot() []ProjectReadiness` (defensive copy). The Runner MUST NOT peek into any credproxy library internal state (no type-assertion to library-internal interfaces).
{% /req %}

{% req id="FR-103" %}
When agent-grid's `credproxy.Runner` observes a non-nil error from its own `Materialize` call, THEN it SHALL emit exactly one `slog.Warn` line grep-distinguishable from the existing generic `credproxy: provider failed` line, carrying provider name and project path fields. This log signal is emitted by agent-grid Runner, NOT by credproxy library.
{% /req %}

## Acceptance

{% acceptance id="AC-001" %}
**Given** credproxy's `container.Provider` interface after this iteration,
**When** `grep -n 'Materialize(ctx' container/provider.go` runs against the fork tree,
**Then** exactly one `Materialize(ctx context.Context, projectPath string) error` signature is defined on the interface; `ContainerSpec` signature is byte-identical to pre-change.
{% /acceptance %}

{% acceptance id="AC-002" %}
**Given** the gcloudcli provider after this iteration; a controlled test environment where `os.WriteFile(tokenHostPath, ...)` is forced to fail once then succeed,
**When** the test calls `provider.Materialize(ctx, project)` twice in sequence,
**Then** the first call returns a non-nil error whose text identifies the write failure; the second call returns nil AND the host-side token file exists with the expected bytes; no internal retry occurred within a single Materialize call.
{% /acceptance %}

{% acceptance id="AC-003" %}
**Given** the gcloudcli metadata handler after this iteration; a test that intercepts `/token` HTTP hits with a Materialize hook that sleeps 500ms and then errors,
**When** the test issues a single HTTP `GET /token` request,
**Then** the response returns within < 100ms with status 200 and body containing the token bytes; a background goroutine invoked Materialize; the write-error path did NOT influence the HTTP status; no `os.WriteFile(tokenHostPath, ...)` call is made outside the Materialize implementation path (verifiable by static grep over the fork).
{% /acceptance %}

{% acceptance id="AC-004" %}
**Given** agent-grid `resolveOverlaySpecs` after this iteration; injected credproxy provider whose Materialize returns error on attempt 1 and nil on attempt 2,
**When** resolveOverlaySpecs runs for a single project under a 30s specCtx,
**Then** at least 2 Materialize calls are issued within the specCtx envelope; the overall call returns success; attempt count for one specCtx tick SHALL NOT exceed the configured attempt_cap=1 per Materialize invocation (retry is by re-invocation, not internal).
{% /acceptance %}

{% acceptance id="AC-005" %}
**Given** agent-grid `credproxy.Runner` after this iteration; the Runner has just observed a `Materialize` error for project P from provider gcloudcli,
**When** an operator reads `Runner.ReadinessSnapshot()`,
**Then** the returned slice contains exactly one `ProjectReadiness{ProviderName:"gcloudcli", ProjectPath:P, Materialized:false, LastError:...}` entry AND a `slog.Warn` line has been emitted by agent-grid Runner with a stable message string grep-distinguishable from `credproxy.go:246` generic warning; the Runner MUST NOT have called any type-assertion to peek credproxy internal state.
{% /acceptance %}

{% acceptance id="AC-006" %}
**Given** `src/go.mod` contains the new REMOTE-fork replace directive for `github.com/takezoh/credproxy` at a content-addressed pseudo-version,
**When** `cd src && go build ./... && go vet ./... && go list -m github.com/takezoh/credproxy` runs,
**Then** the build succeeds, `go list -m` reports the replaced pseudo-version, and a T1 test asserting the new `Materialize` method (which does not exist in the pre-replace pinned version) passes.
{% /acceptance %}

{% acceptance id="AC-007" %}
**Given** `test-harness/dependencies.json` contains a new triple entry with `id="gcloud-cli"`,
**When** the `src/internal/harnesspolicy` validator runs against `dependencies.json` and, separately, against each existing negative fixture (`marker-missing.json`, `missing-fidelity.json`, `name-only-fake.json`, `empty-assertion.json`) applied to the new entry,
**Then** the positive entry passes validation and each negative-fixture variant fails validation.
{% /acceptance %}

{% acceptance id="AC-008" %}
**Given** the FakeVsReal test runs against a live devcontainer with a real gcloud binary (`AG_E2E_GCLOUD_BIN` set) and the `runDir` bind-mount is provisioned via `devcontainer.go:296`,
**When** the test invokes `provider.Materialize(ctx, project)` and reads the resulting token file from inside the container via docker exec / testcontainer stat, comparing bytes to host-side content,
**Then** the byte contents match, proving host-visible ⇒ container-visible for anything written by Materialize under `runDir`.
{% /acceptance %}

## Non-Goals

{% non_goals %}
**Must not**
- Introduce a repo-local `forks/` directory (no such precedent; go.mod:251,253 use REMOTE-fork replace).
- Add an absolute dev-machine path to any file committed to the repo (never CI-portable).
- Push agent-grid-specific vocabulary into credproxy interface (specCtx numeric bounds, coldStartContainerConcurrency, ReadinessSnapshot field names, distinguishable slog message strings).
- Introduce a `MaterializationReporter` interface (or equivalent aggregation-reporting shape) into credproxy library.
- Introduce a `RetryPolicy` interface / struct into credproxy library.
- Change `ContainerSpec` signature.
- Add credential-write side effects to `ContainerSpec`.
- Escalate `metadata.go:78`'s per-request write failure to HTTP 5xx (would break existing curl-based backstop).
- Add HTTP endpoints for readiness snapshot in this iteration (in-process Go API only).
- Give `agent-grid credproxy.Runner` ownership of any per-provider state a provider did not itself report via a Materialize outcome the Runner observed.

**Should not**
- Introduce new synchronization primitives beyond the existing `b.mu` (credproxy) / Runner's existing mutex (agent-grid).
- Add unbounded retry loops or exponential backoff schedules (caller-side retry is bounded by specCtx envelope).
- Repurpose SIGHUP or add admin endpoints for recovery in this iteration (deferred per adr-20260715-credproxy-recovery-lever-accepts-degraded-window).
{% /non_goals %}

## Open Questions

- **Materialize idempotency default choice**: "既に materialized なら no-op (skip)" vs "必ず re-materialize (idempotent write)". 前者は gcloud subprocess cost を節約するが staleness を許す。後者は毎回 fresh state を保証するが cost が加算される。実装時に gcloudcli の specifics (token TTL 1800s vs 25min sweep 周期) と併せて決定する。
- **periodic sweep as thin Materialize wrapper**: gcloudcli の `refreshAllTokens` (25min 周期) は Materialize を wrap するだけの薄い adapter にできるはず — for-loop with time.Ticker + `b.Materialize(bgCtx, project)`. 実装時に既存 refreshAllTokens の branching が Materialize と重複なく collapse できるか確認する。
- **metadata.go:78 async invocation backpressure**: best-effort `go func()` で足りるか、それとも errgroup / sync.Semaphore で bound する必要があるか。現状の endpoint hit 頻度 (container 起動時数回) では problem-free だが、将来 workload profile が変わった場合は再検討する。
