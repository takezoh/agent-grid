---
id: adr-20260814-cli-raw-mode-critical-section
kind: adr
title: Enter CLI raw mode only after subscription and restore through a single exit
  path
status: accepted
created: '2026-08-14'
decision_makers:
- unknown
tags:
- native-clients
- cli
- terminal
owners:
- agent-grid-maintainers
relations:
- {type: originatedFrom, target: change-20260812-cli-terminal-client}
- {type: references, target: adr-20260705-test-tier-taxonomy}
source_paths:
- clients/cli/internal/tty
- clients/cli/internal/attach
consequences:
  positive:
  - Every failure before the subscription is established is outside the restore obligation
    entirely, because the terminal was never touched.
  - A new departure path cannot forget to restore, since restore is the only exit
    from the critical section.
  negative:
  - Attach cannot echo anything to the user between invocation and establishment,
    so a slow establishment looks like silence.
  - Suspend and resume must re-enter raw mode explicitly rather than relying on the
    process's default handling.
  neutral:
  - All terminal side effects sit behind one adapter interface, which is also the
    seam that makes every exit path drivable by a fake terminal in tests.
confirmation: Tests drive every exit path through a fake terminal and assert restoration;
  a pre-subscription failure test asserts the terminal settings never change; a restore
  idempotence test asserts a double restore is safe.
summary: Delay raw entry until the subscription reaches a terminal accepted outcome,
  make restore the single exit from the critical section, and discard the terminal
  input queue before restoring on detach.
updated: '2026-08-14'
---

# Enter CLI raw mode only after subscription and restore through a single exit path

## Context

The source requirement names a corrupted terminal as the worst failure mode of a client like this, and the upstream experience contract makes restoration an invariant over an enumerated exit set that includes suspend and a recovered panic. Two shapes are commonly reached for and both are worse.

Entering raw mode at start-up and unwinding with `defer` puts every establishment failure — selector resolution, ticket minting, socket dial, protocol mismatch, initial geometry rejection, subscription refusal — inside the restore obligation. The obligation then has to be perfect across a large and growing set of paths.

Implementing restore separately in each departure path makes the obligation invisible: adding a path is adding a way to forget.

There is also a subtler observation. Stopping the stdin read on detach does not empty the kernel's tty input queue, so bytes typed after the detach key are consumed by the restored shell as its own input. The upstream contract requires that nothing typed after detaching appears in the returning shell's input line, and without an explicit queue discard the implementation is observationally identical to one that buffers and replays those keystrokes.

## Decision

Raw entry happens exactly once, and only after selector resolution, ticket acquisition, socket establishment, initial geometry acceptance and a terminal accepted subscription outcome have all completed.

Restore is implemented as the single exit from the critical section. Ordinary detach, socket close, server disconnect, a post-entry authentication or protocol failure, a catchable terminating signal, suspend, stdin EOF and the panic boundary all pass through it. Restore is idempotent.

On detach, the buffer is split at the detach byte — everything before it is forwarded normally, that byte and everything after it are discarded — the terminal input queue is discarded, and only then is cooked mode restored. No termination request is sent on any of these paths.

All terminal state changes live behind one adapter interface with a fake implementation.

## Consequences

{% consequence kind="positive" %}
The entire class of establishment failures leaves the terminal untouched, so it is outside the restore obligation rather than inside it and handled correctly. A departure path added later cannot forget to restore, because there is one exit. The adapter seam makes every one of those paths drivable deterministically by a fake terminal, so the obligation is tested rather than reasoned about.
{% /consequence %}

{% consequence kind="negative" %}
Nothing can be echoed to the user between invocation and establishment without violating the upstream decision that raw mode is fully transparent, so a slow establishment presents as silence. Suspend and resume must explicitly restore, fall through to the default behaviour, and re-enter raw mode on foregrounding, instead of relying on default process handling.
{% /consequence %}

{% consequence kind="neutral" %}
Concentrating terminal state in one adapter is also what confines the platform branch: the Unix and Windows implementations differ behind the same interface, and the Windows behaviour is deliberately left unimplemented until the platform spike records it rather than guessed from the Unix side.
{% /consequence %}

## Alternatives

- **Enter raw mode at start-up and restore with `defer`** — rejected: it enlarges the restore obligation by every establishment failure path and fails the witness that a pre-subscription failure leaves terminal settings byte-identical.
- **Implement restore individually per departure path** — rejected: correctness then depends on remembering, and the failure is silent until a user's shell is broken.
- **Filter the detach byte out of the stream instead of splitting at it** — rejected: filtering leaves the following bytes in flight and admits exactly the counterexample the upstream contract names.

Confirmation: `TestTerminalRestoredOnEveryExitPath`, `TestPreSubscribeFailureNeverTouchesTerminal`, `TestRestoreIsIdempotent`, `TestBytesAfterDetachKeySurfaceNowhere`, `TestInputQueueFlushedBeforeRestore`, and `TestFakeTTYDrivesEveryExitPath`.

## Related

- requirements: `FR-002`, `FR-004`, `FR-010`, `FR-011`, `FR-001`
- contracts: `contract-terminal-mode-lifecycle`, `contract-detach-boundary`
- references: `adr-20260705-test-tier-taxonomy` (the adapter seam is the fake half of the external-dependency triple for the local terminal)
- plan: `docs/changes/change-20260812-cli-terminal-client/design-plan/design.md` anchor `adr-cli-raw-mode-critical-section`


{% transition from="proposed" to="accepted" date="2026-08-14" %}
consultation-20260814-cli-terminal-client-design-final
{% /transition %}
