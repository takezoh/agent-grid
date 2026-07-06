---
id: ux-20260625-2026-06-25-web-ui-redesign
kind: ux
title: Web client UI/UX 全面刷新 (適応レイアウト + デザイントークン + テーマ + a11y 波及)
status: draft
created: '2026-06-25'
updated: '2026-07-04'
tags:
- ux
- legacy-import
owners:
- take.gn@gmail.com
relations:
- {type: referencedBy, target: adr-20260624-0059-design-token-and-theme-bridge}
- {type: referencedBy, target: adr-20260624-0060-adaptive-layout-and-drawer}
- {type: referencedBy, target: adr-20260624-0061-apg-tabs-manual-activation}
- {type: referencedBy, target: adr-20260624-0062-search-bar-trigger-and-palette-theme-entry}
- {type: referencedBy, target: adr-20260624-0063-toast-single-live-and-undosnackbar}
- {type: referencedBy, target: adr-20260624-0064-reduced-motion-single-guard}
- {type: referencedBy, target: plan-20260625-2026-06-25-web-ui-redesign}
- {type: implementedBy, target: spec-20260625-2026-06-25-web-ui-redesign}
- {type: referencedBy, target: spec-20260625-2026-06-25-web-ui-redesign}
source_paths:
- src/client/web/
- src/client/web/src/
- src/client/web/src/components/
- src/client/web/src/css/
goal: agent-grid-new の Web client (src/client/web) を、情報設計・認知負荷・一貫性・アクセシビリティ・ 既存プラットフォーム慣習の観点でモダンに全面刷新する。PC
  / スマホ / タブレットの 3 デバイスすべてで 破綻しない適応レイアウトを実現し、デザイントークン体系 + タイポグラフィ階層 + WCAG AA コントラストを
  確立し、コマンドパレットで熟成済みの視覚言語・a11y パターン (unified listbox / focus trap / 単一 aria-live slot
  / disabled visible+skip-navigation / IME 抑止) を全画面 (セッション一覧・ターミナル ホスト・メインタブ・通知トースト・ステータスバナー・DriverViewPanel)
  に波及させる。既存の keyboard / ARIA / IME 資産を退行させないことを不変条件とする。
target_users:
- 複数の coding-agent セッションを並行運用する開発者 (主作業環境は PC、外出先・離席中はスマホ/タブレットで状態確認・切替・軽操作)
- keyboard 主体のパワーユーザー (Cmd/Ctrl+K パレット・矢印/Ctrl-P/N・focus trap に依存)
- screen-reader 利用者 (role / aria-live / aria-activedescendant / aria-expanded に依存して状態を把握する)
- モバイルでセッションの running/idle をグランス確認し、必要なら離席先からセッション切替・パレット起動・短いコマンド送信をしたいユーザー
- タブレットを touch + 外付けキーボードのハイブリッドで使うユーザー (hotkey と touch affordance が同時到達)
primary_flows:
- id: F-001
  name: スマホでセッション状態を確認し切り替える (drawer 経由)
  steps:
  - '[pointer-touch|screen-reader] スマホ幅 (<768px) で読み込むと、sidebar は off-canvas で非可視、ヘッダー左に常時可視の
    ☰ ハンバーガーボタン (aria-label=''Open sessions'', aria-expanded=''false'') が出て、中央は現在セッションの
    DriverView/MainTabs が全幅表示される (横スクロールなし)'
  - '[pointer-mouse|pointer-touch|keyboard] ハンバーガーを click/tap (または keyboard focus
    + Enter) して session drawer を開く。ハンバーガーが全 modality の正典な開閉手段。左端→右スワイプは touch 限定の補助
    (視覚的痕跡は出さない)'
  - '[pointer-touch|screen-reader] drawer が前面に出て scrim がメインを覆い、ハンバーガーの aria-expanded
    が ''true'' になり、drawer は role=''dialog'' aria-modal=''true'' で focus が drawer 内へ移り、scrim
    配下のメインは inert / aria-hidden=''true'' になり screen-reader の仮想カーソルが背後へ到達しない'
  - '[pointer-touch|screen-reader] session list の各行 (最小 44x44px、status icon+text+spinner
    で多重符号化、displayLabel は title→subtitle→id、長文は 2 行 ellipsis に統一) を click/tap して別セッションを選ぶ'
  - '[pointer-touch|keyboard|screen-reader] 選択すると drawer が閉じ scrim が消え、aria-expanded
    が ''false'' に戻り、focus がハンバーガーへ復帰し、中央が選択セッションの DriverView/MainTabs に更新され、切替先 label
    が aria-live で announce され、短命の ''Switched to <label>'' status に Undo affordance
    が出る (tap で直前セッションへ戻す。activeSessionID は client 単独管理で直前 ID は UI-local state 保持)'
  - '[pointer-touch|keyboard] scrim tap / Esc / 左へスワイプは「選択せずに閉じる (キャンセル)」で activeSession
    は変わらない (= 選択クローズと取消クローズの結果が観測上区別できる)'
- id: F-002
  name: テーマを system 連動から明示選択へ切り替える
  steps:
  - '[pointer-mouse|pointer-touch|keyboard|screen-reader] ヘッダーのテーマ segmented control
    (role=''radiogroup'' aria-label=''Theme'', System / Light / Dark の 3 segment が常時一覧表示、各
    role=''radio'' aria-checked) に到達する。スマホでヘッダー幅が狭い時は現在値 icon (🖥/☀/🌙) の 1 ボタンを tap
    して bottom-sheet 内の同 segmented control を開く (drawer に隠れず到達可能)'
  - '[keyboard|screen-reader] 初期既定は system 連動 (prefers-color-scheme)。OS が dark なら
    dark 配色トークンが適用され、System segment が aria-checked=''true'' で『OS 追従中』を示す'
  - '[pointer-mouse|pointer-touch|keyboard] Light segment を選択 (click/tap / 矢印 + Space)
    して light を明示選択する'
  - '[pointer-mouse|pointer-touch|screen-reader] document に data-theme=''light'' が反映され、body
    配色・テキスト・border が light トークン (CSS custom property) に切替わり、xterm の前景/背景色も同じトークンに連動して
    light に追従し、選択が localStorage に永続化される (リロード保持)'
  - '[pointer-mouse|pointer-touch|keyboard] 再度 System を選ぶと localStorage 上書きが解除され OS
    テーマ追従に復帰する。prefers-reduced-motion: reduce の環境では切替時の transition / flash が抑制される'
- id: F-003
  name: いずれのデバイスでもコマンドパレットを起動して操作を実行する
  steps:
  - '[pointer-mouse|pointer-touch|keyboard|screen-reader] PC/タブレットは header の検索バー風トリガ
    (虫眼鏡 icon + placeholder ''Search commands…'' + 右端に ⌘K/Ctrl+K の hint badge を isMacPlatform
    で出し分け) を click/tap、または Cmd/Ctrl+K (capture phase) を押す。スマホは header 全幅の検索バー風 button
    を tap (button のまま、仮想キーボードは呼ばない)。New Session は palette 内の最上位 suggested action として吸収するか検索バー右の
    ''+'' icon button に分離 (プログレッシブ開示)'
  - '[keyboard|pointer-touch|screen-reader] role=''dialog'' aria-modal=''true'' の
    overlay が前面に出て focus が検索 input に入り、xterm が blur され、focus が dialog 内に trap される。スマホでは全幅
    sheet 風配置 (左右マージン各 16px 以内) で input が仮想キーボードに隠れない'
  - '[keyboard|pointer-mouse|pointer-touch] fuzzy listbox を ↑↓/Ctrl-P/N または tap/mousemove
    で navigate し aria-activedescendant が現在 option を指す。disabled 項目は visible + skip-navigation
    され理由が announce される。border/radius/spacing/タイポは正典トークンで他画面と共有される'
  - '[keyboard|pointer-mouse|pointer-touch|screen-reader] Enter / option tap (mousedown
    確定) で tool を選び param phase へ。IME composition 中は誤確定しない'
  - '[keyboard|pointer-mouse|pointer-touch] 送信すると入力が freeze し単一 aria-live slot が ''Sending…''
    を示す。送信が rejected されたら freeze が解け palette は開いたまま inline error が aria-live で 1 回
    announce され retry/Esc を選べる。Esc / Back / scrim(overlay) tap で phase back / close
    し focus が opener へ復帰する'
