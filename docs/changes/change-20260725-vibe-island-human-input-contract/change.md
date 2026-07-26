---
id: change-20260725-vibe-island-human-input-contract
kind: change
title: Vibe Island human-input (approval/question) experience contract
status: done
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
promotion:
- target: none
  section: none
  action: none
  item: {}
  reason: 本 change は production code を変更しない as-is 契約 pin であり、新しい責務・boundary・invariant・capability
    を production design へ導入しない。恒久的な意思決定は accepted 済みの adr-20260725-vibe-island-asis-contract-pin
    と adr-20260725-vibe-island-verification-tiers が所有し、契約の正本は change package の design-plan.json
    と verification.md に閉じる (上流 design plan も promotion を宣言していない)。
evidence_refs:
- type: command
  ref: dotnet test clients/windows-shell/AgentGrid.Shell.Core.Tests
- type: command
  ref: dotnet test clients/windows-shell/AgentGrid.Shell.Platform.Tests
- type: command
  ref: cd src && go test ./host/state/...
- type: command
  ref: cd src && go test ./server/api/...
- type: command
  ref: make test-windows-shell-e2e
- type: contract
  ref: contract-human-input-roundtrip
- type: contract
  ref: contract-island-attention-observability
- type: contract
  ref: contract-pending-card-lifecycle
- type: test
  ref: src/server/api/human_input_contract_test.go
- type: test
  ref: src/host/state/reduce_approval_test.go
- type: test
  ref: clients/windows-shell/AgentGrid.Shell.Core.Tests/SupervisionState/ApprovalSubmissionServiceTests.cs
- type: test
  ref: clients/windows-shell/AgentGrid.Shell.Core.Tests/SupervisionState/SupervisionReducerTests.cs
- type: test
  ref: clients/windows-shell/AgentGrid.Shell.Core.Tests/SupervisionState/PanelPresentationTests.cs
- type: test
  ref: clients/windows-shell/AgentGrid.Shell.Core.Tests/GatewayClient/WsFrameParserTests.cs
- type: test
  ref: clients/windows-shell/AgentGrid.Shell.Platform.Tests/Toast/SupervisionToastRouterTests.cs
- type: source
  ref: docs/changes/change-20260725-vibe-island-human-input-contract/verification.md
unresolved_decisions: []
tags: []
owners: []
relations: []
source_paths: []
summary: 'Vibe Island の human-input (承認/質問) 体験を実装調査に基づく判別可能な as-is 契約として固定する requirements/ux
  成果物。相談済み: pending replay と選択肢質問は導入決定、Claude 一級化は design 検証先行。'
updated: '2026-07-26'
promotion_applied_at: '2026-07-26T01:31:48.950957+00:00'
closure:
  closed_at: '2026-07-26T01:31:51.354856+00:00'
  content_hash: sha256:f116a5c348775b562aff10e9b448212edc8cae622a76b562f0d0459a8150e58e
---

## Summary

Vibe Island の human-input (承認/質問) 体験を **production code 無変更**の as-is 契約として pin した。
実体は 3 つの implementation contract の witness を既存 T0/T1/T3 テスト資産へ帰着させ、
帰着先が無い witness にテストを追加したこと、および `verification.md` に
UAC × Tier × テスト対応表と、検証で判明した契約↔実装の乖離を materialize したこと。

- **contract-human-input-roundtrip** (T1): `reduce_approval_test.go` の held reply 検査を
  真偽値判定 → **shape 非依存の全件計数**へ強化 (「1 要求につき返信は正確に 1 回」を実質強制)。
  期限後応答の非適用・不正 decision の非受理を追加。gateway 側は `human_input_contract_test.go`
  を新設し、hello の human-input field 混入を **json tag に対する構造的検査**で禁止、
  409 resolved_by_other / 400 invalid_argument / ax の全購読者配信を明示 assert。
  再接続非 replay は gateway 層と reducer 層 (`TestSubscribeDoesNotReplayHumanInput`) の 2 層で pin。
- **contract-island-attention-observability** / **contract-pending-card-lifecycle** (T0):
  未知 k フレーム無視・重複 ar の単一カード・連続 v での非重複トースト・注視状態の交錯・
  楽観除去の HTTP 応答前タイミング・復元→再送→Handled elsewhere の順序正規化を追加。
  approval 送信の fake は POST method / `/approvals/` path / `decision` body を検査するため
  「ローカルで消すだけ」の実装を棄却できる。

## Closure Notes

- **outcome 1 (14 UAC の対応付け)**: 達成。`verification.md` の対応表で全 14 UAC が Tier と
  **実ソースから機械導出した**テストクラス/メソッド名、または T3-manual の明示理由を持つ。
- **outcome 2 (production code の diff ゼロ)**: 達成。変更は test 2 プロジェクト・Go test 2 ファイル・
  docs のみ。`adr-20260725-vibe-island-asis-contract-pin` の confirmation を満たす。
