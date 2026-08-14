---
change: change-20260812-cli-terminal-client
role: ux
user_goals:
- goal-find-attach-target
- goal-attach-interact-detach
- goal-interrupt-remote-not-cli
- goal-observe-liveness
- goal-reattach-after-loss
- goal-create-session-and-attach
- goal-explicit-terminate
- goal-script-discovery
- goal-disambiguate-selector
- goal-diagnose-failure-class
- goal-multi-server-qualification
- goal-concurrent-modality-continuity
flows:
- id: F-001
  goal_refs:
  - goal-find-attach-target
  - goal-script-discovery
  experience_contract_refs:
  - exp-discovery
  acceptance_scenario_refs:
  - UAC-001
  - UAC-002
  - UAC-003
  - UAC-004
  - UAC-005
  - UAC-006
  - UAC-007
  - UAC-039
  - UAC-040
  - UAC-008
  - UAC-046
- id: F-002
  goal_refs:
  - goal-attach-interact-detach
  - goal-interrupt-remote-not-cli
  - goal-observe-liveness
  - goal-concurrent-modality-continuity
  experience_contract_refs:
  - exp-attach-interactive
  - exp-concurrent-modality
  acceptance_scenario_refs:
  - UAC-009
  - UAC-010
  - UAC-011
  - UAC-012
  - UAC-013
  - UAC-041
  - UAC-014
  - UAC-015
  - UAC-016
  - UAC-017
  - UAC-042
  - UAC-018
  - UAC-019
  - UAC-020
  - UAC-021
- id: F-003
  goal_refs:
  - goal-reattach-after-loss
  experience_contract_refs:
  - exp-reattach-continuity
  acceptance_scenario_refs:
  - UAC-022
  - UAC-023
  - UAC-024
- id: F-004
  goal_refs:
  - goal-create-session-and-attach
  experience_contract_refs:
  - exp-create-and-attach
  acceptance_scenario_refs:
  - UAC-025
  - UAC-026
  - UAC-027
  - UAC-043
  - UAC-045
- id: F-005
  goal_refs:
  - goal-explicit-terminate
  experience_contract_refs:
  - exp-explicit-terminate
  acceptance_scenario_refs:
  - UAC-029
  - UAC-030
  - UAC-044
- id: F-006
  goal_refs:
  - goal-disambiguate-selector
  - goal-diagnose-failure-class
  - goal-multi-server-qualification
  experience_contract_refs:
  - exp-selector-and-failure-diagnosis
  acceptance_scenario_refs:
  - UAC-031
  - UAC-032
  - UAC-033
  - UAC-034
  - UAC-035
  - UAC-036
  - UAC-037
  - UAC-038
experience_contracts:
- id: exp-discovery
  scenario_refs:
  - UAC-001
  - UAC-002
  - UAC-003
  - UAC-004
  - UAC-005
  - UAC-006
  - UAC-007
  - UAC-039
  - UAC-040
  - UAC-008
  - UAC-046
- id: exp-attach-interactive
  scenario_refs:
  - UAC-009
  - UAC-010
  - UAC-011
  - UAC-012
  - UAC-013
  - UAC-041
  - UAC-014
  - UAC-015
  - UAC-016
  - UAC-017
  - UAC-042
  - UAC-018
  - UAC-019
- id: exp-concurrent-modality
  scenario_refs:
  - UAC-020
  - UAC-021
- id: exp-reattach-continuity
  scenario_refs:
  - UAC-022
  - UAC-023
  - UAC-024
- id: exp-create-and-attach
  scenario_refs:
  - UAC-025
  - UAC-026
  - UAC-027
  - UAC-043
  - UAC-045
- id: exp-explicit-terminate
  scenario_refs:
  - UAC-029
  - UAC-030
  - UAC-044
- id: exp-selector-and-failure-diagnosis
  scenario_refs:
  - UAC-031
  - UAC-032
  - UAC-033
  - UAC-034
  - UAC-035
  - UAC-036
  - UAC-037
  - UAC-038
decision_points:
- id: DP-ENTRY-DISPATCH-STYLE
  decision_key: entry-dispatch-style
  contract_effect: normative
  answer_status: answered
  selected_option_ref: OPT-VERB-FIRST-ONLY
- id: DP-AMBIGUITY-INTERACTIVITY
  decision_key: ambiguity-interactivity
  contract_effect: normative
  answer_status: answered
  selected_option_ref: OPT-STATIC-LIST-NONZERO
- id: DP-TTY-BEHAVIOR-BRANCHING
  decision_key: tty-behavior-branching
  contract_effect: normative
  answer_status: answered
  selected_option_ref: OPT-UNIFORM-BEHAVIOR
- id: DP-LIST-SERVER-SCOPE
  decision_key: list-server-scope
  contract_effect: normative
  answer_status: answered
  selected_option_ref: OPT-ALL-CONNECTED
- id: DP-LIST-LIFECYCLE-SCOPE
  decision_key: list-lifecycle-scope
  contract_effect: normative
  answer_status: answered
  selected_option_ref: OPT-INCLUDE-ENDED
- id: DP-SELECTOR-PRECEDENCE-ORDER
  decision_key: selector-precedence-order
  contract_effect: normative
  answer_status: answered
  selected_option_ref: OPT-EXPLICIT-MODE-FLAGS
- id: DP-SERVER-QUALIFIER-SYNTAX
  decision_key: server-qualifier-syntax
  contract_effect: normative
  answer_status: answered
  selected_option_ref: OPT-SEPARATE-FLAG
- id: DP-LS-DEFAULT-COLUMNS
  decision_key: ls-default-columns
  contract_effect: default
  answer_status: answered
  selected_option_ref: OPT-PLAN-MINIMUM-SET
- id: DP-JSON-SESSION-IDENTIFIER-SHAPE
  decision_key: json-session-identifier-shape
  contract_effect: normative
  answer_status: answered
  selected_option_ref: OPT-STRUCTURED-PAIR
- id: DP-EXIT-CODE-GRANULARITY
  decision_key: exit-code-granularity
  contract_effect: normative
  answer_status: answered
  selected_option_ref: OPT-DISTINCT-CODE-PER-CLASS
- id: DP-DETACH-DEFAULT-KEY
  decision_key: detach-default-key
  contract_effect: default
  answer_status: answered
  selected_option_ref: OPT-CTRL-BRACKET
- id: DP-ATTACH-OBSERVABILITY-CHANNEL
  decision_key: attach-observability-channel
  contract_effect: normative
  answer_status: answered
  selected_option_ref: OPT-TRANSPARENT-ATTACH
- id: DP-CONCURRENT-ATTACH-VISIBILITY
  decision_key: concurrent-attach-visibility
  contract_effect: normative
  answer_status: answered
  selected_option_ref: OPT-NO-VISIBILITY
- id: DP-DETACHING-INPUT-DISPOSITION
  decision_key: detaching-input-disposition
  contract_effect: normative
  answer_status: answered
  selected_option_ref: OPT-DETACHING-INPUT-DISCARDED
- id: DP-CREATE-TARGET-SERVER
  decision_key: create-target-server
  contract_effect: normative
  answer_status: answered
  selected_option_ref: OPT-CREATE-REQUIRE-QUALIFIER
- id: DP-SERVER-QUALIFIER-VALUE-VOCABULARY
  decision_key: server-qualifier-value-vocabulary
  contract_effect: normative
  answer_status: answered
  selected_option_ref: OPT-QUALIFIER-ACCEPTS-DISPLAY-NAME
- id: DP-UNREACHABLE-SERVER-SELECTOR-OUTCOME
  decision_key: unreachable-server-selector-outcome
  contract_effect: normative
  answer_status: answered
  selected_option_ref: OPT-UNREACHABLE-AS-TRANSPORT-FAILURE
- id: DP-JSON-SERVER-REACHABILITY-FIELD
  decision_key: json-server-reachability-field
  contract_effect: normative
  answer_status: answered
  selected_option_ref: OPT-JSON-INCLUDE-REACHABILITY
- id: DP-NON-TTY-ATTACH-BEHAVIOR
  decision_key: non-tty-attach-behavior
  contract_effect: normative
  answer_status: answered
  selected_option_ref: OPT-NON-TTY-FAIL-FAST
dispositions:
- decision_input_ref: ALT-TWO-STEP-EXPLICIT
  disposition: adopted_as_primary_flow_topology
  contract_refs:
  - F-001
  - F-002
  - F-003
  - F-004
  - F-005
  - F-006
- decision_input_ref: ALT-ADAPTIVE-SINGLE-ENTRY
  disposition: deferred_not_revived
- decision_input_ref: candidate-cross-platform-terminal-runtime
  disposition: informs_architecture_not_ux_contract
- decision_input_ref: candidate-existing-public-wire-contract-reuse
  disposition: adopted_as_constraint
  contract_refs:
  - exp-discovery
  - exp-attach-interactive
  - exp-reattach-continuity
- decision_input_ref: candidate-fleet-session-identity-reuse
  disposition: adopted_as_identifier_shape
  contract_refs:
  - UAC-008
  - UAC-032
- decision_input_ref: dh-ctrl-bracket-no-collision
  disposition: kept_provisional_pending_hardware_validation
  contract_refs:
  - UAC-014
- decision_input_ref: dh-lifecycle-contract-sufficient
  disposition: handoff_to_phase0_contract_audit
  contract_refs:
  - UAC-043
  - UAC-044
- decision_input_ref: dh-stable-name-field-unknown
  disposition: handoff_to_phase0_validation_required
  contract_refs:
  - UAC-034
  - UAC-004
- decision_input_ref: dh-geometry-divergence
  disposition: handoff_to_execution_plane
  contract_refs:
  - UAC-020
  - UAC-011
- decision_input_ref: esc-no-presence-field
  disposition: adopted_as_observation_source_limit
  contract_refs:
  - UAC-021
  - UAC-031
- decision_input_ref: esc-fleet-identity
  disposition: adopted_as_row_identity_discipline
  contract_refs:
  - UAC-001
  - UAC-031
- decision_input_ref: esc-status-vocabulary
  disposition: adopted_as_state_vocabulary
  contract_refs:
  - UAC-006
  - UAC-039
  - UAC-018
- decision_input_ref: esc-sessioninfo-fields
  disposition: adopted_as_column_existence_bound
  contract_refs:
  - UAC-004
---

<!-- lifecycle is owned by change.md -->

# Agent Grid Terminal CLI Client — UX (Specify)

mode: specify / upstream: explore-plan.json r2 (base r1) / critique: 33 issues + pass2 33 improvements / consultation: consultation-20260813-cli-terminal-client-specify

本 member は specify phase の成果である。承認済みの explore plan (revision r2) — `approved_by_user: true` の primary alternative ALT-TWO-STEP-EXPLICIT と、利用者が逐語で確定した 13 件の判断および confirmed constraints — を設計入力とし、それを観察可能な acceptance scenario へ展開したものである。pass2 が本 phase で新しい軸として起こした 6 件は、その後の user consultation (consultation-20260813-cli-terminal-client-specify) ですべて確定し、§8 に closed として記録した。確定していないもの (explore が未確定として残した項目、実機未検証の仮説、Phase 0 で測定する項目) は確定として書かず §9 に開いたまま残す。

## Goal

稼働中の Claude Code / Codex PTY を持つ既存 server-side session へ、任意のローカル terminal から tmux 的に attach / detach / reattach できる CLI を、既存 GUI client と同じ identity / lifecycle 語彙で提供する。PTY / multiplexing / snapshot / 所有権制御は新設せず既存 execution plane と public wire contract に乗る。対話利用と自動化利用の双方が、attach 先の発見・曖昧解消・作成・明示終了・失敗種別の判別と、terminal を壊さない離脱を行える。

## Target Users

- ローカル terminal から Agent Grid の Claude Code / Codex session を直接操作したい開発者。
- shell script や他エージェントから session を発見・作成・参照する自動化利用者。
- 複数 server (fleet) へ同時接続し session/server の取り違えを避けたい運用者。

---

## 0. Legacy vocabulary boundary

CLI が置き換える既存 UX は無い。ここでの legacy とは、CLI が新たに扱うのと同じ server-side session を今日すでに観察している既存 client 群 (共有 SPA `clients/ui`、Workspace Electron `clients/workspace`、Windows Shell `clients/windows-shell`) の語彙・識別子・状態表現である。

継承する挙動 (inherited):

