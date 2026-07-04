---
id: adr-20260624-0059-design-token-and-theme-bridge
kind: adr
title: ADR 0059 — Design token 体系 + light/dark/system テーマ + xterm 連動 (ハイブリッド bridge)
status: proposed
created: '2026-06-24'
updated: '2026-07-04'
tags:
- adr
- legacy-import
owners: []
relations: []
source_paths: []
decision_makers:
- unknown
---

<!-- migrated_from: docs/adr/0059-design-token-and-theme-bridge.md -->

# ADR 0059 — Design token 体系 + light/dark/system テーマ + xterm 連動 (ハイブリッド bridge)

Status: Proposed

Related: [spec](../specs/2026-06-25-web-ui-redesign/spec.md), [plan](../specs/2026-06-25-web-ui-redesign/plan.md), [ux](../specs/2026-06-25-web-ui-redesign/ux.md)
Related requirements: FR-THEME-001, FR-THEME-002, FR-THEME-003, FR-THEME-004, FR-THEME-005, FR-THEME-006, FR-THEME-007, FR-TOKEN-001, FR-TOKEN-002, FR-FRAMEWORK-001

## Context

既存 web client は css/{app.css, view.css} の 2 ファイルで、:root に --bg:#1e1e1e/--fg/--accent/--warn のみ、メディアクエリ 0 件、ダーク固定。xterm.js は options.theme へハードコード色を渡している。app.css は 469 行で 500 行制約に逼迫している。ux.md は light/dark/system の 3 テーマと xterm 配色連動 (UAC-005/006/007) を must-fail で要求し、否定役は『xterm DOM 要素の getComputedStyle.backgroundColor は xterm が書き込む inline style を返すため、ITheme オブジェクト経由の単独実装では UAC-006 Then literal「同一 CSS custom property を読む path で」を満たせない』と指摘した。

## Decision

(1) tokens.css を新設し semantic CSS custom property 階層を集約する (NFR-CSS-001 を tokens.css + app.css + view.css の 3 ファイルに改定)。
(2) ThemeProvider が data-theme 属性で document.documentElement を切替え、localStorage キー 'agent-reactor-theme' に永続化する (system 選択時は削除、無効値は system にフォールバック)。
(3) xterm との連動は『ハイブリッド bridge』を採る: xterm 要素の DOM background は CSS で background: var(--bg) を直接付与し、xterm.options.theme には fg / cursor / selection / 黒系のみを ITheme オブジェクトで渡す (背景は透明にし CSS の bg を見せる)。ITheme の値は ThemeProvider が useLayoutEffect + 1 rAF guard で getComputedStyle で tokens を読み構築する。
(4) muted は opacity でなく独立 token (--fg-muted) で表現する。
(5) WCAG AA は token 定義時に実測値で割当てる。

## Consequences

- positive: UAC-006 Then literal 『同一 CSS custom property を読む path で』が CSS 経路 (xterm DOM bg) と JS 経路 (ITheme fg/cursor) の二重充足になる。FR-THEME-003 の観察可能性が rAF 単位で明確。
- positive: tokens.css 分離で app.css の 500 行制約 (NFR-FILESIZE-001 / NFR-CSS-001) に余裕が生まれる。
- positive: 既存 ADR-0019 (stack) / ADR-0029 (terminal-host flex height) を侵害しない (高さ計算は別 ADR-0060 で扱う)。
- negative: xterm の内部背景描画と CSS background が異なる経路で解決される (透明 ITheme + CSS bg) ため、xterm.js のバージョンアップで透明 ITheme 挙動が変わるリスク (検証は MVP テストで吸収)。
- negative: ThemeProvider の getComputedStyle 呼び出しがテーマ切替時に 1 回走るため、テスト環境 (happy-dom) で getComputedStyle が空文字を返すケースの fallback コードが必要 (ITheme の各 field に default 値を持つ)。

## Alternatives Considered

### 案A 純粋: ITheme 経由のみ (CSS background var() を xterm DOM に付与しない)

UAC-006 Then literal『同一 CSS custom property を読む path で』を xterm 内部の inline style 経路で満たせない (否定役 blocker 指摘)。

### 案B MutationObserver で CSS custom property 変化を監視し ITheme を再構築

発火頻度が読みにくくテスト容易性が下がる。useLayoutEffect で data-theme を購読する案A 派生で十分。

### 案C 純粋: xterm DOM に CSS のみ適用し ITheme は固定 (透明)

cursor 色 / selection 色は ITheme でしか渡せず ITheme 構築は必須。

### native <dialog> showModal() で palette を作り直し theme を palette ADR-0036 に集約

ux.md で rejected。palette 内部状態機械を作り直さない方針に反する。
