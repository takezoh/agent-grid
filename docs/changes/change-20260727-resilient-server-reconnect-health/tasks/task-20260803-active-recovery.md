---
id: task-20260803-active-recovery
kind: task
title: Generation-fenced active recovery
status: done
created: '2026-08-03'
priority: critical
effort: large
files_touched:
- clients/ui/src/socket/connection.ts
- clients/ui/src/socket/connection.test.ts
- clients/ui/src/socket/connection.recovery-regression.test.ts
- clients/ui/src/socket/backoff.ts
- clients/ui/src/socket/backoff.test.ts
- clients/ui/src/socket/terminalSubscription.ts
- clients/ui/src/socket/terminalSubscription.severance.test.ts
- clients/ui/src/store/daemon.ts
pr: null
tags: []
owners: []
relations:
- {type: partOf, target: change-20260727-resilient-server-reconnect-health}
source_paths: []
change: change-20260727-resilient-server-reconnect-health
summary: Implement finite hello-confirmed reconnect lifecycle with resource fencing
  and its owning tests.
updated: '2026-08-03'
---

<!-- 本文セクション構成はproducer方法論 (skills/_parts/decompose/knowledge/core.md) が規定する。
     dev-docs skill 自体はセクション構成を強制しない。 -->

## 責務

選択中サーバーのactive `Connection`を、daemon helloだけで復旧成立する有限なlifecycleへ変更する。lifecycle generationとattempt sequenceの二重fence、initiating attempt後の自動16回、再利用可能なmanual Retry、retired resourceのclosureを、同じsocket-layer contextのテストまで含めて完結させる。

## 詳細手順

- injected clock/scheduler、fetch、WebSocketを用いたhostile ordering testを追加してから、private lifecycle tokenとcancelable retryの表現を選ぶ。
- ticket、retry timer、socket open/close/message、daemon hello、store publicationの全continuationでcurrent lifecycleとattemptを照合する。
- WebSocket openは内部transport stateに留め、現在attemptのdaemon helloだけがopen/recoveredを公開し会計をresetする。
- server switch、Retry、shutdownでretired workをcancelまたはfenceし、terminal subscriptionの既存reopen/severance契約を維持する。

## 前提

ADR 0011のtwo-step close、ADR 0012のgateway側unbounded supervisor、ADR 0022のframe-not-ready subscribe retryを維持する。自動回数、hello proof、StatusBar公開stateは設計済みであり変更しない。

## スコープ外

StatusBarのmarkup、fleet health、Connections、gateway daemon supervisor、missed-output replayは扱わない。

## 受け入れ条件

- initiating attempt後に自動retryが正確に16回だけ行われ、17回目を作らずclosed-with-Retryになり、manual lifecycle枯渇後もRetryが再利用できる。
- socket open、ticket成功、old attempt/lifecycleのlate helloではnominalにならず、current attemptのdaemon helloだけが復旧を成立させる。
- Retry連打、server switch、shutdown、late close/messageでlive socket、timer、fetch、callbackが単一flightを超えず、retired workがstoreを変更しない。
- terminal subscriptionの既存close/reopen/severance contract testを弱化せず維持する。
- `cd clients/ui && npm run test:unit -- src/socket/connection.recovery-regression.test.ts src/socket/connection.test.ts src/socket/backoff.test.ts src/socket/terminalSubscription.severance.test.ts` がgreenになる。


{% transition from="todo" to="in_progress" date="2026-08-03" %}
plan-delivery implementation and owning gates accepted
{% /transition %}


{% transition from="in_progress" to="done" date="2026-08-03" %}
implementation integrated; affected mechanical and conformance gates verified
{% /transition %}
