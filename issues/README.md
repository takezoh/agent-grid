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

M1 (P1–P3) + P4 (sandbox 配線) は merge 済み。以降は **P5（agent 切替）と並列トラック（P6/P7/P8）を同時進行**できる。

### P5 batch (claude-app-server shim) — M2 後半 / 内部は直列

| ID | タイトル | Phase | Status | Depends on |
|---|---|---|---|---|
| [017](017-p5a-claude-streamjson.md) | `claude -p` stream-json reader (`platform/lib/claude/streamjson`) | P5a | Open | なし (leaf) |
| [018](018-p5b-claude-app-server.md) | claude-app-server shim — codex stdio + `claude -p` 中継 | P5b | Open | 017, P0c (merged) |
| [019](019-p5c-agent-switch-conformance.md) | token usage + approval posture + agent 切替 end-to-end | P5c | Open | 018 |

### 並列トラック (P5 と独立に進行可) — M3 前倒し

| ID | タイトル | Phase | Status | Depends on |
|---|---|---|---|---|
| [020](020-p6a-continuation-loop.md) | continuation multi-turn loop + worker-exit→state | P6a | Open | 013/011/012 (merged) |
| [021](021-p6b-metrics.md) | token/runtime 集計 + codex activity (stall) tracking | P6b | Open | 013/014 (merged) |
| [022](022-p7-http-server.md) | observability HTTP server (§13.7) | P7 | Open | 011 (merged) |
| [023](023-p8a-hot-reload.md) | WORKFLOW.md hot reload (§6.2) | P8a | Open | 006/007 (merged) |
| [024](024-p8b-linear-graphql-tool.md) | `linear_graphql` agent tool via mcpproxy (§10.5) | P8b | Open | 008/P0b/P0c (merged) |

## 依存関係グラフ

```
  merged (M1+P4): 013 agent runner / 011 state / 012 dispatch / 014 reconcile
                  P0b agentlaunch / P0c codexclient / 015·016 sandbox 配線

  P5 (直列):   017 streamjson ──► 018 shim ──► 019 切替/usage/posture ──► M2 完成
               (leaf)            (+P0c)        (agent 非依存を実証)

  並列トラック (P5 と同時・相互非依存、全て merged 基盤の上):
     020 continuation  ── orchestrator/agent + scheduler   (codex で完結)
     021 metrics/stall  ── platform/metrics + state         (agent 非依存集計)
     022 HTTP server    ── orchestrator/httpserver (read-only state)
     023 hot reload     ── scheduler/wfconfig
     024 linear_graphql ── platform/mcpproxy + tracker
```

- **P5 内は直列**: 017(reader) → 018(shim 本体) → 019(usage/posture/切替)。017 は leaf で即着手可
- **020–024 は P5 に非依存**で並列着手可。いずれも merged 済みの基盤（013/011/012/014/P0b/P0c/006-009）の上に立ち、**触るファイルが P5（`cmd/claude-app-server`）と重ならない**:
  - 020 → `orchestrator/agent`+`scheduler`（M1 レビュー #6 の worker-exit→state を解消）
  - 021 → `platform/metrics`+state（#7 の LastCodexTimestamp を解消）。022 が集計値を表示
  - 022 → 新規 `orchestrator/httpserver`（state を read-only 公開、§13.7 必須）
  - 023 → `scheduler`/`wfconfig`（§6.2 即時 reload + last-known-good）
  - 024 → `platform/mcpproxy`+`tracker`（agent 向け Linear tool、agent 種別非依存）
- **競合に注意**: 020 と 021 はともに `scheduler/state.go` の `RunAttempt` を触るため、片方先行 or 調整推奨。022 は 021 の集計値に依存（先に 021 → 022 が理想だが、022 は空集計でも単独着手可）

## 完了済み (archive)

完了 issue は [.archive/](.archive/) に移動（記録として保持）:

- **M0 / P0 batch** (構造分離): [001](.archive/001-p0a-physical-move.md) 物理移動 / [002](.archive/002-p0b-agentlaunch.md) agentlaunch / [003](.archive/003-p0c-codexclient.md) codexclient / [004](.archive/004-p0d-cmd-scaffolding.md) cmd 雛形
- **M1 / P1 batch** (loader→config→scheduler): [005](.archive/005-p1a-workflowfile.md) loader / [006](.archive/006-p1b-wfconfig.md) wfconfig / [007](.archive/007-p1c-preflight-stub-scheduler.md) preflight+stub loop
- **M1 / P2 batch** (tracker/workspace): [008](.archive/008-p2a-linear-tracker.md) linear adapter / [009](.archive/009-p2b-orchestrator-tracker.md) tracker wrapper / [010](.archive/010-p2c-workspace-manager.md) workspace manager
- **M1 / P3 batch** (scheduler core): [011](.archive/011-p3a-scheduler-state.md) state machine / [012](.archive/012-p3b-dispatch-tick.md) dispatch tick / [013](.archive/013-p3c-agent-runner.md) agent runner / [014](.archive/014-p3d-reconciliation.md) reconciliation
- **M2 / P4 batch** (sandbox 配線): [015](.archive/015-p4a-agentlaunch-seam.md) Dispatcher seam / [016](.archive/016-p4b-devcontainer-mode.md) devcontainer + path 変換

## 次の batch (P9)

- P9: SPEC §17 conformance test 群 + loki retirement（P5–P8 が出揃ってから。一部は先行して書ける）

詳細は [plans/04-phases.md](../plans/04-phases.md) / [plans/roadmap.md](../plans/roadmap.md) を参照。

## ライフサイクル

- 着手時に `Status: Open` → `Status: In Progress`
- PR open 時に PR 番号を Status 横に併記
- merge 後に `Status: Done`、関連 PR と完了日を記録
- 別 issue で blocked になったら `Status: Blocked` + 理由
