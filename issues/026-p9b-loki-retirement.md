# 026: loki retirement + orchestrator サービスの位置付け doc

- **Phase**: P9b ([plans/04-phases.md#p9-conformance-test--loki-retirement](../plans/04-phases.md))
- **Status**: Open
- **Depends on**: M3 機能完成（P6–P8a 済）。[025](025-p9a-conformance-suite.md)（conformance 表）先行が望ましいが独立着手可
- **Blocks**: M4 完成

## Background

`/workspace/loki`（Python、`github.com/takezoh/loki`）は本 orchestrator の**移植元プロトタイプ**。Go 実装が SPEC §1–§16 を満たした（M3）ため loki を **retire（凍結・後継明示）** する。あわせて agent-roost 本体の doc に orchestrator サービスの位置付けを記す。これにより「どの Python が何の Go package になったか」の provenance が追え、二重メンテを防ぐ。

> ⚠ **cross-repo**: loki retirement notice は別リポジトリ `/workspace/loki` への変更で、本リポジトリ（agent-roost-orchestrator）のコミットには含まれない（loki 側で別途コミット）。本 issue の AGENTS.md / ARCHITECTURE.md / provenance doc 更新は本リポジトリ内。

## Tasks

### A. loki retirement notice（cross-repo: `/workspace/loki`）

- [ ] `/workspace/loki/README.md` 冒頭に **retirement notice**: 「Go 移植 `agent-roost-orchestrator` が後継」「新規開発は行わない」「参照用に保持」を明示
- [ ] port-provenance リンク（loki ファイル → Go package）を notice に併記:
  - `lib/linear.py` → `platform/tracker/linear/`（[plans/04-phases.md](../plans/04-phases.md) P2a の移植元）
  - `loki2/config.py` → `orchestrator/wfconfig/` + `orchestrator/workflowfile/`
  - `loki2/prompt.py` → `orchestrator/prompt/`
  - `agent/` → `orchestrator/agent/` + `cmd/claude-app-server/`
  - scheduler 相当ロジック → `orchestrator/scheduler/`
- [ ] loki の `ARCHITECTURE.md` / `CLAUDE.md` にも retired 状態を一行追記（任意）

### B. orchestrator の位置付けを agent-roost doc に追記（本リポジトリ）

- [ ] `AGENTS.md` に orchestrator サービスの節を追加: 3 バイナリ（`roost` / `orchestrator` / `claude-app-server`）の役割、Symphony SPEC 実装である旨、build/test 導線（既存 `make build-all` 等への参照）
- [ ] `ARCHITECTURE.md` に三層（`platform/` / `client/` / `orchestrator/`）と orchestrator の責務（poll/dispatch/reconcile + observability HTTP）を追記。SPEC §3.1 の 8 component との対応は [plans/05-conformance.md](../plans/05-conformance.md) の用語対応表へリンク

### C. provenance map（本リポジトリ）

- [ ] `docs/orchestrator/` 配下（025 が作る conformance 表の隣）に **port-provenance 表**を置く or conformance doc 内に節を設ける: loki Python ファイル ↔ Go package の対応一覧。retirement notice (A) と同じ内容を正本として保持

## Acceptance Criteria

- loki README に retirement notice + 後継リンク + provenance が載り、retired 状態が一目で分かる（cross-repo コミット）
- agent-roost の `AGENTS.md` / `ARCHITECTURE.md` から orchestrator サービスの役割と build/test 導線が辿れる
- provenance 表が本リポジトリに存在し、移植元との対応が追える
- 本リポジトリ分の変更で `make vet` / `make lint` に影響なし（doc のみ）

## References

- [plans/04-phases.md#p9](../plans/04-phases.md)（作業項目 4–5: loki retire / doc 追記）
- [Symphony SPEC](https://github.com/openai/symphony/blob/main/SPEC.md) §1 §2.2 §11.5（loki の Linear status state machine を採用しない方針の根拠）
- [plans/05-conformance.md](../plans/05-conformance.md)（SPEC 用語 ↔ 実装名 対応表）、[025](025-p9a-conformance-suite.md)（conformance 表）
- 移植元: `/workspace/loki`（`github.com/takezoh/loki`）
