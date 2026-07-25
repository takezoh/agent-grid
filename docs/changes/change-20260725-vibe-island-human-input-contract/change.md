---
id: change-20260725-vibe-island-human-input-contract
kind: change
title: Vibe Island human-input (approval/question) experience contract
status: ready
created: '2026-07-25'
profile: sdd@1
intent: Vibe Island の承認/質問ハンドリングの UX がどこまで契約化されているかを可視化し、検証済みの as-is 体験を退行から守る判別的契約
  (ux.md + requirements-plan.json) として固定する。欠落 3 点は後続 change の相談材料として明示保持する。
outcomes:
- ux の 14 UAC すべてが verification.md の対応表で Tier + テスト参照 (または T3-manual 理由) を持ち、T0/T1
  の対象テスト (dotnet test 2 プロジェクト / go test host/state・server/api) が green である
- production code (clients/windows-shell の非 Tests プロジェクト・src/) に diff が無いこと (adr-20260725-vibe-island-asis-contract-pin
  の confirmation)
scope:
- docs/adr/adr-20260725-vibe-island-asis-contract-pin.md
- docs/adr/adr-20260725-vibe-island-verification-tiers.md
- clients/windows-shell/AgentGrid.Shell.Core.Tests/
- clients/windows-shell/AgentGrid.Shell.Platform.Tests/
- src/host/state/
- src/server/api/
non_goals: []
change_classes:
- invariant
governance:
  gate: auto
  reasons: []
members:
- role: requirements
  path: changes/change-20260725-vibe-island-human-input-contract/requirements.md
  required: true
- role: implementation
  path: changes/change-20260725-vibe-island-human-input-contract/implementation.md
  required: true
- role: verification
  path: changes/change-20260725-vibe-island-human-input-contract/verification.md
  required: true
- role: ux
  path: changes/change-20260725-vibe-island-human-input-contract/ux.md
  required: false
- role: consultation
  path: changes/change-20260725-vibe-island-human-input-contract/consultations/20260725-unresolved-dps.md
  required: false
- role: consultation
  path: changes/change-20260725-vibe-island-human-input-contract/consultations/20260725-adr-acceptance.md
  required: false
promotion: []
unresolved_decisions: []
tags: []
owners: []
relations: []
source_paths: []
summary: 'Vibe Island の human-input (承認/質問) 体験を実装調査に基づく判別可能な as-is 契約として固定する requirements/ux
  成果物。相談済み: pending replay と選択肢質問は導入決定、Claude 一級化は design 検証先行。'
updated: '2026-07-25'
---

## Summary

## Closure Notes
