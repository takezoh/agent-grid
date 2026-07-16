---
change: change-20260711-test-harness-north-star
role: implementation
---

# Implementation

## Legacy Source (verbatim)

````markdown
---
id: plan-20260711-test-harness-north-star
kind: plan
title: Implement test harness north-star enforcement
status: done
created: '2026-07-11'
goal: M2〜M6を依存順に実装し、harnessの欠落・弱体化・skip-greenをrepository内で機械検出する
scope_in:
- 外部依存registryとexecutable triple checker
- skip inventory、変更test repeat、case-level nightly report
- protected manifest、CODEOWNERS、diff gate、独立static pin
- 固定mutantによるcritical-path mutation pilot
- save、pre-push、PR、nightlyの共有verification profile
scope_out:
- GitHub branch protection、required checks、ruleset、reviewer権限の変更または保証
- 全packageへのmutation展開
- 実機browser、soft keyboard、assistive technology fidelity
- real pty例外の撤回
milestones:
- id: m1
  title: M2 executable dependency admission
  status: todo
- id: m2
  title: M3 skip and deterministic repeat
  status: todo
- id: m3
  title: M4 anti-tampering trust boundary
  status: todo
- id: m4
  title: M5 deterministic mutation pilot
  status: todo
- id: m5
  title: M6 shared verification profiles
  status: todo
- id: m6
  title: CI integration and full gate
  status: todo
contracts:
- registry rowはpty以外でpublic fake、named T2 invariant contract、T3 FakeVsRealを必須とする
- 例外は理由、owner、expiry、evidenceを要求し、現行accepted ADR上はptyだけを許す
- protected changeは自己承認せずCODEOWNERS reviewへ昇格する
- checker invocationとchecker自己削除は別のstatic testがpinする
- mutationは固定operator、seed 20260711、対象別5分、全体15分、checked-in baselineを使う
tags: []
owners: []
relations:
- {type: implements, target: spec-20260711-test-harness-north-star}
- {type: hasPart, target: adr-20260711-test-harness-dependency-admission}
- {type: hasPart, target: adr-20260711-test-harness-skip-repeat}
- {type: hasPart, target: adr-20260711-test-harness-anti-tampering}
- {type: hasPart, target: adr-20260711-test-harness-mutation-pilot}
- {type: hasPart, target: adr-20260711-test-harness-verification-profiles}
source_paths: []
summary: M2-M6 を依存順に実装する計画
updated: '2026-07-11'
---

## Goal

既存 T0〜T3 資産を置換せず、その加入条件と弱体化防止を repository 内の executable policy にする。各 milestone は parser/decision の pure core、wired fixture、negative contract、real fidelity を同梱し、単独で検証可能な責務単位として完了させる。

## Implementation Sequence

### m1

{% milestone id="m1" %}
M2 を二単位で実装する。

1. **Registry model and pure validator** — objective: `test-harness/dependencies.json` を SSOT とし、Linear HTTP、git、gh、clock、browser、Claude/Codex、docker、app-server、pty を登録する。output: registry、Go stdlib parser/validator、T0 table test。tool guidance: 既存 `FakeVsReal*`、`drivertest`、`runtimetest`、taxonomy ADR を参照する。boundary: fakeやT3本体の作り直し、一般例外の導入はしない。files: `test-harness/dependencies.json`, `src/internal/harnesspolicy/dependency.go`, `src/internal/harnesspolicy/dependency_test.go`。acceptance: AC-001/AC-002、ptyだけがADR参照付きgrandfathered exceptionとして通る。max diff: 300 LOC。
2. **Admission command and contract fixtures** — objective: rowからpublic seam、fake、named invariant assertion、same-input fidelityの結線を検査する。output: command、valid/invalid fixtures、T1/T2 tests。tool guidance: Go AST/parserと既存 `src/gorules` を使う。boundary: workflow結線はm6。files: `src/cmd/harness-check/`, `src/internal/harnesspolicy/testdata/dependencies/`, `scripts/check-harness-dependencies.sh`。acceptance: 名前だけ、空assertion、missing fidelity、pty以外の例外を個別診断で拒否。depends on: Registry model and pure validator。max diff: 400 LOC。
{% /milestone %}

### m2

{% milestone id="m2" %}
M3 を二単位で実装する。

