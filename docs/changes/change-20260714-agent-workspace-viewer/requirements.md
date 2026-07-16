---
change: change-20260714-agent-workspace-viewer
role: requirements
---

# Requirements

## Legacy Source (verbatim)

````markdown
---
id: spec-20260714-agent-workspace-viewer
kind: spec
title: Agent workspace read-only viewer
status: approved
created: '2026-07-14'
updated: '2026-07-14'
tags:
- workspace-viewer
- read-only
- sdd
owners: []
functional_requirements:
- id: FR-001
  statement: While the session view is displayed, the system shall render an Activity
    Rail alongside the terminal, showing zero rows before any tool-call-containing
    turn completes.
  priority: must
- id: FR-002
  statement: When a turn containing at least one file-touching tool call (read/create/edit/delete)
    completes, the system shall append or update exactly one activity row per (turn,
    workspace-relative path) pair, attach an operation-count badge when the count
    exceeds 1, and expose drill-down to the individual events.
  priority: must
- id: FR-003
  statement: The system shall render every path in activity rows, drawer header, and
    tree using workspace-root-relative form, and shall never render host or container
    absolute paths (exp-workspace-relative-paths).
  priority: must
- id: FR-004
  statement: 'When the operator selects an activity row via pointer click or Enter,
    the system shall open a modal Workspace Drawer whose content pane is switched
    by event kind: read/create -> full-content viewer as default, edit -> diff view
    as default tab with viewer as second tab, delete -> metadata-only placeholder,
    and shall never present an empty content pane.'
  priority: must
- id: FR-005
  statement: While the Workspace Drawer is open, the system shall keep background
    Activity Rail row emission and Terminal output visually live, and shall not queue
    or freeze either surface until the drawer closes (exp-live-background).
  priority: must
- id: FR-006
  statement: When a subsequent file-mutating tool call is observed against the file
    backing an open drawer view, the system shall present a visible stale banner and
    issue exactly one screen-reader announcement per stale transition before any further
    UI paint reflects that file, with observable end-to-end latency (tool-call PostToolUse
    to banner render) not exceeding 500 ms.
  priority: must
- id: FR-007
  statement: When the operator activates the reload affordance while the drawer is
    stale, the system shall clear the stale banner and refresh the viewer/diff display
    to the current file/diff content.
  priority: must
- id: FR-008
  statement: The system shall make a new tool-log activity row visible in the Activity
    Rail no later than 750 ms after the underlying tool-log JSONL append (end-to-end
    append-to-render latency ceiling).
  priority: must
- id: FR-009
  statement: When a Markdown file is opened in the viewer, the system shall render
    headings, lists, and code fences as visually distinct structured output (not solely
    monospace text).
  priority: must
- id: FR-010
  statement: When a file containing Mermaid syntax or a .mmd file is opened, the system
    shall render the diagram as SVG when parsing succeeds and, upon parse failure
    or timeout, shall present a visible raw-source fallback pane instead of a blank
    pane or persistent loading state.
  priority: must
- id: FR-011
  statement: When a .json file is opened, the system shall render it as a collapsible
    key tree when parsing succeeds and, upon invalid JSON, shall present a visible
    raw-text fallback pane.
  priority: must
- id: FR-012
  statement: 'The system shall not render untrusted markdown-derived HTML that includes
    script tags, javascript: hrefs, event handler attributes, or off-workspace image
    src, and shall surface a plain-text fallback rather than partially-sanitized rich
    output when the sanitizer schema rejects the input.'
  priority: must
- id: FR-013
  statement: When an edit-event drawer is opened against a workspace whose root contains
    a valid git repository, the system shall render the diff between the git HEAD
    blob and the current working-tree content, visually distinguishing added, removed,
    and changed lines including any changes that predate the current turn.
  priority: must
- id: FR-014
  statement: If the workspace root is not a git repository or its git metadata is
    unreadable, then the system shall present a visible, explicit diff-degradation
    banner distinguishable from a normal empty diff, and shall never present a normal-looking
    diff tab.
  priority: must
- id: FR-015
  statement: The system shall never render an activity row for a file that has not
    been touched by a tool call during the session.
  priority: must
- id: FR-016
  statement: When the operator activates the always-visible Workspace secondary-tree-access
    affordance (which the rail shall render whether row count is zero or non-zero),
    the system shall open the drawer with the Tree tab as initial focus and list the
    workspace root's immediate children.
  priority: must
- id: FR-017
  statement: When the operator selects a file from the Tree tab, the system shall
    open the same viewer that activity-row selection produces for a read/create event
    on that file.
  priority: must
