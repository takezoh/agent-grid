---
id: task-20260811-identity-owner-module
kind: task
title: identity-owner-module
status: done
created: '2026-08-11'
priority: normal
effort: medium
files_touched:
- clients/ui/src/fleet/session-identity.ts
- clients/ui/src/fleet/session-protocol.ts
- clients/ui/src/fleet/session-identity.test.ts
pr: null
tags: []
owners: []
relations:
- {type: partOf, target: change-20260811-fleet-session-id-ssot}
source_paths: []
change: change-20260811-fleet-session-id-ssot
summary: 2 つの id 空間と store 行型、単一 resolver を 1 つのモジュールに定義し、以降の unit が参照できる権威を先に立てる。
max_diff_loc: 300
pinned_context:
- docs/changes/change-20260811-fleet-session-id-ssot/tasks/task-20260811-identity-owner-module.md
- /workspace/agent-grid/docs/changes/change-20260811-fleet-session-id-ssot/design-plan
updated: '2026-08-11'
---

## Responsibility

2 つの id 空間と store 行型、単一 resolver を 1 つのモジュールに定義し、以降の unit が参照できる権威を先に立てる。

## Execution contract

- Output: 新規 TypeScript モジュールと T0 test。既存 import の付け替えのみで挙動は変えない。
- Tool guidance: Read で既存 session-protocol.ts (432 行) の全体を把握してから carve-out する。既存 import 元は grep で列挙する。resolveSessionRoute は throw せず判別 union を返す (api/sessions.ts:289-309 の doFetch が throw を Error("network") へ写像するため)。
- Boundaries: api / store / socket の呼び出し側は変更しない。SessionInfo からの field 削除もこの unit では行わない。fleet key の encoding そのものは変えない。

## Acceptance

- SessionRef / sessionKey / parseSessionKey が clients/ui/src/fleet/session-identity.ts へ移り、session-protocol.ts からは参照のみになる
- FleetSessionRow 型が id (fleet key) / serverId / upstreamSessionId を required で宣言し、同 module に置かれる
- resolveSessionRoute が行由来・decode 由来・unresolved の 3 系統を判別 union で返し、throw しない
- 行の権威が decode より優先されることが T0 test で固定される
- 挙動変更はまだ入らない (既存 test が全て緑のまま)


{% transition from="todo" to="in_progress" date="2026-08-11" %}
delivery/fleet-session-id-ssot
{% /transition %}


{% transition from="in_progress" to="done" date="2026-08-11" %}
Implemented on delivery/fleet-session-id-ssot; unit suite 1918/1918 green, real-stack e2e 3/3 green.
{% /transition %}
