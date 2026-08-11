---
id: change-20260811-fleet-session-id-ssot
kind: change
title: Fleet session id ownership and Workspace IPC response envelope
status: done
created: '2026-08-11'
profile: sdd@1
intent: fleet session key と upstream (bare) session id の変換に境界ごとの単一所有者を与え、Electron
  IPC の response envelope に null-body-status 不変条件を表現させる。debug run (debug-run-workspace-electron-terminal-dead)
  が確定した独立2根本原因を、局所パッチ3度目を招かない形で閉じる。
outcomes:
- SPA が組み立てる全 WS frame の sessionId が wire 境界の単一所有者を通り、compound fleet key が gateway
  へ到達しない。
- store / HTTP 境界の session id 解決が単一 resolver と権威を required で持つ store 行型を唯一の権威とし、 8
  種類に分岐した失敗方針が 1 つに収束する。
- Electron IPC boundary を通る null body status (101/204/205/304) が renderer で Response
  として 再物質化でき、成功した操作が network error として誤報されない。
- 交差点の追加が無音で壊れないことを機械的に検出する手段が残る。
scope:
- clients/ui の wire 境界 (socket/connection.ts, socket/retry.ts, components/TerminalPane.tsx,
  components/DriverShortcutBar.tsx, components/TerminalMobileOverlay.tsx) の session
  id 変換所有権と frame 組み立て・直列化の権威。retry.ts は send 経路の内部 bypass を閉じるため含む
- clients/ui の store / HTTP 境界 (store/daemon.ts, store/statusSignals.ts, store/workspaceActivity.ts,
  api/sessions.ts, api/transcripts.ts, api/workspace.ts, api/capsules.ts, App.tsx)
  の再導出集約
- fleet/session-protocol.ts からの identity 所有者モジュール (fleet/session-identity.ts) の切り出しと
  SessionRoute resolver 新設
- wire/server.ts の SessionInfo と store 行型 FleetSessionRow の分離 (未設定時方針の確定を含む)
- clients/workspace の main IPC envelope (shared/http-envelope.ts, main/electron-entry.ts,
  preload/index.ts, preload/electron-preload.cjs) と clients/ui の消費側 (auth.ts, fleet/provider.ts)
  の null-body-status 不変条件
- 交差点の追加を機械的に検出する gate (clients/ui/src/__meta__/session-identity-ownership.test.ts
  と docs/note/note-20260624-technical-code-enforcement.md の catalogue 追記)
non_goals:
- 症状「Workspace Electron でターミナルが表示されない」の修正 (未帰属。Electron 実機 1 回の観測待ち)
- gateway が解決不能な session id を無音で捨てる挙動の変更 (再発条件そのものだが別 issue で追跡)
- wire protocol (frame 形状・kind) の変更
- fleet 構成 / servers.json 契約の変更
- branded type (FleetSessionKey / UpstreamSessionId) の全面導入 (別 change へ繰り延べ。再訪条件は ADR
  に記録)
- clients/workspace/src/shared/session-ref.ts の第 2 encoder 実装の canonical fixture 拘束
  (scope-adjacent。本 change は encoding を変えないため drift を活性化しない。follow-up issue で追跡)
- e2e/real-stack/terminal-input.spec.ts の CI 常設 (GREEN 化のみ本 change。常設は harness 側の
  follow-up)
- App.tsx の `?? "local"` が ARCHITECTURE.md の implicit default server 禁止に抵触するかの判断 (到達性は
  design decision-6 で実測済み。判断は follow-up issue)
change_classes:
- invariant
- boundary
- responsibility
- behavior
governance:
  gate: hard
  approval_evidence: User selected A = SSOT consolidation over the local patch and
    B = the recommended main-side envelope owner at the debug handoff, then accepted
    all three proposed ADRs at the design consultation (2026-08-11). The record is
    consultations/adr-approval.md with the typed verdicts in consultations/consultation-fleet-session-id-ssot-1.json
    (3x accept / user_consultation / confirmed).
  reasons:
  - 同一機構への 2 回目の修正 (9ef53ddc / 525b3091) であり、局所パッチ継続は 3 回目を構造的に招く
  - store / HTTP 境界の真逆の失敗方針 (bare id 素通し / bare id 拒否) を 1 つに決める挙動変更を含む
evidence_refs:
- type: test
  ref: npm:--prefix clients/ui run test:unit
- type: test
  ref: npm:--prefix clients/ui run typecheck
- type: test
  ref: npm:--prefix clients/ui run lint
- type: test
  ref: npm:--prefix clients/workspace run test:unit
- type: test
  ref: npm:--prefix clients/workspace run typecheck
- type: test
  ref: npm:--prefix clients/ui run test:e2e:real
- type: test
  ref: docs:lint --conformance
members:
- role: requirements
  path: changes/change-20260811-fleet-session-id-ssot/requirements.md
  required: true
- role: implementation
  path: changes/change-20260811-fleet-session-id-ssot/implementation.md
  required: true
- role: verification
  path: changes/change-20260811-fleet-session-id-ssot/verification.md
  required: true
