---
id: adr-20260714-editor-root-disappearance-degrades-save
kind: adr
title: Root disappearance transitions to read-only + clipboard export (save requirement
  degrades explicitly)
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
summary: Root disappearance transitions to read-only + clipboard export (save requirement
  degrades explicitly)
updated: '2026-07-14'
---

## Context

issue-root-disappears-mid-edit-undefined が指摘した通り、workspace root がセッション中に削除/rename された場合の dirty buffer 保存経路が未定義。無データ損失を保つには明示的な degradation path が必要。

## Decision

workspace root disappearance を **failure_recovery** dimension に持ち上げ、drawer は (a) dirty buffer を破棄せず memory に保持、(b) 強制 close せず read-only 化 + typed `root_disappeared` banner、(c) `clipboard export` action を surface する。FR-113 は 'silent 破棄を禁止する unwanted requirement' として書かれる (save 完了は要求されない — degrades to preserved-in-memory)。

## Consequences

- dirty buffer の losses が構造的に発生しない。
- operator は clipboard 経由で dirty content を回復できる (外部への一度きり出口)。
- failure_semantics.requirement_effect=degrades / approval_ref を本 ADR に持たせ、schema の明示承認要件を満たす。

## Alternatives

- **却下: 強制 drawer close + dirty buffer 破棄** — operator のデータ損失が silent に発生する — 最悪 UX。

## Trace

- Requirements: FR-113
- Implementation contracts: contract-workspace-root-disappearance
