---
id: ux-20260713-agent-workspace-viewer
kind: ux
title: Agent workspace read-only file viewer
status: approved
created: '2026-07-13'
updated: '2026-07-13'
summary: 'Specify-confirmed UX for agent-grid の workspace read-only viewer: 8 primary
  flow / 25 acceptance scenario (全て counterexample 併記)、14 DP は 13 answered + 1 rejected、primary
  alternative は ALT-ACTIVITY-STREAM (user-confirmed)。'
goal: 'agent-grid web UI 上で、operator がエージェント (Codex/Claude 系) セッションの workspace を read-only
  で追跡・確認できる: (a) エージェントが今どのファイルに触れているかを turn-aggregated activity row から把握する、(b) そのファイルの内容または
  diff を live-background の modal drawer でその場で開く、(c) ファイル種別 (Markdown / Mermaid / JSON
  / source) に応じて構造化レンダリングされた形で読む、(d) activity と無関係に workspace 全体 (workspace root 境界内)
  を drawer の Tree タブから能動的に閲覧する。書き込み経路は構造的に存在せず、sensitive file (.env 等) もフィルタなしでそのまま表示する。'
target_users:
- agent-grid でエージェントセッションを監督する operator (host または container 上の workspace を直接操作せず、エージェントの作業を確認する人)
- 非同期レビュワー — セッション終了後にログではなく実際の workspace 状態・diff を見て作業内容を検証する人 (operator と同一人物のことが多いが観察タイミングが異なる)
primary_flows:
- id: F-001
  name: エージェントが触れたファイルを activity rail で追跡する
  steps:
  - '[visual] operator は terminal と並んで表示される独立 activity rail を確認できる'
  - '[agent-driven] エージェントが tool call (read/create/edit/delete) を実行する'
  - '[visual] tool call を含む turn が完了すると、rail に対象ファイルの workspace-relative path を含む
    row が追加される'
  - '[visual] 同一 turn 内で同一ファイルへの複数 tool call は 1 row に集約され、件数 badge が付く'
- id: F-002
  name: activity row から event kind に応じた viewer/diff/placeholder を開く
  steps:
  - '[pointer|keyboard] operator は rail 上の row を click するか row にフォーカスして Enter を押す'
  - '[visual] modal drawer が開き、背後の terminal / activity rail は視覚的に更新され続ける (live background)'
  - '[visual] drawer の content 領域に、event kind に対応する viewer / diff / metadata placeholder
    のいずれかが表示される'
  - '[pointer|keyboard] operator は Esc または scrim click で drawer を閉じ、背後 terminal へ
    focus が戻る'
- id: F-003
  name: 構造化コンテンツ (Markdown / Mermaid / JSON) を整形表示で読む
  steps:
  - '[pointer|keyboard] operator が Markdown ファイルの row を選択する'
  - '[visual] viewer に見出し・リスト・code fence が整形表示される'
  - '[pointer|keyboard] operator が Mermaid ブロックを含むファイルまたは .mmd ファイルの row を選択する'
  - '[visual] viewer に Mermaid 図が SVG として描画される'
  - '[pointer|keyboard] operator が .json ファイルの row を選択する'
  - '[visual] viewer に JSON が折りたたみ/展開可能なツリーとして表示される'
- id: F-004
  name: edit event の diff を git HEAD 基準でレビューする
  steps:
  - '[pointer|keyboard] operator が edit event row から drawer を開く'
  - '[visual] diff view が git HEAD 時点の内容と現在の内容を比較して表示される'
  - '[visual] 変更されていないファイルは rail に row として現れない'
- id: F-005
  name: drawer の Tree タブから workspace 全体を能動的に閲覧する
  steps:
  - '[visual] activity rail 上部に Workspace ラベルを持つ secondary-tree-access affordance
    が visible に配置されている'
  - '[pointer|keyboard] operator は当該 affordance を pointer または keyboard で activate
    し drawer を開く'
  - '[visual] drawer は Tree タブが initial focus として選択された状態で開き、workspace root 直下のディレクトリ/ファイル一覧が表示される'
  - '[pointer|keyboard] operator がまだ activity に現れていない任意のファイルを選択する'
  - '[visual] 選択したファイルが viewer で開かれる (goal-open-touched-file の viewer と同一の表示物)'
- id: F-006
  name: tree で workspace 構造の環境健全性を確認する
  steps:
  - '[pointer|keyboard] operator が drawer の Tree タブを開く'
  - '[visual] tree の root が workspace ディレクトリに固定され、それより上位の階層へ navigate する手段が存在しない'
  - '[visual] workspace 内でファイルが新規作成/削除されると、tree はその変化を反映する — 変化の反映は drawer/tree UI
    内に visible な refresh affordance (autoupdate 表示 / drawer 内 reload control / tree
    ヘッダー refresh ボタン等) を通じて観察できる。browser 全体リロード (F5) にのみ依存する path は refresh affordance
    として扱わない'
- id: F-007
  name: viewer 内を vim ライクな read-only motion + search で移動する
  steps:
  - '[keyboard] operator が viewer 内をクリックまたはタブ移動して viewer にフォーカスを移す'
  - '[keyboard] operator が j/k/gg/G を押してカーソル/ビューポートを移動する'
  - '[keyboard] operator が / を押して検索語を入力し n/N でマッチ間を移動する'
  - '[keyboard] operator が i や :w のような mutation 系キーを押しても viewer は編集モードに入らない'
- id: F-008
  name: 巨大テキストファイルと binary ファイルを開く
  steps:
  - '[pointer|keyboard] operator が 5MB 超のテキストファイルの row を選択する'
  - '[visual] viewer が仮想スクロールでファイル全体を表示可能にする (省略・truncate 表示なし)'
  - '[pointer|keyboard] operator が binary ファイル (例: .png) の row を選択する'
  - '[visual] viewer はファイル名・サイズ・MIME 種別のみを示す placeholder を表示する'
acceptance_scenarios:
- id: UAC-001
  flow_ref: F-001
  given: session の activity rail が表示されており、row が 0 件である
  when: エージェントが `src/foo.ts` に対して read tool call を実行し、その turn が完了する
  then: rail に 1 件の row が追加され、row のパス表示が `src/foo.ts` (workspace root からの相対パス) であり、host
    や container の絶対パスは表示されない
- id: UAC-002
  flow_ref: F-001
  given: activity rail に row が 0 件、エージェントの現在 turn が開始した直後である
  when: 同一 turn 内でエージェントが `src/foo.ts` を read → edit → read の順に 3 回操作し、turn が完了する
  then: rail には `src/foo.ts` に対する row が 1 件だけ表示され、その row に操作件数 (3) を示す badge が付き、row
    を選択すると 3 件の個別 event が展開表示される
- id: UAC-003
  flow_ref: F-002
  given: rail に read event の row が表示されている
  when: operator がその row を選択する
  then: drawer が開き、対象ファイルの全内容が読み取り専用の viewer として表示される (diff や metadata placeholder
    ではない)
- id: UAC-004
  flow_ref: F-002
  given: rail に create event (新規ファイル) の row が表示されている
  when: operator がその row を選択する
  then: drawer が開き、新規作成されたファイルの全内容が viewer として表示される
- id: UAC-005
  flow_ref: F-002
  given: rail に edit event の row が表示されている
  when: operator がその row を選択する
  then: drawer が開き、diff view が既定タブとして表示され、追加行・削除行・変更行が視覚的に区別される。viewer は 2nd タブとして切替可能である
- id: UAC-006
  flow_ref: F-002
  given: rail に delete event の row が表示されている
  when: operator がその row を選択する
  then: drawer が開き、削除前のファイル名・サイズ・種別を示す metadata placeholder が表示され、本文 viewer や diff
    は表示されない
