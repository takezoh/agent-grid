---
change: change-20260725-windows-service-wsl-supervision
role: ux
id: ux-20260725-windows-service-wsl-supervision
kind: ux
title: 20260725 Windows Service による WSL Host Server 監視 UX
status: draft
phase: explore
revision: explore-r5
created: '2026-07-25'
change_id: change-20260725-windows-service-wsl-supervision
summary: 'Explore-mode UX plan (revision explore-r5、base explore-r4)。**Specify readiness blocker は 0 件**。plans/windows-service-wsl-host-supervision.md §2-§15 を、Windows Service が WSL 内 Linux host server を supervised unit として起動・readiness 確認・待機付き再起動・graceful 停止・対象 distribution 限定 terminate・記録収集する体験契約へ変換した。対象は host server 自身の Windows 上での起動と監視であり、GUI クライアントは一切登場しない。consultation は 3 回適用済み。(1) explore-r3 / log-destination-and-identity: DP-LOG-DESTINATION = OPT-DEDICATED-LOG-FILE (normative)。派生義務として cc-record-surface-reachable-by-operator (サービスアカウントの profile 配下に閉じない) を要件化し、pa-record-file-survives-supervisor-start-failure と handoff-record-content-and-sink で UAC-007 の成立を担保する (具体 path は design 所有)。同回の DP-SERVICE-IDENTITY-SCOPE への回答は宣言済み option のいずれでもなく軸自体を否定したため丸めずに decision revision とし、確定内容は confirmed constraint cc-configurable-identity-and-distribution として保存した。(2) explore-r4 / default-target-binding-time: DP-SERVICE-IDENTITY-SCOPE = OPT-BIND-AT-REGISTRATION。実行 identity と対象 distribution は登録時点で解決して確定値として記録され、以後 `wsl --set-default` では動かず、terminate の対象も登録時の distro に固定される。帰結は edge-registration-bound-target、handoff-service-account-hardening、UAC-016 の rewrite_in_specify として落とした。(3) explore-r5 / alternatives-demoted-to-deployment: user が『代替案としては廃し、deployment 構成として扱う』と確定。primary_alternative consultation として ALT-SHARED-EXISTING-ACCOUNT を confirmed / approved_by_user へ適用し、alternative_comparison を『同一契約下でどの deployment 構成を既定とするか』の記述へ整理した。ALT-SHARED が primary なのは確定済みの既定 (登録時にログオン中のユーザー / WSL 既定 distro) と一致する既定構成だからであり、体験上の優劣ではない。ALT-DEDICATED-ACCOUNT-DISTRIBUTION は rejected にせず、登録時の明示指定で表現できる deployment 構成として保持し、plan §3 の 5 分離要件と選択条件を rationale に残した。残る未決は design 委譲 2 件 (DP-LAUNCH-INVOCATION-FORM / DP-ALREADY-RUNNING-CLASSIFICATION)、PoC 実測待ち 1 件 (DP-SHUTDOWN-TIMEOUT-PROVENANCE)、Specify 持ち越し 3 件、design_owned の数値のみで、いずれも Specify を止めない。critic pass1 の issue 18 件 (blocker 6 件)、pass2 improvements 18 件、独立検収の差し戻し 3 件は解消済みで本 revision でも維持している。plan §12 の PoC 検証項目 1-11 は 9 flow / 17 scenario と product assumption の validation で全件被覆。plan §13 の数値 (backoff / 各 timeout / port) は repo 内に根拠が無いため design_owned のまま確定せず、停止予算は server 実装の drain 8 秒 + gateway grace 5 秒から再導出する義務 (SCM stop 経路と graceful restart の停止段で同一物)、port は設定由来にする design obligation を付けている。'
technology_candidates:
- id: candidate-golang-windows-svc
  candidate: golang.org/x/sys/windows/svc (Go の Windows Service Control Manager 連携 API)
  source: explicit_user_input
  proposed_for:
  - Windows Service wrapper 本体 (SCM 登録・Start/Stop ハンドラ・状態通知)
  - Session 0 での常駐と Automatic startup
  required_capabilities:
  - SCM への machine-wide 登録と Startup Automatic 設定
  - 指定 Windows ユーザーの security context での実行 (Log On As)
  - SCM からの stop request の受理と、停止完了までの pending 状態通知
  - Session 0 から子プロセスとして wsl.exe を起動し、その終了を検知できること
  ux_constraints_relevant_to_evaluation:
  - 誰もサインインしていない状態で起動が完了し health 確認まで到達できること (goal-connect-unsigned)
  - 停止要求を受けてから Linux 側 teardown の完了を待てること (goal-stop-really-stops / UAC-008)
  - SCM 上は Running のままの Ready / Backoff / Failed を記録面で区別できること (state-supervised-unit-availability)
  design_questions:
  - Session 0 の service context から対象ユーザー所有の WSL distribution を起動できるか (plan §12 検証項目 1・2)
  - SCM の停止待ち予算と threshold-shutdown-wait-budget の関係をどう定めるか
  - service account の password 変更・期限切れ・`Log on as a service` 権限喪失を、起動できない状態としてどう記録面へ出すか (plan §10)
  disqualifiers:
  - Session 0 から対象 distribution を起動できない場合、本構成そのものが成立しない (pa-service-account-can-launch-distribution の反証)
  - SCM の停止待ち予算内に Linux 側 teardown が完了できない場合、UAC-008 を満たせない
- id: candidate-existing-healthz-endpoint
  candidate: server 実装済みの GET /healthz (認証不要、status ok / daemon-unavailable、unhealthy は 503)
  source: repository_context
  proposed_for:
  - 起動シーケンス最終段の readiness 判定
  - 定常 health 確認と、sleep 復帰後の health 再確認
  required_capabilities:
  - 認証なしで到達性を判定できる
  - gateway は生きているが daemon が繋がっていない状態を daemon-unavailable として区別できる
  - 接続先 addr / port を設定由来の単一の値から解決できる
  ux_constraints_relevant_to_evaluation:
  - 『到達できた』と『稼働している』を混同しないこと (UAC-001 の判別性)
  - 復帰後の health 再確認の実行と結果が記録面に現れること (UAC-011)
  design_questions:
  - readiness 到達判定と定常 health 判定で同じ endpoint を使うか、判定の厳しさを変えるか
  - port を host server の -addr 設定とどう同期させるか (plan §13 の 7780 は repo 内に根拠が無く既定 8443 と矛盾する)
  - health 確認 1 回あたりの timeout と、連続失敗の数え方をどう定めるか
  disqualifiers:
  - port を設定由来にできず固定値を新設する場合、cc-health-target-from-configuration に反する
  - daemon-unavailable を成功として扱う判定にした場合、UAC-001 の判別性が失われる
- id: candidate-windows-event-log-sink
  candidate: Windows Event Log を supervisor の記録先にする
  source: explicit_user_input
  proposed_for:
  - 記録先としては不採用 (DP-LOG-DESTINATION は OPT-DEDICATED-LOG-FILE で確定)
  - supervisor プロセスが起動する前に失敗した場合の痕跡経路としてのみ design 評価対象に残す (UAC-007 / pa-record-file-survives-supervisor-start-failure)
  required_capabilities:
  - Session 0 のサービスプロセスから書ける
  - サインインした運用者が OS 標準ツールで後から読める
  - service 自体が起動できなかった場合でも痕跡が残る
  - secret を含めずに構造化して記録できる
  ux_constraints_relevant_to_evaluation:
  - 再試行中と終端 (再試行停止) を記録から区別できること (UAC-004 / UAC-005)
  - 資格情報起因で service が起動できないときにも原因が読めること (UAC-007)
  design_questions:
  - 専用ログファイルを主たる記録面としたうえで、supervisor 起動前の失敗の痕跡だけを OS 側の記録へ残す併用形が成立するか
  - 併用する場合、event source の登録に管理者権限が要る点をインストール手順にどう含めるか
  - 併用しない場合、supervisor 起動前の失敗をどこから読ませるか
  disqualifiers:
  - event source を登録できない環境では記録がまったく残らず goal-readable-record が成立しない
- id: candidate-localappdata-log-file
  candidate: 専用ログファイル (size-based rotation 付き) — DP-LOG-DESTINATION の確定先。plan §9 が例示する %LOCALAPPDATA%\AgentGrid\logs\wsl-host.log は profile 相対であり、Session 0 のサービスアカウントで実行される supervisor が書くと運用者の profile には現れないため、path は design で確定し直す
  source: explicit_user_input
  proposed_for:
  - plan §9 の最低記録項目の出力先 (DP-LOG-DESTINATION = OPT-DEDICATED-LOG-FILE で確定)
  required_capabilities:
  - Session 0 のサービスアカウントから書ける path が解決できる
  - サインインした運用者が同じ場所を読める (サービスアカウントの profile 配下に閉じない / cc-record-surface-reachable-by-operator)
  - size-based rotation
  - 運用者が場所を一意に知れる (登録手順・設定から path が提示される)
  ux_constraints_relevant_to_evaluation:
  - 運用者が『どこを見ればよいか』を一意に知れること (goal-readable-record)
  - サインインしていない時間帯に起きた失敗も後から読めること
  design_questions:
  - サービスアカウントの %LOCALAPPDATA% は運用者がサインインする profile と異なりうるため、profile 相対ではない machine-wide な path をどこに置くか
  - service executable と config の ACL とログファイルの ACL をどう揃えるか (plan §10)
  - supervisor プロセスが起動する前に失敗した場合、ファイルへは書けないため痕跡をどこへ残すか (UAC-007)
  disqualifiers:
  - サービスアカウントの profile 配下に書かれ、運用者が自分の %LOCALAPPDATA% を見ても何も無い状態になる場合、記録面としての目的を果たさない
- id: candidate-toml-supervisor-config
  candidate: supervisor 専用の TOML 設定ファイル (plan §13 の [host] / [restart] 例)
  source: explicit_user_input
  proposed_for:
  - distribution / linux user / server path / health 確認先 / 各予算 / backoff の設定源
  required_capabilities:
  - Session 0 のサービスプロセスから読める path に置ける
  - ACL を設定できる
  - health 確認先を host server の起動設定と同じ正本から解決できる
  ux_constraints_relevant_to_evaluation:
  - distribution / Linux user / 接続先が二重管理されず、設定を 1 箇所直せば supervisor と host server の双方に効くこと
  - port を固定値として埋め込まないこと (cc-health-target-from-configuration)
  design_questions:
  - host server 起動引数 (-addr / -data-dir) と supervisor 設定の重複をどう防ぐか
  - 設定を変更したときに supervisor へ反映されるまでに運用者が何をする必要があるか
  - 設定不正 (invalid configuration) をどの時点で検出し、どう記録面へ出すか
  disqualifiers:
  - host server の起動引数と supervisor の health 確認先がドリフトしうる形になる場合、UAC-001 の稼働判定が構造的に不安定になる
design_handoffs:
- id: handoff-launch-topology
  required_outcome: Linux host server が foreground の supervised unit として動き、その終了が supervised unit の終了に連動し、監督階層に余計な shell process が残らず、Windows 側の停止操作が host server の既存 graceful shutdown 経路へ到達する。どの起動トポロジーを選んでも UAC-003 と UAC-008 の判定基準は変わらない。
  design_obligations:
  - 直接起動と WSL 内 launcher + exec のどちらを採るかを決め、選んだ側で停止要求が Linux 側 host server の既存シグナル経路へ到達することを示す
  - launcher を置く場合は最後を exec にし、監督階層に shell process を残さないことを構造的に保証する (plan §4)
  - supervised unit の終了検知手段 (exit code の取得経路) と、host server の exit code を記録面へ渡す経路を定める
  - stdout / stderr を secret と environment variable の値を含めずに記録面へ渡す経路を定める (plan §9、cc-no-secrets-in-record)
  - 既存 ADR の実測 (起動元がすぐ終了する文脈での detach 生存挙動) を本件の前提として持ち込まず、常駐する親が居続ける構成での挙動を独立に確かめる
  verification_obligations:
  - 'plan §12 検証項目 4: host server を正常終了・異常終了の双方で終わらせ、supervised unit も終了し distribution 内にプロセスが残らないことを実測する'
  - 'plan §12 検証項目 7: SCM stop から host server 自身の drain 完了までが host server 側の記録に現れることを実測する'
  - launcher 経由を採る場合、exec 後に shell process が残らないことを distribution 内のプロセス一覧で確認する
  provenance:
    source: critic_proposal
    confidence: provisional
    evidence_refs:
    - plans/windows-service-wsl-host-supervision.md#4
    - src/cmd/server/coordinator.go:326
    rationale: critic pass2 が『起動形態の選択を confirmed constraint から外し design へ handoff する』ことを求めた。UX 側は結果だけを固定し、トポロジーの選択と検証を design が所有する。
  decision_refs:
  - DP-LAUNCH-INVOCATION-FORM
