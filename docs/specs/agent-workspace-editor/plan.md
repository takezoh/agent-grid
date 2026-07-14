---
id: plan-20260714-agent-workspace-editor
kind: plan
title: Agent workspace mutation-capable editor
status: done
methodology: sdd
created: '2026-07-14'
updated: '2026-07-14'
tags:
- workspace-editor
- write
- sdd
owners: []
goal: agent-grid web UI の workspace read-only viewer (spec-20260714-agent-workspace-viewer)
  に mutation-capable な editor 経路を追加し、operator が vim mutation keys (i/a/o/dd/x/c/y/p/u/Ctrl-R/:w)
  で実際にファイルを編集・保存できるようにする。sensitive file 区別は read 側同様に持たず、path-traversal defense と
  workspace-root 境界は継続する。
scope_in:
- 既存 workspaceVimKeymap.ts MUTATION_KEYS no-op を CodeMirror 6 + @codemirror/vim による実編集に格上げ
- FileViewer の source 表示を編集可能 buffer として再構成 (renderer/virtualization 維持)
- '新規 backend write handler (workspace_write.go): PUT /workspace/file, GuardWorkspacePath
  再利用, tmp+rename atomic, 1 MiB body cap, If-Unmodified-Since optimistic lock, Bearer
  認証継承'
- 'adr-20260714-wsviewer-no-write-depguard の supersedes: forbidigo pkg pattern の module
  path 修正 + per-file exclusion by exclusions.rules'
- drawer chrome の dirty indicator + unsaved-close warning + conflict banner (keep-mine/take-theirs/merge)
  + root_disappeared banner
- operator write を actor=operator の distinct row として ActivityRail に載せる audit trail
  (schema_version=2 additive)
- WS reconnect 時の全 dirty buffer に対する強制 mtime re-fetch (silent overwrite window の閉塞)
- 継承 invariants (workspace-relative paths / workspace-root boundary / turn aggregation
  / live-background / git HEAD diff base) の write 側での継続適用
- sensitive file (.env 等) に対する write 側フィルタ不在の明示継続 (read 側 DP-SENSITIVE-FILE-EXPOSURE=OPT-NO-FILTER
  と対称)
scope_out:
- vim keymap + :w 以外の write 経路 (WorkspaceTree の rename/drag-drop/削除等)
- 複数 operator による同時協調編集
- オフライン/再接続後のキュー済み write (save は live connection 前提)
- sensitive file の新規フィルタ / マスキング
- 複数 workspace root の横断 write、現セッション以外の workspace への write
- crash-across-durability (fsync 追加は future ADR)
- cross-tab / persistent な undo stack (session-local に閉じる)
milestones:
- id: m1
  title: workspace-fs-writer-handler ...
  status: done
- id: m2
  title: audit-schema-additive-actor ...
  status: done
- id: m3
  title: codemirror-editor-swap ...
  status: done
contracts:
- contract-vim-mutation-editing
- contract-write-persistence-save
- contract-write-atomicity
- contract-write-conflict-detection
- contract-write-audit-trail
- contract-dirty-state-visibility
- contract-vim-undo-locality
- contract-write-permission-parity
- contract-write-structural-boundary
- contract-editor-large-file-editing-performance
- contract-encoding-newline-fidelity
- contract-audit-schema-migration
- contract-large-write-resource-bound
- contract-workspace-root-disappearance
- contract-write-endpoint-authz
contract_projections:
- id: contract-vim-mutation-editing
  decision_rules:
  - decision-mutation-key-dispatch
  observable_effects:
  - observable-buffer-content-changed
  operational_inputs:
  - input-editor-buffer-state
  semantic_profiles: []
  failures:
  - failure-vim-mutation-unsupported-key
  verifications:
  - verify-vim-mutation-editing
  witnesses:
  - witness-vim-mutation-normal
  - witness-vim-mutation-operator-boundary-adversarial
- id: contract-write-persistence-save
  decision_rules:
  - decision-write-endpoint-response
  observable_effects:
  - observable-save-response-and-persisted-bytes
  operational_inputs:
  - input-write-request-payload
  semantic_profiles: []
  failures:
  - failure-write-endpoint-precondition-mismatch
  verifications:
  - verify-write-persistence-save
  witnesses:
  - witness-write-persistence-normal
  - witness-write-persistence-precondition-adversarial
- id: contract-write-atomicity
  decision_rules:
  - decision-atomicity-success
  - decision-atomicity-syscall-failure
  - decision-atomicity-syscall-unknown
  - decision-atomicity-syscall-inconclusive
  observable_effects:
  - observable-post-save-disk-state
  operational_inputs:
  - input-fs-write-outcome
  semantic_profiles:
  - profile-atomicity-outcome-partition
  failures:
  - failure-atomicity-syscall-failure
  - failure-atomicity-syscall-unknown
  - failure-atomicity-syscall-inconclusive
  verifications:
  - verify-write-atomicity
  witnesses:
  - witness-atomicity-normal
  - witness-atomicity-syscall-failure-adversarial
  - witness-atomicity-unknown-adversarial
  - witness-atomicity-inconclusive-adversarial
- id: contract-write-conflict-detection
  decision_rules:
  - decision-conflict-none
  - decision-conflict-dirty-background
  - decision-conflict-signal-lost
  - decision-conflict-conflicting-signals
  - decision-conflict-inconclusive-mtime
  observable_effects:
  - observable-conflict-banner-state
  operational_inputs:
  - input-dirty-buffer-flag
  - input-background-write-touch-signal
  - input-reconnect-mtime-snapshot
  semantic_profiles:
  - profile-conflict-outcome-partition
  failures:
  - failure-conflict-signal-lost
  - failure-conflict-conflicting
  - failure-conflict-inconclusive
  verifications:
  - verify-write-conflict-detection
  witnesses:
  - witness-conflict-clean-normal
  - witness-conflict-dirty-and-background-adversarial
  - witness-conflict-reconnect-adversarial
  - witness-conflict-signal-lost-adversarial
  - witness-conflict-conflicting-signals-adversarial
  - witness-conflict-inconclusive-adversarial
- id: contract-write-audit-trail
  decision_rules:
  - decision-audit-emit
  observable_effects:
  - observable-operator-row-visible
  operational_inputs:
  - input-operator-save-event
  semantic_profiles: []
  failures:
  - failure-audit-emit
  verifications:
  - verify-write-audit-trail
  witnesses:
  - witness-audit-normal
  - witness-audit-emit-failure-adversarial
