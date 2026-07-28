---
id: task-20260728-github-host-account-catalog
kind: task
title: Web-host GitHub account and retained catalog
status: todo
created: '2026-07-28'
priority: high
effort: large
files_touched:
- clients/web-host/internal/githubhost/
- clients/config/
pr: null
tags:
- github
- oauth
- client-host
- catalog
owners: []
relations:
- {type: partOf, target: change-20260728-github-repository-sessions}
- {type: dependsOn, target: task-20260728-repository-protocol-domain}
source_paths:
- clients/web-host/internal/githubhost/
- clients/config/
pinned_context:
- docs/changes/change-20260728-github-repository-sessions/requirements.md
- docs/changes/change-20260728-github-repository-sessions/implementation.md
- docs/adr/adr-20260728-client-host-github-device-flow.md
- clients/web-host/internal/webhost/
done_criteria:
- Web host の GitHub account/catalog projection
- protected credential record と complete-generation retained catalog
- FakeGitHubAuth、FakeGitHubCatalog、named contracts、opt-in FakeVsReal tests
max_diff_loc: 550
change: change-20260728-github-repository-sessions
summary: Implement fixed OAuth device flow, protected credential ownership, retained
  complete-generation catalog, and credential-free projection in the Web host.
---

# Web-host GitHub account and retained catalog

## 責務

Web hostで固定 OAuth App の device flow、credential の保護保存・refresh singleflight、bounded complete-generation repository catalog と stale fallbackをcredential-free projectionとして実装し、両hostが消費するconfig schema/fixtureを固定する。

## 詳細手順

1. injected HTTP transport、clock、random source、credential/cache store を使う account state machine をWeb hostに実装し、browser/SPAへ token を渡さない。
2. device-flow の success、denied、expired、cancelled、transport failure、refresh failure を distinct typed outcome にし、Settings 復帰用の状態だけを公開する。
3. catalog refresh を一世代ずつ構築し、timeout、deadline、page/item/byte/cache 上限を超えた世代は publish せず、最後の完全世代を stale selectable として残す。
4. FakeGitHubAuth、FakeGitHubCatalog、各 invariant contract と、authority 未設定時に明示 skip する FakeVsReal backstopをGo seamへ接続し、shared config schema/fixtureを検証する。

## 前提

`task-20260728-repository-protocol-domain` が credential-free account/catalog projection と repository identity の wire 型を提供する。OAuth credential は client host だけが所有する。

`max_diff_loc: 550` は、Go account state machine、catalog generation、二つのexternal-dependency triple、shared config fixturesを一つのWeb-host contextでreviewするための上限である。Workspace実装はclean language/component境界で後続taskへ分離する。

## スコープ外

Workspace/Electron実装、SPA 内認証、web storage、`gh auth` 依存、server durable credential、GitHub Enterprise、複数 account、extended search metadata は扱わない。

## 受け入れ条件

- Web hostの`GitHubAuthContract`がsuccess、denied、expired、cancelled、failureとcredential sentinel非露出を検証する。
- Web hostの`GitHubCatalogContract`がsingleflight、全上限、partial-generation非公開、stale fallback、source-local failureを検証する。
- shared config schema/fixtureが固定OAuth Appとcredential-free projectionだけを公開する。
- `cd clients/web-host && GOCACHE=/tmp/gocache-agent-grid go test ./...` が成功する。
