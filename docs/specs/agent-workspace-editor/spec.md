---
id: spec-20260714-agent-workspace-editor
kind: spec
title: Agent workspace mutation-capable editor
status: approved
methodology: sdd
created: '2026-07-14'
updated: '2026-07-14'
tags:
- workspace-editor
- write
- sdd
owners: []
functional_requirements:
- id: FR-101
  statement: While the Workspace Drawer is in editor mode with a text buffer open
    and focused, the system shall translate vim mutation keystrokes (i/a/o/I/A/O insert-mode
    entry, dd/x/c/y/p operator+register commands) into concrete in-memory buffer edits
    and shall not silently discard them as no-ops.
  priority: must
- id: FR-102
  statement: When the operator issues the vim `:w` command from a dirty buffer, the
    system shall persist the current buffer contents to the corresponding workspace-relative
    file via the write endpoint and shall report an explicit success or typed failure
    to the operator, never a silent success or partial write.
  priority: must
- id: FR-103
  statement: The system shall accept every write request only under the same GuardWorkspacePath
    / workspace-root containment applied to reads, shall apply no sensitive-file filter
    or masking, and shall serve `.env`, `.git/config`, `*.key`, and any other conventional-sensitive
    path through the identical write flow as ordinary source files.
  priority: must
- id: FR-104
  statement: While a buffer is open, the system shall scope the vim undo (u) and redo
    (Ctrl-R) stack strictly to operator-authored edits within that buffer's current
    lifetime, shall never treat background agent writes as undo/redo steps, and shall
    clear the stack when the buffer closes or the operator accepts a conflict resolution
    that replaces its base content.
  priority: must
- id: FR-105
  statement: If the operator holds a dirty buffer for path P and a background agent
    or a different session writes P before the operator's save completes, then the
    system shall present a non-blocking conflict banner offering keep-mine / take-theirs
    / merge and shall never allow either side to silently overwrite the other.
  priority: must
- id: FR-106
  statement: When a save request is processed, the system shall complete it atomically
    (tmp-file plus os.Rename on the workspace filesystem) so that any subsequent GET
    for the same path returns either the pre-save bytes or the post-save bytes, never
    a truncated or partial file.
  priority: must
- id: FR-107
  statement: While a buffer has unsaved changes, the system shall display a dirty
    indicator on the drawer header/tab within one render frame of the mutating keystroke
    and shall present an explicit unsaved-changes warning before discarding the buffer
    on drawer close (Esc, scrim click, or programmatic close).
  priority: must
- id: FR-108
  statement: When an operator save succeeds, the system shall emit a distinct `operator`-kind
    activity row on the Activity Rail carrying the workspace-relative path and shall
    never merge it into or misattribute it as an agent tool-call row.
  priority: must
- id: FR-109
  statement: The system shall preserve every read-only vim motion and search binding
    (j/k/gg/G, /, n, N) documented for the read-only viewer unchanged and shall not
    regress any counterexample assertion from spec-20260714-agent-workspace-viewer
    contract-vim-keymap-bindings.
  priority: must
- id: FR-110
  statement: The system shall write back file contents byte-for-byte in the regions
    the operator did not edit, preserving the original text encoding and line-ending
    convention (CRLF vs LF, UTF-8 byte sequences including invalid subsequences) that
    the file carried when the buffer was opened.
  priority: must
- id: FR-111
  statement: If a write request arrives without the same Bearer token authentication
    that the workspace read handlers require, then the system shall reject it with
    the identical 401/403 typed response as the read handlers, before touching the
    workspace filesystem.
  priority: must
- id: FR-112
  statement: While a dirty buffer exists after a WebSocket transport reconnect, the
    system shall re-fetch the current server-side mtime/ETag for that path and shall
    raise a conflict banner if it differs from the snapshot taken at buffer open,
    closing the reconnect drop window rather than permitting the next `:w` to silently
    overwrite background changes.
  priority: must
- id: FR-113
  statement: If the workspace root directory is deleted or renamed while a dirty buffer
    is open, then the system shall keep the dirty buffer contents in memory, transition
    the drawer to a read-only `root_disappeared` state with a typed banner, expose
    a clipboard-export action for the dirty contents, and shall never silently discard
    the buffer.
  priority: must
non_functional_requirements:
- id: NFR-101
  type: performance
  criteria: :w keypress to visible save completion has end-to-end latency p95 ≤ 750
    ms and p50 ≤ 500 ms on typical source files (< 100 KiB) served by the local daemon.
  measurement: T1 timing test drives 50 saves through the workspace-fs-writer handler
    and asserts percentiles from ordered latency samples.
