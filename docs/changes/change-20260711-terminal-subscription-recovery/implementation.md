---
change: change-20260711-terminal-subscription-recovery
role: implementation
---

# Implementation

## Legacy Source (verbatim)

````markdown
---
id: plan-20260711-terminal-subscription-recovery
kind: plan
title: Web terminal subscription recovery implementation
status: done
created: '2026-07-11'
goal: terminalのdesired subscriptionを単一controllerでreconcileし、一時障害と再接続から自動回復させる
scope_in:
- desired subscriptionと観測状態のSSOT
- single wire writerと同一session ownership handoff
- 一時エラー、恒久エラー、接続断の明示的な遷移
- controller、fake/real transport、React integrationのtest
scope_out:
- daemon protocolへのframe-ready event追加
- server-side scrollback snapshotまたは再送
- 複数terminal pane同時表示
- 新規エラーUI
- WebSocket全体の再接続上限変更
milestones:
- id: m1
  title: Desired reconciliation core
  status: todo
- id: m2
  title: Connection and store integration
  status: todo
- id: m3
  title: Terminal ownership integration
  status: todo
- id: m4
  title: Contract and regression verification
  status: todo
contracts:
- desired subscriptionはconfirmed集合と独立してcontrollerが所有する
- wire上のsubscribe/unsubscribe送信者はcontrollerのreconcile loopだけである
- 同一session acquireはownership handoffでありwire commandを発生させない
- 一時エラーはretry、恒久エラーはblocked、接続断はopen待ちに分類する
tags: []
owners: []
relations:
- {type: implements, target: spec-20260711-terminal-subscription-recovery}
- {type: hasPart, target: adr-20260711-terminal-subscription-desired-reconcile}
source_paths:
- src/client/web/src/socket/
- src/client/web/src/components/TerminalPane.tsx
- src/client/web/src/store/
summary: desired stateを単一wire writerがreconcileし、ownership handoffとerror semanticsを決定的に検証する
updated: '2026-07-11'
---

## Goal

Terminal の表示意図を接続状態から独立して保持し、一つの controller が desired state と wire state の差分を直列に reconcile する。React は ownership lease の acquire/release、Zustand は read-only snapshot の観測だけを担う。

## Implementation Sequence

### m1
{% milestone id="m1" %}
`TerminalSubscriptionController` の純粋な reducer/state model と scheduler を追加する。状態は desired session、ownership token/epoch、connection epoch、phase、attempt、lastError を持つ。reconcile loop だけが wire command を生成し、同一 session acquire は token の handoff、異なる session は旧 unsubscribe 完了後の新 subscribe とする。

エラーを `frame-not-ready`（一時、burst/cooldown）、`connection-closed`（open 待ち）、恒久応答（blocked）、内部不変条件違反（fail fast）に分類する。旧 epoch の非同期結果は状態更新に使わず、その後 current desired を再 reconcile する。
{% /milestone %}

### m2
{% milestone id="m2" %}
既存 `Connection` の request/response と open/close 通知を transport seam に接続し、既存 `SubscriptionRegistry` と直接 `subscribeWithRetry` 呼出しを controller 所有へ集約する。controller snapshot を Zustand slice に一方向投影する。WebSocket close は pending を解放した後に disconnected を通知し、open は connection epoch を進めて desired を再 reconcile する。
{% /milestone %}

### m3
{% milestone id="m3" %}
`TerminalPane` を lease acquire/release のみに変更する。cleanup は取得時 token を release し、controller が current token と同一 session handoff の有無を判定する。既存 keyed remount と sessionId output guard は維持し、component/store に retry timer や wire 判断を残さない。
{% /milestone %}

### m4
{% milestone id="m4" %}
T0 の状態遷移、T1 の fake transport integration、fake/real 共通 contract、`FakeVsReal*` backstop、React regression を追加する。既存 socket/terminal test、frontend build、lint を実行し、ADR 0022 の旧挙動を期待する test は新契約へ更新する。
{% /milestone %}

## Targets

- `src/client/web/src/socket/`: `TerminalSubscriptionController`、desired reducer、retry clock seam、single wire writer。純粋核は WebSocket/Zustand/React を import しない。
- `src/client/web/src/socket/connection.ts`: 既存 pending response と open/close lifecycle を使う transport seam。WebSocket の生成自体は既存 injection/fake pattern を再利用する。
- `src/client/web/src/socket/retry.ts`: retry policy の値計算を再利用または純粋関数化し、継続 ownership は controller に移す。
- `src/client/web/src/store/`: `SubscriptionSnapshotSink` 相当の一方向 projection。store action は wire command を送らない。
- `src/client/web/src/components/TerminalPane.tsx`: acquire/release lease と既存 sessionId output guard。
- Clock seam: production timer と deterministic fake clock を controller constructor へ注入する。
- Transport seam: subscribe/unsubscribe response と command log を fake/real の共通 contract で検証できる狭い adapter とする。
- Lifecycle seam: connection open/close を controller へ通知し、fake と real WebSocket の ordering を同じ suite で検証する。

## Verification

| profile | Tier | 実行コマンド | 判定基準 |
|---|---|---|---|
| controller-transition | T0 pure | `cd src/client/web && npm test -- --run src/socket` | burst/cooldown、blocked、disconnect、epoch無効化、single-flightの遷移表が決定的に成功する |
| connection-wired | T1 wired | `cd src/client/web && npm test -- --run src/socket src/components/TerminalPane.test.tsx` | fake WebSocketでpending close/open、同一session handoff、異session順序、stale outputを再現する |
| transport-contract | T2 contract | `cd src/client/web && npm test -- --run FakeVsReal` | fakeとrealがclose時pending解放、open通知順序、reqId応答対応の同じinvariant-naming contractを満たす |
| frontend-regression | T1 wired | `cd src/client/web && npm test -- --run && npm run build` | Web UI test全件とproduction buildが成功する |
| repository-gate | T2 contract | `make lint && cd src && go test ./...` | layer規則、既存wire contract、全Go testが成功する |

構造規則: pure controller state model が WebSocket/React/Zustand を import しないことは dependency/lint check、wire send 箇所が controller adapter に一意であることは contract test と review、store/component に timer がないことは targeted test と reviewで検証する。


{% transition from="draft" to="active" date="2026-07-11" %}
実装開始
{% /transition %}


{% transition from="active" to="done" date="2026-07-11" %}
controller・Connection・TerminalPane・testsの実装と検証完了
{% /transition %}

````
