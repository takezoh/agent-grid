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

WSL から T0 を実行する場合、Linux .NET SDK は存在しないため `clients/windows-shell/scripts/e2e.sh` と同じ
robocopy 経由の Windows dotnet を使う (`clients/windows-shell/scripts/win-test.ps1` も同経路)。

## UAC × Tier × テスト対応表

テスト参照は `クラス名.メソッド名` 形式で、**クラス帰属は実ソースから機械的に導出済み** (`--filter FullyQualifiedName~<クラス>.<メソッド>` で解決できる)。

Go↔C# の wire key 契約は `TestApprovalWireCarriesIslandRequiredKeys` / `TestQuestionWireCarriesIslandRequiredKeys` **(new)** が pin する — `proto.ApprovalWire`/`QuestionWire` の json tag 集合を、島側 `WsFrameParser.cs` が実際に読む key 名の集合と reflection で突合する。以前は `WsFrameParserTests` が手書き JSON リテラルを パースするだけで、Go 側で `session_id` 等をリネームしても全テストが緑のまま島の表示が壊れる経路が閉じていなかった (対応表 UAC-001/002/010/021/041 の暗黙依存)。
状態: **pinned** = 対応テストが assert 済み / **T3-manual** = 自動化せず手動確認と理由を記録 / **partial** = 一部の観測のみ always-on。
本 change で追加した assert は **(new)** を付す。

