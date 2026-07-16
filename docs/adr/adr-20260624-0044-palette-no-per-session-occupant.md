---
id: adr-20260624-0044-palette-no-per-session-occupant
kind: adr
title: ADR 0044 — Go proto SessionInfo / TS wire SessionInfo に per-session occupant
  を追加しない
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
summary: 初稿は SessionInfo に Occupant ('main'|'log'|'frame') を追加して push スコープ可否判定に使う設計だったが、否定役指摘の通り
  (1) daemon に session 単位の occupant モデルは存在しない (現行は state.State.ActiveOccupant という
  global 値のみ)、(2) viewUpdateFrame は activeSessionID
---

<!-- migrated_from: docs/adr/0044-palette-no-per-session-occupant.md -->

# ADR 0044 — Go proto SessionInfo / TS wire SessionInfo に per-session occupant を追加しない

Status: Accepted

Related: [spec](../changes/change-20260624-2026-06-24-web-ui-command-palette/requirements.md), [plan](../changes/change-20260624-2026-06-24-web-ui-command-palette/implementation.md)
Related requirements: FR-004, FR-005, FR-006

## Context

初稿は SessionInfo に Occupant ('main'|'log'|'frame') を追加して push スコープ可否判定に使う設計だったが、否定役指摘の通り (1) daemon に session 単位の occupant モデルは存在しない (現行は state.State.ActiveOccupant という global 値のみ)、(2) viewUpdateFrame は activeSessionID をドロップする規約 (ADR-0023) で session-scoped occupant を載せると意味論が衝突、(3) push は『現在 active な session に対する操作』なので global active + global occupant で判定できる。

## Decision

SessionInfo proto / wire への occupant フィールド追加を中止し、push 可否判定は既存の daemon-global ActiveSessionID + ActiveOccupant ('main'|'log'|'frame') を入力源とする。store/daemon は既存の RespSessions / EvtSessionsChanged 経路でこれらを既に保持しているので追加の wire 改修は不要。

## Consequences

- **positive**: proto / wire の鏡像改修ゼロ、既存 consumer (claude-app-server 等) への影響なし
- **positive**: viewUpdateFrame の activeSessionID ドロップ規約 (ADR-0023) と衝突しない
- **positive**: PR diff と回帰リスクが大幅縮小
- **negative**: 複数 web client が同じ daemon-global occupant を参照するため、ある web client の押下が他 web client の push 可否シグナルと連動する。これは push が常に daemon-global active を対象にする仕様 (FR-025) と整合

## Alternatives Considered

### proto SessionInfo に Occupant string を追加 (初稿 Q1 案 A)

却下: daemon に source データが無く空文字 omitempty しか出せない、wire 既存規約と衝突、consumer 影響範囲が広い

### view.View に session 単位 occupant をネスト

却下: viewUpdateFrame の activeSessionID ドロップ規約と矛盾

### 別 wire フレーム (session-occupant-changed) を新設

却下: ADR-0030 (WS は I/O 専用 / 既存 view-update 経路に乗せる) 方針に反する
