# Resilient server reconnect and fleet health — technical design

**Revision:** `design-20260803-r1`. This plan closes all design choices that affect ownership, protocol, lifecycle, failure meaning, security, accessibility, or verification. The only remaining discretion is private layout or representation that preserves the named contracts.

<!-- anchor: goal -->
## Goal and scope

Convert the fixed recovery and fleet-health UX into one falsifiable implementation contract for the shared SPA, Web host, and Workspace host. The completed change lets an initial or established active connection recover through a finite, generation-fenced lifecycle; proves recovery only with daemon hello; exposes a reusable Retry from the sole StatusBar owner; and presents one current/stale, server-qualified fleet-health meaning from both hosts.

In scope are `clients/ui` active-connection lifecycle and shell presentation, the host-neutral FleetProvider health projection, the existing application-scoped FleetRuntime implementations in `clients/workspace` and `clients/web-host`, the Workspace Connections delivery, a shared browser Connections diagnosis surface, and deterministic tests at the established T0/T1/T3 seams. No server gateway wire change is required.

Out of scope are Windows Shell tray health, user-configurable retry bounds, preservation or transparent re-subscription of an old WebSocket, missed-output replay or sequence-gap reconciliation, scrollback restoration, runtime config reload, persistence of runtime health, and notification surfaces outside the StatusBar. Web configuration remains the immutable process-start `servers.json` contract; this change must not add a credential mutation endpoint merely to implement health diagnosis.

<!-- anchor: approach -->
## Approach

Use two deliberately independent state machines.

1. The view-local active `Connection` owns a **finite** lifecycle for the selected server. Every start/Retry/server switch gets a monotonically increasing lifecycle generation; every initiating or automatic attempt gets a monotonically increasing attempt sequence. Ticket acquisition, delayed retry, socket callbacks, and store publication are accepted only when both identifiers match current authority. WebSocket open remains internal; only daemon hello transitions the observed state to nominal.
2. Each host's existing application-scoped `FleetRuntime` is the sole host-health writer and owns observation of every enabled configured server. It emits one credential-free revisioned `FleetHealthSnapshot`; an exact protocol mismatch is terminal for that member generation and is not retried. Disabled entries are joined from immutable config without a worker. The SPA subscribes once and performs no probe or interval polling. Delivery liveness is a separate consumer-owned envelope: a disconnected adapter may overlay Health unavailable over the last host revision, but may neither mint nor advance a host health revision.

The shared SPA derives two adjacent StatusBar observations rather than collapsing them: active connection progress/closed Retry and enabled-fleet aggregate/Connections diagnosis. A hello may clear the first while another member keeps the second degraded. The Connections action opens the same shared server-qualified diagnosis view in browser and Workspace; Workspace may route into its existing editable settings window, while browser diagnosis leaves the external immutable `servers.json` editing path available and clearly labels restart semantics.

This is a `boundary-complete` plan. Public ownership, lifecycle, outcome, freshness, and delivery contracts are fixed. Private scheduler, cancellation-token, map, and markup organization remain implementation discretion when their owning contract tests continue to pass.

## Requirements translated to EARS candidates

<!-- anchor: fr-rec-001 -->
### FR-REC-001 — bounded automatic lifecycle

When an initiating active connection fails before hello, or a hello-confirmed active connection closes unexpectedly, the shared UI shall schedule at most sixteen automatic retry attempts and shall transition to closed-with-Retry immediately after the sixteenth automatic attempt fails without hello. The initiating attempt is not one of the sixteen automatic retries. This covers UAC-001, UAC-005, and UAC-010 and rejects both the old reload-only path and unbounded client retry.

<!-- anchor: fr-rec-002 -->
### FR-REC-002 — invariant recovery proof

The active connection shall never publish nominal/open/recovered state unless a daemon-derived hello has been received in the current lifecycle generation. WebSocket open, ticket success, and a retained old hello are insufficient. This is the required invariant FR and covers UAC-001/UAC-002.

<!-- anchor: fr-rec-003 -->
### FR-REC-003 — reusable manual recovery

While automatic recovery is exhausted, activating Retry shall synchronously enter progress, create a fresh lifecycle generation with fresh retry accounting, and prevent a competing activation. If that lifecycle exhausts, Retry shall become available again without reload. This covers UAC-003/UAC-004/UAC-011.

<!-- anchor: fr-rec-004 -->
### FR-REC-004 — invariant presentation ownership and modality

While the app shell exists, active recovery status and Retry shall be present only in the persistent StatusBar, including with no selected session; status changes shall be exposed through one polite status region without taking focus, and removal of a focused Retry shall leave focus at a stable shell target from which the changed status is perceivable. Header, sidebar, toast, modal, and Connections shall not duplicate Retry. This covers UAC-004/UAC-006.

<!-- anchor: fr-fleet-001 -->
### FR-FLEET-001 — invariant host-owned fleet health

For each host process, exactly one application-scoped FleetRuntime shall own enabled-member health generation, freshness, retry, and publication. The SPA and settings views shall consume its credential-free snapshot and shall not issue independent server probes. Runtime health shall never be written into `servers.json` or another configuration source. This covers the ownership and runtime-only invariants behind UAC-007 through UAC-014.

<!-- anchor: fr-fleet-002 -->
### FR-FLEET-002 — qualified outcome partition

