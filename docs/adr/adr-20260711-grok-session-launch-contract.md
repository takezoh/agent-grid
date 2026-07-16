---
id: adr-20260711-grok-session-launch-contract
kind: adr
title: Grok session launch contract
status: proposed
created: '2026-07-11'
decision_makers:
- agent-grid maintainers
tags:
- grok
- session
owners: []
relations:
- {type: partOf, target: change-20260711-grok-driver}
source_paths: []
summary: fresh、continue、resume、fork の公式 Grok session flag 契約を分離する
---

## Context

{% context %}公式契約では fresh UUID は `--session-id`、current directory latest は `--continue`、ID resume は `--resume ID`、resume fork は `--fork-session` を併用する。同じ ID を生成と resume に使う設計や競合 flag の黙認は、既存会話の上書きや別会話への接続を招く。{% /context %}

## Decision

{% decision %}lifecycle を Fresh/ContinueLatest/ResumeID/ForkFromID の closed sum とし、それぞれ `--session-id UUID` / `--continue` / `--resume ID` / `--resume ID --fork-session` に一意に写像する。runtime の `UUIDSource` と `SessionLookup` が fresh UUID を確定し、pure `GrokLaunchPolicy` は注入値の検証と token-aware argv 構築のみ行う。競合は launch 前 typed error とする。{% /decision %}

## Consequences

{% consequence kind="positive" %}session lifecycle の不正組合せを表現しにくくし、fixed UUID の T0 matrix test と deterministic resume routing が可能になる。{% /consequence %}
{% consequence kind="negative" %}ユーザー提供 command の optional `--resume` 値を含む token-aware parse と、runtime 側 collision lookup seam が必要になる。{% /consequence %}
{% consequence kind="neutral" %}既存 flag と requested lifecycle が同一なら重複を消す (i)。矛盾は内部で推測せず fail fast (ii)、filesystem lookup failure は外部由来として診断する (iii)。{% /consequence %}

## Alternatives

Grok が生成した ID を後から採用する案は initial routing が未確定になるため却下する。`--session-id` で resume する案は公式意味論に反する。UUID の「未使用」判定を pure policy 内で行う案は I/O seam と state owner を曖昧にするため却下する。
