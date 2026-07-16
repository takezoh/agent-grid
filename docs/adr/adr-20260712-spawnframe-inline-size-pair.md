---
id: adr-20260712-spawnframe-inline-size-pair
kind: adr
title: Extend SpawnFrame with size via inline uint16 pair
status: proposed
created: '2026-07-12'
decision_makers:
- Takehito Gondo
tags:
- runtime
- interface
- backend
owners: []
relations:
- {type: partOf, target: change-20260712-frame-size-ownership}
- {type: references, target: change-20260712-frame-size-ownership}
source_paths:
- src/client/runtime/backends.go
- src/client/runtime/pty_backend.go
summary: 'FrameLifecycle.SpawnFrame の signature に cols/rows uint16 を追加 (opts struct
  化ではなく最小変更) する。

  Consequences 三極は本文タグに記載 (docs schema が spec-detail v1 frontmatter フィールド未サポートのため本文のみ)。

  '
---

## Context

{% context %}
adr-20260712-launch-size-pass-through で決まった素通し経路 (EffSpawnFrame.Options → runtime.spawnFrameWindow → PtyBackend.SpawnFrame → termvt.Spec) を実現するには、`FrameLifecycle.SpawnFrame` interface に size を渡す口が必要。現状 signature は `SpawnFrame(frameID, name, command, startDir string, env map[string]string) error` で、4 実装 (PtyBackend, noopBackend, fakeBackend, blockingBackend) が追随する。size を追加する表現は 3 択: (a) inline uint16 pair (`..., env, cols, rows uint16`) / (b) opts struct 化 (`SpawnFrame(opts SpawnFrameOpts)` に集約) / (c) FrameLifecycle interface に別 method `SpawnFrameSized` を追加。opts struct 化には TODO(B1: startDir を Spec に載せる) との伏線があるが、本 issue で startDir 移行を同時に行う要件はなく (§12 speculative_generality の cut-point 不在攻撃で fail する)、opts struct への集約は「将来の拡張性」だけを理由にする scope creep になる。inline pair は変更行数最小 (4 実装 + interface = 5 面の signature 更新のみ) で、TODO(B1) 実施時に改めて opts struct 化する自由度を残す。
{% /context %}

## Decision

{% decision %}
`FrameLifecycle.SpawnFrame` の signature を `SpawnFrame(frameID, name, command, startDir string, env map[string]string, cols, rows uint16) error` に拡張する。4 実装 (PtyBackend / noopBackend / fakeBackend / blockingBackend) は signature を追随し、PtyBackend のみが cols/rows を `termvt.Spec.Cols/Rows` に転記する。cols=0, rows=0 は termvt.normalizeSize の既存 80×24 fallback に委ねる (spec FR-002 で invariant として明示)。opts struct 化は本 issue では行わず、将来 startDir を Spec に載せる (TODO(B1)) 際に改めて判断する。
{% /decision %}

## Consequences

{% consequence kind="positive" %}
変更行数が最小に閉じる (interface 1 + 実装 4 + caller 1 = 6 箇所の signature 更新)。funlen 影響もほぼゼロ (PtyBackend.SpawnFrame は現在 12 行程度、Spec への 2 行追加のみ)。
{% /consequence %}

{% consequence kind="positive" %}
opts struct を導入しないため、SpawnFrameOpts の定義場所 (backends.go / driver_iface.go / 独立 file?) や field 追加時の default 値 policy をこの issue で決める必要がない。将来 startDir 移行時に「1 度に 2 引数まとめて struct 化」の cut point で判断できる。
{% /consequence %}

{% consequence kind="negative" %}
将来 SpawnFrame に更なる spawn hint (env の複雑化 / worktree path / init state) を追加する必要が出た場合、signature に引数が増え続ける。3 引数程度で opts struct 化するのが Go の慣行であり、cols/rows で 7 引数になる本 signature は将来の struct 化圧力に晒される (対策: TODO(B1) 実施 ADR で opts struct 化を同時に行う)。
{% /consequence %}

{% consequence kind="negative" %}
FrameLifecycle interface の破壊的変更のため、interface を実装する外部コード (現状は本 repo 内 4 実装のみ) がある場合は追随が必要。src/.golangci.yml の depguard 越しに interface が外部露出していないことを実装フェーズで verify する。
{% /consequence %}

{% consequence kind="neutral" %}
driver interface (LaunchPreparer.PrepareLaunch/PrepareCreate) は変更しない (adr-20260712-launch-size-pass-through の帰結)。driver 実装群は size を意識しないまま Prepare* から Options を verbatim 転写するだけ。
{% /consequence %}

## Alternatives

- **b. SpawnFrameOptions struct を新設し signature を SpawnFrame(opts) に集約** — 却下 (§12 cut-point 不在攻撃)。startDir が同時に struct 化するなら 2 cut point の同期変更として正当化できるが、本 issue の scope に startDir 移行は含まれない (out-of-scope で確定)。将来 TODO(B1) 実施時に改めて判断する余地を残すのが正しい cut point 選択。
- **c. FrameLifecycle に別 method `SpawnFrameSized` を追加 (既存 SpawnFrame は temporary に温存)** — 却下。「どっちを実装すべきか」の判断分散を招き、AGENTS.md の backwards-compat hack 禁止 (unused var / removed comment) の精神にも抵触する。interface と実体の乖離を制度化する anti-pattern。