- id: handoff-already-running-detection
  required_outcome: 既存 instance が稼働しているために起動が拒否された状態が、恒久的失敗としても crash としても分類されず、『既に稼働中』として記録されたうえで health 確認へ進む。健全に稼働している状態が障害として記録されない。
  design_obligations:
  - 起動拒否を識別する信号 (exit code / stderr のパターン) を定め、他の設定エラーと確実に区別できることを示す
  - 識別できなかった場合に安全側 (再試行を続ける) へ倒れるか、危険側 (終端状態へ落ちる) へ倒れるかを明示し、危険側へ倒れない設計にする
  - 『既に稼働中』として記録する内容を定める (どの instance が稼働していると判断したか)
  verification_obligations:
  - 既に稼働している host server がある状態で supervisor に起動を試みさせ、終端状態も失敗原因も記録されず health 確認へ進むことを確認する
  - 起動拒否と、綴り誤りの distribution 名による起動失敗を連続して起こし、両者が別種として記録されることを確認する
  provenance:
    source: repository_evidence
    confidence: provisional
    evidence_refs:
    - src/cmd/server/daemon_lock.go:35
    rationale: 要求される結果は UAC-006 で規範化済みであり、読み分けに使う信号の選択だけが design 所有として残る。
  decision_refs:
  - DP-ALREADY-RUNNING-CLASSIFICATION
- id: handoff-health-target-resolution
  required_outcome: supervisor が health 確認に使う接続先 (addr / port) が、host server の起動設定と同じ単一の正本から解決され、二重管理によるドリフトが『繋がらない』として運用者に現れない。固定 port を新設しない。
  design_obligations:
  - health 確認先を host server 起動引数 (-addr) と同じ正本から解決する経路を定める。plan §13 の 7780 のような repo 既定 (127.0.0.1:8443) と矛盾する固定値を新設しない
  - data dir も同じ正本から解決し、既定の ~/.agent-grid が書けない環境で明示指定できることを保証する (cc-data-dir-explicitly-specifiable)
  - gateway だけが応答し daemon が繋がっていない状態 (daemon-unavailable) を、到達成功として扱わない判定にする
  - health 確認 1 回あたりの timeout と、連続失敗の数え方を明示する
  - 設定を変更したときに supervisor へ反映されるまでに運用者が何をする必要があるかを定める
  verification_obligations:
  - 'plan §12 検証項目 10: 同一マシン上の HTTP クライアントから local endpoint へ実接続し、supervisor の判定と一致することを確認する'
  - gateway は応答するが daemon が繋がっていない状態を作り、supervisor が Ready と判定しないことを確認する
  - host server の -addr を変更し、supervisor の health 確認先が追従することを確認する
  provenance:
    source: repository_evidence
    confidence: provisional
    evidence_refs:
    - src/cmd/server/gateway.go:240
    - scripts/run-dev.sh
    rationale: plan §13 の port には repo 内に根拠が無く、実際の既定と矛盾する。数値を確定せず、設定由来にする義務だけを design へ渡す。
- id: handoff-record-content-and-sink
  required_outcome: 運用者が記録を読むだけで『再試行を待っている』『再試行を止めた (止まった原因つき)』『既に稼働中だった』を区別でき、plan §9 の最低記録項目に到達できる。secret と environment variable の値はどこにも現れない。記録面は専用ログファイル (DP-LOG-DESTINATION = OPT-DEDICATED-LOG-FILE) とし、運用者が到達できる場所に置く (cc-record-surface-reachable-by-operator)。
  design_obligations:
  - plan §9 の最低記録項目 (service start/stop、WSL command line の非秘密部分、distribution / Linux user、child PID、readiness 結果、exit code、restart count、graceful shutdown timeout、force terminate 実行) を記録面へ写す
  - 再試行を待っている間も記録が増え続け、止まった状態と区別できるようにする
  - state-supervised-unit-availability の invariant (Ready / Backoff / Failed は SCM 上では区別できず記録面と health 応答でのみ区別できる) を満たす記録項目を定める
  - supervisor 自身が起動できなかった場合 (資格情報失敗など) にも痕跡が残る経路を定める
  - secret と environment variable の値を記録しない (cc-no-secrets-in-record)
  - 記録面の path を確定する。サービスアカウントの profile 配下に閉じる path (plan §9 の %LOCALAPPDATA%\AgentGrid\logs\wsl-host.log は profile 相対) を採らず、Session 0 のサービスアカウントが書けて、かつサインインした運用者が同じ場所を読める machine-wide な位置を選ぶ (cc-record-surface-reachable-by-operator)
  - 運用者が『どこを見ればよいか』を一意に知れる経路 (登録手順・設定・記録面 path の提示) を定める。path を知らないと読めない記録面は goal-readable-record を満たさない
  - size-based rotation を自前で持つ形を定め、rotation 後も直近の失敗が読めることを保証する
  - 専用ログファイル選択下で UAC-007 を成立させる経路を定める。supervisor プロセスが起動する前に失敗した場合はファイルへ書けないため、OS 側の service 起動失敗記録との併用を含めて『資格情報起因である旨が読める』経路を確保する (pa-record-file-survives-supervisor-start-failure)
  - 記録内容の要求 (plan §9 の最低記録項目) は記録面の実装形に依存しないことを保証する
  verification_obligations:
  - 設定起因の失敗を意図的に起こし、止まった原因の記録から直すべき設定値へ到達できることを確認する
  - 再試行を待っている間に記録が増え続け、止まった状態と区別できることを確認する
  - 記録に token / secret / environment variable の値が含まれないことを確認する
  - supervisor サービス自体が起動できない状態を作り、痕跡が残ることを確認する
  - サインインした運用者の権限で記録面を開き、Session 0 のサービスアカウントが書いた記録がそのまま読めることを確認する (profile 差による空振りが起きないこと)
  - rotation を発生させ、直近の失敗の記録が失われないことを確認する
  provenance:
    source: critic_proposal
    confidence: provisional
    evidence_refs:
    - plans/windows-service-wsl-host-supervision.md#9
    - consultation-20260725-log-destination-and-identity
    rationale: 'critic pass2 が『記録面の legacy delta を独立 scenario で検収可能にする』ことを求めた。記録先の選択は DP-LOG-DESTINATION の product decision、記録内容と実装形は design 所有。 explore-r3: user が OPT-DEDICATED-LOG-FILE を選択したため、記録先は確定し、選択の帰結 (到達可能な path の確定と、supervisor 起動前の失敗に対する痕跡経路) が design obligation と verification obligation としてこの handoff に加わった。'
  decision_refs:
  - DP-LOG-DESTINATION
- id: handoff-service-account-hardening
  required_outcome: service account 側の要因 (password 変更・期限切れ、`Log on as a service` 権限、user profile の読み込み、distribution registration、service executable と config の ACL) で起動できない状態が、原因不明の沈黙にならず記録から辿れる。既定 (登録時にログオンしているユーザー / WSL の既定 distro) のまま登録した運用者にも、この失敗モードが同じ形で観察できる。登録時に解決された実行 identity と対象 distribution は確定値としてサービス設定に記録され、以後 WSL の既定変更に追従しない (DP-SERVICE-IDENTITY-SCOPE = OPT-BIND-AT-REGISTRATION)。
  design_obligations:
  - plan §10 が挙げる各要因について、起動できない状態をどう検出し何を記録するかを定める
  - service executable と config の ACL を定め、記録面の path にも同じ方針を適用する
  - Windows Credential Manager との将来連携を、現時点では credential model を WSL 内に残す前提 (cc-credential-model-stays-in-wsl) と矛盾しない形で保留する
  - 実行 identity を任意の Windows ユーザーで指定できる登録経路と、既定 (登録時にログオンしているユーザー) の解決方法を定める (cc-configurable-identity-and-distribution)
  - 対象 WSL distribution を任意の distro で指定できる経路と、既定 (WSL の既定 distro) の解決方法を定める。既定を採った場合、停止の最終手段である `wsl.exe --terminate <distribution>` の対象が運用者の日常作業 distro と一致しうることを運用手順として明示する
  - 既定がログオン中のユーザーであるため、そのアカウントの password 変更・期限切れ・`Log on as a service` 権限の喪失が可用性へ直結することを運用手順として明示する (plan §10)
  - '登録時点で実行 identity (既定: 登録時にログオンしているユーザー) と対象 distribution (既定: WSL の既定 distro) を解決し、確定値としてサービス設定へ記録する経路を定める。以後の `wsl --set-default` や対話ログオンの変化で対象が動かないことを構造的に保証する'
  - 運用者が登録直後に『どのユーザーとして・どの distro を対象に動くか』を確定値として確認できる経路 (登録結果の提示と記録面への出力) を定める
  - 対象を変更するには再登録 (または設定変更と反映操作) が必要であるという運用コストを手順として明示し、変更前後で何が起きるか (稼働中 unit の停止と再起動) を定める (edge-registration-bound-target)
  verification_obligations:
  - '`Log on as a service` 権限を外した状態で service を起動させ、起動失敗と資格情報起因である旨が記録から読めることを確認する'
  - 'plan §12 検証項目 1: service account の security context から対象 distribution を扱えることを確認する'
  - 実行 identity と対象 distribution を明示指定した登録と、どちらも省略した登録の双方で service を起動し、supervisor が監督している identity / distribution が記録面から読めることを確認する
  - 登録後に `wsl --set-default` で既定 distro を変更し、supervisor の監督対象と `wsl.exe --terminate <distribution>` の対象が登録時の distro のまま動かないことを実測する
  - 対象を変更する再登録手順を実施し、変更後の対象が記録面から読み取れることを確認する
  provenance:
    source: critic_proposal
    confidence: provisional
    evidence_refs:
    - plans/windows-service-wsl-host-supervision.md#10
    - consultation-20260725-log-destination-and-identity
    - consultation-20260725-default-target-binding-time
    rationale: 'critic pass1 が plan §10 の失敗モードがどの flow にも edge case にも写像されていないことを指摘した。UX 側は UAC-007 で『原因不明の沈黙にならない』ことだけを規範化し、検出と ACL は design が所有する。 explore-r3: user が実行 identity と対象 distribution を設定可能とし既定を定めたため (cc-configurable-identity-and-distribution)、共有 / 専用の二者択一は消えた。残るのは既定を採った運用者に何が起きるかであり、その明示がこの handoff の design obligation になった。 explore-r4: user が OPT-BIND-AT-REGISTRATION を選択したため、既定の解決時点が登録時に確定した。plan §10 の資格情報起因の失敗モードは既定が『登録時にログオン中のユーザー』である以上そのまま残るため、この handoff は弱めていない。'
  decision_refs:
  - DP-SERVICE-IDENTITY-SCOPE
---

## Goal

Windows マシン上で Linux host server を、**Windows Service (SCM) を唯一の supervisor として**起動・readiness 確認・異常終了時の待機付き再起動・graceful 停止・最終手段の対象 distribution 限定 terminate・記録収集まで一貫して運用できるようにする。supervisor は対象 WSL distribution を所有する Windows ユーザーの security context で Session 0 に常駐し、その配下で `wsl.exe` を保持して Linux host server を foreground の **supervised unit** として動かす。

