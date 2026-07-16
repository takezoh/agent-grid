---
id: adr-20260715-credproxy-recovery-lever-accepts-degraded-window
kind: adr
title: 'ADR — credproxy: accept degraded-window worst case + operator-observable readiness
  snapshot as the recovery lever set'
status: accepted
created: '2026-07-15'
tags:
- adr
- credproxy
- failure-recovery
- observability
- simplicity
owners:
- take.gn
relations:
- {type: partOf, target: change-20260715-credproxy-materialization-contract}
- {type: references, target: adr-20260715-credproxy-materialize-method}
- {type: references, target: adr-20260715-credproxy-retry-owner-caller-side}
- {type: references, target: adr-20260715-credproxy-runner-readonly-aggregation}
- {type: references, target: change-20260715-credproxy-materialization-contract}
source_paths:
- src/cmd/server/coordinator.go
- src/platform/credproxy/credproxy.go
- src/platform/agentlaunch/devcontainer.go
decision_makers:
- take.gn
summary: 本 iteration では新 operator-facing recovery lever を導入しない。Materialize は idempotent
  で caller が specCtx envelope 内で retry する経路 + 25-min periodic sweep + distinguishable
  slog + Runner.ReadinessSnapshot を lever set とする。SIGHUP repurpose / admin HTTP endpoint
  は future ADR に defer。
---

## Context

critique pass1 の issue-persistent-fail-recovery-window は、boot 時 AdoptFrame の inline retry が失敗した後、25-min periodic sweep が動くまでの間に operator が持てる recovery lever は何かを問うている。3 案があった:

- **(A) SIGHUP を retry sweep trigger に repurpose する** — 現在 SIGHUP は `src/cmd/server/coordinator.go:309-334` で "logged and ignored" (regression test `TestInstallSignalHandlers_SIGHUP_IgnoredKeepsContextAlive` at coordinator_test.go:69 が deliberate な挙動として pin している) — 意図的に無視する設計になっている。
- **(B) 新 admin HTTP endpoint (例: `POST /admin/credproxy/retry`) を server gateway に追加する** — 現在の `src/cmd/server` / `src/server/web` には admin 目的の endpoint 系統は存在しない (verified via grep for `SIGHUP\|adminHandler\|admin.*HTTP` in those trees)。
- **(C) 25-min degraded window を worst case として受け入れる** — 新規 distinguishable log signal と agent-grid 側 `Runner.ReadinessSnapshot()` が operator visibility を提供するので、operator は必要に応じて container を手動で再作成できる (frame recreation は既存パス)。加えて `container.Provider.Materialize` は idempotent なので、caller (agent-grid `resolveOverlaySpecs`) が specCtx envelope 内で bounded retry を行うだけで、boot 直後の failure window の多くは自動で close する。

本 fix が対象とする defect は investigation-2.json で 1 回しか観測されておらず、trigger は LOW confidence。simplicity-critic の "新しい surface / mechanism 追加は立証責任が逆" invariant に照らすと、単一 incident のために SIGHUP semantics 変更や admin surface 導入は overreach。

## Decision

**(C) を採用する。** 本 iteration では operator recovery lever を追加しない:

- **`container.Provider.Materialize(ctx, projectPath) error` は idempotent** な command として設計されており (`adr-20260715-credproxy-materialize-method`)、caller (agent-grid `resolveOverlaySpecs`) は specCtx 30s envelope 内で bounded retry cadence を自身の裁量で組める (`adr-20260715-credproxy-retry-owner-caller-side`)。boot 時の一過性の失敗は caller-side retry で吸収され、25-min window を待たずに解決する場合が多い。
- 25-min periodic sweep (gcloudcli 側の `refreshAllTokens` が `Materialize` を wrap する薄い adapter として実装される予定) が automatic backstop として残る。
- 新 distinguishable slog line (agent-grid 側 Runner が emit — `adr-20260715-credproxy-runner-readonly-aggregation`) が daemon log stream 上で "materialization unconfirmed" を可視化。
- `agent-grid Runner.ReadinessSnapshot()` が in-process Go API で per-project per-provider status を提供 (`adr-20260715-credproxy-runner-readonly-aggregation`)。**Runner は自身の Materialize 呼び出し outcome から map を構築する** — credproxy 内部 state は覗かない。
- Persistent failure に遭遇した operator は、container recreation (既存の frame destroy + recreate パス) を手動で trigger するか、25 分待つ。

将来 persistent failure が積み上がった場合、(A) SIGHUP repurpose か (B) admin HTTP endpoint の追加は future ADR で判断する — その時点で本 ADR を supersede する。

## Consequences

- **failure_recovery contract closes**: automatic recovery time は `min(caller-side retry envelope, next Materialize call, 25 min)` に bounded。caller-side retry (specCtx 30s 内) が最頻の recovery path、25-min sweep が持続的 failure の backstop、manual recovery (container recreation) は常時可能。ambiguity なし。
- **Materialize idempotency 前提**: `adr-20260715-credproxy-materialize-method` が Materialize を idempotent command として pin することで、caller-side retry が安全に組める。仮に将来 idempotency を weaken させる ADR が現れれば、本 ADR も同時に revisit する必要がある。
- **Simplicity-critic cut-point discipline 保持**: 単一観測 incident のために新 admin surface / signal semantic 変更を導入しない。
- **後から lever を追加する道は塞がない**: SIGHUP repurpose の change point は 1 箇所 (coordinator.go:326-334 の switch)。admin HTTP endpoint は既存 `src/server/web/mux.go` パターンにフィットする。
- **25-min worst case を documented SLO 化**: operator が期待できる worst-case degraded window を明示。`ReadinessSnapshot()` が visibility を提供するので "silently 25 分間 broken" にはならない。
- **regression risk**: 実運用で意外な頻度で materialization failure が発生した場合、25 分 window が受け入れられなくなるかもしれない。その場合本 ADR を revisit する。

## Alternatives

### (A) SIGHUP を retry sweep trigger に repurpose する

**却下**: `coordinator.go:309-334` の SIGHUP-ignored semantics は "parent terminal からの spurious SIGHUP で daemon を殺さない" 目的で意図的に選ばれており、`TestInstallSignalHandlers_SIGHUP_IgnoredKeepsContextAlive` が regression test として pin している。この deliberate な挙動を単一 incident の recovery lever のために覆すのは trade-off が悪い。

### (B) 新 admin HTTP endpoint (`POST /admin/credproxy/retry`) を追加する

**却下**: 現在の `src/cmd/server` / `src/server/web` には admin 目的の endpoint パターンが存在しない。単一 incident のためにパターンを新設するのは立証責任が満たされない。もし将来必要になれば mux.go の既存 handler パターンに沿って追加できる (change point は明確)。

### 起動時 verify layer から即座に container recreation を trigger する

**却下**: container recreation は operator が意図した行為であるべきで、fix が自動的に daemon を再起動 relative に broaden するのは overreach。ReadinessSnapshot() を見て operator が判断する経路が正しい境界。
