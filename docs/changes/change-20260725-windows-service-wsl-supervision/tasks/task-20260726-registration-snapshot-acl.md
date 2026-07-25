---
id: task-20260726-registration-snapshot-acl
kind: task
title: Implement registration snapshot and ACL boundary
status: done
created: '2026-07-26'
priority: normal
effort: medium
files_touched:
- src/cmd/agent-grid-wsl-supervisor/
- src/platform/wslhost/config*.go
pr: null
tags: []
owners: []
relations:
- {type: partOf, target: change-20260725-windows-service-wsl-supervision}
source_paths: []
change: change-20260725-windows-service-wsl-supervision
summary: Create atomic JSON registration/apply semantics and fixed target binding.
updated: '2026-07-26'
---

## 責務

Atomic JSON registration/apply semantics と、登録時に固定される Windows identity・WSL distribution の境界を実装する。

## 詳細手順

- `encoding/json` と注入可能な SCM/filesystem/ACL seam を使い、versioned snapshot を検証後に atomic replace する。
- runtime は snapshot generation だけを設定源とし、登録後の既定 identity/distribution の変更を再解決しない。
- filesystem/config/ACL の fake、`FakeVsRealProgramDataACLAndRotation`、`ContractAtomicSnapshotAndLogACL` を同じ責務に含める。

## 前提

Registration snapshot ADR、machine-wide record ADR、implementation member の fixed decisions を正本とする。

## スコープ外

Runtime retry、health state machine、stop sequencing、record rotation 本体は扱わない。

## 受け入れ条件

- Atomic validated generation が唯一の runtime config source である。
- Default identity/distribution は registration 時だけ解決される。
- `verify-data-model`、`verify-security-boundary`、`verify-migration-compatibility`: `cd src && GOCACHE=/tmp/gocache-agent-grid go test ./platform/wslhost/... ./cmd/server/...` が成功する。


{% transition from="todo" to="in_progress" date="2026-07-26" %}
plan-delivery implementation and mechanical gates completed
{% /transition %}


{% transition from="in_progress" to="done" date="2026-07-26" %}
plan-delivery implementation and mechanical gates completed
{% /transition %}
