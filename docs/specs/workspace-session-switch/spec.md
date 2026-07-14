---
id: spec-20260714-workspace-session-switch
kind: spec
title: Workspace session switch reinitialization
status: implemented
created: '2026-07-14'
tags:
- workspace
- session-switch
- bugfix
owners: []
functional_requirements:
- id: FR-001
  statement: The system shall display and use Workspace root handle, target, file,
    dirty/conflict/stale state, and tree state only when they belong to the browser-local
    active session.
  priority: must
  rationale: Active session と単一 Workspace state の混在を禁止する invariant。
- id: FR-002
  statement: When a different session is selected while the Workspace is clean, the
    system shall discard the old Workspace state, commit the selected browser-local
    active session, and, if Workspace mode is visible, keep Workspace mode visible
    while the persistent right-side Files tree shows the new session root listing
    with no expanded node and the editor shows its empty state with no open target,
    file, or diff.
  priority: must
- id: FR-003
  statement: When a different session is selected while any Workspace buffer is dirty,
    the system shall keep the active session and Workspace unchanged, record the selected
    session as pending, and show an App-level discard confirmation.
  priority: must
- id: FR-004
  statement: While switch-pending, the system shall replace the pending target and
    dialog label when another valid session is selected and shall treat selecting
    the same target as an idempotent no-op.
  priority: must
- id: FR-005
  statement: When the operator cancels a pending session switch, the system shall
    clear only the pending request and preserve the active session, dirty buffers,
    open file, root handle, and tree state.
  priority: must
- id: FR-006
  statement: When the operator confirms a pending session switch whose target still
    exists, the system shall discard the old Workspace state before committing the
    pending session and shall initialize the visible Workspace to the Tree state defined
    by FR-002.
  priority: must
- id: FR-007
  statement: If the pending target no longer exists when confirmation is attempted,
    then the system shall cancel the pending request, preserve the old active session
    and dirty Workspace, and expose a visible pending_target_disappeared alert.
  priority: must
- id: FR-008
  statement: If the old active session disappears while a dirty switch is pending,
    then the system shall clear active and pending session selection, preserve dirty
    content in the accepted read-only root-disappearance recovery state, and block
    another Workspace from being displayed until discard or clipboard export completes.
  priority: must
- id: FR-009
  statement: If an asynchronous root-handle, file, diff, tree, or reconnect result
    was started for a different Workspace session or epoch, then the system shall
    not commit that result to UI or store state, including loading and error state.
  priority: must
- id: FR-010
  statement: If a workspace request handle names a session different from the URL
    session or a root different from the currently resolved root, then the system
    shall reject it with invalid_handle before filesystem access.
  priority: must
- id: FR-011
  statement: If a workspace request handle generation differs from the current frame
    generation, then the system shall reject it with handle_stale before filesystem
    access.
  priority: must
- id: FR-012
  statement: When only Terminal or Workspace mode visibility changes without active-session
    selection, the system shall preserve terminal scrollback and subscriptions plus
    the Workspace open file, tabs, dirty buffers, root handle, and tree state, shall
    show no discard confirmation, and shall not execute the session-switch lifecycle;
    active-session selection is an explicit context switch that ends the old Workspace
    session.
  priority: must
- id: FR-013
  statement: The system shall route sidebar, palette, post-create selection, and post-termination
    selection through one active-session selection policy without duplicating the
    dirty decision in callers.
  priority: must
- id: FR-014
  statement: When a session switch completes, the system shall preserve rowsBySession
    and lastSequenceBySession activity history for every session.
  priority: must
non_functional_requirements:
- id: NFR-001
  type: reliability
  criteria: Delayed Promise tests shall prove that every old-session root, file, diff,
    tree, reconnect, loading, and error completion leaves the new Workspace unchanged.
  measurement: T1 component tests with controllable WorkspaceApi promises.
- id: NFR-002
  type: security
  criteria: Cross-session and cross-root handle tests shall return a typed 4xx response
    with zero filesystem or git operations under the supplied root.
  measurement: T2 Go handler tests with an instrumented filesystem/git seam or non-existent
    sentinel path.
- id: NFR-003
  type: usability
  criteria: The discard confirmation and recoverable failure alert shall remain visible
    and keyboard-operable even when Workspace mode is hidden.
  measurement: T1 App test plus T3 Playwright smoke using the deterministic fake backend.
- id: NFR-004
  type: maintainability
  criteria: Browser-local activeSessionID writes shall remain owned by one daemon-store
    policy and Workspace state shall not duplicate active-session ownership.
  measurement: Store tests and repository search for direct activeSessionID writes.
- id: NFR-005
  type: compatibility
  criteria: Existing Terminal/Workspace state preservation, drawer-lifetime root pinning,
    root-disappearance recovery, and sessions-only view-update tests shall remain
    green.
  measurement: Existing web unit, Go gateway, and browser smoke suites.