- id: UAC-007
  flow_ref: F-002
  given: operator が `src/foo.ts` の edit event row から drawer を開いており、viewer/diff に内容が表示されている
  when: drawer が開いたまま、背後でエージェントが同じ `src/foo.ts` を再度編集する tool call を実行する
  then: drawer 内に stale であることを示すバナー (または reload affordance) が視覚的に表示され、screen reader
    も stale 状態をアナウンスする。表示中の内容が無警告のまま古い内容を保持し続けることはない
- id: UAC-008
  flow_ref: F-002
  given: workspace 内に `.env` ファイルが存在し、rail にそのファイルへの read event row が表示されている
  when: operator がその row を選択して drawer を開く
  then: '`.env` の内容がそのまま viewer に表示され、値のマスキングや reveal 操作の要求は一切発生しない'
- id: UAC-009
  flow_ref: F-002
  given: operator が `src/foo.ts` の edit event row から drawer を開いて viewer/diff を表示しており、activity
    rail には他ファイルの行はまだ現れていない
  when: drawer を開いたまま、背後でエージェントが別ファイル `src/bar.ts` への write tool call を実行し、その turn
    が完了する
  then: drawer は表示を維持しつつ、背後の activity rail に `src/bar.ts` の新規 row が可視化される。row は drawer
    を閉じるまで pending queue に溜め込まれず、drawer 表示中に現れる。terminal 領域の直近出力も止まらず更新され続ける
- id: UAC-010
  flow_ref: F-002
  given: drawer が state-stale-background-in-drawer にあり、stale であることを示すバナーと reload affordance
    が表示されている
  when: operator が reload affordance を pointer もしくは keyboard で activate する
  then: stale バナーが drawer から消え、viewer/diff 領域には最新のファイル内容 (背後で行われた変更を反映したもの) が表示される
- id: UAC-011
  flow_ref: F-003
  given: rail に `.md` ファイルの row が表示されている
  when: operator がその row を選択する
  then: viewer に見出し・リスト・code fence が視覚的に整形表示される (見出しはテキストサイズ/太さが変化し、リストは箇条書き記号を伴う)
- id: UAC-012
  flow_ref: F-003
  given: rail に Mermaid 記法を含むファイルの row が表示されている
  when: operator がその row を選択する
  then: viewer に Mermaid 図が SVG 図形として描画される
- id: UAC-013
  flow_ref: F-003
  given: rail に `.json` ファイルの row が表示されている
  when: operator がその row を選択する
  then: viewer に JSON がキー単位で折りたたみ/展開可能なツリー構造として表示される
- id: UAC-014
  flow_ref: F-004
  given: 対象ファイルは現在の turn 開始前 (git HEAD 時点) から既に変更されている状態で、今回の turn ではさらに追加の変更が加えられた
  when: operator が edit event row を選択して diff view を開く
  then: diff view には git HEAD 時点からの全ての変更行 (今回 turn 以前の変更を含む) が追加/削除として表示される
- id: UAC-015
  flow_ref: F-004
  given: workspace 内に、今回のセッションでエージェントが一切触れていないファイルが存在する
  when: operator が activity rail を確認する
  then: そのファイルに対応する row は rail に一切表示されない
- id: UAC-016
  flow_ref: F-004
  given: workspace が git 管理下にない (workspace ルート下に .git が存在しない) 状態で、rail に edit event
    の row が表示されている
  when: operator がその row を選択して drawer を開く
  then: drawer 内 diff 位置に、diff base が git HEAD で解決できないことを示す visible な劣化表示 (banner
    / 無効化 indicator / 明示的説明文言のいずれか) が現れる。通常の diff タブが無警告で表示されることは無い
- id: UAC-017
  flow_ref: F-005
  given: activity rail に row が 1 件も無い状態で、activity rail 上に Workspace ラベル付きの secondary-tree-access
    affordance が visible に表示されている
  when: operator が Workspace affordance を pointer もしくは keyboard で activate する (row
    を経由しない)
  then: drawer が開き、Tree タブが initial focus を持ち workspace root 直下の一覧が表示される。その後 tree
    上で任意のファイルを選択すると event kind に基づく row 選択と同じ viewer が現れる
- id: UAC-018
  flow_ref: F-006
  given: operator が drawer の Tree タブを開いている
  when: operator が tree の root ノードを確認する
  then: root ノードは workspace ディレクトリのみを示し、親ディレクトリやそのきょうだいディレクトリへ辿るための展開/リンクが tree 上に存在しない
- id: UAC-019
  flow_ref: F-006
  given: operator が drawer の Tree タブを開いており、workspace 内のあるディレクトリを展開済みである
  when: エージェントがそのディレクトリ内に新規ファイルを作成する
  then: tree はその新規ファイルを、drawer/tree UI 内に visible な refresh affordance (autoupdate
    表示 / drawer 内 reload control / tree ヘッダー refresh ボタンのいずれか) を通じて観察できる形で一覧に反映する。browser
    全体リロード (F5) 以外に refresh 経路が存在しない実装は本 Then を満たさない
- id: UAC-020
  flow_ref: F-007
  given: viewer にフォーカスがあり、カーソルがファイル先頭付近に表示されている
  when: operator が `G` を押す
  then: ビューポートがファイル末尾までスクロールし、カーソルがファイル末尾の行に表示される。ファイルの文字内容は押す前と一致している
- id: UAC-021
  flow_ref: F-007
  given: viewer にフォーカスがあり、対象ファイル中に検索語に一致する行が複数存在する
  when: operator が `/`, 検索語, Enter, 続けて `n` を押す。その後 `i` を押す
  then: 最初の `n` 入力までで最初の一致行にカーソル/ハイライトが移動し、以降の `n` で次の一致行へ移動する。`i` を押した後も画面上に編集用カーソルや入力モード表示は現れず、ファイル内容は変化しない
- id: UAC-022
  flow_ref: F-007
  given: viewer にフォーカスがあり、対象ファイル (5 行以上の内容) を表示している。ファイル内容は既知の文字列である
  when: operator が `dd` を押す
  then: viewer 上の行はどれも削除・空行化されず表示前と同一の内容が表示される。workspace 上の対象ファイル (fs 実体) の内容も変化しない
- id: UAC-023
  flow_ref: F-007
  given: viewer にフォーカスがあり、対象ファイルを表示している
  when: operator が `:w` を押す
  then: viewer 表示・workspace 上のファイル内容のいずれも変化せず、activity rail 上に write 由来の新規 event row
    も現れない (保存 API が呼ばれない)
- id: UAC-024
  flow_ref: F-008
  given: エージェントが設計閾値 (design で確定) を超える大きさのテキストファイルを作成し、rail にその row が表示されている
  when: operator がその row を選択して viewer を開き、末尾までスクロールする
  then: スクロールに応じてファイル末尾の実際の内容が表示され、『…(truncated)』のような省略表示は一切現れない
- id: UAC-025
  flow_ref: F-008
  given: エージェントが `.png` ファイルを作成し、rail にその row が表示されている
  when: operator がその row を選択して viewer を開く
  then: viewer にはファイル名・サイズ・MIME 種別のみが表示され、画像や生バイト列としての本文レンダリングは行われない
states:
- state-idle
- state-live-activity
- state-drawer-closed
- state-drawer-viewer-open
- state-drawer-tree-open
- state-stale-background-in-drawer
- state-diff-base-git-head-normal
- state-diff-base-degraded-non-git
edge_cases:
- turn 内で同一ファイルへの連続編集は 1 row に集約され、turn 境界をまたぐ編集は別 row になる → row 選択で個別 event に drill
  down できる (DP-ACTIVITY-AGGREGATION=OPT-AGG-TURN)
- workspace root 外 (symlink 経由) のファイルへの tool call → root 外パスは tree/rail から非到達に倒す (具体的な
  symlink 境界検証は design 段階)
- エージェントがファイルを削除した後、同一 turn 内または後続 turn で同じ path が再作成された場合の row 表現 → delete row と
  create row が別々に現れ、重ね書きはしない
