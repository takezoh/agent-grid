# Run as a systemd service (production)

This guide brings the three-process production stack (`runtime`, `server`,
`web`) up as per-user systemd units. The dev launcher (`scripts/run-dev.sh`)
remains the right tool for ad-hoc work — the systemd path is for hosts that
should restart on crash, autostart on boot, and persist logs through
`journald`.

> The TUI's data directory (`~/.agent-reactor/`) and the service's
> (`~/.local/state/agent-reactor/`) are independent — both can run in
> parallel without interfering.

## Architecture (multi-client model)

```
runtime  ── owns sessions, sockets, on-disk state (-data-dir)
   ▲
   ├── server    ── HTTP/WS gateway. One of many runtime clients.
   │      ▲
   │      └── web    ── browser UI + reverse proxy to server
   ├── (TUI client)
   └── (future native clients)
```

Restarting `server` does not lose sessions — sessions live in `runtime`.
Restarting `runtime` drops every attached client and recreates sessions from
disk on the next boot.

## Prerequisites

- Linux with systemd (Ubuntu 24.04 verified).
- Docker reachable from the invoking user — either via membership in the
  `docker` group, or via a rootless docker socket at
  `$XDG_RUNTIME_DIR/docker.sock` (auto-detected).
- A user-writable `~/.local/bin/` and `~/.config/systemd/user/`.

## Install

```sh
# 1) build the three production binaries + libexec helpers
make build build-server build-web

# 2) install binaries (renamed to service vocabulary) and unit files
make install-systemd
#   → ~/.local/bin/agent-reactor-runtime
#   → ~/.local/bin/agent-reactor-server
#   → ~/.local/bin/agent-reactor-web
#   → ~/.local/lib/agent-reactor/{reactor-bridge,notify.ps1}
#   → ~/.config/systemd/user/agent-reactor-{runtime,server,web}.service

# 3) make services survive logout (boot-time autostart)
loginctl enable-linger $USER

# 4) start the stack (cascades down to runtime + server)
systemctl --user daemon-reload
systemctl --user enable --now agent-reactor-web.service
```

The cascade is by `Requires=` / `BindsTo=`: enabling `web` pulls in `server`,
which pulls in `runtime`. There is no need to enable the lower units
separately.

## Connect from a browser

```sh
# from your workstation
ssh -L 8080:127.0.0.1:8080 prod-host

# on prod-host (once)
cat ~/.local/state/agent-reactor/server.token

# in the browser
http://127.0.0.1:8080/#token=<paste>
```

The bearer token is generated on first boot and persisted to
`~/.local/state/agent-reactor/server.token` (mode 0600). Restarting `server`
re-reads the same file, so bookmarked URLs survive a unit reload. Rotate by
deleting the file and restarting `agent-reactor-server`.

## Logs

```sh
journalctl --user -u agent-reactor-runtime -f
journalctl --user -u agent-reactor-server  -f
journalctl --user -u agent-reactor-web     -f
```

## Verify the cascade

```sh
# stopping runtime cascades down: server and web both stop within a few seconds.
systemctl --user stop agent-reactor-runtime
systemctl --user status agent-reactor-server  # → inactive
systemctl --user status agent-reactor-web     # → inactive

# restart server alone — runtime stays active, sessions survive.
systemctl --user start agent-reactor-runtime
systemctl --user start agent-reactor-web      # cascades server too
systemctl --user restart agent-reactor-server
# (existing browser tab keeps its session list; server reconnect happens transparently)
```

## TLS / LAN exposure (out of the box: loopback only)

The shipped units bind `127.0.0.1` and pass `-insecure` — appropriate for
single-user hosts where access is always via SSH tunnel. To expose externally:

1. Drop a TLS certificate pair somewhere readable by your user (e.g.
   `~/.config/agent-reactor/tls/`).
2. Write `~/.local/state/agent-reactor/server.env`:
   ```
   # overrides ExecStart= via EnvironmentFile — server reads these in its env
   ```
   (the gateway flags `-tls-cert` / `-tls-key` / `-addr` are not env-aware
   today; the cleanest path is `systemctl --user edit agent-reactor-server`
   and override `ExecStart=` directly. The `server.env` file is reserved for
   future env-aware knobs.)
3. Mirror the change for `agent-reactor-web` if the browser will hit it
   directly rather than via tunnel.

A reverse proxy (nginx / caddy) in front of `agent-reactor-web` with
Let's Encrypt is the most painless production fronting.

## Uninstall

```sh
systemctl --user disable --now agent-reactor-web.service
systemctl --user stop agent-reactor-server.service agent-reactor-runtime.service
rm ~/.config/systemd/user/agent-reactor-{runtime,server,web}.service
rm ~/.local/bin/agent-reactor-{runtime,server,web}
rm -rf ~/.local/lib/agent-reactor
# Optionally also drop persistent state (sessions, token, logs):
rm -rf ~/.local/state/agent-reactor
loginctl disable-linger $USER  # if you no longer want any user service to autostart
```
