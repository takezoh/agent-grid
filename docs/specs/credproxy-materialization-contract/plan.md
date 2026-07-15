---
id: plan-20260715-credproxy-materialization-contract
kind: plan
title: credproxy credential materialization contract (Alt-A plan)
status: draft
created: '2026-07-15'
goal: credproxy に `Materialize(ctx, projectPath) error` を additive 追加し credential 書き込み SSOT を分離、`ContainerSpec` は pure wiring query のまま signature 不変。credproxy default retry = 0 (fail fast)。gcloudcli 3 write site を Materialize に集約、metadata.go:78 は async Materialize に refactor。retry cadence と readiness aggregation は agent-grid caller (`resolveOverlaySpecs` / `Runner`) 側に閉じる。fork wiring は src/go.mod:251,253 の remote-fork replace precedent と一致。
scope_in:
- credproxy `container.Provider` interface への `Materialize(ctx, projectPath) error` additive 追加
- 5 no-op provider (awssso/sshagent/hostexec/mcpproxy/secretenv) の Materialize 実装
- gcloudcli の real Materialize 実装 + 3 legacy write site (pre-populate / periodic / per-request) の Materialize への集約
- metadata.go:78 の async Materialize refactor (silent-swallow 廃止、HTTP response semantics 保存)
- credproxy 内部 default retry = 0 (fail fast) の pin
- agent-grid `resolveOverlaySpecs` の Materialize 呼び出し + specCtx envelope 内 retry cadence
- agent-grid `credproxy.Runner.ReadinessSnapshot()` の caller-own outcome ベース aggregation
- agent-grid Runner が emit する distinguishable slog.Warn signal
- src/go.mod への REMOTE-fork replace 直接指令 (github.com/takezoh/credproxy pseudo-version)
- PATH-injected fakegcloud + FakeVsReal (container-side probe 込み) + contract test triple
- test-harness/dependencies.json の gcloud-cli triple エントリ
scope_out:
- 単一実測 incident の trigger 特定 (confidence LOW; fix は trigger-independent)
- awssso / sshagent の real credential materialize 実装 (0 same-defect hits; no-op で足りる)
- spec.go:121 の unrelated ctx-ignore smell (別 issue tracked)
- fork の upstream / plain-pin bump (future work)
- Runner-centered generic rehydrate-and-verify (cut-point evidence 不足)
- 新 admin HTTP surface / SIGHUP repurpose (adr で defer)
- readiness snapshot の HTTP 公開 (Go API のみ)
- credproxy に MaterializationReporter / RetryPolicy interface を持ち込む (境界越え)
milestones:
- {id: m1, title: "chunk-01-fork-wiring — Fork wiring: remote-fork replace in src/go.mod", status: todo}
- {id: m2, title: "chunk-02-dependency-registry — Register gcloud-cli dependency triple", status: todo}
- {id: m3, title: "chunk-03-materialize-method — Add container.Provider.Materialize + 5 no-op implementations", status: todo}
- {id: m4, title: "chunk-04-gcloudcli-materialize — gcloudcli real Materialize + 3 write-site collapse", status: todo}
- {id: m5, title: "chunk-05-metadata-async-refactor — metadata.go:78 handler async Materialize refactor", status: todo}
- {id: m6, title: "chunk-06-agent-grid-caller — resolveOverlaySpecs retry envelope + Runner aggregation + slog signal", status: todo}
- {id: m7, title: "chunk-07-fake-and-fidelity-triple — fakegcloud + FakeVsReal (container-side probe) + contract test", status: todo}
contracts:
- contract-materialize-method
- contract-container-spec-purity
- contract-caller-side-retry-envelope
- contract-metadata-handler-async
- contract-runner-aggregation-map
- contract-distinguishable-log-signal
- contract-fork-replace-wiring
- contract-dependency-triple-registration
- contract-fake-and-fidelity-triple
tags:
- credproxy
- container-provider
- materialization
- cqrs
- restart-safety
owners:
- take.gn
relations:
- {type: implements, target: spec-20260715-credproxy-materialization-contract}
- {type: hasPart, target: adr-20260715-credproxy-materialize-method}
- {type: hasPart, target: adr-20260715-credproxy-retry-owner-caller-side}
- {type: hasPart, target: adr-20260715-credproxy-metadata-handler-async-materialize}
- {type: hasPart, target: adr-20260715-credproxy-fork-remote-replace}
- {type: hasPart, target: adr-20260715-credproxy-recovery-lever-accepts-degraded-window}
- {type: hasPart, target: adr-20260715-credproxy-runner-readonly-aggregation}
source_paths:
- /home/dev/dev/credproxy/providers/gcloudcli/spec.go
- /home/dev/dev/credproxy/providers/gcloudcli/metadata.go
- /home/dev/dev/credproxy/container/provider.go
- src/platform/credproxy/credproxy.go
- src/platform/agentlaunch/devcontainer.go
- src/go.mod
- test-harness/dependencies.json
summary: credproxy container.Provider に Materialize(ctx, projectPath) error を additive 追加、CQRS で ContainerSpec と分離。default retry = 0。retry cadence と readiness aggregation は agent-grid caller 側に閉じる。fork wiring は src/go.mod:251,253 の remote-fork replace precedent と一致。
---