When fleet evidence changes, the host shall publish each configured server by stable `serverId` as exactly one member of the qualified observation union: current Checking, Connected, Unavailable, or Incompatible protocol; Disabled with no freshness; or Health unavailable with an optional nested prior observation that is explicitly stale. A protocol mismatch shall stop member operational/reconnect work until config or process generation changes. Disabled servers shall start no worker and shall be omitted from aggregate numerator and denominator. This covers UAC-007/UAC-012/UAC-013.

<!-- anchor: fr-fleet-003 -->
### FR-FLEET-003 — aggregate and diagnosis

When enabled-fleet health is degraded, the StatusBar shall distinguish partial unavailable/incompatible counts from all-enabled-unavailable, and expose a pointer/keyboard-operable Connections action. That action shall reach a server-qualified diagnosis view in Web and Workspace containing every configured server, state, freshness, and available reason while leaving the host's configuration editing path usable. Workspace retains its editable settings controls. Browser Connections shall expose an operable `Copy configuration path` action for the exact host-reported `servers.json` path plus an adjacent restart-required instruction, without adding a credential mutation endpoint. This covers UAC-007/UAC-008/UAC-012/UAC-013.

<!-- anchor: fr-fleet-004 -->
### FR-FLEET-004 — invariant independent axes

The active connection observation and enabled-fleet aggregate shall be reduced independently: active hello shall not hide another member's degradation, and all-member recovery shall not alter an independently reconnecting/closed active connection. This covers UAC-009/UAC-014.

<!-- anchor: nfr-life-001 -->
### NFR-LIFE-001 — resource closure

Closing, replacing, or retrying an active `Connection` shall abort or invalidate pending ticket work, cancel delayed retry, close the retired socket, detach callbacks, and prevent any retired generation from mutating shared state. Host FleetRuntime shutdown shall abort probes, close member sockets/channels, and stop publication.

<!-- anchor: nfr-health-001 -->
### NFR-HEALTH-001 — bounded and coalesced observation

Each capabilities request, ticket request, and hello wait used to classify a fleet member shall terminate within three seconds. A Connected observation is current for five seconds; at its freshness deadline the existing worker starts one bounded refresh. Failed refresh publishes Health unavailable with any prior observation nested as stale. Retry uses the existing bounded backoff policy; exact protocol mismatch is terminal until config/process generation changes. Shutdown synchronously invalidates handles and cancels phase timers, fetches, and sockets. Query/subscription consumers shall reuse the latest host snapshot and shall never multiply work per render.

<!-- anchor: nfr-host-001 -->
### NFR-HOST-001 — host parity and credential confinement

The same scripted configuration/member evidence shall normalize to deep-equal credential-free health snapshots and aggregate/diagnosis meaning in Web and Workspace. Only same-origin HTTP/WS versus preload/IPC delivery may differ. Bearer values and unsanitized transport errors shall not appear in health projection, DOM, logs, or test snapshots.

<!-- anchor: nfr-test-001 -->
### NFR-TEST-001 — deterministic verification

Retry accounting, lifecycle races, health outcome/freshness, and host parity shall be mechanically testable with injected clock/scheduler, fetch, WebSocket/channel, and scripted member evidence. Any new or modified fake of gateway behavior shall ship its fake, an invariant-naming contract test, and a `FakeVsReal*` opt-in backstop.

## Components and repository grounding

<!-- anchor: component-active-connection -->
### component-active-connection — active finite recovery owner

**Paths:** `clients/ui/src/socket/connection.ts`, `clients/ui/src/socket/backoff.ts`, `clients/ui/src/store/daemon.ts` and their tests. **Owner:** shared SPA socket layer. **Integration points:** active server transport from `FleetProvider.activeConnection`, daemon store, terminal-subscription controller. **Test seams:** injected fetch/WebSocket/sleep plus fake timers and deliberately late callbacks.

It owns `contract-active-recovery-lifecycle`, `contract-daemon-hello-proof`, and `contract-active-resource-fence`. ADR 0011 supplies the typed close input, ADR 0012 is explicitly outside its finite bound, and ADR 0022 fixes retry mechanics in this layer.

<!-- anchor: component-status-bar -->
### component-status-bar — sole recovery and aggregate presentation owner

**Paths:** `clients/ui/src/components/StatusBar.tsx`, `clients/ui/src/App.tsx`, `clients/ui/src/css/view.css`, StatusBar/App tests and Playwright shell specs. **Owner:** shared shell presentation. **Integration points:** daemon connection slice, health projection store, `Connection.retry`, Connections navigation. **Test seams:** Testing Library role/focus/status assertions and assembled browser keyboard tests.

It owns `contract-statusbar-recovery`, `contract-independent-status-axes`, and the presentation half of `contract-connections-diagnosis`.

<!-- anchor: component-host-fleet-runtime -->
### component-host-fleet-runtime — single fleet health authority

**Paths:** `clients/workspace/src/main/fleet-session-runtime.ts`, `clients/web-host/internal/webhost/fleet.go`, their runtime/conformance tests, and the existing `clients/workspace/src/main/server-health.ts` logic to be folded into this authority. **Owner:** one runtime per client-host process. **Integration points:** immutable host config, capabilities/ticket/WS gateway routes, fleet session publication, health adapters. **Test seams:** scripted member server, injected fetch/socket/clock in Workspace, `httptest` plus `coder/websocket` in Web.

It owns `contract-fleet-health-authority`, `contract-fleet-health-outcomes`, `contract-fleet-resource-lifecycle`, and the producer side of `contract-host-health-wire`. The application-scoped fleet authority, host-neutral provider, server-qualified partial failure, and single-user auth ADRs apply.

