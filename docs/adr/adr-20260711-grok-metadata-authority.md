---
id: adr-20260711-grok-metadata-authority
kind: adr
title: Grok metadata authority
status: proposed
created: '2026-07-11'
decision_makers: [agent-grid maintainers]
tags: [grok, metadata]
owners: []
relations:
- {type: partOf, target: plan-20260711-grok-driver}
source_paths: []
summary: Grok session metadata の source priority と tri-state を固定する
---

## Context

{% context %}初回 Driver の model/effort/session は launch argv と persisted state から現れる。terminal 本文は metadata の公開契約ではなく、別 process の ACP は TUI session の authority になれない。実機 characterization 済みの同一 process signal が存在する場合だけ status source を追加できる。{% /context %}

## Decision

{% decision %}session/model/effort は persisted authoritative state を restore 正本、launch argv を seed として unset/set/cleared と source を保持する。status は process lifecycle と characterization 済み same-process signal のみ更新できる。VT 本文と ACP は source に含めない。{% /decision %}

## Consequences

{% consequence kind="positive" %}clear 後の古い model/effort 復活を防ぎ、既存 `drivertest.MetadataSourcePriority` を再利用できる。reducer は pure T0 で検証可能になる。{% /consequence %}
{% consequence kind="negative" %}runtime 中の TUI 内 model/effort 変更は、同一 process の明示 signal が確認されない限り Driver 表示へ反映できない。{% /consequence %}
{% consequence kind="neutral" %}metadata は seed/unknown を明示し、process exit は stopped/failed にする。未確認 signal は無視し、内部 authority 違反は fail fast (ii) とする。{% /consequence %}

## Alternatives

最後に観測した値を無条件採用する案は authority を失うため却下する。ACP を live authority にする案は別 agent process のため却下する。session file と VT text parsing は公開 contract/source provenance を保証できず却下する。