## Goal

- credproxy `container.Provider` interface に `Materialize(ctx, projectPath) error` を additive に追加し、credential 状態書き込みの SSOT command を分離する。
- `ContainerSpec` は pure wiring query のまま signature 不変 — credential 書き込み side effect を持たない。
- credproxy 内部 default retry = 0 (fail fast)。retry cadence は 100% caller (agent-grid) 側。
- gcloudcli の 3 write site (pre-populate / periodic / metadata /token) を Materialize に集約。silent-swallow 根絶。
- metadata.go:78 の per-request 経路は async Materialize invocation に refactor し、既存 curl-based backstop / container 側 self-heal 挙動を保存する。
- agent-grid Runner は自身の Materialize 呼び出し outcome から readiness map を構築 — credproxy 内部 state は覗かない。
- fork wiring は src/go.mod:251,253 の実在 remote-fork replace precedent と一致し CI portable。
- test-harness/dependencies.json への gcloud-cli triple 登録で adr-20260711 の validator が machine-check する。

## Implementation Sequence

{% milestone id="chunk-01-fork-wiring" %}
**Fork wiring (prerequisite)** — src/go.mod に remote-fork replace 直接指令を追加。github.com/takezoh/credproxy に fork の HEAD を content-addressed pseudo-version で push、`replace github.com/takezoh/credproxy => github.com/takezoh/credproxy <pseudo-version>` を src/go.mod:251,253 と同 shape で追加、`go mod tidy` で go.sum 反映、`cd src && go build ./... && go vet ./...` で確認。

- Members: `component:component-credproxy-fork-wiring`, `req:FR-008`, `adr:adr-20260715-credproxy-fork-remote-replace`
- Unit: **Wire credproxy fork via remote-fork replace in src/go.mod**
  - files_touched: `src/go.mod`, `src/go.sum`
  - acceptance: replace line 存在; `go build ./...` 成功; `go list -m github.com/takezoh/credproxy` が pseudo-version 報告; forks/ ディレクトリ非導入
  - contract_refs: `contract-fork-replace-wiring`
{% /milestone %}

{% milestone id="chunk-02-dependency-registry" %}
**Register gcloud-cli dependency triple** — test-harness/dependencies.json に id=`gcloud-cli`, strategy=`triple`, seam/fake/contract/fidelity フィールドを追加 (claude-cli, codex-app-server, grok-cli, docker-cli と同 shape)。harnesspolicy validator の positive 経路 + 全 negative fixture を確認。

- Members: `component:component-credproxy-dependency-triple-harness`, `req:FR-007`, `req:NFR-002`, `adr:adr-20260711-test-harness-dependency-admission`
- Unit: **Register gcloud-cli dependency triple in test-harness/dependencies.json**
  - files_touched: `test-harness/dependencies.json`
  - acceptance: 1 つの新 triple エントリ; harnesspolicy validator が accept; 全 negative fixture を reject
  - contract_refs: `contract-dependency-triple-registration`
{% /milestone %}

{% milestone id="chunk-03-materialize-method" %}
**Add `Materialize` method to `container.Provider` + 5 no-op implementations** (fork 内) — `container/provider.go` の Provider interface に `Materialize(ctx context.Context, projectPath string) error` を additive 追加。GoDoc に idempotency と "credproxy shall not retry internally" 規範を書き込む。5 no-op provider (awssso / sshagent / hostexec / mcpproxy / secretenv) に `return nil` の Materialize を実装。fork 内 test suites (各 provider の既存 test) が pass することを確認。