- id: contract-dirty-state-visibility
  decision_rules:
  - decision-dirty-visibility
  observable_effects:
  - observable-dirty-indicator-and-close-warning
  operational_inputs: []
  semantic_profiles: []
  failures:
  - failure-dirty-indicator-render-delay
  verifications:
  - verify-dirty-state-visibility
  witnesses:
  - witness-dirty-normal
  - witness-dirty-close-warning-adversarial
- id: contract-vim-undo-locality
  decision_rules:
  - decision-undo-scope
  observable_effects:
  - observable-undo-scope-effect
  operational_inputs:
  - input-local-undo-stack
  semantic_profiles: []
  failures:
  - failure-undo-stack-empty
  verifications:
  - verify-vim-undo-locality
  witnesses:
  - witness-undo-normal
  - witness-undo-reload-clears-adversarial
- id: contract-write-permission-parity
  decision_rules:
  - decision-write-no-sensitive-filter
  observable_effects:
  - observable-write-no-sensitive-block
  operational_inputs: []
  semantic_profiles: []
  failures:
  - failure-write-out-of-root
  verifications:
  - verify-write-permission-parity
  witnesses:
  - witness-write-permission-normal
  - witness-write-permission-out-of-root-adversarial
- id: contract-write-structural-boundary
  decision_rules:
  - decision-write-structural-boundary
  observable_effects:
  - observable-structural-boundary-enforcement
  operational_inputs: []
  semantic_profiles: []
  failures:
  - failure-structural-boundary-drift
  verifications:
  - verify-write-structural-boundary
  witnesses:
  - witness-structural-normal
  - witness-structural-boundary-adversarial
- id: contract-editor-large-file-editing-performance
  decision_rules:
  - decision-large-file-edit-cost
  observable_effects:
  - observable-large-file-edit-latency
  operational_inputs:
  - input-large-buffer-edit-cost
  semantic_profiles:
  - profile-large-file-edit-cost
  failures:
  - failure-large-file-edit-latency-exceeded
  verifications:
  - verify-editor-large-file-editing-performance
  witnesses:
  - witness-large-file-edit-normal
  - witness-large-file-edit-burst-adversarial
- id: contract-encoding-newline-fidelity
  decision_rules:
  - decision-encoding-byte-transparent
  observable_effects:
  - observable-untouched-region-byte-equivalence
  operational_inputs:
  - input-original-file-bytes
  semantic_profiles: []
  failures:
  - failure-encoding-unsupported
  verifications:
  - verify-encoding-newline-fidelity
  witnesses:
  - witness-encoding-lf-normal
  - witness-encoding-crlf-preserved-adversarial
- id: contract-audit-schema-migration
  decision_rules:
  - decision-audit-migration-additive
  observable_effects:
  - observable-audit-record-classification
  operational_inputs:
  - input-audit-record-actor-field
  semantic_profiles:
  - profile-audit-schema-evolution
  failures:
  - failure-audit-schema-migration-unknown-actor
  verifications:
  - verify-audit-schema-migration
  - verify-audit-schema-migration-baseline-v2
  witnesses:
  - witness-audit-legacy-v2-normal
  - witness-audit-migration-mixed-adversarial
- id: contract-large-write-resource-bound
  decision_rules:
  - decision-write-body-cap
  observable_effects:
  - observable-write-body-cap-decision
  operational_inputs:
  - input-request-body-size
  semantic_profiles:
  - profile-large-write-cost
  failures:
  - failure-write-oversize-body
  verifications:
  - verify-large-write-resource-bound
  witnesses:
  - witness-write-resource-normal
  - witness-write-resource-oversize-adversarial
- id: contract-workspace-root-disappearance
  decision_rules:
  - decision-root-present
  - decision-root-disappeared
  - decision-root-existence-unknown
  - decision-root-existence-inconclusive
  observable_effects:
  - observable-root-disappearance-surface
  operational_inputs:
  - input-workspace-root-existence
  semantic_profiles:
  - profile-root-disappearance-outcome-partition
  failures:
  - failure-root-disappeared
  - failure-root-unknown
  - failure-root-inconclusive
  verifications:
  - verify-workspace-root-disappearance
  witnesses:
  - witness-root-present-normal
  - witness-root-removed-adversarial
  - witness-root-unknown-adversarial
  - witness-root-inconclusive-adversarial
- id: contract-write-endpoint-authz
  decision_rules:
  - decision-write-authz
  observable_effects:
  - observable-authz-response
  operational_inputs:
  - input-bearer-token
  semantic_profiles: []
  failures:
  - failure-authz-missing
  verifications:
  - verify-write-endpoint-authz
  witnesses:
  - witness-authz-authorized-normal
  - witness-authz-missing-token-adversarial
adrs:
- adr-20260714-editor-write-depguard-rescope
- adr-20260714-editor-write-endpoint-shape
- adr-20260714-editor-concurrency-optimistic-lock
- adr-20260714-editor-codemirror-vim-engine
- adr-20260714-editor-encoding-byte-transparent
- adr-20260714-editor-undo-scope-viewer-session-local
- adr-20260714-editor-atomicity-tmp-rename
- adr-20260714-editor-audit-schema-additive-actor
- adr-20260714-editor-audit-emission-failfast
- adr-20260714-editor-conflict-ui-nonblocking
- adr-20260714-editor-reconnect-resync
- adr-20260714-editor-aria-live-precedence
- adr-20260714-editor-sensitive-file-no-filter
- adr-20260714-editor-root-disappearance-degrades-save
decision_dispositions:
- decision_input_ref: decision-input-editor-lib-codemirror-vim
  disposition: adopted
  rationale: CodeMirror 6 + @codemirror/vim を primary engine に確定し hand-roll/Monaco
    は drop。undo/redo/register の counterexample-backed evidence を根拠とする。
  adr_refs:
  - adr-20260714-editor-codemirror-vim-engine
  - adr-20260714-editor-encoding-byte-transparent
  - adr-20260714-editor-undo-scope-viewer-session-local
  contract_refs:
  - contract-vim-mutation-editing
  - contract-vim-undo-locality
  - contract-encoding-newline-fidelity
  - contract-editor-large-file-editing-performance
- decision_input_ref: decision-input-inherited-workspace-relative-paths
  disposition: adopted
  rationale: read 側 exp-workspace-relative-paths を継承し、write endpoint も workspace-relative
    path で解決する。
  adr_refs:
  - adr-20260714-editor-write-endpoint-shape
  contract_refs:
  - contract-write-persistence-save
- decision_input_ref: decision-input-inherited-workspace-root-boundary
  disposition: adopted
  rationale: read 側 GuardWorkspacePath を write 側でも per-request 適用する (path traversal
    defense 継続)。
  adr_refs:
  - adr-20260714-editor-write-depguard-rescope
  contract_refs:
  - contract-write-permission-parity
  - contract-write-structural-boundary