<!-- anchor: component-fleet-provider -->
### component-fleet-provider — credential-free health delivery adapter

**Paths:** `clients/ui/src/fleet/provider.ts`, `clients/ui/src/fleet/session-protocol.ts`, `clients/ui/src/fleet/presentation.ts`, `clients/ui/src/auth.ts`, `clients/ui/src/fleet/fixtures/fleet-health-v1.json`, Workspace preload/IPC/electron-entry codecs, and Web host codec/routes. **Owner:** shared provider contract; `session-protocol.ts` owns the versioned wire schema/decoder and host-specific adapters implement it. **Integration points:** host snapshot stream/query, consumer delivery-liveness envelope, and SPA health store. **Test seams:** one checked-in canonical JSON fixture suite decoded/encoded by TypeScript/Workspace and Go, including unknown-version/state rejection, plus adapter disconnect/reconnect injection.

It owns the consumer side of `contract-host-health-wire` and `contract-health-configuration-boundary`.

<!-- anchor: component-connections-surface -->
### component-connections-surface — shared server-qualified diagnosis

**Paths:** `clients/ui/src/fleet/connections-presentation.ts` (the sole pure row/label/reason projection), `clients/ui/src/components/ConnectionsPanel.tsx`, `clients/workspace/src/main/settings-page.ts`, `clients/workspace/src/main/app-entry.ts`, `clients/workspace/src/preload`, `clients/web-host/internal/webhost/host.go`, and browser/Workspace UI tests. **Owner:** the shared pure projection owns diagnosis semantics; each host owns only navigation and rendering delivery. **Integration points:** StatusBar Connections action, health projection, immutable config projection. **Test seams:** one shared fixture table run against the pure projection and both consumers, Workspace settings DOM harness, and Web Playwright.

It owns the delivery half of `contract-connections-diagnosis`. It does not own Retry and does not persist health.

<!-- anchor: component-gateway-evidence -->
### component-gateway-evidence — unchanged external evidence source

**Paths:** `services/server/api`, its mux/WebSocket scenario tests, and the real-stack fake backend. **Owner:** gateway. **Integration points:** capabilities, ticket, hello/view-update, ADR-0011 control and close. **Test seams:** gateway scenario tests and opt-in real-stack `FakeVsReal*` backstop.

No gateway implementation contract is opened. This component supplies the external evidence consumed by active and fleet runtimes and anchors ADR 0011/0012 compatibility.

## Data flow: active recovery

1. `App` selects the active `serverId`, constructs one `Connection`, and calls `start`; construction, Retry, server switch, or teardown increments lifecycle generation and publishes connecting/reconnecting before asynchronous work. Each initiating or automatic attempt increments a separate attempt sequence.
2. Timer, ticket fetch, and socket factory capture both identifiers. Every continuation first compares both with current authority and explicit teardown state.
3. Socket open may initialize internal transport plumbing but does not publish `open` and does not reset retry accounting.
4. A daemon hello in the current generation atomically resets automatic-attempt accounting, clears daemon-disconnected, publishes `open`, and enables reconnect status-signal handling.
5. Ticket/socket failure or unexpected close drains pending requests and retires that attempt. If fewer than sixteen automatic retries have run, it schedules exactly the next numbered attempt. Failure of retry sixteen publishes `closed` and no timer. A late close or hello from attempt `n` after attempt `n+1` begins is a no-op even within one lifecycle generation.
6. Retry synchronously increments generation, cancels retired work, resets accounting, publishes progress, and starts a new initiating attempt. A second activation observes the active generation and is a no-op.
7. Unmount or active-server replacement increments generation, aborts work, and closes without scheduling recovery.

<!-- anchor: contract-active-recovery-lifecycle -->
### contract-active-recovery-lifecycle

**Dimension:** state lifecycle/control flow. **Owner:** component-active-connection. **Requirements:** FR-REC-001/003, NFR-LIFE-001. **Evidence mode:** total.

Decision rules are: an initiating failure enters automatic attempt 1; automatic failure `n < 16` enters `n+1`; failure 16 enters closed; hello whose lifecycle generation **and** attempt sequence are current enters open; Retry from closed creates a fresh lifecycle; explicit teardown terminates without Retry scheduling. The normal witness is established close followed by a later retry hello. Adversarial witnesses are initial ticket rejection, all sixteen failures, a seventeenth callback request, repeated Retry activation, old-generation callbacks after active-server switch, and attempt `n` close/hello after attempt `n+1` starts. Required observables are exact attempt calls, store state, one live socket maximum, and Retry reuse. Verification: `verify-active-retry-exact`, `verify-initial-recovery-stop`, `verify-manual-retry-repeat`, `verify-stale-generation-noop`.

No operational input beyond current object state and injected transport results is needed. Lifecycle generation is immutable for a manual lifecycle; attempt sequence is immutable for one attempt and advances on every automatic attempt. A plausible wrong implementation checks the bound after scheduling and creates retry 17, resets accounting on socket open, or fences only by lifecycle generation; each fails the call-count/state/race witnesses.

<!-- anchor: contract-daemon-hello-proof -->
### contract-daemon-hello-proof

**Dimension:** integration contract. **Owner:** component-active-connection. **Requirements:** FR-REC-002. **Evidence mode:** fallible.