1. どの既存 client で見ても同じ session 状態は同じ label で読める (Windows Shell も共有 SPA と同一の label 文字列)。
2. 複数の状態が同時に立つとき、前面に出る状態は既存 client 共通の優先順 (failed > human_input > active > stopped > idle) で決まる。
3. session を示す行は所属 server を常時併記する。server 抜きの ID 単独表示、display name 重複のまま区別できない表示は既存 change の must-fail 受入条件である。
4. server の呼称は利用者設定の表示名で示され、表示名が重複するときだけ stable な server 識別子が補助表示される。
5. session の同一性は所属 server と session ID の対で判定され、両者を 1 語へ連結した canonical 表記は既存 client のどこにも無い。
6. server へ届かない・互換性が無い・確認中といった接続側の状態は、session の lifecycle 状態とは別の面・別語彙で示される。
7. 既存 client は画面を閉じたり購読を解除したりしても session を終了させない。利用者は「見るのをやめる」と「終わらせる」を別操作として学習している。
8. 同じ session に別 client が同時に attach していることを示す表示は、どの既存 client にも無い。
9. session の display name は公開されている session 情報から既存 client (共有 SPA) が導出した値であり、利用者はその値で GUI の行を識別している。

CLI 側にこの観察が現れたら UX の失敗と判定する drift (既存 client の挙動を置き換えるものではなく、CLI が守るべき禁止事項):

- 同じ session 状態に CLI 独自の語 (running / live / connected 等) を出し、既存 client の label と突き合わせられなくすること。
- 複数状態が同時に立つ session を既存 client と異なる優先順で提示すること。
- session を所属 server 抜きの ID・表示名だけで示し、別 server の同名・同 prefix session と行だけで区別できなくすること。
- 所属 server を復元できない識別子を出力すること。
- server 到達不能な状態と session 稼働終了状態を同じ 1 語へ潰すこと。
- detach・terminal を閉じる・回線断のいずれかを session の終了として観察させること。
- 他 client の同時 attach を、裏付けとなる観察源なしに CLI が推測して表示すること。
- 既存 client と異なる規則で display name を導出し、GUI の行と CLI の行を同じ session として突き合わせられなくすること。

---

## 1. Observable state model

CLI の attach lifecycle は次の 7 状態で観察できる。各状態の entry / exit は §4 flows と §6 acceptance scenarios が具体化する。

1. **not-attached** — shell prompt のみが見え、打鍵はエコーされ行編集が効く。discovery / 作成 / 明示終了はこの状態で完結する。CLI がここで異常終了しても shell のエコーと行編集は壊れない。
2. **attaching** — 対象 session が 1 件に解決され接続確立中。打鍵のエコーと行編集はまだ効いたまま。この区間の失敗は復元すべき raw mode が無く、失敗種別の提示と not-attached への復帰だけが観察される (UAC-016)。selector が複数一致した場合はこの状態に入らず候補提示のまま失敗する (UAC-031)。
3. **attached-live** — 打鍵がエコーされず remote へそのまま届く状態。既存 session の現在画面が (空白でも reset でもなく) 表示され、出力と resize が反映される (UAC-009 / UAC-010 / UAC-011)。Ctrl-C はこの状態を終わらせず remote の現在操作の中断だけを起こす (UAC-012)。suspend 操作 (Ctrl-Z 相当) はこの状態からの一時的な離脱であり、suspend 中は shell の打鍵が再びエコーされ、前面復帰で attached-live へ戻る (UAC-042)。
4. **detaching** — detach 打鍵の受理から shell prompt が現れるまでの過渡。**この区間以後の打鍵は remote PTY へ届かない** (UAC-041)。session と agent process は稼働し続ける。打鍵されたバイト列は破棄され、remote にも復帰後の shell にも現れない (§8 DP-DETACHING-INPUT-DISPOSITION = OPT-DETACHING-INPUT-DISCARDED)。
5. **detached-clean** — shell prompt へ戻り、打鍵のエコーと行編集が復元済み。CLI process は終了しているが session は稼働を続け、discovery で離脱前と同じ識別子の行として観察できる (UAC-013 / UAC-022)。
6. **disconnected-abnormally** — 利用者の detach 操作なしに、接続が失われた (UAC-019) か attach 先 session の稼働が server 側で終わった (UAC-018) ことによる離脱。エコーと行編集が復元され、離脱理由が示されてから shell prompt へ戻る。理由文言は接続喪失と session 終了で異なり、detach で戻ったときの表示とも異なる。この離脱自体は session を終了させない。
7. **session-terminated** — 明示終了コマンドによってのみ到達する (UAC-029)。detach・Ctrl-C・terminal close・接続断・CLI の異常終了のいずれもこの状態へ遷移させない (UAC-013 / UAC-024 / UAC-030)。

---

## 2. Confirmed constraints (逐語確定・本 plan が守る)

- **cc-execution-plane-ownership** — CLI は agent process をローカルに spawn しない。PTY / session / multiplexing / snapshot は既存 execution plane が所有し、CLI 側の replay/snapshot store を持たない。CLI は ANSI/xterm state を parse/render せずローカル terminal を唯一の renderer とする。
- **cc-termination-only-explicit** — detach・CLI crash・network disconnect は session/PTY/agent process を終了させず終了要求も発行しない。終了は明示コマンドによってのみ行われる。
- **cc-concurrency-semantics-unchanged** — controller / observer / takeover / input lock / exclusive attach を導入せず、既存 server の subscriber semantics に従う。
- **cc-v1-scope** — v1 の command surface は discovery / attach / create / explicit terminate の 4 種に限定し、機械可読出力は discovery の v1 に含める。
- **cc-input-and-resize-passthrough** — raw attach 中の Ctrl-C は local SIGINT として奪わず 0x03 として remote PTY へ転送する。attach 開始時に自分の terminal geometry を宣言し、以後の resize を既存 resize path へ転送する。
- **cc-public-contract-only** — CLI は内部 API に依存せず public wire contract の client として成立する。
- **cc-raw-mode-restore-all-exits** — raw mode 移行後の全終了経路 (通常 detach、接続 close、server disconnect、認証/protocol 失敗、process signal、stdin EOF、可能な範囲の panic 境界) で、打鍵のエコーと行編集を復元してから終了する。
- **cc-disconnect-reason-line** — attach 後の切断は理由を簡潔に示し、必ず terminal を復元して終了する。この提示は復元後の CLI 所有出力として行う。
- **cc-detach-key-configurable** — detach shortcut は既定値に関わらず config/flag で変更できる。
- **cc-list-population-parity** — 一覧の母集団は既存 GUI client と同じ母集団とし、作成主体では絞り込まない。
- **cc-json-field-breadth** — 機械可読出力の field 集合は既定表示列と同じ広さとする (識別子は server と session を別 field、表示列は元要件 §4.2 の候補)。
- **cc-server-display-name-unique** — 設定で server に付ける display name は一意であること。曖昧性を CLI 側の失敗で吸収するのではなく名前側の制約で消す、という利用者の逐語確定である。この制約は `clients/config` の server 命名契約に属し web-host / Electron / Windows Shell にも及ぶため、CLI 単独では閉じない前提 contract change として design へ渡す (handoff-server-display-name-naming-contract)。この前提が満たされるまでは、既存 GUI の「表示名が重複するときだけ stable 識別子を補助表示する」規律 (§0 継承 4) が観察される規則であり、CLI はそれをそのまま踏襲する。
- **cc-server-display-name-cli-safe-charset** — server display name は CLI で扱える文字種に限定されること。同上の逐語確定であり、許す文字種の具体形と既存設定の移行は同じ前提 contract change へ渡す。

---

## 3. User goals

<!-- anchor: goal-find-attach-target -->
### goal-find-attach-target

- actor: 複数 server 上で複数 session が稼働している開発者
- context: attach 前に対象を取り違えず特定したい
- desired_outcome: discovery 出力から selector をそのまま読み取れる

<!-- anchor: goal-find-attach-target.success-observation -->
success observation: discovery 出力の各行が所属 server の呼称 (既存 GUI と同じく利用者設定の表示名を優先し、表示名が重複するときだけ stable 識別子を補助表示) と対で session を示し、display name が同一の行や短縮 ID が近い行があってもその行だけで区別でき、その行に現れた値をそのまま selector として次の実行へ渡せる。

<!-- anchor: goal-attach-interact-detach -->
### goal-attach-interact-detach

- actor: selector が確定している開発者
- context: 既存 PTY へ入り明示 detach だけで shell へ戻りたい
- desired_outcome: attach 直後から既存画面の続きとして操作でき、明示 detach 以外で離脱しても session は残る

<!-- anchor: goal-attach-interact-detach.success-observation -->
success observation: attach 直後に既存 session の現在画面が (空白画面でも reset された画面でもなく) 表示され、打鍵の効果・出力・resize が remote 側に反映され、detach 操作の直後に打鍵のエコーと行編集が戻った shell prompt へ復帰し、detach 打鍵より後の打鍵は remote にも復帰後の shell にも現れずに破棄され、その後も discovery で同じ session が稼働中と確認できる。

<!-- anchor: goal-interrupt-remote-not-cli -->
### goal-interrupt-remote-not-cli

- actor: attach 中の開発者
- context: remote agent が操作の途中
- desired_outcome: Ctrl-C が remote を中断し CLI を落とさない

<!-- anchor: goal-interrupt-remote-not-cli.success-observation -->
success observation: Ctrl-C 押下後、remote session の出力が現在操作の中断を反映し、かつ CLI は終了せず attach したまま次の入力を受け付ける。

<!-- anchor: goal-observe-liveness -->
### goal-observe-liveness

- actor: attach 中の開発者
- context: attach 中に接続か session が終わる
- desired_outcome: 凍結画面に打ち続けず理由を知って shell へ戻る

<!-- anchor: goal-observe-liveness.success-observation -->
success observation: 接続または session が終わると打鍵のエコーと行編集が復元され、離脱理由が示されてから shell prompt へ戻る。自分の detach による離脱・接続が失われた離脱・session が server 側で終了した離脱の 3 者は、復帰後にその場に残る表示だけで読み分けられる。

<!-- anchor: goal-reattach-after-loss -->
### goal-reattach-after-loss

- actor: 開発者
- context: detach / terminal close / 切断で離れた
- desired_outcome: 同じ selector で再 attach し継続状態から再開する

<!-- anchor: goal-reattach-after-loss.success-observation -->
success observation: 再 attach 直後の terminal に離脱前に画面上にあった内容がそのまま表示され (空白画面でも reset された画面でもない)、以後の出力もそこから連続して届き、識別子は離脱前と同じである。

<!-- anchor: goal-create-session-and-attach -->
### goal-create-session-and-attach

- actor: 開発者
- context: 対象 session がまだ無い
- desired_outcome: 作成先 server・agent 種別・cwd を指定して作成し既定でそのまま attach される

<!-- anchor: goal-create-session-and-attach.success-observation -->
success observation: 作成コマンドが示した識別子と、続けて attach された画面が示す session (起動直後の画面であり既存 session の途中経過ではない) と、その後の一覧行の識別子が一致し、同じ識別子が既存 GUI 一覧にも現れる。作成された session は利用者が指定した server 上に現れ、暗黙に選ばれた別 server には現れない。server 指定を省いた作成は session を作らずに使い方の誤りとして終わる。作成自体が失敗した場合は session が存在せず一覧にも現れないことが確認でき、作成成功後に attach だけ失敗した場合は session の存在と識別子が確認できる。attach しない明示指定時は attach されず識別子だけを得て復帰する。

<!-- anchor: goal-explicit-terminate -->
### goal-explicit-terminate

- actor: 開発者
- context: 不要になった session がある
- desired_outcome: detach や切断とは別の明示操作でのみ終了させる

<!-- anchor: goal-explicit-terminate.success-observation -->
success observation: 明示終了コマンドは対象 session が稼働終了 state へ遷移したことを確かめてから戻り、その直後の一覧で対象だけが稼働終了 state になっている。detach のみ・Ctrl-C のみ・terminal close・切断だけを行った他の session は稼働中のまま残る。終了要求が受け付けられなかった場合は非零終了し、対象の state は要求前と変わらない。

<!-- anchor: goal-script-discovery -->
### goal-script-discovery

- actor: 自動化利用者
- context: table を列 parse せず識別子と属性を得たい
- desired_outcome: 後続処理へ渡せる安定した識別子と、既定表示列と同じ広さの値を得る

<!-- anchor: goal-script-discovery.success-observation -->
success observation: 機械可読出力が構造化データとして読め、server と session が別 field で得られ、display name・cwd/worktree・lifecycle state も同じ出力から field として得られる。加えて各 server の到達可否が field として得られ、到達できない server と session が 0 件の server を出力だけで区別できる。同じ session に繰り返し実行しても識別子 field の値は一致し、人間向け表示の列見出しや順序を変えてもこの出力の意味は変わらない。

