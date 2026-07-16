---
id: adr-20260624-0038-palette-fuzzy-pure-function
kind: adr
title: ADR 0038 — ファジー検索は依存ゼロの純関数 (lib/fuzzy.ts) で実装する
status: accepted
created: '2026-06-24'
updated: '2026-07-04'
tags:
- adr
- legacy-import
owners: []
relations:
- {type: references, target: change-20260624-2026-06-24-web-ui-command-palette}
source_paths: []
decision_makers:
- unknown
summary: ツール候補数は最大でも数十程度、自由入力 options のフィルタも軽量。fuse.js などのライブラリを新規導入すると bundle と維持コストが増える。
---

<!-- migrated_from: docs/adr/0038-palette-fuzzy-pure-function.md -->

# ADR 0038 — ファジー検索は依存ゼロの純関数 (lib/fuzzy.ts) で実装する

Status: Accepted

Related: [spec](../changes/change-20260624-2026-06-24-web-ui-command-palette/requirements.md), [plan](../changes/change-20260624-2026-06-24-web-ui-command-palette/implementation.md)
Related requirements: FR-008

## Context

ツール候補数は最大でも数十程度、自由入力 options のフィルタも軽量。fuse.js などのライブラリを新規導入すると bundle と維持コストが増える。

## Decision

fuzzyRank<T>(items, query, getText): Array<{item, score, ranges}> という generic 純関数を lib/fuzzy.ts に新設し、連続一致優先 + マッチ位置返却の挙動を sahilm/fuzzy と揃える。~50 行で依存ゼロ。

## Consequences

- **positive**: bundle 増加ゼロ
- **positive**: ToolSelectPhase / 自由入力 ParamField の両方で再利用可能
- **positive**: 純関数テストで挙動を完全に固定できる
- **negative**: sahilm/fuzzy の細部 (token 分割) を Web で再実装するメンテ負荷を負う

## Alternatives Considered

### fuse.js 導入

却下: bundle 増

### string includes のみで filter

却下: ハイライト ranges が出せず連続一致優先も失う
