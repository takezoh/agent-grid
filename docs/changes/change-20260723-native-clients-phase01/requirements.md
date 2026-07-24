---
change: change-20260723-native-clients-phase01
role: requirements
---

# Requirements

## Summary

Give agent-grid one platform-independent, contract-first wire layer for sessions/approvals/questions/commands/events (Phase 1), grounded on a new server-side approval/question domain that actually holds a pending decision instead of auto-resolving it (Phase 0), so every future native client (desktop shell, iOS, Android) and the browser consume the same typed, generated, simulator-verifiable contract with no privileged back doors.

## In scope

- Phase 0: approval/question as durable server-side domain objects in host/state, surfaced through host/proto and server/api, with expiry, cancel/teardown, and two-client conflict-resolution semantics
- Phase 0: per-WS-connection ephemeral client-instance-id issued by an extended ticketStore, threaded onto CmdApprovalRespond/CmdQuestionRespond so `decided_by` has a named producer (does not modify the bearer scheme)
- Phase 0: terminology docs pass separating runtime layers (host/*, server/*) from user-facing clients (clients/*)
- Phase 0: capability negotiation and compatibility-policy skeleton
- Phase 0: joint decision (with multi-host-gateway.md) on the auth/trust boundary for approval/question answering, scoped to what stays local-bearer-token-valid vs what is deferred to Phase R
- Phase 1: protocol/ as the normative schema source (openapi.yaml, events/commands/capabilities/deep-links/notifications.schema.json)
- Phase 1: generated C#/Swift/Kotlin/TypeScript clients via OpenAPI Generator; a new ADR superseding ADR-0021 for cross-language generation only (Go stays stdlib-only)
- Phase 1: simulator (deterministic fixtures + recorded event stream + simulation server) under protocol/simulator/ as a governed extension of plan-20260723-repo-structure.md
- Phase 1: compatibility CI gate rejecting protocol drift and undocumented generated-SDK behavior
- Phase 1: deep-links/handoff schema adopting the URI shape recorded in plans/remote-control-mobile-session-deep-link.md
- Phase 1: explicit scoping of the event-replay/reconnect extension over existing ADR-0025/0011/0022
- Keeping the wire envelope shape transport-agnostic so a later WebRTC DataChannel-over-Noise transport (multi-host-gateway.md) can carry it unchanged

## Out of scope

- Phase 2 desktop app vertical slice (Windows shell/Electron workspace)
- Phase R remote reachability, push delivery, and the WebRTC/Noise transport implementation
- Phase 3+ mobile client implementations
- Full runtime capability-negotiation behavior beyond the bundled/remote two-axis policy skeleton and its wire declaration
- Distribution, code signing, auto-update
- Structured (non-free-text) question-answer schemas beyond the minimum wire envelope needed for Phase 0/1
- gateway/ multi-host relay/tunnel/authorizer implementation
- Local application control (UE/Blender/browser)
- Persistent per-user identity or passkey rollout (deferred to Phase R; the Phase 0 client-instance-id is ephemeral and does not attempt to identify a human across connections)

## Functional requirements (EARS)

### FR-P0-01 (ubiquitous, must)

The host state domain shall represent every outstanding driver approval request as a durable ApprovalRequest carrying a stable ID, session/frame reference, requested-action fields (kind, command or path, reason), created_at, expires_at, status in {pending, resolved, expired, cancelled}, and (when resolved) resolving_client_instance_id and decision, replacing the current synchronous auto-decide path in src/host/runtime/subsystem/stream/event.go.

_Rationale_: Closure of plan gap #1; the driver's held JSON-RPC request must map to observable pending state before any human client can be shown it.

### FR-P0-02 (event_driven, must)

When a driver subsystem emits an approval-requested driver event for a request not already tracked, host/state shall create a pending ApprovalRequest and emit EvtApprovalRequested to subscribed clients within the same Reduce cycle that created it, with no async gap between creation and visibility.

### FR-P0-03 (event_driven, must)

When a client submits CmdApprovalRespond for a pending ApprovalRequest, host/state shall transition it to resolved, record the resolving client-instance-id (from the WS-ticket-issued ephemeral identity in FR-P0-12) and the decision, reply the decision to the driver's held JSON-RPC request, and broadcast EvtApprovalResolved to every subscriber. Note: multi-host-gateway.md §6.2 is the authoritative surface for cross-host identity; the Phase 0 client-instance-id is deliberately local-scope and does not preempt that chain.

### FR-P0-04 (unwanted, must)

If a second client submits CmdApprovalRespond for an ApprovalRequest that is already resolved, then host/state shall reject the second response with a resolved-by-other error that carries the authoritative decision and the winning resolving_client_instance_id, and shall not re-emit EvtApprovalResolved.

### FR-P0-05 (state_driven, must)

While host/state's single-writer event loop processes CmdApprovalRespond events, at most one commit shall change a given ApprovalRequest from pending to resolved; every subsequent commit attempt against that same request ID shall be a rejected no-op against already-resolved state, never a second authoritative resolution.

### FR-P0-06 (event_driven, must)

When an ApprovalRequest's configured expires_at deadline elapses without a client response, host/state shall transition it to expired, apply the deny-by-default decision captured onto the ApprovalRequest at creation time from the driver's per-session policy, and broadcast EvtApprovalResolved with resolution_reason=expired; no agent-side request shall extend expires_at once the ApprovalRequest is created.

### FR-P0-07 (event_driven, must)

When a driver subsystem emits a request for free-text user input, host/state shall create a durable QuestionRequest carrying a single free-text answer field on the wire and shall apply the same expiry, cancel/teardown, and conflict-resolution rules as ApprovalRequest, instead of the current hard-fail/reject-unhandled-request path.

### FR-P0-08 (event_driven, must)

When a client resubscribes to a session's surface after a disconnect (per ADR-0011/ADR-0022), the resubscribe response shall include the session's current authoritative set of pending ApprovalRequest and QuestionRequest objects, such that the reconnecting client's local view and every still-connected subscriber's view of that set converge to host/state's snapshot without a full event-log replay.

### FR-P0-09 (ubiquitous, must)

Living architecture docs shall consistently name the session daemon layer host/* and the browser/native consumers clients/*, with no remaining doc reference treating client/* as the daemon layer.

### FR-P0-10 (event_driven, must)

When Phase 0 defines the trust boundary for answering an approval/question, the design shall record that same-host clients continue to authenticate via the existing bearer token + single-use WS ticket (server/api/auth.go, ticket.go), that the Phase 0 client-instance-id is minted by the same ticketStore for local-scope identity only, and that any answering path that crosses hosts is deferred to multi-host-gateway.md's user-signed-op + host-authoritative-ACL chain.

### FR-P0-11 (event_driven, must)

When a session's frame is torn down (sandbox release path d1e3a8c4, session eviction, or daemon shutdown), or when the answering client submits CmdApprovalCancel/CmdQuestionCancel for a request it currently holds, host/state shall transition every pending ApprovalRequest/QuestionRequest owned by that frame/session to cancelled, drain the held driver JSON-RPC request with a connection-lost error, and broadcast EvtApprovalResolved with resolution_reason=cancelled; no goroutine or map entry shall outlive the reap.

### FR-P0-12 (event_driven, must)

When the server API mints a WebSocket ticket (server/api/ticket.go ticketStore.mint), it shall also mint a per-WS-connection ephemeral client-instance-id bound to that ticket, and shall thread the same id onto every CmdApprovalRespond/CmdQuestionRespond originating from that WS connection or from REST calls that present a session-header carrying the same id; the id shall be cleared when the WS connection closes and shall never be reused across connections.

### FR-P1-01 (ubiquitous, must)

protocol/ shall contain versioned wire-contract schemas (openapi.yaml, events.schema.json, commands.schema.json, capabilities.schema.json, deep-links.schema.json, notifications.schema.json) as the single normative source for every generated client; hand-written Go wire types in src/host/proto/ shall round-trip-validate against these schemas.

### FR-P1-02 (event_driven, must)

When protocol/*.schema.json or openapi.yaml changes, running the pinned generator (OpenAPI Generator, version pinned in the compatibility profile) for C#, Swift, Kotlin, and TypeScript against identical schema input shall produce byte-identical output across CI runs and developer machines.

### FR-P1-03 (event_driven, must)

When a bundled client (native shell + the co-shipped, same-build daemon) connects, the daemon shall report a build-identical capability set and the client shall treat the connection as fully compatible after a version-match check only, without a full capability-by-capability negotiation round-trip.

### FR-P1-04 (event_driven, must)

When a client connects to a daemon whose protocol/ version differs from the client's compiled contract version, the daemon shall respond with its capabilities.schema.json-declared feature set and the client shall degrade any capability the daemon does not declare to a documented disabled/hidden state, never invoking it speculatively.

### FR-P1-05 (unwanted, must)

If a generated SDK's code invokes a wire field or command not declared in protocol/*.schema.json or openapi.yaml, then the compatibility CI gate shall fail the build and report which SDK and which undeclared surface was used.

### FR-P1-06 (unwanted, must)

When a new generated-SDK target is added under protocol/, the compatibility CI gate shall require that target to pass the full recorded-scenario suite already gating the existing SDK targets before merge.

### FR-P1-07 (event_driven, must)

When the simulation server replays a recorded scenario fixture, every generated SDK (C#, Swift, Kotlin, TypeScript) driven against that simulator shall observe the identical sequence of typed events/commands as the fixture, with no live agent process involved.

### FR-P1-08 (ubiquitous, must)

The simulator shall consist of three version-controlled artifacts under protocol/simulator/: (1) deterministic fixtures with fixed IDs/timestamps, (2) a recorded event stream captured from a real- or fake-agent-driven session, and (3) a simulation server that serves protocol/'s REST+WS surface by deterministically replaying the recorded stream on request.

### FR-P1-09 (event_driven, must)

When deep-links.schema.json declares an agent-grid:// URI shape adopting the session/<id> and approval/<id> shapes recorded in plans/remote-control-mobile-session-deep-link.md, every generated client SDK shall expose a typed parse/construct helper for that shape, so native shells never hand-parse URI strings.

### FR-P1-10 (unwanted, must)

If a generated Go artifact or any Go-side wire/persistence type produced by the Phase 1 generation pipeline depends on a non-stdlib package for its runtime representation, then the existing stdlib-only build/lint gate shall fail; ADR-0021's Go-side constraint continues under the new ADR that supersedes it for cross-language generation only.

### FR-P1-11 (event_driven, must)

When Phase 1 scopes the event-replay extension, the design shall enumerate which parts of ADR-0025 (REST backfill + WS tail), ADR-0011 (two-step close), ADR-0022 (subscribe retry), ADR-0023, and ADR-20260705-view-update-sessions-only remain unchanged versus which require a new additive field or frame (specifically the FR-P0-08 pending-approval resubscribe payload), before any client code is generated against the extended shape.

### FR-P1-12 (event_driven, must)

When the daemon emits EvtApprovalRequested/Resolved or EvtQuestionRequested/Resolved onto a subscriber's WebSocket, the frame shape shall satisfy ADR-0023's discriminated-union broadcast convention and ADR-20260705-view-update-sessions-only's session-scoped delivery scoping, i.e. carried inside the same subscription surface as v/tt/et/n/viewUpdate frames rather than requiring a new subscription negotiation.

## Non-functional requirements

### NFR-01 (compatibility)

Generated-SDK builds must be reproducible: identical protocol/ input plus pinned generator version must produce byte-identical output across CI runs and developer machines.

_Measurement_: CI diff of `git status` after generation must be empty; artifact hash comparison in compatibility profile.

### NFR-02 (security)

The default decision applied on ApprovalRequest expiry must be deny-by-default (destructive command/file-change approvals); a design that defaults to accept-on-timeout is rejected. The deny decision is captured onto the ApprovalRequest at creation time from the driver's per-session policy, not re-read at expiry, so a TOCTOU policy change cannot silently flip an in-flight approval to accept.

_Measurement_: reduce_expiry_test.go asserts expired ApprovalRequest.decision == deny for command/file-change kinds regardless of a mid-flight policy mutation.

### NFR-03 (performance)

The new compatibility CI gate must run inside the existing test-harness/profiles.json profile mechanism (scripts/run-verification-profile.sh) and its own PR-time budget.

_Measurement_: .github/workflows/ci.yml compatibility job wall-clock <= schema-drift job wall-clock * 3.

### NFR-04 (performance)

Capability negotiation on the bundled axis must add no additional round-trip beyond the existing connection handshake.

_Measurement_: bundled-axis handshake trace exactly matches the existing single-round-trip pattern; unit test asserts no extra request/response pair.

### NFR-05 (compatibility)

protocol/ schema evolution within a major version must be additive-only (no removed/renamed required fields), verified by wire-fixture regression discipline (adr-20260705-wire-fixtures-pipeline extended to protocol/*.schema.json).

_Measurement_: compatibility profile runs an old-fixture-decodes-with-new-schema round-trip for every generated SDK.

### NFR-06 (reliability)

A frame/session teardown while an approval/question is pending must never leak a goroutine, map entry, or held driver JSON-RPC request; the reap is verified by the existing sandbox-release regression pattern.

_Measurement_: reduce_teardown_test.go asserts len(state.PendingApprovals) == 0 and no leaked JSON-RPC id after frame teardown, matching the sandbox release path (d1e3a8c4).

## Acceptance scenarios (Given / When / Then)

### AC-P0-01 — traces `FR-P0-03`, `FR-P0-04`, `FR-P0-05`, `FR-P0-12`

- **Given** A fake codex driver has emitted an approval-requested driver event and two clients A and B are subscribed to that session's surface with distinct client-instance-ids ci-A and ci-B.
- **When** Client A submits CmdApprovalRespond(decision=accept) at t0 and client B submits CmdApprovalRespond(decision=deny) at t0+dt where both events are enqueued to Reduce before either is processed.
- **Then** Exactly one CmdApprovalRespond commits; the winning ApprovalRequest.decision equals the winner's payload, resolving_client_instance_id equals the winner's ci-*; the loser receives a resolved-by-other error carrying (decision, resolving_client_instance_id); both A and B observe exactly one EvtApprovalResolved with the winning decision.

### AC-P0-02 — traces `FR-P0-06`

- **Given** An ApprovalRequest has been pending for expires_at - epsilon and no client has responded; the driver's per-session policy at creation time was deny (default).
- **When** The event-loop tick after expires_at fires and a mid-flight policy mutation attempts to change the driver's policy to accept-for-session.
- **Then** ApprovalRequest.status transitions to expired with decision=deny (the value captured at creation, not the mutated policy); EvtApprovalResolved fires with resolution_reason=expired; the driver's held JSON-RPC request is replied with the deny result.

### AC-P0-03 — traces `FR-P0-11`, `NFR-06`

- **Given** A session S has one pending ApprovalRequest and one pending QuestionRequest.
- **When** The frame that owns S is torn down (sandbox release / session eviction / daemon shutdown) before any client responds.
- **Then** Both requests transition to cancelled; each held driver JSON-RPC id receives a connection-lost error; EvtApprovalResolved and EvtQuestionResolved fire with resolution_reason=cancelled; state.PendingApprovals[S] and state.PendingQuestions[S] are empty; no goroutine remains associated with the reaped ids.

### AC-P0-04 — traces `FR-P0-08`, `FR-P0-12`

- **Given** Client A is connected with ci-A and client B disconnects while an ApprovalRequest is pending.
- **When** Client B reconnects with a new WS ticket (which mints ci-B new), resubscribes to session S, and receives the resubscribe response; concurrently, client A resolves the ApprovalRequest.
- **Then** Client B's local view converges to host/state's authoritative pending set (either still pending if B's snapshot arrived first, or absent from pending and reflected in a broadcast EvtApprovalResolved if A won the race); both A's view and B's view eventually agree on the same final ApprovalRequest state.

### AC-P1-01 — traces `FR-P1-07`, `FR-P1-08`

- **Given** A recorded scenario fixture under protocol/simulator/fixtures/ has been checked in with a 5-event approval-round-trip sequence.
- **When** Each of the four generated SDKs (C#, Swift, Kotlin, TS) is driven against a fresh simulation-server process replaying that fixture.
- **Then** Every SDK observes the identical typed event/command sequence declared by the fixture, verified by a diff between the SDK's observed sequence log and the fixture's canonical sequence.

### AC-P1-02 — traces `FR-P1-05`, `FR-P1-06`

- **Given** A PR modifies a generated SDK's call site to reference a wire field name that is not declared in protocol/events.schema.json.
- **When** The compatibility CI job (scripts/run-verification-profile.sh pr compatibility) scans the generated SDK against the checked-out protocol/ schemas at that PR's HEAD.
- **Then** The compatibility job exits non-zero, uploads an artifact naming the SDK and the undeclared surface, and merge is blocked by branch protection.

## Requirement to ADR trace

| Requirement | ADRs |
|---|---|
| `FR-P0-01` | `adr-20260724-approval-question-state-domain-in-host-state` |
| `FR-P0-02` | `adr-20260724-approval-question-state-domain-in-host-state`, `adr-20260724-approval-broadcast-coexists-view-update` |
| `FR-P0-03` | `adr-20260724-approval-answerer-identity-per-ws-instance` |
| `FR-P0-04` | `adr-20260724-approval-single-writer-first-commit` |
| `FR-P0-05` | `adr-20260724-approval-single-writer-first-commit` |
| `FR-P0-06` | `adr-20260724-approval-expiry-deny-default-no-extension` |
| `FR-P0-10` | `adr-20260724-approval-answerer-identity-per-ws-instance` |
| `FR-P0-11` | `adr-20260724-approval-lifecycle-teardown-cancel` |
| `FR-P0-12` | `adr-20260724-approval-answerer-identity-per-ws-instance` |
| `FR-P1-01` | `adr-20260724-protocol-cross-language-sdks-supersedes-0021` |
| `FR-P1-02` | `adr-20260724-sdk-codegen-openapi-generator` |
| `FR-P1-03` | `adr-20260724-capability-negotiation-bundled-remote-two-axis` |
| `FR-P1-04` | `adr-20260724-capability-negotiation-bundled-remote-two-axis` |
| `FR-P1-08` | `adr-20260724-simulator-under-protocol-directory` |
| `FR-P1-09` | `adr-20260724-deep-link-shape-adopts-remote-control-plan` |
| `FR-P1-10` | `adr-20260724-protocol-cross-language-sdks-supersedes-0021` |
| `FR-P1-12` | `adr-20260724-approval-broadcast-coexists-view-update` |
