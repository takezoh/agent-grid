---
id: adr-20260727-typed-launch-profile-schema
kind: adr
title: Typed launch profile schema
status: accepted
created: '2026-07-27'
decision_makers:
- unknown
tags: []
owners: []
relations:
- {type: originatedFrom, target: change-20260727-multi-server-session-palette}
source_paths: []
summary: Server and project settings resolve typed profiles by stable ID with deterministic
  revisions and no path allow-list.
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

Project-specific commands, arbitrary parameters, and deterministic revisions need one model without a parallel string-command authority.

## Decision

The only launch-command setting is `session.command_profiles`, accepted in both `~/.agent-grid/settings.toml` and `<project>/.agent-grid/settings.toml`. Parameters support `enum`, `boolean`, `string`, and `number`. Project settings inherit unspecified server profiles, replace the complete descriptor on the same ID, append new IDs, and remove inherited IDs with `{id, disabled=true}`. Path allow-lists are forbidden. Duplicate IDs and invalid schemas fail config loading. Revision is SHA-256 of the resolved descriptor. `[session].commands` and `[session].default_command` are removed rather than normalized or retained as fallbacks.

## Consequences

Validation and stale detection are deterministic and there is one configuration authority. Existing installations must migrate before upgrading; this is intentional because retaining two command models would preserve ambiguity.

## Alternatives

Legacy normalization, partial field merge, client-side aliases, and execution-only revision inputs were rejected because they create parallel authorities or ambiguous execution.

<!-- OPTIONAL: `confirmation` block (MADR 5.0 由来、spec-detail v1、skills/design/knowledge/artifact-semantics.md §18)。
     この decision が守られていることを機械的に確認する手段 (fitness function / grep / test) を書く。
     frontmatter `confirmation:` (string) が SoT。使うときはこのコメントを削除して次のブロックを埋める。

  {% confirmation %} ... {% /confirmation %}
-->
