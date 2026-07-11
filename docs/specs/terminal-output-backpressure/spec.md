---
id: spec-20260711-terminal-output-backpressure
kind: spec
title: Terminal surface-output priority over shared IPC outbox
status: approved
created: '2026-07-11'
methodology: sdd
tags:
- runtime
- ipc
- terminal
- reliability
owners:
- take.gn@gmail.com
functional_requirements:
- id: FR-001
  statement: システムは、ある (ConnID, SessionID, SubscriberID) 購読の輻輳・遅延が、別の購読または別セッションの surface-output
    / control-plane イベントの配送順序・到達を損なわないことを、daemon の内部イベントループ・IPC outbox・gateway 側 per-tab
    fan-out の全ホップで保証しなければならない。
  priority: must
  rationale: adr-20260705-eventsink-seam-tap-relay-contracts が termvt の fanout() にのみ確立した「1
    source→多 subscriber の隔離」契約を、同型の他ホップへ一貫適用する invariant FR。
- id: FR-002
  statement: もしある (ConnID, SessionID, SubscriberID) 購読の配信が bounded backlog 閾値を超えて遅延するなら、システムはその購読の
    surface-output ストリームのみを sever し、無関係な購読を巻き添えに sever してはならない。
  priority: must
  rationale: 帰責の正確性 — 共有ホップの輻輳を理由に無関係な victim を巻き込まない。
- id: FR-003
  statement: daemon が backpressure を理由に購読を server 起点で sever したとき、システムは既存の EvCmdSurfaceUnsubscribe
    起点経路を拡張した browser 観測可能な明示的通知を、severance の引き金となった同じ輻輳ホップを経由せずに発行しなければならない。
  priority: must
  rationale: 既存の internalSurfaceClosed→EvCmdSurfaceUnsubscribe{ReqID:""} 経路は okResp
    が dispatchResponse で黙って破棄され browser に一切届かないことをコードトレースで確認済み。これをそのまま流用すると「間引かれる」より悪い「無限フリーズ」を生む。
- id: FR-004
  statement: browser が FR-003 の通知を受信したとき、システムは既存の TerminalSubscriptionController の
    reconcile 機構に新しい状態カテゴリを追加せず、その機構が再購読を試みる形で処理しなければならない。
  priority: must
  rationale: adr-20260711-terminal-subscription-desired-reconcile の状態機械 (frame-not-ready/connection-closed/blocked)
    は前提として尊重し、supersede しない。
- id: FR-005
  statement: もし daemon↔gateway 間の唯一の物理 IPC コネクションが輻輳するなら、システムは全 browser タブ・全セッションを無差別に巻き込む形でその物理コネクション全体を
    sever してはならない。
  priority: must
  rationale: 物理トポロジーは維持する (adr-20260711-keep-single-ipc-connection-topology) ため、隔離は論理層で担保する必要がある。
- id: FR-006
  statement: 対話的 surface-output バーストが特定の購読で発生している間、システムは無関係な購読宛の control-plane イベント
    (sessions-changed / agent-notification / session-file-line) を通常の bounded-queue
    遅延を超えて飢餓・欠落させてはならない。
  priority: must
  rationale: Codex `/resume` のような大量再描画バーストが bulk telemetry を飢餓させない対称性。
- id: FR-007
  statement: control-plane イベントのバーストが発生している間、システムは対話的 surface-output (打鍵エコー含む) を通常の
    bounded-queue 遅延を超えて飢餓・欠落させてはならない。
  priority: must
  rationale: FR-006 と対称 — bulk telemetry のバーストが打鍵エコーを飢餓させない。
- id: FR-008
  statement: システムは、severance の判定ポリシー (閾値・帰責方法) を1箇所の再利用可能な機構として実装しなければならず、internalCh・ipcConn.outbox・DaemonClient
    側 fan-out の各ホップで個別に再実装してはならない。
  priority: must
  rationale: 決定権の一本化 (design-quality 5 invariant)。
- id: FR-009
  statement: もし severance の帰責判定が単一の共有チャンネルの観測のみに基づくなら、システムは購読単位の backlog attribution
    によって、無関係な購読を誤って victim として sever するリスクを排除しなければならない。
  priority: should
  rationale: コスト最小化ではなく構造の正しさを優先するユーザー方針に基づき、ヒューリスティックな誤帰責許容ではなく構造的排除を選ぶ。
- id: FR-010
  statement: システムは、既存のテスト契約 (gateway_terminal_test.go の per-session filter、gateway_view_update_test.go
    の hello/view-update 順序、terminal_relay_test.go の sever/re-subscribe/sequence-reset
    契約) と同じ観測可能な振る舞いを、本設計の変更後も維持しなければならない。
  priority: must
  rationale: 既存 accepted 契約の後方互換保持。
non_functional_requirements:
- id: NFR-001
  type: performance
  criteria: severance 機構の追加は、輻輳が起きていない通常運用のレイテンシ・スループットに測定可能な劣化を与えない。ホットパス (毎チャンクの
    fanOut) は定数時間・低競合であること。
  measurement: 既存ベンチマーク/負荷テストで有意な回帰が無いことを確認する。
