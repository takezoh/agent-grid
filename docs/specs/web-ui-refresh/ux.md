---
id: ux-20260714-web-ui-refresh
kind: ux
title: Web UI Refresh — UX Plan (Linear/Vercel minimal)
status: draft
created: '2026-07-14'
goal: Web UI 全体を Linear/Vercel 系ミニマルへ全面刷新し、(a) ターミナルを常に主役とする chrome 設計、(b) 選択=塗り /
  フォーカス=リングの状態分離、(c) ActivityRail 解体によるモバイルのターミナル全幅回復、(d) パレット・ドロワー・エディターの器の近代化 (保存の発見可能性を含む)、(e)
  両テーマ・reduced-motion での品質均一化、を観測可能な振る舞いとして実現する。挙動実装 (focus trap / listbox / dialog
  / 保存安全機構 / ストアロジック) は無改変。
target_users:
- 複数の AI エージェントセッションを並行運用する開発者 (デスクトップ主戦場、キーボード操作・vim 含む)
- 外出先のモバイルから進行を監視し、たまに短い介入をする運用者
- スクリーンリーダー・キーボードのみで操作する支援技術ユーザー (既存 ARIA 契約の維持対象)
primary_flows:
- id: F-001
  name: サイドバーとセッション切替 — 状態の形態分離
  steps:
  - '[pointer-mouse|pointer-touch] セッション行をクリック/タップするとアクティブセッションが切り替わり、行が塗り (--accent-soft)
    の選択表示になる'
  - '[keyboard] Tab / 矢印でフォーカスを移動すると、フォーカスリングだけが移動し選択の塗りは動かない'
  - '[pointer-mouse|screen-reader] running/waiting/idle/stopped/pending がドットの色 + 形
    (stopped は輪郭のみ) + aria-label テキストで区別できる'
- id: F-002
  name: ヘッダの文脈提示とテーマ切替
  steps:
  - '[pointer-mouse] ヘッダに basename(project) / session title のパンくず + status pill +
    等幅メタが常時表示される'
  - '[pointer-mouse|keyboard] overflow (…) メニューから Theme を選ぶと即時反映され localStorage (agent-grid-theme)
    に永続化される'
  - '[pointer-touch] <768px ではハンバーガーが左端にあり、タップで既存 SessionDrawer が左からスライドインする'
- id: F-003
  name: ファイルアクティビティ導線 (2026-07-14 改訂 x2 — Workspace モードの Changes セクションへ統合)
  steps:
  - '[pointer-mouse] Workspace モードの右パネル上段 Changes セクションに M/A/D 行が並び、行クリックで該当ファイルが開く'
  - '[pointer-mouse] Terminal モードにはパネル類が一切無く、ターミナルがメイン領域全幅を使う'
  - '[pointer-touch] モバイルも同様: Workspace モードで変更一覧に到達する'
- id: F-004
  name: メインビュー — ヘッダ統合・タブ・ステータスバー
  steps:
  - '[pointer-mouse] 旧 DriverViewPanel の情報はヘッダ (title/pill/メタ) と下端ステータスバー (status_line)
    に分配され、独立パネルは存在しない'
  - '[pointer-mouse|keyboard] 停止アイコンボタンから既存 ConfirmDialog の終了フローが開く (ワンクリック即終了はしない)'
  - '[keyboard] タブは下線式で、既存 ARIA tabs manual activation (矢印移動 + Enter/Space 活性化) が維持される'
- id: F-005
  name: コマンドパレット — 3 部構成の器
  steps:
  - '[keyboard] Cmd/Ctrl+K で開くと入力 / セクションリスト (Actions, Push) / フッタ文脈 1 行の 3 部構成で表示される'
  - '[screen-reader] セッション切替等のフィードバックはトースト + aria-live 告知で行われ、パレット内に常設ステータス行は無い'
  - '[pointer-mouse] 使えない push コマンドは減光 + 理由付きで表示され、activation はブロックされる'
- id: F-006
  name: Workspace モードとファイルエディター (2026-07-14 改訂 — ドロワー案を撤回)
  steps:
  - '[pointer-mouse|keyboard] ヘッダの Terminal / Workspace スイッチ (または Changes 行クリック) で
    Workspace モードへ切り替わり、メイン領域全体がツリー + エディターになる。ターミナルはマウントされたまま (スクロールバック維持)'
  - '[pointer-mouse] 左の常設ツリーはインデントガイド + シェブロン + dir/file アイコンで階層が読める'
  - '[pointer-mouse|keyboard] dirty 状態で Save ボタンまたは Cmd/Ctrl+S (エディターフォーカス時のみ) で保存でき、vim
    :w も従来どおり動く'
  - '[pointer-mouse] read-only 縮退時はバナーで理由が表示され、バッファ内容は保持される'
