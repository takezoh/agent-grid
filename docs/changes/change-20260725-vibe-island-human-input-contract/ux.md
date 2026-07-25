---
change: change-20260725-vibe-island-human-input-contract
role: ux
phase: specify
goal: >-
  Codex エージェントのパーミッション要求 (コマンド実行/ファイル変更) とユーザー質問を、
  Vibe Island (Windows Shell) がフォーカスを奪わずネイティブに受けて応答往復を完了させる体験を、
  実装調査 (2026-07-25、gateway〜島の全経路) で検証済みの観察可能な契約として固定する。
  実装変更は伴わない契約 pin であり、調査で判明した 3 つの欠落
  (再接続 replay / Claude 承認の一級化 / 選択肢付き質問) は未解決 decision point として明示的に保持する。
target_users:
- Windows デスクトップで複数のエージェントセッション (Codex / Claude) を並行監督しながら別アプリで作業する開発者
primary_flows:
- id: F-001
  name: 承認要求の到着と島での解決
  steps:
  - '[system] Codex がコマンド実行/ファイル変更の承認を要求すると、島のコンパクトバーに件数バッジが付き、収納中なら自動で展開する'
  - '[system] 展開パネルに警告色の「Permission request」カード (要求概要・セッション短縮 ID・Deny/Allow ボタン) が表示される。フォーカスは移動しない'
  - '[pointer-mouse|keyboard] Allow (Ctrl+Y) または Deny (Ctrl+N) を 1 操作で実行する'
  - '[system] カードが即時に消え、pending が 0 なら自動展開分は収納される。許可時はセッションが実行中表示に戻る'
- id: F-002
  name: 質問への島内 inline 返信
  steps:
  - '[system] Codex の質問が到着すると、情報色の「Question」カードと返信欄・「Reply below — Enter sends」ヒントが表示される。フォーカスは移動しない'
  - '[pointer-mouse] 返信欄をクリック (この時点で初めて島が入力を受け付ける)'
  - '[keyboard] IME を含む通常入力で回答を書き、Enter (または送信ボタン) で送信する'
  - '[system] カードが消え、返信欄がクリアされ、フォーカスは直前に使っていたアプリへ返る'
- id: F-003
  name: 複数クライアント間の単一解決
  steps:
  - '[system] 同じ承認要求が島とブラウザ UI (Workspace) の両方に表示されている'
  - '[pointer-mouse] いずれかの面で許可または拒否を操作する'
  - '[system] 先着の決定だけがエージェントに適用され、他の面ではカードが「Handled elsewhere」通知に置き換わる'
- id: F-004
  name: パネル非注視時のトースト経由承認
  steps:
  - '[system] パネルが注視されていない状態で承認が到着すると、Windows トーストが 1 回だけ表示される (Approve/Deny ボタン付き)'
  - '[pointer-mouse] トーストのボタンで応答する'
  - '[system] パネルと同一の応答経路で解決され、島のカードも消える'
- id: F-005
  name: 静穏・自動整理・期限切れ
  steps:
  - '[system] pending がすべて解決されると、自動展開されていた島はコンパクトへ自動収納される'
  - '[system] 30 秒間応答されなかった承認/質問は既定の決定で自動解決され、カードは消える'
  - '[system] 平常時はコンパクトバー 1 行のみが残り、承認/質問 UI は存在しない'
acceptance_scenarios:
- id: UAC-001
  flow_ref: F-001
  given: 島がコンパクト表示で、監督者は別アプリでテキスト入力中
  when: Codex がコマンド実行の承認を要求する
  then: 島が自動展開して警告色の「Permission request」カードに要求概要 (実行しようとするコマンド) が表示され、コンパクトバーには件数バッジが付く。監督者のキャレットは作業中アプリに残ったままで、入力は中断されない
  counterexample: バッジ数だけ増やして展開しない実装も、到着時に島へフォーカスを移す実装も、それぞれ展開観察とキャレット観察で fail する
  vs_legacy: must-pass
- id: UAC-002
  flow_ref: F-001
  given: 「Permission request」カードが表示されている
  when: Allow ボタンを押す (または Ctrl+Y)
  then: カードは即時に消え、同じ要求のカード/トーストは再出現しない。該当セッションのチップが待機表示から実行中表示に戻る
  counterexample: サーバー応答までカードを残すブロッキング実装は「即時に消える」で、ローカルで消すだけの実装はセッションが待機のままになることで fail する
  vs_legacy: must-pass