- id: F-004
  name: アクティブセッションのターミナル/トランスクリプト/イベントを閲覧する (全デバイス共通の最小集合)
  steps:
  - '[pointer-mouse|pointer-touch|keyboard|screen-reader] セッション選択後、上部に DriverViewPanel
    (title/subtitle/tags/elapsed + RunStateBadge: running=green+spinner / idle=gray、icon+text+色で多重符号化)、下に
    MainTabs (TERMINAL/TRANSCRIPT/EVENTS, role=''tablist'') が表示される'
  - '[keyboard|screen-reader] tablist は WAI-ARIA APG tabs pattern (roving tabindex:
    選択タブ tabindex=0 / 他 -1、ArrowLeft/Right で移動、Home/End で端) で操作でき、各 tab body は role=''tabpanel''
    + aria-labelledby を持つ。palette で確立した keyboard nav 規律を踏襲し focus ring (:focus-visible
    token) が出る'
  - '[pointer-mouse|pointer-touch|keyboard] タブ (最小 44x44px) を click/tap/矢印で切替える。TERMINAL
    は常時 mount され CSS で可視切替されるため scrollback と subscribe lifecycle が保持される'
  - '[pointer-mouse|pointer-touch] TERMINAL タブでターミナル出力を閲覧・スクロールする。ホスト高さは dvh + safe-area
    で算出され、ツールバー出没 (縮小/拡大) や仮想キーボード出現でリサイズしても xterm refit が走り 0 rows / overflow にならない'
  - '[pointer-touch|keyboard] モバイルのターミナル本格打鍵は read-first (案B) を MVP デフォルトとし、短いコマンド送信は
    palette の param text input (ソフトキーボードで完結) に逃がす。xterm 直接フル打鍵 (Esc/Ctrl/方向キー = 仮想キー補助バー、案A)
    は後続フェーズの progressive enhancement。本フローは両案共通の閲覧/スクロール/タブ切替/状態把握のみを観察対象に固定する (どちらに倒すかは
    Open Questions)'
acceptance_scenarios:
- id: UAC-001
  given: viewport 幅 375px で 2 件以上のセッションがある初期表示
  when: ページが描画され、その後ハンバーガーボタンを tap する
  then: 初期描画では aria-label='sessions' の session list 要素が off-canvas (非可視) で、中央の DriverView/MainTabs
    が viewport 全幅に収まり横スクロールが発生しておらず、aria-expanded='false' のハンバーガーが visible である。 ハンバーガー
    tap 後にハンバーガーの aria-expanded が 'true' になり session list が画面内に visible になる (= ハンバーガーが配線されており
    drawer が実際に開く)
  flow_ref: F-001
  counterexample: 280px sidebar を @media で left:-280px の画面外に absolute 配置し、ハンバーガーを
    aria-expanded='false' で 常時描画するが onClick は未配線 (no-op) の死んだ UI。初期状態の『list off-canvas
    / ハンバーガー visible / 横 スクロールなし』だけを Then にすると、この死んだ UI でも pass してしまう。よって When に『tap
    する』を含め、 Then で『tap 後に aria-expanded が true になり list が visible になる』まで縛り、配線されていない
    drawer を fail させる。legacy はメディアクエリ 0 件で sidebar 常時可視のため初期の off-canvas で既に fail
    する。
  vs_legacy: must-fail
- id: UAC-002
  given: viewport 幅 375px、session drawer が開いている (ハンバーガー aria-expanded='true')
  when: drawer が前面に出た直後の DOM を観察する
  then: drawer 要素が role='dialog' aria-modal='true' を持ち、document.activeElement が drawer
    の内側にあり、scrim 要素が visible で、scrim 配下のメイン領域 (DriverView/MainTabs を含む) が inert または
    aria-hidden='true' に なっている
  flow_ref: F-001
  counterexample: drawer を CSS で前面に出し focus も drawer 内に移すが、背後のメインを inert / aria-hidden
    にしない実装。focus trap だけでは VoiceOver の rotor / 仮想カーソルが背後の session 外要素へ到達でき modal の意図が
    screen-reader で破れる。Then に『scrim 配下が inert/aria-hidden』を含めることでこの背後到達を fail させる。
  vs_legacy: must-fail
- id: UAC-003
  given: 幅 375px で drawer が開き、別の非アクティブセッション行が表示されている
  when: その行を tap する
  then: drawer が閉じ scrim が消え、ハンバーガーの aria-expanded が 'false' になり、focus がハンバーガーへ復帰し、中央
    DriverView の title/subtitle がその選択セッションのものに変わり、'Switched to <label>' の status が
    Undo 付きで 表示され、その Undo を tap すると直前セッションへ戻る (DriverView の title が元に戻る)
  flow_ref: F-001
  counterexample: 行 tap で activeSession を切替え drawer を閉じるが Undo 経路を持たない実装。Then の『Undo
    tap で直前セッション へ戻る』を満たせず fail する。誤切替からの回復が観測可能であることを縛る。
  vs_legacy: must-pass
- id: UAC-004
  given: 幅 375px で drawer が開いている
  when: scrim を tap する (= 選択せずに閉じる)
  then: drawer が閉じ aria-expanded が 'false' になり、focus がハンバーガーへ復帰するが、中央 DriverView の
    title/subtitle は drawer を開く前と同一のままで activeSession が変わっていない
  flow_ref: F-001
  counterexample: scrim tap も行 tap も同じ『閉じる』ハンドラに流し、scrim tap で直近 highlight 行を誤って選択してしまう実装。
    Then の『activeSession が変わらない』を満たせず fail する。選択クローズと取消クローズの結果差を縛る。
  vs_legacy: irrelevant
