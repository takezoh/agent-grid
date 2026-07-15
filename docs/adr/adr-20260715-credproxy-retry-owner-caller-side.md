---
id: adr-20260715-credproxy-retry-owner-caller-side
kind: adr
title: "ADR — credproxy: retry ownership is caller-only; credproxy internal default retry = 0 (fail fast)"
status: accepted
created: '2026-07-15'
tags:
- adr
- credproxy
- retry
- ownership
- boundary
- failure-recovery
owners:
- take.gn
relations:
- {type: partOf, target: plan-20260715-credproxy-materialization-contract}
- {type: references, target: spec-20260715-credproxy-materialization-contract}
- {type: references, target: adr-20260715-credproxy-materialize-method}
- {type: references, target: adr-20260715-credproxy-recovery-lever-accepts-degraded-window}
source_paths:
- /home/dev/dev/credproxy/container/provider.go
- /home/dev/dev/credproxy/providers/gcloudcli/spec.go
- src/platform/agentlaunch/devcontainer.go
- src/platform/credproxy/credproxy.go
decision_makers:
- take.gn
summary: "credproxy 内部の Materialize retry を 0 attempts (fail fast) に pin し、retry cadence は 100% caller (agent-grid の resolveOverlaySpecs / Runner) が specCtx envelope 内で所有する。library の部分保有 (default 1 attempt + caller envelope) は禁じる。"
---

## Context

adversarial verdict の major finding "retry owner 分割矛盾":

> 「credproxy default 1 attempt + caller envelope retry」を同居させると caller は credproxy が何回試みたか知らずに retry policy を組む。structurally 健全な唯一の分割は **credproxy = 0-attempt (fail fast)、retry は完全に caller (agent-grid) 側**。

3 案があった:

- **(A) credproxy default = 0 attempts (fail fast); retry は 100% caller** — credproxy `Materialize` は 1 度試みて成否を返すだけ。caller が envelope 内で cadence を組む。
- **(B) credproxy default = 1 attempt (silent internal retry) + caller envelope** — credproxy が失敗時に 1 回だけ内部 retry。caller はさらに外側で retry envelope を持つ。
- **(C) credproxy が retry policy を DI で受け取る** — caller が `RetryPolicy` struct を渡し、credproxy がその通りに retry。

## Decision

**(A) を採用する。**

### credproxy 側規範

- `container.Provider.Materialize(ctx, projectPath) error` は **1 度だけ試みて成否を返す**。内部で retry loop を回さない。
- gcloudcli 実装の `Materialize` は `gcpToken` (subprocess exec) を 1 度呼び、成功時に `os.WriteFile` を 1 度呼び、成否を error として返す。retry しない。
- 25-min periodic sweep (`refreshAllTokens`) は "retry" ではなく "periodic re-materialization" — 別 concept で、caller が Materialize を wrap して周期的に呼ぶ薄い adapter として実装する (`adr-20260715-credproxy-materialize-method` の consequences 参照)。
- credproxy library には `RetryPolicy` interface / struct を導入しない (option (C) 却下)。

### caller (agent-grid) 側規範

- `resolveOverlaySpecs` (src/platform/agentlaunch/devcontainer.go:427-441) は specCtx (30s envelope) 内で `proxy.ContainerSpec(specCtx, projectPath)` と `proxy.Materialize(specCtx, projectPath)` の両方を呼ぶ。
- Materialize が error を返した場合、agent-grid 側で bounded retry を組める:
  - `attempt_cap` は specCtx envelope と `coldStartContainerConcurrency=4` から caller が算出する (agent-grid concern)。
  - 具体的 cadence (retry 回数、per-attempt timeout、backoff shape) は agent-grid 実装詳細。credproxy には持ち込まない。
- retry が envelope を使い切っても panic せず、Materialize error を最上位に伝播する。次の frame launch (次の `resolveOverlaySpecs` 呼び出し) が 0 から retry する。

## Consequences

- **retry ownership が unambiguous**: caller は credproxy が 0 回試みたと確信して retry を組める。credproxy 側で「隠された 1 attempt」が無いので、caller の envelope 見積もりが正確になる。
- **境界 clean**: `RetryPolicy` interface / struct が credproxy に露出しない。retry cadence numbers (specCtx 30s / concurrency=4 / attempt_cap=1 / per-attempt=6s) は全て agent-grid 側の実装詳細として `src/platform/agentlaunch/` に閉じる。credproxy library は他 consumer (batch / CI) に "agent-grid の envelope" を強要しない。
- **fail-fast は Materialize idempotency と compatible**: `adr-20260715-credproxy-materialize-method` が Materialize を idempotent と宣言しているので、caller は安全に何度でも Materialize を呼び直せる。
- **caller retry test は agent-grid 側で書ける**: fake credproxy provider を injected して "N 回目で成功" を simulate し、agent-grid Runner の retry cadence を T1 test で検証する。credproxy library の T2 contract test は "single call returns error on failure" だけ assert すればよい (retry の semantic は含まない)。
- **cost bounding は caller responsibility**: specCtx 30s bound、concurrency=4 での fair share (=6s per attempt) といった数値は agent-grid の `resolveOverlaySpecs` テストで assert される。credproxy library には数値契約が無い (implementation-detail に丸投げ)。

## Alternatives

### (B) credproxy default = 1 attempt (silent internal retry) + caller envelope

**却下**: caller は library が何回 try したか知らずに retry を組む必要があり、envelope 見積もりが不正確になる。「credproxy が 1 回 retry したのに caller がさらに 3 回 retry したから合計 4 attempts…あれ、gcloud subprocess は 4 回呼ばれるはずが 8 回呼ばれてる?」のような bug の温床。adversarial verdict が明示的に却下。

### (C) `RetryPolicy` interface を credproxy に持ち込む (DI で caller が渡す)

**却下**: agent-grid 特有の retry cadence 概念 (specCtx envelope、fair-share division by concurrency) を credproxy library の interface に晒す。他 consumer (batch/CI) が RetryPolicy を強制的に選ぶことになる (default policy を意味的に決めなければならず、それ自体が境界越え)。CQRS で言えば retry policy は caller side の control-flow concern であり library の domain concept ではない。

### credproxy が exponential backoff を default で持つ

**却下**: (B) 同型の問題に加えて "backoff timing が library に埋め込まれる" ので caller が envelope 総時間を制御できない。

### `Materialize` を fail-fast にせず、代わりに `Materialize(ctx, projectPath) (attempts int, err error)` にする

**却下**: signature に "内部 attempts の透明化" を持ち込むと、その attempts の semantic (retry-eligible vs config-error vs 意図的 no-op) を library の interface が説明する責務が発生する。fail-fast にしておけば "1 call = 1 attempt" が定義になる。
