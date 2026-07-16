---
id: spec-20260716-codex-thread-event-subscription
kind: spec
title: Codex thread event subscription ownership
status: implemented
created: '2026-07-16'
tags:
- codex
- app-server
- driver
owners: []
functional_requirements:
- id: FR-001
  statement: The system shall publish Codex SessionReady for a bound frame exactly
    once and only after runtime activation, observer subscription, and canonical thread
    identity validation are all committed for that frame.
  priority: must
  rationale: Ready is the public commit point of one attach lifecycle transaction,
    not a synonym for identity discovery.
- id: FR-002
  statement: When a fresh Codex frame observes a cross-connection thread/started for
    its unique initState pending slot, the system shall use that notification only
    to discover identity and shall immediately resume that identity on the backend
    connection before validating the canonical thread ID.
  priority: must
  rationale: Fresh identity keeps the existing single pending-slot owner while lifecycle
    observation becomes an explicit backend subscription.
- id: FR-003
  statement: When a recovered Codex frame has a persisted thread ID, the system shall
    resume that ID on the backend connection independently of the TUI codex resume
    connection and shall require the returned canonical ID to match.
  priority: must
  rationale: The intentional double resume gives both the observer and interactive
    connections their own subscription.
- id: FR-004
  statement: When an authoritative thread/status/changed notification reports active
    or idle for a subscribed Codex thread, the system shall map it to the existing
    Running or Waiting/Idle driver status and expose it in the next WebSocket view-update
    for the same frame without changing any other frame.
  priority: must
  rationale: This preserves the user-observable status transition through daemon projection,
    gateway delivery, and Web rendering.
- id: FR-005
  statement: If identity discovery, observer resume, canonical identity validation,
    wrap, or spawn fails before ready commit, then the system shall not publish SessionReady
    and shall route the frame through the existing failed-release cleanup path.
  priority: must
  rationale: A partially attached frame must remain externally failed rather than
    silently freezing at Idle.
- id: FR-006
  statement: When a bound Codex frame is released, the system shall atomically tombstone
    it, stop local event routing, best-effort unsubscribe the backend connection,
    and continue resource cleanup exactly once.
  priority: must
  rationale: Local safety and cleanup cannot depend on the availability of the external
    app-server.
- id: FR-007
  statement: If thread/unsubscribe returns unsubscribed, notSubscribed, or notLoaded,
    then the system shall treat release as idempotently successful; if it times out
    or the connection closes, then the system shall record a diagnostic failure while
    completing local cleanup; repeated release shall be a no-op.
  priority: must
  rationale: The typed result partitions semantic success from external diagnostic
    failure.
- id: FR-008
  statement: When the fake app-server handles thread/start or thread/resume, the system
    shall register the requesting connection as subscribed, deliver thread/status
    notifications only to subscribed connections, deliver turn notifications only
    to the initiating connection, and limit bootstrap thread/started broadcast behavior
    to the real-server fidelity contract.
  priority: must
  rationale: The fake must expose the connection-scoped contract that the prior global
    broadcast concealed.
non_functional_requirements:
- id: NFR-001
  type: reliability
  criteria: Scripted fresh and recovery active-to-idle scenarios shall produce zero
    missing, duplicate, or foreign-frame Web status transitions after SessionReady.
  measurement: T0/T1/T2 and Go gateway scenario assertions count missing, duplicate,
    and foreign-frame transitions; each count must equal 0 under normal and race test
    runs.
- id: NFR-002
  type: maintainability
  criteria: Exactly one stream attach coordinator shall decide readiness from runtimeActivated,
    observerSubscribed, and canonicalIdentityValidated, and every Subsystem implementation
    shall explicitly implement ActivateFrame.
  measurement: Compile-time interface satisfaction plus runtime/stream contract tests;
    no optional type assertion or second SessionReady commit owner is permitted.
