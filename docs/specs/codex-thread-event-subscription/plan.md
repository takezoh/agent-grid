---
id: plan-20260716-codex-thread-event-subscription
kind: plan
title: Implement Codex thread event subscription ownership
status: done
created: '2026-07-16'
goal: Make Codex frame readiness a single lifecycle transaction that commits only
  after runtime activation, backend observer subscription, and canonical identity
  validation.
scope_in:
- Required Subsystem ActivateFrame lifecycle contract and runtime spawn-complete call
  site
- Fresh identity-only discovery and fresh/recovery backend ResumeThread subscription
- Typed thread/unsubscribe and idempotent tombstone-first release
- Connection-scoped fake semantics and external dependency triple
- Gateway view-update and minimum Web status smoke coverage
scope_out:
- Targeted shim relay or full JSON-RPC broker
- Codex rollout file generation or mutation
- Driver reducer, Web status mapping, or shim notification transparency redesign
- Codex-unrelated lifecycle generalization beyond explicit non-stream no-op implementations
milestones:
- id: m1
  title: Pin typed Codex subscription and fake connection scope
  status: done
- id: m2
  title: Add required runtime activation lifecycle contract
  status: done
- id: m3
  title: Commit stream attach and release as one transaction
  status: done
- id: m4
  title: Preserve status observability through gateway and Web
  status: done
contracts:
- Subsystem.ActivateFrame(frameID) is required; every implementation is explicit and
  non-stream implementations are no-op.
- SessionReady commits once under the stream mutex iff runtimeActivated, observerSubscribed,
  and canonicalIdentityValidated are true.
- Fresh thread/started is identity-only; fresh and recovery both subscribe through
  backend ResumeThread.
- Observer ResumeThread runs outside Conn.Run callbacks and outside the backend mutex;
  completion commits only to the same live generation, and stale success is compensated
  by UnsubscribeThread.
- ReleaseFrame commits tombstone before routing stop and best-effort typed UnsubscribeThread.
- Fake thread/start and thread/resume subscribe the requester; status targets subscribers;
  turn events target the initiator.
- External dependency triple is fake plus invariant-naming contract plus FakeVsReal
  T3 backstop.
verification_profiles:
- profile: codexclient-protocol-contract
  milestone: m1
  tier: T2
  command: cd src && GOCACHE=/tmp/gocache-agent-grid go test ./platform/agent/codexclient
    ./platform/agent/codexschema/...
  criterion: Typed UnsubscribeThread uses the pinned method constant and accepts only
    unsubscribed, notSubscribed, and notLoaded as semantic success.
  milestone_dod: Protocol API and invariant-naming contract tests pass.
- profile: connection-scoped-fake
  milestone: m1
  tier: T1
  command: cd src && GOCACHE=/tmp/gocache-agent-grid go test ./client/runtime/subsystem/stream/fake
  criterion: Multi-connection tests report zero status delivery to non-subscribers
    and zero turn delivery to non-initiators.
  milestone_dod: Fake registry and delivery matrix tests pass.
- profile: activation-predicate-pure
  milestone: m2
  tier: T0
  command: cd src && GOCACHE=/tmp/gocache-agent-grid go test ./client/runtime/subsystem/stream
    -run 'Test.*Ready|Test.*Activate'
  criterion: All ordering and duplicate cases commit SessionReady exactly once only
    when all three predicates are true.
  milestone_dod: Predicate table and race-sensitive unit tests pass.
- profile: runtime-activation-wiring
  milestone: m2
  tier: T1
  command: cd src && GOCACHE=/tmp/gocache-agent-grid go test ./client/runtime -run
    'Test.*SpawnComplete|Test.*Subsystem'
  criterion: ActivateFrame is called after loop-owned maps and resource handles are
    committed; every non-stream fake and implementation compiles with an explicit
    no-op.
  milestone_dod: Runtime spawn/discard and subsystem dispatch tests pass.
