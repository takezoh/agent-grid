# Agent Reactor

**Run many AI agents in parallel without losing track of any of them.**

Agent Reactor is a web control surface for running Claude, Codex, Gemini, and other CLI agents across all your projects at once. It replaces the manual work of opening tabs, remembering which agent is doing what, and checking back for completion — and it can also run agents unattended against an issue tracker.

### What it does

- **Launch an agent without typing commands.** Select a project from the list and Reactor handles the directory, environment, and command for you.
- **See every agent's status at a glance.** Each session shows whether the agent is running, waiting for your input, awaiting tool approval, or idle.
- **Jump into any session instantly.** Live-preview a session, then press Enter to take over. Supervise dozens of concurrent tasks without losing focus.
- **Keep agents running after you disconnect.** Sessions live in an in-process pty multiplexer owned by the `server` backend, so closing the browser tab or dropping the connection doesn't stop the work.
- **Run each agent in its own sandbox.** Optional per-project devcontainer with brokered AWS / gcloud / SSH credentials and a policy-gated host-exec channel. Long-lived secrets stay on the host.
- **Automate end to end.** The orchestrator reads a `WORKFLOW.md`, polls a tracker, and drives agents through issues with no human in the loop.

## Three binaries, three layers

This module builds three binaries from a single Go module, mapping onto a three-layer architecture (`platform/` · `client/` · `orchestrator/`):

- **`server`** — the single-process backend: the pty session daemon that owns every agent session (pty-multiplexer–backed; the `client` layer) plus the HTTP/WS gateway that translates browser REST/WS traffic into in-process daemon calls
- **`orchestrator`** — the scheduling brain that reads `WORKFLOW.md`, dispatches work to agents, and reconciles state — the `orchestrator` layer
- **`claude-app-server`** — a stdio JSON-RPC shim that wraps a Claude agent as a drop-in Codex app-server

The layers and their enforced import boundaries are defined in [ARCHITECTURE.md](ARCHITECTURE.md).

## Requirements

- Go 1.26+

## Install

```bash
make install
```

Installs to `~/.local/bin/server`. Then:

```bash
server                              # start the backend (daemon + HTTP/WS gateway).
                                    # Hooks in ~/.claude/settings.json and
                                    # ~/.gemini/settings.json are registered
                                    # automatically against this binary's
                                    # path on every boot — no manual setup.
```

To reach sessions from a browser, run `make run-dev` (boots `server` + `web` together) or follow [web stack (ad-hoc launch)](docs/note/note-20260624-user-web-server.md).

See [Getting started](docs/note/note-20260624-user-getting-started.md) for the full walkthrough.

## Documentation

Documentation is stored as structured docs-skill records under [`docs/`](docs/note/note-20260624-docs-overview.md), with audience × architecture layer pages kept as the primary navigation.

| | Start here |
|---|---|
| **Using the tools** | [User guide](docs/note/note-20260624-user-overview.md) — [getting started](docs/note/note-20260624-user-getting-started.md), [web stack](docs/note/note-20260624-user-web-server.md), [orchestrator](docs/note/note-20260624-user-orchestrator.md), [sandbox](docs/note/note-20260624-user-sandbox.md), [systemd service](docs/note/note-20260624-user-systemd.md) |
| **Working in the repo** | [Agent guide](docs/note/note-20260624-agent-overview.md) — [contributing](docs/note/note-20260624-agent-contributing.md), [WORKFLOW.md authoring](docs/note/note-20260624-agent-workflow-authoring.md), [testing](docs/note/note-20260624-agent-testing.md) |
| **Internals** | [Technical docs](docs/note/note-20260624-technical-overview.md) — [platform/](docs/component/component-20260624-platform-overview.md), [client/](docs/component/component-20260624-client-overview.md), [orchestrator/](docs/component/component-20260624-orchestrator-overview.md) · [ARCHITECTURE.md](ARCHITECTURE.md) |
