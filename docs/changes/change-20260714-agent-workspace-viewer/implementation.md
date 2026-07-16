---
change: change-20260714-agent-workspace-viewer
role: implementation
---

# Implementation

## Legacy Source (verbatim)

````markdown
---
id: plan-20260714-agent-workspace-viewer
kind: plan
title: Agent workspace read-only viewer
status: active
created: '2026-07-14'
updated: '2026-07-14'
goal: agent-grid web UI 上で、operator がエージェントセッションの workspace を read-only で追跡・確認できる
  viewer (turn-aggregated activity rail + live-background modal drawer + structured
  content rendering + git HEAD diff + read-only vim motion + workspace-root-bounded
  tree) を追加する。
scope_in:
- 既存 App.tsx 構成に turn-aggregated Activity Rail を追加 (AppShell main grid area 内 wrapping
  container)
- Modal Workspace Drawer (Viewer / Diff / Tree タブ) を新設し、live-background で表示 (背後 terminal/rail
  は inert 化されず視覚更新継続)
- Markdown 整形表示 (+ rehype-sanitize XSS defense + parse error 時 raw fallback)、Mermaid
  SVG 描画 (+ 300ms parse timeout + raw fallback)、JSON 折りたたみツリー (+ invalid JSON raw
  fallback)、source viewer 上の read-only vim motion (j/k/gg/G + /,n,N)
- edit event の diff を git HEAD 基準で表示; not_a_repo / git_metadata_corrupted / git_binary_missing
  の 3 typed 劣化表示
- activity row を経由しない Workspace secondary-tree affordance (row 0 件でも always visible)
  から Tree タブを開ける
- 1 MiB 超テキストの virtualized scroll (truncate なし) + binary metadata-only placeholder
- workspace root 境界内に限定した read-only backend endpoint (tree / file / diff / root-handle);
  per-segment EvalSymlinks による path traversal 構造禁止; depguard rule で os.WriteFile 系の
  import を静的禁止
- 既存 write-only tool-call log を schema_version=2 に upgrade (turn_id + normalized kind);
  system 初の reader を新規追加
- row / drawer header / tree いずれの path 表示も workspace-relative に統一
scope_out:
- 書き込み・保存 API の追加 (cc-no-write; contract-no-write-boundary で depguard 強制)
- 複数 workspace root の横断閲覧 (DP-TREE-ROOT-BOUNDARY=OPT-ROOT-WORKSPACE-ONLY)
- filesystem watch を event source とすること (DP-EVENT-SOURCE=OPT-TOOL-CALL-LOG)
- 現在セッション以外の履歴再生 (DP-HISTORY-MODE=OPT-LIVE-ONLY)
- sensitive file のフィルタ/マスキング (DP-SENSITIVE-FILE-EXPOSURE=OPT-NO-FILTER)
- vim insert mode / :w 系の実編集コマンド (exp-vim-no-mutation)
- AppShell の named-grid-area 契約や MainTabs の exclusive-tab パラダイムの変更 (cc-appshell-preserve;
  contract-appshell-preserve で bounding-rect regression 強制)
- Claude driver における Bash-based delete の row 表示 (v1 では structured signal のみで row を出す;
  unclassified diagnostic counter で観測)
milestones:
- id: m1
  title: component:component-workspace-fs-api
  status: done
- id: m2
  title: component:component-tool-log-writer
  status: done
- id: m3
  title: component:component-workspace-activity-store
  status: done
- id: m4
  title: component:component-activity-rail
  status: done
- id: m5
  title: component:component-file-viewer
  status: done
contracts:
- contract-appshell-preserve
- contract-workspace-root-resolution
- contract-workspace-path-traversal-defense
- contract-no-write-boundary
- contract-markdown-sanitization
- contract-large-file-threshold
- contract-structured-render-fallback
- contract-vim-keymap-bindings
- contract-secondary-tree-entry
- contract-tree-refresh
- contract-diff-view-layout
- contract-diff-base-non-git-fallback
- contract-activity-event-source
- contract-turn-aggregation
- contract-live-background-transport
- contract-stale-banner-presentation
contract_projections:
- id: contract-appshell-preserve
  decision_rules:
  - decision-appshell-composition
  observable_effects:
  - observable-appshell-terminal-slot-rect
  operational_inputs: []
  semantic_profiles: []
  verifications:
  - verify-appshell-terminal-rect-nonregression
  witnesses:
  - witness-appshell-normal
  - witness-appshell-breakpoint-boundary
- id: contract-workspace-root-resolution
  decision_rules:
  - decision-workspace-root-handle-snapshot
  observable_effects:
  - observable-workspace-root-handle-consistency
  operational_inputs:
  - input-session-workspace-root-ssot
  semantic_profiles: []
  verifications:
  - verify-workspace-root-handle-pinning
  witnesses:
  - witness-workspace-root-normal
  - witness-workspace-root-frame-push-adversarial
- id: contract-workspace-path-traversal-defense
  decision_rules:
  - decision-path-guard-per-segment
  observable_effects:
  - observable-path-guard-containment
  operational_inputs:
  - input-workspace-root-handle-for-guard
  semantic_profiles: []
  verifications:
  - verify-path-guard-symlink-escape
  witnesses:
  - witness-path-guard-normal
  - witness-path-guard-intermediate-symlink-adversarial
- id: contract-no-write-boundary
  decision_rules:
  - decision-no-write-depguard
  observable_effects:
  - observable-no-write-structural
  operational_inputs: []
  semantic_profiles: []
  verifications:
  - verify-no-write-lint
  - verify-no-write-verb-scan
  witnesses:
  - witness-no-write-normal
  - witness-no-write-mutation-adversarial
- id: contract-markdown-sanitization
  decision_rules:
  - decision-markdown-sanitize-schema
  observable_effects:
  - observable-markdown-no-forbidden-tokens
  operational_inputs:
  - input-markdown-source-bytes
  semantic_profiles: []
  verifications:
  - verify-markdown-sanitization-fixture
  witnesses:
  - witness-markdown-clean-normal
  - witness-markdown-malicious-adversarial
- id: contract-large-file-threshold
  decision_rules:
  - decision-large-file-virtualize
  observable_effects:
  - observable-large-file-scrolls-to-eof
  operational_inputs:
  - input-file-bytes-for-viewer
  semantic_profiles:
  - profile-large-file-cost
  verifications:
  - verify-large-file-scroll-bench
  witnesses:
  - witness-large-file-normal
  - witness-large-file-scale-adversarial
- id: contract-structured-render-fallback
  decision_rules:
  - decision-structured-parse-success
  - decision-structured-parse-error
  - decision-structured-timeout
  observable_effects:
  - observable-structured-render-or-fallback
  operational_inputs:
  - input-structured-source-bytes
  semantic_profiles:
  - profile-structured-outcome-partition
  verifications:
  - verify-structured-fallback-bound
  witnesses:
  - witness-structured-clean-normal
  - witness-structured-timeout-adversarial
  - witness-structured-parse-error-adversarial
- id: contract-vim-keymap-bindings
  decision_rules:
  - decision-vim-keymap-dispatch
  observable_effects:
  - observable-vim-keymap-motion-only
  operational_inputs:
  - input-viewer-focus-and-key
  semantic_profiles: []
  verifications:
  - verify-vim-motion-unit
  - verify-vim-mutation-integration
  witnesses:
  - witness-vim-motion-normal
  - witness-vim-mutation-adversarial
- id: contract-secondary-tree-entry
  decision_rules:
  - decision-workspace-affordance-visibility
  observable_effects:
  - observable-workspace-affordance-reachable
  operational_inputs: []
  semantic_profiles: []
  verifications:
  - verify-workspace-affordance-a11y
  witnesses:
  - witness-workspace-affordance-normal
  - witness-workspace-affordance-a11y-adversarial