- profile: stream-attach-release-wired
  milestone: m3
  tier: T1
  command: cd src && GOCACHE=/tmp/gocache-agent-grid go test ./client/runtime/subsystem/stream/...
  criterion: Fresh and recovery backend ResumeThread, canonical validation, spawn
    failure rollback, unsubscribe matrix, duplicate release, and routing isolation
    all pass with zero leaked binding.
  milestone_dod: Stream package tests and race-sensitive lifecycle tests pass.
- profile: gateway-resume-status-scenario
  milestone: m4
  tier: T1
  command: cd src && GOCACHE=/tmp/gocache-agent-grid go test ./server/web -run 'Test.*Codex.*Resume.*Status'
  criterion: Public session bootstrap and WebSocket view-update show Running then
    Waiting/Idle on the resumed frame and no foreign-frame status change.
  milestone_dod: Go gateway scenario reaches the user-observable view-update boundary.
- profile: web-status-render-smoke
  milestone: m4
  tier: T1
  command: cd src/client/web && npm run test:unit -- --run src/store/daemon.test.ts
    src/App.test.tsx && npm run test:e2e -- --grep 'resume status'
  criterion: Successive view-updates render the active session status as running then
    waiting using the minimum added Playwright smoke.
  milestone_dod: Store/render unit assertions and one browser smoke pass.
- profile: real-codex-fidelity
  milestone: m4
  tier: T3
  command: GOCACHE=/tmp/gocache-agent-grid make test-e2e
  criterion: With AG_E2E_CODEX_BIN configured, Codex 0.144.4 exposes fresh thread/started
    cross-connection for identity, backend resume receives status, unsubscribe matches
    the status matrix, and fake-versus-real invariants agree.
  milestone_dod: Configured real subtests pass; absent binaries skip as documented.
- profile: full-feature-gate
  milestone: m4
  tier: T2
  command: cd src && GOCACHE=/tmp/gocache-agent-grid go test ./... && cd client/web
    && npm run test:web
  criterion: All Go gateway scenarios, Go contracts, Web lint/unit/e2e, and existing
    routing isolation tests pass.
  milestone_dod: No regression remains in always-on project gates.
relations:
- {type: implements, target: spec-20260716-codex-thread-event-subscription}
- {type: hasPart, target: adr-20260716-codex-observer-subscription-ready-ownership}
source_paths:
- src/client/runtime/subsystem/subsystem.go
- src/client/runtime/interpret_spawn.go
- src/client/runtime/spawn_complete_test.go
- src/client/runtime/subsystem/cli/backend.go
- src/client/runtime/subsystem/stream/backend.go
- src/client/runtime/subsystem/stream/event.go
- src/client/runtime/subsystem/stream/fake/appserver.go
- src/platform/agent/codexclient/client.go
- src/platform/agent/codexschema/methods.go
- src/server/web/mux_scenario_test.go
- src/client/web/src/store/daemon.test.ts
- src/client/web/src/App.test.tsx
- src/client/web/e2e/app.smoke.spec.ts
summary: Required runtime activation、backend observer subscription、canonical identity
  validation を同一 ready transaction に統合し、fake/real/gateway/Web で検証する。
tags:
- codex
- app-server
- driver
owners: []
updated: '2026-07-16'
---

# Implement Codex thread event subscription ownership

## Goal

Codex frame の `SessionReady` を identity adoption の副作用から切り離し、runtime resource commit、backend connection の observer subscription、canonical identity validation が揃った一度だけの lifecycle commit にする。Fresh と recovery は identity 取得点だけを分け、その後は同じ backend `ResumeThread`、ready predicate、release transaction を使う。

要件の正本は `spec-20260716-codex-thread-event-subscription`、この transaction を選ぶ理由と代替案は `adr-20260716-codex-observer-subscription-ready-ownership` に置く。

## Implementation Sequence

### m1

{% milestone id="m1" %}
Codex protocol seam と fake fidelity を先に固定し、後続 lifecycle 実装が「接続しているだけで status が届く」誤契約へ逃げられないようにする。

Members: `component:codexclient-thread-subscription-api`, `component:connection-scoped-fake-app-server`, `req:FR-007`, `req:FR-008`。

Task-grade unit — **Add typed unsubscribe protocol contract**