| UAC | 要件 | Tier | テスト参照 | 状態 |
|---|---|---|---|---|
| UAC-001 | FR-001 | T0 + **T3-manual (フォーカス)** | `IslandStateMachineTests.Pending_arrival_auto_expands_a_compact_island` / `.New_pending_reexpands_even_when_collapsed_mid_queue` / `WsFrameParserTests.Parses_approval_requested_fixture` (ar) / `SupervisionReducerTests.Approval_requested_adds_to_queue` / `PanelPresentationTests.Compact_summary_shows_pending_before_session_phases` (バッジ)。**フォーカス非奪取を観測する自動テストはどの Tier にも存在しない** — `PanelWindowUiE2ETests` の 5 fact は AutomationId chrome / StatusText / 空セクション非存在 / 行幅 / 収納展開のみで、フォーカスも `WS_EX_NOACTIVATE` も assert していない (D-2 も参照)。手動チェックリストのみ | 到着表示は pinned / **フォーカスは T3-manual (自動観測なし)** |
| UAC-002 | FR-002, FR-003 | T0 + T1 | `SupervisionReducerTests.Optimistic_submit_removes_item` / `ApprovalSubmissionServiceTests.Submit_removes_card_before_http_response_completes` **(new、前提 assert + 発行回数 assert 付き)** / `.Success_leaves_queue_empty` **(new: 発行回数 assert)** / `reduce_approval_test.go: TestApprovalSingleWriterFirstCommit` (held reply を **shape 非依存で全件計数**して 1 回 **(new)**)。「ローカルで消すだけ」の counterexample を棄却するのは **fake の `Requests` カウンタ (POST が実際に発行されたかの assert)** **(new)** である。`AssertApprovalPost` (method / `/approvals/` path / `decision` body) は**発行された**リクエストの形を検査するだけで、1 本も発行しない実装には原理的に発火しない。なお UAC-002 の第 2 観測「セッションが実行中表示に戻る」に対応する always-on テストは無い | pinned (第 2 観測は未カバー) |
| UAC-003 | FR-002 | T0 | `ApprovalSubmissionServiceTests.Network_error_rolls_back` / `SupervisionReducerTests.Submit_failed_rolls_back` / `ApprovalSubmissionServiceTests.Network_error_restores_card_and_connection_failure_surfaces_offline` **(new)** (復元 + オフライン可視化) | pinned |
| UAC-010 | FR-001 | T0 + T3-manual | `WsFrameParserTests.Parses_question_requested` (qr) / `SupervisionReducerTests.Question_request_and_resolve` / `PanelPresentationTests.Question_shows_pending_with_reply_entry` (返信欄) / `.Approval_shows_pending_without_reply_entry` (種別差)。**「返信欄へ自動フォーカスしない」は自動テストで判別できない** — 上記は item 種別による返信欄の有無を assert するだけで、フォーカスは観測していない (UAC-001 / D-2 と同じ構造的 gap) | 表示は pinned / **自動フォーカス非発生は T3-manual** |
| UAC-011 | FR-006 | **T3-manual (結線は全面)** | 隣接部品のみ T0: `QuestionSubmissionServiceTests.Submit_question_success_removes_item` / `.Submit_question_network_error_restores_item` (カードがキューから消えること) / `EngageFocusServiceTests.Restore_live_when_target_still_exists` / `.Restore_denied_when_os_refuses` / `.Target_destroyed_does_not_activate_shell` / `EngageAttachThreadInputTests.Restore_uses_AttachThreadInput_when_threads_differ` (`EngageFocusService` を fake Win32 で単体検証)。**FR-006 の 2 つの観測 (返信欄クリア・フォーカス返却) を生む結線は `AgentGrid.Shell.WinUI/Panel/PanelWindow.xaml.cs:348` にあり、どのテストプロジェクトも WinUI を参照していない** — UAC-031 と構造的に同じ gap。「送信完了 → クリア → 返却」の連鎖は always-on では観測できず、手動確認のみ | T3-manual (構造的理由を記録) |
| UAC-020 | FR-003 | T0 + T1 | カード除去: `SupervisionReducerTests.Approval_resolved_removes_item_without_notice` **(new)** / `TestGatewayLifecycle_ApprovalRequestedAndResolvedFrames` **(new)** (ax が全購読者へ配信)。**「Handled elsewhere 通知」は本 UAC の経路では出ない — 下記 D-4 の乖離**。`Resolved_by_other_shows_notice_and_removes_item` は島が自分で送信して 409 を受けた UAC-021 の経路であり本行の evidence にはならない。トースト非発火は間接 (解決済み id が snapshot に存在しない構造の帰結) | **partial** (D-4 の乖離あり) |
| UAC-021 | FR-003 | T0 + T1 | `ApprovalSubmissionServiceTests.Conflict_shows_resolved_by_other` **(new: notice.Kind=="already-handled" と `KindLabel`=="Handled elsewhere" を追加)** — 公開 API `SubmitApprovalAsync` 経由で 409→notice の全鎖を通す唯一のテスト / `SupervisionReducerTests.Resolved_by_other_shows_notice_and_removes_item` / `.Duplicate_resolved_by_other_keeps_a_single_notice` **(new)** / `reduce_approval_test.go: TestApprovalSingleWriterFirstCommit` (tombstone・後着の held reply なし・decision 非上書き — 戻り state を束縛して assert **(new)**) / `TestMux_ApprovalRespondFirstWriterWinsHTTP` **(new)** (後着が 5xx でないことを先に assert し、次に 409 を assert)。**counterexample の「競合をエラーモーダルで提示する」半分は WinUI 層でのみ観測可能** (UAC-001/UAC-010 と同じ構造的 gap) で自動テストは無い | pinned (モーダル非表示の半分は T3-manual) |
| UAC-030 | FR-004 | T0 | `SupervisionToastRouterTests.Fires_once_per_new_approval_when_unwatched` / `PanelWatchedPredicateTests.Flyout_closed_fires_toast` / `SupervisionToastRouterTests.Flyout_open_close_after_fire_does_not_retoast` **(new)** / `ToastDecisionServiceTests.DecideForNewPending_uses_item_kind` (種別に応じたボタン判定)。**下記「契約と実装の乖離」D-1 参照** — 「高々 1 回」は queue 連続在籍中に限って成立する | **partial** (D-1 の乖離あり) |
| UAC-031 | FR-003, FR-004 | **T3-manual (全面)** | **T0 参照なし。** `ToastXml.ApprovalToastXml` は `AgentGrid.Shell.WinUI` 内の public static だが、`AgentGrid.Shell.WinUI` を参照するテストプロジェクトは存在せず (Core.Tests / Platform.Tests は WinUI に依存しない)、`AppNotificationToastService.OnNotificationInvoked → ShellFleet.SubmitApprovalAsync` の結線も WinUI 層にある。本 change は production code を変更しない ([adr-20260725-vibe-island-asis-contract-pin](../../adr/adr-20260725-vibe-island-asis-contract-pin.md)) ため、テストプロジェクト参照の追加も行わない。**実機トースト操作による手動確認**のみ | T3-manual (理由記録済み) |
| UAC-032 | FR-004 | T0 | `SupervisionToastRouterTests.Suppresses_when_panel_watched` / `PanelWatchedPredicateTests.Flyout_open_suppresses_toast` / `SupervisionToastRouterTests.Watched_arrival_then_unwatched_reapply_does_not_fire` **(new)** / `PanelWatchedPredicateTests.Dnd_suppresses_toast` / `.Locked_suppresses_toast` / `.Notifications_disallowed_suppresses` / `.Notification_query_fail_opens_allows_toast` (fail-open) | pinned |
| UAC-040 | FR-007 | T0 | `IslandStateMachineTests.Auto_expanded_island_collapses_when_queue_drains` (AutoPending のみ自動収納) / `.User_expanded_island_stays_open_when_queue_drains` (ユーザー展開は維持) / `.Pending_decrease_does_not_reexpand_a_compact_island` | pinned |
| UAC-041 | FR-005 | T0 + T1 | `reduce_approval_test.go: TestApprovalExpiryDecisionCaptureAtCreation` (作成時捕捉の default_decision で解決・held reply を shape 非依存で全件計数して 1 回・期限後応答は resolved_by_other で非適用 — いずれも **(new)** assert) / `TestApprovalExpiryTOCTOU` / `SupervisionReducerTests.Approval_resolved_removes_item_without_notice` **(new)** (期限解決はカード除去のみ) | pinned |
| UAC-042 | FR-007 | T0 + T3 | `PanelPresentationTests.Sections_render_nothing_when_there_is_nothing` / `.Sections_show_only_what_has_content` / `.Pending_drain_hides_approval_and_reply_sections` **(new — presentation model 上の bool 遷移のみ)** / `PanelWindowUiE2ETests.Empty_sections_do_not_exist_in_the_tree` (UIA ツリー上の要素非存在) / `.Status_line_reflects_live_snapshot_and_healthy_state_is_silent`。**「要素として生成されない」を T0 は判別できない** — production は当該 bool を `Visibility = Collapsed` として消費するため、T0 は「空 ListView を Collapsed で維持する実装」を棄却できない。**さらに T3 も pending→解決の遷移は観測していない** — run-dev には live agent が居らず pending が発生しないため、当該 e2e は「最初から空のパネル」を見ているだけ。witness-quiet-normal の forbidden outcome 「空 ListView や不可視ボタンの残存」(ax 解決の**後**) はどの Tier でも未観測で、手動チェックリストに送る | partial (drain 遷移は未カバー) |
| UAC-043 | FR-008 | T1 (gateway 層のみ) | `TestHelloFrameOmitsPendingHumanInput` **(new — `helloFrame` / `EvtSessionsChanged` の json tag に対する構造的検査。field 追加時点で fail する)** / `TestGatewayLifecycle_ReconnectDoesNotReplayResolvedApproval` **(new — gateway 層)** / `TestSubscribeDoesNotReplayHumanInput` **(new — reducer 層: pending + 既解決を保持した state で `EvCmdSubscribe` が human-input event を broadcast しないこと。effect 種別は `EffBroadcastEvent`/`EffReplyHeldApproval`/`EffReplyHeldQuestion`/`EffSendResponse(Sync).Body` を検査)** / `reduce_approval_test.go: TestApprovalLifecycleTeardownReap` (teardown 済み pending は registry から除去、held reply は shape 非依存で正確に 1 件 **(new)**)。`TestPendingHumanInputPlaceholderIsEmpty` **(new)** は REST placeholder の形の change detector であり FR-008 の pin には数えない (下記 D-3)。**島側の再接続層 (`SupervisionWsSession.cs` の再接続/backoff ループ) には対称な T0 pin が無い** — counterexample (「再接続時にイベント履歴から既解決要求を再構築して再表示する」) をこの層で実装しても、上記いずれの cited test も検出できない | pinned |

