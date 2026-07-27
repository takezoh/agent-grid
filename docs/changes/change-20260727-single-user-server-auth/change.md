---
id: change-20260727-single-user-server-auth
kind: change
title: Simplify server authentication for single-user devices
status: done
created: '2026-07-27'
profile: sdd@1
intent: Single-user locally trusted devices use an explicit per-server network authentication
  contract without client-side token files or independent rotation, while operators
  may deliberately expose no-auth listeners and receive a strong warning.
outcomes:
- Each enabled desktop server entry explicitly selects auth.mode none or bearer; bearer
  stores its token directly and UI surfaces mask it.
- Normal bearer server restarts retain the same server-owned token; an explicit coordinated
  change is complete only after all clients authenticate with the new value.
- Non-loopback -no-auth startup succeeds without an escape-hatch flag and emits a
  conspicuous WARN based on the bound listener address.
scope:
- clients/desktop-config schema_version 2 and both native-client config/auth consumers
- server authentication lifecycle, daemon flags, listener warning, and tests
- settings/default/e2e writers and operator documentation
- supersession of the fresh-read ADR and replacement of the LAN escape-hatch change
non_goals:
- Protecting secrets from local compromise or other local users
- Automatic token rotation, refresh, TLS redesign, or new human/client identity
- Changing the existing REST bearer to single-use WebSocket ticket protocol
change_classes:
- behavior
- boundary
- invariant
governance:
  gate: hard
  approval_evidence: User directed「全て修正して。理由をつけて後回しにするな」after approving the single-user
    trust model and explicit auth contract (2026-07-27).
  reasons:
  - Persisted configuration schema and network exposure behavior change.
evidence_refs:
- type: test
  ref: go:test ./...
- type: test
  ref: workspace:npm run typecheck && npm run test:unit (80 tests)
- type: test
  ref: windows:dotnet test AgentGrid.Shell.sln (157 Core, 27 Platform)
- type: test
  ref: jsonschema:shared desktop-config auth fixtures
- type: test
  ref: dev-docs:lint --conformance
members:
- role: requirements
  path: changes/change-20260727-single-user-server-auth/requirements.md
  required: true
- role: implementation
  path: changes/change-20260727-single-user-server-auth/implementation.md
  required: true
- role: verification
  path: changes/change-20260727-single-user-server-auth/verification.md
  required: true
promotion:
- action: none
  reason: The durable decision is recorded by adr-20260727-single-user-server-auth;
    current design docs are updated during implementation only where their governed
    contract changes.
unresolved_decisions: []
tags:
- server
- auth
- native-clients
owners:
- agent-grid-maintainers
relations:
- {type: supersedes, target: change-20260724-server-lan-exposure}
- {type: references, target: adr-20260727-single-user-server-auth}
- {type: references, target: change-20260725-windows-service-wsl-supervision}
source_paths:
- src/cmd/server
- clients/workspace
- clients/windows-shell
- docs/note/note-20260624-user-systemd.md
summary: Make auth mode explicit, retire user-configured token paths and automatic-rotation
  claims, and replace the non-loopback no-auth escape hatch with an unconditional
  startup warning.
updated: '2026-07-27'
promotion_applied_at: '2026-07-27T03:06:59.891885+00:00'
closure:
  closed_at: '2026-07-27T03:07:19.090802+00:00'
  content_hash: sha256:3313c8e58b136978de05dff71cae00675804ebe71da1dd1c449c8411e6a4136e
---

## Summary

This package replaces file-backed desktop token acquisition with a versioned discriminated auth value, preserves the server-owned `-token-file` lifecycle, and changes non-loopback no-auth from fail-closed opt-in to warning-only startup. The local device is trusted; the network boundary remains enforced whenever bearer mode is selected.

## Closure Notes

- Desktop server schema version 2, Workspace, Windows Shell, settings UI,
  health probes, helper writers, and WinUI E2E configuration now share the
  explicit nested auth contract.
- Server startup has no implicit credential rotation: authentication mode is
  required, token files are read-only inputs, and non-loopback no-auth starts
  with a stable warning.
- Shared valid/invalid fixtures pass JSON Schema, TypeScript, and C# consumers.
- Verification passed: Go full test suite; Workspace typecheck and 80 unit
  tests; Windows Shell 157 Core and 27 Platform tests; dev-docs conformance.
  Five opt-in WinUI UI tests and two real-gateway tests were skipped by their
  documented environment gates.
- Repository-wide Go lint remains red on nine pre-existing WSL supervisor
  findings in files outside this change; no finding points to this change's
  files.


{% transition from="draft" to="ready" date="2026-07-27" %}
Design decisions are accepted and implementation evidence is complete.
{% /transition %}


{% transition from="ready" to="active" date="2026-07-27" %}
Implementation executed across all declared source paths.
{% /transition %}


{% transition from="active" to="closing" date="2026-07-27" %}
All declared outcomes are implemented and verification evidence is recorded.
{% /transition %}