- id: FR-018
  statement: The system shall render the tree tab's root node as only the session's
    resolved workspace directory and shall never expose a control that navigates to
    a parent, sibling, or symlink-escaped path (exp-workspace-root-boundary).
  priority: must
- id: FR-019
  statement: When workspace files are created or deleted inside an expanded tree directory,
    the system shall reflect the change through a visible in-UI refresh affordance
    (auto-update indicator, in-drawer reload control, or tree-header refresh button)
    without requiring a full browser reload.
  priority: must
- id: FR-020
  statement: If the resolved workspace root becomes unreachable (deleted, renamed,
    or permission-lost) mid-session, then the system shall render the tree in a visibly
    degraded state with an explicit banner and shall never present a silently empty
    tree or falsely-successful response.
  priority: must
- id: FR-021
  statement: While the viewer has focus, the system shall interpret j, k, gg, G as
    cursor/viewport motion and /, n, N as incremental search-and-jump.
  priority: must
- id: FR-022
  statement: If the operator presses insert-mode or write-family keys (i, o, dd, :w,
    and any other non-motion/non-search key) while the viewer has focus, then the
    system shall neither modify the displayed content nor issue any write, save, or
    persistence call, and shall not leak the keystroke to the terminal xterm.js input
    handler (exp-vim-no-mutation).
  priority: must
- id: FR-023
  statement: While a text file exceeds the configured large-file byte threshold, the
    system shall provide virtualized scrolling reaching the true end of file and shall
    never render a truncation banner.
  priority: must
- id: FR-024
  statement: When a binary file is opened as determined by content sniffing rather
    than extension alone, the system shall render only file name, byte size, and MIME
    type metadata and shall not attempt any textual or raw-byte rendering.
  priority: must
- id: FR-025
  statement: The system shall never filter, mask, gate, or require reveal steps for
    any file's content based on name or sensitivity heuristics (.env, secrets, keys),
    and shall serve every file inside the workspace root verbatim (exp-no-sensitive-filter).
  priority: must
- id: FR-026
  statement: The system shall not expose any HTTP verb other than GET on any route
    under the workspace-viewer path prefix, and no source file in the workspace-viewer
    handler package shall reference an fs-mutating call (os.WriteFile, os.Create,
    os.Remove, os.Rename, os.Chmod, io.Writer sinks over a workspace path) as enforced
    by a depguard rule (exp-read-only, cc-no-write).
  priority: must
- id: FR-027
  statement: The system shall not alter the AppShell named-grid-area layout contract
    or the MainTabs exclusive single-active-tab paradigm; the terminal slot's rendered
    rect shall remain identical whether ActivityRail is mounted or unmounted (cc-appshell-preserve).
  priority: must
- id: FR-028
  statement: While one Workspace Drawer session remains open, the system shall resolve
    every tree, file, and diff request against a single workspace root snapshotted
    at drawer open, even if the session's frame stack changes underneath the drawer.
  priority: must
- id: FR-029
  statement: If a client-supplied path resolves (after per-segment symlink evaluation)
    to any location outside the session's resolved workspace root, then the system
    shall reject the request before opening any file and shall never return content
    from outside the root.
  priority: must
non_functional_requirements:
- id: NFR-001
  type: scalability
  criteria: Tree rendering remains interactive (<250 ms per expand action p95) for
    workspace directories containing >=1000 immediate children via lazy/deferred expansion.
  measurement: Playwright browser-smoke bench harness against a synthetic 1000-child
    workspace fixture; measured wall-clock between expand click and children rendered.
- id: NFR-002
  type: performance
  criteria: JSON tree renderer parses and initially paints a 5 MiB JSON file within
    1500 ms on the reference desktop tier and yields to the event loop between top-level
    key batches so the tab remains interactive.
  measurement: T1 vitest performance test with fixture 5 MiB JSON; assert first-paint
    elapsed and per-batch yield count.
- id: NFR-003
  type: security
  criteria: 'Markdown renderer strips or refuses all script tags, on* event attributes,
    javascript:/data: hrefs, and non-workspace image src references; sanitizer rejection
    triggers a plain-text fallback rather than partial output.'
  measurement: T2 contract test feeds a malicious markdown fixture and asserts no
    forbidden token appears in rendered DOM.
- id: NFR-004
  type: reliability
  criteria: Structured render (Mermaid or JSON) parse timeout upper bound is 300 ms
    per file; on exceed, the fallback pane renders within an additional 100 ms.
  measurement: T1 vitest test with pathological malformed input asserts fallback DOM
    appears within 400 ms.
