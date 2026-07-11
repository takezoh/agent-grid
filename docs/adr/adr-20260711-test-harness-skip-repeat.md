---
id: adr-20260711-test-harness-skip-repeat
kind: adr
title: Centralize skip inventory and deterministic repeat policy
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
summary: skip台帳と変更test repeatの選択・判定・artifact契約を一元化する
updated: '2026-07-11'
---

## Context

skip 数だけの pin では理由や期限を監査できず、repeat の対象選択・回数・途中失敗の扱いが暗黙だと同じ diff でも結果が変わる。nightly job 単位の成功は case-level skip-green を隠す。

## Decision

中央 JSON inventory を skip の SSOT とする。repeat selectorは`git diff --name-status <merge-base>...HEAD`を入力とし、Goは変更`_test.go`と同package、非test Go変更は同package全test、TSは変更test fileと変更sourceの同directory/import sibling testを選ぶ。renameは旧新pathの和、deleteは旧path対応を選ぶ。空集合・mapping不能はGo全packageとWeb全testへ昇格する。path/test名sort、seed 20260711、固定10回とし、1失敗・timeout・途中終了をfail closedにする。

## Consequences

### Positive
{% consequence kind="positive" %}skip と flaky 判定を artifact から再計算でき、silent skip-green を T2 contract と nightly T3 の両方で検出できる。selector pure core と fake command runner により T0/T1 検証が可能になる。{% /consequence %}

### Negative
{% consequence kind="negative" %}既存 skip の初回棚卸しと期限更新に保守コストが生じ、repeat は pre-push 時間を増やす。{% /consequence %}

### Neutral
{% consequence kind="neutral" %}source 近傍 helper への一括移行は行わず、inventory が source location を参照する。{% /consequence %}

## Alternatives

**source 近傍コメント。** 言語別 parser と表現 drift を増やすため却下する。

**単発 rerun。** 低頻度 flaky を見逃すため却下する。

**対象不明時 skip。** green の意味を壊すため却下する。


{% transition from="proposed" to="accepted" date="2026-07-11" %}
独立Verify 3項目pass、実装指示に基づき承認
{% /transition %}