- エージェントが同一 turn 内で複数の異なるファイルを操作した → path 単位で row が分かれる (1 turn = 1 row に束ねない)
- workspace directory 自体がセッション途中で削除/リネームされる → tree/drawer は劣化を明示表示し、silent success
  を装わない
- 拡張子は text 系だが実体が非UTF-8/binary なファイル → content-based 判定が拡張子と矛盾する場合は binary 扱い (metadata
  placeholder) を優先する
- 数千ファイル規模の深い/広いディレクトリツリーでの描画性能 → tree の遅延展開が design 段階の性能要件として必要
- 同時に複数セッション (複数 workspace root) を operator が開いている → row は現在アクティブな session の rail
  にのみ現れ、session 境界を跨がない
- 5MB のテキストファイルを仮想スクロールで末尾までスクロールした際の末尾到達検出 → 末尾内容が確実に表示され truncate 表示が現れない
- 単一 turn 内で同一ファイルが read → edit → read された場合の集約 row の event kind badge → 最終的な kind
  (edit) を代表として示すか複合表示するかは design 段階で確定 (aggregation の drill-down は個別 event 表示で担保)
- drawer が Tree タブを開いている最中に、背後の activity rail に新 row が追加された → Tree タブ表示を維持したまま row
  追加が visible に反映される (exp-live-background)
- 5MB の仮想化済みファイルで vim `/search/` を実行した場合、未レンダリング領域内のマッチも検索対象に含まれるか → 検索対象は仮想化領域内も含み、マッチ位置へビューポート移動できる
  (含まれない実装は design 段階で explicit fallback を宣言する)
assumptions:
- DP-EVENT-SOURCE=OPT-TOOL-CALL-LOG 確定。tool call ログのみを event source とし read event
  も含めて可視化する。tool call 外の fs 変更は本 UX の scope 外として捕捉しない。
- 1 session = 1 workspace directory を前提とする。multi-root は scope 外のまま (DP-TREE-ROOT-BOUNDARY=OPT-ROOT-WORKSPACE-ONLY
  確定により tree root は workspace 直下に固定)。
- DP-DIFF-BASE=OPT-BASE-GIT-HEAD 確定。非 git workspace では diff 機能が state-diff-base-degraded-non-git
  として明示的に劣化する。非 git workspace の実際の分布は design 段階で検証する (open_questions 参照)。
reference_ux:
- name: GitHub PR 'Files changed' タブ
  stance: modeled_on
  aspects:
  - 変更されたファイルの一覧をイベント起点で集約表示する
  - 各項目から diff / full file へワンクリックで展開する
  - event kind (added / modified / renamed / deleted) ごとに表示物を差し替える
- name: Slack thread / Linear activity log
  stance: modeled_on
  aspects:
  - 時系列 row として event を表示する
  - row 選択で detail が overlay として展開される
  - event が発生した文脈 (terminal) を離れずに参照できる
- name: 既存 SessionDrawer.tsx の off-canvas drawer パターン
  stance: modeled_on
  aspects:
  - overlay 表示中に背後コンテンツを inert 化する既存 accessibility guard
  - selection close / cancel close の 2 経路を明確に分離する設計
- name: vim 風 read-only navigation (motion キー / search)
  stance: modeled_on
  aspects:
  - 移動・検索 motion のみを採用する
  - 書き込み系コマンドは対象外にする
- name: VS Code 常設 explorer + editor split view
  stance: rejected
  aspects:
  - 常設 chrome を primary にしない
- name: vim modal editing (insert mode / :w)
  stance: rejected
  aspects:
  - text mutation
  - 書き込みコマンド
- name: agent-grid 既存 DriverShortcutBar
  stance: rejected
  aspects:
  - agent プロセス自身の入力補助であり本 viewer とは別レイヤ
tags:
- workspace-viewer
- read-only
- specify
owners: []
relations:
- {type: implementedBy, target: spec-20260714-agent-workspace-viewer}
source_paths:
- src/client/web/src/components/AppShell.tsx
- src/client/web/src/components/MainTabs.tsx
- src/client/web/src/components/SessionDrawer.tsx
- src/client/web/src/components/LogTabs.tsx
- src/client/web/src/components/TerminalPane.tsx
- src/client/web/src/components/CommandSearchTrigger.tsx
- src/client/web/src/components/DriverShortcutBar.tsx
---

> **Phase**: specify — revision r4 (base r3)。all 14 DPs closed (13 answered / 1 rejected)。primary alternative は ALT-ACTIVITY-STREAM (user-confirmed)。ATDD-ready acceptance scenarios は全 25 件、いずれも counterexample を併記。
>
> **Canonical plan**: `/home/dev/.requirements/agent-workspace-viewer/artifacts/plan.json` (revision r4)

## Goal

agent-grid web UI 上で、operator がエージェント (Codex / Claude 系) セッションの workspace を **read-only** で追跡・確認できる:

- (a) エージェントが今どのファイルに触れているかを turn-aggregated activity row から把握する
- (b) そのファイルの内容または diff を live-background の modal drawer でその場で開く
- (c) ファイル種別 (Markdown / Mermaid / JSON / source) に応じて構造化レンダリングされた形で読む
- (d) activity と無関係に workspace 全体 (workspace root 境界内) を drawer の Tree タブから能動的に閲覧する

書き込み経路は構造的に存在しない (`cc-no-write`)。sensitive file (.env 等) はフィルタなしでそのまま表示する (DP-SENSITIVE-FILE-EXPOSURE=OPT-NO-FILTER)。

## Confirmed Constraints

| ID | 制約 |
|---|---|
| cc-no-write | viewer は書き込み/保存 API を提供しない。fs アクセス経路そのものが read-only であること (UI の 'view mode' toggle だけで見せかける実装は不可)。 |
| cc-workspace-scope | 対象は『エージェントが作業しているワークスペース』のみ。tree root は workspace ディレクトリに固定 (DP-TREE-ROOT-BOUNDARY=OPT-ROOT-WORKSPACE-ONLY 確定)。 |
| cc-appshell-preserve | 既存 AppShell の named-grid-area 契約と MainTabs の exclusive-tab パラダイムを破壊的に変更しない。ALT-ACTIVITY-STREAM は既存 terminal/SessionDrawer/LogTabs 資産に統合する。 |

## User Goals

| ID | Actor | Desired outcome | Success observation |
|---|---|---|---|
| goal-track-touched-files | session operator | 今エージェントがどのファイルに触れているかをリアルタイムで把握する | 独立 activity rail 上に、turn 単位で集約された file 操作 row が現れ、workspace-relative path が読み取れる |
| goal-open-touched-file | session operator | そのファイルの内容または差分を、workspace を変更せずにその場で確認する | row を選択すると modal drawer が live-background で開き、event kind (read/create/edit/delete) に応じて viewer / diff / metadata placeholder のいずれかが必ず現れる (空 pane にはならない) |
| goal-render-structured-content | session operator | 生テキストの構文ではなく、意味のある構造としてレンダリングされた形で読める | Markdown は見出し/リスト/コードブロックが整形表示され、Mermaid は図として描画され、JSON はキーを折りたたみ/展開できるツリーとして見える |
| goal-review-diff | session operator | 変更前後の差分を git HEAD を base として確認する | diff 表示で追加行・削除行・変更行が視覚的に区別され、変更されていないファイルは rail に row として出現しない |
| goal-browse-full-workspace | session operator | session に紐づく workspace 全体のファイルツリーから、エージェントがまだ触れていないファイルを含め任意のファイルへ到達する | drawer 内の Tree タブでディレクトリを展開でき、activity に紐付いていないファイルも選択でき、選択後は goal-open-touched-file と同じ viewer に到達する |
| goal-environment-sanity-check | session operator | workspace の構造が想定通りか (期待するディレクトリ/ファイルが存在するか) を素早く確認できる | tree の root が workspace ディレクトリに固定され、実際の filesystem 状態を反映し、自動更新または明示的 refresh で最新状態に追随する |
| goal-navigate-file-with-keyboard | session operator (power user) | マウスを使わず vim ライクな motion (検索・行ジャンプ・スクロール) でファイル内を移動する。編集はしない | j/k/gg/G/ (search) /n/N によりカーソル/ビューポートが移動し、i や :w のような mutation 系キーはファイル内容に一切影響しない |