- id: NFR-005
  type: maintainability
  criteria: The workspace-viewer HTTP handler package shall carry a depguard rule
    (in src/.golangci.yml) forbidding os.WriteFile/os.Create/os.Remove/os.Rename/os.Chmod/os.OpenFile-with-write-flag
    imports.
  measurement: '`make lint` fails when a synthetic patch introduces an fs-mutating
    call in the workspace-prefixed package.'
acceptance:
- id: AC-001
  given: session の activity rail が表示されており row が 0 件である
  when: エージェントが src/foo.ts に対して read tool call を実行しその turn が完了する
  then: rail に 1 件 row が追加され、path 表示が src/foo.ts (workspace root 相対) であり host/container
    絶対 path は表示されない
  requirement_refs:
  - FR-002
  - FR-003
- id: AC-002
  given: activity rail に row が 0 件、エージェントの現在 turn が開始した直後である
  when: 同一 turn 内でエージェントが src/foo.ts を read → edit → read の順に 3 回操作し turn が完了する
  then: rail に src/foo.ts への row が 1 件だけ表示され、count=3 の badge が付き、drill down で 3 件の個別
    event が展開表示される
  requirement_refs:
  - FR-002
- id: AC-006
  given: operator が src/foo.ts の edit event row から drawer を開いており viewer/diff に内容が表示されている
  when: drawer が開いたまま背後でエージェントが同じ src/foo.ts を再度編集する tool call を実行する
  then: 500 ms 以内に stale バナーと reload affordance が drawer 内に visible に表示され、aria-live
    で 1 回のみ announce される
  requirement_refs:
  - FR-006
- id: AC-008
  given: workspace 内に .env が存在し rail に read event row が表示されている
  when: operator がその row を選択して drawer を開く
  then: .env の内容がそのまま viewer に表示され、値のマスキングや reveal 操作の要求は一切発生しない
  requirement_refs:
  - FR-025
- id: AC-016
  given: workspace が git 管理下にない状態で rail に edit event の row が表示されている
  when: operator がその row を選択して drawer を開く
  then: diff 位置に not_a_repo の typed 劣化バナーが表示され、通常の diff タブが無警告で表示されることは無い
  requirement_refs:
  - FR-014
- id: AC-022
  given: viewer にフォーカスがあり対象ファイル (5 行以上) を表示している
  when: operator が dd を押す
  then: viewer 上の行はどれも削除・空行化されず表示前と同一の内容が表示される; workspace 上の対象ファイルの内容も変化しない
  requirement_refs:
  - FR-022
relations:
- type: implements
  target: ux-20260713-agent-workspace-viewer
- type: implementedBy
  target: plan-20260714-agent-workspace-viewer
source_paths:
- src/client/web/src/components/AppShell.tsx
- src/client/web/src/components/MainTabs.tsx
- src/client/web/src/components/SessionDrawer.tsx
- src/client/runtime/tool_log.go
- src/server/web/transcript.go
- src/platform/lib/git/git.go
methodology: sdd
summary: agent-grid web UI の workspace read-only viewer 仕様 (activity rail + modal
  drawer + Tree tab + git HEAD diff + read-only vim motion + structured content render
  + sensitive-file exposure guarantee)。
---

# Agent workspace read-only viewer

## Goal

agent-grid web UI 上で、operator がエージェントセッションの workspace を read-only で追跡・確認できる viewer (turn-aggregated activity rail + live-background modal drawer + structured content rendering + git HEAD diff + read-only vim motion + workspace-root-bounded tree) を追加する。

## Scope

### In scope

- 既存 App.tsx 構成に turn-aggregated Activity Rail を追加 (AppShell main grid area 内 wrapping container)
- Modal Workspace Drawer (Viewer / Diff / Tree タブ) を新設し、live-background で表示 (背後 terminal/rail は inert 化されず視覚更新継続)
- Markdown 整形表示 (+ rehype-sanitize XSS defense + parse error 時 raw fallback)、Mermaid SVG 描画 (+ 300ms parse timeout + raw fallback)、JSON 折りたたみツリー (+ invalid JSON raw fallback)、source viewer 上の read-only vim motion (j/k/gg/G + /,n,N)
- edit event の diff を git HEAD 基準で表示; not_a_repo / git_metadata_corrupted / git_binary_missing の 3 typed 劣化表示
- activity row を経由しない Workspace secondary-tree affordance (row 0 件でも always visible) から Tree タブを開ける
- 1 MiB 超テキストの virtualized scroll (truncate なし) + binary metadata-only placeholder
- workspace root 境界内に限定した read-only backend endpoint (tree / file / diff / root-handle); per-segment EvalSymlinks による path traversal 構造禁止; depguard rule で os.WriteFile 系の import を静的禁止
- 既存 write-only tool-call log を schema_version=2 に upgrade (turn_id + normalized kind); system 初の reader を新規追加
- row / drawer header / tree いずれの path 表示も workspace-relative に統一