- id: NFR-003
  type: compatibility
  criteria: The design shall preserve the pinned Codex 0.144.4 initialize, thread/resume,
    thread/unsubscribe status matrix and shall perform zero writes to Codex rollout
    files.
  measurement: FakeVsReal fidelity tests cover resume/unsubscribe and a source scan
    reports zero production rollout-file creation or mutation introduced by this change.
- id: NFR-004
  type: reliability
  criteria: Fresh identity discovery shall retain the existing 60-second initState
    adoption deadline, and the cross-connection thread/started assumption shall be
    used for identity only.
  measurement: T3 real Codex fresh attach observes thread/started on the backend connection
    before the deadline, followed by backend ResumeThread and status subscription;
    absence skips locally unless AG_E2E_CODEX_BIN is configured and fails when configured.
acceptance:
- id: AC-001
  given: A fresh Codex frame has one initState pending slot and its TUI has spawned
    successfully.
  when: The backend observes the TUI-created thread/started and then receives active
    followed by idle on its own resumed subscription.
  then: SessionReady is emitted once after all three readiness predicates, and the
    same Web frame changes to Running and then Waiting/Idle on successive view-updates
    while every other frame is unchanged.
  requirement_refs:
  - FR-001
  - FR-002
  - FR-004
  - NFR-001
- id: AC-002
  given: A recovered frame has a persisted canonical thread ID and the TUI will execute
    codex resume on a separate connection.
  when: The backend first resumes the persisted ID and both connections attach.
  then: The backend validates the same canonical ID, commits one SessionReady only
    after runtime activation, and its connection receives active and idle status changes
    rendered by the Web UI.
  requirement_refs:
  - FR-001
  - FR-003
  - FR-004
- id: AC-003
  given: Backend observer subscription has succeeded but runtime wrap or TUI spawn
    fails before activation.
  when: The existing spawn failure path releases the frame.
  then: No SessionReady is emitted, routing is tombstoned before unsubscribe, and
    binding, worktree, and launch resources are cleaned even if unsubscribe times
    out.
  requirement_refs:
  - FR-005
  - FR-006
  - FR-007
- id: AC-004
  given: A released frame has a live, closed, or already-unsubscribed backend connection.
  when: ReleaseFrame is called once or repeatedly.
  then: unsubscribed, notSubscribed, and notLoaded converge to success; timeout or
    close produces one diagnostic; local cleanup completes once; and post-tombstone
    events never reach the frame.
  requirement_refs:
  - FR-006
  - FR-007
- id: AC-005
  given: Two fake app-server connections have introduced the same thread but only
    one has started or resumed it.
  when: A status notification and a turn notification are emitted.
  then: Status reaches only subscribed connections, turn events reach only their initiator,
    and an uninitiated observer receives neither until it explicitly resumes.
  requirement_refs:
  - FR-008
  - NFR-001
- id: AC-006
  given: A configured real Codex 0.144.4 app-server and separate backend and TUI connections.
  when: A fresh thread is created and a persisted thread is resumed.
  then: Fresh thread/started identity is observable cross-connection, backend ResumeThread
    establishes status delivery, typed unsubscribe matches the pinned status matrix,
    and no rollout file is written by agent-grid.
  requirement_refs:
  - FR-002
  - FR-003
  - FR-007
  - NFR-003
  - NFR-004
- id: AC-007
  given: The backend has discovered an identity or spawned a TUI but has not satisfied
    all three readiness predicates.
  when: Events arrive in any ordering or one predicate never arrives.
  then: The frame never appears ready merely from identity discovery or spawn completion,
    preventing the legacy permanently Idle Web status.
  requirement_refs:
  - FR-001
  - FR-005
  - NFR-002
failure_modes:
- class: identity-discovery-timeout
  detection: initState pending slot exceeds the existing 60-second adoption deadline
  recovery: escalate
  operator_action: Emit the existing subsystem failure diagnostic and release the
    frame.
  related_fr:
  - FR-005
