---
id: adr-20260715-credproxy-runner-readonly-aggregation
kind: adr
title: ADR — agent-grid credproxy Runner is a read-only aggregation built from the
  caller's own Materialize/ContainerSpec outcomes; no peek into credproxy internal
  state
status: accepted
created: '2026-07-15'
tags:
- adr
- credproxy
- ownership
- observability
- boundary
owners:
- take.gn
relations:
- {type: partOf, target: change-20260715-credproxy-materialization-contract}
- {type: references, target: adr-20260715-credproxy-materialize-method}
- {type: references, target: adr-20260715-credproxy-retry-owner-caller-side}
- {type: references, target: change-20260715-credproxy-materialization-contract}
source_paths:
- src/platform/credproxy/credproxy.go
- src/platform/agentlaunch/devcontainer.go
decision_makers:
- take.gn
summary: agent-grid credproxy.Runner は自身が発行した Materialize 呼び出し outcome をそのまま aggregate
  する consumer-side aggregator。ReadinessSnapshot (defensive copy) と distinguishable
  slog を提供するが、credproxy library の内部 state は覗かない。境界は本 ADR で pin される。
---

## Context

`issue-cross-provider-observability-absent` は、operator が「どの project の credential が unconfirmed か」を 1 箇所で問える surface を要求している。同時に adversarial verdict は、agent-grid の関心事 (specCtx 由来 timing、Runner の観測モデル、per-project per-provider aggregation shape) を credproxy library に持ち込むことを禁じている。両者を同時に満たす形は次の 2 案があった:

- **(A) caller-side aggregation** — agent-grid の `credproxy.Runner` が自身で発行した `Materialize` / `ContainerSpec` 呼び出しの outcome (err / no-err) を per-project per-provider の in-memory map に記憶し、その map の defensive copy を `ReadinessSnapshot()` で返す。credproxy library には aggregation の関心を一切持ち込まない。
- **(B) library-side reporter interface** — credproxy library に optional `MaterializationReporter` interface を追加し、Runner が provider を type-assert して internal state を "覗く"。

(B) は adversarial verdict が blocker と判定した境界越えパターン。credproxy library に agent-grid readiness 概念 (last-verified timestamp / lastError の形式 / ReadinessSnapshot 構造) を持ち込むことになり、他 consumer (batch / CI) に不要な interface を強制する。

## Decision

**(A) を採用する。** 具体的境界:

1. `agent-grid Runner.ContainerSpec` / `Runner.Materialize` の per-provider loop で、各 `Materialize(ctx, projectPath)` 呼び出しの返り値 `error` を Runner 自身が観測する。credproxy library の内部 state を type-assert して覗かない。
2. `Materialize` が non-nil error を返した場合、`slog.Warn("credproxy: credential materialization unconfirmed", "provider", ..., "project", ..., "err", ...)` を **agent-grid Runner から** emit する — credproxy library の generic warn line と grep-distinguishable。
3. Runner は per-project per-provider の last-reported `ProjectReadiness{ProjectPath, ProviderName, Materialized bool, LastVerifiedAt time.Time, LastError string}` 値を Runner の既存 mutex 内で保持する map を持つ (新 mutex は導入しない)。map の値は **caller (Runner) 自身の Materialize 呼び出し outcome から構築される** — credproxy library に何も要求しない。
4. `Runner.ReadinessSnapshot() []ProjectReadiness` は上記 map の **defensive copy** を返す。呼び出し側が返り値 slice を mutate しても Runner 内部には影響しない。
5. Runner は **polling / retry / 独自 state 遷移をしない**。map の値は Runner 自身が発行した最後の Materialize 呼び出しの outcome そのまま (retry cadence は `resolveOverlaySpecs` 側 — `adr-20260715-credproxy-retry-owner-caller-side`)。
6. credproxy library で `Materialize` が意味的に no-op (=書き込む状態が無い provider — hostexec / mcpproxy / secretenv / awssso / sshagent の想定 default) の場合、その provider は Runner のこの aggregation map に entry を作らない (silence = healthy、`PeriodicRegistrar` opt-out precedent 準拠)。

**Runner が per-provider の state machine (retry counter、独自 timestamp、polling loop など) を実装し始めることを本 ADR は明示的に禁じる。** 将来そうしたい理由が現れたら、本 ADR を supersede する新 ADR が必要。

**credproxy library に aggregation の関心 (MaterializationReporter / ReadinessSnapshot 相当) を後付けすることも本 ADR は明示的に禁じる。** 境界は「caller が観測する」側で pin されている。

## Consequences

- **境界保持**: credproxy library は "credential proxy が credential 状態を自分で materialize する" 一般化された責務のみを持つ。agent-grid の readiness aggregation 概念は agent-grid 側に閉じている。他 consumer (batch / CI ワンショット) にとっても credproxy interface は minimal のまま。
- **Runner-vs-library ownership 境界が pin される**: 将来の refactor で "credproxy に move する" 誘惑が生じても本 ADR が拒否する根拠になる。
- **Materialize 呼び出しは 1-in / 1-out**: caller が発行して error を受け取る、それだけ。library に extra interface を要求しない (`adr-20260715-credproxy-materialize-method` の additive-only 前提と compatible)。
- **snapshot は in-process Go API のみ**。HTTP endpoint 化は out-of-scope (spec の Non-Goals に明記)。将来必要になれば mux.go の handler pattern に沿って追加可能で、その時点で本 contract を変えずに wrapping できる。
- **実装的には新 mutex を導入せず、Runner の既存 mutex を reuse する**。契約違反を検出する T0 test (defensive copy と caller-own outcome 依存を assert) が chunk-06 で入る。

## Alternatives

### (B) credproxy library に `MaterializationReporter` interface を持ち込んで Runner が type-assert する

**却下**: adversarial verdict が判定した境界越えパターン。agent-grid の観測モデル (last-verified timestamp / lastError shape / ReadinessSnapshot) を credproxy library に押し付けることになり、batch/CI 消費者に不要な interface を強制する。additive とはいえ、意味的には agent-grid concept の leakage。caller-side aggregation は同じ observable を **library に何も要求せず** 達成する。

### Runner が full state machine (polling + retry + timestamps) を持つ

**却下**: retry owner は `adr-20260715-credproxy-retry-owner-caller-side` により caller = `resolveOverlaySpecs` (specCtx envelope 内 cadence) と決まっている。Runner に retry state を持たせるとその決定と衝突する。real caller が 1 provider のみの現状で generic layer を先に置くのは simplicity-critic の cut-point 違反。

### snapshot を HTTP endpoint として直接公開する

**却下**: `src/server/web` に admin/monitoring endpoint pattern が現在存在しない (`adr-20260715-credproxy-recovery-lever-accepts-degraded-window` で verified)。in-process Go API は test でも production でも十分。HTTP 化は future ADR で扱える (本 contract を変更せずに wrapper を追加可能)。

### snapshot が全 provider (opt-out 含む) の entry を含み、opt-out 側は Materialized=true をデフォルトにする

**却下**: opt-out provider の状態を Runner が invent することになり、"Runner が Materialize 呼び出しを発行していない state を捏造する" 禁止事項に違反する。silence = healthy が正しい semantics (`PeriodicRegistrar` precedent 通り)。
