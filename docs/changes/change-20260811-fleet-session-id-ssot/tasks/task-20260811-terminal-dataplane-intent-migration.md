---
id: task-20260811-terminal-dataplane-intent-migration
kind: task
title: terminal-dataplane-intent-migration
status: done
created: '2026-08-11'
priority: normal
effort: medium
files_touched:
- clients/ui/src/components/TerminalPane.tsx
- clients/ui/src/components/DriverShortcutBar.tsx
- clients/ui/src/components/TerminalMobileOverlay.tsx
- clients/ui/src/components/TerminalPane.test.tsx
pr: null
tags: []
owners: []
relations:
- {type: partOf, target: change-20260811-fleet-session-id-ssot}
- {type: dependsOn, target: task-20260811-connection-egress-intent-api}
source_paths: []
change: change-20260811-fleet-session-id-ssot
summary: frame 組み立て権威を component から取り除き、component は意図だけを渡す形にする。
max_diff_loc: 300
pinned_context:
- docs/changes/change-20260811-fleet-session-id-ssot/tasks/task-20260811-terminal-dataplane-intent-migration.md
- /workspace/agent-grid/docs/changes/change-20260811-fleet-session-id-ssot/design-plan
updated: '2026-08-11'
---

## Responsibility

frame 組み立て権威を component から取り除き、component は意図だけを渡す形にする。

## Execution contract

- Output: component の呼び出し変更と既存 test の追随。
- Tool guidance: TerminalPane.tsx の 169-194 と 271-277 を読み、xterm の同期 emitter 内で例外が出ないことを確認する。fake conn の形を intent API に合わせる際、assertion を弱めない。
- Boundaries: xterm の初期化・fit・geometry publish のロジックには触らない。

## Acceptance

- TerminalPane の 4 emission site (:172 :182 :193 :276) が intent API を呼び、wire frame を組み立てない
- fake conn を使う既存 test 群 (newline-modifier / pc-baseline) が新しい intent API 形状に追随し、bare id 前提の assertion が壊れない
- mobile overlay と shortcut bar の sendInput 経路が同じ intent API に合流する
- fit 後の実寸 (debug run 実測 cols 107 / rows 32) で {k "r"} が wire に出ることが test で固定される


{% transition from="todo" to="in_progress" date="2026-08-11" %}
delivery/fleet-session-id-ssot
{% /transition %}


{% transition from="in_progress" to="done" date="2026-08-11" %}
Implemented on delivery/fleet-session-id-ssot; unit suite 1918/1918 green, real-stack e2e 3/3 green.
{% /transition %}
