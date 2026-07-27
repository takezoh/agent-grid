---
change: change-20260727-multi-server-session-palette
role: implementation
---

<!-- lifecycle is owned by change.md -->

# Implementation

## Contracts

`design-plan.json` is the canonical projection for `contract-profile-data-model`, descriptor REST, server-side materialization, create-wire replacement, Electron auth routing, partial outcomes, browser adaptation, parameter form, confirm/create control flow, and server-qualified identity. The later user decision removes every legacy normalization/fallback path.

Key seams and authorities:

- `host/config` owns server/project profile resolution, revision generation, typed defaults, and argv materialization.
- `server/api` accepts only version-2 profile creates and reloads the current config before revision/value validation.
- Electron main owns credentialed REST fan-out and target routing; preload exposes typed projections only. Browser remains a one-server adapter.
- Palette state renders all profile parameters in declaration order and preserves compound server/project identity.

## Configuration

```toml
[[session.command_profiles]]
id = "sol"
label = "Codex Sol"
executable = "codex"
argv = ["--model", "{{model}}", "-c", "model_reasoning_effort=\"{{effort}}\""]
[[session.command_profiles.parameters]]
name = "model"
label = "Model"
type = "enum"
default = "gpt-5.6-sol"
values = ["gpt-5.6-sol", "gpt-5.6-terra"]

[[session.command_profiles.parameters]]
name = "effort"
label = "Effort"
type = "enum"
default = "low"
values = ["low", "medium", "high"]
```

The same shape is valid in `<project>/.agent-grid/settings.toml`. Project entries replace a server profile with the same ID as a complete descriptor and append new IDs. To remove an inherited profile:

```toml
[[session.command_profiles]]
id = "sol"
disabled = true
```

Parameter types are `enum`, `boolean`, `string`, and `number`. Each `argv` entry produces exactly one token; references may appear inside that token and the completed token is shell-quoted by the server. Referenced optional parameters require a default, preventing a missing value from leaving a dangling flag. `[session].commands`, `[session].default_command`, profile path allow-lists, and raw `command` create requests are not supported.

## Delivery order

The seven dependency-ordered milestones and task-grade units in `design-plan.json` proceed from config → descriptor REST → create materializer → providers/projection → form/reconciliation → verification triples. Every unit references its complete implementation contracts and has no open design choice.

## Algorithms and bounds

Profile revision hashes the normalized descriptor. Fleet requests have a ten-second per-server timeout and return connected/unavailable outcomes independently. Parameter form state is O(N) in the declared parameter count.