- id: NFR-002
  type: reliability
  criteria: severance の発火・伝播・resubscribe による回復の全経路が、複数タブ・複数セッション・再接続競合下でも race free
    であること。
  measurement: -race 下の T2 contract test で pin する (adr-20260705 と同じ流儀)。
- id: NFR-003
  type: scalability
  criteria: 購読数増加に対し severance 判定の per-subscription accounting コストは O(1) per-event
    以下であること。
  measurement: ベンチマークで購読数 N に対し線形以下のスケーリングを確認する。
- id: NFR-004
  type: maintainability
  criteria: 4箇所の select/default drop 分岐が1つの再利用可能な helper/型に集約されていること。
  measurement: grep で select/default drop 分岐が新 helper の外に残っていないことを確認する。
- id: NFR-005
  type: usability
  criteria: severance からの回復時間 (通知受信から termvt reattach snapshot 反映まで) は、ユーザー体感で入力が効かない状態が数秒以上継続しない範囲に収まること。
  measurement: 閾値を意図的に低くした T2 test で severance→resubscribe→resync までの時間を計測する。
acceptance:
- id: AC-001
  given: あるセッションの対話的出力購読が輻輳し backlog 閾値を超えた
  when: daemon が backlog 超過を検知する
  then: その (ConnID,SessionID,SubscriberID) 購読のみが sever され明示的な wire 通知が browser へ届き、無関係セッションのイベントは順序どおり届き続ける
  requirement_refs:
  - FR-001
  - FR-002
  - FR-009
- id: AC-002
  given: severance 通知が browser に届いた
  when: TerminalSubscriptionController がそれを処理する
  then: 新しい phase を追加せず既存の reconcile ループが再購読を試み、termvt の reattach snapshot が画面に反映される
  requirement_refs:
  - FR-003
  - FR-004
- id: AC-003
  given: セッション A で対話的 surface-output バーストが発生している
  when: 同じ共有ホップにセッション B 宛の control-plane イベントが滞留している
  then: セッション B のイベントは bounded-queue 遅延を超えて飢餓しない
  requirement_refs:
  - FR-006
- id: AC-004
  given: control-plane イベントのバーストが発生している
  when: 同じ共有ホップにアクティブセッションの対話的 surface-output が滞留している
  then: 対話的 surface-output は bounded-queue 遅延を超えて飢餓しない
  requirement_refs:
  - FR-007
- id: AC-005
  given: 既存の gateway_terminal_test.go / gateway_view_update_test.go / terminal_relay_test.go
    のテスト群がある
  when: 本設計を適用する
  then: 既存の全アサーションが変更前と同じ観測可能な振る舞いとして pass し続ける
  requirement_refs:
  - FR-010
relations:
- {type: implementedBy, target: plan-20260711-terminal-output-backpressure}
- {type: referencedBy, target: adr-20260711-extend-sever-not-drop-shared-ipc-hops}
- {type: referencedBy, target: adr-20260711-keep-single-ipc-connection-topology}
- {type: referencedBy, target: adr-20260711-priority-lane-interactive-vs-bulk}
- {type: referencedBy, target: adr-20260711-server-initiated-severance-signal}
source_paths:
- src/client/runtime/ipc.go
- src/client/runtime/terminal_relay.go
- src/client/runtime/proto_bridge_surface.go
- src/server/web/daemon_client.go
- src/server/web/gateway.go
summary: 共有・無優先度の IPC 出力チャネルが対話的な surface-output(打鍵エコー)を bulk telemetry に埋もれさせてドロップする問題を、既存の
  sever-not-drop 契約(adr-20260705)の一貫適用と対話的/bulk レーン分離で構造的に修正する。
updated: '2026-07-11'
---

## Overview

agent-grid の Web UI で、TERMINAL 入力欄への打鍵エコーの大半が画面に反映されない不具合が確認された。根本原因は、daemon↔gateway 間の唯一の共有 IPC チャネル (`internalCh` / `ipcConn.outbox` / gateway 側 `DaemonClient` の per-tab fan-out) が、対話的な surface-output (打鍵エコー) と bulk telemetry (session-file-line 等、セッションIDで絞られない全daemon分の通知) を優先度なく同居させ、輻輳時に `select{default: drop}` で無言破棄することにある。Codex CLI の `/resume` のような大量再描画バーストがこの共有ホップを溢れさせ、対話的な打鍵エコーが間引かれたまま browser の画面が stale に固着する。本 spec は、既に `adr-20260705-eventsink-seam-tap-relay-contracts` が termvt の fanout() に確立している「追従できない subscriber は sever し、他 subscriber / 他 session の配送順序は維持する」契約を、この原則が及んでいない残りのホップへ一貫適用し、かつ対話的/bulk のレーン分離で相互飢餓を防ぐことで、対症療法 (バッファ拡大) ではない構造的な修正を定義する。

