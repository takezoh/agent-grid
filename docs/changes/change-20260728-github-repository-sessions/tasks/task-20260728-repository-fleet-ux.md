---
id: task-20260728-repository-fleet-ux
kind: task
title: Categorized accessible repository picker UX
status: todo
created: '2026-07-28'
priority: high
effort: large
files_touched:
- clients/ui/src/
- clients/ui/e2e/
pr: null
tags:
- github
- fleet
- ui
- accessibility
owners: []
relations:
- {type: partOf, target: change-20260728-github-repository-sessions}
- {type: dependsOn, target: task-20260729-repository-host-projection}
source_paths:
- clients/ui/src/
- clients/ui/e2e/
pinned_context:
- docs/changes/change-20260728-github-repository-sessions/ux.md
- docs/changes/change-20260728-github-repository-sessions/requirements.md
- docs/changes/change-20260728-github-repository-sessions/verification.md
- docs/adr/adr-20260728-client-host-github-device-flow.md
- clients/ui/src/store/palette.ts
- clients/ui/src/fleet/provider.ts
done_criteria:
- host-neutral credential-free FleetRuntime projection
- categorized project/repository picker、Settings return、repository-first server selection
- accessible preparation progress/retry と browser/Workspace parity tests
max_diff_loc: 800
change: change-20260728-github-repository-sessions
summary: Consume the credential-free host projection in the shared SPA and deliver
  the accessible categorized repository-first picker and staged recovery flow.
---

# Categorized accessible repository picker UX

## 責務

credential-free host projectionを共有 SPA で消費し、既存 fleet project flowを保ったcategorized target picker、Settings認証復帰、repository-first server selection、段階別progress/retryをaccessibleに実装する。

## 詳細手順

1. target を project/repository の discriminated union とし、row ごとに source と stable identity を常時表示する純粋 palette reducer を実装する。
2. source-local auth/refresh/failure control を listbox 外の focusable control group に置き、query/all-source filter、健康な結果、有効な選択、stable-key focus を scoped invalidation 後も保持する。
3. Settings account surface への遷移と success/denied/expired/cancel/failure 復帰を実装し、GitHub category entry へ deterministic に focus を戻す。
4. repository 選択後に execution server を選び、clone/devcontainer の stage、typed failure、retry origin、environment reuse、revision を一つの polite live region で重複なく通知する。
5. UI unit/real-DOM、Playwright fake backend、real-stackを機能実装と同じReact contextに含める。

## 前提

`task-20260729-repository-host-projection` のcredential-free account/catalog/preparation/environment snapshotとSettings actionsを消費する。

`max_diff_loc: 800` は、pure palette reducer、React DOM/accessibility、fake-backend/real-stack Playwrightで19 UACを一つのUI contextとしてreviewするための上限である。host/Electron/Windows wiringはclean boundaryとして先行taskへ分離する。

## スコープ外

source-first topology、interleaved ranking、inline OAuth、extended metadata、branch/fork chooser、renderer credential authority、project flow の単一 server 化は扱わない。

## 受け入れ条件

- UAC-001〜UAC-019 が browser と Workspace-hosted mode で通り、GitHub failure 中も fleet project session を作成できる。
- grouped listbox には target option だけが入り、status controls、aria-activedescendant、focus recovery、単一 polite live region が real-DOM/Playwright で検証される。
- repository→server→clone→devcontainer/reuse→available の順序と、cancel/retry/invalidation の context preservation が観測できる。
- owner/name-only search corpus の known-target または ambiguity failure は assumption を invalidated にし、metadata/ranking を黙って拡張しない。
- `cd clients/ui && npm run lint && npm run test:unit && npm run test:e2e && npm run test:e2e:real` が成功する。T3 authorityが無い場合はnamed authorityとexplicit skip evidenceを記録する。