<!-- anchor: goal-disambiguate-selector -->
### goal-disambiguate-selector

- actor: 開発者 / 自動化利用者
- context: selector が 2 件以上に一致
- desired_outcome: 自動選択せず候補を提示する

<!-- anchor: goal-disambiguate-selector.success-observation -->
success observation: 曖昧な selector で attach / 明示終了を実行すると、一致候補が各行だけで相互に判別できる形で提示され、コマンドは非零 exit code で失敗として終了する。どの session にも終了操作は行われておらず、対象の lifecycle state が変化していないことを一覧で確認できる (同時 attach の有無を示す観察源は現行の公開情報に存在しないため、attach の非実行は候補提示と非零終了、および対象状態の不変によって判定する)。

<!-- anchor: goal-diagnose-failure-class -->
### goal-diagnose-failure-class

- actor: 開発者 / 自動化利用者
- context: いずれかのコマンドが失敗する
- desired_outcome: 対応を選ぶ前に失敗種別を判別する

<!-- anchor: goal-diagnose-failure-class.success-observation -->
success observation: 未発見・曖昧・認証や互換性の失敗・接続断の 4 種それぞれについて、表示内容と exit code の組が他のどの失敗種別とも一致せず、同じ入力に対して再現する。到達できない server 上にしか存在しない session を指した場合は未発見ではなく接続断として示され、利用者は selector ではなく接続を直す行動を選べる。使い方の誤りはこの 4 種のいずれとも異なる exit code で終わる。

<!-- anchor: goal-multi-server-qualification -->
### goal-multi-server-qualification

- actor: fleet 運用者
- context: 短縮 ID や名前が別 server と重なりうる
- desired_outcome: selector を server へ明示修飾できる

<!-- anchor: goal-multi-server-qualification.success-observation -->
success observation: 一覧行に現れる server の display name、および機械可読出力から読み取った server 識別子のいずれを server 指定として添えても、その selector は単一 server の session だけに一致する。server 指定を添えない同じ selector が複数 server の session に一致すると候補提示になる。display name をそのまま指定に使えるのは、設定側で display name が一意かつ CLI で扱える文字種に限定されていること (cc-server-display-name-unique / cc-server-display-name-cli-safe-charset) が前提であり、この前提は CLI 単独では閉じない。

<!-- anchor: goal-concurrent-modality-continuity -->
### goal-concurrent-modality-continuity

- actor: 開発者
- context: 同じ session に browser / workspace が attach 中
- desired_outcome: 所有権制御なしに既存 semantics のまま使える

<!-- anchor: goal-concurrent-modality-continuity.success-observation -->
success observation: browser/workspace が attach 中でも CLI の attach がそれを理由に拒否されず追加の許可要求も挟まれず、geometry が一致している限り双方が同じテキスト内容の出力を観測できる。

---

## 4. Flows

<!-- anchor: f-001 -->
### F-001 discovery (`ag ls`)

- entry observation: shell から `ag ls` (または `ag ls --json`) を実行する。not-attached のまま。
- exit observation: 一覧 (または構造化データ) が出力され not-attached のまま終了する。attach・作成・終了の副作用は一切発生しない。
- modality reachability: 対話 terminal・script のいずれからも同一の出力形状で到達でき、機械可読出力は既定表示列と同じ広さの値を持つ (cc-json-field-breadth)。
- failure/recovery: 一部 server が到達不能・互換性なしでも健康な server の session を表示し続け、一覧全体を空にしない (UAC-002)。接続側の状態は session の lifecycle 状態とは別語彙で示す (UAC-040)。機械可読出力にも各 server の到達可否が field として現れ、到達不能な server と session 0 件の server を script が区別できる (UAC-046、§8 DP-JSON-SERVER-REACHABILITY-FIELD = OPT-JSON-INCLUDE-REACHABILITY)。

<!-- anchor: f-002 -->
### F-002 attach / interact / detach

- entry observation: not-attached から `ag attach <selector>` を実行し、selector が単一 session に解決される。
- exit observation: 明示 detach なら失敗理由を伴わずに、異常離脱なら離脱理由が示されてから、いずれもエコーと行編集が戻った shell prompt へ復帰する。detach 打鍵より後の打鍵は remote PTY へ届かず、復帰後の shell にも現れずに破棄される。
- modality reachability: browser/workspace が同じ session に同時 attach していても CLI の attach は拒否されず追加の許可要求も挟まれない (exp-concurrent-modality)。stdin が terminal でない起動では attach を開始せず、raw mode 相当へも移行せずに失敗種別を示して非零終了する (§8 DP-NON-TTY-ATTACH-BEHAVIOR = OPT-NON-TTY-FAIL-FAST)。
- failure/recovery: attaching 中の失敗は raw mode 未移行のため復元不要で not-attached へ戻る (UAC-016)。attached-live 中の接続喪失と session 終了は、terminal 復元 + 種別の異なる理由提示 + shell 復帰 (UAC-018 / UAC-019)。復旧手段は同じ selector での再 attach (F-003)。

<!-- anchor: f-003 -->
### F-003 reattach after loss

- entry observation: detached-clean または disconnected-abnormally の状態から、同じ selector で再度 `ag attach` を実行する。
- exit observation: attached-live へ戻り、離脱前に画面上にあった内容がそのまま表示され、以後の出力が連続する。
- modality reachability: 対話 terminal からの再実行を想定し (自動再接続は発明しない)、script からの再実行でも同じ selector 解決規則に従う。
- failure/recovery: 対象が明示終了で既に session-terminated の場合、再 attach は exp-selector-and-failure-diagnosis の失敗種別提示 (未発見相当) に落ちる。

<!-- anchor: f-004 -->
### F-004 create and attach (`ag new`)

- entry observation: not-attached から `ag new --server <server> --agent <kind> --cwd <path>` (任意で `--no-attach`) を実行する。作成先 server の指定は接続台数に関わらず常に必須であり、暗黙の既定 server を持たない (§8 DP-CREATE-TARGET-SERVER = OPT-CREATE-REQUIRE-QUALIFIER)。server 指定を省いた実行はこの flow に入らず、session を作らずに使い方の誤りとして終わる (UAC-045)。
- exit observation: 既定では作成された session の識別子が示された上で、その session の起動直後の画面へ attach する。作成された session は指定した server 上に現れる。`--no-attach` 指定時は attach されず、識別子を示して not-attached のまま終了する。
- modality reachability: 作成結果の識別子は人間向け表示から読み取れる。server 指定は selector と同じ語彙 (display name または機械可読出力の server 識別子) を受理する。作成側の機械可読出力を v1 で必須にするかは §9 に未確定で残る (cc-v1-scope は機械可読出力を discovery の v1 に含めるとだけ確定している)。
- failure/recovery: 作成自体が失敗すれば session は存在せず一覧にも現れず、失敗種別が示される (UAC-043)。作成は成功したが attach だけ失敗した場合は、session の存在と識別子が確認でき、同じ識別子で改めて `ag attach` できる (UAC-026)。server 指定の欠落は使い方の誤りであり、4 失敗種別のいずれとも異なる exit code で終わる (UAC-045)。

<!-- anchor: f-005 -->
### F-005 explicit terminate (`ag kill`)

- entry observation: not-attached から `ag kill <selector>` を実行する。
- exit observation: 対象 session の lifecycle state が稼働終了へ遷移したことを確かめてからコマンドが終了し、直後の一覧で対象だけが稼働終了 state になっている。
- modality reachability: 対話・script いずれでも追加の確認 prompt なしに同一挙動 (DP-TTY-BEHAVIOR-BRANCHING = uniform)。
- failure/recovery: selector が解決できない/曖昧な場合は exp-selector-and-failure-diagnosis に委ね、いかなる session にも終了操作を行わない。解決後に終了要求が受け付けられなかった場合は非零終了し、対象の state は要求前と変わらない (UAC-044)。

<!-- anchor: f-006 -->
### F-006 selector resolution and failure diagnosis

- entry observation: `ag attach` / `ag kill` / `ag new` のいずれかが、解決できない・曖昧・認証や互換性・接続断のいずれかの状況に遭遇する。または第一引数が verb でない呼び出しが行われる。
- exit observation: 状況に応じた文言と、他の失敗種別とは異なる exit code で終了し、対象 session への副作用 (attach / 終了) は発生しない。
- modality reachability: 対話 terminal と非対話 script とで、同じ入力に対する出力内容と exit code が変わらない。
- failure/recovery: 曖昧な場合は各行だけで相互に判別できる候補一覧を示し、利用者/script は selector を絞り込んで再実行する。server 指定を添えれば同じ selector 文字列でも単一 server に絞り込める。到達できない server 上にしか存在しない session を指した selector は、未発見ではなく接続断 (transport 失敗) として示され、復旧行動は selector の修正ではなく接続の回復になる (§8 DP-UNREACHABLE-SERVER-SELECTOR-OUTCOME = OPT-UNREACHABLE-AS-TRANSPORT-FAILURE、UAC-036)。

---

## 5. Experience contracts

<!-- anchor: exp-discovery -->
### exp-discovery

goal_refs: goal-find-attach-target, goal-script-discovery / flow_refs: F-001

<!-- anchor: exp-discovery.interaction-rules -->
interaction rules: `ag ls` は常に接続中の全 server を母集団とし (DP-LIST-SERVER-SCOPE=OPT-ALL-CONNECTED)、停止/失敗した session も含める (DP-LIST-LIFECYCLE-SCOPE=OPT-INCLUDE-ENDED)。既定列は ID または短縮 ID・display name・所属 server・cwd/worktree (存在する範囲)・lifecycle state・model の集合とする (DP-LS-DEFAULT-COLUMNS=OPT-PLAN-MINIMUM-SET)。display name・lifecycle state の label・model は既存 client が同じ session に対して示す値と同じ値を用い、CLI 独自の導出規則や独自語を作らない。所属 server の呼称は利用者設定の表示名を優先し、表示名が重複するときだけ stable 識別子を補助表示する。接続側の状態 (到達不能・互換性なし) は session の lifecycle state とは別の語彙で示す。`--json` は同じ母集団を server / session 別 field の構造化データとして返し (DP-JSON-SESSION-IDENTIFIER-SHAPE=OPT-STRUCTURED-PAIR)、その field 集合は既定表示列と同じ広さを持つ (cc-json-field-breadth)。加えて `--json` は各 server の到達可否を field として持つ (DP-JSON-SERVER-REACHABILITY-FIELD=OPT-JSON-INCLUDE-REACHABILITY)。到達可否は session 行の列ではなく server 側の要素として表し、session の lifecycle state と同じ値空間へ潰さない。

<!-- anchor: exp-discovery.invariants -->
invariants: 一覧の母集団は作成主体で絞り込まない (cc-list-population-parity)。一部 server が到達不能でも一覧全体を空にせず、健康な server の session を示し続ける。session を示す行は所属 server を必ず併記する。同じ session に対する識別子 field の値は実行をまたいで安定し、人間向け表示の変更で機械可読出力の意味が変わらない。到達不能な server と session が 0 件の server は、人間向け表示でも機械可読出力でも区別できる (前者は接続側の語/要素で示され、後者は単に session 行を持たない)。

<!-- anchor: exp-attach-interactive -->
### exp-attach-interactive

goal_refs: goal-attach-interact-detach, goal-interrupt-remote-not-cli, goal-observe-liveness / flow_refs: F-002

<!-- anchor: exp-attach-interactive.interaction-rules -->
interaction rules: attach は selector 解決と接続確立の完了後にのみ raw mode へ移行する (失敗範囲を小さくする)。移行後は既存 session の現在画面がそのまま表示される。打鍵および貼り付けを含む入力は入力順序を保ったまま remote PTY へ届き、Ctrl-C は local SIGINT として奪わず `0x03` として転送される (cc-input-and-resize-passthrough)。1 回の送信に載せる byte 数や結合の有無は観察を変えないため design へ委ねる (handoff-input-transport-batching)。resize はローカル geometry の変化を検出し既存 resize path へ転送する。detach 操作 (既定 Ctrl-]、config/flag で変更可能) は local transport の解除だけを行い終了要求を送らない。detach 打鍵の受理以後、続く打鍵は remote PTY へ届かず、復帰後の shell の入力としても現れず破棄される (DP-DETACHING-INPUT-DISPOSITION=OPT-DETACHING-INPUT-DISCARDED)。stdin が terminal でない起動では attach を開始せず、失敗種別を示して非零終了する (DP-NON-TTY-ATTACH-BEHAVIOR=OPT-NON-TTY-FAIL-FAST。出力だけを流す動作や読み取り専用 attach という別の契約を `attach` に名乗らせない)。raw mode 中は CLI 所有の出力を持たない (DP-ATTACH-OBSERVABILITY-CHANNEL=OPT-TRANSPARENT-ATTACH)。raw mode 復元後に、離脱が clean detach であることを別 channel で示すことは許容される (確定判断の逐語どおり、状態の出所を別 channel にすること自体は許容される)。

