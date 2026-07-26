---
id: adr-20260726-multi-source-driver-status-evidence
kind: adr
title: Keep driver status observations separate and resolve one canonical status
status: accepted
created: '2026-07-26'
decision_makers:
- user
tags: []
owners: []
relations:
- {type: references, target: adr-20260726-shared-status-display-matrix}
source_paths: []
summary: hook・terminal OSC・process lifecycle を独立した観測として保持し、Driver ごとの純粋な resolver
  で Codex ベースの canonical status を決定する。Codex は app-server のみを状態根拠とする。
updated: '2026-07-26'
---

<!-- 本文セクション構成は利用側の方法論 (例: design/PRINCIPLES.md の Nygard 4 節 Context/Decision/Consequences/Alternatives) が規定する。
     dev-docs skill 自体はセクション構成を強制しない。Markdoc tag `{% context %}` `{% decision %}` `{% consequence kind="positive" %}` を任意で本文に bind できる。 -->

<!-- CONSEQUENCES: Nygard 対称ルール — positive / negative / neutral の 3 極を本文でも書き出す (skills/design/knowledge/artifact-semantics.md §18 / adr-20260709-adr-consequences-tripolar)。
     frontmatter の consequences{positive[], negative[], neutral[]} が SoT。spec-detail 導入以降 (created >= 2026-07-09) の新規 ADR は三極必須。

  {% consequence kind="positive" %} ... {% /consequence %}
  {% consequence kind="negative" %} ... {% /consequence %}
  {% consequence kind="neutral" %}  ... {% /consequence %}
-->

<!-- OPTIONAL: `confirmation` block (MADR 5.0 由来、spec-detail v1、skills/design/knowledge/artifact-semantics.md §18)。
     この decision が守られていることを機械的に確認する手段 (fitness function / grep / test) を書く。
     frontmatter `confirmation:` (string) が SoT。使うときはこのコメントを削除して次のブロックを埋める。

  {% confirmation %} ... {% /confirmation %}
-->

## Context

Claude and legacy terminal Drivers previously let hook events and OSC title events
overwrite the same `CommonState.Status`. Missing or reordered completion
events could therefore leave a session stuck Running.

Terminal output also carries signals with different semantics: OSC title is a
status presentation, OSC 9;4 is progress, OSC 9/99/777 and bare BEL are
transient notifications. Agent Grid currently collapses OSC command 9 into one
notification path. Codex is different: app-server events are its complete
authoritative lifecycle source and terminal OSC must not be reintroduced as a
fallback.

## Decision

Keep source observations separate from the canonical displayed status.

Each Driver owns only observations appropriate to its characterized CLI:

- hook lifecycle and human-input observation;
- terminal title observation;
- OSC 9;4 progress observation;
- process lifecycle observation;
- terminal notification observation, which is transient and never a lifecycle
  state by itself.

Custom Claude hook `terminalSequence` is not a lifecycle input. It may appear
in the same PTY stream, so OSC remains corroborating terminal evidence rather
than an authoritative hook event.

A pure Driver-specific resolver computes the one shared canonical status used
by all clients. It applies these rules:

1. stopped/failed process lifecycle is terminal and wins;
2. a current unresolved human-input observation wins over running;
3. a newer explicit waiting/ready observation clears stale Running;
4. elapsed time alone never changes status; only an explicit observation can
   end Running;
5. notification evidence never creates or preserves Running/Pending;
6. unsupported evidence is ignored rather than converted through a timer.

Source timestamps are compared only within a source unless they share the
Agent Grid ingestion clock. The resolver receives normalized observations and
contains no I/O.

Codex remains a separate specialization: only app-server lifecycle and request
events populate status observations. Codex OSC/title/progress/BEL is ignored
for status and is not a fallback.

The sources are complementary, not interchangeable. Hook/app-server events
identify semantic boundaries and the reason human input is required. Native
OSC title/progress closes gaps in hook coverage and reports TUI presentation
state. Process lifecycle alone owns Stopped. Notification OSC/BEL alone owns
the one-shot bell and cannot infer lifecycle or action resolution.

OSC parsing must classify OSC 9 notification versus OSC 9;4 progress before
observations reach a Driver.

## Consequences

Positive:

- A missing Stop/AfterAgent hook can be corrected by newer explicit terminal
  evidence instead of leaving Running forever.
- Conflicts become inspectable because raw observations survive status
  projection.
- All clients retain the accepted Codex-derived display vocabulary and icons.
- Tests can cover the resolver as a pure state table and cover each external
  source with fake/contract/fidelity layers.

