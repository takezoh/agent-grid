---
id: adr-20260712-ws-resize-bound-error-frame
kind: adr
title: Reject out-of-bound lifecycle WS resize with error frame instead of silent drop
status: proposed
created: '2026-07-12'
decision_makers:
- Takehito Gondo
tags:
- server
- websocket
- observability
- size-ownership
owners: []
relations:
- {type: references, target: spec-20260712-frame-size-ownership}
- {type: partOf, target: plan-20260712-frame-size-ownership}
source_paths:
- src/server/web/gateway.go
- src/server/web/mux.go
summary: lifecycle WS の {k:'r'} フレームが cols/rows の値域境界を越えた場合、既存 silent continue ではなく writeRespErrFrame で {k:'e', reason:'invalid_dim'} を返し既存 winsize/grid を維持する。 attach 経路 (response 契約なし) は silent drop 維持。 Consequences は三極 (positive/negative/neutral) を本文で記載。
updated: '2026-07-12'
---

## Context

{% context %}
`src/server/web/gateway.go:353` は lifecycle WebSocket の resize frame (`{k:'r'}`) を受信した際、`msg.Cols <= 0 || msg.Rows <= 0` を silent `continue` (drop) している。 現状これは client が誤って 0 を送った場合のみ発火するため実害が薄い。 しかし本 spec (`spec-20260712-frame-size-ownership`) の FR-006 が導入する `maxSpawnDim` (2000) 境界検証を resize 経路にも適用すると、silent drop 対象が「境界外要求」全般に広がり、client 側で『resize が効かない』を診断する手段が失われる。

lifecycle WS は既に reqId 付き error response frame (`{k:'e'}`) を `writeRespErrFrame` ヘルパーとして持っており (subscribe / unsubscribe で使用中)、error 契約を追加の infrastructure なしで再利用できる。 一方で `readInbound` (attach WS, `gateway.go:539`) の resize フレームは reqId を持たない別契約 (attach 経路は pipe 用途で response 保証なし) — 同じ error frame 化を適用できない。
{% /context %}

## Decision

{% decision %}
lifecycle WS の `{k:'r'}` ハンドラで境界外 (`cols<=0 || rows<=0 || cols>maxSpawnDim || rows>maxSpawnDim`) を検出したら、既存 winsize/grid を維持したうえで `writeRespErrFrame({k:'e', reqId, reason:'invalid_dim'})` を返す。 `sess.Resize` は呼ばない。

attach 経路 (`readInbound` の resize) は response 契約 (reqId) を持たないため、既存の silent drop を維持する。 この非対称は「lifecycle WS が interactive 制御用、attach WS が pipe 用途」という契約差の反映であり、attach 経路が resize を大量に送るのは異常系のみ (本計画では対応スコープ外)。
{% /decision %}

## Consequences

{% consequence kind="positive" %}
境界越え resize が可観測な失敗として client に到達し、debug 手段が保たれる (design/PRINCIPLES.md §0 観察可能性の保存)。 silent 80×24 fallback や silent clamp の regression を prod ログと WS frame から即断できる。
{% /consequence %}

{% consequence kind="positive" %}
`writeRespErrFrame` の既存パターンを再利用するため実装が薄く、lifecycle WS 内の error 契約が一貫する (subscribe/unsubscribe/resize が全て `{k:'e'}` 契約を持つ)。 新規 infrastructure ゼロ。
{% /consequence %}

{% consequence kind="negative" %}
lifecycle WS の error `reason` enum に `invalid_dim` を追加するため、client 側 (`src/client/web/src/socket/`) にも 1 分岐追加を要する。 既存 reason enum との重複回避のため命名を確認する必要がある。
{% /consequence %}

{% consequence kind="neutral" %}
attach 経路の resize は silent drop 維持のため、境界検証の観察可能性は lifecycle WS 経由に限定される。 attach 経路が境界外 resize を送るのは client bug ケースのみを想定しており、その診断は将来の別 issue に譲る (現状 attach WS の resize は response 契約自体を持たないため対称化には契約変更が要る)。
{% /consequence %}

## Alternatives

- **境界外 resize を全経路で silent drop (現行維持)** — 却下。 design/PRINCIPLES.md §13 握り潰し前提 API に抵触し、client が原因不明で resize 不能になり prod でも診断できない。 §0 観察可能性の連鎖を断つ。
- **境界外 resize で WS 接続そのものを close** — 却下。 1 回の境界外 frame で全 session の subscription を落とす blast radius は過剰。 lifecycle WS は複数 session を跨いで制御を持つため、単一 frame の validation error で全体を切ると健全な subscription まで巻き添えになる。
- **境界外 resize を `maxSpawnDim` にサイレントに clamp** — 却下。 client が『2001 rows を要求したのに 2000 で描画された』を認識できず、silent clamp は §0 観察可能性を断つ。 clamp する場合でも result を frame で返さないと同型の握り潰しになる。
- **attach 経路の resize も同じ error frame 化する** — 却下 (現時点)。 attach WS は response 契約 (reqId) を持たない設計であり、error frame の宛先 reqId が無い。 対称化には attach WS の契約変更が必要で、本計画のスコープ外。