<!-- anchor: exp-attach-interactive.invariants -->
invariants: raw mode 移行後の全終了経路 (通常 detach・接続 close・server disconnect・移行後の認証/protocol 失敗・process signal・suspend・stdin EOF・可能な範囲の panic 境界) で、打鍵のエコーと行編集を復元してから終了または離脱する (cc-raw-mode-restore-all-exits)。detach・CLI crash・network disconnect は session/PTY/agent process を終了させない (cc-termination-only-explicit)。異常離脱時は理由を簡潔に示してから shell prompt へ戻り (cc-disconnect-reason-line)、接続が失われた離脱と session が server 側で終了した離脱は異なる文言で示され、後者は既存 client の Stopped / Failed 語彙と対応が読み取れる。明示 detach の復帰表示は、いずれの失敗理由文言とも一致しない。stdin が terminal でない起動では raw mode 相当の設定を制御端末へ適用せず、実行の前後で shell のエコーと行編集を変化させず、attach を開始しないまま非零 exit code で終了する。

<!-- anchor: exp-concurrent-modality -->
### exp-concurrent-modality

goal_refs: goal-concurrent-modality-continuity / flow_refs: F-002

<!-- anchor: exp-concurrent-modality.interaction-rules -->
interaction rules: 同じ session に browser/workspace が既に attach していても、CLI の attach は既存 server の subscriber semantics にそのまま従って成立する。controller/observer/takeover の判定や追加の確認手順を挟まない。

<!-- anchor: exp-concurrent-modality.invariants -->
invariants: CLI は他 client の同時 attach を示す表示を一切持たない (DP-CONCURRENT-ATTACH-VISIBILITY=OPT-NO-VISIBILITY、それを支える公開情報が存在しないため)。geometry が一致している同時 attach では双方が観測するテキスト内容が一致する。geometry が一致しない場合の描画結果はこの契約の観察対象外であり、共有 PTY の cols/rows の決め方は既存 execution plane への handoff とする (handoff-shared-pty-geometry-resolution)。

<!-- anchor: exp-reattach-continuity -->
### exp-reattach-continuity

goal_refs: goal-reattach-after-loss / flow_refs: F-003

<!-- anchor: exp-reattach-continuity.interaction-rules -->
interaction rules: 再 attach は通常の attach (F-002 entry) と同じ selector 解決を経て行われる。CLI 側に新しい replay/snapshot store を持たず、新規 subscriber へ先に現在画面を届ける既存の subscribe 挙動にそのまま乗る (cc-execution-plane-ownership)。

<!-- anchor: exp-reattach-continuity.invariants -->
invariants: 離脱理由が detached-clean であれ disconnected-abnormally であれ、session が session-terminated に達していない限り再 attach は同じ画面内容から継続し、識別子は離脱前と同じである。CLI 側で画面を再構成・再生成しない。ローカル transport の消失 (CLI プロセスの不意な終了を含む) は session の稼働を止めない。

<!-- anchor: exp-create-and-attach -->
### exp-create-and-attach

goal_refs: goal-create-session-and-attach / flow_refs: F-004

<!-- anchor: exp-create-and-attach.interaction-rules -->
interaction rules: `ag new` は既存 lifecycle の作成要求と同じ wire model を使い、CLI 固有の作成 model を新設しない。作成先 server の指定は接続台数に関わらず常に必須であり、暗黙の既定 server も「1 台しか繋がっていなければ推測する」振る舞いも持たない (DP-CREATE-TARGET-SERVER=OPT-CREATE-REQUIRE-QUALIFIER)。server 指定は selector への server 修飾と同じ語彙を受理する (DP-SERVER-QUALIFIER-VALUE-VOCABULARY=OPT-QUALIFIER-ACCEPTS-DISPLAY-NAME)。作成が成功すると既定でその session への attach (F-002 entry) が続けて行われ、`--no-attach` は作成だけで終える。

<!-- anchor: exp-create-and-attach.invariants -->
invariants: 作成コマンドが示す識別子と、その後の attach 先および一覧行が示す識別子は常に一致し、作成された session は利用者が指定した server 上に現れる。作成直後の attach 先は新規 session の起動直後の画面であり、既に稼働している別 session の画面ではない。作成失敗・作成成功後の attach 失敗・成功の 3 者はそれぞれ異なる観察になり、作成が失敗した場合は一覧に新しい行が現れない。server 指定を欠いた実行は session を作らず、4 失敗種別のいずれとも異なる使い方の誤りとして終わる。

<!-- anchor: exp-explicit-terminate -->
### exp-explicit-terminate

goal_refs: goal-explicit-terminate / flow_refs: F-005

<!-- anchor: exp-explicit-terminate.interaction-rules -->
interaction rules: `ag kill <selector>` は session termination を明示的に要求する唯一の command である。selector 解決規則は F-006 と共有する。コマンドは対象の lifecycle state が稼働終了へ遷移したことを確かめてから終了する。server が graceful と force を区別するかは未測定であり、両方を CLI に公開するかは §9 に未確定で残す。

<!-- anchor: exp-explicit-terminate.invariants -->
invariants: 明示終了コマンドを実行した session だけが稼働終了 state へ遷移する。detach のみ・Ctrl-C のみ・terminal close・切断のいずれも終了要求を発行しない (cc-termination-only-explicit)。終了要求が受け付けられなかった場合は非零終了し、対象の lifecycle state は要求前と変わらない。対話/script で挙動は変わらず、確認 prompt を追加で挟まない (DP-TTY-BEHAVIOR-BRANCHING=OPT-UNIFORM-BEHAVIOR)。

<!-- anchor: exp-selector-and-failure-diagnosis -->
### exp-selector-and-failure-diagnosis

goal_refs: goal-disambiguate-selector, goal-diagnose-failure-class, goal-multi-server-qualification / flow_refs: F-006

<!-- anchor: exp-selector-and-failure-diagnosis.interaction-rules -->
interaction rules: selector の種別 (ID / 短縮 ID prefix / 条件が揃えば名前) は明示 flag で宣言させ、暗黙の優先順位で推測しない (DP-SELECTOR-PRECEDENCE-ORDER=OPT-EXPLICIT-MODE-FLAGS)。server への修飾は selector 文字列とは別の独立した指定で行い (DP-SERVER-QUALIFIER-SYNTAX=OPT-SEPARATE-FLAG)、機械可読出力の server 識別子 field の値と、一覧行に現れる利用者設定の display name の双方を受理する (DP-SERVER-QUALIFIER-VALUE-VOCABULARY=OPT-QUALIFIER-ACCEPTS-DISPLAY-NAME)。display name を受理しても修飾後に複数 server へ一致する経路を作らないのは、名前側の制約 (cc-server-display-name-unique / cc-server-display-name-cli-safe-charset) が曖昧性を先に消すからであり、CLI 側で追加の曖昧解消を導入するからではない。この制約は `clients/config` の server 命名契約に属する前提 contract change であり CLI 単独では閉じない (handoff-server-display-name-naming-contract)。第一引数は常に verb として解釈され、selector を位置引数として受けない (DP-ENTRY-DISPATCH-STYLE=OPT-VERB-FIRST-ONLY)。曖昧な selector は候補一覧を示して非零終了する (DP-AMBIGUITY-INTERACTIVITY=OPT-STATIC-LIST-NONZERO)。候補一覧は各行だけで相互に判別でき、その行の値で次の実行の対象を一意に指定できる。候補行にどこまでの情報を併記するかの広さは §9 に未確定で残す。

<!-- anchor: exp-selector-and-failure-diagnosis.invariants -->
invariants: 未発見・曖昧・認証や互換性の失敗・接続断の 4 種はそれぞれ他と異なる exit code を持ち、同じ入力に対して再現する (DP-EXIT-CODE-GRANULARITY=OPT-DISTINCT-CODE-PER-CLASS。具体的な番号割当は handoff-exit-code-numeric-assignment)。到達できない server 上にしか存在しない session を指した selector は接続断に分類し、未発見へ寄せない (DP-UNREACHABLE-SERVER-SELECTOR-OUTCOME=OPT-UNREACHABLE-AS-TRANSPORT-FAILURE)。使い方の誤りはこの 4 種のいずれとも異なる exit code で終わる。これらの失敗経路はいずれの session にも attach / 終了の副作用を生じさせない。対話 terminal と非対話実行とで出力内容と exit code は変わらない。

---

## 6. Acceptance scenarios

### F-001 discovery

<!-- anchor: uac-001 -->
#### UAC-001 discovery rows carry the server the way existing clients name it (vs_legacy: must-fail)

<!-- anchor: uac-001.given -->
Given: 表示名の異なる 2 台の server に接続しており、両 server にそれぞれ 1 件ずつ session が稼働している。2 件の session の display name は同一である。

<!-- anchor: uac-001.when -->
When: `ag ls` を実行する。

<!-- anchor: uac-001.then -->
Then: 2 つの行それぞれに、その session が属する server の呼称が併記される。呼称は既存 GUI と同じ規律 (利用者設定の表示名を用い、表示名が重複するときだけ stable 識別子を補助表示する) に従う。display name が同一でも、利用者は各行だけでどちらの server の session かを読み取れる。

<!-- anchor: uac-001.counterexample -->
Counterexample: session ID と display name だけを並べ所属 server を出さない実装は、2 行が同一に見えて判別できず fail する。逆に server 表示名が重複していない状況でも全行へ不透明な stable server 識別子だけを出す実装は、既存 GUI が見せている server 呼称と一致せず、GUI の行と CLI の行を同じ server として突き合わせられなくなり fail する。

<!-- anchor: uac-002 -->
#### UAC-002 partial server outage does not empty the list

<!-- anchor: uac-002.given -->
Given: 2 台の server に接続しており、一方は到達できないか protocol 互換性が無く、もう一方は正常で 1 件以上の session を抱えている。

<!-- anchor: uac-002.when -->
When: `ag ls` を実行する。

<!-- anchor: uac-002.then -->
Then: 正常な server の session 行が表示され、一覧全体が空にならない。

<!-- anchor: uac-002.counterexample -->
Counterexample: 1 台でも到達できないと一覧全体をエラーにして何も表示しない実装は、健康な server の session を見落とさせるため fail する。

<!-- anchor: uac-003 -->
#### UAC-003 machine-readable identifiers are stable across repeated calls

<!-- anchor: uac-003.given -->
Given: 稼働中の session が 1 件以上あり、観測の間その状態は変化しない。

<!-- anchor: uac-003.when -->
When: `ag ls --json` を間隔をおいて 2 回実行する。

<!-- anchor: uac-003.then -->
Then: 同じ session を表す entry の server 識別子 field と session 識別子 field の値が、2 回の実行で一致する。

<!-- anchor: uac-003.counterexample -->
Counterexample: その実行の中でだけ有効な一時的な値 (実行ごとに振り直す連番など) を識別子 field に入れる実装は、1 回目に得た値を 2 回目の対象指定へ渡しても同じ session を指せず fail する。

<!-- anchor: uac-004 -->
#### UAC-004 default columns and the display name match the existing clients (vs_legacy: must-fail)

<!-- anchor: uac-004.given -->
Given: 既存 GUI 一覧上で特定の display name として表示されている session が 1 件あり、その session は cwd/worktree を持ち、既存 GUI 一覧はその session の model を示している。

<!-- anchor: uac-004.when -->
When: flag を付けずに `ag ls` を実行する。

<!-- anchor: uac-004.then -->
Then: その行に、対象を一意に指せる識別子 (完全な ID、または selector 種別として prefix を宣言すれば解決できる短縮 ID)、display name、所属 server、cwd/worktree、lifecycle state、model が現れる。display name と model は、既存 GUI 一覧が同じ session に対して示す値と同一である。

<!-- anchor: uac-004.counterexample -->
Counterexample: 所属 server 列を省く実装、既定列から model を落とす実装、および起動コマンド文字列や worktree path など既存 client と異なる値から display name を導く実装、既存 client が同じ session に対して示す model 値を用いず CLI 独自に導出した値 (起動コマンドの引数からの推測など) を model として示す実装は、いずれも利用者が GUI の行と CLI の行を同じ session として突き合わせられなくなり fail する。

