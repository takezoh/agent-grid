# 020: orchestrator/agent — continuation multi-turn loop + worker-exit → scheduler state

- **Phase**: P6a ([plans/04-phases.md#p6-continuation--reconciliation--metrics](../plans/04-phases.md))
- **Status**: Open
- **Depends on**: 013 (merged; single-turn runner)、011 (merged; state machine)、012 (merged; retry/dispatch)
- **並行可**: P5 (017–019) と別ファイル（P5 は `cmd/claude-app-server`、本 issue は `orchestrator/agent`+`scheduler`）。codex で完結し claude shim 不要
- **Blocks**: M3

## Background

013 は **single turn** で、turn 解決後に session を停止する（M1 レビューで continuation は P6 送りと明記）。本 issue で SPEC §16.5 の while-loop を実装し、**worker の正常終了を scheduler state に反映**する（M1 レビュー #6 の積み残し）。これにより 1 issue で複数 turn を回し、turn 完了後に slot が解放される。

現状の積み残し（M1 レビュー）:
- `Runner.Spawn` の emit は slog ログのみで、`WorkerExitNormal/Abnormal` を呼ばない → 完了後も issue が `running` に残り slot が解放されない
- 011 の `WorkerExitNormal`（continuation retry 生成）/ `WorkerExitAbnormal`（backoff）は実装済みだが **呼ばれていない**

## Tasks

### A. continuation while-loop (§16.5)

- [ ] turn 完了後に session を**停止せず**、tracker で issue 状態を再取得（`FetchIssueStatesByIDs`／§16.5）
- [ ] issue が active かつ `turn_number < max_turns` なら同一 thread で次 turn（`thread/resume` 経由、codexclient）
- [ ] terminal/non-active へ遷移、または `max_turns` 到達で loop 終了 → session 停止 → `after_run` → 正常終了
- [ ] 単一 turn 用に M1 で入れた「turn 解決後 cancel」を multi-turn 用に作り替える（最終 turn 後のみ停止）

### B. worker-exit → scheduler state 配線（#6 解消）

- [ ] worker 正常終了で `state.WorkerExitNormal(issueID)` → continuation RetryEntry を 1s 固定遅延で enqueue（§8.4）。これで再 dispatch 機会が来る
- [ ] worker 異常終了で `state.WorkerExitAbnormal(issueID, err, attempt)` → backoff retry（§8.4）
- [ ] 配線箇所: scheduler が worker 完了を観測できる seam を入れる（`SpawnFunc` が完了 channel/コールバックを返す、もしくは emit を scheduler 側ハンドラに繋ぐ）。011/012 の既存関数を使い single-authority（loop goroutine）で state 変更する

### C. 設計判断（実装前に PR で確定）

- [ ] **worker 完了の通知経路**: (1) `LiveSession` に done channel を持たせ loop が select、(2) Spawn にコールバック注入、(3) emit を scheduler の event channel に流す。single-authority（state 変更は loop goroutine のみ）を守る案を選ぶ

## Tasks（テスト §17.5）

- [ ] fake codex で 2 turn 回り、active のまま 2 回目に進む / terminal で停止する
- [ ] `max_turns` 到達で停止
- [ ] 正常終了で `running` から消え continuation retry が enqueue される
- [ ] 異常終了で backoff retry が enqueue される

## Acceptance Criteria

- 1 issue で複数 turn を回せる（active 継続中）
- 正常完了で slot が解放され、continuation retry（1s）で再 dispatch 機会が来る
- single-authority invariant（state 変更は loop goroutine のみ）を維持
- `go test ./orchestrator/...` 緑、lint 緑

## References

- [Symphony SPEC](https://github.com/openai/symphony/blob/main/SPEC.md) §16.5 (Worker Attempt while-loop), §16.6 (Worker Exit/Retry), §8.4 (continuation 1s / backoff), §7.3
- [plans/04-phases.md#p6](../plans/04-phases.md)、`orchestrator/scheduler`（`WorkerExitNormal`/`WorkerExitAbnormal`）、`orchestrator/agent`