## Experience Dimensions

| ID | 観察対象値 (values) | 由来 (rationale) |
|---|---|---|
| dim-activity-visibility | timeline-row-per-turn-aggregated-touch / workspace-relative-path-label / read-create-edit-delete-kind-indicator-on-row | DP-ACTIVITY-PRIMARY-PRIMITIVE=OPT-PRIM-ROW / DP-ACTIVITY-AGGREGATION=OPT-AGG-TURN / DP-PATH-DISPLAY-SCOPE=OPT-PATH-REL の確定帰結を 1 軸に固定する。 |
| dim-viewer-open | row-select-opens-modal-drawer / background-terminal-remains-live-while-drawer-open / event-kind-switched-drawer-content | DP-DRAWER-TERMINAL-CONFLICT=OPT-MODAL-LIVE-BACKGROUND / DP-CHIP-EVENT-KIND-TO-VIEW=OPT-MAP-KIND-SWITCHED の確定帰結。 |
| dim-structured-render | markdown-headings-lists-code-fences-styled / mermaid-rendered-as-svg-diagram / json-collapsible-tree / other-text-monospace | goal-render-structured-content の success_observation を観察可能な描画物として固定する。 |
| dim-diff-review | edit-shows-added-deleted-changed-lines-vs-git-head / no-change-files-absent-from-rail | DP-DIFF-BASE=OPT-BASE-GIT-HEAD の確定帰結。 |
| dim-workspace-browse | tree-tab-inside-drawer-reachable-without-activity-row / tree-selection-opens-same-viewer-as-activity-path | DP-SECONDARY-TREE-ACCESS=OPT-DRAWER-TAB の確定帰結。 |
| dim-large-and-binary | large-text-virtualized-full-scroll-no-truncation-banner / binary-shows-name-size-mime-placeholder-no-render-attempt | DP-LARGE-BINARY-FILE=OPT-VIRTUALIZE-TEXT の確定帰結 (threshold 数値は design 段階 unmeasured のまま)。 |
| dim-vim-nav | motion-keys-move-cursor-viewport / search-slash-n-capital-n-jumps-to-match / mutation-keys-have-no-effect-file-content-unchanged | DP-VIM-NAV-DEPTH=OPT-VIM-MOTION-SEARCH の確定帰結。 |
| dim-drawer-terminal-conflict | background-agent-progresses-visibly-while-drawer-open / stale-file-surfaced-via-banner-or-reload-affordance / no-silent-stale | DP-DRAWER-TERMINAL-CONFLICT=OPT-MODAL-LIVE-BACKGROUND の recovery observation 要件 (silent stale 禁止)。 |

## Primary Flows

### F-001: エージェントが触れたファイルを activity rail で追跡する

**User goal**: goal-track-touched-files

**Entry observation**: session 画面内に独立 activity rail が表示されており、エージェントがまだ file 操作を行っていない turn では rail に row が無い

**Exit observation**: tool call 完了後、rail に workspace-relative path を含む row が turn 単位で現れ、event kind が視覚的に読み取れる

**Steps**:

- [visual] operator は terminal と並んで表示される独立 activity rail を確認できる
- [agent-driven] エージェントが tool call (read/create/edit/delete) を実行する
- [visual] tool call を含む turn が完了すると、rail に対象ファイルの workspace-relative path を含む row が追加される
- [visual] 同一 turn 内で同一ファイルへの複数 tool call は 1 row に集約され、件数 badge が付く

### F-002: activity row から event kind に応じた viewer/diff/placeholder を開く

**User goal**: goal-open-touched-file

**Entry observation**: activity rail 上に file 操作 row が表示されている

**Exit observation**: drawer 内に event kind に応じた content (viewer / diff / metadata placeholder) が表示され、空 pane にはならない

**Steps**:

- [pointer|keyboard] operator は rail 上の row を click するか row にフォーカスして Enter を押す
- [visual] modal drawer が開き、背後の terminal / activity rail は視覚的に更新され続ける (live background)
- [visual] drawer の content 領域に、event kind に対応する viewer / diff / metadata placeholder のいずれかが表示される
- [pointer|keyboard] operator は Esc または scrim click で drawer を閉じ、背後 terminal へ focus が戻る

### F-003: 構造化コンテンツ (Markdown / Mermaid / JSON) を整形表示で読む

**User goal**: goal-render-structured-content

**Entry observation**: operator が Markdown / Mermaid / JSON いずれかのファイルの row から drawer を開いた直後、viewer 領域に該当ファイル種別に応じた構造化表示が現れる

**Exit observation**: viewer に生テキストの等幅表示ではなく、種別ごとに整形された構造 (見出し/リスト/コードブロック、図、折りたたみツリー) が表示されている

**Steps**:

- [pointer|keyboard] operator が Markdown ファイルの row を選択する
- [visual] viewer に見出し・リスト・code fence が整形表示される
- [pointer|keyboard] operator が Mermaid ブロックを含むファイルまたは .mmd ファイルの row を選択する
- [visual] viewer に Mermaid 図が SVG として描画される
- [pointer|keyboard] operator が .json ファイルの row を選択する
- [visual] viewer に JSON が折りたたみ/展開可能なツリーとして表示される

### F-004: edit event の diff を git HEAD 基準でレビューする

**User goal**: goal-review-diff

**Entry observation**: rail に edit event の row が表示されている

**Exit observation**: diff view に追加/削除/変更行が視覚区別され、変更されていないファイルは rail/diff 一覧に出現しない

**Steps**:

- [pointer|keyboard] operator が edit event row から drawer を開く
- [visual] diff view が git HEAD 時点の内容と現在の内容を比較して表示される
- [visual] 変更されていないファイルは rail に row として現れない

### F-005: drawer の Tree タブから workspace 全体を能動的に閲覧する

**User goal**: goal-browse-full-workspace

**Entry observation**: operator は activity rail 上の visible な Workspace エントリ affordance (row が 0 件でも到達可能) を観察でき、そこから drawer を開くと Tree タブが初期選択されて workspace root 直下の一覧が表示される

**Exit observation**: tree 上で任意のファイルを選択すると、goal-open-touched-file と同一の viewer が開く

**Steps**:

- [visual] activity rail 上部に Workspace ラベルを持つ secondary-tree-access affordance が visible に配置されている
- [pointer|keyboard] operator は当該 affordance を pointer または keyboard で activate し drawer を開く
- [visual] drawer は Tree タブが initial focus として選択された状態で開き、workspace root 直下のディレクトリ/ファイル一覧が表示される
- [pointer|keyboard] operator がまだ activity に現れていない任意のファイルを選択する
- [visual] 選択したファイルが viewer で開かれる (goal-open-touched-file の viewer と同一の表示物)

### F-006: tree で workspace 構造の環境健全性を確認する

**User goal**: goal-environment-sanity-check

**Entry observation**: operator が drawer の Tree タブを開いた直後、tree の root ラベルが workspace ディレクトリ名を示している

**Exit observation**: tree が実際の filesystem 状態を反映しており、workspace 外のディレクトリへは辿れない

**Steps**:

- [pointer|keyboard] operator が drawer の Tree タブを開く
- [visual] tree の root が workspace ディレクトリに固定され、それより上位の階層へ navigate する手段が存在しない
- [visual] workspace 内でファイルが新規作成/削除されると、tree はその変化を反映する — 変化の反映は drawer/tree UI 内に visible な refresh affordance (autoupdate 表示 / drawer 内 reload control / tree ヘッダー refresh ボタン等) を通じて観察できる。browser 全体リロード (F5) にのみ依存する path は refresh affordance として扱わない

