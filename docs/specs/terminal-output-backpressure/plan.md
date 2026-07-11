---
id: plan-20260711-terminal-output-backpressure
kind: plan
title: Fix terminal surface-output starvation on shared IPC outbox
status: draft
created: '2026-07-11'
goal: 既存の sever-not-drop 契約 (adr-20260705) を daemon 側の全共有 IPC ホップへ一貫適用し、対話的 surface-output
  と bulk telemetry を優先度分離することで、共有バッファ拡大のような対症療法ではなく構造的に打鍵エコー消失バグを解消する。
scope_in:
- src/client/runtime/ipc.go (internalCh, ipcConn.outbox の drop 経路置換)
- src/client/runtime/terminal_relay.go (per-subscription backlog attribution, severance
  threshold seam)
- src/client/runtime/proto_bridge.go / proto_bridge_surface.go (priority lane 分離,
  severance 通知配信)
- src/server/web/daemon_client.go (per-tab sever-not-drop)
- src/server/web/gateway.go (severance シグナルの配線点)
- src/client/web/src/socket/terminalSubscription.ts / connection.ts (severance シグナルの最小解釈、既存
  reconcile 再利用)
scope_out:
- platform/termvt の VT emulator ロジック・fanout() 実装そのもの
- src/client/web/src/socket/terminalSubscription.ts の TerminalSubscriptionController
  アーキテクチャ全体の再設計
- Codex CLI 自体の /resume 実装
- daemon↔gateway 間の物理輸送層の複数コネクション化 (adr-20260711-keep-single-ipc-connection-topology)
milestones:
- id: m1
  title: per-subscription backlog attribution + severance threshold seam
  status: todo
- id: m2
  title: internalCh / ipcConn.outbox の priority lane 分離
  status: todo
- id: m3
  title: DaemonClient per-tab sever-not-drop
  status: todo
- id: m4
  title: severance の browser 観測可能シグナル配信 (ReqID defect fix)
  status: todo
- id: m5
  title: 'frontend: 既存 reconcile への最小配線'
  status: todo
- id: m6
  title: end-to-end 統合テスト + 既存回帰保証
  status: todo
contracts:
- gateway_terminal_test.go の per-session output filter
- gateway_view_update_test.go の hello/view-update 順序
- terminal_relay_test.go の sever/re-subscribe/sequence-reset 契約
- adr-20260705 の relay severance contract (real-pty T2)
tags:
- runtime
- ipc
- terminal
- reliability
owners:
- take.gn@gmail.com
relations:
- {type: implements, target: spec-20260711-terminal-output-backpressure}
- {type: hasPart, target: adr-20260711-extend-sever-not-drop-shared-ipc-hops}
- {type: hasPart, target: adr-20260711-priority-lane-interactive-vs-bulk}
- {type: hasPart, target: adr-20260711-server-initiated-severance-signal}
- {type: hasPart, target: adr-20260711-keep-single-ipc-connection-topology}
source_paths:
- src/client/runtime/ipc.go
- src/client/runtime/terminal_relay.go
- src/client/runtime/proto_bridge_surface.go
- src/server/web/daemon_client.go
- src/server/web/gateway.go
summary: adr-20260705 の sever-not-drop 契約を daemon 側の残りホップへ一貫適用し、対話的/bulk レーン分離で相互飢餓を防ぐ構造的修正計画。
---

## Goal

agent-grid の TERMINAL 打鍵エコー消失バグを、共有 IPC バッファの単純拡大ではなく、既存の accepted 契約 (`adr-20260705-eventsink-seam-tap-relay-contracts` の sever-not-drop) をまだこの原則が及んでいない daemon 側の残りホップ (`internalCh`, `ipcConn.outbox`, gateway 側 `DaemonClient` の per-tab fan-out) へ一貫適用することで解消する。あわせて、対話的 surface-output と bulk telemetry (`sessions-changed` / `agent-notification` / `session-file-line`) を優先度分離し、一方のバーストが他方を飢餓させない構造にする。詳細な要件根拠は spec.md、各設計判断の Why は個別 ADR を参照 — ここでは述べない。

## Implementation Sequence

{% milestone id="m1" %}
**per-subscription backlog attribution + severance threshold seam** (`adr-20260711-extend-sever-not-drop-shared-ipc-hops`)

