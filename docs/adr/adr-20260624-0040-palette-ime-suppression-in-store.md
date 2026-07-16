---
id: adr-20260624-0040-palette-ime-suppression-in-store
kind: adr
title: ADR 0040 — IME composition 抑止は store.composing フラグ 1 箇所に集約する
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
summary: Enter / ↑↓ / Ctrl+P/N の各ハンドラで個別に composition 判定すると分散しテストカバレッジが落ちる。
---

<!-- migrated_from: docs/adr/0040-palette-ime-suppression-in-store.md -->

# ADR 0040 — IME composition 抑止は store.composing フラグ 1 箇所に集約する

Status: Accepted

Related: [spec](../changes/change-20260624-2026-06-24-web-ui-command-palette/requirements.md), [plan](../changes/change-20260624-2026-06-24-web-ui-command-palette/implementation.md)
Related requirements: FR-019

## Context

Enter / ↑↓ / Ctrl+P/N の各ハンドラで個別に composition 判定すると分散しテストカバレッジが落ちる。

## Decision

input 要素の onCompositionStart/onCompositionEnd を store.setComposing(boolean) に直接紐付け、key handler は store.composing を見て early-return する 1 箇所集約パターン。専用 hook (useImeComposition) は作らない。

## Consequences

- **positive**: IME 抑止テストが『store.composing と key handler』の組合せ単体で完結
- **positive**: Single source of truth
- neutral: composition イベントを発するブラウザの差異 (古い Safari) は別途検証

## Alternatives Considered

### 各 input ハンドラで isComposing 引数を見る

却下: 分散するため漏れリスク

### 専用 useImeComposition hook を作る

却下: state 所有者が増え hooks ファイルが 1 つ増える