### F-007: viewer 内を vim ライクな read-only motion + search で移動する

**User goal**: goal-navigate-file-with-keyboard

**Entry observation**: viewer にフォーカスがある状態で、カーソル/ビューポートの初期位置がファイル先頭に表示されている

**Exit observation**: motion/search キー操作後もファイル内容自体は変更されておらず、カーソル/ビューポート位置のみが変化している

**Steps**:

- [keyboard] operator が viewer 内をクリックまたはタブ移動して viewer にフォーカスを移す
- [keyboard] operator が j/k/gg/G を押してカーソル/ビューポートを移動する
- [keyboard] operator が / を押して検索語を入力し n/N でマッチ間を移動する
- [keyboard] operator が i や :w のような mutation 系キーを押しても viewer は編集モードに入らない

### F-008: 巨大テキストファイルと binary ファイルを開く

**User goal**: goal-open-touched-file, goal-render-structured-content

**Entry observation**: rail に大きなテキストファイルまたは binary ファイルへの操作 row が表示されている

**Exit observation**: 大きなテキストは全内容にスクロール到達可能であり、binary はメタデータのみが表示され本文レンダリングは試みられない

**Steps**:

- [pointer|keyboard] operator が 5MB 超のテキストファイルの row を選択する
- [visual] viewer が仮想スクロールでファイル全体を表示可能にする (省略・truncate 表示なし)
- [pointer|keyboard] operator が binary ファイル (例: .png) の row を選択する
- [visual] viewer はファイル名・サイズ・MIME 種別のみを示す placeholder を表示する

## Acceptance Scenarios

### UAC-001: エージェントが触れたファイルを activity rail で追跡する

**Flow**: F-001

**Given**:

- session の activity rail が表示されており、row が 0 件である

**When**:

- エージェントが `src/foo.ts` に対して read tool call を実行し、その turn が完了する

**Then**:

- rail に 1 件の row が追加され、row のパス表示が `src/foo.ts` (workspace root からの相対パス) であり、host や container の絶対パスは表示されない

**Counterexample**: path をサーバから返却された host 絶対パス (例: `/home/dev/dev/agent-grid/src/foo.ts`) のまま row に表示する実装は、この Then の『workspace 相対パス』を満たさず fail する。

### UAC-002: エージェントが触れたファイルを activity rail で追跡する

**Flow**: F-001

**Given**:

- activity rail に row が 0 件、エージェントの現在 turn が開始した直後である

**When**:

- 同一 turn 内でエージェントが `src/foo.ts` を read → edit → read の順に 3 回操作し、turn が完了する

**Then**:

- rail には `src/foo.ts` に対する row が 1 件だけ表示され、その row に操作件数 (3) を示す badge が付き、row を選択すると 3 件の個別 event が展開表示される

**Counterexample**: 1 event = 1 row として `src/foo.ts` の row が 3 件 rail に並ぶ実装は、この Then の『1 件だけ表示』を満たさず fail する。

### UAC-003: activity row から event kind に応じた viewer/diff/placeholder を開く

**Flow**: F-002

**Given**:

- rail に read event の row が表示されている

**When**:

- operator がその row を選択する

**Then**:

- drawer が開き、対象ファイルの全内容が読み取り専用の viewer として表示される (diff や metadata placeholder ではない)

**Counterexample**: read event でも diff タブを既定表示にする、または path のみ表示して本文が空 pane のままになる実装は、この Then の『全内容が viewer 表示』を満たさず fail する。

### UAC-004: activity row から event kind に応じた viewer/diff/placeholder を開く

**Flow**: F-002

**Given**:

- rail に create event (新規ファイル) の row が表示されている

**When**:

- operator がその row を選択する

**Then**:

- drawer が開き、新規作成されたファイルの全内容が viewer として表示される

**Counterexample**: create event を『全行 add の疑似 diff』として表示する実装は、この Then の『viewer として表示』(diff ではない) を満たさず fail する。

### UAC-005: activity row から event kind に応じた viewer/diff/placeholder を開く

**Flow**: F-002

**Given**:

- rail に edit event の row が表示されている

**When**:

- operator がその row を選択する

**Then**:

- drawer が開き、diff view が既定タブとして表示され、追加行・削除行・変更行が視覚的に区別される。viewer は 2nd タブとして切替可能である

**Counterexample**: edit event でも viewer タブを既定表示にし diff が 2nd タブに退く実装は、この Then の『diff view が既定タブ』を満たさず fail する。

### UAC-006: activity row から event kind に応じた viewer/diff/placeholder を開く

**Flow**: F-002

**Given**:

- rail に delete event の row が表示されている

**When**:

- operator がその row を選択する

**Then**:

- drawer が開き、削除前のファイル名・サイズ・種別を示す metadata placeholder が表示され、本文 viewer や diff は表示されない

**Counterexample**: delete event で削除前ファイルの全文を viewer 表示する実装は、この Then の『metadata placeholder のみ』を満たさず fail する。

### UAC-007: activity row から event kind に応じた viewer/diff/placeholder を開く

**Flow**: F-002

**Given**:

- operator が `src/foo.ts` の edit event row から drawer を開いており、viewer/diff に内容が表示されている

**When**:

- drawer が開いたまま、背後でエージェントが同じ `src/foo.ts` を再度編集する tool call を実行する

**Then**:

- drawer 内に stale であることを示すバナー (または reload affordance) が視覚的に表示され、screen reader も stale 状態をアナウンスする。表示中の内容が無警告のまま古い内容を保持し続けることはない

**Assistive technology observation**:

- screen reader が stale 状態をアナウンスする

**Counterexample**: 背後変更後も drawer の内容とバナー表示が変化せず、無警告で古い内容を表示し続ける実装 (silent stale) は、この Then の『stale バナー表示 + AT announce』を満たさず fail する。

### UAC-008: activity row から event kind に応じた viewer/diff/placeholder を開く

**Flow**: F-002

**Given**:

- workspace 内に `.env` ファイルが存在し、rail にそのファイルへの read event row が表示されている

**When**:

- operator がその row を選択して drawer を開く

**Then**:

- `.env` の内容がそのまま viewer に表示され、値のマスキングや reveal 操作の要求は一切発生しない

**Counterexample**: .env の値を `***` 等でマスクし、明示的な reveal 操作を要求する実装は、この Then の『そのまま表示・reveal 不要』を満たさず fail する。DP-SENSITIVE-FILE-EXPOSURE=OPT-NO-FILTER の帰結として、この誤実装は本 UX 契約に存在しない sensitive-blocked state を勝手に導入したことになり fail する。

### UAC-009: activity row から event kind に応じた viewer/diff/placeholder を開く

**Flow**: F-002

**Given**:

- operator が `src/foo.ts` の edit event row から drawer を開いて viewer/diff を表示しており、activity rail には他ファイルの行はまだ現れていない

**When**:

- drawer を開いたまま、背後でエージェントが別ファイル `src/bar.ts` への write tool call を実行し、その turn が完了する

**Then**:

- drawer は表示を維持しつつ、背後の activity rail に `src/bar.ts` の新規 row が可視化される。row は drawer を閉じるまで pending queue に溜め込まれず、drawer 表示中に現れる。terminal 領域の直近出力も止まらず更新され続ける

**Counterexample**: drawer が開いている間、背後 activity rail への row 追加を drawer を閉じるまで pending queue に溜める / terminal 出力を凍結する実装は、この Then の『drawer 表示中に新 row が可視化される』を満たさず fail する。exp-live-background 違反。

### UAC-010: activity row から event kind に応じた viewer/diff/placeholder を開く

**Flow**: F-002

**Given**:

- drawer が state-stale-background-in-drawer にあり、stale であることを示すバナーと reload affordance が表示されている

