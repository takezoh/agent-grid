---
change: change-20260711-terminal-subscription-recovery
role: requirements
---

# Requirements

## Legacy Source (verbatim)

````markdown
---
id: spec-20260711-terminal-subscription-recovery
kind: spec
title: Web terminal subscription recovery
status: implemented
created: '2026-07-11'
tags: []
owners: []
functional_requirements:
- id: FR-001
  statement: システムはmount中のterminalが要求するsessionを、wire上の購読確認状態と独立したdesired subscriptionとして保持しなければならない
  priority: must
  rationale: 一時的な購読失敗でユーザーの表示意図を失わせないため
- id: FR-002
  statement: desired subscriptionが存在する間、システムは一時的なframe-not-readyに対するbounded retry
    burstとcooldownを繰り返さなければならない
  priority: must
- id: FR-003
  statement: frame-not-readyのretry burstが枯渇したとき、システムはwaiting状態を観測可能にし、session再選択なしで次のburstを開始しなければならない
  priority: must
- id: FR-004
  statement: 購読応答待ちまたはconfirmed中にWebSocketが切断されたとき、システムはdesired subscriptionを保持し、再接続後に同じsessionの購読を再開しなければならない
  priority: must
- id: FR-005
  statement: 同一sessionのterminal ownershipがhandoffされたとき、システムは有効なwire購読を解除または重複作成してはならない
  priority: must
- id: FR-006
  statement: 異なるsessionへ切り替わったとき、システムは単一のwire writerで旧sessionのunsubscribeを新sessionのsubscribeより先に送信しなければならない
  priority: must
- id: FR-007
  statement: もし旧ownershipの応答、timer、またはcleanupが完了した場合、システムはcurrent ownershipのdesired
    stateを変更してはならない
  priority: must
- id: FR-008
  statement: terminal outputを受信したとき、システムは現在のterminal sessionと一致しないoutputをxtermへ書き込んではならない
  priority: must
- id: FR-009
  statement: 恒久的な購読エラーを受信したとき、システムはdesired subscriptionを保持したblocked状態を観測可能にし、自動retryを停止しなければならない
  priority: must
- id: FR-010
  statement: blocked中にdesired sessionが変更または明示的に再取得されたとき、システムはblocked原因をクリアしてreconcileを再開しなければならない
  priority: must
- id: FR-011
  statement: WebSocketの再接続方針が停止したとき、システムはdesired subscriptionを保持したdisconnected状態に留まり、新しいopen通知までsubscribe
    flightを開始してはならない
  priority: must
- id: FR-012
  statement: 購読状態が遷移したとき、システムはsessionId、phase、attempt、lastErrorおよびownership epochをread-only観測状態へ投影しなければならない
  priority: should
non_functional_requirements:
- id: NFR-001
  type: reliability
  criteria: 各connectionでwire command writerとsubscribe flightはそれぞれ最大1つであり、同一session
    handoff中の不要なunsubscribeは0件である
  measurement: controllerの決定的状態遷移testとwire command logで検証する
- id: NFR-002
  type: maintainability
  criteria: retry、cooldown、connection lifecycleおよびdesired reconciliationの判断をsocket
    controllerだけが所有する
  measurement: ReactおよびZustand sliceにbackoff timerまたはwire送信判断がないことをreviewとtestで確認する
- id: NFR-003
  type: compatibility
  criteria: frame-not-ready wire contract、keyed terminal remountおよびsessionId output
    filterを変更しない
  measurement: 既存testと追加contract testが成功する
acceptance:
- id: AC-001
  given: 選択中sessionのframeがretry burst終了時点でも未生成である
  when: cooldown後にframeが利用可能になる
  then: sessionを切り替えなくても自動的に購読がconfirmedとなりterminal outputが表示される
  requirement_refs:
  - FR-001
  - FR-002
  - FR-003
- id: AC-002
  given: 選択中sessionのsubscribe応答がpendingまたはconfirmedである
  when: WebSocketが切断され、その後再接続する
  then: 同じdesired sessionが自動的に再購読されterminal outputが再開する
  requirement_refs:
  - FR-004
  - FR-011
