---
id: adr-20260727-server-qualified-identity-partial-failure
kind: adr
title: Server-qualified identity and scoped partial failure
status: accepted
created: '2026-07-27'
decision_makers:
- unknown
tags: []
owners: []
relations:
- {type: originatedFrom, target: change-20260727-multi-server-session-palette}
source_paths: []
summary: Compound identities and generation-fenced per-server outcomes preserve healthy
  scopes and recoverable selection.
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

Colliding local IDs and partial fleet failures must not misroute operations or erase healthy work.

## Decision

Project/session identity is `(serverId, localId)` end-to-end. ADR-0041 remains authoritative: each server exposes one versioned `GET /api/session-config` snapshot. Per-server results carry typed failure and retry. Electron runs at most four REST calls, times each out at ten seconds, retains only the latest generation, and cancels/cleans work on supersede, removal, or close; outcomes converge within one second after timeout.

## Consequences

Healthy scopes remain usable and resource lifetime is bounded; resource-scoped retry inside one server snapshot is unavailable; all projections must preserve compound keys.

## Alternatives

Split project/profile endpoints, global fail-fast, and unqualified local identity were rejected.

<!-- OPTIONAL: `confirmation` block (MADR 5.0 由来、spec-detail v1、skills/design/knowledge/artifact-semantics.md §18)。
     この decision が守られていることを機械的に確認する手段 (fitness function / grep / test) を書く。
     frontmatter `confirmation:` (string) が SoT。使うときはこのコメントを削除して次のブロックを埋める。

  {% confirmation %} ... {% /confirmation %}
-->