**When**:

- operator が reload affordance を pointer もしくは keyboard で activate する

**Then**:

- stale バナーが drawer から消え、viewer/diff 領域には最新のファイル内容 (背後で行われた変更を反映したもの) が表示される

**Counterexample**: reload affordance を activate しても banner が消えず、viewer/diff の表示内容も更新されない実装 (dead affordance) は、この Then の『banner が消え、最新内容が表示される』を満たさず fail する。exp-live-background の recovery path 違反。

### UAC-011: 構造化コンテンツ (Markdown / Mermaid / JSON) を整形表示で読む

**Flow**: F-003

**Given**:

- rail に `.md` ファイルの row が表示されている

**When**:

- operator がその row を選択する

**Then**:

- viewer に見出し・リスト・code fence が視覚的に整形表示される (見出しはテキストサイズ/太さが変化し、リストは箇条書き記号を伴う)

**Counterexample**: Markdown の生テキストをそのまま等幅 code block としてのみ表示する実装は、この Then の『見出し・リストが整形表示される』を満たさず fail する。

### UAC-012: 構造化コンテンツ (Markdown / Mermaid / JSON) を整形表示で読む

**Flow**: F-003

**Given**:

- rail に Mermaid 記法を含むファイルの row が表示されている

**When**:

- operator がその row を選択する

**Then**:

- viewer に Mermaid 図が SVG 図形として描画される

**Counterexample**: Mermaid のソーステキストをそのまま code block 表示するだけで図を描画しない実装は、この Then の『SVG 図形として描画』を満たさず fail する。

### UAC-013: 構造化コンテンツ (Markdown / Mermaid / JSON) を整形表示で読む

**Flow**: F-003

**Given**:

- rail に `.json` ファイルの row が表示されている

**When**:

- operator がその row を選択する

**Then**:

- viewer に JSON がキー単位で折りたたみ/展開可能なツリー構造として表示される

**Counterexample**: JSON を生テキストのまま等幅表示し折りたたみ操作が存在しない実装は、この Then の『折りたたみ可能なツリー』を満たさず fail する。

### UAC-014: edit event の diff を git HEAD 基準でレビューする

**Flow**: F-004

**Given**:

- 対象ファイルは現在の turn 開始前 (git HEAD 時点) から既に変更されている状態で、今回の turn ではさらに追加の変更が加えられた

**When**:

- operator が edit event row を選択して diff view を開く

**Then**:

- diff view には git HEAD 時点からの全ての変更行 (今回 turn 以前の変更を含む) が追加/削除として表示される

**Counterexample**: diff base を turn 開始時点にする実装は、今回 turn 開始前に既に加えられていた変更行を diff に表示できず、この Then の『git HEAD 時点からの全ての変更行』を満たさず fail する。

### UAC-015: edit event の diff を git HEAD 基準でレビューする

**Flow**: F-004

**Given**:

- workspace 内に、今回のセッションでエージェントが一切触れていないファイルが存在する

**When**:

- operator が activity rail を確認する

**Then**:

- そのファイルに対応する row は rail に一切表示されない

**Counterexample**: workspace 内の全ファイルを『変更なし』として rail に row 表示する実装は、この Then の『row は一切表示されない』を満たさず fail する。

### UAC-016: edit event の diff を git HEAD 基準でレビューする

**Flow**: F-004

**Given**:

- workspace が git 管理下にない (workspace ルート下に .git が存在しない) 状態で、rail に edit event の row が表示されている

**When**:

- operator がその row を選択して drawer を開く

**Then**:

- drawer 内 diff 位置に、diff base が git HEAD で解決できないことを示す visible な劣化表示 (banner / 無効化 indicator / 明示的説明文言のいずれか) が現れる。通常の diff タブが無警告で表示されることは無い

**Counterexample**: 非 git workspace でも通常の diff タブを何の劣化表示もなく表示する / diff タブを silently 非表示にして viewer タブのみ表示する実装は、この Then の『visible な劣化表示が現れ、silent success にならない』を満たさず fail する。exp-git-head-diff-base 違反。

### UAC-017: drawer の Tree タブから workspace 全体を能動的に閲覧する

**Flow**: F-005

**Given**:

- activity rail に row が 1 件も無い状態で、activity rail 上に Workspace ラベル付きの secondary-tree-access affordance が visible に表示されている

**When**:

- operator が Workspace affordance を pointer もしくは keyboard で activate する (row を経由しない)

**Then**:

- drawer が開き、Tree タブが initial focus を持ち workspace root 直下の一覧が表示される。その後 tree 上で任意のファイルを選択すると event kind に基づく row 選択と同じ viewer が現れる

**Counterexample**: activity rail が row が 0 件のときに Workspace affordance を非表示にする実装、または drawer が特定の row 選択後にしか開かない実装は、この Given/Then の『row が 0 件でも Workspace affordance が visible で、それを activate すると Tree タブ initial focus の drawer が開く』を満たさず fail する。

### UAC-018: tree で workspace 構造の環境健全性を確認する

**Flow**: F-006

**Given**:

- operator が drawer の Tree タブを開いている

**When**:

- operator が tree の root ノードを確認する

**Then**:

- root ノードは workspace ディレクトリのみを示し、親ディレクトリやそのきょうだいディレクトリへ辿るための展開/リンクが tree 上に存在しない

**Counterexample**: tree の root を container `/` にし、workspace の親ディレクトリやきょうだいディレクトリへ展開可能にする実装は、この Then の『上位/きょうだいへ辿る手段が存在しない』を満たさず fail する。

### UAC-019: tree で workspace 構造の環境健全性を確認する

**Flow**: F-006

**Given**:

- operator が drawer の Tree タブを開いており、workspace 内のあるディレクトリを展開済みである

**When**:

- エージェントがそのディレクトリ内に新規ファイルを作成する

**Then**:

- tree はその新規ファイルを、drawer/tree UI 内に visible な refresh affordance (autoupdate 表示 / drawer 内 reload control / tree ヘッダー refresh ボタンのいずれか) を通じて観察できる形で一覧に反映する。browser 全体リロード (F5) 以外に refresh 経路が存在しない実装は本 Then を満たさない

**Counterexample**: tree はエージェントの新規ファイル作成を能動的に検知せず autoupdate も実装せず、drawer/tree UI 内に reload ボタンや pull-to-refresh も存在しない — operator が唯一使える refresh 手段が browser F5 だけ、という実装は、この Then の『drawer/tree UI 内に visible な refresh affordance』を満たさず fail する。

### UAC-020: viewer 内を vim ライクな read-only motion + search で移動する

**Flow**: F-007

**Given**:

- viewer にフォーカスがあり、カーソルがファイル先頭付近に表示されている

**When**:

- operator が `G` を押す

**Then**:

- ビューポートがファイル末尾までスクロールし、カーソルがファイル末尾の行に表示される。ファイルの文字内容は押す前と一致している

**Counterexample**: `G` 押下時にビューポートが末尾へ移動せずファイル先頭のまま留まる実装 (motion 表面の破壊) は、この Then の『ビューポートがファイル末尾までスクロールし、カーソルがファイル末尾の行に表示される』を満たさず fail する。

### UAC-021: viewer 内を vim ライクな read-only motion + search で移動する

**Flow**: F-007

**Given**:

- viewer にフォーカスがあり、対象ファイル中に検索語に一致する行が複数存在する

**When**:

- operator が `/`, 検索語, Enter, 続けて `n` を押す。その後 `i` を押す

**Then**:

- 最初の `n` 入力までで最初の一致行にカーソル/ハイライトが移動し、以降の `n` で次の一致行へ移動する。`i` を押した後も画面上に編集用カーソルや入力モード表示は現れず、ファイル内容は変化しない

**Counterexample**: `i` 押下後にテキスト入力可能なカーソル (点滅キャレット) を表示し文字入力を受け付ける実装は、この Then の『編集用カーソルや入力モード表示は現れない』を満たさず fail する。

