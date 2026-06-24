# Issues

[plans/](../plans/.archive/symphony-orchestrator/) で定めた計画を実行可能な単位に分解した issue 群。

## 表記規約

各 issue ファイルは以下のセクションを持つ:

```markdown
# <ID>: <タイトル>

- **Phase**: P0a / P0b / ... ([plans/04-phases.md](../plans/.archive/symphony-orchestrator/04-phases.md))
- **Status**: Open / In Progress / Blocked / Done
- **Depends on**: 他 issue ID または PR
- **Blocks**: 他 issue ID

## Background
## Tasks
## Acceptance Criteria
## References
```

**SPEC 参照は必須**: 全 issue は `References` に該当する [Symphony SPEC](https://github.com/openai/symphony/blob/main/SPEC.md) のセクション (§番号付き) を含める。SPEC が source of truth であり、実装判断はここに帰着させる。直接の SPEC 要件が無い基盤作業でも、それが実現する SPEC コンポーネント (例: §3 System Overview) を明示する。

全体の進捗は [plans/roadmap.md](../plans/.archive/symphony-orchestrator/roadmap.md) を参照。

## 直近 issue 一覧

M0–M4 (P0–P9) は実装・レビュー・archive 済み。SPEC §1–§17 を満たす実装が一通り揃い、Symphony SPEC 範囲の作業は完了。残るのは SPEC 範囲外の client-runtime follow-up（027 / 029）のみ。

### client-runtime フォローアップ（orchestrator-migration マージ後の single-writer port 由来）

roost client runtime（session manager）の issue。Symphony SPEC 範囲外で source of truth は [ARCHITECTURE.md](../ARCHITECTURE.md) "Single-writer event loop"。1179fcf single-writer port の code review で surface した、稀／既存／設計トレードオフ系の findings。

| ID | タイトル | Status | 概要 |
|---|---|---|---|
| [027](027-client-spawn-complete-resurrection.md) | spawn-complete が kill 済み frame を resurrect | Open | spawn 実行中に session 削除 → kill 先行（空 map）→ 後着 handleSpawnComplete が死んだ frame を登録し subsystem/container/endpoint/warm を leak |
| [029](029-client-warm-restart-registry-correctness.md) | warm-restart の registry 整合性 | Open | warm 復元で token 先行登録・mounts 後追いの窓（path 変換漏れ）。warm Save/Delete 競合・token 衝突非検出も併記 |

028（container endpoint/token が agent spawn 後に登録 → 早期 hook 取りこぼし）は解決済みで archive へ移動（下記）。

### client-runtime フォローアップ（2026-06-22 web-gateway-isolation インシデント由来）

`feat/tmux-free-web-server` 上の wedge インシデント残課題。直接の wedge は `slog.Warn`→`Debug` 降格と process isolation で塞いだが、daemon 内部のロバスト性に残った窪み 2 件。daemon が wedge する経路ではなく、UX レベルの単発 event ロスとして残る。

| ID | タイトル | Status | 概要 |
|---|---|---|---|
| [030](030-runtime-internalch-saturation-diagnosis.md) | internalCh の初回 saturation 発生源を診断 | Open | drop はもう自己増幅しないが、誰が最初に cap 64 を埋めるか未診断。`internalBroadcastSurface` / `internalBroadcastWire` / `connOpen|Close` を計装し主犯を特定 |
| [031](031-filerelay-drop-dirty-restore.md) | FileRelay sweep が broadcast drop 時に dirty/offset を戻さない | Open | sweep が dirty を先に落とし offset を進めた後で internalCh drop が起きると、読まれたログ行が永久に失われる。Web UI Log タブの瞬間的な行抜けに対応 |

## 完了済み (archive)

完了 issue は [.archive/](.archive/) に移動（記録として保持）:

- **M0 / P0 batch** (構造分離): [001](.archive/001-p0a-physical-move.md) 物理移動 / [002](.archive/002-p0b-agentlaunch.md) agentlaunch / [003](.archive/003-p0c-codexclient.md) codexclient / [004](.archive/004-p0d-cmd-scaffolding.md) cmd 雛形
- **M1 / P1 batch** (loader→config→scheduler): [005](.archive/005-p1a-workflowfile.md) loader / [006](.archive/006-p1b-wfconfig.md) wfconfig / [007](.archive/007-p1c-preflight-stub-scheduler.md) preflight+stub loop
- **M1 / P2 batch** (tracker/workspace): [008](.archive/008-p2a-linear-tracker.md) linear adapter / [009](.archive/009-p2b-orchestrator-tracker.md) tracker wrapper / [010](.archive/010-p2c-workspace-manager.md) workspace manager
- **M1 / P3 batch** (scheduler core): [011](.archive/011-p3a-scheduler-state.md) state machine / [012](.archive/012-p3b-dispatch-tick.md) dispatch tick / [013](.archive/013-p3c-agent-runner.md) agent runner / [014](.archive/014-p3d-reconciliation.md) reconciliation
- **M2 / P4 batch** (sandbox 配線): [015](.archive/015-p4a-agentlaunch-seam.md) Dispatcher seam / [016](.archive/016-p4b-devcontainer-mode.md) devcontainer + path 変換
- **M2 / P5 batch** (claude-app-server shim): [017](.archive/017-p5a-claude-streamjson.md) stream-json reader / [018](.archive/018-p5b-claude-app-server.md) shim 本体 / [019](.archive/019-p5c-agent-switch-conformance.md) usage+posture+agent 切替
- **M3 / P6–P8 batch** (機能完成): [020](.archive/020-p6a-continuation-loop.md) continuation loop / [021](.archive/021-p6b-metrics.md) metrics+stall / [022](.archive/022-p7-http-server.md) HTTP server / [023](.archive/023-p8a-hot-reload.md) hot reload / [024](.archive/024-p8b-linear-graphql-tool.md) `linear_graphql` tool (handler+wiring+advertise)
- **M4 / P9 batch** (conformance + docs): [025](.archive/025-p9a-conformance-suite.md) SPEC §17 conformance 群 + 対応表 / [026](.archive/026-p9b-positioning-docs.md) orchestrator 位置付け doc
- **client-runtime follow-up** (single-writer port 由来): [028](.archive/028-client-container-endpoint-registration-ordering.md) container hook 配送の bounded retry — container frame の status/要約/タグ + claude resume を復旧（fix aa1da86）

詳細は [plans/04-phases.md](../plans/.archive/symphony-orchestrator/04-phases.md) / [plans/roadmap.md](../plans/.archive/symphony-orchestrator/roadmap.md) を参照。

## ライフサイクル

- 着手時に `Status: Open` → `Status: In Progress`
- PR open 時に PR 番号を Status 横に併記
- merge 後に `Status: Done`、関連 PR と完了日を記録
- 別 issue で blocked になったら `Status: Blocked` + 理由
