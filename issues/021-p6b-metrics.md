# 021: platform/metrics — token/runtime aggregation + codex activity (stall) tracking

- **Phase**: P6b ([plans/04-phases.md#p6-continuation--reconciliation--metrics](../plans/04-phases.md))
- **Status**: Open
- **Depends on**: 013 (merged; runner emits events)、014 (merged; stall 検知の枠)
- **並行可**: P5 と別領域。codex の既存 event から集計でき claude shim 不要（agent 非依存の集計）
- **Blocks**: M3、022 (HTTP server が集計値を出す)

## Background

SPEC §13.5 が要請する token/runtime 集計と、§8.5 Part A の stall 検知に必要な **`last_codex_timestamp` 更新**を実装する（後者は M1 レビュー #7 の積み残し）。現状 `RunAttempt.LastCodexTimestamp`/`Total*Tokens` は誰も更新せず、stall は常に `StartedAt` 基準・token は 0。

## Tasks

### A. codex activity tracking（#7 解消）

- [ ] agent runner が codex event 受信ごとに `RunAttempt.LastCodexTimestamp`/`LastCodexEvent`/`LastCodexMessage` を更新する seam を入れる（state 更新は single-authority を守る経路で）
- [ ] これにより 014 の stall 検知が「最終活動からの経過」で正しく効く（現状 dispatch からの経過）

### B. token / runtime 集計 (§13.5)

- [ ] `platform/metrics/` 新設: input/output/total tokens の集計。**absolute thread totals 優先・delta フォールバック**（§13.5 の優先順位）を判別
- [ ] runtime seconds 集計（turn/session の経過）
- [ ] rate-limit snapshot（codex/claude が返す場合）の保持
- [ ] codex の `turn/completed` usage（および claude は 019 が emit）を取り込み `RunAttempt.Total*Tokens` に反映

### C. テスト (§17.5)

- [ ] event 受信で LastCodexTimestamp が進む → stall 検知が活動基準になる
- [ ] absolute totals と delta フォールバックの判別が正しい
- [ ] usage event から input/output/total が集計される

## Acceptance Criteria

- stall 検知が「最終 codex 活動からの経過」で動く（dispatch 基準でない）
- token（input/output/total）と runtime が正確に集計され RunAttempt/observability に載る
- agent 非依存（codex / claude いずれの usage event でも集計できる）
- `go test ./platform/metrics/ ./orchestrator/...` 緑、lint 緑

## References

- [Symphony SPEC](https://github.com/openai/symphony/blob/main/SPEC.md) §13.5 (Token/runtime accounting), §8.5 Part A (stall via last activity), §10.4 (usage events)
- [plans/04-phases.md#p6](../plans/04-phases.md)、`orchestrator/scheduler/state.go`（`RunAttempt.LastCodex*`/`Total*Tokens`）、`orchestrator/scheduler/reconcile.go`（stall）
