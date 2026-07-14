---
id: adr-20260714-wsviewer-markdown-xss-boundary
kind: adr
title: Markdown renderer sanitizes via rehype-sanitize; fail-closed to plain text
status: accepted
created: '2026-07-14'
decision_makers:
- Takehito Gondo
tags:
- workspace-viewer
- design
owners: []
relations:
- {type: partOf, target: plan-20260714-agent-workspace-viewer}
source_paths: []
summary: Markdown renderer sanitizes via rehype-sanitize; fail-closed to plain text
---

## Context

Workspace files are attacker-influenced content served through a read-only viewer. cc-no-write / exp-read-only guard the fs boundary but not renderer XSS (critique issue-html-sanitization-absent). The upstream Technology Candidates explicitly requires 'HTML sanitization 必須' for react-markdown. Integrator judgment (per user guidance): keep this as its own ADR distinct from the core design ADRs.

## Decision

The MarkdownRenderer wraps react-markdown with a rehype-sanitize pipeline configured against a schema that (a) forbids <script>, <iframe>, <object>, <embed>, and any on* event-handler attribute; (b) allows only href schemes http(s) and mailto; (c) rejects javascript: and data: URIs anywhere; (d) rejects <img src> pointing outside the workspace root (only relative or same-origin paths mapped through the workspace-file endpoint pass). Sanitizer rejection (schema violation) fails closed: MarkdownRenderer swaps to a plain-text pane containing the raw markdown source and surfaces a banner explaining the fallback. Sanitizer library choice and schema are fixed at implementation time; the contract-markdown-sanitization observable is that no forbidden token reaches the DOM in the T2 contract test.

## Consequences

- A malicious .md fixture cannot execute script or navigate to javascript:/data: URIs from the viewer.
- Partial rendering is not permitted — either the sanitizer accepts the whole schema or the fallback pane replaces the render.
- One new external dependency (rehype-sanitize) is justified against 'no XSS defense' as the only alternative that satisfies the upstream constraint.

## Alternatives

- **却下: Trust markdown source (no sanitization)** — Upstream ux Technology Candidates explicitly forbids; XSS regression.
- **却下: Client-side allowlist parsed by hand** — Reinvents rehype-sanitize with worse coverage; violates AGENTS.md 'Actively use libraries'.

## Trace

- Requirements: FR-009, FR-012
- Implementation contracts: contract-markdown-sanitization