- id: NFR-102
  type: performance
  criteria: For files at or above the 1 MiB large-file threshold, per-keystroke render
    latency stays at p95 ≤ 33 ms and p99 ≤ 50 ms during a continuous burst of 200
    sequential keystrokes.
  measurement: T1 Playwright bench drives a scripted keystroke burst against a 1 MiB
    fixture and records per-frame paint latency.
- id: NFR-103
  type: scalability
  criteria: Write request body is capped at 1 MiB (matching the read-side virtualization
    threshold); over-cap requests are rejected with typed HTTP 413 before the server
    buffers the body; under a concurrent load of at most 16 in-flight writes the server
    memory footprint added by writes remains below 32 MiB.
  measurement: T2 Go integration test streams over-cap and under-cap bodies against
    the handler, asserting 413 and the peak resident-set delta on the write-handler
    subsystem.
acceptance:
- id: AC-1001
  given: operator が編集可能な text file を drawer で開いている
  when: operator が `i` を押して文字入力後 `Esc` を押す
  then: buffer 内容が入力を反映し、xterm session にキーが漏れない
  requirement_refs:
  - FR-101
  - FR-109
- id: AC-1002
  given: dirty buffer を持つ operator
  when: operator が `:w` を押す
  then: workspace 上のファイルが新内容で永続化され、drawer に成功インジケータが出る
  requirement_refs:
  - FR-102
  - FR-106
- id: AC-1003
  given: operator が `.env` を dirty で編集中
  when: operator が `:w` を押す
  then: 他 file と同一 flow で保存され、mask/reveal 追加 UI は現れない
  requirement_refs:
  - FR-103
- id: AC-1004
  given: operator が A→B→C 編集後の buffer
  when: operator が `u` を 2 回押す
  then: buffer が A 後の状態に戻り、agent の背後 write は undo stack に現れない
  requirement_refs:
  - FR-104
- id: AC-1005
  given: operator が dirty buffer を持ち、背後で agent が同 path を write
  when: operator が `:w` を押す前に conflict が検知される
  then: 非-blocking banner + keep-mine/take-theirs/merge が現れ、どちら側も silent 上書きされない
  requirement_refs:
  - FR-105
- id: AC-1006
  given: operator の save が rename ステップで失敗
  when: handler が response を返す
  then: response は typed 5xx、disk 上のファイルは pre-save バイト列のまま
  requirement_refs:
  - FR-106
- id: AC-1007
  given: dirty buffer
  when: operator が drawer を close しようとする
  then: unsaved warning が出て buffer は operator 確認まで破棄されない
  requirement_refs:
  - FR-107
- id: AC-1008
  given: operator が src/foo.ts を保存
  when: activity pipeline がイベントを処理
  then: ActivityRail に operator kind の distinct row が 1 件現れる (agent row と混同されない)
  requirement_refs:
  - FR-108
- id: AC-1009
  given: read-only viewer から motion/search テストが緑
  when: editor 差し替え後にも同一 test を実行
  then: 全 motion/search assertion が緑のまま
  requirement_refs:
  - FR-109
- id: AC-1010
  given: CRLF 終端の Windows ファイル
  when: operator が 1 行だけ編集して保存
  then: 触れていない領域が byte-identical、CRLF が保持される
  requirement_refs:
  - FR-110
- id: AC-1011
  given: 無認証の curl リクエスト
  when: PUT /workspace/file を送信
  then: handler は 401 を返し fs access を行わない
  requirement_refs:
  - FR-111
- id: AC-1012
  given: dirty buffer 中に WS が 5 秒ドロップ、gap 中に背後 write
  when: WS reconnect + operator が `:w`
  then: reconnect 直後に conflict-check re-fetch が走り、conflict banner が :w より前に現れる
  requirement_refs:
  - FR-112
- id: AC-1013
  given: dirty buffer 中に workspace root が削除される
  when: drawer が状態を更新
  then: drawer は root_disappeared に遷移、buffer は memory に保持され、clipboard export が可能
  requirement_refs:
  - FR-113
relations:
- {type: implements, target: ux-20260713-agent-workspace-viewer}
- {type: implementedBy, target: plan-20260714-agent-workspace-editor}
source_paths:
- src/client/web/src/components/workspace/FileViewer.tsx
- src/client/web/src/lib/workspaceVimKeymap.ts
- src/server/web/workspace.go
- src/server/web/workspace_path.go
summary: agent-workspace-viewer に mutation-capable editor 経路を追加する EARS spec (13 FR
  / 3 NFR / 13 acceptance)。write は viewer+vim keymap 経由のみ、If-Unmodified-Since による
  optimistic lock、tmp+rename atomic、shipped v2 reader 互換の additive audit。
---

> **Phase**: SDD — mutation-capable editor overlay on the viewer (ux追認・spec 新設・plan 新設)。
>
> **Canonical plan**: `/home/dev/.design/agent-workspace-editor/artifacts/plan.json`

## Goal

