---
id: note-20260624-user-web-server
kind: note
title: Web client⇄server (`server` + `web`)
status: published
created: '2026-06-24'
updated: '2026-07-24'
tags:
- user
- legacy-import
owners: []
relations:
- {type: references, target: note-20260624-technical-web-gateway}
source_paths:
- Makefile
- services/cmd/server/
- clients/web-host/
- services/server/api/
- clients/ui/
topic: user
summary: the pty session daemon (typed proto over a Unix socket, -data-dir rooted)
  and the HTTP/WS gateway that translates browser REST/WebSocket traffic into in-process
  daemon calls. Every session is a pty managed in the same
---

<!-- migrated_from: docs/user/web-server.md -->

# Web client⇄server (`server` + `web`)

The runtime stack is two processes:

- **`server`** — the single-process backend. One Go binary that owns both
  the pty session daemon (typed proto over a Unix socket, `-data-dir`
  rooted) and the HTTP/WS gateway that translates browser REST/WebSocket
  traffic into in-process daemon calls. Every session is a pty managed in
  the same process (host or devcontainer launch); several browser tabs can
  attach to and share one session. Sessions outlive any single browser
  client.
- **`web-host`** — an ordinary configured client host. It loads the same
  `servers.json` fleet contract as Workspace Electron, serves the shared SPA,
  and exposes `/host/servers/{serverId}/api/...` and
  `/host/servers/{serverId}/ws` on one browser origin.

Wire-level detail and the full REST + WebSocket vocabulary live in
[server gateway internals](note-20260624-technical-web-gateway.md).

## Build & run

The fastest path for local dev launches the **entire** stack — an isolated
`server` backend plus the web host — together:

```sh
make run-dev
#   → backend : http://127.0.0.1:8443  (sock: $ROOT/.run-dev/server/server.sock)
#     web     : http://127.0.0.1:8080
#     Open  →  the URL printed by run-dev (includes an explicit server ID)
#   Ctrl-C stops both processes (the scratch dir is preserved by default;
#   set CLEAN_DATA_DIR=1 to wipe it).
```

The scratch data dir means `make run-dev` never collides with the user's
production `server` backend (`~/.agent-grid/`); the two can run side by
side. Overrides: `BACKEND_ADDR`, `WEB_ADDR`, `SERVER_DATA_DIR` (custom
isolated dir), `CLEAN_DATA_DIR=1` (wipe the scratch dir on exit).

Or run the backend + web host standalone:

```sh
make build-server build-web      # → dist/bin/server dist/bin/web-host

# The server binary boots the daemon and a co-resident HTTP/WS gateway in
# one process under -data-dir. SIGTERM tears down both. Pick any writable
# path; the example uses an XDG-style cache dir so the command works for
# non-root users out of the box.
./dist/bin/server -addr :8443 -data-dir "$HOME/.cache/agent-grid-web"
#   → "agent-grid backend on https://:8443  sock=…/server.sock"

# Web client host reads <config-dir>/servers.json once at startup:
./dist/bin/web-host -addr :8080 -config-dir "$HOME/.config/agent-grid"
```

The backend always owns its socket — there is no "attach mode" because the
gateway is in-process. The socket path is derived from `-data-dir`
(`<data-dir>/server.sock`); two backends with distinct `-data-dir` values
run independently, which is what `make run-dev` relies on to stay isolated
from a user-scope `~/.agent-grid/` install.

Open `http://<web-host>:8080/?server=<serverId>`. `serverId` comes from
`GET /host/config`; it is never inferred from the browser origin. Bearer
credentials remain inside the Web host process and are injected only into the
selected upstream request. They are not exposed to browser JavaScript or URLs.
For an SSH-only host, forward the web port
(`ssh -L 8080:localhost:8080 <host>`).

> The `web` process proxies to an **http** backend (local dev: `server -insecure`)
> or a **real-certificate https** backend; it cannot verify a self-signed https
> backend. `make run-dev` runs both over plain HTTP on localhost.

## Using it

- **Create** — type a command (e.g. `bash`) and an optional project directory,
  then **+ create**. The session starts in a pty and you attach automatically.
- **Attach / switch** — click a session in the left list. Each attach starts from
  a screen snapshot (reattach-safe), then streams live; multiple tabs — and
  multiple clients — can attach to and share the same session.
- **Detach-safe** — closing the browser/tab detaches only; the session keeps
  running on the backend and can be re-attached later.
- **Interact** — type in the terminal; it resizes with the window.
- **Server-side events** — OSC 9/777 notifications, OSC 133 prompt markers, and
  window titles are captured on the backend and shown in the events panel (so a
  notification reaches the operating client, not the server host).
- **Stop** — the ✕ next to a session terminates it.

## Flags

`server` (backend) and `web` (web-client host) share the same transport flags:

| Flag | Default | Meaning |
|---|---|---|
| `-addr` | `server`: `:8443`, `web`: `:8080` | Listen address |
| `-server` | `web` only: `http://127.0.0.1:8443` | Backend base URL to proxy `/api` and `/ws` to |
| `-token` | `server` only: generated | Bearer token (REST: `Authorization: Bearer …`; WebSocket: single-use ticket; never accepted as a query parameter) |
| `-tls-cert` / `-tls-key` | — | TLS certificate/key; self-signed if omitted |
| `-insecure` | false | Serve plain HTTP (local dev only) |
