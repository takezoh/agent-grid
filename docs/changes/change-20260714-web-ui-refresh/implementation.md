---
change: change-20260714-web-ui-refresh
role: implementation
---

# Implementation

## Legacy Source (verbatim)

````markdown
---
id: plan-20260714-web-ui-refresh
kind: plan
title: Web UI Refresh — Implementation Plan
status: draft
created: '2026-07-14'
goal: spec-20260714-web-ui-refresh の FR-001..037 を、独立に出荷可能な 6 マイルストーン (m1 トークン →
  m2 シェル / m3 Changes / m5 パレット 並行 → m4 メインビュー → m6 ドロワー+仕上げ) で実装し、各境界で lint / unit
  / e2e green と両テーマのスクリーンショット確認を通す。
scope_in:
- src/client/web/src/css/ (全 CSS)
- src/client/web/src/components/ (view 層)
- src/client/web/src/App.tsx (composition)
- 新規 runtime 依存 2 パッケージ (adr-20260714-behavior-lib-and-icons)
scope_out:
- src/wire/ / Go 側全部
- 挙動実装 (useFocusTrap / UnifiedListbox / ConfirmDialog / palette フェーズ遷移)
- 保存安全機構 (performSave の中身・API 層)
- ストアロジック (daemon / workspaceActivity / palette)
milestones:
- {id: m1, title: トークン体系 + タイポグラフィ (FR-001..007), status: todo}
- {id: m2, title: シェル + サイドバー + ヘッダ (FR-008..014), status: todo}
- {id: m3, title: Changes パネル / シート — rail 解体 (FR-015..021), status: todo}
- {id: m4, title: メインビュー — DriverViewPanel 解体 + タブ + ステータスバー (FR-022..025), status: todo}
- {id: m5, title: コマンドパレット刷新 (FR-026..030), status: todo}
- {id: m6, title: ワークスペースドロワー + エディタークローム + モーション仕上げ (FR-031..037), status: todo}
contracts:
- agent-grid-theme localStorage キーのセマンティクス (set on light/dark, remove on system) を維持する
- palette-input / palette-param-* / dirty-indicator 等の data-testid を維持する (変更時は同一チャンク内でテスト更新)
- ARIA 契約 (tabs manual activation / dialog roles / focus trap) を維持する
- performSave / workspaceActivity ストアのシグネチャを変更しない
- e2e smoke が依存する aria-label (Open command menu 等) を維持する
- wire 型 (src/wire/*) に触れない
tags:
- plan
- web
- ui-refresh
owners: []
relations:
- {type: implements, target: spec-20260714-web-ui-refresh}
source_paths:
- src/client/web/src/
summary: 依存順マイルストーン m1..m6 (トークン → シェル/Changes/パレット並行 → メインビュー → ドロワー+仕上げ) と互換性契約。
---

## Goal

frontmatter `goal` を参照。全チャンク共通の完了条件: `npm run lint` / `npm run test:unit` / `npm run test:e2e` green、両テーマのスクリーンショット確認 (survey spec を `e2e/screenshots.spec.ts` として opt-in 恒久化して使用)、frontmatter `contracts` の維持。

## Dependency-ordered Chunks

```
m1 ──┬── m2 ──── m4
     ├── m3 ──┐
     ├── m5   ├── m6
     └────────┘
```

m2 / m3 / m5 は m1 完了後に並行着手可能。m4 は m2 の後 (ヘッダが受け皿)。m6 は m3 の後 (導線が ChangesPanel に移る)。

### m1 — トークン体系 + タイポグラフィ (FR-001..007)

- **実装:** `tokens.css` 全面書き換え (primitive + semantic の 2 層化、新パレット)。`app.css` に `--font-ui` / `--font-mono` の適用ルール。`--session-status-*` 消費側 (`StatusIcon.tsx` / `SessionList.tsx` / `session-list.css`) の移行。色リテラル静的ガード追加。xterm `ITheme` 値の更新 (`ThemeProvider.tsx`)。
- **files_touched:** `src/css/tokens.css`, `src/css/app.css`, `src/css/*.css` (alias 追従のみ), `src/components/StatusIcon.tsx`, `src/components/ThemeProvider.tsx`, `src/__tests__/tokens-css-structure.test.ts` (2 層検証へ書き直し), 新規 `src/__tests__/no-color-literals.test.ts`
- **テスト:** tokens-css-structure (2 層検証) / no-color-literals / status 色 × 両テーマの contrast 検証
- **リスク:** semantic alias の漏れで一部だけ旧色が残る → ガードテストとスクリーンショットで検出

### m2 — シェル + サイドバー + ヘッダ (FR-008..014)

- **実装:** `AppShell` グリッド改修 (banner/header/sidebar/main 構造は維持)。`SessionList` 行の再構成 (選択=塗り / フォーカス=リング、メタ行)。ブランド行 + Cmd/Ctrl+K トリガー移設。ヘッダのパンくず + pill + アイコンアクション化。`ThemeSegmentedControl` → overflow メニュー内へ (Radix DropdownMenu 導入)。`<Icon>` 基盤 + lucide SVG 初期セット。
- **files_touched:** `AppShell.tsx`, `SessionList.tsx`, `CommandSearchTrigger.tsx`, `ThemeSegmentedControl.tsx` (改廃), 新規 `HeaderBar.tsx` / `OverflowMenu.tsx` / `components/icons/*`, `css/shell.css`, `css/session-list.css`, `App.tsx`, 各テスト
- **テスト:** AppShell / SessionList / Theme 系テストの更新。メニュー経由テーマ切替の localStorage 互換テスト (UAC-005)
- **リスク:** e2e smoke が `Open command menu` aria-label に依存 → ラベル維持で吸収

### m3 — Changes パネル / シート — rail 解体 (FR-015..021)

- **実装:** `ChangesPanel.tsx` / `ChangesSheet.tsx` 新規。`ActivityRail.tsx` 削除。`App.tsx` の main 構成変更 (rail 列 → terminal + panel の 2 列 grid)。collapse 永続化 (`web.changes.collapsed`)。モバイルでターミナル全幅。
- **files_touched:** `components/workspace/ActivityRail.*` (削除), 新規 `ChangesPanel.tsx` / `ChangesSheet.tsx` + テスト, `App.tsx`, `css/workspace.css`
- **テスト:** ストア契約 (selectTurnRows / openDrawerFromRow 呼び出し) を新 view で再検証。collapse 永続化。<768px で rail 由来の縦帯 DOM が存在しないこと (UAC-009 の否定形)
- **リスク:** モバイル既存スクリーンショット・e2e の基準変更が大きい → チャンク内で基準更新

### m4 — メインビュー (FR-022..025)

- **実装:** `DriverViewPanel` の解体と `HeaderBar` への統合。`SessionTerminateButton` のアイコン化 (Radix Tooltip 対)。`MainTabs` の下線タブ化。`StatusBar.tsx` 新規 (status_line + 接続状態)。
- **files_touched:** `DriverViewPanel.tsx` (解体), `SessionTerminateButton.tsx`, `MainTabs.tsx`, `RunStateBadge.tsx`, 新規 `StatusBar.tsx`, `css/view.css`, `App.tsx`, 各テスト
- **テスト:** ConfirmDialog フロー不変 (UAC-011)。ARIA tabs manual activation 維持 (既存テスト流用)。status_line 表示位置
- **リスク:** DriverViewPanel のテストが panel 存在前提 → ヘッダ統合版に書き直し (責務は同一)

### m5 — コマンドパレット刷新 (FR-026..030)

- **実装:** 器の DOM/CSS 再構成 (入力 + セクション + フッタ)。`InlineStatus` の常設廃止 → notifications store 経由トースト + announcer 維持。無効 push の減光 + 理由。worktree トグルのフッタ化。フェーズ遷移・listbox・fuzzy は無改変。
- **files_touched:** `palette/CommandPalette.tsx`, `palette/ToolSelectPhase.tsx`, `palette/ParamSelectPhase.tsx`, `palette/InlineStatus.tsx` (改廃), `palette/ActiveContextHeader.tsx` (フッタ化), `store/palette_inline_status.ts` (出力先), `css/palette.css`, 各テスト
- **テスト:** `palette-input` / `palette-param-*` testid 維持で既存 e2e smoke がそのまま通ること。announcer テスト (UAC-014)
- **リスク:** inline status に依存する a11y テストの改修範囲 → announce 経路は維持するため差分は表示位置のみ

### m6 — ワークスペースドロワー + エディタークローム + モーション仕上げ (FR-031..037)

- **実装:** ドロワーの再配置 (ヘッダ下・右スライド)。下線タブ統一。標準ツリー化。Save ボタン + Cmd/Ctrl+S (既存 `performSave` 呼び出しのみ)。read-only 理由バナー。CodeMirror / Diff / xterm のトークン接続。`--motion-*` の全遷移適用と reduced-motion ガード拡張。両テーマ総点検。
- **files_touched:** `workspace/WorkspaceDrawer.tsx`, `workspace/WorkspaceTree.tsx`, `workspace/FileViewer.tsx`, `workspace/DiffViewer.tsx`, `css/workspace.css`, `__tests__/reduced-motion-guard.test.ts`, 各テスト
- **テスト:** 保存フロー既存テスト (mtime / stale / allowlist / dirty 保持) が無改変で green。Save ボタン状態遷移 (UAC-018)。Cmd+S スコープ (UAC-018 反例)。e2e `workspace/*.spec.ts` 継続
- **リスク:** ドロワー再配置で focus trap の境界が変わる → `useFocusTrap` は流用しトラップ根要素だけ差し替え

## Verification

| 対象 | コマンド | 判定 |
|---|---|---|
| lint | `cd src/client/web && npm run lint` | exit 0 |
| unit | `cd src/client/web && npm run test:unit` | 全 pass |
| e2e | `cd src/client/web && npm run test:e2e` | 全 pass |
| スクリーンショット | `npx playwright test e2e/screenshots.spec.ts` (opt-in) | 両テーマ + モバイルを目視で before/after 確認 |

## Compatibility Matrix

frontmatter `contracts` が SoT。退行防止の要点:

| 契約 | 維持方法 |
|---|---|
| `agent-grid-theme` localStorage | ThemeProvider の STORAGE_KEY / 削除セマンティクス無改変 (FR-012) |
| `palette-*` data-testid | m5 で DOM 再構成しても testid は同名で移植 (FR-029) |
| ARIA: tabs manual activation / dialog / focus trap | 挙動実装を流用し CSS のみ差し替え (FR-025, FR-031) |
| 保存安全機構 | `performSave` と API 層無改変、呼び出し元を追加するのみ (FR-034/035) |
| workspaceActivity ストア契約 | view のみ差し替え、selector/action シグネチャ不変 (FR-015/016) |
| wire 型 | 触らない |

````
