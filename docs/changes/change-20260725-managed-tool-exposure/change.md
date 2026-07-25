---
id: change-20260725-managed-tool-exposure
kind: change
title: Unify managed tool exposure across launch adapters
status: done
created: '2026-07-25'
profile: sdd@1
intent: Claude 固有の boolean と重複した agent_frames 定義を、明示的な host placement 契約と共通 MCP endpoint
  catalog に置換する。
outcomes:
- Host Claude と container workspace の MCP exposure が同じ endpoint descriptor から生成される。
- Host authority は検証後にのみ発行され、container overlay は user config を変更せず一度だけ合成される。
scope:
- ManagedMCPTools 型、endpoint catalog、host inline MCP materializer
- LaunchPlan の契約伝播と token 発行順序
- container .mcp.json overlay と mcpproxy ownership
- ADR、persistent design、conformance tests
non_goals:
- Codex dynamicTools の protocol/schema 統合
- per-frame LaunchPlan から container provisioning を導出すること
- user-owned .mcp.json、HOME、Claude/Codex config の変更
change_classes:
- boundary
- behavior
- invariant
governance:
  gate: hard
  approval_evidence: User approved the direction and requested implementation and
    ADR fixation on 2026-07-25.
  reasons:
  - Accepted ADR の置換と security authority ordering を含む。
members:
- role: requirements
  path: changes/change-20260725-managed-tool-exposure/requirements.md
  required: true
- role: implementation
  path: changes/change-20260725-managed-tool-exposure/implementation.md
  required: true
- role: verification
  path: changes/change-20260725-managed-tool-exposure/verification.md
  required: true
promotion:
- target: design-platform
  section: invariants
  action: upsert
  item:
    id: INV-009
    statement: Managed MCP endpoint semantics have one platform catalog; lifecycle-specific
      materializers resolve only environment placement and endpoint values.
    enforcement: contract
  reason: Endpoint ownership and materializer separation must govern future tools.
- target: design-client
  section: invariants
  action: upsert
  item:
    id: INV-008
    statement: Host managed MCP requests are validated before frame authority issuance
      and process spawn.
    enforcement: contract
  reason: Fail-closed launch authority ordering must persist beyond this change.
unresolved_decisions: []
evidence_refs:
- {type: test, ref: src/platform/agentlaunch/managed_claude_mcp_test.go}
- {type: contract, ref: src/platform/agentlaunch/frame_messaging_mcp_overlay_test.go}
- {type: test, ref: src/platform/mcpproxy/provider_test.go}
- {type: test, ref: src/host/runtime/frame_launch_matrix_test.go}
- {type: command, ref: make lint}
- {type: command, ref: cd src && go test ./...}
tags:
- mcp
- agentlaunch
- container
owners: []
relations:
- {type: modifies, target: design-platform}
- {type: modifies, target: design-client}
- {type: introduces, target: adr-20260725-managed-mcp-exposure-materialization}
source_paths:
- src/platform/agentlaunch/
- src/host/runtime/
- src/host/state/
- src/host/driver/
- docs/adr/
summary: Declare managed MCP endpoint semantics once and render Claude-compatible
  host CLI and container workspace overlays at their native lifecycle boundaries.
updated: '2026-07-25'
promotion_applied_at: '2026-07-25T17:47:02.874140+00:00'
closure:
  closed_at: '2026-07-25T17:47:08.968635+00:00'
  content_hash: sha256:ddc708d0909263b1c582bfb1a1339331a041c6b58d4d2aa2e6ea8e1e1dd3e670
---

## Summary

`ManagedFrameMessaging` を `ManagedMCPTools` に置換する。ただし host
field は任意 MCP client 向けではなく、inline `--mcp-config` の末尾配置と
互換な launch に限定する closed-world contract とする。現在の production
declarer は Claude driver のみであり、追加 client は placement compatibility
を contract test で証明するまで設定してはならない。

`agent_frames` の MCP alias、stdio type、subcommand、authority property は
`platform/agentlaunch` の endpoint catalog が所有する。host と container は
command/socket だけを各環境で解決する。container は per-frame plan ではなく
container-scope default set を materialize する。

## Closure Notes


{% transition from="draft" to="ready" date="2026-07-25" %}
Design protocol completed and user direction recorded.
{% /transition %}


{% transition from="ready" to="active" date="2026-07-25" %}
Implementation and verification in progress.
{% /transition %}


{% transition from="active" to="closing" date="2026-07-25" %}
Implementation, full tests, lint, docs conformance, and persistent design updates completed.
{% /transition %}
