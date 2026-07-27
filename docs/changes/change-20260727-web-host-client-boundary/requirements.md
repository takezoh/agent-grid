---
change: change-20260727-web-host-client-boundary
role: requirements
functional_requirements:
- id: FR-WHC-001
  statement: The system MUST expose one host-neutral, credential-free, server-qualified fleet/provider contract to clients/ui regardless of whether the host is Web host or Workspace Electron.
  priority: must
- id: FR-WHC-002
  statement: If a host provides a fleet, the system MUST NOT branch on browser versus Electron, collapse the fleet to one server, or synthesize an implicit server.
  priority: must
- id: FR-WHC-003
  statement: When either client host starts, the system MUST load and validate servers.json schema version 2 exactly once and publish one immutable version-consistent snapshot or a typed startup failure.
  priority: must
- id: FR-WHC-004
  statement: If bearer authentication is configured, the system MUST NOT expose bearer material to clients/ui, browser JavaScript, Electron renderer, logs, or host projections.
  priority: must
- id: FR-WHC-005
  statement: clients/web-host MUST remain an independently buildable nested Go module with no require, replace, internal import, or source import from the services module.
  priority: must
- id: FR-WHC-006
  statement: While Web host serves clients/ui, the system MUST preserve embedded static delivery, CSP and security headers, same-origin WebSocket checks, credentialed upstream routing, and typed retryable upstream-not-ready behavior on server-qualified routes.
  priority: must
- id: FR-WHC-007
  statement: When the services-module flag day is applied, the system MUST move src module metadata and cmd, host, server, platform, orchestrator, internal, gorules, protocolsim, and .golangci.yml together into services/, use module github.com/takezoh/agent-grid/services, update every active consumer, and leave no src wrapper.
  priority: must
- id: FR-WHC-008
  statement: If repository references are migrated, the system MUST NOT rewrite completed or abandoned change history merely to mirror the current source layout, and MUST NOT copy tracked empty src/.claude placeholders over repository-local host configuration.
  priority: must
---

# Requirements

## Invariants

- `FR-WHC-001` is the permanent host-parity invariant: host kind changes delivery transport only.
- Fleet cardinality, qualified identity, partial failure, retry eligibility, config version and auth capability are common semantics.
- Credentials and config-source details are host-private; clients/ui can represent neither.

## Failure Outcomes

- `config_unavailable`: source is missing/unreadable; operator fixes access and restarts.
- `config_invalid`: bytes exist but JSON/schema/version/auth is unsupported; operator edits and restarts.
- `server_unavailable`: one server operation failed; healthy server scopes remain visible and the failed scope carries retry eligibility.

There is no fallback to an implicit server, no fallback from bearer to none, and no runtime mixed-version reload.

## Acceptance

- The same multi-server fixture produces deep-equal credential-free projections through Web and Workspace.
- Missing/malformed config produces exactly one stable startup outcome before listener/window creation.
- `clients/web-host` passes its own `GOWORK=off` module gate without a services-module edge.
- Qualified Web routes preserve static/CSP/header/same-origin/retry semantics and typed partial fleet outcomes.
- Services and Web-host gates pass, `src/` and `clients/desktop-config` are absent, `clients/config` owns shared schema/fixtures, allowed top-level ownership holds, and terminal change history hashes are unchanged.
