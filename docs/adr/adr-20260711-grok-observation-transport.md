---
id: adr-20260711-grok-observation-transport
kind: adr
title: Grok observation transport
status: proposed
created: '2026-07-11'
decision_makers:
- agent-grid maintainers
tags:
- grok
- driver
- terminal
owners: []
relations:
- {type: partOf, target: change-20260711-grok-driver}
source_paths: []
summary: Grok TUI は PTY、状態観測は process lifecycle と確認済み same-process signal に限定する
---

## Context

{% context %}Grok は interactive TUI と `grok agent stdio` ACP を公開するが、ACP は TUI の観測 sidecar ではなく別 agent process である。初回 Driver の目的は interactive TUI を既存 terminal surface で安全に起動・再開することであり、別 process の状態を TUI session の状態として扱ってはならない。{% /context %}

## Decision

{% decision %}interactive `grok` TUI は既存 PTY/CLI subsystem の単一 process として起動する。status は process lifecycle の idle/running/stopped/failed と、実機 characterization で同一 process の明示契約と確認できた OSC/window-title signal（存在する場合）だけから得る。`grok agent stdio` ACP process/bind は起動しない。VT 本文から identity/model/effort/rich status を推測しない。{% /decision %}

## Consequences

{% consequence kind="positive" %}別 agent の状態混入を防ぎ、既存 PTY lifecycle と fake process seam を再利用できる。未確認 signal が無くても coarse status は決定論的に検証できる。{% /consequence %}
{% consequence kind="negative" %}初回 Driver は turn/tool/approval の rich structured status を提供せず、terminal signal が確認できない場合は process lifecycle のみとなる。{% /consequence %}
{% consequence kind="neutral" %}model/effort は launch seed と persisted state を表示する。同一 process signal の追加は characterization と contract test を通した場合だけ行う。外部 process failure は回復対象 (iii) とする。{% /consequence %}

## Alternatives

`grok agent stdio` を TUI sidecar として起動する案は別 process のため却下する。ACP を primary UI transport として TUI を置換する案は将来の別設計とする。session store parser と VT text parsing は非公開 format または推測になるため却下する。