### Out of scope

- 書き込み・保存 API の追加 (cc-no-write; contract-no-write-boundary で depguard 強制)
- 複数 workspace root の横断閲覧 (DP-TREE-ROOT-BOUNDARY=OPT-ROOT-WORKSPACE-ONLY)
- filesystem watch を event source とすること (DP-EVENT-SOURCE=OPT-TOOL-CALL-LOG)
- 現在セッション以外の履歴再生 (DP-HISTORY-MODE=OPT-LIVE-ONLY)
- sensitive file のフィルタ/マスキング (DP-SENSITIVE-FILE-EXPOSURE=OPT-NO-FILTER)
- vim insert mode / :w 系の実編集コマンド (exp-vim-no-mutation)
- AppShell の named-grid-area 契約や MainTabs の exclusive-tab パラダイムの変更 (cc-appshell-preserve; contract-appshell-preserve で bounding-rect regression 強制)
- Claude driver における Bash-based delete の row 表示 (v1 では structured signal のみで row を出す; unclassified diagnostic counter で観測)

## Functional Requirements

### FR-001 (state_driven, must)

While the session view is displayed, the system shall render an Activity Rail alongside the terminal, showing zero rows before any tool-call-containing turn completes.

_Rationale_: F-001, UAC-001 initial state

{% req id="FR-001" %}
While the session view is displayed, the system shall render an Activity Rail alongside the terminal, showing zero rows before any tool-call-containing turn completes.
{% /req %}

### FR-002 (event_driven, must)

When a turn containing at least one file-touching tool call (read/create/edit/delete) completes, the system shall append or update exactly one activity row per (turn, workspace-relative path) pair, attach an operation-count badge when the count exceeds 1, and expose drill-down to the individual events.

_Rationale_: F-001, UAC-001, UAC-002, exp-turn-aggregation

{% req id="FR-002" %}
When a turn containing at least one file-touching tool call (read/create/edit/delete) completes, the system shall append or update exactly one activity row per (turn, workspace-relative path) pair, attach an operation-count badge when the count exceeds 1, and expose drill-down to the individual events.
{% /req %}

### FR-003 (ubiquitous, must)

The system shall render every path in activity rows, drawer header, and tree using workspace-root-relative form, and shall never render host or container absolute paths (exp-workspace-relative-paths).

_Rationale_: UAC-001, DP-PATH-DISPLAY-SCOPE

{% req id="FR-003" %}
The system shall render every path in activity rows, drawer header, and tree using workspace-root-relative form, and shall never render host or container absolute paths (exp-workspace-relative-paths).
{% /req %}

### FR-004 (event_driven, must)

When the operator selects an activity row via pointer click or Enter, the system shall open a modal Workspace Drawer whose content pane is switched by event kind: read/create -> full-content viewer as default, edit -> diff view as default tab with viewer as second tab, delete -> metadata-only placeholder, and shall never present an empty content pane.

_Rationale_: F-002, UAC-003..006

{% req id="FR-004" %}
When the operator selects an activity row via pointer click or Enter, the system shall open a modal Workspace Drawer whose content pane is switched by event kind: read/create -> full-content viewer as default, edit -> diff view as default tab with viewer as second tab, delete -> metadata-only placeholder, and shall never present an empty content pane.
{% /req %}

### FR-005 (state_driven, must)

While the Workspace Drawer is open, the system shall keep background Activity Rail row emission and Terminal output visually live, and shall not queue or freeze either surface until the drawer closes (exp-live-background).

_Rationale_: F-002, UAC-009

{% req id="FR-005" %}
While the Workspace Drawer is open, the system shall keep background Activity Rail row emission and Terminal output visually live, and shall not queue or freeze either surface until the drawer closes (exp-live-background).
{% /req %}

### FR-006 (event_driven, must)

When a subsequent file-mutating tool call is observed against the file backing an open drawer view, the system shall present a visible stale banner and issue exactly one screen-reader announcement per stale transition before any further UI paint reflects that file, with observable end-to-end latency (tool-call PostToolUse to banner render) not exceeding 500 ms.

_Rationale_: F-002, UAC-007, exp-live-background silent-stale prohibition

