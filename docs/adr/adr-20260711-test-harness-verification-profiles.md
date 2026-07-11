---
id: adr-20260711-test-harness-verification-profiles
kind: adr
title: Share repository verification profiles across local and CI entrypoints
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
summary: save/pre-push/PR/nightlyを共通repository commandへ集約する
updated: '2026-07-11'
---

## Context

local と CI が別々の command を持つと、速い feedback と正式 gate の内容が drift する。user config の無断変更や新 task runner 導入は避けつつ、save から nightly へ Tier を段階拡張する必要がある。

## Decision

save、pre-push、pr、nightly を checked-in profile として宣言し、単一 repository runner が既存 Make/script command を実行することにする。save は T0、pre-push は変更範囲 T0〜T2、pr は全 T0〜T2/race/fuzz/coverage/diff、nightly は T3 とする。workflow と opt-in hook は同じ runner の薄い caller にし、command、duration、result、省略理由を artifact 化する。

## Consequences

### Positive
{% consequence kind="positive" %}開発者と CI が同じ command を再利用し、profile drift を static contract で検出できる。selector/runner は T0/T1、profile 包含関係は T2、real nightly は T3 で検証できる。{% /consequence %}

### Negative
{% consequence kind="negative" %}共通 runner が新しい入口となり、既存 script 名変更時に profile 更新が必要になる。{% /consequence %}

### Neutral
{% consequence kind="neutral" %}hook 導入は opt-in で、user config は変更しない。既存 Makefile と toolchain を継続利用する。{% /consequence %}

## Alternatives

**workflow ごとに command 列挙。** local/CI drift と policy 重複を増やすため却下する。

**mise 等の新 runner。** 既存資産で要件を満たせるため却下する。

**hook を強制 install。** user config を無断変更するため却下する。


{% transition from="proposed" to="accepted" date="2026-07-11" %}
独立Verify 3項目pass、実装指示に基づき承認
{% /transition %}