agent-grid web UI の workspace read-only viewer (spec-20260714-agent-workspace-viewer) に mutation-capable な editor 経路を追加し、operator が vim mutation keys (i/a/o/dd/x/c/y/p/u/Ctrl-R/:w) で実際にファイルを編集・保存できるようにする。sensitive file 区別は read 側同様に持たず、path-traversal defense と workspace-root 境界は継続する。

## UX Contract (inherited, override table applied at plan-level)

上流 `ux-20260713-agent-workspace-viewer` の観察可能な contract を全て継承する (workspace-relative path 表示 / workspace-root boundary / turn aggregation / live-background / git HEAD diff base)。書き込みに関する 3 invariant (exp-read-only / exp-vim-no-mutation / cc-no-write) は本 feature scope でのみ **反転** される — 詳細は `plan.md` の UX Invariant Override table を参照。

## Functional Requirements

### FR-101

While the Workspace Drawer is in editor mode with a text buffer open and focused, the system shall translate vim mutation keystrokes (i/a/o/I/A/O insert-mode entry, dd/x/c/y/p operator+register commands) into concrete in-memory buffer edits and shall not silently discard them as no-ops.

### FR-102

When the operator issues the vim `:w` command from a dirty buffer, the system shall persist the current buffer contents to the corresponding workspace-relative file via the write endpoint and shall report an explicit success or typed failure to the operator, never a silent success or partial write.

### FR-103

The system shall accept every write request only under the same GuardWorkspacePath / workspace-root containment applied to reads, shall apply no sensitive-file filter or masking, and shall serve `.env`, `.git/config`, `*.key`, and any other conventional-sensitive path through the identical write flow as ordinary source files.

### FR-104

While a buffer is open, the system shall scope the vim undo (u) and redo (Ctrl-R) stack strictly to operator-authored edits within that buffer's current lifetime, shall never treat background agent writes as undo/redo steps, and shall clear the stack when the buffer closes or the operator accepts a conflict resolution that replaces its base content.

### FR-105

If the operator holds a dirty buffer for path P and a background agent or a different session writes P before the operator's save completes, then the system shall present a non-blocking conflict banner offering keep-mine / take-theirs / merge and shall never allow either side to silently overwrite the other.

### FR-106

When a save request is processed, the system shall complete it atomically (tmp-file plus os.Rename on the workspace filesystem) so that any subsequent GET for the same path returns either the pre-save bytes or the post-save bytes, never a truncated or partial file.

### FR-107

While a buffer has unsaved changes, the system shall display a dirty indicator on the drawer header/tab within one render frame of the mutating keystroke and shall present an explicit unsaved-changes warning before discarding the buffer on drawer close (Esc, scrim click, or programmatic close).

### FR-108

When an operator save succeeds, the system shall emit a distinct `operator`-kind activity row on the Activity Rail carrying the workspace-relative path and shall never merge it into or misattribute it as an agent tool-call row.

### FR-109

The system shall preserve every read-only vim motion and search binding (j/k/gg/G, /, n, N) documented for the read-only viewer unchanged and shall not regress any counterexample assertion from spec-20260714-agent-workspace-viewer contract-vim-keymap-bindings.

### FR-110

The system shall write back file contents byte-for-byte in the regions the operator did not edit, preserving the original text encoding and line-ending convention (CRLF vs LF, UTF-8 byte sequences including invalid subsequences) that the file carried when the buffer was opened.

### FR-111

If a write request arrives without the same Bearer token authentication that the workspace read handlers require, then the system shall reject it with the identical 401/403 typed response as the read handlers, before touching the workspace filesystem.

### FR-112

While a dirty buffer exists after a WebSocket transport reconnect, the system shall re-fetch the current server-side mtime/ETag for that path and shall raise a conflict banner if it differs from the snapshot taken at buffer open, closing the reconnect drop window rather than permitting the next `:w` to silently overwrite background changes.

### FR-113

If the workspace root directory is deleted or renamed while a dirty buffer is open, then the system shall keep the dirty buffer contents in memory, transition the drawer to a read-only `root_disappeared` state with a typed banner, expose a clipboard-export action for the dirty contents, and shall never silently discard the buffer.

## Non-Functional Requirements

### NFR-101 (performance)

**Criteria**: :w keypress to visible save completion has end-to-end latency p95 ≤ 750 ms and p50 ≤ 500 ms on typical source files (< 100 KiB) served by the local daemon.

**Measurement**: T1 timing test drives 50 saves through the workspace-fs-writer handler and asserts percentiles from ordered latency samples.

### NFR-102 (performance)

**Criteria**: For files at or above the 1 MiB large-file threshold, per-keystroke render latency stays at p95 ≤ 33 ms and p99 ≤ 50 ms during a continuous burst of 200 sequential keystrokes.