- id: contract-tree-refresh
  decision_rules:
  - decision-tree-refresh-ok
  - decision-tree-refresh-root-unreachable
  - decision-tree-refresh-transport-failure
  observable_effects:
  - observable-tree-refresh-or-degraded
  operational_inputs:
  - input-tree-listing
  semantic_profiles:
  - profile-tree-refresh-outcome-partition
  verifications:
  - verify-tree-refresh-outcomes
  witnesses:
  - witness-tree-refresh-normal
  - witness-tree-torn-down-adversarial
  - witness-tree-refresh-transport-adversarial
- id: contract-diff-view-layout
  decision_rules:
  - decision-diff-layout-unified
  observable_effects:
  - observable-diff-lines-distinguishable
  operational_inputs:
  - input-diff-payload
  semantic_profiles: []
  verifications:
  - verify-diff-layout-a11y
  witnesses:
  - witness-diff-modest-normal
  - witness-diff-large-adversarial
- id: contract-diff-base-non-git-fallback
  decision_rules:
  - decision-diff-base-ok
  - decision-diff-base-not-a-repo
  - decision-diff-base-metadata-corrupted
  - decision-diff-base-binary-missing
  observable_effects:
  - observable-diff-base-typed-outcome
  operational_inputs:
  - input-git-repo-detection
  semantic_profiles:
  - profile-diff-base-outcome-partition
  verifications:
  - verify-diff-base-outcomes
  witnesses:
  - witness-diff-base-ok-normal
  - witness-diff-base-not-a-repo-adversarial
  - witness-diff-base-corrupted-adversarial
  - witness-diff-base-binary-missing-adversarial
- id: contract-activity-event-source
  decision_rules:
  - decision-tool-log-classify-structured
  - decision-tool-log-classify-unstructured
  - decision-tool-log-classify-legacy
  observable_effects:
  - observable-activity-row-defensible
  operational_inputs:
  - input-tool-log-jsonl-tail
  semantic_profiles:
  - profile-activity-classification-partition
  verifications:
  - verify-activity-classification
  witnesses:
  - witness-activity-classified-normal
  - witness-activity-bash-rm-adversarial
  - witness-activity-legacy-adversarial
- id: contract-turn-aggregation
  decision_rules:
  - decision-turn-boundary-codex
  - decision-turn-boundary-claude-stop
  - decision-turn-boundary-claude-subagent
  - decision-turn-boundary-claude-overlap
  observable_effects:
  - observable-turn-aggregation-correct
  operational_inputs:
  - input-turn-boundary-signals
  semantic_profiles:
  - profile-turn-aggregation-partition
  verifications:
  - verify-turn-aggregation
  witnesses:
  - witness-turn-codex-normal
  - witness-turn-claude-stop-failure-adversarial
  - witness-turn-claude-subagent-adversarial
  - witness-turn-cross-boundary-adversarial
- id: contract-live-background-transport
  decision_rules:
  - decision-transport-viewupdate-extension
  - decision-transport-reconnect
  - decision-transport-cross-session-guard
  - decision-transport-out-of-order
  observable_effects:
  - observable-transport-latency-and-scoping
  operational_inputs:
  - input-activity-events-stream
  - input-session-scoping
  semantic_profiles:
  - profile-transport-cost
  - profile-transport-outcome-partition
  - profile-transport-scope-consistency
  verifications:
  - verify-transport-latency-bound
  witnesses:
  - witness-transport-normal
  - witness-transport-latency-adversarial
  - witness-transport-reconnect-adversarial
  - witness-transport-cross-session-adversarial
  - witness-transport-out-of-order-adversarial
- id: contract-stale-banner-presentation
  decision_rules:
  - decision-stale-render-and-announce
  - decision-stale-rapid-repeats
  - decision-stale-signal-lost
  observable_effects:
  - observable-stale-banner-and-announce
  operational_inputs:
  - input-drawer-target-and-touch-signal
  semantic_profiles:
  - profile-stale-outcome-partition
  verifications:
  - verify-stale-render-latency
  witnesses:
  - witness-stale-mid-turn-normal
  - witness-stale-rapid-repeat-adversarial
  - witness-stale-ws-drop-adversarial
adrs:
- adr-20260714-wsviewer-appshell-composition
- adr-20260714-wsviewer-workspace-root-handle
- adr-20260714-wsviewer-path-guard-symlink
- adr-20260714-wsviewer-tool-log-schema-and-turn-boundary
- adr-20260714-wsviewer-live-transport-and-mid-turn-stale
- adr-20260714-wsviewer-markdown-xss-boundary
- adr-20260714-wsviewer-no-write-depguard
- adr-20260714-wsviewer-fallback-observability-bounds
reference_algorithms: []
tags:
- workspace-viewer
- read-only
- sdd
owners: []
relations:
- {type: implements, target: spec-20260714-agent-workspace-viewer}
- {type: hasPart, target: adr-20260714-wsviewer-appshell-composition}
- {type: hasPart, target: adr-20260714-wsviewer-workspace-root-handle}
- {type: hasPart, target: adr-20260714-wsviewer-path-guard-symlink}
- {type: hasPart, target: adr-20260714-wsviewer-tool-log-schema-and-turn-boundary}
- {type: hasPart, target: adr-20260714-wsviewer-live-transport-and-mid-turn-stale}
- {type: hasPart, target: adr-20260714-wsviewer-markdown-xss-boundary}
- {type: hasPart, target: adr-20260714-wsviewer-no-write-depguard}
- {type: hasPart, target: adr-20260714-wsviewer-fallback-observability-bounds}
source_paths:
- src/client/web/src/components/AppShell.tsx
- src/server/web/transcript.go
- src/client/runtime/tool_log.go
methodology: sdd
summary: 'workspace read-only viewer 実装計画: server 側 read-only fs endpoint + depguard、tool-log
  reader 新規追加 (schema_version=2)、client 側 activity rail + drawer + Tree + viewer/diff、react-markdown
  + rehype-sanitize、CodeMirror-style vim keymap の integration。'
---

# Agent workspace read-only viewer — implementation plan

## Goal

agent-grid web UI 上で、operator がエージェントセッションの workspace を read-only で追跡・確認できる viewer (turn-aggregated activity rail + live-background modal drawer + structured content rendering + git HEAD diff + read-only vim motion + workspace-root-bounded tree) を追加する。

## Approach

