---
id: adr-20260724-hosted-mode-preload-contextbridge
kind: adr
title: Hosted-mode BrowserWindow receives token via preload contextBridge (window.hostedModeInfo);
  never in URL
status: accepted
created: '2026-07-24'
summary: Hosted-mode BrowserWindow receives token via preload contextBridge (window.hostedModeInfo);
  never in URL
decision_makers:
- agent-grid-maintainers
consulted:
- windows-shell-maintainers
- workspace-maintainers
- server-api-maintainers
informed:
- agent-grid-users
tags:
- native-clients
- windows-shell
- phase2
owners:
- agent-grid-maintainers
relations:
- type: originatedFrom
  target: change-20260723-windows-shell-phase2
source_paths: []
consequences:
  positive:
  - Consistent with auth.go's own anti-query-param rationale.
  - Token invisible in DevTools/network trace.
  negative:
  - Requires preload contextIsolation:true/nodeIntegration:false/sandbox:true discipline;
    enforced by Electron project conventions.
  neutral:
  - Playwright network-trace test provides a regression gate.
---
# Hosted-mode BrowserWindow receives token via preload contextBridge (window.hostedModeInfo); never in URL

## Context

{% context %}
src/server/api/auth.go documents that the browser path avoids query-string tokens for history/logs/Referer leakage reasons. Hosted-mode BrowserWindow could inject via query string, custom scheme header, or preload/contextBridge.
{% /context %}

## Decision

{% decision %}
Workspace main resolves {port, token, sessionId} via daemon-config.ts, pushes them to preload before the page's first script runs, and preload exposes them as window.hostedModeInfo via contextBridge. The URL carries only ?hosted=1&session=<id>. On daemon restart the main process pushes a refreshed hostedModeInfo via IPC without a page reload.
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- Consistent with auth.go's own anti-query-param rationale.
- Token invisible in DevTools/network trace.
{% /consequence %}

{% consequence kind="negative" %}
- Requires preload contextIsolation:true/nodeIntegration:false/sandbox:true discipline; enforced by Electron project conventions.
{% /consequence %}

{% consequence kind="neutral" %}
- Playwright network-trace test provides a regression gate.
{% /consequence %}

## Alternatives

- **Query-string token** — Reintroduces history/logs/Referer leakage inside the BrowserWindow's DevTools/network trace.
- **Custom agent-grid-hosted:// scheme with header injection** — Extra Electron protocol-handler plumbing without benefit over contextBridge.

## Related

- decision inputs: (none)
- requirements: `FR-B2-04`
- contracts: `contract-b2-hosted-mode-token-injection`
- change: `change-20260723-windows-shell-phase2`
