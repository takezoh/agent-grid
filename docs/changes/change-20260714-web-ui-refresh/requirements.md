---
change: change-20260714-web-ui-refresh
role: requirements
---

# Requirements

## Legacy Source (verbatim)

````markdown
---
id: spec-20260714-web-ui-refresh
kind: spec
title: Web UI Refresh — Specification
status: draft
created: '2026-07-14'
tags:
- spec
- web
- ui-refresh
owners: []
functional_requirements:
- id: FR-001
  statement: The system shall define a two-layer token system in tokens.css — a primitive
    scale layer (color ramp, --space-1..8, --text-xs..xl, --radius-1..3/full, --motion-1..3,
    --font-ui, --font-mono) and a semantic alias layer that resolves only to primitives.
  priority: must
- id: FR-002
  statement: The system shall preserve existing semantic token names consumed by components
    (--bg, --fg, --accent, --border-color, --focus-ring, row tokens, toast tokens)
    as aliases so component CSS keeps working during incremental migration.
  priority: must
- id: FR-003
  statement: The system shall expose exactly one status color family (--status-running/waiting/idle/stopped/pending);
    --session-status-* is removed and its consumers migrated in the same change.
  priority: must
- id: FR-004
  statement: The system shall render UI text in --font-ui (system sans stack) and
    render terminal content, file paths, IDs, timestamps, and numeric metadata in
    --font-mono.
  priority: must
- id: FR-005
  statement: When the theme resolves to light or dark, the system shall derive all
    component surface colors exclusively from semantic tokens.
  priority: must
- id: FR-006
  statement: If a component CSS file other than tokens.css contains a hex/rgb color
    literal, then the system shall fail the static guard test (extension of tokens-css-structure).
  priority: must
- id: FR-007
  statement: The system shall define motion durations and easing only via --motion-1
    (120ms), --motion-2 (180ms), --motion-3 (240ms) with cubic-bezier(.2,.7,.3,1),
    and the prefers-reduced-motion guard shall zero these tokens globally.
  priority: must
- id: FR-008
  statement: The system shall lay out the sidebar top-to-bottom as brand row (logo
    + product name + Cmd/Ctrl+K hint), workspace switcher (existing, when >=2), project
    groups with session rows, and a bottom-anchored New session button.
  priority: must
- id: FR-009
  statement: The system shall render a session row as status dot (color + shape per
    state), single-line ellipsized title, right-aligned relative age, and a second
    mono metadata line (driver · model · effort); driver tag chips shall not appear
    in the title line.
  priority: must
- id: FR-010
  statement: When a session becomes active, the system shall show the selected state
    as a filled background (--accent-soft) while keyboard focus is indicated only
    by --focus-ring outline.
  priority: must
- id: FR-011
  statement: The system shall render a 44px header containing breadcrumb basename(project)
    / session title, status pill, mono metadata, and right-aligned icon actions (stop
    session, overflow menu).
  priority: must
- id: FR-012
  statement: The system shall host theme selection in the overflow menu and keep the
    agent-grid-theme localStorage contract of ThemeProvider (set on light/dark, remove
    on system).
  priority: must
- id: FR-013
  statement: While the viewport is narrower than 768px, the system shall place the
    hamburger at the left edge of the header and show the active session title, reusing
    the existing SessionDrawer behavior unchanged.
  priority: must
- id: FR-014
  statement: The system shall move the command search trigger into the sidebar brand
    row as the Cmd/Ctrl+K affordance and remove the dedicated centered header search
    bar, keeping all palette open paths functional.
  priority: must
- id: FR-015
  statement: The system shall surface file-change activity as a Changes section at
    the top of the Workspace-mode side panel (above the Files tree), reading the existing
    workspaceActivity store selectors; no standalone changes panel or sheet shall
    exist outside Workspace mode. (2026-07-14 改訂 x2 — rail 解体 → panel/sheet → Workspace
    モードへ統合)
  priority: must