TerminalRelay の fanOut 経路に (ConnID,SessionID,SubscriberID) 単位の backlog カウンタを追加し、既存の `internalSurfaceClosed → EvCmdSurfaceUnsubscribe` 合成経路を再利用して閾値超過購読のみを sever する。閾値は `WithTerminalRelaySubscriberBuffer` と同型のコンストラクタ注入 seam (`WithSeveranceThreshold` 相当) として提供する。他の全ホップの severance 判定はこの機構を呼び出し、個別に再実装しない (FR-008)。

Unit:
- title: "Add per-subscription backlog attribution + constructor-injected severance threshold to TerminalRelay"
- objective: TerminalRelay の fanOut 経路に (ConnID,SessionID,SubscriberID) 単位の backlog カウンタを追加し、閾値超過時にその購読だけを sever する (既存の internalSurfaceClosed→EvCmdSurfaceUnsubscribe 合成経路を再利用)。閾値は adr-20260705 の WithTerminalRelaySubscriberBuffer と同型のコンストラクタ注入 seam として提供する。
- output_format: src/client/runtime/terminal_relay.go の変更 (新 field/method 追加、既存 public API 互換維持) + 同ファイル横の *_test.go への contract test 追加 (容量極小値注入で決定的に severance を駆動、-race 下)。
- tool_guidance: adr-20260705-eventsink-seam-tap-relay-contracts の relay severance contract (subscriber channel 容量のコンストラクタ注入) と同じ流儀を参照。既存の WithTerminalRelaySubscriberBuffer オプション関数パターンを踏襲する。
- task_boundaries: internalCh / ipcConn.outbox 側の変更はこの unit に含めない (m2/m3 の責務)。severance 通知の wire 配信 (m4) もこの unit に含めない — ここでは既存 internalSurfaceClosed 経路をそのまま呼ぶところまで。
- files_touched: [src/client/runtime/terminal_relay.go, src/client/runtime/terminal_relay_test.go]
- acceptance: [容量を極小値に注入したテストで輻輳した購読のみが sever され他購読・他セッションの配送順序が保たれることを -race 下で検証する, 既存の terminal_relay_test.go の全既存テストが変更後も pass する]
- max_diff_loc: 300
- depends_on: []
{% /milestone %}

{% milestone id="m2" %}
**internalCh / ipcConn.outbox の priority lane 分離** (`adr-20260711-priority-lane-interactive-vs-bulk`)

共有の `internalCh` と `ipcConn.outbox` を、surface-output 用の interactive レーンと `sessions-changed`/`agent-notification`/`session-file-line` 用の bulk レーンに分離し、priority-select drain (interactive を優先、bulk は空き容量のみ) で相互飢餓を防ぐ (FR-006/FR-007)。severance 判定は m1 の機構を呼び出す。

Unit:
- title: "Split internalCh and ipcConn.outbox into interactive/bulk priority lanes"
- objective: 共有の internalCh と ipcConn.outbox を interactive レーンと bulk レーンに分離し、priority-select drain で相互飢餓を防ぐ。
- output_format: src/client/runtime/ipc.go, src/client/runtime/proto_bridge.go, src/client/runtime/proto_bridge_surface.go の変更 (2レーン化 + priority drain loop) + 対応する unit/contract test。
- tool_guidance: 既存の select/default drop 分岐 (queueWire, queueWireToConn, broadcastWire, enqueueInternal 相当) を grep で全箇所洗い出してから着手する。m1 の severance 機構を呼び出し、再実装しない (FR-008)。
- task_boundaries: DaemonClient (gateway 側) の変更は含めない (m3 の責務)。frontend 側の変更は含めない。
- files_touched: [src/client/runtime/ipc.go, src/client/runtime/proto_bridge.go, src/client/runtime/proto_bridge_surface.go]
- acceptance: [対話的 surface-output バースト時に無関係セッションの bulk イベントが飢餓しないことを確認する (AC-003), bulk イベントバースト時に対話的 surface-output が飢餓しないことを確認する (AC-004), severance が m1 の機構経由でのみ発生することを確認する (FR-008)]
- max_diff_loc: 300
- depends_on: [m1]
{% /milestone %}

{% milestone id="m3" %}
**DaemonClient per-tab sever-not-drop** (`adr-20260711-extend-sever-not-drop-shared-ipc-hops`)

gateway 側 `DaemonClient.broadcastEvent` の drop を、既に per-tab isolated なチャンネル特性を活かし、該当タブのチャンネルのみを close する sever に置換する。

