---
id: plan-20260625-2026-06-25-web-ui-redesign
kind: plan
title: Plan — Web UI 全面刷新
status: draft
created: '2026-06-25'
updated: '2026-07-04'
tags:
- plan
- legacy-import
owners: []
relations: []
source_paths: []
goal: Plan — Web UI 全面刷新
scope_in: []
scope_out: []
milestones: []
contracts: []
---

<!-- migrated_from: docs/specs/2026-06-25-web-ui-redesign/plan.md -->

# Plan — Web UI 全面刷新

- **spec**: [spec.md](../../specs/2026-06-25-web-ui-redesign/spec.md)
- **ux**: [ux.md](../../specs/2026-06-25-web-ui-redesign/ux.md) (`ux-2026-06-25-web-ui-redesign`)
- **ADRs**: [0059](../../adr/adr-20260624-0059-design-token-and-theme-bridge.md), [0060](../../adr/adr-20260624-0060-adaptive-layout-and-drawer.md), [0061](../../adr/adr-20260624-0061-apg-tabs-manual-activation.md), [0062](../../adr/adr-20260624-0062-search-bar-trigger-and-palette-theme-entry.md), [0063](../../adr/adr-20260624-0063-toast-single-live-and-undosnackbar.md), [0064](../../adr/adr-20260624-0064-reduced-motion-single-guard.md)

## Goal

agent-reactor-new の Web client (src/client/web) UI/UX 全面刷新の実装計画を、ux.md (16 UAC / 4 flow / vs_legacy: must-fail 11 件) を起点に EARS 形式 FR と Nygard 4 節 ADR で固める。(1) 適応レイアウト (PC/タブレット/スマホ + dvh + safe-area)、(2) デザイントークン体系 + light/dark/system テーマ + xterm 配色連動、(3) palette で熟成済みの視覚言語と a11y パターン (unified listbox / focus trap / 単一 aria-live slot / disabled visible+skip-navigation / IME 抑止) を全画面 (session list / MainTabs / toast / DriverViewPanel) へ波及させる、を観察可能な振る舞いとして実装する。既存 keyboard/ARIA/IME 資産・palette 内部状態機械・store pureness・wire 型 stdlib・activeSessionID client 単独管理・ADR-0029/0030/0032/0033/0034 を退行させない。

## Components