- id: FR-016
  statement: When a change row is activated, the system shall invoke the existing
    openDrawerFromRow flow with identical arguments.
  priority: must
- id: FR-017
  statement: The system shall render a change row as operation glyph M/A/D/R (mono,
    colored by kind), tail-truncated mono path, and occurrence count, keeping the
    existing operator-actor marker semantics with an accessible label.
  priority: must
- id: FR-018
  statement: The system shall persist a changes-panel collapse state. (2026-07-14
    撤回 — panel 廃止に伴い collapse 概念ごと削除; web.changes.collapsed キーは廃止)
  priority: wont
- id: FR-019
  statement: While in Terminal mode, the system shall give the terminal the full main-area
    width at every viewport size; changes are reached via Workspace mode. (2026-07-14
    改訂 — bottom sheet 廃止)
  priority: must
- id: FR-020
  statement: The system shall provide an Open-workspace-tree affordance in the changes
    chrome. (2026-07-14 撤回 — ツリーは Workspace モードの常設パネルになり専用アフォーダンス不要; ヘッダの Terminal
    / Workspace スイッチが導線)
  priority: wont
- id: FR-021
  statement: While the transport is degraded, the system shall render the reconnecting
    notice inside the Workspace-mode Changes section (existing transportDegraded semantics).
  priority: must
- id: FR-022
  statement: The system shall dissolve DriverViewPanel — title to header breadcrumb,
    model/effort to header mono metadata, status to header pill, status_line to bottom
    status bar, driver tags to sidebar row metadata — leaving no standalone panel
    above the tabs.
  priority: must
- id: FR-023
  statement: While a session is active, the system shall render a persistent 26px
    bottom status bar (mono, --text-faint) showing status_line left and connection
    state right (connected when nominal). (2026-07-14 改訂 — 旧「nominal 時は隠す」は操作者の現在地感覚を損なうため撤回)
  priority: must
- id: FR-024
  statement: When the stop-session icon button is activated, the system shall run
    the existing ConfirmDialog termination flow unchanged (destructive variant, pending
    state, opener focus restore).
  priority: must
- id: FR-025
  statement: The system shall render MainTabs as token-driven underline tabs while
    preserving the ARIA tabs pattern with manual activation (ADR-0061) and the always-mounted
    terminal slot (ADR-0065).
  priority: must
- id: FR-026
  statement: The system shall compose the palette dialog of input row, sectioned list
    (Actions, Push · session), and a footer context line, removing the title row,
    ACTIVE row, and persistent inline-status row.
  priority: must
- id: FR-027
  statement: When a palette action changes the active session or fails, the system
    shall route feedback to the existing notifications store (toast) and the existing
    aria-live announcer, rendering no persistent status row inside the palette.
  priority: must
- id: FR-028
  statement: While a push command is unavailable (fail-closed occupant gate), the
    system shall render it dimmed in the Push section with a reason string and keep
    activation blocked.
  priority: must
- id: FR-029
  statement: The system shall preserve palette phase-transition logic, keyboard handling,
    fuzzy matching, and the existing data-testid contract (palette-input, palette-param-*),
    changing only surrounding DOM structure and CSS.
  priority: must
- id: FR-030
  statement: The system shall render session options (worktree for git projects, host
    for sandboxed projects) in the param-phase actions row immediately before the
    confirm control. (2026-07-14 改訂 — 旧 footer 配置は確定動線から遠く操作性が悪い)
  priority: must
- id: FR-031
  statement: The system shall present the workspace as a main-area mode (Terminal
    / Workspace switch in the header) with a persistent file-tree panel to the RIGHT
    of the editor content (replacing the Changes panel while in Workspace mode). Both
    mode layers shall preserve state across switches — the terminal keeps scrollback/subscriptions
    and the workspace keeps its open file, tabs, and dirty editor buffer (mode switch
    is pure visibility; only the explicit close action ends the workspace session).
    (2026-07-14 改訂 x2 — ドロワー撤回 → 右ツリー + 状態永続)
  priority: must