Unit:
- title: "Replace silent drop with per-tab sever in DaemonClient.broadcastEvent"
- objective: DaemonClient.broadcastEvent の select/default drop を、該当タブのチャンネルのみを close する sever に置換する (他タブは無影響)。
- output_format: src/server/web/daemon_client.go の変更 + 既存 markDown/closeAllSubs パターンを個別チャンネル単位へ縮小適用する新メソッド + contract test。
- tool_guidance: 既存の markDown (全チャンネル close) の実装を参考に、単一チャンネルのみを対象にした severOne 相当のメソッドを追加する。
- task_boundaries: internalCh / ipcConn.outbox 側 (daemon 側) の変更は含めない (m1/m2 の責務)。severance の browser 向け通知配信は含めない (m4)。
- files_touched: [src/server/web/daemon_client.go]
- acceptance: [1タブの購読が輻輳しても他タブの SubscribeEvents チャンネルは無影響であることをテストで確認する (FR-005), 既存の daemon_client 関連テストが全て pass する]
- max_diff_loc: 250
- depends_on: [m1]
{% /milestone %}

{% milestone id="m4" %}
**severance の browser 観測可能シグナル配信 (ReqID defect fix)** (`adr-20260711-server-initiated-severance-signal`)

`internalSurfaceClosed → EvCmdSurfaceUnsubscribe{ReqID:""}` 経路の `okResp` が `dispatchResponse` で黙って破棄される欠陥を修正し、daemon 起点の severance (backpressure 由来含む) が browser に確実に届く、輻輳ホップを経由しない優先配信経路を追加する。新規 `proto.ServerEvent` 型は追加せず、既存 wire 経路の拡張に留める。

Unit:
- title: "Fix ReqID=\"\" silent-discard defect and deliver severance as a priority-bypass browser-observable signal"
- objective: internalSurfaceClosed→EvCmdSurfaceUnsubscribe{ReqID:""} 経路の okResp 黙殺欠陥を修正し、severance を輻輳ホップ非経由の優先配信経路で browser へ届ける。新規イベント型は追加しない。
- output_format: src/client/runtime/proto_bridge_surface.go / interpret.go の変更 + client/proto の該当型への最小フィールド追加 (新規イベント型は追加しない) + contract test。
- tool_guidance: 既存の tr.sendNow (internalSurfaceClosed の送出に使われるブロッキング送出) と同じ優先配信パターンを再利用する。
- task_boundaries: frontend 側の解釈ロジック変更は含めない (m5 の責務)。
- files_touched: [src/client/runtime/proto_bridge_surface.go, src/client/runtime/interpret.go, src/client/proto]
- acceptance: [severance 発生時に browser 側 WS で明示的な wire frame が観測できることを確認する (FR-003), この通知が輻輳中の共有レーンをバイパスして到達することをレーン飽和テストで確認する]
- max_diff_loc: 250
- depends_on: [m1, m2, m3]
{% /milestone %}

{% milestone id="m5" %}
**frontend: 既存 reconcile への最小配線** (`adr-20260711-server-initiated-severance-signal`)

TerminalSubscriptionController が m4 の severance signal を受信したとき、新しい `TerminalSubscriptionPhase` を追加せず、既存の reconcile ループが再購読を試みるよう wireSessionId を該当セッションについてのみ無効化する最小変更を行う。`adr-20260711-terminal-subscription-desired-reconcile` の状態機械は変更しない。

Unit:
- title: "Frontend: interpret severance signal as an existing-state resubscribe trigger (no new phase)"
- objective: severance signal 受信時、新しい phase を追加せず既存の reconcile ループが再購読を試みるよう最小変更する。
- output_format: src/client/web/src/socket/terminalSubscription.ts / connection.ts への最小変更 + 既存 vitest スイートへのテスト追加。
- tool_guidance: adr-20260711-terminal-subscription-desired-reconcile の既存状態機械 (idle/subscribing/confirmed/waiting/blocked/disconnected) を変更しない。
- task_boundaries: adr-20260711 が確立した設計思想そのもの (single wire writer、desired state の所有権) の再設計はしない。
- files_touched: [src/client/web/src/socket/terminalSubscription.ts, src/client/web/src/socket/connection.ts]
- acceptance: [severance signal 受信後、既存 phase 集合のみで自動的に再購読が試行され termvt の reattach snapshot が反映されることを vitest で確認する (AC-002), TerminalSubscriptionPhase の型に新しい値が追加されていないことを型チェックで確認する]
- max_diff_loc: 200
- depends_on: [m4]
{% /milestone %}

{% milestone id="m6" %}
**end-to-end 統合テスト + 既存回帰保証**

m1〜m5 を組み合わせた end-to-end シナリオ (burst → severance → browser 通知 → resubscribe → resync) を1つの統合テストとして pin し、既存の `gateway_terminal_test.go` / `gateway_view_update_test.go` / `terminal_relay_test.go` の全既存アサーションが変更後も pass することを確認する。

