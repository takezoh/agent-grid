---
id: adr-20260711-test-harness-mutation-pilot
kind: adr
title: Use checked-in deterministic mutants for the mutation pilot
status: accepted
created: '2026-07-11'
decision_makers:
- unknown
tags: []
owners: []
relations:
- {type: partOf, target: plan-20260711-test-harness-north-star}
- {type: references, target: spec-20260711-test-harness-north-star}
source_paths: []
summary: 固定operator・seed・timeout・baselineでcritical pathの検出力を再現可能に測る
updated: '2026-07-11'
---

## Context

coverage は test が壊れた振る舞いを検知するかを示さない。一方、汎用 mutation engine を全体導入すると新依存、非決定性、PR 時間増大が先行する。pilot には score を再現できる operator、seed、timeout、baseline 契約が必要である。

## Decision

stream routing、state.Reduce、Go-TS wire codec だけに checked-in deterministic mutant manifest を使うことにする。operator set は conditional-negation、route-target-substitution、event-drop、codec-field-omission、mutant identityはpath、byte span、operator、normalized source SHA-256、runner identityはrunner source SHA-256、seedは20260711とする。対象ごと5分・全体15分、timeoutはsurvive扱いとする。

baseline manifestはmutant identity、must-kill、最小score、operator-set version、runner hashを保持する。更新には新旧artifact、survivor理由、owner、expiry、evidenceを要求しanti-tamperingのprotected changeとしてreview-requiredにする。mutantとbaselineを同時変更してもmerge-base baselineで判定する。artifactには全identity、seed、timeout、baseline hash、toolchain identity、再現commandを含める。

## Consequences

### Positive
{% consequence kind="positive" %}同一 commit と seed で mutant 集合と score を再現でき、critical path の検出力低下を T2 gate で検出できる。runner/parser は fake command seam で T0/T1 検証できる。{% /consequence %}

### Negative
{% consequence kind="negative" %}operator 網羅性は汎用 engine より狭く、production code 変更時に manifest 保守が必要になる。{% /consequence %}

### Neutral
{% consequence kind="neutral" %}pilot 成功は全 project の mutation 品質を意味せず、対象拡大や第三者 engine 採用は別判断とする。{% /consequence %}

## Alternatives

**Go/TS 汎用 mutation tool。** 二言語の新依存と version drift を pilot 前に持ち込むため却下する。

**random mutation 生成。** seed だけでは tool/version 差を抑えられないため却下する。

**coverage のみ。** 検出力を観測できないため却下する。


{% transition from="proposed" to="accepted" date="2026-07-11" %}
独立Verify 3項目pass、実装指示に基づき承認
{% /transition %}