- id: UAC-003
  flow_ref: F-001
  given: カード表示中にネットワークが切断されている
  when: Allow を押す
  then: カードは一旦消えた後に復元表示され、オフラインバッジが現れる。復元されたカードから再試行できる
  counterexample: 失敗を握りつぶしてカードを消したままにする silent-failure 実装は復元観察で fail する
  vs_legacy: must-pass
- id: UAC-010
  flow_ref: F-002
  given: 島に pending がなく、監督者は別アプリで入力中
  when: Codex が自由文の質問を送る
  then: 島が展開して「Question」カードに質問文が表示され、返信欄と「Reply below — Enter sends」ヒントが現れる。返信欄へ自動フォーカスはされず、作業中アプリの入力は続行できる
  counterexample: 質問到着で返信欄へ自動フォーカスする「気が利く」誤実装は、作業中アプリの入力中断として観察され fail する
  vs_legacy: must-pass
- id: UAC-011
  flow_ref: F-002
  given: 質問カードと返信欄が表示されている
  when: 返信欄をクリックし、日本語 (IME 変換確定を含む) で回答を入力して Enter を押す
  then: 確定済みの回答文がそのまま送信されてカードが消え、返信欄はクリアされる。フォーカスは直前に使っていたアプリへ戻り、そのままタイプを継続できる
  counterexample: 送信後フォーカスが島に残る実装は「直前のアプリでタイプ継続」で、IME 未確定文字列で送信する実装は確定文字列との差で fail する
  vs_legacy: must-pass
- id: UAC-020
  flow_ref: F-003
  given: 同じ承認が島とブラウザ UI の両方に見えている
  when: ブラウザ側で先に許可する
  then: 島のカードは消え、「Handled elsewhere」の通知行が表示される。島のトーストは発火しない
  counterexample: カードだけ黙って消す実装は「なぜ消えたか」を観察できず、通知行の存在確認で fail する
  vs_legacy: must-pass
- id: UAC-021
  flow_ref: F-003
  given: 同じ承認が両面に見えており、ブラウザ側が許可した直後
  when: ほぼ同時に島側で Deny を押す (競り負ける)
  then: エラーダイアログは出ず、島の表示は「Handled elsewhere」になる。エージェントには先着の決定 (許可) だけが効き、コマンドは実行される
  counterexample: 後着の Deny を上書き適用する実装はエージェント停止として、競合をエラーモーダルで提示する実装は「ダイアログなし」で fail する
  vs_legacy: must-pass
- id: UAC-030
  flow_ref: F-004
  given: 島のパネルが閉じられ注視されておらず、DND でもない
  when: 承認要求が到着する
  then: Approve/Deny ボタンを持つトーストが 1 回だけ表示される。同じ要求について 2 通目のトーストは出ない
  counterexample: スナップショット更新のたびにトーストを再発火する実装は 2 通目の出現で fail する
  vs_legacy: must-pass
- id: UAC-031
  flow_ref: F-004
  given: 承認トーストが表示されている
  when: トーストの Approve を押す
  then: 'パネルで許可した場合と同一の結果になる: 島のカードも消え、セッションが実行中表示に戻る'
  counterexample: トースト側だけを既読処理してサーバーへ送らない実装は、島のカードが残り続けることで fail する
  vs_legacy: must-pass
- id: UAC-032
  flow_ref: F-004
  given: 展開パネルを注視している
  when: 承認要求が到着する
  then: トーストは発火せず、パネルのカードだけで通知される
  counterexample: 注視状態を無視して常にトーストも出す実装は二重通知として観察され fail する
  vs_legacy: must-pass
- id: UAC-040
  flow_ref: F-005
  given: 承認到着により島が自動展開され、pending が 1 件だけ残っている
  when: その承認を解決する
  then: 島は自動でコンパクトへ戻る。ただし監督者が自分でコンパクトバーをクリックして展開していた場合は展開されたまま残る
  counterexample: 展開理由を区別せず一律自動収納する実装は、ユーザー展開が勝手に閉じることで fail する
  vs_legacy: must-pass
- id: UAC-041
  flow_ref: F-005
  given: 承認カードが表示されたまま誰も操作しない
  when: 30 秒の期限が経過する
  then: カードは自動で消え、エージェントには既定の決定 (通常は拒否) が適用される。島にエラーは表示されない
  counterexample: 期限切れ後もカードを残す実装は、操作しても効かないゾンビカードとして観察され fail する
  vs_legacy: must-pass
