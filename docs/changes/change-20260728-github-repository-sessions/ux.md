---
change: change-20260728-github-repository-sessions
role: ux
technology_candidates:
  - {"id":"candidate-github-app-auth","candidate":"GitHub App または OAuth device/web flow による client-mediated authentication","source":"explicit_user_input","proposed_for":["repository catalog access","credential-safe authentication"],"required_capabilities":["browser と Electron の shared UX","credential を URL/web storage に置かない","拒否・期限切れ・再認証を区別できる"],"ux_constraints_relevant_to_evaluation":["健康な server projects を認証失敗で遮断しない","選択した auth recovery policy を実現できる"],"design_questions":["credential mediation の authority と保存境界","browser と Electron の callback handoff"],"disqualifiers":["bearer credential を URL または web storage に置く","frontend を session lifecycle authority にする"]}
  - {"id":"candidate-github-repository-api","candidate":"GitHub repository listing API with pagination and refresh metadata","source":"planner_proposal","proposed_for":["repository catalog","loading・stale・partial failure の観察"],"required_capabilities":["owner/name を含む識別","pagination","失敗 source の局所化"],"ux_constraints_relevant_to_evaluation":["保持済み一覧と鮮度を区別できる","refresh policy の options を実現できる"],"design_questions":["refresh metadata の正本","rate limit と pagination の扱い"],"disqualifiers":["GitHub 取得失敗で project catalog 全体を利用不能にする"]}
  - {"id":"candidate-unified-target-model","candidate":"source-discriminated target projection shared by browser and Electron SPA hosts","source":"repository_context","proposed_for":["project と repository の一貫した row semantics","stable source identity"],"required_capabilities":["server project と repository を区別する","project server identity と repository owner/name を保持する"],"ux_constraints_relevant_to_evaluation":["同名 target が常時識別可能","shared SPA の feature parity"],"design_questions":["static metadata の REST projection","repository と execution server の typed association"],"disqualifiers":["host mode ごとの feature fork","source identity を hover のみに隠す"]}
