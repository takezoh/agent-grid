---
id: plan-20260714-workspace-session-switch
kind: plan
title: Workspace session switch reinitialization plan
status: done
created: '2026-07-14'
goal: セッション切替を dirty-aware な Workspace 終了 transaction として実装し、新 active session の persistent
  Files tree と editor empty state へ安全に再初期化する。
scope_in:
- browser-local active session selection と単一 Workspace lifecycle の協調
- dirty pending confirmation と session disappearance recovery
- session/epoch による旧非同期応答の commit 拒否
- WorkspaceRootHandle の URL session/current root binding
- clean、dirty、late response、handle misuse の回帰テスト
scope_out:
- per-session Workspace state map と切替後の状態復元
- Terminal/Workspace mode round-trip semantics の変更
- activity history の破棄
- opaque token、server registry、multi-root、新規依存
milestones:
- id: m1
  title: Guarded session-switch lifecycle and App confirmation
  status: todo
- id: m2
  title: Session and epoch guarded asynchronous commits
  status: todo
- id: m3
  title: Session-bound WorkspaceRootHandle validation
  status: todo
contracts:
- activeSessionID の browser-local owner は useDaemonStore の単一 selection policy
- Workspace state は一つだけで、session switch は discard-before-active-commit
- mode visibility switch は FR-031/UAC-016 に従い両 mode state を保持し、active-session selection
  は別の明示的 context switch として旧 Workspace session を終了する
- visible な session switch 後は Workspace mode を維持し、persistent right-side Files tree
  は新 root 直下一覧を未展開で表示し、editor は open target/file/diff のない empty state を表示する（focus
  は規定しない）
- rowsBySession と lastSequenceBySession は session switch で保持
- 非同期 commit は発行元 session と monotonic Workspace epoch の一致を必要とする
- handle session/root mismatch は invalid_handle、generation drift は handle_stale
adrs:
- adr-20260714-workspace-session-switch-lifecycle
- adr-20260714-workspace-request-epoch
- adr-20260714-workspace-handle-session-binding
- adr-20260714-workspace-switch-confirmation-pattern
decision_dispositions:
- decision_input_ref: DP-d1
  disposition: adopted
  rationale: daemon store の selection policy を active owner として維持し、Workspace lifecycle
    の allow/pending を同期的に適用する。
  adr_refs:
  - adr-20260714-workspace-session-switch-lifecycle
- decision_input_ref: DP-d2
  disposition: adopted
  rationale: epoch を commit の正本、effect cleanup を補助とする。
  adr_refs:
  - adr-20260714-workspace-request-epoch
- decision_input_ref: DP-d3
  disposition: adopted
  rationale: accepted tuple contract を最小拡張し、server 境界で全値を再照合する。
  adr_refs:
  - adr-20260714-workspace-handle-session-binding
- decision_input_ref: DP-d4
  disposition: adopted
  rationale: App overlay と ConfirmDialog を再利用し、Workspace 非表示でも確認可能にする。
  adr_refs:
  - adr-20260714-workspace-switch-confirmation-pattern
- decision_input_ref: DP-d5
  disposition: adopted
  rationale: session/root mismatch は invalid_handle、通常の generation drift は handle_stale
    に分ける。
  adr_refs:
  - adr-20260714-workspace-handle-session-binding
tags:
- workspace
- session-switch
- bugfix
owners: []
relations:
- {type: implements, target: spec-20260714-workspace-session-switch}
- {type: hasPart, target: adr-20260714-workspace-session-switch-lifecycle}
- {type: hasPart, target: adr-20260714-workspace-request-epoch}
- {type: hasPart, target: adr-20260714-workspace-handle-session-binding}
- {type: hasPart, target: adr-20260714-workspace-switch-confirmation-pattern}
source_paths: []
methodology: sdd
summary: 既存 store、ConfirmDialog、WorkspaceApi、Go handler seam を再利用し、lifecycle、epoch
  guard、handle binding を依存順に実装する。
updated: '2026-07-14'
---

## Goal

active session の選択権を browser-local daemon store に保ったまま、単一 Workspace state の終了可否と commit 順序を一つの selection policy で協調する。web-ui-refresh FR-031/UAC-016 の mode visibility switch は terminal と Workspace state を保持する一方、active-session selection は別の明示的 context switch として old Workspace session を終了する。dirty 時は App overlay で選択を保留し、clean/Confirm 時だけ旧状態を破棄して新 session を commit する。

## Implementation Sequence

### m1

{% milestone id="m1" %}
`component:workspace lifecycle`、`component:daemon selection policy`、`component:App confirmation`、`req:FR-001..FR-008`、`req:FR-012..FR-014`、`adr:adr-20260714-workspace-session-switch-lifecycle`、`adr:adr-20260714-workspace-switch-confirmation-pattern`。

Unit `session-switch-lifecycle`: `workspaceActivity.ts` に pending target、typed failure、epoch、dirty-aware prepare/cancel/confirm/reset を追加し、`daemon.ts` の `selectSession` を全 caller が使う policy にする。activity history は reset 対象外に固定する。対象は store と store tests。完了条件は clean、dirty Cancel/Confirm、pending replacement、target/active disappearance、mode-only preservation の T0/T1 tests。

