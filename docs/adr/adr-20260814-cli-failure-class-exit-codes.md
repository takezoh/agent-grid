---
id: adr-20260814-cli-failure-class-exit-codes
kind: adr
title: Fix CLI exit codes zero through six for the seven terminal outcomes
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
- {type: references, target: adr-20260727-exact-match-client-server-protocol}
- {type: references, target: adr-20260814-transport-close-reason-taxonomy}
source_paths:
- clients/cli/internal/app
consequences:
  positive:
  - Scripts can branch on a failure class without parsing text, and the same input
    reproduces the same code across implementations.
  negative:
  - Seven values become a public contract of the client and cannot be renumbered without
    a breaking change.
  neutral:
  - The capacity-refusal outcome maps to the transport code and is distinguished by
    the reason the execution plane publishes rather than by a number of its own.
  - A qualifier whose every match is a configuration-invalid entry is a usage error,
    because the failure is decided locally from operator input before any I/O.
confirmation: A table test asserts every failure class maps to its specified code
  and that no two classes share one, and a second test asserts identical codes and
  output for interactive and piped invocations.
summary: Assign success 0, internal fault 1, usage 2, not found 3, ambiguous selector
  4, authentication or compatibility 5, transport 6, and reject a number space above
  100.
updated: '2026-08-14'
---

# Fix CLI exit codes zero through six for the seven terminal outcomes

## Context

The upstream experience contract settled that each failure class carries its own exit code (`DP-EXIT-CODE-GRANULARITY`) and handed the numbering to design as a confirmed `design_handoff` — not as a product decision awaiting further consultation. Its recorded rationale is explicit that only the numbering remained open.

Leaving distinctness as the only contracted property would make the codes a public interface that varies per implementer, which defeats the reproducibility the requirement asks for: automation that branches on a code needs the code to be the same tomorrow and in the next build.

The source requirement's error-model section already proposes a concrete list, so adopting it introduces no new product choice.

## Decision

- `0` success, including a normal detach
- `1` generic or internal fault
- `2` usage error, including an invalid selector syntax and a missing mandatory server qualifier
- `3` target session not found
- `4` ambiguous selector
- `5` authentication or compatibility failure
- `6` transport failure, including a server that was unreachable before the attach was established

A selector that matches only on an unreachable server is `6`, not `3` — the distinction is epistemic: "absent from every server I could read" is not-found, "could not read the server that might hold it" is transport.

A server qualifier whose every match is a configuration-invalid entry is `2`, not `3` and not `6`: the failure is decided locally from operator-authored input before any I/O, which is what usage error means. Calling it transport would send the operator to the network for a problem in their own file; calling it not-found would deny that the entry exists.

A capacity refusal of attach establishment is `6`, and it is deliberately **not** given a number of its own. What separates it from any other transport failure is the reason the execution plane publishes, not a sentence the client writes — see `adr-20260814-transport-close-reason-taxonomy`.

## Consequences

{% consequence kind="positive" %}
Scripts branch on a class without parsing text, and the conventional meanings of `0`, `1` and `2` are preserved, so the CLI behaves like a well-formed Unix tool. The mapping lives in exactly one place in the entry point, so it can be asserted value by value.
{% /consequence %}

{% consequence kind="negative" %}
Seven values become a public contract of the client and cannot be renumbered without a breaking change. A future failure class takes a new number rather than reusing one, so the space only grows.
{% /consequence %}

{% consequence kind="neutral" %}
The capacity-refusal class maps onto `6` and is distinguished by the reason the execution plane publishes rather than by a number of its own. The granularity the upstream decision point settled is therefore untouched, and a script that needs to branch on capacity reads a published value instead of a code.
{% /consequence %}

## The capacity clause, after consultation

The numbering above is accepted and is not reopened. One clause changed.

The earlier form of this decision said a capacity refusal maps to `6` "with a reason that names capacity", meaning a reason the CLI writes, and escalated the question of whether capacity deserves a dedicated code. That question was live only because a client-authored sentence is not something automation can branch on reliably — which is the same per-implementer variance this ADR exists to remove from the codes themselves.

`adr-20260814-transport-close-reason-taxonomy` puts the discriminator in the public contract instead: a closed, published vocabulary carried on every server-initiated close and in every pre-upgrade rejection body. With that, the dedicated-code question is answered **no** — a second public number would say what the published reason already says. What remains open is a different question that was bundled with it: whether the eight-subscription budget should be raised or made per client rather than per gateway-to-daemon IPC connection. That is an execution-plane capacity decision, not an exit-code one.

Each published reason maps to a class already in the table: `unauthenticated` and `incompatible_protocol` to `5`, `capacity` and `going_away` and `backend_unavailable` and `internal_error` to `6`, `invalid_request` to `1` (a frame the client sent outside contract is the client's own fault, not the user's). A departure carrying no reason at all is `6`, presented as an unattributed loss rather than as one of the named causes.

## Alternatives

- **Coarser buckets** — rejected by the upstream decision, which fixed per-class codes.
- **A number space above 100 to avoid colliding with a proxied remote exit code** — considered and rejected: the client never proxies a remote process's exit code, so there is nothing to collide with; the range above 125 is shell-reserved and a hazard rather than a safeguard; and small codes are what scripts expect.
- **Give the capacity refusal its own code** — rejected, and now for a stronger reason than deferral: with a published close reason the code would carry no information the reason does not already carry, while permanently consuming a number from a space that only grows.

Confirmation: `TestFailureClassesMapToTheFixedExitCodes` (each class maps to its specified value; no two classes share a code) and `TestInteractiveAndPipedInvocationsAgree` (the same input produces the same output content and exit code with and without a terminal).

## Related

- requirements: `FR-024`, `FR-025`, `FR-029`, `FR-032`
- contracts: `contract-exit-classification`
- references: `adr-20260727-exact-match-client-server-protocol` (defines the compatibility failures that share code `5` with authentication), `adr-20260814-transport-close-reason-taxonomy` (supplies the discriminator inside code `6`)
- open: `open-resource-exhausted-failure-class` (budget ownership only; the dedicated-code half is closed)
- plan: `docs/changes/change-20260812-cli-terminal-client/design-plan/design.md` anchor `adr-cli-failure-class-exit-codes`


{% transition from="proposed" to="accepted" date="2026-08-14" %}
consultation-20260814-cli-terminal-client-design-final
{% /transition %}
