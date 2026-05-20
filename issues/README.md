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

M0–M2 と M3 前半 (P5–P8a) は実装・レビュー・archive 済み。残るのは **024 の tool advertise（pinned codex schema 制約で blocked）** と **M4 (P9 conformance)** のみ。

### 進行中 / 残タスク

| ID | タイトル | Phase | Status | 残り |
|---|---|---|---|---|
| [024](024-p8b-linear-graphql-tool.md) | `linear_graphql` agent tool (native `item/tool/call`, §10.5) | P8b | Partial / Blocked | handler + wiring は実装済。**advertise** は codex 0.128.0 で `DynamicToolSpec` が orphan（request からの `$ref` 参照ゼロ）のため不能 → codex schema bump 待ち |
| [025](025-p9a-conformance-suite.md) | SPEC §17 conformance test 群 + conformance 表 | P9a | Open | 005–023（merged） |
| [026](026-p9b-positioning-docs.md) | orchestrator サービスの位置付けを agent-roost doc に追記 | P9b | Open | M3（done）、025 推奨先行 |

### 次の batch (M4 / P9)

- **025 (P9a)**: SPEC §17.1–§17.7 の網羅監査 + `TestSPEC_*` canonical マーカー + §17.8 実 Linear profile + `docs/orchestrator/symphony-conformance.md`
- **026 (P9b)**: `AGENTS.md` / `ARCHITECTURE.md` への orchestrator サービス位置付け追記 + SPEC component ↔ package 対応表
- 024 advertise の end-to-end のみ codex schema bump 待ちで保留（P9 の範囲外）

## 完了済み (archive)

完了 issue は [.archive/](.archive/) に移動（記録として保持）:

- **M0 / P0 batch** (構造分離): [001](.archive/001-p0a-physical-move.md) 物理移動 / [002](.archive/002-p0b-agentlaunch.md) agentlaunch / [003](.archive/003-p0c-codexclient.md) codexclient / [004](.archive/004-p0d-cmd-scaffolding.md) cmd 雛形
- **M1 / P1 batch** (loader→config→scheduler): [005](.archive/005-p1a-workflowfile.md) loader / [006](.archive/006-p1b-wfconfig.md) wfconfig / [007](.archive/007-p1c-preflight-stub-scheduler.md) preflight+stub loop
- **M1 / P2 batch** (tracker/workspace): [008](.archive/008-p2a-linear-tracker.md) linear adapter / [009](.archive/009-p2b-orchestrator-tracker.md) tracker wrapper / [010](.archive/010-p2c-workspace-manager.md) workspace manager
- **M1 / P3 batch** (scheduler core): [011](.archive/011-p3a-scheduler-state.md) state machine / [012](.archive/012-p3b-dispatch-tick.md) dispatch tick / [013](.archive/013-p3c-agent-runner.md) agent runner / [014](.archive/014-p3d-reconciliation.md) reconciliation
- **M2 / P4 batch** (sandbox 配線): [015](.archive/015-p4a-agentlaunch-seam.md) Dispatcher seam / [016](.archive/016-p4b-devcontainer-mode.md) devcontainer + path 変換
- **M2 / P5 batch** (claude-app-server shim): [017](.archive/017-p5a-claude-streamjson.md) stream-json reader / [018](.archive/018-p5b-claude-app-server.md) shim 本体 / [019](.archive/019-p5c-agent-switch-conformance.md) usage+posture+agent 切替
- **M3 / P6–P8a batch** (機能完成): [020](.archive/020-p6a-continuation-loop.md) continuation loop / [021](.archive/021-p6b-metrics.md) metrics+stall / [022](.archive/022-p7-http-server.md) HTTP server / [023](.archive/023-p8a-hot-reload.md) hot reload

詳細は [plans/04-phases.md](../plans/04-phases.md) / [plans/roadmap.md](../plans/roadmap.md) を参照。

## ライフサイクル

- 着手時に `Status: Open` → `Status: In Progress`
- PR open 時に PR 番号を Status 横に併記
- merge 後に `Status: Done`、関連 PR と完了日を記録
- 別 issue で blocked になったら `Status: Blocked` + 理由
