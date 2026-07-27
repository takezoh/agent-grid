---
change: change-20260727-multi-server-session-palette
role: requirements
---

<!-- lifecycle is owned by change.md -->

# Requirements

## Functional requirements

- `FR-001`–`FR-012` in [design-plan.json](design-plan.json) are the EARS obligations derived from UAC-001–UAC-012: fleet discovery, project-scoped profiles, arbitrary-N typed parameters, typed validation/stale recovery, confirmed create, one-server operation, server-qualified identity, partial failure, and Electron credential isolation.
- `FR-INVARIANT-001` (ubiquitous): the same stable server identity is preserved from every displayed project/session through selection, confirmation, routing, create result, and active context.
- `FR-INVARIANT-002` (unwanted): stale, malformed, unknown, unavailable, or unauthorized revision/value/server/routing authority creates no session and yields a typed scoped outcome.

The canonical wording, priorities, rationale, NFR-001–NFR-007, and AC-001–AC-012 are machine-readable in `design-plan.json`. The approved counterexamples remain owned by [ux.md](ux.md) and are not duplicated here.

## Fixed behavior

Parameter kinds are `enum`, `boolean`, `string`, and `number`. `session.command_profiles` is the only launch-command setting; optional project allow-lists determine applicability and duplicate IDs fail. `[session].commands`, `[session].default_command`, and the old `{project, command}` create request are unsupported. One server-local `GET /api/session-config` snapshot remains the REST contract.
