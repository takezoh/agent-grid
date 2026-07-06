---
id: spec-20260625-2026-06-25-web-ui-redesign
kind: spec
title: Spec — Web UI 全面刷新 (適応レイアウト + デザイントークン + テーマ + a11y 波及)
status: draft
created: '2026-06-25'
updated: '2026-07-04'
tags:
- spec
- legacy-import
owners: []
relations:
- {type: referencedBy, target: adr-20260624-0059-design-token-and-theme-bridge}
- {type: referencedBy, target: adr-20260624-0060-adaptive-layout-and-drawer}
- {type: referencedBy, target: adr-20260624-0061-apg-tabs-manual-activation}
- {type: referencedBy, target: adr-20260624-0062-search-bar-trigger-and-palette-theme-entry}
- {type: referencedBy, target: adr-20260624-0063-toast-single-live-and-undosnackbar}
- {type: referencedBy, target: adr-20260624-0064-reduced-motion-single-guard}
- {type: implementedBy, target: plan-20260625-2026-06-25-web-ui-redesign}
- {type: referencedBy, target: plan-20260625-2026-06-25-web-ui-redesign}
- {type: implements, target: ux-20260625-2026-06-25-web-ui-redesign}
- {type: references, target: adr-20260624-0059-design-token-and-theme-bridge}
- {type: references, target: adr-20260624-0060-adaptive-layout-and-drawer}
- {type: references, target: adr-20260624-0061-apg-tabs-manual-activation}
- {type: references, target: adr-20260624-0062-search-bar-trigger-and-palette-theme-entry}
- {type: references, target: adr-20260624-0063-toast-single-live-and-undosnackbar}
- {type: references, target: adr-20260624-0064-reduced-motion-single-guard}
- {type: references, target: plan-20260625-2026-06-25-web-ui-redesign}
- {type: references, target: ux-20260625-2026-06-25-web-ui-redesign}
source_paths:
- src/client/web/
- src/client/web/src/css/tokens.css
- src/client/web/src/util/contrast.ts
- src/client/web/src/store/theme.ts
- src/client/web/src/css/
- src/client/web/src/
- src/client/web/src/components/
functional_requirements: []
non_functional_requirements: []
acceptance: []
---

<!-- migrated_from: docs/specs/2026-06-25-web-ui-redesign/spec.md -->

# Spec — Web UI 全面刷新 (適応レイアウト + デザイントークン + テーマ + a11y 波及)

- **作成日**: 2026-06-25
- **ブランチ**: `main`
- **ux**: [ux.md](../../specs/2026-06-25-web-ui-redesign/ux.md) (`ux-2026-06-25-web-ui-redesign`)
- **plan**: [plan.md](../../specs/2026-06-25-web-ui-redesign/plan.md)
- **ADRs**: [0059](../../adr/adr-20260624-0059-design-token-and-theme-bridge.md), [0060](../../adr/adr-20260624-0060-adaptive-layout-and-drawer.md), [0061](../../adr/adr-20260624-0061-apg-tabs-manual-activation.md), [0062](../../adr/adr-20260624-0062-search-bar-trigger-and-palette-theme-entry.md), [0063](../../adr/adr-20260624-0063-toast-single-live-and-undosnackbar.md), [0064](../../adr/adr-20260624-0064-reduced-motion-single-guard.md)

## Goal

agent-grid-new の Web client (src/client/web) UI/UX 全面刷新を、ux.md (16 UAC / 4 flow / vs_legacy: must-fail 11 件) を起点に EARS 形式 FR と Nygard 4 節 ADR で固める。(1) 適応レイアウト (PC/タブレット/スマホ + dvh + safe-area)、(2) デザイントークン体系 + light/dark/system テーマ + xterm 配色連動、(3) palette で熟成済みの視覚言語と a11y パターン (unified listbox / focus trap / 単一 aria-live slot / disabled visible+skip-navigation / IME 抑止) を全画面 (session list / MainTabs / toast / DriverViewPanel) へ波及させる、を観察可能な振る舞いとして EARS 化する。新規 ADR 6 件 (0059 token / 0060 適応レイアウト+drawer / 0061 APG Tabs / 0062 search-bar trigger + theme palette entry / 0063 toast 単一 aria-live + UndoSnackbar 共有 + safe-area / 0064 reduced-motion 一元 guard) を起こす。既存 keyboard/ARIA/IME 資産・palette 内部状態機械・store pureness・wire 型 stdlib・activeSessionID client 単独管理・ADR-0029/0030/0032/0033/0034 を退行させない。

## Scope

### In Scope