本 member は `plans/windows-service-wsl-host-supervision.md` が採用候補とした構成を**所与**とし、その §2〜§15 を観察可能な受け入れ条件へ変換したものである。アーキテクチャ自体は再検討しない。**GUI クライアントは本件の対象外**であり、監督主体・権限・fallback の判断軸には一切登場しない。

これは greenfield ではないが、置き換える legacy は**クライアントではなく「Windows 上で host server を動かす現在の方法」そのもの**である。現状 Windows には host server を監督する常駐主体が存在せず、SCM 登録の実装も repo に無い。したがって現状は「誰かが起動しない限り上がらない / 落ちても誰も起動し直さない / サインインしないと上がらない / 復帰の保証が無い / Windows 側に記録が残らない」であり、各 primary flow はこの現状を fail させる `must-fail` scenario を持つ。

Reference UX の姿勢: `Windows SCM 標準サービスライフサイクル` (Log On As 指定アカウント / Startup Automatic / Session 0) は **modeled_on**。`Windows per-user service template`、`LocalSystem から通常ユーザー所有の distribution を利用する構成`、`wsl.exe --shutdown による停止` はいずれも **rejected** (plan §3・§7 が明示的に退けている)。`WSL 内 systemd unit による autostart (SCM と systemd の二重監督)` は plan §1・§14 の非目的として **scope_rejected**。

**mode: Explore。** 本 member は closure 前の consultation packet であり、未解決の product choice を保持したままである。ATDD-ready な確定契約ではない。

## Target Users

- マシンの所有者 / 運用者 — Windows にサインインしていない状態 (Session 0 のみ) を含む
- host server が起動しない・落ち続けるときに Windows 側の記録から原因を追う運用者
- plan §15 の受け入れ条件として PoC 検証項目 1-11 を実施する開発者 / 運用者

## Confirmed Constraints

| id | 制約 | 出所 |
|----|------|------|
| cc-service-runs-as-distribution-owner | LocalSystem ではなく、対象 WSL distribution を所有する Windows ユーザーの security context で実行する | plan §2, §3 |
| cc-standard-service-session0-automatic | per-user service template ではなく通常の Windows Service を SCM へ machine-wide 登録。Log On As は distribution 所有アカウント、Startup は Automatic、実行は Session 0 | plan §3 |
| cc-explicit-distribution-and-linux-user | 対象 distribution と Linux user は起動引数で明示する。実行時の探索・推測は要件に含めない | plan §2, §4 |
| cc-foreground-supervised-unit | host server は foreground で動き、その終了が supervised unit の終了に連動する。監督階層に余計な shell process を残さない (**起動トポロジーの選択は含まない** — DP-LAUNCH-INVOCATION-FORM へ) | plan §2, §4 |
| cc-supervisor-holds-no-domain-state | supervisor はドメイン状態を持たない。session / frame / driver registry / sandbox / devcontainer / bridge protocol / credential broker policy / Linux process 単位の監督は Linux host server が所有し続け、supervisor から見る Linux host 全体は**一つの supervised unit** | plan §5 |
| cc-sigterm-is-the-existing-stop-path | graceful stop は既存のシグナル経路。**新規の shutdown endpoint / control channel を設けない** | coordinator.go:326, :305 |
| cc-no-wsl-shutdown | `wsl.exe --shutdown` は使用しない。最終手段は対象 distribution だけの terminate | plan §7 |
| cc-no-secrets-in-record | secret と environment variable の値を記録しない。WSL command line は非秘密部分だけ | plan §9 |
| cc-credential-model-stays-in-wsl | credential model は WSL 内に残す。Windows Service は credential broker policy も agent credential も保持しない | plan §10 |
| cc-no-systemd-in-wsl | WSL 内へ systemd を追加せず、SCM との二重監督も作らない | plan §1, §14 |
| cc-health-target-from-configuration | health 確認先の addr / port は設定由来の単一の値から解決する。**plan §13 の 7780 を既定として要件化しない** (repo 既定は 127.0.0.1:8443 で矛盾する) | gateway.go:240, run-dev.sh |
| cc-data-dir-explicitly-specifiable | data dir は明示指定できる。既定の `~/.agent-grid` が書けない環境が実在する | design-host.md:435 |
| cc-non-goals | Windows native port / Windows Container sandbox / Windows process-isolated frame / OS を跨ぐ bridge 接続 / WSL の汎用 daemon manager 化はいずれも対象外 | plan §14 |
| cc-configurable-identity-and-distribution | 実行 identity は**任意の Windows ユーザーを指定でき、既定は登録時にログオンしているユーザー**。対象 distribution も**任意の distro を指定でき、既定は WSL の既定 distro**。専用ユーザーも専用 distribution も必須にしない (explore-r3 / user consultation) | consultation-20260725, plan §2, §3 |
| cc-record-surface-reachable-by-operator | 記録面は**運用者が到達できる場所**に置く。**サービスアカウントの profile 配下に閉じる path を記録面にしない**。具体 path と rotation の実装形は design 所有 (explore-r3 / DP-LOG-DESTINATION 確定の帰結) | consultation-20260725, plan §9 |

## Existing System Context

- **esc-signal-graceful-shutdown** — `coordinator.go:326` で SIGINT/SIGTERM/SIGHUP を Notify。SIGTERM で `EventShutdown` を enqueue → reducer が sandbox cleanup を drain → context cancel → event loop 終了。SIGHUP は記録のみで無視。
- **esc-stop-budget-designed-for-service-manager** — `coordinator.go:305` 付近のコメント: drain timeout 8 秒 / gateway grace 5 秒は「サービスマネージャの停止予算に収まるよう」既に設計済み。**Windows 側の停止予算はこの内訳から導出できる。**
- **esc-healthz-endpoint** — `GET /healthz` は認証不要。`status: ok | daemon-unavailable` を返し unhealthy は 503。gateway だけが listen して daemon が繋がっていない状態を区別できる。
- **esc-flock-double-start-refusal** — data dir の pid file に `flock(LOCK_EX|LOCK_NB)`。失敗時は「already running; refusing to start」で起動拒否し、socket も token file も触らない。プロセス終了で自動解放されるため stale lock は残らない。
- **esc-server-launch-flags** — `-data-dir` / `-addr` / `-token-file` / `-insecure` / `-no-auth`。既定 addr は `127.0.0.1:8443`。
- **esc-no-windows-supervisor-implementation** — SCM 登録実装も `golang.org/x/sys/windows/svc` の利用も repo に無い (ヒットは plan doc のみ)。ただし `golang.org/x/sys` 自体は既に直接依存。
- **esc-dev-run-is-separate-lineage** — `scripts/run-dev.sh` / `make run-dev` は Linux 内の開発用 foreground 起動 (readiness は `server.sock` を最大 5 秒ポーリング)。**Windows Service とは別系統。**
- **esc-wsl-launch-survival-measurement** — wsl.exe 経由の生存挙動には実測があるが、それは「起動元がすぐ終了する」文脈のものであり、常駐する親が居続ける本件を**統治しない**。機構の知見としてのみ扱う。

## Legacy Context

**Source implementation**: Windows 上に host server を監督する常駐主体が存在しない。誰かが手動で `wsl.exe` 経由で起動する必要がある。

**Inherited behaviors**: SIGINT/SIGTERM の graceful shutdown 経路 / drain 8 秒 + gateway grace 5 秒の停止予算 / `GET /healthz` / flock による二重起動拒否 / 既存の起動フラグ。いずれも変更しない。

**Replaced behaviors** (すべて must-fail scenario で追跡):

| 現状の挙動 | must-fail で追跡する scenario |
|---|---|
| 誰かが手動で起動しない限り上がらない | UAC-001, UAC-014 |
| 落ちても誰も起動し直さない | UAC-004, UAC-012, UAC-015 |
| サインインしていない状態では上がらない | UAC-001, UAC-014 |
| sleep 復帰 / WSL subsystem 障害 / update 後に戻る保証が無い | UAC-011, UAC-014, UAC-015 |
| Windows 側に記録が残らない | UAC-016 |

## User Goals

| id | context | desired_outcome | success_observation |
|----|---------|-----------------|---------------------|
| goal-connect-unsigned | Windows 起動直後、誰もサインインしていない (Session 0 のみ) | サインインせずに local endpoint へ到達でき、実際に稼働している | サインイン操作を一度も行わないまま HTTP クライアントが health endpoint から**稼働中を示す成功応答 (status ok)** を受け取る。`daemon-unavailable` は到達成功として数えない |
| goal-recover-or-known-down | 異常終了した後、誰も操作しない | 放置しても復帰するか、復帰しないと分かる | 再試行間隔と連続失敗上限から導出される**最大復旧予算の範囲内**に health が稼働中へ戻るか、予算超過時点で終端状態と原因が記録から読める |
| goal-permanent-failure-not-silent-looped | 設定起因の恒久的な失敗が起きている | 無限リトライで問題が隠れない | 上限到達後は再試行の記録が増えず、止まった原因が読める。**既に稼働中による起動拒否はこの終端判定と混同されない** |
| goal-stop-really-stops | 運用者または SCM が停止操作を行う | 本当に止まり、Linux 側にプロセスが残らない | SCM が Stopped になり、local endpoint が応答せず、対象 distribution 内に host server プロセスも余計な shell process も残らない |
| goal-readable-record | 起動・異常終了・再起動・停止・強制終了の後 | 何が起きたかを後から読める | 確定した記録面で plan §9 の最低記録項目を読める。secret と environment variable の値はどこにも現れない |
| goal-resume-after-disruption | sleep 復帰後 / 再起動なしの WSL subsystem 障害後 / Windows Update・WSL update 後 | 中断前と同様に使える状態へ戻る、戻れないなら失敗として記録される | 何も操作しないまま稼働中へ戻る。その過程で**プロセス生存だけを理由に稼働中と判定しておらず、health 再確認の記録が残っている** |

## Deployment Configurations (旧 UX Alternatives)

> **explore-r5 で確定 (user 承認済み)**: user 回答は逐語で「**代替案としては廃し、deployment 構成として扱う**」。実行 identity と対象 distribution はどちらも設定可能で既定も定まっており (`cc-configurable-identity-and-distribution`)、既定は登録時に確定する (`DP-SERVICE-IDENTITY-SCOPE` = `OPT-BIND-AT-REGISTRATION`)。したがって以下の 2 案は **UX の代替案ではなく、登録時の指定値が違うだけの deployment 構成**であり、flow / scenario / observation を何も変えない。**どちらも削除せず保持する。**

### ALT-SHARED-EXISTING-ACCOUNT (source: draft-1 / **既定 deployment 構成** / approval: **confirmed** / approved_by_user: **true**)

- **primary_entry**: 運用者は既に日常利用している Windows アカウントと、そのアカウントが既に所有している WSL distribution をそのまま指定して supervisor サービスを登録する。専用アカウントも専用 distribution も事前に用意しない。
- **構造次元**: service 実行 identity = 既存 WSL owner アカウント / 対象 distribution = 既存ユーザー所有 / 事前プロビジョニング = 不要 / 資格情報運用 = 日常アカウントと共有
- **要旨**: plan §3 が PoC の出発点として明示的に許可している最小構成。追加手順が無く既存資産をそのまま再利用できる。代償として、資格情報運用・distribution 所有権・filesystem access・専用設定・日常 WSL との干渉がすべて同一アカウントに同居し、**日常アカウントの password 変更や期限切れがそのまま host server の可用性リスクになる** (plan §10)。
- **なぜ既定か**: 確定済みの既定 (**登録時にログオンしているユーザー** / **WSL の既定 distro**) と一致する構成だから。**体験上優れているからではない。**

