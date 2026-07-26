---
id: change-20260726-unified-driver-session-status
kind: change
title: 全 Driver 共通セッション状態と統一ステータス表示
status: done
created: '2026-07-26'
profile: sdd@1
intent: Codex Driver の OSC 依存状態推定を廃止し、全 Driver の権威ある観測を共通セッション状態へ正規化して、Windows Shell・Workspace・Web
  UI が同一の状態意味論とアイコンを表示する。
outcomes:
- 全 Driver が単一の共通セッション状態契約へ正規化され、同じ状態遷移・優先順位・終端規則を共有する。
- Codex Driver は OSC 0/2 のタイトルや OSC 通知を状態検出に使わず、app-server の通知・server request・応答を網羅的に分類して状態へ反映する。
- Windows Shell・Workspace・Web UI は同一の状態別アイコン、アニメーション、アクセシブル名称を表示する。
- 未知または将来追加された構造化イベントを黙殺せず、現在の composite 契約を保ったまま観測可能に処理する。
scope:
- src/host/state の Driver 共通状態契約、優先順位、reducer
- Codex・Claude・Gemini・Grok・Shell Driver の観測ソースから共通状態への正規化
- Codex app-server の notification、server request、response、lifecycle event の網羅的な分類と
  routing
- Codex の状態検出用途 OSC 0/2 pipeline と title parser の削除
- Windows Shell・Workspace・Web UI のステータス視覚契約と生成済み SDK/wire contract
- fake、contract、gateway scenario、client unit、Web smoke、Windows Shell UIA backstop
non_goals:
- 端末レンダリング自体に必要な VT 処理または Shell の OSC 133 prompt semantics の一律削除
- Agent 固有の生イベントを wire API として各クライアントへ露出すること
- Codex app-server protocol 自体の変更
change_classes:
- behavior
- responsibility
- boundary
- invariant
governance:
  gate: hard
  approval_evidence: User directed「それで進めて」「全て完了させて」「進めて」and clarified OSC removal
    is Codex Driver only (2026-07-26).
  reasons:
  - 全 Driver の状態意味論と server-client wire contract を変更する横断的な責務移動
  - 外部 app-server protocol の未知イベント互換性と human-input request の安全性に影響する
evidence_refs:
- type: test
  ref: go:test ./host/... ./server/api/... ./platform/agent/codexschema/... ./gorules
- type: test
  ref: web:unit+lint+typecheck
- type: test
  ref: codex-schema-check+protocol-compatibility
members:
- role: requirements
  path: changes/change-20260726-unified-driver-session-status/requirements.md
  required: true
- role: implementation
  path: changes/change-20260726-unified-driver-session-status/implementation.md
  required: true
- role: verification
  path: changes/change-20260726-unified-driver-session-status/verification.md
  required: true
- role: ux
  path: changes/change-20260726-unified-driver-session-status/ux.md
  required: false
promotion:
- action: none
  reason: The durable cross-driver and wire decisions are recorded in the accepted
    ADRs created by this change; no existing persistent design section requires promotion.
unresolved_decisions: []
tags:
- driver
- session-status
- app-server
- native-clients
- web-ui
owners: []
relations:
- {type: references, target: adr-20260624-0032-runstate-spinner-additive}
- {type: references, target: adr-20260624-0080-status-indicator-exempt-from-reduced-motion}
source_paths:
- src/host/state
- src/host/driver
- src/host/runtime/subsystem/stream
- clients/ui
- clients/workspace
- clients/windows-shell
summary: 全 Driver の観測を共通セッション状態へ正規化し、Codex の OSC 状態検出を app-server の網羅的イベント処理へ置換して、全クライアントの状態アイコンを統一する。
updated: '2026-07-26'
promotion_applied_at: '2026-07-26T09:14:19.023462+00:00'
closure:
  closed_at: '2026-07-26T09:14:23.713208+00:00'
  content_hash: sha256:0c17e32820e52940f1edab3fa67d384f3b37bbcc2f2f355608e34c6c68bac1db
---

## Summary

状態の正本をクライアント固有の表示判定ではなく server-side の共通 Driver 状態機械に置く。Codex は app-server の全メッセージ種別を分類し、Claude・Gemini・Grok・Shell も各自の権威ある観測を同じ状態へ正規化する。legacy scalar、legacy event fallback、Codex OSC status inference は composite-only wire、全生成 SDK、全クライアントへの flag-day cutover と同じ変更で削除する。Claude/Gemini の既存 title observation と Shell OSC 133 は維持する。

## Closure Notes


{% transition from="draft" to="ready" date="2026-07-26" %}
Requirements, ADRs, and implementation plan accepted.
{% /transition %}


{% transition from="ready" to="active" date="2026-07-26" %}
Implementation started.
{% /transition %}


{% transition from="active" to="closing" date="2026-07-26" %}
Implementation and scoped verification complete; environment limitations recorded.
{% /transition %}


{% transition from="closing" to="active" date="2026-07-26" %}
Reopened to add closure evidence required by the hard-governance gate.
{% /transition %}


{% transition from="active" to="closing" date="2026-07-26" %}
Closure evidence and no-promotion rationale recorded.
{% /transition %}