## 契約と実装の乖離 (as-is pin で判明した記録)

独立 cross-task review (3 attempt / 延べ 9 lens 実行) が検出し、production code を読んで裏取りした 4 件 (D-1〜D-4)。

as-is 契約 pin の目的は「実装と契約の差を可視化する」ことなので、検証で判明した乖離を隠さず記録する。
いずれも production code の変更が必要なため本 change では修正しない
([adr-20260725-vibe-island-asis-contract-pin](../../adr/adr-20260725-vibe-island-asis-contract-pin.md))。

### D-1: 「同一 pending のトーストは高々 1 回」が offline-rollback 経路で成立しない

`design-plan.json` の `contract-island-attention-observability.invariants` は
「同一 pending のトーストは高々 1 回」を宣言しているが、実装ではそうならない経路がある:

- `SupervisionToastRouter.OnSnapshotAsync` は末尾で `_seen.RemoveWhere(id => !live.Contains(id))` を実行し、
  キューを離れた id を忘れる。
- `ApprovalSubmissionService.SubmitApprovalAsync` はネットワーク失敗時に
  `EvtApprovalSubmitRequested` (楽観除去) → `EvtApprovalSubmitFailed` (復元) の **2 回**
  `SnapshotChanged` を発火し、`AppNotificationToastService` が両方を router へ転送する。
