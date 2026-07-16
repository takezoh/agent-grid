---
change: change-20260711-test-harness-north-star
role: requirements
---

# Requirements

## Legacy Source (verbatim)

````markdown
---
id: spec-20260711-test-harness-north-star
kind: spec
title: Test harness north-star enforcement
status: implemented
created: '2026-07-11'
tags: []
owners: []
functional_requirements:
- id: FR-001
  statement: システムは、ptyを唯一のgrandfathered exceptionとし、その他の外部依存をpublic in-process fake、名前付きT2
    invariant contract、同一入力をrealと比較するT3 fidelityへ結線しなければならない。
  priority: must
  rationale: accepted tier taxonomyを維持し、例外の一般化を防ぐ。
- id: FR-002
  statement: 外部依存registryを検査するとき、システムは各tripleの参照存在だけでなくseam、fakeの公開性、contractの観測可能なinvariant、fidelityの同一入力比較を検証しなければならない。
  priority: must
- id: FR-003
  statement: skipが存在する間、システムは各skipの分類、理由、owner、expiry、追跡先または恒久根拠、evidenceを中央inventoryに保持し、未登録または期限切れのskipを拒否しなければならない。
  priority: must
- id: FR-004
  statement: 変更test repeatを実行するとき、システムはbase/head diffから決定的に対象を選び、既定10回反復し、1回の失敗、timeout、途中終了のいずれも失敗とし、case、attempt、duration、resultをartifactへ記録しなければならない。
  priority: must
- id: FR-005
  statement: 変更testをdiffから決定できない場合、システムはrepeatを省略せず、言語ごとの保守的suiteへ昇格しなければならない。
  priority: must
- id: FR-006
  statement: nightly T3が終了したとき、システムはsuiteとcaseごとのpass、fail、skip、skip理由を機械可読artifactに出力し、必須caseのskipを失敗にしなければならない。
  priority: must
- id: FR-007
  statement: protected harness変更を検出したとき、システムは理由、owner、expiry、evidenceを含むescalation
    requestを出力し、変更主体だけでは通常成功へ変更できない専用statusで終了しなければならない。
  priority: must
- id: FR-008
  statement: システムは、coverage floor、必須workflow invocation、test、registry、checker、protected
    manifest、CODEOWNERSの削除または弱体化をmerge-base差分とmanifestの両方で検出しなければならない。
  priority: must
- id: FR-009
  statement: もしPR headがanti-tampering checkerまたはそのworkflow invocationを削除する場合、システムは別のstatic
    enforcement testによりその自己削除を拒否しなければならない。
  priority: must
- id: FR-010
  statement: mutation pilotを実行するとき、システムは固定したoperator集合、seed 20260711、mutant manifest、対象別timeout、baseline、killまたはsurviveをartifact化し、timeoutまたはbaseline未満を失敗にしなければならない。
  priority: must
- id: FR-011
  statement: save、pre-push、PR、nightly profileを実行するとき、システムは同じrepository command群を再利用してT0、T0-T2、全PR
    gate、T3の順に範囲を拡張し、実行項目と省略理由を表示しなければならない。
  priority: must
- id: FR-012
  statement: FakeVsRealが不一致の間、システムはassertion緩和を通常変更として許容せず、fakeまたはfixture更新のevidenceを伴うprotected変更として扱わなければならない。
  priority: must
non_functional_requirements:
- id: NFR-001
  type: performance
  criteria: save profileのp95はincremental checkoutで10秒以内、pre-push profileは5分以内とし、超過commandを個別表示する。
  measurement: profile artifactのcommand duration集計
- id: NFR-002
  type: reliability
  criteria: registry、skip、repeat、anti-tampering checkerはnetworkとcredentialなしに同一入力から同一exit
    codeと正規化artifactを生成する。
  measurement: Linux/macOS fixture contractとrepeat実行
- id: NFR-003
  type: maintainability
  criteria: policy parserと判定はGo stdlibのpure coreに置き、workflowはrepository commandを呼ぶだけにする。
  measurement: gorules static testとpackage unit test
- id: NFR-004
  type: usability
  criteria: policy failureは対象ID、path、違反規則、修復方法をすべて含む。
  measurement: negative fixtureのdiagnostic assertion
- id: NFR-005
  type: performance
  criteria: mutationはstream routing、state.Reduce、Go-TS wire codecの固定mutantだけを対象とし、対象ごとに5分、全体15分でfail
    closedする。
  measurement: mutation artifactのdurationとtimeout result
acceptance:
- id: AC-001
  given: fake名だけを持ちcontract assertionが空の外部依存row
  when: dependency admission checkerを実行する
  then: dependency IDとmissing invariantを示して非0終了する
  requirement_refs:
  - FR-001
  - FR-002
  - NFR-004
- id: AC-002
  given: pty以外でfidelityを欠くrow、または必須項目を欠く例外
  when: registry checkerを実行する
  then: accepted taxonomy違反として非0終了する
  requirement_refs:
  - FR-001
  - FR-002
- id: AC-003
  given: 未登録skip、期限切れskip、または反復10回中1回だけ失敗するfixture
  when: skip checkerまたはrepeat profileを実行する
  then: greenにせず、再計算可能なattempt artifactを残す
  requirement_refs:
  - FR-003
  - FR-004
  - FR-005
- id: AC-004
  given: 必須T3 caseがskipされたnightly結果
  when: nightly reportを集約する
  then: case理由をartifactに残してjobを失敗させる
  requirement_refs:
  - FR-006
