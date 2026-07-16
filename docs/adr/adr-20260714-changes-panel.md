---
id: adr-20260714-changes-panel
kind: adr
title: ActivityRail の解体と Changes パネル/シート
status: deprecated
created: '2026-07-14'
decision_makers:
- Takehito Gondo
tags:
- adr
- web
- ui-refresh
owners: []
relations:
- {type: references, target: change-20260714-web-ui-refresh}
source_paths:
- src/client/web/src/components/workspace/ActivityRail.tsx
- src/client/web/src/store/workspaceActivity.ts
summary: rail を廃止し desktop は右の折りたたみパネル、mobile は bottom sheet。workspaceActivity store
  は無改変で view のみ差し替え。
updated: '2026-07-14'
---

## Context

ActivityRail はメイン列左の常駐縦帯で、(a) hardcoded 濃色でライトテーマ非追従、(b) モバイル (390px) で画面幅の過半を占有しターミナルを約 170px の帯に圧縮 (実質使用不能)、(c) ラベル・境界の意味が無くカード状の行が浮いて見える。一方、供給元の `workspaceActivity` ストア (turn_row 集約・mid_turn_touch・drawer 連携・transportDegraded) は健全でテストも厚い。UI/UX 刷新提案 (2026-07-14) のサーベイで最重要問題と判定された。

## Decision

view 層のみ差し替える。デスクトップ (>=1024px) はターミナル右の折りたたみ可能な `ChangesPanel` (~216px、`--bg-surface`)、モバイル (<768px) は画面下端の `ChangesSheet` (bottom sheet、既存ドロワーのつまみ/ジェスチャ実装を流用)。タブレット (768–1023px) はデスクトップ形を踏襲し折りたたみ既定 ON。ストア・イベント意味論・`openDrawerFromRow` 契約は無改変。折りたたみ状態は `usePersistedValue` (key: `web.changes.collapsed`) で永続化し、畳み中も件数バッジ付きハンドルで存在に気づける。

## Consequences

- (+) モバイルのターミナルが全幅を取り戻す (刷新の最大の実利)。ライトテーマ非追従も器の作り直しで同時に解消。
- (+) ストア無改変のため、アクティビティ集約ロジックとそのテストは無リスクで温存される。
- (-) `ActivityRail.test.tsx` は `ChangesPanel` / `ChangesSheet` のテストへ書き直し。モバイル e2e スクリーンショットの基準画像更新が必要。
- (0) bottom sheet の展開ジェスチャは既存 drawer 実装の流用範囲に留め、新規ジェスチャ体系は導入しない。

## Alternatives

- **却下: rail を改修して残す (テーマ対応 + 幅調整)** — モバイルの占有問題が本質的に解決しない。何 px であってもターミナルを侵食する。
- **却下: Changes を MainTabs のタブにする** — ターミナルと排他表示になり、「出力を見ながら変更を追う」ユースケースを失う。
- **却下: ドロワー内のみに集約 (常設 UI なし)** — 変更の存在に気づく導線が消える。折りたたみハンドル + 件数バッジが妥協点。

## Trace

- Requirements: FR-015..FR-021, NFR-005
- UX: UAC-007..UAC-009


{% transition from="proposed" to="accepted" date="2026-07-14" %}
実装済み (rail 解体時点の決定として受理)
{% /transition %}


{% transition from="accepted" to="deprecated" date="2026-07-14" %}
2026-07-14 ユーザー意思決定: 常設 ChangesPanel/Sheet は Workspace/Terminal モード UX の残骸となるため廃止。Changes は Workspace モードの右パネルへ統合 (spec FR-015 改訂 x2)
{% /transition %}
