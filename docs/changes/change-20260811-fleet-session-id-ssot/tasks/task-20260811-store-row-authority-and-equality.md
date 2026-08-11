---
id: task-20260811-store-row-authority-and-equality
kind: task
title: store-row-authority-and-equality
status: done
created: '2026-08-11'
priority: normal
effort: medium
files_touched:
- clients/ui/src/store/daemon.ts
- clients/ui/src/store/statusSignals.ts
- clients/ui/src/store/workspaceActivity.ts
- clients/ui/src/App.tsx
- clients/ui/src/wire/server.ts
pr: null
tags: []
owners: []
relations:
- {type: partOf, target: change-20260811-fleet-session-id-ssot}
- {type: dependsOn, target: task-20260811-identity-owner-module}
- {type: dependsOn, target: task-20260811-wire-id-ingress}
source_paths: []
change: change-20260811-fleet-session-id-ssot
summary: 権威を型で required にし、下流の alias 吸収を撤去する。
max_diff_loc: 300
pinned_context:
- docs/changes/change-20260811-fleet-session-id-ssot/tasks/task-20260811-store-row-authority-and-equality.md
- /workspace/agent-grid/docs/changes/change-20260811-fleet-session-id-ssot/design-plan
updated: '2026-08-11'
---

## Responsibility

権威を型で required にし、下流の alias 吸収を撤去する。

## Execution contract

- Output: store 3 モジュールと App.tsx / wire/server.ts の変更、既存 test の追随。
- Tool guidance: 先に connection-ingress-requalification が入っていることを確認してから alias を消す (順序制約)。fixture factory 9 箇所と session literal を作る test file 21 を grep で列挙して一括更新する。
- Boundaries: publishSessions !== false 分岐そのものの削除と frameMessaging の production 未到達問題は直さない。fleet key の encoding は変えない。

## Acceptance

- store が FleetSessionRow を保持し、権威なしの行を作ると typecheck が落ちる
- SessionInfo から serverId? / serverLabel? / upstreamSessionId? が削除される
- sessionIdAliases / sessionIdsMatch が削除され、同一性判定が厳密一致になる
- signalIdentityKey が row.id と epoch の合成だけを返し、parseSessionKey / sessionKey / upstreamSessionId / serverId を参照しない
- Go 生成 fixture の round-trip (wire/codec.test.ts) が引き続き緑
- 525b3091 が固定した再接続時の bell 抑止挙動が保たれる
- 同一 upstream id を持つ別 server session が同時に存在しても status signal の consume map が混ざらない


{% transition from="todo" to="in_progress" date="2026-08-11" %}
delivery/fleet-session-id-ssot
{% /transition %}


{% transition from="in_progress" to="done" date="2026-08-11" %}
Implemented on delivery/fleet-session-id-ssot; unit suite 1918/1918 green, real-stack e2e 3/3 green.
{% /transition %}