- id: AC-005
  given: checker invocation削除、checker自己削除、floor低下、test skip化のいずれかを含むdiff
  when: repo内enforcementを実行する
  then: escalation artifactを生成し、独立static testまたはdiff gateが非0終了する
  requirement_refs:
  - FR-007
  - FR-008
  - FR-009
  - FR-012
- id: AC-006
  given: 理由だけありowner、expiry、evidenceのいずれかを欠くprotected change
  when: anti-tampering gateを実行する
  then: 自己承認として受理せず非0終了する
  requirement_refs:
  - FR-007
  - FR-008
- id: AC-007
  given: 固定mutant集合とbaseline artifact
  when: 同じseedでmutation profileを2回実行する
  then: mutant ID集合とscoreが一致し、survivor、timeout、baseline未満のいずれかで失敗する
  requirement_refs:
  - FR-010
  - NFR-005
- id: AC-008
  given: clean checkoutと各verification profile
  when: local commandとCI entrypointを実行する
  then: 同じrepository commandが使われ、profileごとのTier、実行項目、省略理由、durationが観測できる
  requirement_refs:
  - FR-011
  - NFR-001
  - NFR-002
  - NFR-003
relations:
- {type: implementedBy, target: plan-20260711-test-harness-north-star}
- {type: referencedBy, target: adr-20260711-test-harness-dependency-admission}
- {type: referencedBy, target: adr-20260711-test-harness-skip-repeat}
- {type: referencedBy, target: adr-20260711-test-harness-anti-tampering}
- {type: referencedBy, target: adr-20260711-test-harness-mutation-pilot}
- {type: referencedBy, target: adr-20260711-test-harness-verification-profiles}
source_paths: []
summary: 外部依存 triple、skip/flaky、anti-tampering、mutation、速度 tier を機械強制する
updated: '2026-07-11'
---

## Overview

既存の T0〜T3 taxonomy と stream routing の模範象限を全 harness policy の基準にする。対象は M2〜M6 であり、外部依存 admission、skip/flaky、改ざん防止、mutation、速度 profile を repository 内で観測可能かつ fail-closed にする。

GitHub branch protection 等の repo 外設定の変更・保証は対象外である。ただし repo 内の `CODEOWNERS`、protected manifest、独立 static test、外部 prerequisite の明示は対象に含む。

## Requirements

{% req id="FR-001" %}pty 以外の外部依存は fake、named T2 contract、T3 fidelity の executable triple を持つ。{% /req %}
{% req id="FR-002" %}admission checker は参照名ではなく triple の意味契約を負例で検査する。{% /req %}
{% req id="FR-003" %}全 skip は中央 inventory で理由、owner、expiry、evidence を監査できる。{% /req %}
{% req id="FR-004" %}変更 test は決定的に選択され、10 回中 1 回でも失敗すれば flaky と判定される。{% /req %}
{% req id="FR-005" %}選択不能時は保守的 suite へ昇格する。{% /req %}
{% req id="FR-006" %}nightly は case-level skip-green を許さない。{% /req %}
{% req id="FR-007" %}protected change は自己承認できず人間 review へ昇格する。{% /req %}
{% req id="FR-008" %}diff と manifest の双方で harness 弱体化を検出する。{% /req %}
{% req id="FR-009" %}checker 自己削除は別 static test が拒否する。{% /req %}
{% req id="FR-010" %}mutation は固定 operator、seed、timeout、baseline で再現可能にする。{% /req %}
{% req id="FR-011" %}local と CI は同じ repository command を共有する。{% /req %}
{% req id="FR-012" %}FakeVsReal 不一致は assertion 緩和ではなく fake 修復へ向ける。{% /req %}

NFR-001〜NFR-005 は速度、決定性、診断可能性、stdlib 優先、時間上限を数値で固定する。

## Acceptance Criteria

{% acceptance id="AC-001" %}空の triple を負例が拒否する。{% /acceptance %}
{% acceptance id="AC-002" %}pty 以外の例外拡大を拒否する。{% /acceptance %}
{% acceptance id="AC-003" %}`merge-base...HEAD` の name-status diff から Go は変更 `_test.go` file と同 package、TS は変更 `*.test.*` file と import/source sibling を選ぶ。rename は旧新 path、delete は旧 path の package/siblingを対象化し、空集合は Go 全 package と Web全testへ昇格する。固定 seed 20260711・10回中1回でも失敗すれば green にしない。{% /acceptance %}
{% acceptance id="AC-004" %}必須 T3 case の skip を失敗にする。{% /acceptance %}
{% acceptance id="AC-005" %}trusted baseline は PR head から変更不能な Git merge-base tree とする。head の checker と `static_enforcement_test` は merge-base版で相互pinし、merge-base workflow と head workflow の比較で checker invocation 削除を拒否する。checker・static test・workflow invocation の同時削除も失敗する。{% /acceptance %}
{% acceptance id="AC-006" %}不完全な escalation request を自己承認として拒否する。{% /acceptance %}
{% acceptance id="AC-007" %}mutant identity を path、byte span、operator、normalized source hash で固定し、seed 20260711 と同じ baseline hash から同じ集合・scoreを再現する。baseline更新は survivor理由とevidence付き protected change とし、mutant/baseline同時弱体化をanti-tamperingがreview-requiredにする。{% /acceptance %}
{% acceptance id="AC-008" %}profile の Tier、command、duration、省略理由を観測できる。{% /acceptance %}


{% transition from="draft" to="approved" date="2026-07-11" %}
独立Verify 3項目pass、実装指示に基づき承認
{% /transition %}


{% transition from="approved" to="implemented" date="2026-07-11" %}
EARS acceptanceをregistry、skip/repeat、tampering、mutation、profile、CIで実装
{% /transition %}

````
