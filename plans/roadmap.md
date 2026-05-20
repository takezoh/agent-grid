# Roadmap

Symphony SPEC 実装の全体ロードマップと進捗。設計の詳細は [04-phases.md](04-phases.md)、
個別の作業単位は [issues/](../issues/) を参照。

更新日: 2026-05-20

## 現在地

**M0 (構造分離)・M1 (最小単線通電) 完了。M2 (多 agent 対応) は P4 完了・P5 着手。**
P1–P3 (005–014) と P4 (015–016) を全て実装・レビュー・merge 済み（015/016 のレビュー修正は `1cacc74`、
main へ ff-merge 済み）。direct/devcontainer の両モードで agent launch が `agentlaunch.Dispatcher` 経由に。
残るは **P5 (claude-app-server shim)** を 3 issue (017–019) に分解して起票。017 で `claude -p` の stream-json
reader、018 で shim 本体、019 で agent 切替 end-to-end を通すと M2 完成。

## Phase 進捗

| Phase | 内容 | 状態 | issue |
|---|---|---|---|
| P0a | 物理移動 (`platform/`/`client/`/`cmd/`) | ✅ Done | [001](../issues/.archive/001-p0a-physical-move.md) |
| P0b | `agentlaunch/` を `platform/` へ抽出 | ✅ Done | [002](../issues/.archive/002-p0b-agentlaunch.md) |
| P0c | `codexclient/`+`codexschema/` 抽出・schema pin | ✅ Done | [003](../issues/.archive/003-p0c-codexclient.md) |
| P0d | `cmd/orchestrator`/`cmd/claude-app-server` 雛形 | ✅ Done | [004](../issues/.archive/004-p0d-cmd-scaffolding.md) |
| P1a | WORKFLOW.md loader | ✅ Done | [005](../issues/.archive/005-p1a-workflowfile.md) |
| P1b | wfconfig typed config | ✅ Done | [006](../issues/.archive/006-p1b-wfconfig.md) |
| P1c | preflight + stub scheduler loop | ✅ Done | [007](../issues/.archive/007-p1c-preflight-stub-scheduler.md) |
| P2a | `platform/tracker` Linear adapter | ✅ Done | [008](../issues/.archive/008-p2a-linear-tracker.md) |
| P2b | `orchestrator/tracker` config wrapper | ✅ Done | [009](../issues/.archive/009-p2b-orchestrator-tracker.md) |
| P2c | `orchestrator/workspace` manager + hooks | ✅ Done | [010](../issues/.archive/010-p2c-workspace-manager.md) |
| P3a | scheduler state machine + runtime state (§7) | ✅ Done | [011](../issues/.archive/011-p3a-scheduler-state.md) |
| P3b | poll/dispatch tick — eligibility/sort/concurrency/retry (§8) | ✅ Done | [012](../issues/.archive/012-p3b-dispatch-tick.md) |
| P3c | agent runner — prompt + codex 1 turn + events (§10/§16.5) | ✅ Done | [013](../issues/.archive/013-p3c-agent-runner.md) |
| P3d | reconciliation + startup cleanup (§8.5/§8.6) | ✅ Done | [014](../issues/.archive/014-p3d-reconciliation.md) |
| P4a | launch を `agentlaunch.Dispatcher` 経由に (direct mode) | ✅ Done | [015](../issues/.archive/015-p4a-agentlaunch-seam.md) |
| P4b | devcontainer モード + host↔container path 変換 | ✅ Done | [016](../issues/.archive/016-p4b-devcontainer-mode.md) |
| P5a | `claude -p` stream-json reader (`platform/lib/claude/streamjson`) | ▶ Next | [017](../issues/017-p5a-claude-streamjson.md) |
| P5b | `claude-app-server` shim — codex stdio + `claude -p` 中継 | ⬜ Open | [018](../issues/018-p5b-claude-app-server.md) |
| P5c | token usage + approval posture + agent 切替 end-to-end | ⬜ Open | [019](../issues/019-p5c-agent-switch-conformance.md) |
| P6a | continuation multi-turn loop + worker-exit→state | ⬜ Open (並列可) | [020](../issues/020-p6a-continuation-loop.md) |
| P6b | token/runtime 集計 + codex activity (stall) tracking | ⬜ Open (並列可) | [021](../issues/021-p6b-metrics.md) |
| P7 | observability HTTP server (`/`, `/api/v1/*`) | ⬜ Open (並列可) | [022](../issues/022-p7-http-server.md) |
| P8a | WORKFLOW.md hot reload (§6.2) | ⬜ Open (並列可) | [023](../issues/023-p8a-hot-reload.md) |
| P8b | `linear_graphql` agent tool (mcpproxy, §10.5) | ⬜ Open (並列可) | [024](../issues/024-p8b-linear-graphql-tool.md) |
| P9 | SPEC §17 conformance test + loki retirement | ⬜ Pending | — |

## マイルストーン

| | Phase | 意義 | 状態 |
|---|---|---|---|
| **M0** 構造分離完了 | P0a–P0d | 後続の物理基盤確立 | ✅ Done |
| **M1** 最小単線通電 | P1–P3 | 1 issue → codex app-server で 1 turn | ✅ Done |
| **M2** 多 agent 対応 | P4–P5 | sandbox 配線 + claude / codex 切替 | ▶ 進行中 |
| **M3** SPEC 機能完成 | P6–P8 | SPEC §1–§16 を満たす | ⬜ |
| **M4** conformance 確認 | P9 | SPEC §17 test pass + loki retire | ⬜ |

## P0 で確立した基盤 (現状)

- **三層境界**: `platform/` (共有基盤) ↛ `client/`/`orchestrator/`、`client/` ↛ `orchestrator/` を depguard で実効化
- **agentlaunch**: `platform/agentlaunch/` の `Dispatcher` を import すれば sandbox 配線済みで agent を起動可能。client 固有概念 (FrameID/SandboxOverride) は adapter で遮断
- **codexclient**: transport 非依存の JSON-RPC framing (`Conn`)。ws (roost) と stdio (shim/orchestrator) の両 transport。server helper を shim に提供
- **codexschema**: codex-cli 0.128.0 で pin、CI で drift 検出
- **3 バイナリ**: `roost` / `orchestrator` (stub) / `claude-app-server` (stub) が同一 module から build

## issue の置き場

- [issues/](../issues/) — 進行中・未着手の作業単位
- [issues/.archive/](../issues/.archive/) — 完了済み issue (記録として保持)
