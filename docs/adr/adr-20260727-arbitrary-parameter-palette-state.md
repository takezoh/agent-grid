---
id: adr-20260727-arbitrary-parameter-palette-state
kind: adr
title: Arbitrary-N parameter palette form and pure state
status: accepted
created: '2026-07-27'
decision_makers:
- unknown
tags: []
owners: []
relations:
- {type: originatedFrom, target: change-20260727-multi-server-session-palette}
source_paths: []
summary: The palette renders ordered parameters in one accessible form while provider
  I/O remains outside pure state transitions.
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

The approved UX requires one accessible form for arbitrary N parameters with recoverable stale state.

## Decision

Provider I/O remains injected and state transitions pure. Ordered enum/boolean controls retain distinct absent/default/explicit states and parameter-addressed errors. Same-ID compatible-kind values and valid focus survive refresh. A 100-parameter fixture renders and updates within 100 ms in CI with O(N) state; virtualization, if used, preserves DOM-order navigation and accessible error relations.

## Consequences

Form behavior is model-driven and testable; UI optimization is bounded by accessibility; string/number controls are deferred.

## Alternatives

Per-command alias branches and an initial extensible renderer registry were rejected.

<!-- OPTIONAL: `confirmation` block (MADR 5.0 由来、spec-detail v1、skills/design/knowledge/artifact-semantics.md §18)。
     この decision が守られていることを機械的に確認する手段 (fitness function / grep / test) を書く。
     frontmatter `confirmation:` (string) が SoT。使うときはこのコメントを削除して次のブロックを埋める。

  {% confirmation %} ... {% /confirmation %}
-->