Unit:
- title: "End-to-end backpressure/severance integration test + existing regression guard"
- objective: burst→severance→通知→resubscribe→resync の一連を1統合テストとして pin し、既存3ファイルの全既存アサーションの regression が無いことを確認する。
- output_format: 新規 integration test ファイル (T2 相当、real pty または既存 fake の組み合わせ) + 既存テストファイルの回帰確認結果。
- tool_guidance: adr-20260705 の real-pty T2 contract test の流儀 (fanout_contract_test.go 等) を参考にする。
- task_boundaries: 新規の実装変更はここでは行わない (m1〜m5 で完了済みの振る舞いを検証するのみ)。
- files_touched: [src/server/web/gateway_terminal_test.go, src/server/web/gateway_view_update_test.go, src/client/runtime/terminal_relay_test.go, "(新規 integration test file)"]
- acceptance: [burst→severance→通知→resubscribe→resync の一連が1テストで pin されている (FR-010, NFR-002), 既存3ファイルの既存テストが1件も regression していない]
- max_diff_loc: 300
- depends_on: [m1, m2, m3, m4, m5]
{% /milestone %}

## Targets

- **`src/client/runtime/terminal_relay.go`**: severance 閾値のコンストラクタ注入 seam (`WithSeveranceThreshold` 相当、既存 `WithTerminalRelaySubscriberBuffer` と同型)。時刻は既存 `startTS` 注入を再利用 (新規 seam 不要)。
- **`src/client/runtime/pty_backend.go` / `SurfaceBackend` interface**: 既存の fake (`terminal_relay_test.go` 内) をそのまま再利用 — 新規 seam 不要。
- **`src/server/web/daemon_client.go`**: 既存 `dialFunc` (`NewDaemonClientWithDialer`) を再利用し、テストで `net.Pipe` を注入する。
- **`src/server/web/gateway_view_update_test.go` / `gateway_terminal_test.go`**: 既存 `httptest` + `fakeLifecycleAttacher` / `fakeSessionAttacher` パターンを再利用。
- **`src/client/web/src/socket/terminalSubscription.ts`**: 既存 `TerminalSubscriptionTransport` interface (`subscribe`/`unsubscribe` を差し替え可能) をそのまま再利用 — 新規 seam 不要。

## Verification

| profile | Tier | 実行コマンド | 判定基準 |
|---|---|---|---|
| m1-severance | T1 | `cd src && go test ./client/runtime/... -run TestTerminalRelay_Severance -race` | 容量注入で決定的に severance が発火し他購読が無傷であること |
| m2-priority-lane | T1 | `cd src && go test ./client/runtime/... -run TestPriorityLane -race` | 対話的/bulk 双方向で相互飢餓ゼロであること (AC-003/AC-004) |
| m3-pertab-sever | T1 | `cd src && go test ./server/web/... -run TestDaemonClient_PerTabSever -race` | 1タブの sever が他タブに無影響であること (FR-005) |
| m4-severance-signal | T1 | `cd src && go test ./client/runtime/... ./server/web/... -run TestSeveranceSignal -race` | severance が輻輳ホップを経由せず browser まで届くこと (FR-003) |
| m5-frontend-reconcile | T0 | `cd src/client/web && npx vitest run src/socket/terminalSubscription.test.ts` | 新規 phase を追加せず既存 reconcile が再購読すること、型に新カテゴリが増えていないこと (AC-002) |
| m6-e2e | T2 | `cd src && go test ./server/web/... ./client/runtime/... -race` | burst→severance→通知→resubscribe→resync の一連 + 既存3ファイルの全既存テストが regression なく pass すること (AC-005) |

**構造規則 → 検証手段**:
- 「severance 判定は1箇所の機構に集約する」(FR-008) → 機械検証: grep で `select {` + `default:` を含む drop 分岐が `terminal_relay.go` の severance helper 呼び出し以外に残っていないことを確認する (lint 化は future work、initial は grep による規範チェック)
- 「frontend 状態機械に新カテゴリを追加しない」(FR-004) → 機械検証: `TerminalSubscriptionPhase` 型の union メンバー数が変更前後で変わらないことを型テストで確認する
- 「既存 accepted ADR を無自覚に変更しない」→ 規範 (レビュー観点、機械検証なし): 統合役 / レビュアーが adr-20260705 / adr-20260624-0010 / adr-20260624-0066 / adr-20260711 の Decision 節との整合を目視確認する
