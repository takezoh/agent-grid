# ADR 0028 — `termvt.Session` is an actor with lock-free exit state

Status: Accepted

## Context

`platform/termvt.Session` originally held one `sync.Mutex` that guarded the VT
emulator, the subscriber map, the pending control buffer, and the dimensions.
`readLoop` held the mutex while calling `em.Write(chunk)`; every other public
method (`Subscribe`, `Unsubscribe`, `Snapshot`, `Resize`, `Size`, `ExitCode`)
acquired the same lock.

That coupling crashed the entire IPC stack the first time a shell emitted a
CSI "Report Mode" sequence (`\033[?1$p`, DECRQM). Upstream
`vt.Emulator.handleRequestMode` writes the reply synchronously to an
**internal `io.Pipe`** whose read end nothing in `termvt` drained. `em.Write`
parked in the pipe send, holding the mutex, and the daemon's single dispatch
goroutine (which polls `ExitCode` via `PaneAlive` every tick) blocked on the
same lock. From the browser's side, every `GET /api/sessions` hung indefinitely
— the symptom that surfaced this bug end-to-end (gateway timeout / 401-shaped
failure in the UI).

The fix had to remove three coupled defects at once:

1. **No reader on the VT reply pipe** — DECRQM, DSR, cursor-position queries,
   and any other CSI that wants a reply blocked their write.
2. **One mutex covers unrelated state** — even after fixing #1, any slow
   operation under the mutex would still freeze every poller. The runtime's
   `ExitCode` tick poll was the obvious casualty; future slow OSC handlers
   would be the next.
3. **`Subscribe` after shutdown** — the pre-existing TOCTOU in `pty_tap.go`
   (`ExitCode` check then `Subscribe`) leaked a goroutine on a Session whose
   `readLoop` had already returned.

## Decision

`Session` becomes an actor. All previously-shared state lives behind a single
event loop:

```
readerLoop    : pty.Read → chunkCh (cap 1)
responseLoop  : io.Copy(pty, em) — drains the VT reply pipe back into pty stdin
mainLoop      : select { chunkCh | cmdCh } — sole owner of em, subs, dims, pending
```

Public methods marshal commands onto `cmdCh` and wait on a per-call reply
channel. The select-on-`s.done` shutdown branch is centralised in one generic
`call[R]` helper so every RPC honours shutdown identically; a future sixth
RPC cannot reintroduce the original deadlock class by forgetting one branch.

Exit state is the one exception: `exited atomic.Bool` + `exitCode atomic.Int32`
make `ExitCode()` lock-free. The runtime's dispatch goroutine polls it on
every tick, and Go's `sync/atomic` package gives sequentially-consistent
ordering since 1.19, so writing `exitCode` then `exited` in `handleExit`
guarantees any reader who observes `exited == true` also reads the final
`exitCode`. ExitCode never enters `mainLoop`, so an arbitrarily slow chunk
parse cannot freeze IPC.

`responseLoop` is the **structural** fix for the deadlock: with the reply pipe
drained, `em.Write` is bounded by parser work only. The actor model is
orthogonal — it would not be required to *unblock* CSI replies — but it makes
the lock-free `ExitCode` invariant impossible to violate by accident, and it
gives `Subscribe` a clean post-shutdown contract: ids ≥ 1 from the actor,
`0` reserved as the sentinel returned (with a closed channel) when `mainLoop`
has already exited. Callers that previously raced (`pty_tap.go`) get a typed
EOF instead of a leaked goroutine.

A small upstream subtlety: `vt.Emulator.Close()` writes an unsynchronised
`closed` boolean that races with the parked `em.Read`, flagged by `-race` on
every shutdown. We dodge it by calling `em.InputPipe().(io.Closer).Close()`
to wake `Read` via `io.EOF` without touching the racy field. The `Emulator`
interface exposes `CloseInputPipe()` for this — `realEmulator` delegates via
type assertion. Filed for upstream as a follow-up; the wrapper is the
contained workaround.

### Test seams introduced

The refactor adds two minimal interfaces — `Emulator` and `PTY` — and a
`NewSessionWithDeps(em, pty, cmd, cols, rows)` constructor. The pre-existing
real-pty contract suite stays untouched; new tests in `session_actor_test.go`
use fakes to exercise actor-specific properties that real-pty tests can't pin
deterministically (chunk-vs-RPC ordering, ExitCode latency under a parked
mainLoop, post-shutdown Subscribe contract, unique non-zero subscriber ids).

The `make test-race` target was added so the actor's no-mutex discipline is
verified under the race detector on every iteration. Scope is currently
`platform/termvt/... client/runtime/...`; other subtrees opt in as audited.

## Consequences

**Positive**

- The original deadlock cannot recur: the response pipe is always drained and
  `ExitCode` never blocks. End-to-end verified — `GET /api/sessions` returns
  in <1 ms where it previously hung to gateway timeout.
- `pty_tap.go`'s TOCTOU is structurally fixed: post-shutdown `Subscribe`
  returns `(0, closedCh)` rather than leaking.
- A future contributor cannot reintroduce the "mu-held-during-emulator-write"
  pattern — there is no mu to hold. Adding a new RPC routes through `call[R]`
  and inherits the shutdown contract by construction.
- Deterministic actor-shape tests now exist; previously every property had to
  be observed indirectly through a real pty + timing.

**Negative**

- Three goroutines per `Session` instead of one. A few extra hundred bytes per
  Session; immaterial.
- `cmdCh` is unbuffered, so every RPC pays a goroutine rendezvous switch.
  Acceptable: the hot path (`ExitCode`) bypasses this entirely; other RPCs are
  driven by user actions (Subscribe on attach, Resize on window change).
- The `CloseInputPipe` interface method is a workaround for an upstream race
  rather than a Session-level concept. The interface name makes the
  motivation visible; the upstream fix would simplify it back to `io.Closer`.

**Out of scope**

- `WriteInput` and `responseLoop` both write to the pty master fd. Small
  payloads are kernel-atomic up to PIPE_BUF; a paste exceeding that could
  interleave with a concurrent CSI reply. This is unchanged from the
  pre-refactor behaviour and a separate concern (it would matter equally to
  any pty multiplexer with two writers). Tracked as a follow-up if the paste
  path grows past 4 KiB in practice.
- The runtime dispatch loop itself was *not* restructured. With the leaf-level
  block gone, the existing single-dispatcher design is correct; further
  decomposition (effect timeouts, priority lanes) is a separate decision.

## References

- Cross-reference: [ADR 0003](0003-termvt-fanout-isolation.md) (the original
  fan-out isolation contract — still valid; the implementation mechanics
  evolved from "fanout holds the single-writer lock" to "fanout runs inside
  the sole-owner mainLoop", which is the same property by stronger means).
- Testing details: [termvt multiplexer testing](../technical/platform/termvt-multiplexer-testing.md).
- Lint-time enforcement is still nothing extra: file caps and function caps
  apply; the actor model is verified at runtime via the contract suite + the
  new `TestActor_*` tests.