既存の 3 つの seam を拡張し、平行構造の新設を避ける: (1) 既存 write-only tool-call log (runtime/tool_log.go + driver/*_tool_log.go) を schema_version=2 に upgrade して turn_id + normalized kind を追加し、system 初の reader (component-tool-log-reader) を新規追加して turn_row + mid_turn_touch イベントを emit する。(2) server/web に workspace.go を新設し、transcript.go と同じ shape (ID allowlist → daemon RPC → resolved path → stream) を踏襲し、depguard rule で cc-no-write を構造的に enforce する。(3) platform/lib/git に HEAD-diff helper を追加し、not_a_repo / metadata_corrupted / binary_missing の typed outcome partition で非git状態を明示する。frontend は AppShell 内 main grid area に wrapping container を挟んで ActivityRail + MainTabs を共存させ、ADR-0065 terminal-slot rect を保持する。WorkspaceDrawer は drawer 開時に WorkspaceRootHandle を snapshot し、以降 tree/file/diff request 全てを同じ root で解決する。Markdown は rehype-sanitize で XSS を fail-closed で防ぐ。

## Implementation Sequence

### Milestone m1

- **Depends on**: nothing
- **Members**: component:component-workspace-fs-api, component:component-workspace-path-guard, component:component-session-workspace-root-field, component:component-git-diff-head, req:FR-013, req:FR-014, req:FR-018, req:FR-026, req:FR-028, req:FR-029, adr:adr-20260714-wsviewer-workspace-root-handle, adr:adr-20260714-wsviewer-path-guard-symlink, adr:adr-20260714-wsviewer-no-write-depguard, adr:adr-20260714-wsviewer-fallback-observability-bounds

{% milestone id="m1" %}
- component:component-workspace-fs-api
- component:component-workspace-path-guard
- component:component-session-workspace-root-field
- component:component-git-diff-head
- req:FR-013
- req:FR-014
- req:FR-018
- req:FR-026
- req:FR-028
- req:FR-029
- adr:adr-20260714-wsviewer-workspace-root-handle
- adr:adr-20260714-wsviewer-path-guard-symlink
- adr:adr-20260714-wsviewer-no-write-depguard
- adr:adr-20260714-wsviewer-fallback-observability-bounds
{% /milestone %}

#### Units

##### session-workspace-root-exposure

- **Objective**: Expose the resolved workspace root through the daemon IPC + proto.SessionInfo.WorkspaceRoot so the browser can obtain a WorkspaceRootHandle on drawer open (adr-20260714-wsviewer-workspace-root-handle).
- **Output format**: Go source + tests: extended proto.SessionInfo, resolver reading the SSOT triple (LaunchPlan.StartDir / DEvWorktreeResolved.WorktreeStartDir / Session.Project), wire-fixture pipeline updated.
- **Tool guidance**: Read src/client/state/state.go, state/effect.go, state/event.go, state/reduce_helpers.go. Extend proto/*.go. Follow adr-20260705-wire-fixtures-pipeline for the new field.
- **Task boundaries**: Does not add the HTTP handler (workspace-fs-api-handlers) or the path guard; only the daemon-side resolution and IPC surface.
- **Files touched**: src/client/state/state.go, src/client/state/effect.go, src/client/state/reduce_helpers.go, src/client/proto, src/client/state/testdata
- **Acceptance**:
  - New unit test asserts resolution priority DEvWorktreeResolved.WorktreeStartDir > LaunchPlan.StartDir > Session.Project.
  - Existing drivertest.Conformance / MetadataSourcePriority stay green.
  - Wire-fixture pipeline emits the new WorkspaceRoot field.
- **Contract refs**: contract-workspace-root-resolution
- **max_diff_loc**: 300

##### workspace-path-guard

- **Objective**: Implement the per-segment EvalSymlinks-based containment check for all workspace path parameters (adr-20260714-wsviewer-path-guard-symlink).
- **Output format**: New src/server/web/workspace_path.go + package-local table-driven test + fuzz test.
- **Tool guidance**: Model on adr-20260624-0026 concept but reject '..'/absolute up front, join with root, filepath.EvalSymlinks on full path, verify descendant of EvalSymlinks-normalized root. Uniform 404 on rejection.
- **Task boundaries**: Does not register HTTP routes; only exposes the guard function.
- **Files touched**: src/server/web/workspace_path.go, src/server/web/workspace_path_test.go
- **Acceptance**:
  - Table-driven test covers: relative in-root, absolute, .., encoded .., intermediate-symlink-outside-root, final-symlink-outside-root.
  - Fuzz test runs without panic; all rejections return the uniform 404-shaped error type.
  - No bytes from outside root are returned in any test case.
- **Contract refs**: contract-workspace-path-traversal-defense
- **Depends on**: session-workspace-root-exposure
- **max_diff_loc**: 250

##### git-diff-head-helper

- **Objective**: Add HEAD-vs-worktree diff helper in platform/lib/git with typed outcome partition (ok/not_a_repo/git_metadata_corrupted/git_binary_missing) per adr-20260714-wsviewer-fallback-observability-bounds.
- **Output format**: Go source in src/platform/lib/git/git.go (new function) + table-driven test.
- **Tool guidance**: Reuse IsRepo/RepoRoot/findGitRoot; exec.LookPath('git') for binary detection; distinguish stderr-based corrupted case from clean not-a-repo.
- **Task boundaries**: Does not add HTTP surface; only the helper + tests.
- **Files touched**: src/platform/lib/git/git.go, src/platform/lib/git/git_test.go
- **Acceptance**:
  - Test covers all 4 outcome classes with realistic fixtures.
  - Typed enum/response is exported for use by workspace-fs-api-handlers.
  - Existing platform/lib/git tests remain green.
- **Contract refs**: contract-diff-base-non-git-fallback
- **max_diff_loc**: 300

##### workspace-fs-api-handlers

- **Objective**: Register read-only workspace endpoints (GET tree, GET file, GET diff, GET root-handle) on server/web mux; call path guard on every request; wire diff handler to git-diff-head; enforce no-write structurally via depguard rule.
- **Output format**: src/server/web/workspace.go + workspace_test.go + mux.go registration + src/.golangci.yml depguard rule.
- **Tool guidance**: Model on transcript.go (serveSessionFile / resolveSessionFilePath / sessionIDPattern). Depguard rule targets src/server/web/workspace*.go.
- **Task boundaries**: Does not implement client-side rendering or store; only the server HTTP surface + lint rule.
- **Files touched**: src/server/web/workspace.go, src/server/web/workspace_test.go, src/server/web/mux.go, src/server/web/mux_scenario_test.go, src/.golangci.yml
- **Acceptance**:
  - GET tree/file/diff/root-handle return correct responses under the containment guard.
  - TestWorkspaceReadOnlyVerbs asserts PUT/POST/PATCH/DELETE return 405 on every route.
  - TestWorkspaceRootHandlePinning asserts handle-stale behavior on frame_generation mismatch.
  - Synthetic-mutation regression: injecting os.WriteFile into workspace.go causes `make lint` to fail; removing it makes lint pass.
- **Contract refs**: contract-no-write-boundary, contract-workspace-root-resolution, contract-diff-base-non-git-fallback
- **Depends on**: workspace-path-guard, git-diff-head-helper, session-workspace-root-exposure
- **max_diff_loc**: 400

### Milestone m2

- **Depends on**: m1
- **Members**: component:component-tool-log-writer, component:component-tool-log-reader, req:FR-002, req:FR-003, req:FR-015, adr:adr-20260714-wsviewer-tool-log-schema-and-turn-boundary

{% milestone id="m2" %}
- component:component-tool-log-writer
- component:component-tool-log-reader
- req:FR-002
- req:FR-003
- req:FR-015
- adr:adr-20260714-wsviewer-tool-log-schema-and-turn-boundary
{% /milestone %}

#### Units

##### tool-log-schema-extension

- **Objective**: Bump tool-log JSONL schema_version to 2; add turn_id and normalized file-event kind fields; keep the writer append-only and schema_version-stamped.
- **Output format**: Extend src/client/runtime/tool_log.go + driver/*_tool_log.go with the new fields and versioning; existing test files updated.
- **Tool guidance**: Reference runtime/tool_log.go current schema; ensure legacy lines remain readable byte-for-byte.
- **Task boundaries**: Does not implement the reader; only the writer-side schema evolution.
- **Files touched**: src/client/runtime/tool_log.go, src/client/runtime/tool_log_test.go, src/client/driver/claude_tool_log.go, src/client/driver/claude_tool_log_test.go, src/client/driver/codex_tool_log.go, src/client/driver/codex_tool_log_test.go
- **Acceptance**:
  - All v2 emissions carry schema_version=2, turn_id, and normalized kind.
  - drivertest.Conformance / MetadataSourcePriority stay green.
  - Pre-upgrade fixture lines round-trip without modification.
- **Contract refs**: contract-activity-event-source
- **max_diff_loc**: 300

##### driver-tool-log-updates-claude

- **Objective**: Compute turn_id and normalized kind for Claude driver: Stop / StopFailure / SubagentStart / SubagentStop hooks per adr-20260714-wsviewer-tool-log-schema-and-turn-boundary.
- **Output format**: Update src/client/driver/claude_tool_log.go + claude_event.go correlation with tests.
- **Tool guidance**: Enumerate Stop family in claude_event.go; synthesize monotonic turn counter; nested sub-turn id = parent-turn.sub-N.
- **Task boundaries**: Does not touch Codex driver; does not implement the reader.
- **Files touched**: src/client/driver/claude_tool_log.go, src/client/driver/claude_event.go, src/client/driver/claude_tool_log_test.go, src/client/driver/claude_event_test.go
- **Acceptance**:
  - Unit tests cover Stop, StopFailure, SubagentStart+SubagentStop, and overlapping PreToolUse cases.
  - Bash-based deletion (no structured file_path) produces the 'unclassified' shape (no fabricated path).
- **Contract refs**: contract-turn-aggregation, contract-activity-event-source
- **Depends on**: tool-log-schema-extension
- **max_diff_loc**: 300

##### driver-tool-log-updates-codex

- **Objective**: Compute turn_id from SubsystemTurnStarted/Completed for Codex driver; classify apply_patch and read/edit tool shapes into normalized kind.
- **Output format**: Update src/client/driver/codex_tool_log.go with tests.
- **Tool guidance**: Codex has native turn semantics; kind classification is direct.
- **Task boundaries**: Does not touch Claude driver.
- **Files touched**: src/client/driver/codex_tool_log.go, src/client/driver/codex_tool_log_test.go
- **Acceptance**:
  - Unit tests cover apply_patch add/edit/delete and read tool shapes.
  - turn_id changes on SubsystemTurnStarted; stays constant across intra-turn calls.
- **Contract refs**: contract-activity-event-source
- **Depends on**: tool-log-schema-extension
- **max_diff_loc**: 250

##### tool-log-reader

- **Objective**: New tailer/classifier in src/client/runtime/tool_log_reader.go; emits {turn_row, mid_turn_touch} events; skips legacy lines with counter; suppresses unclassified with counter.
- **Output format**: New Go source + tests + fake tailer used by activity-store store tests.
- **Tool guidance**: Persist per-(namespace, project) read offset; expose fake via runtime test helper.
- **Task boundaries**: Does not touch server broadcast wiring; that is view-update-activity-extension.
- **Files touched**: src/client/runtime/tool_log_reader.go, src/client/runtime/tool_log_reader_test.go
- **Acceptance**:
  - TestToolLogReaderClassification asserts row emission for structured entries only.
  - TestTurnAggregation asserts per-driver turn grouping including nested sub-agent.
  - Legacy-line skip counter increments; no fabricated rows.
  - Reader resumes tailing from persisted offset after simulated restart.
- **Contract refs**: contract-activity-event-source, contract-turn-aggregation
- **Depends on**: tool-log-schema-extension, driver-tool-log-updates-claude, driver-tool-log-updates-codex
- **max_diff_loc**: 400

### Milestone m3

- **Depends on**: m2
- **Members**: component:component-workspace-activity-store, req:FR-005, req:FR-006, req:FR-007, req:FR-008, adr:adr-20260714-wsviewer-live-transport-and-mid-turn-stale

{% milestone id="m3" %}
- component:component-workspace-activity-store
- req:FR-005
- req:FR-006
- req:FR-007
- req:FR-008
- adr:adr-20260714-wsviewer-live-transport-and-mid-turn-stale
{% /milestone %}

#### Units

##### view-update-activity-extension

- **Objective**: Extend server/web ViewUpdate broadcast (adr-20260624-0023) with activity_events payload carrying turn_row + mid_turn_touch events; preserve session-scoping (adr-20260705); expose sequence numbers for gap detection.
- **Output format**: Go source changes in src/server/web/viewupdate*.go + tests.
- **Tool guidance**: Reuse the existing broadcaster fan-out; augment the payload; do NOT introduce a new WS channel.
- **Task boundaries**: Does not implement the browser store; only the server-side payload.
- **Files touched**: src/server/web/viewupdate.go, src/server/web/viewupdate_test.go, src/server/web/mux.go
- **Acceptance**:
  - TestViewUpdateActivityLatency asserts append-to-broadcast delay bounds.
  - Cross-session leak test asserts no A payload reaches B subscribers.
  - Monotonic sequence numbers exposed.
- **Contract refs**: contract-live-background-transport
- **max_diff_loc**: 300

##### mid-turn-stale-signal

- **Objective**: Extend tool-log reader to emit mid_turn_touch synchronously with PostToolUse for any tool call that touches a file (does not wait for turn completion) so the drawer can detect mid-turn stale within 500 ms.
- **Output format**: Additive change in src/client/runtime/tool_log_reader.go + tests demonstrating the mid-turn emission path.
- **Tool guidance**: This is the integrator's mid-turn stale routing decision: extend the existing reader (per user guidance) rather than creating a new component.
- **Task boundaries**: Does not change the store or the drawer; only the reader-side emission.
- **Files touched**: src/client/runtime/tool_log_reader.go, src/client/runtime/tool_log_reader_test.go
- **Acceptance**:
  - mid_turn_touch event is emitted for every PostToolUse whose classified path is non-empty.
  - The event carries (session_id, path, tool_call_id, sequence) so the store can coalesce rapid repeats.
  - Existing turn-completion emission remains unchanged.
- **Contract refs**: contract-live-background-transport, contract-stale-banner-presentation
- **Depends on**: tool-log-reader, view-update-activity-extension
- **max_diff_loc**: 200

##### workspace-activity-store

- **Objective**: New Zustand store src/client/web/src/store/workspaceActivity.ts consuming activity_events; enforces cross-session guard, coalesces rapid touches, exposes stale-state selector keyed to the drawer's open target and WorkspaceRootHandle.
- **Output format**: TypeScript source + __tests__/ over synthetic transport events.
- **Tool guidance**: Model on daemon.ts + transcripts.ts precedents.
- **Task boundaries**: Does not implement UI components; only store.
- **Files touched**: src/client/web/src/store/workspaceActivity.ts, src/client/web/src/store/__tests__/workspaceActivity.test.ts
- **Acceptance**:
  - Unit tests cover: turn_row apply, mid_turn_touch apply, rapid-repeat coalesce, cross-session discard, out-of-order discard + resync, reconnect + backfill.
  - Store exposes selectors used by ActivityRail and WorkspaceDrawer.
- **Contract refs**: contract-live-background-transport, contract-stale-banner-presentation, contract-workspace-root-resolution
- **Depends on**: view-update-activity-extension
- **max_diff_loc**: 350

### Milestone m4

- **Depends on**: m3
- **Members**: component:component-activity-rail, component:component-workspace-drawer, req:FR-001, req:FR-004, req:FR-016, req:FR-017, req:FR-027, adr:adr-20260714-wsviewer-appshell-composition

{% milestone id="m4" %}
- component:component-activity-rail
- component:component-workspace-drawer
- req:FR-001
- req:FR-004
- req:FR-016
- req:FR-017
- req:FR-027
- adr:adr-20260714-wsviewer-appshell-composition
{% /milestone %}

#### Units

##### activity-rail-shell-composition

- **Objective**: Introduce wrapping container inside AppShell main grid area hosting ActivityRail + MainTabs; render turn-aggregated rows and the always-visible Workspace secondary-tree affordance.
- **Output format**: New src/client/web/src/components/workspace/ActivityRail.tsx + App.tsx composition change + tests.
- **Tool guidance**: Reuse CommandSearchTrigger button precedent for the Workspace affordance; verify ADR-0065 terminal-slot rect is unchanged via a bounding-rect regression test.
- **Task boundaries**: Does not implement the drawer; only rail + affordance + composition.
- **Files touched**: src/client/web/src/App.tsx, src/client/web/src/components/workspace/ActivityRail.tsx, src/client/web/src/components/workspace/ActivityRail.test.tsx
- **Acceptance**:
  - verify-appshell-terminal-rect-nonregression passes (bounding rect unchanged with/without ActivityRail).
  - verify-workspace-affordance-a11y passes (row-0 and row-N configurations).
  - Rail renders turn-aggregated rows from workspaceActivityStore.
- **Contract refs**: contract-appshell-preserve, contract-secondary-tree-entry
- **max_diff_loc**: 400

##### workspace-drawer

- **Objective**: Implement WorkspaceDrawer (modal aria-modal drawer) hosting Viewer/Diff/Tree tabs; obtain WorkspaceRootHandle at open; render stale banner + reload affordance; coalesce aria-live announcements through the palette single-slot channel.
- **Output format**: New src/client/web/src/components/workspace/WorkspaceDrawer.tsx + tests.
- **Tool guidance**: Reuse SessionDrawer's inert-guard pattern (do NOT share the component); route stale aria-live through adr-20260624-0057 single-slot channel.
- **Task boundaries**: Does not implement FileViewer/DiffViewer/WorkspaceTree bodies; those live in their own units.
- **Files touched**: src/client/web/src/components/workspace/WorkspaceDrawer.tsx, src/client/web/src/components/workspace/WorkspaceDrawer.test.tsx
- **Acceptance**:
  - verify-stale-render-latency passes (banner within 500 ms; one aria-live per transition; reload clears banner + re-fetches).
  - verify-workspace-root-handle-pinning passes (handle snapshotted at open; frame-push does not re-resolve).
- **Contract refs**: contract-stale-banner-presentation, contract-workspace-root-resolution
- **Depends on**: activity-rail-shell-composition, workspace-activity-store
- **max_diff_loc**: 400

### Milestone m5

- **Depends on**: m4
- **Members**: component:component-file-viewer, component:component-diff-viewer, component:component-workspace-tree, component:component-workspace-vim-keymap, component:component-markdown-sanitizer, req:FR-009, req:FR-010, req:FR-011, req:FR-012, req:FR-019, req:FR-020, req:FR-021, req:FR-022, req:FR-023, req:FR-024, req:FR-025, adr:adr-20260714-wsviewer-markdown-xss-boundary

{% milestone id="m5" %}
- component:component-file-viewer
- component:component-diff-viewer
- component:component-workspace-tree
- component:component-workspace-vim-keymap
- component:component-markdown-sanitizer
- req:FR-009
- req:FR-010
- req:FR-011
- req:FR-012
- req:FR-019
- req:FR-020
- req:FR-021
- req:FR-022
- req:FR-023
- req:FR-024
- req:FR-025
- adr:adr-20260714-wsviewer-markdown-xss-boundary
{% /milestone %}

#### Units

##### file-viewer-and-metadata

- **Objective**: Implement FileViewer shell + MetadataPlaceholder + 1 MiB large-file virtualization (contract-large-file-threshold); dispatch to Markdown/Mermaid/JSON/source per file type; enforce sensitive-file exposure invariant (no filtering).
- **Output format**: src/client/web/src/components/workspace/FileViewer.tsx + MetadataPlaceholder.tsx + tests.
- **Tool guidance**: Windowed rendering (react-window-style); range fetch on scroll; content-sniffed binary detection.
- **Task boundaries**: Does not implement individual renderers (Markdown/Mermaid/JSON) — those are separate units.
- **Files touched**: src/client/web/src/components/workspace/FileViewer.tsx, src/client/web/src/components/workspace/MetadataPlaceholder.tsx, src/client/web/src/components/workspace/FileViewer.test.tsx
- **Acceptance**:
  - verify-large-file-scroll-bench passes for 6 MiB and 200 MiB fixtures.
  - Binary files render metadata only.
  - .env content is served verbatim (no masking).
  - verify-vim-mutation-integration passes (mutation keys have zero effect and no xterm leak).
- **Contract refs**: contract-large-file-threshold, contract-vim-keymap-bindings
- **Implementation decisions remaining**: binary-content-sniffing-heuristic
- **Depends on**: workspace-drawer
- **max_diff_loc**: 400

##### markdown-renderer

- **Objective**: Implement MarkdownRenderer wrapping react-markdown with rehype-sanitize schema (adr-20260714-wsviewer-markdown-xss-boundary); fail-closed to plain text on schema violation.
- **Output format**: src/client/web/src/components/workspace/renderers/MarkdownRenderer.tsx + src/client/web/src/lib/markdownSanitizer.ts + tests.
- **Tool guidance**: rehype-sanitize schema described in the ADR; fail-closed = swap to <pre>{rawSource}</pre> + banner.
- **Task boundaries**: Does not implement Mermaid/JSON renderers.
- **Files touched**: src/client/web/src/components/workspace/renderers/MarkdownRenderer.tsx, src/client/web/src/lib/markdownSanitizer.ts, src/client/web/src/lib/markdownSanitizer.test.ts
- **Acceptance**:
  - verify-markdown-sanitization-fixture passes (no forbidden token in DOM; malicious inputs render plain-text fallback with banner).
  - Clean fixtures render structured markdown.
- **Contract refs**: contract-markdown-sanitization, contract-structured-render-fallback
- **Implementation decisions remaining**: rehype-sanitize-schema-tuning
- **Depends on**: file-viewer-and-metadata
- **max_diff_loc**: 300

##### mermaid-renderer

- **Objective**: Implement MermaidRenderer with 300 ms parse timeout and raw-source fallback pane (adr-20260714-wsviewer-fallback-observability-bounds).
- **Output format**: src/client/web/src/components/workspace/renderers/MermaidRenderer.tsx + tests.
- **Tool guidance**: Run parser in a Web Worker with a hard timeout; on timeout/parse-error render <pre>{rawSource}</pre> + banner.
- **Task boundaries**: Does not touch Markdown/JSON renderers.
- **Files touched**: src/client/web/src/components/workspace/renderers/MermaidRenderer.tsx, src/client/web/src/components/workspace/renderers/MermaidRenderer.test.tsx
- **Acceptance**:
  - verify-structured-fallback-bound passes for Mermaid inputs.
  - Clean Mermaid renders as SVG.
- **Contract refs**: contract-structured-render-fallback
- **Implementation decisions remaining**: mermaid-worker-configuration
- **Depends on**: file-viewer-and-metadata
- **max_diff_loc**: 250

##### json-tree-renderer

- **Objective**: Implement JsonTreeRenderer as collapsible tree with 300 ms parse timeout and raw-text fallback pane; lazy expansion for large JSON (NFR-002).
- **Output format**: src/client/web/src/components/workspace/renderers/JsonTreeRenderer.tsx + tests.
- **Tool guidance**: Yield to event loop between top-level keys; enforce parse timeout.
- **Task boundaries**: Does not touch Markdown/Mermaid renderers.
- **Files touched**: src/client/web/src/components/workspace/renderers/JsonTreeRenderer.tsx, src/client/web/src/components/workspace/renderers/JsonTreeRenderer.test.tsx
- **Acceptance**:
  - verify-structured-fallback-bound passes for JSON inputs.
  - 5 MiB JSON fixture reaches first paint within 1500 ms (NFR-002).
- **Contract refs**: contract-structured-render-fallback
- **Implementation decisions remaining**: json-tree-batch-size
- **Depends on**: file-viewer-and-metadata
- **max_diff_loc**: 250

##### diff-viewer

- **Objective**: Implement DiffViewer with unified layout, non-color line cues, large-diff folding, and non-git degraded banner per typed outcome from workspace-fs-api diff endpoint.
- **Output format**: src/client/web/src/components/workspace/DiffViewer.tsx + tests.
- **Tool guidance**: Leading +/-/~ + paired icon; fold at >500 changed lines with named hidden-count control.
- **Task boundaries**: Does not touch git-diff helper (server-side); only the client rendering.
- **Files touched**: src/client/web/src/components/workspace/DiffViewer.tsx, src/client/web/src/components/workspace/DiffViewer.test.tsx
- **Acceptance**:
  - verify-diff-layout-a11y passes.
  - verify-diff-base-outcomes passes for the client-side banner mapping (all 4 outcome classes render distinct banners).
- **Contract refs**: contract-diff-view-layout, contract-diff-base-non-git-fallback
- **Implementation decisions remaining**: large-diff-fold-threshold-fine-tune
- **Depends on**: workspace-drawer
- **max_diff_loc**: 350

##### workspace-tree-and-refresh

- **Objective**: Implement WorkspaceTree with lazy expansion, refresh control (header button), typed-outcome banner rendering (ok / root_unreachable / refresh_failed) per adr-20260714-wsviewer-fallback-observability-bounds.
- **Output format**: src/client/web/src/components/workspace/WorkspaceTree.tsx + tests.
- **Tool guidance**: Never navigate above workspace root; refresh control activates the endpoint refresh.
- **Task boundaries**: Does not implement server tree endpoint (that is workspace-fs-api-handlers).
- **Files touched**: src/client/web/src/components/workspace/WorkspaceTree.tsx, src/client/web/src/components/workspace/WorkspaceTree.test.tsx
- **Acceptance**:
  - verify-tree-refresh-outcomes passes for all three scenarios (normal, torn-down, transport-fail).
  - Tree root shows only the workspace directory; no parent/sibling affordance.
- **Contract refs**: contract-tree-refresh
- **Implementation decisions remaining**: tree-node-batch-size
- **Depends on**: workspace-drawer
- **max_diff_loc**: 350

##### workspace-vim-keymap

- **Objective**: Implement pure motion/search keymap module + capture-phase listener wired by FileViewer; assert zero DOM change and zero xterm leak on mutation keys.
- **Output format**: src/client/web/src/lib/workspaceVimKeymap.ts + tests.
- **Tool guidance**: Pure state machine + capture-phase listener at viewer container.
- **Task boundaries**: Does not touch FileViewer body beyond the listener registration hook.
- **Files touched**: src/client/web/src/lib/workspaceVimKeymap.ts, src/client/web/src/lib/workspaceVimKeymap.test.ts
- **Acceptance**:
  - verify-vim-motion-unit passes (motion state machine).
  - verify-vim-mutation-integration passes (mutation keys inert; no xterm leak).
- **Contract refs**: contract-vim-keymap-bindings
- **Depends on**: file-viewer-and-metadata
- **max_diff_loc**: 250

## Targets (seams)

- server/web workspace.go — mirrors transcript.go shape; depguard-locked read-only handlers
- server/web workspace_path.go — per-segment EvalSymlinks + descendant-of-root
- platform/lib/git/git.go — HEAD-diff helper with typed non-git outcome partition
- client/runtime/tool_log.go — schema_version=2 evolution
- client/runtime/tool_log_reader.go — new tailer/classifier + mid_turn_touch emission
- server/web/viewupdate.go — activity_events extension carrying turn_row + mid_turn_touch
- client/web/src/store/workspaceActivity.ts — new Zustand store with cross-session guard
- client/web/src/components/workspace/* — ActivityRail, WorkspaceDrawer, FileViewer, DiffViewer, WorkspaceTree, renderers, MetadataPlaceholder
- client/web/src/lib/workspaceVimKeymap.ts — pure motion/search state machine + capture-phase listener
- client/web/src/lib/markdownSanitizer.ts — rehype-sanitize wrapper with fail-closed
- src/.golangci.yml — depguard rule for workspace-viewer handler package

## Verification

| Tier | Contract | Test | Command | Criterion |
|---|---|---|---|---|
| T1 | contract-appshell-preserve | verify-appshell-terminal-rect-nonregression | `cd src/client/web && pnpm vitest run components/workspace/ActivityRail.test.tsx` | delta(width, height, top, left) <= 1 px in both mounted and unmounted configurations. |
| T2 | contract-workspace-root-resolution | verify-workspace-root-handle-pinning | `cd src && go test ./server/web -run TestWorkspaceRootHandlePinning` | Second tree response resolves to the original root path OR returns 409 handle_stale with the original resolved_root_path echoed for diagnostic — never resolves against the pushed frame's StartDir. |
| T2 | contract-workspace-path-traversal-defense | verify-path-guard-symlink-escape | `cd src && go test ./server/web -run TestWorkspacePathGuard` | All out-of-root paths return 404 with zero bytes of out-of-root content in the response body; all in-root paths return 200. |
| T2 | contract-no-write-boundary | verify-no-write-lint | `make lint` | Injecting os.WriteFile into any workspace*.go causes `make lint` to fail; removing it makes lint pass. |
| T1 | contract-no-write-boundary | verify-no-write-verb-scan | `cd src && go test ./server/web -run TestWorkspaceReadOnlyVerbs` | All non-GET verbs return 405 (Method Not Allowed) or 404; no route accepts them. |
| T2 | contract-markdown-sanitization | verify-markdown-sanitization-fixture | `cd src/client/web && pnpm vitest run lib/markdownSanitizer.test.ts` | getRoot().querySelector('script,[onclick],a[href^=javascript],a[href^=data],img[src^=http]') is null for the malicious fixture AND the fallback banner is visible. |
| T1 | contract-large-file-threshold | verify-large-file-scroll-bench | `cd src/client/web && pnpm playwright test workspace/large-file-scroll.spec.ts && pnpm vitest run components/workspace/FileViewer.test.tsx -t verify-large-file-scroll-bench` | 6 MiB Playwright + 200 MiB Vitest: p95 per-scroll re-paint <= 33 ms; final visible line equals fixture last line; no '...(truncated)' in viewer DOM. |
| T1 | contract-structured-render-fallback | verify-structured-fallback-bound | `cd src/client/web && pnpm vitest run components/workspace/renderers` | Clean inputs render structured within 300 ms; adversarial inputs render fallback pane with banner within 400 ms; no case leaves a blank/loading pane past 400 ms. |
| T0 | contract-vim-keymap-bindings | verify-vim-motion-unit | `cd src/client/web && pnpm vitest run lib/workspaceVimKeymap.test.ts` | j moves down one line; k up; gg to top; G to bottom; / opens search; n advances to next match; N previous. |
| T1 | contract-vim-keymap-bindings | verify-vim-mutation-integration | `cd src/client/web && pnpm vitest run components/workspace/FileViewer.test.tsx` | Viewer.innerHTML unchanged; workspace endpoints receive zero PUT/POST/PATCH/DELETE; xterm mock received zero keydown. |
| T2 | contract-secondary-tree-entry | verify-workspace-affordance-a11y | `cd src/client/web && pnpm vitest run components/workspace/ActivityRail.test.tsx` | Both rows-0 and rows-5 configurations expose the affordance; keyboard activation opens the drawer with Tree tab focused. |
| T2 | contract-tree-refresh | verify-tree-refresh-outcomes | `cd src/client/web && pnpm vitest run components/workspace/WorkspaceTree.test.tsx` | All three scenarios produce the declared observable state (new file visible / torn-down banner + retry / refresh-failed banner + retry); tree is never silently up-to-date after a failure. |
| T1 | contract-diff-view-layout | verify-diff-layout-a11y | `cd src/client/web && pnpm vitest run components/workspace/DiffViewer.test.tsx` | All changed lines carry non-color cues; large-diff folding visible for 5000-line case; simulated scroll frame-time p95 <= 33 ms. |
| T2 | contract-diff-base-non-git-fallback | verify-diff-base-outcomes | `cd src && go test ./platform/lib/git ./server/web -run TestDiffBase` | Each scenario returns the declared typed outcome; DiffViewer test double asserts distinct banner strings per class. |
| T2 | contract-activity-event-source | verify-activity-classification | `cd src && go test ./client/runtime -run TestToolLogReaderClassification` | Row set matches expected structured entries only; counters match expected increments; no fabricated paths appear anywhere. |
| T1 | contract-turn-aggregation | verify-turn-aggregation | `cd src && go test ./client/runtime -run TestTurnAggregation` | (a) 1 row count=3. (b) 1 row count=3. (c) 2 rows (first count=3 with failure indicator, second is the post-stop calls). (d) parent turn has 1 nested sub-row for the sub-agent + separate row for the parent call. |
| T1 | contract-live-background-transport | verify-transport-latency-bound | `cd src && go test ./server/web -run TestViewUpdateActivityLatency && cd client/web && pnpm vitest run store/workspaceActivity.test.ts` | turn_row p95 <= 750 ms; mid_turn_touch -> stale banner p95 <= 500 ms; cross-session payload discarded 100 % of times; drop triggers visible connectivity indicator within 1 s. |
| T1 | contract-stale-banner-presentation | verify-stale-render-latency | `cd src/client/web && pnpm vitest run components/workspace/WorkspaceDrawer.test.tsx` | Banner appears within 500 ms; one aria-live message per transition; reload clears banner and issues a fresh file/diff fetch. |

## Contracts

### contract-appshell-preserve

- **Dimension**: integration_contract
- **Owner**: component-activity-rail
- **Subject**: Wrapping-container composition inside the existing main grid area preserves AppShell's named-grid-area layout and MainTabs' ADR-0065 terminal-slot rendered rect whether ActivityRail is mounted or not.
- **Requirements**: FR-027
- **ADRs**: adr-20260714-wsviewer-appshell-composition
- **Invariants**:
  - MainTabs' terminal-slot rendered rect is byte-identical (rounded to 1 CSS px) with/without ActivityRail mounted.
  - AppShell's grid-template-areas string is unchanged by this feature.

### contract-workspace-root-resolution

- **Dimension**: state_lifecycle
- **Owner**: component-workspace-drawer
- **Subject**: Drawer-scoped WorkspaceRootHandle snapshots the workspace root at drawer open (SSOT triple: DEvWorktreeResolved.WorktreeStartDir > LaunchPlan.StartDir > Session.Project) and pins it for the drawer's entire lifetime.
- **Requirements**: FR-028, FR-018
- **ADRs**: adr-20260714-wsviewer-workspace-root-handle
- **Invariants**:
  - Two requests belonging to the same drawer session always resolve against the same workspace root.
  - A background frame push/pop between two requests of the same drawer produces a typed handle-stale banner, not a silent root switch.

### contract-workspace-path-traversal-defense

- **Dimension**: security_boundary
- **Owner**: component-workspace-path-guard
- **Subject**: Per-segment EvalSymlinks-based descendant check on every workspace path parameter using the WorkspaceRootHandle-snapshotted root.
- **Requirements**: FR-029, FR-018
- **ADRs**: adr-20260714-wsviewer-path-guard-symlink
- **Invariants**:
  - No response ever contains bytes from a path whose EvalSymlinks-normalized form escapes the EvalSymlinks-normalized workspace root.
  - Traversal via any combination of '..', encoded traversal, or intermediate directory symlink is rejected before any fs read.

### contract-no-write-boundary

- **Dimension**: security_boundary
- **Owner**: component-workspace-fs-api
- **Subject**: Structural absence of fs-mutating imports in the workspace-viewer handler package, enforced by a depguard rule in src/.golangci.yml.
- **Requirements**: FR-026
- **ADRs**: adr-20260714-wsviewer-no-write-depguard
- **Invariants**:
  - No non-GET verb handler is registered under the /api/sessions/{id}/workspace/ prefix.
  - No fs-mutating symbol is importable from src/server/web/workspace*.go per depguard rule.

### contract-markdown-sanitization

- **Dimension**: security_boundary
- **Owner**: component-markdown-sanitizer
- **Subject**: Markdown renderer sanitizes via rehype-sanitize schema; forbidden tokens (script tags, on* handlers, javascript:/data: hrefs, off-workspace img src) never reach the DOM; sanitizer rejection fails closed to a plain-text fallback pane.
- **Requirements**: FR-012, FR-009
- **ADRs**: adr-20260714-wsviewer-markdown-xss-boundary
- **Invariants**:
  - No forbidden token (script tag, on* handler, javascript:/data: href, off-workspace img src) reaches the rendered DOM.
  - Sanitizer rejection produces the plain-text fallback pane; no partial rich rendering is ever displayed.

### contract-large-file-threshold

- **Dimension**: performance_budget
- **Owner**: component-file-viewer
- **Subject**: Text-file large threshold fixed at 1 MiB; virtualized scrolling reaches true EOF; no truncation banner ever appears regardless of file size.
- **Requirements**: FR-023
- **Invariants**:
  - No '...(truncated)' or equivalent placeholder appears in the viewer for any text file size.
  - Scrolling to the visual EOF renders the actual final bytes of the file.

### contract-structured-render-fallback

- **Dimension**: failure_recovery
- **Owner**: component-file-viewer
- **Subject**: Mermaid and JSON renderers parse within 300 ms per file; on parse failure, timeout, or inconclusive outcome, a raw-source/raw-text fallback pane renders within an additional 100 ms; outcome partition is closed (success / parse_error / timeout / unknown).
- **Requirements**: FR-010, FR-011
- **ADRs**: adr-20260714-wsviewer-fallback-observability-bounds
- **Invariants**:
  - The viewer never remains blank or in a loading spinner for more than 400 ms after opening a structured-renderable file.
  - Every fallback render carries a visible banner explaining the reason (parse error, timeout, generic).

### contract-vim-keymap-bindings

- **Dimension**: control_flow
- **Owner**: component-workspace-vim-keymap
- **Subject**: Exact vim motion/search keymap (j/k/gg/G, /,n,N) gated by a capture-phase listener bound to viewer focus; mutation keys (i, o, dd, :w, and any non-motion/search key) neither mutate content nor leak to xterm.js.
- **Requirements**: FR-021, FR-022
- **Invariants**:
  - Every non-motion/non-search keydown is fully stopped at capture phase while viewer has focus; xterm.js receives none of them.
  - Mutation-shaped keys (i, o, dd, :w) produce zero DOM change and zero underlying file write.

### contract-secondary-tree-entry

- **Dimension**: user_observability
- **Owner**: component-activity-rail
- **Subject**: Workspace secondary-tree-access affordance is always visible in ActivityRail (row count 0 or otherwise), reachable via Tab in the rail's roving-tabindex sequence, activatable by pointer / Enter / Space, and carries a distinct screen-reader label.
- **Requirements**: FR-016
- **Invariants**:
  - Workspace affordance is present in DOM whether the rail has 0 rows or many rows.
  - Its accessible name is distinct from any per-row control.

### contract-tree-refresh

- **Dimension**: state_lifecycle
- **Owner**: component-workspace-tree
- **Subject**: WorkspaceTree exposes an in-UI reload control (header refresh button) whose activation re-fetches the current visible tree state; workspace-root-unreachable produces a distinct typed response and a visible banner (never a silently empty tree).
- **Requirements**: FR-019, FR-020, FR-018
- **ADRs**: adr-20260714-wsviewer-fallback-observability-bounds
- **Invariants**:
  - The tree never appears refreshed unless the last request returned a normal listing; every non-normal outcome renders a visible banner.
  - Workspace-root-unreachable is a distinct typed response, never conflated with 'empty directory'.

### contract-diff-view-layout

- **Dimension**: user_observability
- **Owner**: component-diff-viewer
- **Subject**: Unified diff as default layout; added/removed/changed lines carry non-color cues (leading +/-/~ symbol + icon) alongside color; large diffs (>500 changed lines) fold intermediate hunks with a visible expand control.
- **Requirements**: FR-013
- **Invariants**:
  - Every diff line carries a non-color cue (leading symbol + paired icon) in addition to color.
  - Diffs with >500 changed lines render with folded intermediate hunks; hidden count is visible.

### contract-diff-base-non-git-fallback

- **Dimension**: failure_recovery
- **Owner**: component-git-diff-head
- **Subject**: git-diff-helper returns a typed outcome partition (ok / not_a_repo / git_metadata_corrupted / git_binary_missing); DiffViewer surfaces a distinct banner per non-ok class.
- **Requirements**: FR-014
- **ADRs**: adr-20260714-wsviewer-fallback-observability-bounds
- **Invariants**:
  - Non-git or degraded git states never produce a normal-looking empty diff.
  - Each non-ok class has distinct banner copy so operators can differentiate cause.

### contract-activity-event-source

- **Dimension**: data_model
- **Owner**: component-tool-log-reader
- **Subject**: Tool-log JSONL schema_version=2 adds turn_id and normalized file-event kind fields; the reader classifies tool calls into {read, create, edit, delete, unclassified} without fabricating paths; legacy (pre-v2) lines are skipped with a named diagnostic counter.
- **Requirements**: FR-002, FR-003, FR-015
- **ADRs**: adr-20260714-wsviewer-tool-log-schema-and-turn-boundary
- **Invariants**:
  - No activity row is emitted for an entry without a structured, workspace-relative file_path.
  - Legacy entries are skipped with a counter increment; no field synthesis; no phantom row.

### contract-turn-aggregation

- **Dimension**: state_lifecycle
- **Owner**: component-tool-log-reader
- **Subject**: Turn-boundary semantics grouping tool calls into rows: Codex uses SubsystemTurnStarted/Completed; Claude uses a synthesized counter incremented on Stop and StopFailure, with SubagentStart/Stop delimiting nested sub-turns.
- **Requirements**: FR-002
- **ADRs**: adr-20260714-wsviewer-tool-log-schema-and-turn-boundary
- **Invariants**:
  - Same-path calls within one turn always collapse to one row.
  - Same-path calls across a turn boundary always appear as separate rows.
  - Sub-agent calls appear as nested sub-rows, not merged into the parent turn.

### contract-live-background-transport

- **Dimension**: concurrency
- **Owner**: component-workspace-activity-store
- **Subject**: ViewUpdate WS extension carries {turn_row, mid_turn_touch} events per session; end-to-end append-to-render latency bounded p95 750 ms (rows) / 500 ms (mid-turn touch -> stale banner); session scoping preserves adr-20260705-view-update-sessions-only.
- **Requirements**: FR-005, FR-008
- **ADRs**: adr-20260714-wsviewer-live-transport-and-mid-turn-stale
- **Invariants**:
  - Turn-row latency append-to-render p95 <= 750 ms.
  - Mid-turn touch -> stale banner render p95 <= 500 ms.
  - Cross-session payload leak is structurally impossible on the client store.
  - A WS drop always surfaces a visible connectivity-degraded indicator until reconnect.

### contract-stale-banner-presentation

- **Dimension**: user_observability
- **Owner**: component-workspace-drawer
- **Subject**: Drawer stale banner + reload affordance render within 500 ms of a mid-turn PostToolUse on the drawer's open file; screen-reader announcement is coalesced to exactly one message per stale transition through the single aria-live slot (adr-20260624-0057).
- **Requirements**: FR-006, FR-007
- **ADRs**: adr-20260714-wsviewer-live-transport-and-mid-turn-stale
- **Invariants**:
  - Mid-turn PostToolUse -> stale banner render p95 <= 500 ms.
  - Exactly one aria-live announcement per stale transition; rapid repeats do not flood the slot.
  - Reload affordance always clears the banner and refreshes viewer/diff content.

## ADR Trace

- adr-20260714-wsviewer-appshell-composition: Workspace viewer composes via wrapping container preserving ADR-0065 terminal-slot rect — contracts: contract-appshell-preserve
- adr-20260714-wsviewer-workspace-root-handle: Drawer-scoped WorkspaceRootHandle pins workspace root for drawer lifetime — contracts: contract-workspace-root-resolution
- adr-20260714-wsviewer-path-guard-symlink: Per-segment symlink evaluation for workspace path traversal defense — contracts: contract-workspace-path-traversal-defense
- adr-20260714-wsviewer-tool-log-schema-and-turn-boundary: Tool-log schema versioning, reader classification, and Claude Stop-family turn boundary — contracts: contract-activity-event-source, contract-turn-aggregation
- adr-20260714-wsviewer-live-transport-and-mid-turn-stale: ViewUpdate WS extension carries activity rows and mid-turn stale signals with a 750 ms end-to-end latency bound — contracts: contract-live-background-transport, contract-stale-banner-presentation
- adr-20260714-wsviewer-markdown-xss-boundary: Markdown renderer sanitizes via rehype-sanitize; fail-closed to plain text — contracts: contract-markdown-sanitization
- adr-20260714-wsviewer-no-write-depguard: depguard rule structurally forbids fs-mutating calls in the workspace-viewer handler package — contracts: contract-no-write-boundary
- adr-20260714-wsviewer-fallback-observability-bounds: Parse-timeout, tree-torn-down, and non-git degradation observables have closed epistemic partitions and numeric bounds — contracts: contract-structured-render-fallback, contract-tree-refresh, contract-diff-base-non-git-fallback

## Open Questions

- Claude Bash-based delete detection is scoped out of v1 (unclassified diagnostic counter); future work may add pattern-recognition ADR if operator feedback shows the gap matters.
- Non-git workspace prevalence (ux open_questions carry-over) remains unmeasured; contract-diff-base-non-git-fallback already partitions the outcome so the presentation can be measured in production without contract change.

## Implementation Decisions Remaining

- **binary-content-sniffing-heuristic**: alternatives = http.DetectContentType on the first 512 bytes (Go stdlib) forwarded to the client as MIME type; file(1)-style magic-number library invoked via a small Go dependency. Invariance: Both alternatives produce a deterministic binary/text classification for the same bytes; UAC-025 observable requires only that binary files show metadata-only (regardless of which detection library reports the MIME). Verified by verify-large-file-scroll-bench.
- **rehype-sanitize-schema-tuning**: alternatives = Start from rehype-sanitize defaultSchema then whitelist code-fence highlighting classes; Custom minimal schema listing only structural markdown elements. Invariance: Both preserve the invariant that no forbidden token (script tag, on* handler, javascript:/data: href, off-workspace img src) reaches the DOM; the T2 contract test asserts on that invariant, not on which internal schema was configured. Verified by verify-markdown-sanitization-fixture.
- **mermaid-worker-configuration**: alternatives = Web Worker with structured-clone parser bootstrap; iframe sandbox running the Mermaid parser off the main thread. Invariance: Both preserve the 300 ms hard timeout + fallback-within-400 ms observable; T1 test asserts on the observable, not the parse execution mechanism. Verified by verify-structured-fallback-bound.
- **json-tree-batch-size**: alternatives = Batch 100 top-level keys per event-loop yield; Adaptive batch size tuned to keep per-yield <=8 ms. Invariance: Both preserve the tab-remains-interactive invariant (NFR-002) and the fallback-within-400 ms partition; the T1 timing test asserts the observable outcome. Verified by verify-structured-fallback-bound.
- **large-diff-fold-threshold-fine-tune**: alternatives = Fixed threshold: 500 changed lines triggers folding; Adaptive threshold: viewport-height * heuristic factor. Invariance: Both preserve the 'large diffs render folded with visible expand control naming hidden hunk count' observable; the T1 test asserts on the presence of the expand control and frame-time bound, not on which numeric threshold triggered it. Verified by verify-diff-layout-a11y.
- **tree-node-batch-size**: alternatives = Fetch immediate children only per expand click; Prefetch 1 additional depth level during idle time. Invariance: Both preserve the lazy-expansion invariant (NFR-001) and the refresh-affordance observable; the T2 test asserts on outcome partitions (ok/root_unreachable/refresh_failed) independent of prefetch strategy. Verified by verify-tree-refresh-outcomes.


````
