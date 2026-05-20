# Roadmap

Symphony SPEC 実装の全体ロードマップと進捗。設計の詳細は [04-phases.md](04-phases.md)、
個別の作業単位は [issues/](../issues/) を参照。

更新日: 2026-05-20

## 現在地

**M0 (構造分離) 完了。** P0a–P0d で `platform/` `client/` `cmd/` の三層分離と、
agentlaunch・codexclient/codexschema の抽出、3 バイナリ雛形が揃った。次は **M1 (最小単線通電)** に向けた P1。

## Phase 進捗

| Phase | 内容 | 状態 | issue |
|---|---|---|---|
| P0a | 物理移動 (`platform/`/`client/`/`cmd/`) | ✅ Done | [001](../issues/.archive/001-p0a-physical-move.md) |
| P0b | `agentlaunch/` を `platform/` へ抽出 | ✅ Done | [002](../issues/.archive/002-p0b-agentlaunch.md) |
| P0c | `codexclient/`+`codexschema/` 抽出・schema pin | ✅ Done | [003](../issues/.archive/003-p0c-codexclient.md) |
| P0d | `cmd/orchestrator`/`cmd/claude-app-server` 雛形 | ✅ Done | [004](../issues/.archive/004-p0d-cmd-scaffolding.md) |
| P1 | WORKFLOW.md loader + wfconfig + preflight + stub scheduler | ▶ Next | 005 / 006 / 007 |
| P2 | Linear adapter + workspace + 4 hooks | ⬜ Pending | — |
| P3 | scheduler core (poll/dispatch/retry/reconcile) + 生 codex 単線 | ⬜ Pending | — |
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