- id: UAC-042
  flow_ref: F-005
  given: pending もセッション異常もない
  when: 島を目視および支援技術で走査する
  then: コンパクトバーに「idle」等の 1 行のみがあり、承認ボタン・返信欄・空リストなどの承認/質問 UI 要素は画面にも支援技術ツリーにも存在しない
  counterexample: 空のリストや不可視ボタンを残す実装は目視では pass するが、支援技術ツリー走査で要素が発見され fail する
  vs_legacy: must-pass
- id: UAC-043
  flow_ref: F-005
  given: 島がサーバーと切断されている間に承認が発生し、ブラウザ側で解決済みになっている
  when: 島が再接続する
  then: 島は過去の要求のカードやトーストを捏造して表示しない (静穏なまま)。なお切断中に発生し未解決のまま残っている pending も現状は再接続直後には表示されない — この欠落は DP-PENDING-REPLAY (OPT-REPLAY-ON-CONNECT 決定済み) により後続 change で解消され、その際も本 scenario の非捏造不変条件は維持される
  counterexample: 再接続時にイベント履歴から既解決の要求を再構築して再表示・トースト再発火する実装は、済んだ判断の再提示として観察され fail する
  vs_legacy: must-pass
reference_ux:
- name: iOS Dynamic Island
  stance: modeled_on
  aspects:
  - 到着イベント時の自動展開と内容に密着する可変サイズ
  - 非侵襲な常駐 (フォーカス・視界を消費しない)
- name: Windows Action Center トースト慣行
  stance: modeled_on
  aspects:
  - ボタン付きトーストによる非注視時の応答
  - DND / Focus Assist の尊重
legacy_context:
  source_implementation: clients/windows-shell (AgentGrid.Shell.Core/Platform/WinUI) + src/host/state + src/server/api — 2026-07-25 実施の全経路実装調査に基づく as-is 契約の固定 (置換なし)
  inherited_behaviors:
  - 新規 pending 到着ごとの自動展開と、自動展開分のみの自動収納
  - 楽観送信 + 失敗時のカード復元 (contract-approve-submission-rollback)
  - 競合解決の「Handled elsewhere」正規化 (contract-resolved-by-other-display)
  - パネル非注視時のみ 1 要求 1 回のトースト、応答経路はパネルと共通
  - healthy is silent — 空セクションは UI ツリーに存在しない
  - 未応答 30 秒での既定決定による自動解決 (TTL)
  replaced_behaviors: []
states:
- '島の展開状態: expanded-initial → compact ⇄ expanded-auto / expanded-user (ユーザー展開は自動収納されない)'
- '承認/質問カード: visible → absent (本人決定/期限切れ) / visible → notice-handled-elsewhere (他クライアント解決) / 送信失敗時は visible へ復元'
edge_cases:
- '承認/質問が 30 秒間応答されない: カードが自動で消え、既定の決定がエージェントに適用される。島は静かに整理される'
- '応答送信時にネットワーク断: カードが復元されオフラインバッジが表示される。silent failure にならない'
- '複数クライアントがほぼ同時に応答: 先着のみ有効。競り負けた面はエラーではなく「Handled elsewhere」を観察する'
- 'WS 切断中に承認/質問が発生: 再接続後、既解決の要求は捏造再表示されない。未解決 pending の replay は導入決定済み (DP-PENDING-REPLAY) で後続 change が実装する'
assumptions:
- 承認/質問カードの供給源は Codex (app-server JSON-RPC 経由) のみであり、Claude セッションの permission_prompt はセッション待機表示にのみ反映される
- 質問は自由文 1 回答のみで、選択肢はワイヤにも島 UI にも存在しない
- 島・トースト・ブラウザは同一ユーザーの代替面であり、異なるユーザー間の承認競合は想定しない
technology_candidates:
- id: candidate-winui3-appnotification
  candidate: WinUI 3 / Windows App SDK + AppNotificationManager (実装済みスタック)
  source: repository_context
  proposed_for:
  - F-001
  - F-002
  - F-004
  - F-005
  required_capabilities:
  - WS_EX_NOACTIVATE の常駐 notch ウィンドウ
  - ボタン付きトーストと COM background activation
  - HWND リージョンクリップによるノッチ形状
  - UIA AutomationId 契約
  ux_constraints_relevant_to_evaluation:
  - フォーカス非奪取 invariant
  - トースト 1 要求 1 回
  - 空セクションの UI ツリー不存在
  design_questions:
  - COM activation 失敗時の縮退文言と再登録リトライの要否
  disqualifiers:
  - unpackaged で COM background activation が構造的に不能な環境では、トーストボタン応答を expand-only に縮退する
