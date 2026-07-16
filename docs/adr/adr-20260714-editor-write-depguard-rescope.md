---
id: adr-20260714-editor-write-depguard-rescope
kind: adr
title: Rescope the workspace forbidigo rule for a per-file write-handler exclusion
  (supersedes wsviewer-no-write-depguard)
status: accepted
created: '2026-07-14'
decision_makers:
- Takehito Gondo
tags:
- workspace-editor
- design
owners: []
relations:
- {type: partOf, target: change-20260714-agent-workspace-editor}
- {type: supersedes, target: adr-20260714-wsviewer-no-write-depguard}
source_paths: []
summary: Rescope the workspace forbidigo rule for a per-file write-handler exclusion
  (supersedes wsviewer-no-write-depguard)
updated: '2026-07-14'
---

## Context

adr-20260714-wsviewer-no-write-depguard の accepted forbidigo rule は `pkg: github\.com/takezoh/agent-reactor/server/web` で全 workspace*.go の os.WriteFile 等を禁止する。しかし現行 module path は `github.com/takezoh/agent-grid` であり、pkg pattern は現時点で dormant (実質的に何も enforce していない)。本 editor plan は同一 package に新設 workspace_write.go を追加して write を許容する必要があり、単なる silent rescope は accepted ADR への conflict になる。

## Decision

(a) forbidigo rule の pkg pattern を `github\.com/takezoh/agent-grid/server/web` に修正して読み側 mutation 禁止を実際に aktivate する。(b) exclusions.rules[] に `path: "server/web/workspace_write\\.go$"; linters: [forbidigo]` を追加して write handler ファイルだけ mutation を許容する。この構成は adr-20260714-wsviewer-no-write-depguard を supersedes し、write handler の新設ファイル分離を構造的に enforce する。

## Consequences

- Read-side workspace*.go への os.WriteFile 等の追加は make lint 失敗として実際に阻止される (dormant 状態からの復活)。
- write handler 側の mutation は per-file exclusion で唯一許容され、regression テスト (exclusion を外すと lint 失敗) で allowlist の scope を証明する。
- 将来 write ファイル追加が必要になれば exclusions.rules[] の path pattern を明示的に拡張する PR が review 対象になる (silent 拡張が起きない)。

## Alternatives

- **却下: file-scoped pkg pattern による除外** — forbidigo の pkg pattern は Go の import path で照合するため file-level 除外はできない。
- **却下: workspace handler を別 Go package (例: workspacewrite) へ分離** — GuardWorkspacePath / resolveWorkspaceSession の同一 package 内再利用が失われ、unexport helper を新規 export する必要が生じる — 目的 (write handler の隔離) に対する影響が大きすぎる。
- **却下: per-call-site nolint directive** — 新規 mutation callsite ごとに annotation が要り、構造的保証が徐々に弱まる — nolintlint require-specific=true と組み合わせても drift 源になる。

## Trace

- Requirements: FR-103
- Implementation contracts: contract-write-structural-boundary