## Requirements

{% req id="FR-001" %}
システムは、ある (ConnID, SessionID, SubscriberID) 購読の輻輳・遅延が、別の購読または別セッションの surface-output / control-plane イベントの配送順序・到達を損なわないことを、daemon の内部イベントループ・IPC outbox・gateway 側 per-tab fan-out の全ホップで保証しなければならない。
{% /req %}

{% req id="FR-002" %}
もしある (ConnID, SessionID, SubscriberID) 購読の配信が bounded backlog 閾値を超えて遅延するなら、システムはその購読の surface-output ストリームのみを sever し、無関係な購読を巻き添えに sever してはならない。
{% /req %}

{% req id="FR-003" %}
daemon が backpressure を理由に購読を server 起点で sever したとき、システムは既存の EvCmdSurfaceUnsubscribe 起点経路を拡張した browser 観測可能な明示的通知を、severance の引き金となった同じ輻輳ホップを経由せずに発行しなければならない。
{% /req %}

{% req id="FR-004" %}
browser が FR-003 の通知を受信したとき、システムは既存の TerminalSubscriptionController の reconcile 機構に新しい状態カテゴリを追加せず、その機構が再購読を試みる形で処理しなければならない。
{% /req %}

{% req id="FR-005" %}
もし daemon↔gateway 間の唯一の物理 IPC コネクションが輻輳するなら、システムは全 browser タブ・全セッションを無差別に巻き込む形でその物理コネクション全体を sever してはならない。
{% /req %}

{% req id="FR-006" %}
対話的 surface-output バーストが特定の購読で発生している間、システムは無関係な購読宛の control-plane イベント (sessions-changed / agent-notification / session-file-line) を通常の bounded-queue 遅延を超えて飢餓・欠落させてはならない。
{% /req %}

{% req id="FR-007" %}
control-plane イベントのバーストが発生している間、システムは対話的 surface-output (打鍵エコー含む) を通常の bounded-queue 遅延を超えて飢餓・欠落させてはならない。
{% /req %}

{% req id="FR-008" %}
システムは、severance の判定ポリシー (閾値・帰責方法) を1箇所の再利用可能な機構として実装しなければならず、internalCh・ipcConn.outbox・DaemonClient 側 fan-out の各ホップで個別に再実装してはならない。
{% /req %}

{% req id="FR-009" %}
もし severance の帰責判定が単一の共有チャンネルの観測のみに基づくなら、システムは購読単位の backlog attribution によって、無関係な購読を誤って victim として sever するリスクを排除しなければならない。
{% /req %}

{% req id="FR-010" %}
システムは、既存のテスト契約 (gateway_terminal_test.go の per-session filter、gateway_view_update_test.go の hello/view-update 順序、terminal_relay_test.go の sever/re-subscribe/sequence-reset 契約) と同じ観測可能な振る舞いを、本設計の変更後も維持しなければならない。
{% /req %}

## Acceptance Criteria

{% acceptance id="AC-001" %}
Given あるセッションの対話的出力購読が輻輳し backlog 閾値を超えた、When daemon が backlog 超過を検知する、Then その購読のみが sever され明示的な wire 通知が browser へ届き、無関係セッションのイベントは順序どおり届き続ける。
{% /acceptance %}

{% acceptance id="AC-002" %}
Given severance 通知が browser に届いた、When TerminalSubscriptionController がそれを処理する、Then 新しい phase を追加せず既存の reconcile ループが再購読を試み、termvt の reattach snapshot が画面に反映される。
{% /acceptance %}

{% acceptance id="AC-003" %}
Given セッション A で対話的 surface-output バーストが発生している、When 同じ共有ホップにセッション B 宛の control-plane イベントが滞留している、Then セッション B のイベントは bounded-queue 遅延を超えて飢餓しない。
{% /acceptance %}

{% acceptance id="AC-004" %}
Given control-plane イベントのバーストが発生している、When 同じ共有ホップにアクティブセッションの対話的 surface-output が滞留している、Then 対話的 surface-output は bounded-queue 遅延を超えて飢餓しない。
{% /acceptance %}

{% acceptance id="AC-005" %}
Given 既存の gateway_terminal_test.go / gateway_view_update_test.go / terminal_relay_test.go のテスト群がある、When 本設計を適用する、Then 既存の全アサーションが変更前と同じ観測可能な振る舞いとして pass し続ける。
{% /acceptance %}

## Non-Goals

{% non_goals %}
must_not: 既存の accepted ADR (adr-20260705 / adr-20260624-0010 / adr-20260624-0066 / adr-20260711) が確立した契約を無自覚に変更すること。対症療法としてバッファサイズを単純に増やすだけの変更をすること。frontend TerminalSubscriptionPhase に新しいカテゴリを追加すること。
should_not: daemon↔gateway 間の物理輸送層を今回の変更で複数コネクション化すること (adr-20260711-keep-single-ipc-connection-topology で見送り、将来の north-star として記録する)。
{% /non_goals %}