- Members: `component:component-container-provider-materialize-method`, `req:FR-001`, `req:FR-002`, `req:NFR-001`, `adr:adr-20260715-credproxy-materialize-method`
- Depends on: `chunk-01-fork-wiring`
- Units:
  1. **Add `Materialize(ctx, projectPath) error` to container.Provider interface with GoDoc** — files_touched: `container/provider.go`. contract_refs: `contract-materialize-method`, `contract-container-spec-purity`.
  2. **Implement no-op `Materialize` on 5 opt-out providers** — files_touched: `providers/awssso/spec.go`, `providers/sshagent/spec.go`, `providers/hostexec/spec.go`, `providers/mcpproxy/spec.go`, `providers/secretenv/spec.go`. contract_refs: `contract-materialize-method`.
{% /milestone %}

{% milestone id="chunk-04-gcloudcli-materialize" %}
**gcloudcli real Materialize + 3 write-site collapse** (fork 内) — gcloudcli `SpecBuilder.Materialize` を実装 (`gcpToken` 呼び出し + `os.WriteFile(tokenHostPath, ...)` の SSOT path)。3 write site (pre-populate in `ensureMetadataServer`, periodic in `refreshAllTokens`, per-request in `metadataHandler`) を Materialize 呼び出しに置き換え。credproxy 内 default retry = 0 を規範として維持 (`Materialize` は 1 attempt のみで返る、内部 loop 無し)。`ensureMetadataServer` の early-return guard は listener-registered state のみを見る (Materialize 呼び出しは caller 側から発火するので guard に materialized-state を混ぜない)。

- Members: `component:component-gcloudcli-materialize-implementation`, `req:FR-001`, `req:FR-003`, `req:FR-005`, `req:FR-006`, `adr:adr-20260715-credproxy-materialize-method`, `adr:adr-20260715-credproxy-retry-owner-caller-side`
- Depends on: `chunk-01-fork-wiring`, `chunk-03-materialize-method`
- Units:
  1. **Implement gcloudcli.SpecBuilder.Materialize as SSOT write path** — files_touched: `providers/gcloudcli/spec.go`, `providers/gcloudcli/spec_test.go`. contract_refs: `contract-materialize-method`.
  2. **Collapse 3 legacy write sites to Materialize invocations** — files_touched: `providers/gcloudcli/spec.go` (ensureMetadataServer, refreshAllTokens). contract_refs: `contract-materialize-method`.
{% /milestone %}

{% milestone id="chunk-05-metadata-async-refactor" %}
**metadata.go:78 handler async Materialize refactor** (fork 内) — `metadata.go:78` の `_ = os.WriteFile(...)` を `go b.Materialize(bgCtx, project)` (fire-and-forget) に置き換え。HTTP response は従来通り (token fetch 成功時 200 + JSON、失敗時 4xx/5xx)。`bgCtx` は SpecBuilder-lifetime に紐付いた ctx (per-request `reqCtx` は使わない — response 直後に cancel されるため)。既存 `metadata_test.go` の HTTP shape assertion が pass することを確認 + goroutine leak が無いことを T1 test で確認。

- Members: `component:component-gcloudcli-metadata-async-refactor`, `req:FR-004`, `req:FR-005`, `req:NFR-003`, `adr:adr-20260715-credproxy-metadata-handler-async-materialize`
- Depends on: `chunk-04-gcloudcli-materialize`
- Unit: **Refactor metadata handler /token to async Materialize invocation** — files_touched: `providers/gcloudcli/metadata.go`, `providers/gcloudcli/metadata_test.go`. contract_refs: `contract-metadata-handler-async`.
{% /milestone %}

{% milestone id="chunk-06-agent-grid-caller" %}
**agent-grid caller — resolveOverlaySpecs retry envelope + Runner aggregation + slog signal** — `src/platform/agentlaunch/devcontainer.go:427-441` の `resolveOverlaySpecs` を、`proxy.ContainerSpec` に加えて `proxy.Materialize(specCtx, projectPath)` も呼ぶよう変更。Materialize error 時は agent-grid 内で `attempt_cap=1` per Materialize call, per-attempt ≤ 6s の bounded retry cadence を組む (retry は再呼び出しによる — Materialize 内部 loop ではない)。`src/platform/credproxy/credproxy.go` の Runner を、自身の Materialize 呼び出し outcome から per-project per-provider aggregation map を build するよう変更、`ReadinessSnapshot() []ProjectReadiness` (defensive copy) を expose、Materialize error 観測時に distinguishable `slog.Warn` を Runner 自身が emit。**Runner は credproxy library の internal state を覗かない** (type-assert で MaterializationReporter 等の interface を hit しない)。

