---
id: note-20260624-user-systemd
kind: note
title: Run as a systemd service (production)
status: published
created: '2026-06-24'
updated: '2026-07-04'
tags:
- user
- legacy-import
owners: []
relations: []
source_paths:
- scripts/run-dev.sh
- deploy/systemd/
- Makefile
topic: user
summary: This guide brings the two-process production stack (server, web) up as per-user
  systemd units. The dev launcher (scripts/run-dev.sh) remains the right tool for
  ad-hoc work — the systemd path is for hosts that should
---

<!-- migrated_from: docs/user/systemd.md -->

# Run as a systemd service (production)

This guide brings the two-process production stack (`server`, `web`) up as
per-user systemd units. The dev launcher (`scripts/run-dev.sh`) remains the
right tool for ad-hoc work — the systemd path is for hosts that should
restart on crash, autostart on boot, and persist logs through `journald`.

> A user-launched `server` backend's data directory (`~/.agent-grid/`)
> and the service's (`~/.local/state/agent-grid/`) are independent — both
> can run in parallel without interfering.

## Architecture (gateway + web host)

```
server   ── owns sessions, sockets, on-disk state (-data-dir),
            and serves the HTTP/WS gateway in the same process
   ▲
   └── web    ── browser UI + reverse proxy to server
```

Restarting `web` does not lose sessions — sessions live in `server`.
Restarting `server` drops every attached browser tab and recreates sessions
from disk on the next boot.

## Prerequisites

- Linux with systemd (Ubuntu 24.04 verified).
- Docker reachable from the invoking user — either via membership in the
  `docker` group, or via a rootless docker socket at
  `$XDG_RUNTIME_DIR/docker.sock` (auto-detected).
- A user-writable `~/.local/bin/` and `~/.config/systemd/user/`.

## Install

```sh
# 1) build the production binaries + libexec helpers
make build-server build-web

# 2) install binaries (renamed to service vocabulary) and unit files
make install-systemd
#   → ~/.local/bin/agent-grid-server
#   → ~/.local/bin/agent-grid-web
#   → ~/.local/lib/agent-grid/{bridge,notify.ps1}
#   → ~/.config/systemd/user/agent-grid-{server,web}.service

# 3) make services survive logout (boot-time autostart)
loginctl enable-linger $USER

# 4) start the stack (cascades down to server)
systemctl --user daemon-reload
systemctl --user enable --now agent-grid-web.service
```

The cascade is by `Requires=` / `BindsTo=`: enabling `web` pulls in
`server` (the daemon + gateway). There is no need to enable the lower unit
separately.

`agent-grid-server.service` now uses `Type=notify`, so `web` waits until
the server has bound its HTTP listener before it starts proxying requests.
On interactive launches, where `NOTIFY_SOCKET` is unset, the readiness call
is a silent no-op and startup behaves as before.

## Connect from a browser

```sh
# from your workstation
ssh -L 8080:127.0.0.1:8080 prod-host

# on prod-host (once)
cat ~/.local/state/agent-grid/server.token

# in the browser
http://127.0.0.1:8080/#token=<paste>
```

The bearer token is generated on first boot and persisted to
`~/.local/state/agent-grid/server.token` (mode 0600). Restarting `server`
re-reads the same file, so bookmarked URLs survive a unit reload. Rotate by
deleting the file and restarting `agent-grid-server`.

## Logs

```sh
journalctl --user -u agent-grid-server -f
journalctl --user -u agent-grid-web    -f
```

The backend's slog output is appended to
`~/.local/state/agent-grid/server.log` (rotated per startup), and its
session socket is at `~/.local/state/agent-grid/server.sock`.

## Verify the cascade

```sh
# stopping server cascades down: web stops within a few seconds.
systemctl --user stop agent-grid-server
systemctl --user status agent-grid-web     # → inactive

# restart web alone — server stays active, sessions survive.
systemctl --user start agent-grid-server
systemctl --user restart agent-grid-web
# (existing browser tab keeps its session list; web reconnect is transparent)
```

## LAN / external exposure

The shipped units bind `127.0.0.1` and pass `-insecure` — appropriate for
single-user hosts where access is always via SSH tunnel. The browser only
talks to `agent-grid-web`; `agent-grid-server` is an internal backend
the web unit reverse-proxies to. **For LAN access, override `-web` only and
keep `-server` on loopback.**

### Bind `-web` to 0.0.0.0 (plain HTTP — loopback-grade trust required)

Create a drop-in (`systemctl --user edit agent-grid-web` opens an editor;
or write the file directly):