- Objective: `codexschema.MethodThreadUnsubscribe` と `codexclient.UnsubscribeThread` を追加し、response status を型付きで分類する。
- Output format: production API、method constant、unit/contract tests。
- Tool guidance: pinned v2 generated typesと既存 `ResumeThread` API shapeを再利用し、wire persistence typeは stdlib/既存 schemaに留める。
- Task boundaries: stream binding/release と fake registry は変更しない。新規 dependency は追加しない。
- Files touched: `src/platform/agent/codexschema/methods.go`, `src/platform/agent/codexclient/client.go`, `src/platform/agent/codexclient/*_test.go`。
- Acceptance: request method/params/typed statuses を contract test が固定し、unknown status は成功に分類されない。
- Max diff LOC: 180。
- Depends on: none。

Task-grade unit — **Make fake subscriptions connection scoped**

- Objective: per-connection `introduced` / `subscribed` registry を SSOT にし、requester/initiator/subscriber の配信規則を実装する。
- Output format: fake implementation、multi-connection contract tests、FakeVsReal T3 backstop extension。
- Tool guidance: existing `serverConn` と `turnEmitter` seam を使い、`thread/start` / `thread/resume` requester を subscribe、`thread/status/*` を subscribers、`turn/*` を initiator に限定する。
- Task boundaries: bootstrap `thread/started` の real範囲は T3 で pin し、fake の汎用 global broadcast として lifecycle 配信へ拡張しない。
- Files touched: `src/client/runtime/subsystem/stream/fake/appserver.go`, `src/client/runtime/subsystem/stream/fake/appserver_test.go`, `src/client/runtime/subsystem/stream/routing_e2e_test.go`。
- Acceptance: 未購読 connection への status/turn delivery が各 0 件、start/resume requester が subscribed、real configured run と contract が一致する。
- Max diff LOC: 280。
- Depends on: Add typed unsubscribe protocol contract。
{% /milestone %}

### m2

{% milestone id="m2" %}
Runtime が frame resource の commit 完了を全 subsystem に明示通知する required lifecycle contract を導入する。Optional interface/type assertion は使用しない。

Members: `component:runtime-subsystem-activation-contract`, `component:stream-attach-coordinator`, `req:FR-001`, `req:FR-005`, `adr:adr-20260716-codex-observer-subscription-ready-ownership`。

Task-grade unit — **Require ActivateFrame on every subsystem**

- Objective: `Subsystem` interface に `ActivateFrame(frameID)` を required method として追加し、`handleSpawnComplete` が loop-owned maps、token/container registration、cleanup handle の commit 後かつ `EvFrameSpawned` dispatch 前に呼ぶ。
- Output format: interface、runtime wiring、CLI no-op、stream implementation entrypoint、全 test fake の明示 implementation、runtime tests。
- Tool guidance: `handleSpawnComplete` の existing single-writer discipline と `discardSpawnResult` の existing `ReleaseFrame` path を保持する。
- Task boundaries: activation methodに optional detection、I/O、Ready emission policyを持たせない。stream以外は明示 no-op のみ。
- Files touched: `src/client/runtime/subsystem/subsystem.go`, `src/client/runtime/interpret_spawn.go`, `src/client/runtime/spawn_complete_test.go`, `src/client/runtime/subsystem/cli/backend.go`, `src/client/runtime/**/*_test.go` の Subsystem fakes。
- Acceptance: maps/resource commit前には呼ばれず、dead target discardでは呼ばれず、全実装がcompile-timeにrequired interfaceを満たす。
- Max diff LOC: 260。
- Depends on: m1。
{% /milestone %}

### m3

{% milestone id="m3" %}
Stream backend の mutex 下に attach/release transaction owner を一本化する。Fresh は pending slot の `thread/started` で identity を得た直後、recovery は persisted ID から、どちらも backend connection が `ResumeThread` し canonical ID を検証する。`runtimeActivated && observerSubscribed && canonicalIdentityValidated` の conjunction を一度だけ commit して `SessionReady` を出す。

Members: `component:stream-attach-coordinator`, `component:codexclient-thread-subscription-api`, `req:FR-001`, `req:FR-002`, `req:FR-003`, `req:FR-005`, `req:FR-006`, `req:FR-007`, `adr:adr-20260716-codex-observer-subscription-ready-ownership`。