- Members: `component:component-agent-grid-resolveoverlay-caller`, `component:component-agent-grid-runner-materialize-aggregation`, `req:FR-101`, `req:FR-102`, `req:FR-103`, `adr:adr-20260715-credproxy-retry-owner-caller-side`, `adr:adr-20260715-credproxy-runner-readonly-aggregation`, `adr:adr-20260715-credproxy-recovery-lever-accepts-degraded-window`
- Depends on: `chunk-01-fork-wiring`, `chunk-03-materialize-method` (interface 存在が prerequisite)
- Units:
  1. **Wire resolveOverlaySpecs to call Materialize with specCtx-bounded caller-side retry** — files_touched: `src/platform/agentlaunch/devcontainer.go`, `src/platform/agentlaunch/devcontainer_test.go`. contract_refs: `contract-caller-side-retry-envelope`.
  2. **Add agent-grid Runner readiness aggregation from own Materialize outcomes + distinguishable slog signal** — files_touched: `src/platform/credproxy/credproxy.go`, `src/platform/credproxy/credproxy_test.go`. contract_refs: `contract-runner-aggregation-map`, `contract-distinguishable-log-signal`.
{% /milestone %}

{% milestone id="chunk-07-fake-and-fidelity-triple" %}
**fakegcloud + FakeVsReal (container-side probe 込み) + contract test** — `src/platform/lib/gcloud/fakegcloud/` を新設 (fakedocker mirror)。`gcloud auth print-access-token` 等を PATH 経由 intercept、`//go:build e2e` FakeVsReal は container-side probe (docker exec / testcontainer stat) で bind-mount 経由の container-visible バイト一致を witness。invariant-naming contract test で fake vs real の argv/stdout shape 一致を確認。Makefile test-e2e に追加。ここで test target は Materialize call outcome + resulting host FS state。

- Members: `component:component-gcloudcli-fake-and-fidelity-harness`, `req:FR-006`, `req:FR-007`, `req:NFR-002`, `adr:adr-20260705-fakedocker-path-injection`, `adr:adr-20260704-cli-fake-validated-by-real-cli-e2e`
- Depends on: `chunk-01-fork-wiring`, `chunk-02-dependency-registry`, `chunk-04-gcloudcli-materialize`
- Unit: **Ship PATH-injected fakegcloud + contract test + FakeVsReal (with container-side probe)** — files_touched: `src/platform/lib/gcloud/fakegcloud/fakegcloud.go`, `.../fakegcloud_test.go`, `.../gcloud_cli_e2e_test.go`, `Makefile`. contract_refs: `contract-fake-and-fidelity-triple`.
{% /milestone %}

## Targets

- **Seam layer 1 (fork wiring)**: `src/go.mod` remote-fork replace — precedent lines 251, 253.
- **Seam layer 2 (interface addition)**: `credproxy` fork の `container/provider.go` に `Materialize(ctx, projectPath) error` を additive 追加。5 no-op provider が `return nil` 実装。
- **Seam layer 3 (real Materialize)**: `credproxy` fork の `providers/gcloudcli/spec.go` に `SpecBuilder.Materialize` 実装、3 write site collapse。
- **Seam layer 4 (async refactor)**: `credproxy` fork の `providers/gcloudcli/metadata.go` handler が `go b.Materialize(bgCtx, project)` を呼ぶ形に refactor。
- **Seam layer 5 (caller-side)**: `src/platform/agentlaunch/devcontainer.go` の `resolveOverlaySpecs` が Materialize を呼び、agent-grid 側 retry envelope + Runner aggregation + distinguishable slog line を持つ。
- **Seam layer 6 (harness)**: `src/platform/lib/gcloud/fakegcloud/` (PATH-injected fake); `test-harness/dependencies.json` triple エントリ (harnesspolicy validator machine-check); container-side probe を持つ FakeVsReal (`AG_E2E_GCLOUD_BIN` gated)。
- **Bind-mount invariant** (host-visible ⇒ container-visible): `src/platform/agentlaunch/devcontainer.go:296` `type=bind,source=<runDir>,target=<ContainerRunDir>` — T3 で end-to-end witness。

