# 023: orchestrator — WORKFLOW.md hot reload (§6.2 live re-apply)

- **Phase**: P8a ([plans/04-phases.md#p8-hot-reload--linear_graphql-tool](../plans/04-phases.md))
- **Status**: Open
- **Depends on**: 006 (merged; wfconfig)、007 (merged; scheduler loop)
- **並行可**: P5 と完全独立。scheduler/wfconfig のみに触れる
- **Blocks**: M3

## Background

SPEC §6.2 は WORKFLOW.md 変更時に **再起動なしで config を re-read・re-apply** することを要請。現状 scheduler は tick ごとに `reloadConfig()` で再読込しており毎 tick で反映はされるが、§6.2 が求める (1) **即時** reload（fsnotify）と (2) **不正 reload 時の last-known-good 保持 + operator-visible warn** が未実装。

## Tasks

### A. fsnotify watch

- [ ] WORKFLOW.md を fsnotify で監視（`go.mod` に `fsnotify` 既存 — `platform/lib` の利用パターン参照）。変更検知で即時 reload signal を loop に送る（single-authority: 実際の re-apply は loop goroutine）
- [ ] poll interval を待たずに反映

### B. live re-apply (§6.2)

- [ ] poll interval / concurrency（global・per-state）/ active-terminal state set を**動的反映**
- [ ] codex settings は次回 dispatch から反映（実行中 turn は触らない）
- [ ] 不正な reload（parse/validate/preflight 失敗）は **last known good を保持**し、operator-visible warn を出す（§5.5 dispatch gating: workflow エラーは新規 dispatch をブロックするが既存は壊さない）

### C. テスト (§17.1 系)

- [ ] WORKFLOW.md 変更で interval/concurrency が即時に変わる
- [ ] 不正 reload で last-known-good が保持され warn が出る、既存 running は影響を受けない
- [ ] fsnotify の発火を fake 化（または一時ファイル書換）して検証

## Acceptance Criteria

- WORKFLOW.md を save すると orchestrator が即時に再読込し設定を live 反映
- 不正な reload で停止せず last-known-good 継続 + 警告
- `go test ./orchestrator/...` 緑、lint 緑

## References

- [Symphony SPEC](https://github.com/openai/symphony/blob/main/SPEC.md) §6.2 (Dynamic reload), §5.5 (error surface / dispatch gating)
- [plans/04-phases.md#p8](../plans/04-phases.md)、`orchestrator/scheduler/scheduler.go`（`reloadConfig`）、`orchestrator/wfconfig`