<!-- anchor: uac-005 -->
#### UAC-005 population parity across creators (vs_legacy: must-pass)

<!-- anchor: uac-005.given -->
Given: 既存 GUI client から作成された session と、CLI から作成された session がそれぞれ 1 件ずつ稼働している。

<!-- anchor: uac-005.when -->
When: `ag ls` を実行する。

<!-- anchor: uac-005.then -->
Then: 作成主体に関わらず両方の session が一覧に現れる。

<!-- anchor: uac-005.counterexample -->
Counterexample: CLI 経由で作成された session だけを表示する (作成主体でのフィルタを新設する) 実装は、他端末・他利用者が作った attach 対象を見落とさせるため fail する。

<!-- anchor: uac-006 -->
#### UAC-006 lifecycle vocabulary matches existing clients (vs_legacy: must-fail)

<!-- anchor: uac-006.given -->
Given: 既存 GUI client 上で「Working」と表示される状態の session が 1 件ある。

<!-- anchor: uac-006.when -->
When: `ag ls` を実行する。

<!-- anchor: uac-006.then -->
Then: 同じ session の lifecycle state 列に、既存 client と同一の label 文字列が現れる。

<!-- anchor: uac-006.counterexample -->
Counterexample: 同じ状態を "running" や "live" のような CLI 独自語で表示する実装は、同じ session を GUI と CLI で別状態と読ませ fail する。

<!-- anchor: uac-007 -->
#### UAC-007 ended sessions remain listed with a state change

<!-- anchor: uac-007.given -->
Given: 稼働していた session の remote process が自発的に終了しており、明示終了コマンドは実行されていない。

<!-- anchor: uac-007.when -->
When: `ag ls` を実行する。

<!-- anchor: uac-007.then -->
Then: その session の行は一覧から消えず、lifecycle state 列が終了を示す既存 label に変わっている。

<!-- anchor: uac-007.counterexample -->
Counterexample: 終了した session を一覧から削除する実装は、「行が消えた」ことと「明示終了された」ことを利用者に区別させず fail する。

<!-- anchor: uac-039 -->
#### UAC-039 state priority ordering matches existing clients (vs_legacy: must-fail)

<!-- anchor: uac-039.given -->
Given: ある session が同時に複数の状態条件を満たしており、既存 GUI client がその組み合わせを固定の優先順 (failed > human_input > active > stopped > idle) で 1 つの label に解決している。

<!-- anchor: uac-039.when -->
When: `ag ls` を実行する。

<!-- anchor: uac-039.then -->
Then: その session の lifecycle state 列に現れる label は、既存 GUI client が同じ組み合わせに対して示すのと同じ label である。

<!-- anchor: uac-039.counterexample -->
Counterexample: 既存 client と異なる優先順 (例えば active を human_input より前面に出す) で状態語を選ぶ実装は、同じ session を見ても CLI と GUI で異なる状態が示され fail する。

<!-- anchor: uac-040 -->
#### UAC-040 an unreachable server and an ended session use distinct vocabulary (vs_legacy: must-fail)

<!-- anchor: uac-040.given -->
Given: 2 台の接続先 server のうち 1 台が到達できないか protocol 互換性が無く、もう 1 台は健康で、その健康な server 上に稼働終了した session が 1 件ある。

<!-- anchor: uac-040.when -->
When: `ag ls` を実行する。

<!-- anchor: uac-040.then -->
Then: 接続できない server について示される語と、稼働終了した session の lifecycle state として示される語は異なる。接続側の語は既存 GUI の接続一覧が同じ状況で使う語と対応し、利用者はどちらが「接続を直す」でどちらが「session を見直す」かを読み分けられる。

<!-- anchor: uac-040.counterexample -->
Counterexample: 両者へ同じ 1 語 (単に "Failed" 相当) を使う実装は、利用者に接続を直すべきか再 attach すべきかを判別させず fail する。

<!-- anchor: uac-008 -->
#### UAC-008 machine-readable output pairs server and session and carries the default columns (vs_legacy: must-fail)

<!-- anchor: uac-008.given -->
Given: 稼働中の session が 1 件あり、display name・cwd/worktree・lifecycle state を持つ。

<!-- anchor: uac-008.when -->
When: `ag ls --json` を実行し、得られた entry の server 識別子 field の値をそのまま server 指定として渡して `ag attach` を実行する。

<!-- anchor: uac-008.then -->
Then: entry は server と session を別々の field として持ち、加えて display name・cwd/worktree・lifecycle state を field として持つ。渡した server 値による attach は同じ session に到達する。

<!-- anchor: uac-008.counterexample -->
Counterexample: server と session を 1 個の合成文字列 field でしか返さない実装は、呼び出し側に文書化されていない区切り文字の推測を強いるため fail する。識別子 field だけを返す実装は、自動化利用者が display name や lifecycle state を人間向け table の列 parse からしか得られず fail する。

<!-- anchor: uac-046 -->
#### UAC-046 machine-readable output separates an unreachable server from a server with no sessions

<!-- anchor: uac-046.given -->
Given: 3 台の server に接続しており、1 台目は到達できないか protocol 互換性が無く、2 台目は健康だが session を 1 件も持たず、3 台目は健康で session を 1 件以上持つ。

<!-- anchor: uac-046.when -->
When: `ag ls --json` を実行し、出力だけを読む (人間向け表示は参照しない)。

<!-- anchor: uac-046.then -->
Then: 出力から 1 台目と 2 台目を区別できる。1 台目には到達できないことを表す値が現れ、2 台目にはその値が現れずに session の要素が空である。3 台目の session 要素は空でない。到達可否を表す値は session の lifecycle state と同じ値空間には属さない。

<!-- anchor: uac-046.counterexample -->
Counterexample: 到達できない server を出力から丸ごと省く実装、および到達できない server を「session 0 件の server」と同じ形で返す実装は、script が「接続を直す」のか「その server には対象が無い」のかを判別できず fail する (人間向け表示ではこの区別が UAC-040 で得られるのに、自動化利用者だけが列 parse へ追いやられる)。到達可否を session の lifecycle state と同じ field へ混ぜる実装は、稼働終了した session と到達不能な server を同じ語で読ませるため fail する。

---

### F-002 attach / interact / detach

<!-- anchor: uac-009 -->
#### UAC-009 attach shows the current screen, not a blank or reset one

<!-- anchor: uac-009.given -->
Given: 稼働中の session の画面に、直前の操作に由来する内容が既に表示されている (idle であっても白紙ではない)。

<!-- anchor: uac-009.when -->
When: `ag attach <selector>` でその session に attach する。

<!-- anchor: uac-009.then -->
Then: attach 直後の terminal に、attach 前から画面上にあった内容がそのまま表示される。

<!-- anchor: uac-009.counterexample -->
Counterexample: 現在画面を受け取らず購読開始以降の出力だけを流す実装は、idle な session では画面が空白のまま表示され fail する。

<!-- anchor: uac-010 -->
#### UAC-010 keystrokes round-trip to the remote PTY

<!-- anchor: uac-010.given -->
Given: session に attach 済みで、remote 側では入力された行を解釈して、打鍵した文字列そのものとは異なる固有の出力を返せる状態 (既知のコマンドとその応答が確定している) にある。

<!-- anchor: uac-010.when -->
When: その既知の入力を打鍵して確定する。

<!-- anchor: uac-010.then -->
Then: 打鍵した文字列の echo ではなく、remote 側の処理結果に由来する出力が terminal に現れる。

<!-- anchor: uac-010.counterexample -->
Counterexample: 打鍵を local で echo するだけで remote へ送らない実装は、terminal 上は正常に見えるが remote 側でしか生成できない応答を再現できないため fail する。

<!-- anchor: uac-011 -->
#### UAC-011 resize reaches the remote surface, including during full-screen redraw

<!-- anchor: uac-011.given -->
Given: session に attach 済みで、この session に同時 attach している他 client は無く、remote 側が full-screen UI を再描画し続けている最中である。

<!-- anchor: uac-011.when -->
When: local terminal のサイズを変更する。

<!-- anchor: uac-011.then -->
Then: 新しい列/行数を反映した形で remote 側の画面が再描画される。

<!-- anchor: uac-011.counterexample -->
Counterexample: resize を転送しない実装、または送るが full-screen 再描画中は取りこぼす実装は、terminal サイズと remote 描画がずれたまま残り fail する。

<!-- anchor: uac-012 -->
#### UAC-012 Ctrl-C interrupts the remote operation without ending the CLI

<!-- anchor: uac-012.given -->
Given: session に attach 済みで、remote 側が長時間操作の途中である。

<!-- anchor: uac-012.when -->
When: Ctrl-C を押す。

<!-- anchor: uac-012.then -->
Then: remote 側の出力が操作の中断を反映し、CLI は終了せず attach したまま次の入力を受け付ける。

<!-- anchor: uac-012.counterexample -->
Counterexample: Ctrl-C を local の中断 signal として先取りして CLI を終了させる実装は、remote の操作が中断されないまま shell へ戻り fail する。

<!-- anchor: uac-013 -->
#### UAC-013 explicit detach returns to a cooked shell without terminating the session (vs_legacy: must-fail)

<!-- anchor: uac-013.given -->
Given: session に attach 済みである。

<!-- anchor: uac-013.when -->
When: detach 操作 (既定 Ctrl-]) を行う。

<!-- anchor: uac-013.then -->
Then: 打鍵のエコーと行編集が戻った shell prompt へ復帰し、その場に残る表示はいずれの失敗理由文言とも一致しない。その後の `ag ls` で同じ session が稼働中のまま観察できる。

<!-- anchor: uac-013.counterexample -->
Counterexample: detach を暗黙の終了要求として送る実装は、直後の `ag ls` でその session が稼働終了 state になり fail する。

<!-- anchor: uac-041 -->
#### UAC-041 keystrokes after the detach key are discarded and surface nowhere

<!-- anchor: uac-041.given -->
Given: session に attach 済みで、remote 側では入力した文字がそのまま画面に見える状態が動作しており、現在の画面内容が分かっている。

<!-- anchor: uac-041.when -->
When: detach 操作 (既定 Ctrl-]) を行い、その直後に続けて既知の文字列を打鍵する。shell prompt が戻ったところで shell の入力行を確認し、その後で同じ selector で再度 attach する。

<!-- anchor: uac-041.then -->
Then: shell prompt へ戻った時点で、detach 操作より後に打鍵した文字列は shell の入力行にも現れていない (打鍵は破棄されている)。再 attach 後の画面にもその文字列は現れず、detach 直前までに打鍵した内容はそのまま残っている。

<!-- anchor: uac-041.counterexample -->
Counterexample: detach 受理後も入力の読み取りと転送を続ける実装は、再 attach 後の画面に detach 後の打鍵が現れて fail する (利用者は shell へ打ったつもりの文字列を remote agent へ送ってしまう)。detach 受理後の打鍵を buffer して復帰後の shell へ流し込む実装は、shell の入力行に意図しない文字列が現れて fail する。

<!-- anchor: uac-014 -->
#### UAC-014 the detach shortcut is reconfigurable and the default key then reaches the remote

<!-- anchor: uac-014.given -->
Given: config または flag で detach shortcut を既定の Ctrl-] とは別のキーに設定しており、remote 側では入力バイトをそのまま可視の形で表示する状態が動作している。

<!-- anchor: uac-014.when -->
When: session に attach し、まず Ctrl-] を打鍵し、続いて設定した別キーを打鍵する。

<!-- anchor: uac-014.then -->
Then: Ctrl-] の打鍵に対応する可視表示が remote 側の画面に現れて attach は継続し、その後の別キーの打鍵で detach して shell prompt へ戻る。

<!-- anchor: uac-014.counterexample -->
Counterexample: 設定を無視して常に Ctrl-] を detach として扱う実装は、Ctrl-] の時点で離脱してしまい fail する。Ctrl-] を detach にも転送にも使わず無言で捨てる実装は、remote 側の可視表示が現れず fail する。

<!-- anchor: uac-015 -->
#### UAC-015 a non-TTY stdin fails fast without touching the invoking terminal

<!-- anchor: uac-015.given -->
Given: 対話 terminal の shell から、stdin だけを pipe/redirect にして CLI を起動する (shell 自身の terminal は同じ window に残る)。selector は稼働中の単一 session へ解決できる。

<!-- anchor: uac-015.when -->
When: この状態で `ag attach <selector>` を実行し、コマンドの終了を待ってから shell に戻って文字を打つ。