### UAC-022: viewer 内を vim ライクな read-only motion + search で移動する

**Flow**: F-007

**Given**:

- viewer にフォーカスがあり、対象ファイル (5 行以上の内容) を表示している。ファイル内容は既知の文字列である

**When**:

- operator が `dd` を押す

**Then**:

- viewer 上の行はどれも削除・空行化されず表示前と同一の内容が表示される。workspace 上の対象ファイル (fs 実体) の内容も変化しない

**Counterexample**: `dd` 押下でカーソル行を viewer 表示から削除する / workspace 上のファイルから 1 行削除する実装は、この Then の『どの行も削除・空行化されず / workspace 上の内容も変化しない』を満たさず fail する。exp-vim-no-mutation 違反。

### UAC-023: viewer 内を vim ライクな read-only motion + search で移動する

**Flow**: F-007

**Given**:

- viewer にフォーカスがあり、対象ファイルを表示している

**When**:

- operator が `:w` を押す

**Then**:

- viewer 表示・workspace 上のファイル内容のいずれも変化せず、activity rail 上に write 由来の新規 event row も現れない (保存 API が呼ばれない)

**Counterexample**: `:w` 押下で保存 API を呼び、workspace 上のファイルを上書き / rail 上に write 由来 row を追加する実装は、この Then の『write 由来 event row も現れない』を満たさず fail する。exp-vim-no-mutation 違反。

### UAC-024: 巨大テキストファイルと binary ファイルを開く

**Flow**: F-008

**Given**:

- エージェントが設計閾値 (design で確定) を超える大きさのテキストファイルを作成し、rail にその row が表示されている

**When**:

- operator がその row を選択して viewer を開き、末尾までスクロールする

**Then**:

- スクロールに応じてファイル末尾の実際の内容が表示され、『…(truncated)』のような省略表示は一切現れない

**Counterexample**: 一定行数を超えた箇所で『…(truncated)』を表示し以降の内容をレンダリングしない実装は、この Then の『省略表示は一切現れない』を満たさず fail する。

### UAC-025: 巨大テキストファイルと binary ファイルを開く

**Flow**: F-008

**Given**:

- エージェントが `.png` ファイルを作成し、rail にその row が表示されている

**When**:

- operator がその row を選択して viewer を開く

**Then**:

- viewer にはファイル名・サイズ・MIME 種別のみが表示され、画像や生バイト列としての本文レンダリングは行われない

**Counterexample**: binary ファイルの生バイト列をテキストとして (文字化けした状態で) そのまま表示する実装は、この Then の『本文レンダリングは行われない』を満たさず fail する。

## Experience Contract

| ID | Subject (契約文) | Enforced by (scenario ids) | Non-goal |
|---|---|---|---|
| exp-read-only | viewer / diff / tree はいずれの経路からも書き込み・保存 API を提供せず、fs アクセス経路自体が read-only である | UAC-003, UAC-004, UAC-006, UAC-008, UAC-011, UAC-012, UAC-013, UAC-020, UAC-022, UAC-023, UAC-021, UAC-024, UAC-025 | read-only 閲覧は連続的な状態遷移を持たない (entry と success の 2 端が十分); 書き込み経路そのものが構造的に存在しないため復旧対象なし |
| exp-live-background | drawer が開いている間も背後 terminal / activity rail はエージェントの進行を視覚的に更新し続け、drawer 内対象ファイルへの背後変更は silent stale を許容しない | UAC-009, UAC-007, UAC-010 | — |
| exp-workspace-relative-paths | activity row / drawer header / tree のいずれの path 表示も workspace root からの相対パスで統一される | UAC-001 | path 表示は一度確定すると閲覧中に遷移しない; workspace 相対 path 変換は構造的に workspace 内でのみ解決するため失敗経路を持たない; 失敗経路が無いため復旧経路も持たない |
| exp-git-head-diff-base | edit event の diff は常に git HEAD を base として表示され、非 git workspace では明示的に劣化表示する | UAC-005, UAC-016, UAC-014, UAC-015 | diff base 解決は drawer 開いた瞬間に確定し途中で遷移しない |
| exp-turn-aggregation | 同一 turn 内の同一ファイルへの複数操作は 1 row に集約され、展開操作で個別 event に drill down できる | UAC-002 | aggregation は turn 完了時に確定する 2 端観察 (entry/success) で十分; aggregation 失敗は個別 event 展開で回復するため独立 failure 状態を持たない; failure 状態を持たないため recovery 経路も持たない |
| exp-workspace-root-boundary | tree の root は session の workspace ディレクトリに固定され、上位/きょうだいディレクトリへ辿る手段を一切提供しない | UAC-017, UAC-018, UAC-019 | root 境界は構造的に閉じており、境界越え自体が発生しないため復旧対象なし |
| exp-no-sensitive-filter | viewer / diff / tree はファイル名や内容に基づく sensitive 判定・マスキング・reveal ステップを一切持たず、.env や秘密鍵を含む任意ファイルをそのまま表示する | UAC-008 | sensitive filter が存在しないため中間状態を持たない; sensitive-blocked state を UX contract に持たない; failure 状態を持たないため recovery 経路も持たない |
| exp-vim-no-mutation | vim 風キー入力は motion (j/k/gg/G) と search (/, n, N) のみを提供し、insert/write 系キーはいかなる視覚変化もファイル内容変化も生まない | UAC-020, UAC-022, UAC-023, UAC-021 | motion / search は瞬時 (単一 keypress) 完結; mutation は構造的に無視されるため復旧対象事象が発生しない |

## States

| ID | 説明 | Observable signal |
|---|---|---|
| state-idle | session 開始直後、rail が row 0 件 | activity rail が visible、row 0 件 |
| state-live-activity | エージェントの tool call を含む turn 完了後 | rail に turn-aggregated row が visible、workspace 相対 path が読み取れる |
| state-drawer-closed | drawer が閉じている | drawer overlay が非表示、terminal focus が active |
| state-drawer-viewer-open | drawer が viewer/diff/metadata を表示中 | drawer が visible、event kind に対応する content 領域が描画 |
| state-drawer-tree-open | drawer が Tree タブを表示中 | drawer が visible、Tree タブ initial focus、workspace root 直下の一覧 |
| state-stale-background-in-drawer | 背後変更で drawer 内表示が古くなった | stale banner または reload affordance が visible |
| state-diff-base-git-head-normal | workspace が git 管理下、diff は git HEAD 基準 | diff タブが initial 表示、追加 / 削除行が視覚区別 |
| state-diff-base-degraded-non-git | workspace が非 git、diff base 解決不能 | diff タブ位置に visible な劣化表示 (banner / 無効化 indicator / 説明文言) が現れる |

<!-- requirements-decisions:start -->
## Decision State
Selected alternative: ALT-ACTIVITY-STREAM
Decision DP-ACTIVITY-AGGREGATION: OPT-AGG-TURN
Decision DP-ACTIVITY-PRIMARY-PRIMITIVE: OPT-PRIM-ROW
Decision DP-CHIP-EVENT-KIND-TO-VIEW: OPT-MAP-KIND-SWITCHED
Decision DP-DIFF-BASE: OPT-BASE-GIT-HEAD
Decision DP-DRAWER-TERMINAL-CONFLICT: OPT-MODAL-LIVE-BACKGROUND
Decision DP-EVENT-SOURCE: OPT-TOOL-CALL-LOG
Decision DP-HISTORY-MODE: OPT-LIVE-ONLY
Decision DP-LARGE-BINARY-FILE: OPT-VIRTUALIZE-TEXT
Decision DP-PATH-DISPLAY-SCOPE: OPT-PATH-REL
Decision DP-SECONDARY-TREE-ACCESS: OPT-DRAWER-TAB
Decision DP-SENSITIVE-DETECTION-SOURCE: unresolved
Decision DP-SENSITIVE-FILE-EXPOSURE: OPT-NO-FILTER
Decision DP-TREE-ROOT-BOUNDARY: OPT-ROOT-WORKSPACE-ONLY
Decision DP-VIM-NAV-DEPTH: OPT-VIM-MOTION-SEARCH
<!-- requirements-decisions:end -->