```sh
mkdir -p ~/.config/systemd/user/agent-grid-web.service.d
cat > ~/.config/systemd/user/agent-grid-web.service.d/override.conf <<'EOF'
[Service]
ExecStart=
ExecStart=%h/.local/bin/agent-grid-web -addr 0.0.0.0:8080 -insecure -server http://127.0.0.1:8443
EOF
systemctl --user daemon-reload
systemctl --user restart agent-grid-web
ss -tlnp | grep 8080   # expect *:8080
```

The empty `ExecStart=` line on its own is required — systemd refuses to
append a second `ExecStart=` to a `Type=simple` unit, so without the reset
the drop-in is silently ignored and the unit keeps the shipped
`127.0.0.1:8080` binding. `systemctl --user show agent-grid-web -p
ExecStart -p DropInPaths` is the fastest way to confirm an override is in
effect.

If you override `agent-grid-server.service`, keep its `Type=notify`
setting unless you intentionally want to reintroduce the startup race. The
server still starts normally when launched outside systemd; readiness
notification simply becomes a no-op there.

Plain HTTP on `0.0.0.0` means the bearer token (`server.token`) crosses the
LAN in cleartext. Acceptable only on a trusted segment; otherwise add TLS
(below) or front with a reverse proxy.

### Bind `-server` itself to 0.0.0.0 without auth (dev networks only)

Normally `-server` stays on loopback and only `-web` fronts it. If you
truly need to reach the server directly from another host on an isolated
dev network — e.g. debugging a native client that talks to the gateway
without the web proxy in between — you can combine `-no-auth` with a
non-loopback bind, but this requires an explicit opt-in:

```ini
[Service]
ExecStart=
ExecStart=%h/.local/bin/agent-grid-server \
  -addr 0.0.0.0:8443 -insecure -no-auth -allow-non-loopback-no-auth \
  -data-dir %S/agent-grid -token-file %S/agent-grid/server.token
```

`-no-auth` alone still refuses non-loopback binds — the daemon exits with
`-no-auth refuses non-loopback bind …`. `-allow-non-loopback-no-auth` is
the opt-in that suppresses that guard. When both are set, startup emits a
distinct WARN in `~/.local/state/agent-grid/server.log`:

```
WARN msg="gateway: -no-auth on NON-LOOPBACK bind — auth is disabled and
     the REST/WS surface is reachable from the network. Anyone who can
     reach this port can drive every session." addr=0.0.0.0:8443
```

This exposes every session, every workspace file, and every attached
agent process to any host that can route to this port. Use only on
isolated dev networks. To revert, restore `-addr 127.0.0.1:8443` and
drop `-allow-non-loopback-no-auth` (leaving `-no-auth` intact is fine on
loopback).

### TLS direct on `-web`

1. Drop a certificate pair somewhere readable by your user (e.g.
   `~/.config/agent-grid/tls/{fullchain.pem,privkey.pem}`).
2. Extend the drop-in:
   ```ini
   [Service]
   ExecStart=
   ExecStart=%h/.local/bin/agent-grid-web \
     -addr 0.0.0.0:8443 \
     -tls-cert %h/.config/agent-grid/tls/fullchain.pem \
     -tls-key  %h/.config/agent-grid/tls/privkey.pem \
     -server http://127.0.0.1:8443
   ```
   (drop `-insecure`; `-tls-cert` / `-tls-key` / `-addr` are CLI flags, not
   env-aware. A future `web.env` hook is reserved by `EnvironmentFile=-` but
   has no env-aware knobs today.)
3. `daemon-reload` + `restart agent-grid-web`.

### Reverse proxy in front (recommended for real production)

Keep `-web` on `127.0.0.1:8080 -insecure` and front it with nginx / caddy +
Let's Encrypt on the host. The proxy terminates TLS and forwards to
loopback; no drop-in needed.

### Firewall

Binding to `0.0.0.0` is not enough on hosts with a firewall. Check + open:

```sh
sudo ufw status                       # if active:
sudo ufw allow 8080/tcp               # (or 8443 for TLS)
sudo iptables -nL INPUT | grep -E '8080|policy'
```

Cloud VMs additionally need the provider's security-group / VPC firewall
open on the same port.

## Uninstall

```sh
systemctl --user disable --now agent-grid-web.service
systemctl --user stop agent-grid-server.service
rm ~/.config/systemd/user/agent-grid-{server,web}.service
rm ~/.local/bin/agent-grid-{server,web}
rm -rf ~/.local/lib/agent-grid
# Optionally also drop persistent state (sessions, token, logs):
rm -rf ~/.local/state/agent-grid
loginctl disable-linger $USER  # if you no longer want any user service to autostart
```
