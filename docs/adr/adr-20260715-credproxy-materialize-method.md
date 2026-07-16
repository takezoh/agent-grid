---
id: adr-20260715-credproxy-materialize-method
kind: adr
title: 'ADR — credproxy: add `Materialize(ctx, projectPath) error` to container.Provider
  as the SSOT command for credential state writes; ContainerSpec remains a pure wiring
  query'
status: accepted
created: '2026-07-15'
tags:
- adr
- credproxy
- container-provider
- cqrs
- interface-evolution
- boundary
owners:
- take.gn
relations:
- {type: partOf, target: change-20260715-credproxy-materialization-contract}
- {type: references, target: adr-20260715-credproxy-metadata-handler-async-materialize}
- {type: references, target: adr-20260715-credproxy-retry-owner-caller-side}
- {type: references, target: adr-20260715-credproxy-runner-readonly-aggregation}
- {type: references, target: change-20260715-credproxy-materialization-contract}
source_paths: []
decision_makers:
- take.gn
summary: container.Provider に Materialize(ctx, projectPath) error を additive 追加し credential
  書き込み SSOT を分離する。ContainerSpec は pure wiring query のまま signature 不変。CQRS 分割で write
  と wiring を interface level で分ける。gcloudcli の 3 write site は Materialize に集約。
---

## Context

investigation-2.json の RCA は、`gcloudcli.SpecBuilder` (spec.go:181-185 の guard) が「listener registered」と「credential materialized」を混同していること、および `metadata.go:78` の per-request write site が `_ = os.WriteFile(...)` で silent-swallow していることを指摘している。修正の interface 形状には次の 3 案があった:

- **(A) `ContainerSpec` に must-verify semantic を強化する (signature 不変)** — ContainerSpec が返る前に credential materialization を verify する意味を持たせる。
- **(B) `Spec` 構造体に `MaterializationErrors []error` を追加する** — wiring 記述に累積 error 状態を混ぜる。
- **(C) `Materialize(ctx, projectPath) error` method を additive に追加する** — CQRS 分割で write と wiring を分ける。

adversarial verdict が (A) と (B) を blocker と判定した理由:

- **(A) は境界越え**: "must-verify" は devcontainer bind-mount 前提を暗黙に持ち込む。他消費者 (batch / CI ワンショット) には verify latency を強要する overreach。加えて sync token fetch を ContainerSpec に固定し、fake の T2 contract test が過度に狭くなる。
- **(B) は CQRS 破綻**: Spec は launch あたり 1 回の wiring 記述型なのに、時系列・累積 error という observation state を混ぜる。1 つの type に query 責務と observation 責務が同居する。

## Decision

**(C) を採用する。** `container.Provider` interface に以下を additive に追加する:

```go
// Materialize prepares any host-side credential state that this provider owns
// for the given projectPath. Idempotent: repeated calls with the same
// projectPath and no external state change SHALL leave the observable
// filesystem state unchanged. Returns nil on success, or an error describing
// the specific failure (typed if the provider chooses; the interface itself
// carries only error).
//
// Providers with no host-side credential state (e.g. hostexec, mcpproxy,
// secretenv, awssso, sshagent as of this iteration) MAY implement this
// method as a no-op returning nil.
//
// Retry: this method SHALL NOT retry internally. See
// adr-20260715-credproxy-retry-owner-caller-side.
Materialize(ctx context.Context, projectPath string) error
```

`ContainerSpec(ctx, projectPath) (container.Spec, error)` の signature は **完全に不変**。ContainerSpec は wiring 記述 (bind mounts / env / listener addresses / etc.) を返す pure query であり、credential 書き込み side effect を **含めない**。

gcloudcli 側の 3 write site (`ensureMetadataServer` の pre-populate、`refreshAllTokens` の periodic sweep、`metadataHandler` の per-request write at metadata.go:78) は、いずれも `SpecBuilder.Materialize(ctx, projectPath)` を 1 度呼ぶだけの薄い adapter に collapse する。SSOT は `Materialize` の実装 (単一 method) であり、それ以外の場所で `os.WriteFile(tokenHostPath, ...)` を直接呼ぶことは禁じる。