- id: F-007
  name: テーマ・モーション横断品質
  steps:
  - '[pointer-mouse] ライトテーマで全コンポーネントがトークン経由で追従し、hardcoded 濃色の残留が無い'
  - '[pointer-mouse] prefers-reduced-motion で新設トランジションが全て即時切替になる'
acceptance_scenarios:
- id: UAC-001
  flow_ref: F-001
  given: デスクトップ (>=1024px) でセッション複数、session-A がアクティブ
  when: ポインタで session-B の行をクリックし、その後 Tab で session-C にフォーカスを移す
  then: session-B の行だけが塗り背景 (--accent-soft) の選択表示になり session-A の塗りは外れ、ヘッダのパンくずとターミナルが
    session-B に切り替わる。session-C にはフォーカスリングのみが付き、塗りは session-B に残る
  vs_legacy: must-fail
  counterexample: '誤実装: 現行同様、選択もフォーカスも青枠 (outline) で表現する。Tab 移動だけで「アクティブセッションが変わった」ように見え、session-C
    のリングと session-B の塗りが視覚的に区別できず fail する。'
- id: UAC-002
  flow_ref: F-001
  given: running / waiting / idle / stopped / pending のセッションが各 1 件
  when: サイドバーを表示する
  then: running は緑ドット + グロー、waiting は琥珀、idle は灰、stopped は赤の輪郭のみ (塗り無し)、pending は青で区別でき、各行にステータス名が
    aria-label または title でテキスト提供される
  vs_legacy: must-fail
  counterexample: '誤実装: 全状態を塗りドットにして色相だけ変える。色覚特性者が running と stopped を判別できず、stopped
    の「輪郭のみ」という形状 assertion とテキスト提供 assertion で fail する。'
- id: UAC-003
  flow_ref: F-001
  given: title / driver / model / effort / 経過時間を全部持つセッション、サイドバー幅 240px
  when: セッション行を表示する
  then: 行は「ドット + 省略記号付きタイトル + 経過時間」1 行 + 等幅メタ (driver · model · effort) 1 行の計 2 行以内で、driver
    tag の色 chip はタイトル行に存在しない
  vs_legacy: must-fail
  counterexample: '誤実装: 現行の tag chips を行内に残して 3 行以上に積む。240px で折返しが発生し行高が不定になり、2 行以内
    assertion で fail する。'
- id: UAC-004
  flow_ref: F-002
  given: プロジェクト /home/dev/dev/agent-grid の session「Fix WS reconnect backoff」がアクティブ
  when: ヘッダを表示する
  then: agent-grid / Fix WS reconnect backoff のパンくず (プロジェクトは basename) + status pill
    + 等幅メタが 44px 高のヘッダ 1 行に収まる
  vs_legacy: must-fail
  counterexample: '誤実装: パンくずにフルパスを表示する。横幅を食い潰しタイトルが省略され、basename assertion で fail
    する。'
- id: UAC-005
  flow_ref: F-002
  given: ヘッダの overflow (…) メニュー
  when: Theme → Light を選択する
  then: 即時にライトテーマが適用され localStorage["agent-grid-theme"] に light が保存される。System 選択時はキーが削除される
    (既存 ThemeProvider の STORAGE_KEY 契約)
  vs_legacy: must-pass
  counterexample: '誤実装: 新しい storage key を導入する。既存ユーザーのテーマ設定がリセットされ、キー名 assertion で
    fail する。'
- id: UAC-006
  flow_ref: F-002
  given: <768px ビューポート
  when: ヘッダを表示する
  then: ハンバーガーは左端、アクティブセッションタイトルが続き、status pill が右端。ドロワーは左からスライドインする (既存 SessionDrawer
    流用)
  vs_legacy: must-fail
  counterexample: '誤実装: 現行の右端ハンバーガーのままスタイルだけ更新する。開閉の出現方向 (左) と操作起点 (右) が食い違い、位置 assertion
    で fail する。'
- id: UAC-007
  flow_ref: F-003
  given: アクティブセッションにファイルアクティビティ (edit x3 / edit x1 / create x1) があり Workspace モード
  when: 右パネルを表示する
  then: 上段 Changes セクションに M/A/D 記号 + 末尾優先省略パス + 回数の行が並び、行クリックで該当ファイルがエディターに開く。下段は
    Files ツリー。ライトテーマでもトークン経由で追従する
  vs_legacy: must-fail
  counterexample: '誤実装: Changes をモード外の常設パネル (class=changes-panel) として残す。Terminal モードに
    UI 残骸が現れ、「Terminal モードにパネル類が存在しない」の否定形 assertion で fail する。'