Operational input `input-current-daemon-hello` is a parsed server frame produced by component-gateway-evidence, acquired by the current generation's socket message callback, required until the atomic status transition, preserved by generation-bound capability, invalidated on generation change, and unavailable when the socket closes or emits no hello. Outcomes partition into determinate (valid current hello → open), unknown (socket open/no qualifying frame → remain progress), and inconclusive (malformed/non-hello data → ignore/remain progress); none may publish open. The normal witness is ticket/open/hello. The adversarial witness is ticket/open with no hello, followed either by long-lived silence or close. Verification: `verify-hello-only-open`, `verify-open-without-hello`.

A plausible wrong implementation keeps the current `handleOpen -> setStatus("open")`; UAC-001/UAC-002 catch its false recovered interval.

<!-- anchor: contract-active-resource-fence -->
### contract-active-resource-fence

**Dimension:** resource management/concurrency. **Owner:** component-active-connection. **Requirements:** FR-REC-003, NFR-LIFE-001. **Evidence mode:** total.

At most one current ticket request, scheduled retry, and active socket are retained. Lifecycle change or attempt advance aborts a fetch when supported and always fences its result with both identifiers; cancels the scheduler handle; detaches/closes the retired socket; and drains pending requests once. Late completion is observable only as a no-op. Normal witness: one close, one delayed reconnect. Adversarial witness: close/retry/server-switch while fetch and sleep are unresolved, then resolve every old promise and fire old socket events, including hello from attempt `n` after `n+1` starts. Verification: `verify-active-resource-closure`, `verify-single-flight-retry`.

No external operational input is required. An AbortController is defense-in-depth; correctness rests on the generation fence even when a test fetch ignores abort.

<!-- anchor: contract-statusbar-recovery -->
### contract-statusbar-recovery

**Dimension:** user observability. **Owner:** component-status-bar. **Requirements:** FR-REC-003/004. **Evidence mode:** total.

Connecting/reconnecting, closed, and hello-confirmed nominal text live in a dedicated polite `role=status` text region in the persistent footer. Retry is a sibling native button outside that live region. The StatusBar root is the single persistent focus landmark, with an accessible label and `tabIndex=-1`; only when focused Retry disappears does its handler move focus to that landmark after synchronously starting the lifecycle. No other transition steals focus. The normal witness uses pointer then hello. Adversarial witnesses use no selected session, keyboard activation, screen-reader role/name queries, focus on disappearing Retry, duplicate-announcement checks, and inspection for duplicate Retry in header/sidebar/toast/modal/Connections. Verification: `verify-statusbar-retry-access`, `verify-status-announcement-focus`, `verify-single-recovery-owner`.

No external input is needed beyond the two owned stores. Exact prose/CSS class names remain private discretion; live-region/action separation, focus destination, accessible name `Retry`, and status meanings do not.

<!-- anchor: contract-fleet-health-authority -->
### contract-fleet-health-authority

**Dimension:** integration contract/state lifecycle. **Owner:** component-host-fleet-runtime. **Requirements:** FR-FLEET-001, NFR-HOST-001, NFR-TEST-001. **Evidence mode:** multi-source.

Operational inputs are `input-configured-fleet`, the immutable version-2 host config snapshot acquired before listener/window startup and valid until host exit, and `input-member-evidence`, the per-enabled-server capability/hello/typed-error stream acquired by one generation-fenced worker and valid until superseded/shutdown. The runtime is the only writer of member generation and host health revision. A view query or subscription only reads the latest projection. Existing renderer polling in StatusBar and independent Workspace `FleetHealthMonitor` probing must be removed or converted into projection-only adapters. The accepted exact-match protocol ADR applies before ticket/socket operational work: HTTP 426, missing/malformed, or unequal version publishes current Incompatible protocol, cancels member operational handles, and schedules no retry until immutable configuration or process generation changes.

Outcome partition: determinate when config plus current member evidence produce a state; unknown while enabled evidence is not yet available (Checking); inconclusive when a bounded observation/refresh cannot establish current evidence (Health unavailable); conflicting generations select the newest host generation and reject older publication. Adapter delivery loss is not host member evidence and cannot update this partition or host revision; it is the consumer overlay defined by `contract-host-health-wire`. Because global aggregate is built from partial member streams, the snapshot includes every configured identity at one revision. Normal witness: two healthy enabled plus one disabled. Adversarial witness: A healthy, B incompatible and terminal, C disabled, old B generation arrives after a newer observation, and one subscriber disconnects without changing the host snapshot. Verification: `verify-single-host-health-writer`, `verify-cross-host-health-conformance`, `verify-generation-fence`.

A wrong design keeps `StatusBar` polling `listConfigs()` while Settings probes separately; scripted failure then yields contradictory current states and fails deep-equal revision tests.

<!-- anchor: contract-fleet-health-outcomes -->
### contract-fleet-health-outcomes

**Dimension:** failure recovery/data model. **Owner:** component-host-fleet-runtime. **Requirements:** FR-FLEET-002/004. **Evidence mode:** fallible.

