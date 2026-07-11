---
id: adr-20260711-test-harness-anti-tampering
kind: adr
title: Escalate protected harness changes through repository ownership
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
summary: protected manifest、CODEOWNERS、独立static pinでharness自己改ざんを人間reviewへ昇格する
updated: '2026-07-11'
---

## Context

変更主体が理由を追加するだけで protected 変更を成功にできれば自己承認になる。また diff checker 自身や workflow invocation を PR head から削除すると、同じ checker だけでは自己削除を観測できない。repo 外 branch protection は本計画から変更・保証できない。

## Decision

trusted baseline は PR head が同じ差分で変更できない Git merge-base tree とする。CI bootstrap は merge-base 版 checker を一時materializeして head を検査する。merge-base 版 checker は head の checker、`static_enforcement_test`、workflow invocation、manifest、CODEOWNERSを検査し、merge-base 側static contractはhead checker invocationを相互pinする。workflow invocationはmerge-base workflowのrequired command集合とhead workflowを比較する。checker・static test・invocationの同時削除もmerge-base実装が観測する。

例外 request は理由、owner、expiry、evidence を必須とするが成功には変換せず review-required で人間 review へ昇格する。mutation baselineを含むprotected manifestの同時変更もmerge-base値を判定基準にする。repo外required-check設定は変更しないが、merge-base checkoutを使うtrusted bootstrapがrequired環境で実行されることを外部prerequisiteとして明示する。

## Consequences

### Positive
{% consequence kind="positive" %}自己承認と checker 自己削除をPR差分外のmerge-base実装で検出し、checker・static test・workflow invocationの同時削除も拒否できる。diff classifier は T0、base/head fixture repository は T1、相互pin負例は T2 で検証できる。{% /consequence %}

### Negative
{% consequence kind="negative" %}正当な test 整理や floor 変更も review-required になり、repo 外 required review が無効なら最終的な merge 阻止は保証できない。{% /consequence %}

### Neutral
{% consequence kind="neutral" %}GitHub branch protection、ruleset、reviewer 権限は非目標であり、外部 prerequisite として残る。{% /consequence %}

## Alternatives

**理由 file があれば success。** 自己承認できるため却下する。

**manifest hash だけ。** rename や意味カテゴリを説明しにくいため diff 分類と併用する。

**checker 単体で自己保護。** checker 削除時に実行不能になるため却下する。


{% transition from="proposed" to="accepted" date="2026-07-11" %}
独立Verify 3項目pass、実装指示に基づき承認
{% /transition %}
