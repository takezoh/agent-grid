# Roadmap

Symphony SPEC 実装の全体ロードマップと進捗。設計の詳細は [04-phases.md](04-phases.md)、
個別の作業単位は [issues/](../issues/) を参照。

更新日: 2026-05-20

## 現在地

**M0 (構造分離) 完了。M1 (最小単線通電) 進行中。** P1 は loader (005)・wfconfig (006) 完了、残るは
preflight + stub scheduler (007)。P2 (tracker 008/009・workspace 010) は完了。次は **P3 (scheduler core)** を
4 issue (011–014) に分解して起票済み。011+012+013 が揃うと M1 の単線が通る。

## Phase 進捗

| Phase | 内容 | 状態 | issue |
|---|---|---|---|
| P0a | 物理移動 (`platform/`/`client/`/`cmd/`) | ✅ Done | [001](../issues/.archive/001-p0a-physical-move.md) |
| P0b | `agentlaunch/` を `platform/` へ抽出 | ✅ Done | [002](../issues/.archive/002-p0b-agentlaunch.md) |
| P0c | `codexclient/`+`codexschema/` 抽出・schema pin | ✅ Done | [003](../issues/.archive/003-p0c-codexclient.md) |
| P0d | `cmd/orchestrator`/`cmd/claude-app-server` 雛形 | ✅ Done | [004](../issues/.archive/004-p0d-cmd-scaffolding.md) |
| P1a | WORKFLOW.md loader | ✅ Done | [005](../issues/005-p1a-workflowfile.md) |
| P1b | wfconfig typed config | ✅ Done | [006](../issues/006-p1b-wfconfig.md) |
| P1c | preflight + stub scheduler loop | ▶ Next | [007](../issues/007-p1c-preflight-stub-scheduler.md) |
| P2a | `platform/tracker` Linear adapter | ✅ Done | [008](../issues/008-p2a-linear-tracker.md) |
| P2b | `orchestrator/tracker` config wrapper | ✅ Done | [009](../issues/009-p2b-orchestrator-tracker.md) |
| P2c | `orchestrator/workspace` manager + hooks | ✅ Done | [010](../issues/010-p2c-workspace-manager.md) |
| P3a | scheduler state machine + runtime state (§7) | ⬜ Open | [011](../issues/011-p3a-scheduler-state.md) |
| P3b | poll/dispatch tick — eligibility/sort/concurrency/retry (§8) | ⬜ Open | [012](../issues/012-p3b-dispatch-tick.md) |
| P3c | agent runner — prompt + 生 codex 1 turn + events (§10/§16.5) | ⬜ Open | [013](../issues/013-p3c-agent-runner.md) |
| P3d | reconciliation + startup cleanup (§8.5/§8.6) | ⬜ Open | [014](../issues/014-p3d-reconciliation.md) |
| P4 | agent 起動を codexclient 経由に + sandbox 配線 | ⬜ Pending | — |
| P5 | `claude-app-server` shim 実装 | ⬜ Pending | — |
| P6 | continuation turn + stall + reconciliation + metrics | ⬜ Pending | — |
| P7 | HTTP server (`/`, `/api/v1/*`) | ⬜ Pending | — |
| P8 | WORKFLOW.md hot reload + `linear_graphql` tool | ⬜ Pending | — |
| P9 | SPEC §17 conformance test + loki retirement | ⬜ Pending | — |

## マイルストーン

| | Phase | 意義 | 状態 |
|---|---|---|---|
| **M0** 構造分離完了 | P0a–P0d | 後続の物理基盤確立 | ✅ Done |
| **M1** 最小単線通電 | P1–P3 | 1 issue → codex app-server で 1 turn | ▶ 進行中 |
| **M2** 多 agent 対応 | P4–P5 | claude / codex 切替 | ⬜ |
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
