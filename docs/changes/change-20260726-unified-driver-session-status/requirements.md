---
change: change-20260726-unified-driver-session-status
role: requirements
---

<!-- lifecycle is owned by change.md -->

# Requirements

## Functional requirements
- **FR-STATUS-001 (ubiquitous)**: The system shall keep a driver-neutral composite session status as the sole status authority and shall not delegate status decisions to clients or terminal titles.
- **FR-CODEX-002 (event_driven)**: When a Codex app-server message is received, the router shall classify every known notification, server request, response, lifecycle, elicitation, and tool-proposal method into exactly one stateful or explicit no-state-effect disposition.
- **FR-UNKNOWN-003 (event_driven)**: When an unknown, malformed, or inventory-unavailable structured message is observed, the system shall preserve the current trustworthy facets and emit a bounded correlated diagnostic without fabricating idle or another status.
- **FR-HUMAN-004 (state_driven)**: While at least one unresolved human-input request exists, the system shall expose human input required, suppress the activity spinner, and terminalize each request exactly once on resolution, cancellation, expiry, or connection loss.
- **FR-SIGNAL-005 (event_driven)**: When a notification-worthy event occurs, the system shall issue a transient monotonically identified signal that may coexist with human input required and shall not replace the base status.
- **FR-DRIVER-006 (ubiquitous)**: The system shall normalize Codex, Claude, Gemini, Grok, and Shell authoritative observations through one typed facet-operation contract and shall produce explicit unsupported, malformed, or unavailable outcomes instead of title-based inference.
- **FR-UI-007 (ubiquitous)**: Web, Workspace, and Windows Shell shall consume only the composite session-status wire and render the same semantic visual key, animation rule, and accessible label from the shared matrix without Driver branching, legacy scalar status, legacy event fallback, or OSC-derived status.
- **FR-OSC-008 (unwanted)**: If Codex Driver receives OSC 0/2 title, BEL, or OSC notification, it shall not change session facets; Claude/Gemini title observations and Shell OSC 133 prompt semantics shall remain operational.
- **FR-FAILURE-009 (state_driven)**: When failure or connection loss occurs, the system shall expose a failure indication distinct from human input required and shall drain held requests with correlated diagnostics.

## Non-functional requirements
- **NFR-COMPAT-001**: The composite status wire shall replace the legacy scalar status as an intentional breaking flag-day change: server schema, generated SDKs, Web, Workspace, and Windows Shell shall land atomically, with no mixed-version support, compatibility projection, legacy event fallback, or OSC status inference.
- **NFR-A11Y-002**: Status shall not depend on color or motion alone and all clients shall publish the same accessible name.
- **NFR-EXHAUSTIVE-003**: Protocol and display inventory additions shall fail an invariant-naming contract test when unclassified.
- **NFR-RESOURCE-004**: Unknown diagnostics and transient signals shall use bounded memory and non-blocking backpressure.

## Acceptance

- AC-001: spinner と `!` は排他、bell と `!` は併存可能であり、failure は `!` と別表示である。
- AC-002: Codex・Claude・Gemini・Grok・Shell の各 authoritative source と supported/unsupported/malformed/unavailable outcome が閉じている。
- AC-003: Codex の OSC 0/2、BEL、OSC notification は status を変更せず、Claude/Gemini の既存 title observation と Shell OSC 133 は維持される。
- AC-004: 未知状態は専用 unknown visual と共通 accessible label で表示し、last-known を current として表示しない。
- AC-005: 同一 revision で server は composite status のみを emit し、全生成 SDK と Web・Workspace・Windows Shell は composite のみを consume する。legacy scalar、legacy event fallback、Codex OSC status inference の producer/consumer はゼロである。