- decision_input_ref: decision-input-inherited-turn-aggregation
  disposition: adopted
  rationale: 既存 tool_log_reader の turn aggregation grouping を additive actor field
    経由で再利用する。
  adr_refs:
  - adr-20260714-editor-audit-schema-additive-actor
  contract_refs:
  - contract-audit-schema-migration
- decision_input_ref: decision-input-inherited-live-background
  disposition: adopted
  rationale: 既存 mid_turn_touch 転送を conflict detector 入力に再利用し、reconnect resync を追加する。
  adr_refs:
  - adr-20260714-editor-reconnect-resync
  - adr-20260714-editor-aria-live-precedence
  contract_refs:
  - contract-write-conflict-detection
  - contract-dirty-state-visibility
- decision_input_ref: decision-input-inherited-git-head-diff-base
  disposition: adopted
  rationale: write 後の diff base も git HEAD を再利用する (既存 DiffHeadVsWorktree)。write が
    diff base を変えない invariant として維持。
  adr_refs: []
  contract_refs:
  - contract-write-persistence-save
- decision_input_ref: decision-input-reversed-no-write-depguard
  disposition: adopted
  rationale: 新 ADR adr-20260714-editor-write-depguard-rescope が accepted adr-20260714-wsviewer-no-write-depguard
    を supersedes として明示 rescope する。
  adr_refs:
  - adr-20260714-editor-write-depguard-rescope
  contract_refs:
  - contract-write-structural-boundary
- decision_input_ref: decision-input-sensitive-file-no-filter-parity
  disposition: adopted
  rationale: read 側 DP-SENSITIVE-FILE-EXPOSURE=OPT-NO-FILTER を write 側でも継続。user consultation
    決定(2b) に基づく。adr-20260714-editor-sensitive-file-no-filter で明示的に ADR 化。
  adr_refs:
  - adr-20260714-editor-sensitive-file-no-filter
  contract_refs:
  - contract-write-permission-parity
- decision_input_ref: decision-input-write-scope-vim-only
  disposition: adopted
  rationale: write は viewer + vim keymap → save endpoint のみ。WorkspaceTree の rename/drag-drop
    は本 design で明示的に out-of-scope。
  adr_refs:
  - adr-20260714-editor-atomicity-tmp-rename
  - adr-20260714-editor-audit-schema-additive-actor
  - adr-20260714-editor-audit-emission-failfast
  - adr-20260714-editor-root-disappearance-degrades-save
  contract_refs:
  - contract-write-structural-boundary
  - contract-write-atomicity
  - contract-write-audit-trail
  - contract-workspace-root-disappearance
- decision_input_ref: decision-input-concurrency-policy-candidates
  disposition: adopted
  rationale: optimistic lock (If-Unmodified-Since) を確定。LWW と operator-priority は却下。
  adr_refs:
  - adr-20260714-editor-concurrency-optimistic-lock
  - adr-20260714-editor-conflict-ui-nonblocking
  contract_refs:
  - contract-write-persistence-save
  - contract-write-conflict-detection
reference_algorithms: []
relations:
- {type: implements, target: spec-20260714-agent-workspace-editor}
- {type: hasPart, target: adr-20260714-editor-write-depguard-rescope}
- {type: hasPart, target: adr-20260714-editor-write-endpoint-shape}
- {type: hasPart, target: adr-20260714-editor-concurrency-optimistic-lock}
- {type: hasPart, target: adr-20260714-editor-codemirror-vim-engine}
- {type: hasPart, target: adr-20260714-editor-encoding-byte-transparent}
- {type: hasPart, target: adr-20260714-editor-undo-scope-viewer-session-local}
- {type: hasPart, target: adr-20260714-editor-atomicity-tmp-rename}
- {type: hasPart, target: adr-20260714-editor-audit-schema-additive-actor}
- {type: hasPart, target: adr-20260714-editor-audit-emission-failfast}
- {type: hasPart, target: adr-20260714-editor-conflict-ui-nonblocking}
- {type: hasPart, target: adr-20260714-editor-reconnect-resync}
- {type: hasPart, target: adr-20260714-editor-aria-live-precedence}
- {type: hasPart, target: adr-20260714-editor-sensitive-file-no-filter}
- {type: hasPart, target: adr-20260714-editor-root-disappearance-degrades-save}
source_paths:
- src/server/web/workspace.go
- src/server/web/workspace_path.go
- src/client/web/src/components/workspace/FileViewer.tsx
- src/client/web/src/lib/workspaceVimKeymap.ts
- src/.golangci.yml
summary: 既存 workspace viewer 資産 (GuardWorkspacePath / resolveWorkspaceSession / DiffHeadVsWorktree
  / tool_log schema v2 + reader / ViewUpdate activity_events / ActivityRail / WorkspaceDrawer
  / FileViewer / workspaceVimKeymap) を w…
---

# Agent workspace mutation-capable editor — implementation plan

## Goal

agent-grid web UI の workspace read-only viewer (spec-20260714-agent-workspace-viewer) に mutation-capable な editor 経路を追加し、operator が vim mutation keys (i/a/o/dd/x/c/y/p/u/Ctrl-R/:w) で実際にファイルを編集・保存できるようにする。sensitive file 区別は read 側同様に持たず、path-traversal defense と workspace-root 境界は継続する。

## Approach

既存 workspace viewer 資産 (GuardWorkspacePath / resolveWorkspaceSession / DiffHeadVsWorktree / tool_log schema v2 + reader / ViewUpdate activity_events / ActivityRail / WorkspaceDrawer / FileViewer / workspaceVimKeymap) を write 側 SSOT として再利用し平行構造を作らない。新設は (1) 新 backend write handler (component-workspace-fs-writer) + forbidigo rule の per-file exception、(2) FileViewer のエディタを CodeMirror 6 + @codemirror/vim に差し替えて MUTATION_KEYS no-op を実編集へ格上げ、(3) 既存 tool_log schema へ additive optional actor field を追加した operator write audit、(4) mid_turn_touch + reconnect resync を再利用した write conflict detector、の 4 系統。既存 read handler (workspace.go) は変更せず read-only のまま維持。既存 TokenAuth 中間層により write endpoint は同一 Bearer 認証を継承する。

## UX Invariant Override Table

上流 `ux-20260713-agent-workspace-viewer` は本 feature scope に対して以下の 3 invariant を明示的に反転する。ux 文書自体は unchanged (追認)、反転の trace は本 plan の FR/contract/ADR に集約する。