Each entry is keyed by stable `serverId` and is an exclusive qualified observation union: `current:{kind:checking|connected|unavailable|incompatible_protocol, observedAt, reason?}`, `disabled:{kind:disabled}`, or `unavailable:{kind:health_unavailable, observedAt, prior?:{kind:connected|unavailable|incompatible_protocol, observedAt}}`. The optional prior is stale by construction and cannot be Checking or Disabled. Independent `state` and `stale` fields are forbidden, so Disabled+stale, Checking+stale, and simultaneous top-level Connected/Health unavailable are unrepresentable and rejected by codecs. Exact version mismatch is Incompatible protocol and terminal for the member generation; current connection/ticket/HTTP/network failure is Unavailable; no completed evidence is Checking; config-disabled is Disabled; bounded refresh failure is Health unavailable; hello is Connected. A later current observation replaces unavailable/prior evidence and increments host revision.

Operational evidence is produced by component-gateway-evidence and immutable config. Every Connected observation expires after five seconds. At the deadline the same worker begins bounded refresh; while refresh is in flight it may retain current evidence only until the deadline, then a timeout/close/failure publishes Health unavailable with prior Connected nested as stale. Unknown and inconclusive never default to Connected. Normal witness: partial unavailable/incompatible and later all healthy. Adversarial witnesses: one hello followed by silence past expiry, failed refresh, later recovery, total outage with disabled entries, missing reason, and exact protocol mismatch with no subsequent retry. Verification: `verify-health-outcome-partition`, `verify-stale-never-current`, `verify-total-outage`, `verify-fleet-recovery`.

<!-- anchor: contract-fleet-resource-lifecycle -->
### contract-fleet-resource-lifecycle

**Dimension:** resource management/performance budget. **Owner:** component-host-fleet-runtime. **Requirements:** NFR-LIFE-001/NFR-HEALTH-001. **Evidence mode:** total.

One worker exists per enabled server, none per disabled server. Capabilities, ticket acquisition, and hello wait each have independent three-second deadlines. Each generation owns one phase timer and at most one ticket/socket sequence. Connected evidence expires after five seconds and refresh reuses that worker. Retriable Unavailable/Health unavailable uses the existing backoff policy and its named maximum; Incompatible protocol schedules none. Shutdown first invalidates all generations synchronously, then cancels phase/retry timers and requests and closes sockets/channels; no callback can publish afterward, and observable handle counts immediately become zero. Concurrent consumers receive one cached revision; render frequency does not multiply work. Normal witness is two consumers and one member attempt. Adversarial witnesses are 100 renders, concurrent settings/StatusBar reads, capability/ticket/hello timeout, silent open socket, exact mismatch, disable-at-start, and shutdown during each phase. Verification: `verify-health-singleflight`, `verify-health-three-second-bound`, `verify-health-resource-closure`.

No run-time scale target beyond configured member count is invented. Cost convergence evidence is worker/socket/timer counts bounded by enabled server count plus active attempts, independent of renderer count.

<!-- anchor: contract-host-health-wire -->
### contract-host-health-wire

**Dimension:** migration compatibility/integration contract. **Owner:** component-fleet-provider. **Requirements:** FR-FLEET-001/002, NFR-HOST-001. **Evidence mode:** fallible.

Add a credential-free `FleetHealthSnapshotV1` projection and subscription/query capability to the host-neutral FleetProvider. `clients/ui/src/fleet/session-protocol.ts` owns its closed union, version decoder, and canonical fixtures; Workspace preload/IPC and the Web Go codec implement that contract and reject unknown version/state or invalid union combinations. Web transports it over same-origin host routes/WS; Workspace transports it through typed preload/IPC. Existing FleetSnapshot session membership remains compatible; active terminal channels remain separate. During flag-day migration, both host adapters and SPA consumer change together; no fallback to renderer probing is permitted. Rollback restores old adapters and StatusBar as one commit, not a mixed-version fallback.

Operational inputs are host runtime revisioned state and adapter delivery liveness. Runtime state has authority `(hostInstanceId, revision)`; the provider accepts only monotonic revisions for the current instance. Delivery liveness is a consumer-owned envelope, not a second writer. Startup before a host snapshot yields an effective Checking view. Delivery close derives an effective Health unavailable row for every configured enabled identity, nests the last accepted host observation as stale evidence, and preserves its `(hostInstanceId, revision)` unchanged. Reconnection removes the overlay only when the next valid accepted host snapshot arrives. Verification runs old session-fleet conformance unchanged plus shared canonical fixtures, unknown version/state, invalid union, and closed/reconnected adapter cases: `verify-health-wire-parity`, `verify-session-wire-compat`, `verify-health-delivery-loss`. The contract-evolution witness is a two-host fixture before/after migration showing unchanged session identity and new equal health projection.

<!-- anchor: contract-independent-status-axes -->
### contract-independent-status-axes

**Dimension:** user observability/control flow. **Owner:** component-status-bar. **Requirements:** FR-FLEET-003/004. **Evidence mode:** multi-source.

The StatusBar reducer consumes active connection state and fleet health as independent inputs. Active recovery has display precedence for its own compact text/action, but fleet degradation and Connections remain concurrently reachable; neither reducer writes the other's store. Aggregate uses enabled entries only: partial wording reports unavailable/incompatible counts over enabled total, all-unavailable uses explicit total-outage wording, and all-current-healthy clears degradation. Unknown/health-unavailable is never nominal.

Normal witness: active hello while another server unavailable leaves Connections visible. Adversarial witnesses: fleet recovery while active closed, disabled-only difference, all-enabled unavailable, and stale Connected after health delivery loss. Verification: `verify-independent-status-axes`, `verify-enabled-denominator`, `verify-fleet-summary-exits`.

<!-- anchor: contract-connections-diagnosis -->
### contract-connections-diagnosis

