---
id: adr-20260814-cli-selector-kinds-id-and-prefix
kind: adr
title: Restrict CLI session selectors to the full identifier and an identifier prefix
status: accepted
created: '2026-08-14'
decision_makers:
- unknown
tags:
- native-clients
- cli
owners:
- agent-grid-maintainers
relations:
- {type: originatedFrom, target: change-20260812-cli-terminal-client}
source_paths:
- clients/cli/internal/selector
consequences:
  positive:
  - Every selector kind is grounded in a value the server guarantees to be stable
    and unique, so resolution is reproducible across runs.
  negative:
  - Users cannot address a session by a human-friendly name, which is the ergonomics
    the upstream plan hoped for.
  neutral:
  - The reduction is the invalidation behaviour the upstream plan recorded for its
    own assumption, fired by measurement rather than chosen by design.
confirmation: A test asserts a declared kind decides resolution even when the other
  kind would match a single session, and no code path falls back between kinds.
summary: Selector kinds are the full session identifier and an identifier prefix,
  because measurement shows the public payload carries no stable operator-assigned
  name field.
updated: '2026-08-14'
---

# Restrict CLI session selectors to the full identifier and an identifier prefix

## Context

The upstream experience contract settled that a selector kind is always declared explicitly and never falls back to another kind. It left the *set* of kinds contingent, recording an assumption that a stable operator-assigned name field might exist on the public payload, with an explicit invalidation behaviour: if it does not exist, the kinds reduce to two.

Measurement resolves the assumption negatively. `proto.SessionInfo` has no `name` field. The name-like values that do exist — `view.card.title` and `view.display_name` — are mutable and non-unique: the card title is server-owned and changes as the session's work changes, and the display name is the driver name (`codex` and similar), shared by every session of that driver. Neither can ground identity, and a selector keyed on either would resolve differently between two runs with no user action in between.

## Decision

Selector kinds are the full session identifier and an identifier prefix. A selector resolves only under its declared kind, with no implicit fallback to the other.

## Consequences

{% consequence kind="positive" %}
Both kinds are grounded in a value the server guarantees stable and unique, so the same selector resolves the same way across runs and automation can branch on the outcome. The prefix kind keeps the ergonomics of short identifiers without introducing a second identity space.
{% /consequence %}

{% consequence kind="negative" %}
A session cannot be addressed by anything human-friendly. That is a real ergonomics loss relative to what the upstream plan hoped for, and it is a loss caused by the payload, not by this decision.
{% /consequence %}

{% consequence kind="neutral" %}
Reopening this needs a public-contract change that adds a stable, unique, operator-assigned name field to the session payload — not a client change. The upstream ambiguity behaviour is unaffected: two or more matches remain an ambiguous failure that lists distinguishable candidates and selects none.
{% /consequence %}

## Alternatives

- **Add a name kind keyed on `view.card.title`** — rejected: the title is mutable and non-unique, so the selector's meaning changes without user action.
- **Add a name kind keyed on `view.display_name`** — rejected: it is the driver name, shared by every session of that driver, so it identifies a class rather than a session.
- **Maintain a client-side name-to-identifier map** — rejected: it creates a second identity space that no other client shares, and it would disagree with the graphical clients about what a session is called.

Confirmation: `TestDeclaredKindDecidesResolution` — a string that is one session's full identifier and simultaneously a prefix of another's resolves to one session when the id kind is declared, and reports two candidates with a non-zero exit when the prefix kind is declared.

## Related

- requirements: `FR-020`, `FR-021`, `FR-024`
- contracts: `contract-selector-resolution`
- plan: `docs/changes/change-20260812-cli-terminal-client/design-plan/design.md` anchor `adr-cli-selector-kinds-reduced-to-id-and-prefix`


{% transition from="proposed" to="accepted" date="2026-08-14" %}
consultation-20260814-cli-terminal-client-design-final
{% /transition %}
