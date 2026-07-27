---
id: adr-20260727-server-argv-materialization
kind: adr
title: Server-owned argv materialization and stale validation
status: accepted
created: '2026-07-27'
decision_makers:
- unknown
tags: []
owners: []
relations:
- {type: originatedFrom, target: change-20260727-multi-server-session-palette}
source_paths: []
summary: A host/state pure service validates one immutable profile revision and emits
  shell-free executable/argv.
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

Create must never execute stale values or client-interpolated shell text, and must expose only one request shape.

## Decision

A pure launch-profile service owns immutable snapshot lookup, revision/value validation, defaults, and executable/argv production. Each argv template emits exactly one complete token; `{{name}}` references are replaced only after typed validation and the completed token is shell-quoted. `version=2` with `profile_id` and `profile_revision` is mandatory. Absent/unknown versions and the former `{project, command}` shape fail without creating a session.

## Consequences

API and runtime cannot disagree and raw client command text cannot cross the create boundary. Configuration is more explicit and upgrade requires deliberate migration.

## Alternatives

API/state split ownership, placeholders, partial-token expansion, and mutually inferred JSON shapes were rejected.

<!-- OPTIONAL: `confirmation` block (MADR 5.0 由来、spec-detail v1、skills/design/knowledge/artifact-semantics.md §18)。
     この decision が守られていることを機械的に確認する手段 (fitness function / grep / test) を書く。
     frontmatter `confirmation:` (string) が SoT。使うときはこのコメントを削除して次のブロックを埋める。

  {% confirmation %} ... {% /confirmation %}
-->