- id: UAC-008
  flow_ref: F-003
  given: Terminal モード
  when: メイン領域を表示する
  then: ターミナル以外のパネル・ハンドル・シート類は存在せず、ターミナルがメイン領域全幅を使う (2026-07-14 改訂 — collapse 機構は
    panel 廃止に伴い削除)
  vs_legacy: must-fail
  counterexample: '誤実装: 旧 ChangesPanel/collapse ハンドルの DOM を残す。「Terminal モードにパネル類が存在しない」assertion
    で fail する。'
- id: UAC-009
  flow_ref: F-003
  given: <768px、ファイルアクティビティあり
  when: Terminal モードでメイン画面を表示し、Workspace モードへ切り替える
  then: Terminal モードではターミナルがビューポート全幅を使い (シート・帯なし)、Workspace モードで Changes セクション + Files
    ツリーに到達できる (縦分割)
  vs_legacy: must-fail
  counterexample: '誤実装: bottom sheet や縮小パネルをモード外に残す。390px では何 px であってもターミナルを侵食し、「Terminal
    モードにパネル類が存在しない」assertion で fail する。'
- id: UAC-010
  flow_ref: F-004
  given: title / model / effort / tags / status_line を全部持つ running セッション
  when: デスクトップでメインビューを表示する
  then: 独立した DriverViewPanel は存在せず、title はパンくず、model/effort は等幅メタ、status_line は下端ステータスバー、driver
    tags はサイドバー行メタに分配されている
  vs_legacy: must-fail
  counterexample: '誤実装: 4 段積みパネルのままスタイルだけ更新する。「独立パネルとして存在しない」の否定形 assertion で fail
    する。'
- id: UAC-011
  flow_ref: F-004
  given: アクティブセッションのヘッダ
  when: 停止アイコンボタン (tooltip「Stop session」) をクリックする
  then: 既存 ConfirmDialog (destructive variant) が開き、確認後にのみ終了する。ボタンのヒット領域は 36px 以上
  vs_legacy: must-pass
  counterexample: '誤実装: アイコン化と同時に確認ダイアログを外しワンクリック即終了にする。ConfirmDialog 表示 assertion
    で fail する。'
- id: UAC-012
  flow_ref: F-004
  given: TRANSCRIPT / EVENTS の log_tabs と unread 2 件の frame_messaging_summary
  when: タブストリップをキーボードで操作する
  then: Terminal / Transcript / Events / Messages(2) が下線式タブで並び、矢印キーはフォーカス移動のみ・Enter/Space
    で活性化する (manual activation / ADR-0061 維持)
  vs_legacy: must-pass
  counterexample: '誤実装: 見た目刷新時に roving tabindex を壊し、矢印キーで即活性化 (automatic activation)
    へ退行する。矢印移動後に aria-selected が変わらないことの assertion で fail する。'
- id: UAC-013
  flow_ref: F-005
  given: Cmd/Ctrl+K でパレットを開く
  when: 開いた直後の DOM を検査する
  then: 上から検索入力 / セクション見出し付きリスト (Actions, Push) / フッタ 1 行 (project / session 文脈 +
    キーヒント) の 3 部構成で、旧 3 行ヘッダ (タイトル / ACTIVE 行 / inline status 行) は存在しない
  vs_legacy: must-fail
  counterexample: '誤実装: ヘッダ 3 行を残したまま角丸と影だけ足す。「存在しない」の否定形 assertion で fail する。'
- id: UAC-014
  flow_ref: F-005
  given: パレット操作でアクティブセッションが変わる
  when: 変更が確定する
  then: 既存 notifications 系トーストに Switched to <label> が流れ、aria-live 告知も発火する。パレット内に常設ステータス行は残らない
  vs_legacy: must-fail
  counterexample: '誤実装: 視覚要素だけ消して aria-live 告知も一緒に消す。スクリーンリーダー向け announce の発火 assertion
    で fail する。'
- id: UAC-015
  flow_ref: F-005
  given: frame 不在 (activeOccupant が frame でない) で push コマンド save/status/diff が使えない
  when: パレットのリストを表示し、無効項目で Enter を押す
  then: Push セクションに減光表示され右端に理由 (driver not ready 等) が出る。Enter しても実行されない (fail-closed
    維持)
  vs_legacy: must-fail
  counterexample: '誤実装: 無効項目をリストから隠す。「なぜ使えないか」の学習機会が消え、表示 + 理由 assertion で fail する。'