- **cross-task review 3 attempt (延べ 9 lens 実行) が実質的な欠陥を段階的に検出し、全件修正済み**:
  major 件数は 7 → 5 → 2 と収束し、round 3 では 3 lens 中 1 lens が approved を返した。
  全 findings をメインセッションが production code で裏取りしてから都度 1 commit で一括修正した。
  代表的なもの:
  - round 1: hello 非 replay の pin がテスト自身の入力に依存して vacuous だったため json tag への
    構造的検査へ変更。「held reply ちょうど 1 回」の計数が Decision/Error で絞られており別 shape の
    二重返信を見逃していたため全件計数へ変更。approval 送信の fake がリクエストを検査せず
    UAC-002 の counterexample が生存していたため method/path/body 検査を追加。再接続非 replay が
    自作 fake の層しか見ていなかったため reducer 層の pin を追加。恒真だった `FocusTransferGuard`
    テストを撤去 (理由をコード内に記録)。
  - round 2: Go reducer の 2 箇所で `Reduce` の戻り state を捨てて変更前の state を読み直しており、
    片方は値を自分自身と比較する恒真アサートになっていた (戻り state を束縛して修正)。
    `TestSubscribeDoesNotReplayHumanInput` が subscribe 応答 body を素通ししていたため
    `EffSendResponse`/`EffReplyHeldQuestion` も禁止対象に追加。hello 禁止判定を完全一致から
    部分一致 + 型到達判定へ強化 (camelCase の取りこぼしを解消)。UAC-001 が実在しないフォーカス観測を
    引用していたため削除。UAC-020 で「Handled elsewhere 通知」が本 UAC の経路では出ないことを発見。
  - round 3: `proto.ApprovalWire`/`QuestionWire` の json tag 集合を Go 側で pin するテストがどこにも
    無く、Go↔C# の wire key 契約が未 pin だった (`TestApprovalWireCarriesIslandRequiredKeys` /
    `TestQuestionWireCarriesIslandRequiredKeys` を追加)。`TestApprovalLifecycleTeardownReap` が
    shape 絞り込みカウンタのままで、round 1/2 で他 2 テストから排除したのと同じパターンが 1 箇所
    残っていた (shape 非依存の全件計数へ修正)。
- **契約と実装の乖離を 4 件記録** (`verification.md` の D-1〜D-4、production 変更が必要なため本 change では未修正):
  - **D-1**: 宣言 invariant「同一 pending のトーストは高々 1 回」は offline-rollback 経路で成立しない
    (`_seen` がキューを離れた id を忘れ、楽観除去→復元の 2 snapshot で再発火する)。as-is 挙動を
    `Rollback_restore_while_unwatched_refires_toast_as_is` で pin した。**後続 change の修正対象**。
  - **D-2**: `FocusTransferGuard` は「production Win32 adapter が使う」と文書化されているが
    呼び出し箇所が 0 件で、NFR-002 のフォーカス非奪取は機械強制されていない。
  - **D-3**: `TestPendingHumanInputPlaceholderIsEmpty` は REST replay 経路の不在を pin する
    change detector であり FR-008 の証明ではない (replay 実装時に意図的に赤くなる)。
  - **D-4**: UAC-020 (「他面が先に解決 → Handled elsewhere 通知」) は as-is では成立しない —
    `EvtApprovalResolved` (ax 経路) はカードを除去するだけで通知を作らない。通知を作る
    `EvtApprovalResolvedByOther` は島自身の送信が 409 を受けた UAC-021 の経路でのみ発行される。
    UX 上どちらが正しいか (他面解決も通知すべきか) は後続 change の判断対象。
- **UAC-011 / UAC-031 の evidence を訂正**: どちらも観測に必要な結線が `AgentGrid.Shell.WinUI`
  (どのテストプロジェクトも参照していない) にあるため、**全面 T3-manual** として構造的理由を記録した。
  当初 UAC-011 は「CI で安定観測できない」を理由に pinned としていたが、実際は構造的な gap である。
  同じ理由で UAC-001 (フォーカス非奪取)・UAC-010 (自動フォーカス非発生)・UAC-021 (競合モーダル非表示) も
  該当半分を T3-manual へ限定した。
- **未クローズの持ち越し**: T3-manual **4 項** (UAC-011 の結線、UAC-031 の実機トースト操作、
  UAC-001/UAC-010/NFR-002 のフォーカス非奪取、UAC-042 の drain 遷移) は `verification.md` の
  リリース前チェックリストに **未実施**として明示保持する (偽の緑を作る fake unit test は書かない)。
- **本 change 起因でない既存失敗**: `cd src && go test ./...` は `uihost` (実行環境で閉じたポートへの
  `127.0.0.1` 接続が RST ではなく i/o timeout になることが原因) が決定的に赤い。`host/runtime` の
  `TestTerminalRelay_SeversSlowSubscriberWithoutBlockingOthers` はスケジューリング依存の flaky と
  実測確認済み (base で失敗・別実行で成功)。いずれも base commit で同一に再現し、本 change のスコープ外
  (別途 issue 起票が必要)。
- **後続 change へ**: `DP-PENDING-REPLAY` / `DP-QUESTION-OPTIONS` / `DP-CLAUDE-FIRST-CLASS` は
  本 change のスコープ外 (design-plan の `out_of_scope`)。D-1 のトースト二重発火・D-2 の
  未結線ガード・D-4 の UX 判断も production 変更を要するため後続で扱う。


{% transition from="ready" to="active" date="2026-07-26" %}
plan-delivery: delivery workflow 着手
{% /transition %}


{% transition from="active" to="closing" date="2026-07-26" %}
plan-delivery: 全 task done / 機械 gate approved / cross-task review 3 ラウンド分の findings を全件修正 (round3 は 3 lens 中 1 lens approved)
{% /transition %}
