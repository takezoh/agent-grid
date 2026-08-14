---
id: design-clients-cli
kind: design
title: Terminal CLI client (agent-grid)
status: active
created: '2026-08-14'
scope_type: component
responsibilities:
- id: RESP-001
  statement: Be the terminal-first client of the public wire contract — discovery,
    attach, detach, create, terminate — over the same REST/WS surface every other
    client host consumes, with no endpoint of its own.
- id: RESP-002
  statement: Own the module's Go wire codec. Frame decoding and the three outbound
    encoders (lifecycle desired, input, resize) are the only places a frame's kind
    discriminator is read or written.
- id: RESP-003
  statement: Own every local-terminal side effect behind one Terminal seam — raw entry,
    restore, geometry query, size-change events, input-queue discard — so the attach
    runtime can be driven to all of its exit paths without a terminal present.
- id: RESP-004
  statement: Own the process's failure classification and its exit-code assignment
    in exactly one place, so no command decides a number.
- id: RESP-005
  statement: Own fleet observation and selector resolution over the shared servers.json
    contract, consumed through clients/goclient/clientconfig as one of its three loaders'
    callers rather than by parsing the file again.
invariants:
- id: INV-001
  statement: clients/cli has no require, replace, or transitive dependency edge to
    the services module; it is a client of the published contract, never of an internal
    type.
  enforcement: test
- id: INV-002
  statement: The failure-class-to-exit-code map has exactly one owner; an unknown class
    exits 1 rather than 0.
  enforcement: test
- id: INV-003
  statement: Raw mode is entered only after the daemon reports an established lifecycle
    outcome — accepted and waiting are non-terminal progress — and the terminal is
    restored exactly once on every exit path, including the ones a signal takes.
  enforcement: test
- id: INV-004
  statement: Server-qualified session identity is constructed in one place with unexported
    fields, so a compound identity can be neither assembled elsewhere nor put on the
    wire.
  enforcement: test
- id: INV-005
  statement: A protocol-version mismatch is terminal for that server and refuses later
    calls locally without touching the network; it never disables a peer server.
  enforcement: test
- id: INV-006
  statement: A configuration-invalid entry is never given a transport, which is what
    guarantees it can never acquire a reachability observation; it is rendered in a
    vocabulary disjoint from both reachability and session lifecycle state.
  enforcement: test
- id: INV-007
  statement: Session termination has exactly one emission site, and a module-wide scan
    fails when a second appears; detach emits none.
  enforcement: test
- id: INV-008
  statement: Attach subscribes over the lifecycle-multiplexed socket (GET /ws plus a
    desired publication), never the per-session route, whose shared empty subscriber
    identity denies a second client its snapshot.
  enforcement: test
- id: INV-009
  statement: The CLI reads configuration and never writes it; no failure path creates,
    repairs, or migrates a user configuration file.
  enforcement: test
- id: INV-010
  statement: Credentials reach the Authorization header and the per-dial ticket mint
    and nothing else — never a URL, never a diagnostic, never persisted state.
  enforcement: test
- id: INV-011
  statement: The REST calls and the lifecycle socket share one transport policy — trust
    store, proxy and timeouts — so a server reachable over REST is reachable over its
    socket. The two halves may not be configured independently.
  enforcement: test
boundaries:
  provides:
  - The terminal session surface — list, attach, create, terminate — and its machine-readable
    discovery output.
  consumes:
  - The public REST/WS gateway contract, including the published transport-close vocabulary.
  - clients/goclient/clientconfig, the one Go loader of the clients/config servers.json
    contract.
  - protocol/session-status-display.json as the canonical status matrix, and protocol/fixtures/session-info-golden.json
    as the pinned session payload shape.
  forbidden:
  - Any dependency on the services module, including its internal packages and its
    wire types.
  - Owning a PTY, or any ownership, takeover, or exclusivity concept over a session.
  - A replay, transcript, or scrollback store; the CLI presents what the session publishes.
  - An endpoint that exists for the CLI alone.
  - Writing user configuration.
variability:
  fixed:
  - The independent module with no services edge, the single Terminal seam, the single
    exit-code owner, and the lifecycle-multiplexed attach topology.
  free:
  - Table rendering, flag spellings, and the default detach byte, which is configurable
    precisely because its collision behaviour is unmeasured.
capabilities:
- id: cap:terminal-session-client
  uniqueness: global
failure_responsibilities:
- Every failure resolves into one of six classes with a fixed exit code; a departure
  that carries no published close reason is reported as an unattributed loss rather
  than guessed into a named cause.
- A partially unreachable fleet still reports its reachable members; one server's
  outage, mismatch, or invalid configuration never suppresses its peers.