- id: AC-003
  given: 同一sessionの旧TerminalPane cleanupと新TerminalPane acquireが重なる
  when: ownership handoffと遅延cleanupが処理される
  then: unsubscribeは送信されずwire購読は1つのまま維持される
  requirement_refs:
  - FR-005
  - FR-007
- id: AC-004
  given: session Aが購読中である
  when: session Bへ切り替え、session A由来の遅延処理とoutputが到着する
  then: wire logはAのunsubscribeからBのsubscribeの順となり、Aの遅延処理とoutputはBの状態とxtermを変更しない
  requirement_refs:
  - FR-006
  - FR-007
  - FR-008
- id: AC-005
  given: desired sessionへのsubscribeが恒久エラーで拒否される
  when: cooldown相当時間が経過する
  then: phaseはblockedとして観測でき追加subscribeは送信されず、明示的な再取得後にだけ再試行する
  requirement_refs:
  - FR-009
  - FR-010
  - FR-012
relations:
- {type: implementedBy, target: plan-20260711-terminal-subscription-recovery}
- {type: referencedBy, target: adr-20260711-terminal-subscription-desired-reconcile}
source_paths:
- src/client/web/src/socket/
- src/client/web/src/components/TerminalPane.tsx
- src/client/web/src/store/
summary: 選択中terminalの購読意図を一時障害と接続断を越えて保持し、再選択なしで自動回復させる
updated: '2026-07-11'
---

## Overview

Web terminal の表示意図を `desired subscription` として保持し、購読確認済み集合とは分離する。`frame-not-ready` と接続断からは自動回復し、恒久エラーは観測可能に停止する。

対象は単一 terminal pane の購読 lifecycle であり、scrollback 再送、daemon protocol 変更、複数 pane 同時表示、新しいエラー UI は含まない。

## Requirements

{% req id="FR-001" %}mount 中の terminal が要求する session は、wire の成否と独立した desired subscription として常に保持する。{% /req %}
{% req id="FR-002" %}desired がある間は、一時的な `frame-not-ready` に bounded retry burst と cooldown を適用する。{% /req %}
{% req id="FR-003" %}burst 枯渇は terminal の再選択を要求せず、waiting を経て再試行する。{% /req %}
{% req id="FR-004" %}pending/confirmed を問わず、接続断後は同じ desired session を再購読する。{% /req %}
{% req id="FR-005" %}同一 session の ownership handoff は wire 購読を維持する。{% /req %}
{% req id="FR-006" %}異なる session の切替では unsubscribe、subscribe の順序を一つの writer が保証する。{% /req %}
{% req id="FR-007" %}旧 ownership の非同期完了は current desired state を変更しない。{% /req %}
{% req id="FR-008" %}現在 session と異なる output は xterm に書き込まない。{% /req %}
{% req id="FR-009" %}恒久エラーは blocked として停止する。{% /req %}
{% req id="FR-010" %}blocked は desired の変更または明示的な再取得でのみ再開する。{% /req %}
{% req id="FR-011" %}再接続停止中は disconnected に留まり、open 通知まで wire flight を開始しない。{% /req %}
{% req id="FR-012" %}controller の状態を read-only store projection として観測可能にする。{% /req %}

## Acceptance Criteria

{% acceptance id="AC-001" %}retry burst 枯渇後も session 再選択なしで terminal が表示される。{% /acceptance %}
{% acceptance id="AC-002" %}pending/confirmed 中の切断後も自動再購読される。{% /acceptance %}
{% acceptance id="AC-003" %}同一 session handoff で旧 cleanup が新 ownership を解除しない。{% /acceptance %}
{% acceptance id="AC-004" %}異なる session の wire 順序と stale output 拒否を保証する。{% /acceptance %}
{% acceptance id="AC-005" %}恒久エラーは自動 retry せず、明示的な再取得でのみ再開する。{% /acceptance %}


{% transition from="draft" to="approved" date="2026-07-11" %}
構造修正方針を承認し実装・検証済み
{% /transition %}


{% transition from="approved" to="implemented" date="2026-07-11" %}
全acceptance scenarioをcontroller・connection・component testsで検証
{% /transition %}

````
