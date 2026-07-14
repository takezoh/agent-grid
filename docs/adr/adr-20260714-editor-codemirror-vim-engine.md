---
id: adr-20260714-editor-codemirror-vim-engine
kind: adr
title: Mutation-capable editor is CodeMirror 6 + @codemirror/vim (primary); hand-roll
  dropped
status: accepted
created: '2026-07-14'
decision_makers:
- Takehito Gondo
tags:
- workspace-editor
- design
owners: []
relations:
- {type: partOf, target: plan-20260714-agent-workspace-editor}
source_paths: []
summary: Mutation-capable editor is CodeMirror 6 + @codemirror/vim (primary); hand-roll
  dropped
updated: '2026-07-14'
---

## Context

decision-input-editor-lib-codemirror-vim の 3 候補 (CodeMirror 6 + @codemirror/vim / hand-rolled workspaceVimKeymap 拡張 / Monaco + monaco-vim) を評価する必要がある。hand-roll は undo/redo stack と register semantics を無から実装するリスクを持ち、Monaco はバンドルサイズと vim layer の保守状況で劣る。

## Decision

**CodeMirror 6 + @codemirror/vim** を primary editor engine に確定する。rope-based document、成熟した @codemirror/vim (operator/motion/register/undo が既に閉じている)、明示的な EditorState.lineSeparator による改行 fidelity を根拠とする。hand-roll 拡張は undo/redo/register の観察不変性 evidence を counterexample-backed に示せないため drop。Monaco はサイズと保守状況で fallback すら不要。

## Consequences

- 新規 frontend 依存 (@codemirror/state, @codemirror/view, @codemirror/vim) を導入する。AGENTS.md の新規依存立証責任は本 ADR の accept を根拠に満たされる。
- 既存 workspaceVimKeymap.ts の motion/search reducer は CodeMirror bindings 経由に置き換える (viewer 側の read-only 挙動は互換維持)。
- large-file 編集性能 (contract-editor-large-file-editing-performance) は rope structure に依存する。

## Alternatives

- **却下: 既存 hand-rolled reducer を拡張** — undo/redo stack と register semantics を無から実装するコストとリスクが CodeMirror の counterexample-backed evidence と釣り合わない。
- **却下: Monaco + monaco-vim** — バンドルサイズが CodeMirror 6 より重く、vim layer は core でなくサードパーティ。fallback として保つ必然性がない。

## Trace

- Requirements: FR-101, FR-104, FR-109, FR-110, NFR-102
- Implementation contracts: contract-vim-mutation-editing, contract-vim-undo-locality, contract-editor-large-file-editing-performance, contract-encoding-newline-fidelity