trust_boundaries:
- Credentials come from the client-config loader and are redacted out of every diagnostic,
  along with any URL, before it is printed.
- All gateway input is validated at the codec boundary; an unknown frame kind is ignored
  deliberately, a known one must be handled.
compatibility_policies:
- The protocol version is asserted before any operational call, and a mismatch is terminal
  for that server rather than retried.
- The session payload shape the CLI decodes is pinned by a golden vector the server's
  own serializer is tested against, so a rename fails on the side that renamed it.
tags:
- native-clients
- cli
- go-module
owners: []
relations:
- {type: references, target: design-client}
- {type: references, target: design-clients-ui}
- {type: references, target: design-workspace-desktop}
source_paths:
- clients/cli/
summary: Governs the terminal CLI client — an independent Go module that consumes the
  public wire contract for discovery, attach, create and terminate, owns every local-terminal
  side effect behind one seam, and holds a single owner for exit classification and
  for server-qualified identity.
updated: '2026-08-14'
---

## Purpose

`clients/cli` is the terminal surface of Agent Grid. It attaches a local
terminal to a session the backend already owns, and it does so as a client of
the same published REST/WS contract the browser, the Electron workspace and the
Windows Shell consume. It is the fifth Go module in the repository and the
third caller of `clients/goclient/clientconfig`.

The boundary is the point of the design. The backend owns the PTY, the session
lifetime and the output fan-out; the CLI owns a terminal, a socket, and the
decision of what to do when either goes away. Every rule below exists to keep a
convenience from crossing that line.

## What the module is not

Four absences are load-bearing rather than incidental:

- **No PTY.** The CLI never allocates one. The only terminal it touches is the
  operator's, through `internal/tty`.
- **No ownership.** There is no takeover, no exclusivity, no "steal the
  session". Two clients attached to one session is a supported state, which is
  why the attach topology had to be the multiplexed socket.
- **No store.** No replay buffer, no transcript, no scrollback file. What the
  session publishes is what the operator sees.
- **No private endpoint.** A CLI that needed a route no other client had would
  be a CLI that had stopped being a client.

## Conformance

- Module boundary: three mechanisms at three moments — a depguard rule as the
  import is written, `TestCLIModuleRegisteredInBuildAndVerificationEntryPoints`
  in `services/gorules` for the `go.mod` edge and for entry-point coverage, and
  the `cli-module-boundary` profile command walking `go list -deps`.
- Single-owner invariants (exit codes, identity construction, outbound
  serialization, the terminate emission site) are held by module-wide scans that
  fail on a synthesized second site, not by review.
- Wire shape: `protocol/fixtures/session-info-golden.json` is reproduced byte
  for byte by the server's serializer and decoded by the CLI, with a symmetric
  consumer registry — an unregistered consumer that reads it fails, and a
  registered consumer that stops reading it fails.
- Both external boundaries — the gateway and the local terminal — carry the full
  dependency triple (seam, fake, invariant contract test, `FakeVsReal*`
  backstop) in `test-harness/dependencies.json` and `test-harness/e2e-suites.json`.
  Both backstops have been observed passing, which is a separate fact from being
  registered: the gateway one found INV-011's violation on its first real run,
  and no test built on the fake could have, because the fake supplies both
  transport halves itself.

## Known gaps

`unit-platform-hardening-spikes` is open. On Windows, `SizeChanges` yields
nothing and `DiscardInputQueue` returns `ErrUnsupportedPlatform`; the BSD
input-queue flush is in the same state. These are unimplemented rather than
guessed, because a wrong guess about a console leaves the operator with a shell
that does not echo. The detach byte's default (`0x1D`) has not been checked
against Claude Code's or Codex's own key chords, which is why `--detach-key`
exists.

## Related Decisions

- `adr-20260814-cli-independent-go-module` — the module boundary and its gates.
- `adr-20260814-cli-attach-via-lifecycle-multiplexed-ws` — why the per-session
  route is disqualified.
- `adr-20260814-cli-raw-mode-critical-section` — when raw mode may be entered
  and how it is guaranteed to come back.
- `adr-20260814-cli-failure-class-exit-codes` — the fixed six-class assignment.
- `adr-20260814-cli-selector-kinds-id-and-prefix` — why there is no name
  selector.
- `adr-20260814-cli-status-display-matrix-pinning` — the CLI-only canonical
  status matrix.
- `adr-20260814-sessioninfo-golden-vector-conformance` — the pinned payload
  shape and its consumer registry.
- `adr-20260814-cli-canonical-binary-name` — the installed name, and the alias
  the installer declines to claim.