### ALT-DEDICATED-ACCOUNT-DISTRIBUTION (source: draft-2 / **既定ではない deployment 構成** / disposition: **deferred — rejected ではない**)

- **primary_entry**: 専用 Windows アカウントと専用 WSL distribution を事前に作成し、Log On As に設定してから supervisor サービスを登録する。
- **構造次元**: service 実行 identity = 専用アカウント / 対象 distribution = 専用 / 事前プロビジョニング = 必要 / 資格情報運用 = 日常アカウントから独立
- **要旨**: plan §3 が「分離したい場合」の選択肢として挙げる構成。5 つの分離要件を満たし、対象 distribution 限定の強制終了が日常作業を巻き込む余地も構造的に消える。代償は事前プロビジョニング手順。
- **どう選ぶか**: 登録時に実行 identity と対象 distribution を**明示指定するだけ**でこの構成になる。plan §3 の 5 分離要件 (資格情報運用 / distribution 所有権 / filesystem access / Agent Grid 専用設定 / 日常利用 WSL との干渉) のいずれかが具体化した場合、または無人運用で日常アカウントの資格情報変化を可用性から切り離したい場合に選ぶ。運用手順は `handoff-service-account-hardening` が所有する。

## Deployment Configuration Decision (旧 Alternative Comparison)

既定構成: **ALT-SHARED-EXISTING-ACCOUNT** / approval: **confirmed** / approved_by_user: **true** (consultation-20260725-alternatives-demoted-to-deployment)

**この節は体験契約の優劣比較ではなく、「同一契約下でどの deployment 構成を既定とするか」の記述である。**

- **ALT-SHARED-EXISTING-ACCOUNT (既定)**: 既定である理由は、確定済みの既定 (`OPT-BIND-AT-REGISTRATION` により登録時に解決される「ログオンしているユーザー」と「WSL の既定 distro」) と一致する構成だから、**その一点に限る**。運用上の代償は消えていない — plan §3 の 5 分離要件がすべて同一アカウントに同居し、そのアカウントの password 変更・期限切れ・`Log on as a service` 権限の喪失が可用性へ直結する (plan §10)。これは `edge-service-account-credentials` / UAC-007 / `handoff-service-account-hardening` が持つ。
- **ALT-DEDICATED-ACCOUNT-DISTRIBUTION (既定ではない / deferred)**: **rejected にしていない。** user は ALT-SHARED を選び ALT-DEDICATED を退けたのではなく、**代替軸そのものを廃止**した。この構成は登録時の明示指定だけで表現でき、体験契約を何も変えない。選択条件 (plan §3 の 5 分離要件 / 無人運用での資格情報分離) と、`pa-dedicated-account-not-required` が PoC で反証された場合にこの構成が推奨既定へ格上げされる関係は保持している。
- **draft 統合時の扱い**: draft-1 の `alt-shared-account` と draft-2 の `ALT-poc-shared-eventlog` は同一の構造軸を持つため 1 案に保存した。draft-2 が同案に束ねていた**記録先の次元は DP-LOG-DESTINATION として切り離し**、構成の選択が記録先を暗黙に決めてしまわないようにした。

## Primary Flows

各 scenario には「最も pass に近い誤実装」と、それが fail する論証を counterexample として付けている。

### F-001 サインインなしでの自動起動と到達可能性 (goal-connect-unsigned / exp-headless-availability)

steps: `[system]` Windows 起動・誰もサインインしない → `[system]` SCM が対象アカウントの security context で supervisor を Automatic 起動 → `[system]` **起動引数で明示された** distribution と Linux user で supervised unit を起動 → `[system]` host server が foreground で起動し listen socket が ready → `[system]` health 確認に成功し稼働中を記録 → `[operator-cli]` 同一マシン上の HTTP クライアントから local endpoint を呼ぶ

**UAC-001** (PoC 検証項目 2・10) — vs_legacy: must-fail
- **Given**: Windows が起動しており、どの Windows ユーザーもサインインしていない (Session 0 のみ)。supervisor サービスは Automatic 起動に設定されており、まだ稼働していない
- **When**: SCM が Automatic 起動設定に従い supervisor サービスを起動する
- **Then**: 運用者がサインイン操作を一度も行わないまま、同一マシン上の HTTP クライアントから health endpoint を呼ぶと**稼働中を示す成功応答 (status ok)** が返る。gateway だけが応答し daemon が繋がっていない状態 (`daemon-unavailable`) は到達成功として数えない
- **観察**: 電源投入以外の操作なしで成功応答を受け取る / 記録に supervised unit 起動・listen socket ready・health 確認成功が順に現れ、**health 確認の成功をもって初めて稼働中が記録されている**
- **Counterexample**: supervised unit の起動 (spawn) 成功時点で稼働中を宣言し health 確認を行わない supervisor。gateway だけが listen して daemon 起動に失敗した状態では `daemon-unavailable` を返し続けるため、Then の「稼働中を示す成功応答」が得られず fail する。

**UAC-002** (PoC 検証項目 3・10) — vs_legacy: must-fail
- **Given**: supervisor が稼働中を記録し、health endpoint が稼働中を示す成功応答を返している。誰もサインインしていない
- **When**: 運用者が何の操作もせずに時間を経過させ、その間 supervisor サービスは稼働し続ける
- **Then**: host server は supervised unit として稼働し続け、いつ health endpoint を呼んでも稼働中の成功応答が返る。この間、**記録に再起動 (再試行回数の増加) は一度も現れない**
- **観察**: 繰り返し呼んでも毎回成功応答 / 運用者は再起動も再接続もしていない / 再試行回数の増加が一度も現れない
- **Counterexample**: supervised unit を background へ回して起動し、直後に戻った起動プロセスの exit を「異常終了」と読んで再起動ループへ入る supervisor。host server 自身の二重起動防止により endpoint の応答自体は続くが、記録に再試行回数の増加が繰り返し現れるため Then を fail する。

### F-002 host server の終了と supervised unit の終了連動 (goal-stop-really-stops / exp-supervised-unit-lifecycle)

steps: `[system]` supervisor が稼働中を記録し supervised unit が管理下で生存 → `[system]` host server が正常/異常終了 → `[system]` supervised unit が連動して終了 → `[system]` exit code とともに記録

**UAC-003** (PoC 検証項目 4) — vs_legacy: must-fail
- **Given**: supervisor が稼働中を記録し、health endpoint が稼働中の成功応答を返しており、supervised unit が supervisor の管理下で生存している
- **When**: Linux 側の host server プロセスが正常終了または異常終了のいずれかの経路で exit する
- **Then**: supervised unit も終了し、対象 distribution 内に host server プロセスも**監督階層の余計な shell process も残らない**。supervisor はこの終了を exit code とともに記録する
- **観察**: distribution 内に host server プロセスが残っていない / 起動時に経由した shell や launcher に相当するプロセスが残っていない / 記録に終了と exit code が現れる
- **Counterexample**: WSL 内 launcher を最後に `exec` で置き換えず shell を親として残したまま host server を起動する構成。host server が exit しても親の shell process が残り supervised unit も終了しないため fail する。

### F-003 異常終了の検知・待機付き再起動・恒久的失敗の終端 (goal-recover-or-known-down, goal-permanent-failure-not-silent-looped / exp-failure-recovery)

steps: `[system]` supervised unit の異常終了または起動失敗 → `[system]` 失敗を検知し待機したうえで再起動を試みる → `[system]` **設定起因**の恒久的な失敗が `threshold-max-consecutive-failures` まで連続したら再試行を止め終端状態と原因を記録 → `[system]` **一過性**の失敗は同じ回数では打ち切らず `threshold-transient-failure-terminal-limit` まで待機付きの再試行を続ける → `[operator-cli]` 運用者が記録を読み、復帰待ちか止まったかを判断

**UAC-004** (PoC 検証項目 5・6) — vs_legacy: must-fail
- **Given**: supervisor が稼働中を記録し、health endpoint が稼働中の成功応答を返している
- **When**: host server が一過性の異常終了をする (プロセスの異常終了、または health 確認の連続失敗)
- **Then**: supervisor は待機したうえで再起動を試み、記録に**待機中であることと再試行回数の増加**が現れる。再起動後に health 確認が成功すれば local endpoint は再び稼働中の成功応答を返す
- **観察**: 何も操作しないまま待機中と再試行回数の増加が順に現れる / 最大復旧予算の範囲内に稼働中へ戻る
- **Counterexample**: 異常終了の原因を一切分類せず、**一過性の異常終了を検知しても待機も再起動もまったく行わずに直ちに終端状態を記録して停止する** supervisor。Then が要求する「待機したうえで再起動を試み、再試行回数が増える」が検知直後に破られるため、**この scenario 単独で** fail する (判別性を他 scenario へ委譲しない)。

**UAC-005** — vs_legacy: must-fail
- **Given**: supervisor の設定が指す distribution が対象マシンに登録されていない (または server 実行ファイルの path が存在しない / 権限が拒否される / 設定が不正である)。supervisor は起動を試みるたびに同一原因で失敗しており、**この時点で失敗原因の分類は何も行われていない**
- **When**: 同一原因の失敗が `threshold-max-consecutive-failures` (design が所有する、**設定起因の恒久的な失敗にだけ適用される**終端上限) で定めた連続回数に達する
- **Then**: supervisor は再試行を止めて終端状態を記録し、記録には**止まった原因 (どの設定値がどう解決できなかったか)** が現れる。以後、再試行を示す記録は増えない
- **観察**: 再試行を止めたことと原因が現れる / その後いくら待っても再試行の記録が増えない / 運用者は原因から直すべき設定値へ到達できる
- **Counterexample**: exit code も stderr も見ず、すべての失敗を一過性のものとして待機ループへ入れる supervisor。distribution 名を綴り誤った設定では上限なく再試行を続けるため fail する。**Given は分類前の生の事象で書かれているため、分類できないこの誤実装も Given に到達し vacuous pass にならない。**

**UAC-006** — vs_legacy: must-fail
- **Given**: 対象 distribution 内で host server が既に稼働しており、health endpoint が稼働中の成功応答を返している
- **When**: supervisor がもう一度 host server を起動しようとし、host server 自身の二重起動防止によって後発の起動要求が拒否される
- **Then**: supervisor はこの拒否を「再試行を止めるべき恒久的失敗」として扱わず、**既に稼働中である旨を記録して稼働中の判定へ進む**。終端状態も失敗原因も記録されない
- **観察**: 「既に稼働中」が現れ終端状態も失敗原因も記録されない / health endpoint は成功応答を返し続ける
- **Counterexample**: 起動拒否を他の設定エラーと同列に扱い、連続失敗の上限に達した時点で終端状態と失敗原因を記録して停止する supervisor。実際には健全に稼働しているのに Then を破るため fail する。運用者には存在しない障害の記録だけが残る。

**UAC-007** (plan §10) — vs_legacy: must-fail
- **Given**: supervisor サービスの Log On As に指定した Windows アカウントの資格情報が失効している (password の変更・期限切れ、`Log on as a service` 権限の欠如、user profile の読み込み失敗、または service executable と config の ACL による拒否)
- **When**: Windows 起動後に SCM が supervisor サービスを起動しようとする
- **Then**: 運用者は local endpoint へ到達できず、かつ「サービスが起動できなかったこと」と「**資格情報側に原因があること**」を Windows 側の記録から読み取れる。原因不明の沈黙にはならない
- **観察**: local endpoint が応答を得られない / 記録に起動失敗と資格情報起因である旨が現れる / 直すべき対象へ到達できる
- **Counterexample**: 記録の出力を supervisor プロセス自身の起動後にしか持たない構成。資格情報失敗ではプロセスが起動しないため記録がどこにも現れず、運用者から見ると endpoint が応答しないだけで原因が辿れないため fail する。