{% req id="FR-006" %}
When a subsequent file-mutating tool call is observed against the file backing an open drawer view, the system shall present a visible stale banner and issue exactly one screen-reader announcement per stale transition before any further UI paint reflects that file, with observable end-to-end latency (tool-call PostToolUse to banner render) not exceeding 500 ms.
{% /req %}

### FR-007 (event_driven, must)

When the operator activates the reload affordance while the drawer is stale, the system shall clear the stale banner and refresh the viewer/diff display to the current file/diff content.

_Rationale_: F-002, UAC-010

{% req id="FR-007" %}
When the operator activates the reload affordance while the drawer is stale, the system shall clear the stale banner and refresh the viewer/diff display to the current file/diff content.
{% /req %}

### FR-008 (ubiquitous, must)

The system shall make a new tool-log activity row visible in the Activity Rail no later than 750 ms after the underlying tool-log JSONL append (end-to-end append-to-render latency ceiling).

_Rationale_: UAC-009 exp-live-background latency invariant

{% req id="FR-008" %}
The system shall make a new tool-log activity row visible in the Activity Rail no later than 750 ms after the underlying tool-log JSONL append (end-to-end append-to-render latency ceiling).
{% /req %}

### FR-009 (event_driven, must)

When a Markdown file is opened in the viewer, the system shall render headings, lists, and code fences as visually distinct structured output (not solely monospace text).

_Rationale_: F-003, UAC-011

{% req id="FR-009" %}
When a Markdown file is opened in the viewer, the system shall render headings, lists, and code fences as visually distinct structured output (not solely monospace text).
{% /req %}

### FR-010 (event_driven, must)

When a file containing Mermaid syntax or a .mmd file is opened, the system shall render the diagram as SVG when parsing succeeds and, upon parse failure or timeout, shall present a visible raw-source fallback pane instead of a blank pane or persistent loading state.

_Rationale_: F-003, UAC-012, handoff-structured-render-fallback

{% req id="FR-010" %}
When a file containing Mermaid syntax or a .mmd file is opened, the system shall render the diagram as SVG when parsing succeeds and, upon parse failure or timeout, shall present a visible raw-source fallback pane instead of a blank pane or persistent loading state.
{% /req %}

### FR-011 (event_driven, must)

When a .json file is opened, the system shall render it as a collapsible key tree when parsing succeeds and, upon invalid JSON, shall present a visible raw-text fallback pane.

_Rationale_: F-003, UAC-013, handoff-structured-render-fallback

{% req id="FR-011" %}
When a .json file is opened, the system shall render it as a collapsible key tree when parsing succeeds and, upon invalid JSON, shall present a visible raw-text fallback pane.
{% /req %}

### FR-012 (ubiquitous, must)

The system shall not render untrusted markdown-derived HTML that includes script tags, javascript: hrefs, event handler attributes, or off-workspace image src, and shall surface a plain-text fallback rather than partially-sanitized rich output when the sanitizer schema rejects the input.

_Rationale_: exp-read-only + technology-candidate constraint (react-markdown HTML sanitization)

{% req id="FR-012" %}
The system shall not render untrusted markdown-derived HTML that includes script tags, javascript: hrefs, event handler attributes, or off-workspace image src, and shall surface a plain-text fallback rather than partially-sanitized rich output when the sanitizer schema rejects the input.
{% /req %}

### FR-013 (event_driven, must)

When an edit-event drawer is opened against a workspace whose root contains a valid git repository, the system shall render the diff between the git HEAD blob and the current working-tree content, visually distinguishing added, removed, and changed lines including any changes that predate the current turn.

_Rationale_: F-004, UAC-005, UAC-014, exp-git-head-diff-base

{% req id="FR-013" %}
When an edit-event drawer is opened against a workspace whose root contains a valid git repository, the system shall render the diff between the git HEAD blob and the current working-tree content, visually distinguishing added, removed, and changed lines including any changes that predate the current turn.
{% /req %}

### FR-014 (unwanted, must)

If the workspace root is not a git repository or its git metadata is unreadable, then the system shall present a visible, explicit diff-degradation banner distinguishable from a normal empty diff, and shall never present a normal-looking diff tab.

_Rationale_: F-004, UAC-016, exp-git-head-diff-base non-git failure path

{% req id="FR-014" %}
If the workspace root is not a git repository or its git metadata is unreadable, then the system shall present a visible, explicit diff-degradation banner distinguishable from a normal empty diff, and shall never present a normal-looking diff tab.
{% /req %}

### FR-015 (ubiquitous, must)

The system shall never render an activity row for a file that has not been touched by a tool call during the session.

_Rationale_: UAC-015