- id: UAC-016
  flow_ref: F-006
  given: Terminal モードでターミナルに scrollback がある状態
  when: Workspace モードへ切り替え、ファイルを編集し、Terminal モードへ戻る
  then: メイン領域はモードごとに全面が切り替わり (ドロワーではない)、戻ったターミナルは scrollback・購読・表示サイズを維持している。再度 Workspace
    へ切り替えると、開いていたファイル・タブ・未保存バッファがそのまま残っている (モード切替は純粋な表示切替で、破棄確認も出ない)。Esc でも Terminal
    へ戻れる (エディター内フォーカス時を除く — Esc は vim 通貨)
  vs_legacy: must-fail
  counterexample: '誤実装 1: モード切替でターミナルを unmount して再マウントする — scrollback が消え「切替後も scrollback
    が残る」assertion で fail。誤実装 2: Terminal への切替を close 扱いにして workspace セッションをリセットする
    — 戻ったとき空状態になり「ファイルが開いたまま」assertion で fail。誤実装 3: エディターにフォーカスがあるとき Esc がモードを離脱し
    vim 操作を破壊する。'
- id: UAC-017
  flow_ref: F-006
  given: ドロワーの Tree タブでネストしたディレクトリを表示
  when: src/ を展開する
  then: 子エントリはインデント + ガイド線で階層表示され、dir はシェブロン回転 + フォルダアイコン、file はファイルアイコンを持つ。展開状態はドロワーを閉じるまで保持される
  vs_legacy: must-fail
  counterexample: '誤実装: ボタン羅列のままアイコンだけ足す。インデントとシェブロンの assertion で fail する。'
- id: UAC-018
  flow_ref: F-006
  given: エディターで編集して dirty 状態 (既存 dirty-indicator が示す状態)
  when: Save ボタンをクリック、または Cmd/Ctrl+S を押す
  then: 既存 performSave が呼ばれ、成功で dirty 表示が消え Save は saved (disabled) に戻る。Cmd/Ctrl+S
    はエディターフォーカス時のみ preventDefault され、ターミナルやパレットにフォーカスがある時は横取りしない。vim :w は従来どおり動く
  vs_legacy: irrelevant
  counterexample: '誤実装: keydown を document 全体で拾い、ターミナルフォーカス時まで Cmd+S を横取りする。エディター外フォーカスでのショートカット不発
    assertion で fail する。'
- id: UAC-019
  flow_ref: F-006
  given: dirty バッファを持つエディター表示中に workspace root が消失する (既存の縮退トリガ)
  when: read-only 縮退が起きる
  then: バッファ内容は保持されたまま (既存挙動)、エディター上部にバナーで理由 (root unreachable) が表示され、Save ボタンは disabled
    + tooltip で同じ理由を示す
  vs_legacy: must-pass
  counterexample: '誤実装: disabled にするだけで理由を出さない。「保存できないのにボタンがある」状態になり、バナー + tooltip
    の理由表示 assertion で fail する。'
- id: UAC-020
  flow_ref: F-007
  given: ライトテーマ
  when: シェル / サイドバー / Changes / ドロワー / パレット / トーストを巡回する
  then: hardcoded 濃色の残留がゼロで、通常テキスト 4.5:1 / 大テキスト・UI 部品 3:1 (WCAG AA) を満たす
  vs_legacy: must-fail
  counterexample: '誤実装: 現行 ActivityRail のような component CSS 内の色直値。tokens.css 以外の色リテラルを検出する静的ガードで
    fail する。'
- id: UAC-021
  flow_ref: F-007
  given: 'prefers-reduced-motion: reduce'
  when: ドロワー開閉・タブ切替・パレット開閉・トースト表示を行う
  then: 新設トランジションは全て即時切替になる (--motion-* トークンの一括無効化で保証)
  vs_legacy: must-pass
  counterexample: '誤実装: 新設モーションを個別 CSS に直書きしてガードの網から漏らす。motion トークン経由の静的検査で fail する。'