- id: FR-032
  statement: The system shall render Viewer/Diff as underline tabs in the workspace
    toolbar (the tree is a persistent panel, not a tab) and show a breadcrumb of the
    open path with the dirty dot. (2026-07-14 改訂 — Tree タブ廃止)
  priority: must
- id: FR-033
  statement: The system shall render WorkspaceTree with indentation guides, chevron
    rotation for expanded dirs, and dir/file icons, persisting expansion state while
    the drawer stays open.
  priority: must
- id: FR-034
  statement: The system shall provide an editor toolbar Save control with states saved
    (disabled), dirty (enabled + dot), saving (spinner), and read-only (disabled +
    reason tooltip), calling the existing performSave.
  priority: must
- id: FR-035
  statement: When Cmd/Ctrl+S is pressed while the drawer editor has focus, the system
    shall preventDefault and invoke performSave; the shortcut shall not be captured
    when focus is elsewhere, and vim :w keeps working.
  priority: must
- id: FR-036
  statement: While the buffer is degraded to read-only (root disappearance / handle
    stale), the system shall show a banner stating the reason and keep the buffer
    preservation semantics unchanged.
  priority: must
- id: FR-037
  statement: The system shall derive the CodeMirror syntax theme, DiffViewer add/remove
    colors, and xterm ITheme wiring from semantic tokens so they follow both themes.
  priority: must
- id: FR-038
  statement: When the final parameter field receives Enter or an option click, the
    system shall move focus to an explicit confirm control without submitting; submission
    shall occur only through the confirm control. (2026-07-14 追加 — 選択と確定の分離)
  priority: must
non_functional_requirements:
- id: NFR-001
  type: usability
  criteria: 両テーマで通常テキストのコントラスト 4.5:1 以上、大テキストと UI 部品 3:1 以上 (WCAG AA)。util/contrast
    ベースのテストで検証する。
- id: NFR-002
  type: maintainability
  criteria: 新規 runtime 依存は adr-20260714-behavior-lib-and-icons で決定した 2 パッケージのみ。アイコン起因のバンドル増は
    gzip 10KB 以下 (静的 SVG のみ)。
- id: NFR-003
  type: maintainability
  criteria: 各マイルストーン境界で既存 unit / e2e 全suite が green。マイルストーンごとにスクリーンショットハーネス (opt-in
    spec) で before/after を確認する。
- id: NFR-004
  type: usability
  criteria: タッチ対象 44x44px 以上、デスクトップのアイコンボタンは実効ヒット 36px 以上。
- id: NFR-005
  type: compatibility
  criteria: agent-grid-theme localStorage キー、palette-input 等の data-testid、ARIA 契約
    (tabs manual activation / dialog roles / focus trap)、performSave と workspaceActivity
    ストアのシグネチャを退行させない。
acceptance: []
relations:
- {type: implements, target: ux-20260714-web-ui-refresh}
- {type: implementedBy, target: plan-20260714-web-ui-refresh}
- {type: referencedBy, target: adr-20260714-design-token-two-layer}
- {type: referencedBy, target: adr-20260714-changes-panel}
- {type: referencedBy, target: adr-20260714-behavior-lib-and-icons}
- {type: referencedBy, target: spec-20260714-workspace-session-switch}
source_paths:
- src/client/web/src/css/tokens.css
- src/client/web/src/components/
methodology: sdd
summary: Web UI 全面刷新の EARS 機能要求 (FR-001..037) と NFR。トークン 2 層化・シェル再構成・rail 解体・パレット刷新・エディタークローム。
---

## Overview

Web UI を Linear/Vercel 系ミニマルへ全面刷新する。受け入れシナリオの SoT は `ux-20260714-web-ui-refresh` (UAC-001..021)。設計判断は 3 本の ADR (token 2 層化 / Changes パネル / 挙動ライブラリ限定導入) を参照。

