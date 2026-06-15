# webterm — PoC: tmux-free pty multiplexer + Web client

Proof-of-concept for the remote client-server architecture in
[`docs/technical/remote-client/design.md`](../../docs/technical/remote-client/design.md).
It validates the **Phase-2 core**: run a command server-side in a pty, parse its
output with a server-side VT emulator, handle OSC sequences server-side, and
render the stream in a browser (xterm.js) over a WebSocket.

This is an **isolated nested Go module** (own `go.mod`) so it does not touch the
production module's dependency graph or lint gates. Promotion to production means
moving it into `src/` behind the `TmuxBackend → PtyBackend` seam and satisfying
the repo's enforcement (funlen, depguard, library justification).

## What it demonstrates

- **pty supervision** — `github.com/creack/pty` spawns the command in a pty.
- **server-side screen state** — `github.com/charmbracelet/x/vt` parses output
  for a reattach snapshot (`Render()`) and OSC handling.
- **server-side OSC "tee"** — OSC 9 (notification), OSC 133 (prompt markers), and
  window title (OSC 0/2) are captured server-side and delivered as **structured
  control events**, separate from the raw output stream.
- **wire format** — output frames are asciicast v2 `[t,"o",data]`; control events
  are JSON objects; both share one WebSocket text channel.
- **multiplexing + reattach** — one shared session, multiple browser tabs; each
  new client gets a screen snapshot first, then the live stream.
- **input + resize** — `term.onData` → pty; `term.onResize` → `pty.Setsize` +
  emulator resize.

## Run

```sh
cd playground/webterm
go run . -addr 127.0.0.1:8090 -cmd "$SHELL"
# open http://127.0.0.1:8090 in a browser (xterm.js loads from jsdelivr CDN)
```

Flags: `-addr` (listen address), `-cmd` (command to run in the pty; space-split,
no quoting in the PoC).

Try the OSC tee from inside the shell — these appear in the right-hand
"server-side events" panel, not as raw text:

```sh
printf '\033]9;hello from the agent\a'   # OSC 9 notification  → control event
printf '\033]0;my window title\a'        # OSC 0 title         → control event
```

## Test

```sh
go test -race ./...
```

Headless coverage: input echo, OSC 9 / OSC 133 / title capture, reattach
snapshot ordering, resize, and the full `http → websocket → pty → emulator →
frame` path (`TestWSEndToEnd`).

## Known PoC shortcuts (production must address)

- Frame-dropping on slow subscribers (production: bounded buffer + disconnect).
- No auth / TLS (production: bearer token + TLS per the design doc).
- Raw passthrough only; OSC the client must not see (e.g. OSC 52) is not yet
  stripped from the stream.
- `-cmd` is space-split with no shell quoting.