- id: UAC-005
  given: 'prefers-color-scheme: light をエミュレートし、ユーザーがまだテーマを明示選択していない初回ロード'
  when: アプリが描画される
  then: 'document の data-theme が ''light'' 相当 (または media 連動で light トークンが解決) で、body
    の computed background-color が light トークン値 (暗色 #1e1e1e 系ではない明色) であり、本文テキストと背景のコントラスト比が
    4.5:1 以上で、かつ xterm terminal 要素の computed background-color も light 系である'
  flow_ref: F-002
  counterexample: 'テーマトグルの onClick 内で xterm.options.theme を #ffffff にハードコード直書きする実装
    (custom property 非連動、 system 追従ロジックなし)。これは『light を明示選択した時 xterm が light』だけなら pass
    するが、本 scenario は 『明示選択していない初回ロードで prefers-color-scheme: light に追従して xterm も light』を縛るので、初期ダーク据え
    置きの直書き実装は fail する。さらに 4.5:1 を Then に含めるので色だけ替えてコントラスト未調整でも fail する。'
  vs_legacy: must-fail
- id: UAC-006
  given: テーマ segmented control が 'System' で OS=dark のため dark 配色が出ている
  when: Light segment を選択する
  then: document の data-theme が 'light' になり、body の computed background-color が light
    トークン値に変わり、かつ xterm terminal 要素の背景色も同一の light トークン経由 (CSS custom property を読む path)
    で light 系に変わり、 Light segment の aria-checked が 'true'、System segment が 'false'
    になる
  flow_ref: F-002
  counterexample: 'body だけ light にして xterm の theme を #1e1e1e のまま据え置く、あるいは xterm を別の直書きハードコード色に
    する実装。前者は『xterm も light 系』を満たせず fail。後者を排除するため Then に『同一トークン経由』を含め、 data-theme
    / custom property を読まない実装が UAC-005 (system 追従) と組で fail することを縛る。'
  vs_legacy: must-fail
- id: UAC-007
  given: ユーザーが Light を明示選択済みでページをリロードした (OS は dark)
  when: アプリが再描画される
  then: body の computed background-color が light トークン値のままで、OS が dark でも dark に戻らず、Light
    segment が aria-checked='true' である
  flow_ref: F-002
  counterexample: data-theme を localStorage に永続化せずセッション中の state でのみ保持する実装。リロードで state
    が失われ system (=OS dark) に戻るため Then の『light のまま』を満たせず fail する。永続化媒体の存在を縛る。
  vs_legacy: irrelevant
- id: UAC-008
  given: viewport 幅 375px でパレットが閉じている
  when: ヘッダーの検索バー風 Command 起動 affordance を tap する
  then: role='dialog' aria-modal='true' の要素が表示され、その dialog の幅が viewport に対して全幅に近い
    sheet 風 (左右マージンが各 16px 程度以内) で配置され、検索 input に focus が当たっており、xterm が blur されている
  flow_ref: F-003
  counterexample: legacy の .palette-dialog は max-width:600px / width:90% / margin-top:10vh
    固定で 375px では中央に小さめの箱が 出るだけ。スマホ全幅 sheet (左右マージン各 16px 以内) を満たせず fail する。さらに input
    に focus が当たらなければ そこでも fail する。
  vs_legacy: must-fail
- id: UAC-009
  given: パレットが開き、disabled な tool 行と enabled な tool 行が listbox にある
  when: ↑↓ または Ctrl-P/N で navigation する
  then: aria-activedescendant が enabled 行のみを指し disabled 行を skip し、disabled 行は DOM
    上 visible のままで理由テキストが付随している
  flow_ref: F-003
  counterexample: navigation が disabled 行に止まる実装。Then の『aria-activedescendant が disabled
    行を指さない』を満たせず fail する。legacy で既に成立しているため退行防止の must-pass。
  vs_legacy: must-pass
- id: UAC-010
  given: パレットが開き検索 input に focus がある状態で IME 変換中 (composition 中)
  when: 変換確定のための Enter を押す
  then: param phase へ遷移せず listbox に留まり (phase が toolSelect のまま観察できる)、tool 確定が起きていない
  flow_ref: F-003
  counterexample: composition 中も Enter で tool を確定してしまう実装。Then の『phase が toolSelect
    のまま』を満たせず fail する。 legacy で既に成立しているため退行防止の must-pass。
  vs_legacy: must-pass
- id: UAC-011
  given: session list (drawer 内) に disabled なセッション行と enabled な tool 行があり、パレットも開ける状態
  when: session list の disabled 行と palette listbox の disabled 行を両方観察し、各々の border-radius
    / padding / フォントサイズ (computed) を比較する
  then: session list 行と palette listbox 行が同一の正典トークン (--radius / --space / --font-size
    custom property) に解決 され computed 値が一致し、両者とも disabled 項目が visible + 理由テキスト付きで同じ
    skip-navigation 方針になっている
  flow_ref: F-003
  counterexample: 'palette だけ新トークンで再設計し session list は legacy のハードコード値 (border:1px
    solid #555 等) のまま据え置く 実装。視覚的に別デザインのままだが個々の画面は動く。Then に『同一 custom property に解決され
    computed 値が一致』を 含めることで、トークン非共有 (= 視覚的一貫性の宣言止まり) を fail させる。legacy は palette だけ別デザインなので
    fail。'
  vs_legacy: must-fail
- id: UAC-012
  given: パレットが開き検索 input に focus がある状態で、送信が必ず失敗する状況 (例 daemon 切断中) を作る
  when: tool を確定して送信する
  then: 送信直後に入力が freeze し単一 aria-live slot に 'Sending…' が出た後、失敗時に freeze が解け palette
    は開いたまま inline error メッセージが同一 aria-live slot で announce され、retry または Esc を選べる (palette
    が無言で閉じたり 固まったままにならない)
  flow_ref: F-003
  counterexample: 送信失敗時に freeze を解かず palette が 'Sending…' のまま固まる、あるいは失敗を握り潰して何も announce
    せず閉じる 実装。Then の『freeze 解除 + inline error の announce + 開いたまま』を満たせず fail する。missing_failure_path
    を縛る。
  vs_legacy: irrelevant
- id: UAC-013
  given: running 状態のアクティブセッションが選択され TERMINAL タブが active、viewport 幅 375px
  when: 画面が描画される
  then: DriverView に RunStateBadge が running の text と icon を伴って表示され、terminal-host
    要素の computed height が 0 より大きく、DriverView/MainTabs が viewport 全幅に収まり横スクロールが発生しない
  flow_ref: F-004
  counterexample: legacy は 280px sidebar が常時占有するため 375px では中央ペインが横にはみ出すか sidebar に押し出される。『375px
    で 横スクロールなしに DriverView/MainTabs が収まる』を満たせず fail する。
  vs_legacy: must-fail
- id: UAC-014
  given: TERMINAL タブで scrollback のあるターミナルを表示中、keyboard focus が tablist にある
  when: ArrowRight で TRANSCRIPT タブへ移し Space/Enter で activate、再び ArrowLeft + activate
    で TERMINAL タブへ戻す
  then: '矢印キーだけでタブ間を移動でき (roving tabindex: アクティブタブのみ tabindex=0)、aria-selected が移動先
    tab に 移り、TERMINAL の scrollback 内容が保持され (terminal-host が再生成されず同一 xterm が可視化される)、terminal-host
    の height が 0 より大きい'
  flow_ref: F-004
  counterexample: 'タブを onClick のみで実装し onKeyDown / roving tabindex が無い legacy 実装 (確認済:
    MainTabs に onKeyDown 0 件)。 矢印キーでタブ移動できず Then の『矢印だけで移動し aria-selected が移る』を満たせず
    fail する。scrollback 保持 (常時 mount) は legacy で成立しているので、矢印 nav を Then に含めることで keyboard
    一貫性の波及を縛る。'
  vs_legacy: must-fail
- id: UAC-015
  given: viewport 高さを iOS Safari ツールバー出没相当に変化させる準備 (高さ 667px の表示)
  when: viewport 高さを 600px へ縮め、その後 667px へ戻す (出没の両方向)
  then: 縮小・拡大いずれの方向でも terminal-host の computed height が 0 より大きいまま新しい可視高さ (dvh ベース)
    に追従し、 ホストの bottom が safe-area-inset を尊重した位置に収まり、overflow による二重スクロールが縦横とも発生しない
  flow_ref: F-004
  counterexample: 高さを 100vh のまま据え置き overflow:hidden だけ足す実装。667px→600px の縮小方向では ResizeObserver+refit
    (ADR-0034) が fit() を再実行し height>0 を保つため、縮小片側だけの Then なら pass してしまう。本 scenario
    は 『拡大方向 (600px→667px) にも追従』と『bottom が safe-area-inset を尊重』を Then に含めるので、100vh
    は大きい方を 指し続け拡大時に可視高さへ追従せず、safe-area 未導入で fail する。片側観察を排除して dvh/safe-area 置換を縛る。
  vs_legacy: must-fail
- id: UAC-016
  given: 'prefers-reduced-motion: reduce をエミュレートした環境で running セッションが表示されている'
  when: RunStateBadge / session-status の spinner と drawer slide / active-context flash
    の有無を観察する
  then: RunStateBadge / session-status の spinner が回転アニメーションを行わず (computed animation
    が none もしくは animation-duration が 0 相当)、drawer 開閉の slide と active-context flash
    も抑制されている (running 状態自体は icon+text で引き続き読み取れる)
  flow_ref: F-002
  counterexample: 'view.css の .run-state-spinner / .session-status-spinner が animation:run-state-spin
    0.8s linear infinite を 無条件適用する legacy 実装 (確認済、reduced-motion guard なし)。reduce
    環境でも回り続け Then の『animation none/0』 を満たせず fail する。@media (prefers-reduced-motion:
    reduce) guard の欠落を縛る。'
  vs_legacy: must-fail
reference_ux:
- name: Material Design / iOS navigation drawer (off-canvas side panel)
  stance: modeled_on
  aspects:
  - スマホ幅で sidebar を off-canvas drawer 化。ハンバーガー (☰) を常時可視の正典 affordance とし、左端→右スワイプは上級者向け補助
    (Material 公式ガイダンス準拠、スワイプ単独に依存しない)
  - drawer open 時は scrim でメインを覆い、scrim tap / Esc / 項目選択 / 左スワイプで閉じる
  - drawer は role='dialog' aria-modal='true' + focus trap + 背後 inert/aria-hidden、開閉状態を
    aria-expanded で公開
  - PC/タブレット幅では drawer でなく常設 sidebar / 折り畳み rail に degrade (breakpoint で振る舞いを変える)
  - empty state に 'Sessions are in the menu ☰' のヒントで drawer を discoverable にする
- name: VS Code / Linear / Raycast command palette (search-bar-as-entry-point)
  stance: modeled_on
  aspects:
  - header の 2 ボタン (Command + New Session) を Raycast/Spotlight 系の単一検索バー風トリガに寄せる (虫眼鏡
    icon + 'Search commands…' placeholder + ⌘K/Ctrl+K hint badge)。New Session は palette
    内 suggested action か '+' に分離 (プログレッシブ開示)
  - 既にパレットで確立した unified listbox・fuzzy filter・aria-activedescendant・focus trap・IME
    抑止を正典とし、他画面の border/radius/spacing/タイポをパレットの custom property token に揃える
  - Cmd/Ctrl+K の keyboard 起動を維持しつつ、モバイルでは常設の起動 affordance (header 全幅 button) を pointer-touch
    で到達可能にする
  - disabled 項目は visible + skip-navigation + 理由 announce の方針を session list 等の他リスト系
    UI にも一貫適用する