**Measurement**: T1 Playwright bench drives a scripted keystroke burst against a 1 MiB fixture and records per-frame paint latency.

### NFR-103 (scalability)

**Criteria**: Write request body is capped at 1 MiB (matching the read-side virtualization threshold); over-cap requests are rejected with typed HTTP 413 before the server buffers the body; under a concurrent load of at most 16 in-flight writes the server memory footprint added by writes remains below 32 MiB.

**Measurement**: T2 Go integration test streams over-cap and under-cap bodies against the handler, asserting 413 and the peak resident-set delta on the write-handler subsystem.

## Acceptance Scenarios

### AC-1001

**Given**: operator が編集可能な text file を drawer で開いている

**When**: operator が `i` を押して文字入力後 `Esc` を押す

**Then**: buffer 内容が入力を反映し、xterm session にキーが漏れない

**Requirement refs**: FR-101, FR-109

### AC-1002

**Given**: dirty buffer を持つ operator

**When**: operator が `:w` を押す

**Then**: workspace 上のファイルが新内容で永続化され、drawer に成功インジケータが出る

**Requirement refs**: FR-102, FR-106

### AC-1003

**Given**: operator が `.env` を dirty で編集中

**When**: operator が `:w` を押す

**Then**: 他 file と同一 flow で保存され、mask/reveal 追加 UI は現れない

**Requirement refs**: FR-103

### AC-1004

**Given**: operator が A→B→C 編集後の buffer

**When**: operator が `u` を 2 回押す

**Then**: buffer が A 後の状態に戻り、agent の背後 write は undo stack に現れない

**Requirement refs**: FR-104

### AC-1005

**Given**: operator が dirty buffer を持ち、背後で agent が同 path を write

**When**: operator が `:w` を押す前に conflict が検知される

**Then**: 非-blocking banner + keep-mine/take-theirs/merge が現れ、どちら側も silent 上書きされない

**Requirement refs**: FR-105

### AC-1006

**Given**: operator の save が rename ステップで失敗

**When**: handler が response を返す

**Then**: response は typed 5xx、disk 上のファイルは pre-save バイト列のまま

**Requirement refs**: FR-106

### AC-1007

**Given**: dirty buffer

**When**: operator が drawer を close しようとする

**Then**: unsaved warning が出て buffer は operator 確認まで破棄されない

**Requirement refs**: FR-107

### AC-1008

**Given**: operator が src/foo.ts を保存

**When**: activity pipeline がイベントを処理

**Then**: ActivityRail に operator kind の distinct row が 1 件現れる (agent row と混同されない)

**Requirement refs**: FR-108

### AC-1009

**Given**: read-only viewer から motion/search テストが緑

**When**: editor 差し替え後にも同一 test を実行

**Then**: 全 motion/search assertion が緑のまま

**Requirement refs**: FR-109

### AC-1010

**Given**: CRLF 終端の Windows ファイル

**When**: operator が 1 行だけ編集して保存

**Then**: 触れていない領域が byte-identical、CRLF が保持される

**Requirement refs**: FR-110

### AC-1011

**Given**: 無認証の curl リクエスト

**When**: PUT /workspace/file を送信

**Then**: handler は 401 を返し fs access を行わない

**Requirement refs**: FR-111

### AC-1012

**Given**: dirty buffer 中に WS が 5 秒ドロップ、gap 中に背後 write

**When**: WS reconnect + operator が `:w`

**Then**: reconnect 直後に conflict-check re-fetch が走り、conflict banner が :w より前に現れる

**Requirement refs**: FR-112

### AC-1013

**Given**: dirty buffer 中に workspace root が削除される

**When**: drawer が状態を更新

**Then**: drawer は root_disappeared に遷移、buffer は memory に保持され、clipboard export が可能

**Requirement refs**: FR-113

## Non-Goals

See plan.md scope_out for the exhaustive list. Highlights:

- WorkspaceTree rename / drag-drop / delete UI (write is exclusively via viewer + vim keymap)
- Multi-operator collaborative editing (1 drawer = 1 operator)
- Offline queued writes (save requires a live connection)
- Sensitive-file masking / reveal step (parity with read side — no filter)
- Cross-workspace or cross-session write
- Crash-across-durability fsync (future ADR)
- Cross-tab or persistent undo stack (viewer-session-local only)

## Open Questions

- fsync による crash-across-durability は future ADR: 現段階では tmp+rename の atomic 置換のみで invariant を閉じる。
- cross-tab / persistent な undo stack は要求が無いため viewer-session-local に閉じる (adr-20260714-editor-undo-scope-viewer-session-local)。
- shipped v2 reader での operator record 誤帰属 (actor field 未対応時に agent と誤分類する UX 劣化) は additive migration の trade-off として accept、rollout 順序 (writer → reader → UI) で影響を最小化する。