<!-- anchor: uac-015.then -->
Then: コマンドは利用者の中断を要さず自ら終了し、attach は開始されないまま非 TTY であることを示す失敗種別と理由が示されて非零 exit code で終わる。shell の打鍵のエコーと行編集は実行前と同じままである。その後の `ag ls` で対象 session は稼働中のまま変化していない。

<!-- anchor: uac-015.counterexample -->
Counterexample: stdin が terminal かどうかを確かめずに制御端末へ raw mode 相当の設定を適用し、この経路で復元を経ずに終わる実装は、コマンド後の shell で打鍵が見えなくなり fail する。remote の出力だけを流し続けて終了しない実装、および入力を受け付けない読み取り専用 attach として成功終了する実装は、`attach` という同じ語に別の契約を名乗らせ、利用者が中断するまで戻らない/成功と誤認させるため fail する。

<!-- anchor: uac-016 -->
#### UAC-016 a connection failure before raw mode leaves the terminal untouched

<!-- anchor: uac-016.given -->
Given: 対象 session は到達可能な server の一覧に現れており selector は 1 件に解決できるが、その session の画面へ接続する段階だけが失敗する状況を再現する。

<!-- anchor: uac-016.when -->
When: `ag attach <selector>` を実行する。

<!-- anchor: uac-016.then -->
Then: 失敗種別を伴う理由が示されて非零 exit code で終了し、shell の打鍵のエコーと行編集は実行前から一度も変化していない。

<!-- anchor: uac-016.counterexample -->
Counterexample: 接続確立より前に raw mode へ入り、この失敗経路では復元を経ずに終了する実装は、shell のエコーが壊れたまま戻り fail する (raw mode 移行を解決と接続の後まで遅らせないと、この経路が復元漏れの対象になる)。

<!-- anchor: uac-017 -->
#### UAC-017 a catchable terminating signal restores the terminal before exiting

<!-- anchor: uac-017.given -->
Given: session に attach 済みで raw mode に入っている。

<!-- anchor: uac-017.when -->
When: CLI process が、process を終了させる catchable な signal (SIGINT / SIGTERM 相当) を受け取る。

<!-- anchor: uac-017.then -->
Then: CLI が終了する前に打鍵のエコーと行編集が復元され、shell では打鍵が再び見える。その後の `ag ls` で対象 session は稼働中のまま観察できる。

<!-- anchor: uac-017.counterexample -->
Counterexample: この signal を受け取ると復元処理を経由せず即座に終了する実装は、shell が raw mode のまま残り打鍵が見えなくなって fail する。

<!-- anchor: uac-042 -->
#### UAC-042 suspend and resume restore and re-establish the terminal mode

<!-- anchor: uac-042.given -->
Given: session に attach 済みで raw mode に入っている。

<!-- anchor: uac-042.when -->
When: CLI process を suspend する操作 (Ctrl-Z 相当) を行い、shell で数文字打鍵したのち前面へ戻す (fg 相当)。

<!-- anchor: uac-042.then -->
Then: suspend 直後の shell では打鍵がエコーされ行編集が効く。前面へ戻したあとは再び attach した画面が表示され、打鍵はエコーされずそのまま remote へ届く。

<!-- anchor: uac-042.counterexample -->
Counterexample: suspend 時に terminal 設定を戻さない実装は、shell 側で打鍵が見えず fail する。前面復帰時に raw mode を張り直さない実装は、復帰後の打鍵が行単位でしか届かず remote の対話操作を続けられないため fail する。

<!-- anchor: uac-018 -->
#### UAC-018 a server-side session end restores the terminal with its own reason (vs_legacy: must-fail)

<!-- anchor: uac-018.given -->
Given: session に attach 済みで raw mode に入っており、利用者は detach 操作を行わない。

<!-- anchor: uac-018.when -->
When: その session が server 側で稼働終了し、lifecycle state が既存 client の Stopped または Failed に相当する状態へ遷移する。

<!-- anchor: uac-018.then -->
Then: 打鍵のエコーと行編集が復元され、離脱理由として session の稼働が終わったことが示される。その文言は既存 client が同じ状態に使う label (Stopped / Failed) と対応が読み取れ、接続が失われたときの理由文言 (UAC-019) とは異なる。その後の `ag ls` でこの session は終了状態の行として残る。

<!-- anchor: uac-018.counterexample -->
Counterexample: 異常離脱の全経路へ同一の 1 行 (単に "disconnected" 相当) を出す実装は、利用者に「接続を直す」のか「session はもう終わっている」のかを判別させず fail する。

<!-- anchor: uac-019 -->
#### UAC-019 clean detach, lost connection, and ended session are distinguishable on return (vs_legacy: must-fail)

<!-- anchor: uac-019.given -->
Given: 稼働を続ける同じ session に対して 2 回 attach し、1 回目は明示 detach で、2 回目は利用者操作なしの接続喪失で離脱する。

<!-- anchor: uac-019.when -->
When: それぞれの離脱直後に terminal に残る表示を比べ、さらに UAC-018 の離脱表示と比べる。

<!-- anchor: uac-019.then -->
Then: いずれも打鍵のエコーと行編集が戻った shell prompt へ復帰する。接続喪失側には接続が失われたことを示す理由が現れ、明示 detach 側にはその失敗理由文言が現れない (clean detach である旨を復元後に別 channel で示すことは許容されるが、失敗理由と同じ文言にはならない)。接続喪失の理由文言は、session が server 側で終了したときの理由文言とも異なる。利用者はこの表示だけで、再 attach すればよいのか、接続を直すのか、session がもう無いのかを選び分けられる。

<!-- anchor: uac-019.counterexample -->
Counterexample: 明示 detach と接続喪失で同じ表示を出す実装、および異常離脱の全経路へ同じ 1 語だけを出す実装は、利用者に次の行動を選ばせず fail する。

---

### F-002 concurrent modality

<!-- anchor: uac-020 -->
#### UAC-020 concurrent CLI attach is not gated by an existing GUI attach

<!-- anchor: uac-020.given -->
Given: browser または workspace client が既に同じ session へ attach しており、その client の表示 geometry は CLI を起動する terminal の geometry と一致している。

<!-- anchor: uac-020.when -->
When: 同じ session へ `ag attach <selector>` を実行する。

<!-- anchor: uac-020.then -->
Then: CLI の attach は拒否されず、追加の許可要求や所有権の確認手順も挟まれずに成立し、以後 remote が出力するテキストを双方が同じ内容として観測できる。

<!-- anchor: uac-020.counterexample -->
Counterexample: 「既に他 client が attach 中」を理由に拒否する、または確認手順を挟む実装は、controller/observer 相当の所有権制御を新設したことになり fail する。geometry が一致しない同時 attach の描画結果は本 scenario の対象外であり、その解決は既存 execution plane への handoff とする。

<!-- anchor: uac-021 -->
#### UAC-021 no presence indicator is shown for concurrent attach (vs_legacy: must-fail)

<!-- anchor: uac-021.given -->
Given: browser client が既に session へ attach 中である。

<!-- anchor: uac-021.when -->
When: 同じ session へ CLI で attach する。

<!-- anchor: uac-021.then -->
Then: CLI の出力のどこにも「他に N 件の client が attach 中」に相当する表示が現れない。

<!-- anchor: uac-021.counterexample -->
Counterexample: 同時 attach の件数や一覧を表示する実装は、それを支える公開情報が存在しないため推測を表示することになり fail する。

---

### F-003 reattach after loss

<!-- anchor: uac-022 -->
#### UAC-022 reattach after a clean detach continues from the prior screen

<!-- anchor: uac-022.given -->
Given: session から明示 detach した直後で、session は稼働を続けている。

<!-- anchor: uac-022.when -->
When: 同じ selector で再度 `ag attach` する。

<!-- anchor: uac-022.then -->
Then: detach 直前に画面上にあった内容がそのまま表示され、以後の出力がそこから連続する。

<!-- anchor: uac-022.counterexample -->
Counterexample: 再 attach 時に画面を空白または初期状態から描き直す実装は、利用者が離脱前の文脈を失い fail する。

<!-- anchor: uac-023 -->
#### UAC-023 reattach after an abnormal disconnect continues from the prior screen

<!-- anchor: uac-023.given -->
Given: session への接続が失われ (session 自体は継続稼働)、離脱理由が示されたあと shell に戻っている。

<!-- anchor: uac-023.when -->
When: 同じ selector で再度 `ag attach` する。

<!-- anchor: uac-023.then -->
Then: 切断前に画面上にあった内容がそのまま表示され、以後の出力がそこから連続し、識別子は離脱前と同じである。

<!-- anchor: uac-023.counterexample -->
Counterexample: 接続喪失後の再 attach を新しい session の作成として扱う実装は、識別子が変わって継続にならず fail する。

<!-- anchor: uac-024 -->
#### UAC-024 an uncatchable local kill does not terminate the remote session (vs_legacy: must-fail)

<!-- anchor: uac-024.given -->
Given: session に attach 済みである。

<!-- anchor: uac-024.when -->
When: local の CLI process が捕捉不能な signal で強制終了させられる。

<!-- anchor: uac-024.then -->
Then: その後の `ag ls` で対象 session は稼働中のまま観察でき、同じ selector で再 attach して離脱前の画面内容から継続できる。

<!-- anchor: uac-024.counterexample -->
Counterexample: attach 開始時に「この接続が失われたら session も終了させる」に相当する要求を server へ登録する実装は、CLI の異常終了がそのまま session の終了を引き起こし、利用者は terminal が落ちただけで agent を失って fail する。

---

### F-004 create and attach

<!-- anchor: uac-025 -->
#### UAC-025 `ag new` attaches to the session it just created

<!-- anchor: uac-025.given -->
Given: display name の異なる 2 台の server に接続しており (display name は一意で CLI で扱える文字種に収まっている)、一方の server には既に別の session が 1 件稼働していて、その session の画面には進行中の内容が表示されている。

<!-- anchor: uac-025.when -->
When: `ag new --server <その server の display name> --agent <kind> --cwd <path>` を `--no-attach` なしで実行する。

<!-- anchor: uac-025.then -->
Then: attach 直後の画面は新しく作成された session の起動直後の画面であり、Given で確認した既存 session の進行中の画面ではないことが terminal 上で見分けられる。作成コマンドが示した識別子は、その後の `ag ls` の行および既存 GUI 一覧に現れる識別子と一致し、その行の所属 server は指定した server である。

<!-- anchor: uac-025.counterexample -->
Counterexample: 作成した session の識別子を表示しながら一覧の先頭行など別の session へ attach する実装は、既存 session の途中経過画面が見えてしまい fail する。display name を server 指定として受理せず失敗する実装、および指定を無視してもう一方の server 上に session を作る実装も fail する。

<!-- anchor: uac-026 -->
#### UAC-026 create success + attach failure is reported distinctly

<!-- anchor: uac-026.given -->
Given: server 指定には有効な値を与えたうえで、session の作成自体は成功するが直後の画面接続だけが失敗する状況を再現する。

<!-- anchor: uac-026.when -->
When: `ag new --server <server> --agent <kind> --cwd <path>` を実行する。

<!-- anchor: uac-026.then -->
Then: 作成が成功したこととその識別子が示され、attach が失敗したことは作成失敗とは別の事象として示される。その後の `ag ls` に同じ識別子の行が稼働中として現れ、利用者はその識別子で `ag attach` を再実行できる。

<!-- anchor: uac-026.counterexample -->
Counterexample: 作成と attach を単一の成否として扱い attach 失敗を「作成失敗」として報告する実装は、既に存在する session を利用者から隠し、再実行で重複 session を作らせて fail する。

<!-- anchor: uac-027 -->
#### UAC-027 `--no-attach` creates without attaching

<!-- anchor: uac-027.given -->
Given: 対象 session はまだ存在せず、作成先とする server の呼称が分かっている。

<!-- anchor: uac-027.when -->
When: `ag new --server <server> --agent <kind> --cwd <path> --no-attach` を実行する。

<!-- anchor: uac-027.then -->
Then: attach は行われず not-attached のまま終了し、示された識別子がその後の `ag ls` に指定した server 上の稼働中の行として現れる。

<!-- anchor: uac-027.counterexample -->
Counterexample: `--no-attach` を無視して attach する実装、または識別子を示さない実装は、利用者が後から対象を特定できず fail する。

<!-- anchor: uac-043 -->
#### UAC-043 a failed create leaves no session behind

<!-- anchor: uac-043.given -->
Given: server 指定には到達可能な server の有効な呼称を与えたうえで、存在しない cwd または未知の agent 種別を指定する。

