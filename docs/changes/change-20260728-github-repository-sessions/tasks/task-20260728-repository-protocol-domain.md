---
id: task-20260728-repository-protocol-domain
kind: task
title: Repository protocol and preparation domain
status: todo
created: '2026-07-28'
priority: critical
effort: large
files_touched:
- protocol/
- services/host/state/repository*.go
- services/host/state/state.go
- services/host/state/event.go
- services/host/state/effect.go
- services/host/state/reduce.go
- services/host/state/state_test.go
- services/host/state/event_dispatch_test.go
- services/host/state/reduce_event_test.go
- services/host/state/reduce_lifecycle_test.go
- services/host/proto/repository*.go
- services/host/proto/codec*.go
- services/server/api/repository_wire*.go
- services/server/api/protocol_version*.go
pr: null
tags:
- github
- protocol
- repository-preparation
owners: []
relations:
- {type: partOf, target: change-20260728-github-repository-sessions}
source_paths:
- protocol/
- services/host/state/
- services/host/proto/
- services/server/api/
pinned_context:
- docs/changes/change-20260728-github-repository-sessions/requirements.md
- docs/changes/change-20260728-github-repository-sessions/implementation.md
- docs/changes/change-20260728-github-repository-sessions/verification.md
- docs/adr/adr-20260728-durable-repository-preparation.md
- protocol/protocol-version.json
- services/host/state/state.go
- services/host/proto/envelope.go
done_criteria:
- Repository、creation intent、preparation、session、clone path、repository environment の型と codec
- 純粋 reducer の lifecycle・revision fencing・availability invariant テスト
- exact-version mismatch の no-side-effect 契約テスト
max_diff_loc: 800
change: change-20260728-github-repository-sessions
summary: Define exact-version repository/preparation identities and pure lifecycle
  types before I/O.
---

# Repository protocol and preparation domain

## 責務

GitHub repository session に必要な fully-qualified identity、exact-version wire surface、永続 preparation aggregate と純粋な lifecycle transition を、外部 I/O より先に定義する。未完成 preparation を available Session に投影できる唯一の commit 条件もこの責務で固定する。

## 詳細手順

1. repository、execution server、creation intent、preparation、session、clone path、repository environment を相互に代用できない型として protocol、daemon IPC、gateway DTO に追加する。
2. preparation の stage、typed failure、retry origin、environment disposition、monotonic revision と event/effect table を reducer に実装し、available Session への commit を clone・environment・launch 成功後の一遷移に限定する。
3. codec round trip、invalid identity、revision conflict、incomplete/cancelled/interrupted/corrupt preparation の非公開、exact-version mismatch の no-side-effect を focused test で固定する。

## 前提

既存の exact-version policy、single-writer reducer、`runtimetest.Harness`、server-qualified fleet identity を維持する。公開 interface の変更と同じ compile 単位の codec・fixture・test はこのタスクに閉じ込める。

`max_diff_loc: 800` は、`State`、closed `Event`/`Effect` sum、`Reduce` dispatch、protocol codec を一つの compile-safe interface change に閉じ込めるための上限である。これらを別taskへ分けると中間commitが未処理eventまたは不完全wire contractを持つため、interface confinementを粒度より優先する。

## スコープ外

OAuth、credential lease、git process、filesystem publication、devcontainer、共有 UI は実装しない。mixed-version fallback、shared clone、per-session devcontainer、新規第三者依存は追加しない。

## 受け入れ条件

- `contract-target-projection` の identity と source-discriminated round trip が通る。
- `contract-repository-preparation-lifecycle` の全 terminal/non-terminal transition と sole availability commit が reducer test で通る。
- exact-version mismatch が lease、preparation、process、filesystem の side effect より前に拒否される契約テストが通る。
- `cd services && GOCACHE=/tmp/gocache-agent-grid go test ./host/state/... ./host/proto/... ./server/api/...` が成功する。