{% req id="FR-015" %}
The system shall never render an activity row for a file that has not been touched by a tool call during the session.
{% /req %}

### FR-016 (event_driven, must)

When the operator activates the always-visible Workspace secondary-tree-access affordance (which the rail shall render whether row count is zero or non-zero), the system shall open the drawer with the Tree tab as initial focus and list the workspace root's immediate children.

_Rationale_: F-005, UAC-017, handoff-secondary-tree-entry

{% req id="FR-016" %}
When the operator activates the always-visible Workspace secondary-tree-access affordance (which the rail shall render whether row count is zero or non-zero), the system shall open the drawer with the Tree tab as initial focus and list the workspace root's immediate children.
{% /req %}

### FR-017 (event_driven, must)

When the operator selects a file from the Tree tab, the system shall open the same viewer that activity-row selection produces for a read/create event on that file.

_Rationale_: F-005, UAC-017

{% req id="FR-017" %}
When the operator selects a file from the Tree tab, the system shall open the same viewer that activity-row selection produces for a read/create event on that file.
{% /req %}

### FR-018 (ubiquitous, must)

The system shall render the tree tab's root node as only the session's resolved workspace directory and shall never expose a control that navigates to a parent, sibling, or symlink-escaped path (exp-workspace-root-boundary).

_Rationale_: UAC-018, DP-TREE-ROOT-BOUNDARY

{% req id="FR-018" %}
The system shall render the tree tab's root node as only the session's resolved workspace directory and shall never expose a control that navigates to a parent, sibling, or symlink-escaped path (exp-workspace-root-boundary).
{% /req %}

### FR-019 (event_driven, must)

When workspace files are created or deleted inside an expanded tree directory, the system shall reflect the change through a visible in-UI refresh affordance (auto-update indicator, in-drawer reload control, or tree-header refresh button) without requiring a full browser reload.

_Rationale_: F-006, UAC-019, handoff-tree-refresh

{% req id="FR-019" %}
When workspace files are created or deleted inside an expanded tree directory, the system shall reflect the change through a visible in-UI refresh affordance (auto-update indicator, in-drawer reload control, or tree-header refresh button) without requiring a full browser reload.
{% /req %}

### FR-020 (unwanted, must)

If the resolved workspace root becomes unreachable (deleted, renamed, or permission-lost) mid-session, then the system shall render the tree in a visibly degraded state with an explicit banner and shall never present a silently empty tree or falsely-successful response.

_Rationale_: ux edge_case: workspace torn down

{% req id="FR-020" %}
If the resolved workspace root becomes unreachable (deleted, renamed, or permission-lost) mid-session, then the system shall render the tree in a visibly degraded state with an explicit banner and shall never present a silently empty tree or falsely-successful response.
{% /req %}

### FR-021 (state_driven, must)

While the viewer has focus, the system shall interpret j, k, gg, G as cursor/viewport motion and /, n, N as incremental search-and-jump.

_Rationale_: F-007, UAC-020, UAC-021, handoff-vim-keymap-bindings

{% req id="FR-021" %}
While the viewer has focus, the system shall interpret j, k, gg, G as cursor/viewport motion and /, n, N as incremental search-and-jump.
{% /req %}

### FR-022 (unwanted, must)

If the operator presses insert-mode or write-family keys (i, o, dd, :w, and any other non-motion/non-search key) while the viewer has focus, then the system shall neither modify the displayed content nor issue any write, save, or persistence call, and shall not leak the keystroke to the terminal xterm.js input handler (exp-vim-no-mutation).

_Rationale_: F-007, UAC-022, UAC-023

{% req id="FR-022" %}
If the operator presses insert-mode or write-family keys (i, o, dd, :w, and any other non-motion/non-search key) while the viewer has focus, then the system shall neither modify the displayed content nor issue any write, save, or persistence call, and shall not leak the keystroke to the terminal xterm.js input handler (exp-vim-no-mutation).
{% /req %}

### FR-023 (state_driven, must)

While a text file exceeds the configured large-file byte threshold, the system shall provide virtualized scrolling reaching the true end of file and shall never render a truncation banner.

_Rationale_: F-008, UAC-024, handoff-large-file-threshold

{% req id="FR-023" %}
While a text file exceeds the configured large-file byte threshold, the system shall provide virtualized scrolling reaching the true end of file and shall never render a truncation banner.
{% /req %}

### FR-024 (event_driven, must)

When a binary file is opened as determined by content sniffing rather than extension alone, the system shall render only file name, byte size, and MIME type metadata and shall not attempt any textual or raw-byte rendering.

_Rationale_: F-008, UAC-025

