---
id: adr-20260711-grok-home-automation-policy
kind: adr
title: Grok home and automation policy
status: proposed
created: '2026-07-11'
decision_makers: [agent-grid maintainers]
tags: [grok, automation]
owners: []
relations:
- {type: partOf, target: plan-20260711-grok-driver}
source_paths: []
summary: GROK_HOME を保存境界とし automation launch で auto-update を抑止する
---

## Context

{% context %}resume は `$GROK_HOME/sessions` の継続性を必要とする一方、ユーザーの `config.toml` と credentials は CLI 所有資産である。automation 中の auto-update は同じ launch の再現性を失わせる。公式 contract は `--no-auto-update` を提供する。{% /context %}

## Decision

{% decision %}host/container launch は `GrokHomeResolver` と mount planner を介して同一 `GROK_HOME` namespace を使い、config/session file の生成・移行・上書きをしない。agent-grid が生成する automation argv には `--no-auto-update` を一度だけ付ける。real test は isolated temporary `GROK_HOME` のみ使用する。{% /decision %}

## Consequences

{% consequence kind="positive" %}cold resume と再現可能な automation を両立し、ユーザー config 非変更を filesystem contract と T3 で検証できる。{% /consequence %}
{% consequence kind="negative" %}container mount/permission failure を区別する home seam と、installed CLI が flag を受理する fidelity backstop が必要になる。{% /consequence %}
{% consequence kind="neutral" %}既存 `--no-auto-update` は重複除去でエラーを消す (i)。inaccessible home は外部 failure として停止理由を公開する (iii)。user config の内容は読み取り・正規化しない。{% /consequence %}

## Alternatives

config の update setting を agent-grid が書き換える案は user ownership に反するため却下する。auto-update を許容して version だけ記録する案は実行中 drift を防げない。ユーザー home で実機 test する案は既存資産を壊すため却下する。