| Upstream invariant (viewer) | Overridden in editor scope by | FR | Contract | ADR |
|---|---|---|---|---|
| `exp-vim-no-mutation` (viewer UAC-022 `dd`, UAC-023 `:w` は viewer/workspace を変えない) | mutation keys が buffer と workspace を実際に変える | FR-EDIT-001, FR-EDIT-002 | contract-vim-mutation-editing, contract-write-persistence-save | adr-20260714-editor-codemirror-vim-engine |
| `exp-read-only` (viewer は書き込み/保存 API を提供しない) | 新 write endpoint PUT /workspace/file を提供する | FR-EDIT-002 | contract-write-persistence-save | adr-20260714-editor-write-endpoint-shape |
| `cc-no-write` (fs アクセス経路自体が read-only、depguard で構造禁止) | workspace_write.go 単一ファイルだけを per-file exclusion で許容 | FR-EDIT-003 | contract-write-structural-boundary | adr-20260714-editor-write-depguard-rescope (supersedes adr-20260714-wsviewer-no-write-depguard) |

## Inherited Invariants (unchanged from viewer)

本 feature は以下の viewer 側 invariant を継承する (反転しない)。

| Invariant | Source (viewer) | Preserved by |
|---|---|---|
| workspace-relative path 表示 | ux exp-workspace-relative-paths | FR-EDIT-003 / contract-write-persistence-save |
| workspace-root boundary (path traversal defense) | ux exp-workspace-root-boundary + adr-wsviewer-path-guard-symlink | FR-EDIT-003 / contract-write-permission-parity |
| turn aggregation | ux exp-turn-aggregation | FR-EDIT-008 / contract-write-audit-trail / contract-audit-schema-migration |
| live-background (drawer 中も rail/terminal 更新継続) | ux exp-live-background | FR-EDIT-005 / contract-write-conflict-detection |
| git HEAD diff base | ux exp-git-head-diff-base | FR-EDIT-002 に含む (write 後の diff base 変更なし) |

## Implementation Sequence

### Milestone m1

- **Depends on**: nothing
- **Members**: unit:workspace-fs-writer-handler, unit:depguard-rescope-editor

{% milestone id="m1" %}
- unit:workspace-fs-writer-handler
- unit:depguard-rescope-editor
{% /milestone %}

#### Units

##### workspace-fs-writer-handler

- **Objective**: PUT /api/sessions/{id}/workspace/file handler with GuardWorkspacePath, resolveWorkspaceSession, If-Unmodified-Since precondition, tmp+rename atomic write, 1 MiB body cap with typed 413, 409 handle_stale on mtime-precondition failure.
- **Output format**: src/server/web/workspace_write.go plus workspace_write_test.go plus mux.go registration; body validated table-driven; auth inherited from TokenAuth.
- **Tool guidance**: Read src/server/web/workspace.go, workspace_path.go, mux.go, transcript.go for the existing shape; reuse GuardWorkspacePath and resolveWorkspaceSession verbatim.
- **Task boundaries**: Does NOT modify workspace.go read handler. Does NOT change the mux TokenAuth middleware. Does NOT emit tool-log records (that is operator-audit-emission).
- **Files touched**: src/server/web/workspace_write.go, src/server/web/workspace_write_test.go, src/server/web/mux.go, src/server/web/mux_scenario_test.go
- **Acceptance**:
  - TestWorkspaceWriteHandlerHappyPath, TestWorkspaceWriteHandlerPathGuard, TestWorkspaceWriteHandlerBodyCap, TestWorkspaceWriteHandlerPreconditionFailed, TestWorkspaceWriteHandlerAuth all pass
  - verify-write-persistence-save, verify-write-atomicity, verify-write-endpoint-authz, verify-write-resource-bound green
  - make lint green (no new forbidigo violations after the exclusion is added)
- **Contract refs**: contract-write-persistence-save, contract-write-atomicity, contract-write-permission-parity, contract-write-endpoint-authz, contract-large-write-resource-bound
- **max_diff_loc**: 300

##### depguard-rescope-editor

- **Objective**: Correct the stale forbidigo pkg pattern (agent-reactor → agent-grid) and add per-file exclusions.rules[].path exception for `server/web/workspace_write\.go$`; ship the synthetic-mutation regression that proves removing the exclusion makes lint fail.
- **Output format**: src/.golangci.yml (patched) plus a lint-regression test invocation captured in a script or CI job.
- **Tool guidance**: Read the existing exclusions.rules block for the shape of path-scoped exclusions; keep the read-handler restriction on workspace.go.
- **Task boundaries**: Does NOT touch other depguard rules. Does NOT add new mutation callsites — only the exclusion.
- **Files touched**: src/.golangci.yml
- **Acceptance**:
  - make lint green with the exclusion; without it (temporarily removed) make lint fails on workspace_write.go
  - verify-write-structural-boundary passes
- **Contract refs**: contract-write-structural-boundary
- **max_diff_loc**: 300

### Milestone m2

- **Depends on**: m1
- **Members**: unit:audit-schema-additive-actor, unit:operator-audit-emission, unit:activity-rail-operator-badge

{% milestone id="m2" %}
- unit:audit-schema-additive-actor
- unit:operator-audit-emission
- unit:activity-rail-operator-badge
{% /milestone %}

#### Units

##### audit-schema-additive-actor

- **Objective**: Add optional actor field (default=agent) to the schema_version=2 tool_log record; reader classifies actor=operator into a distinct kind; ViewUpdate payload carries the field verbatim.
- **Output format**: src/client/runtime/tool_log_schema.go + tool_log_reader.go additions; server viewupdate.go pass-through; tests.
- **Tool guidance**: Read runtime/tool_log*.go, server/web/viewupdate.go; do NOT bump schema_version — keep 2 to preserve shipped-reader compatibility.
- **Task boundaries**: Does NOT emit records (that is a writer concern); does NOT render UI (that is activity-rail-operator-badge).
- **Files touched**: src/client/runtime/tool_log_schema.go, src/client/runtime/tool_log_reader.go, src/client/runtime/tool_log_reader_test.go, src/server/web/viewupdate.go, src/server/web/viewupdate_test.go
- **Acceptance**:
  - Legacy v2 fixture (no actor) round-trips byte-identically
  - New v2 fixture with actor=operator classifies as operator-kind row
  - verify-audit-schema-migration green
- **Contract refs**: contract-audit-schema-migration
- **max_diff_loc**: 300

##### operator-audit-emission

- **Objective**: workspace_write.go writes an actor=operator tool_log record on save success and typed audit_emit_failed HTTP 5xx if emission fails (no silent under-report).
- **Output format**: Addition to workspace_write.go plus tool_log writer helper reuse; failure-path test.
- **Tool guidance**: Reuse the existing tool_log writer facility from client/runtime; do NOT create a parallel audit log.
- **Task boundaries**: Does NOT change the schema (that is audit-schema-additive-actor); does NOT render (that is activity-rail-operator-badge).
- **Files touched**: src/server/web/workspace_write.go, src/client/runtime/tool_log.go, src/server/web/workspace_write_test.go
- **Acceptance**:
  - On successful save exactly one operator record appears in the tool-log tail
  - On simulated emission failure the client receives typed 5xx audit_emit_failed and the dirty buffer is retained
  - verify-write-audit-trail green
