---
id: task-20260729-workspace-github-account-catalog
kind: task
title: Workspace GitHub account and retained catalog
status: todo
created: '2026-07-29'
priority: normal
effort: large
files_touched:
- clients/workspace/src/main/github-*.ts
- clients/workspace/src/main/file-state-store.ts
- clients/workspace/tests/github-*.test.ts
- clients/workspace/tests/file-state-store.test.ts
pr: null
tags:
- github
- oauth
- workspace
- catalog
owners: []
relations:
- {type: partOf, target: change-20260728-github-repository-sessions}
- {type: dependsOn, target: task-20260728-github-host-account-catalog}
source_paths:
- clients/workspace/src/main/
- clients/workspace/tests/
pinned_context:
- docs/changes/change-20260728-github-repository-sessions/requirements.md
- docs/changes/change-20260728-github-repository-sessions/implementation.md
- docs/adr/adr-20260728-client-host-github-device-flow.md
- clients/workspace/src/main/file-state-store.ts
done_criteria:
- Workspace GitHub account/catalog serviceとprotected persistence
- FakeGitHubAuth/FakeGitHubCatalog contractとWeb-host parity
max_diff_loc: 550
change: change-20260728-github-repository-sessions
summary: Implement the fixed GitHub account/catalog contract in the Electron main
  process with protected persistence and credential-free projection.
---

# Workspace GitHub account and retained catalog

## 責務

Web hostで固定されたGitHub account/catalog contractをElectron main processへ実装し、protected file-state persistence、refresh singleflight、complete-generation stale catalog、credential-free projectionを同じfixtureで再現する。

## 詳細手順

1. injected HTTP/clock/random/credential-cache storeを使うdevice-flowとrefresh state machineをTypeScriptで実装する。
2. file-state-storeのatomic persistenceをcredential recordとcomplete catalog generationへ適用し、preload/rendererへcredentialを渡さない。
3. Web-host fixtureと同じoutcome/limit/cancellation/sentinel matrixをVitestで実行し、projectionをdeep-equal検証する。
4. FakeGitHubAuth/FakeGitHubCatalogとauthority未設定時のnamed FakeVsReal skipを機能seamに含める。

## 前提

`task-20260728-github-host-account-catalog`がshared config/fixtureとWeb-host基準projectionを提供する。`max_diff_loc: 550`はElectron mainのaccount/catalog/persistenceという単一TypeScript contextの上限である。

## スコープ外

Electron IPC/preload/Settings wiring、共有SPA、server lease/clone、extended metadataは扱わない。

## 受け入れ条件

- Workspaceの`GitHubAuthContract`がdistinct outcomesとcredential sentinel非露出を検証する。
- Workspaceの`GitHubCatalogContract`が全上限、partial-generation非公開、stale fallbackを検証する。
- 同一fixtureに対するWeb/Workspace credential-free projectionがdeep-equalになる。
- `cd clients/workspace && npm run test` が成功する。