- name: WAI-ARIA Authoring Practices (APG) Tabs Pattern
  stance: modeled_on
  aspects:
  - MainTabs に roving tabindex (アクティブタブ tabindex=0 / 他 -1)、ArrowLeft/Right で移動、Home/End
    で端、role='tabpanel' + aria-labelledby を実装 (legacy は onClick のみで onKeyDown 0 件の
    APG 未充足を解消)
  - palette で確立した keyboard nav 規律を tablist へ波及させ、操作系を揃える (:focus-visible token の focus
    ring)
- name: システム連動テーマ (prefers-color-scheme) + 明示トグル (GitHub / system-aware web apps)
  stance: modeled_on
  aspects:
  - 既定は system 連動。ユーザーが light/dark を明示選択したらそれを data-theme で優先・localStorage 永続化
  - テーマ UI は segmented control (System/Light/Dark, role=radiogroup + role=radio/aria-checked)。cycle
    ボタンより 3 択が self-evident で発見性が高い
  - テーマ state は CSS data 属性 / custom property で表現し、xterm 配色もトークン連動で追従させる
  - prefers-reduced-motion を尊重しアニメーション (spinner/flash/slide/transition) を抑制する
- name: GitHub Primer / Radix Colors の semantic color token 階層
  stance: modeled_on
  aspects:
  - 前景/背景ペアを semantic token (--fg / --fg-muted / --bg / --bg-elevated / --accent /
    --status-*) として宣言し各テーマで AA 実測値を割当てる
  - muted を opacity でなく独立トークンで表現 (背景依存のコントラスト破綻を構造的に防ぐ)。status badge 色も token 化し light/dark
    で別値を持つ
- name: Gmail / Material Snackbar の action + Undo パターン
  stance: modeled_on
  aspects:
  - セッション切替成功時に短命 status で 'Switched to <label>' を出し、tap で直前セッションへ戻す Undo を一定時間提示
    (破壊的でない操作にも回復経路)
- name: palette ADR-0057 の単一 aria-live slot
  stance: modeled_on
  aspects:
  - NotificationToast の各 item 個別 <output aria-live> をコンテナ 1 つの単一 aria-live='polite'
    role='status' に統合 (多重読み上げを防ぐ)。inline ハードコード配色を semantic token + env(safe-area-inset)
    に置換し bottom 寄せ
- name: Termius / Blink Shell 等モバイルターミナルの read-first + 補助バー割り切り
  stance: modeled_on
  aspects:
  - 閲覧/スクロールは快適に、フル打鍵は仮想キー補助バー前提という割り切り。短文入力は通常 text field (palette param) へ逃がし、xterm
    直接打鍵を MVP スコープから外す (案B = read-first を MVP、案A = 補助バーを後続)
- name: ネイティブ <dialog> showModal() への移行
  stance: rejected
  aspects:
  - 既存パレットは store-driven open state + role=dialog generic container で focus trap /
    IME 抑止 / submit freeze を成立させており (ADR-0036/0039/0055)、native <dialog> showModal()/HTMLDialogElement
    API はこの store 駆動と composability が悪い
  - 刷新の主眼はレイアウト/テーマ/一貫性であり、成熟した palette の内部状態機械を作り直さない (要件で明示)
- name: 重量級 CSS フレームワーク / CSS-in-JS ランタイム (Tailwind 全部入り / styled-components 等)
  stance: rejected
  aspects:
  - CSP が script-src 'self' で単一 chunk 出力、本体依存を React/Zustand/xterm の 4 つに絞っている制約に反する
    (bundle 増・inline style/CSP 摩擦)
  - デザイントークンは CSS custom properties で表現可能で、フレームワーク無しで token 体系・テーマ切替・レスポンシブを達成できる
- name: スマホでのターミナル全打鍵を pointer-touch のソフトキーボードだけに委ねる素朴実装 (補助バーなし)
  stance: rejected
  aspects:
  - xterm.js は touch 選択/スクロール/IME に制約があり、補助なしのソフトキーボードでは Esc/Ctrl/方向キーが打てず実用に耐えない
  - モバイル打鍵を本気で支えるなら仮想キー補助バー (案A) が必要。素朴な『ソフトキーボードに丸投げ』は採用しない (案A=補助バー vs 案B=read-first
    のトレードオフと採否は Open Questions)
states:
- 'PC レイアウト (>1024px): sidebar 常時表示 + 中央ターミナル/タブ + ヘッダー。named grid areas (''banner
  banner'' ''header header'' ''sidebar main'') で宣言、token-based 余白で再構成 (legacy の未使用
  auto 列を廃止)'
- 'タブレットレイアウト (768-1024px): sidebar は折り畳み可能 (collapse toggle)。展開時は push、折り畳み時は icon-rail
  もしくは hidden'
- 'スマホレイアウト (<768px): 単一ペイン。grid areas は単一カラム (''banner'' ''header'' ''main'')、sidebar
  area を drop。sidebar は off-canvas drawer (ハンバーガー = 正典 / 左端スワイプ = touch 補助で開閉)'
- 'drawer open 状態 (スマホ): 半透明 scrim + role=''dialog'' aria-modal=''true'' drawer panel
  が前面。focus が drawer 内に trap、scrim 配下メインは inert / aria-hidden。Esc / scrim tap / 選択
  / 左スワイプで閉じる'
- テーマ = light / dark / system(初期既定)。data-theme 属性 (明示選択時, localStorage 永続) または prefers-color-scheme
  (system 時) で配色トークンが切替わり、xterm 配色も同トークンに連動
- 'セッション 0 件 (empty state): drawer/sidebar に『セッションがない』旨 + ''Sessions are in the menu
  ☰'' ヒント + New Session 導線。中央は空状態プレースホルダ'
- 'セッション選択あり: DriverViewPanel (title/subtitle/tags/elapsed/RunStateBadge) + MainTabs
  (TERMINAL/TRANSCRIPT/EVENTS, roving tabindex tablist)'
- 'RunState: running/waiting = active (green + spinner badge, reduced-motion で静止)、idle/stopped/pending
  = 非アクティブ表現、unknown = 中立表現。すべて icon+text+色で多重符号化'
- '接続状態: connected / reconnecting / closed / daemonDisconnected — StatusBanner が非
  connected 時に表示。非 connected 中はセッション切替/送信が失敗導線 (disabled もしくは inline error) を持つ'
- 'palette open 状態: 全デバイスで dialog overlay。focus trap・aria-modal・IME 抑止を維持。スマホでは全幅
  sheet 風'
- 'submitting (palette): 入力 freeze + 単一 aria-live slot ''Sending…'''
- 'send failure (palette): freeze 解除 + palette は開いたまま + inline error を同一 aria-live
  slot で announce + retry/Esc'
- 'iOS Safari 動的ツールバー / 仮想キーボード: 可視ビューポート変動時もレイアウトが破綻しない (100vh 脱却し dvh / safe-area-inset
  使用)'
- '通知トースト表示: 1〜3 件のスタック。コンテナ 1 つが単一 aria-live=''polite'' role=''status'' (各 item は
  live を持たない)。スマホは bottom 寄せ + safe-area-inset オフセット、auto-dismiss (5s) + tap dismiss'
edge_cases:
- スマホ幅で displayLabel が長い → drawer/sidebar 内で 2 行 ellipsis (-webkit-line-clamp:2) に方針統一
  (現状 word-break と ellipsis 混在を解消)、44px 行高確保
- iOS Safari でアドレスバー出没により可視高さがジャンプ → terminal-host 高さが再計算され dvh + ResizeObserver/refit
  で吸収、xterm fit が 0 rows / overflow にならない (出没の両方向)
- スマホで仮想キーボード出現 → ビューポート縮小でターミナル/パレット入力が隠れない (palette は全幅 sheet で input が visualViewport
  に追従。案A 採用時は補助バーも追従)
- drawer open 中に画面回転 / リサイズで breakpoint を跨ぐ → PC 幅に戻ったら drawer を閉じ sidebar 常時表示へ。drawer
  が消えたとき trap 中の focus を有効な祖先 (ハンバーガー相当) へ移し focus loss を防ぐ。連続リサイズ / slide アニメ中の跨ぎでも状態が破綻しない