- 結果、パネル非注視時 (例: トーストの Approve で応答した UAC-031 の経路) に
  同一 `ApprovalId` が `_seen` から落ちて再登録され、**2 通目のトーストが発火する**。

as-is 挙動は `SupervisionToastRouterTests.Rollback_restore_while_unwatched_refires_toast_as_is` **(new)**
が pin している (router の `_seen.RemoveWhere` を実際に通す 3 snapshot 列)。
ただし**連鎖の前半 — `SubmitApprovalAsync` が失敗時に `SnapshotChanged` を 2 回上げ、
`AppNotificationToastService` が両方を転送すること — は assert していない** (テストは 3 snapshot を
直接与えている)。producer 側が将来 1 回に束ねると、production では D-1 が消えるのにこのテストは
緑のまま残る。修正には production 変更 (`_seen` を「id が *解決* されるまで保持する」への変更) が
必要で、後続 change の検討対象。

### D-2: `FocusTransferGuard` は production から呼ばれていない

`FocusTransferGuard` は 「Runtime guard used by production Win32 adapter」と文書化されているが、
`EnsureAllowed` / `IsAllowed` の production 呼び出し箇所は **0 件**である
(`JumpBackService` / `EngageFocusService` は `_win32.SetForegroundWindow` を直接呼ぶ)。
したがって「許可された caller だけがフォーカスを移す」という NFR-002 の不変条件は
機械的に強制されていない。

この不変条件について T0 テストを追加することは**意図的に見送った** — enum 3 値と
`Allowed` 3 値を突き合わせる assert は production 定数の言い換えであり、
ガードを迂回する経路 (直接 `SetForegroundWindow` 呼び出し) を検出できないため、
`adr-20260725-vibe-island-verification-tiers` の偽の緑禁止に反する。
判断の記録は `FocusTransferGuardTests.cs` のコメントに残した。フォーカス非奪取は T3 観測のみ。

### D-3: 再接続 replay の観測範囲

`TestPendingHumanInputPlaceholderIsEmpty` が pin するのは「REST replay 経路がまだ存在しない」ことであり、
FR-008 (既解決要求の非再提示) の証明ではない。**未解決** pending の replay は導入決定済み
(`DP-PENDING-REPLAY`、design-plan の `out_of_scope`) なので、その実装時にこのテストは意図的に赤くなる
— 契約を明示的に書き換えさせるための change detector である。
FR-008 本体の pin は gateway 層 (`TestGatewayLifecycle_ReconnectDoesNotReplayResolvedApproval`) と
reducer 層 (`TestSubscribeDoesNotReplayHumanInput`) の 2 層が担う。

