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
- boundary
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
promotion: []
unresolved_decisions: []
tags: []
owners: []
relations: []
source_paths:
- clients/ui/src/socket/connection.ts
- clients/ui/src/components/StatusBar.tsx
- clients/workspace/src/main/server-health.ts
- clients/workspace/src/main/settings-page.ts
summary: Bound automatic reconnect, preserve manual recovery, and present per-server health through one fleet contract.
---

## Summary

自動再接続は有限のまま維持し、停止後はステータスバーの Retry から再開する。
daemonの復旧成立点はWebSocket openではなくdaemon由来のhelloとする。
Workspace mainが設定済みサーバーのruntime healthを観測し、rendererへ集約結果を配布する。

## Closure Notes
