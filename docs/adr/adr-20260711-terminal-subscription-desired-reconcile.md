---
id: adr-20260711-terminal-subscription-desired-reconcile
kind: adr
title: Reconcile desired terminal subscriptions through one wire writer
status: superseded
created: '2026-07-11'
decision_makers:
- unknown
tags: []
owners: []
relations:
- {type: partOf, target: change-20260711-terminal-subscription-recovery}
- {type: references, target: change-20260711-terminal-subscription-recovery}
- {type: supersedes, target: adr-20260624-0022-subscribe-retry-in-socket-layer}
source_paths:
- src/client/web/src/socket/
- src/client/web/src/components/TerminalPane.tsx
- src/client/web/src/store/
summary: desired subscriptionを単一wire writerがreconcileし、一時障害から自動回復する
updated: '2026-07-24'
---

## Context

ADR 0022 は subscribe retry を socket 層に置き、Zustand を観測用途にする判断を確立した。一方で、16回失敗後は `failed` としてユーザーの session 再選択を要求し、再接続時は `confirmed` 集合だけを再購読するとした。実装も requested/retrying/failed を永続的に所有せず、初回 subscribe が枯渇または pending 中に切断すると選択中 session の表示意図を失う。

また、generation は wire protocol に存在しない。同一 session の再取得で旧 cleanup が unsubscribe を送ると新 ownership まで解除し、generation 不一致を理由に送信を捨てるだけでは旧購読が残る。安全性には、非同期結果の採否だけでなく wire command の writer と順序の一意化が必要である。

## Decision

Terminal subscription は socket 層の controller が `desired state` を権威として所有し、単一の reconcile loop だけが subscribe/unsubscribe を送ることにする。Zustand は controller snapshot の read-only projection、React は session ownership lease の acquire/release のみを担う。

同一 session の acquire は wire subscription を作り直さない ownership handoff とする。release は token が current ownership の場合だけ desired を解除する。異なる session への切替は controller が旧 unsubscribe を完了してから新 subscribe を開始する。ownership epoch と connection epoch は遅延した response/timer/cleanup の状態更新を拒否するために使い、wire generation の代替とはみなさない。拒否後は current desired を再 reconcile する。

エラーは次の三分法で扱う。

- 外部由来で一時的な `frame-not-ready` は bounded burst と cooldown で回復し、desired がある限り waiting から再開する。
- 外部由来の `connection-closed` は disconnected で open を待ち、pending/confirmed を問わず desired を新接続で再 reconcile する。再接続 policy の枯渇後も open 通知が来るまで wire retry は行わない。
- session 不在や権限拒否など恒久応答は desired を保持した blocked として自動 retry を止め、desired の変更または明示的な再取得でだけ再開する。
- impossible phase combination、複数 wire writer、token invariant 違反は内部契約違反として fail fast し、fallback 状態で継続しない。

この判断は ADR 0022 の「socket 層が retry を所有する」「Zustand は観測用途」という部分を継承し、「16回後は failed として手動再選択」「confirmed 集合だけを reconnect 時に再購読」を置換する。

## Consequences

### Positive

{% consequence kind="positive" %}購読確認より長寿命な desired state が一時障害と接続断を越えて残り、ユーザー操作なしで terminal が回復する。single wire writer と ownership handoff により、同一 session の旧 cleanup が新購読を解除しない。純粋 reducer、clock/transport/lifecycle seam により T0/T1/T2 の決定的検証が可能になる。{% /consequence %}

### Negative

{% consequence kind="negative" %}controller は desired、wire、connection、ownership の状態を明示的に持つため、従来の confirmed registry より状態数が増える。frame-ready event を導入しない限り、frame-not-ready 中は cooldown timer による低頻度の再試行が続く。{% /consequence %}

### Neutral

{% consequence kind="neutral" %}daemon の wire format、`frame-not-ready` code、keyed terminal remount、sessionId output filter、WebSocket 全体の reconnect policy は変更しない。外部エラーは回復または観測可能な停止、内部 invariant 違反は fail fast という分類を contract test で固定する。{% /consequence %}

## Alternatives

**ADR 0022 の failed/manual reselect を維持する。** 実装は小さいが、ユーザーの desired session が変わっていないのに一時的な readiness race で表示が恒久的に空になるため却下する。

**daemon に frame-ready event を追加する。** timer を排除できるが protocol と server の変更を伴い、今回の問題は既存 response contract 内で回復できるため却下する。

**Zustand を desired state の権威にする。** UI 観測と wire lifecycle の決定権が分散し、React/store/socket の複数 writer を招くため却下する。

**generation を wire protocol に追加する。** 同一 session の旧/new output を完全識別できるが server変更と互換性コストが大きい。単一 writer と同一 session handoffで重複stream自体を作らない設計で要件を満たせるため却下する。


{% transition from="proposed" to="accepted" date="2026-07-11" %}
single wire writerとdesired reconcileを実装・テストで検証
{% /transition %}


{% transition from="accepted" to="superseded" date="2026-07-24" %}
承認済み current lifecycle v2 の actor-only authority と TransportObservation が旧 imperative reconcile を置換
{% /transition %}