{% req id="FR-024" %}
When a binary file is opened as determined by content sniffing rather than extension alone, the system shall render only file name, byte size, and MIME type metadata and shall not attempt any textual or raw-byte rendering.
{% /req %}

### FR-025 (ubiquitous, must)

The system shall never filter, mask, gate, or require reveal steps for any file's content based on name or sensitivity heuristics (.env, secrets, keys), and shall serve every file inside the workspace root verbatim (exp-no-sensitive-filter).

_Rationale_: UAC-008, DP-SENSITIVE-FILE-EXPOSURE

{% req id="FR-025" %}
The system shall never filter, mask, gate, or require reveal steps for any file's content based on name or sensitivity heuristics (.env, secrets, keys), and shall serve every file inside the workspace root verbatim (exp-no-sensitive-filter).
{% /req %}

### FR-026 (ubiquitous, must)

The system shall not expose any HTTP verb other than GET on any route under the workspace-viewer path prefix, and no source file in the workspace-viewer handler package shall reference an fs-mutating call (os.WriteFile, os.Create, os.Remove, os.Rename, os.Chmod, io.Writer sinks over a workspace path) as enforced by a depguard rule (exp-read-only, cc-no-write).

_Rationale_: cc-no-write structural enforcement, exp-read-only

{% req id="FR-026" %}
The system shall not expose any HTTP verb other than GET on any route under the workspace-viewer path prefix, and no source file in the workspace-viewer handler package shall reference an fs-mutating call (os.WriteFile, os.Create, os.Remove, os.Rename, os.Chmod, io.Writer sinks over a workspace path) as enforced by a depguard rule (exp-read-only, cc-no-write).
{% /req %}

### FR-027 (ubiquitous, must)

The system shall not alter the AppShell named-grid-area layout contract or the MainTabs exclusive single-active-tab paradigm; the terminal slot's rendered rect shall remain identical whether ActivityRail is mounted or unmounted (cc-appshell-preserve).

_Rationale_: cc-appshell-preserve

{% req id="FR-027" %}
The system shall not alter the AppShell named-grid-area layout contract or the MainTabs exclusive single-active-tab paradigm; the terminal slot's rendered rect shall remain identical whether ActivityRail is mounted or unmounted (cc-appshell-preserve).
{% /req %}

### FR-028 (state_driven, must)

While one Workspace Drawer session remains open, the system shall resolve every tree, file, and diff request against a single workspace root snapshotted at drawer open, even if the session's frame stack changes underneath the drawer.

_Rationale_: issue-workspace-root-drawer-lifetime-consistency

{% req id="FR-028" %}
While one Workspace Drawer session remains open, the system shall resolve every tree, file, and diff request against a single workspace root snapshotted at drawer open, even if the session's frame stack changes underneath the drawer.
{% /req %}

### FR-029 (unwanted, must)

If a client-supplied path resolves (after per-segment symlink evaluation) to any location outside the session's resolved workspace root, then the system shall reject the request before opening any file and shall never return content from outside the root.

_Rationale_: issue-symlink-resolution-policy-open, DP-TREE-ROOT-BOUNDARY

{% req id="FR-029" %}
If a client-supplied path resolves (after per-segment symlink evaluation) to any location outside the session's resolved workspace root, then the system shall reject the request before opening any file and shall never return content from outside the root.
{% /req %}

## Non-Functional Requirements

- **NFR-001** (scalability) — Tree rendering remains interactive (<250 ms per expand action p95) for workspace directories containing >=1000 immediate children via lazy/deferred expansion. _Measurement_: Playwright browser-smoke bench harness against a synthetic 1000-child workspace fixture; measured wall-clock between expand click and children rendered.
- **NFR-002** (performance) — JSON tree renderer parses and initially paints a 5 MiB JSON file within 1500 ms on the reference desktop tier and yields to the event loop between top-level key batches so the tab remains interactive. _Measurement_: T1 vitest performance test with fixture 5 MiB JSON; assert first-paint elapsed and per-batch yield count.
- **NFR-003** (security) — Markdown renderer strips or refuses all script tags, on* event attributes, javascript:/data: hrefs, and non-workspace image src references; sanitizer rejection triggers a plain-text fallback rather than partial output. _Measurement_: T2 contract test feeds a malicious markdown fixture and asserts no forbidden token appears in rendered DOM.
- **NFR-004** (reliability) — Structured render (Mermaid or JSON) parse timeout upper bound is 300 ms per file; on exceed, the fallback pane renders within an additional 100 ms. _Measurement_: T1 vitest test with pathological malformed input asserts fallback DOM appears within 400 ms.
- **NFR-005** (maintainability) — The workspace-viewer HTTP handler package shall carry a depguard rule (in src/.golangci.yml) forbidding os.WriteFile/os.Create/os.Remove/os.Rename/os.Chmod/os.OpenFile-with-write-flag imports. _Measurement_: `make lint` fails when a synthetic patch introduces an fs-mutating call in the workspace-prefixed package.