design_handoffs:
  - {"id":"handoff-auth-mediation","required_outcome":"Settings の account surface を経る GitHub 認証と New Session 復帰を、credential 境界を破らず browser と Electron で同じ体験にする。","design_obligations":["credential authority と callback 境界を定める","URL または web storage に bearer credential を置かない","query/filter と GitHub category 入口の復元責務を定める"],"verification_obligations":["未認証、拒否、失敗、成功復帰を browser と Electron で照合する","外部依存の fake、invariant contract、FakeVsReal backstop を配置する"],"provenance":{"source":"repository_evidence","confidence":"confirmed","evidence_refs":["explore-plan-reconciled","handoff-auth-mediation"],"rationale":"explore_plan:handoff-auth-mediation、DP-GITHUB-AUTH-ENTRY、constraint-shared-client"},"decision_refs":["DP-GITHUB-AUTH-ENTRY"]}
  - {"id":"handoff-target-projection","required_outcome":"project と repository の source、stable identity、execution server を shared clients へ一貫して投影する。","design_obligations":["source-discriminated target と repository/server association を型で表す","server-owned lifecycle authority を保つ","host mode ごとの feature fork を作らない"],"verification_obligations":["同名 cross-source target、fleet-wide projects、repository 選択後の server 選択を browser と Electron の双方で検証する"],"provenance":{"source":"repository_evidence","confidence":"confirmed","evidence_refs":["explore-plan-reconciled","handoff-target-projection"],"rationale":"explore_plan:handoff-target-projection、DP-TARGET-TOPOLOGY、DP-ROW-PROVENANCE、DP-EXECUTION-SERVER"},"decision_refs":["DP-TARGET-TOPOLOGY","DP-ROW-PROVENANCE","DP-EXECUTION-SERVER"]}
  - {"id":"handoff-accessible-catalog","required_outcome":"category、row provenance、loading、stale、source-scoped failure、recovery の意味と操作可能性を全 modality で同等に伝える。","design_obligations":["視覚だけに依存しない category と source identity を定める","状態変化と focus recovery の支援技術向け意味を定める","健康な候補の操作を error 状態から分離する"],"verification_obligations":["keyboard、pointer、accessibility tree で entry、selection、Settings 復帰、refresh、部分失敗を検証する","hover-only provenance と global blocking の counterexample を must-fail にする"],"provenance":{"source":"repository_evidence","confidence":"confirmed","evidence_refs":["explore-plan-reconciled","handoff-accessible-catalog"],"rationale":"DP-ROW-PROVENANCE、DP-PARTIAL-CATALOG、DP-GITHUB-AUTH-ENTRY、DP-REPOSITORY-REFRESH"},"decision_refs":["DP-ROW-PROVENANCE","DP-PARTIAL-CATALOG","DP-GITHUB-AUTH-ENTRY","DP-REPOSITORY-REFRESH"]}
  - {"id":"handoff-repository-lifecycle","required_outcome":"server-owned creation authority の下で session 固有 clone と repository 単位 per-project devcontainer を一貫して管理する。","design_obligations":["同一 repository の session clone を隔離する","repository と execution server の確認を create authority へ渡す","失敗または中止した部分 session を available として投影しない"],"verification_obligations":["成功、clone failure、devcontainer failure、cancel、retry、同一 repository の複数 session を検証する","外部依存ごとに fake、invariant contract、FakeVsReal backstop を配置する"],"provenance":{"source":"repository_evidence","confidence":"confirmed","evidence_refs":["explore-plan-reconciled","handoff-repository-lifecycle"],"rationale":"constraint-repository-runtime、context-server-owned-lifecycle、DP-CLONE-COLLISION、DP-CANCEL-RECOVERY"},"decision_refs":["DP-CLONE-COLLISION","DP-CANCEL-RECOVERY"]}
  - {"id":"handoff-preparation-progress","required_outcome":"clone と devcontainer 準備の段階、失敗、中止、retry、availability を視覚と支援技術で判別可能にする。","design_obligations":["段階遷移と失敗 scope の user-facing vocabulary を定める","retry 面で repository、server、失敗または中止段階を保持する","通知方式や timing threshold は設計時に検証可能な形で定める"],"verification_obligations":["段階順序、失敗段階、retry、退出、未完成 session 非表示を各 modality で検証する"],"provenance":{"source":"repository_evidence","confidence":"confirmed","evidence_refs":["explore-plan-reconciled","handoff-preparation-progress"],"rationale":"DP-DEVCONTAINER-VISIBILITY、DP-CANCEL-RECOVERY"},"decision_refs":["DP-DEVCONTAINER-VISIBILITY","DP-CANCEL-RECOVERY"]}
  - {"id":"handoff-search-validation","required_outcome":"owner/name を provisional initial search scope として検証し、不十分なら extended metadata の decision を再開する。","design_obligations":["owner/name と query の対応が予測可能な初期 behavior を定める","description 等を検証前の confirmed capability に含めない","invalidation 後の変更を topology/ranking redesign と束ねない"],"verification_obligations":["実データ相当 catalog で既知 target 発見と曖昧記憶の失敗例を収集する","validated または invalidated の根拠を DP-REPOSITORY-SEARCH-FIELDS に記録する"],"provenance":{"source":"repository_evidence","confidence":"confirmed","evidence_refs":["explore-plan-reconciled","handoff-search-validation"],"rationale":"DP-REPOSITORY-SEARCH-FIELDS、assumption-search-fields"},"decision_refs":["DP-REPOSITORY-SEARCH-FIELDS"]}
---

<!-- lifecycle is owned by change.md -->

# GitHub repository sessions UX

## Goal

New Session から、既存の server-qualified project 検索を損なわず、GitHub repository を同じ categorized discovery へ追加し、出所と実行先を取り違えずに session を開始できるようにする。

