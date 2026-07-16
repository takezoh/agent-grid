---
id: adr-20260624-0043-palette-createsessionform-replacement
kind: adr
title: ADR 0043 — CreateSessionForm は本 spec 内で同時撤去する (1 PR / 3 commit phase)
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
summary: Spec 要件として『CreateSessionForm を撤去しパレット new-session に一本化』と明記されている。共存させると挙動差テストと
  UX 混乱を生む。
---

<!-- migrated_from: docs/adr/0043-palette-createsessionform-replacement.md -->

# ADR 0043 — CreateSessionForm は本 spec 内で同時撤去する (1 PR / 3 commit phase)

Status: Accepted

Related: [spec](../changes/change-20260624-2026-06-24-web-ui-command-palette/requirements.md), [plan](../changes/change-20260624-2026-06-24-web-ui-command-palette/implementation.md)
Related requirements: FR-021

## Context

Spec 要件として『CreateSessionForm を撤去しパレット new-session に一本化』と明記されている。共存させると挙動差テストと UX 混乱を生む。

## Decision

本 spec の plan 内で同時撤去するが、commit phase は (F1) palette shell + Header 配線、(F2) CreateSessionForm 削除 + App.test.tsx 書き換え + session-config 拡張、(F3) push route 追加、の 3 段に分けてレビュー可能とする。chunks の f1/f2/f3 はこの順序を踏襲する。

## Consequences

- **positive**: 機能重複期間ゼロ、UX 混乱なし
- **positive**: F1/F2/F3 のレビュー粒度が小さく、回帰時 bisect が容易
- **negative**: PR 内の diff 量は大きい (ただし plan-impl の commit 分割で吸収)

## Alternatives Considered

### 段階撤去 (CreateSessionForm を残しつつパレット投入)

却下: 機能重複期間に挙動差テスト負担が出る

### CreateSessionForm を残し共存

却下: spec の一本化要件に反する