| Component | Responsibility | Depends on |
|-----------|----------------|------------|
| tokens.css (src/client/web/src/css/tokens.css 新設) | semantic CSS custom property 階層 (--fg / --fg-muted / --bg / --bg-elevated / --accent / --status-running/-waiting/-idle/-stopped/-unknown / --row-radius / --row-padding-y / --row-padding-x / --row-font-size / --row-line-height / --row-min-height / --focus-ring / --xterm-fg / --xterm-cursor / --xterm-selection / --toast-bg-info/success/warn/error / --border-color) を :root に宣言。light/dark 各々 `[data-theme='light']` / `[data-theme='dark']` block で値割当 (各値は WCAG AA 実測値)。breakpoint と dvh/safe-area を CSS variable 化。muted は opacity でなく独立 token | — |
| ThemeStoreSlice (src/client/web/src/store/theme.ts) | theme: 'system' \| 'light' \| 'dark' を pure に保持。setTheme(value) アクションで state 更新のみ。永続化と data-theme 反映は ThemeProvider の effect 側で行う (store は DOM 非操作) | — |
| ThemeProvider (src/client/web/src/components/ThemeProvider.tsx) | useLayoutEffect で store.theme を購読し document.documentElement.dataset.theme を反映。useEffect で localStorage 永続化 (system 選択時は key 削除)。matchMedia('(prefers-color-scheme: dark)') の change を購読し theme=system のときのみ data-theme を切替。getComputedStyle で --xterm-fg / --xterm-cursor / --xterm-selection を読み ITheme を構築 (1 rAF guard) して TerminalPane に prop 伝播。localStorage 値が無効文字列なら system にフォールバック (FR-THEME-001 の data integrity) | tokens.css, ThemeStoreSlice |
| SegmentedControl primitive (src/client/web/src/components/primitives/SegmentedControl.tsx) | role='radiogroup' + role='radio' + aria-checked + roving tabindex + ArrowLeft/Right + Space/Enter (manual activation) の共通プリミティブ。palette/ChipSwitch.tsx から抽出 (薄い wrapper として ChipSwitch を再構成) | tokens.css |
| ThemeSegmentedControl (src/client/web/src/components/ThemeSegmentedControl.tsx) | SegmentedControl primitive を使い System/Light/Dark の 3 segment を render。スマホ狭幅では palette suggested action 'Theme: System / Light / Dark' に吸収 (ADR-0062) し、header に segmented control を表示しない | SegmentedControl primitive, ThemeStoreSlice |
| UnifiedListbox primitive (src/client/web/src/components/primitives/UnifiedListbox.tsx) | role='listbox' + aria-activedescendant + disabled-skip-navigation + IME 抑止フック + 行の共通 token (--row-*) class を提供。palette の ToolSelectPhase / ParamListbox / SessionList が import する。MVP は palette の既存 hook (components/palette/hooks/*) を共有可能な形に薄く抽出 | tokens.css |
| AppShell (src/client/web/src/components/AppShell.tsx, 既存 App.tsx を refactor) | grid-template-areas を breakpoint で切替える骨格。<768 = 'banner' 'header' 'main' / 768-1024 = 'banner banner' 'header header' 'sidebar main' (sidebar 折り畳み可) / >=1024 = 同形 (sidebar 常設)。100dvh + env(safe-area-inset-*) を root に適用。header 内に data-role='hamburger' button (CSS で <768px のみ visible) と CommandSearchTrigger を統合。drawer.open および previousActiveSessionId を useState で UI-local 保持。breakpoint 跨ぎ時の drawer close + focus 復帰 + 連続リサイズ idempotent guard を内包 (50ms debounce で過剰 fire 抑止) | tokens.css, SessionList, SessionDrawer, CommandSearchTrigger, ThemeSegmentedControl, DriverViewPanel, MainTabs, NotificationToast, UndoSnackbar |
| SessionDrawer (src/client/web/src/components/SessionDrawer.tsx) | スマホ幅 (<768) で SessionList を off-canvas wrapper として render。open 時は role='dialog' aria-modal='true' + focus trap (1 番目の focusable へ focus 移動) + 背後 main 領域に inert + aria-hidden='true' + pointer-events:none の三重 guard + scrim mount + slide-in transition (reduced-motion で抑制)。close 経路は『行 select (= 選択クローズ、open=false + AppShell に previousActiveSessionId を渡す)』『scrim click / Esc / 左→右スワイプ (touchstart→touchend で水平>=50 垂直<30 を SwipeHandler util で判定、= 取消クローズ、open=false のみ)』の 2 経路に分離 | SessionList, tokens.css |
| CommandSearchTrigger (src/client/web/src/components/CommandSearchTrigger.tsx) | AppShell header 内の検索バー風 button (虫眼鏡 icon + 'Search commands…' placeholder + 右端に ⌘K/Ctrl+K hint badge を isMacPlatform で出し分け)。<768px では header の幅 100%、>=768px では中央寄せ最大幅。click/tap で palette を open (既存 PaletteStoreSlice 経由)。New Session は palette 内 suggested action として吸収するため独立 button を持たない (ADR-0062) | tokens.css |
| UndoSnackbar (src/client/web/src/components/UndoSnackbar.tsx) | AppShell の previousActiveSessionId が非 null の間 visible。'Switched to <label>' テキスト (live region 内) + Undo button (live region 外、隣接 wrapper) を render。Undo click で AppShell から activeSessionID を直前値に戻す + previousActiveSessionId を null へ。5s で auto-dismiss (NotificationToast と同期、reduced-motion で抑制)。NotificationToast コンテナ内に隣接配置し、独立 aria-live slot (FR-TOAST-003 の 3 系統独立) を持つ | tokens.css, NotificationToast |
| NotificationToast (改修: src/client/web/src/components/NotificationToast.tsx) | コンテナ 1 つに aria-live='polite' role='status' (passive 通知用)。item の aria-live を削除。inline ハードコード配色を tokens.css の --toast-bg-* に置換。<768px は bottom 寄せ + env(safe-area-inset-bottom) + 16px、>=768px は top-right (legacy 維持) を CSS で切替。auto-dismiss (5s) + tap dismiss + 最大 3 件は維持。UndoSnackbar 用の独立 aria-live slot 領域を隣接で提供 (FR-TOAST-003) | tokens.css |
| MainTabs (改修: src/client/web/src/components/MainTabs.tsx) | WAI-ARIA APG Tabs Pattern に改修。roving tabindex + onKeyDown で ArrowLeft/Right (focus 移動のみ、manual activation) / Home/End (端) / Space/Enter (activate)。常時 mount + CSS 可視切替で TERMINAL の scrollback と xterm subscribe lifecycle を保持 (FR-TABS-003) | tokens.css |
| TerminalPane (改修: src/client/web/src/components/TerminalPane.tsx) | 高さ計算を 100vh → 100dvh + env(safe-area-inset-bottom) に置換。xterm + FitAddon + ResizeObserver + rAF coalesce refit + keyed remount (ADR-0029/0030/0034) は維持。xterm 要素の DOM に CSS で background: var(--bg) を直接適用 (UAC-006 Then の literal 充足経路)。xterm.options.theme prop を ThemeProvider 構築 ITheme で受領し data-theme 変化時に xterm に再適用 (FitAddon refit は ResizeObserver 任せで明示呼び出しは不要、ADR-0034 既存挙動) | ThemeProvider, tokens.css |
| SessionList (改修: src/client/web/src/components/SessionList.tsx) | UnifiedListbox primitive を使い role='listbox' に格上げ。aria-activedescendant ナビで disabled 行を skip (FR-TOKEN-002)。行の border-radius / padding / font-size / line-height / min-height は --row-* 参照に統一 (FR-TOKEN-001)。長文 displayLabel は 2 行 ellipsis (-webkit-line-clamp:2 + word-break:normal)、最小 44px 行高。displayLabel chain (ADR-0033) と status spinner (ADR-0032) は維持 | UnifiedListbox primitive, tokens.css |
| DriverViewPanel TagPill (改修: src/client/web/src/components/DriverViewPanel.tsx 内 TagPill) | src/client/web/src/util/contrast.ts (新設、約 30 行: srgbToLinear + relativeLuminance + contrastRatio) を呼び出し driver 提供 fg/bg の ratio を計算。4.5 未満なら fg を bg と高コントラスト側 (黒 or 白) に置換しさらに border: 1px solid currentColor を付与。computed ratio が >=4.5 になることをテストで観察 | tokens.css |
| reduced-motion guard (view.css 末尾 @media block) | @media (prefers-reduced-motion: reduce) の 1 ブロックに .run-state-spinner / .session-status-spinner / .drawer-slide / .palette-active-context--flash / .palette-listbox__row--flash / .undo-snackbar / 各種 transition を集約 (FR-MOTION-002) | tokens.css |
| contrast util (src/client/web/src/util/contrast.ts 新設) | srgbToLinear(channel) / relativeLuminance(rgb) / contrastRatio(fg, bg) を提供する stdlib JS 関数群 (約 30 行、依存なし) | — |
| SwipeHandler util (src/client/web/src/util/swipe.ts 新設) | touchstart/touchend を受け取り水平距離 / 垂直距離を返す純関数。SessionDrawer が左→右スワイプ判定 (水平>=50 垂直<30) に利用 (touch 限定 + 視覚的痕跡なし) | — |
| test-setup.ts 拡張 (src/client/web/src/test-setup.ts) | 既存の ResizeObserver / rAF mock に加え matchMedia mock を追加 (prefers-color-scheme / prefers-reduced-motion を切替可能にする setMatchMedia helper を globalThis に expose)。visualViewport mock は MVP では追加せず TerminalPane dvh テストは window.innerHeight 変動で代替 | happy-dom, vitest |
| Vitest test suite (src/client/web/src/**/__tests__/*) | 各 FR に 1 件以上のテストを追加。getComputedStyle / aria 属性 / focus / scrollWidth / getBoundingClientRect / matchMedia mock / DOM 属性 (inert / aria-hidden / pointer-events) で観察。FR-TOKEN-001 の CSS source 構造観察は fs.readFile + 正規表現で実装 | happy-dom, @testing-library/react, vitest.config |

## Build Sequence (chunks 依存順)

依存方向: `m1 → m2 → m3 → m4, m5 → m6`

### Chunk: `m1-token-and-test-infra`

- **Depends on**: (なし、起点)

- **Members**:
  - component:tokens.css
  - component:test-setup.ts 拡張
  - component:contrast util
  - req:FR-FRAMEWORK-001
  - req:FR-TOKEN-001
  - req:FR-TOKEN-002
  - adr: [0059-design-token-and-theme-bridge](../../adr/adr-20260624-0059-design-token-and-theme-bridge.md)

### Chunk: `m2-theme`

- **Depends on**: `m1-token-and-test-infra`

- **Members**:
  - component:ThemeStoreSlice
  - component:ThemeProvider
  - component:SegmentedControl primitive
  - component:ThemeSegmentedControl
  - component:TerminalPane (改修)
  - req:FR-THEME-001
  - req:FR-THEME-002
  - req:FR-THEME-003
  - req:FR-THEME-004
  - req:FR-THEME-005
  - req:FR-THEME-006
  - req:FR-THEME-007
  - req:FR-STORE-001
  - adr: [0059-design-token-and-theme-bridge](../../adr/adr-20260624-0059-design-token-and-theme-bridge.md)

### Chunk: `m3-adaptive-layout-and-drawer`

- **Depends on**: `m1-token-and-test-infra`, `m2-theme`

- **Members**:
  - component:AppShell
  - component:SessionDrawer
  - component:SwipeHandler util
  - component:UndoSnackbar
  - component:CommandSearchTrigger
  - req:FR-LAYOUT-001
  - req:FR-LAYOUT-002
  - req:FR-LAYOUT-003
  - req:FR-LAYOUT-004
  - req:FR-DRAWER-001
  - req:FR-DRAWER-002
  - req:FR-DRAWER-003
  - req:FR-DRAWER-004
  - req:FR-DRAWER-005
  - req:FR-DRAWER-006
  - req:FR-DRAWER-007
  - req:FR-PALETTE-TRIGGER-001
  - req:FR-WIRE-001
  - adr: [0060-adaptive-layout-and-drawer](../../adr/adr-20260624-0060-adaptive-layout-and-drawer.md)
  - adr: [0062-search-bar-trigger-and-palette-theme-entry](../../adr/adr-20260624-0062-search-bar-trigger-and-palette-theme-entry.md)

### Chunk: `m4-tabs-and-terminal-dvh`

- **Depends on**: `m1-token-and-test-infra`, `m3-adaptive-layout-and-drawer`

- **Members**:
  - component:MainTabs (改修)
  - component:TerminalPane (改修, dvh + safe-area)
  - req:FR-TABS-001
  - req:FR-TABS-002
  - req:FR-TABS-003
  - req:FR-TERMINAL-001
  - req:FR-TERMINAL-002
  - req:FR-RUNSTATE-001
  - adr: [0061-apg-tabs-manual-activation](../../adr/adr-20260624-0061-apg-tabs-manual-activation.md)
  - adr: [0060-adaptive-layout-and-drawer](../../adr/adr-20260624-0060-adaptive-layout-and-drawer.md)

### Chunk: `m5-toast-and-tagpill-and-listbox`

- **Depends on**: `m1-token-and-test-infra`, `m3-adaptive-layout-and-drawer`

- **Members**:
  - component:NotificationToast (改修)
  - component:UndoSnackbar
  - component:UnifiedListbox primitive
  - component:SessionList (改修)
  - component:DriverViewPanel TagPill (改修)
  - component:contrast util
  - req:FR-PALETTE-NAV-001
  - req:FR-PALETTE-IME-001
  - req:FR-PALETTE-FAIL-001
  - req:FR-TOAST-001
  - req:FR-TOAST-002
  - req:FR-TOAST-003
  - req:FR-TAGPILL-001
  - req:FR-A11Y-001
  - req:FR-A11Y-002
  - adr: [0063-toast-single-live-and-undosnackbar](../../adr/adr-20260624-0063-toast-single-live-and-undosnackbar.md)

### Chunk: `m6-reduced-motion-and-final-consistency`

- **Depends on**: `m1-token-and-test-infra`, `m2-theme`, `m3-adaptive-layout-and-drawer`, `m4-tabs-and-terminal-dvh`, `m5-toast-and-tagpill-and-listbox`

- **Members**:
  - component:reduced-motion guard
  - component:SessionList (改修)
  - component:NotificationToast (改修)
  - req:FR-MOTION-001
  - req:FR-MOTION-002
  - req:FR-TOKEN-001
  - req:FR-TOKEN-002
  - adr: [0064-reduced-motion-single-guard](../../adr/adr-20260624-0064-reduced-motion-single-guard.md)

## テスト戦略

各 chunk の完了は次の手段で検証する:

- **静的検証**: `cd src && go vet ./...` / `make lint` (golangci-lint depguard / funlen / staticcheck) / `cd src/client/web && pnpm biome check` / `pnpm tsc --noEmit`
- **Vitest 単体テスト**: `cd src/client/web && pnpm vitest run`。各 FR を 1 件以上の observable assertion でカバー。
  - DOM 観察: `getComputedStyle` / `getBoundingClientRect` / `scrollWidth` / `document.activeElement` / `data-*` 属性 / `aria-*` 属性 (inert は属性付与の有無で代替)。
  - matchMedia mock: `test-setup.ts` の setMatchMedia helper で prefers-color-scheme / prefers-reduced-motion を切替。
  - FR-TOKEN-001 の構造観察: `fs.readFile` で SessionList / palette listbox の CSS source を読み、正規表現で `--row-*` 参照を持つことを assert (ハードコード値の混入を構造的に排除)。
  - FR-TAGPILL-001 の数値観察: contrast util の `contrastRatio` を直接呼び 4.5 以上を assert + DOM の computed style からも assert (二重観察)。
- **Go 側テスト**: 本刷新は wire/persistence/daemon 変更なし (FR-WIRE-001) のため Go 側追加テストなし。既存 `cd src && go test ./server/web/... ./client/state/...` の緑維持のみ確認。
- **手動検証**: `make build && ./server` で実機起動し、PC (>=1024px) / タブレット (768-1024px) / スマホ (<768px) の 3 帯で各 chunk 完了時に主要 flow (F-001 drawer / F-002 theme / F-003 palette / F-004 tab 切替) を一度ずつ実施。iOS Safari は dvh + safe-area の挙動確認に必須。

## 移行戦略 (visual 退行ゼロを最初の安全弁)

- **feature flag を採らない**。token 置換 (m1) → theme (m2) → 構造刷新 (m3) → terminal/tabs (m4) → toast/tagpill/listbox (m5) → reduced-motion (m6) の順で段階的に PR を分ける。各 PR は独立に merge 可能で、各 PR 完了時点で対応 UAC subset が緑になる。
- **m1 (token 置換) の安全弁**: 既存ハードコード色/値を tokens.css の var() に置換するとき、置換前後で `getComputedStyle` の pixel/color 値が完全一致することを Vitest で観察する (= 視覚回帰テスト不在の代替)。computed pixel 同値だけでは構造的トークン共有を縛れないため、CSS source の正規表現観察 (FR-TOKEN-001) を併用する。
- **m2 (theme) の進入経路**: 初期既定は `localStorage` 未設定 → matchMedia 連動 = legacy ダーク固定の挙動を破壊しない (legacy ユーザの初回表示は OS dark なら従来通り dark)。
- **m3 (構造刷新) で破壊的になる箇所**: App.tsx → AppShell へ rename + grid-template-areas 切替で `.app-header` セレクタが新規に named area に乗る。CreateSessionForm 関連は ADR-0043 (前 PR) で既に撤去済みのため再撤去不要。
- **m4 (dvh)**: 100vh → 100dvh は legacy ブラウザ (Safari 15 以前) では fallback として 100vh が必要。tokens.css で `--dvh: 100dvh` を宣言し、`@supports not (height: 100dvh) { :root { --dvh: 100vh; } }` で graceful degrade。
- **multi-tab race**: previousActiveSessionId / drawer.open は AppShell の useState で UI-local 保持 (FR-STORE-001) のため各 tab で独立。Zustand store の race は構造的に無い。
- **localStorage 不正値**: ThemeProvider で 'system' / 'light' / 'dark' 以外なら system にフォールバック (FR-THEME-001 の data integrity)。テストで '<script>' や 空文字 / 数値 を入れて system 挙動を assert。

## Resolved Issues (plan-how 統合役による収束)

否定役 (critic) と最適化役 (optimizer) が指摘した論点と、その解決内容。

- **Issue**: [blocker] test-setup.ts に matchMedia / visualViewport mock が存在しないため UAC-005/006/007/016 のテストが既存 setup では構築不能
  - **Resolution**: m1-token-and-test-infra の最初の作業として test-setup.ts に matchMedia mock を追加 (Components に明記)。visualViewport は MVP では追加せず TerminalPane dvh テストは window.innerHeight 変動で代替 (新規依存ゼロ NFR-PERF-001 と整合)。
- **Issue**: [major] 0035 連番が使用済みなのに『空き連番』と主張する不整合
  - **Resolution**: 新規 ADR は全て 0059 以降の連番で割当 (0059-0064)。0035 は使わない。
- **Issue**: [blocker] xterm.options.theme 経由実装では UAC-006 Then literal『同一 CSS custom property を読む path で』を満たせない (xterm が inline style で背景を書き込むため)
  - **Resolution**: ADR-0059 で『ハイブリッド bridge』を確定 (xterm DOM bg は CSS で `background: var(--bg)` を直接付与し ITheme は fg/cursor/selection のみ)。FR-THEME-003 と TerminalPane Components 説明に明記。
- **Issue**: [blocker] FR-DRAWER-002 の OR 緩和で aria-hidden 単独実装が通り抜けうる + scrim 越しタップが防げない
  - **Resolution**: FR-DRAWER-002 を『inert + aria-hidden + pointer-events:none』の三重 guard 必須 (AND) に強化。ADR-0060 Decision にも明示。
- **Issue**: [major] breakpoint 跨ぎ時の連続リサイズで二重発火 / focus 喪失
  - **Resolution**: FR-DRAWER-007 を 50ms debounce + idempotent + focus 復帰先 (HamburgerToggle visible なら → else AppShell header 最初の focusable) で明示。ADR-0060 Decision にも明示。
- **Issue**: [major] Undo の previousActiveSessionId を Zustand store に置くと multi-tab で破壊される
  - **Resolution**: FR-STORE-001 で『previousActiveSessionId は AppShell の useState で UI-local 保持』を明示。DrawerStoreSlice を新設しない。multi-tab race を構造的に排除 (各 tab で独立 state)。
- **Issue**: [major] dvh 値変動と ResizeObserver/rAF refit の整合 (ADR-0029/0030/0034 不変条件)
  - **Resolution**: FR-TERMINAL-001 を state_driven (連続変化中の継続要件) + rAF coalesce 1 frame 1 回で書き換え。ADR-0060 で 0029 の本質 (flex contract) を保ったまま root 高さ計算式のみ差替えることを明示し ADR-0060 frontmatter で 0029 を extends 関係に置く。
- **Issue**: [blocker] FR-PALETTE-TRIGGER-001 の counterexample 通り抜け (overlay full-bleed + 内側 sheet が legacy 互換で max-width 中央配置)
  - **Resolution**: FR-PALETTE-TRIGGER-001 で観察対象を『data-role='palette-sheet' を持つ sheet container』に固定し、width 下限 (viewport.innerWidth - 32px 以上) も追加。
- **Issue**: [major] FR-TOKEN-001 が computed pixel 同値だけで構造的トークン共有を縛れない
  - **Resolution**: FR-TOKEN-001 に『CSS source を fs.readFile + 正規表現で読み --row-* 参照を持つことを構造的に観察する Vitest テスト』を追記。
- **Issue**: [major] TagPill 縁取り単独では 4.5:1 数値 AA 保証不能 (Open Q5)
  - **Resolution**: FR-TAGPILL-001 を『fg を黒/白に反転 + 縁取り付与のハイブリッド』に変更。contrast util を新設し relativeLuminance + contrastRatio で観察可能化。
- **Issue**: [major] APG Tabs automatic vs manual activation の齟齬
  - **Resolution**: FR-TABS-002 を manual activation 流儀 (ArrowRight は focus 移動のみ、Space/Enter で activate) に固定。ADR-0061 Decision に明示。
- **Issue**: [major] UndoSnackbar を NotificationToast コンテナに乗せると live region 内 interactive 問題と announce race
  - **Resolution**: FR-TOAST-001 と FR-TOAST-003 で『3 region 独立 (passive 通知 / UndoSnackbar status / palette InlineStatus)』+ Undo button は live region 外の兄弟 wrapper に分離を明示。ADR-0063 で構造的に固定。
- **Issue**: [major] palette 既存 flash (palette-active-context--flash / palette-listbox__row--flash) が reduced-motion guard 配下に入るか
  - **Resolution**: FR-MOTION-001 のセレクタ列挙に palette 系 flash を含め、ADR-0064 Decision でも明示。
- **Issue**: [major] 視覚回帰テストの仕組み不在
  - **Resolution**: MVP では Playwright / visual snapshot を導入しない (新規依存 NFR-PERF-001 と摩擦)。代替として Step 1 (token 置換) のテストは『token 参照経由でも computed pixel 値が legacy と完全一致 (border-radius / padding / font-size 各ピクセル単位)』を unit test で観察。Out of Scope に明記し将来 Open Questions として残す。
- **Issue**: [major] out_of_scope と TerminalPane 改修の関係 (ADR-0029 との位置づけ)
  - **Resolution**: ADR-0060 frontmatter の relations で『extends: adr-0029-terminal-host-flex-height』を明示し、0029 の flex contract を保ったまま root 高さ計算式を 100vh → 100dvh + safe-area に差替える delta であることを Decision に明記。
- **Issue**: [major] FR-PALETTE-NAV-001 / FR-TOKEN-002 が SessionList の role 変更 (button → listbox) を含意するが波及度が不明
  - **Resolution**: FR-TOKEN-002 に『SessionList を role='listbox' に格上げ + UnifiedListbox primitive を共有』を明示。Components に UnifiedListbox primitive 新設を追加 (palette 既存 hook を薄く抽出)。
- **Issue**: [minor] app.css 469 行が 500 行制約に逼迫
  - **Resolution**: NFR-CSS-001 を『2 ファイル → 3 ファイル (tokens.css + app.css + view.css)』に改定し FR-FRAMEWORK-001 で正典化。tokens を独立ファイルへ分離。
- **Issue**: [minor] FR-PALETTE-FAIL-001 と ADR-0057 の関係
  - **Resolution**: FR-PALETTE-FAIL-001 の rationale に『ADR-0057 を extends せず単一 slot 方針を継承』を明示。ADR-0063 の Decision でも 3 region 独立 (palette InlineStatus = ADR-0057 と非衝突) を明示。
- **Issue**: [minor] localStorage 'agent-reactor-theme' の data integrity (無効値 fallback)
  - **Resolution**: FR-THEME-001 を state_driven の condition に『値が system/light/dark のいずれにも一致しない間』を含め、無効値は system 等価扱い (fallback) を構造的に成立させる。
- **Issue**: [minor] 左スワイプ取消経路と FR-DRAWER-005/006 の片肺
  - **Resolution**: FR-DRAWER-005 に左→右スワイプを追加し threshold (水平>=50 垂直<30) を明示。Components に SwipeHandler util を追加。
- **Issue**: [minor] 44px 対象の網羅性 (UndoSnackbar / TagPill 含むか)
  - **Resolution**: FR-A11Y-001 を enum 列挙 (SessionList row / MainTabs tab / close-back / NotificationToast item dismiss / UndoSnackbar Undo / ThemeSegmentedControl segment / HamburgerToggle / CommandSearchTrigger / palette listbox option) で具体化。TagPill は interactive ではないため対象外を明示。
- **Issue**: [improvement] Optimizer: ADR 件数を 7 → 6 に圧縮 (drawer を適応レイアウト ADR に内包)
  - **Resolution**: 採用。ADR を adr-20260625-{design-token-and-theme-bridge / adaptive-layout-and-drawer / apg-tabs-manual-activation / search-bar-trigger-and-palette-theme-entry / toast-single-live-three-regions-and-safe-area / reduced-motion-single-guard} の 6 件で確定 (連番 0059-0064)。
- **Issue**: [improvement] Optimizer: ChipSwitch から SegmentedControl primitive 抽出 / palette ToolSelectPhase から UnifiedListbox primitive 抽出 / palette InlineStatus から LiveStatusRegion 抽出
  - **Resolution**: 採用 (一部)。Components に primitives/ サブディレクトリを新設し SegmentedControl primitive と UnifiedListbox primitive を追加。LiveStatusRegion は FR-TOAST-003 の 3 region 独立構造で十分代替できるため MVP では別ファイル抽出せず NotificationToast 内に閉じる (YAGNI、Open Questions に格上げの可能性を残す)。
- **Issue**: [improvement] Optimizer: Open Q3 (テーマトグルのスマホ配置) を案 C (palette 内 entry) で決着
  - **Resolution**: 採用。ADR-0062 で palette suggested action 'Theme: System / Light / Dark' に吸収を確定。
- **Issue**: [improvement] Optimizer: Open Q4 (search-bar と New Session) を案 A (palette 完全吸収) で決着
  - **Resolution**: 採用。ADR-0062 で header の button 数を 1 (CommandSearchTrigger のみ) に確定。empty state ヒントで発見性補強を ADR-0062 Consequences に明示。
- **Issue**: [improvement] Optimizer: Build sequence 7 → 6 段に圧縮
  - **Resolution**: 採用。chunks を m1-m6 の 6 段で確定。各 chunk が独立 PR にできるよう depends_on は非循環。
- **Issue**: [improvement] Optimizer: feature flag 採否
  - **Resolution**: feature flag を採らず token 置換による visual 退行ゼロを最初の安全弁とし、各 Build step で対応 UAC subset を緑にする方針で確定。
- **Issue**: [翻訳] ux UAC-001/002/008/011/013/015/016 の counterexample → 各 FR で discriminate
  - **Resolution**: 各 counterexample を FR の literal text に DOM/aria/computed style の観察事実として埋め込み、通り抜け攻撃を排除 (例: FR-PALETTE-TRIGGER-001 の data-role='palette-sheet' + width 下限 / FR-DRAWER-002 の三重 guard AND / FR-TOKEN-001 の CSS source 構造観察)。
- **Issue**: [翻訳] ux legacy_context.source_implementation + replaced_behaviors → Migration ADR
  - **Resolution**: replaced_behaviors の各項目 (280px 固定 sidebar / メディアクエリ 0 件 / ダーク固定 / 100vh / touch 非対応 / コントラスト未レビュー / inline NotificationToast / MainTabs onClick / spinner 無条件 / TagPill 任意色) を ADR-0059..0064 の Context / Decision で明示参照。各 ADR は ux.md (ux-2026-06-25-web-ui-redesign) を relations.references で後方リンク。
- **Issue**: [翻訳] ux reference_ux.stance=modeled_on → Pattern Adoption ADR
  - **Resolution**: ADR-0060 (Material drawer) / ADR-0062 (Raycast palette + Spotlight + segmented control) / ADR-0061 (APG Tabs) / ADR-0059 (Primer-Radix token + GitHub system-aware) / ADR-0063 (Gmail Snackbar + palette ADR-0057) の 5 ADR で分担し、各 ADR の Context に reference を明示。
- **Issue**: [翻訳] ux reference_ux.stance=rejected → 各 Pattern Adoption ADR の Alternatives へ
  - **Resolution**: ADR-0059 / ADR-0060 / ADR-0062 の Alternatives に分散して『rejected』を記録 (native <dialog> showModal / 重量級 CSS framework / 素朴ソフトキーボード打鍵)。

## Open Questions (実装段階で確定する事項)

> いずれも plan-impl 段階で grep / 観察により確定する。設計判断は ADR で決着済み。

- モバイル仮想キー補助バー (案A: Esc/Ctrl/Tab/方向キー/|/- の最小キー集合) の着手時期と実装方針。MVP は read-first (案B) で進めるが、後続フェーズで案A をいつ・どこまで入れるかは spec/plan の範囲外。Edge Case (visualViewport 追従 + xterm key event 注入 + sticky modifier 状態管理) の事前検証が必要 (実装の不確実性)。
- LiveStatusRegion プリミティブを palette InlineStatus から抽出するかは MVP では YAGNI として見送り、ADR-0063 の 3 region 独立構造で代替する。後続 PR で palette InlineStatus と UndoSnackbar の重複度が高まったら抽出を再評価 (実装の不確実性)。
- 視覚回帰テストの仕組み (Playwright / Storybook visual snapshot 等) を後続で導入するか。MVP は token 置換による visual 退行ゼロを宣言で担保するが、Step 3 (drawer) 以降の big delta では人手レビュー負荷が増える。新規依存導入の正当化を伴うため Open Questions として残す (実装の不確実性)。