**UAC-017** (plan §8 の permanent failure と crash の区別) — vs_legacy: must-fail
- **Given**: supervisor の設定は正しく、起動と health 確認に一度は成功している。その稼働中に、**設定に起因しない一過性の失敗** (host server の crash、または WSL subsystem の一時的な利用不可) が繰り返し発生している
- **When**: 一過性の失敗が、設定起因の恒久的な失敗を終端へ落とす回数 (`threshold-max-consecutive-failures`) と**同じ回数**だけ連続して発生し、そのあと失敗の原因が解消する
- **Then**: supervisor はこの時点で**再試行を止めておらず**、原因の解消後の再起動で local endpoint が稼働中の成功応答へ戻る。記録に終端状態は現れない。すなわち**一過性の失敗は設定起因の失敗と同じ回数では打ち切られない**
- **観察**: 設定起因なら止まっているはずの回数を超えても記録に再試行の継続が現れ終端状態は現れない / 原因解消後、何も操作しないまま稼働中へ戻る
- **Counterexample**: exit code も stderr も一切見ず、**あらゆる失敗を一律に `threshold-max-consecutive-failures` 回で打ち切る無分類の** supervisor。一時的に落ちただけの host server も設定ミスと同じ速さで見捨てるため、原因が解消しているのに既に終端状態へ落ちており Then の双方を fail する。**この誤実装は UAC-004 (上限未満の再試行で足りる)・UAC-005 (停止を要求するだけ)・UAC-015 (上限到達なら終端でよい) をいずれも pass してしまうため、permanent と crash の区別はこの scenario だけが判別する。**

### F-004 SCM stop からの graceful shutdown (goal-stop-really-stops / exp-supervised-unit-lifecycle)

steps: `[operator-cli]` SCM へ stop request → `[system]` supervisor が**既存 graceful shutdown 経路**を通して停止要求を送る → `[system]` host server が新規 frame 受付を止め既存 teardown contract に従って終了 → `[system]` supervisor が supervised unit の終了を待つ → `[system]` SCM が Stopped になる

**UAC-008** (PoC 検証項目 7) — vs_legacy: must-fail
- **Given**: supervisor が稼働中を記録し、health endpoint が稼働中の成功応答を返しており、実行中の frame が存在しうる
- **When**: SCM から stop request が発行される
- **Then**: supervisor は supervised unit を即座に強制終了せず、host server の既存 graceful shutdown 経路を通して停止させる。host server は新規 frame の受付を止め既存 teardown contract に従って終了する。supervisor はその終了を待ち、最終的に SCM が Stopped になり local endpoint は応答しなくなる
- **観察**: SCM が Stopped になり local endpoint が応答しない / **host server 自身が対象 distribution 内に書く記録に、停止要求の受領から sandbox cleanup の drain 完了までの graceful shutdown の完了が現れる** / Windows 側の記録に停止要求送出・supervised unit 終了・停止完了が順に現れる
- **Counterexample**: SCM stop を受けて Windows 側から supervised unit を即時に強制終了し、その戻りをもって「停止要求送信」「host server 終了」「終了待ち完了」の 3 行を Windows 側の記録へ書く supervisor。SCM は Stopped になり Windows 側の 3 段の記録も残るが、**Linux 側 host server は停止シグナルを受け取らず drain を実行しないまま消えるため、host server 自身が書く drain 完了の記録が存在せず** fail する。

### F-005 停止予算超過時の対象 distribution 限定 terminate (goal-stop-really-stops / exp-supervised-unit-lifecycle)

steps: `[system]` graceful shutdown の待機が停止予算に達する → `[system]` supervised unit の強制終了を試みる → `[system]` 残る場合は**対象 distribution だけ**を terminate → `[system]` 用いた手段を記録

**UAC-009** (PoC 検証項目 8) — vs_legacy: must-fail
- **Given**: 対象マシンには対象 distribution のほかに**少なくとも 1 個の WSL distribution が登録され、そちらでも運用者の作業が動いている**。supervisor は graceful shutdown を試みたが停止予算を過ぎても supervised unit が終了しない
- **When**: supervisor が最終手段の強制終了へ進む
- **Then**: 対象 distribution だけが停止し、他の distribution で動いていた運用者の作業は停止しない。記録に停止予算の超過と強制終了の実行が現れる
- **Counterexample**: 停止予算の超過時に全 WSL distribution を停止する手段を呼ぶ supervisor。他の distribution の作業が巻き添えで停止するため fail する。

**UAC-010** (単一 distribution 環境での判別) — vs_legacy: must-fail
- **Given**: 対象マシンには**対象 distribution しか登録されていない** (plan §3 が出発点とする最小構成)。supervisor は graceful shutdown を試みたが停止予算を過ぎても supervised unit が終了しない
- **When**: supervisor が最終手段の強制終了へ進む
- **Then**: 記録には**対象 distribution を名指しした terminate の実行だけ**が現れ、全 distribution を停止する手段を用いた記録は現れない
- **Counterexample**: 単一 distribution 環境では巻き添えの観察ができないことを利用し、常に全 distribution を停止する手段を呼ぶ supervisor。対象を名指しした terminate の記録が残らず全停止手段の形跡が現れるため fail する。**UAC-009 が空振りする環境でも、この scenario が記録面から判別する。**

### F-006 sleep / hibernate 復帰後の health 再検証と段階的な回復 (goal-resume-after-disruption / exp-failure-recovery)

steps: `[system]` sleep / hibernate から復帰 → `[system]` **プロセスの生存確認だけでなく health 確認を再実行**し実行と結果を記録 → `[system]` 失敗が続けば supervised unit を graceful restart → `[system]` それでも続けば対象 distribution だけを terminate してから起動をやり直す

**UAC-011** (PoC 検証項目 9) — vs_legacy: must-fail
- **Given**: supervisor が稼働中を記録した状態でマシンが sleep / hibernate に入り、復帰した。supervised unit のプロセスは生存している
- **When**: マシンが復帰する
- **Then**: supervisor は「プロセスが生きている」ことだけを理由に稼働中の判定を保持せず、復帰の検知から `threshold-resume-health-recheck-budget` (design が所有する上限) の範囲内に health 確認を再実行し、**その実行時刻と結果を記録する**
- **観察**: 復帰後の記録に health 確認の再実行とその結果が現れる / 再実行が記録に現れないまま稼働中の記録だけが続くことはない
- **Counterexample**: 復帰を検知せず、プロセスが生存していることだけを理由に稼働中の判定を保持し続ける supervisor。sleep 復帰後に localhost forwarding が OS 側で遅れて再確立されれば health は自然に通るようになるが、**復帰後の記録に health 確認の再実行が一度も現れないため** fail する。localhost forwarding が戻らない環境では稼働中のまま放置される。

**UAC-012** — vs_legacy: must-fail
- **Given**: sleep / hibernate からの復帰後、supervisor が再実行した health 確認が**失敗し始めており**、supervised unit のプロセスは生存したままである。**この時点で supervisor はまだ再起動も終端も行っていない**
- **When**: health 確認の連続失敗が `threshold-health-recheck-failures-before-restart` (design が所有する、**再起動を駆動するための**閾値) に達する
- **Then**: supervisor は supervised unit を graceful restart し、記録に restart の実行が現れる。**この時点では終端状態へは遷移しない** (終端は設定起因なら `threshold-max-consecutive-failures`、一過性なら `threshold-transient-failure-terminal-limit` が別に駆動する)。restart 後に health 確認が成功すれば local endpoint は再び稼働中の成功応答を返す
- **Counterexample**: health 確認の失敗を記録するだけで再起動へ進まない supervisor。プロセスは生存しているため異常終了としても検知されず、記録に graceful restart の実行が現れないため fail する。運用者から見ると到達できないまま永久に放置される。**なお、health 連続失敗をそのまま終端状態への遷移に使う実装も Then (この時点では終端へ遷移しない) を破って fail する。**

**UAC-013** (plan §11) — vs_legacy: must-fail
- **Given**: graceful restart を実行したあとも health 確認の失敗が続いている
- **When**: graceful restart による supervised unit の停止が、**UAC-009 / UAC-010 と同一の `threshold-shutdown-wait-budget`** (SCM stop 経路と同じ停止予算。graceful restart の停止段も supervised unit の停止であるため**別予算を新設しない**) を過ぎても完了せず、health 確認の失敗も続いている
- **Then**: supervisor は**対象 distribution だけを terminate してから起動をやり直す**。記録に対象 distribution を名指しした terminate の実行が現れ、全 distribution を停止する手段は用いられない
- **Counterexample**: graceful restart までしか経路を持たず、それでも回復しない場合は health 失敗の記録を出し続けて待ち続ける supervisor。記録に terminate の実行が現れないため fail する。plan §11 が最終段として定めた distribution terminate に到達しない。

### F-007 マシン再起動を伴う Windows Update / WSL update 後の復帰 (goal-resume-after-disruption / exp-headless-availability)

steps: `[system]` update 適用でマシン再起動 → `[system]` SCM が Automatic 起動で supervisor を再度起動 → `[system]` 起動シーケンスを最初からやり直し health 確認成功まで到達 → `[operator-cli]` 運用者がサインインせずに local endpoint を呼ぶ

**UAC-014** (PoC 検証項目 11) — vs_legacy: must-fail
- **Given**: Windows Update または WSL update が適用されてマシンが再起動し、誰もサインインしていない
- **When**: SCM が Automatic 起動設定に従い supervisor サービスを起動する
- **Then**: supervisor は起動シーケンス (supervised unit の起動 → host server の起動 → listen socket の ready → health 確認の成功) を最初からやり直し、**health 確認が成功した時点で初めて稼働中を記録する**。運用者はサインインせずに稼働中の成功応答を得られる
- **観察**: サインイン操作なしで稼働中の成功応答が返る / 記録に起動シーケンスの各段の結果が順に現れる
- **Counterexample**: 再起動後に supervised unit の起動成功だけで稼働中を宣言し、**listen socket の ready と health 確認の成功という後半 2 段を確認しないまま**稼働中を記録する supervisor。host server の初期化が終わる前に稼働中と記録されるため、記録に後半 2 段の結果が現れず fail する。運用者は稼働中と記録されているのに接続できない時間帯を踏む。

### F-008 マシン再起動を伴わない WSL subsystem 障害からの復帰 (goal-resume-after-disruption / exp-failure-recovery)

steps: `[system]` **マシン再起動を伴わずに** WSL subsystem が一時的に利用不可になる → `[system]` supervised unit が失われ health 確認が失敗 → `[system]` 待機付きで再起動を試み続ける → `[system]` subsystem 復帰で起動成功、あるいは上限到達で終端状態を記録

**UAC-015** (plan §11) — vs_legacy: must-fail
- **Given**: supervisor が稼働中を記録して動いており、マシンは再起動していない
- **When**: WSL subsystem が (WSL update や subsystem restart により) 一時的に利用不可になり、supervised unit が失われる
- **Then**: supervisor は**マシン再起動という契機を待たずに** health 確認の失敗を検知し、待機付きの再起動を試みる。subsystem が再び利用可能になった時点で local endpoint は稼働中の成功応答へ戻る。利用不可が続く場合に終端状態と原因が記録されるのは**一過性の失敗の上限 `threshold-transient-failure-terminal-limit` に達したとき**であり、設定起因の上限 (`threshold-max-consecutive-failures`) では打ち切られない (plan §8, §11)
- **Counterexample**: 復帰の契機をマシン再起動時のサービス起動だけに持つ supervisor。マシンが再起動しない WSL subsystem 障害では復帰の契機が一度も訪れず、local endpoint は応答しないまま放置され記録にも再試行が現れないため fail する。

### F-009 Windows 側の記録から何が起きたかを読む (goal-readable-record / exp-operational-record)

steps: `[system]` service start/stop・異常終了・再起動・graceful stop・強制終了のいずれかが起きる → `[system]` plan §9 の最低記録項目を記録面へ書く → `[operator-cli]` 運用者があとから記録面を開く

