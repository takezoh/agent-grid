---
change: change-20260723-windows-shell-phase2
role: ux
id: ux-20260723-windows-shell-phase2
kind: ux
title: 20260723 Windows Shell Phase 2 UX
status: draft
created: '2026-07-23'
summary: 'Explore-mode UX plan (revision explore-r-reconciled-02) for the Windows Shell (WinUI3) + Workspace (Electron) + WSL daemon supervision surface (Phase 2, S1-S5). Primary alternative ALT-PANEL-PRIMARY-DUAL-SURFACE (panel-primary lens) confirmed by user consultation 2026-07-24; ALT-TOAST-PRIMARY-ANCHOR-WORKSPACE retained as deferred. Twelve decision points: nine answered via user consultation (r1 + r2, incl. DP-PANEL-ENGAGE-FOCUS-RETURN normative promoted in r2); three remain defaulted from repository/planner as non-blocking (DP-JUMP-BACK-TARGET-PROVENANCE=defer/handoff, DP-HOSTED-MODE-FRAME-INTEGRATION=may_ask/default, DP-DAEMON-HEALTH-VISIBILITY=may_default/default). All eight critic pass2 improvements are traced in critique_resolutions. Specify-ready with no unresolved normative blocker.'
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
  - F-007
  - F-101
  - F-102
  - F-105
  required_capabilities:
  - 常時可視 top-level 非アクティベート window (panel-primary lens の上端バー)
  - トレイアイコン + フライアウトの非アクティベート展開
  - AppNotificationManager によるボタン付き toast と inline textbox
  - unpackaged 構成での COM background activation
  - Windows Composition API による滑らかな展開アニメーション
  - summon-only panel フライアウトの非アクティベート展開 (toast-primary lens 選定時)
  ux_constraints_relevant_to_evaluation:
  - panel が focus を奪わない (glance-primary の観察不変性)
  - toast 応答が threshold-toast-response-latency (暫定 5s) 級で完了する
  - panel 展開が threshold-engage-expand-latency (暫定 150ms) 級
  - テーマ (dark/light) 追従
  design_questions:
  - unpackaged 構成での COM 背景アクティベーションの実測遅延はどれくらいか (assumption-com-background-activation-unpackaged、S3 実装直前 prototype)
  - 上端バーが exclusive fullscreen アプリと共存する際の副作用 (assumption-fullscreen-panel-coexistence)
  - AppNotification inline textbox は IME 変換を扱えるか (assumption-appnotification-textbox-ime、S3 実装直前 prototype)
  disqualifiers:
  - unpackaged で COM background activation が構造的に機能しない場合 → toast-primary lens の primary 動線としての実用性が損なわれ、panel-primary lens 下でも toast fallback の応答性 (F-007) に影響
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
  - アイコン外観変化だけで daemon Healthy/Degraded/Spawning を弁別可能 (DP-DAEMON-HEALTH-VISIBILITY=OPT-TRAY-ONLY defaulted 下では primary observable)
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
  - DP-TOAST-PERSISTENCE-FALLBACK
  required_capabilities:
  - ボタン付き toast (バックグラウンド activation で foreground を奪わない)
  - inline textbox (短文回答)
  - Action Center 落ち後もボタンが有効
  - supervision イベントごとの一意な toast identity (再送しても重複しない)
  ux_constraints_relevant_to_evaluation:
  - toast 応答が threshold-toast-response-latency (5s 級) で完了する
  - COM 再活性化が threshold-com-reactivation-latency (1s 級) で着火する
  - supervision 目的以外の toast が発行されない invariant (goal-supervision-toast-budget)
  design_questions:
  - inline textbox の文字数上限は?
  - IME 変換対応の実挙動 (assumption-appnotification-textbox-ime、S3 実装直前 prototype)
  - DND / Focus Assist 中の挙動と fallback
  disqualifiers:
  - unpackaged 構成で COM background activation が構造的に機能しない場合 → toast-primary lens (deferred) の一次動線としての実用性が損なわれる
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
  - DP-WORKSPACE-WINDOW-LIFECYCLE
  required_capabilities:
  - 1 窓 1 セッションの BrowserWindow 生成/再利用
  - window-registry (session id → BrowserWindow の唯一生成点)
  - close 時に session を停止しない (窓 close ≠ session end)
  - close-re-open 時の scroll/pane 状態復元 (assumption-workspace-window-restore-fidelity と対応)
  ux_constraints_relevant_to_evaluation:
  - goal-workspace-deep-dive (窓数が増えない)
  - goal-workspace-window-anchor (close ≠ end)
  - goal-hosted-mode-no-browser (脱ブラウザ化)
  - OPT-ALWAYS-OPEN-ANCHOR 採用時の常駐メモリコスト (assumption-workspace-anchor-memory-cost-acceptable)
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
  - panel/toast の [Open in Workspace] から窓 activate までの体感遅延
  - 重複窓を生まない (window-registry と協調)
  design_questions:
  - Shell と Workspace の起動順序 (Shell 常駐 → Workspace オンデマンド または常時アンカー起動)
  disqualifiers:
  - Windows named pipe のセキュリティ属性が個人利用境界を越える場合 (単一ユーザー前提のため通常想定内)
design_handoffs:
- id: handoff-jump-back-target-provenance
  required_outcome: ジャンプバック先の外部窓を staged best-effort (HWND → プロセス+タイトルマッチ → 正直な失敗表示) で特定し、成功時は対象窓をフォアグラウンド化、失敗時は panel に『見つからない』を明示表示する。fabricated fallback (任意の窓へのフォーカス移動) を禁止する。
  design_obligations:
  - HWND キャッシュのライフサイクル (session 作成時登録、対象窓 close 時失効) を設計する
  - プロセス名 + タイトルマッチのマッチング規則 (Windows Terminal / VS Code / UE / Blender など主要外部アプリごとに検証) を定義する
  - 失敗表示の文言・アクセシブルネーム (『ジャンプ先が見つかりません』相当) を確定する
  - OPT-STAGED-BEST-EFFORT の各段の遷移条件を state machine として仕様化する
  verification_obligations:
  - 対象アプリごとの特定成功率を実機検証する (Windows Terminal, VS Code, UE/Blender)
  - 失敗時に別窓へのフォーカス移動が発生しないことを observable に検証する
  - 対象窓 close 後に [Jump back] を押した場合の Then が『見つからない明示表示』であることを scenario 検証する (F-005)
  provenance:
    source: repository_evidence
    confidence: inferred
    evidence_refs:
    - plans/plan-20260723-windows-shell-design.md#35
    rationale: DP-JUMP-BACK-TARGET-PROVENANCE の defer/handoff 契約効果に対応する design obligation を明示するため。
  decision_refs:
  - DP-JUMP-BACK-TARGET-PROVENANCE
---

## Goal

Phase 2 (S1-S5) の Windows Shell (WinUI3) + Workspace (Electron) + WSL 内デーモン監督 UX の supervision surface を、panel-primary lens (ALT-PANEL-PRIMARY-DUAL-SURFACE) と toast-primary lens (ALT-TOAST-PRIMARY-ANCHOR-WORKSPACE) の 2 構造 alternative として同格に保持したまま Explore し、承認/質問/ジャンプバック/窓規律/デーモン監督+再起動耐性を Given/When/Then + counterexample として固定する。critic pass1 winner_index=0 の provisional 判定に基づき ALT-PANEL-PRIMARY-DUAL-SURFACE を暫定的な primary alternative としているが、DP-SUPERVISION-PRIMARY-ENTRY を含む 4 件の must_ask blocker と 3 件の may_ask 開放は user consultation で確定される。integrator が defaulted 化しない。S3 実装直前に先行 prototype で 2 assumption (AppNotification IME、unpackaged COM background activation) を検証し、その結果次第で toast fallback ladder と lens 選定を再評価する。