## Acceptance Scenarios

### AC-001

- **Given**: session の activity rail が表示されており row が 0 件である
- **When**: エージェントが src/foo.ts に対して read tool call を実行しその turn が完了する
- **Then**: rail に 1 件 row が追加され、path 表示が src/foo.ts (workspace root 相対) であり host/container 絶対 path は表示されない
- **Requirements**: FR-002, FR-003

{% acceptance id="AC-001" %}
Given session の activity rail が表示されており row が 0 件である When エージェントが src/foo.ts に対して read tool call を実行しその turn が完了する Then rail に 1 件 row が追加され、path 表示が src/foo.ts (workspace root 相対) であり host/container 絶対 path は表示されない
{% /acceptance %}

### AC-002

- **Given**: activity rail に row が 0 件、エージェントの現在 turn が開始した直後である
- **When**: 同一 turn 内でエージェントが src/foo.ts を read → edit → read の順に 3 回操作し turn が完了する
- **Then**: rail に src/foo.ts への row が 1 件だけ表示され、count=3 の badge が付き、drill down で 3 件の個別 event が展開表示される
- **Requirements**: FR-002

{% acceptance id="AC-002" %}
Given activity rail に row が 0 件、エージェントの現在 turn が開始した直後である When 同一 turn 内でエージェントが src/foo.ts を read → edit → read の順に 3 回操作し turn が完了する Then rail に src/foo.ts への row が 1 件だけ表示され、count=3 の badge が付き、drill down で 3 件の個別 event が展開表示される
{% /acceptance %}

### AC-006

- **Given**: operator が src/foo.ts の edit event row から drawer を開いており viewer/diff に内容が表示されている
- **When**: drawer が開いたまま背後でエージェントが同じ src/foo.ts を再度編集する tool call を実行する
- **Then**: 500 ms 以内に stale バナーと reload affordance が drawer 内に visible に表示され、aria-live で 1 回のみ announce される
- **Requirements**: FR-006

{% acceptance id="AC-006" %}
Given operator が src/foo.ts の edit event row から drawer を開いており viewer/diff に内容が表示されている When drawer が開いたまま背後でエージェントが同じ src/foo.ts を再度編集する tool call を実行する Then 500 ms 以内に stale バナーと reload affordance が drawer 内に visible に表示され、aria-live で 1 回のみ announce される
{% /acceptance %}

### AC-008

- **Given**: workspace 内に .env が存在し rail に read event row が表示されている
- **When**: operator がその row を選択して drawer を開く
- **Then**: .env の内容がそのまま viewer に表示され、値のマスキングや reveal 操作の要求は一切発生しない
- **Requirements**: FR-025

{% acceptance id="AC-008" %}
Given workspace 内に .env が存在し rail に read event row が表示されている When operator がその row を選択して drawer を開く Then .env の内容がそのまま viewer に表示され、値のマスキングや reveal 操作の要求は一切発生しない
{% /acceptance %}

### AC-016

- **Given**: workspace が git 管理下にない状態で rail に edit event の row が表示されている
- **When**: operator がその row を選択して drawer を開く
- **Then**: diff 位置に not_a_repo の typed 劣化バナーが表示され、通常の diff タブが無警告で表示されることは無い
- **Requirements**: FR-014

{% acceptance id="AC-016" %}
Given workspace が git 管理下にない状態で rail に edit event の row が表示されている When operator がその row を選択して drawer を開く Then diff 位置に not_a_repo の typed 劣化バナーが表示され、通常の diff タブが無警告で表示されることは無い
{% /acceptance %}

### AC-022

- **Given**: viewer にフォーカスがあり対象ファイル (5 行以上) を表示している
- **When**: operator が dd を押す
- **Then**: viewer 上の行はどれも削除・空行化されず表示前と同一の内容が表示される; workspace 上の対象ファイルの内容も変化しない
- **Requirements**: FR-022

{% acceptance id="AC-022" %}
Given viewer にフォーカスがあり対象ファイル (5 行以上) を表示している When operator が dd を押す Then viewer 上の行はどれも削除・空行化されず表示前と同一の内容が表示される; workspace 上の対象ファイルの内容も変化しない
{% /acceptance %}


````
