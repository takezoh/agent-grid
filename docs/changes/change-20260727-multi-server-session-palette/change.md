---
id: change-20260727-multi-server-session-palette
kind: change
title: Multi-server session palette
status: active
created: '2026-07-27'
profile: sdd@1
intent: 複数サーバー接続時も宛先境界を失わず、全サーバーのプロジェクトから有効な起動条件を選べるようにする。
outcomes:
- New Session の最初の候補面で全接続サーバーの project を比較できる
- session と project の所属 server を一覧走査中から識別できる
- server + project に有効な typed launch profile と任意個 parameter を確認して安全に作成できる
- 部分障害時も健康な server scope と有効な入力・focus を維持して再取得できる
scope:
- multi-server project discovery
- server-qualified session and project presentation
- typed project-scoped launch profiles and server-owned argv materialization
- partial fleet failure and one-server browser adaptation
non_goals:
- arbitrary shell command editor or client interpolation
change_classes:
- behavior
- responsibility
- boundary
- invariant
governance:
  gate: hard
  approval_evidence: User directed「進めて」and explicitly required removal of all legacy
    command definitions, compatibility paths, and fallbacks (2026-07-27).
  reasons:
  - multi-server credential and routing authority moves to the desktop main-process boundary
  - session creation wire/config contracts are intentionally breaking
members:
- role: requirements
  path: changes/change-20260727-multi-server-session-palette/requirements.md
  required: true
- role: implementation
  path: changes/change-20260727-multi-server-session-palette/implementation.md
  required: true
- role: verification
  path: changes/change-20260727-multi-server-session-palette/verification.md
  required: true
- role: ux
  path: changes/change-20260727-multi-server-session-palette/ux.md
  required: false
promotion:
- target: design-client
  section: invariants
  action: upsert
  item:
    id: INV-MULTI-SERVER-IDENTITY
    statement: Project and session identity remains server-qualified from discovery
      through active context and routing.
- target: design-client
  section: boundaries.provides
  action: upsert
  item:
    id: BOUNDARY-FLEET-AUTH-PROXY
    statement: Credentialed multi-server routing is owned outside the renderer; renderer
      consumes a narrow typed provider projection.
- target: design-client
  section: capabilities
  action: upsert
  item:
    id: CAPABILITY-TYPED-LAUNCH-PROFILE
    statement: A versioned server-owned launch profile validates typed values and
      materializes shell-free executable/argv.
unresolved_decisions: []
tags: []
owners: []
relations:
- {type: modifies, target: design-client}
- {type: conformsTo, target: design-platform}
source_paths: []
methodology: sdd
summary: 全接続サーバーのプロジェクトとサーバー別コマンドを識別可能に表示し、可変エージェント引数を選択できるセッション作成UXへ更新する。
updated: '2026-07-27'
---

## Summary

Specify 済み UX を、server-qualified identity、typed launch profile、main-owned fleet proxy、server-owned argv materialization、部分障害回復の反証可能な EARS/ADR/implementation contract へ確定した。

## Closure Notes