Reference UX の姿勢: `Vibe Island (macOS notch ambient panel)` は panel-primary lens (draft-1) の中核リファレンスとして `modeled_on`。`iOS/Android 通知アクションボタン` は toast-primary lens (draft-2) 側では modeled_on 相当だが、panel-primary lens が provisional primary であるため `provisionally_rejected` としマークし、lens 決定後に再確定する。`現行 clients/ui ブラウザタブ` は両 lens 共通に `rejected` (親計画 Desired outcome #2、ブラウザは local flow に現れない)。

## Target Users

作者本人 (単一 Windows マシン、WSL 内 agent-grid デーモンを個人利用する開発者)。配布・複数ユーザー・チーム運用・installer/署名/自動更新は Phase 2 の対象外。

## Confirmed Constraints

- **cc-single-user-personal**: 個人利用前提。単一ユーザー・単一 Windows マシン、WSL 内 agent-grid デーモン。配布・署名・自動更新・多ユーザーは Phase 2 の非スコープ。(source: explicit_user_requirement, confidence: confirmed)
- **cc-shell-resident-workspace-lifecycle-open**: Shell はログイン時常駐。Workspace の起動性質 (on-demand か常時アンカーか) は DP-WORKSPACE-WINDOW-LIFECYCLE の unresolved product choice。(source: planner_proposal, confidence: inferred)
- **cc-no-browser-in-local-flow**: ブラウザアプリは local flow から完全に排除される (deep link 解決・セッション表示のいずれもブラウザを経由しない)。(source: plans/plan-20260723-native-clients.md#desired-outcome, confidence: confirmed)
- **cc-daemon-linux-only**: デーモン (server) は Windows へ移植しない。WSL または Linux ホストで動作し、Windows 側は loopback/LAN 経由で接続する。
- **cc-duplicate-response-contract-layer**: 二重応答 (panel と toast、または Workspace と toast など) はクライアント側で防がない。契約層 (approval-contract の単回裁定・resolved-by-other) に委ねる。(source: plans/plan-20260723-windows-shell-design.md#5)
- **cc-quit-vs-daemon-stop-separation**: Shell 終了 (明示 Quit) はデーモンを停止しない。デーモン停止は別メニュー項目として明示的に分離する。(source: plans/plan-20260723-windows-shell-design.md#36)
- **cc-phase0-1-contract-dependency**: Phase 2 UX の supervision イベント (approval, question, resolved-by-other, notification payload, deep link) は Phase 0 の approval/question サーバ側ドメイン確定と Phase 1 の approval-contract.md / question-contract.md / notifications.schema.json / deep-links.schema.json / reconnect-contract.md 確定を前提とする。契約 shape 確定後に Given/When/Then を再照合する必要がある。

## Existing System Context

- **esc-browser-spa**: 現行 clients/ui (React SPA) はブラウザタブ上で複数セッションを表示し、cmd/uihost が go:embed 配信、/api・/ws をゲートウェイへリバースプロキシしている。hosted mode で 1 窓 1 セッションへ反転。
- **esc-adr-0025-backfill**: ADR-0025 の transcript REST backfill → WS tail 再接続経路が既存。F-008 (デーモン再起動耐性) の自動再接続はこの経路のネイティブクライアント向け適用を前提。
- **esc-approval-question-not-modeled**: approval/question は現時点で codex app-server 内部にしかモデル化されておらず、host/state・server/api 側には未モデル化。Phase 0 のサーバ側ドメイン発出が Phase 2 UX の観察イベントの前提。

## User Goals

| id | actor | context | desired_outcome | success_observation |
|----|-------|---------|-----------------|---------------------|
| goal-daemon-supervised | operator | Windows ログイン時 Shell 自動起動 | WSL 内デーモンの生存確認・採用・新規起動が手動なしで完了しその結果が supervision surface に反映される | supervision surface (両 lens 共通のトレイアイコン外観、panel-primary lens 確定下では上端バーも) が Connected 相当へ到達 |
| goal-glance-supervision | operator | 別窓 (エディタ/ターミナル/UE/Blender 等) 注視中 | フォーカスや窓を切り替えずに session 群の状態と承認/質問キューを把握 | supervision surface (panel-primary lens 下では上端バー counts-plus-latest glance、toast-primary lens 下では toast 出現/トレイ外観変化) がクリック・フォーカス変更なしで最新集計を示す |
| goal-approval-round-trip | operator | agent が承認を要求 | Workspace 窓を開かずに supervision surface だけで承認/拒否を完結 | 対象キュー項目が supervision surface から消え、agent 側の実行が再開/停止 |
| goal-question-answer | operator | agent が質問を発行 | supervision surface のテキスト入力手段 (panel engage テキスト欄 / toast inline textbox) で回答 | 確定後、質問キュー項目が supervision surface から消え agent が回答受理 |
| goal-engage-focus-return | operator | engage テキスト入力を終えた直後 | engage 前 foreground 外部窓へ手動なしで戻る | 確定/Esc 直後に engage 直前 foreground 窓がフォアグラウンド、追加操作不要 |
| goal-jump-back | operator | 承認/質問対応後、元の外部窓へ復帰 | supervision surface の [Jump back] または toast 本文タップ一操作で対象外部窓へ復帰、失敗時は正直に『見つからない』 | 対象外部窓がフォアグラウンド、または明示メッセージ |
| goal-workspace-deep-dive | operator | diff レビュー・長い出力・複数ペインの深掘り | supervision surface または deep link から重複窓を生まず Workspace へ | 同一 session を繰り返し開いても Workspace 窓数が増えず既存窓がフォーカス |
| goal-hosted-mode-no-browser | operator | Workspace を開く一連の操作 | 一度もブラウザアプリを経由せずネイティブ窓に到達 | セッションを開く操作が Electron 窓に完結、既定ブラウザ起動 event が発生しない |
| goal-workspace-window-anchor | operator | Workspace 窓の close 操作 | 窓を閉じても対応 session は agent 側で継続 | 窓 close 直後も overview 上で対応 session が running/waiting、'ended'/'terminated' に遷移しない |
| goal-daemon-restart-resilience | operator | デーモン更新後の手動 Restart | 実行中 session を失わず supervision surface と Workspace が自動再接続 | 再起動完了後、健全性表示が Connected へ戻り、再起動前と同じ session が両サーフェスで観測できる (復帰までの遅延は threshold-daemon-restart-reconnect-delay の対象) |
| goal-toast-fallback-recovery | operator | panel を見ていなかった間 (別窓 foreground / 離席 / DND / ロック中) に承認/質問発生、または toast だけでは応答が完結しない (文字数超過・IME 制約) | 画面に戻ったとき見逃した要求に気づき直接応答、または fallback 段 (panel/Workspace) へシームレスに進める | Windows 通知センターまたは Toast 表示上に要求が残り、その affordance から応答が完結、または次段の入力欄にフォーカスが移る |
| goal-supervision-toast-budget | operator | supervision 目的以外 (daemon health 等) のイベント発生 | supervision toast 予算が infra ノイズで浪費されない | Healthy 継続中の観察窓 (threshold-daemon-healthy-toast-budget-window、暫定 5min) 内に supervision 目的以外の Windows 通知が 0 件、状態はトレイアイコン外観のみで読み取れる (DP-DAEMON-HEALTH-VISIBILITY=OPT-TRAY-ONLY defaulted) |

## Legacy Context

- **Source implementation**: 現行 clients/ui (React SPA、ブラウザタブで複数セッション表示)。cmd/uihost が go:embed で配信し、既定ブラウザで開く運用。
- **Inherited behaviors**: セッション状態一覧・承認/質問表示・diff/transcript レビューの表現要素は hosted mode の Workspace 窓へ継承。ADR-0025 transcript REST backfill → WS tail 再接続経路は F-008 の前提として継承。
- **Replaced behaviors**: ブラウザタブによる 1 タブ N セッション表示 → Workspace の 1 窓 1 セッションへ反転 (F-006)。ブラウザアドレスバー・タブバー・token 入力 UI・ブラウザショートカット (Ctrl+T/Ctrl+L 等) を hosted mode で除去 (F-006)。deep link を browser owner に委ねる挙動 → Shell/Workspace の window-registry によるアクティベート。SPA 内 toast だけで済ませていた supervision 通知 → Shell/Workspace の常駐 supervision surface (panel/tray/AppNotification) へ引き上げ。

## UX Alternatives

**ALT-PANEL-PRIMARY-DUAL-SURFACE** (source: draft-1、disposition: selected (provisional)、user_approved: false)
- primary_entry: 上端フローティングバー + トレイフライアウト (常時可視、glance 既定)
- 構造次元: primary_entry=panel(always-visible-bar+tray) / toast=fallback(panel-unwatched-only) / workspace_open_trigger=on-demand / panel_default_visibility=always-visible-bar / panel_glance_content=counts-plus-latest / daemon_health_visibility=tray-only / engage_focus_return=capture-restore / fallback_ladder=panel-with-toast-inline-link
- 要旨: 上端バー (counts-plus-latest glance) とトレイフライアウトが同一 supervision state を反映する常時可視面を持ち、承認/質問/ジャンプバックは panel から完結。Workspace は深掘り時のみオンデマンド (on-demand)、toast は panel 非注視時 (別窓 foreground / 離席 / DND / ロック中) に限って短文 approve/deny の fallback として発行。長文/IME 制約時は toast 内 inline-panel-link で panel へエスカレーション。engage モード確定後は engage 前 foreground 窓へ明示的にフォーカス返却。daemon 監督はトレイアイコン外観のみで表現し supervision toast 予算を保護。
- provenance: planner_proposal / inferred (draft-1 由来、plan §3.2 上端バー+トレイフライアウト詳細設計と Vibe Island 実証根拠)

**ALT-TOAST-PRIMARY-ANCHOR-WORKSPACE** (source: draft-2、disposition: deferred)
- primary_entry: AppNotification toast (ボタン + inline textbox、event 発生の都度ポップアップ)
- 構造次元: primary_entry=toast(event-driven) / panel=summon-only-overview / workspace_window_lifecycle=always-open-anchor / fallback_ladder=toast-then-panel-then-workspace / daemon_health_visibility=tray-only / top_bar=opt-in-summon
- 要旨: 承認/質問はボタン付き toast + inline textbox で単独完結を狙い、panel は summon-only の overview に格下げ。Workspace は session 生存中のアンカーとして常時開いておく (close ≠ session end)。応答不能時は toast→panel展開→workspace エスカレーションの 3 段 fallback ladder。daemon 監督イベントは toast 予算を消費せずトレイアイコン外観のみで表現。
- provenance: planner_proposal / inferred (draft-2 由来、iOS/Android 通知アクションボタンをリファレンスとし plan §3.3 の AppNotification 詳細設計を根拠)

**Alternative Comparison (selected: ALT-PANEL-PRIMARY-DUAL-SURFACE / approval=provisional / approved_by_user=false)**
- draft-1 → ALT-PANEL-PRIMARY-DUAL-SURFACE (selected, provisional): critic pass1 winner_index=0、swap_verdict=Y (draft-1 が provisional primary alternative)。draft-1 が counterexample・discrimination・legacy_context・quality_thresholds の観点で draft-2 より完成度が高いと判定された。Vibe Island の実証 + plan §3.2 上端バー+トレイフライアウト詳細設計を根拠とする。user consultation で確定必須。
- draft-2 → ALT-TOAST-PRIMARY-ANCHOR-WORKSPACE (deferred): 構造的に独立した lens として保持。primary_entry (toast) / panel role (summon-only overview) / workspace lifecycle (always-open-anchor) の 3 次元で ALT-PANEL-PRIMARY-DUAL-SURFACE と非対称。画面占有ゼロ・fullscreen 干渉最小の利点あり。assumption-com-background-activation-unpackaged と assumption-appnotification-textbox-ime の S3 実装直前 prototype 結果次第で復帰余地がある。integrator は user consultation なしに rejected 化しない。3 段 fallback ladder graft (critic graft #1) は panel-primary lens 側でも長文/IME 制約時の workspace escalation 指針として参照する。

## Product Assumptions (validation-required)

- **assumption-appnotification-textbox-ime**: Windows App SDK AppNotificationManager の inline textbox が日本語 IME 変換を扱えるかは未検証。panel-primary lens 下では toast fallback は panel-unwatched-only の短文 approve/deny のみで IME 影響は軽微だが、toast-primary lens 下では primary 動線の質問応答が IME を要するため中核仮説。
  - validation: S3 実装直前 prototype で AppNotification inline textbox の日本語 IME 変換対応を実機検証する (先行検証)
  - invalidation_behavior: 対応不可なら toast textbox 入力を非 IME 短文に限定し、DP-TOAST-PERSISTENCE-FALLBACK の fallback ladder 発火条件と DP-TOAST-FALLBACK-AFFORDANCE の affordance 文言・呼び出し頻度を再検討する。toast-primary lens 側では primary 動線の実用性が損なわれ DP-SUPERVISION-PRIMARY-ENTRY を再考する
  - decision_refs: DP-SUPERVISION-PRIMARY-ENTRY, DP-TOAST-PERSISTENCE-FALLBACK, DP-TOAST-FALLBACK-AFFORDANCE
- **assumption-com-background-activation-unpackaged**: unpackaged 構成での AppNotification COM background activation が toast ボタン押下から Shell 側処理起動までを実用的な遅延 (threshold-com-reactivation-latency、1s 暫定) で完了できるかは未実測。toast-primary lens の一次動線としての応答性の根拠、panel-primary lens 下でも toast fallback の応答性に影響する。
  - validation: S3 実装直前 prototype で threshold-com-reactivation-latency をヒストグラム検証する (先行検証)
  - invalidation_behavior: 遅延過大なら toast-primary lens の primary 動線としての実用性が損なわれ DP-SUPERVISION-PRIMARY-ENTRY の toast-primary option を disqualify し、panel-primary lens 下の toast fallback (F-007) は Then の latency 記述を実測値に置換する
  - decision_refs: DP-SUPERVISION-PRIMARY-ENTRY
- **assumption-fullscreen-panel-coexistence**: OPT-ALWAYS-VISIBLE-BAR (常時可視上端バー) が Blender/UE 等の exclusive fullscreen アプリと共存する際、入力奪取・フレームドロップの副作用が出るかは未検証。DP-PANEL-DEFAULT-VISIBILITY が OPT-ALWAYS-VISIBLE-BAR で確定した場合に顕在化する。
  - validation: S3 実装時、Blender または UE の fullscreen mode 下で上端バー可視性と入力継続性をチェックリスト検証する
  - invalidation_behavior: 副作用が許容範囲外なら fullscreen 中は panel を自動非表示にする副契約を追加する。副次的に fullscreen 中の supervision gap を既知の observable gap として scenario に反映する
  - decision_refs: DP-PANEL-DEFAULT-VISIBILITY
- **assumption-panel-completes-majority**: panel-primary lens 下で承認/質問応答の大多数が panel 上で完結し、Workspace 窓を開く頻度が低頻度に留まる (DP-WORKSPACE-WINDOW-LIFECYCLE を OPT-ON-DEMAND とする panel-primary lens 側の中核仮説)。
  - validation: S4/S5 実装後の実運用ログで Workspace 窓オープン頻度を数週間観察する
  - invalidation_behavior: 頻度が高ければ OPT-ALWAYS-OPEN-ANCHOR への切替をユーザーに諮る
  - decision_refs: DP-WORKSPACE-WINDOW-LIFECYCLE
- **assumption-toast-completes-majority**: toast-primary lens (draft-2 の中核仮説) 下で Windows App SDK のボタン付き toast + inline textbox で supervision 往復の 8 割を toast 単独完結できる。
  - validation: toast-primary lens が primary alternative として選定された場合、S3 実装後の承認/質問イベントに対する toast 単独完結率を実運用計測する
  - invalidation_behavior: 完結率低ければ DP-SUPERVISION-PRIMARY-ENTRY を panel-primary へ差し戻す。DP-TOAST-PERSISTENCE-FALLBACK の fallback ladder 3 段目 (workspace) 呼び出し頻度が想定を超え、DP-WORKSPACE-WINDOW-LIFECYCLE=OPT-ALWAYS-OPEN-ANCHOR の必要性も高まる
  - decision_refs: DP-SUPERVISION-PRIMARY-ENTRY, DP-TOAST-PERSISTENCE-FALLBACK, DP-WORKSPACE-WINDOW-LIFECYCLE
- **assumption-workspace-anchor-memory-cost-acceptable**: OPT-ALWAYS-OPEN-ANCHOR (session 常時 1 窓の Electron 窓) を採用した場合の窓ごとの常駐メモリコストが個人利用単一ユーザー前提で許容範囲。
  - validation: toast-primary lens 側で OPT-ALWAYS-OPEN-ANCHOR が採用された場合、S4 実装後に実機メモリ計測 (同時稼働 session 数を変えて測定)
  - invalidation_behavior: 許容範囲外なら DP-WORKSPACE-WINDOW-LIFECYCLE を OPT-ON-DEMAND へ差し戻す
  - decision_refs: DP-WORKSPACE-WINDOW-LIFECYCLE
- **assumption-workspace-window-restore-fidelity**: Workspace 窓の close/re-open でスクロール位置・ペイン分割・各ペイン表示対象を近い状態に復元できる (Electron 側で state 永続化 capability あり)。
  - validation: S4 実装時に Playwright for Electron で state persistence の忠実度を検証する
  - invalidation_behavior: 復元忠実度が確保できないなら goal-workspace-window-anchor を『窓 close ≠ session 停止』の invariant のみに縮退し、状態復元 scenario の Then を緩める
  - decision_refs: DP-WORKSPACE-WINDOW-LIFECYCLE

## Design Hypotheses

- **dh-panel-reduces-notification-fatigue**: 常時可視 panel は toast-primary より通知過多による疲弊を下げる (panel-primary lens 選定の中核仮説、S3-S5 実装後の運用観察が必要)。
- **dh-toast-reduces-window-management-overhead**: toast-primary は常時可視 panel より画面占有・ウィンドウ管理コストを下げ、fullscreen/ゲーム/UE/Blender 作業中の干渉が最小になる (toast-primary lens 選定の中核仮説)。
- **dh-engage-restore-reduces-friction**: engage → 元窓フォーカス返却の返却先明示性が高いほど、承認/質問対応後の作業断絶感 (主観) が下がる。両 lens 共通 (toast-primary lens の panel fallback 経路でも engage は発生)。
- **dh-toast-budget-protects-trust**: supervision 目的以外 toast をゼロに保つことで toast への信頼を維持し、両 lens 共通で通知チャネルとしての supervision toast 予算を保護する。DP-DAEMON-HEALTH-VISIBILITY=OPT-TRAY-ONLY defaulted の根拠。

## Quality Thresholds (proposed / provisional)

すべて status=proposed。S3-S5 実装後に validation_method に沿って再確定する。value 未確定 (null) の項目は critic pass2 由来で、確定は S1 実装後の実測に委ねる。

- threshold-engage-expand-latency: engage 展開レイテンシ 150ms (owner=presentation、provenance=plan §3.2 verbatim『150ms 級』)
- threshold-panel-animation-framerate: panel 展開アニメ 60fps (owner=presentation、provenance=plan §3.2 verbatim『目標 60fps』)
- threshold-daemon-health-observation-delay: デーモン健全性表示の観察遅延上限 5s (owner=presentation、provenance=plan §3.6 polling cadence を user-visible 遅延へ translation)
- threshold-workspace-idle-autoquit: Workspace 全窓 close 後 5min で自然終了 (owner=presentation、provenance=plan §4.2 verbatim、DP-WORKSPACE-WINDOW-LIFECYCLE=OPT-ON-DEMAND 選定時に有効)
- threshold-toast-response-latency: toast → user 応答確定の体感遅延 5s (owner=presentation、provenance=planner_proposal、S3 実装後にヒストグラム検証)
- threshold-com-reactivation-latency: toast ボタン押下 → COM 再活性化 → Shell 側処理着火 1s (owner=presentation、provenance=planner_proposal、unpackaged COM 実測未取得、S3 実装直前 prototype で先行検証)
- threshold-daemon-healthy-toast-budget-window: Healthy 継続中 supervision 目的以外 toast 0 件 invariant を判別する最小観察窓 5min (owner=presentation、provenance=critic pass2 反映: arbitrary_precision の疑いあり、1h→5min 短縮方向のみ推論、桁の導出根拠未確立)
- threshold-daemon-restart-reconnect-delay: デーモン Restart 完了から supervision surface Connected 復帰までの遅延上限 (owner=presentation、value 未確定=null、provenance=critic pass2 反映: ADR-0025 拡張の再接続バックオフ設計待ち、S1 実装後の実測で確定)

## Technology Candidates for Design Evaluation

次の候補は非規範的な design handoff。採否 (最終選定・組合わせ) は design phase で決定する。この一覧は plan.json の `technology_candidates` と ux frontmatter に同じ stable ID + provenance で複写されている。

- **candidate-winui3-windows-app-sdk** — WinUI 3 / Windows App SDK。panel/toast/composition の主要ホスト候補。unpackaged COM background activation 実測 + fullscreen 共存副作用 + AppNotification IME 対応が open (assumption 群参照)。
- **candidate-h-notifyicon-winui** — H.NotifyIcon.WinUI (MIT)。トレイアイコンの状態別外観 + アクセシブルネーム + 非アクティベートフライアウト。DP-DAEMON-HEALTH-VISIBILITY=OPT-TRAY-ONLY defaulted 下で daemon health 表現の primary observable を担う。
- **candidate-appnotification-inline-textbox** — Windows App SDK AppNotificationManager (ボタン + inline textbox 付き toast)。panel-primary lens 下では panel-unwatched-only fallback、toast-primary lens 下では primary entry。IME / DND / 文字数上限が open。
- **candidate-composition-api-acrylic** — Windows Composition API + Acrylic 素材。panel 展開アニメーション/素材。60fps / 150ms 級を design で評価。
- **candidate-electron-workspace-host** — Electron (electron-builder dir target)。Workspace 窓ホスト、1 窓 1 セッションと window-registry、close ≠ session end、状態復元 (assumption-workspace-window-restore-fidelity と対応)。
- **candidate-named-pipe-jsonlines-ipc** — Node 標準 net (named pipe) + JSON Lines。Shell↔Workspace の低レイテンシ制御チャネル (openSession/focus/lifecycle、ドメインデータは流さない)。

## Design Handoffs

- **handoff-jump-back-target-provenance** (decision_refs: DP-JUMP-BACK-TARGET-PROVENANCE)
  - required_outcome: staged best-effort (HWND → プロセス+タイトルマッチ → 正直な失敗表示) で対象外部窓を特定。fabricated fallback (任意窓へのフォーカス移動) は禁止。
  - design_obligations: HWND キャッシュのライフサイクル / プロセス名+タイトルマッチ規則 / 失敗表示文言・アクセシブルネーム / OPT-STAGED-BEST-EFFORT の state machine 化
  - verification_obligations: 対象アプリ (Windows Terminal, VS Code, UE/Blender) ごとの特定成功率 / 失敗時に別窓へのフォーカス移動が発生しない / 対象窓 close 後の [Jump back] が『見つからない』を明示表示する

## Decision Points

各 decision の canonical projection (id + selected/unresolved + recommendation) は下部の managed block を参照。ここは human-readable context を記述する。全 must_ask / may_ask blocker は user consultation で解消される (integrator は defaulted 化しない)。

### DP-SUPERVISION-PRIMARY-ENTRY (must_ask / unanswered / normative)
承認/質問への一次対応窓口を常時可視 panel と AppNotification toast のどちらに置くか。
- OPT-PANEL-PRIMARY (recommendation): 常時可視 panel が一次対応窓口、toast は panel-unwatched-only の短文 approve/deny fallback。ALT-PANEL-PRIMARY-DUAL-SURFACE と整合。
- OPT-TOAST-PRIMARY: event-driven toast が第一動線、panel は summon-only overview へ後退。ALT-TOAST-PRIMARY-ANCHOR-WORKSPACE と整合。
- user impact: OPT-PANEL-PRIMARY は一望性↑・通知疲弊↓だが常時画面領域を占有・fullscreen 共存に工夫 (assumption-fullscreen-panel-coexistence)。OPT-TOAST-PRIMARY は画面占有ゼロだが DND/Focus Assist・Action Center 消失・IME 対応 (assumption-appnotification-textbox-ime) に弱い。
- provenance: planner_proposal / inferred。critic pass1 winner_index=0 の provisional 判定に基づく暫定推奨。user consultation で確定必須。

### DP-TOAST-FALLBACK-CONDITION (may_ask / unanswered / normative)
toast をどの条件で発行するか (panel-primary lens 側の fallback 発火条件)。
- OPT-ALWAYS-SUPPLEMENT: panel 注視状態に関わらず常に toast も発行。
- OPT-PANEL-UNWATCHED-ONLY (recommendation): panel が『見られていない』条件 (別アプリ foreground / DND / ロック中) のときのみ toast。判定基準の具体閾値は design で確定。

### DP-TOAST-PERSISTENCE-FALLBACK (must_ask / unanswered / normative)
toast で応答が完結しない場合 (文字数超過・IME 制約・DND 等) にどこまで fallback させるか (toast-primary lens 側の fallback ladder 段数)。
- OPT-3-STAGE-LADDER (recommendation): toast→panel展開→workspaceエスカレーション。
- OPT-2-STAGE-DIRECT-WORKSPACE: toast→workspace直行、panel省略。
- OPT-NO-FALLBACK-TOAST-ONLY: fallback なし (toast 単独のみ)。
- toast-primary lens 選定時の中核決定。panel-primary lens 選定時も panel-unwatched-only toast の完結不能時経路として関連。

### DP-WORKSPACE-WINDOW-LIFECYCLE (must_ask / unanswered / normative)
Workspace 窓を session 生存中の常時アンカーとするかオンデマンドとするか。
- OPT-ALWAYS-OPEN-ANCHOR: session 生存中は常に窓を保持、close は畳むだけで session 停止と連動しない。toast-primary lens の fallback ladder 到達点として自然。
- OPT-ON-DEMAND (recommendation): 必要な時だけ開く。panel-primary lens の低頻度オープン想定と整合。日常 supervision は panel 完結、workspace 窓は『深く見る』時のみ。assumption-panel-completes-majority が invalidate された場合の再検討余地は open_questions に残す。

### DP-PANEL-ENGAGE-FOCUS-RETURN (may_default / answered / normative → OPT-CAPTURE-RESTORE)
engage モード (テキスト入力) 確定/キャンセル後のフォーカス返却先。
- OPT-CAPTURE-RESTORE (answered、r2 consultation): engage 開始前の foreground 窓を GetForegroundWindow で記録し、確定/Esc で明示的に返す。plan §3.2 verbatim。両 lens 共通 (toast-primary lens 復帰時の panel fallback 経路でも engage は発生)。
- OPT-OS-DEFAULT-FOCUS: 明示的な記録/復元をせず OS の既定に委ねる。engage 中に別窓が foreground を奪った場合の戻り先が不定になる。
- provenance: user_consultation / confirmed (consultation-20260724-windows-shell-phase2-regen-r2)。normative defaulted→answered。関連する意図的 Alt+Tab 切替扱いは DP-ENGAGE-FOCUS-RETURN-INTENTIONAL-SWITCH (OPT-RESPECT-INTENTIONAL-SWITCH) で切り出し済み。

### DP-ENGAGE-FOCUS-RETURN-INTENTIONAL-SWITCH (must_ask / unanswered / normative)
engage 中に user が Alt+Tab 等で意図的に別窓へ切り替えた場合の扱い。critic pass2 issue 8 由来の new_product_capability。
- OPT-RESTORE-ALL-CASES: engage 前窓へ常に復元、意図切替を上書き。
- OPT-RESPECT-INTENTIONAL-SWITCH: 意図切替は復元対象外、切替後の foreground を尊重。
- planner recommendation なし (user 主体判断を要する)。

### DP-JUMP-BACK-TARGET-PROVENANCE (defer / defaulted / handoff → OPT-STAGED-BEST-EFFORT)
ジャンプバック先の外部窓特定手段。OPT-STAGED-BEST-EFFORT (HWND 既知 → プロセス+タイトルマッチ → 正直な失敗表示) を defaulted。実装 mechanism は handoff-jump-back-target-provenance で design に委譲。

### DP-HOSTED-MODE-FRAME-INTEGRATION (may_ask / defaulted / default → OPT-INCREMENTAL-DEFRAME)
hosted mode 脱ブラウザ化 5 項目を S4 で全量既定にするか、S4→S5 で段階的にするか。OPT-INCREMENTAL-DEFRAME (S4 最小 + S5 で仕上げ) を defaulted。plan §8 スライス分け verbatim + §10 リスクでデザインレビュー exit 基準。

### DP-PANEL-DEFAULT-VISIBILITY (may_ask / unanswered / default)
panel (上端フローティングバー) の既定可視性。
- OPT-ALWAYS-VISIBLE-BAR (recommendation): 常時表示、非表示化はオプトアウト。ALT-PANEL-PRIMARY-DUAL-SURFACE と整合。plan §3.2 の上端バー常時表示を default 化。
- OPT-SUMMON-ONLY: 既定は非表示、トレイ/hotkey で召喚。ALT-TOAST-PRIMARY-ANCHOR-WORKSPACE と整合。
- assumption-fullscreen-panel-coexistence の validation で fullscreen 中 auto-hide 副契約を検討する余地は残る。

### DP-PANEL-GLANCE-CONTENT-SCOPE (may_ask / unanswered / default)
glance (非アクティベート) 表示にどこまで出すか。
- OPT-COUNTS-ONLY: running/waiting/failed/done のカウントのみ。fullscreen 共存優先の planner 暫定案。
- OPT-COUNTS-PLUS-LATEST (recommendation): カウント + 直近 1 件の要約行。plan §3.2 の『セッション状態の要約 (running/waiting/failed/done のカウント + 直近)』を採用。fullscreen 共存の課題は assumption-fullscreen-panel-coexistence の validation で扱う。

### DP-DAEMON-HEALTH-VISIBILITY (may_default / defaulted / default → OPT-TRAY-ONLY)
デーモン監督イベントを Windows toast で知らせるかトレイアイコン外観のみに留めるか。
- OPT-TRAY-ONLY (defaulted): toast 予算を supervision 専用に保護。goal-supervision-toast-budget と整合。両 lens 共通 invariant として F-108 UAC-108 で固定。
- OPT-TOAST-ON-DEGRADED: Healthy/Spawning は静か、Degraded/失敗のみ toast。

### DP-TOAST-FALLBACK-AFFORDANCE (may_ask / unanswered / default)
toast textbox で入力続行不能時、toast 内に affordance を出すか状態表示のみか。
- OPT-INLINE-PANEL-LINK (recommendation): toast 内に『panel で回答』相当のボタン/リンクを表示。plan §3.3 の第一動線を維持しつつ fallback を明示化。AppNotification template 制約下での実装可否は S3 で確認 (assumption-appnotification-textbox-ime の validation と連動)。
- OPT-STATUS-ONLY: toast は状態表示のみ、panel 展開は user がトレイ/hotkey で行う。

## Recommended Direction (Explore)

lens 選定 (DP-SUPERVISION-PRIMARY-ENTRY) を軸に従属推奨が連動する構造。以下は provisional recommendation で、user consultation で確定される。

- **DP-SUPERVISION-PRIMARY-ENTRY → OPT-PANEL-PRIMARY** (planner_proposal / provisional): critic pass1 winner_index=0 に基づく provisional 選定。Vibe Island 実証 + plan §3.2 詳細設計を根拠。
- **DP-TOAST-FALLBACK-CONDITION → OPT-PANEL-UNWATCHED-ONLY** (panel-primary lens 選定時の従属推奨、panel 注視中の二重専有回避)。
- **DP-TOAST-PERSISTENCE-FALLBACK → OPT-3-STAGE-LADDER** (toast-primary lens 選定時の従属推奨、中間 panel 展開で workspace 起動コストを最終手段に留める)。
- **DP-WORKSPACE-WINDOW-LIFECYCLE** は lens 追従: panel-primary → OPT-ON-DEMAND、toast-primary → OPT-ALWAYS-OPEN-ANCHOR。
- **DP-PANEL-DEFAULT-VISIBILITY** は lens 追従: panel-primary → OPT-ALWAYS-VISIBLE-BAR、toast-primary → OPT-SUMMON-ONLY。
- **DP-PANEL-GLANCE-CONTENT-SCOPE → OPT-COUNTS-PLUS-LATEST** (plan §3.2 verbatim、panel-primary lens 選定時のみ意味)。
- **DP-TOAST-FALLBACK-AFFORDANCE → OPT-INLINE-PANEL-LINK** (fallback 発見性を上げる。AppNotification template 制約下での実装可否は S3 prototype 依存)。
- **DP-ENGAGE-FOCUS-RETURN-INTENTIONAL-SWITCH**: planner から明確な推奨を持たない (critic pass2 由来、user 主体判断が必要)。

## Validation Questions (User Consultation Packet)

以下 8 件は user consultation で回答する必要がある。must_ask blocker (4 件) を最優先とし、may_ask (3 件) と critic 由来 new blocker (1 件) を続ける。

1. **DP-SUPERVISION-PRIMARY-ENTRY** (must_ask): 承認/質問への一次対応窓口を、常時可視 panel (OPT-PANEL-PRIMARY、ALT-PANEL-PRIMARY-DUAL-SURFACE 相当) と AppNotification toast (OPT-TOAST-PRIMARY、ALT-TOAST-PRIMARY-ANCHOR-WORKSPACE 相当) のどちらに置きますか？
2. **DP-TOAST-PERSISTENCE-FALLBACK** (must_ask): toast で応答が完結しない場合の fallback ladder は？ OPT-3-STAGE-LADDER (toast→panel→workspace) / OPT-2-STAGE-DIRECT-WORKSPACE / OPT-NO-FALLBACK-TOAST-ONLY のどれを採用しますか？
3. **DP-WORKSPACE-WINDOW-LIFECYCLE** (must_ask): Workspace 窓を session 生存中の常時アンカー (OPT-ALWAYS-OPEN-ANCHOR) にするか、必要時のみ開くオンデマンド (OPT-ON-DEMAND) にするか？ lens 選定に強く連動します。
4. **DP-ENGAGE-FOCUS-RETURN-INTENTIONAL-SWITCH** (must_ask): engage 中に user が Alt+Tab 等で意図的に別窓へ切り替えた場合、engage 前窓へ復元 (OPT-RESTORE-ALL-CASES) するか、切替後の foreground を尊重 (OPT-RESPECT-INTENTIONAL-SWITCH) するか？ critic pass2 で明示化された新しい open question です。
5. **DP-TOAST-FALLBACK-CONDITION** (may_ask): toast をどの条件で発行しますか？ panel-primary lens 下では OPT-PANEL-UNWATCHED-ONLY を推奨、OPT-ALWAYS-SUPPLEMENT だと panel 注視中でも toast が出て通知疲弊源になります。
6. **DP-PANEL-DEFAULT-VISIBILITY** (may_ask): panel (上端フローティングバー) の既定可視性を常時表示 (OPT-ALWAYS-VISIBLE-BAR) にするか、召喚制 (OPT-SUMMON-ONLY) にするか？ lens 選定に連動しますが fullscreen 共存 (assumption-fullscreen-panel-coexistence) の許容度で個別判断も可能です。
7. **DP-PANEL-GLANCE-CONTENT-SCOPE** (may_ask): glance 表示にどこまで情報を出しますか？ OPT-COUNTS-ONLY (件数のみ) と OPT-COUNTS-PLUS-LATEST (件数 + 直近 1 件) のいずれ。
8. **DP-TOAST-FALLBACK-AFFORDANCE** (may_ask): toast textbox で入力続行不能な時の toast 内 affordance をどうしますか？ OPT-INLINE-PANEL-LINK (『panel で回答』ボタン表示) / OPT-STATUS-ONLY (状態表示のみ) のいずれ。

## Primary Flows (Observable Contracts)

各 flow は entry (steps 先頭の system/glance/pointer 記述) と exit (Then で示される最終 observable 状態) を持つ。scenario の Given/Then は user-visible / assistive_technology の観察に限定し、instrumentation (spawn プロセス数・API 応答コード・contract payload 名・selector 等) は counterexample の論証補助または instrumentation_hints へ落とす。全 scenario に counterexample (誤実装 + それが本 scenario で fail する論証) を付ける。lens 依存 flow (F-003/F-004: panel-primary 主動線 / F-101-F-108: toast-primary lens graft) は両方保持し、lens 決定後に scenario_impacts で分類する。

### F-001 Shell 起動時のデーモン接続と健全性表示 (lens 中立)
Entry: Windows ログイン → Shell 自動起動 → adopt/spawn。Exit: supervision surface の健全性表示が Connected へ到達 (threshold-daemon-health-observation-delay 暫定 5s 以内)。

- **UAC-001**: Given: 既存 server が稼働し token が一致している。When: Shell が起動する。Then: 健全性表示が threshold-daemon-health-observation-delay (5s 暫定) 以内に Connected へ到達し、到達直後の表示に起動中相当の中間状態が残っていない。CE: 疎通確認をせず無条件で新規 spawn する実装は、既存プロセスとの二重稼働で health flicker が観察され fail する (spawn→Connected の内部遷移経路は instrumentation_hints で telemetry ログ検証)。
- **UAC-002a** (成功パス): Given: デーモン不在。When: Shell が spawn する。Then: 起動中表示を経て Connected へ遷移する。CE: 起動中フェーズを表示しない実装は spawn 完了前の API 呼び出しが無応答になり fail する。
- **UAC-002b** (失敗パス): Given: デーモン不在かつ spawn 直後クラッシュ。When: health check がタイムアウトする。Then: 起動中表示のまま固まらず、操作可能な失敗表示へ遷移する。CE: エラーを握りつぶし起動中表示のまま無限ポーリングする実装は fail する。

### F-002 supervision surface による glance 監督 (panel-primary lens 下では上端バー + counts-plus-latest、両 lens 共通でトレイアイコン外観)
Entry: session 状態 WS 配信 (contract dependency: Phase 0/1) → 別窓 focus のまま視線移動。Exit: 承認/質問カウント更新 + フォーカス保持。

- **UAC-003**: Given: 承認待ち 0 件で別窓 (VS Code) にフォーカスがある。When: 承認イベントが 1 件到着する。Then: 上端バーのカウント表示が +1 で非アクティベートで更新され、元窓へのキー入力が欠落しない。CE: toast のみでカウントを更新し panel 表示を据え置く実装は panel-primary lens の『panel を見れば全部わかる』第一体験を満たさず fail する。
- **UAC-004a** (fullscreen 共存): Given: Blender が exclusive fullscreen 中。When: 承認イベントが到着する。Then: supervision surface の観察 gap は許容された既知 gap として扱われ、対象アプリの入力奪取・フレームドロップは発生しない。CE: fullscreen 中も上端バーを最前面へ強制表示する実装は対象アプリの入力を奪う/フレームを落とし、assumption-fullscreen-panel-coexistence の否定側として fail する。

### F-003 承認往復 (panel からの完結、panel-primary lens 主動線、pass2 issue 4 反映で modality tagging 拡張)
Entry: Phase 0 approval WS 配信 → panel カウント↑を視認 → panel を展開する。Exit: [Approve]/[Deny] 押下でキュー項目消失+残数-1、元 foreground アプリ (VS Code) のフォーカス保持、agent 側の実行が再開/停止。
Steps:
- [system] Phase 0 approval WS イベントが配信される (contract dependency: Phase 0/1)
- [glance] 上端バーのカウントが増加する
- [pointer|keyboard] バーへフォーカス移動またはクリックしフライアウトを展開する
- [pointer|keyboard] Tab で [Approve]/[Deny] へ到達し Enter (または対応 accelerator) で押下する

- **UAC-005** (成功パス): Given: 承認待ち 1 件・フライアウト展開中・VS Code が foreground。When: [Approve] を押下する。Then: 項目がキューから消失し残数 -1、VS Code のフォーカスが維持され、agent 側の実行が再開する。CE(A): 承認処理のため一時的に Shell を activate する実装は VS Code のフォーカスを奪い fail する。CE(B): マウス pointer 経由のみ実装で Tab フォーカス遷移も accelerator も持たない実装は steps に明記された [keyboard] modality 要求を満たせず到達性検証で fail する (pass2 issue 4 反映)。
- **UAC-006** (失敗パス、pass2 issue 3 由来の新 scenario): Given: 承認待ち 1 件・[Approve] クリック直後に承認 API 呼び出しがネットワークエラーで失敗する。When: 失敗レスポンスを受け取る。Then: キュー項目が UI 上に復帰する (楽観的削除が rollback される) か、明示的なエラー表示が出て resolved 状態を偽装しない。agent 側は未承認のまま停止を維持する。CE: 楽観的に UI からキュー項目を消し残数 -1 表示するが承認 API 失敗時に UI を戻さず agent 側は未承認のまま停止する実装は、Then の 2 観察 (項目復帰またはエラー表示、agent 未承認維持) を fail する。
- **UAC-006r** (resolved-by-other): Given: 別クライアント (toast) が既に応答済みである。When: panel 側で同じ項目に [Approve] を押下する。Then: 『既に処理済み』相当の表示が出て二重実行は発生しない。CE: panel 側のローカル未処理フラグのみで判定し resolved-by-other を無視する実装は二重実行が観察され fail する。契約層 resolved-by-other payload 名は observation ではなく instrumentation_hints。

### F-004 質問応答 (engage + フォーカス返却、panel-primary lens 主動線)
Entry: Phase 0 question WS 配信 → キュー↑を視認 → 質問項目クリックで engage 遷移 (engage 前 foreground 窓を record)。Exit: 確定/Esc 直後、engage 直前窓 (VS Code) が foreground。

- **UAC-007**: Given: engage 開始前に VS Code が foreground、engage 中に無関係の別窓が一時的に foreground を奪う。When: 回答を確定する。Then: engage 直前の VS Code へ明示的にフォーカスが復帰する。CE: OS の既定フォーカス挙動に委ね明示的な記録/復元を行わない実装 (DP-PANEL-ENGAGE-FOCUS-RETURN=OPT-OS-DEFAULT-FOCUS) は、確定後も奪取した別窓に留まり fail する。
- **UAC-008**: Given: engage 開始前 foreground 窓が engage 中に閉じられる。When: 回答を確定する。Then: Shell は無条件に自分自身へフォーカスを奪わず、返却先不在を偽装しない (フォーカス移動なし、または明示的な代替提示のいずれか)。CE: 返却先が消えた場合に無条件で Shell 自身へフォーカスを移す実装は glance-primary (非アクティベート優先) の原則に反し fail する。
- **注**: engage 中 user が Alt+Tab で意図的に別窓へ切り替えた場合の Then 詳細は DP-ENGAGE-FOCUS-RETURN-INTENTIONAL-SWITCH の consultation 結果で確定する。Explore 段階では scenario を先取り固定しない (pass2 issue 8 反映)。

### F-005 ジャンプバック (外部窓アクティベーション、lens 中立)
Entry: [Jump back] クリック → staged best-effort (HWND → プロセス+タイトルマッチ → 失敗)。Exit: 対象窓 foreground、または明示的『見つからない』表示。

- **UAC-009**: Given: 対象 Windows Terminal 窓の HWND が既知かつ生存している。When: [Jump back] を押下する。Then: 対象窓が foreground になり、supervision surface は最前面に残らない。CE: supervision surface 自身が常に最前面を維持し続ける実装は対象窓のフォアグラウンド化を妨げ fail する。design_handoff_refs: handoff-jump-back-target-provenance。
- **UAC-010**: Given: HWND もプロセス+タイトルマッチも失敗する。When: [Jump back] を押下する。Then: 『ジャンプ先が見つかりません』の明示表示が出て、フォーカス移動は発生しない。CE(A): 任意の既定窓 (例: 最後に activate した窓) へフォーカスを移す fabricated fallback は誤った窓へ復帰させ fail する。CE(B): 何も表示せず無反応のままにする実装は失敗を隠蔽し fail する。design_handoff_refs: handoff-jump-back-target-provenance。

### F-006 Workspace 深掘り (窓再利用・脱ブラウザ、lens 中立だが panel-primary 下では低頻度)
Entry: supervision surface の [Open] または deep link (contract dependency: Phase 1 deep-links.schema.json) → named pipe openSession → window-registry。Exit: 既存窓 focus (窓数不変) または新規生成、ブラウザ非経由。

- **UAC-011** (vs_legacy=must-fail): Given: session X の Workspace 窓が既に開いている。When: 同じ session X を再度 [Open] する。Then: Workspace 窓数は増えず既存窓がフォーカスされる。CE: クリックの都度無条件で新規 BrowserWindow を生成する実装は窓数が増加し『タブ工場』化し fail する。
- **UAC-012** (vs_legacy=must-fail、pass2 issue 7 反映で accelerator 観察追加): Given: Workspace 窓が未生成。When: 初回 [Open] する。Then: 生成された窓にアドレスバー・タブバーが表示されず、既定ブラウザプロセスは起動せず、かつ Ctrl+T (新規タブ) や Ctrl+L (アドレスバーフォーカス) 等の Chromium 既定 accelerator を押下しても何も起こらない。CE(A): 既定ブラウザで clients/ui URL を開く実装は cc-no-browser-in-local-flow を破り fail する。CE(B): アドレスバー・タブバーの DOM を隠すが Chromium accelerator (Ctrl+T / Ctrl+L) が生きている実装は追加 Then の observation を fail する (legacy_context.replaced_behaviors のブラウザショートカット除去を must-fail scenario でカバー)。

### F-007 Toast fallback (panel-primary lens 下では panel-unwatched-only 短文 approve/deny、pass2 issue 2 反映で観察不能語彙除去)
Entry: 承認/質問イベント → supervision surface 更新 + DP-TOAST-FALLBACK-CONDITION=OPT-PANEL-UNWATCHED-ONLY (推奨) 条件成立時に AppNotification 発行 (contract dependency: Phase 1 notifications.schema.json)。Exit: 通知センターに toast 残存、[Approve]/inline textbox から短文応答完結 (panel 側キューも同時消失)。

- **UAC-013** (離席復帰): Given: PC 離席中に承認が発生する。When: 復帰後に通知センターを確認する。Then: toast が残存しており、[Approve] 押下で supervision surface のキューも同時に消失する。CE: 一過性ポップアップのみで通知センターに残さない実装は離席復帰時に見逃され fail する。
- **UAC-014** (pass2 issue 2 反映で observable 化): Given: panel フライアウトが展開されており、かつ別アプリ foreground でも DND でもロック中でもない (DP-TOAST-FALLBACK-CONDITION=OPT-PANEL-UNWATCHED-ONLY 選定時の判定条件と同じ observable 語彙)。When: 承認イベントが発生する。Then: toast は発行されず panel キューのみ更新される。CE: この条件下でも無条件で toast を発行する実装は二重専有による通知疲弊を招き、OPT-PANEL-UNWATCHED-ONLY の判別条件で fail する。

### F-008 デーモン再起動耐性と自動再接続 (両 lens 共通、pass2 issue 1 反映で reconnect delay threshold 参照)
Entry: Shell メニューから『Restart daemon』選択 → graceful shutdown → セッション永続化 → 再 spawn → WS 再接続 (contract dependency: Phase 1 reconnect-contract.md / ADR-0025 拡張)。Exit: 健全性表示 Connected 復帰 + 再起動前と同じ session 群が両サーフェスで観察できる + 手動再接続不要 + 復帰までの遅延が threshold-daemon-restart-reconnect-delay 以内。

- **UAC-015**: Given: 3 session が稼働中である。When: Restart daemon を実行する。Then: 完了後に健全性表示が threshold-daemon-restart-reconnect-delay (S1 実装後実測で確定、value 未確定) 以内に Connected へ復帰し、再起動前と同じ 3 session が supervision surface と Workspace 双方で観察でき、手動再接続操作は不要。CE(A): 自動 WS 再接続を実装せず『切断』表示のまま固定される実装は手動操作を要求し fail する。CE(B): Restart 完了後 threshold-daemon-restart-reconnect-delay を超えて Connected へ戻らない実装 (WS 再接続バックオフが極端に長い) は新設 threshold の判別条件で fail する。ADR-0025 backfill のネイティブクライアント適用判別。

### F-101 toast 単独承認 (toast-primary lens 主動線、deferred)
Entry: 承認待ち → AppNotification ボタン付き toast 表示 (フォアグラウンド窓のフォーカスを奪わない)。Exit: [Approve]/[Deny] 押下で toast が確定表示 (承認済み) へ更新、元アプリフォーカス保持、agent 実行再開/停止、Workspace 窓もブラウザも起動しない。

- **UAC-101**: Given: エディタ (VS Code) が foreground、承認待ちイベントが 1 件発生し AppNotification toast が表示されている。When: ユーザーが toast 上の [Approve] を選択する。Then: threshold-toast-response-latency (5s 暫定) 以内に toast が確定表示へ更新され、VS Code のフォーカスは維持され、Workspace 窓もブラウザも起動しない。CE: Shell が toast 押下時にまず自身の窓を前面化してから承認 API を叩く実装は、VS Code のフォアグラウンドフォーカスを一瞬奪い fail する。
- **UAC-102** (Action Center 残存): Given: toast が Action Center に落ちた後、承認待ちのまま。When: Action Center 上で [Deny] を押下する。Then: ポップアップ時と同じ結果 (拒否確定) + supervision surface のキュー消失。CE: Action Center 上ではボタン無反応で操作できない実装は fail する。

### F-102 toast inline textbox 質問応答 (toast-primary lens 主動線、deferred)
Entry: 質問 → inline textbox 付き toast。Exit: textbox 入力 + Enter/送信で確定、panel/Workspace 非起動。

- **UAC-103**: Given: textbox 入力可能な短文の質問 toast が表示されている (IME 変換不要な条件。具体条件は assumption-appnotification-textbox-ime の S3 検証後に確定)。When: ユーザーが短文を入力し Enter で送信する。Then: toast が確定表示へ更新され、panel/Workspace は一切展開されない。CE: 送信操作を下書き保存のみに留め実際の回答 API を呼ばない実装は agent 側が回答を受理できず fail する。
- **UAC-104x** (誤操作): Given: toast textbox に文字を入力中。When: 確定前にユーザーが toast を閉じる。Then: 未確定の入力内容は破棄され、質問キューは未応答のまま残る (勝手に空回答を送信しない)。CE: toast close イベントを空文字列の回答送信に変換する実装は agent へ意図しない空回答が送られ fail する。

### F-103 長文/入力方式制約による panel 展開 fallback (toast-primary lens の 2 段目、deferred)
Entry: 質問 toast → textbox 入力可能条件超過 → DP-TOAST-FALLBACK-AFFORDANCE=OPT-INLINE-PANEL-LINK (推奨) 選定時は toast 内『panel で回答』affordance が現れる → panel 展開。Exit: panel 内テキスト欄で入力完了+送信。

- **UAC-105**: Given: toast textbox で長文または日本語 IME 変換を要する回答を試みている。When: 文字数上限超過または IME 変換操作を行う。Then: toast 上に『panel で回答』affordance が現れ、選択すると panel が召喚され入力欄にフォーカスが移る。toast 側の未確定文字列は破棄される。CE: 上限超過分を黙って切り詰めて toast から送信する実装は agent が意図と異なる切り詰め回答を受理し fail する。
- **UAC-106**: Given: panel 展開で回答入力中。When: 送信を確定する。Then: panel のキュー項目が消え、toast 側 (まだ残っていれば) も同時に確定表示へ更新される。CE: panel 側の送信が toast 側の UI 状態を更新しない実装は、toast が未確定のまま残りユーザーが再度操作しようとして混乱するため fail する (二重応答の温床)。

### F-104 Workspace エスカレーション (toast-primary lens の 3 段目、deferred)
Entry: panel でも収まらない長文/複数行 → [Open in Workspace]。Exit: 既存窓 activate (窓数不変) または新規生成 1 窓。

- **UAC-107** (vs_legacy=must-fail): Given: 対象 session の workspace anchor 窓が既に開いている。When: panel の [Open in Workspace] を選択する。Then: 既存窓が foreground 化されるだけで、窓数は増えない (タスクバー/Alt+Tab の窓数不変)。CE: 常に新規 BrowserWindow を生成する実装は『窓工場』化し fail する。
- **UAC-108** (vs_legacy=must-fail): Given: 対象 session の workspace anchor 窓が未生成。When: panel の [Open in Workspace] を選択する。Then: 新規に 1 窓生成され foreground 化、ブラウザは一切起動しない。CE: 既定ブラウザで clients/ui URL を開く実装は cc-no-browser-in-local-flow を破り fail する。

### F-105 panel 召喚 (toast-primary lens の overview、DP-PANEL-DEFAULT-VISIBILITY=OPT-SUMMON-ONLY 選定時に主動線)
Entry: OPT-SUMMON-ONLY 選定時は Shell 起動直後は panel/上端バー未描画 (トレイのみ可視)。トレイクリック/hotkey → フライアウト展開。OPT-ALWAYS-VISIBLE-BAR 選定時は Shell 起動直後から常時可視。Exit: overview (承認/質問キュー + 直近 session 状態要約) 表示。

- **UAC-109** (OPT-SUMMON-ONLY branch): Given: Shell 起動後、承認/質問イベント未発生、panel 未召喚。When: ユーザーが何も操作しない。Then: 画面上に panel/バーは一切描画されず、トレイアイコンのみ可視。CE: 既定で表示し初回だけオプトアウト可能な実装は召喚型 (OPT-SUMMON-ONLY) の『既定非表示』が観察できず fail する。
- **UAC-110**: Given: panel 非表示状態、承認 2 件・質問 1 件がキューにある。When: トレイアイコンをクリックする。Then: フライアウトが展開し、承認/質問の件数と直近 session 状態要約が読み取れる。CE: Workspace 窓を前面化するだけで overview を表示しない実装は panel の役割 (overview) を果たせず fail する。

### F-107 Workspace 窓 close ≠ session 停止 (両 lens 共通 invariant)
Entry: session 生存中の Workspace 窓を close → agent 継続 → [Open] または deep link で再オープン。Exit: 再オープン後、状態復元 (assumption-workspace-window-restore-fidelity 依存)。

- **UAC-111** (vs_legacy=must-fail): Given: anchor 窓 close 直後。When: supervision surface (トレイ/toast/panel) で session 状態を確認する。Then: session 状態は running/waiting 継続で表示され、'ended'/'terminated' へ遷移しない。CE: close イベントを stop 要求に変換する実装は goal-workspace-window-anchor の close 独立性を崩し fail する。
- **UAC-112**: Given: anchor 窓 close 後、再度 toast または panel から該当 session を開く。When: 再オープン操作を行う。Then: 既存の窓状態が概ね復元されて foreground 化され、重複窓は生成されない (窓数不変)。CE: 毎回新規 BrowserWindow を生成する実装は窓数が増え続け fail する。

### F-108 デーモン監督 toast 予算保護 (両 lens 共通、DP-DAEMON-HEALTH-VISIBILITY=OPT-TRAY-ONLY defaulted)
Entry: デーモン Healthy 到達 → トレイアイコン外観反映。Exit: Degraded/失敗以外は toast 未発行、状態はトレイのみで読み取れる。

- **UAC-113**: Given: Healthy 継続中に threshold-daemon-healthy-toast-budget-window (5min 暫定) の観察窓経過 + 承認/質問イベント 0 件。When: 観察窓を通じて Windows 通知履歴を確認する。Then: supervision 目的以外 toast 0 件、トレイ外観のみで Healthy 読み取り可能。CE: health check 成功のたびに『daemon healthy』toast を発行する実装は観察窓内で toast 件数 > 0 となり goal-supervision-toast-budget の判別条件で fail する。
- **UAC-114**: Given: デーモンが Degraded へ遷移する。When: トレイアイコンにカーソルを合わせる。Then: アイコン外観が警告色へ変化し、ツールチップで『Degraded』相当の文言が読み取れる。CE: 外観無変化で panel を展開しないと状態が分からない実装は tray-only observable 契約を満たさず fail する。

## Cross-Flow Consistency (統一語彙)

- 承認: approve / deny (両 lens 共通)。「承認」「拒否」は日本語補助表現。
- 質問応答: engage (panel 経路、テキスト入力モード、panel-primary 主動線) / inline textbox (toast 経路、panel-primary 下では短文 approve/deny fallback)。
- 復帰: jump-back (外部窓へ復帰)。panel/toast どちらから発火しても同じ contract に集約。
- 常時可視: glance (非アクティベート観察、panel-primary lens の中核概念、DP-PANEL-DEFAULT-VISIBILITY + DP-PANEL-GLANCE-CONTENT-SCOPE の結果で実装形状が確定)。
- キャンセル/戻る: Esc は engage キャンセル、panel 外側クリック/無操作は panel 非表示化 (OPT-ALWAYS-VISIBLE-BAR 選定下ではフライアウト非表示化のみ)。
- フォーカス規律: engage 開始前 foreground 窓を record し確定/Esc 時に restore (DP-PANEL-ENGAGE-FOCUS-RETURN=OPT-CAPTURE-RESTORE、r2 consultation で answered)。engage 中 Alt+Tab 意図切替の扱いは DP-ENGAGE-FOCUS-RETURN-INTENTIONAL-SWITCH=OPT-RESPECT-INTENTIONAL-SWITCH で確定 (r1)。ジャンプバック成功時は対象外部窓へ foreground 移動、supervision surface は最前面に残らない。
- resolved-by-other: 契約層 payload 名。UI 側の観察は『既に処理済み』表示のみ。
- Modality tagging: 全 flow の steps 先頭に `[system|glance|pointer|keyboard]` を明示し、pointer/keyboard 両モダリティ到達性を pass2 issue 4 準拠で保持する。

## Contract Dependency Notes

以下 flow の Given/When/Then は Phase 0/1 契約に依存し、契約の実際の shape 確定後に再照合が必要:
- **Phase 0 approval/question domain**: F-003, F-004, F-007, F-101, F-102, F-103
- **Phase 1 approval-contract.md / question-contract.md**: F-003, F-004, F-101, F-102, F-103
- **Phase 1 notifications.schema.json**: F-007, F-101, F-102, F-103, F-108
- **Phase 1 deep-links.schema.json**: F-005, F-006, F-104, F-107
- **Phase 1 reconnect-contract.md / ADR-0025 拡張**: F-008 (デーモン再起動耐性、threshold-daemon-restart-reconnect-delay 具体値含む)、F-107 (Workspace 再オープン時の再接続)

## Open Questions

- S3 実装直前の先行 prototype: assumption-appnotification-textbox-ime (AppNotification inline textbox の IME 変換対応可否) と assumption-com-background-activation-unpackaged (unpackaged 構成での COM background activation 遅延) を先行検証し、invalidation_behavior を発動させるか判断する。結果次第で ALT-TOAST-PRIMARY-ANCHOR-WORKSPACE の deferred 復帰余地および DP-SUPERVISION-PRIMARY-ENTRY の逆転可能性を評価する。
- assumption-fullscreen-panel-coexistence — DP-PANEL-DEFAULT-VISIBILITY が OPT-ALWAYS-VISIBLE-BAR で確定した場合、S3 実装時に副作用が観察されたら fullscreen 中 auto-hide 副契約 (F-002 UAC-004a の Then 拡張) を Specify に持ち越し。
- assumption-panel-completes-majority — DP-WORKSPACE-WINDOW-LIFECYCLE=OPT-ON-DEMAND (panel-primary lens 選定時) の中核仮説。S4/S5 実装後に Workspace 窓オープン頻度を観察し、頻度が高い場合は always-open-anchor への切り替えを user に諮る。
- assumption-workspace-window-restore-fidelity — F-107 UAC-112 の Then 詳細度は Playwright for Electron 検証結果で確定。invalidation 時は Then 緩和ルートを Specify に持ち越し。
- threshold-daemon-restart-reconnect-delay の具体値は ADR-0025 拡張の再接続バックオフ設計が確定するまで未定 (value=null、status=proposed)。S1 実装後の実測で確定する。
- threshold-daemon-healthy-toast-budget-window の 5min はまだ arbitrary_precision の疑いあり (critic pass2 issue 5)。S3-S5 実装後の実運用ログで再確定する。
- 全 threshold は status=proposed。S3-S5 実装後に validation_method に沿って確定値を差し戻す。
- contract_dependency: Phase 0 (approval/question domain) と Phase 1 (approval-contract.md, question-contract.md, notifications.schema.json, deep-links.schema.json, reconnect-contract.md) の shape 確定後、F-003/F-004/F-005/F-006/F-007/F-008/F-101/F-102/F-103/F-104/F-107 の Given/When/Then を再照合する必要がある。
- panel-primary lens の下位トポロジーとして draft-1 が持っていた ALT-PANEL-PRIMARY-FLYOUT-ONLY-SUMMON (上端バー廃止・トレイフライアウトのみ) の扱い — 現時点では DP-PANEL-DEFAULT-VISIBILITY=OPT-SUMMON-ONLY で表現可能とみなし独立 alternative としては保持しない。user が『panel-primary だが常時バーは不要』へ切り替える余地は DP-PANEL-DEFAULT-VISIBILITY で拾う。
- ALT-TOAST-ONLY-MINIMAL-NO-PANEL (draft-2 の 2 番目 alternative、panel サーフェス自体を持たない minimum lens) の扱い — 現時点では ALT-TOAST-PRIMARY-ANCHOR-WORKSPACE の下位トポロジー (panel=none) として保持せず、DP-PANEL-DEFAULT-VISIBILITY=OPT-SUMMON-ONLY 相当で近似する。復帰余地は open として保持。

<!-- requirements-decisions:start -->
## Decision State
Selected alternative: ALT-PANEL-PRIMARY-DUAL-SURFACE
Decision DP-DAEMON-HEALTH-VISIBILITY: OPT-TRAY-ONLY
Decision DP-ENGAGE-FOCUS-RETURN-INTENTIONAL-SWITCH: OPT-RESPECT-INTENTIONAL-SWITCH
Decision DP-HOSTED-MODE-FRAME-INTEGRATION: OPT-INCREMENTAL-DEFRAME
Decision DP-JUMP-BACK-TARGET-PROVENANCE: OPT-STAGED-BEST-EFFORT
Decision DP-PANEL-DEFAULT-VISIBILITY: OPT-ALWAYS-VISIBLE-BAR
Decision DP-PANEL-ENGAGE-FOCUS-RETURN: OPT-CAPTURE-RESTORE
Decision DP-PANEL-GLANCE-CONTENT-SCOPE: OPT-COUNTS-PLUS-LATEST
Decision DP-SUPERVISION-PRIMARY-ENTRY: OPT-PANEL-PRIMARY
Decision DP-TOAST-FALLBACK-AFFORDANCE: OPT-INLINE-PANEL-LINK
Decision DP-TOAST-FALLBACK-CONDITION: OPT-PANEL-UNWATCHED-ONLY
Decision DP-TOAST-PERSISTENCE-FALLBACK: OPT-3-STAGE-LADDER
Decision DP-WORKSPACE-WINDOW-LIFECYCLE: OPT-ON-DEMAND
<!-- requirements-decisions:end -->