- **Contract refs**: contract-write-audit-trail
- **max_diff_loc**: 300

##### activity-rail-operator-badge

- **Objective**: ActivityRail renders a distinct operator-kind badge and aria-label; distinct from agent rows on both DOM and aria surfaces.
- **Output format**: TypeScript changes in ActivityRail.tsx + tests.
- **Tool guidance**: Reuse the existing badge component from viewer rail; add an operator variant.
- **Task boundaries**: Does NOT touch backend classification; does NOT touch the tool_log schema.
- **Files touched**: src/client/web/src/components/workspace/ActivityRail.tsx, src/client/web/src/components/workspace/ActivityRail.test.tsx
- **Acceptance**:
  - Operator row is queryable by aria-label distinct from agent
  - verify-write-audit-trail (client-side portion) green
- **Contract refs**: contract-write-audit-trail
- **max_diff_loc**: 300

### Milestone m3

- **Depends on**: m2
- **Members**: unit:codemirror-editor-swap, unit:buffer-save-endpoint-wiring, unit:dirty-and-conflict-drawer-chrome, unit:workspace-activity-store-conflict-selectors

{% milestone id="m3" %}
- unit:codemirror-editor-swap
- unit:buffer-save-endpoint-wiring
- unit:dirty-and-conflict-drawer-chrome
- unit:workspace-activity-store-conflict-selectors
{% /milestone %}

#### Units

##### codemirror-editor-swap

- **Objective**: Replace FileViewer source pane with a CodeMirror 6 editor + @codemirror/vim; explicitly set EditorState.lineSeparator so line endings are preserved byte-transparent; wire the existing motion/search bindings via @codemirror/vim.
- **Output format**: TypeScript changes to FileViewer.tsx; new dependency in package.json (CodeMirror 6 + @codemirror/vim + @codemirror/view).
- **Tool guidance**: Follow CodeMirror 6 documentation for read-only vs read-write mode toggling; consult context7 for @codemirror/vim API if needed.
- **Task boundaries**: Does NOT wire the write endpoint (that is buffer-save-endpoint-wiring); does NOT change diff/tree/renderers.
- **Files touched**: src/client/web/src/components/workspace/FileViewer.tsx, src/client/web/src/components/workspace/FileViewer.test.tsx, src/client/web/package.json
- **Acceptance**:
  - Existing FR-109 motion/search assertions from viewer stay green (verify-vim-motion-unit, verify-vim-mutation-integration flipped semantics)
  - verify-vim-mutation-editing green
  - verify-editor-large-file-editing-performance green
  - verify-encoding-newline-fidelity green
- **Contract refs**: contract-vim-mutation-editing, contract-editor-large-file-editing-performance, contract-encoding-newline-fidelity, contract-vim-undo-locality
- **max_diff_loc**: 300

##### buffer-save-endpoint-wiring

- **Objective**: api/workspace.ts adds save(sessionId, handle, path, bytes, ifUnmodifiedSince) using PUT with If-Unmodified-Since; FileViewer :w invokes it; typed error mapping to conflict / stale / auth / oversize surfaces.
- **Output format**: TypeScript source additions in src/client/web/src/api/workspace.ts, hook into FileViewer :w handler, tests.
- **Tool guidance**: Match the existing getFile() shape and error mapping conventions in api/workspace.ts.
- **Task boundaries**: Does NOT implement the conflict banner UI (that is dirty-and-conflict-drawer-chrome).
- **Files touched**: src/client/web/src/api/workspace.ts, src/client/web/src/components/workspace/FileViewer.tsx, src/client/web/src/api/__tests__/workspace.test.ts
- **Acceptance**:
  - :w calls save() exactly once; each typed error class is surfaced as a distinct callback
  - verify-write-persistence-save (client half) green
- **Contract refs**: contract-write-persistence-save
- **max_diff_loc**: 300

##### dirty-and-conflict-drawer-chrome

- **Objective**: WorkspaceDrawer adds dirty indicator, unsaved-close warning, conflict banner + keep-mine/take-theirs/merge, root_disappeared banner + clipboard export; aria-live precedence rule (conflict > stale > close-warning > dirty).
- **Output format**: TypeScript changes to WorkspaceDrawer.tsx + tests.
- **Tool guidance**: Reuse the single aria-live slot from adr-20260624-0057; add a precedence tie-breaker.
- **Task boundaries**: Does NOT implement the detector logic (that is workspace-activity-store-conflict-selectors).
- **Files touched**: src/client/web/src/components/workspace/WorkspaceDrawer.tsx, src/client/web/src/components/workspace/WorkspaceDrawer.test.tsx
- **Acceptance**:
  - verify-dirty-state-visibility green
  - verify-write-conflict-detection green
  - verify-workspace-root-disappearance green
  - verify-drawer-terminal-rect-nonregression green (MainTabs terminal-slot rect unchanged with new UI mounted)
- **Contract refs**: contract-dirty-state-visibility, contract-write-conflict-detection, contract-workspace-root-disappearance
- **max_diff_loc**: 300

##### workspace-activity-store-conflict-selectors

- **Objective**: workspaceActivity store adds conflict outcome partition (no_conflict / background_touch_clean_buffer / background_touch_dirty_buffer / reconnect_mtime_differs); reconnect handler triggers conflict-check re-fetch for every dirty buffer.
- **Output format**: TypeScript additions in workspaceActivity.ts + tests.
- **Tool guidance**: Reuse the existing mid_turn_touch selector; add a dirty-buffer registry indexed by (session_id, path).
- **Task boundaries**: Does NOT render UI (that is dirty-and-conflict-drawer-chrome).
- **Files touched**: src/client/web/src/store/workspaceActivity.ts, src/client/web/src/store/__tests__/workspaceActivity.test.ts
- **Acceptance**:
  - All 4 outcome-partition classes covered by unit tests
  - verify-write-conflict-detection (store portion) green
- **Contract refs**: contract-write-conflict-detection
- **max_diff_loc**: 300

## Contracts

### contract-vim-mutation-editing

