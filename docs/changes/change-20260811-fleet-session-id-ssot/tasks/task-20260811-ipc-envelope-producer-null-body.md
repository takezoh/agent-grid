---
id: task-20260811-ipc-envelope-producer-null-body
kind: task
title: ipc-envelope-producer-null-body
status: done
created: '2026-08-11'
priority: normal
effort: medium
files_touched:
- clients/workspace/src/shared/http-envelope.ts
- clients/workspace/src/main/electron-entry.ts
- clients/workspace/tests/http-envelope.test.ts
pr: null
tags: []
owners: []
relations:
- {type: partOf, target: change-20260811-fleet-session-id-ssot}
- {type: dependsOn, target: task-20260811-host-transport-parity}
source_paths: []
change: change-20260811-fleet-session-id-ssot
summary: 不変条件の所有者を main 側 envelope 型に置き、同じ境界を通る次のルートが無音で壊れないようにする。
max_diff_loc: 300
pinned_context:
- docs/changes/change-20260811-fleet-session-id-ssot/tasks/task-20260811-ipc-envelope-producer-null-body.md
- /workspace/agent-grid/docs/changes/change-20260811-fleet-session-id-ssot/design-plan
updated: '2026-08-11'
---

## Responsibility

不変条件の所有者を main 側 envelope 型に置き、同じ境界を通る次のルートが無音で壊れないようにする。

## Execution contract

- Output: 新規 shared モジュールと electron-entry の handler 変更、T1/T2 test。
- Tool guidance: ipcMain.handle は import 時に登録され直接 test できないため、envelope 生成を pure function に切り出してから handler を書き換える。clients/workspace/src/shared/control-envelope.ts が先例。定数は platform oracle に従属させる。
- Boundaries: fleetFetch の認証・redirect 拒否ロジックには触らない。webProvider 側は変更しない。renderer / preload の型はこの unit では変えない (前 unit で済んでいる)。

## Acceptance

- envelope の型と builder が clients/workspace/src/shared/http-envelope.ts に置かれ、preload がそこから型を得る
- main が null body status のときだけ body null を出す
- platform oracle guard が null body status 集合の各要素で new Response("", {status}) が throw し、集合外の代表 status (200/201/400/500) では throw しないことを検証し、集合が変わったら落ちる
- 200 で本文が空文字のときは body が空文字であって null ではない
- upstream が 204 に本文を付けても envelope は null を出す


{% transition from="todo" to="in_progress" date="2026-08-11" %}
delivery/fleet-session-id-ssot
{% /transition %}


{% transition from="in_progress" to="done" date="2026-08-11" %}
Implemented on delivery/fleet-session-id-ssot; unit suite 1918/1918 green, real-stack e2e 3/3 green.
{% /transition %}
