---
id: change-20260812-cli-terminal-client
kind: change
title: Terminal CLI client for Agent Grid sessions
status: active
created: '2026-08-12'
profile: sdd@1
intent: <変更理由と期待結果>
outcomes: []
scope: []
non_goals: []
change_classes: []
governance:
  gate: auto
  reasons: []
members:
- role: requirements
  path: changes/change-20260812-cli-terminal-client/requirements.md
  required: true
- role: implementation
  path: changes/change-20260812-cli-terminal-client/implementation.md
  required: true
- role: verification
  path: changes/change-20260812-cli-terminal-client/verification.md
  required: true
- role: ux
  path: changes/change-20260812-cli-terminal-client/ux.md
  required: false
promotion: []
unresolved_decisions: []
tags: []
owners: []
relations:
- {type: dependsOn, target: change-20260814-cli-prerequisite-contracts}
source_paths: []
summary: 既存 server-side の Claude Code / Codex session へローカル terminal から attach / detach
  / reattach する terminal-first CLI client の explore requirements
updated: '2026-08-14'
---

## Summary

## Closure Notes


{% transition from="draft" to="ready" date="2026-08-14" %}
design closure passed; 11 ADR accepted via consultation-20260814-cli-terminal-client-design-final
{% /transition %}


{% transition from="ready" to="active" date="2026-08-14" %}
main session implementation; twelve of the thirteen units landed. unit-platform-hardening-spikes is outstanding: its three observations need a real Windows console and a real agent session, and the Windows paths are explicitly unimplemented rather than guessed (verification.md "Not yet acquired").
{% /transition %}
