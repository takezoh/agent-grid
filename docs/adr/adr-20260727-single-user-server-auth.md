---
id: adr-20260727-single-user-server-auth
kind: adr
title: Use explicit persisted server authentication for trusted single-user devices
status: accepted
created: '2026-07-27'
decision_makers:
- agent-grid-maintainers
tags:
- auth
- server
- native-clients
owners:
- agent-grid-maintainers
relations:
- {type: supersedes, target: adr-20260724-boundary-2-token-unc-fresh-read}
- {type: originatedFrom, target: change-20260727-single-user-server-auth}
source_paths:
- clients/desktop-config/servers.schema.json
- src/cmd/server
consequences:
  positive:
  - One explicit persisted value defines each client's network-auth behavior.
  - Normal restart preserves bearer identity and non-loopback no-auth risk is conspicuous.
  negative:
  - Direct bearer material is stored in servers.json and legacy files require manual
    migration.
  - Non-loopback no-auth is easier to enable and therefore remains an explicit network
    risk.
  neutral:
  - Server-owned -token-file and the REST-bearer to single-use WebSocket-ticket protocol
    remain unchanged.
confirmation: Shared schema/client fixtures, server restart and listener-warning tests,
  sentinel redaction tests, and dev-docs conformance.
summary: Store direct bearer credentials in versioned desktop config, keep server
  bearer persistence stable, and warn rather than reject non-loopback no-auth binds.
updated: '2026-07-27'
---

# Use explicit persisted server authentication for trusted single-user devices

## Context

The device and its local users are trusted; network reachability remains in the threat model. The accepted fresh-UNC-read ADR assumes client-side token paths and independent rotation, while the active LAN exposure change requires an additional escape-hatch flag. Both conflict with the new requirements.

## Decision

Desktop configuration schema version 2 stores a nested discriminated value: `auth: {"mode":"none"}` or `auth: {"mode":"bearer","token":"<non-empty>"}`. Version 1 and `token_path` fail with manual-migration guidance; clients never read a token file or fall back to none.

The server retains its independently owned `-token-file` as the stable bearer source. Missing, empty, or unreadable input fails startup. Token changes are explicit coordinated operations with rollback, never automatic independent rotation.

`-allow-non-loopback-no-auth` is removed. `-no-auth` may bind any address; after binding, non-loopback or unspecified listener addresses emit a strong WARN about unauthenticated REST/WebSocket network reachability. This replaces, rather than preserves, the escape-hatch behavior in `change-20260724-server-lan-exposure`.

## Consequences

{% consequence kind="positive" %}
One explicit persisted value defines each client's network-auth behavior. Normal restart preserves bearer identity and dangerous no-auth exposure is conspicuous.
{% /consequence %}

{% consequence kind="negative" %}
Direct bearer material is stored in `servers.json`, legacy configuration requires manual migration, and warning-only no-auth exposure accepts more operator risk.
{% /consequence %}

{% consequence kind="neutral" %}
Server-owned `-token-file` and the REST-bearer to single-use WebSocket-ticket protocol remain unchanged, including the Windows service supervision launch contract.
{% /consequence %}

## Alternatives

- Keep sibling `auth_mode`/`token`: rejected because it permits more ambiguous combinations than the nested union.
- Automatically read and rewrite `token_path`: rejected because it retains the removed authority and creates partial-migration secret handling.
- Generate or persist a new server token elsewhere: rejected because existing `-token-file` already owns stable server persistence.
- Retain the LAN escape-hatch startup rejection: rejected because the requirement explicitly selects warning-only startup.

Confirmation: shared schema/client fixtures, server restart and listener-warning tests, sentinel redaction tests, and dev-docs conformance.

## Related

- requirements: `FR-AUTH-001` through `FR-AUTH-009`, `NFR-AUTH-001` through `NFR-AUTH-003`
- contracts: `contract-auth-config`, `contract-client-auth`, `contract-server-auth`, `contract-token-change`
- supersedes: `adr-20260724-boundary-2-token-unc-fresh-read`
- replaces behavior of: `change-20260724-server-lan-exposure`


{% transition from="proposed" to="accepted" date="2026-07-27" %}
Accepted from explicit user decisions: trust the single-user device, store client bearer values directly, prohibit independent automatic rotation, and warn rather than reject non-loopback no-auth.
{% /transition %}
