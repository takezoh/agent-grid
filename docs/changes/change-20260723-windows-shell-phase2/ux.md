---
change: change-20260723-windows-shell-phase2
role: ux
id: ux-20260723-windows-shell-phase2
kind: ux
title: 20260723 Windows Shell Phase 2 UX
status: draft
created: '2026-07-23'
summary: Explore-mode UX plan (revision explore-r3) for the Windows Shell (WinUI3) + Workspace (Electron) + WSL daemon supervision surface (Phase 2, S1-S5). Primary alternative = ALT-PANEL-PRIMARY-DUAL-SURFACE (user confirmed); ALT-TOAST-PRIMARY-ANCHOR-WORKSPACE retained as deferred material for design/impl review. Eight decision points answered via user consultation 2026-07-24 (rounds r1-r3, including DP-PANEL-ENGAGE-FOCUS-RETURN promoted to answered in r3); two remain defaulted (DP-JUMP-BACK-TARGET-PROVENANCE handoff, DP-HOSTED-MODE-FRAME-INTEGRATION default).
change_id: change-20260723-windows-shell-phase2
technology_candidates:
- id: candidate-winui3-windows-app-sdk
  candidate: WinUI 3 / Windows App SDK
  source: repository_context
  proposed_for:
  - F-001
  - F-002
  - F-003
  - F-004
  - F-101
  - F-102
  - F-108
  required_capabilities:
  - 常時可視 top-level 非アクティベート window (panel-primary の上端バー)
  - トレイアイコン + フライアウトの非アクティベート展開
  - AppNotificationManager によるボタン付き toast と inline textbox
  - unpackaged 構成での COM background activation
  - Windows Composition API による滑らかな展開アニメーション
  ux_constraints_relevant_to_evaluation:
  - panel が focus を奪わない (glance-primary の観察不変性)
  - toast 応答が 5s 級で完了する (threshold-toast-response-latency)
  - panel 展開 150ms 級 (threshold-engage-expand-latency)
  - テーマ (dark/light) 追従
  design_questions:
  - unpackaged 構成での COM 背景アクティベーションの実測遅延はどれくらいか (assumption-com-background-activation-unpackaged、S3 実装直前 prototype)
  - 上端バーが exclusive fullscreen アプリと共存する際の副作用 (assumption-fullscreen-panel-coexistence)
  - AppNotification inline textbox は IME 変換を扱えるか (assumption-appnotification-textbox-ime、S3 実装直前 prototype)
  disqualifiers:
  - unpackaged で COM background activation が構造的に機能しない場合 → toast-primary lens 復帰余地が失われる
  - Windows 10 21H2 未満での API 非互換 (要 Windows App SDK 前提)
- id: candidate-h-notifyicon-winui
  candidate: H.NotifyIcon.WinUI (MIT)
  source: repository_context
  proposed_for:
  - F-002
  - F-105
  - F-108
  required_capabilities:
  - トレイアイコンの状態別外観切り替え (色/形状)
  - アクセシブルネームによる assistive technology 読み上げ
  - フライアウト展開時の非アクティベート維持
  ux_constraints_relevant_to_evaluation:
  - アイコン外観変化だけで daemon Healthy/Degraded/Spawning を弁別可能 (DP-DAEMON-HEALTH-VISIBILITY=OPT-TRAY-ONLY 確定下では primary observable)
  - ホバー時のツールチップで状態テキストを読める
  design_questions:
  - 既存 tray アイコンパターンと整合するアクセシブルネームの命名規則
  - アイコン色/形状のパレット (theme 追従 vs 固定)
  disqualifiers:
  - .NET 依存が Shell 全体のランタイム前提と矛盾する場合
- id: candidate-appnotification-inline-textbox
  candidate: Windows App SDK AppNotificationManager (ボタン + inline textbox 付き toast)
  source: repository_context
  proposed_for:
  - F-007
  - F-101
  - F-102
  - F-103
  - DP-SUPERVISION-PRIMARY-ENTRY
  required_capabilities:
  - ボタン付き toast (バックグラウンド activation で foreground を奪わない)
  - inline textbox (短文回答)
  - Action Center 落ち後もボタンが有効
  - supervision イベントごとの一意な toast identity (再送しても重複しない)
  ux_constraints_relevant_to_evaluation:
  - toast 応答が 5s 級 (threshold-toast-response-latency)
  - Action Center に落ちても同一結果 (F-101 UAC-102)
  - supervision 目的以外の toast が発行されない invariant (goal-supervision-toast-budget)
  design_questions:
  - inline textbox の文字数上限は?
  - IME 変換対応の実挙動 (assumption-appnotification-textbox-ime、S3 実装直前 prototype)
  - DND / Focus Assist 中の挙動と fallback
  disqualifiers:
  - Windows DND 中に toast 到達を意図的にブロックする OS 側動作 (取りこぼしは product_assumption として許容範囲)
- id: candidate-composition-api-acrylic
  candidate: Windows Composition API + Acrylic 素材
  source: repository_context
  proposed_for:
  - F-002
  - F-004
  required_capabilities:
  - 60fps のパネル展開アニメーション
  - Acrylic (半透過素材) による背面透過
  ux_constraints_relevant_to_evaluation:
  - threshold-engage-expand-latency (150ms 級)
  - threshold-panel-animation-framerate (60fps 目標)
  design_questions:
  - 低スペック GPU 環境での 60fps 維持可否
  - Acrylic 有効時のバッテリー影響
  disqualifiers:
  - Windows 10 一部バージョンでの Acrylic 制限
- id: candidate-electron-workspace-host
  candidate: Electron (electron-builder dir target)
  source: repository_context
  proposed_for:
  - F-006
  - F-104
  - F-107
  required_capabilities:
  - 1 窓 1 セッションの BrowserWindow 生成/再利用
  - window-registry (session id → BrowserWindow の唯一生成点)
  - close 時に session を停止しない (窓 close ≠ session end)
  - close-re-open 時の scroll/pane 状態復元
  ux_constraints_relevant_to_evaluation:
  - goal-workspace-deep-dive (窓数が増えない)
  - goal-workspace-window-anchor (close ≠ end)
  - hosted mode の脱ブラウザ化 (goal-hosted-mode-no-browser)
  design_questions:
  - hosted mode 脱ブラウザ化の段階 (DP-HOSTED-MODE-FRAME-INTEGRATION=OPT-INCREMENTAL-DEFRAME defaulted)
  - window state persistence 忠実度 (assumption-workspace-window-restore-fidelity)
  disqualifiers:
  - electron-builder dir target で auto-updater が要求される場合 (Phase 2 非スコープ)
- id: candidate-named-pipe-jsonlines-ipc
  candidate: Node 標準 net (named pipe) + JSON Lines
  source: repository_context
  proposed_for:
  - F-006
  - F-104
  required_capabilities:
  - Shell↔Workspace の低レイテンシ双方向制御チャネル
  - openSession/focus/lifecycle 制御 (ドメインデータは流さない)
  ux_constraints_relevant_to_evaluation:
  - Workspace 窓オープンの体感遅延 (パネル [Open] から窓 activate まで)
  - 重複窓を生まない (window-registry と協調)
  design_questions:
  - Shell と Workspace の起動順序 (Shell 常駐 → Workspace オンデマンド)
  disqualifiers:
  - Windows named pipe のセキュリティ属性が個人利用境界を越える場合 (単一ユーザー前提のため通常想定内)
