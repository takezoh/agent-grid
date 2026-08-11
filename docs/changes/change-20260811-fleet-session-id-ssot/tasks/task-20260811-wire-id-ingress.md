---
id: task-20260811-wire-id-ingress
kind: task
title: connection-ingress-requalification
status: done
created: '2026-08-11'
priority: normal
effort: medium
files_touched:
- clients/ui/src/socket/connection.ts
- clients/ui/src/socket/connection.test.ts
pr: null
tags: []
owners: []
relations:
- {type: partOf, target: change-20260811-fleet-session-id-ssot}
- {type: dependsOn, target: task-20260811-identity-owner-module}
source_paths: []
change: change-20260811-fleet-session-id-ssot
summary: inbound 側の id 空間を fleet key に統一し、下流の alias 総当たりを不要にする前提条件を作る。
max_diff_loc: 300
pinned_context:
- docs/changes/change-20260811-fleet-session-id-ssot/tasks/task-20260811-wire-id-ingress.md
- /workspace/agent-grid/docs/changes/change-20260811-fleet-session-id-ssot/design-plan
updated: '2026-08-11'
---

## Responsibility

inbound 側の id 空間を fleet key に統一し、下流の alias 総当たりを不要にする前提条件を作る。

## Execution contract

- Output: connection.ts の inbound 経路の変更と T1 test 追加。
- Tool guidance: handleMessage の全 case を読み、sessionId を保持する要素を漏れなく列挙する (:305-309 output / fleetSessionID の tt,et / :356-363 activity_session_id / :405-415 qualifyHelloSessions / :326,:342 tagSessions)。wire/server.ts の ViewUpdateFrame も確認する。
- Boundaries: outbound (send / intent API) はこの unit では触らない。store 側の alias 撤去も行わない。cfg.serverId が fleet member でない場合の特別扱い (magic string 分岐) を作らない。

## Acceptance

- output / tt / et / activity_session_id / activity event の session_id / hello・view の sessions がすべて 1 つの再修飾経路を通る
- activity_session_id が bare のまま store へ渡らないことが test で固定される
- publishSessions !== false 分岐の tagSessions も同じ再修飾経路へ合流し、initialServerHint による独立 tagging が残らない (分岐自体は削除しない)
- 再修飾の割り当てが frame 数に比例しない (session 単位 memo)
- cfg.serverId 未設定時の素通し挙動が保たれる


{% transition from="todo" to="in_progress" date="2026-08-11" %}
delivery/fleet-session-id-ssot
{% /transition %}


{% transition from="in_progress" to="done" date="2026-08-11" %}
Implemented on delivery/fleet-session-id-ssot; unit suite 1918/1918 green, real-stack e2e 3/3 green.
{% /transition %}
