---
id: adr-20260715-geometry-bearing-terminal-attach
kind: adr
title: Separate current observation from geometry-bearing terminal attach
status: accepted
created: '2026-07-15'
decision_makers:
- unknown
tags:
- terminal
- subscription
- geometry
- actor
owners: []
relations:
- {type: partOf, target: change-20260715-terminal-scrollback-reflow}
- {type: references, target: adr-20260711-terminal-subscription-desired-reconcile}
- {type: references, target: change-20260712-frame-size-ownership}
- {type: references, target: change-20260715-terminal-scrollback-reflow}
source_paths:
- src/platform/termvt
- src/client/runtime
- src/server/web
- src/client/web
summary: Make viewer attach a linearizable geometry-bearing size write while internal
  taps observe the current geometry without mutation.
updated: '2026-07-15'
---

## Context

現行 `Session.Subscribe()` は geometry を持たず、browser viewer と `pty_tap` の internal observer が同じ API を使う。browser は rAF 後の resize を別 frame で送るため、subscribe seed が旧 geometry で先行する。単に resize frame を先に送る設計は二要求間の原子性と reconnect geometry freshness を暗黙にする。一方、tap 開始が global PTY size を変更してはならない。

termvt は session size の SoT であり、通常 Resize と multi-viewer last-writer-wins policy は既存契約である。新 viewer attach も producer の PTY winsize を変えるため、この size-write semantics と seed/live linearization を一つの owner で定義する必要がある。

## Decision

termvt の購読 API を viewer 用 `AttachAtGeometry(cols, rows)` と internal observer 用 `SubscribeCurrent()` に分離する。`pty_tap` は `SubscribeCurrent`、`PtyBackend` と browser relay は `AttachAtGeometry` を使う。後者は通常 Resize と同格の session-wide last-writer-wins size write であり、勝者を決める唯一の順序は termvt actor mailbox とする。multi-viewer arbitration policy 自体は変えない。

`AttachAtGeometry` は rollback transaction ではなく、次の linearizable prepare/commit 契約を持つ。

1. prepare で geometry を検証し、fallible PTY SetSize を行う。この段階では VT、size SoT、subscriber、seed/live watermark を公開しない。失敗ならすべて旧状態のまま返す。
2. PTY SetSize 成功後、同じ actor turn で infallible と契約された VT resize、opaque snapshot 生成、seed enqueue、subscriber publish を commit する。publish が linearization point であり、以後の live output は mailbox 順に一度だけ続く。
3. VT invariant/snapshot/enqueue の impossible failure は rollback せず session を unusable にして全 subscriber を sever する。旧 serializer fallback は行わない。

inbound subscribe request は必須 cols/rows を持つ geometry-bearing contract に変更する。outbound terminal output ANSI bytes は維持する。既存 desired subscription controller が唯一の wire writerとして最新 fitted geometry を保持し、初回 fit までは desired を保留する。initial、reconnect、session switch は毎回最新 geometry を subscribe に含め、post-subscribe resize は既存経路を使う。欠落・invalid geometry は購読不成立とし既存 WebSocket warn/drop error postureに合わせる。

cancel/disconnect は connection/ownership generation で seed の採否を決め、stale seed を transport に書かず current desired attach を再 reconcile する。

## Consequences

### Positive

{% consequence kind="positive" %}viewer attach の geometry、PTY producer、VT snapshot、seed/live boundary が一つの actor contract で説明できる。internal tap の no-size-change contract も API から判別でき、初回・reconnect・switch の stale geometry race を除ける。{% /consequence %}

### Negative

{% consequence kind="negative" %}subscribe inbound protocol、runtime consumer、gateway、browser controller の同時移行が必要になる。attach は既存 viewer にも global size change として影響し、PTY commit 後の内部不変条件違反では session fail-fast が必要になる。{% /consequence %}

### Neutral

{% consequence kind="neutral" %}通常 resize、termvt size ownership、last-writer-wins multi-viewer policy、outbound ANSI frame、single wire writer は維持する。AttachAtGeometry は新しい arbitration policy ではなく既存 size-write ordering への参加である。{% /consequence %}

## Alternatives

**resize frame を subscribe より先に送る。** 現行 gateway 順には依存できるが、geometry readiness と二要求間 failure を attach 契約にできないため却下する。

**全 consumer を geometry-bearing API にする。** pty tap の開始が global size を変更し、観測と制御を混同するため却下する。

**subscriber publish 後に snapshot を生成する。** live output との重複・欠落を許し、failure 時に半成立するため却下する。

**fallible operation 全体を rollback する。** PTY SetSize は外部 side effect で完全 rollback を保証できない。fallible step を prepare に限定し、commit 後 failure を内部契約違反として扱う方が観測可能である。

## Confirmation

API-level contract testsで `SubscribeCurrentDoesNotResize`、`AttachAtGeometryIsLastWriterWins`、`AttachPTYFailureLeavesStateUnchanged`、`AttachSeedPrecedesLiveWithoutGap` を固定し、gateway/controller testsで geometry 必須と generation discard を確認する。


{% transition from="proposed" to="accepted" date="2026-07-15" %}
User approved implementation of the designed contract
{% /transition %}
