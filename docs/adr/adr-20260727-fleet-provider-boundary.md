---
id: adr-20260727-fleet-provider-boundary
kind: adr
title: Fleet provider auth proxy and single-server adapter
status: accepted
created: '2026-07-27'
decision_makers:
- unknown
tags: []
owners: []
relations:
- {type: originatedFrom, target: change-20260727-multi-server-session-palette}
source_paths: []
summary: Electron main owns authenticated fleet routing; browser adapts one gateway
  to the same provider contract.
---

<!-- 本文セクション構成は利用側の方法論 (例: design/PRINCIPLES.md の Nygard 4 節 Context/Decision/Consequences/Alternatives) が規定する。
     dev-docs skill 自体はセクション構成を強制しない。Markdoc tag `{% context %}` `{% decision %}` `{% consequence kind="positive" %}` を任意で本文に bind できる。 -->

<!-- CONSEQUENCES: Nygard 対称ルール — positive / negative / neutral の 3 極を本文でも書き出す (skills/design/knowledge/artifact-semantics.md §18 / adr-20260709-adr-consequences-tripolar)。
     frontmatter の consequences{positive[], negative[], neutral[]} が SoT。spec-detail 導入以降 (created >= 2026-07-09) の新規 ADR は三極必須。

  {% consequence kind="positive" %} ... {% /consequence %}
  {% consequence kind="negative" %} ... {% /consequence %}
  {% consequence kind="neutral" %}  ... {% /consequence %}
-->

## Context

Electron owns fleet credentials while browser has one gateway; both must expose the same server-qualified provider behavior.

## Decision

Electron main owns authenticated REST/WS fan-out, generation fencing, redirect rejection, and cleanup. Preload exposes typed projections without credentials. Browser derives `serverId` from the RFC3986-canonical gateway origin and adapts it to the same provider contract. Authorization is never replayed across origin, downgrade, or indeterminate redirects.

## Consequences

Token and routing policy have one authority; main-process IPC/stream lifecycle becomes an explicit implementation responsibility; browser remains a one-member fleet.

## Alternatives

Renderer fan-out was rejected because it distributes credentials and fleet policy. Separate browser metadata identity was rejected because identical origins could split identity.

<!-- OPTIONAL: `confirmation` block (MADR 5.0 由来、spec-detail v1、skills/design/knowledge/artifact-semantics.md §18)。
     この decision が守られていることを機械的に確認する手段 (fitness function / grep / test) を書く。
     frontmatter `confirmation:` (string) が SoT。使うときはこのコメントを削除して次のブロックを埋める。

  {% confirmation %} ... {% /confirmation %}
-->