- role: consultation
  path: changes/change-20260811-fleet-session-id-ssot/consultations/adr-approval.md
  required: false
promotion:
- target: design-clients-ui
  section: responsibilities
  action: upsert
  item:
    id: RESP-005
    statement: Own the translation between the compound fleet session key ([serverId,
      sessionId]) and the upstream bare session id, with one owner per boundary —
      Connection for the WS wire, and the fleet identity module (SessionRoute resolver
      + FleetSessionRow) for store and HTTP routing.
  reason: 変換の所有者が境界ごとに定まらないことが 17 交差点 / 8 失敗方針の根源であり、所有権の割当は このコードから導出できない設計判断として
    persistent design に属する。
- target: design-clients-ui
  section: invariants
  action: upsert
  item:
    id: INV-007
    statement: Connection's public API speaks fleet keys only; ClientFrame assembly
      and serialization stay private to socket/connection.ts, and every outbound WS
      frame carries the upstream bare session id.
    enforcement: test
  reason: frame 組み立て権威の所在は公開契約の意味であり、API 形状と fitness gate の両方で強制する。
- target: design-clients-ui
  section: invariants
  action: upsert
  item:
    id: INV-008
    statement: Every inbound session-id-bearing element is requalified to a fleet
      key through one path before reaching the store; store rows carry (serverId,
      upstreamSessionId) as required fields and session identity is decided by strict
      fleet-key equality (no alias fallbacks).
    enforcement: test
  reason: 下流の alias 総当たりは上流の再修飾漏れを吸収する仕組みであり、それを許すかどうかは store 所有権の不変条件。
- target: design-clients-ui
  section: invariants
  action: upsert
  item:
    id: INV-009
    statement: Data-plane session-id resolution failures never throw into the xterm
      emitter; the frame is dropped and recorded by reason — internal contract violations
      reach the user, benign server-switch transitions only increment a diagnostic
      counter.
    enforcement: test
  reason: 無音 drop の禁止と良性遷移の誤報防止は failure responsibility の割当であり、 実装ごとに揺れてはならない。
- target: design-clients-ui
  section: boundaries.forbidden
  action: upsert
  item:
    id: BND-004
    statement: Handling the fleet session key syntactically (sessionKey / parseSessionKey)
      or serializing a ClientFrame outside the declared owner set, and re-deriving
      a session route from an id string when the authoritative row is in hand.
  reason: 所有者集合の外での変換が新しい交差点を作る唯一の経路であり、禁止依存として明示する。
- target: design-workspace-desktop
  section: invariants
  action: upsert
  item:
    id: INV-006
    statement: The IPC response envelope represents "no body" as body null and never
      as an empty string; the type and builder in src/shared/http-envelope.ts own
      that invariant, and renderer-side tolerance is deliberate duplication, not the
      owner.
    enforcement: test
  reason: 「別意味を持つ zero value は別の型へ」という design-host の IPC Type Design Invariants を
    この境界へ適用した結果であり、所有者の所在は persistent な設計判断。
- target: design-workspace-desktop
  section: compatibility_policies
  action: upsert
  item:
    id: COMPAT-002
    statement: IPC envelope type changes roll out consumer-first (renderer and preload
      widen before main narrows), because the SPA dist is copied into the app and
      the main/renderer pair can be broken by dev runs or partial rebuilds.
  reason: コピー同梱という build 構造から導かれる移行順序であり、次に同じ境界を触る変更も従う必要がある。
unresolved_decisions: []
tags:
- fleet
- ui
- workspace
- invariant
owners: []
relations:
- {type: modifies, target: design-clients-ui}
- {type: modifies, target: design-workspace-desktop}
source_paths:
- clients/ui/src/socket/
- clients/ui/src/fleet/
- clients/ui/src/api/
- clients/ui/src/store/
- clients/ui/src/wire/server.ts
- clients/ui/src/components/
- clients/ui/src/App.tsx
- clients/ui/src/auth.ts
- clients/ui/src/__meta__/
- clients/workspace/src/shared/
- clients/workspace/src/main/electron-entry.ts
- clients/workspace/src/preload/
- clients/workspace/tests/
summary: Give the fleet-key/upstream-session-id translation a single owner per boundary,
  and make the Electron IPC response envelope carry the null-body-status invariant.
updated: '2026-08-11'
promotion_applied_at: '2026-08-11T14:14:36.199477+00:00'
closure:
  closed_at: '2026-08-11T14:14:55.356369+00:00'
  content_hash: sha256:1aa6de13020c558d7ad70d1e44b449e05b6ee570ff8780c559422203a33d1427
---

## Summary

## Closure Notes


{% transition from="draft" to="ready" date="2026-08-11" %}
Design closure complete: spine lint green, docs conform ok, 13 verdict=Y critique findings resolved, 10 grafts applied, and all three ADRs accepted by user consultation (consultation-fleet-session-id-ssot-1).
{% /transition %}


{% transition from="ready" to="active" date="2026-08-11" %}
Delivery in progress on delivery/fleet-session-id-ssot.
{% /transition %}


{% transition from="active" to="closing" date="2026-08-11" %}
All nine tasks done; entering promotion and closure.
{% /transition %}
