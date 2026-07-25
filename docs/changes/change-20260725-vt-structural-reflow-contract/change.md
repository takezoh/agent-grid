---
id: change-20260725-vt-structural-reflow-contract
kind: change
title: VT structural reflow contract
status: draft
created: '2026-07-16'
profile: sdd@1
intent: Replace split cell/provenance ownership with one mutation-closed x/vt boundary
  and preserve it through resize, snapshot, concurrency, and integration.
outcomes:
- Cells と RowBoundary は x/vt semantic owner 経由でのみ変更される。
- primary resize は semantic history を reflow し、alternate resize は physical row identity
  を保つ。
- Concrete ReattachSnapshot は bytes または typed error を返し、SafeEmulator は観測全体で lock を保持する。
scope:
- x/vt semantic row ownership and defensive projections
- primary semantic reflow and alternate physical resize
- typed concrete snapshot API and SafeEmulator locking
- 'agent-grid failure mapping and PR #908/#919 integration regression'
non_goals:
- ultraviolet changes
- raw PTY replay or browser-side terminal reconstruction
- Terminal interface widening
- 'replacement or folding of the independent PR #908 margin clamp'
change_classes:
- behavior
- boundary
- invariant
governance:
  gate: auto
  reasons: []
members:
- role: requirements
  path: changes/change-20260725-vt-structural-reflow-contract/requirements.md
  required: true
- role: implementation
  path: changes/change-20260725-vt-structural-reflow-contract/implementation.md
  required: true
- role: verification
  path: changes/change-20260725-vt-structural-reflow-contract/verification.md
  required: true
promotion: []
unresolved_decisions: []
tags: []
owners: []
relations:
- {type: references, target: adr-20260716-vt-semantic-buffer-owner}
- {type: references, target: adr-20260716-vt-screen-specific-resize}
- {type: references, target: adr-20260716-vt-snapshot-failure-and-locking}
- {type: references, target: adr-20260716-vt-reflow-pr-migration}
source_paths: []
summary: x/vt の cell と provenance の分割所有をやめ、mutation-closed な semantic row owner 1
  箇所に集約し、resize・snapshot・並行性・統合まで契約を貫通させる。
---

## Summary

format v2 cutover (`df834977`) より後に v1 形式で追加された `docs/specs/vt-structural-reflow-contract/` (spec + plan) を v2 change package へ移行したもの。requirements / implementation member は当時の v1 artifact を verbatim で保持している。

移行時点で plan の milestone m1-m4 は全て `todo`、spec / plan とも `draft`。関連 ADR 4 件も `proposed` のままであり、この change は未着手の作業として `draft` を維持する。`71980572 fix(terminal): reflow retained history at viewer geometry` は先行して着地した局所修正であり、本 change の milestone 完了を意味しない。

## Closure Notes