design_handoffs:
- id: handoff-jump-back-target-provenance
  required_outcome: ジャンプバック先の外部窓を staged best-effort (HWND → プロセス+タイトルマッチ → 正直な失敗表示) で特定し、成功時は対象窓をフォアグラウンド化、失敗時は panel に『見つからない』を明示表示する。fabricated fallback (任意の窓へのフォーカス移動) を禁止する。
  design_obligations:
  - HWND キャッシュのライフサイクル (session 作成時登録、対象窓 close 時失効) を設計
  - プロセス名 + タイトルマッチのマッチング規則 (Windows Terminal / VS Code / UE / Blender など主要外部アプリごとに検証)
  - 失敗表示の文言・アクセシブルネーム (『ジャンプ先が見つかりません』相当) を確定
  - OPT-STAGED-BEST-EFFORT の各段の遷移条件を state machine として仕様化
  verification_obligations:
  - 対象アプリごとの特定成功率を実機検証 (Windows Terminal, VS Code, UE/Blender)
  - 失敗時に別窓へのフォーカス移動が発生しないことを observable に検証
  - 対象窓 close 後に [Jump back] を押した場合の Then が『見つからない明示表示』であることの scenario 検証 (F-005 UAC-005b)
  provenance:
    source: repository_evidence
    confidence: inferred
    evidence_refs:
    - plans/plan-20260723-windows-shell-design.md#35
    - draft-1.json DEC-4
    rationale: DP-JUMP-BACK-TARGET-PROVENANCE の defaulted + handoff effect に対応する design obligation。
  decision_refs:
  - DP-JUMP-BACK-TARGET-PROVENANCE
---

## Goal

Phase 2 (S1-S5) の Windows Shell (WinUI3) + Workspace (Electron) + WSL 内デーモン監督 UX の supervision surface を、ユーザーが選定した panel-primary lens (ALT-PANEL-PRIMARY-DUAL-SURFACE) を primary alternative としつつ、toast-primary lens (ALT-TOAST-PRIMARY-ANCHOR-WORKSPACE) を design/impl フェーズでの見直し材料として deferred で保持し、承認/質問/ジャンプバック/窓規律/デーモン監督+再起動耐性を Given/When/Then + counterexample として固定する。S3 実装直前に先行 prototype で 2 assumption (AppNotification IME、unpackaged COM background activation) を検証し、その結果次第で toast fallback ladder と toast-primary 復帰余地を再評価する。