### D-4: UAC-020 の「Handled elsewhere 通知」は他面解決の経路では出ない

`ux.md` の UAC-020 は「同一承認が島とブラウザに表示中 → ブラウザが先に許可する →
島のカードが消え **Handled elsewhere 通知が表示され**、トーストは発火しない」を要求する。
実装ではこの経路で通知は出ない:

- 他面が解決すると島には `ax` フレームが届き、`WsFrameParser` → `EvtApprovalResolved` →
  `SupervisionReducer` はカードを**除去するだけ**で notice を作らない。
- 「Handled elsewhere」notice を作る `EvtApprovalResolvedByOther` は
  `ApprovalSubmissionService` が **島自身の送信に 409 が返ったとき**だけ発行する (UAC-021 の経路)。

したがって UAC-020 の第 2 観測は as-is では成立せず、`SupervisionReducerTests
.Approval_resolved_removes_item_without_notice` **(new)** が「notice 無しで除去される」という
実際の挙動を pin している。production 変更が必要なため本 change では修正しない。
UX 上どちらが正しいか (他面解決も通知すべきか、静かに消えるべきか) は後続 change の判断対象。

## 契約横断の補強 (adversarial witness)

| witness | 帰着先 |
|---|---|
| witness-frame-flood-adversarial | `WsFrameParserTests.Ignores_unknown_k_frame_without_throwing` **(new)** / `.Ignores_output_array_and_unknown` (未知フレーム無視) / `SupervisionReducerTests.Duplicate_approval_request_keeps_a_single_card` **(new)** (同一 id の重複 ar) / `SupervisionToastRouterTests.Session_churn_snapshots_do_not_retoast_seen_approval` **(new)** (連続 v で非重複トースト) / `.Flyout_open_close_after_fire_does_not_retoast` **(new)** (注視状態の交錯) |
| witness-lifecycle-adversarial | `ApprovalSubmissionServiceTests.Rollback_then_resubmit_conflict_normalizes_to_handled_elsewhere` **(new)** (復元 → 再送 → Handled elsewhere の順序正規化) / `EngageFocusServiceTests.Target_destroyed_does_not_activate_shell` (破棄済み HWND へ返却しない) / `SupervisionReducerTests.Duplicate_resolved_by_other_keeps_a_single_notice` **(new)** (カードは高々 1 枚) |
| witness-approve-roundtrip-normal (wire 形) | `TestWireEncode_ApprovalFramesArAx` **(new)** — ar/ax フレームの `k` 判別子を pin する。id/status/decision の payload は `TestGatewayLifecycle_ApprovalRequestedAndResolvedFrames` が担当 |
| witness-race-adversarial | `reduce_approval_test.go: TestApprovalSingleWriterFirstCommit` / `TestApprovalRespondInvalidDecisionRejected` **(new)** (不正 decision は pending 不変・held reply なし) / `TestApprovalLifecycleTeardownReap` / `TestMux_ApprovalRespondInvalidDecision400` **(new)** (gateway 段で 400・daemon RPC を出さない) / `TestGatewayLifecycle_ReconnectDoesNotReplayResolvedApproval` **(new)** / `TestSubscribeDoesNotReplayHumanInput` **(new)** |

## T3-manual の記録 (リリース前チェックリスト)

自動化せず手動確認する項目。偽の緑を作る fake unit test は書かない (F2/no-fake-green)。

