---
id: change-20260727-resilient-server-reconnect-health
kind: change
title: Resilient server reconnect and fleet health presentation
status: done
created: '2026-07-27'
profile: sdd@1
intent: 接続障害をヘッダーから除去し、有限な自動復旧と明示的な再試行、複数サーバーの健全性表示を一貫した契約にする。
outcomes:
- daemon復旧後に切断表示と操作制限が解除される
- 自動再試行停止後もステータスバーから再接続できる
- 接続障害はステータスバーで集約され、設定画面でサーバー別に診断できる
scope:
- clients/ui の接続状態遷移とステータスバー
- clients/workspace のruntime-only fleet health観測と設定画面
- clients/web-host/
- services/server/api/
- services/internal/harnesspolicy/
- test-harness/
- docs/design/design-host.md
non_goals:
- Windows Shell の既存tray health監視の統合
- servers.jsonへのruntime health永続化
change_classes:
- behavior
- responsibility
- boundary
- invariant
- capability
governance:
  gate: auto
  reasons: []
members:
- role: requirements
  path: changes/change-20260727-resilient-server-reconnect-health/requirements.md
  required: true
- role: implementation
  path: changes/change-20260727-resilient-server-reconnect-health/implementation.md
  required: true
- role: verification
  path: changes/change-20260727-resilient-server-reconnect-health/verification.md
  required: true
- role: ux
  path: changes/change-20260727-resilient-server-reconnect-health/ux.md
  required: false
promotion:
- target: design-client
  section: invariants
  action: upsert
  item:
    id: INV-010
    statement: Active client recovery is finite at sixteen automatic retries per manual
      lifecycle, is proved only by a current lifecycle-and-attempt daemon hello, and
      fences every retired asynchronous continuation.
    enforcement: contract
  reason: Recovery proof, progress bound, and stale-callback exclusion constrain future
    shared-client connection changes.
- target: design-client
  section: responsibilities
  action: upsert
  item:
    id: RESP-010
    statement: The existing application-scoped FleetRuntime is the sole host writer
      of configured-member health revisions; views consume its qualified projection
      and cannot probe or persist health.
  reason: Health ownership extends the accepted fleet-runtime boundary and prevents
    renderer/settings split authority.
- target: design-client
  section: failure_responsibilities
  action: upsert
  item:
    id: FAILURE-006
    statement: Host runtimes own member health evidence and revisions; provider adapters
      own delivery-liveness overlays only and cannot mint or advance host health revisions.
  reason: Delivery failure must remain observable without creating a second health
    authority.
unresolved_decisions: []
tags: []
owners: []
relations:
- {type: references, target: design-client}
- {type: references, target: adr-20260803-resilient-fleet-health-single-authority}
- {type: references, target: adr-20260727-application-scoped-fleet-session-authority}
- {type: references, target: adr-20260727-exact-match-client-server-protocol}
- {type: modifies, target: design-client}
source_paths:
- clients/ui/src/socket/connection.ts
- clients/ui/src/components/StatusBar.tsx
- clients/workspace/src/main/server-health.ts
- clients/workspace/src/main/settings-page.ts
- clients/workspace/src/main/fleet-session-runtime.ts
- clients/web-host/internal/webhost/fleet.go
- clients/ui/src/fleet/session-protocol.ts
- clients/ui/src/fleet/connections-presentation.ts
summary: Bound automatic reconnect, preserve manual recovery, and present per-server
  health through one fleet contract.
updated: '2026-08-03'
evidence_refs:
- type: test
  ref: clients/ui/e2e/reconnect-health.spec.ts
- type: test
  ref: clients/ui/e2e/real-stack/reconnect-health.spec.ts
- type: test
  ref: services/internal/harnesspolicy/testsupport/fleet_health_gateway_test.go
- type: test
  ref: services/server/api/fleet_health_e2e_test.go::TestFakeVsRealFleetHealthProbeEvidence
- type: command
  ref: cd clients/ui && npm run test:e2e
- type: command
  ref: cd clients/ui && npm run test:e2e:real
- type: command
  ref: GOCACHE=/tmp/gocache-agent-grid make test-e2e
- type: command
  ref: cd services && go test ./server/api/... ./internal/harnesspolicy/...
promotion_applied_at: '2026-08-03T06:56:38.079710+00:00'
closure:
  closed_at: '2026-08-03T09:10:52.639037+00:00'
  content_hash: sha256:eff19b5558bf02dc9c9ea0a6bacf367209b6c95e1ec2ec876ca6ba0c330340e1
---

## Summary

自動再接続は有限のまま維持し、停止後はステータスバーの Retry から再開する。
daemonの復旧成立点はWebSocket openではなくdaemon由来のhelloとする。
Web/Workspace hostの既存application-scoped FleetRuntimeが設定済みサーバーのruntime healthを唯一生成し、host-neutral providerを通じて同じqualified snapshotを配布する。delivery failureはprovider overlayとして表示するがhost revisionを更新しない。

## Closure Notes


{% transition from="draft" to="ready" date="2026-08-03" %}
implementation complete for conformance gate
{% /transition %}


{% transition from="ready" to="active" date="2026-08-03" %}
delivery in progress
{% /transition %}


{% transition from="active" to="closing" date="2026-08-03" %}
conformance and promotion
{% /transition %}


{% transition from="closing" to="active" date="2026-08-03" %}
prepare evidence and promotion before closing
{% /transition %}


{% transition from="active" to="closing" date="2026-08-03" %}
conformance complete; apply promotion
{% /transition %}


{% transition from="closing" to="active" date="2026-08-03" %}
fix promotion item ids for design schema
{% /transition %}


{% transition from="active" to="closing" date="2026-08-03" %}
apply schema-valid promotion
{% /transition %}


{% transition from="closing" to="active" date="2026-08-03" %}
reopen package to apply accepted task lifecycle transitions before closure
{% /transition %}


{% transition from="active" to="closing" date="2026-08-03" %}
all implementation tasks accepted; entering final validation and review
{% /transition %}


{% transition from="closing" to="active" date="2026-08-03" %}
reopen package to align declared scope with accepted implementation surface
{% /transition %}


{% transition from="active" to="closing" date="2026-08-03" %}
declared scope aligned with accepted implementation; resume final validation
{% /transition %}