**UAC-016** (plan §9) — vs_legacy: must-fail
- **Given**: supervisor の稼働中に service の start / stop、supervised unit の異常終了、再起動、graceful stop、強制終了のいずれかが起きている
- **When**: 運用者があとから supervisor の記録面を開く
- **Then**: 運用者は service の start / stop、対象 distribution と Linux user、supervised unit の識別子、health 確認の結果、exit code、再試行回数、停止予算の超過、強制終了の実行を読み取れる。**secret と environment variable の値はどこにも現れない**
- **観察**: 最低記録項目が読み取れる形で現れる / token・secret・environment variable の値が現れず WSL command line も非秘密部分だけ / **再試行を待っている状態と再試行を止めた状態を区別できる**
- **Counterexample**: supervised unit の stdout / stderr を読み捨て、Windows 側にはサービスの起動と停止だけを残す supervisor。異常終了の exit code、再試行回数、停止予算の超過、強制終了の実行が記録面に現れないため fail する。**これは現状 (Windows 側に記録が残らない) と同じ観察であり、legacy delta が検収されない。**

## Observable State (state-supervised-unit-availability)

plan §6 の 6 状態が**どの観察面に現れるか**を固定する。状態名そのものは内部語彙であり、運用者は次の 3 面でしか区別できない。

| 状態 | SCM 上の見え方 | health endpoint | 記録面 |
|------|---------------|-----------------|--------|
| Stopped | Stopped | 応答なし | 停止完了 |
| Starting | 起動保留 | 未成功 | 起動シーケンスの各段 |
| Ready | Running | **稼働中を示す成功応答** | 稼働中 |
| Backoff | Running (区別されない) | 成功応答を返さない | **再試行回数が増え続ける** |
| Failed | Running (区別されない) | 成功応答を返さない | **止まった原因が記録され、以後の再試行が増えない** |
| Stopping | 停止保留 | 応答が止まる | 停止手順の各段 |

**遷移を駆動する閾値は二段に分かれる** (同一の値へ潰さない):

| 遷移 | 駆動する閾値 |
|------|-------------|
| Ready → Backoff (supervised unit の異常終了) | 即時 (閾値なし) |
| Ready → Backoff (プロセスは生存したまま health 確認が連続失敗 → graceful restart) | `threshold-health-recheck-failures-before-restart` |
| Backoff → Ready | 待機後の再起動で health 確認が成功 |
| Backoff → Failed (**設定起因**の恒久的な失敗) | `threshold-max-consecutive-failures` |
| Backoff → Failed (**一過性**の失敗) | `threshold-transient-failure-terminal-limit` (設定起因の上限より**厳密に大きい**) |

**Invariants**:
- SCM 上は Ready / Backoff / Failed のいずれも Running に見える。区別は**記録面と health 応答でのみ**観察できる。
- Ready の間だけ health endpoint が稼働中の成功応答を返す。
- Backoff と Failed は記録面で区別できる (再試行が増え続けるか、止まった原因が記録され増えないか)。
- Failed からは、運用者が記録に現れた原因を直して再起動するまで自動では Ready へ戻らない。
- **既存 instance が稼働しているために起動が拒否された状態は Backoff にも Failed にも入らず、Ready の判定へ進む。**
- **再起動試行へ向かう遷移と終端へ向かう遷移は別の閾値で駆動される二段である。** health 確認の連続失敗は再起動閾値に達した時点でまず graceful restart を駆動し、その時点では Failed へ遷移しない (UAC-012)。
- **一過性の失敗は、設定起因の恒久的な失敗と同じ回数では Failed へ遷移しない** (UAC-017)。

## Edge Cases

| id | 条件 | 期待される体験 | scenario |
|----|------|---------------|----------|
| edge-permanent-failure-vs-crash | 恒久的な失敗と一過性の失敗が同じ「起動できなかった」として現れる | 恒久的な失敗は設定起因の上限で止まり原因が読める。一過性の失敗は**同じ回数では打ち切られず**待機付きで再試行され続け、原因が解消すれば復帰する。設定起因が無限に再試行されることも、一過性が設定起因と同じ短さで見捨てられることもない | UAC-004, UAC-005, UAC-017 |
| edge-already-running-refusal | 既に稼働している host server がいる状態で起動を試み、二重起動防止で拒否される | 健全に稼働している状態が障害として記録されない | UAC-006 |
| edge-service-account-credentials | password の変更・期限切れ / `Log on as a service` 権限欠如 / profile 読み込み失敗 / ACL 拒否 | 到達できない状態になるが原因不明の沈黙にはならず、資格情報起因であることが記録から読める | UAC-007 |
| edge-wsl-subsystem-unavailable-without-reboot | マシン再起動を伴わずに WSL subsystem が利用不可になる | 再起動という契機が来なくても検知され、待機付きの再起動が試みられる | UAC-015 |
| edge-forwarding-recovered-but-health-fails | localhost forwarding は再確立したが host server 自体は health 失敗のまま | プロセス生存を理由に稼働中と判定され続けず、graceful restart → 対象 distribution terminate へ進む | UAC-012, UAC-013 |
| edge-force-terminate-still-leaves-processes | 対象 distribution 限定 terminate 後もプロセスが残って見える | 強制終了を行ったこと自体が記録に現れ、他の distribution は停止しない | UAC-009, UAC-010 |
| edge-registration-bound-target | 登録後に `wsl --set-default` で既定 distro が変わる / 監督させたい distro・実行ユーザーを変えたくなる | 監督対象と terminate の対象は**登録時に確定した値のまま動かない**。既定の変更が対象を勝手に変えず、日常作業 distro が巻き添えで停止しない。対象を変えるには**再登録 (または設定変更と反映操作)** が要り、その手順と「いま何を監督しているか」は登録内容と記録面から読める | UAC-009, UAC-016 |

## Product Assumptions (validation required)

- **pa-service-account-can-launch-distribution** — Session 0 の指定ユーザー context から、そのユーザー所有の distribution を起動でき、サインインしていない状態でも readiness へ到達する。
  - validation: **PoC 検証項目 1** (service account から `wsl.exe -l -q` で対象 distribution が見える) と**検証項目 2** (未ログインでの自動起動) を最小 Windows Service で実施し pass/fail と観測値を記録する
  - invalidation: 成立しなければ本構成そのものが成立せず、plan §15 の受け入れ条件が満たされなかったものとして採用判断を差し戻し、goal-connect-unsigned と F-001 / F-007 を取り下げる
- **pa-stop-signal-reaches-linux-process** — Windows Service から発した停止操作が、選択した起動トポロジーを通って Linux 側の既存 graceful shutdown 経路 (drain 8 秒 / gateway grace 5 秒) へ到達する。
  - validation: **PoC 検証項目 7**。host server 自身の記録に停止要求受領から drain 完了までが現れることを確認する
  - invalidation: 到達しなければ DP-LAUNCH-INVOCATION-FORM を到達可能な側へ固定し、それでも駄目なら cc-sigterm-is-the-existing-stop-path を開き直して伝達手段を design の必須課題として handoff する
- **pa-host-exit-propagates-to-unit-exit** — host server の終了で supervised unit も終了し、distribution 内にプロセスが残らない。
  - validation: **PoC 検証項目 4** を正常終了・異常終了の双方で実施する / invalidation: UAC-003 を満たせないものとして扱い、DP-LAUNCH-INVOCATION-FORM の disqualifier へ差し戻す
- **pa-dedicated-account-not-required** — 専用アカウントと専用 distribution は必須ではなく、**既定 (登録時にログオン中のユーザー / WSL 既定 distro) のまま**既存ユーザー所有 distribution を対象にする deployment 構成で成立する。
  - validation: 既定のまま登録し (登録時にログオン中のユーザー / 登録時点の WSL 既定 distro)、**その登録時に確定した distro を対象に** PoC を実施して、**PoC 検証項目 8** の対象 distribution 限定 terminate が日常作業を巻き込まないことを観測する。**確定した distro が日常作業の distro と一致する環境で観測する**
  - invalidation: 巻き込む場合は専用アカウント + 専用 distribution 構成を**推奨既定として運用手順へ格上げ**し `handoff-service-account-hardening` の design obligation とする。`cc-configurable-identity-and-distribution` 自体は変わらない (どちらも設定で表現できる)
- **pa-record-file-survives-supervisor-start-failure** — 記録面を専用ログファイルにした構成でも、**supervisor プロセス自身が起動できなかった場合** (資格情報失敗 / `Log on as a service` 権限欠如 / ACL 拒否) に運用者が到達できる痕跡が残る。
  - validation: `Log on as a service` を外した状態で service を起動させ、運用者が到達できる記録面 (専用ログファイル、および起動前の失敗については OS 側の service 起動失敗記録) から資格情報起因である旨を読み取れるかを実測する (**UAC-007**)
  - invalidation: 残らなければ UAC-007 は専用ログファイル単独では成立しないものとして扱い、`handoff-record-content-and-sink` の「起動前の失敗も痕跡が残る経路を持つ」を必須 design obligation へ引き上げる。それでも成立しなければ **DP-LOG-DESTINATION を再相談へ差し戻す**
  - 背景: OPT-EVENT-LOG はこの性質を**自明に**持っており UAC-007 の成立に効いていた。OPT-DEDICATED-LOG-FILE 確定後は自明ではなく、検証対象になる
- **pa-localhost-forwarding-recovers-or-is-detectable** — 復帰後 forwarding が自動再確立されるか、されなくても health 失敗として検出できる。
  - validation: **PoC 検証項目 9・11** / invalidation: F-006 の entry 条件を「local endpoint へ到達できない」へ広げ、復帰検知の契機を design へ handoff する
- **pa-poc-precedes-adoption** — 本採用は **PoC 検証項目 1-11** の確認後に行う (plan §15。アーキテクチャ上の否定材料ではなく受け入れ条件)。
  - validation: 項目ごとに pass/fail と観測値を残し未検証項目が無いことを確認する / invalidation: PoC を経ずに進む場合は未検証の仮定が実装契約へ流れることを unresolved decision として明示する

## Design Hypotheses

UX 契約へは昇格させない (design 所有): **dh-health-poll-shape** (health 確認の周期・timeout・連続失敗の数え方) / **dh-failure-classification-signal** (exit code と stderr から permanent と crash を分類できる) / **dh-record-sink-shape** (Event Log の source 登録と event ID、または log file の rotation の実装形) / **dh-resume-detection-trigger** (電源イベント購読と定常 health 確認の組み合わせで復帰を検知できる)。

## Quality Thresholds (すべて design-owned / 未確定)

plan §13 の提案値には **repo 内の根拠が無い**ため、いずれも `owner: design` / `status: design_owned` とし **UX の pass/fail 条件へ昇格させない**。UX 側が要求するのは値ではなく次の関係だけである: 「待機が存在し観察できること」「上限が存在し超えたら止まること」「**再起動を駆動する閾値と終端を駆動する閾値が別物であること**」「**一過性の失敗が設定起因と同じ回数では打ち切られないこと**」。