既存 fleet-wide palette を基準にし、hover-only provenance と project-only catalog は置換する。

## User Goals

- 一つの入口から目的の target を発見し、source と識別情報を確認して選択する — 成功観察: 候補と選択後の確認で、project は server identity、repository は owner/name と実行先 server を識別できる
- 従来どおり全接続 server の project を検索し、正しい server-qualified target を選択する — 成功観察: GitHub 認証状態にかかわらず project を検索でき、stable server identity を保ったまま command 選択と確認へ進める
- 利用可能な候補を失わず、失敗範囲を理解して回復操作を選ぶ — 成功観察: 健康な source の結果と現在の検索文脈が残り、失敗した source の理由と回復操作が分かる
- session 単位の clone と repository 単位の devcontainer 準備の進行を理解し、成功または失敗から次へ進む — 成功観察: clone と devcontainer 準備を区別して確認でき、成功時は session が利用可能になり、失敗または中止時は対象と段階を保って再試行または退出できる

## Primary Flows and Acceptance Scenarios

### F-001 categorized target picker から source と identity を確認して選択する

Entry/Progress/Exit: [keyboard|pointer|assistive-technology] New Session の既存単一入口を開き、初期状態で Projects と GitHub repositories の category を確認する → [keyboard|pointer|assistive-technology] 一つの query で target を検索し、各 row の source badge と stable identity を確認する → [keyboard|pointer|assistive-technology] target を選び、後続画面でも source と stable identity を確認する

- UAC-001: Given browser と Electron-hosted の双方で、GitHub 認証済みで project と repository の保持済み候補があり、New Session は未入力の初期状態である When 利用者が keyboard、pointer、または支援技術で New Session を開く Then 一つの検索欄の下に Projects と GitHub repositories が category として現れ、source filter は全 source を対象とし、各 row には source と project の server identity または repository の owner/name が常時分かる Counterexample: source-first の追加選択を要求する、前回の source filter を初期値にする、または category heading だけに source を置く実装は、承認済みの categorized single-entry、全 source 初期表示、row 単位 provenance を観察できず fail する。 `vs_legacy: must-fail`
- UAC-002: Given browser と Electron-hosted の双方で、同じ表示名を持つ server project と GitHub repository が検索結果にあり、それぞれ別の stable identity を持つ When 利用者が repository row を選択する Then 選択前と選択後の双方で repository source と owner/name が分かり、実行 server 選択へ進んでも選んだ repository identity が残る Counterexample: hover 中だけ source を出す、または repository 選択後に basename だけを示す実装は、非 pointer modality と選択後の識別性を満たさず fail する。 `vs_legacy: must-fail`
- UAC-003: Given browser と Electron-hosted の双方で、owner/name を検索対象とする repository catalog が利用可能である When 利用者が既知の owner/name と、owner/name だけでは一意に識別できない曖昧な手掛かりで repository を探す Then 既知の owner/name では目的の repository を発見して owner/name で識別できる。owner/name が同じまたは曖昧な候補から目的を一意に選べない事例が一つでも観察された場合は owner/name を不十分として検証を invalidated にし、拡張 metadata の判断へ戻す Counterexample: 既知 target を発見できない、または曖昧候補を識別できない結果を記録しただけで「十分」と判定する実装は、明示した invalidation 条件に反して fail する。 `vs_legacy: irrelevant`
- UAC-018: Given browser と Electron-hosted の双方で、選択中 repository が background refresh により利用不能になった When 更新結果が New Session に反映される Then 無効な repository 選択は解除され、理由を示し、query と健康な候補を保持し、focus は該当状態または再選択可能な候補へ移り、別 repository の選択または退出ができる Counterexample: 無効な選択を残す、理由なく消す、または focus を失う実装は fail する。 `vs_legacy: must-fail`

### F-002 GitHub 状態に遮断されず既存 server project session を作成する

