# Codex Remote Control on Ubuntu Server

Date: 2026-07-15
Status: Investigation note

## Goal

Enable a Codex task running on an Ubuntu Server to be inspected and operated from the ChatGPT mobile app, similarly to Claude Code Remote Control.

The desired workflow is:

1. Start or continue Codex work on an Ubuntu Server.
2. Leave execution and repository access on that server.
3. Inspect progress, answer questions, approve actions, and send additional instructions from the mobile app.

## Session models discussed

Two possible server-side entry points were considered.

### 1. Codex App Server

`codex app-server` exposes Codex through its application protocol and is suitable as the backend for a custom client, IDE integration, or another controller.

Conceptually:

```text
Ubuntu Server
  └─ codex app-server
       ├─ local or custom client
       └─ remote/mobile client, if an officially supported bridge is available
```

This is the natural architecture for:

- a long-running server process;
- multiple clients;
- reconnectable threads;
- integration into agent-grid;
- separating the Codex execution backend from the user interface.

However, App Server availability alone does not prove that a thread is automatically published to the ChatGPT mobile application's Remote UI. App Server protocol access and ChatGPT Remote Control pairing must be treated as separate capabilities until an official bridge is confirmed.

### 2. Codex CLI / TUI

A normal interactive session starts with:

```bash
codex
```

and an existing thread can normally be selected or resumed from the CLI's own thread history.

Conceptually:

```text
SSH terminal
  └─ codex TUI
       └─ one interactive terminal session
```

A normal TUI session should not be assumed to become remotely controllable after startup. Remote Control must either:

- be enabled when the session is created;
- attach to the same persisted Codex thread through a supported command;
- or be provided by a separate controller connected to App Server.

Attaching a mobile client directly to an arbitrary, already-running terminal process is a stronger requirement than merely resuming the same persisted thread. These must not be conflated.

## Start, resume, and process attachment

The discussion identified three different operations:

| Operation | Meaning |
|---|---|
| Start | Create a new Codex thread from a remote-capable client or backend. |
| Resume | Open an existing persisted Codex thread in another supported client. |
| Attach | Take control of the exact currently running TUI process, including its live terminal state. |

The expected support boundary is:

- **Start:** plausible through a Remote-capable Codex client.
- **Resume:** plausible when clients share the same Codex thread store/backend.
- **Attach to an arbitrary TUI process:** should be considered unsupported unless explicitly documented.

For agent-grid, thread-level continuation is more important than terminal-process mirroring. The system should model the Codex thread as the durable unit and treat each TUI, mobile UI, or web UI as a replaceable client.

## Ubuntu Server feasibility

Ubuntu Server is technically suitable for Codex execution because:

- Codex CLI and App Server can run without a desktop environment;
- repositories, credentials, tools, and MCP servers can remain on the host;
- the host can remain online while the user disconnects SSH;
- a service manager such as systemd can supervise a long-running backend.

The unresolved point is not whether Codex can execute on Ubuntu Server. It is whether OpenAI's ChatGPT mobile Remote UI can officially pair with that headless host without using the desktop application.

## Important correction to the discussion

Several concrete commands were mentioned during the conversation, including examples resembling:

```bash
codex features enable remote_control
codex remote-control start
codex app-server daemon bootstrap
codex app-server daemon start
codex app-server daemon enable-remote-control
```

These command names were not confirmed against an authoritative OpenAI source during the discussion. They must therefore be treated as **unverified examples, not implementation instructions**.

Before implementing the workflow, confirm against the installed Codex version:

```bash
codex --help
codex app-server --help
codex features list
```

Also confirm the current OpenAI documentation and release notes. CLI command names and feature flags may change between releases.

## Recommended architecture for agent-grid

Do not make mobile access depend on attaching to a particular SSH terminal or TUI process.

Prefer this model:

```text
Ubuntu Server
  ├─ Codex execution backend / App Server
  ├─ persisted Codex threads
  └─ agent-grid gateway
       ├─ desktop browser
       ├─ mobile browser
       └─ optional ChatGPT Remote integration
```

Responsibilities:

- **Codex backend:** owns task execution, approvals, thread state, and tool calls.
- **agent-grid gateway:** authenticates clients and exposes thread status and control operations.
- **UI clients:** render the same durable thread from desktop or mobile.
- **ChatGPT Remote integration:** optional adapter, only if OpenAI exposes an officially supported pairing or deep-link mechanism.

This architecture works even if ChatGPT's native mobile app cannot directly connect to a headless Ubuntu host.

## Operational fallback

If native ChatGPT Remote pairing is unavailable on Ubuntu Server, the practical fallback is:

1. Run Codex or App Server inside `tmux`/`screen` or as a supervised service.
2. Connect through agent-grid's mobile web UI or a secure SSH client.
3. Use persisted thread IDs to reopen work rather than relying on the lifetime of one terminal process.

`tmux` solves terminal reconnection, but it is only terminal transport. It does not provide the richer mobile session UI, structured approvals, diffs, or thread navigation expected from a native Remote Control feature.

## Decision summary

- Ubuntu Server can host Codex CLI and App Server without a desktop environment.
- App Server is the better foundation for agent-grid and multiple clients.
- A normal CLI/TUI session must not be assumed to become remotely controllable after it starts.
- Resuming the same Codex thread and attaching to the exact running TUI process are different capabilities.
- Native ChatGPT mobile Remote support for a headless Ubuntu host remains to be confirmed through authoritative OpenAI documentation or the installed CLI help.
- Previously proposed `remote-control` and `app-server daemon` commands remain unverified and must not be encoded into production setup scripts yet.
- agent-grid should use durable threads and a server-side gateway so mobile operation does not depend on OpenAI's native Remote UI.

## Verification checklist

- [ ] Check the installed Codex version.
- [ ] Inspect all available `codex` and `codex app-server` subcommands.
- [ ] Confirm whether a `remote-control` feature or command exists.
- [ ] Confirm whether headless pairing is supported without ChatGPT Desktop.
- [ ] Confirm whether an existing thread can be resumed from mobile.
- [ ] Confirm whether the exact active TUI process can be attached to.
- [ ] Document authentication, transport, and host-liveness requirements.
- [ ] Decide whether agent-grid integrates App Server directly or treats native Remote Control as an optional adapter.
