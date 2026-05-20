# Issues

[plans/](../plans/) で定めた計画を実行可能な単位に分解した issue 群。

## 表記規約

各 issue ファイルは以下のセクションを持つ:

```markdown
# <ID>: <タイトル>

- **Phase**: P0a / P0b / ... ([plans/04-phases.md](../plans/04-phases.md))
- **Status**: Open / In Progress / Blocked / Done
- **Depends on**: 他 issue ID または PR
- **Blocks**: 他 issue ID

## Background
## Tasks
## Acceptance Criteria
## References
```

**SPEC 参照は必須**: 全 issue は `References` に該当する [Symphony SPEC](https://github.com/openai/symphony/blob/main/SPEC.md) のセクション (§番号付き) を含める。SPEC が source of truth であり、実装判断はここに帰着させる。直接の SPEC 要件が無い基盤作業でも、それが実現する SPEC コンポーネント (例: §3 System Overview) を明示する。

全体の進捗は [plans/roadmap.md](../plans/roadmap.md) を参照。

## 直近 issue 一覧

### P1 batch (loader → config → scheduler)

| ID | タイトル | Phase | Status | Depends on |
|---|---|---|---|---|
| [005](005-p1a-workflowfile.md) | WORKFLOW.md loader (front matter + body 分離) | P1a | Done | P0 (merged) |
| [006](006-p1b-wfconfig.md) | wfconfig — typed config view (default/$VAR/~/検証) | P1b | Done | 005 |
| [007](007-p1c-preflight-stub-scheduler.md) | dispatch preflight + stub scheduler loop | P1c | Open | 006 |

### P2 batch (tracker / workspace) — 完了

| ID | タイトル | Phase | Status | Depends on |
|---|---|---|---|---|
| [008](008-p2a-linear-tracker.md) | `platform/tracker` Linear GraphQL adapter | P2a | Done | P0 (merged) |
| [009](009-p2b-orchestrator-tracker.md) | `orchestrator/tracker` config wrapper | P2b | Done | 008, 006 |
| [010](010-p2c-workspace-manager.md) | `orchestrator/workspace` manager + hooks + safety | P2c | Done | 006 |

### P3 batch (scheduler core)

| ID | タイトル | Phase | Status | Depends on |
|---|---|---|---|---|
| [011](011-p3a-scheduler-state.md) | scheduler state machine + runtime state (§7) | P3a | Open | 007, 008 |
| [012](012-p3b-dispatch-tick.md) | poll/dispatch tick — eligibility/sort/concurrency/retry (§8) | P3b | Open | 011, 009 |
| [013](013-p3c-agent-runner.md) | agent runner — prompt + 生 codex 1 turn + events | P3c | Open | 010, P0c, 008 |
| [014](014-p3d-reconciliation.md) | reconciliation + startup cleanup (§8.5/§8.6) | P3d | Open | 011, 009, 010 |

## 依存関係グラフ

```
  P1:  005 ── 006 ── 007 (Open)
       (Done) (Done)  preflight + stub loop
                │
  P2:  008 ──── 009          010      ← 全 Done
       (Done)   (Done)       (Done)

  P3:        007 ─────────────┐
              │               │
              ▼               ▼
       011 ─┬─ 012 ────────────────┐
       state │  dispatch tick      ▼   ← 011+012+013 で M1 単線
             └─ 014               013
                reconcile         agent runner
                                  (010 + P0c)
```

- **P3a (011)** が scheduler の core。007 (loop scaffold) と 008 (Issue 型) が前提
- **012/014** は 011 に依存（dispatch tick と reconcile が state machine を使う）。012 は 009 (candidates)、014 は 009+010 も要る
- **013 (agent runner)** は 011 に**非依存**で並行可 — 010 (workspace) + P0c (codexclient) があれば書ける。012 は 013 の runner を **spawn 関数注入**で後から配線
- **M1 単線通電 = 007 + 011 + 012 + 013**。014 は堅牢性（stall/terminal 整理）を足す

## 完了済み (archive)

P0 batch (M0: 構造分離) は完了し [.archive/](.archive/) に移動:

- [001](.archive/001-p0a-physical-move.md) P0a 物理移動 / [002](.archive/002-p0b-agentlaunch.md) P0b agentlaunch / [003](.archive/003-p0c-codexclient.md) P0c codexclient / [004](.archive/004-p0d-cmd-scaffolding.md) P0d cmd 雛形

## 次の batch (P4 以降)

- P4: agent 起動を `agentlaunch` 経由の sandbox 配線に + codexclient 経由へ統一 — P3 + P0b/P0c が前提
- P5: `claude-app-server` shim 実装

詳細は [plans/04-phases.md](../plans/04-phases.md) / [plans/roadmap.md](../plans/roadmap.md) を参照。

## ライフサイクル

- 着手時に `Status: Open` → `Status: In Progress`
- PR open 時に PR 番号を Status 横に併記
- merge 後に `Status: Done`、関連 PR と完了日を記録
- 別 issue で blocked になったら `Status: Blocked` + 理由