Task-grade unit — **Coordinate fresh and recovery readiness**

- Objective: frame binding に3 predicateとone-shot ready commitを持たせ、identity/spawn到着順に依存しない共通 transitionを実装する。
- Output format: stream coordinator state、fresh/recovery ResumeThread、canonical validation、T0 table/T1 wired tests。
- Tool guidance: fresh owner は既存 `initState.takeAny()`、external seam は既存 backend `codexclient.Conn` と `ResumeThread`、emission は既存 runtime hookを再利用する。`handleThreadStarted` は locator/generation を記録して return し、RPC worker が read loop と mutex の外で ResumeThread を実行する。completion は generation/tombstone を再検証し、stale success を compensating unsubscribe する。
- Task boundaries: shim、driver reducer、Web mapping、rollout fileは変更しない。targeted relay/full brokerは導入しない。
- Files touched: `src/client/runtime/subsystem/stream/backend.go`, `src/client/runtime/subsystem/stream/event.go`, `src/client/runtime/subsystem/stream/initsem.go`, `src/client/runtime/subsystem/stream/*_test.go`。
- Acceptance: fresh/recovery、activation-first/subscription-first、duplicate event、resume-in-flight release、stale responseの全表でReadyは0または1回。canonical mismatchは0回Ready + failure/release、stale successは0回commit + compensating unsubscribeとなり、Conn.Run handler deadlockはない。
- Max diff LOC: 300。
- Depends on: Require ActivateFrame on every subsystem。

Task-grade unit — **Make release tombstone-first and idempotent**

- Objective: backend subscription 後の wrap/spawn failureを含む全既存 `ReleaseFrame` entryから、tombstone→routing停止→best-effort unsubscribe→cleanupを一度だけ実行する。
- Output format: release implementation、typed status/error classification、race/contract tests。
- Tool guidance: existing `ReleaseFrame` call sitesとworktree cleanupを再利用し、unsubscribe I/Oはmutex外、local tombstoneはmutex下で先にcommitする。
- Task boundaries: timeout/closedでlocal cleanupをretry待ちにしない。duplicate releaseから二重unsubscribe/cleanupを出さない。
- Files touched: `src/client/runtime/subsystem/stream/backend.go`, `src/client/runtime/subsystem/stream/backend_test.go`, `src/client/runtime/subsystem/stream/init_serialize_test.go`, `src/client/runtime/spawn_complete_test.go`, `src/client/runtime/bootstrap_coldstart_test.go`。
- Acceptance: success/notSubscribed/notLoadedは同じ成功、timeout/closedは診断1件 + cleanup完了、post-tombstone routing 0件、duplicate releaseはno-op。
- Max diff LOC: 260。
- Depends on: Coordinate fresh and recovery readiness。
{% /milestone %}

### m4

{% milestone id="m4" %}
Subsystem の内部 event だけでなく、daemon projection、gateway WebSocket、Web store/render まで status observability を検証する。Web production wiring は既存を再利用し、追加は判別に必要な最小 smoke に限る。

Members: `component:codex-lifecycle-fidelity-suite`, `req:FR-004`, `req:FR-008`。

Task-grade unit — **Pin resume status at gateway view-update**

- Objective: public session bootstrap/recoveryから subscribed Codex status `active → idle` を駆動し、同一 frame の `view-update` が `running → waiting/idle`、foreign frame unchanged を示す Go gateway scenarioを追加する。
- Output format: always-on Go gateway scenario testと必要な deterministic fake fixture。
- Tool guidance: `src/server/web/mux_scenario_test.go` の existing real server + fake-agent harness、公開 HTTP/WS 操作だけを使う。
- Task boundaries: reducer内部stateの直接mutationやprivate handler直呼びで受け入れ条件を代用しない。
- Files touched: `src/server/web/mux_scenario_test.go`, 必要なら同 package の test support。
- Acceptance: missing/duplicate/foreign status各0件で、freshとresumeの少なくともresume経路がview-update boundaryまで到達する。
- Max diff LOC: 220。
- Depends on: m3。