## Functional Requirements

frontmatter `functional_requirements` が SoT。マイルストーン別の対応:

| 領域 (plan milestone) | FR |
|---|---|
| m1 デザイントークン体系 | FR-001..FR-007 |
| m2 シェル・サイドバー・ヘッダ | FR-008..FR-014 |
| m3 Changes パネル (rail 解体) | FR-015..FR-021 |
| m4 メインビュー | FR-022..FR-025 |
| m5 コマンドパレット | FR-026..FR-030, FR-038 |
| m6 ドロワー・エディター・仕上げ | FR-031..FR-037 |

UAC への主な対応: FR-010→UAC-001, FR-003/009→UAC-002/003, FR-011→UAC-004, FR-012→UAC-005, FR-013→UAC-006, FR-015/017→UAC-007, FR-018→UAC-008, FR-019→UAC-009, FR-022→UAC-010, FR-024→UAC-011, FR-025→UAC-012, FR-026→UAC-013, FR-027→UAC-014, FR-028→UAC-015, FR-031→UAC-016, FR-033→UAC-017, FR-034/035→UAC-018, FR-036→UAC-019, FR-005/006→UAC-020, FR-007→UAC-021。

## Design Token Values (normative)

FR-001..FR-007 の**具体値の正本**。当初この表が無く「2 層構造 + 既存 semantic 名維持」だけが仕様化されていたため、実装が旧パレット値を temp 温存したまま構造だけ 2 層化する読みが成立してしまった (2026-07-14 divergence 調査で確認)。値の変更はこの表 → `tokens.css` primitive 層 → pinned test (`tokens-css-structure` / `token-contrast`) の順に同期する。

| 役割 (semantic) | Dark | Light |
|---|---|---|
| canvas `--bg` | `#0b0c0e` | `#fafafa` |
| surface `--bg-surface` | `#111318` | `#ffffff` |
| raised `--bg-elevated` | `#191c22` | `#ffffff` (境界は hairline) |
| text `--fg` / `--fg-muted` / `--text-faint` | `#e8eaee` / `#9aa1ad` / `#646b78` | `#17181c` / `#5d6370` / `#8a8f9c` |
| accent `--accent` | `#7c86ff` | `#4650d8` |
| accent soft `--accent-soft` | `rgba(124,134,255,.14)` | `rgba(70,80,216,.09)` |
| hairline `--border-hairline` | `rgba(255,255,255,.06)` | `rgba(0,0,0,.08)` |
| status running / waiting / stopped / pending / idle | `#3fcf7a` / `#e0b45c` / `#f2555a` / `#6b9bff` / `#646b78` | `#217a4b` / `#a06a1b` / `#c93a40` / `#3556c9` / `#6a7180` |

状態表現の規範 (UAC-001/002 の実装契約):

- 選択 (active session / listbox cursor) は**塗りのみ** — 常時 outline は禁止。focus ring は `:focus-visible` / listbox の focus-within に限定
- status バッジ/pill は **soft 塗り (`--status-*-soft`) + vivid 文字 (`--status-*`)** — ベタ塗り + 白文字は禁止 (NFR-001 コントラストは `token-contrast.test.ts` が両テーマで検証)
- `<button>` の UA デフォルトクロームは `app.css` の基底リセットで一括無効化 — クロームが必要なコンポーネントが明示的に張り直す

## Non-Goals

- wire プロトコル (`src/wire/*`) と Go 側の変更
- 挙動実装 (focus trap / listbox / dialog / palette フェーズ遷移) の置き換え
- 保存安全機構 (mtime 412 / staleness 409 / allowlist / dirty 保持) の変更
- ストアロジック (daemon / workspaceActivity / palette) の変更
- UI 文言の言語ポリシー変更

````