<!-- anchor: uac-043.when -->
When: `ag new --server <server> --agent <kind> --cwd <path>` を実行する。

<!-- anchor: uac-043.then -->
Then: 作成が失敗したことが attach 失敗とは異なる文言で示されて非零 exit code で終了し、その後の `ag ls` に新しい行は現れない。

<!-- anchor: uac-043.counterexample -->
Counterexample: 要求を送っただけで成功として識別子らしき値を表示する実装は、`ag ls` に対応する行が無いのに利用者と script が session の存在を前提にしてしまい fail する。

<!-- anchor: uac-045 -->
#### UAC-045 creating without a server qualifier fails as a usage error and creates nothing

<!-- anchor: uac-045.given -->
Given: 接続中の server がちょうど 1 台であり、その 1 台は健康で agent 種別と cwd はいずれも有効である (作成先を推測しようと思えばできる状況)。実行前の `ag ls` の行数が分かっている。

<!-- anchor: uac-045.when -->
When: server 指定を添えずに `ag new --agent <kind> --cwd <path>` を実行する。

<!-- anchor: uac-045.then -->
Then: session は作成されず、`ag ls` の行は実行前と同じままである。コマンドは使い方の誤りとして非零 exit code で終了し、その exit code は未発見・曖昧・認証や互換性・接続断の 4 失敗種別のいずれとも異なる。

<!-- anchor: uac-045.counterexample -->
Counterexample: 接続中の server が 1 台しかないことを根拠に暗黙の作成先として採用する実装は、この実行で session を作ってしまい fail する (利用者はどの server に作られたかを指定していないのに、後日 2 台目を繋いだ時点で同じコマンドの意味が変わる)。省略を曖昧 (候補提示) として扱い曖昧の exit code を返す実装も、script が「selector を絞り込む」経路へ誤って分岐するため fail する。

---

### F-005 explicit terminate

<!-- anchor: uac-029 -->
#### UAC-029 explicit terminate affects only the targeted session and confirms the transition (vs_legacy: must-fail)

<!-- anchor: uac-029.given -->
Given: 3 件の session が稼働しており、うち 1 件だけを対象にする。他の 2 件はそれぞれ detach のみ、Ctrl-C のみを経験している。

<!-- anchor: uac-029.when -->
When: 対象の 1 件に対して `ag kill <selector>` を実行し、コマンドが戻った直後に `ag ls` を実行する。

<!-- anchor: uac-029.then -->
Then: コマンドは対象の lifecycle state が稼働終了へ遷移したことを確かめてから戻り、直後の `ag ls` で対象の 1 件だけが稼働終了 state になっている。他の 2 件は稼働中のまま残る。

<!-- anchor: uac-029.counterexample -->
Counterexample: 終了要求を送るだけで遷移を確かめずに戻る実装は、直後の `ag ls` で対象がまだ稼働中と見えることがあり、利用者と script は終了できたのか判別できず fail する。detach や Ctrl-C を経験しただけの session まで終了状態にする実装も fail する。

<!-- anchor: uac-030 -->
#### UAC-030 stdin EOF (terminal closed) does not terminate the session (vs_legacy: must-fail)

<!-- anchor: uac-030.given -->
Given: session に attach 済みである。

<!-- anchor: uac-030.when -->
When: local terminal window が閉じられ、CLI の入力が終端に達する。

<!-- anchor: uac-030.then -->
Then: その後の `ag ls` で対象 session は稼働中のまま観察でき、同じ selector で再 attach できる。

<!-- anchor: uac-030.counterexample -->
Counterexample: 入力の終端を検出して終了要求を送る実装は、terminal を閉じただけで session が終了し fail する。

<!-- anchor: uac-044 -->
#### UAC-044 a rejected terminate request is reported and leaves the state unchanged

<!-- anchor: uac-044.given -->
Given: selector は 1 件に解決できるが、その対象への終了要求が受け付けられない状況を再現する (既に終了している、要求が拒否される等)。

<!-- anchor: uac-044.when -->
When: `ag kill <selector>` を実行する。

<!-- anchor: uac-044.then -->
Then: 終了できなかったことが示されて非零 exit code で終了し、その後の `ag ls` で対象の lifecycle state は要求前と変わっていない。

<!-- anchor: uac-044.counterexample -->
Counterexample: 要求の結果を確かめず常に成功として 0 で終了する実装は、script に終了できたと誤認させて後続処理を進めさせ fail する。

---

### F-006 selector resolution and failure diagnosis

<!-- anchor: uac-031 -->
#### UAC-031 an ambiguous selector produces mutually distinguishable candidates with no side effect (vs_legacy: must-fail)

<!-- anchor: uac-031.given -->
Given: 表示名の異なる 2 台の server に、同じ selector 文字列へ一致する session が 1 件ずつあり、2 件の display name は同一である。

<!-- anchor: uac-031.when -->
When: この selector で `ag attach` (または `ag kill`) を実行する。

<!-- anchor: uac-031.then -->
Then: 一致した候補が一覧として示され、各行だけで候補同士を判別でき、その行に現れた値だけで次の実行の対象を一意に指定できる。コマンドは非零 exit code で終了し、両 session の lifecycle state はどちらも変化していないことを `ag ls` で確認できる。

<!-- anchor: uac-031.counterexample -->
Counterexample: 候補の中から自動的に 1 件 (例えば最新の 1 件) を選んで実行する実装は、意図しない session に作用し fail する。一致件数だけを出す実装や、display name が同一で見分けのつかない同じ行を並べる実装は、利用者が次にどちらを指定すればよいか決められず fail する (候補行にどこまで併記するかの広さは未確定であり、判別可能でありさえすれば手段は問わない)。

<!-- anchor: uac-032 -->
#### UAC-032 an explicit server qualifier narrows a cross-server collision, by display name or by stable id

<!-- anchor: uac-032.given -->
Given: 2 台の server にそれぞれ同じ selector 文字列へ一致する session があり、2 台の display name は互いに異なり CLI で扱える文字種に収まっている (cc-server-display-name-unique / cc-server-display-name-cli-safe-charset が満たされている)。`ag ls` の行から一方の display name を、`ag ls --json` から同じ server の識別子 field の値を読み取ってある。

<!-- anchor: uac-032.when -->
When: 読み取った display name を server 指定として添えて `ag attach <selector>` を実行し、続いて同じ server の識別子 field の値を server 指定として添えて同じ実行を行う。

<!-- anchor: uac-032.then -->
Then: どちらの実行も指定した server 上の session だけに一致し、同じ session への attach が成立する。もう一方の server の session は対象にならない。

<!-- anchor: uac-032.counterexample -->
Counterexample: server 指定を無視して依然として両 server の候補を曖昧として提示する実装は、fleet 利用者が 1 行で対象を確定できず fail する。display name を受理せず stable 識別子でしか修飾できない実装は、利用者が人間向け一覧の行から読み取った呼称をそのまま渡せず fail する (この受理は cc-server-display-name-unique / cc-server-display-name-cli-safe-charset を前提とし、その前提は `clients/config` 側の contract change として handoff-server-display-name-naming-contract に依存する)。

<!-- anchor: uac-033 -->
#### UAC-033 an unqualified cross-server collision falls back to ambiguity

<!-- anchor: uac-033.given -->
Given: 2 台の server にそれぞれ同じ selector 文字列へ一致する session がある。

<!-- anchor: uac-033.when -->
When: server 指定を付けずに `ag attach <selector>` を実行する。

<!-- anchor: uac-033.then -->
Then: 両 server の候補が提示され、非零 exit code で終了し、どちらの session にも終了操作は行われず lifecycle state も変化しない。

<!-- anchor: uac-033.counterexample -->
Counterexample: 設定順で先の server などを暗黙に優先して attach する実装は、利用者が指定していない server の session を操作してしまい fail する。

<!-- anchor: uac-034 -->
#### UAC-034 a declared selector kind decides the resolution

<!-- anchor: uac-034.given -->
Given: ある文字列が、一方の session の完全な ID そのものであり、同時に別の session の ID の prefix でもある。

<!-- anchor: uac-034.when -->
When: 同じ文字列で、selector 種別として prefix を宣言した実行と、ID を宣言した実行をそれぞれ行う。

<!-- anchor: uac-034.then -->
Then: prefix を宣言した実行は 2 件に一致して候補提示と非零終了になり、ID を宣言した実行は完全一致する 1 件へ解決して attach が成立する。

<!-- anchor: uac-034.counterexample -->
Counterexample: 宣言された種別を無視して ID 完全一致を優先する暗黙順位で解決する実装は、prefix を宣言した実行でも 1 件へ attach してしまい、利用者が宣言と異なる session に入って fail する。

<!-- anchor: uac-035 -->
#### UAC-035 the first argument is always the verb, never an implicit selector

<!-- anchor: uac-035.given -->
Given: 稼働中の session が 1 件あり、その完全な ID を知っている。その文字列は CLI の subcommand 名のいずれとも一致しない。

<!-- anchor: uac-035.when -->
When: verb を書かずに、その session ID を第一引数として CLI を実行する。

<!-- anchor: uac-035.then -->
Then: その session への attach は起きず、使い方の誤りとして非零 exit code で終了する。この exit code は未発見・曖昧・認証や互換性・接続断の 4 失敗種別のいずれとも異なる。その後の `ag ls` で対象 session は稼働中のまま変化していない。

<!-- anchor: uac-035.counterexample -->
Counterexample: 第一引数が verb でなければ attach 対象として解釈する実装は、この session へ attach してしまい fail する。

<!-- anchor: uac-036 -->
#### UAC-036 the four failure classes produce mutually distinct, reproducible exit codes

<!-- anchor: uac-036.given -->
Given: 未発見・曖昧・認証や互換性の失敗・接続断のそれぞれを引き起こす 4 つの入力状況を用意する。接続断の状況は、2 台の server のうち 1 台が到達不能で、その到達不能な server 上にしか存在しない session を指す selector を与えることで作る。未発見の状況は、到達可能な全 server のどこにも存在しない selector を与えることで作る。

<!-- anchor: uac-036.when -->
When: それぞれの状況で対応するコマンドを実行し、同じ状況をもう一度再現して再実行する。

<!-- anchor: uac-036.then -->
Then: 4 つの exit code は互いに異なり、同じ状況の再実行でも同じ (表示内容, exit code) の組が再現する。到達不能 server 上の session を指した実行は接続断の (表示内容, exit code) になり、未発見のそれとは一致しない。具体的な番号割当はここでは確定しない。

<!-- anchor: uac-036.counterexample -->
Counterexample: 曖昧と接続断へ同じ exit code を割り当てる実装は、script が「selector を絞り込む」のか「接続を直す」のかを分岐できず fail する。到達不能な server 上の session を「どこにも見つからない」として未発見に分類する実装は、接続を直せば解決する状況で利用者と script に selector の修正を試させ fail する。

<!-- anchor: uac-037 -->
#### UAC-037 interactive and non-interactive invocations behave identically for failing commands

<!-- anchor: uac-037.given -->
Given: 曖昧な selector など、失敗を引き起こす同一の入力がある。

<!-- anchor: uac-037.when -->
When: この入力を対話 terminal から実行した場合と、出力を pipe に繋いだ非対話実行とで比べる。

<!-- anchor: uac-037.then -->
Then: 両者の出力内容と exit code が一致し、対話側にだけ候補から選ばせる prompt が現れることはない。

<!-- anchor: uac-037.counterexample -->
Counterexample: 対話 terminal のときだけ候補選択の prompt を表示する実装は、同じ入力の観察が環境で分岐し、手元で見た結果と script から見える結果が食い違って fail する。

<!-- anchor: uac-038 -->
#### UAC-038 explicit terminate has no TTY-only confirmation prompt

<!-- anchor: uac-038.given -->
Given: 一意に解決できる selector で対象 session が特定できている。

<!-- anchor: uac-038.when -->
When: 対話 terminal から `ag kill <selector>` を実行した場合と、script から同じコマンドを実行した場合とで比べる。

<!-- anchor: uac-038.then -->
Then: 両者とも追加の確認入力なしに同じ結果 (対象 session の稼働終了) に至る。

<!-- anchor: uac-038.counterexample -->
Counterexample: 対話 terminal でのみ「本当に終了しますか」の確認を挟む実装は、script からの呼び出しと挙動が分岐し fail する。

---

## 7. Decision points closed by user consultation

<!-- anchor: dp-entry-dispatch-style -->
**DP-ENTRY-DISPATCH-STYLE** (normative / answered): 第一引数は常に verb とし、selector を位置引数として受けない (OPT-VERB-FIRST-ONLY)。selector が subcommand 名と同じ文字列でも予測可能性が崩れない。→ UAC-035。