Entry/Progress/Exit: [keyboard|pointer|assistive-technology] GitHub が未認証または利用不能な状態で New Session を開く → [keyboard|pointer|assistive-technology] 全接続 server の project を検索し、stable server identity を確認して選択する → [keyboard|pointer|assistive-technology] 既存の command 選択と server-qualified 確認を経て session を作成する

- UAC-004: Given browser と Electron-hosted の双方で、GitHub は未認証または取得失敗だが、複数の接続 server から project catalog を取得できる When 利用者が project を検索して選択し、既存 command 選択と確認を完了する Then GitHub 認証を要求されず、全接続 server の健康な project を検索でき、row から作成結果まで選択した stable server identity が保たれた session が作成される Counterexample: GitHub 未認証を理由に picker 全体を遮断する実装、単一 server の project だけを出す実装、または確認面で server identity を失う実装は fail する。 `vs_legacy: must-pass`
- UAC-019: Given browser と Electron-hosted の双方で、GitHub 認証済みかつ project と repository の候補がある When 利用者が New Session を開いて target を検索する Then 既存 project flow を保ちながら GitHub repositories category も現れ、全 row で source badge と stable identity が常時分かる Counterexample: 旧 project-only catalog、または source を示さない row のままの実装は fail する。 `vs_legacy: must-fail`

### F-003 GitHub 認証と catalog 更新の中断から検索文脈を保って回復する

Entry/Progress/Exit: [keyboard|pointer|assistive-technology] query と全 source filter を使った状態で GitHub category の認証要求または更新状態を確認する → [keyboard|pointer|assistive-technology] Settings の account surface で認証する、または失敗 source を再試行する → [keyboard|pointer|assistive-technology] New Session の同じ query と filter に戻り、GitHub category の入口から操作を再開する

- UAC-005: Given browser と Electron-hosted の双方で、New Session で query を入力し全 source filter を使っているが、GitHub は未認証で project 結果は利用できる When 利用者が GitHub category の認証 entry から Settings の account surface へ進み、認証を完了して New Session へ戻る Then 認証成功後は同じ query と全 source filter、健康な project 結果が復元され、focus は GitHub category の入口へ戻り、repository 選択を開始できる Counterexample: inline 認証、query/filter の reset、または復帰 focus が GitHub category の入口以外になる実装は fail する。 `vs_legacy: must-fail`
- UAC-006: Given browser と Electron-hosted の双方で、保持済み repository 一覧と query が表示されており、background refresh が開始される When refresh が進行した後に失敗する Then 保持済み repository 結果と query は操作可能なまま残り、GitHub category は更新中から stale な状態と失敗理由、再試行を示し、Projects category は影響を受けない Counterexample: refresh 完了まで repository を空にする、失敗時に picker 全体を error にする、または stale であることを示さず保持値を最新として見せる実装は fail する。 `vs_legacy: must-fail`
- UAC-007: Given browser と Electron-hosted の双方で、一つの query に対して Projects と GitHub repositories の結果があり、GitHub source だけが取得失敗する When 利用者が GitHub の再試行を実行するか、健康な project 候補の操作を続ける Then 再試行中、成功、再失敗の各状態で元の query と全 source filter、健康な結果、依然有効な選択が保持され、focus は GitHub の状態または再試行操作に残る。成功時は更新結果、再失敗時は理由と再試行を同じ New Session 内で確認できる Counterexample: 再試行中または結果表示で query、filter、有効な選択、focus のいずれかを失う実装は fail する。 `vs_legacy: must-fail`
- UAC-013: Given browser と Electron-hosted の双方で、query と全 source filter を保持した New Session から Settings の GitHub account 認証へ進んでいる When 認証を拒否、失敗、cancel、または back して New Session へ戻る Then 元の query と filter、健康な project 結果が残り、repository 選択は解除され、focus は GitHub category の認証入口へ戻り、認証再試行、別候補選択、または退出を選べる Counterexample: 認証未完了時に初期状態へ reset する、repository 選択を有効なまま残す、または focus を失う実装は fail する。 `vs_legacy: must-fail`
- UAC-014: Given browser と Electron-hosted の双方で、repository 結果と query が利用できる一方、Projects category だけが取得失敗している When 利用者が repository を選択するか Projects の再試行を行う Then repository 結果、query、全 source filter、有効な選択を保持し、Projects category だけに失敗理由、再試行、退出を示す。repository 選択は継続でき、再試行の進行と結果を同じ文脈で確認できる Counterexample: project 障害で repository 結果または picker 全体を遮断する実装は fail する。 `vs_legacy: must-fail`

