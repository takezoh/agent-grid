---
id: adr-20260712-size-hint-boundary-validation
kind: adr
title: Validate size hint at HTTP/WS boundary before uint16 narrowing
status: proposed
created: '2026-07-12'
decision_makers:
- Takehito Gondo
tags:
- server
- validation
- reliability
- boundary
owners: []
relations:
- {type: partOf, target: plan-20260712-frame-size-ownership}
- {type: references, target: spec-20260712-frame-size-ownership}
source_paths:
- src/server/web/mux.go
- src/server/web/gateway.go
- src/server/web/wire.go
- src/platform/termvt/session.go
summary: 'HTTP は 400 invalid_cols_rows で reject、WS は warn log + drop、narrow conversion
  前に境界検証する。

  Consequences 三極は本文タグに記載 (docs schema が spec-detail v1 frontmatter フィールド未サポートのため本文のみ)。

  '
---

## Context

{% context %}
web 境界 (`mux.go` apiCreateReq と gateway.go / wire.go の WS 'r' 経路 2 箇所) で cols/rows が `int` で受信され、下流の `LaunchOptions.Cols/Rows` および `CmdSurfaceResize` は `uint16`。この間で narrow conversion が発生するが、現状は 65536 以上の入力が silent に 0 へ wrap し、termvt の `normalizeSize` が 80×24 に fallback する — 呼び出し側の観測点からは「hint 無視」と区別できない。負値も uint16 で unsigned に化ける。境界検証が narrow conversion より前に無いと、silent 分岐が spec の invariant (size hint はそのまま届く / default は明示的な 80×24 のみ) を裏切る。境界検証は HTTP と WS で observability の作法が異なる: HTTP は同期 request/response なので 400 body で status を返せる、WS は既存の非正値 drop 慣行が確立しており connection close は過剰。維持したいのは (a) narrow 前に validate すること、(b) HTTP と WS で validation 述語を共有すること、(c) それぞれの transport に自然な失敗観測手段を選ぶこと。
{% /context %}

## Decision

{% decision %}
narrow conversion (int → uint16) の pre-check として、共通の size-hint validation helper を導入する。helper は 3 条件 ((i) 両方 0 は「未指定」として通過、(ii) いずれか片側だけが 0 は asymmetric として reject、(iii) 1..maxDim (=2000) 範囲外は reject) を判定し、valid / invalid のみを返す (helper 自身は narrow しない)。HTTP 境界 (`mux.go` apiCreateReq) は invalid を **400 `invalid_cols_rows`** body `{"code":"invalid_cols_rows","message":"cols must be 0 or 1..2000; got <n>"}` で reject し、daemon には CreateSession を送らない。WS 境界 (gateway.go / wire.go の 'r' 経路 2 箇所) は invalid を **warn log + drop** で処理し、既存の非正値 drop 慣行と対称に保つ (WS は connection close しない、error frame も追加しない)。helper 実装は 1 箇所 (state package の pure helper) に集約し、HTTP と WS の両 caller が呼ぶ。maxDim=2000 は termvt が SoT なので `termvt.MaxDim` として export し、helper が参照する (platform 層は client を import しない一方向のため合法)。
{% /decision %}

## Consequences

{% consequence kind="positive" %}
narrow conversion による silent wrap-around が仕組みで発生しなくなる。65535 と 0 と 130000 が区別可能になり、debug 時に「hint 未送信 vs hint 無視 vs invalid hint」の 3 分岐が観測点 (HTTP status / WS log / spawn log) から復元できる。
{% /consequence %}

{% consequence kind="positive" %}
validation 述語が 1 箇所 (helper) に集約されるため、HTTP と WS で「片側だけ 0 は asymmetric として reject する」等の policy 変更を将来行う際、touch する箇所が 1 つで済む。shotgun parsing (同じ入力への検証が散る) を仕組みで防げる。
{% /consequence %}

{% consequence kind="negative" %}
既存 browser の初回 fit 経路は妥当な範囲を送っているため regression 影響は極小だが、curl などで手動 create する既存 script / integration test が >maxDim を送っていた場合は 400 で reject される (behavior change)。移行影響は「呼び出し 0 の実測」と「maxDim=2000 は既に termvt 内で clamp されていた事実」から負担 0 の見込みだが、実装フェーズで grep verify が必要。
{% /consequence %}

{% consequence kind="negative" %}
`termvt.MaxDim` を export することで platform 層の API surface が 1 定数分肥大し、将来 maxDim を config 化したくなった場合は「export 済み定数を変数に変える」migration が必要になる (これは現時点で hard-coded 2000 で足りているので過剰設計は避けている)。
{% /consequence %}

{% consequence kind="neutral" %}
WS の resize 経路自体の挙動 (`termvt.Session.Resize`, last-writer-wins) は変更しない。境界検証は前段の hardening のみで、multi-viewer size reconcile policy は本 issue のスコープ外。
{% /consequence %}

## Alternatives

- **silent clamp to [1, maxDim] (HTTP と WS 両方)** — 却下。65535 / 0 / 130000 の 3 種類の invalid を全て同じ観測結果 (spawn 成功) に fold してしまい、debug 不能な silent 分岐を制度化する。error-design-triage §13 (i) 意味論再定義ではなく (ii) 内部契約違反として fail_fast すべき ケース (境界の contract 違反は外部入力起源なので (iii) 回復系だが、回復の形式は fail-fast reject が最も明示的)。
- **silent drop → 80×24 fallback + spawn log で救う** — 却下。FR-005 (spawn 時 hint 反映の observability log) があっても、「なぜ default で spawn したか」の理由 (hint 未送信 vs invalid hint) を log から復元するのは caller 側の負担で、後段の misuse 検出コストが大きい。
- **WS 側を error frame `{k:"e"}` で client に通知する** — 却下。wire.go に新 kind を追加するとスコープ膨張、かつ既存の非正値 drop 慣行と非対称になる。log + drop で observability は運用側で確保できる。
- **WS 側を connection close する** — 却下。hostile client への signal としては強すぎ、真っ当な bug を切ってしまう。境界検査失敗は connection level の異常ではなく単一 frame level の異常。
- **helper を termvt (platform) に置き export する** — 却下。depguard 上は合法だが、validation policy (asymmetric reject / HTTP-vs-WS の分岐) は web 境界の policy であって termvt の invariant ではない。helper 自身は state package の pure function に置き、maxDim 定数のみを termvt から拝借する分離が SoT として正しい。
- **helper を server/web 内 (client 層) に置く** — 却下。state 層の pure invariant として test しやすい配置を優先する。server/web 内配置は 2 caller 間の距離最小だが T0 pure test 記述容易性で敗ける。