- **Dimension**: control_flow
- **Owner**: component-workspace-vim-keymap
- **Subject**: vim mutation keys (i/a/o/I/A/O insert entry, dd/x/c/y/p operator + register commands) applied through CodeMirror 6 + @codemirror/vim actually change the in-memory buffer of the FileViewer editor, replacing the previous no-op MUTATION_KEYS branch.
- **Requirements**: FR-101, FR-109
- **Units**: codemirror-editor-swap
- **ADRs**: adr-20260714-editor-codemirror-vim-engine
- **Invariants**:
  - Every non-motion/non-search keystroke is either consumed by the CodeMirror doc (real edit) or stopped at capture phase; xterm.js receives zero of them.
  - Motion/search bindings from spec-20260714-agent-workspace-viewer (j/k/gg/G, /, n, N) remain observable-equivalent to the read-only viewer.

### contract-write-persistence-save

- **Dimension**: integration_contract
- **Owner**: component-workspace-fs-writer
- **Subject**: The `:w` command issues PUT /api/sessions/{id}/workspace/file?path=<rel> with raw body bytes and If-Unmodified-Since header (open-time mtime/ETag) to the workspace-fs-writer handler; the handler returns 200 + updated_mtime JSON on success, 409 handle_stale on frame push mismatch, 412 precondition_failed on mtime mismatch (drives conflict detector), and 4xx typed errors otherwise.
- **Requirements**: FR-102
- **Units**: workspace-fs-writer-handler, buffer-save-endpoint-wiring
- **ADRs**: adr-20260714-editor-write-endpoint-shape, adr-20260714-editor-concurrency-optimistic-lock
- **Invariants**:
  - Every save attempt returns either a 2xx success (with matching persisted bytes on subsequent GET) or a typed 4xx/5xx error — never a silent success or half-written file.
  - The route only accepts PUT for writes; GET keeps its read semantics; other verbs (POST/PATCH/DELETE) return 405.

### contract-write-atomicity

- **Dimension**: failure_recovery
- **Owner**: component-workspace-fs-writer
- **Subject**: Save writes to a same-directory temp file then os.Rename onto the target path; a mid-write fault (disk full, permission error, directory removal) leaves the original file bytes intact and produces a visible typed error, never a truncated file or silent success.
- **Requirements**: FR-106
- **Units**: workspace-fs-writer-handler
- **ADRs**: adr-20260714-editor-atomicity-tmp-rename
- **Invariants**:
  - Post-save file bytes on disk equal either the pre-save bytes or the request body bytes for every fixture in the failure matrix.
  - No response reports 200 unless the rename returned success AND the resulting inode carries the request-body content length.

### contract-write-conflict-detection

- **Dimension**: concurrency
- **Owner**: component-write-conflict-detector
- **Subject**: For every dirty buffer, the system observes the pair (dirty flag, background-write signal / reconnect resync); if the pair indicates a possible overwrite it drives a non-blocking conflict banner with keep-mine/take-theirs/merge; the outcome partition covers no_conflict / background_touch_clean_buffer / background_touch_dirty_buffer / reconnect_mtime_differs.
- **Requirements**: FR-105, FR-112
- **Units**: dirty-and-conflict-drawer-chrome, workspace-activity-store-conflict-selectors
- **ADRs**: adr-20260714-editor-concurrency-optimistic-lock, adr-20260714-editor-reconnect-resync, adr-20260714-editor-conflict-ui-nonblocking, adr-20260714-editor-aria-live-precedence
- **Invariants**:
  - No :w after a background write on the same dirty path completes without the operator explicitly choosing keep-mine/take-theirs/merge.
  - WS reconnect never leaves a silent-overwrite window: every dirty path is mtime-checked before the next save is enabled.

### contract-write-audit-trail

- **Dimension**: user_observability
- **Owner**: component-operator-write-audit
- **Subject**: Every successful operator save produces exactly one operator-kind row on the Activity Rail with the correct workspace-relative path; audit-emission failure yields a typed 5xx to the client (dirty buffer retained), never a silent under-report.
- **Requirements**: FR-108
- **Units**: operator-audit-emission, activity-rail-operator-badge
- **ADRs**: adr-20260714-editor-audit-schema-additive-actor, adr-20260714-editor-audit-emission-failfast
- **Invariants**:
  - Every 2xx save response is accompanied by exactly one operator-kind row eventually visible on the ActivityRail.
  - No 2xx save response ever produces zero rows — emission failure escalates to 5xx audit_emit_failed.

### contract-dirty-state-visibility

- **Dimension**: user_observability
- **Owner**: component-workspace-drawer
- **Subject**: Dirty indicator visible on header/tab within one render frame of a mutating keystroke; drawer close attempt with unsaved changes presents an explicit warning; announcements share the single aria-live slot with precedence conflict > stale > close-warning > dirty.
- **Requirements**: FR-107
- **Units**: dirty-and-conflict-drawer-chrome
- **ADRs**: adr-20260714-editor-aria-live-precedence
- **Invariants**:
  - Dirty indicator visibility tracks the dirty flag within one render frame.
  - Drawer close with unsaved changes always produces an explicit warning; there is no path that discards the buffer without operator confirmation.
  - MainTabs terminal-slot rendered rect (adr-20260714-wsviewer-appshell-composition) is byte-identical (±1 CSS px) with or without dirty indicator / conflict banner / close-warning mounted.

### contract-vim-undo-locality

- **Dimension**: state_lifecycle
- **Owner**: component-workspace-vim-keymap
- **Subject**: Undo/redo stack is viewer-session-local (per buffer instance): only operator-authored edits push, background writes never do, and stack is strictly cleared on buffer close AND on any reload / conflict-resolution acceptance (no silent replay on new base). Unnamed register is drawer-session-local (shared across buffers in the same drawer instance, cleared on drawer close).
- **Requirements**: FR-104
- **Units**: codemirror-editor-swap
- **ADRs**: adr-20260714-editor-undo-scope-viewer-session-local, adr-20260714-editor-codemirror-vim-engine
- **Invariants**:
  - Background agent writes never appear as steps on the operator's undo stack.
  - Undo stack is strictly cleared on reload / conflict-resolution accept; no replay against a new base.

### contract-write-permission-parity

- **Dimension**: security_boundary
- **Owner**: component-workspace-fs-writer
- **Subject**: The write path applies no sensitive-file filter — `.env`, `.git/config`, `*.key` and any conventional-sensitive name write through the identical flow as ordinary files, mirroring the read side DP-SENSITIVE-FILE-EXPOSURE=OPT-NO-FILTER decision; GuardWorkspacePath containment is applied unconditionally and cannot be conflated with the sensitivity policy.
- **Requirements**: FR-103
- **Units**: workspace-fs-writer-handler
- **ADRs**: adr-20260714-editor-sensitive-file-no-filter
- **Invariants**:
  - No content- or filename-based masking or reveal step exists on the write path.
  - GuardWorkspacePath is applied to every write request; out-of-root paths are 404 regardless of filename.