- prefers-color-scheme が system でユーザーが OS テーマを切替 → 明示トグルで上書きしていない限りリアルタイム追従
- 'prefers-reduced-motion: reduce → spinner / flash / transition / drawer slide を
  @media block で一律抑制 (現状 spinner は無条件 animation)'
- タッチターゲットが 44x44px 未満 → session list 行・タブ・close/back・トースト dismiss・テーマ segment がすべて
  44px 以上
- screen-reader でテーマ/drawer トグルの状態 → aria-checked (radio) / aria-expanded (drawer)
  で開閉・選択状態が読み上げられる
- タブレットの touch + 外付けキーボード混在 → hotkey (Cmd/Ctrl+K 等) と touch affordance (ハンバーガー/segment)
  が同時到達し競合しない。スワイプは touch 補助で keyboard/mouse 到達性に影響しない
- 'WCAG AA: tag pill は driver 提供の任意 fg/bg を inline 直当て (現状)。任意色のコントラスト比を計算し閾値 (本文
  4.5:1) 未満なら縁取り / 前景反転の fallback を当てて最低限の可読性を保証する'
- 通知トーストが safe-area (ノッチ/ホームインジケータ) に重なる → env(safe-area-inset-*) 分のオフセット。スマホは bottom
  寄せ (片手到達域)
- セッション 0 件で activeSession が null → 中央ペインの空状態と drawer の空状態が両方破綻しない
- status badge の muted 表現を opacity でなく独立トークン (--fg-muted) で持つ → 背景依存のコントラスト破綻を防ぐ (driver-view-subtitle/footer
  の opacity:0.7/0.75、palette-progress opacity:0.8 を token 化)
- breakpoint 値は <768/768-1024/>1024 を初期案とするが境界 (768px ちょうど・タブレット帯) の振る舞いは最適化役が実機/コンテンツ幅で確定する。現
  scenario は 375px (スマホ) を基準に固定 (境界値が動いてもスマホ帯の観察は安定)
assumptions:
- 技術スタック (React 18 / TS strict / Vite / Zustand / xterm.js) は ADR-0019 で確定済みで維持する。CSS
  は生 CSS (app.css + view.css) 継続を第一候補とし、デザイントークンは CSS custom properties で表現する (CSS-in-JS
  / 重量級フレームワーク導入は bundle 増 + CSP script-src 'self' 制約のため原則回避)
- Zustand store は pure (DOM 非操作)、wire/persistence 型は stdlib のみ という不変条件を刷新後も守る。レイアウト/テーマ/drawer/Undo
  の状態は CSS 媒体 (media query / data 属性 / custom property) と最小の UI-local state で表現し store
  形状は変えない
- activeSessionID は client 単独管理。drawer 開閉・テーマ選択・直前セッション ID (Undo 用) など新 UI 状態は daemon
  の view-update に乗せない (複数セッションでの勝手な切替 wedge を避ける)
- breakpoint は概ね <768px スマホ (drawer + 単一ペイン)、768-1024px タブレット (折り畳み可 sidebar)、>1024px
  PC (常時 sidebar)。具体境界値は最適化役が実機/コンテンツ幅で確定する (scenario は 375px 基準で境界非依存)
