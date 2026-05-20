# 009: orchestrator/tracker — config-driven tracker wrapper

- **Phase**: P2b ([plans/04-phases.md#p2-linear-adapter--workspace--hooks](../plans/04-phases.md))
- **Status**: Open
- **Depends on**: [008](008-p2a-linear-tracker.md) (adapter)、006 (merged; `wfconfig.Config`)
- **Blocks**: P3 (scheduler poll/reconcile が candidate/refresh を呼ぶ)

## Background

`platform/tracker/linear` の純粋な adapter を、`wfconfig.Config` から組み立てて業務観点で使う薄い wrapper を `orchestrator/tracker/` に実装する。platform 側に config 概念を持ち込まないための境界層。

## Tasks

### A. wrapper 構築

- [ ] `src/orchestrator/tracker/` 新設 (`package tracker`)
- [ ] `New(cfg wfconfig.Config) (Tracker, error)`:
  - [ ] `cfg.Tracker.Kind` を検証 (`linear` 以外は `unsupported_tracker_kind`)
  - [ ] `cfg.Tracker.{Endpoint, APIKey, ProjectSlug}` を `platform/tracker/linear.New` に渡す
  - [ ] `api_key` 空なら `missing_tracker_api_key`、`project_slug` 空なら `missing_tracker_project_slug` (preflight=007 と整合する error 分類を再利用)
- [ ] `cfg.Tracker.{ActiveStates, TerminalStates}` を保持し、候補/クリーンアップ呼び出し時に適用

### B. 業務オペレーション

- [ ] `Candidates(ctx)` → active states で `FetchCandidateIssues`
- [ ] `RefreshStates(ctx, ids)` → `FetchIssueStatesByIDs` (reconciliation 用、空 ids は空返し)
- [ ] `TerminalIssues(ctx)` → terminal states で `FetchIssuesByStates` (startup cleanup 用)

### C. エラー伝播 (§11.4)

- [ ] platform 層の typed error をそのまま透過 (`errors.Is` 維持)
- [ ] orchestrator 側の挙動 (candidate 失敗→tick skip 等) は **P3 の scheduler 責務**。本 issue は error を返すところまで

### D. テスト (§17.3)

- [ ] `kind != linear` で `unsupported_tracker_kind`
- [ ] api_key / project_slug 欠落で対応 typed error
- [ ] active/terminal state set が adapter 呼び出しに渡る (fake adapter で検証)
- [ ] 空 ids の `RefreshStates` が API 呼び出しなしで空返し

## Acceptance Criteria

- `wfconfig.Config` を渡すだけで tracker を構築し 3 オペレーションを呼べる
- `orchestrator/tracker` は `platform/tracker/linear` と `wfconfig` のみに依存 (scheduler に依存しない)
- §17.3 関連項目を test で pass、`go test ./orchestrator/tracker/` 緑、lint 緑

## References

- [Symphony SPEC](https://github.com/openai/symphony/blob/main/SPEC.md) §11.1 (required ops), §11.4 (error categories), §17.3
- [plans/04-phases.md#p2](../plans/04-phases.md)
- [008](008-p2a-linear-tracker.md) (adapter)、[007](007-p1c-preflight-stub-scheduler.md) (preflight の error 分類と整合)