- id: candidate-flaui-uia3-e2e
  candidate: FlaUI / UIA3 (AutomationId 契約に基づく UI e2e)
  source: repository_context
  proposed_for:
  - F-001
  - F-002
  - F-005
  required_capabilities:
  - UIA ツリー走査による要素存在/不存在の検証
  - 実ウィンドウのフォーカス遷移観察
  ux_constraints_relevant_to_evaluation:
  - UAC-042 の『要素が存在しない』検証は UIA 走査でのみ判別可能
  design_questions:
  - フォーカス返却 (UAC-011) を CI 上の e2e で安定観察する方法
  disqualifiers:
  - run-dev ハーネス外での実行 (実サーバーへのテスト流入) は禁止
design_handoffs:
- id: handoff-uia-observation-contract
  required_outcome: 各 acceptance scenario の観察 (要素の存在/不存在・フォーカス遷移・通知発火) が、安定した UIA AutomationId 契約とテストシームによって機械検証可能であり続ける
  design_obligations:
  - AutomationId 契約 (StatusText / PendingList / ApproveButton / DenyButton / EngageBox / NoticeText / ConnectionText 等) の維持と、UI 変更時の契約更新手順の遵守
  - instrumentation (wire フレーム形・REST エンドポイント・reducer イベント) と UX 観察の対応関係を verification が参照できる形に保つ
  verification_obligations:
  - FlaUI/UIA e2e が AutomationId 契約経由で scenario を検証する
  - T0 unit tests (parser / reducer / presentation / state machine) が wire→表示の対応を pin する
  provenance:
    source: repository_evidence
    confidence: confirmed
    evidence_refs:
    - clients/windows-shell/docs/e2e.md
    - docs/note/note-20260724-windows-shell-ui-e2e-authoring.md
    rationale: 既存の AutomationId 契約と e2e 規約を scenario 検証のシームとして流用する
  decision_refs: []
- id: handoff-pending-replay
  required_outcome: 接続/再接続の直後に、その時点で未解決の承認/質問が島に表示され、既解決の要求は再表示されない
  design_obligations:
  - server の pending-human-input snapshot (placeholder) の本実装と、島側の接続時初期同期の設計
  - 既解決要求の重複排除 (replay とライブイベントの整合)
  verification_obligations:
  - Go gateway scenario test (subscribe → snapshot に pending が含まれる)
  - 島側 reducer unit test (snapshot 適用とライブイベントの合流)
  provenance:
    source: repository_evidence
    confidence: confirmed
    evidence_refs:
    - src/server/api/approval.go:117-123
    rationale: 欠落の存在は確認済み。導入判断は DP-PENDING-REPLAY
  decision_refs:
  - DP-PENDING-REPLAY
- id: handoff-claude-approval-source
  required_outcome: Claude セッションのパーミッション要求もカードとして表示・応答できる (実現可否と方式は design が判定)
  design_obligations:
  - Claude hook (PreToolUse の decision 制御) で保留→応答を仲介できるかの技術検証
  - 供給源非依存の ApprovalRequest 契約の維持 (TTL・既定決定・単一解決の意味論を Claude 側でも成立させる)
  verification_obligations:
  - driver contract test (Claude 供給源での ApprovalRequest 生成)
  - FakeVsReal e2e backstop (実 Claude Code との fidelity)
  provenance:
    source: planner_proposal
    confidence: provisional
    evidence_refs:
    - src/host/driver/claude_event.go:23-44
    rationale: 現状 status-only であることは確認済みだが、hook 仲介の実現可否は未検証
  decision_refs:
  - DP-CLAUDE-FIRST-CLASS
- id: handoff-question-options
  required_outcome: 選択肢付き質問を島でボタン選択でき、自由文 fallback も維持される
  design_obligations:
  - ワイヤ (qr フレーム/REST) への options 追加を後方互換で設計する
  - 島カードの選択肢描画・キーボード操作・AutomationId 契約の拡張
  verification_obligations:
  - wire codec の互換テスト (options なしクライアントが壊れない)
  - FlaUI e2e (選択肢ボタンの操作)
  provenance:
    source: planner_proposal
    confidence: provisional
    evidence_refs:
    - src/server/api/approval.go:65-112
    rationale: 自由文限定は確認済み。拡張形式は design 所有
  decision_refs:
  - DP-QUESTION-OPTIONS