_Note_: `DP-SENSITIVE-DETECTION-SOURCE` は canonical plan で `answer_status: rejected` (DP-SENSITIVE-FILE-EXPOSURE=OPT-NO-FILTER の帰結として functionally moot)。managed block 上の `unresolved` 表記は projection renderer の canonical 出力仕様 (answered/defaulted のみ selected 表示) による — 実状態は plan.json を SoT とする。

## Design Handoff

Specify で closure に至らなかった presentation / measurement / affordance detail は次の handoff として design skill へ受け渡す:

### handoff-large-file-threshold

**Required outcome**: 5MB 以上のテキストファイルが truncate されず、仮想スクロールで全内容へ到達できる

**Design obligations**:
- 実 agent workspace のファイルサイズ分布 (mean/p95/max) を N セッションから採取する
- target device tier (desktop / low-end tablet) で virtualization の応答時間をベンチする
- threshold-large-file-viewer-limit の具体値を plan.json quality_thresholds に確定する

**Verification obligations**:
- 測定済み閾値以上のファイルでスクロールして末尾まで到達し、実内容が表示され truncate バナーが出ないことを確認する

### handoff-vim-keymap-bindings

**Required outcome**: j/k/gg/G/(検索)/n/N が viewer フォーカス時のみ有効化され、mutation 系キー入力は視覚上・内容上の変化を一切生まない

**Design obligations**:
- 正確な keymap 表を定義する (押下キー → カーソル/ビューポート挙動)
- focus-scope gating を DriverShortcutBar の visibility gate パターンを参考に設計し、xterm.js の raw capture と衝突しないようにする

**Verification obligations**:
- mutation 系キー (i, o, dd, :w 等) を押しても編集モード表示・ファイル内容変化のいずれも発生しないことを確認する

### handoff-stale-banner-presentation

**Required outcome**: drawer 表示中に背後の対象ファイルが変更された場合、stale 状態が視覚的にもAT的にも silent にならず必ず可視化される

**Design obligations**:
- stale バナーまたは reload affordance の視覚表現 (色/アイコン/文言) を確定する
- AT announce 文言を確定する

**Verification obligations**:
- 背後変更後、drawer 内にバナー/reload affordance が出現し screen reader がそれをアナウンスすることを確認する

### handoff-diff-view-layout

**Required outcome**: git HEAD diff の追加/削除/変更行が視覚的に (色のみに依存せず) 区別される

**Design obligations**:
- unified / split view のどちらを既定にするか決める
- large diff の折り畳み方式を決める

**Verification obligations**:
- 追加/削除行が色以外の手がかり (記号・アイコン) でも区別可能であることを確認する (アクセシビリティ)

### handoff-structured-render-fallback

**Required outcome**: Markdown / Mermaid / JSON の構造化レンダリングは parse error 時にも空白にならず、何らかの観察可能な fallback を返す

**Design obligations**:
- Mermaid parse error 時の raw source fallback 表示を定義する
- invalid JSON 時の raw text fallback 表示を定義する

**Verification obligations**:
- 壊れた Mermaid / invalid JSON を開いた際、空白 pane ではなく fallback コンテンツが表示されることを確認する

### handoff-tree-refresh

**Required outcome**: workspace のファイル変化 (作成 / 削除) が drawer/tree UI 内に visible な refresh affordance を通じて反映される。browser 全体リロード (F5) にのみ依存する path は refresh 経路として扱わない

**Design obligations**:
- autoupdate 表示 / drawer 内 reload control / tree ヘッダーのボタン / kbd shortcut のいずれを normative とするかを決定する
- refresh 中の loading indicator と失敗時の visible な fallback を定義する

**Verification obligations**:
- 新規ファイル作成後に drawer を閉じずに refresh affordance を activate し、tree に visible に反映されることを確認する

### handoff-secondary-tree-entry

**Required outcome**: activity rail 上の Workspace ラベル付き secondary-tree-access affordance が pointer / keyboard / screen reader いずれの modality からも row 0 件時にも visible に到達できる

**Design obligations**:
- affordance の具体的 icon / label / hotkey / focus order を定義する
- screen reader label と keyboard shortcut を activity rail の focus scope と衝突しない形で定める

**Verification obligations**:
- row 0 件の状態で Workspace affordance を pointer と keyboard の両方から activate でき、drawer が Tree タブ initial focus で開くことを確認する

### Open questions for design

- large-file / binary の threshold 数値は現時点で unmeasured (threshold-large-file-viewer-limit; DP-LARGE-BINARY-FILE=OPT-VIRTUALIZE-TEXT 前提で design が測定手順に沿って確定する)。
- 非 git workspace の割合が高い場合、DP-DIFF-BASE=OPT-BASE-GIT-HEAD の劣化表示 (state-diff-base-degraded-non-git) の具体的 UI は design 段階で pa-workspace-is-git の validation 結果を待って確定する。

## Technology Candidates for Design Evaluation

user が候補として提示した library / API は UX 契約に採用として書き込まず、design 段階での比較対象として保持する。各候補の required_capabilities / disqualifiers は `plan.json` の `technology_candidates` を SoT とする。要約:

- **CodeMirror 6** — source viewer / read-only vim keymap の受け皿 (mutation command 構造的無効化必須)
- **react-markdown** — Markdown 描画 (untrusted content の HTML sanitization 必須)
- **Mermaid** — Mermaid 図描画 (parse error 時 raw source fallback 必須)
- **JSON tree viewer** — 折りたたみ tree (巨大 JSON 遅延展開必須)
- **diff viewer** — DP-DIFF-BASE=OPT-BASE-GIT-HEAD 前提の diff レンダリング (行単位視覚区別 / large diff 折り畳み)
- **Go filesystem API + 既存 client layer server** — read-only endpoint 群 (path traversal 禁止 / cc-no-write の構造保証)

design 段階では上記に限らず代替を検討してよい。上記は evaluation の出発点であり最終決定ではない。

## Critique Resolutions (summary)

Specify pass2 の 7 improvement は plan.json `critique_resolutions` で完全に trace 済み:

1. **critique-specify.pass2.improvement[0] — UAC-020 counterexample non_discriminating** — disposition: `patch_contract` — resolves: F-007, UAC-020
2. **critique-specify.pass2.improvement[1] — F-007 / DP-VIM-NAV-DEPTH coverage (mutation keys)** — disposition: `patch_contract` — resolves: F-007, UAC-022, UAC-023, exp-vim-no-mutation
3. **critique-specify.pass2.improvement[2] — F-005.entry_observation / UAC-017 hidden affordance** — disposition: `patch_contract` — resolves: F-005, UAC-017, handoff-secondary-tree-entry
4. **critique-specify.pass2.improvement[3] — exp-live-background progress observation** — disposition: `patch_contract` — resolves: F-002, UAC-009, exp-live-background
5. **critique-specify.pass2.improvement[4] — exp-git-head-diff-base failure path (non-git workspace)** — disposition: `patch_contract` — resolves: F-004, UAC-016, exp-git-head-diff-base, osc-diff-base
6. **critique-specify.pass2.improvement[5] — UAC-019 Then / F-006 step 3 refresh affordance** — disposition: `patch_contract` — resolves: F-006, UAC-019, handoff-tree-refresh
7. **critique-specify.pass2.improvement[6] — osc-drawer.transitions (stale recovery dead-end)** — disposition: `patch_contract` — resolves: osc-drawer, F-002, UAC-010


{% transition from="draft" to="approved" date="2026-07-13" %}
UX 計画合意
{% /transition %}