| id | metric | 提案値 | 検証方法 |
|----|--------|--------|----------|
| threshold-restart-backoff-schedule | 再起動までの待機時間の系列 | 1, 2, 5, 15, 60 秒 (以降 60 で頭打ち) | PoC 検証項目 6 で記録の timestamp から実測間隔を逆算 |
| threshold-max-consecutive-failures | **設定起因**の恒久的な失敗について、終端へ遷移するまでの同一原因の連続失敗回数 | 5 回 | 設定起因の失敗を意図的に起こし、無限リトライにならず何回目で止まり原因が記録されるかを確認。あわせて同じ回数の一過性の失敗では終端へ落ちないこと (UAC-017) を対にして確認 |
| threshold-transient-failure-terminal-limit | **一過性**の失敗 (crash / health timeout / WSL subsystem 利用不可) について、終端へ遷移するまでの連続失敗回数 | 未確定。ただし `threshold-max-consecutive-failures` より**厳密に大きい**こと (plan §8 が permanent と crash の区別を明示要求しているため) | 一過性の失敗を設定起因の上限と同じ回数だけ起こし原因を解消したとき終端へ落ちていないことを確認 (UAC-017)。さらに継続させ、どの回数で終端へ遷移するかを実測して値を導出 |
| threshold-health-recheck-failures-before-restart | プロセスが生存したまま health 確認が失敗し続けるとき、**graceful restart を開始する**までの連続失敗回数 | 未確定 (plan は値を示していない) | health 確認だけが失敗する状態を作り、何回連続失敗で graceful restart が開始されるかを実測。あわせてこの回数では終端へ遷移していないこと (UAC-012 → UAC-013 の順序) を確認 |
| threshold-startup-readiness-budget | 起動から health 成功までの許容時間 | 60 秒 | PoC 検証項目 2・3・11 の実測所要時間分布から導出し直す |
| threshold-shutdown-wait-budget | 停止要求から強制終了へ切り替えるまでの許容時間。**SCM stop 経路 (UAC-009 / UAC-010) と graceful restart の停止段 (UAC-013) で同一物**であり、後者に別予算を新設しない | **host server の drain 8 秒 + gateway grace 5 秒 + supervised unit 終了オーバーヘッドから再導出する。plan §13 の 30 秒はそのままでは採用しない** | PoC 検証項目 7・8 で実測し、短すぎ (teardown 中断) と長すぎ (SCM の停止待ちに間に合わない) の両側を実測で挟む |
| threshold-resume-health-recheck-budget | 復帰検知から health 再確認までの許容時間 | 未確定 (plan は値を示していない) | PoC 検証項目 9 で復帰イベントから再確認までの遅れと自然回復所要時間を計測し、自然回復と区別できる上限を導出 |

plan §13 の `health_url` に含まれる port は repo 内に根拠が無く既定 (`127.0.0.1:8443`) と矛盾するため、**threshold としては保持せず** `handoff-health-target-resolution` の「port は必ず設定由来にする」という design obligation として扱う。

## Technology Candidates for Design Evaluation

次の候補は**非規範的な design handoff** であり、採否は design phase で決定する。この一覧は plan.json の `technology_candidates` と本 member の frontmatter に同じ stable ID + provenance で複写されている。

- **candidate-golang-windows-svc** — `golang.org/x/sys/windows/svc`。SCM 登録・stop request 受理・Session 0 常駐。最大の open は Session 0 から対象 distribution を起動できるか (PoC 検証項目 1・2)。
- **candidate-existing-healthz-endpoint** — 実装済みの `GET /healthz`。open は port の設定由来解決と、`daemon-unavailable` を成功として扱わない判定。
- **candidate-windows-event-log-sink** — **記録先としては不採用** (DP-LOG-DESTINATION は OPT-DEDICATED-LOG-FILE で確定)。**supervisor プロセスが起動する前に失敗した場合の痕跡経路**としてのみ design 評価対象に残す (UAC-007 / `pa-record-file-survives-supervisor-start-failure`)。併用するなら event source 登録の権限が open。
- **candidate-localappdata-log-file** — 専用ログファイル + size-based rotation。**DP-LOG-DESTINATION の確定先**。ただし plan §9 が例示する `%LOCALAPPDATA%\AgentGrid\logs\wsl-host.log` は **profile 相対**であり、Session 0 のサービスアカウントで書くと運用者の profile には現れない。**path は design で確定し直す** (`cc-record-surface-reachable-by-operator`)。profile 配下に閉じる path は disqualifier。
- **candidate-toml-supervisor-config** — plan §13 の専用 TOML。host server の起動引数と health 確認先がドリフトしうる形が disqualifier。

## Design Handoffs

- **handoff-launch-topology** (DP-LAUNCH-INVOCATION-FORM) — 必要成果: host server が foreground の supervised unit として動き、終了が連動し、余計な shell process が残らず、停止操作が既存 graceful shutdown 経路へ到達する。**どの起動トポロジーを選んでも UAC-003 と UAC-008 の判定基準は変わらない。**
- **handoff-already-running-detection** (DP-ALREADY-RUNNING-CLASSIFICATION) — 必要成果: 起動拒否が恒久的失敗としても crash としても分類されず「既に稼働中」として記録される。識別できない場合に危険側 (終端へ落ちる) へ倒れない設計にする。
- **handoff-health-target-resolution** — 必要成果: health 確認先が host server の起動設定と同じ単一の正本から解決され、**固定 port を新設しない**。data dir も同じ正本から解決し明示指定できる。
- **handoff-record-content-and-sink** (DP-LOG-DESTINATION) — 必要成果: 記録を読むだけで「再試行を待っている」「再試行を止めた (原因つき)」「既に稼働中だった」を区別でき、plan §9 の最低記録項目へ到達できる。記録内容の要求は記録面の実装形に依存しない。**explore-r3 で追加**: 記録面は専用ログファイルとし、**profile 相対でない machine-wide な path** を design が確定する / 運用者が「どこを見ればよいか」を一意に知れる経路を定める / rotation 後も直近の失敗が読める / **supervisor 起動前の失敗の痕跡経路** (OS 側の service 起動失敗記録との併用を含む) を確保する。
- **handoff-service-account-hardening** (DP-SERVICE-IDENTITY-SCOPE) — 必要成果: plan §10 の各要因で起動できない状態が原因不明の沈黙にならず記録から辿れる。executable と config の ACL を定める。**explore-r3 で追加**: 任意ユーザー / 任意 distro を指定できる登録経路と既定の解決方法を定める / **既定 distro を採ると `wsl.exe --terminate <distribution>` の対象が日常作業 distro と一致しうる**ことを運用手順として明示する / **既定がログオン中のユーザーであるため資格情報の変化が可用性へ直結する**ことを運用手順として明示する。**explore-r4 で追加** (OPT-BIND-AT-REGISTRATION 確定): 登録時点で identity と distribution を解決し**確定値としてサービス設定へ記録**する (以後の `wsl --set-default` や対話ログオンの変化で対象が動かないことを構造的に保証) / 運用者が**登録直後に確定値を確認できる経路**を定める / **対象変更には再登録 (または設定変更と反映操作) が要る**運用コストと、変更前後で何が起きるか (稼働中 unit の停止と再起動) を手順として明示する。検証義務には「登録後に `wsl --set-default` しても対象が動かないことの実測」と「再登録後の対象が記録面から読めることの確認」を追加した。

## Decision Matrix

各 decision の canonical projection は末尾の managed block を参照。ここでは human-readable な文脈だけを書く。

### DP-SERVICE-IDENTITY-SCOPE (product / default / **must_ask** / default / **answered → OPT-BIND-AT-REGISTRATION**)
**旧軸は破棄した。** explore-r2 は「既存ユーザー共有 (OPT-SHARED-EXISTING) か専用アカウント (OPT-DEDICATED) か」を問うていたが、user 回答は**そのどちらでもなく二者択一の軸自体を否定**した (「任意のユーザー。デフォルトはログオン中のユーザー」「任意の distro。デフォルトはデフォルトの distro」)。確定した内容は product choice ではなく **confirmed constraint `cc-configurable-identity-and-distribution`** として保存し、旧 options は破棄した。**近い option へ丸めていない。**

revision 後の軸 (どちらも明示指定せずに登録したとき、**既定をいつ束縛するか**) に対し、**explore-r4 で user が `OPT-BIND-AT-REGISTRATION` を明示選択して確定**した (consultation-20260725-default-target-binding-time / 分類軸も options も変更していない)。

- **OPT-BIND-AT-REGISTRATION** — **確定**: 登録時点で既定を解決し、実行 identity と対象 distribution を確定値としてサービス設定へ記録する。
- **OPT-RESOLVE-AT-EACH-LAUNCH** — 不採用: 起動のたびに現在の既定を解決して追従する。再登録なしで切り替えられるが、別作業での `wsl --set-default` が監督対象と terminate の対象を変えうる。
- **確定の帰結 (要件へ落とした 4 点)**:
  1. **`wsl.exe --terminate <distribution>` の対象は登録時に確定した distro に固定される。** 運用者が別作業で `wsl --set-default` しても supervisor の対象は動かない。**plan §7 手順 7 の最終手段が意図しない distro を巻き込む懸念はこの選択で閉じた** (UAC-009 は `retain`)。
  2. **運用者は登録直後に「どのユーザーとして・どの distro を対象に動くか」を確定値として確認できる。** 観察面は既存の記録面 (UAC-016) が持つため新規 flow は作らず、**UAC-016 を `rewrite_in_specify`** に分類した。
  3. **対象を変えるには再登録 (または設定変更と反映操作) が要る** — `edge-registration-bound-target` と `handoff-service-account-hardening` が持つ。
  4. 既定が「登録時にログオン中のユーザー」であることによる **plan §10 の資格情報起因の可用性直結は引き続き残る**。`handoff-service-account-hardening` と UAC-007 / `edge-service-account-credentials` は弱めていない。

### DP-LOG-DESTINATION (interaction / outcome / **must_ask** / **normative** / **answered → OPT-DEDICATED-LOG-FILE**)
plan §9 の最低記録項目を運用者が読む記録面をどこに置くか。**critic 指摘により advisory から normative へ引き上げた** — goal-readable-record の成立可否そのものがこの決定に依存するため。**user が OPT-DEDICATED-LOG-FILE を選択して確定** (consultation-20260725 / 分類軸も options も変更していない)。

- **OPT-DEDICATED-LOG-FILE** — **確定**: 任意の手段でテキストとして読め外部ツールと合わせやすい。rotation を自前で持つ必要がある。
- **OPT-EVENT-LOG** — 不採用 (planner 推奨だったが user 判断が優先する)。ただし「supervisor が起動できなかった場合の痕跡が同じ面に残る」性質だけは `candidate-windows-event-log-sink` の**併用候補**として design 評価に残す。
- **確定の帰結 (要件へ落とした 2 点)**:
  1. `cc-record-surface-reachable-by-operator` — 記録面は**運用者が到達できる場所**に置き、**サービスアカウントの profile 配下に閉じない**。plan §9 が挙げる `%LOCALAPPDATA%\AgentGrid\logs\wsl-host.log` は profile 相対であり、Session 0 のサービスアカウントが書くと運用者が自分の profile を見ても何も無いという**空振り**が起きる。**具体 path は確定しない** (design 所有)。
  2. `pa-record-file-survives-supervisor-start-failure` + `handoff-record-content-and-sink` — OPT-EVENT-LOG が自明に持っていた「supervisor 自体が起動できなかった場合の痕跡」(**UAC-007 の成立に効いていた**) は専用ログファイルでは自明でないため、validation obligation と design obligation として明示した。UAC-007 は `rewrite_in_specify`。

### DP-LAUNCH-INVOCATION-FORM (implementation / mechanism / **defer** / handoff / unanswered)
supervised unit の起動トポロジー — shell を挟まない直接起動か、WSL 内 launcher で環境を整えて最後を `exec` にするか。
- **OPT-DIRECT-EXEC** (推奨、confidence low): 監督階層が最も浅い。data dir や PATH の準備を起動引数だけで賄う必要がある。停止操作が Linux 側へ届くかは未検証。
- **OPT-WSL-LAUNCHER-EXEC**: 環境の準備は確実。`exec` で置き換え損ねると余計な shell process が残り停止が届かなくなる。
- user impact: この選択の結果によって UAC-003 と UAC-008 が成立するかが決まる / 届かない場合 drain (sandbox cleanup) が実行されないまま消える。
- **critic 指摘により confirmed constraint から外した**。UX 側は結果だけを固定し、選択と検証は design が所有する。

### DP-ALREADY-RUNNING-CLASSIFICATION (implementation / mechanism / **defer** / handoff / **defaulted**)
既存 instance の起動拒否を「既に稼働中」と読み分ける信号の選択。要求される結果は UAC-006 で規範化済みであり、**手段だけ**が design 所有として残る。
- **OPT-TREAT-AS-ALREADY-RUNNING** (既定): 拒否を「既に稼働中」の信号として扱い health 確認へ進む。
- **OPT-TREAT-AS-CONFIGURATION-FAILURE**: 他の設定エラーと同列の恒久的失敗として扱う → 健全に稼働している状態が障害として記録される。
- 根拠: flock はプロセス終了で自動解放され stale lock が残らない (`daemon_lock.go:35`)。

