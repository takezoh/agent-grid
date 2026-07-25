---
change: change-20260725-vibe-island-human-input-contract
role: requirements
functional_requirements:
- id: FR-001
  type: event_driven
  statement: 承認または質問の要求が到着したとき、島は自動展開して種別色のカードと件数バッジを表示し、フォーカスを移動しない。
  priority: must
  rationale: UAC-001 / UAC-010 (exp-approval-roundtrip / exp-question-inline-reply の entry 観察)
- id: FR-002
  type: event_driven
  statement: ユーザーが応答操作 (Allow/Deny/回答送信) を行ったとき、UI は楽観反映でカードを即時除去し、送信失敗時はカードを復元してオフラインバッジで可視化する。
  priority: must
  rationale: UAC-002 / UAC-003 (contract-approve-submission-rollback の契約化)
- id: FR-003
  type: ubiquitous
  statement: システムは 1 つの human-input 要求につき解決を正確に 1 回だけエージェントの保留 JSON-RPC へ返信し、後着の応答には resolved_by_other を返す。
  priority: must
  rationale: UAC-020 / UAC-021 (単一解決 invariant — ubiquitous)
- id: FR-004
  type: state_driven
  statement: パネルが注視されていない間、システムは新規承認 1 件につき最大 1 回だけトーストを発火し、注視中・DND 中は発火しない。
  priority: must
  rationale: UAC-030 / UAC-032 (state_driven invariant)
- id: FR-005
  type: event_driven
  statement: 承認または質問が 30 秒間未応答のまま期限に達したとき、システムは作成時に捕捉した既定決定で自動解決し、カードを除去する。
  priority: must
  rationale: UAC-041 (threshold-approval-ttl、TOCTOU-free な既定適用)
- id: FR-006
  type: event_driven
  statement: 質問回答の送信が完了したとき、システムは返信欄をクリアし、捕捉済みの直前フォアグラウンドウィンドウが生存していればフォーカスを返却する。
  priority: must
  rationale: UAC-011 (DP-FOCUS-DISCIPLINE normative)
- id: FR-007
  type: state_driven
  statement: pending が 0 の間、島は承認/質問の UI 要素を UI ツリーに生成しない。
  priority: must
  rationale: UAC-040 / UAC-042 (healthy is silent — 要素の非存在まで要求)
- id: FR-008
  type: unwanted
  statement: 再接続時、システムは既に解決済みの要求のカードやトーストを再提示してはならない。
  priority: must
  rationale: UAC-043 (非捏造。未解決 pending の replay 導入後もこの禁止は維持される)
non_functional_requirements:
- id: NFR-001
  type: reliability
  criteria: 複数クライアントの同時応答・切断・期限切れが重なっても、エージェントへの決定返信は要求ごとに正確に 1 回であること
  measurement: state reducer の並行応答テストと gateway scenario テストでの返信回数 assert
- id: NFR-002
  type: usability
  criteria: pending の到着・表示・解決のどの時点でも作業中アプリのフォーカスを奪わないこと
  measurement: WS_EX_NOACTIVATE 属性の unit assert + UIA フォーカスイベントの e2e/手動観察
acceptance:
- id: UAC-001
  given: 島がコンパクト表示で監督者が別アプリで入力中
  when: Codex が承認を要求する
  then: 自動展開・警告色カード・バッジが表示され、キャレットは移動しない
  requirement_refs: [FR-001]
- id: UAC-002
  given: 承認カード表示中
  when: Allow (Ctrl+Y) を操作する
  then: カードは即時に消え、同一要求は再出現せず、セッションが実行中表示に戻る
  requirement_refs: [FR-002, FR-003]
- id: UAC-003
  given: ネットワーク断でカード表示中
  when: Allow を操作する
  then: カードが復元されオフラインバッジが表示され、再試行できる
  requirement_refs: [FR-002]
- id: UAC-010
  given: pending なしで別アプリ入力中
  when: 質問が到着する
  then: Question カードと返信欄が表示され、自動フォーカスされない
  requirement_refs: [FR-001]