Task-grade unit — **Render successive status updates in one Web smoke**

- Objective: successive `view-update` framesがactive sessionのstatus表示をrunningからwaitingへ更新することをstore/render unitと最小Playwright smokeで固定する。
- Output format: existing Web testsの拡張と1 browser smoke。production UI変更は観測が失敗した場合のみ同unit内で最小修正する。
- Tool guidance: existing fake backendと`App`/daemon store fixturesを再利用する。
- Task boundaries:新しいvisual design、status vocabulary、component abstractionは導入しない。
- Files touched: `src/client/web/src/store/daemon.test.ts`, `src/client/web/src/App.test.tsx`, `src/client/web/e2e/app.smoke.spec.ts`, `src/client/web/e2e/support/fake-backend.ts`。
- Acceptance: DOMのsession statusがrunning→waitingへ変化し、別session表示は変化しない。1 smoke以外にbrowser caseを増やさない。
- Max diff LOC: 180。
- Depends on: Pin resume status at gateway view-update。

Task-grade unit — **Extend real Codex identity-only fidelity backstop**

- Objective: fresh cross-connection `thread/started`をidentity-only external contractとしてpinし、その後のbackend resume/status/unsubscribeをCodex 0.144.4で検証する。
- Output format: env-gated `FakeVsReal*` T3 subtestとfake invariant naming contractへのtrace。
- Tool guidance: existing `routing_e2e_test.go` / `make test-e2e` setupを拡張し、configured binary absent時のskip postureを保持する。
- Task boundaries: real-only assertionをalways-on CI gateにせず、bootstrap broadcastをstatus delivery contractへ拡張しない。
- Files touched: `src/client/runtime/subsystem/stream/routing_e2e_test.go`, `src/client/runtime/subsystem/stream/fake/appserver_test.go`。
- Acceptance: configured real runでidentity discovery→backend resume→active/idle→unsubscribe matrixがpassし、fake drift時はFakeVsRealがfailする。
- Max diff LOC: 220。
- Depends on: m3。
{% /milestone %}

## Targets

- Runtime lifecycle owner: `src/client/runtime/subsystem/subsystem.go` の required `ActivateFrame(frameID)` と `src/client/runtime/interpret_spawn.go` の `handleSpawnComplete`。Maps/token/container/cleanup commit 後に呼び、全 subsystem・test fake が明示実装する。CLI は no-op、stream は coordinator predicateを更新する。
- Stream transaction owner: `src/client/runtime/subsystem/stream/backend.go`, `event.go`, `initsem.go`。`frameBinding`/専用 coordinator が identity、subscription、activation、ready、tombstone を mutex 下の SSOT として持つ。
- Async RPC seam: notification handler は state capture のみで `Conn.Run` に復帰し、generation-scoped worker が mutex 外で ResumeThread/UnsubscribeThread を行う。Response commit は live generation に限定し、release と競合した success は補償解除する。
- Codex external API seam: existing `*codexclient.Conn` + `ResumeThread` を注入点として再利用し、`src/platform/agent/codexclient/client.go` に typed `UnsubscribeThread`、`codexschema/methods.go` に method constantを追加する。新規third-party dependencyなし。
- Fake seam: `fake.AppServer` の各 `serverConn` が per-connection introduced/subscribed registryを持ち、existing `TurnHandler`/`Emitter` seamで配送先を検査する。Fake + invariant-naming contract + `FakeVsReal*` が external dependency triple。
- Fresh identity seam: existing `initState` pending slot + backend connection が観測する `thread/started` のみ。Shim relay/brokerは作らず、real broadcast assumptionは T3 identity-only contractに閉じる。
- Recovery seam: persisted `ResumeTarget` を backend `ResumeThread` へ渡す。TUI `codex resume` は別 connection の意図的double resumeであり、同じ canonical IDだけを共有する。
- Release seam: existing runtime/bootstrap/spawn failureの `ReleaseFrame` call sitesを再利用する。Tombstone後のunsubscribe I/Oはconnection seamを通し、timeout/closeでもlocal cleanup継続。
- User-observable seams: `src/server/web/mux_scenario_test.go` の HTTP/WS gateway harness、Web daemon store fixture、Playwright fake backend。Production Web component seamは増やさない。
- Pure core: 3 predicate ready判定、unsubscribe result classification、fake delivery target selectionはI/Oから分離した値判定としてT0検証可能にする。

