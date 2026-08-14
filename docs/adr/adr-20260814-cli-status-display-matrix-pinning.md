---
id: adr-20260814-cli-status-display-matrix-pinning
kind: adr
title: Pin the shared status display matrix as canonical for the CLI consumer only
status: accepted
created: '2026-08-14'
decision_makers:
- unknown
tags:
- native-clients
- cli
- protocol
owners:
- agent-grid-maintainers
relations:
- {type: originatedFrom, target: change-20260812-cli-terminal-client}
- {type: references, target: adr-20260726-shared-status-display-matrix}
source_paths:
- protocol/session-status-display.json
- clients/cli/internal/view
consequences:
  positive:
  - A checked-in file that no code read becomes authoritative for at least one consumer,
    with a test that fails when the copy drifts.
  - The CLI cannot invent a state word, so a CLI row and a GUI row describe the same
    session with the same label.
  negative:
  - The TypeScript and C# copies stay hand-maintained and unpinned, so the file is
    canonical for one consumer out of three.
  neutral:
  - The canonical file itself is unchanged; this decision adds a reader and a test,
    not a new structure.
confirmation: A test asserts the in-module copy equals protocol/session-status-display.json
  read over a relative path, and a precedence test asserts simultaneous conditions
  collapse to the same label the graphical clients show.
summary: Take CLI state labels and precedence from protocol/session-status-display.json
  and pin the in-module copy to it, without claiming the file becomes the repository's
  single source of truth.
updated: '2026-08-14'
---

# Pin the shared status display matrix as canonical for the CLI consumer only

## Context

`protocol/session-status-display.json` holds six labels (`Working`, `Action required`, `Idle`, `Stopped`, `Failed`, `Unknown`) and the precedence `failed > human_input > active > stopped > idle`. Measurement finds **no code in the repository reads it**. The effective definitions are hand-mirrored in TypeScript (`clients/ui/src/components/StatusIcon.tsx`) and C# (`PanelPresentation.cs`), with no cross-language parity test between them.

The upstream experience contract requires that a CLI row and a graphical row show the same label for the same session, including when several conditions hold at once. A third hand copy made by eye would satisfy that requirement on the day it was written and drift afterwards.

There is a specific trap nearby: `clients/ui/src/lib/favicon.ts` carries a different precedence for cross-session aggregation in which `idle` outranks `stopped` — partly reversed. Copying the wrong one produces labels that look plausible and disagree with the GUI.

## Decision

CLI state labels and the precedence used to collapse simultaneous conditions come from `protocol/session-status-display.json`. The in-module copy is asserted equal to the repository's canonical file by a test that reads it over a relative path — the pattern `clients/web-host/internal/clientconfig/config_test.go` already uses for the shared fixtures.

The claim is deliberately bounded: this makes the file canonical **for the CLI consumer**. It does not make it the repository's single source of truth. The TypeScript and C# copies remain hand-maintained and unpinned, and converting them is a separate change.

## Consequences

{% consequence kind="positive" %}
A file that was checked in but orphaned acquires a real reader and a failing test when the copy drifts. The CLI cannot introduce a state word the matrix does not define, and it cannot pick the aggregation precedence by mistake, because the precedence test compares against the shared matrix rather than against an expectation written by hand.
{% /consequence %}

{% consequence kind="negative" %}
Two of the three consumers stay free to drift. Framing this as a promotion to repository-wide single source of truth would overstate what is gated: after this change the canonical file still has no gate that fails when the TypeScript or C# copy diverges. That residual risk is recorded rather than implied solved.
{% /consequence %}

{% consequence kind="neutral" %}
The canonical file is not modified. The `signal` (bell) axis stays orthogonal and out of precedence, and `Unknown` stays a client-side sentinel for a missing status object that a conforming server never produces.
{% /consequence %}

## Alternatives

- **Copy the labels and precedence from the TypeScript implementation by eye** — rejected: it adds a third unpinned copy to an already-diverging pair and reproduces the exact failure mode the requirement is about.
- **Add Go to the generated SDK** — rejected: the quicktype pipeline emits four languages and adding a fifth needs its own decision, and this file is a display matrix rather than a schema, so it is not a generation target.
- **Pin all three copies in this change** — rejected as scope: it touches two shipping clients for a benefit the CLI does not need, and it is a cleaner standalone change.

Confirmation: `TestStatusMatrixCopyMatchesRepositoryCanonical` (in-module copy equals the canonical file, read over a relative path) and `TestStatusPrecedenceMatchesSharedMatrix` (a session with `human_input` true and `lifecycle` active resolves to `Action required`, not `Working`).

## Related

- requirements: `FR-017`
- contracts: `contract-status-vocabulary`
- references: `adr-20260726-shared-status-display-matrix`
- open: `open-status-matrix-cross-language-drift`
- plan: `docs/changes/change-20260812-cli-terminal-client/design-plan/design.md` anchor `adr-cli-status-display-matrix-cli-pinning`


{% transition from="proposed" to="accepted" date="2026-08-14" %}
consultation-20260814-cli-terminal-client-design-final
{% /transition %}
