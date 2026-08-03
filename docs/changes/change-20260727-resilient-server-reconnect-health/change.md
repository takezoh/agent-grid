---
id: change-20260727-resilient-server-reconnect-health
kind: change
title: Resilient server reconnect and fleet health presentation
status: draft
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
    id: resilient-active-connection-recovery
    statement: Active client recovery is finite at sixteen automatic retries per manual
      lifecycle, is proved only by a current lifecycle-and-attempt daemon hello, and
      fences every retired asynchronous continuation.
  reason: Recovery proof, progress bound, and stale-callback exclusion constrain future
    shared-client connection changes.
- target: design-client
  section: responsibilities
  action: upsert
  item:
    id: application-scoped-fleet-health-authority
    statement: The existing application-scoped FleetRuntime is the sole host writer
      of configured-member health revisions; views consume its qualified projection
      and cannot probe or persist health.
  reason: Health ownership extends the accepted fleet-runtime boundary and prevents
    renderer/settings split authority.
- target: design-client
  section: failure_responsibilities
  action: upsert
  item:
    id: fleet-health-delivery-overlay
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
---

## Summary

自動再接続は有限のまま維持し、停止後はステータスバーの Retry から再開する。
daemonの復旧成立点はWebSocket openではなくdaemon由来のhelloとする。
Web/Workspace hostの既存application-scoped FleetRuntimeが設定済みサーバーのruntime healthを唯一生成し、host-neutral providerを通じて同じqualified snapshotを配布する。delivery failureはprovider overlayとして表示するがhost revisionを更新しない。

## Closure Notes
