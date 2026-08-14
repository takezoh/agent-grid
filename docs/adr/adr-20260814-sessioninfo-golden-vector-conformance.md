---
id: adr-20260814-sessioninfo-golden-vector-conformance
kind: adr
title: Pin the session payload shape the CLI reads with a server-tested golden vector
status: accepted
created: '2026-08-14'
decision_makers:
- unknown
tags:
- protocol
- native-clients
- testing
owners:
- agent-grid-maintainers
relations:
- {type: originatedFrom, target: change-20260812-cli-terminal-client}
- {type: references, target: adr-20260724-protocol-cross-language-sdks-supersedes-0021}
- {type: references, target: adr-20260726-server-owned-session-card-title}
source_paths:
- protocol/fixtures
- services/host/state/view/view.go
- clients/cli/internal/gateway
consequences:
  positive:
  - Renaming or moving a session payload field the CLI reads fails a test on the producing
    side instead of breaking a client silently.
  negative:
  - A deliberate field rename now costs an extra edit, and one CLI-facing test lives
    inside the services module.
  neutral:
  - The stronger fix — a protocol schema entry plus SDK generation — is recorded as
    a follow-up rather than taken, because it changes the generated surface of four
    languages.
confirmation: The server's serializer must reproduce the checked-in golden vector,
  the CLI's decoder must consume the same file, and an enumerating gate must fail
  when a consumer is added without registering it.
summary: Capture a canonical session payload as a checked-in golden vector that both
  the server serializer and the CLI decoder are tested against, so a server-side rename
  of a field the CLI reads fails a gate.
updated: '2026-08-14'
---

# Pin the session payload shape the CLI reads with a server-tested golden vector

## Context

The CLI reads several fields off the session payload: the identifier, the card title it renders verbatim as the display name, the working-directory field, and the status object. Measurement shows none of that shape is pinned anywhere. `SessionInfo` and `View` appear in neither `protocol/*.schema.json` nor `clients/sdk/quicktype-emit.json`; the TypeScript side is a hand mirror, and no Go SDK is generated at all.

The status matrix is pinned to a canonical file, which is what makes labels safe. The fields taken from `view.card` and the working-directory field have no equivalent.

The failure mode this creates is specific and quiet. A CLI-side test asserting "the display name is the card title" proves only that the CLI reads the field it thinks it reads. Renaming `view.card.title` on the server leaves that test green while the CLI's display name silently becomes empty — every gate in the repository passes and the client is broken.

## Decision

A canonical session payload — carrying at minimum the identifier, the card title, the working-directory field and the status object — is captured once as a checked-in golden vector.

Two tests read it. The server's own serializer must reproduce it, and the CLI's decoder must consume it. The vector is registered in the repository's enumerating gate, so adding a consumer without registering it fails.

The vector is captured during the contract-inventory phase, from a real server, before any consumer depends on it.

## Consequences

{% consequence kind="positive" %}
A rename or a move of a field the CLI reads fails on the **producing** side, which is the only place a test can catch it. The vector also gives the CLI decoder a real payload to test against instead of a convenience shape written by hand, which is the same drift the fake-versus-real discipline exists to prevent.
{% /consequence %}

{% consequence kind="negative" %}
A deliberate field rename now costs one extra edit — updating the vector in the same change — which is the intended cost but is still friction. One test that exists for a client's benefit lives inside the services module, which is a small ownership smell justified by the fact that only a producer-side test can fail on a producer-side rename.
{% /consequence %}

{% consequence kind="neutral" %}
The golden vector complements rather than replaces the fake-versus-real backstop: the backstop exercises frame sequences, the vector exercises field names. Nothing about the wire format changes.
{% /consequence %}

## Alternatives

- **Add `SessionInfo` to `protocol/*.schema.json` and register it in the SDK emitter** — strictly stronger, and it would cover the TypeScript hand mirror as well. Rejected for this change because it changes the generated surface of four languages and warrants its own decision; recorded as the preferred follow-up.
- **Import the Go types from the services module** — rejected: it creates the dependency edge the CLI module boundary exists to forbid.
- **Rely on the fake-versus-real backstop alone** — rejected: it compares frame behaviour, not payload field names, and a rename that both sides of the fake agree on passes it.
- **Assert the mapping only in the CLI** — rejected: that is the current situation, and it is exactly what stays green while the client breaks.

Confirmation: `TestSessionInfoGoldenVectorIsStable` in `services/host/state/view` (the server's serializer reproduces the checked-in vector), `TestDecodesCheckedInSessionInfoGoldenVector` in the CLI gateway package, and `TestSessionInfoGoldenVectorIsRegistered` in `services/gorules` (the enumerating gate covers it).

## Related

- requirements: `NFR-008`, `FR-018`
- contracts: `contract-wire-shape-conformance`
- references: `adr-20260724-protocol-cross-language-sdks-supersedes-0021` (why no Go SDK is generated), `adr-20260726-server-owned-session-card-title` (why the title is server-owned)
- plan: `docs/changes/change-20260812-cli-terminal-client/design-plan/design.md` anchor `adr-cli-sessioninfo-golden-vector-conformance`


{% transition from="proposed" to="accepted" date="2026-08-14" %}
consultation-20260814-cli-terminal-client-design-final
{% /transition %}