**Dimension:** integration contract/user observability. **Owner:** component-connections-surface. **Requirements:** FR-FLEET-003. **Evidence mode:** fallible.

StatusBar exposes one native `Connections` action whenever diagnosis is needed. `clients/ui/src/fleet/connections-presentation.ts` is the sole pure semantic owner that maps config plus qualified health into stable rows, labels, and closed sanitized reasons; React and Workspace settings consume the same typed output and may vary only navigation/rendering. Workspace delivery opens its existing Connections settings section and binds health to the host runtime projection; controls remain editable during health failure. Browser delivery opens the shared SPA Connections panel backed by the same projection; a keyboard-operable `Copy configuration path` action copies the exact credential-free path reported by Web host, with adjacent restart-required instruction. The action remains enabled during health failure and introduces no config mutation or credential endpoint. Connections does not own Retry.

Operational inputs are current host config projection, health snapshot/effective delivery overlay, and credential-free config-path metadata. Absence of current health yields Health unavailable, never omission. Normal witness opens both hosts by pointer and keyboard and compares normalized rows. Adversarial witnesses cover affected server with reason, duplicate display names (serverId disambiguates), delivery failure, disabled server, no session selected, and failed health with successful browser path copy/restart instruction. Verification: `verify-connections-parity`, `verify-browser-connections-reachability`, `verify-workspace-connections-editable`, `verify-connections-no-retry`.

<!-- anchor: contract-health-configuration-boundary -->
### contract-health-configuration-boundary

**Dimension:** security boundary. **Owner:** component-fleet-provider. **Requirements:** FR-FLEET-001/002, NFR-HOST-001. **Evidence mode:** multi-source.

Host runtime reads credential-bearing immutable config; health projection contains only serverId, display label, enabled flag, qualified observation, host instance/revision, and a closed `HealthReason` enum. Each host transport adapter owns a producer-side typed classifier that converts raw HTTP/WS/fetch errors to that enum **before** snapshot construction and before any health log call. Raw errors, URLs, tickets, authorization headers, and userinfo remain local to the transport adapter and are never interpolated into projection or logs. Neither Workspace config save payload nor Web config source is augmented with health fields. Config generation and health revision are distinct. Verification injects bearer, URL-userinfo, and query-token sentinels into both hosts and inspects projection, serialized wire data, DOM, and captured logs; it also reloads config to prove health absence: `verify-health-secret-redaction`, `verify-health-not-persisted`.

## Decision inputs and ADR posture

The planner classified the following inputs. All UX selections are design choices because changing them changes observable behavior; private implementation options are delegated below.

- `decision-input-recovery-policy` — UX selected one initiating attempt plus sixteen automatic retries, then manual Retry; baseline alternatives unbounded and one-attempt are rejected. Contracts: active recovery lifecycle.
- `decision-input-recovery-proof` — UX selected daemon hello; baseline socket-open proof is rejected. Contract: daemon hello proof.
- `decision-input-status-owner` — UX selected StatusBar-only; baseline duplicate banner/sidebar ownership is rejected. Contract: StatusBar recovery.
- `decision-input-fleet-denominator` — UX selected enabled entries only; baseline all configured entries is rejected. Contracts: health outcomes/independent axes.
- `decision-input-health-boundary` — UX selected runtime-only evidence; baseline persistence in `servers.json` is rejected. Contract: health/config boundary.
- `decision-input-diagnosis-entry` — UX selected Connections in both hosts; baseline Workspace-only settings is rejected. Contract: Connections diagnosis.
- `decision-input-retry-reachability`, `decision-input-status-announcement`, `decision-input-runtime-health-boundary`, and `decision-input-shared-connections` preserve the four explicit design handoffs and are closed by the contracts above.
<!-- anchor: adr-20260624-0011-two-step-ws-close-on-daemon-disconnect -->
- `decision-input-adr-0011` preserves two-step close and creation of a new client socket.

<!-- anchor: adr-20260624-0012-daemon-client-eager-dial-supervisor -->
- `decision-input-adr-0012` preserves unbounded gateway daemon supervision outside the finite client lifecycle.

<!-- anchor: adr-20260624-0022-subscribe-retry-in-socket-layer -->
- `decision-input-adr-0022` preserves only its actual frame-not-ready subscribe retry and reusable socket-layer backoff mechanics. Exact active lifecycle ownership, sixteen-attempt accounting, hello proof, and generation/attempt fencing are fixed by this change's requirements/contracts, not inherited from ADR 0022.

<!-- anchor: adr-20260727-host-neutral-fleet-provider -->
- `decision-input-adr-host-neutral` preserves host-neutral, credential-free, server-qualified fleet meaning.

<!-- anchor: adr-20260727-application-scoped-fleet-session-authority -->
- `decision-input-adr-app-fleet` preserves one application-scoped FleetRuntime authority per host.

<!-- anchor: adr-20260727-server-qualified-identity-partial-failure -->
- `decision-input-adr-partial-failure` preserves compound server identity, latest-generation wins, and scoped partial failure.

<!-- anchor: adr-20260727-single-user-server-auth -->
- `decision-input-adr-auth` preserves credential confinement and excludes bearer material from health projection.
<!-- anchor: adr-20260727-exact-match-client-server-protocol -->
- `decision-input-adr-exact-protocol` preserves exact version admission and requires Incompatible protocol to stop member operational/reconnect work until a new config/process generation.
- `decision-input-testing-policy` requires feature/bug tests and the external-dependency fake/FakeVsReal/invariant triple.