構造規則は、platform が client をimportしない既存 depguard、required interfaceのcompile-time satisfaction、routing isolation contract、one-shot coordinator testsで機械検証する。Shim notification transparency と ADR-0001 routing isolation は規範ではなく既存contract testを維持する。

## Verification

| Profile | Tier | Command | Criterion / milestone DoD |
|---|---|---|---|
| codexclient-protocol-contract | T2 contract | `cd src && GOCACHE=/tmp/gocache-agent-grid go test ./platform/agent/codexclient ./platform/agent/codexschema/...` | Typed unsubscribe wire/status contractがpassし、unknown statusを成功扱いしない。m1 DoD。 |
| connection-scoped-fake | T1 wired | `cd src && GOCACHE=/tmp/gocache-agent-grid go test ./client/runtime/subsystem/stream/fake` | subscriber/initiator matrixの欠落・foreign delivery各0件。m1 DoD。 |
| activation-predicate-pure | T0 pure | `cd src && GOCACHE=/tmp/gocache-agent-grid go test ./client/runtime/subsystem/stream -run 'Test.*Ready|Test.*Activate'` | 3 predicateの全順序でReady exactly once。m2 DoD。 |
| runtime-activation-wiring | T1 wired | `cd src && GOCACHE=/tmp/gocache-agent-grid go test ./client/runtime -run 'Test.*SpawnComplete|Test.*Subsystem'` | Resource commit後だけActivateFrame、discardでは0回、全実装explicit。m2 DoD。 |
| stream-attach-release-wired | T1 wired | `cd src && GOCACHE=/tmp/gocache-agent-grid go test ./client/runtime/subsystem/stream/...` | Fresh/recovery、rollback、unsubscribe matrix、routing isolationがpass。m3 DoD。 |
| gateway-resume-status-scenario | T1 wired | `cd src && GOCACHE=/tmp/gocache-agent-grid go test ./server/web -run 'Test.*Codex.*Resume.*Status'` | Public bootstrap→WSで対象frameがrunning→waiting/idle、foreign frame不変。m4 DoD。 |
| web-status-render-smoke | T1 wired | `cd src/client/web && npm run test:unit -- --run src/store/daemon.test.ts src/App.test.tsx && npm run test:e2e -- --grep 'resume status'` | Store/DOMと最小browser smokeがsuccessive statusを描画。m4 DoD。 |
| real-codex-fidelity | T3 fidelity | `GOCACHE=/tmp/gocache-agent-grid make test-e2e` | Configured Codex 0.144.4でidentity-only broadcast、backend resume/status/unsubscribeがfake contractと一致。m4 DoD。 |
| full-feature-gate | T2 contract | `cd src && GOCACHE=/tmp/gocache-agent-grid go test ./... && cd client/web && npm run test:web` | Always-on Go gateway、race-sensitive contracts、Web gateに回帰なし。最終DoD。 |

構造 fitness functions:

- Layer direction → existing depguardを `make lint` で検証する。
- Required lifecycle contract → 全 `Subsystem` implementation/test fakeのcompileで検証し、optional interface assertionの導入をreviewで禁止する。
- Ready owner一意性 → stream coordinatorのpredicate tableとSessionReady emission contract testで検証する。
- Routing isolation → existing direct/wired/fuzz contractを `go test ./client/runtime/subsystem/stream/...` で維持する。
- Shim transparency → shim production変更なしをscope reviewし、既存shim testsをfull gateで維持する。
- Rollout非改変 → production source scanとT3 before/after observationで新規write 0件を確認する。


{% transition from="draft" to="active" date="2026-07-16" %}
実装開始。
{% /transition %}


{% transition from="active" to="done" date="2026-07-16" %}
実装と always-on/race/gateway/Web 検証が完了。
{% /transition %}