no-op providers (hostexec / mcpproxy / secretenv / awssso / sshagent) は minimum の実装:

```go
func (b *someProvider) Materialize(ctx context.Context, projectPath string) error {
    return nil
}
```

## Consequences

- **CQRS 明示化**: wiring は query (ContainerSpec)、materialization は command (Materialize)。1 method で 2 責務を担うことがなくなり、契約解釈が単純になる。
- **SSOT 確立**: credential state 書き込みは `Materialize` の実装 1 箇所に集約。gcloudcli の 3 write site 重複が消化される。silent-swallow は最上位 caller が error を受け取るので構造的に不可能になる (`metadata.go:78` の refactor は `adr-20260715-credproxy-metadata-handler-async-materialize`)。
- **Additive backwards-compatible**: 既存の `ContainerSpec` consumer は signature 変更なし。5 provider (`awssso` / `sshagent` / `hostexec` / `mcpproxy` / `secretenv` / `gcloudcli`) それぞれが `Materialize` method を実装する必要があるが、5 つのうち 5 つは no-op 1 行で済む (real work は gcloudcli のみ)。既存 `container.Provider` を実装する外部コードは compile-time で "Materialize method missing" と気付けるので、silent breakage にならない。
- **境界 minimal**: interface に持ち込むのは `Materialize(ctx, string) error` のみ。agent-grid 特有の readiness / aggregation / timing 値は一切持ち込まない (`adr-20260715-credproxy-runner-readonly-aggregation`)。
- **他消費者に自然**: batch / CI ワンショット consumer は "wiring だけ欲しい" ときは ContainerSpec を呼び、"credential も準備したい" ときは Materialize を呼ぶ、と選べる。verify latency を強要しない。
- **retry ownership が単純**: `Materialize` は 0 attempts (fail fast) が default (`adr-20260715-credproxy-retry-owner-caller-side`)。caller は返り値 error を見て自分の envelope 内で再試行できる。
- **Materialize の idempotency は default 実装で保証しにくい面がある** — 「既に materialized なら no-op」 vs 「毎回 re-materialize」 の default 選択は Open Question として保留 (spec.md Open Questions 参照)。ただし interface contract 上「同一 projectPath への繰り返し呼び出しは observable FS state を追加変更しない」という semantic は宣言され、gcloudcli 実装が具体化する。

## Alternatives

### (A) must-verify semantic 強化 on `ContainerSpec` (signature 不変)

**却下**: devcontainer bind-mount 前提を interface 意味論に持ち込む境界越え。他消費者に verify latency を強要する overreach。T2 contract test で "fake gcpToken err → ContainerSpec non-nil error" を強制すると ContainerSpec の shape が sync token fetch に固定される。加えて RCA が示す 24 時間後の再発シナリオ (token TTL 1800s + 25min 周期の隙間 + gcloud reauth 期限切れ) は must-verify で救えない。

### (B) `Spec.MaterializationErrors []error` フィールド追加

**却下**: Spec は 1 launch あたり 1 回作られる wiring 記述型なのに、時系列・累積 error という observation 状態を持ち込む CQRS 破綻。1 つの type に query 値と observation state が同居し、consumer が「この Spec は当該 launch の wiring か、それとも観測結果か」を毎回判定する必要が生じる。

### 消費者側 stat-based external verification

**却下**: 各 caller に対して "provider の materialization を外から stat して confirm する" 責務を分散させる。5 provider ごとに verify method が forks され、boundary duplication を招く。SSOT が消え、agent-grid 側で 2 度手間になる。

### ContainerSpec の返り値に materialization error を join する (touple return を膨らませる)

**却下**: signature 変更が必要で backwards compatibility を破る。かつ (B) と同型の CQRS 破綻を再生産する。

### `Materialize` を receive するのを Runner 側に移す (library には interface を持ち込まず、Runner が provider-specific dispatch を持つ)

**却下**: Runner に provider-specific dispatch table を書くと、awssso/gcloudcli/sshagent ごとの branching を Runner が持つことになり、その責務が provider を触るたびに Runner に書き戻される。interface method 1 本のほうが均質で、no-op default で扱える。