<!-- anchor: adr-20260803-resilient-fleet-health-single-authority -->
The accepted ADR `adr-20260803-resilient-fleet-health-single-authority` records a new, complementary persistent decision: health is a versioned, exclusive qualified projection of the **same** FleetRuntime authority established for fleet membership, while adapter delivery failure is a consumer overlay that cannot advance host revisions. It does not establish a second owner and does not supersede the accepted application-scoped FleetRuntime ADR; it references that ADR and the host-neutral provider, partial-failure, auth, and exact-protocol decisions. No ADR is needed for exact StatusBar markup or private generation-token representation.

## Typed implementation discretion

- `discretion-active-generation-token`: within `connection.ts`, implementation may use a numeric epoch plus AbortController or a private lifecycle object. Escalate if the choice changes attempt counting, hello proof, store API, or permits more than one live resource.
- `discretion-cancelable-scheduler`: within the socket module, implementation may extend injected `sleep` to a cancelable scheduler or pair sleep with generation checks. Escalate if deterministic no-timer-leak verification cannot observe cancellation.
- `discretion-status-focus-implementation`: within StatusBar/App shell, implementation may choose the ref/effect plumbing used to focus the already-fixed persistent StatusBar landmark. Escalate if the destination changes, a visible surface is added, a non-Retry transition steals focus, or the landmark is not perceivable by keyboard/assistive technology.
- `discretion-health-storage`: within each host runtime, implementation may store health fields on the existing member record or in a private parallel map before projection. Escalate if session membership and health acquire different generation owners or host snapshots cease to normalize equally.
- `discretion-connections-layout`: the shared diagnosis may be a panel, drawer, or route. Escalate if action name, server-qualified contents, modality reachability, config editability, or no-duplicate-Retry contract changes.

## Dependency-ordered implementation units

### Unit 1 — generation-fenced active recovery

Refactor `Connection` and daemon status semantics first. Add current-generation hello-only open, exact lifecycle accounting, cancellation/fencing, and T0 regression/race tests. Preserve ADR 0011 control/close behavior and terminal-subscription reopening. Acceptance: UAC-001/002/003/005/010/011 at socket/store seam.

### Unit 2 — single-authority fleet health projection

Define the exclusive qualified health observation and extend Workspace/Web FleetRuntime owners. Fold Workspace probe classification and Web capability classification into runtime generation; remove independent probe authority. Apply exact-match admission before operational work, add five-second freshness and three-second phase deadlines, terminal incompatibility, typed producer-side redaction, resource closure, disabled join, and host conformance tests. Acceptance: UAC-007/012/013/014 data semantics plus NFR resource/security gates.

### Unit 3 — host-neutral delivery and SPA store

Make `session-protocol.ts` the schema/decoder owner, add canonical cross-codec fixtures, implement Web same-origin and Workspace preload/IPC health delivery, and install one App-level subscription/store. Delivery loss derives a consumer overlay without minting host revisions. Delete StatusBar interval probing and silent failure. Run session-fleet compatibility plus new health parity/invalid-codec tests. Depends on Unit 2.

### Unit 4 — StatusBar and Connections observations

Implement the independent reducer, sibling status-region/Retry semantics, fixed StatusBar focus landmark, no-session reachability, and one shared pure Connections projection consumed by both hosts. Keep Workspace controls usable and make browser editability concrete through Copy configuration path plus restart instruction. Depends on Units 1 and 3. Acceptance: UAC-004/006/007/008/009/012/013/014.

### Unit 5 — cross-boundary verification and documentation promotion

Run browser smoke and real-stack shared UI/UX traces, Workspace host wiring tests, gateway compatibility, fake/FakeVsReal/invariant triple, lint/typecheck, and full affected package tests. Materialize the accepted ADR/persistent design promotion if integrator confirms the candidate. Depends on Units 1–4.

## Acceptance and legacy-delta trace

- `acceptance-UAC-001`: established close remains reconnecting until a later hello, then recovers without reload; rejects socket-open success and old reload-only behavior.
- `acceptance-UAC-002`: open/no-hello remains non-nominal whether it stays open or closes; rejects current `handleOpen` publication.
- `acceptance-UAC-003`: Retry starts one fresh lifecycle and hello completes it; rejects page reload or exhausted accounting reuse.
- `acceptance-UAC-004`: no-session pointer/keyboard/AT users reach the same Retry and stable focus/status; rejects session-only/visual-only presentation.
- `acceptance-UAC-005`: exactly sixteen automatic calls occur after initiating failure, never fewer or seventeen.
- `acceptance-UAC-006`: repository/UI inspection finds recovery/Retry in StatusBar only and no second owner.
- `acceptance-UAC-007`: partial summary distinguishes unavailable/incompatible over enabled total and disabled does not affect it.
- `acceptance-UAC-008`: Connections in Web and Workspace yields normalized server-qualified current state/reason and leaves configuration editing path usable; rejects Workspace-only diagnosis.
- `acceptance-UAC-009`: active hello clears only active recovery while another member keeps fleet degradation/Connections.
- `acceptance-UAC-010`: startup before hello progresses through exact exhaustion to Retry with no session.
- `acceptance-UAC-011`: a failed manual lifecycle returns to reusable Retry rather than indefinite progress/permanent disablement.
- `acceptance-UAC-012`: all enabled unavailable uses explicit total-outage wording and still identifies every configured server.
- `acceptance-UAC-013`: failed health delivery shows Health unavailable and any retained Connected is stale; configuration controls/path remain usable.
- `acceptance-UAC-014`: a current all-healthy revision clears aggregate degradation without altering active connection state.

