---
id: task-20260811-identity-ownership-gate
kind: task
title: identity-ownership-gate
status: done
created: '2026-08-11'
priority: normal
effort: medium
files_touched:
- clients/ui/src/__meta__/session-identity-ownership.test.ts
- docs/note/note-20260624-technical-code-enforcement.md
pr: null
tags: []
owners: []
relations:
- {type: partOf, target: change-20260811-fleet-session-id-ssot}
- {type: dependsOn, target: task-20260811-connection-egress-intent-api}
- {type: dependsOn, target: task-20260811-store-row-authority-and-equality}
- {type: dependsOn, target: task-20260811-http-route-resolution}
source_paths: []
change: change-20260811-fleet-session-id-ssot
summary: 境界の帰結として得られる compile 時検出を、import 所有者の allowlist と検出器自身の発火証明で補う。
max_diff_loc: 300
pinned_context:
- docs/changes/change-20260811-fleet-session-id-ssot/tasks/task-20260811-identity-ownership-gate.md
- /workspace/agent-grid/docs/changes/change-20260811-fleet-session-id-ssot/design-plan
updated: '2026-08-11'
---

## Responsibility

境界の帰結として得られる compile 時検出を、import 所有者の allowlist と検出器自身の発火証明で補う。

## Execution contract

- Output: fitness test 1 file と note への追記。
- Tool guidance: src/__meta__/no-japanese.test.ts の per-file allowlist と締め方針の書き方に倣う。走査範囲は src/**/*.ts(x) から *.test.ts(x) / __meta__ / __tests__ を除いた production source。
- Boundaries: branded types は導入しない。ESLint / typescript-eslint も追加しない。走査対象を clients/workspace へ広げない (scope 外)。

## Acceptance

- 所有者集合の外から sessionKey / parseSessionKey を import すると test が落ちる (allowlist = src/fleet/session-identity.ts, src/socket/connection.ts)
- production source で serializeClientFrame を import できるのは src/socket/connection.ts だけであり、retry.ts に 1 行戻すと落ちる
- selectCodec / generatedCodecStub の production importer が src/wire/codec.ts だけであることも走査する
- 合成した違反 source (allowlist 外 module からの sessionKey import、および serializeClientFrame import) に対して gate が実際に失敗する negative test を持つ
- test の doc comment が検出できない範囲を明記している (helper を呼ばない暗黙依存 / tsconfig exclude により test tree は typecheck されないこと / clients/workspace は走査対象外であること)
- 新規依存を追加していない
- enforcement catalogue に節が追加されている


{% transition from="todo" to="in_progress" date="2026-08-11" %}
delivery/fleet-session-id-ssot
{% /transition %}


{% transition from="in_progress" to="done" date="2026-08-11" %}
Implemented on delivery/fleet-session-id-ssot; unit suite 1918/1918 green, real-stack e2e 3/3 green.
{% /transition %}