- id: UAC-022
  flow_ref: F-005
  given: new-session のパラメータフェーズで project と command を選択済み
  when: command の option をクリック、または command フィルタで Enter を押す
  then: セッションは作成されず、フォーカスが明示の確定ボタン (New Session) に移る。確定ボタンの click / Enter で初めて createSession
    が発火する。worktree / host のオプションチップは確定ボタンの直前 (アクション行) に並ぶ
  vs_legacy: must-fail
  counterexample: '誤実装: 最終フィールドの選択で即 submit する (選択=確定の混同)。2 クリックで意図しないセッションが作成され、確定前の
    worktree トグル操作が構造的に不可能になる。「選択後に createSession が呼ばれていない」assertion で fail する。'
states:
- 'セッション行: default / hover / selected (塗り) / focused (リング) — selected と focused は独立に共存する'
- 'Changes パネル (デスクトップ): expanded / collapsed (件数バッジ付きハンドル) — usePersistedValue で永続'
- 'エディター Save: saved (disabled) / dirty (enabled + ドット) / saving (spinner) / read-only
  (disabled + 理由 tooltip)'
- 'テーマ: dark (既定) / light / system — agent-grid-theme キー互換'
edge_cases:
- セッション 0 件時のサイドバー空状態 (New session への導線を維持)
- ファイルアクティビティ 0 件時は Changes パネル/シートを非表示にせず空状態表示 (存在に気づく導線を保つ)
- transportDegraded 時の reconnecting 表示はパネル/シートのヘッダ領域に出す (既存セマンティクス)
- サイドバー幅下限 (240px) での長タイトル・深いパスの省略表示
assumptions:
- UI 文言は既存の英語コピー方針 (__meta__/no-japanese.test.ts) を踏襲する
- 挙動実装 (useFocusTrap / UnifiedListbox / ConfirmDialog / palette フェーズ遷移 / 保存安全機構 /
  各ストア) は無改変で流用できる
- 既存 e2e / unit の role・data-testid 契約は原則維持し、変更時は同一チャンク内でテストを更新する
reference_ux:
- name: Linear
  stance: modeled_on
  aspects:
  - サイドバー主導シェル
  - 選択=塗り / フォーカス=リングの状態分離
  - 単一アクセント + ヘアライン境界
- name: Vercel dashboard
  stance: modeled_on
  aspects:
  - 等幅メタデータの使い分け
  - 薄い 44px ヘッダ
- name: Raycast
  stance: modeled_on
  aspects:
  - コマンドパレットの 3 部構成 (入力 / セクションリスト / フッタ)
- name: 現行 ActivityRail (常駐縦帯)
  stance: rejected
  aspects:
  - モバイルでメインコンテンツを侵食する常駐サイドレール
tags:
- ux
- web
- ui-refresh
owners: []
relations:
- {type: implementedBy, target: spec-20260714-web-ui-refresh}
- {type: referencedBy, target: spec-20260714-workspace-session-switch}
source_paths:
- src/client/web/src/components/
- src/client/web/src/css/
methodology: atdd
summary: Web UI 全面刷新の UX プラン。7 フロー 22 の Given-When-Then 受け入れシナリオ + 反例 (誤実装論証)。
---

## Goal

frontmatter `goal` を参照。上位文書は UI/UX 刷新提案 (2026-07-14 採用済み、スクリーンショット付き批評は artifact ui-refresh-proposal)。

## Scope

**対象:** デザイントークン体系 / アプリシェル / セッションリスト / メインビュー / ActivityRail の解体と Changes パネル化 / コマンドパレットの器 / ワークスペースドロワーとエディタークローム / モバイル UX / モーション / ライトテーマ品質。

**非スコープ (無改変):** wire プロトコル・API・Go 側全部 / 挙動実装 (useFocusTrap・UnifiedListbox・ConfirmDialog・palette フェーズ遷移・subscribe ライフサイクル) / エディター保存安全機構 (mtime 前提条件・handle staleness・save allowlist・dirty バッファ保持) / ストアロジック (daemon・workspaceActivity・palette)。

## Primary Flows / Acceptance Scenarios

frontmatter `primary_flows` (F-001..F-007) と `acceptance_scenarios` (UAC-001..UAC-022) が SoT。各 UAC は counterexample (誤実装論証) を持ち、`vs_legacy` で現行実装との差分を符号化する (must-fail = 現行 UI はこのシナリオに落ちる = 刷新の delta)。

## Non-functional UX

- ヒット領域: タッチ対象 44x44px 以上 (既存方針維持)、デスクトップのアイコンボタンは実効 36px 以上
- キーボード: 既存のフォーカス順・ショートカット (Cmd/Ctrl+K、drawer の Esc、タブ矢印移動) を退行させない
- コントラスト: WCAG AA (UAC-020)。検証は既存 `util/contrast` を流用