acceptance:
- id: AC-001
  given: Session s1 is active, Workspace mode shows an expanded tree and an open clean
    file, and s2 exists.
  when: The operator selects s2.
  then: s2 becomes active while Workspace mode remains visible; the persistent right-side
    Files tree shows s2 root entries with no expanded node; the editor shows its empty
    state; no open target, file, or diff exists; and no s1 handle, conflict, stale,
    or dirty state remains. Focus is not prescribed.
  requirement_refs:
  - FR-001
  - FR-002
- id: AC-002
  given: Session s1 has a dirty Workspace buffer and s2 exists.
  when: The operator selects s2 and cancels the App-level confirmation.
  then: s1 remains active with identical dirty content, file, root handle, and tree
    state, and no s2 Workspace request is sent.
  requirement_refs:
  - FR-003
  - FR-005
- id: AC-003
  given: Session s1 has a dirty Workspace buffer and s2 exists.
  when: The operator selects s2 and confirms discard.
  then: The s1 Workspace state is discarded before s2 becomes active and the visible
    Workspace matches the persistent Files tree and editor empty state in AC-001.
  requirement_refs:
  - FR-003
  - FR-006
- id: AC-004
  given: A switch from dirty s1 to s2 is pending and s3 exists.
  when: The operator selects s3 and confirms.
  then: The dialog identifies s3, only s3 becomes active, and a repeated selection
    of s3 does not create another pending transition.
  requirement_refs:
  - FR-004
- id: AC-005
  given: A switch from dirty s1 to s2 is pending.
  when: s2 disappears before confirmation.
  then: s1 and its dirty Workspace remain unchanged, pending is cleared, and a visible
    pending_target_disappeared alert is rendered.
  requirement_refs:
  - FR-007
- id: AC-006
  given: A switch from dirty s1 to s2 is pending.
  when: s1 disappears from the session list.
  then: No session is active, the dirty content is available in read-only recovery
    with clipboard export, and s2 Workspace is not displayed.
  requirement_refs:
  - FR-008
- id: AC-007
  given: An s1 Workspace request is unresolved and a clean switch to s2 completes.
  when: The s1 request later resolves or rejects.
  then: No s2 root, file, diff, tree, loading, error, dirty, conflict, or stale state
    changes.
  requirement_refs:
  - FR-009
- id: AC-008
  given: A workspace request URL names s2 while its handle names s1 or a root not
    currently resolved for s2.
  when: The server handles the request.
  then: It returns invalid_handle before any filesystem or git operation.
  requirement_refs:
  - FR-010
- id: AC-009
  given: Terminal has scrollback and subscriptions, and s1 Workspace has an open file,
    tabs, a dirty buffer, root handle, and expanded Files tree.
  when: The operator changes Workspace mode to Terminal and back without selecting
    another session, and afterward explicitly selects s2 and confirms discard.
  then: The mode-only round-trip preserves the terminal and complete s1 Workspace
    state without confirmation; the later active-session context switch ends and discards
    the s1 Workspace session; and activity history for s1 and s2 remains available.
  requirement_refs:
  - FR-012
  - FR-014
- id: AC-010
  given: Session selection can originate from sidebar, palette, post-create, or post-termination
    flows.
  when: Each flow requests a selection while Workspace is dirty.
  then: Each flow reaches the same pending confirmation policy without directly changing
    activeSessionID.
  requirement_refs:
  - FR-013
relations:
- {type: implements, target: ux-20260713-agent-workspace-viewer}
- {type: implementedBy, target: plan-20260714-workspace-session-switch}
- {type: references, target: spec-20260714-web-ui-refresh}
- {type: references, target: ux-20260714-web-ui-refresh}
source_paths: []
methodology: sdd
summary: セッション切替を単一 Workspace state の明示的な終了境界とし、dirty 保護、旧応答隔離、handle binding を保証する。
updated: '2026-07-14'
---

## Overview

ブラウザローカルな active session の選択は明示的な context switch であり、単一の Workspace state を旧 session から切り離して終了し、Workspace mode を維持したまま新 session の persistent Files tree と editor empty state へ再初期化する。dirty buffer がある場合は選択を保留し、App レベルの確認を経る。session ごとの Workspace state 保存・復元は行わない。

本仕様は、accepted な sessions-only view-update、drawer-lifetime WorkspaceRootHandle、root disappearance recovery、および web-ui-refresh FR-031/032 と UAC-016 の Terminal/Workspace 両 mode layer 状態保持を維持する。FR-031 の「explicit close だけが Workspace session を終了する」は mode visibility の文脈に限定し、active-session selection を別の明示的終了境界として refinement する。

## Requirements