### F-004 repository と execution server を選び段階別準備から session を開始する

Entry/Progress/Exit: [keyboard|pointer|assistive-technology] GitHub repository を選択した後に execution server を選択する → [keyboard|pointer|assistive-technology] repository owner/name と stable server identity を確認して作成を開始する → [visual|assistive-technology] session 固有 clone と repository 単位 devcontainer 準備の段階を確認する → [keyboard|pointer|assistive-technology] 成功した session へ進むか、失敗または中止後に対象と段階を保って再試行または退出する

- UAC-008: Given browser と Electron-hosted の双方で、利用者が GitHub repository を選択し、複数の execution server が選択可能である When repository 選択後に server を選び、作成確認へ進む Then 確認面で repository owner/name と stable server identity が同時に分かる。戻るまたは cancel では repository query と選択を保持し、focus は元の repository row に戻る Counterexample: server-first、組の重複表示、identity 省略、または戻る操作で query・選択・focus を失う実装は fail する。 `vs_legacy: must-fail`
- UAC-009: Given browser と Electron-hosted の双方で、repository と execution server を確認して新しい repository session の作成を開始した When session 固有 clone が完了し、repository 単位の per-project devcontainer 準備も完了する Then clone と devcontainer 準備が別段階として順に分かり、両方の完了後にだけ repository session が利用可能として示される Counterexample: 単一の「準備中」だけを示す、clone 完了時点で session を利用可能にする、または devcontainer を per-session mode として見せる実装は fail する。 `vs_legacy: must-fail`
- UAC-010: Given browser と Electron-hosted の双方で、clone が完了し、対象 repository と server を示した devcontainer 準備段階にいる When devcontainer 準備が失敗し、利用者が retry を選ぶ Then clone ではなく devcontainer 準備で失敗したこと、repository、server、失敗段階が retry 面に残り、未完成 session は利用可能に見えず、retry の進行を同じ段階から確認できる Counterexample: 「準備失敗」とだけ示す、retry 時に repository/server を再選択させる、または失敗 session を session 一覧で利用可能にする実装は fail する。 `vs_legacy: must-fail`
- UAC-011: Given browser と Electron-hosted の双方で、repository session の clone または devcontainer 準備が進行中である When 利用者が中止し、その後 retry 面を確認する Then 中止した段階、repository、server が retry 面に残り、部分 session は利用可能に見えない。clone 中止の retry は clone から、devcontainer 中止の retry は clone を再実行せず devcontainer 準備から再開したことが分かり、退出も選べる Counterexample: devcontainer 中止後に clone から再開する、初期 picker に reset する、または部分 session を利用可能に示す実装は fail する。 `vs_legacy: must-fail`
- UAC-012: Given browser と Electron-hosted の双方で、同じ repository の一つ目の session が存在し、利用者が同じ repository と任意の execution server で二つ目を作成する When 二つ目の session 作成を開始して clone 段階を確認する Then 既存 session への切替や duplicate 確認なしに独立 clone が進み、各 session row で repository owner/name と既存の session 識別情報が常時分かり、選択前後で二つを判別して別々に利用できる Counterexample: clone 共有、duplicate blocking、自動切替、または owner/name だけの識別不能な二行を示す実装は fail する。 `vs_legacy: must-fail`
- UAC-015: Given browser と Electron-hosted の双方で、同一 repository の repository 単位 devcontainer が準備済みで、一つ目の session が存在する When 同じ repository の二つ目の session 作成を開始する Then 新しい session 固有 clone の進行と既存 repository devcontainer の利用が別に分かり、新しい devcontainer 準備は開始されず、clone 完了後に二つ目が利用可能になる Counterexample: 二つ目にも別 devcontainer を準備しながら repository 単位と表示する実装は進行観察が異なり fail する。 `vs_legacy: must-fail`
- UAC-016: Given browser と Electron-hosted の双方で、repository と execution server を確認し clone を開始した When clone が失敗し、利用者が retry または退出を選ぶ Then repository、server、失敗段階が clone として示され、未完成 session は利用可能に見えず、retry は clone 段階から始まり、退出もできる Counterexample: clone 失敗を汎用準備失敗とする、devcontainer から retry する、または未完成 session を利用可能にする実装は fail する。 `vs_legacy: must-fail`
- UAC-017: Given browser と Electron-hosted の双方で、repository 選択後に選んだ execution server が利用不能になった When 利用者が server 選択面を確認する Then repository owner/name と query は保持され、利用不能理由が示され、無効な server 選択は解除され、focus は利用不能状態または次の server 候補へ移り、別 server の選択または退出ができる Counterexample: repository 選択を失う、無効 server を残す、または行き止まりにする実装は fail する。 `vs_legacy: must-fail`