1. **Skip inventory and scanner** — objective: Go/TS/Playwright skipを中央inventoryへ照合する。output: JSON inventory、language scanner adapters、T0/T2 negative tests。tool guidance: source scannerは薄く、policyは`internal/harnesspolicy` pure coreへ置く。boundary: repeat実行とnightly集約は次単位。files: `test-harness/skips.json`, `src/internal/harnesspolicy/skip.go`, `src/internal/harnesspolicy/skip_test.go`, `scripts/check-test-skips.sh`。acceptance: reason/owner/expiry/evidence欠落、未登録、期限切れを拒否。depends on: m1。max diff: 350 LOC。
2. **Changed-test repeat and nightly result schema** — objective: `git diff --name-status <merge-base>...HEAD` を正規化し、Go は変更 `_test.go` と同 package、非test `.go` は同 package の全test、TS は変更 `*.test.*` と変更sourceの同directory/import sibling testを選ぶ。renameは旧新pathの和、deleteは旧pathのpackage/siblingを選ぶ。対象空集合またはmapping不能は Go=`go test ./...`、TS=`npm test -- --run` へ昇格する。seedは`20260711`、順序はpath/test名sort、反復は固定10回とする。output: selector、fake command runner、attempt JSON、nightly case JSON、T1/T2 tests。boundary: workflow uploadはm6。files: `src/internal/harnesspolicy/repeat.go`, `src/internal/harnesspolicy/repeat_test.go`, `scripts/repeat-changed-tests.sh`, `scripts/collect-e2e-results.sh`, `test-harness/schemas/test-results.schema.json`。acceptance: AC-003/AC-004、add/modify/rename/delete/空集合のfixtureが同じ対象順を返し、1失敗・timeout・途中終了がfail closed。depends on: Skip inventory and scanner。max diff: 500 LOC。
{% /milestone %}

### m3

{% milestone id="m3" %}
M4 を二単位で実装する。

1. **Protected manifest and diff policy** — objective: trusted baselineをPR headから同時変更不能なGit merge-base treeとして読み、base側のfloor、workflow invocation、test、registry、checker、static test、manifest、CODEOWNERSとhead側を分類する。output: protected JSON、base/head reader seam、pure classifier、fixture contracts、escalation artifact schema。boundary: GitHub APIと承認判定は行わない。files: `test-harness/protected.json`, `src/internal/harnesspolicy/tampering.go`, `src/internal/harnesspolicy/tampering_test.go`, `scripts/check-harness-tampering.sh`。acceptance: 理由、owner、expiry、evidenceが揃ってもreview-requiredであり、baseline manifestとmutation baselineの同時変更を通常successにしない。depends on: m1。max diff: 500 LOC。
2. **Merge-base bootstrap and mutual self-deletion pin** — objective: CI bootstrapがmerge-base版checkerをmaterializeしてhead diffを検査し、merge-base版checkerがhead `static_enforcement_test` とworkflow invocationをpin、merge-base版static test契約がhead checker invocationをpinする相互契約を作る。workflow自体のhead変更はmerge-base workflowからrequired invocationを抽出して比較する。output: bootstrap script、static test、CODEOWNERS、workflow contract。boundary: repo外required-check設定の変更はしないが、trusted checkout/bootstrapの実行を外部prerequisiteとして明示する。files: `scripts/run-trusted-harness-gate.sh`, `src/gorules/static_enforcement_test.go`, `.github/CODEOWNERS`, `.github/workflows/ci.yml`。acceptance: AC-005/AC-006、checker・static test・workflow invocationの個別削除と同時削除fixtureがすべてmerge-base実装で失敗。depends on: Protected manifest and diff policy。max diff: 350 LOC。
{% /milestone %}

### m4

{% milestone id="m4" %}
M5 は一単位で実装する。**Deterministic mutant runner** — objective: stream routing、`state.Reduce`、Go-TS wire codecにchecked-in mutant manifestを適用する。operator setは `conditional-negation`、`route-target-substitution`、`event-drop`、`codec-field-omission` に固定し、mutant identityは`path + byte span + operator + normalized source SHA-256`、runner identityはmerge-baseにあるrunner source SHA-256、seedは`20260711`とする。対象ごと5分、全体15分、timeoutはsurvive扱い。baseline manifestはmutant identity、must-kill、最小score、runner hash、operator-set versionを持つ。更新は新旧artifact、survivor理由、owner、expiry、evidenceを添え、anti-tamperingのprotected changeとしてreview-requiredにする。mutant manifestとbaselineの同時変更でもmerge-base baselineを判定基準にする。output: manifest、runner、再現commandを含むartifact、T0 parser/T1 fake runner/T2 baseline contract。boundary:汎用mutation engineや対象拡大はしない。files: `test-harness/mutants.json`, `test-harness/mutation-baseline.json`, `src/internal/harnesspolicy/mutation.go`, `src/internal/harnesspolicy/mutation_test.go`, `scripts/run-mutation-pilot.sh`。acceptance: AC-007、同じseed/baseline hashでID集合・score一致、identity drift、survivor、timeout、baseline低下、無承認baseline更新は非0。depends on: m2。max diff: 550 LOC。
{% /milestone %}

### m5

{% milestone id="m5" %}
M6 は一単位で実装する。**Shared profile runner** — objective: `save`、`pre-push`、`pr`、`nightly` の宣言と実行を一元化する。output: profile JSON、runner、Make targets、fake runner tests、duration/skip report。tool guidance:既存Makefileとscriptsを再利用し、hook installerはopt-in。boundary:user configを変更せず、新task runnerを導入しない。files: `test-harness/profiles.json`, `src/internal/harnesspolicy/profile.go`, `src/internal/harnesspolicy/profile_test.go`, `scripts/run-verification-profile.sh`, `Makefile`。acceptance: AC-008、save=T0、pre-push=T0-T2変更範囲、pr=全T0-T2/race/fuzz/coverage/diff、nightly=T3。depends on: m2,m3。max diff: 400 LOC。
{% /milestone %}