## Verification candidates

T0 covers pure attempt accounting/backoff, lifecycle reducer/generation fences, fleet outcome/aggregate reduction, sanitization, and normalized cross-host fixtures. T1 covers Connection with fake fetch/WebSocket and hostile async ordering, StatusBar roles/focus/keyboard/no-session, Workspace IPC/runtime/settings, Web host `httptest` runtime/routes, and gateway scenario compatibility. Browser Playwright proves shared Connections delivery and assembled-shell focus/status behavior. T3 opt-in `FakeVsRealFleetHealth*` traces compare the deterministic fake with a real `server` for capabilities, ticket, hello, protocol mismatch, daemon disconnect, and recovery without weakening assertions.

Every mandatory verification has an executable owning-package command:

- `cd clients/ui && npm run test:unit -- src/socket/connection.recovery-regression.test.ts src/socket/connection.test.ts src/components/StatusBar.test.tsx src/fleet`
- `cd clients/workspace && npm run test:unit -- tests/fleet-session-runtime.test.ts tests/server-health.test.ts tests/settings-page.test.ts tests/fleet-health-wire.test.ts`
- `cd clients/web-host && go test ./internal/webhost -run 'TestFleetHealth|TestFleetHealthWire|TestFleetHealthRedaction'`
- `cd clients/ui && npm run test:unit -- src/fleet/session-protocol.test.ts src/fleet/connections-presentation.test.ts src/components/ConnectionsPanel.test.tsx`
- `cd services && GOCACHE=/tmp/gocache-agent-grid go test ./server/api/...`
- `cd clients/ui && npm run test:e2e`
- `cd clients/ui && npm run test:e2e:real` for the canonical shared UI/UX real-stack gate
- `GOCACHE=/tmp/gocache-agent-grid make test-e2e` runs the opt-in `FakeVsRealFleetHealth*` backstop; local skip is expected only when the required real binary environment is unset.
- affected TypeScript typecheck/lint and repository Go lint/test gates from `AGENTS.md`

## Resolved critique issues

All fourteen `verdict:Y` findings are closed; none is deferred as an open design choice.

- `issue-exact-match-protocol-adr-omitted`: adopted the patch intent. Exact-match ADR is traced; mismatch is terminal Incompatible protocol and schedules no retry until config/process generation changes.
- `issue-fleet-authority-double-owner`: refined rather than deleting the new ADR. The accepted FleetRuntime remains the only writer; the accepted complementary ADR adds projection and delivery-overlay semantics and explicitly cannot create a second owner.
- `issue-delivery-failure-owner-gap`: adopted. Provider owns only a liveness overlay and preserves the last `(hostInstanceId, revision)`; it cannot mint host health.
- `issue-health-refresh-lifecycle-undefined`: adopted. Connected expires after five seconds, refresh is bounded, failure yields Health unavailable plus nested stale prior, and later evidence recovers.
- `issue-active-attempt-generation-race`: adopted. Lifecycle generation and per-attempt sequence are separate and both gate every continuation.
- `issue-health-wire-schema-owner-missing`: adopted. `session-protocol.ts` owns schema/decoder and one canonical fixture suite constrains Workspace and Go codecs.
- `issue-health-state-invalid-combinations`: adopted. An exclusive qualified observation union makes invalid state/freshness combinations unrepresentable.
- `issue-status-live-region-interaction-conflict`: adopted. Status text and Retry are siblings; the persistent StatusBar landmark is the fixed focus destination.
- `issue-test-closure-nonexecuting`: adopted. Targeted T0/T1 commands and the named `FakeVsRealFleetHealth*` T3 gate are executable and local-skip semantics are explicit.
- `issue-connections-semantics-split-owner`: adopted. One pure shared row projection owns labels/reasons; hosts own navigation/rendering only.
- `issue-adr-0022-overclaimed`: adopted. ADR 0022 trace is narrowed to frame-not-ready subscribe retry/backoff mechanics.
- `issue-security-redaction-boundary-unverifiable`: adopted. Producer-side closed classifiers precede snapshot/log boundaries and sentinel tests cover wire, DOM, and logs.
- `issue-resource-progress-bounds-incomplete`: adopted. Capabilities, ticket, and hello each terminate within three seconds; refresh, retry, and shutdown resource convergence are specified.
- `issue-browser-editability-outcome-weakened`: adopted. Browser Connections exposes an operable configuration-path copy action and restart instruction even during health failure.

## Assumptions, risks, and closed decisions

Repository evidence is sufficient for implementation; no spike-first decision remains. Exact browser layout and private token/ref plumbing are delegated only within their contracts. Governance is closed: the accepted complementary ADR fixes FleetRuntime-owned qualified health projection and delivery overlay, and the change upserts stable client design invariants/failure responsibility. The plan contains one fixed design regardless of markup or token representation.

The largest implementation risk is preserving current active terminal/subscription behavior while moving `open` publication to hello; tests must distinguish internal transport open from user-observable recovery. The second is mixed-version host/provider migration; it is a flag-day source migration with no renderer-probe fallback. The third is browser configuration wording: diagnosis must not imply live reload or credential exposure, but the external config path must remain usable rather than being disabled by health failure.