{% req id="FR-001" %}Workspace 表示・要求に使う全状態は browser-local active session と同一 session に属する。{% /req %}
{% req id="FR-002" %}clean 切替は Workspace mode を維持し、persistent right-side Files tree を未展開の新 root 一覧に、editor を open target/file/diff のない empty state に初期化する。focus は規定しない。{% /req %}
{% req id="FR-003" %}dirty 切替は active session を変えず pending confirmation に入る。{% /req %}
{% req id="FR-004" %}pending 中の再選択は最新の valid target に置換し、同一 target は冪等である。{% /req %}
{% req id="FR-005" %}Cancel は pending だけを消去する。{% /req %}
{% req id="FR-006" %}Confirm は旧 Workspace を破棄してから target を active にする。{% /req %}
{% req id="FR-007" %}pending target 消失は旧状態を保って visible alert に回復する。{% /req %}
{% req id="FR-008" %}旧 active session 消失時は dirty content を既存 read-only recovery に保つ。{% /req %}
{% req id="FR-009" %}異なる session/epoch の非同期結果は全 commit を拒否する。{% /req %}
{% req id="FR-010" %}session/root mismatch は invalid_handle として filesystem access 前に拒否する。{% /req %}
{% req id="FR-011" %}generation drift は handle_stale として filesystem access 前に拒否する。{% /req %}
{% req id="FR-012" %}mode visibility 変更だけなら terminal と Workspace の全 state を保持し、active-session selection のみを旧 Workspace session の明示的終了境界とする。{% /req %}
{% req id="FR-013" %}全 selection caller は単一 policy を通る。{% /req %}
{% req id="FR-014" %}session switch は activity history を保持する。{% /req %}

{% req id="NFR-001" %}旧 Promise の全 completion partition を deterministic に検証する。{% /req %}
{% req id="NFR-002" %}handle 混用時の filesystem/git access は 0 件である。{% /req %}
{% req id="NFR-003" %}確認と回復 alert は Workspace 非表示でも操作できる。{% /req %}
{% req id="NFR-004" %}active session の決定権は daemon selection policy に一本化する。{% /req %}
{% req id="NFR-005" %}既存 accepted contracts の回帰テストを維持する。{% /req %}

### State Machine

状態の正本は `stable`、`switch-pending`、`orphaned-recovery` とする。`stable` から clean 選択は旧 Workspace reset と active commit を一つの transaction として `stable` に遷移する。dirty 選択は `switch-pending` へ遷移する。pending 中の別 target は pending を置換し、Cancel・valid Confirm・target disappearance は `stable` へ戻る。old active disappearance は `orphaned-recovery` へ遷移し、discard または clipboard export 完了まで新 Workspace 表示を禁止する。

### Failure Modes

- `pending_target_disappeared`: session list で検出し、pending を解除して旧 Workspace を保持し、visible alert へ degrade する（外部由来回復）。
- `active_session_disappeared_dirty`: session list で検出し、accepted root-disappearance recovery へ degrade する（外部由来回復）。
- `workspace_identity_mismatch`: response commit 前の session/epoch 比較で検出し、結果を no-op と定義してエラー条件を消す（意味論再定義）。
- `invalid_handle`: server 境界で URL session/current root と照合し、typed 4xx で拒否する（外部入力の防御的回復）。
- `handle_stale`: server 境界で generation drift を検出し、既存 typed 409 degradation を保つ（外部由来回復）。

### Non-Goals

- MUST NOT: session ごとの file、dirty buffer、tree expansion、root handle を保存・復元する state map を導入しない。
- MUST NOT: session switch で rowsBySession または lastSequenceBySession を破棄しない。
- SHOULD NOT: Terminal/Workspace mode visibility の既存 state-preservation semantics を変更しない。
- SHOULD NOT: opaque token、server-side handle registry、新しい runtime dependency、multi-root 対応を導入しない。

## Acceptance Criteria

{% acceptance id="AC-001" %}clean switch は Workspace mode を維持し、persistent Files tree の新 root 一覧と editor empty state を表示する。{% /acceptance %}
{% acceptance id="AC-002" %}dirty Cancel は selection と editor 内容を完全に保つ。{% /acceptance %}
{% acceptance id="AC-003" %}dirty Confirm は discard-before-commit 順序を守る。{% /acceptance %}
{% acceptance id="AC-004" %}pending 再選択は dialog と commit target を一致させる。{% /acceptance %}
{% acceptance id="AC-005" %}pending target 消失は visible recovery になる。{% /acceptance %}
{% acceptance id="AC-006" %}old active 消失は dirty content を recovery UI に保持する。{% /acceptance %}
{% acceptance id="AC-007" %}旧非同期結果は新 state のどの partition も変更しない。{% /acceptance %}
{% acceptance id="AC-008" %}handle 混用は I/O 前に拒否される。{% /acceptance %}
{% acceptance id="AC-009" %}UAC-016 の counterexample である mode switch reset を禁止し、active-session context switch だけが旧 Workspace session を終了する。{% /acceptance %}
{% acceptance id="AC-010" %}全 selection flow が同一 guard を通る。{% /acceptance %}


{% transition from="draft" to="approved" date="2026-07-14" %}
実装開始の承認をユーザーから得たため
{% /transition %}


{% transition from="approved" to="implemented" date="2026-07-14" %}
Accepted design implemented and verified
{% /transition %}