- id: UAC-011
  given: 質問カードと返信欄が表示中
  when: IME 入力で回答し Enter を押す
  then: 確定文字列が送信され、返信欄がクリアされ、フォーカスが直前のアプリへ返る
  requirement_refs: [FR-006]
- id: UAC-020
  given: 同一承認が島とブラウザに表示中
  when: ブラウザが先に許可する
  then: 島のカードが消え Handled elsewhere 通知が表示され、トーストは発火しない
  requirement_refs: [FR-003]
- id: UAC-021
  given: ブラウザが許可した直後
  when: 島で Deny して競り負ける
  then: エラーダイアログなしで Handled elsewhere になり、先着の許可だけが効く
  requirement_refs: [FR-003]
- id: UAC-030
  given: パネル非注視・非 DND
  when: 承認が到着する
  then: Approve/Deny ボタン付きトーストが 1 回だけ表示される
  requirement_refs: [FR-004]
- id: UAC-031
  given: 承認トースト表示中
  when: トーストの Approve を押す
  then: パネルと同一経路で解決され、島のカードも消える
  requirement_refs: [FR-003, FR-004]
- id: UAC-032
  given: 展開パネル注視中
  when: 承認が到着する
  then: トーストは発火せずパネルのカードのみで通知される
  requirement_refs: [FR-004]
- id: UAC-040
  given: 自動展開中で pending 1 件
  when: 解決する
  then: 島は自動収納される。ユーザー展開分は収納されない
  requirement_refs: [FR-007]
- id: UAC-041
  given: 承認カードを放置
  when: 30 秒経過する
  then: カードが消え、既定決定がエージェントに適用される
  requirement_refs: [FR-005]
- id: UAC-042
  given: pending も異常もない
  when: UI と支援技術で走査する
  then: コンパクトバー 1 行のみで、承認/質問要素は UIA ツリーに存在しない
  requirement_refs: [FR-007]
- id: UAC-043
  given: 切断中に発生し既解決の承認がある
  when: 再接続する
  then: 過去要求のカード/トーストは再提示されない
  requirement_refs: [FR-008]
---

<!-- lifecycle is owned by change.md -->

# Requirements

## Content

本 change の要求は 2 層で表現される。体験契約 (UX) と、それを技術契約へ翻訳した EARS 機能要件 (frontmatter) である。

- [ux.md](ux.md) — /requirements (specify mode) の成果物。5 primary flows / 14 acceptance scenarios (全 `vs_legacy: must-pass`) / edge cases / open questions。
- [requirements-plan.json](requirements-plan.json) — canonical PLAN_UX。experience contracts、coverage claims、decision points、observable state contracts、design handoffs、provenance を保持する。
- [design-plan.json](design-plan.json) — canonical design PLAN。frontmatter の EARS / NFR / acceptance はここからの投影で、implementation contract と witness は [implementation.md](implementation.md) が保持する。

## EARS 機能要件 (要旨)

frontmatter `functional_requirements` が正本。invariant 型は FR-003 (ubiquitous: 単一解決)、FR-004 / FR-007 (state_driven: トースト上限・要素非存在) の 3 件で、ux の counterexample (二重通知・ゾンビカード・silent failure・フォーカス奪取) を否定形で拘束する。FR-008 (unwanted) は再接続時の非捏造を禁止形で固定し、pending replay (後続 change) 導入後も維持される。

数値契約は 2 点のみ: TTL 30 秒 (FR-005、実装確認済み) とトースト 1 要求 1 回 (FR-004)。いずれも requirements 所有の確定値である。

スコープ外 (slice-gap-closure、本 change では実装しない): 接続時 pending replay (DP-PENDING-REPLAY = OPT-REPLAY-ON-CONNECT 決定済み)、選択肢付き質問 (DP-QUESTION-OPTIONS = OPT-OPTION-BUTTONS 決定済み)、Claude 承認の一級化 (DP-CLAUDE-FIRST-CLASS — design 検証を先行し可否判明後に再相談)。相談記録: [consultations/20260725-unresolved-dps.md](consultations/20260725-unresolved-dps.md)。
