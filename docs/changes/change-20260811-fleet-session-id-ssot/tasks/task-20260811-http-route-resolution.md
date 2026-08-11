---
id: task-20260811-http-route-resolution
kind: task
title: http-route-resolver-adoption
status: done
created: '2026-08-11'
priority: normal
effort: medium
files_touched:
- clients/ui/src/api/sessions.ts
- clients/ui/src/api/transcripts.ts
- clients/ui/src/api/workspace.ts
- clients/ui/src/api/capsules.ts
- clients/ui/src/api/workspace.test.ts
pr: null
tags: []
owners: []
relations:
- {type: partOf, target: change-20260811-fleet-session-id-ssot}
- {type: dependsOn, target: task-20260811-identity-owner-module}
source_paths: []
change: change-20260811-fleet-session-id-ssot
summary: store と HTTP 境界の 3 方針を 1 つに収束させ、失敗の名前を形式ではなく結果にする。
max_diff_loc: 300
pinned_context:
- docs/changes/change-20260811-fleet-session-id-ssot/tasks/task-20260811-http-route-resolution.md
- /workspace/agent-grid/docs/changes/change-20260811-fleet-session-id-ssot/design-plan
updated: '2026-08-11'
---

## Responsibility

store と HTTP 境界の 3 方針を 1 つに収束させ、失敗の名前を形式ではなく結果にする。

## Execution contract

- Output: api 4 モジュールの変更と T0/T1/T2 test。api/workspace.test.ts は新規。
- Tool guidance: 各モジュールの現行方針 (P4 sessions.ts:469-474 / P5 transcripts.ts:26 workspace.ts:191 / P6 capsules.ts:104) を先に列挙してから 1 方針へ寄せる。呼び出し元が catch で分岐を再発明していないか確認する。api/sessions.ts:471 の fleet.initialServerId() fallback は resolver に取り込まない。
- Boundaries: fleet/provider.ts の request 実装 (B 側) には触らない。store の行型変更は次の unit。handle_session の値を変換しない。

## Acceptance

- 4 モジュールが URL path の session 成分を resolver 経由で得る。pinnedQuery の handle_session query param は routing 対象外として resolver を通さない
- 解決不能な id で HTTP 要求が 1 件も発行されず、かつその失敗が Error("network") として観察されないことが test で固定される
- P5 の bare id 素通しと P6 の invalid key throw が 1 つの unresolved に収束する
- transcripts の 304 / 204 分岐が新経路でも到達可能である
- resolver の分岐が doFetch の try に入る前 (要求発行前) で行われる


{% transition from="todo" to="in_progress" date="2026-08-11" %}
delivery/fleet-session-id-ssot
{% /transition %}


{% transition from="in_progress" to="done" date="2026-08-11" %}
Implemented on delivery/fleet-session-id-ssot; unit suite 1918/1918 green, real-stack e2e 3/3 green.
{% /transition %}
