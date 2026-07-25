---
change: change-20260725-vibe-island-human-input-contract
role: verification
---

<!-- lifecycle is owned by change.md -->

# Verification

## Content

Tier 戦略は [adr-20260725-vibe-island-verification-tiers](../../adr/adr-20260725-vibe-island-verification-tiers.md) が所有する: 新ハーネスは作らず、witness を **T0 (純粋層 unit)** / **T1 (Go always-on)** / **T3 (FlaUI UIA e2e・run-dev ハーネス)** の既存 3 層へ帰着させる。verification id ↔ requirement ↔ witness の正本は [design-plan.json](design-plan.json) の `implementation_contracts[].verification / witnesses`。

## 実行コマンド

| verification id | Tier | コマンド |
|---|---|---|
| verify-island-core-t0 / verify-lifecycle-core-t0 | T0 | `dotnet test clients/windows-shell/AgentGrid.Shell.Core.Tests` |
| verify-toast-router-t0 / verify-engage-focus-t0 | T0 | `dotnet test clients/windows-shell/AgentGrid.Shell.Platform.Tests` |
| verify-state-reducer | T1 | `cd src && go test ./host/state/...` |
| verify-gateway-scenario | T1 | `cd src && go test ./server/api/...` |
| verify-uia-e2e | T3 | `make test-windows-shell-e2e` |

## UAC × Tier × テスト対応表

状態: **pinned** = 既存テストが assert 済み / **補強** = unit 2/3 でテスト追加 / **T3-manual** = 自動化せず手動確認と理由を記録。

| UAC | 要件 | Tier | テスト参照 | 状態 |
|---|---|---|---|---|
| UAC-001 | FR-001 | T0 + T3 | IslandStateMachineTests (新規 pending で再展開) / WsFrameParserTests (ar) / フォーカス非奪取は UIA (NOACTIVATE) | pinned (フォーカスは T3) |
| UAC-002 | FR-002, FR-003 | T0 + T1 | ApprovalSubmissionServiceTests (楽観成功) / reduce_approval_test (held reply 1 回) | pinned |
| UAC-003 | FR-002 | T0 | ApprovalSubmissionServiceTests (rollback 復元) | pinned |
| UAC-010 | FR-001 | T0 | WsFrameParserTests (qr) / PanelPresentationTests (ShowReply) | pinned |
| UAC-011 | FR-006 | T0 + T3-manual | QuestionSubmissionServiceTests (送信/復元) / EngageFocusServiceTests (返却・liveness)。IME 実挙動と実フォーカス返却は CI で安定観測できないため T3-manual | pinned (実挙動は T3-manual) |
| UAC-020 | FR-003 | T0 | SupervisionReducerTests (EvtApprovalResolved 除去 + notice) / SupervisionToastRouterTests (解決済み id の drop) | pinned |
| UAC-021 | FR-003 | T0 + T1 | SupervisionReducerTests (EvtApprovalResolvedByOther) / reduce_approval_test (tombstone, resolved_by_other) | pinned |
| UAC-030 | FR-004 | T0 | SupervisionToastRouterTests (1 pending 1 回) | pinned |
| UAC-031 | FR-003, FR-004 | T0 + T3-manual | ToastXml 純関数 (ボタン引数)。OnNotificationInvoked → SubmitApprovalAsync の実結線は WinUI 層のため T3-manual (実機トースト操作) | **補強候補外・T3-manual 明示** |
| UAC-032 | FR-004 | T0 | ToastDecisionServiceTests (watched 抑制) / PanelWatchedPredicateTests | pinned |
| UAC-040 | FR-007 | T0 | IslandStateMachineTests (AutoPending のみ自動収納 / ユーザー展開維持) | pinned |
| UAC-041 | FR-005 | T0 + T1 | reduce_approval_test (TTL default decision) / SupervisionReducerTests (resolved 除去) | pinned |
| UAC-042 | FR-007 | T0 + T3 | PanelPresentationTests (空 sections) / PanelWindowUiE2ETests (Empty_sections_do_not_exist_in_the_tree) | pinned |
| UAC-043 | FR-008 | T1 | hello フレームが pending を含まないこと・既解決要求が再配信されないことの明示 assert | **補強** (unit 3: gateway scenario へ追加) |

## 補強対象 (unit の acceptance に対応)

- **unit 3 (gateway)**: UAC-043 の「hello 非 replay / 既解決要求の非再配信」を gateway scenario test へ明示 assert として追加する。first-writer-wins / TTL / teardown connection-lost は既存 assert の存在を確認し、欠けていれば同時に補強する。
- **unit 2 (island)**: witness-frame-flood-adversarial (未知 k フレーム無視・重複 ar の単一カード・連続 v での非重複トースト) のうち既存テストに無い組み合わせを追加する。
- **T3-manual の記録**: UAC-011 (IME + フォーカス返却の実挙動) と UAC-031 (実機トースト操作) は、リリース前チェックリストとして本 member に手動確認結果を記録する (偽の緑を作る fake unit test は書かない — F2/no-fake-green)。

## Closure に必要な evidence

change close 時、本 member に以下を記録する: T0/T1 の実行結果 (コマンドと green の証跡)、補強テストの追加先と assert 内容、T3-manual 2 項の手動確認結果。
