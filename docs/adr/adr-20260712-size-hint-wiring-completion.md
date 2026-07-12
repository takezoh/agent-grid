---
id: adr-20260712-size-hint-wiring-completion
kind: adr
title: Complete size hint plumbing from LaunchOptions to termvt.Spec
status: proposed
created: '2026-07-12'
decision_makers:
- Takehito Gondo
tags:
- runtime
- terminal
- pty
- size-ownership
owners: []
relations:
- {type: references, target: spec-20260712-frame-size-ownership}
- {type: partOf, target: plan-20260712-frame-size-ownership}
- {type: references, target: adr-20260624-0004-ptybackend-reuses-pure-core}
source_paths:
- src/client/state/driver_iface.go
- src/client/runtime/backends.go
- src/client/runtime/pty_backend.go
- src/client/runtime/interpret_spawn.go
summary: β scope として定義されていた LaunchOptions.Cols/Rows を backend.SpawnFrame の signature 拡張で termvt.Spec まで貫通させ、RespawnFrame は termvt.Session.Size() 照会で SoT を単一化する。 併せて死 API FrameSize を削除する。 Consequences は三極 (positive/negative/neutral) を本文で記載。
updated: '2026-07-12'
---

## Context

{% context %}
`src/client/state/driver_iface.go:335-337` の `LaunchOptions.Cols` / `LaunchOptions.Rows` は「The runtime bridges these to termvt.Spec on session launch (β scope)」というコメントを伴って定義されているが、リポジトリ内に `.Cols` / `.Rows` を読むコードが存在しない (grep 済み)。 このため全 frame は `termvt.normalizeSize` の 80×24 fallback で spawn され、browser 初回の `fit.fit()` (`src/client/web/src/components/TerminalPane.tsx:131`) が `CmdSurfaceResize` を発火して即座に mid-flight resize が走る。 上流 VT 側の crash window (80×24 → 64 → 63 の揺れ) は上流 lib の fork で修正済みだが、初期表示の見栄え悪化と直後の不要な resize は残っている。

加えて `src/client/runtime/backends.go:65-66` の `FrameInspect.FrameSize` は `pty_backend_test.go` からのテスト呼び出しのみで、production 呼び出しがゼロの死 API になっている (grep 済み)。 将来の消費者は現状の roadmap には存在しない。

`size` の SoT は既存の architecture 決定 (`adr-20260624-0004-ptybackend-reuses-pure-core`) と `src/platform/termvt/session_actor.go:22-23` の設計により、**kernel pty winsize + emulator grid** の 2 owner のみとしている。 hint を配線する際にもこの SoT を壊さないことが制約となる。
{% /context %}

## Decision

{% decision %}
`state.LaunchOptions.Cols/Rows` を既存の `EffSpawnFrame → runtime.spawnFrameWindow → backend.SpawnFrame` 経路の最後まで貫通させる。 具体的には:

1. `FrameLifecycle.SpawnFrame` の signature を `(frameID, name, command, startDir string, env map[string]string, cols, rows int)` に拡張し、`PtyBackend` / `noopBackend` / 全 test fake が新 signature に追随する。
2. `spawnFrameWindow` は `e.Options.Cols/Rows` を `int` にキャストして backend へ渡す。 hint 欠落 (少なくとも一方が 0) は既存 `termvt.normalizeSize` の 80×24 fallback をそのまま利用する (fallback SoT を termvt に集中維持)。
3. `PtyBackend.SpawnFrame` は受け取った cols/rows を `termvt.Spec.Cols/Rows` へ転記する。
4. `spawnFrameWindow` は `slog.Info` で `(hint_cols, hint_rows, effective_cols, effective_rows)` を必ず出力し、fallback 発火時は `slog.Warn` を併記する (silent fallback の可観測化)。
5. `PtyBackend.RespawnFrame` は new session 作成前に既存 `termvt.Session.Size()` を照会し、その値を `termvt.Spec` に載せる。 既存 session が teardown 済みの場合は 80×24 fallback。 `PtyBackend` に in-process の size cache を作らない (SoT を kernel/emulator に単一化)。
6. `FrameInspect.FrameSize` を interface / `PtyBackend` 実装 / `noopBackend` / 全 test fake から削除する。 テスト側で size 参照が必要なケースは `termvt.Session.Size()` 直接呼び出しへ書き換える。
{% /decision %}

## Consequences

{% consequence kind="positive" %}
新規セッションの初期表示が作成元 device の cols/rows で始まり、browser 側 `fit.fit()` 直後の mid-flight resize が消える (spec の NFR-003)。 上流 VT の crash window は既に修正済みだが、structural correctness + defense-in-depth の観点で hint 配線は残す価値がある。
{% /consequence %}

{% consequence kind="positive" %}
size の SoT が `kernel pty winsize + emulator grid` の 2 owner に単一化され、`PtyBackend` に in-process cache が生えない。 `RespawnFrame` の size 継承も `termvt.Session.Size()` 経由なので、SoT owner が増えない (design-quality §SSOT + §決定権の一意化)。
{% /consequence %}

{% consequence kind="positive" %}
死 API `FrameSize` の削除で `FrameInspect` interface が narrower になり、fake 追随の負担が減る (interface segregation)。 テスト用途の size 参照は `termvt.Session.Size()` に一本化される。
{% /consequence %}

{% consequence kind="negative" %}
`FrameLifecycle.SpawnFrame` の signature 変更は全 fake test の追随を要する。 破壊的 signature 変更は同 package の test 込みで 1 unit に閉じる (memory: `feedback_handler_signature_change_single_task`) 運用で吸収するが、初回 PR のレビュー粒度は上がる。
{% /consequence %}

{% consequence kind="negative" %}
`spawnFrameWindow` の slog 出力が spawn hot path に軽い overhead を追加する (kv 4 個追加)。 spawn 頻度が低いため実運用の影響は測定閾値以下だが、hot path であることは記録する。
{% /consequence %}

{% consequence kind="neutral" %}
`RespawnFrame` は options ではなく既存 session `Size()` 照会で size を決めるため、respawn 時に「意図的に別 size で起動し直したい」変更契約は本 ADR では提供しない。 その要求が発生した時点で別 ADR (respawn options 拡張) を追加する — 現状 use case が無いので speculative_generality を避けて延期する (§12 立証責任攻撃を自ら適用)。
{% /consequence %}

## Alternatives

- **backend.SpawnFrame の signature を変えず、PtyBackend に `SetSpawnHint(cols, rows)` を別 API として追加し spawn 前に呼ぶ** — 却下。 size は spawn の atomic な入力であり独立の setter は競合を生む (呼び忘れると silently 80×24 に落ちる)。 契約の穴 (design-quality §契約) を残す。
- **backend.SpawnFrame に `state.LaunchOptions` を直接渡す** — 却下。 runtime → backend 境界に上位 state 型が漏れる (layer 越境)。 primitive int で十分な情報を transit できる。
- **`EffSpawnFrame` に `Cols/Rows` を `Options` から独立したフィールドとして持たせる** — 却下。 `Options.Cols/Rows` と重複した 2 経路が生まれ、決定権 (§SSOT) が分散する。
- **`FrameSize` API を残置 (将来の WORKFLOW.md / orchestrator 消費者を憶測)** — 却下。 design/PRINCIPLES.md §12 立証責任攻撃 — 具体的 roadmap 消費者を挙げられない。 死 API は削除が原則、将来の消費者が現れた時点で対称に復活させる方が cost/complexity の分散が健全。
