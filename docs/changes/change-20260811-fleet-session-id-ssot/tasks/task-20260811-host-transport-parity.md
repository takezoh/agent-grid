---
id: task-20260811-host-transport-parity
kind: task
title: ipc-envelope-consumer-widening
status: done
created: '2026-08-11'
priority: normal
effort: medium
files_touched:
- clients/ui/src/auth.ts
- clients/ui/src/fleet/provider.ts
- clients/ui/src/fleet/provider.null-body-status.test.ts
- clients/ui/src/fleet/provider.status-parity.test.ts
- clients/workspace/src/preload/index.ts
- clients/workspace/src/preload/electron-preload.cjs
- clients/workspace/tests/electron-preload-bridge.test.ts
pr: null
tags: []
owners: []
relations:
- {type: partOf, target: change-20260811-fleet-session-id-ssot}
source_paths: []
change: change-20260811-fleet-session-id-ssot
summary: consumer を先に広げ、producer 変更が入る前後どちらでも renderer が壊れない状態を作る。
max_diff_loc: 300
pinned_context:
- docs/changes/change-20260811-fleet-session-id-ssot/tasks/task-20260811-host-transport-parity.md
- /workspace/agent-grid/docs/changes/change-20260811-fleet-session-id-ssot/design-plan
updated: '2026-08-11'
---

## Responsibility

consumer を先に広げ、producer 変更が入る前後どちらでも renderer が壊れない状態を作る。

## Execution contract

- Output: 型宣言 2 箇所 (renderer / preload) の変更、provider の 2 分岐、preload bridge の runtime test、T1/T2 test。
- Tool guidance: fake を real より寛容にしないこと。fake が real と食い違ったら fake を直す。electron-preload.cjs の runtime test は clients/workspace/tests/app-entry.test.ts の require 方式に倣い electron を stub する。
- Boundaries: main 側 electron-entry.ts はこの unit では変更しない (順序制約)。fleetFetch の認証・redirect 拒否ロジックには触らない。renderer 側 defensive 分岐は削除しない (恒久の二重化)。

## Acceptance

- renderer (auth.ts) と preload (index.ts) の envelope 型宣言が body string | null を受け入れる
- provider.request が null body status で例外を出さず Response を返す
- provider.request が旧 main の {status 204, body ""} を受けても例外を出さず、その分岐の存在理由 (build がコピー同梱のため main/renderer の対が崩れうる) を code comment に書く
- provider.ts:536-538 の repository-preparations が result.body null のとき JSON.parse せず tracked.preparation を維持する
- 実行時 bridge (electron-preload.cjs) が envelope の body null を coercion なしで renderer へ渡すことを runtime test が固定する
- 既存 RED provider.null-body-status.test.ts が緑になる
- fake bridge が real の builder に合わせて null body status で null を返すよう修正され、型注釈も string | null になる
- status 行列 {200,201,204,205,304,400,404,500} の parity test が両 provider で一致し、fake が real より寛容だと落ちる


{% transition from="todo" to="in_progress" date="2026-08-11" %}
delivery/fleet-session-id-ssot
{% /transition %}


{% transition from="in_progress" to="done" date="2026-08-11" %}
Implemented on delivery/fleet-session-id-ssot; unit suite 1918/1918 green, real-stack e2e 3/3 green.
{% /transition %}