- class: canonical-identity-mismatch
  detection: ResumeThread response canonical ID differs from the requested or discovered
    thread ID
  recovery: fail_fast
  related_fr:
  - FR-005
- class: observer-subscription-failure
  detection: Backend ResumeThread returns an RPC, timeout, or transport error
  recovery: escalate
  operator_action: Fail and release the frame without publishing SessionReady.
  related_fr:
  - FR-005
- class: observer-subscribed-before-spawn-failure
  detection: Wrap or spawn completion fails after backend subscription and before
    ActivateFrame
  recovery: degrade
  related_fr:
  - FR-005
  - FR-006
- class: unsubscribe-external-failure
  detection: thread/unsubscribe times out or the backend connection is closed
  recovery: degrade
  related_fr:
  - FR-006
  - FR-007
non_goals:
  must_not:
  - Add a targeted shim relay or a full JSON-RPC broker for thread identity or lifecycle
    events.
  - Generate, edit, or fabricate Codex rollout files.
  - Weaken the one-thread-to-one-frame routing isolation invariant.
  should_not:
  - Redesign the driver reducer, Web status mapping, or shim notification transparency.
  - Generalize the lifecycle protocol beyond the required ActivateFrame contract and
    Codex stream implementation.
relations:
- {type: implementedBy, target: plan-20260716-codex-thread-event-subscription}
- {type: referencedBy, target: adr-20260716-codex-observer-subscription-ready-ownership}
source_paths:
- src/client/runtime/subsystem/subsystem.go
- src/client/runtime/interpret_spawn.go
- src/client/runtime/subsystem/stream/
- src/platform/agent/codexclient/
- src/client/runtime/subsystem/stream/fake/
- src/server/web/mux_scenario_test.go
- src/client/web/e2e/app.smoke.spec.ts
methodology: sdd
summary: Codex の fresh/resume 双方で backend connection が thread event 購読を明示所有し、runtime
  activation と同一 transaction で ready を確定する。
updated: '2026-07-16'
---

# Codex thread event subscription ownership

## Overview

Codex app-server の thread event 購読は connection scoped である。したがって、TUI connection が thread を start/resume していても、backend connection が identity を知っているだけでは Web UI の driver status を更新できない。本仕様は identity discovery、backend observer subscription、runtime activation を別の事実として扱い、3 条件が揃う一点だけを `SessionReady` とする。

Fresh identity discovery は既存 `initState` pending slot と cross-connection `thread/started` observation に限定する。その通知は identity にだけ使い、直後に backend connection 自身が `ResumeThread` して canonical ID と lifecycle subscription を確定する。Recovery は persisted ID に同じ backend resume を行い、別 connection の TUI `codex resume` と意図的に二重 attach する。

## Requirements

{% req id="FR-001" %}
`SessionReady` は `runtimeActivated && observerSubscribed && canonicalIdentityValidated` が同一 frame で成立した一度だけの commit event である。
{% /req %}

{% req id="FR-002" %}
Fresh の `thread/started` broadcast は既存 pending slot の identity discovery にだけ使い、その後の backend `ResumeThread` が lifecycle 購読を所有する。
{% /req %}

{% req id="FR-003" %}
Recovery は persisted ID を backend connection が resume し、TUI connection の `codex resume` と独立に subscription を得る。
{% /req %}

{% req id="FR-004" %}
Subscribed thread の authoritative `active → idle` は、既存 mapping を経て同一 frame の次の `view-update` に `Running → Waiting/Idle` として現れる。
{% /req %}

{% req id="FR-005" %}
Identity、subscription、canonical validation、wrap、spawn の ready 前 failure は Ready を閉じ、既存 failed-release 経路へ収束する。
{% /req %}

{% req id="FR-006" %}
Release は local tombstone、routing stop、best-effort unsubscribe、resource cleanup の順で一度だけ進む。
{% /req %}

{% req id="FR-007" %}
Unsubscribe の意味論的な冪等成功と外部診断 failure を区別し、後者も local cleanup を妨げない。
{% /req %}

