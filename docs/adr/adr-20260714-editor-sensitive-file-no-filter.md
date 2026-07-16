---
id: adr-20260714-editor-sensitive-file-no-filter
kind: adr
title: Write path applies no sensitive-file filter (parity with read side)
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
source_paths: []
summary: Write path applies no sensitive-file filter (parity with read side)
updated: '2026-07-14'
---

## Context

user consultation 決定(2b) と read 側 DP-SENSITIVE-FILE-EXPOSURE=OPT-NO-FILTER の対称性維持について、design_choice として明示的に ADR で固定する必要がある。

## Decision

.env / .git/config / *.key など慣習的に sensitive とされる path 名を write path が特別扱いしない。GuardWorkspacePath による workspace-root 境界のみが検査され、コンテンツ/ファイル名ベースの masking / reveal / 追加確認 UI は導入しない。

## Consequences

- read 側と write 側の invariant が対称になり、operator の期待動作が predictable になる。
- sensitive file の露出は既存の read 側と同じ trust boundary (workspace root 境界内は operator の責任) に依存する。
- 将来 sensitive-file filter が必要になった場合は、read/write 両方に対する新規 design proposal が要求される。

## Alternatives

- **却下: write 側だけ mask/reveal を導入** — read 側との非対称性を作り、operator のメンタルモデルを壊す。

## Trace

- Requirements: FR-103
- Implementation contracts: contract-write-permission-parity