### contract-write-structural-boundary

- **Dimension**: integration_contract
- **Owner**: component-workspace-fs-writer
- **Subject**: The only code path that reaches the write endpoint is FileViewer + vim keymap → api/workspace.ts save(); WorkspaceTree and all other UI paths remain read-only. Structural enforcement is dual: (a) per-file forbidigo exclusion allowlists ONLY workspace_write.go for os.WriteFile/Rename inside src/server/web, (b) frontend caller-allowlist test asserts a single call site.
- **Requirements**: FR-103
- **Units**: depguard-rescope-editor
- **ADRs**: adr-20260714-editor-write-depguard-rescope
- **Invariants**:
  - The per-file forbidigo exclusion covers exactly workspace_write.go — removing it makes lint fail there.
  - Exactly one frontend call site of api/workspace.ts save() exists (FileViewer :w handler).

### contract-editor-large-file-editing-performance

- **Dimension**: performance_budget
- **Owner**: component-file-viewer
- **Subject**: For files at or above the 1 MiB threshold, per-keystroke render latency stays p95 ≤ 33 ms and p99 ≤ 50 ms under a 200-keystroke burst; :w serialization completes without blocking the keydown handler.
- **Requirements**: NFR-102
- **Units**: codemirror-editor-swap
- **ADRs**: adr-20260714-editor-codemirror-vim-engine
- **Invariants**:
  - Editing does not degrade with the same file-size axis that the read viewer already bounded (contract-large-file-threshold in viewer).

### contract-encoding-newline-fidelity