Unit `app-confirmation-and-selection-routes`: App-level `ConfirmDialog` と visible alert を公開 action に接続し、SessionList、palette、post-create、post-termination が同じ policy を通ることを検証する。対象は `App.tsx`、selection caller tests、App tests、必要最小限の Playwright smoke。Workspace drawer 内 close warning の一般化は行わない。
{% /milestone %}

### m2

{% milestone id="m2" %}
`component:Workspace request commit guards`、`req:FR-009`、`adr:adr-20260714-workspace-request-epoch`。m1 の store-owned epoch に依存する。

Unit `workspace-request-commit-guards`: root handle、file、diff、tree、reconnect resync の開始時に `{sessionId, epoch}` を捕捉し、すべての success/error/finally commit 前に current identity と比較する。effect cleanup/AbortController は資源解放の補助に限定する。対象は `WorkspaceDrawer.tsx`、`WorkspaceTree.tsx` と deterministic delayed-Promise tests。完了条件は旧 resolve/reject が loading/error を含む新 state を一切変えないこと。
{% /milestone %}

### m3

{% milestone id="m3" %}
`component:WorkspaceApi handle contract`、`component:Go workspace handle validator`、`req:FR-010..FR-011`、`adr:adr-20260714-workspace-handle-session-binding`。

Unit `workspace-handle-binding`: client pin に `sessionId` を保持して全 request に渡し、server は URL session を現在解決した後、handle session/root/generation を filesystem/git より先に検証する。session/root mismatch は `invalid_handle`、generation drift は既存 `handle_stale` とする。対象は `api/workspace.ts`、`workspaceActivity.ts`、`server/web/workspace.go` と TypeScript/Go contract tests。opaque token と registry は導入しない。
{% /milestone %}

## Targets

| Target | Responsibility / seam |
|---|---|
| `src/client/web/src/store/workspaceActivity.ts` | 単一 Workspace lifecycle、pending state、epoch、typed recovery。純粋 transition helper を store I/O から分離する |
| `src/client/web/src/store/daemon.ts` | browser-local activeSessionID の単一 selection policy。Workspace store の公開 lifecycle action が store 間 seam |
| `src/client/web/src/App.tsx` | 既存 `ConfirmDialog` に pending state と actions を注入する App overlay seam |
| `src/client/web/src/components/SessionList.tsx`, `src/client/web/src/components/palette/hooks/useToolCtx.ts`, `src/client/web/src/lib/tools.ts`, `src/client/web/src/hooks/useTerminateSession.ts` | direct selection を増やさず既存 daemon action を再利用する caller 群 |
| `src/client/web/src/components/workspace/WorkspaceDrawer.tsx`, `WorkspaceTree.tsx` | `makeWorkspaceApi` の既存 fetch seam と session/epoch commit guard |
| `src/client/web/src/api/workspace.ts` | `makeWorkspaceApi(fetchFn)` seam で session-bound handle を wire へ投影 |
| `src/server/web/workspace.go` | `resolveWorkspaceSession` 後・`GuardWorkspacePath`/filesystem/git 前の一意な handle validator 境界 |
| tests | Zustand public actions、DOM/aria、injected fetch、Go httptest handler、fake backend をそれぞれ既存 seam として再利用 |

外部依存は増やさない。ブラウザ、HTTP/daemon、filesystem/git が不純殻であり、selection transition と identity comparison を純粋核として値注入可能にする。

## Verification

| Profile | Tier | Command | Criterion / milestone DoD |
|---|---|---|---|
| lifecycle pure/store | T0 pure / T1 wired | `cd src/client/web && npm run test:unit` | m1: 全 state transition、caller route、DOM alert が FR-001..FR-008/012..014 を満たす |
| delayed workspace requests | T1 wired | `cd src/client/web && npm run test:unit` | m2: old resolve/reject が root/file/diff/tree/loading/error/store を変更しない |
| handle contract | T2 contract | `cd src && go test ./server/web` | m3: mismatch は typed 4xx、sentinel filesystem/git access は 0、valid tuple は既存挙動を維持 |
| browser overlay smoke | T3 fidelity | `cd src/client/web && PLAYWRIGHT_BROWSERS_PATH=/tmp/ms-playwright npm run test:e2e` | Workspace 非表示でも confirmation が keyboard 操作可能で Cancel/Confirm が観測できる |
| structural and compatibility gate | T0/T1/T2 | `GOCACHE=/tmp/gocache-agent-grid GOLANGCI_LINT_CACHE=/tmp/golangci-lint-cache make lint && cd src && GOCACHE=/tmp/gocache-agent-grid go test ./...` | active owner、層依存、既存 accepted contract tests が維持される |

構造規則の fitness function: `activeSessionID` の direct write と selection caller を `rg` で列挙し、daemon policy 外の新規 decision が 0 件であることを review する。client/orchestrator/platform の依存方向は既存 lint、wire/session contract は Go/TypeScript tests が検査する。


{% transition from="draft" to="active" date="2026-07-14" %}
承認済み設計の実装を開始したため
{% /transition %}


{% transition from="active" to="done" date="2026-07-14" %}
All implementation milestones completed and verified
{% /transition %}