---

<!-- lifecycle is owned by change.md -->

# Vibe Island human-input (approval/question) UX 契約

## Goal

Codex エージェントのパーミッション要求 (コマンド実行/ファイル変更) とユーザー質問を、Vibe Island (Windows Shell) がフォーカスを奪わずネイティブに受けて応答往復を完了させる体験を、実装調査 (2026-07-25、gateway〜島の全経路) で検証済みの観察可能な契約として固定する。実装変更は伴わない契約 pin であり、counterexample によって現状追認ではなく判別的な契約になっている。

体験は 4 つのユーザーゴールに紐づく: (1) 作業を離れず数秒・1 操作で承認する、(2) 質問に島内で回答し即座に元の作業へ戻る、(3) どの面から応答しても解決は 1 回だけと信頼できる、(4) 平常時は島が視界・フォーカス・通知を消費しない。

Modeled on: iOS Dynamic Island (到着イベント時の自動展開と内容密着サイズ / 非侵襲な常駐)、Windows Action Center トースト慣行 (ボタン付きトーストによる非注視時応答 / DND 尊重)。

Migration Context: 本契約は既存実装 (clients/windows-shell の AgentGrid.Shell.* + src/host/state + src/server/api) の as-is 固定であり、`replaced_behaviors` は空。継承する挙動は frontmatter `legacy_context.inherited_behaviors` のとおりで、全 scenario が `vs_legacy: must-pass` (現行実装が通るべき契約)。

<!-- requirements-decisions:start -->
## Decision State
Decision DP-CLAUDE-FIRST-CLASS: unresolved
Recommendation DP-CLAUDE-FIRST-CLASS: OPT-CLAUDE-HOOK-APPROVAL
Decision DP-FOCUS-DISCIPLINE: OPT-NOACTIVATE-RETURN
Decision DP-ISLAND-AUTOEXPAND: OPT-AUTOEXPAND-EACH
Decision DP-PENDING-REPLAY: OPT-REPLAY-ON-CONNECT
Decision DP-QUESTION-OPTIONS: OPT-OPTION-BUTTONS
Decision DP-SUBMIT-FEEDBACK: OPT-OPTIMISTIC-ROLLBACK
<!-- requirements-decisions:end -->

## Target Users

Windows デスクトップで複数のエージェントセッション (Codex / Claude) を並行監督しながら、別アプリで自分の作業を進める開発者。エージェントの停止 (承認待ち・質問待ち) に数十秒単位で気づき、作業文脈を壊さずに応答したい。

## Primary Flows

Delivery slice **initial (slice-asis-contract)** — 実装変更なしの契約固定。全 5 フロー (F-001〜F-005) が所属する:

- **F-001 承認要求の到着と島での解決** (exp-approval-roundtrip): 自動展開 → 警告色カード → 1 操作 (Allow/Deny, Ctrl+Y/N) → 即時消滅・自動収納。
- **F-002 質問への島内 inline 返信** (exp-question-inline-reply): 情報色カード + 返信欄 → 触れたときだけ入力可能化 (IME 可) → Enter 送信 → フォーカスを直前のアプリへ返却。
- **F-003 複数クライアント間の単一解決** (exp-single-resolution): 先着のみ有効。競り負け・他面解決は「Handled elsewhere」に正規化され、エラーダイアログを出さない。
- **F-004 パネル非注視時のトースト経由承認** (exp-approval-roundtrip): 1 要求 1 回のトースト。ボタンはパネルと同一経路・同一効果。注視中は発火しない。
- **F-005 静穏・自動整理・期限切れ** (exp-quiet-island): 自動展開分のみ自動収納。30 秒 TTL で既定決定へ。平常時は承認/質問 UI が UI ツリーに存在しない。再接続時に過去要求を捏造しない。

観察可能な状態機械 (実装非依存):

- 島の展開状態 `state-island-attention`: expanded-initial (初回起動の発見性) → compact ⇄ expanded-auto (新規 pending 到着ごとに再展開 / pending 0 で自動収納) / expanded-user (ユーザー操作でのみ開閉)。不変条件: ユーザー展開は自動収納されない。
- カード `state-pending-card`: visible → absent (本人決定 / 期限切れ) / → notice-handled-elsewhere (他面解決) / 送信失敗で visible へ復元。不変条件: 同一要求のカードは高々 1 枚。