- **Dimension**: data_model
- **Owner**: component-workspace-fs-writer
- **Subject**: The write path preserves the byte encoding and line-ending convention of the source file in the regions the operator did not edit (byte-transparent round-trip). CodeMirror 6 EditorState.lineSeparator is set explicitly at open so that no silent CRLF↔LF normalization occurs.
- **Requirements**: FR-110
- **Units**: codemirror-editor-swap
- **ADRs**: adr-20260714-editor-encoding-byte-transparent, adr-20260714-editor-codemirror-vim-engine
- **Invariants**:
  - CRLF stays CRLF, LF stays LF, and mixed / non-UTF-8 sequences are round-tripped byte-identical in untouched regions.
  - CodeMirror EditorState.lineSeparator is set explicitly (never left to the engine's default).

### contract-audit-schema-migration

- **Dimension**: migration_compatibility
- **Owner**: component-operator-write-audit
- **Subject**: schema_version stays at 2; add an optional `actor` field (default=agent) to the record; shipped v2 readers ignore unknown fields and continue emitting the same rows unchanged; the new reader classifies actor=operator into operator-kind rows.
- **Requirements**: FR-108
- **Units**: audit-schema-additive-actor
- **ADRs**: adr-20260714-editor-audit-schema-additive-actor
- **Invariants**:
  - A pre-migration v2 fixture (no actor field) produces byte-identical row output before and after the migration.
  - A migrated fixture with actor=operator produces distinct operator rows in the new reader and is ignored (treated as agent) by the old reader — no phantom rows anywhere.

### contract-large-write-resource-bound

- **Dimension**: resource_management
- **Owner**: component-workspace-fs-writer
- **Subject**: Server enforces a 1 MiB per-write body cap matching the read-side virtualization threshold; over-cap requests are rejected with typed HTTP 413 based on Content-Length before buffering (streaming cap on unknown length); under a concurrent load of at most 16 in-flight writes, server memory footprint added by writes stays below 32 MiB.
- **Requirements**: NFR-103
- **Units**: workspace-fs-writer-handler
- **Invariants**:
  - Server never fully buffers a body > 1 MiB in memory before rejecting.
  - 16 concurrent 1 MiB uploads do not add more than 32 MiB to server RSS.

### contract-workspace-root-disappearance

- **Dimension**: failure_recovery
- **Owner**: component-workspace-drawer
- **Subject**: If the workspace root is deleted or renamed while a dirty buffer is open, the drawer transitions to a typed root_disappeared state, keeps the dirty buffer in memory, exposes a clipboard-export action, and never silently discards the operator's work.
- **Requirements**: FR-113
- **Units**: dirty-and-conflict-drawer-chrome
- **ADRs**: adr-20260714-editor-root-disappearance-degrades-save
- **Invariants**:
  - No path exists where the drawer discards the dirty buffer without operator action.
  - root_disappeared always exposes the clipboard-export action for the dirty contents.

### contract-write-endpoint-authz

- **Dimension**: security_boundary
- **Owner**: component-workspace-fs-writer
- **Subject**: The write endpoint is registered inside the same TokenAuth-wrapped apiHandler as the read handlers; requests without a valid Bearer Authorization header are rejected with HTTP 401 before any fs access; there is no cookie-based auth or CSRF-style flow (REST + bearer, matching the existing gateway invariant).
- **Requirements**: FR-111
- **Units**: workspace-fs-writer-handler
- **Invariants**:
  - No PUT to /workspace/file is dispatched to the handler without the same TokenAuth check that guards GET.
  - There is no query-parameter alternative for auth (never a URL token).

## ADR Trace

- **adr-20260714-editor-write-depguard-rescope** — Rescope the workspace forbidigo rule for a per-file write-handler exclusion (supersedes wsviewer-no-write-depguard) — contracts: contract-write-structural-boundary
- **adr-20260714-editor-write-endpoint-shape** — Editor save wire contract is PUT /workspace/file with raw body and If-Unmodified-Since — contracts: contract-write-persistence-save
- **adr-20260714-editor-concurrency-optimistic-lock** — Concurrency policy for operator/agent write race is optimistic lock via If-Unmodified-Since — contracts: contract-write-persistence-save, contract-write-conflict-detection
- **adr-20260714-editor-codemirror-vim-engine** — Mutation-capable editor is CodeMirror 6 + @codemirror/vim (primary); hand-roll dropped — contracts: contract-vim-mutation-editing, contract-vim-undo-locality, contract-editor-large-file-editing-performance, contract-encoding-newline-fidelity
- **adr-20260714-editor-encoding-byte-transparent** — Editor save preserves encoding and line endings byte-transparently — contracts: contract-encoding-newline-fidelity
- **adr-20260714-editor-undo-scope-viewer-session-local** — Undo/redo scope is viewer-session-local (strict clear on reload/conflict accept) — contracts: contract-vim-undo-locality
- **adr-20260714-editor-atomicity-tmp-rename** — Save atomicity uses same-directory tmp file + os.Rename with inode verification — contracts: contract-write-atomicity
- **adr-20260714-editor-audit-schema-additive-actor** — Operator audit uses additive optional actor field on schema_version=2 (no version bump) — contracts: contract-audit-schema-migration, contract-write-audit-trail
- **adr-20260714-editor-audit-emission-failfast** — Audit emission failure escalates to typed 5xx (no silent under-report) — contracts: contract-write-audit-trail
- **adr-20260714-editor-conflict-ui-nonblocking** — Conflict UI is a non-blocking banner with keep-mine / take-theirs / merge — contracts: contract-write-conflict-detection
- **adr-20260714-editor-reconnect-resync** — WS reconnect triggers a forced conflict-check re-fetch for every dirty buffer — contracts: contract-write-conflict-detection
- **adr-20260714-editor-aria-live-precedence** — Single aria-live slot precedence is conflict > stale > close-warning > dirty — contracts: contract-dirty-state-visibility, contract-write-conflict-detection
- **adr-20260714-editor-sensitive-file-no-filter** — Write path applies no sensitive-file filter (parity with read side) — contracts: contract-write-permission-parity
- **adr-20260714-editor-root-disappearance-degrades-save** — Root disappearance transitions to read-only + clipboard export (save requirement degrades explicitly) — contracts: contract-workspace-root-disappearance

## Verification

| Tier | Contract | Test | Command | Criterion |
|---|---|---|---|---|
| T1 | contract-vim-mutation-editing | verify-vim-mutation-editing | `cd src/client/web && pnpm vitest run components/workspace/FileViewer.test.tsx` | For each of {i+text+Esc, dd, ciw, 3dd, yy/p} the resulting CodeMirror doc equals the expected string; xterm mock keydown count is 0. |
| T2 | contract-write-persistence-save | verify-write-persistence-save | `cd src && go test ./server/web -run TestWorkspaceWriteHandler` | 200 case → GET returns request bytes exactly; 412 case → GET returns pre-save bytes; 409 case → GET returns pre-save bytes. |
| T2 | contract-write-atomicity | verify-write-atomicity | `cd src && go test ./server/web -run TestWorkspaceWriteAtomicity` | For every failure fixture the disk shows pre-save bytes and the response is typed 5xx; success fixture shows post-save bytes and 200. |
| T1 | contract-write-conflict-detection | verify-write-conflict-detection | `cd src/client/web && pnpm vitest run store/workspaceActivity.test.ts components/workspace/WorkspaceDrawer.test.tsx` | Each outcome partition class produces the expected banner state; keep-mine/take-theirs/merge each finalize disk state matching operator choice. |
| T1 | contract-write-audit-trail | verify-write-audit-trail | `cd src && go test ./server/web -run TestWorkspaceWriteAudit && cd client/web && pnpm vitest run components/workspace/ActivityRail.test.tsx` | One save → one operator record in the tail; ActivityRail renders one operator row; injected emission failure → 5xx from the handler and no row on the rail. |
| T1 | contract-dirty-state-visibility | verify-dirty-state-visibility | `cd src/client/web && pnpm vitest run components/workspace/WorkspaceDrawer.test.tsx` | Dirty flag flip → indicator appears in DOM within one frame; close attempt → warning modal shown; concurrent conflict+stale+close-warning+dirty produce exactly one aria-live announcement per transition with declared precedence. |
| T0 | contract-vim-undo-locality | verify-vim-undo-locality | `cd src/client/web && pnpm vitest run lib/workspaceVimKeymap.test.ts` | After edits A,B,C then two u the buffer equals state after A; after take-theirs accept the stack is empty (u is no-op). |
| T2 | contract-write-permission-parity | verify-write-permission-parity | `cd src && go test ./server/web -run TestWorkspaceWritePermissionParity` | In-root writes succeed with 200; out-of-root are 404; no filename-based extra step is invoked. |
| T2 | contract-write-structural-boundary | verify-write-structural-boundary | `make lint && cd src/client/web && pnpm vitest run components/workspace/save-allowlist.test.ts` | Removing the exclusions.rules[] entry for workspace_write.go OR injecting os.WriteFile elsewhere in workspace*.go makes `make lint` fail; frontend test finds exactly one save() call site. |
| T1 | contract-editor-large-file-editing-performance | verify-editor-large-file-editing-performance | `cd src/client/web && pnpm playwright test workspace/large-file-edit.spec.ts` | Ordered latency samples yield p95 ≤ 33 ms, p99 ≤ 50 ms. |
| T1 | contract-encoding-newline-fidelity | verify-encoding-newline-fidelity | `cd src/client/web && pnpm vitest run components/workspace/encoding-fidelity.test.tsx` | For every fixture, untouched-region byte diff is empty. |
| T2 | contract-audit-schema-migration | verify-audit-schema-migration | `cd src && go test ./client/runtime -run TestToolLogReaderActorClassification` | Mixed fixture produces two distinct kind rows correctly attributed; legacy fixture rounds-trip byte-identically. |
| T2 | contract-audit-schema-migration | verify-audit-schema-migration-baseline-v2 | `cd src && go test ./client/runtime -run TestToolLogReaderClassification` | Row set equals the pre-migration golden. |
| T2 | contract-large-write-resource-bound | verify-large-write-resource-bound | `cd src && go test ./server/web -run TestWorkspaceWriteResourceBound` | Over-cap PUT returns 413 before body finishes; peak RSS delta over 16 concurrent 1 MiB uploads is ≤ 32 MiB. |
| T1 | contract-workspace-root-disappearance | verify-workspace-root-disappearance | `cd src/client/web && pnpm vitest run components/workspace/root-disappearance.test.tsx && cd ../../src && go test ./server/web -run TestWorkspaceRootDisappearance` | Each outcome triggers the declared observable; dirty buffer never discarded silently; clipboard-export action present in root_disappeared. |
| T2 | contract-write-endpoint-authz | verify-write-endpoint-authz | `cd src && go test ./server/web -run TestWorkspaceWriteAuth` | Without header → 401; with valid header → handler runs and further path/precondition logic applies. |

## Open Questions

- fsync による crash-across-durability は future ADR: 現段階では tmp+rename の atomic 置換のみで invariant を閉じる。
- cross-tab / persistent な undo stack は要求が無いため viewer-session-local に閉じる (adr-20260714-editor-undo-scope-viewer-session-local)。
- shipped v2 reader での operator record 誤帰属 (actor field 未対応時に agent と誤分類する UX 劣化) は additive migration の trade-off として accept、rollout 順序 (writer → reader → UI) で影響を最小化する。

## Implementation Decisions Remaining

本 plan は全 design_choice を ADR/contract で閉じており、`implementation_decisions_remaining` は空 (schema 上の 0 件)。個別 unit 内の `implementation_decisions_remaining` も現時点で 0 件。