- src/client/web/src/css/tokens.css 新設 (semantic CSS custom property 階層)。app.css は 469 行で 500 行制約に逼迫しているため tokens を独立ファイルへ分離し、NFR-CSS-001 を『tokens.css + app.css + view.css の 3 ファイル構成』に改定
- 既存ハードコード色 (#1e1e1e / #555 / #333 / opacity:0.7/0.75/0.8 等) を tokens.css の var(--*) 参照へ全置換 (visual 退行ゼロを最初の安全弁とする)
- ThemeProvider / ThemeStoreSlice (theme.ts) 新設 — system/light/dark + localStorage 永続 + prefers-color-scheme 連動 + xterm ITheme bridge (案A=getComputedStyle 読取 + useLayoutEffect + 1 rAF guard)
- ThemeSegmentedControl 新設 — 既存 palette/ChipSwitch を共通 SegmentedControl プリミティブとして抽出し再利用 (palette と theme が共有)
- AppShell 新設 (App.tsx を refactor) — named grid areas を breakpoint (<768 / 768-1024 / >=1024) で切替、100dvh + env(safe-area-inset-*)、HamburgerToggle を header 内に統合
- SessionDrawer 新設 — スマホ off-canvas + role='dialog' aria-modal='true' + 背後 inert + aria-hidden + main 領域 pointer-events:none の三重 guard + scrim + 左→右スワイプ touch 補助 (視覚的痕跡なし) + breakpoint 跨ぎ guard (連続リサイズ idempotent)
- UndoSnackbar — NotificationToast コンテナへ吸収、previousActiveSessionId は AppShell の useState で UI-local 管理 (Zustand slice 化しない)
- CommandSearchTrigger — header の検索バー風 button (虫眼鏡 + 'Search commands…' + ⌘K/Ctrl+K hint badge を isMacPlatform で出し分け)。New Session を palette 内 suggested action に完全吸収 (ADR-0062)
- MainTabs 改修 — WAI-ARIA APG Tabs Pattern (roving tabindex + ArrowLeft/Right/Home/End + manual activation: focus 移動と activate を分離 / Space/Enter で activate)
- TerminalPane 改修 — 100vh → 100dvh + env(safe-area-inset-bottom)。xterm.options.theme を ThemeProvider bridge の prop で受領しテーマ変更時に再適用。ADR-0029/0030/0034 の不変条件 (flex contract / keyed remount / rAF coalesce refit) は保持
- NotificationToast 改修 — コンテナ 1 つに aria-live='polite' role='status' を集約 (item の aria-live を削除、ADR-0057 の方針を波及)。inline ハードコード配色を token 参照へ。スマホは bottom 寄せ + env(safe-area-inset-bottom)。UndoSnackbar 用に interactive 領域は live region 外の隣接 wrapper に分離 (live region 内 interactive 問題を回避)
- DriverViewPanel TagPill — driver 任意色のコントラスト比 (相対輝度 → 4.5:1) を計算し閾値未満なら fg を黒/白の高コントラスト側へ反転 + 縁取りを追加するハイブリッド fallback
- reduced-motion 一元 guard — view.css 末尾の単一 @media (prefers-reduced-motion: reduce) block に .run-state-spinner / .session-status-spinner / drawer slide / palette-active-context flash / palette-listbox__row flash / その他 transition を集約
- SessionList 改修 — palette ToolSelectPhase / ParamListbox と同一 listbox semantics に格上げ (role='listbox' + aria-activedescendant)、disabled visible+skip-navigation。border-radius / padding / font-size を tokens.css の同一 var() 参照に統一。長文 displayLabel は 2 行 ellipsis (-webkit-line-clamp:2)、最小 44x44px。displayLabel chain (ADR-0033) は維持
- Vitest テスト追加 — 各 FR を 1 つ以上のテストで observable に assert。matchMedia mock を test-setup.ts に追加 (prefers-color-scheme / prefers-reduced-motion 切替)。inert 観察は属性付与の有無で代替し、加えて aria-hidden + pointer-events 三重 guard を観察

### Out of Scope

- palette 内部状態機械 (ADR-0036/0039/0050/0055/0057 系) の作り直し — palette が読む CSS custom property のみ新 token 体系に統一
- wire/persistence 型の変更 (src/client/web/src/wire/* および Go ミラー)
- daemon の view-update payload 変更 (activeSessionID は client 単独管理を維持)
- Zustand store を DOM 操作主体に変える (pure 不変条件維持)
- モバイル仮想キー補助バー (案A) の MVP 実装 (read-first 案B が MVP)。後続 progressive enhancement に残す
- 新規 npm 依存追加 (React + Zustand + xterm + FitAddon の 4 依存維持)
- CSS framework / Tailwind / CSS-in-JS ランタイム (ux.md で rejected)
- ネイティブ <dialog> showModal() への移行 (ux.md で rejected)
- TerminalPane の xterm + FitAddon + ResizeObserver + rAF coalesce refit + keyed remount (ADR-0029/0030/0034) の再設計 — 高さ計算式の差替 (100vh → dvh) と xterm theme 再適用フローのみ delta
- RunStateBadge の表示モデル変更 (ADR-0032 維持、token を読む path のみ統一)
- displayLabel chain (ADR-0033) の変更
- StatusBanner の機能変更 (token を読む path のみ統一)
- 視覚回帰テストの新規導入 (Playwright / visual snapshot は新規依存となり NFR-PERF-001 と摩擦。Open Question として残し、Step 1 token 置換は『tokens 参照経由でも computed pixel 値が legacy と完全一致』を unit test で観察する形で代替)
- テーマトグルのスマホ専用 bottom-sheet 新規 UI (Open Q3 を palette 内 entry に吸収 = ADR-0062 で完結)
- DrawerStoreSlice 新設 (drawer 状態は AppShell/SessionDrawer の useState で UI-local 化)

## Requirements (EARS)

> EARS = Easy Approach to Requirements Syntax。`ubiquitous` (常に成立) / `event_driven` (X したとき) / `state_driven` (X の間) / `unwanted` (X したら) / `optional` / `complex` の型を持つ。

### Layout (FR-LAYOUT-001..004)

- **FR-LAYOUT-001** *(ubiquitous)* — システムは viewport 幅が 768px 未満のとき、AppShell の grid-template-areas を 'banner' 'header' 'main' の単一カラムで構成し、SessionList 領域を off-canvas (CSS transform で viewport 外、display:none ではない) で配置し、AppShell header 内の data-role='hamburger' な button (aria-label='Open sessions', aria-expanded='false') を visible に保ち、document.scrollingElement.scrollWidth が viewport.innerWidth を超えてはならない。
  - *Rationale*: UAC-001 entry observation を ubiquitous で固定して初期表示 invariant を欠落させない。
- **FR-LAYOUT-002** *(event_driven)* — data-role='hamburger' button が click/Enter/Space で activate されたとき、システムは SessionDrawer の aria-expanded を 'true' に遷移させ、SessionList 要素 (role='listbox') の getBoundingClientRect().width を 0 より大きく viewport.innerWidth 以内に配置しなければならない。
  - *Rationale*: UAC-001 配線確認 (counterexample: 死んだ UI が初期 off-canvas だけで pass する攻撃を排除)。
- **FR-LAYOUT-003** *(state_driven)* — viewport 幅が 1024px 以上の間、システムは AppShell の grid-template-areas を 'banner banner' 'header header' 'sidebar main' で構成し、sidebar area に SessionList を常時可視で配置しなければならない。
  - *Rationale*: UAC-013 PC 帯 invariant + 768-1024 タブレット帯は同 grid + sidebar 折り畳み可で degrade。state_driven で『その幅の間』を継続要件化。
- **FR-LAYOUT-004** *(ubiquitous)* — システムは AppShell root および main area の高さを 100dvh ベースで算出し、bottom/top に env(safe-area-inset-bottom) / env(safe-area-inset-top) を尊重した padding または margin を持たせなければならない。
  - *Rationale*: UAC-015 dvh + safe-area の前提を ubiquitous で固定。100vh 依存実装が pass しないよう dvh を invariant 化。

### Drawer (FR-DRAWER-001..007)

- **FR-DRAWER-001** *(state_driven)* — SessionDrawer が open の間、システムは drawer ルート要素に role='dialog' および aria-modal='true' を付与し、document.activeElement を drawer subtree 内に維持しなければならない。
  - *Rationale*: UAC-002 a11y modal の半分 (focus trap 側)。
- **FR-DRAWER-002** *(state_driven)* — SessionDrawer が open の間、システムは scrim 配下の main 領域 (DriverView/MainTabs を含むコンテナ) に inert 属性を付与しかつ aria-hidden='true' を付与しかつ computed pointer-events が 'none' でなければならない。
  - *Rationale*: UAC-002 counterexample (focus trap だけでは VoiceOver rotor が背後到達) + 否定役指摘 (OR 緩和で aria-hidden 単独 pass を排除、pointer-events 三重 guard を強制)。inert は AT/keyboard、aria-hidden は VoiceOver rotor、pointer-events は scrim 越しタップを各々遮断する直交責務。
- **FR-DRAWER-003** *(event_driven)* — drawer 内の非アクティブセッション行が click/Enter で activate されたとき、システムは drawer を close (aria-expanded='false') し focus を data-role='hamburger' button へ復帰させ、中央 DriverView の title/subtitle を選択セッションの値に更新し、UndoSnackbar に 'Switched to <label>' を visible 表示しなければならない。
  - *Rationale*: UAC-003 選択クローズの exit observation。
- **FR-DRAWER-004** *(event_driven)* — UndoSnackbar の Undo button が activate されたとき、システムは activeSessionID を直前のセッション ID へ戻し、中央 DriverView の title/subtitle を直前セッションの値に戻さなければならない。
  - *Rationale*: UAC-003 Undo 回復経路。previousActiveSessionId は AppShell の useState で UI-local 管理 (FR-STORE-001)。
- **FR-DRAWER-005** *(event_driven)* — SessionDrawer が open の状態で scrim の click または Esc keydown または drawer 内の左→右スワイプ (touchstart→touchend で水平距離 50px 以上かつ垂直距離 30px 未満) が発生したとき、システムは drawer を close し focus を data-role='hamburger' button へ復帰させなければならない。
  - *Rationale*: UAC-004 取消クローズの 3 経路 (ux.md F-001 step 末尾)。否定役指摘の片肺 FR を 3 経路に拡張。
- **FR-DRAWER-006** *(unwanted)* — もし FR-DRAWER-005 の 3 経路 (scrim click / Esc / 左→右スワイプ) いずれかで SessionDrawer が close した場合、システムは activeSessionID および中央 DriverView の title/subtitle を drawer を開く前の値から変更してはならない。
  - *Rationale*: UAC-004 counterexample (scrim tap が直近 highlight 行を誤選択する実装) を unwanted で排除。
- **FR-DRAWER-007** *(event_driven)* — SessionDrawer が open の状態で viewport 幅が 1024px 以上に変化したとき、システムは drawer を close 状態へ遷移させ、main 領域の inert / aria-hidden / pointer-events を解除し、focus が drawer subtree 外の data-role='hamburger' button (visible なら) または AppShell header 内の最初の focusable へ移動しなければならない。連続リサイズ (50ms 以内の連続変化) でも本遷移は idempotent でなければならない (二重 close / focus 喪失を起こさない)。
  - *Rationale*: ux.md Edge Case 『drawer open 中の breakpoint 跨ぎ』+ 否定役指摘 (連続リサイズ idempotency)。

### Theme (FR-THEME-001..007)

- **FR-THEME-001** *(state_driven)* — localStorage のキー 'agent-grid-theme' が未設定または値が 'system' / 'light' / 'dark' のいずれにも一致しない間、システムは matchMedia('(prefers-color-scheme: dark)').matches に応じて document.documentElement の data-theme を 'dark' (match=true) または 'light' (match=false) に同期し、body の getComputedStyle.backgroundColor および本文テキスト/背景のコントラスト比が WCAG AA (4.5:1) を満たし、xterm terminal 要素 (.xterm-screen 等) の getComputedStyle.backgroundColor が同 data-theme で解決される token 値と一致しなければならない。
  - *Rationale*: UAC-005 entry observation (system 連動初回ロード)。state_driven + 不正値 fallback (data integrity) を統合。否定役指摘の localStorage validation を内包。
- **FR-THEME-002** *(event_driven)* — ThemeSegmentedControl の Light / Dark segment が click または Space で activate されたとき、システムは document.documentElement の data-theme 属性を 'light' / 'dark' に設定し localStorage の 'agent-grid-theme' に同値を永続化し、body および xterm terminal 要素の getComputedStyle.backgroundColor を data-theme 切替後 1 rAF 以内に新 token 値へ更新し、選択 segment の aria-checked を 'true'、他 segment を 'false' にしなければならない。
  - *Rationale*: UAC-006 明示切替。否定役指摘 (xterm DOM の getComputedStyle.backgroundColor は xterm が書き込む inline style を返す問題) を ADR-0059 で『xterm の .xterm-screen 系要素は CSS で background: var(--bg) を直接付与し、ITheme には透明系の fg/cursor/selection を渡す混成方式 (案A+C ハイブリッド)』として確定し、その方針下で『同一 token 経由で computed bg が一致』を観察可能にする。
- **FR-THEME-003** *(ubiquitous)* — システムは xterm.options.theme を ThemeProvider が getComputedStyle で tokens.css の custom property (--xterm-fg / --xterm-cursor / --xterm-selection) から構築した ITheme オブジェクトで設定し、xterm 要素の DOM background は CSS で background: var(--bg) として token を直接読まなければならない (= JS ITheme 経路と CSS 経路の 2 方向で同一 token に連動)。ThemeProvider は data-theme 属性変更後 useLayoutEffect + 1 rAF guard で再構築する。
  - *Rationale*: UAC-006 counterexample (xterm 直書きハードコード) を ITheme 経路で排除し、UAC-006 Then の literal text (『同一 CSS custom property を読む path で』) を CSS background 経路で満たす。否定役 blocker (案A 単独では UAC-006 literal を満たせない懸念) の解決。
- **FR-THEME-004** *(state_driven)* — localStorage の 'agent-grid-theme' に 'light' が永続化されている間、システムは prefers-color-scheme が dark に変化してもリロードしても body と xterm の computed backgroundColor を light token 値のまま維持し、Light segment の aria-checked を 'true' に保たなければならない。
  - *Rationale*: UAC-007 永続化。
- **FR-THEME-005** *(event_driven)* — ThemeSegmentedControl の System segment が activate されたとき、システムは localStorage の 'agent-grid-theme' キーを削除しなければならない。
  - *Rationale*: UAC-007 / Edge OS テーマ追従復帰。
- **FR-THEME-006** *(event_driven)* — localStorage の 'agent-grid-theme' が未設定の状態で matchMedia('(prefers-color-scheme: dark)') の change イベントが発火したとき、システムは document.documentElement の data-theme を新値に同期し、xterm ITheme を再構築して TerminalPane に伝播しなければならない。
  - *Rationale*: ux.md Edge Case (OS テーマ切替リアルタイム追従)。
- **FR-THEME-007** *(ubiquitous)* — システムは ThemeSegmentedControl を role='radiogroup' aria-label='Theme' とし、3 つの segment を常時一覧表示で各 role='radio' に持たせ、現在選択を aria-checked='true' に他を 'false' に設定し、ArrowLeft/Right で focus 移動 + Space/Enter で activate (manual activation) を受理しなければならない (WAI-ARIA radiogroup pattern)。
  - *Rationale*: UAC-006 a11y。共通 SegmentedControl プリミティブ (palette ChipSwitch 抽出) で実現。

### Palette (FR-PALETTE-TRIGGER-001 / NAV-001 / IME-001 / FAIL-001)

- **FR-PALETTE-TRIGGER-001** *(event_driven)* — viewport 幅 768px 未満で CommandSearchTrigger が tap で activate されたとき、システムは role='dialog' aria-modal='true' を持つ palette overlay を表示し、その overlay の直下にある palette sheet container 要素 (data-role='palette-sheet' を持つ) の getBoundingClientRect().left および viewport.innerWidth - right がそれぞれ 16px 以下、かつ getBoundingClientRect().width が viewport.innerWidth - 32px 以上であり、検索 input 要素に document.activeElement が当たり、palette 起動前に focus を持っていた xterm terminal 要素が blur されていなければならない。
  - *Rationale*: UAC-008。否定役指摘 (overlay full-bleed + 内側 sheet が legacy 互換で max-width 中央配置の通り抜け攻撃) を sheet container の data 属性指定 + width 下限指定で排除。
- **FR-PALETTE-NAV-001** *(state_driven)* — palette が open かつ listbox に disabled 行と enabled 行が混在する間、システムは ArrowDown/ArrowUp/Ctrl-P/Ctrl-N によるナビゲーションで aria-activedescendant を enabled 行の id のみに順次設定し、disabled 行は DOM 上 visible (display:none ではない) のまま理由テキストノードを子に持たせなければならない。
  - *Rationale*: UAC-009 (legacy ADR-0050 退行防止)。
- **FR-PALETTE-IME-001** *(unwanted)* — もし palette 検索 input に focus がある状態で IME composition 中 (compositionstart 後 compositionend 前) に Enter keydown が発生した場合、システムは tool 確定を行ってはならず、palette 内部 phase を data-phase='toolSelect' で観察可能な状態に保持しなければならない。
  - *Rationale*: UAC-010 (legacy ADR-0040 退行防止)。
- **FR-PALETTE-FAIL-001** *(event_driven)* — palette からの送信が daemon から reject されたとき、システムは検索 input の disabled / aria-busy を false に戻し、palette overlay を open 状態のまま保ち、palette の単一 aria-live='polite' slot (ADR-0057) に inline error メッセージを 1 回 announce し、Esc または Retry button での次操作を受理可能としなければならない。
  - *Rationale*: UAC-012。ADR-0057 を extends せず単一 slot 方針を継承。

### Token (FR-TOKEN-001..002)

- **FR-TOKEN-001** *(ubiquitous)* — システムは SessionList の行および palette listbox の行に同一の semantic CSS custom property (--row-radius / --row-padding-y / --row-padding-x / --row-font-size / --row-line-height / --row-min-height) を読ませ、両者の getComputedStyle.borderRadius / paddingTop / paddingBottom / paddingLeft / paddingRight / fontSize / lineHeight / minHeight がピクセル単位で一致しなければならない。tokens.css 内では SessionList および palette が読む全 CSS source が同一 var(--row-*) 参照であり、ハードコード値を持たないことを Vitest テスト (CSS source 文字列を読み取り正規表現で --row-* 参照を確認) で構造的に観察可能にする。
  - *Rationale*: UAC-011 counterexample (palette だけ新トークンで session list はハードコード値据え置き) + 否定役指摘 (computed pixel 同値だけでは偶然一致を排除できない) を CSS source の構造観察で解消。
- **FR-TOKEN-002** *(ubiquitous)* — システムは SessionList の disabled 行および palette listbox の disabled 行を visible のまま skip-navigation し、両者の disabled 行に理由テキストノードを付随させなければならない。SessionList は role='listbox' に格上げされ aria-activedescendant ナビで disabled 行を skip する (= palette ToolSelectPhase と同一 UnifiedListbox プリミティブを共有)。
  - *Rationale*: UAC-011 構造的一貫性。最適化役指摘 (UnifiedListbox プリミティブ抽出) を反映。

### Tabs (FR-TABS-001..003)

- **FR-TABS-001** *(ubiquitous)* — システムは MainTabs に role='tablist' を持つコンテナを置き、その内側に 3 つの role='tab' 要素を置き、aria-selected='true' の tab のみ tabindex='0' に他は tabindex='-1' に設定しなければならない (roving tabindex)。
  - *Rationale*: UAC-014 APG 構造。
- **FR-TABS-002** *(event_driven)* — MainTabs の tablist 内のいずれかの tab に focus がある状態で ArrowRight keydown が発生したとき、システムは次の tab へ focus を移動し tabindex roving を更新しなければならない (focus 移動のみ、aria-selected はまだ遷移しない = manual activation)。ArrowLeft で前 tab、Home で先頭、End で末尾、Space または Enter で focus 中の tab を activate し aria-selected を 'true' に遷移させる。
  - *Rationale*: UAC-014 (counterexample: onKeyDown 0 件 legacy 排除) + 否定役指摘 (data_flows と FR-TABS-002 の activation 流儀齟齬を manual activation 固定で解消)。
- **FR-TABS-003** *(state_driven)* — MainTabs の active tab が TRANSCRIPT または EVENTS の間も、システムは TERMINAL タブの terminal-host 要素を DOM に常時 mount し、CSS で visibility/display を切替えなければならない。TRANSCRIPT/EVENTS から TERMINAL へ戻したとき、xterm インスタンスは再生成されず terminal-host の getComputedStyle.height は 0px より大きく scrollback 内容が保持されなければならない。
  - *Rationale*: UAC-014 (scrollback 保持の inherited behavior)。

### Terminal (FR-TERMINAL-001..002)

- **FR-TERMINAL-001** *(state_driven)* — viewport 高さが iOS Safari ツールバー出没や仮想キーボード出現で連続変化する間、システムは terminal-host の高さを 100dvh ベースで再算出し、ResizeObserver による fit() を rAF coalesce で 1 frame 1 回に制限し、縮小・拡大いずれの方向にも terminal-host の getComputedStyle.height を 0px より大きく追従させなければならない。
  - *Rationale*: UAC-015 + 否定役指摘 (dvh 連続値変化と ResizeObserver/rAF refit の整合)。
- **FR-TERMINAL-002** *(unwanted)* — もし viewport 高さの変化中に body の overflow-y または overflow-x いずれかでスクロールバーが現れた場合 (window.innerHeight < document.documentElement.scrollHeight または window.innerWidth < document.documentElement.scrollWidth)、システムはこれを二重スクロール状態として fail させなければならない (= テストで観察可能なように、AppShell は body の overflow を hidden に保つ)。
  - *Rationale*: UAC-015 二重スクロール無発生。

### RunState (FR-RUNSTATE-001)

- **FR-RUNSTATE-001** *(state_driven)* — viewport 幅 375px で running 状態のセッションが選択されている間、システムは RunStateBadge を icon + text の両方が可視で表示しなければならず (ADR-0032 維持)、DriverView/MainTabs を含む main 領域の getBoundingClientRect().width は viewport.innerWidth 以下でなければならない (横スクロールなし)。
  - *Rationale*: UAC-013 + ADR-0032 多重符号化維持。

### Motion (FR-MOTION-001..002)

- **FR-MOTION-001** *(state_driven)* — matchMedia('(prefers-reduced-motion: reduce)').matches が true の間、システムは .run-state-spinner / .session-status-spinner / drawer slide / palette-active-context flash / palette-listbox__row flash / その他 view.css 内 transition の getComputedStyle.animationName を 'none' とし、または getComputedStyle.animationDuration を '0s' としなければならず、running 状態は icon + text で引き続き読み取れなければならない。
  - *Rationale*: UAC-016 (counterexample: spinner 無条件 animation) + 否定役指摘 (palette 既存 flash 群も guard 対象に明示)。
- **FR-MOTION-002** *(ubiquitous)* — システムは reduced-motion 抑制の @media (prefers-reduced-motion: reduce) ブロックを view.css 内の 1 箇所に集約し、新規アニメーション (drawer slide / UndoSnackbar slide-in 等) を追加するときは同ブロックで一律抑制対象に含めなければならない。
  - *Rationale*: UAC-016 + 一元 guard 維持の運用要件。

### Toast (FR-TOAST-001..003)

- **FR-TOAST-001** *(ubiquitous)* — システムは NotificationToast のコンテナ要素 1 つに aria-live='polite' および role='status' を付与し、各 item に aria-live 属性を付与してはならない (二重読み上げ防止)。UndoSnackbar のテキスト (passive) はこの live region 内に置き、Undo button (interactive) はこの live region の外 (兄弟 wrapper) に配置しなければならない。
  - *Rationale*: ADR-0057 波及 + 否定役指摘 (live region 内 interactive control 問題を兄弟 wrapper 分離で回避)。
- **FR-TOAST-002** *(state_driven)* — viewport 幅 768px 未満の間、システムは NotificationToast コンテナを position: fixed で bottom 寄せに配置し、bottom に env(safe-area-inset-bottom) + 16px のオフセットを持たせ、各 item の配色は tokens.css の semantic token (--toast-bg-info / --toast-bg-success / --toast-bg-warn / --toast-bg-error) を読まなければならない (inline ハードコード配色を持たない)。
  - *Rationale*: ux.md Edge トースト safe-area + 配色 token 化。
- **FR-TOAST-003** *(unwanted)* — もし UndoSnackbar の 'Switched to <label>' announce 直後に palette 送信失敗の inline error が同一 aria-live slot を上書きする経路が存在する場合、システムは UndoSnackbar 用の独立 aria-live='polite' slot を NotificationToast コンテナ内に分割確保し、palette の単一 aria-live slot とは別 region としなければならない (= live region は『passive notification』『UndoSnackbar status』『palette inline status (ADR-0057)』の 3 系統に独立化)。
  - *Rationale*: 否定役指摘 (Undo announce の race 握り潰し)。3 系統独立で announce 順序の race を構造的に排除。

### TagPill (FR-TAGPILL-001)

- **FR-TAGPILL-001** *(event_driven)* — DriverViewPanel TagPill が driver 提供の fg/bg を inline で適用する直前に、システムは src/client/web/src/util/contrast.ts の relativeLuminance() および contrastRatio() を呼び出し、ratio が 4.5 未満の場合 fg を黒 (#000) または白 (#fff) のうち bg とのコントラスト比が大きい側に置換し、その置換後の computed ratio が 4.5 以上でかつ追加で border: 1px solid currentColor 相当を付与しなければならない。最終的に TagPill 内テキストと bg の getComputedStyle 由来 ratio が 4.5 以上であることをテストで観察する。
  - *Rationale*: ux.md Edge TagPill + 否定役指摘 (縁取り単独では数値 AA 保証不能) を fg 反転ハイブリッド (Open Q5 = 縁取り + 前景反転の混成) で解消。

### A11y (FR-A11Y-001..002)

- **FR-A11Y-001** *(ubiquitous)* — システムは click / Enter / Space / tap で動作するすべての control 要素 (button / role=tab / role=radio / role=listbox option / role=menuitem / 行 selectable) の getBoundingClientRect().width および height を 44px 以上に保たなければならない。対象は SessionList row / MainTabs tab / close-back button / NotificationToast item dismiss / UndoSnackbar Undo button / ThemeSegmentedControl segment / HamburgerToggle / CommandSearchTrigger / palette listbox option を含む。
  - *Rationale*: ux.md Edge 44px。否定役指摘 (網羅性) を enum 列挙で具体化。
- **FR-A11Y-002** *(ubiquitous)* — システムは aria 状態属性 (HamburgerToggle aria-expanded / ThemeSegmentedControl aria-checked / palette dialog aria-modal / SessionDrawer aria-modal / MainTabs aria-selected / SessionList option aria-selected / disabled 行 aria-disabled) を対応する内部状態の変化と同 microtask 内 (React の同 batch) で更新しなければならない。
  - *Rationale*: 全 a11y 同期。

### Store / Wire / Framework (FR-STORE-001 / WIRE-001 / FRAMEWORK-001)

- **FR-STORE-001** *(ubiquitous)* — システムは新規 Zustand store slice として src/client/web/src/store/theme.ts のみを追加し、theme = 'system' | 'light' | 'dark' を pure に保持しなければならない。drawer.open / previousActiveSessionId / undoSnackbar.visible は AppShell および UndoSnackbar の React useState (UI-local state) で表現し、Zustand store には乗せてはならない。すべての store slice は DOM を操作してはならない (DOM 反映は React の useEffect / useLayoutEffect 経由のみ)。
  - *Rationale*: ux.md assumption (新 UI 状態は最小 UI-local) + 最適化役指摘 (DrawerStoreSlice YAGNI)。
- **FR-WIRE-001** *(ubiquitous)* — システムは本刷新で src/client/web/src/wire/* および Go ミラー側の wire/persistence 型を変更してはならず、daemon の view-update payload に activeSessionID / theme / drawer.open を乗せてはならない。activeSessionID は client 単独管理を維持する。
  - *Rationale*: 既存 MEMORY web_active_session_ownership + ADR-0023 不変条件。
- **FR-FRAMEWORK-001** *(ubiquitous)* — システムは src/client/web/src/css/ 配下のスタイル定義を tokens.css (semantic CSS custom property + breakpoint variable) / app.css (構造 + コンポーネント) / view.css (アニメーション + reduced-motion guard) の 3 ファイルに分離して維持し、各ファイルは 500 行以下を保たなければならない。
  - *Rationale*: 否定役指摘 (app.css 469 行が 500 行制約に逼迫) を tokens.css 分離で解消。NFR-CSS-001 を 2→3 ファイルに改定する正典宣言。

## Acceptance Mapping (UAC ↔ FR)

| UAC | FR |
|---|---|
| UAC-001 (off-canvas + ☰ + 横スクロールなし) | FR-LAYOUT-001, FR-LAYOUT-002 |
| UAC-002 (drawer aria-modal + inert + 三重 guard) | FR-DRAWER-001, FR-DRAWER-002 |
| UAC-003 (選択クローズ + Undo) | FR-DRAWER-003, FR-DRAWER-004 |
| UAC-004 (取消クローズ 3 経路 + activeSession 変更なし) | FR-DRAWER-005, FR-DRAWER-006 |
| UAC-005 (system 連動初回) | FR-THEME-001 |
| UAC-006 (Light/Dark 明示切替 + xterm 連動) | FR-THEME-002, FR-THEME-003, FR-THEME-007 |
| UAC-007 (localStorage 永続 + System 復帰) | FR-THEME-004, FR-THEME-005 |
| UAC-008 (palette スマホ sheet 全幅) | FR-PALETTE-TRIGGER-001 |
| UAC-009 (palette disabled visible+skip) | FR-PALETTE-NAV-001 |
| UAC-010 (palette IME 抑止) | FR-PALETTE-IME-001 |
| UAC-011 (SessionList と palette の token 共有) | FR-TOKEN-001, FR-TOKEN-002 |
| UAC-012 (palette 送信失敗 inline) | FR-PALETTE-FAIL-001 |
| UAC-013 (PC layout + 375px running) | FR-LAYOUT-003, FR-RUNSTATE-001 |
| UAC-014 (MainTabs APG + scrollback 保持) | FR-TABS-001, FR-TABS-002, FR-TABS-003 |
| UAC-015 (dvh + safe-area + 二重スクロールなし) | FR-LAYOUT-004, FR-TERMINAL-001, FR-TERMINAL-002 |
| UAC-016 (reduced-motion) | FR-MOTION-001, FR-MOTION-002 |
| Edge トースト | FR-TOAST-001, FR-TOAST-002, FR-TOAST-003 |
| Edge TagPill コントラスト | FR-TAGPILL-001 |
| Edge 44px tap target | FR-A11Y-001 |
| Edge aria 同期 | FR-A11Y-002 |
| Edge OS テーマ追従 | FR-THEME-006 |
| Edge breakpoint 跨ぎ drawer | FR-DRAWER-007 |
| 構造前提 (store pure / wire 不変 / 3 ファイル CSS) | FR-STORE-001, FR-WIRE-001, FR-FRAMEWORK-001 |

## Open Questions

> 設計判断ではなく plan-impl 段階で grep / 観察により確定する implementation-time の確認事項。

- breakpoint 境界値 (<768 / 768-1024 / >=1024 は初期案) の最終確定。sidebar 最小幅 (PC のセッションラベル可読性とターミナル最小桁数) およびタブレット帯の sidebar 折り畳み/常設の閾値は実機ベースで検証してから ADR-0060 を amend する。現 spec は 375px を観察基準に固定 (境界値が動いてもスマホ帯の観察は安定)。
- empty state ヒント文言 ('Sessions are in the menu ☰' / 'Press ⌘K to start a new session' / 'Theme settings are in the command menu') の最終確定 (i18n を考慮した一義性、ADR-0049 no-japanese gate と整合)。
- タブレットの sidebar 折り畳み時の icon-rail UI の具体形 (sidebar collapse toggle の置き場所 / icon のみで displayLabel をどう代替するか)。タブレット帯の UAC を ux.md が明示的に書いていないため、現 spec は PC と同 grid + sidebar 折り畳み可で degrade する方針までを縛り、collapse toggle の UX 詳細は後続 plan-ux に委ねる。