{% req id="FR-008" %}
Fake は per-connection introduced/subscribed registry を SSOT とし、status と turn の配信範囲を real contract に合わせる。
{% /req %}

NFR は frontmatter の測定欄を正本とする。とくに event 欠落・重複・foreign-frame delivery は全て 0 件、ready owner は 1 箇所、rollout file write は 0 件でなければならない。

## Acceptance Criteria

{% acceptance id="AC-001" %}
Fresh attach を公開 bootstrap から駆動し、WebSocket の同一 frame に `Running → Waiting/Idle` が現れることを確認する。

Counterexample: identity adoption だけで Ready になり、backend が status を購読しないため Web が永久に Idle のままでも成功扱いになる実装は不合格。
{% /acceptance %}

{% acceptance id="AC-002" %}
Persisted ID recovery で backend と TUI の別 connection が同じ canonical thread に attach し、Web status が active/idle に追随することを確認する。

Counterexample: TUI の `codex resume` だけを行い、backend は ID map への pre-register だけで済ませる実装は不合格。
{% /acceptance %}

{% acceptance id="AC-003" %}
Backend subscribe 後の wrap/spawn failure が Ready を出さず、unsubscribe failure にかかわらず全 local resource を回収することを確認する。

Counterexample: observer subscription を残したまま binding だけを削除する、または unsubscribe timeout で cleanup を止める実装は不合格。
{% /acceptance %}

{% acceptance id="AC-004" %}
Release の全 status、connection close、timeout、duplicate call と post-tombstone event を表形式 contract test で検証する。

Fresh identity notification の handler は locator と binding generation だけを記録して `Conn.Run` に復帰し、observer `ResumeThread` は別の generation-scoped operation として mutex 外で実行する。Release と競合した resume response は live generation にだけ commit でき、tombstone 済み generation の成功 response は Ready を出さず compensating unsubscribe へ進む。

Counterexample: unsubscribe 応答を待つ間に event を released frame へ routing する実装は不合格。
{% /acceptance %}

{% acceptance id="AC-005" %}
Fake の複数 connection で introduced と subscribed を区別し、status は subscribers、turn は initiator だけへ届くことを確認する。

Counterexample: connected client 全体への broadcast を維持して未購読 backend に status が届く実装は不合格。
{% /acceptance %}

{% acceptance id="AC-006" %}
T3 で cross-connection `thread/started` を identity-only external contract として pin し、その直後の backend resume、status delivery、unsubscribe matrix を real Codex 0.144.4 で確認する。

Counterexample: real 範囲を fake の global broadcast で代用する、または broadcast を lifecycle status delivery に使う実装は不合格。
{% /acceptance %}

{% acceptance id="AC-007" %}
Spawn と subscription の到着順を入れ替え、3 predicate の conjunction が一度だけ Ready を commit することを確認する。

Counterexample: `ActivateFrame` を optional interface にして stream だけを type assertion で呼ぶ、または identity/spawn の片方だけで Ready を出す実装は不合格。
{% /acceptance %}

## Failure Modes

{% failure_modes %}
失敗 class、検出、recovery の正本は frontmatter `failure_modes` とする。内部 canonical mismatch は fail fast、外部 RPC/transport failure は診断付き frame failure、unsubscribe failure は local cleanup を継続する degrade として仕分ける。
{% /failure_modes %}

## Non-Goals

{% non_goals %}
Targeted shim relay/full broker、rollout file 操作、routing isolation の弱化は行わない。Driver/Web mapping と shim transparency は既存契約を再利用する。
{% /non_goals %}


{% transition from="draft" to="approved" date="2026-07-16" %}
ユーザーが実装着手を承認したため。
{% /transition %}


{% transition from="approved" to="implemented" date="2026-07-16" %}
Codex observer subscription ownership を実装し受け入れ条件を検証。
{% /transition %}