Negative:

- Driver state gains ephemeral source-specific observation fields. Persistence
  continues to store the canonical status; restored Running is demoted on Tick
  unless fresh evidence arrives.
- Each supported CLI/version needs a characterized event mapping.
- Existing tests that assert direct event-to-Status mutation must move to
  event-to-observation plus resolver assertions.

Neutral:

- Native terminal combinations remain CLI-specific; Agent Grid does not copy
  Grok’s simultaneous spinner and Action Required presentation.
- Notifications remain useful for logs/toasts but do not become durable session
  states.

## Alternatives considered

### Keep last-writer-wins on one Status

Rejected. It loses provenance, compares unrelated clocks implicitly, and
cannot recover reliably when one source omits its closing event.

### Make OSC authoritative for every terminal CLI

Rejected. OSC can be disabled, filtered by tmux, or changed by configuration.
It also conflates title, progress, and notification protocols. Codex already
has a stronger app-server source.

### Ignore OSC and trust hooks only

Rejected for non-Codex Drivers. The observed Running-stuck failure is exactly a
missing closing hook, while Claude and Grok emit independently derived terminal
state. OSC is valuable corroborating evidence when stored with provenance.

## Confirmation

- OSC parser contract tests distinguish `9;<message>` from
  `9;4;<state>;<progress>` and bare BEL.
- Driver table tests exercise missing completion hook, reordered sources,
  human-input precedence, explicit terminal completion, and process exit.
- Codex conformance tests prove OSC cannot change Codex status.
- Cross-client parity continues to consume the shared semantic status matrix.

## Driver detection contract

| Driver | Hook/app-server responsibility | OSC responsibility | Process responsibility | Remaining real scenario gap |
|---|---|---|---|---|
| Codex | app-server turn start/end → Running/Waiting; unresolved approval, question, elicitation → Pending; their explicit resolution clears Pending | none for status | exit → Stopped | none currently identified |
| Claude | prompt/tool start and continuation → Running; normal Stop → Waiting; `PermissionRequest`/`Elicitation` → Pending; `PermissionDenied`/`ElicitationResult` resolve directly, while the next continuation/Stop recovers an allowed permission | native Braille/`✳` title → Running/Waiting, including the measured Esc-interrupt close missing from Stop hook; characterized OSC 9;4 may corroborate Running/Waiting | exit → Stopped | none currently identified; a dedicated `PermissionGranted` event is unnecessary because later explicit lifecycle evidence safely recovers the projected state |
| agy | dedicated Driver consumes managed passive `PreInvocation` → Running, `PostInvocation`/`PostToolUse` → Running, and non-stale conformant `Stop{fullyIdle:true}` → Waiting; `invocationNum`/`executionNum` prevent delayed Stop from clearing newer work; PreToolUse remains excluded because its required decision changes gating policy | none characterized: OSC 0/9/9;4 remained absent during startup, Running, Esc, and permission prompt captures, including a Windows Terminal environment | exit → Stopped | Esc cancels the Stop-hook context before command `execve`; native permission-wait onset lacks a policy-neutral hook event |
| Grok | no characterized hook contract | Braille title or active OSC 9;4 → Running; exact idle title or clear progress → Waiting; approval/turn notifications → bell only | exit → Stopped | approval-required is a source-confirmed real flow; its blinking title and post-approval resolution sequence have not yet been characterized, so canonical Pending is not emitted |
| generic/shell | shell uses its explicit prompt/command protocol; generic has no hook contract | only protocols characterized by that Driver; arbitrary output is not a lifecycle boundary | exit → Stopped | not an agent-flow coverage target |

Only a product flow confirmed by source, official contract, or live capture is
a coverage gap. Hypothetical suppression of every signal is an infrastructure
failure condition, not a user scenario, and is not listed as a Driver
requirement. For a confirmed flow whose event sequence is not yet
characterized, the resolver must not invent Waiting/Pending from elapsed time,
output activity, or a bell.

The Gemini-family terminal integration in this contract is Antigravity CLI
(`agy`) only.

For Claude, a native idle title immediately supersedes earlier hook Running
evidence. This covers user interruption, for which Claude's official hook
contract does not emit Stop. Elapsed time never changes the projected status.
Action Required from a structured hook remains latched until a later explicit
structured event resolves it. Grok’s blinking Action Required title has no
characterized explicit resolution edge, so it remains notification evidence
and is not projected to Pending.


{% transition from="proposed" to="accepted" date="2026-07-26" %}
User directed unified OSC/hook classification and canonical Agent Grid status conversion
{% /transition %}