<!-- anchor: dp-ambiguity-interactivity -->
**DP-AMBIGUITY-INTERACTIVITY** (normative / answered): 曖昧な selector は静的な候補一覧 + 非零終了 (OPT-STATIC-LIST-NONZERO)。CLI 側の選択描画を持たず、手元と script で観察が一致する。→ UAC-031, UAC-033, UAC-034。

<!-- anchor: dp-tty-behavior-branching -->
**DP-TTY-BEHAVIOR-BRANCHING** (normative / answered): TTY 検出で挙動を分岐させない (OPT-UNIFORM-BEHAVIOR)。手元で見たものと script から見えるものが一致し、明示終了の確認 prompt もこの軸に従い持たない。適用範囲は CLI が選べる挙動 (確認 prompt の有無、候補選択方式、失敗時の出力と exit code) であり、打鍵のエコーを外す raw mode へ移行できるかどうかは端末の物理的能力に従う不可避の差でこの軸の対象ではない。→ UAC-037, UAC-038, UAC-015。

<!-- anchor: dp-list-server-scope -->
**DP-LIST-SERVER-SCOPE** (normative / answered): 一覧の母集団は接続中の全 server (OPT-ALL-CONNECTED)。既存 GUI の fleet 表示と同じ母集団になり server 併記が必須になる。→ UAC-001, UAC-002。

<!-- anchor: dp-list-lifecycle-scope -->
**DP-LIST-LIFECYCLE-SCOPE** (normative / answered): 停止・失敗した session も一覧に出す (OPT-INCLUDE-ENDED)。明示終了の効果を行の消滅ではなく状態語の変化として観察でき、接続失敗と稼働終了を取り違えない。→ UAC-007, UAC-029, UAC-040。

<!-- anchor: dp-selector-precedence-order -->
**DP-SELECTOR-PRECEDENCE-ORDER** (normative / answered): selector の種別を明示指定させ、暗黙の優先順位を持たない (OPT-EXPLICIT-MODE-FLAGS)。対話利用で冗長になる代償を受け入れる。→ UAC-034, UAC-004。

<!-- anchor: dp-server-qualifier-syntax -->
**DP-SERVER-QUALIFIER-SYNTAX** (normative / answered): server 修飾は selector とは別の独立した指定として与える (OPT-SEPARATE-FLAG)。selector 文字列に区切り文字の意味を持たせない。→ UAC-008, UAC-032, UAC-033。

<!-- anchor: dp-ls-default-columns -->
**DP-LS-DEFAULT-COLUMNS** (default / answered): 既定表示列は元要件 §4.2 の候補列 (OPT-PLAN-MINIMUM-SET) とし、その候補列に model を含める。agent 種別は公開情報から導ける範囲に限る。当初この plan は model 相当を「公開されている session 情報に存在しない」ことを理由に除外していたが、design 段階の repository 実測でその前提は偽と判明した (公開 payload に model の JSON tag があり `services/host/state/view/view.go:23`、共有 SPA が実際に読んでいる `clients/ui/src/components/SessionList.tsx:83`)。したがって除外は取得可否ではなく product decision であり、user_consultation (consultation-20260814-cli-terminal-client-model-column) で「model 列を含める」と逐語確定した。cc-json-field-breadth により `--json` の field 集合も同じ広さで model を含む。→ UAC-004。

<!-- anchor: dp-json-session-identifier-shape -->
**DP-JSON-SESSION-IDENTIFIER-SHAPE** (normative / answered): 機械可読出力は server と session を別 field で返す (OPT-STRUCTURED-PAIR)。所属 server を復元でき、独立した server 指定へそのまま渡せる。→ UAC-008, UAC-032。

<!-- anchor: dp-exit-code-granularity -->
**DP-EXIT-CODE-GRANULARITY** (normative / answered): 未発見・曖昧・認証や互換性の失敗・接続断を script が exit code で分岐できる粒度にする (OPT-DISTINCT-CODE-PER-CLASS)。番号の具体的な割当は確定していない。→ UAC-036, UAC-035。

<!-- anchor: dp-detach-default-key -->
**DP-DETACH-DEFAULT-KEY** (default / answered): 既定 detach は単発 Ctrl-] (OPT-CTRL-BRACKET)。Ctrl-D 案は Claude Code / Codex の終了操作と PTY 内 shell の EOF を奪うため退けた。実機の chord 衝突確認は未検証で残り、config/flag による変更手段が回避路になる。→ UAC-013, UAC-014, UAC-041。

<!-- anchor: dp-attach-observability-channel -->
**DP-ATTACH-OBSERVABILITY-CHANNEL** (normative / answered): raw mode 中は CLI 所有の観察出力を持たず完全透過とする (OPT-TRANSPARENT-ATTACH)。切断されたら attach を解除して端末を復元し、失敗種別を伴うエラーを出すことが観察要件になる。状態の出所を別 channel にすること自体は許容される。→ UAC-013, UAC-018, UAC-019。

<!-- anchor: dp-concurrent-attach-visibility -->
**DP-CONCURRENT-ATTACH-VISIBILITY** (normative / answered): 他 client の同時 attach を表示しない (OPT-NO-VISIBILITY)。現行の公開情報に presence に相当する field が無く、元要件 §17 の server 変更規則を発動させない。→ UAC-021, UAC-031。

---

## 8. Decision points created in this phase (closed by follow-up consultation)

批判が指摘した未定義の境界を軸として起こしたものである。6 件すべてが consultation-20260813-cli-terminal-client-specify で利用者により確定した。integrator が「どの option が選ばれても pass/fail が変わらない」よう前提を限定していた scenario は、確定内容を観察する形へ書き直してある。

<!-- anchor: dp-detaching-input-disposition -->
**DP-DETACHING-INPUT-DISPOSITION** (normative / answered): detach 打鍵の受理後に打たれたバイト列は破棄する (OPT-DETACHING-INPUT-DISCARDED)。remote PTY へ届かないことは UAC-041 で既に確定しており、本回答は手元側の帰着を閉じた。離脱中の打鍵はどこにも現れない。→ UAC-041。

<!-- anchor: dp-create-target-server -->
**DP-CREATE-TARGET-SERVER** (normative / answered): `ag new` は作成先 server の指定を常に必須とする (OPT-CREATE-REQUIRE-QUALIFIER)。selector 種別の明示指定 (OPT-EXPLICIT-MODE-FLAGS) と曖昧時の非零終了 (OPT-STATIC-LIST-NONZERO) と揃い、暗黙の既定へ寄る非対称を作らない。接続が 1 台でも推測しない。→ UAC-045, UAC-025, UAC-026, UAC-027, UAC-043。

<!-- anchor: dp-server-qualifier-value-vocabulary -->
**DP-SERVER-QUALIFIER-VALUE-VOCABULARY** (normative / answered): server 指定は stable な server 識別子に加えて利用者設定の display name も受理する (OPT-QUALIFIER-ACCEPTS-DISPLAY-NAME)。修飾後の曖昧経路は CLI 側の追加失敗ではなく名前側の制約で消す — すなわち display name は一意であること、CLI で扱える文字種に限定されることの 2 点を confirmed constraint として持つ (cc-server-display-name-unique / cc-server-display-name-cli-safe-charset)。この制約は `clients/config` の server 命名契約に属し web-host / Electron / Windows Shell にも及ぶため、元要件 §17 の前提 contract change として design へ渡し、CLI 単独では閉じない (handoff-server-display-name-naming-contract)。前提が満たされるまでは §0 継承 4 の重複時 disambiguation が観察される規則として残る。→ UAC-032, UAC-025。

<!-- anchor: dp-unreachable-server-selector-outcome -->
**DP-UNREACHABLE-SERVER-SELECTOR-OUTCOME** (normative / answered): 到達不能な server 上にしか存在しない session を指す selector は、未発見ではなく接続断 (transport 失敗) として示す (OPT-UNREACHABLE-AS-TRANSPORT-FAILURE)。利用者は selector ではなく接続を直す行動へ向かい、OPT-DISTINCT-CODE-PER-CLASS により script も両者を分岐できる。→ UAC-036。

<!-- anchor: dp-json-server-reachability-field -->
**DP-JSON-SERVER-REACHABILITY-FIELD** (normative / answered): 機械可読出力に各 server の到達可否を含める (OPT-JSON-INCLUDE-REACHABILITY)。到達不能な server と session 0 件の server を script が区別できる。cc-json-field-breadth に従い、表示側にも到達不能が現れることが前提になる (UAC-040)。→ UAC-046。

<!-- anchor: dp-non-tty-attach-behavior -->
**DP-NON-TTY-ATTACH-BEHAVIOR** (normative / answered): stdin が terminal でないときの `ag attach` は attach を開始せず、失敗種別を示して非零終了する (OPT-NON-TTY-FAIL-FAST)。`attach` という語が環境により別の契約を名乗ることを避け、読み取り専用 attach という v1 surface も増やさない。DP-TTY-BEHAVIOR-BRANCHING の適用範囲 (CLI が選べる挙動の一様性) とは矛盾しない。→ UAC-015。

---

## 9. Product decisions remaining (carried, not closed here)

- **曖昧候補の提示に所属 server と display name を併記するか ID だけか** — explore が未確定として残した軸を、そのままの文言で保持する。planner が既定値を置いたが、利用者確認を経ていないため確定要求へ昇格させない。UAC-031 は「候補同士が相互に判別できること」だけを要求し、併記の広さには依存しない (handoff-ambiguous-candidate-breadth)。
- **実行 binary の canonical 名** — 短縮名か自己記述名かは packaging 時に決定する (handoff-cli-binary-canonical-name)。
- **name selector および短縮 ID の一意性を fleet 全体で見るか server 単位で見るか** — stable name field の実在が未測定であり、短縮 ID の一意性判定範囲も同じ未測定軸に束ねる。存在しなければ selector 種別は完全 ID と prefix の 2 段に縮む。本 consultation で確定した cc-server-display-name-unique は **server** の display name にかかる制約であり、session 側の name / 短縮 ID の一意性範囲を決めない。server 名が一意になると、この軸が「server 単位」を選んだ場合に server 指定で範囲を確定させる手段は確保されるが、指定を添えない実行の解決範囲は依然として未確定である。確定要求へ昇格させない。
- **server が graceful/force を区別する場合に CLI も両方公開するか** — 区別の実在が未測定。
- **異なる geometry の同時 attach で共有 PTY の cols/rows をどう決めるか** — 既存 execution plane の semantics に属する (handoff-shared-pty-geometry-resolution)。UAC-020 は geometry 一致時の観察だけを固定する。
- **exit code の具体的な番号割当** — 種別ごとに専用 code を持つことは確定、番号は未定 (handoff-exit-code-numeric-assignment)。
- **Ctrl-] が Claude Code / Codex の key chord と衝突しないか** — 実機未検証。衝突時の回避手段 (shortcut の変更) は UAC-014 で確保済み。
- **作成側の機械可読出力を v1 で必須にするか** — cc-v1-scope は機械可読出力を discovery の v1 に含めるとだけ確定しており、元要件 §13 も作成側については「option を持たせてもよい」に留まる。確定要求へ昇格させない。

---

## 10. Design handoffs

- **handoff-cli-binary-canonical-name** — 実行 binary の canonical 名。観察を変えない packaging 判断。
- **handoff-shared-pty-geometry-resolution** — 共有 PTY が単一の cols/rows しか持てないときの解決方法。既存 execution plane の semantics に属する。
- **handoff-exit-code-numeric-assignment** — 4 失敗種別と使い方の誤りに割り当てる具体的な番号。
- **handoff-input-transport-batching** — 入力を 1 回の送信へどうまとめるか。順序が保たれる限り観察は変わらない。
- **handoff-ambiguous-candidate-breadth** — 曖昧候補行に併記する情報の広さ。判別可能性という観察要件の内側で design が選ぶか、利用者へ確認する。
- **handoff-server-display-name-naming-contract** — server display name を一意にし CLI で扱える文字種へ限定するという確定制約 (cc-server-display-name-unique / cc-server-display-name-cli-safe-charset) の具体形。許す文字種、一意性をどこで強制するか、既存設定の移行、そして `clients/config` の server 命名契約を共有する web-host / Electron / Windows Shell への波及を design が決める。元要件 §17 の前提 contract change であり CLI 単独では閉じない。UAC-032 と UAC-025 の Given はこの前提が満たされた状態を明示しており、前提が未達のあいだ両 scenario の display name 経路は成立しない。