### m6

{% milestone id="m6" %}
**CI wiring and end-to-end enforcement** — objective: CI/nightlyを共有commandの薄いcallerにし、artifact uploadと全static pinを有効化する。output: workflow edits、gorules pins、cross-profile contract。boundary:branch protection API変更はしない。files: `.github/workflows/ci.yml`, `.github/workflows/e2e-nightly.yml`, `src/gorules/static_enforcement_test.go`, `scripts/check-e2e-siblings.sh`。acceptance: 全AC、workflow内にpolicy判断の複製がなく、必須command/artifactがpinされる。depends on: m3,m4,m5。max diff: 350 LOC。
{% /milestone %}

## Targets

| Target | 責務 | seam / pure core |
|---|---|---|
| `src/internal/harnesspolicy` | registry、skip、diff、mutation、profileのparse/decision | filesystem、clock、command実行を引数化したpure Go core |
| Linear HTTP | existing `httptest` client seam | public HTTP fake + named contract + opt-in real credential T3 |
| git / gh CLI | existing Runner/command seam | fake command runner + output contract + opt-in installed binary T3 |
| clock | existing `Clock`/値注入 | fake clock + scheduling invariant + bounded real-clock fidelity |
| browser | existing Playwright fake backend | backend seam + user-flow contract + installed browser T3 smoke |
| Claude/Codex/docker/app-server | existing fake packages and `FakeVsReal*` | registryから既存3点セットを参照し再実装しない |
| real pty | existing real pty T2 | accepted taxonomy ADRを参照する唯一のexception |
| git diff input | merge-base tree reader、head tree reader、name-status diffを別引数化 | rename/delete/同時削除fixtureをT1へ注入。trusted判定はmerge-base側実装を使う |
| command/time | `CommandRunner`, `Clock` | repeat/mutation/profileのT1 fake |

構造規則は「policy判断をworkflow/shellへ置かない」「checker自己削除pinをchecker自身へ置かない」「外部依存rowをtripleまたはpty例外へ全加入」の3つとする。

## Verification

| profile | Tier | 実行コマンド | 判定基準 / milestone DoD |
|---|---|---|---|
| policy-pure | T0 pure | `cd src && go test ./internal/harnesspolicy/...` | parser/selector/classifierがfixture期待値と一致。m1〜m5共通 |
| dependency-wired | T1 wired | `scripts/check-harness-dependencies.sh --fixture valid` | 全registry rowのseamと参照を解決。m1 |
| dependency-contract | T2 contract | `scripts/check-harness-dependencies.sh --fixtures test-harness/testdata/dependencies` | 空fake/contract/fidelityとpty以外例外を拒否。m1 |
| skip-repeat-wired | T1 wired | `scripts/repeat-changed-tests.sh --fixture test-harness/testdata/repeat` | attempt artifactから結果を再計算可能。m2 |
| skip-repeat-contract | T2 contract | `scripts/check-test-skips.sh && scripts/repeat-changed-tests.sh --repeat 10` | 未登録/期限切れ/単発失敗/timeoutを拒否。m2 |
| tampering-contract | T2 contract | `scripts/check-harness-tampering.sh --fixtures test-harness/testdata/tampering` | protected変更はreview-required、自己承認不可。m3 |
| independent-static-pin | T2 contract | `cd src && go test ./gorules` | checker、workflow呼出し、manifest、CODEOWNERSの欠落を独立検出。m3/m6 |
| mutation-pilot | T2 contract | `scripts/run-mutation-pilot.sh --seed 20260711 --timeout 15m` | 全must-kill、baseline以上、timeoutなし。m4 |
| profile-wired | T1 wired | `scripts/run-verification-profile.sh --fixture save` | command、duration、省略理由をartifact化。m5 |
| pr-full | T0-T2 | `scripts/run-verification-profile.sh pr` | 全PR gateが成功しprofile driftなし。m6 |
| fidelity-nightly | T3 fidelity | `scripts/run-verification-profile.sh nightly` | real binary/API/browserを同一入力で比較し必須case skipなし。m6 |

構造規則 → 検証手段:

- pure policy coreがI/Oを直接生成しない → `go test ./gorules` のimport/static scan。
- workflowは共有commandだけを呼ぶ → `static_enforcement_test.go` のliteral/semantic assertion。
- checker自己削除を別機構がpinする → merge-base版checkerとmerge-base契約からの`gorules`相互pin negative fixture。head内の二機構だけを信頼基点にしない。
- repo外required reviewが有効であること → 本計画では機械保証しない外部 prerequisite。repo内では `CODEOWNERS` と `review-required` artifactまでを検証する。


{% transition from="draft" to="active" date="2026-07-11" %}
実装開始
{% /transition %}


{% transition from="active" to="done" date="2026-07-11" %}
M2-M6実装と全Go/Web/policy/mutation/docs gate完了
{% /transition %}

````
