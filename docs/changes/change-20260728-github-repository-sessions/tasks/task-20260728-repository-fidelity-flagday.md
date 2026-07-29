---
id: task-20260728-repository-fidelity-flagday
kind: task
title: External fidelity and exact-version flag day
status: todo
created: '2026-07-28'
priority: high
effort: large
files_touched:
- services/
- clients/
- protocol/
- scripts/
pr: null
tags:
- github
- protocol
- fidelity
- delivery
owners: []
relations:
- {type: partOf, target: change-20260728-github-repository-sessions}
- {type: dependsOn, target: task-20260728-repository-fleet-ux}
source_paths:
- services/
- clients/
- protocol/
- scripts/
pinned_context:
- docs/changes/change-20260728-github-repository-sessions/implementation.md
- docs/changes/change-20260728-github-repository-sessions/verification.md
- docs/changes/change-20260728-github-repository-sessions/design-plan.json
- protocol/protocol-version.json
- protocol/simulator/
- clients/sdk/
done_criteria:
- daemon/gateway/host/SPA/SDK/simulator/fixture/fake の exact-version consumer inventory
- external dependency triple inventory と explicit T3 skip authority
- full repository gates と old/new mismatch no-side-effect matrix
max_diff_loc: 1000
change: change-20260728-github-repository-sessions
summary: Complete all dependency triples and migrate every protocol consumer under
  the same exact-version token.
---

# External fidelity and exact-version flag day

## 責務

repository protocol の closed consumer inventory を同じ exact-version token へ移行し、各実装タスクが所有する fake・named invariant contract・FakeVsReal triple と fixture authority を横断的に登録する。残存 consumer/fixture/drift tooling の更新だけを担当し、上流機能のテストを後付けするフェーズにはしない。

## 詳細手順

1. daemon IPC、gateway REST/WS、Web/Workspace/Windows host parser、SPA codec、SDK、simulator、recording/fixture、Playwright backend の inventory を確定し、同じ protocol token と repository/preparation shape へ一括移行する。
2. old-client/new-server と new-client/old-server の mismatch matrix が credential、lease、preparation、process、filesystem side effect 前に失敗することを round-trip/drift tests で固定する。
3. GitHub auth/API、git clone、filesystem/process、repository devcontainer の fake、contract、FakeVsReal、fixture authority、明示 skip 条件を inventory と gate に登録し、欠けた triple を owner task の seam に帰属させる。
4. no-new-dependency を module/package/lock/import diff で検査し、owner/name validation evidence と T3 authority の結果または explicit skip を記録する。

## 前提

`task-20260728-repository-fleet-ux` までの全 production seam と task-owned tests が存在する。standard-library composition と既存 wrapper の選択は、同じ contract observables を返す限り自由である。

`max_diff_loc: 1000` は、daemon/gateway/全host/SPA/SDK/simulator/fixture/generatorのclosed consumer inventoryと元ファイルcontract assertionを一つのexact-version tokenへ原子的に移す校正済みinterface-confinement上限である。consumer groupを別taskへ分けるmixed-version中間commitは禁止する。

## スコープ外

上流機能の実装/テスト分離、test 削除、skip 追加による gate 回避、mixed-version fallback、新規第三者依存、extended search metadata は扱わない。

## 受け入れ条件

- closed consumer inventory の全 surface が同じ exact-version token と repository/preparation round trip を使用する。
- old/new mismatch matrix が credential、preparation、process、filesystem side effect を一件も生成しない。
- 五つの external dependency triple が fake・named contract・FakeVsReal・fixture authority・explicit skip condition を持つ。
- `make lint`、`cd services && GOCACHE=/tmp/gocache-agent-grid go test ./...`、`cd clients/ui && npm run lint && npm run test:unit && npm run test:e2e && npm run test:e2e:real`、`dotnet test clients/windows-shell/AgentGrid.Shell.Core.Tests/AgentGrid.Shell.Core.Tests.csproj`、`make test-e2e` が成功する。未設定T3 authorityはnamed explicit skip evidenceを返す。
