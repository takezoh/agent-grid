---
id: task-20260728-durable-repository-preparation
kind: task
title: Durable repository preparation and environment runtime
status: todo
created: '2026-07-28'
priority: critical
effort: large
files_touched:
- services/host/runtime/runtime.go
- services/host/runtime/runtime_test.go
- services/host/runtime/interpret.go
- services/host/runtime/interpret_test.go
- services/host/runtime/bootstrap.go
- services/host/runtime/bootstrap_test.go
- services/host/runtime/backends.go
- services/host/runtime/persist.go
- services/host/runtime/persist_test.go
- services/host/runtime/repository_preparation*.go
- services/host/runtime/repository_environment*.go
- services/host/runtime/interpret_spawn.go
- services/host/runtime/interpret_spawn_test.go
- services/host/runtime/spawn_complete_test.go
- services/host/runtime/spawn_panic_test.go
- services/host/state/reduce_session.go
- services/host/state/reduce_session_test.go
- services/host/state/reduce_session_nav.go
- services/host/state/reduce_helpers.go
- services/host/state/reduce_lifecycle_test.go
- services/platform/lib/gitclone/publication*.go
- services/platform/lib/gitclone/reconcile*.go
- services/platform/agentlaunch/
- services/platform/sandbox/
pr: null
tags:
- github
- repository-preparation
- durability
- recovery
- devcontainer
- repository-environment
owners: []
relations:
- {type: partOf, target: change-20260728-github-repository-sessions}
- {type: dependsOn, target: task-20260728-secure-clone-authority}
source_paths:
- services/host/state/
- services/host/runtime/
- services/platform/lib/gitclone/
pinned_context:
- docs/changes/change-20260728-github-repository-sessions/requirements.md
- docs/changes/change-20260728-github-repository-sessions/implementation.md
- docs/changes/change-20260728-github-repository-sessions/verification.md
- docs/adr/adr-20260728-durable-repository-preparation.md
- docs/adr/adr-20260728-repository-devcontainer-scope.md
- services/host/runtime/runtimetest/harness.go
- services/host/runtime/persist.go
- services/host/state/reduce.go
done_criteria:
- durable creation intent/preparation store と revision-fenced reducer/effects
- atomic clone publication、capacity、cancel/retry/abandon、startup reconciliation
- FakeFilesystemProcess、named contract、opt-in FakeVsRealFilesystemProcess
- repository-scoped environment store/manager と FakeVsRealRepositoryDevcontainer
max_diff_loc: 2200
change: change-20260728-github-repository-sessions
summary: Compose durable preparation, atomic clone publication, restart recovery,
  and repository-scoped devcontainer reuse through the closed Runtime interpreter.
---

# Durable repository preparation and environment runtime

## 責務

creation intent を I/O 前に永続化し、session 固有 staging clone を durability claim に従って atomic publish する。capacity、revision fencing、cancel/retry/abandon、restart reconciliation と repository-scoped devcontainer ensure/reuse を、同じ `Runtime` composition・effect interpreter・bootstrap authorityで管理する。

## 詳細手順

1. intent/preparation store を same-filesystem temp write、file fsync/close、atomic rename、parent-directory fsync で実装し、generation と observed revision による at-most-one worker を保証する。
2. active 4、queued 32、repository ごとの active 2、intent ごとの process/staging 1、never-available clone 20 GiB の admission を filesystem mutation 前に判定する。
3. clone staging、immutable marker、atomic rename、directory durability、record commit の crash point を明示し、conflict/unsupported path を overwrite せず fail closed にする。
4. cancel は owned process だけを停止し、clone-ready は devcontainer retry 用に保持する。restart 時は lease を破棄し、進行中を interrupted に正規化して durable evidence を reconcile する。
5. execution-server-qualified repository identity、config generation、sessions rootを持つenvironment store/managerを注入し、matching ready environmentを独立clone間で再利用する。FakeFilesystemProcessとFakeRepositoryDevcontainerのcontract/FakeVsRealを同じcomposition seamへ接続する。

## 前提

`task-20260728-repository-protocol-domain` の aggregate/reducer 型と、`task-20260728-secure-clone-authority` の one-use clone seam を使用する。runtime propagation test は `runtimetest.Harness` で構築する。

`max_diff_loc: 2200` は、preparation store、runtime fields/config、closed effect/spawn-result dispatch、bootstrap reconciliation、repository environment manager、pending terminal launchが同じState/Runtime owner filesを変更するための上限である。分割するとspawn成功前公開または直接owner競合が生じるため、launch-to-availability linearizationを同じcomposition責務へ閉じ込める。

## スコープ外

UI projection、completed workspace retention policy、shared clone、per-session/shared devcontainerは扱わない。

## 受け入れ条件

- distinct intents、再送、active/queued/repository/disk capsがat-most-one workerとmutation前admissionを満たす。
- crash-point、cancel/retry/abandon/restart matrixがpartial Session/overwriteを生まず、clone-readyからのenvironment retryはcloneを再実行しない。
- 同じserver/repository/config generationはready environmentを再利用し、別scope/conflicting generationは共有しない。
- sessions rootだけがrepository mount scopeとなり各frame cwdはsession cloneになる。terminal launch成功resultだけがpreparationをavailableへ一度commitし、failure/cancel/late-success raceはSessionを公開せず資源を解放する。
- `cd services && GOCACHE=/tmp/gocache-agent-grid go test ./host/state/... ./host/runtime/... ./platform/lib/gitclone/... ./platform/sandbox/... ./platform/agentlaunch/...` が成功し、filesystem/process/devcontainer fakesとlaunch race testsが通る。
