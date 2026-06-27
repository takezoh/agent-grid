# Issues

Symphony orchestrator 計画と client-runtime follow-up を実行可能な単位に分解した issue 群。
当初の計画ドキュメント (`plans/symphony-orchestrator/`) は実装完了に伴い削除済 — 経緯は git history を参照。

## 表記規約

各 issue ファイルは以下のセクションを持つ:

```markdown
# <ID>: <タイトル>

- **Phase**: P0a / P0b / ...
- **Status**: Open / In Progress / Blocked / Done
- **Depends on**: 他 issue ID または PR
- **Blocks**: 他 issue ID

## Background
## Tasks
## Acceptance Criteria
## References
```

**SPEC 参照は必須**: Symphony 範囲の issue は `References` に該当する [Symphony SPEC](https://github.com/openai/symphony/blob/main/SPEC.md) のセクション (§番号付き) を含める。SPEC が source of truth であり、実装判断はここに帰着させる。直接の SPEC 要件が無い基盤作業でも、それが実現する SPEC コンポーネント (例: §3 System Overview) を明示する。

## 直近 issue 一覧

**Open issue は現状なし** — Symphony SPEC §1–§17 を満たす実装 (M0–M4 / P0–P9) と client-runtime follow-up (027–031) はすべて archive 済。新規の課題はここに追記する。

## 完了済み (archive)

完了 issue は [.archive/](.archive/) に移動 (記録として保持):

- **M0 / P0 batch** (構造分離): [001](.archive/001-p0a-physical-move.md) 物理移動 / [002](.archive/002-p0b-agentlaunch.md) agentlaunch / [003](.archive/003-p0c-codexclient.md) codexclient / [004](.archive/004-p0d-cmd-scaffolding.md) cmd 雛形
- **M1 / P1 batch** (loader→config→scheduler): [005](.archive/005-p1a-workflowfile.md) loader / [006](.archive/006-p1b-wfconfig.md) wfconfig / [007](.archive/007-p1c-preflight-stub-scheduler.md) preflight+stub loop
- **M1 / P2 batch** (tracker/workspace): [008](.archive/008-p2a-linear-tracker.md) linear adapter / [009](.archive/009-p2b-orchestrator-tracker.md) tracker wrapper / [010](.archive/010-p2c-workspace-manager.md) workspace manager
- **M1 / P3 batch** (scheduler core): [011](.archive/011-p3a-scheduler-state.md) state machine / [012](.archive/012-p3b-dispatch-tick.md) dispatch tick / [013](.archive/013-p3c-agent-runner.md) agent runner / [014](.archive/014-p3d-reconciliation.md) reconciliation
- **M2 / P4 batch** (sandbox 配線): [015](.archive/015-p4a-agentlaunch-seam.md) Dispatcher seam / [016](.archive/016-p4b-devcontainer-mode.md) devcontainer + path 変換
- **M2 / P5 batch** (claude-app-server shim): [017](.archive/017-p5a-claude-streamjson.md) stream-json reader / [018](.archive/018-p5b-claude-app-server.md) shim 本体 / [019](.archive/019-p5c-agent-switch-conformance.md) usage+posture+agent 切替
- **M3 / P6–P8 batch** (機能完成): [020](.archive/020-p6a-continuation-loop.md) continuation loop / [021](.archive/021-p6b-metrics.md) metrics+stall / [022](.archive/022-p7-http-server.md) HTTP server / [023](.archive/023-p8a-hot-reload.md) hot reload / [024](.archive/024-p8b-linear-graphql-tool.md) `linear_graphql` tool (handler+wiring+advertise)
- **M4 / P9 batch** (conformance + docs): [025](.archive/025-p9a-conformance-suite.md) SPEC §17 conformance 群 + 対応表 / [026](.archive/026-p9b-positioning-docs.md) orchestrator 位置付け doc
- **client-runtime follow-up** (single-writer port 由来): [027](.archive/027-client-spawn-complete-resurrection.md) handleSpawnComplete の resurrection leak (fix 046e5558) / [028](.archive/028-client-container-endpoint-registration-ordering.md) container hook 配送の bounded retry (fix aa1da86) / [029](.archive/029-client-warm-restart-registry-correctness.md) warm restart の token+mounts atomic 登録 + Save 同期化 + token rebind 検出 (fix 4fe60dea)
- **client-runtime follow-up** (2026-06-22 web-gateway-isolation 由来): [030](.archive/030-runtime-internalch-saturation-diagnosis.md) internalCh drop counter (fix 55321ad0) / [031](.archive/031-filerelay-drop-dirty-restore.md) FileRelay sweep の dirty/offset rollback (fix 55321ad0)

## ライフサイクル

- 着手時に `Status: Open` → `Status: In Progress`
- PR open 時に PR 番号を Status 横に併記
- merge 後に `Status: Done`、関連 PR と完了日を記録
- 別 issue で blocked になったら `Status: Blocked` + 理由