Reference UX の姿勢: `Vibe Island (macOS notch ambient panel)` は panel-primary lens (確定 primary alternative) の中核リファレンスとして `modeled_on` に昇格した。`iOS/Android 通知アクションボタン` は panel-primary lens 下で toast を短文 approve/deny fallback のみに位置づけたため `rejected` (ただし toast-primary alternative が deferred のため復帰余地は保持)。`現行 clients/ui ブラウザタブ` は両 lens 共通に `rejected` (親計画 Desired outcome #2、ブラウザは local flow に現れない)。

## Target Users

作者本人 (単一 Windows マシン、WSL 内 agent-grid デーモンを個人利用する開発者)。配布・複数ユーザー・チーム運用・installer/署名/自動更新は Phase 2 の対象外。

## Confirmed Constraints

- **cc-single-user-personal**: 個人利用前提。単一ユーザー・単一 Windows マシン、WSL 内 agent-grid デーモン。配布・署名・自動更新・多ユーザーは Phase 2 の非スコープ。(source: 元要件、confidence: confirmed)
- **cc-shell-resident-workspace-ondemand**: Shell はログイン時常駐、Workspace はオンデマンド起動 (panel/通知/deep link から起動・アクティベート)。(source: plans/plan-20260723-windows-shell-design.md#0)
- **cc-no-browser-in-local-flow**: ブラウザアプリは local flow から完全に排除される (deep link 解決・セッション表示のいずれもブラウザを経由しない)。(source: plans/plan-20260723-native-clients.md#desired-outcome)
- **cc-daemon-linux-only**: デーモン (server) は Windows へ移植しない。WSL または Linux ホストで動作し、Windows 側は loopback/LAN 経由で接続する。
- **cc-duplicate-response-contract-layer**: 二重応答 (panel と toast、または Workspace と toast など) はクライアント側で防がない。契約層 (approval-contract の単回裁定・resolved-by-other) に委ねる。(source: plan-20260723-windows-shell-design.md#5)
- **cc-quit-vs-daemon-stop-separation**: Shell 終了 (明示 Quit) はデーモンを停止しない。デーモン停止は別メニュー項目として明示的に分離する。(source: plan-20260723-windows-shell-design.md#36)
- **cc-phase0-1-contract-dependency**: Phase 2 UX の supervision イベント (approval, question, resolved-by-other, notification payload, deep link) は Phase 0 の approval/question サーバ側ドメイン確定と Phase 1 の approval-contract.md / question-contract.md / notifications.schema.json / deep-links.schema.json / reconnect-contract.md 確定を前提とする。契約 shape 確定後に Given/When/Then を再照合する必要がある。

## Existing System Context

- **esc-browser-spa**: 現行 clients/ui (React SPA) はブラウザタブ上で複数セッションを表示し、cmd/uihost が go:embed 配信、/api・/ws をゲートウェイへリバースプロキシしている。hosted mode で 1 窓 1 セッションへ反転。
- **esc-adr-0025-backfill**: ADR-0025 の transcript REST backfill → WS tail 再接続経路が既存。F-008 (デーモン再起動耐性) の自動再接続はこの経路のネイティブクライアント向け適用を前提。
- **esc-approval-question-not-modeled**: approval/question は現時点で codex app-server 内部にしかモデル化されておらず、host/state・server/api 側には未モデル化。Phase 0 のサーバ側ドメイン発出が Phase 2 UX の観察イベントの前提。

## User Goals

| id | actor | context | desired_outcome | success_observation |
|----|-------|---------|-----------------|---------------------|
| goal-daemon-supervised | operator | Windows ログイン時 Shell 自動起動 | WSL 内デーモンの生存確認・採用・新規起動が手動なしで完了しその結果が supervision surface に反映される | トレイアイコン+supervision surface (panel-primary 確定下では上端バー) が Connected 相当へ到達 |
| goal-glance-supervision | operator | 別窓 (エディタ/ターミナル/UE/Blender 等) 注視中 | フォーカスや窓を切り替えずに session 群の状態と承認/質問キューを把握 | supervision surface (panel-primary 確定下では上端バー + counts-plus-latest glance、両 lens 共通でトレイアイコン外観) がクリック・フォーカス変更なしで最新集計を示す |
| goal-approval-round-trip | operator | agent が承認を要求 | Workspace 窓を開かずに supervision surface だけで承認/拒否を完結 | 対象キュー項目が supervision surface から消え、agent 側の実行が再開/停止 |
| goal-question-answer | operator | agent が質問を発行 | supervision surface 上の textbox (panel-primary 確定下では主に panel engage テキスト欄、toast fallback は短文 approve/deny のみ) で回答 | 確定後、質問キュー項目が supervision surface から消え agent が回答受理 |
| goal-engage-focus-return | operator | engage テキスト入力を終えた直後 | engage 前 foreground 外部窓へ手動なしで戻る | 確定/Esc 直後に engage 直前 foreground 窓がフォアグラウンド、追加操作不要 |
| goal-jump-back | operator | 承認/質問対応後、元の外部窓へ復帰 | supervision surface の [Jump back] 一操作で対象外部窓へ復帰、失敗時は正直に『見つからない』 | 対象外部窓がフォアグラウンド、または明示メッセージ |
| goal-workspace-deep-dive | operator | diff レビュー・長い出力・複数ペインの深掘り | supervision surface または deep link から重複窓を生まず Workspace へ | 同一 session を繰り返し開いても Workspace 窓数が増えず既存窓がフォーカス |
| goal-hosted-mode-no-browser | operator | Workspace を開く一連の操作 | 一度もブラウザアプリを経由せずネイティブ窓に到達 | セッションを開く操作が Electron 窓に完結、既定ブラウザ起動 event が発生しない |
| goal-workspace-window-anchor | operator | Workspace 窓の close 操作 | 窓を閉じても対応 session は agent 側で継続 | 窓 close 直後も overview 上で対応 session が running/waiting、'ended'/'terminated' に遷移しない |
| goal-daemon-restart-resilience | operator | デーモン更新後の手動 Restart | 実行中 session を失わず supervision surface と Workspace が自動再接続 | 再起動完了後、健全性表示が Connected へ戻り、再起動前と同じ session が両サーフェスで観測できる |
| goal-toast-fallback-recovery | operator | supervision surface (panel) を見ていなかった間に承認/質問発生 (panel-primary 確定下では『panel 非注視』= 別窓 foreground / 離席 / DND / ロック中) | 画面に戻ったとき見逃した要求に気づき直接応答 | Windows 通知センターまたは Toast 表示上に要求が残り、その affordance から応答完結 (長文/IME 制約時は inline-panel-link で panel へエスカレーション) |
| goal-supervision-toast-budget | operator | supervision 目的以外 (daemon health 等) のイベント発生 | supervision toast 予算が infra ノイズで浪費されない | Healthy 継続中の観察窓 (threshold-daemon-healthy-toast-budget-window、暫定 5min) 内に supervision 目的以外の Windows 通知が 0 件、状態はトレイアイコン外観のみで読み取れる (DP-DAEMON-HEALTH-VISIBILITY=OPT-TRAY-ONLY 確定) |

## Legacy Context

- **Source implementation**: 現行 clients/ui (React SPA、ブラウザタブで複数セッション表示)。cmd/uihost が go:embed で配信し、既定ブラウザで開く運用。
- **Inherited behaviors**: セッション状態一覧・承認/質問表示・diff/transcript レビューの表現要素は hosted mode の Workspace 窓へ継承。ADR-0025 transcript REST backfill → WS tail 再接続経路は F-008 の前提として継承。
- **Replaced behaviors**: ブラウザタブによる 1 タブ N セッション表示 → Workspace の 1 窓 1 セッションへ反転。ブラウザアドレスバー・タブバー・token 入力 UI・ブラウザショートカットを hosted mode で除去。deep link を browser owner に委ねる挙動 → Shell/Workspace の window-registry によるアクティベート。SPA 内 toast だけで済ませていた supervision 通知 → Shell/Workspace の常駐 supervision surface (panel/tray/AppNotification) へ引き上げ。

## UX Alternatives

**ALT-PANEL-PRIMARY-DUAL-SURFACE** (source: draft-1、disposition: selected、user confirmed)
- primary_entry: 上端フローティングバー + トレイフライアウト (常時可視、glance 既定)
- 構造次元: primary_entry=panel(常時可視) / toast=fallback (panel-unwatched-only) / workspace 開閉頻度=低頻度 (on-demand) / panel default visibility=always-visible-bar / panel glance content=counts-plus-latest / daemon health visibility=tray-only / toast fallback affordance=inline-panel-link / engage focus return=capture-restore / fullscreen 共存=panel 側で妥協 (副作用リスク、assumption-fullscreen-panel-coexistence)
- 要旨: 上端バー (counts-plus-latest glance) とトレイフライアウトが同一 supervision state を反映する常時可視面を持ち、承認/質問/ジャンプバックは panel から完結。Workspace は深掘り時のみオンデマンド (on-demand)、toast は panel 非注視時 (別窓 foreground / 離席 / DND / ロック中) に限って短文 approve/deny の fallback として発行。長文/IME 制約時は toast 内 inline-panel-link で panel へエスカレーション。engage モード確定後は engage 前 foreground 窓へ明示的にフォーカス返却。daemon 監督はトレイアイコン外観のみで表現し supervision toast 予算を保護。
- provenance: user_consultation / confirmed (consultation-20260724-windows-shell-phase2-r1)

**ALT-TOAST-PRIMARY-ANCHOR-WORKSPACE** (source: draft-2、disposition: deferred)
- primary_entry: AppNotification toast (ボタン + inline textbox、event 発生の都度ポップアップ)
- 構造次元: primary_entry=toast(event-driven) / toast=primary / workspace 開閉頻度=常時アンカー(session 生存中は開き続ける) / panel default visibility=summon-only / fallback ladder=toast→panel→workspace(3 段) / daemon health visibility=tray-only(toast 予算保護)
- 要旨: 承認/質問はボタン付き toast + inline textbox で単独完結を狙い、panel は summon-only の overview に格下げ。Workspace は session 生存中のアンカーとして常時開いておく (close ≠ session end)。daemon 監督イベントは toast 予算を消費せずトレイアイコン外観のみで表現。
- provenance: planner_proposal / provisional (deferred として design/impl 見直し材料に保持。復帰条件は assumption-com-background-activation-unpackaged / assumption-appnotification-textbox-ime の S3 実装直前 prototype 結果)

**Alternative Comparison (selected: ALT-PANEL-PRIMARY-DUAL-SURFACE / confirmed / user-approved 2026-07-24)**
- draft-1 → ALT-PANEL-PRIMARY-DUAL-SURFACE (selected, confirmed): ユーザー相談で primary_alternative として確定。panel-cluster default (always-visible-bar / counts-plus-latest / tray-only daemon health / on-demand workspace / panel-unwatched-only toast fallback / inline-panel-link affordance) と構造整合。plan-20260723-windows-shell-design.md §3.2 が上端バー+トレイフライアウトの詳細設計を提供、native-clients plan の Vibe Island カテゴリを実証根拠とする。
- draft-2 → ALT-TOAST-PRIMARY-ANCHOR-WORKSPACE (deferred): primary lens は panel-primary で確定したが、toast-primary lens を design/impl フェーズの見直し材料として deferred で保持することをユーザーが明示。assumption-com-background-activation-unpackaged / assumption-appnotification-textbox-ime の S3 実装直前 prototype 結果が toast-primary lens 復帰を示唆する余地を残す。toast-primary lens 中核 scenario (F-101/F-102/F-103/F-104/F-105/F-107/F-108) は削除せず deferred lens 対応として保持。

## Product Assumptions (validation-required)

- **assumption-appnotification-textbox-ime**: Windows App SDK AppNotificationManager の inline textbox が日本語 IME 変換を扱えるかは未検証。扱えない場合、長文/日本語回答は panel/Workspace fallback が必要。panel-primary lens 確定下では toast fallback は panel-unwatched-only の短文 approve/deny のみで IME 影響は軽微だが、ユーザーは S3 実装直前 prototype による先行検証を残すことを明示した。
  - validation: S3 実装直前に prototype で AppNotification inline textbox の日本語 IME 変換対応を実機検証する (先行検証)
  - invalidation_behavior: 対応不可なら F-102 UAC-103 の Given から『IME を使わない短文』条件を除去し、F-103 UAC-104b の Given を『日本語または IME 変換を要する回答』へ拡張。DP-TOAST-FALLBACK-AFFORDANCE=OPT-INLINE-PANEL-LINK の affordance 文言・呼び出し頻度も再検討する
  - decision_refs: DP-TOAST-FALLBACK-CONDITION, DP-TOAST-FALLBACK-AFFORDANCE
- **assumption-com-background-activation-unpackaged**: unpackaged 構成での AppNotification COM background activation が toast ボタン押下から Shell 側処理起動までを実用的な遅延で完了できるかは未実測。遅延が過大なら toast-primary lens (現 deferred) の復帰余地が損なわれ、panel-primary lens 下でも toast fallback の応答性 (F-101 UAC-101 相当) に影響する。
  - validation: S3 実装直前に prototype で threshold-com-reactivation-latency (1s 暫定) をヒストグラム検証する (先行検証)
  - invalidation_behavior: 遅延過大なら ALT-TOAST-PRIMARY-ANCHOR-WORKSPACE の deferred 復帰余地を撤回。panel-primary lens 下の toast fallback (F-007/F-101) は Then の latency 記述を実測値に置換
  - decision_refs: DP-SUPERVISION-PRIMARY-ENTRY
- **assumption-fullscreen-panel-coexistence**: Blender/UE 等の exclusive fullscreen アプリと常時可視 panel (OPT-ALWAYS-VISIBLE-BAR 確定) を両立させると、対象アプリの入力奪取・フレームドロップが発生し得る。
  - validation: S3 実装時、Blender または UE の fullscreen mode 下で上端バー可視性と入力継続性をチェックリスト検証
  - invalidation_behavior: 副作用が許容範囲外なら OPT-ALWAYS-VISIBLE-BAR 確定下でも fullscreen 中は panel を自動非表示にする副契約を追加。fullscreen 中の supervision gap は既知の observable gap として F-002 UAC-004a に反映
  - decision_refs: DP-PANEL-DEFAULT-VISIBILITY
- **assumption-panel-completes-majority**: panel-primary lens 確定下で承認/質問応答の大多数が panel 上で完結し、Workspace 窓を開く頻度は低頻度に留まる (DP-WORKSPACE-WINDOW-LIFECYCLE=OPT-ON-DEMAND の中核仮説)。
  - validation: S4/S5 実装後、実運用ログで Workspace 窓オープン頻度を数週間観察
  - invalidation_behavior: 頻度が高ければ DP-WORKSPACE-WINDOW-LIFECYCLE 再検討 (always-open-anchor への切り替え) を user に諮る
  - decision_refs: DP-WORKSPACE-WINDOW-LIFECYCLE
- **assumption-toast-completes-majority**: toast-primary lens (現 deferred) 復帰時、Windows App SDK のボタン付き toast + inline textbox で supervision 往復の 8 割を toast 単独完結できる。
  - validation: toast-primary lens 復帰時に S3 実装後、承認/質問イベントに対する toast 単独完結率を実運用計測
  - invalidation_behavior: 完結率低なら toast-primary lens 復帰余地を再度撤回
  - decision_refs: DP-SUPERVISION-PRIMARY-ENTRY
- **assumption-workspace-window-restore-fidelity**: Workspace 窓の close/re-open で近い状態 (スクロール位置、ペイン分割、各ペインの表示対象) を復元できる (Electron 側で state 永続化 capability あり)。
  - validation: S4 実装時、Playwright for Electron で state persistence の忠実度を検証
  - invalidation_behavior: 復元忠実度が確保できないなら F-107 UAC-111 の Then を緩め、goal-workspace-window-anchor を『窓 close ≠ session 停止』の invariant のみに縮退
  - decision_refs: DP-WORKSPACE-WINDOW-LIFECYCLE

## Design Hypotheses

- **dh-panel-reduces-notification-fatigue**: 常時可視 panel は通知過多による疲弊を toast-primary より下げる (panel-primary lens 確定の中核仮説)。
- **dh-engage-restore-reduces-friction**: engage → 元窓フォーカス返却が最短経路であるほど、ジャンプバック操作の主観的な断絶感が下がる。
- **dh-toast-primary-signal-to-noise**: toast-primary lens 復帰時、supervision 目的以外の toast (daemon health 等) を toast 予算から排除する必要がある。panel-primary lens 確定下でも DP-DAEMON-HEALTH-VISIBILITY=OPT-TRAY-ONLY で両 lens 共通 invariant (goal-supervision-toast-budget) として保持。

## Quality Thresholds (proposed / provisional)

すべて status=provisional。S3-S5 実装後に validation_method に沿って再確定する。

- threshold-engage-expand-latency: engage 展開レイテンシ 150ms (owner=presentation、provenance=plan §3.2 verbatim『150ms 級』)
- threshold-panel-animation-framerate: panel 展開アニメ 60fps (owner=presentation、provenance=plan §3.2 verbatim『目標 60fps』)
- threshold-daemon-health-observation-delay: デーモン健全性表示の観察遅延上限 5s (owner=presentation、provenance=plan §3.6 の polling cadence を user-visible 遅延へ translation)
- threshold-workspace-idle-autoquit: Workspace 全窓 close 後 5min で自然終了 (owner=presentation、provenance=plan §4.2 verbatim)
- threshold-toast-response-latency: toast → user 応答確定の体感遅延 5s (owner=presentation、provenance=draft-2 planner_proposal、S3 実装後にヒストグラム検証)
- threshold-com-reactivation-latency: toast ボタン押下 → COM 再活性化 → Shell 側処理着火 1s (owner=presentation、provenance=draft-2 planner_proposal、unpackaged COM 実測未取得、S3 実装直前 prototype で先行検証)
- threshold-daemon-healthy-toast-budget-window: Healthy 継続中 supervision 目的以外 toast 0 件 invariant を判別する最小観察窓 5min (owner=presentation、provenance=critic_proposal、draft-2 の 1h が arbitrary_precision 指摘を受け短縮した暫定値)

## Technology Candidates for Design Evaluation

次の候補は非規範的な design handoff。採否 (最終選定・組合わせ) は design phase で決定する。この一覧は plan.json の `technology_candidates` と ux frontmatter に同じ stable ID + provenance で複写されている。

- **candidate-winui3-windows-app-sdk** — WinUI 3 / Windows App SDK。panel/toast/composition の主要ホスト候補。unpackaged COM background activation 実測 + fullscreen 共存副作用 + AppNotification IME 対応が open (assumption 群参照)。
- **candidate-h-notifyicon-winui** — H.NotifyIcon.WinUI (MIT)。トレイアイコンの状態別外観 + アクセシブルネーム + 非アクティベートフライアウト。DP-DAEMON-HEALTH-VISIBILITY=OPT-TRAY-ONLY 確定下で daemon health 表現の primary observable を担う。
- **candidate-appnotification-inline-textbox** — Windows App SDK AppNotificationManager (ボタン + inline textbox 付き toast)。panel-primary lens 下では panel-unwatched-only fallback の実装候補、toast-primary lens 復帰時は primary entry。IME / DND / 文字数上限が open。
- **candidate-composition-api-acrylic** — Windows Composition API + Acrylic 素材。panel 展開アニメーション/素材。60fps / 150ms 級を design で評価。
- **candidate-electron-workspace-host** — Electron (electron-builder dir target)。Workspace 窓ホスト、1 窓 1 セッションと window-registry、close ≠ session end、状態復元 (assumption-workspace-window-restore-fidelity と対応)。
- **candidate-named-pipe-jsonlines-ipc** — Node 標準 net (named pipe) + JSON Lines。Shell↔Workspace の低レイテンシ制御チャネル (openSession/focus/lifecycle、ドメインデータは流さない)。

## Design Handoffs

- **handoff-jump-back-target-provenance** (decision_refs: DP-JUMP-BACK-TARGET-PROVENANCE)
  - required_outcome: staged best-effort (HWND → プロセス+タイトルマッチ → 正直な失敗表示) で対象外部窓を特定。fabricated fallback (任意窓へのフォーカス移動) は禁止。
  - design_obligations: HWND キャッシュのライフサイクル / プロセス名+タイトルマッチ規則 / 失敗表示文言・アクセシブルネーム / OPT-STAGED-BEST-EFFORT の state machine 化
  - verification_obligations: 対象アプリ (Windows Terminal, VS Code, UE/Blender) ごとの特定成功率 / 失敗時に別窓へのフォーカス移動が発生しない / 対象窓 close 後の [Jump back] が『見つからない』を明示表示する

## Decision Points

各 decision の canonical projection (id + selected/unresolved + recommendation) は下部の managed block を参照。ここは human-readable context を記述する。

### DP-SUPERVISION-PRIMARY-ENTRY (must_ask / answered / normative → OPT-PANEL-PRIMARY)
承認/質問への一次対応窓口を常時可視 panel と AppNotification toast のどちらに置くか。
- OPT-PANEL-PRIMARY (**selected by user consultation 2026-07-24**): 常時可視 panel が一次対応窓口。toast は panel-unwatched-only の短文 approve/deny fallback。ALT-PANEL-PRIMARY-DUAL-SURFACE と整合。
- OPT-TOAST-PRIMARY: event-driven toast が第一動線、panel は summon-only overview へ後退。ALT-TOAST-PRIMARY-ANCHOR-WORKSPACE (deferred) と整合。
- user impact: OPT-PANEL-PRIMARY は一望性↑・通知疲弊↓だが常時画面領域を占有・fullscreen 共存に工夫 (assumption-fullscreen-panel-coexistence)。OPT-TOAST-PRIMARY は画面占有ゼロだが DND/Focus Assist・Action Center 消失・IME 対応 (assumption-appnotification-textbox-ime) に弱い。
- provenance: user_consultation / confirmed (consultation-20260724-windows-shell-phase2-r1)。S3 実装直前 prototype で assumption 2 件を検証し、結果次第で toast-primary lens 復帰余地を再評価する open が open_questions に残る。

### DP-TOAST-FALLBACK-CONDITION (must_ask / answered / normative → OPT-PANEL-UNWATCHED-ONLY)
toast をどの条件で発行するか。
- OPT-ALWAYS-SUPPLEMENT: panel 注視状態に関わらず常に toast も発行。
- OPT-PANEL-UNWATCHED-ONLY (**selected by user consultation 2026-07-24**): panel が『見られていない』条件 (別アプリ foreground / DND / ロック中) のときのみ toast。判定基準の具体閾値は design で確定。
- panel-primary 整合。panel 注視状態下では toast を発行せず二重専有による通知疲弊を回避する。

### DP-WORKSPACE-WINDOW-LIFECYCLE (must_ask / answered / default → OPT-ON-DEMAND)
Workspace 窓を session 生存中の常時アンカーとするかオンデマンドとするか。
- OPT-ALWAYS-OPEN-ANCHOR: session 生存中は常に窓を保持、close は畳むだけで session 停止と連動しない。toast-primary lens の fallback ladder 到達点として自然。
- OPT-ON-DEMAND (**selected by user consultation 2026-07-24**): 必要な時だけ開く。panel-primary lens の低頻度オープン想定と整合。日常 supervision は panel 完結、workspace 窓は『深く見る』時のみ。session close ≠ window close の invariant は F-107 で保持。assumption-panel-completes-majority が invalidate された場合の再検討余地は open_questions に残す。

### DP-PANEL-ENGAGE-FOCUS-RETURN (may_default / answered / normative → OPT-CAPTURE-RESTORE)
engage モード (テキスト入力) 確定/キャンセル後のフォーカス返却先。
- OPT-CAPTURE-RESTORE (**selected by user consultation 2026-07-24 round 3**): engage 開始前の foreground 窓を GetForegroundWindow で記録し、確定/Esc で明示的に返す。plan §3.2 verbatim を契約として確定。F-004 UAC-007 の Then (engage 中に別窓が一時 foreground を奪っても engage 直前 VS Code へ復帰) を保証。両 lens 共通 (toast-primary lens 復帰時の panel fallback 経路でも engage は発生する)。
- OPT-OS-DEFAULT-FOCUS: 明示的な記録/復元をせず OS の既定に委ねる。engage 中に別窓が foreground を奪った場合の戻り先が不定になり、F-004 UAC-007 の counterexample が示す fail 挙動。
- provenance: user_consultation / confirmed (consultation-20260724-windows-shell-phase2-r2)。answered へ昇格したことで OS default focus 挙動の不安定性を UX 契約として排除する。

### DP-JUMP-BACK-TARGET-PROVENANCE (defer / defaulted / handoff → OPT-STAGED-BEST-EFFORT)
ジャンプバック先の外部窓特定手段。OPT-STAGED-BEST-EFFORT (HWND 既知 → プロセス+タイトルマッチ → 正直な失敗表示) を defaulted。実装 mechanism は handoff-jump-back-target-provenance で design に委譲。

### DP-HOSTED-MODE-FRAME-INTEGRATION (may_ask / defaulted / default → OPT-INCREMENTAL-DEFRAME)
hosted mode 脱ブラウザ化 5 項目を S4 で全量既定にするか、S4→S5 で段階的にするか。OPT-INCREMENTAL-DEFRAME (S4 最小 + S5 で仕上げ) を defaulted。plan §8 スライス分け verbatim + §10 リスクでデザインレビュー exit 基準。

### DP-PANEL-DEFAULT-VISIBILITY (may_ask / answered / default → OPT-ALWAYS-VISIBLE-BAR)
panel (上端フローティングバー) の既定可視性。
- OPT-ALWAYS-VISIBLE-BAR (**selected by user consultation 2026-07-24**): 常時表示、非表示化はオプトアウト。ALT-PANEL-PRIMARY-DUAL-SURFACE と整合。plan §3.2 の上端バー常時表示を default 化。
- OPT-SUMMON-ONLY: 既定は非表示、トレイ/hotkey で召喚。ALT-TOAST-PRIMARY-ANCHOR-WORKSPACE (deferred) と整合。
- assumption-fullscreen-panel-coexistence の validation で fullscreen 中 auto-hide 副契約を検討する余地は残る。

### DP-PANEL-GLANCE-CONTENT-SCOPE (may_ask / answered / advisory → OPT-COUNTS-PLUS-LATEST)
glance (非アクティベート) 表示にどこまで出すか。
- OPT-COUNTS-ONLY: running/waiting/failed/done のカウントのみ。fullscreen 共存優先の planner 暫定案。
- OPT-COUNTS-PLUS-LATEST (**selected by user consultation 2026-07-24**): カウント + 直近 1 件の要約行。plan §3.2 の『セッション状態の要約 (running/waiting/failed/done のカウント + 直近)』を採用。fullscreen 共存の課題は assumption-fullscreen-panel-coexistence の validation で扱う。

### DP-DAEMON-HEALTH-VISIBILITY (may_default / answered / advisory → OPT-TRAY-ONLY)
デーモン監督イベントを Windows toast で知らせるかトレイアイコン外観のみに留めるか。
- OPT-TRAY-ONLY (**selected by user consultation 2026-07-24**): toast 予算を supervision 専用に保護。goal-supervision-toast-budget と整合。両 lens 共通 invariant として F-108 UAC-108 で固定。
- OPT-TOAST-ON-DEGRADED: Healthy/Spawning は静か、Degraded/失敗のみ toast。

### DP-TOAST-FALLBACK-AFFORDANCE (may_ask / answered / advisory → OPT-INLINE-PANEL-LINK)
toast textbox で入力続行不能時、toast 内に affordance を出すか状態表示のみか。
- OPT-INLINE-PANEL-LINK (**selected by user consultation 2026-07-24**): toast 内に『panel で回答』相当のボタン/リンクを表示。plan §3.3 の第一動線を維持しつつ fallback を明示化。AppNotification template 制約下での実装可否は S3 で確認 (assumption-appnotification-textbox-ime の validation と連動)。
- OPT-STATUS-ONLY: toast は状態表示のみ、panel 展開は user がトレイ/hotkey で行う。

## Recommended Direction (Explore)

全 7 件の must_ask / may_ask decisions がユーザー相談 r1 で answered へ確定し、DP-PANEL-ENGAGE-FOCUS-RETURN (may_default / normative) が r3 で OPT-CAPTURE-RESTORE に answered へ昇格したため、Explore 段階の provisional recommended_direction は空に整理した。残る defaulted 2 件は DP-JUMP-BACK-TARGET-PROVENANCE (defer / handoff) と DP-HOSTED-MODE-FRAME-INTEGRATION (may_ask / default) のみ。deferred alternative (ALT-TOAST-PRIMARY-ANCHOR-WORKSPACE) の復帰余地と S3 実装直前 prototype は open_questions を参照。

## Validation Questions (User Consultation Packet)

consultation-20260724-windows-shell-phase2-r1 で 7 件、r2 で DP-PANEL-ENGAGE-FOCUS-RETURN の 1 件が回答済み。追加の user consultation はない (次段階は S3 実装直前 prototype の invalidation_behavior 判定へ移る)。

## Primary Flows (Observable Contracts)

各 flow は entry (steps 先頭の system/glance/pointer 記述) と exit (Then で示される最終 observable 状態) を持つ。scenario の Given/Then は user-visible / assistive_technology の観察に限定し、instrumentation (spawn プロセス数・API 応答コード・contract payload 名) は counterexample の論証補助または instrumentation_hints へ落とす。全 scenario に counterexample (誤実装 + それが本 scenario で fail する論証) を付ける。lens 依存 flow (F-003/F-004: panel-primary 主動線 / F-101-108: toast-primary graft) は削除せず、deferred lens 対応として保持する (scenario_impacts で分類)。

### F-001 Shell 起動時のデーモン接続と健全性表示 (lens 中立)
Entry: Windows ログイン → Shell 自動起動 → adopt/spawn。Exit: supervision surface の健全性表示が Connected へ安定遷移。表示更新遅延は threshold-daemon-health-observation-delay (暫定 5s) 以内。

- **UAC-001**: 既存 server 稼働+token 一致で Shell 起動 → 健全性表示が不安定切り替えなしで Connected 安定。CE: 二重 spawn 実装は起動中↔Connected 反復が観察できる。
- **UAC-002a** (成功パス): デーモン不在で spawn 成功 → 起動中を経て Connected 遷移。CE: 起動中フェーズを表示しない実装は UAC-002b で fail。
- **UAC-002b** (失敗パス): デーモン不在+spawn クラッシュ → 起動中で固まらず失敗表示へ遷移。CE: エラー握りつぶし実装は起動中のまま推移。

### F-002 supervision surface による glance 監督 (panel-primary 確定下では上端バー + counts-plus-latest、両 lens 共通でトレイアイコン外観)
Entry: session 状態 WS 配信 → 別窓 focus のまま視線移動。Exit: 承認/質問カウント更新 + フォーカス保持。

- **UAC-003**: 承認待ち 1 件発生 → カウント表示 +1 が非アクティベートで視認、他窓入力欠落なし。CE: toast のみ更新で panel カウント非更新の実装は fail。
- **UAC-004a**: exclusive fullscreen アプリ中 → panel 監督 gap は observable な既知 gap、supervision surface は fullscreen アプリの入力/フレームレート奪取をしない。CE: fullscreen 中も上端バー最前面強制で対象アプリのフレームドロップ/入力奪取が発生 (assumption-fullscreen-panel-coexistence の否定側)。

### F-003 承認往復 (panel からの完結、panel-primary lens 主動線)
Entry: Phase 0 approval WS 配信 → panel カウント↑を視認 → クリックしてフライアウト展開。Exit: [Approve]/[Deny] 押下でキュー項目消失+残数-1、元 foreground アプリ (VS Code) のフォーカス保持。

- **UAC-005**: 承認待ち 1 件・panel フライアウト展開中・VS Code foreground → [Approve] クリック → 項目消失+VS Code フォーカス維持。CE(A) 一時 activate で VS Code フォーカス喪失、CE(B) 楽観的消失+失敗時未復活で resolved-by-other 齟齬。
- **UAC-006** (resolved-by-other): 別クライアントで既に応答済 → panel [Approve] 押下 → 『既に処理済み』表示、二重実行なし。CE: ローカル『未処理』フラグで resolved-by-other 無視。契約層の resolved-by-other payload 名は observation ではなく instrumentation_hints。

### F-004 質問応答 (engage + フォーカス返却、panel-primary lens 主動線)
Entry: Phase 0 question WS 配信 → キュー↑を視認 → 質問項目クリックで engage 遷移 (engage 前 foreground 窓を record)。Exit: 確定/Esc 直後、engage 直前窓 (VS Code) が foreground。

- **UAC-007**: engage 前に VS Code foreground、engage 中に別窓が一時 foreground を奪う → 確定直後 VS Code へ明示復帰。CE: OS 既定挙動委任で別窓に留まる/戻る (DP-PANEL-ENGAGE-FOCUS-RETURN OPT-OS-DEFAULT-FOCUS が確実に fail する scenario)。
- **UAC-008**: engage 前 foreground 窓が engage 中に閉鎖 → 確定時 Shell (panel) が activate されない次善策 + 返却先不在を偽装しない。CE: 無条件で Shell に focus 移動 → glance-primary 原則違反。

### F-005 ジャンプバック (外部窓アクティベーション、lens 中立)
Entry: [Jump back] クリック → staged best-effort (HWND → プロセス+タイトルマッチ → 失敗)。Exit: 対象窓 foreground、または明示的『見つからない』表示。

- **UAC-009**: HWND 既知+タブ生存 → 対象 Windows Terminal 窓が foreground、supervision surface は最前面に残らない。CE: 常に panel focus 保持で対象窓復帰の単独十分性を壊す。design_handoff_refs: handoff-jump-back-target-provenance。
- **UAC-010**: HWND もマッチも失敗 → 『ジャンプ先が見つかりません』明示表示、フォーカス移動なし。CE(A) 任意既定窓へフォーカス移動、CE(B) 無反応で失敗を隠蔽 (fabricated fallback 禁止)。

### F-006 Workspace 深掘り (窓再利用・脱ブラウザ、lens 中立)
Entry: [Open] または deep link → named pipe openSession → window-registry。Exit: 既存窓 focus (窓数不変) または新規生成、ブラウザ非経由。

- **UAC-011** (vs_legacy=must-fail): session X 窓既存 → [Open] 再クリックで窓数不変+session X 窓 focus。CE: 無条件新規生成で『タブ工場』化。window discipline (reuse) 判別。
- **UAC-012** (vs_legacy=must-fail): 初回オープン → 生成窓にアドレスバー・タブバー・session index 非表示、既定ブラウザプロセス未起動。CE: 既定ブラウザで clients/ui URL を開いてブラウザ経由。cc-no-browser-in-local-flow 判別。

### F-007 Toast fallback (panel-primary lens 確定下では panel-unwatched-only 短文 approve/deny)
Entry: 承認/質問イベント → supervision surface 更新 + DP-TOAST-FALLBACK-CONDITION=OPT-PANEL-UNWATCHED-ONLY 条件 (別アプリ foreground / DND / ロック中) 成立時に AppNotification 発行。Exit: 通知センターに toast 残存、[Approve]/inline textbox から短文応答完結 (panel 側キューも同時消失)。

- **UAC-013** (離席復帰): PC 離席中に承認発生 → 復帰時、通知センターに toast 残存、[Approve] 押下で supervision surface キューも消失。CE: 一過性ポップアップで通知センターに残さない実装 → 離席復帰で気づけない。
- **UAC-014** (panel 注視中): panel フライアウト展開中に承認発生 → OPT-PANEL-UNWATCHED-ONLY 確定下では toast 発行なし + panel キュー更新のみ。CE: OPT-PANEL-UNWATCHED-ONLY 確定下でも toast 発行される (option と実装の乖離)。Specify では option 依存の二分岐記述を単一 Then に統合する予定 (scenario_impacts=rewrite_in_specify)。

### F-008 デーモン再起動耐性と自動再接続 (両 lens 共通)
Entry: メニューから『Restart daemon』選択 → graceful shutdown → 永続化 → 再 spawn → WS 再接続。Exit: 健全性表示 Connected 復帰 + 再起動前と同じ session 群が両サーフェスで観察できる + 手動再接続不要。

- **UAC-015**: 3 session 稼働中+Restart daemon → 再起動完了時、健全性表示 Connected 復帰 + 3 session 再表示 + 手動操作不要。CE: 自動 WS 再接続なしで『切断』表示のまま固定。ADR-0025 backfill のネイティブクライアント適用判別。

### F-101 toast 単独承認 (toast-primary lens 主動線、deferred)
Entry: 承認待ち → AppNotification ボタン付き toast 表示 (ブラウザ非起動)。Exit: [Approve]/[Deny] 押下で確定表示 + 元アプリフォーカス保持、agent 再開。

- **UAC-101**: fullscreen ゲーム中に承認待ち → toast [Approve] 押下、threshold-toast-response-latency (5s、暫定目標) 以内に確定表示、元アプリフォーカス維持、Workspace/ブラウザ非起動。CE: Shell 窓を前面化してから承認 → 元アプリフォーカス喪失。panel-primary lens 確定下でも短文 approve/deny fallback の応答性検証として保持。
- **UAC-102**: toast が Action Center に落ちた後、[Deny] 押下 → ポップアップ時と同じ結果 (拒否確定) + supervision surface キュー消失。CE: Action Center 上ではボタン無反応。

### F-102 toast inline textbox 質問応答 (toast-primary lens 主動線、deferred)
Entry: 質問 → inline textbox 付き toast。Exit: textbox 入力 + Enter/送信で確定、panel/Workspace 非起動。

- **UAC-103**: toast textbox 入力可能な短文質問 (assumption-appnotification-textbox-ime validation 後に具体条件確定) → Enter 送信で確定 + panel/Workspace 未展開 + supervision surface キュー消失。CE: 送信が下書き保存のみで panel 再送信必須。panel-primary lens 下では toast fallback を短文 approve/deny に限定し、任意テキスト回答は panel engage を主動線とする。

### F-103 長文/入力方式制約による panel 展開 fallback (toast-primary lens の 2 段目、deferred)
Entry: 質問 toast → textbox 入力可能条件超過 → DP-TOAST-FALLBACK-AFFORDANCE=OPT-INLINE-PANEL-LINK 確定下では toast 内『panel で回答』affordance が現れる → panel 展開。Exit: panel 内テキスト欄で入力完了+送信。

- **UAC-104a** (文字数上限): 上限超過長文 → toast 上限拒否 → panel 展開で送信完了。CE: 上限超過を黙って切り詰めて送信 (agent 応答が意図と異なる)。
- **UAC-104b** (入力方式制約、observable 化): toast textbox で入力続行不能 (具体条件は assumption-appnotification-textbox-ime の S3 実装直前 prototype 検証結果で確定) → panel 展開で入力完了+送信 → 未確定/切り詰め文字列は送信されない。CE: 制約検出せず未確定 IME 文字列/切り詰めをそのまま送信。

### F-104 Workspace エスカレーション (toast-primary lens の 3 段目、deferred)
Entry: panel でも収まらない長文/複数行 → [Open in Workspace]。Exit: 既存窓 activate (窓数不変) または新規生成 1 窓。

- **UAC-105** (vs_legacy=must-fail): 複数行編集要 → [Open in Workspace] → 既存窓 activate または新規 1 窓、重複生成なし。CE: 毎回新規生成。panel-primary lens 確定下でも window discipline (re-use) invariant として保持。

### F-105 panel 召喚 (toast-primary lens の overview、DP-PANEL-DEFAULT-VISIBILITY=OPT-ALWAYS-VISIBLE-BAR 確定下では always-visible 主動線に格下げ)
Entry: OPT-ALWAYS-VISIBLE-BAR 確定下では Shell 起動直後から panel/上端バーが可視。トレイクリック → フライアウト展開。Exit: overview 表示。OPT-SUMMON-ONLY branch は toast-primary lens (deferred) 復帰時に活性化。

- **UAC-106** (OPT-SUMMON-ONLY branch): 起動後未操作 → バー・panel が画面上未描画 (トレイのみ可視)。CE: 既定表示 + オプトアウトのみで消えない。Specify では toast-primary lens 復帰時の branch scenario として明示分離予定 (scenario_impacts=rewrite_in_specify)。
- **UAC-107**: panel 非表示状態でトレイクリック → フライアウトに承認/質問キュー+直近 session 状態要約が読み切れる。CE: Workspace 前面化のみで overview 非表示。default visibility に非依存の観察として保持。

### F-107 Workspace 窓 close ≠ session 停止 (両 lens 共通 invariant)
Entry: session 生存中の Workspace 窓を close → agent 継続 → [Open] または deep link で再オープン。Exit: 再オープン後、状態復元 (assumption-workspace-window-restore-fidelity 依存)。

- **UAC-110** (vs_legacy=must-fail): 窓 close 直後 → supervision surface で session 状態が running/waiting 継続、'ended'/'terminated' 未遷移。CE: close イベントを stop 要求に変換。
- **UAC-111**: 再オープン → スクロール位置一致 + ペイン分割数一致 + 各ペイン表示対象種別 (terminal/diff/markdown) 一致 (assumption-workspace-window-restore-fidelity validation 済み前提)。CE: 毎回初期状態生成で復元なし。

### F-108 デーモン監督 toast 予算保護 (両 lens 共通、DP-DAEMON-HEALTH-VISIBILITY=OPT-TRAY-ONLY 確定)
Entry: デーモン Healthy 到達 → トレイアイコン外観反映。Exit: Degraded/失敗以外は toast 未発行、状態はトレイのみで読み取れる。

- **UAC-108**: Healthy 継続中に threshold-daemon-healthy-toast-budget-window (5min、暫定) の観察窓経過 + 承認/質問イベント 0 件 → supervision 目的以外 toast 0 件、トレイ外観のみで Healthy 読み取り可能。CE: health check 成功のたびに『daemon healthy』toast → 観察窓内で toast 件数>0。goal-supervision-toast-budget 判別。
- **UAC-109**: デーモン Degraded → トレイアイコン外観変化 (警告色) + ツールチップ『Degraded』読み取り可能。CE: 外観無変化で panel 展開しないと気づけない。

## Cross-Flow Consistency (統一語彙)

- 承認: approve / deny (両 lens 共通)。「承認」「拒否」は日本語補助表現。
- 質問応答: engage (panel 経路、テキスト入力モード、panel-primary 確定下の主動線) / inline textbox (toast 経路、panel-primary 下では短文 approve/deny fallback)。
- 復帰: jump-back (外部窓へ復帰)。パネル/toast どちらから発火しても同じ contract に集約。
- 常時可視: glance (非アクティベート観察、panel-primary lens の中核、DP-PANEL-DEFAULT-VISIBILITY=OPT-ALWAYS-VISIBLE-BAR で default 化、DP-PANEL-GLANCE-CONTENT-SCOPE=OPT-COUNTS-PLUS-LATEST で内容確定)。
- キャンセル/戻る: Esc は engage キャンセル、panel 外側クリック/無操作は panel 非表示化 (OPT-ALWAYS-VISIBLE-BAR 確定下ではフライアウト非表示化のみ)。
- フォーカス規律: engage 開始前 foreground 窓を record し確定/Esc 時に restore (DP-PANEL-ENGAGE-FOCUS-RETURN OPT-CAPTURE-RESTORE)。ジャンプバック成功時は対象外部窓へ foreground 移動、supervision surface は最前面に残らない。
- resolved-by-other: 契約層 payload 名。UI 側の観察は『既に処理済み』表示のみ。

## Contract Dependency Notes

以下 flow の Given/When/Then は Phase 0/1 契約に依存し、契約の実際の shape 確定後に再照合が必要:
- Phase 0 approval/question 契約: F-003, F-004, F-007, F-101, F-102, F-103
- Phase 1 notifications.schema.json: F-007, F-101, F-102, F-103, F-108
- Phase 1 deep-links.schema.json: F-005, F-006, F-104, F-107
- Phase 1 reconnect-contract / ADR-0025 拡張: F-008 (デーモン再起動耐性)、F-107 (Workspace 再オープン時の再接続)

## Open Questions

- S3 実装直前の先行 prototype: assumption-appnotification-textbox-ime (AppNotification inline textbox の IME 変換対応可否) と assumption-com-background-activation-unpackaged (unpackaged 構成での COM background activation 遅延) を先行検証し、invalidation_behavior を発動させるか判断する。panel-primary lens 下では toast fallback は短文 approve/deny のみで IME 影響は軽微だが、S3 検証結果次第で toast-primary lens (ALT-TOAST-PRIMARY-ANCHOR-WORKSPACE、deferred) 復帰余地の再評価および DP-SUPERVISION-PRIMARY-ENTRY の逆転可能性を open として残す。
- assumption-fullscreen-panel-coexistence — DP-PANEL-DEFAULT-VISIBILITY=OPT-ALWAYS-VISIBLE-BAR 確定により fullscreen 共存副作用が顕在化。S3 実装時に副作用が観察された場合の fullscreen 中 auto-hide 副契約 (F-002 UAC-004a の Then 拡張) を Specify に持ち越し。
- assumption-panel-completes-majority — DP-WORKSPACE-WINDOW-LIFECYCLE=OPT-ON-DEMAND 前提の中核仮説。S4/S5 実装後に Workspace 窓オープン頻度を観察し、頻度が高い場合は always-open-anchor への切り替えを user に諮る。
- assumption-workspace-window-restore-fidelity — F-107 UAC-111 の Then 詳細度は Playwright for Electron 検証結果で確定。invalidation 時は Then 緩和ルートを Specify に持ち越し。
- threshold-* — 全 threshold は status=provisional。S3-S5 実装後に validation_method に沿って確定値を差し戻す。
- contract_dependency: Phase 0 (approval/question domain) と Phase 1 (approval-contract.md, question-contract.md, notifications.schema.json, deep-links.schema.json, reconnect-contract.md) の shape 確定後、F-003/F-004/F-006/F-007/F-008/F-101/F-102/F-103/F-104 の Given/When/Then を再照合する必要がある。
- panel-primary lens の変形として draft-1 が持っていた UXA-3 (panel-primary + flyout-only、上端バー廃止) の扱い — 現時点では ALT-PANEL-PRIMARY-DUAL-SURFACE の下位トポロジーとして DP-PANEL-DEFAULT-VISIBILITY = OPT-SUMMON-ONLY で表現可能なため独立 alternative としては保持しない。DP-PANEL-DEFAULT-VISIBILITY は OPT-ALWAYS-VISIBLE-BAR で answered となったが、user が『panel-primary だが常時バーは不要』へ切り替える場合の構造復活余地は保持する。
- toast-primary lens (ALT-TOAST-PRIMARY-ANCHOR-WORKSPACE) の deferred 保持 — design/impl フェーズでの見直し材料として保持する。deferred lens 中核 flow (F-101/F-102/F-103/F-104/F-105/F-107 の toast 経路/F-108 の tray-only 共通 invariant) は削除せず、scenario_impacts で retain 分類のまま Specify に持ち越す。復帰条件は S3 assumption 先行検証結果とする。

<!-- requirements-decisions:start -->
## Decision State
Selected alternative: ALT-PANEL-PRIMARY-DUAL-SURFACE
Decision DP-DAEMON-HEALTH-VISIBILITY: OPT-TRAY-ONLY
Decision DP-HOSTED-MODE-FRAME-INTEGRATION: OPT-INCREMENTAL-DEFRAME
Decision DP-JUMP-BACK-TARGET-PROVENANCE: OPT-STAGED-BEST-EFFORT
Decision DP-PANEL-DEFAULT-VISIBILITY: OPT-ALWAYS-VISIBLE-BAR
Decision DP-PANEL-ENGAGE-FOCUS-RETURN: OPT-CAPTURE-RESTORE
Decision DP-PANEL-GLANCE-CONTENT-SCOPE: OPT-COUNTS-PLUS-LATEST
Decision DP-SUPERVISION-PRIMARY-ENTRY: OPT-PANEL-PRIMARY
Decision DP-TOAST-FALLBACK-AFFORDANCE: OPT-INLINE-PANEL-LINK
Decision DP-TOAST-FALLBACK-CONDITION: OPT-PANEL-UNWATCHED-ONLY
Decision DP-WORKSPACE-WINDOW-LIFECYCLE: OPT-ON-DEMAND
<!-- requirements-decisions:end -->