## Verification

| Profile | Tier | Command | Criterion | Milestone DoD |
|---------|------|---------|-----------|---------------|
| profile-t0-runner | T0 | `cd src && go test ./platform/credproxy/...` | `TestReadinessSnapshot` (from own Materialize outcomes) + `TestReadinessLogDistinguishable` + `TestNoLibraryStatePeek` pass | chunk-06 |
| profile-t1-fork-materialize | T1 | `cd /home/dev/dev/credproxy && go test ./providers/gcloudcli/...` | `TestMaterialize_Idempotent` + `TestMaterialize_FailFast` + `TestMetadataHandler_AsyncMaterialize` + `TestMetadataHandler_NoSilentSwallow` pass | chunk-04, chunk-05 |
| profile-t1-caller-retry | T1 | `cd src && go test ./platform/agentlaunch/...` | `TestResolveOverlaySpecs_MaterializeRetryEnvelope` (attempt_cap=1, per-attempt ≤6s, uses specCtx) pass | chunk-06 |
| profile-t2-harnesspolicy | T2 | `cd src && go test ./internal/harnesspolicy/...` | positive triple accepted + 4 negative fixtures rejected | chunk-02 |
| profile-t3-fidelity | T3 | `AG_E2E_GCLOUD_BIN=/usr/bin/gcloud GOCACHE=/tmp/gocache-agent-grid make test-e2e` | `TestE2E_FakeVsRealShape` + container-side probe bytes match after Materialize call | chunk-07 |

## Implementation Checklist

{% checklist kind="required" %}
- src/go.mod contains a remote-fork replace directive for github.com/takezoh/credproxy
- test-harness/dependencies.json has a valid gcloud-cli triple entry
- container/provider.go declares `Materialize(ctx context.Context, projectPath string) error` on the Provider interface
- awssso / sshagent / hostexec / mcpproxy / secretenv each implement Materialize as `return nil`
- gcloudcli SpecBuilder.Materialize is the single SSOT write path; no `os.WriteFile(tokenHostPath, ...)` calls exist outside it
- credproxy internal retry count for Materialize is 0 (no `for {...retry...}` loop inside Materialize)
- metadata.go:78 handler invokes `go b.Materialize(bgCtx, project)` instead of `_ = os.WriteFile(...)`; HTTP response returns before Materialize completes
- resolveOverlaySpecs calls both ContainerSpec and Materialize; retry cadence lives in agent-grid code (specCtx envelope, attempt_cap=1, per-attempt ≤6s)
- agent-grid Runner ReadinessSnapshot builds its map from Runner's own Materialize call outcomes
- agent-grid Runner does not type-assert to any credproxy library-internal interface for state peeking
- agent-grid Runner emits distinguishable slog.Warn on Materialize error
- fakegcloud PATH-injected fake + FakeVsReal (with container-side probe) + contract test all present
{% /checklist %}

{% checklist kind="recommended" %}
- ProjectReadiness includes `LastError string` for in-process operator diagnostics
- ProjectReadiness includes `LastVerifiedAt time.Time` carrying recency
- gcloudcli.SpecBuilder.Materialize logs (via `slog.Debug`) each write attempt's outcome for future observability enrichment
- refreshAllTokens sweep function's body simplifies to a for-loop that calls Materialize per project (no branching duplication)
{% /checklist %}

{% checklist kind="operational" %}
- Document the caller-side retry cadence (attempt_cap=1, per-attempt ≤6s, specCtx-bounded) in the agent-grid Runner GoDoc so future callers do not re-invent numbers.
- Document that credproxy library-internal retry is 0 (fail fast) so external consumers know they must own retry.
- Document the metadata.go:78 async invocation trade-off (best-effort; no backpressure) for future workload-profile changes.
{% /checklist %}

## Open Questions

- **Materialize idempotency default**: skip-if-materialized vs always-re-materialize の default 選択は実装時に gcloudcli specifics (token TTL 1800s vs 25min sweep 周期) と併せて決める。
- **periodic sweep thinness**: gcloudcli の `refreshAllTokens` (25min 周期) を Materialize を wrap するだけの薄い adapter に collapse できるか — 実装時に既存 branching が Materialize と重複なく畳めるか確認。
- **metadata.go:78 backpressure**: best-effort `go func()` で足りるか errgroup / sync.Semaphore が必要か — 実装時に endpoint hit 頻度と workload profile を照合して判断。
