---
id: change-20260803-execution-server-selector-health
kind: change
title: Stable fleet health and recoverable Execution Server selection
status: draft
created: '2026-08-03'
profile: sdd@1
intent: quiet な正常接続を維持し、復旧後も Execution Server の設定・選択操作を利用可能にする
outcomes:
- lifecycle heartbeat により正常な quiet socket が health_unavailable へ誤遷移しない
- unavailable からの復旧時に session config と Command List が再取得される
- Change 後の正常なサーバーをクリックして実行先を変更できる
- サーバー名の文字色で接続状態を識別できる
- 正常でないサーバーはグレーアウトされ、クリックしても選択されない
scope:
- services/server/api/gateway.go
- services/server/api/gateway_heartbeat_test.go
- clients/web-host/internal/webhost/fleet.go
- clients/web-host/internal/webhost/fleet_health_test.go
- clients/workspace/src/main/fleet-session-runtime.ts
- clients/workspace/tests/fleet-session-runtime.test.ts
- clients/ui/src/App.tsx
- clients/ui/src/App.health-subscription.test.tsx
- clients/ui/src/components/palette/ExecutionServerSelector.tsx
- clients/ui/src/css/palette.css
- clients/ui/e2e/app.smoke.spec.ts
non_goals:
- Connections panel の状態表現変更
change_classes:
- behavior
- implementation_only
governance:
  gate: auto
  reasons: []
members:
- role: requirements
  path: changes/change-20260803-execution-server-selector-health/requirements.md
  required: true
- role: implementation
  path: changes/change-20260803-execution-server-selector-health/implementation.md
  required: true
- role: verification
  path: changes/change-20260803-execution-server-selector-health/verification.md
  required: true
promotion: []
unresolved_decisions: []
tags: []
owners: []
relations: []
source_paths:
- services/server/api/gateway.go
- clients/web-host/internal/webhost/fleet.go
- clients/workspace/src/main/fleet-session-runtime.ts
- clients/ui/src/App.tsx
- clients/ui/src/components/palette/ExecutionServerSelector.tsx
- clients/ui/src/css/palette.css
---

## Summary

Lifecycle WS の heartbeat、両 client host の freshness 更新、SPA の設定再取得、Execution Server selector の health presentation を一体で修正する。

## Closure Notes
