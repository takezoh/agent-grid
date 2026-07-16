---
id: adr-20260711-server-initiated-severance-signal
kind: adr
title: Deliver server-initiated severance as an observable signal via the existing
  unsubscribe path
status: accepted
created: '2026-07-11'
decision_makers:
- Takehito Gondo
tags:
- runtime
- ipc
- terminal
- reliability
owners: []
relations:
- {type: partOf, target: change-20260711-terminal-output-backpressure}
- {type: references, target: adr-20260711-extend-sever-not-drop-shared-ipc-hops}
- {type: references, target: adr-20260711-terminal-subscription-desired-reconcile}
- {type: references, target: change-20260711-terminal-output-backpressure}
source_paths:
- src/client/runtime/proto_bridge_surface.go
- src/client/runtime/interpret.go
- src/client/proto
- src/client/web/src/socket/terminalSubscription.ts
summary: 既存 EvCmdSurfaceUnsubscribe 経路を拡張しReqIdの黙殺欠陥を修正、新規proto型は追加しない。 Consequences
  は三極を本文 Consequences 節に記載 (このリポジトリの docs schema は spec-detail v1 の consequences/confirmation
  frontmatter フィールドを未サポートのため本文のみ)。
updated: '2026-07-11'
---

## Context

{% context %}
既存の daemon 起点 severance 前例は `internalSurfaceClosed` (termvt の slow-close) → `TerminalRelay.fanOut` が `tr.sendNow(internalSurfaceClosed{...})` → `dispatchInternal` が `shouldApplySlowClose` を確認後 `r.dispatch(state.EvCmdSurfaceUnsubscribe{ConnID, ReqID:"", SessionID, SubscriberID})` を呼ぶ、という経路である。`reduceSurfaceUnsubscribe` は `EffSurfaceSubscribeStop` (wire event を発行しない) と `okResp(ConnID, ReqID="", nil)` を返すが、`okResp` は `proto.EncodeResponse` で wire 化され IPC socket に書き込まれるものの、`ReqID=""` のため gateway 側 `proto.Client.dispatchResponse` の `c.pending[""]` が存在せず応答は黙って破棄される — browser には一切のシグナルが届かない。この欠陥をコードトレースで確認済みであり、`adr-20260711-extend-sever-not-drop-shared-ipc-hops` が導入する backpressure severance をそのままこの経路に流用すると、現状の「間引かれる」バグより悪い「browser が永久に無音のまま固着する」失敗モードを新設することになる。
{% /context %}

## Decision

{% decision %}
新規の `proto.ServerEvent` 型 (`SeveranceSignal` 相当) を追加するのではなく、既存の `EvCmdSurfaceUnsubscribe` 起点経路を拡張して観測可能にすることにする。具体的には (1) `ReqID=""` で `dispatchResponse` に黙って破棄される欠陥を修正し、daemon が能動的に unsubscribe した際は该当セッション宛の browser 通知として届く形にする、(2) この通知の送出は `tr.sendNow` と同様のブロッキング送出 (輻輳の引き金となった同じ select/default drop 経路を経由しない優先配信) で行う。browser 側は `TerminalSubscriptionController` の既存 reconcile ループが、この通知を「この購読の wire 状態が非確定になった」ことのトリガーとして扱い、新しい `TerminalSubscriptionPhase` を追加せずに再購読を試みる — `adr-20260711-terminal-subscription-desired-reconcile` が確立した状態機械はそのまま維持する。
{% /decision %}

## Consequences

{% consequence kind="positive" %}
daemon 起点の severance (backpressure 由来含む) が browser に確実に届くようになり、「間引かれる」よりも悪い「無限フリーズ」という新しい失敗モードを未然に防ぐ。
{% /consequence %}

{% consequence kind="positive" %}
新規 proto.ServerEvent 型を追加しないため、client/proto の型カタログが肥大せず、既存の EvCmdSurfaceUnsubscribe 起点経路の理解だけで severance 通知を追える。
{% /consequence %}

{% consequence kind="negative" %}
既存の internalSurfaceClosed → EvCmdSurfaceUnsubscribe{ReqID:""} 経路 (ReqID の扱い含む) に手を入れるため、termvt process-exit による既存の slow-close 経路への影響がないことを回帰テストで確認する追加コストが発生する。
{% /consequence %}

{% consequence kind="negative" %}
severance 通知を輻輳ホップ非経由の優先配信経路で送るため、tr.sendNow 相当のブロッキング送出を新たに使う箇所が増え、event loop のブロッキング特性を把握すべき箇所が広がる。
{% /consequence %}

{% consequence kind="neutral" %}
frontend TerminalSubscriptionController の状態機械 (TerminalSubscriptionPhase) には新しいカテゴリを追加せず、既存の reconcile ループが自然に再購読する形に留める (adr-20260711-terminal-subscription-desired-reconcile を supersede しない)。
{% /consequence %}

## Alternatives

- **新規 `SeveranceSignal` proto.ServerEvent 型を追加する** — 却下 (critic pass1 blocker: speculative_generality)。既存 EvCmdSurfaceUnsubscribe/okResp 経路の拡張で同じ observable signal を実現できることが確認でき、新規依存導入の立証責任 (design/PRINCIPLES.md §12) を満たせなかった。
- **frontend の TerminalSubscriptionPhase に新カテゴリ (`subscription-severed` 等) を追加する** (DP-d4 option C) — 却下。`adr-20260711-terminal-subscription-desired-reconcile` が確立した状態機械への無自覚な変更になり、当該 ADR を前提として尊重するという制約に反する (adr_conflict)。将来的にどうしても必要になった場合は、本 ADR を supersede する専用 ADR を別途起票する。
- **severance 通知を既存の輻輳ホップ (internalCh / ipcConn.outbox) 経由でそのまま送る** — 却下。通知自体が severance の引き金になった輻輳で drop され得るため、FR-003 が要求する「確実な到達」を満たせない (observability_loss)。