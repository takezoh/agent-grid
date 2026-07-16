---
id: adr-20260624-0063-toast-single-live-and-undosnackbar
kind: adr
title: ADR 0063 — NotificationToast 単一 aria-live 化 + 3 region 独立 + safe-area + UndoSnackbar
  吸収
status: proposed
created: '2026-06-24'
updated: '2026-07-04'
tags:
- adr
- legacy-import
owners: []
relations:
- {type: references, target: adr-20260624-0057-palette-single-aria-live-slot}
- {type: references, target: change-20260625-2026-06-25-web-ui-redesign}
source_paths:
- src/client/web/src/
decision_makers:
- unknown
summary: '既存 NotificationToast は各 item が個別 <output aria-live=''polite''> を持ち、position:
  fixed; top:16px; right:16px、inline ハードコード配色、5s auto-dismiss、最大 3 件。ADR-0057 が palette
  で単一 aria-live slot を確立済み。ux.md は『コンテナ 1 つに集約 + bottom 寄せ +'
---

<!-- migrated_from: docs/adr/0063-toast-single-live-and-undosnackbar.md -->

# ADR 0063 — NotificationToast 単一 aria-live 化 + 3 region 独立 + safe-area + UndoSnackbar 吸収

Status: Proposed

Related: [spec](../changes/change-20260625-2026-06-25-web-ui-redesign/requirements.md), [plan](../changes/change-20260625-2026-06-25-web-ui-redesign/implementation.md), [ux](../changes/change-20260625-2026-06-25-web-ui-redesign/ux.md), extends [0057](adr-20260624-0057-palette-single-aria-live-slot.md)
Related requirements: FR-TOAST-001, FR-TOAST-002, FR-TOAST-003, FR-PALETTE-FAIL-001, FR-DRAWER-003, FR-DRAWER-004

## Context

既存 NotificationToast は各 item が個別 <output aria-live='polite'> を持ち、position: fixed; top:16px; right:16px、inline ハードコード配色、5s auto-dismiss、最大 3 件。ADR-0057 が palette で単一 aria-live slot を確立済み。ux.md は『コンテナ 1 つに集約 + bottom 寄せ + safe-area + token 化』『'Switched to <label>' Undo』を要求。否定役は『live region 内に Undo button (interactive) を置くと AT が読み上げない/focus できない懸念 + Undo announce と palette inline error の同 slot 上書き race』を指摘した。最適化役は『UndoSnackbar を NotificationToast コンテナに吸収 + LiveStatusRegion プリミティブで palette InlineStatus と共有』を推奨した。

## Decision

(1) NotificationToast コンテナに aria-live='polite' role='status' を 1 つ集約 (passive 通知用)。item の aria-live は削除。
(2) UndoSnackbar を NotificationToast コンテナ内に吸収しつつ aria-live slot は 3 系統 (passive 通知 / UndoSnackbar status / palette InlineStatus = ADR-0057) を独立 region 化して race を排除。
(3) UndoSnackbar 内では status テキストを live region (内側 div) に置き、Undo button (interactive) を live region 外の兄弟 wrapper に配置する。
(4) inline 配色を tokens.css の --toast-bg-info / --toast-bg-success / --toast-bg-warn / --toast-bg-error 参照に置換。
(5) <768px では bottom 寄せ (position: fixed; bottom: calc(env(safe-area-inset-bottom) + 16px))、>=768px は legacy 互換で top-right に置く。
(6) 5s auto-dismiss + tap dismiss + 最大 3 件は維持。

## Consequences

- positive: ADR-0057 の方針が toast 全体へ波及 (二重読み上げ防止)。
- positive: UndoSnackbar が独立 UI でなくなり components 数が 1 件削減。
- positive: 3 region 独立で UndoSnackbar announce と palette inline error の race が構造的に消える。
- negative: 3 region 独立は HTML 構造が複雑化 (兄弟 wrapper が 3 つ並ぶ)。テストで 3 region の独立性 (相互に override しない) を観察する必要がある。
- negative: Undo button を live region 外に置くため AT は『status』と『Undo』を別 announce で読み上げる (UX として Undo の意図が status と組で伝わるかは後続評価)。

## Alternatives Considered

### UndoSnackbar を別 UI として SessionDrawer 関心領域に閉じ込める (Material 慣習)

aria-live が toast と 2 系統に分かれ単一 slot 方針が分散。

### UndoSnackbar を live region 内に Undo button ごと置く

AT で Undo button が読み上げ/focus できない懸念 + interactive control の live region 配置慣習が弱い。

### NotificationToast item に aria-live='polite' を残す

多重読み上げが起きる。ADR-0057 方針と矛盾。