### DP-SHUTDOWN-TIMEOUT-PROVENANCE (performance / assumption / **validate** / validation_required / unanswered)
停止予算を server 側の内訳から導出するか、独立値 30 秒を採るか。
- **OPT-DERIVE-FROM-SERVER-BUDGET** (推奨): drain 8 秒 + gateway grace 5 秒 + supervised unit 終了オーバーヘッドから導出。server 側の設定変更に追従する。
- **OPT-INDEPENDENT-FIXED-30S**: 実装は単純だが追従せず乖離する。**30 秒という値自体に repo 内の根拠が無い。**
- user impact: 短すぎると teardown が中断される / 長すぎると SCM の停止待ちに間に合わず止まらないように見える。

## Recommended Direction

いずれも provisional であり user 承認を得ていない。**確定した decision (DP-LOG-DESTINATION / DP-SERVICE-IDENTITY-SCOPE) はここから外してある。**

- 起動トポロジーは UAC-003 と UAC-008 を満たせるかという**結果だけ**で選ぶ。plan §4 の直接起動を既定候補としつつ、環境変数と working directory の準備が必要になった時点で launcher + `exec` へ切り替える。
- 停止予算は drain 8 秒 + gateway grace 5 秒 + supervised unit 終了オーバーヘッドの実測から導出し、**plan §13 の 30 秒を独立の要件値として置かない**。

## Unresolved Product Decisions

| decision | effect | 状態 |
|----------|--------|------|
| 起動トポロジー (直接 / launcher + exec) | handoff | design 委譲 |
| 起動拒否の読み分け信号 | handoff | 既定選択済み、検出手段は design 委譲 |
| 停止予算の導出方法 | validation_required | PoC 実測待ち |

**user consultation で確定済み**: 記録面の置き場所 (`DP-LOG-DESTINATION` = **OPT-DEDICATED-LOG-FILE** / normative / explore-r3) / 既定の束縛時点 (`DP-SERVICE-IDENTITY-SCOPE` = **OPT-BIND-AT-REGISTRATION** / default / explore-r4)。実行 identity と対象 distribution の**設定可能性と既定**は decision ではなく `cc-configurable-identity-and-distribution` として確定。**normative / default の未解決 decision は残っていない。**

## Validation Questions

**未回答の validation question は無い。Specify readiness の blocker も 0 件。** consultation blocker だった 2 件は user 回答で確定し (DP-LOG-DESTINATION = OPT-DEDICATED-LOG-FILE / explore-r3、DP-SERVICE-IDENTITY-SCOPE = OPT-BIND-AT-REGISTRATION / explore-r4)、最後に残っていた primary alternative も explore-r5 で user 確定した (代替軸の廃止と既定 deployment 構成の承認)。残る未決は design 委譲・PoC 実測待ち・Specify 持ち越しのみで、いずれも Specify を止めない (Open Questions を参照)。

## Consultation Impact (explore-r3 / explore-r4 の scenario_impacts)

| scenario | 起因 decision | 分類 | 理由 |
|----------|--------------|------|------|
| UAC-007 | DP-LOG-DESTINATION, DP-SERVICE-IDENTITY-SCOPE | **rewrite_in_specify** | 専用ログファイル確定により「supervisor 自身が起動できなかった場合の痕跡が同じ面に残る」性質 (OPT-EVENT-LOG では自明) が失われた。Specify で Then を「**運用者が到達できる記録面**から読み取れる (supervisor 起動前の失敗を含む)」へ書き直し、counterexample も「supervisor プロセス起動後にしか書けない記録面しか持たない実装」が fail することを明示する。既定 identity が「登録時にログオン中のユーザー」で確定したため、Given の資格情報失敗が**既定構成でそのまま起きうる**点も明示する |
| UAC-016 | DP-LOG-DESTINATION | **retain** | 記録内容の要求は記録先に依存しない (`handoff-record-content-and-sink`)。最低記録項目・secret 非記録・counterexample の判別性はいずれも変化しない。到達可能性は `cc-record-surface-reachable-by-operator` と design obligation が担う |
| UAC-016 | DP-SERVICE-IDENTITY-SCOPE | **rewrite_in_specify** | OPT-BIND-AT-REGISTRATION の確定により「登録時に解決された identity / distribution が確定値である」ことが観察可能になった。UAC-016 は既に「対象 distribution と Linux user を記録面から読める」ことを要求しているため、**新規 flow を作らず**この scenario の Then を「読み取れる対象が**登録時に確定した値と一致し、その後の `wsl --set-default` では変化しない**」へ拡張する。counterexample は「起動のたびに現在の既定 distro を解決して記録する実装」が fail することで判別する (`vs_legacy: must-fail` のまま) |
| UAC-009 | DP-SERVICE-IDENTITY-SCOPE | **retain** | 「対象 distribution だけが停止し他の distribution の作業は停止しない」という判定基準は、設定可否にも既定の束縛時点にも依存しない。OPT-BIND-AT-REGISTRATION により terminate の対象が登録時の distro へ固定され、判定はむしろ安定する方向へ動いた (Given / Then は不変) |

## PoC Coverage (plan §12 検証項目 1-11)

| # | 検証項目 | 被覆 |
|---|---------|------|
| 1 | service account から `wsl.exe -l -q` で distribution が見える | `pa-service-account-can-launch-distribution` の validation (critic 指摘により acceptance scenario から validation へ移した — 列挙は supervisor の常時挙動ではなく PoC の前提検証) |
| 2 | 未ログイン状態で Windows 起動後に自動起動する | UAC-001 |
| 3 | host server が foreground で維持される | UAC-002 |
| 4 | host server 終了時に `wsl.exe` が終了する | UAC-003 |
| 5 | abnormal exit を検出できる | UAC-004 |
| 6 | backoff 付き再起動が働く | UAC-004 |
| 7 | Service stop が graceful shutdown になる | UAC-008 |
| 8 | timeout 時に対象 distribution だけを terminate できる | UAC-009, UAC-010 |
| 9 | sleep / hibernate 復帰後に health が戻る | UAC-011, UAC-012, UAC-013 |
| 10 | Windows 側から local endpoint に接続できる | UAC-001, UAC-002 |
| 11 | Windows Update / WSL update 後に再起動できる | UAC-014 (再起動あり) / UAC-015 (再起動なしの subsystem 障害) |

なお **UAC-017** は plan §12 の検証項目ではなく **plan §8 が明示要求する「permanent failure と crash の区別」**を検収する scenario である。exit code も stderr も見ずあらゆる失敗を一律の回数で打ち切る無分類の supervisor は他の 16 scenario をすべて pass できてしまうため、この区別は UAC-017 だけが判別する。

## Open Questions

- **[運用前提 / 残存リスク・再相談不要]** 既定の実行 identity が「**登録時にログオンしているユーザー**」で確定したため、そのアカウントの password 変更・期限切れ・`Log on as a service` 権限の喪失は引き続き可用性へ直結する (plan §10 / `edge-service-account-credentials` / UAC-007)。**user が確定した既定であり再相談はしない。** 無人運用で専用アカウントを使う選択は登録時の指定で表現でき、その運用手順の明示は `handoff-service-account-hardening` が持つ。
- **[DP-LOG-DESTINATION 確定の派生義務]** 記録面は専用ログファイルで確定。派生する未決は 2 点: (1) **具体 path は design 所有**。要件は「サービスアカウントの profile 配下に閉じない」(`cc-record-surface-reachable-by-operator`) だけを固定している。plan §9 の `%LOCALAPPDATA%\AgentGrid\logs\wsl-host.log` をそのまま採ると **Session 0 のサービスアカウントが書いた記録を運用者が自分の profile で探して空振りする**。(2) supervisor 自身が起動できなかった場合の痕跡は専用ログファイルでは自明に残らないため、UAC-007 の成立は `pa-record-file-survives-supervisor-start-failure` の validation と `handoff-record-content-and-sink` の obligation で担保する。
- **[Specify 持ち越し / UAC-016 の rewrite]** `OPT-BIND-AT-REGISTRATION` の確定により「登録時に確定した対象 identity / distribution が記録面から読み取れ、その後の `wsl --set-default` では変化しない」が観察可能になった。Specify で UAC-016 の Then と counterexample を拡張する。**新規 flow / scenario は足していない** — 登録という契機を持つ flow が存在せず、新設すると PoC 検証項目 1-11 の被覆表と 17 scenario の既検収構造を壊すため。
- **[Specify 持ち越し / UAC-005]** Given が「先行する成功起動の有無」を固定していないため、「一度でも Ready に到達したか」だけで permanent / transient を分類する heuristic 実装が環境次第で緑になりうる。Specify で Given を「**一度も Ready に到達していない**」と「**稼働後に設定が壊れた**」の 2 条件へ分けるか、後者を追加 scenario にすると閉じる。
- **[Specify 持ち越し / F-005 と F-006 の escalation 差]** 同一の停止予算に対する escalation 動作が F-005 と F-006 で異なる (F-005 は plan §7 の「子 process 強制終了 → なお残れば distribution terminate」、UAC-013 は plan §11 の「予算超過 → 直ちに distribution terminate」)。どちらも plan 由来で矛盾ではないが、plan §7 の順序に従って子 kill を成功させる実装は UAC-013 を fail しうる。**design で「復帰経路では中間段を踏まない」と明記すれば閉じる。**
- **[PoC 実測待ち / DP-SHUTDOWN-TIMEOUT-PROVENANCE]** 停止予算を drain 8 秒 + gateway grace 5 秒から導出するか、plan §13 の 30 秒を採るか。30 秒には repo 内の根拠が無い。
- **[design 委譲 / DP-LAUNCH-INVOCATION-FORM]** 起動トポロジー。UAC-003 と UAC-008 の判定基準はどちらでも変わらないため UX 契約からは外したが、**Windows Service から WSL 内 Linux プロセスへ停止シグナルが到達するかは未検証**であり handoff-launch-topology の verification obligation として残っている。
- **[design 委譲 / DP-ALREADY-RUNNING-CLASSIFICATION]** 起動拒否を読み分ける信号の選択。結果は UAC-006 で規範化済み、手段は未確定。
- **[数値未確定]** 再試行間隔・設定起因の連続失敗上限・一過性の失敗の終端上限・health 再確認からの再起動閾値・起動予算・復帰後 health 再確認予算はいずれも design_owned のまま。UX が固定しているのは値ではなく「**再起動を駆動する閾値と終端を駆動する閾値が別物であること**」「**一過性の失敗が設定起因と同じ回数では打ち切られないこと**」「**停止予算は SCM stop 経路と graceful restart の停止段で同一物であること**」という関係だけである。plan §13 の port は threshold として保持せず `handoff-health-target-resolution` の「設定由来にする」義務として扱っている。

<!-- requirements-decisions:start -->
## Decision State
Selected alternative: ALT-SHARED-EXISTING-ACCOUNT
Decision DP-ALREADY-RUNNING-CLASSIFICATION: OPT-TREAT-AS-ALREADY-RUNNING
Decision DP-LAUNCH-INVOCATION-FORM: unresolved
Recommendation DP-LAUNCH-INVOCATION-FORM: OPT-DIRECT-EXEC
Decision DP-LOG-DESTINATION: OPT-DEDICATED-LOG-FILE
Decision DP-SERVICE-IDENTITY-SCOPE: OPT-BIND-AT-REGISTRATION
Decision DP-SHUTDOWN-TIMEOUT-PROVENANCE: unresolved
Recommendation DP-SHUTDOWN-TIMEOUT-PROVENANCE: OPT-DERIVE-FROM-SERVER-BUDGET
<!-- requirements-decisions:end -->