| 項目 | UAC | 確認手順 | 結果 |
|---|---|---|---|
| IME 入力 + 返信欄クリア + フォーカス返却の結線 | UAC-011 | 別アプリで入力中に質問が到着 → 返信欄へ IME 入力 → Enter → 返信欄クリアと直前アプリへのキャレット復帰を目視 | 未実施 (次回リリース前) |
| 実機トースト操作 | UAC-031 | パネル非注視で承認到着 → トーストの Approve を押下 → 島のカードが消え、エージェントが再開することを目視 | 未実施 (次回リリース前) |
| フォーカス非奪取 (到着・表示・解決) | UAC-001, UAC-010, NFR-002 | pending 到着中に別アプリで入力を続け、キャレットが移動しないことを目視。質問到着時に返信欄へ自動フォーカスしないことも確認 (D-2 によりガードは機械強制されておらず、自動テストも存在しない) | 未実施 (次回リリース前) |
| 解決後の要素非残存 (drain 遷移) | UAC-042 | pending を 1 件出してから解決し、承認/質問セクションが UIA ツリーから**消える**ことを inspect で確認 (T3 e2e は live agent が居らず pending が発生しないため、この遷移を観測していない) | 未実施 (次回リリース前) |

`make test-windows-shell-e2e` (T3) は常時実行されない ([adr-20260725-vibe-island-verification-tiers](../../adr/adr-20260725-vibe-island-verification-tiers.md) の negative consequence)。UIA でのみ観測できる退行 (要素残存・フォーカス奪取) の検出は本コマンドの実行時に限られる。

## Closure に必要な evidence

- T0/T1 の実行結果 (コマンドと green の証跡) → 下記「実行証跡」
- 補強テストの追加先と assert 内容 → 上記対応表の **(new)** 行
- T3-manual 3 項の手動確認結果 → 上記チェックリスト (未実施として明示保持)
- 契約と実装の乖離 → 上記 D-1 / D-2 / D-3 / D-4

### 実行証跡

2026-07-26、統合ブランチ `delivery/vibe-island-human-input-contract` で実行した結果。

| verification id | Tier | コマンド | 結果 |
|---|---|---|---|
| verify-island-core-t0 / verify-lifecycle-core-t0 | T0 | `dotnet test clients/windows-shell/AgentGrid.Shell.Core.Tests` | **171 passed / 0 failed / 4 skipped** (skip は `AG_E2E_RUN_DEV` 未設定時の `RunDevGatewayE2ETests`) |
| verify-toast-router-t0 / verify-engage-focus-t0 | T0 | `dotnet test clients/windows-shell/AgentGrid.Shell.Platform.Tests` | **27 passed / 0 failed / 0 skipped** |
| verify-state-reducer | T1 | `cd src && go test ./host/state/...` | **ok** (`host/state`, `host/state/view`) |
| verify-gateway-scenario | T1 | `cd src && go test ./server/api/...` | **ok** (`server/api`) |
| verify-uia-e2e | T3 | `make test-windows-shell-e2e` | **PASS (exit 0)** — Platform 27 passed / Core **175 passed skipped 0** (`AG_E2E_RUN_DEV=1` で e2e fact も実行) / WinUI self-contained build 0 Error(s) + launch smoke PASS / FlaUI UIA **5 passed** |

repo 全体 gate (`make lint` / `make build-all`) も rc=0。
`cd src && go test ./...` は 2 package (`host/runtime`, `uihost`) が赤だが、
**base commit `edb1b526` で同一の失敗集合が再現する既存条件**であり本 change の退行ではない
(`uihost` 側はこの実行環境で閉じたポートへの `127.0.0.1` 接続が RST ではなく i/o timeout になるため
dial-refused → 503 の写像に到達しないもの)。

production code の diff はゼロ ([adr-20260725-vibe-island-asis-contract-pin](../../adr/adr-20260725-vibe-island-asis-contract-pin.md) の confirmation を満たす)。

**実行環境の注記** (退行ではない環境要因):

- WSL に Linux .NET SDK が無いため、T0 は `e2e.sh` / `win-test.ps1` と同じ robocopy 経由の Windows dotnet で実行した。
- `make test-windows-shell-e2e` は worktree パスが深いと daemon の unix socket パスが `sun_path` 上限
  (108 バイト) を超えて `bind: invalid argument` で fixture 起動に失敗する。短いパスの worktree
  から実行して PASS を確認した。
- 閉じたポートへの `127.0.0.1` 接続がタイムアウトするため、`e2e.sh` の gateway probe が
  1 回あたり約 2 分ブロックする (機能上の問題ではない)。