Delivery slice **follow_up (slice-gap-closure)** — user consultation (2026-07-25) で導入決定済みの pending replay と選択肢付き質問を実装し、Claude 一級化は design 検証後に採否を再相談する (フローの具体化は後続 change の requirements で行う)。initial slice から明示的に除外する能力: 接続/再接続時の pending replay、Claude パーミッション要求の一級化、選択肢付き質問のボタン回答、Workspace (ブラウザ SPA) 側の並行 UX。

Reference UX trace: Dynamic Island の自動展開/内容密着 → F-001/F-005 (UAC-001, UAC-040, UAC-042)。トースト慣行 → F-004 (UAC-030, UAC-031, UAC-032)。

## Acceptance Scenarios

各 scenario は frontmatter `acceptance_scenarios` が正本 (Given-When-Then + counterexample)。本節は契約への trace を補足する。

- **UAC-001 / UAC-002 / UAC-003** (F-001, exp-approval-roundtrip): 到着の非侵襲通知、1 操作解決、送信失敗時の復元。coverage: surface=island-panel、input-kind=approval、resolution-outcome=accepted-locally / connection-lost。
- **UAC-010 / UAC-011** (F-002, exp-question-inline-reply): 到着時フォーカス非移動、IME 入力、送信後のフォーカス返却。coverage: input-kind=question。
- **UAC-020 / UAC-021** (F-003, exp-single-resolution): 他面解決の通知化、競合負けの正規化。coverage: resolution-outcome=resolved-by-other。
- **UAC-030 / UAC-031 / UAC-032** (F-004, exp-approval-roundtrip): トースト 1 回、同一経路、注視時抑制。coverage: surface=toast。
- **UAC-040 / UAC-041 / UAC-042 / UAC-043** (F-005, exp-quiet-island + exp-approval-roundtrip): 自動収納、TTL 期限切れ、静穏、再接続時の非捏造。coverage: resolution-outcome=expired / accepted-locally / connection-lost。

すべての scenario は `vs_legacy: must-pass` — 現行実装が通り、退行や「もっともらしい誤実装」(ブロッキング送信・自動フォーカス・二重通知・silent failure・ゾンビカード) が落ちる契約である。counterexample は各 scenario の frontmatter に併記した。

## Edge Cases

- 30 秒 TTL 期限切れ → 既定決定で自動解決 (UAC-041)。前提: TTL とその既定決定はサーバー実装で確認済みの確定値 (threshold-approval-ttl)。
- 送信時ネットワーク断 → 復元 + オフラインバッジ (UAC-003)。無効化条件: 楽観送信モデル (Decision State の DP-SUBMIT-FEEDBACK) を変更した場合はこの体験を再定義する。
- ほぼ同時応答の競合 → 「Handled elsewhere」(UAC-021)。前提: 島・トースト・ブラウザは同一ユーザーの代替面 (マルチユーザー承認が要件化されたら単一解決契約を再設計)。
- WS 切断中の発生 → 再接続後の非捏造。未解決 pending の replay は導入決定済みで後続 change が実装する (UAC-043)。replay 導入後も非捏造不変条件 (既解決分の再表示なし) は維持される。
- 供給源の前提: カードの供給源は Codex のみ。Claude は待機状態表示に留まる (一級化されたら全 flow を供給源非依存で再検証)。
- 質問形式の前提: 自由文のみ (選択肢導入時は F-002 を書き換え)。
- トースト COM activation が失敗する環境では、トーストは「Open panel」ボタンのみに縮退し、UAC-031 はパネル展開経由の応答として読み替える。

## Open Questions

2026-07-25 の user consultation ([consultations/20260725-unresolved-dps.md](consultations/20260725-unresolved-dps.md)) で pending replay と選択肢付き質問は導入決定済み (Decision State 参照)。残る未決:

- Claude パーミッション要求の一級化: hook 仲介の実現可否を design (handoff-claude-approval-source) で検証した後、採否を再相談する (consultation の方向性は一級化志向)。
- 否定役 (critic) pass 未実施: 本 plan はユーザー指示によりスキル/サブエージェントを使わず単一セッションで作成した。counterexample の判別性を独立検収する場合は /requirements の critique phase を別途実行する。