- 'テーマは推奨確定: 既定 = system 連動 (prefers-color-scheme)、明示選択時は data-theme 属性で上書き + localStorage
  永続、ライト/ダーク両対応 (ダーク基調片寄せにしない)。既存ダーク配色 (#1e1e1e 系) はダークトークン初期値として継承するが WCAG AA を満たすよう再調整する'
- xterm.js のテーマ (前景/背景/カーソル色) はデザイントークン (CSS custom property) と連動させ、light/dark/system
  いずれの解決経路でもターミナル配色が追従する
- テーマトグルは header の segmented control (System/Light/Dark, role=radiogroup) に確定配置。スマホでヘッダー幅が狭い時は
  icon ボタン → bottom-sheet で同 control を開き、drawer に隠れず全 modality から到達可能
- モバイルのターミナル本格打鍵は read-first (案B) を MVP 推奨デフォルトとし、案A (仮想キー補助バー) を progressive enhancement
  として後続に残す。短文送信は palette param input に逃がす。最終確定 (案A をいつ入れるか) は Open Questions
- ハンバーガーが drawer 開閉の正典な affordance (全 modality 等価)。左端スワイプは touch 限定の補助で、視覚的痕跡を出さず
  keyboard/mouse 到達性には影響しない
- muted/secondary テキストは opacity でなく独立 semantic token (--fg-muted 等) で表現する。semantic
  token は --fg / --fg-muted / --bg / --bg-elevated / --accent / --status-running/-waiting/-idle/-stopped
  を最小集合とし、各テーマで AA 実測値を割当てる
- ファイル 500 行 / 関数 80 行 / 新規・修正にテスト必須 という規約を刷新作業全体に適用する。既存 keyboard/ARIA (focus trap・aria-live
  単一 slot・skip-navigation・IME 抑止) を退行させない
- 通知トーストの legacy 実際位置は top-right (top:16px right:16px, 確認済)。要件記載の『右下』との差異は刷新で解消し、スマホは
  safe-area を尊重した bottom 寄せに再定義する (片手到達域)
methodology: atdd
legacy_context:
  source_implementation: '既存稼働中の Web client (src/client/web)。App.tsx (CSS grid ''grid-template-columns:
    280px 1fr auto; grid-template-rows: auto 1fr''、header に Command (⌘K) ボタン + 独立
    New Session ボタンの 2 つ、.app-header セレクタは app.css に未定義で grid auto-placement 任せ)、css/app.css
    + view.css (生 CSS 2 ファイル、:root に --bg:#1e1e1e/--fg/--accent/--warn のみ、メディアクエリ
    0 件、ダーク固定)、SessionList.tsx (左 280px 固定 sidebar・status spinner・displayLabel chain
    ADR-0033、word-break:break-word)、TerminalPane.tsx (xterm.js + FitAddon + ResizeObserver、activeSessionID
    keyed remount ADR-0030、flex:1 1 0 + min-height:0 ADR-0029、rAF coalesce refit ADR-0034、100vh
    依存)、MainTabs.tsx (TERMINAL/TRANSCRIPT/EVENTS 排他タブ、role=tab/tablist/ tabpanel はあるが
    onKeyDown / roving tabindex 0 件、terminal は常時 mount し CSS で可視切替)、DriverViewPanel.tsx
    (title/subtitle/tags/elapsed + RunStateBadge ADR-0032、TagPill は driver 任意色を style.color/backgroundColor
    で inline 直当て)、NotificationToast.tsx (各 item が個別 <output aria-live=''polite''>、inline
    ハードコード配色、 position top:16px right:16px 固定、5s auto-dismiss、最大 3 件)、StatusBanner.tsx
    (上部全幅 daemon 切断警告)、 components/palette/* (2 フェーズ・Cmd/Ctrl+K・focus trap・aria-live
    単一 slot・IME 抑止・unified listbox ADR-0050・ submit freeze ADR-0055)。view.css の .run-state-spinner
    / .session-status-spinner は animation:run-state-spin 0.8s linear infinite を無条件適用
    (reduced-motion guard なし)。run-state-idle=#555 bg / run-state-unknown=#333 bg でコントラスト未レビュー。'
  inherited_behaviors:
  - Cmd/Ctrl+K (capture phase) でのパレット起動、↑↓/Ctrl-P/N navigation、Tab/Shift+Tab focus
    trap、Enter 確定、Esc の phase back/close、IME composition 中の誤送信抑止
  - role=dialog/aria-modal、role=listbox + aria-activedescendant、aria-live=polite 単一
    slot、close 時の opener への focus 復帰、palette open 時の xterm blur
  - TerminalPane の xterm.js + FitAddon + ResizeObserver + rAF coalesce refit (ADR-0029/0030/0034)
    と sessionId filter による stale output 排除、常時 mount での scrollback 保持
  - displayLabel の title→subtitle→id trim chain (ADR-0033)、RunStateBadge の running=green+spinner
    / idle=gray テキストバッジ (ADR-0032)
  - MainTabs が terminal を常時 mount し CSS で可視切替する (xterm scrollback と subscribe/unsubscribe
    lifecycle を tab 切替で保持)
  - 色 + icon + text による状態の多重符号化
  - activeSessionID の client 単独管理 (daemon の state.ActiveSession を view-update に乗せない)
  replaced_behaviors:
  - 280px 固定 sidebar が collapse されずスマホ幅で破綻する → breakpoint に応じた適応レイアウト (PC=常時 sidebar
    / タブレット=折り畳み可 / スマホ=off-canvas drawer) に置換
  - メディアクエリ 0 件 (レスポンシブ皆無) + .app-header の grid 未配置 → named grid areas (banner/header/sidebar/main
    を breakpoint で差し替え) + container/safe-area 対応を導入
  - ダークテーマ固定 (#1e1e1e ハードコード、prefers-color-scheme 非連動) → デザイントークン体系 + light/dark/system
    3 テーマ + segmented control 明示トグル + xterm 配色連動 + localStorage 永続に置換
  - フォント階層なし (ui-monospace のみ、h3/p/small 不統一) → タイポグラフィ階層 (font scale / weight / line-height
    トークン) を確立
  - 100vh 依存 (iOS Safari 動的ツールバー未対策) → dvh + safe-area-inset に置換 (出没の両方向に追従)
  - タッチ非対応 (44px ターゲット保証なし、touch affordance 不在) → 最小 44x44px タッチターゲット + drawer 開閉ジェスチャ等の
    touch affordance を追加
  - 配色コントラスト未レビュー (run-state-idle=#555 等) → WCAG AA を満たすトークン値に再調整、muted を opacity
    でなく独立トークンで表現
  - NotificationToast/StatusBanner の inline style ハードコード配色 + top-right 固定 + item 個別
    aria-live → semantic token + 単一 aria-live slot + safe-area 尊重 bottom 寄せに統一
  - MainTabs が onClick のみで onKeyDown / roving tabindex 0 件 (APG tabs 未充足) → WAI-ARIA
    APG roving tabindex + Arrow/Home/End keyboard interaction に置換
  - 'spinner が animation:infinite を無条件適用 (reduced-motion 非対応) → @media (prefers-reduced-motion:
    reduce) で spinner/flash/slide/transition を一律抑制'
  - TagPill が driver 任意色を inline 直当てしコントラスト無保証 → 任意色のコントラスト比を計算し閾値未満なら縁取り/前景反転 fallback
    を当てる
summary: agent-grid-new の Web client (src/client/web) を、情報設計・認知負荷・一貫性・アクセシビリティ・既存プラットフォーム慣習
  の観点でモダンに全面刷新する。最重視するのは UX。対象は PC・スマホ・タブレットの 3 デバイスすべてで、いずれでも破綻しない適応レイアウトを実現する。
---

<!-- migrated_from: docs/specs/2026-06-25-web-ui-redesign/ux.md -->

## Goal

agent-grid-new の Web client (`src/client/web`) を、**情報設計・認知負荷・一貫性・アクセシビリティ・既存プラットフォーム慣習** の観点でモダンに全面刷新する。最重視するのは UX。対象は PC・スマホ・タブレットの 3 デバイスすべてで、いずれでも破綻しない**適応レイアウト**を実現する。

刷新は 3 つの柱で成る。(1) **適応レイアウト** — sidebar を PC=常設 / タブレット=折り畳み可 / スマホ=off-canvas drawer に振り分け、named grid areas を breakpoint で差し替える骨格、dvh + safe-area-inset、44px タッチターゲット。(2) **デザイントークン体系 + テーマ** — semantic な CSS custom property (`--fg` / `--fg-muted` / `--bg` / `--bg-elevated` / `--accent` / `--status-*`)、タイポグラフィ階層、WCAG AA コントラスト、light / dark / system(既定)の 3 テーマと segmented control 明示トグル + xterm 配色連動。(3) **視覚的一貫性 + a11y 波及** — コマンドパレットで熟成済みの視覚言語 (border/radius/spacing/タイポ token) と a11y パターン (unified listbox / focus trap / 単一 aria-live slot / disabled visible+skip-navigation / IME 抑止) を session list・MainTabs・toast 等の全画面へ波及させる。

不変条件として、既存の keyboard / ARIA / IME 資産 (Cmd/Ctrl+K capture phase・↑↓/Ctrl-P/N・focus trap・aria-live 単一 slot・skip-navigation・IME 誤送信抑止) を退行させない。Zustand store は pure (DOM 非操作)、wire/persistence 型は stdlib のみ、`activeSessionID` は client 単独管理という設計上の不変条件も維持する。

### Reference UX

- **Modeled on**:
  - Material / iOS navigation drawer — off-canvas drawer + scrim + aria-modal。**ハンバーガー (☰) を全 modality 等価の正典 affordance とし、左端スワイプは touch 限定の補助** (Material 公式ガイダンス: スワイプ単独に依存しない)。
  - VS Code / Linear / Raycast の **search-bar-as-entry-point** — header の 2 ボタン (Command + New Session) を単一の検索バー風トリガに集約 (虫眼鏡 + placeholder + ⌘K/Ctrl+K hint badge、New Session はプログレッシブ開示)。確立済み palette の token と a11y パターンを正典として他画面へ波及。
  - WAI-ARIA APG Tabs Pattern — MainTabs に roving tabindex + Arrow/Home/End を実装 (legacy の onClick-only / onKeyDown 0 件を解消)。
  - GitHub 等の system-aware theme — 既定 system 連動、明示選択は data-theme + localStorage 永続、segmented control (radiogroup) で 3 択を self-evident に。
  - GitHub Primer / Radix Colors の semantic token 階層 — muted を opacity でなく独立トークンで表現。
  - Gmail / Material Snackbar の action + Undo — 誤切替からの回復を 1 tap に。
  - palette ADR-0057 の単一 aria-live slot — toast の多重読み上げを解消。
  - Termius / Blink Shell の read-first + 補助バー割り切り — モバイル打鍵の現実的設計。
- **Rejected**:
  - native `<dialog>` showModal() 移行 — store-driven palette との composability が悪く、成熟した内部状態機械を作り直さない方針に反する。
  - 重量級 CSS フレームワーク / CSS-in-JS ランタイム — CSP script-src 'self' / 単一 chunk / 依存 4 つの制約に反する。token は CSS custom property で達成可能。
  - スマホのターミナル全打鍵をソフトキーボードだけに委ねる素朴実装 — xterm の touch/IME 制約で Esc/Ctrl/方向キーが打てず実用に耐えない (補助バー=案A 前提)。

### Migration Context

本刷新は既存稼働中の Web client の **refactor / migration** である。現状追認を排除するため、各 flow は最低 1 件の `vs Legacy: must-fail` scenario を持つ。

- **source_implementation**: `src/client/web` 一式。App.tsx (grid `280px 1fr auto`、header 2 ボタン、`.app-header` は grid 未配置)、css/app.css + view.css (生 CSS 2 ファイル、メディアクエリ 0 件、`#1e1e1e` ダーク固定)、SessionList / TerminalPane (100vh 依存) / MainTabs (role はあるが onKeyDown 0 件) / DriverViewPanel (TagPill 任意色 inline 直当て) / NotificationToast (top-right 固定・item 個別 aria-live) / StatusBanner / palette/*。view.css の spinner は `animation infinite` 無条件 (reduced-motion guard なし)、`run-state-idle=#555` 等コントラスト未レビュー。
- **inherited_behaviors** (退行させない): palette の keyboard/ARIA/IME 一式、TerminalPane の xterm + refit (ADR-0029/0030/0034) と常時 mount での scrollback 保持、displayLabel chain (ADR-0033)、RunStateBadge (ADR-0032)、状態の icon+text+色 多重符号化、activeSessionID の client 単独管理。
- **replaced_behaviors** (delta): 280px 固定 sidebar → 適応レイアウト、メディアクエリ 0 件 + `.app-header` grid 未配置 → named grid areas、ダーク固定 → token + 3 テーマ + xterm 連動、フォント階層なし → タイポ階層、100vh → dvh + safe-area、touch 非対応 → 44px + drawer ジェスチャ、コントラスト未レビュー → WCAG AA (muted を独立トークンで)、toast inline 配色 + top-right + item 別 aria-live → token + 単一 slot + bottom safe-area、MainTabs onKeyDown 0 件 → APG roving tabindex、spinner 無条件 animation → reduced-motion 抑制、TagPill 任意色 inline → コントラスト fallback。

## Target Users

- **複数 coding-agent セッションを並行運用する開発者** (筆頭): 主作業環境は PC。外出先・離席中はスマホ/タブレットで running/idle のグランス確認・セッション切替・軽操作を行う。セッション切替は頻繁で、密な list での誤タップが起きやすい (→ Undo 経路が要る)。
- **keyboard 主体のパワーユーザー**: Cmd/Ctrl+K パレット・矢印/Ctrl-P/N・focus trap に依存。MainTabs も矢印で切替えたい (現状不可)。
- **screen-reader 利用者**: role / aria-live / aria-activedescendant / aria-expanded / aria-checked に依存して状態 (drawer 開閉・テーマ選択・running/idle・disabled 理由・切替先 label) を把握する。drawer の背後到達が起きないこと (aria-modal/inert) が要件。
- **モバイルでグランス確認 + 軽操作するユーザー**: 主操作は閲覧・セッション切替・パレット起動・短いコマンド送信。xterm 直接フル打鍵は前提にしない。
- **タブレットを touch + 外付けキーボードのハイブリッドで使うユーザー**: hotkey と touch affordance が同時到達し競合しないこと。スワイプは touch 補助に留め、keyboard/mouse 到達性に影響させない。

## Primary Flows

各 flow の entry observation (最初の Given) と exit observation (最後の Then) を観察可能な事実で固定し、各 flow に `vs Legacy: must-fail` を最低 1 件持たせて現状追認を排除した。activation modality は各 step の `[...]` プレフィクスで明示する。

- **F-001 スマホでセッション状態を確認し切り替える (drawer 経由)**: スマホ幅の中核フロー。ハンバーガー (正典) で drawer を開き、aria-modal + 背後 inert を満たし、行選択で切替 + 切替先 announce + Undo、scrim/Esc/スワイプはキャンセル (activeSession 不変)。entry = 375px で sidebar off-canvas / ハンバーガー visible / 横スクロールなし。exit = 選択後 drawer 閉じ + focus 復帰 + 中央更新 + Undo。
- **F-002 テーマを system 連動から明示選択へ切り替える**: 要件の二大柱。header の segmented control (radiogroup) でスマホでも到達可能 (狭幅時は icon→sheet)。entry = system 連動で OS=light なら light トークン + AA。exit = 明示 light が data-theme + xterm 連動 + localStorage 永続。reduced-motion で transition 抑制。
- **F-003 いずれのデバイスでもコマンドパレットを起動して操作を実行する**: 熟成済み palette を全デバイスで起動 (検索バー風トリガ / ⌘K / スマホ全幅 sheet)。token と a11y パターンを他画面と共有。送信失敗パスを含む。entry = 起動後 dialog + input focus + xterm blur。exit = 送信 freeze / 失敗時 inline error / Esc で opener 復帰。
- **F-004 アクティブセッションのターミナル/トランスクリプト/イベントを閲覧する**: 全デバイス共通の最小集合。DriverView + APG roving tabindex tablist + 常時 mount scrollback + dvh/safe-area refit。モバイル打鍵は read-first (案B) を MVP とし観察対象は閲覧/スクロール/タブ切替/状態把握に固定 (案A/案B の採否は Open Questions)。entry = 375px で RunStateBadge + terminal-host height>0 + 横スクロールなし。exit = タブ往復で scrollback 保持 + ツールバー出没両方向に追従。

## Acceptance Scenarios

各 UAC に判別性論証 (Counterexample) と refactor delta (vs Legacy) を併記する。Given/Then は DOM / aria / 視覚 / focus の観察事実のみで記述し、内部 state には言及しない。

### UAC-001 (F-001 entry / 適応レイアウト)
**Counterexample**: 280px sidebar を `@media` で `left:-280px` の画面外に absolute 配置し、ハンバーガーを `aria-expanded='false'` で常時描画するが `onClick` は未配線 (no-op) の死んだ UI。初期状態の『list off-canvas / ハンバーガー visible / 横スクロールなし』だけを Then にするとこの死んだ UI でも pass する。よって When に『tap する』を含め、Then を『tap 後に aria-expanded が true になり list が visible になる』まで縛り、配線されていない drawer を fail させる。
**vs Legacy**: must-fail — legacy はメディアクエリ 0 件で sidebar 常時可視のため初期の off-canvas で既に fail する。

### UAC-002 (F-001 / a11y modal)
**Counterexample**: drawer を前面に出し focus も移すが背後を inert/aria-hidden にしない実装。focus trap だけでは VoiceOver の rotor / 仮想カーソルが背後へ到達でき modal の意図が screen-reader で破れる。Then に『scrim 配下が inert/aria-hidden』を含めて fail させる。
**vs Legacy**: must-fail — legacy に drawer 概念が無く role=dialog/aria-modal の drawer は存在しない。

### UAC-003 (F-001 exit / Undo 回復性)
**Counterexample**: 行 tap で切替え drawer を閉じるが Undo 経路を持たない実装。Then の『Undo tap で直前セッションへ戻る』を満たせず fail する。
**vs Legacy**: must-pass — 行 tap での切替 + 中央更新自体は legacy でも成立する (Undo は新規付加だが切替挙動の保護として must-pass)。

### UAC-004 (F-001 / 選択クローズ vs 取消クローズの判別)
**Counterexample**: scrim tap と行 tap を同じ『閉じる』ハンドラに流し、scrim tap で直近 highlight 行を誤選択する実装。Then の『activeSession が変わらない』を満たせず fail する。
**vs Legacy**: irrelevant — drawer 自体が新規領域。

### UAC-005 (F-002 entry / system 追従 + AA + xterm)
**Counterexample**: トグル onClick 内で `xterm.options.theme` を `#ffffff` にハードコード直書き (custom property 非連動 / system 追従ロジックなし)。これは『light を明示選択した時 xterm が light』だけなら pass するが、本 scenario は『明示選択していない初回ロードで prefers-color-scheme: light に追従して xterm も light』を縛るため初期ダーク据え置きの直書きは fail。さらに 4.5:1 を Then に含めるので色だけ替えてコントラスト未調整でも fail。
**vs Legacy**: must-fail — legacy は prefers-color-scheme 非連動で常に `#1e1e1e` ダーク固定。

### UAC-006 (F-002 / data-theme + xterm token 連動)
**Counterexample**: body だけ light にして xterm を `#1e1e1e` 据え置き (fail)、あるいは xterm を別の直書きハードコード色にする実装。後者を排除するため Then に『同一トークン経由 (custom property を読む path)』を含め、UAC-005 (system 追従) と組で fail することを縛る。
**vs Legacy**: must-fail — legacy にテーマ切替が存在しない。

### UAC-007 (F-002 / 永続化)
**Counterexample**: data-theme を localStorage に永続化せずセッション中 state でのみ保持する実装。リロードで state が失われ system (OS dark) に戻り Then の『light のまま』を満たせず fail する。
**vs Legacy**: irrelevant — テーマ選択が新規領域。

### UAC-008 (F-003 entry / スマホ全幅 sheet)
**Counterexample**: legacy の `.palette-dialog` は `max-width:600px / width:90% / margin-top:10vh` 固定で 375px では中央に小さめの箱。スマホ全幅 sheet (左右マージン各 16px 以内) を満たせず fail。input に focus が当たらなければそこでも fail。
**vs Legacy**: must-fail — legacy の固定 max-width はスマホ全幅 sheet にならない。

### UAC-009 (F-003 / disabled skip-navigation 退行防止)
**Counterexample**: navigation が disabled 行に止まる実装。Then の『aria-activedescendant が disabled 行を指さない』を満たせず fail する。
**vs Legacy**: must-pass — legacy (ADR-0050) で既に成立。退行防止。

### UAC-010 (F-003 / IME 抑止 退行防止)
**Counterexample**: composition 中も Enter で tool を確定する実装。Then の『phase が toolSelect のまま』を満たせず fail する。
**vs Legacy**: must-pass — legacy で既に成立。退行防止。

### UAC-011 (F-003 / 視覚的一貫性のトークン共有 = 要件主眼3)
**Counterexample**: palette だけ新トークンで再設計し session list は legacy のハードコード値 (`border:1px solid #555` 等) のまま据え置く実装。視覚的に別デザインのままだが個々の画面は動く。Then に『同一 custom property に解決され computed 値が一致』を含めてトークン非共有 (=一貫性の宣言止まり) を fail させる。
**vs Legacy**: must-fail — legacy は palette だけ別デザインのため session list と token が一致しない。

### UAC-012 (F-003 / 送信失敗パス = missing_failure_path)
**Counterexample**: 送信失敗時に freeze を解かず palette が 'Sending…' のまま固まる、あるいは失敗を握り潰して何も announce せず閉じる実装。Then の『freeze 解除 + inline error の announce + 開いたまま』を満たせず fail する。
**vs Legacy**: irrelevant — 失敗導線の明確化は新規。

### UAC-013 (F-004 entry / 適応レイアウト + RunState)
**Counterexample**: legacy は 280px sidebar が常時占有するため 375px では中央ペインが横にはみ出すか押し出される。『375px で横スクロールなしに DriverView/MainTabs が収まる』を満たせず fail する。
**vs Legacy**: must-fail — legacy の固定 sidebar で 375px は横スクロールが出る。

### UAC-014 (F-004 / APG roving tabindex 波及 = 要件主眼3)
**Counterexample**: タブを onClick のみで実装し onKeyDown / roving tabindex が無い legacy 実装 (確認済: MainTabs に onKeyDown 0 件)。矢印キーでタブ移動できず Then の『矢印だけで移動し aria-selected が移る』を満たせず fail する。scrollback 保持 (常時 mount) は legacy で成立しているので矢印 nav を Then に含めて keyboard 一貫性波及を縛る。
**vs Legacy**: must-fail — legacy は矢印キーでタブ移動できない。

### UAC-015 (F-004 exit / dvh + safe-area 両方向)
**Counterexample**: 高さを `100vh` のまま据え置き `overflow:hidden` だけ足す実装。667px→600px の縮小方向では ResizeObserver+refit (ADR-0034) が fit() を再実行し height>0 を保つため縮小片側だけの Then なら pass する。本 scenario は『拡大方向 (600px→667px) にも追従』と『bottom が safe-area-inset を尊重』を Then に含めるので、100vh は大きい方を指し続け拡大時に可視高さへ追従せず safe-area 未導入で fail する。
**vs Legacy**: must-fail — legacy は 100vh 依存で拡大方向に可視高さへ追従せず safe-area 未対応。

### UAC-016 (F-002 / reduced-motion = a11y 拡張)
**Counterexample**: view.css の `.run-state-spinner` / `.session-status-spinner` が `animation:run-state-spin 0.8s linear infinite` を無条件適用する legacy 実装 (確認済、guard なし)。reduce 環境でも回り続け Then の『animation none/0』を満たせず fail する。`@media (prefers-reduced-motion: reduce)` guard の欠落を縛る。
**vs Legacy**: must-fail — legacy は reduced-motion 非対応で spinner が回り続ける。

## Edge Cases

- **長い displayLabel** (スマホ幅): drawer/sidebar 内で 2 行 ellipsis (`-webkit-line-clamp:2`) に方針統一 (現状 word-break と ellipsis 混在を解消)、44px 行高確保。
- **iOS Safari アドレスバー出没** (両方向): terminal-host 高さが dvh + ResizeObserver/refit で吸収され、xterm fit が 0 rows / overflow にならない。
- **スマホ仮想キーボード出現**: palette は全幅 sheet で input が `visualViewport` に追従し隠れない (案A 採用時は補助バーも追従)。
- **drawer open 中の breakpoint 跨ぎ** (回転/リサイズ): PC 幅に戻ったら drawer を閉じ sidebar 常設へ。drawer が消えたとき trap 中の focus を有効な祖先 (ハンバーガー相当) へ移し focus loss を防ぐ。連続リサイズ / slide アニメ中の跨ぎでも破綻しない。
- **OS テーマ切替** (system 時): 明示トグルで上書きしていない限りリアルタイム追従。
- **prefers-reduced-motion: reduce**: spinner / flash / transition / drawer slide を `@media` block で一律抑制 (現状 spinner は無条件 animation)。
- **44px 未満タッチターゲット**: session list 行・タブ・close/back・トースト dismiss・テーマ segment がすべて 44px 以上。
- **テーマ/drawer トグルの screen-reader 状態**: `aria-checked` (radio) / `aria-expanded` (drawer) で開閉・選択が読み上げられる。
- **タブレット touch + 外付けキーボード混在**: hotkey と touch affordance が同時到達し競合しない。スワイプは touch 補助で keyboard/mouse 到達性に影響しない。
- **TagPill 任意色のコントラスト**: driver 提供 fg/bg の inline 直当て。任意色のコントラスト比を計算し閾値 (本文 4.5:1) 未満なら縁取り / 前景反転 fallback を当てる。
- **トーストの safe-area 重なり**: `env(safe-area-inset-*)` 分のオフセット。スマホは bottom 寄せ (片手到達域)。
- **セッション 0 件 (activeSession null)**: 中央ペインの空状態と drawer の空状態が両方破綻しない。
- **muted 表現の opacity 脱却**: muted を opacity でなく独立トークン (`--fg-muted` 等) で持ち、背景依存のコントラスト破綻を防ぐ (driver-view-subtitle/footer opacity:0.7/0.75、palette-progress opacity:0.8 を token 化)。
- **breakpoint 境界値の未確定**: `<768/768-1024/>1024` を初期案とするが境界 (768px ちょうど・タブレット帯) の振る舞いは最適化役が実機/コンテンツ幅で確定する。現 scenario は 375px (スマホ) 基準に固定し境界値が動いても観察が安定する。

## Open Questions

未解決の UX 判断。確定したら ADR に格上げするか frontmatter に反映する。

1. **モバイルでのターミナル本格打鍵 (案A vs 案B) — 要件で両案併記 + トレードオフ明示が必須**。両案を以下に並べ、MVP は案B を推奨スタンスとして置く (案A をいつ・どこまで入れるかが open)。

   | 観点 | 案A: 仮想キー補助バーでフル打鍵 | 案B: read-first で割り切り (MVP 推奨) |
   |---|---|---|
   | 目標 | スマホでも入力・スクロール・選択・コマンド送信を快適に。Esc/Ctrl/方向キー/Tab を補助バーで供給 | 閲覧・スクロール・セッション切替・パレット起動が主。本格打鍵は PC/タブレット前提 |
   | xterm.js タッチ制約 | touch 選択/スクロール/IME に制約があり、補助バー (modifier sticky key / 方向キー) を自前で実装し xterm key event に注入する必要 | 直接打鍵を期待させない。短文送信は palette param の通常 text input (ソフトキーボードで完結) に逃がす |
   | 実装難度 | 高い (補助バー UI + visualViewport 追従 + xterm への key 注入 + sticky modifier 状態管理 + テスト)。刷新の主眼 (レイアウト/テーマ/一貫性) を遅延させる | 低い (打鍵 affordance を出さない = 誤入力期待を作らない)。MVP スコープに収まる |
   | 想定利用文脈 | スマホを主作業端末にしうるユーザー、長時間モバイル運用 | 主作業は PC、スマホは離席先のグランス確認 + 軽操作 (target_users と整合) |
   | 共通の最小集合 | — | F-004 は両案共通の閲覧/スクロール/タブ切替/状態把握のみを観察対象に固定 (UAC-013/014/015) |

   **推奨**: MVP は案B (read-first) をデフォルトとし、短文送信は palette param input に逃がす。案A の仮想キー補助バーは progressive enhancement として後続フェーズの拡張点に残す。**Open**: 案A を MVP に含めるか、後続でいつ着手するか、補助バーのキー集合 (Esc/Ctrl/Tab/方向キー/`|`/`-` 等) の最小定義。

2. **breakpoint 境界値の確定**: `<768/768-1024/>1024` は初期案。最適化役が実機/コンテンツ幅 (sidebar 最小幅・ターミナル最小桁数) で境界値と、768px ちょうど・タブレット帯 (折り畳み/常設の中間) の具体的振る舞いを確定する必要がある。

3. **テーマトグルのスマホ配置の具体形**: header segmented control を確定したが、スマホ狭幅で icon→bottom-sheet を開く際、その sheet を palette とは別 UI にするか palette 内の 1 entry にするか (実装コストと発見性のトレードオフ)。

4. **New Session の検索バー風トリガへの吸収度**: palette 内最上位 suggested action に完全吸収するか、検索バー右の `+` icon button として明示分離を残すか (プログレッシブ開示の度合い)。要件確定の操作頻度データが無いため open。

5. **TagPill の AA fallback の具体アルゴリズム**: コントラスト比計算 (相対輝度 → 4.5:1 判定) のうえ縁取り / 前景反転 / 自動調整のいずれを採るか。実装難度が高いので最小実装 (縁取り付与) から始める案を推す。