## Observable State Contracts

- `state-catalog`: categorized target catalog。states: available → refreshing-retained → authentication-required → source-failed-retained。invariants: 健康な source と query/filter を保持 / source badge と identity を常時提示
- `state-repository-target`: repository and execution server selection。states: repository-unselected → repository-selected → server-confirmed → server-invalid。invariants: repository を server より先に選ぶ / 戻る・server failure でも repository query を保持
- `state-repository-preparation`: repository session availability。states: confirmed → clone-in-progress → devcontainer-in-progress → failed-or-cancelled → available。invariants: 未完成 session は available でない / retry は保持段階から再開 / 各 session clone は独立、devcontainer は repository 単位
- `state-search-validation`: repository search field assumption。states: validation-required → validated → invalidated。invariants: validation 前に extended metadata を確定しない / invalidated は product blocker でなく follow-up decision を再開

## Experience Closure

4 flow、19 scenarios が entry、progress、success/exit、failure、recovery、browser/Electron、keyboard/pointer/支援技術、両 source の部分失敗、認証中断、選択無効化、clone/devcontainer の成功・失敗・中止・retry、複数 session を覆う。観察を変える product choice は解決済みである。DP-REPOSITORY-SEARCH-FIELDS は owner/name の validation_required assumption であり、既知 target 発見または曖昧候補識別に失敗した場合は invalidated として拡張 metadata の判断へ戻す。

## Legacy Traces

- 継承: fleet-wide project discovery、stable server identity、command 選択と確認（UAC-004）。
- 置換: project-only catalog と source 非表示 row（UAC-001、UAC-002、UAC-019 は must-fail）。

## Delivery Slices

- `slice-initial` (initial): categorized discovery、認証・部分失敗 recovery、repository session の段階別準備を end-to-end で提供する。 Flow scope: F-001、F-002、F-003、F-004。 除外: 検証前の extended repository search、source-first/interleaved topology、clone 共有、per-session devcontainer
- `slice-follow-up-search-fields` (follow_up): owner/name validation が invalidated の場合だけ search fields を再決定する。 Flow scope: なし。 除外: 検証なしの extended metadata 採用、topology/ranking の同時 redesign

## Design Handoff

technology candidates と design handoffs は frontmatter と requirements-plan.json に同一 stable ID で投影した非規範入力であり、UX capability の採否を確定しない。

## Open Questions

なし。
