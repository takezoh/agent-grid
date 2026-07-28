---
change: change-20260727-multi-server-session-palette
role: requirements
functional_requirements:
- id: FR-001
  type: event_driven
  statement: New Session entry は接続対象 server 全体の project を server-qualified row として返し、検索・戻る・選択後も stable server identity
    を保持する。
  priority: must
  rationale: Approved UX and explicit compatibility/security requirement.
- id: FR-002
  type: event_driven
  statement: 選択した server + project に適用される typed command profile だけを返し、旧 opaque command へ fallback しない。
  priority: must
  rationale: Approved UX and explicit compatibility/security requirement.
- id: FR-003
  type: event_driven
  statement: command profile は stable ID、明示順序、label、kind、required/default/validation を持つ任意個 parameter を宣言し、client
    は一つの form に順序どおり描画する。
  priority: must
  rationale: Approved UX and explicit compatibility/security requirement.
- id: FR-004
  type: event_driven
  statement: required missing または invalid value の create は session を作成せず、parameter ID ごとの typed error を返す。
  priority: must
  rationale: Approved UX and explicit compatibility/security requirement.
- id: FR-005
  type: event_driven
  statement: create 時の schema revision 不一致は stale conflict とし、古い値を実行せず、再取得後は同一 ID・互換 kind の値だけを保持できる情報を返す。
  priority: must
  rationale: Approved UX and explicit compatibility/security requirement.
- id: FR-006
  type: event_driven
  statement: 明示 Confirm 後だけ、server/profile/revision/value snapshot を target server へ送信し、server が default を適用して argv
    を生成し、一つの server-qualified session を返す。
  priority: must
  rationale: Approved UX and explicit compatibility/security requirement.
- id: FR-007
  type: event_driven
  statement: zero-parameter profile は空 form を完了状態として作成できる。
  priority: must
  rationale: Approved UX and explicit compatibility/security requirement.
- id: FR-008
  type: ubiquitous
  statement: project/session row と active context は所属 server label を常時示し、display-name collision 時は stable server
    ID を補助表示する。
  priority: must
  rationale: Approved UX and explicit compatibility/security requirement.
- id: FR-009
  type: event_driven
  statement: fleet/project/profile の部分取得失敗は失敗 scope・理由・retry を示し、健康な scope と依然有効な selection/value/focus を保持する。
  priority: must
  rationale: Approved UX and explicit compatibility/security requirement.
- id: FR-010
  type: ubiquitous
  statement: server は version=2 profile request だけを受理し、旧 session.commands、default_command、POST {project,command}
    を fallback として扱わない。
  priority: must
  rationale: Approved UX and explicit compatibility/security requirement.
- id: FR-011
  type: ubiquitous
  statement: Web host と Workspace は同じ configured fleet cardinality と server-qualified session lifecycle を公開し、host
    kind による one-server adapter や implicit default server を持たない。
  priority: must
  rationale: Approved UX and explicit compatibility/security requirement.
- id: FR-012
  type: ubiquitous
  statement: Web host と Workspace main は credential/upstream address を renderer へ列挙せず、stable serverId で catalog/create/session
    lifecycle/operation を route する。
  priority: must
  rationale: Approved UX and explicit compatibility/security requirement.
- id: FR-INVARIANT-001
  type: ubiquitous
  statement: The system shall always preserve the same stable server identity from the displayed project/session
    through selection, confirmation, routing, create result, and active context.
  priority: must
  rationale: Entry and persistent-state observation require a server-qualified invariant.
- id: FR-INVARIANT-002
  type: unwanted
  statement: If a profile revision, parameter value, server identity, or routing authority is stale, malformed,
    unknown, or unavailable, then the system shall create no session and shall expose a typed scoped outcome.
  priority: must
  rationale: Prevents unsafe fallback across all create boundaries.
- id: FR-FLEET-001
  type: event_driven
  statement: When a Web or Workspace view subscribes, the host shall immediately return one full credential-free
    FleetSnapshot containing every enabled server member, its typed state, and every currently known compound SessionRef.
  priority: must
  rationale: Fleet session runtime requirement grounded in the original application-scoped authority outcome.
- id: FR-FLEET-002
  type: state_driven
  statement: While multiple views are open in one Workspace application, every view shall observe the same monotonically
    ordered fleet revisions from the one application-scoped authority; view-local selection shall not change fleet
    membership.
  priority: must
  rationale: Fleet session runtime requirement grounded in the original application-scoped authority outcome.
- id: FR-FLEET-003
  type: event_driven
  statement: When New Session creates on any enabled server, the create outcome shall preserve serverId and shall
    not complete as confirmed until the same compound SessionRef is present in an authoritative fleet snapshot visible
    to the requesting view.
  priority: must
  rationale: Fleet session runtime requirement grounded in the original application-scoped authority outcome.
- id: FR-FLEET-004
  type: event_driven
  statement: When a user selects any listed session, the current window shall switch its active SessionRef and session
    channel in place; the system shall not create a process, BrowserWindow, renderer partition, or fleet store because
    serverId changed.
  priority: must
  rationale: Fleet session runtime requirement grounded in the original application-scoped authority outcome.
- id: FR-FLEET-005
  type: ubiquitous
  statement: All session-scoped operations shall accept a compound {serverId, sessionId} reference and shall route
    inside FleetProvider; raw local IDs, implicit currentServerId, and renderer-constructed server URLs are forbidden.
  priority: must
  rationale: Fleet session runtime requirement grounded in the original application-scoped authority outcome.
- id: FR-FLEET-006
  type: unwanted
  statement: If one server is unavailable, reconnecting, incompatible, or stale, healthy server members shall remain
    current and usable; the failed member shall retain only an explicitly stale last-known snapshot and typed retry
    state, never silently disappear or erase another member.
  priority: must
  rationale: Fleet session runtime requirement grounded in the original application-scoped authority outcome.
- id: FR-FLEET-007
  type: ubiquitous
  statement: Workspace main and Web host shall retain credentials and upstream addresses, reject unsafe redirects,
    and expose no bearer token, upstream base URL, or authorization header through the FleetProvider projection.
  priority: must
  rationale: Fleet session runtime requirement grounded in the original application-scoped authority outcome.
- id: FR-FLEET-008
  type: state_driven
  statement: A current-generation hello or session-bearing view update shall atomically replace only its server
    member; superseded generation frames shall have no observable effect.
  priority: must
  rationale: Fleet session runtime requirement grounded in the original application-scoped authority outcome.
- id: FR-FLEET-009
  type: unwanted
  statement: A successful create whose identity is not observed before the convergence bound shall return created_unconfirmed
    with the compound reference and observation retry, and shall never automatically repeat POST.
  priority: must
  rationale: Fleet session runtime requirement grounded in the original application-scoped authority outcome.
- id: FR-FLEET-010
  type: ubiquitous
  statement: Web and Workspace adapters shall produce deep-equal FleetSnapshot and typed failure outcomes for the
    same configured fleet and upstream fixture; only HTTP/WS versus preload/IPC delivery may differ.
  priority: must
  rationale: Fleet session runtime requirement grounded in the original application-scoped authority outcome.
- id: FR-FLEET-011
  type: ubiquitous
  statement: Static project/profile discovery shall continue to use one GET /api/session-config snapshot per server
    and shall join the same stable server identity used by fleet session lifecycle.
  priority: must
  rationale: Fleet session runtime requirement grounded in the original application-scoped authority outcome.
- id: FR-FLEET-012
  type: event_driven
  statement: External Workspace open-session requests may provide an initial compound SessionRef, but the resulting
    generic fleet view shall subscribe to the application fleet and thereafter switch sessions in place.
  priority: must
  rationale: Fleet session runtime requirement grounded in the original application-scoped authority outcome.
- id: FR-FLEET-013
  type: event_driven
  priority: must
  statement: When session rows or active context are rendered, the owning server display label shall always be visible;
    when labels collide, stable serverId shall also be visible and the two compound identities shall remain separately
    selectable.
  rationale: Preserves UX UAC-009/UAC-010 and rejects local-ID/display-label ambiguity.
non_functional_requirements:
- id: NFR-001
  type: security
  criteria: bearer token は URL に含めず、target server 以外へ転送せず、client は shell 文字列を補間しない。
  measurement: Referenced implementation-contract witnesses and commands pass.
- id: NFR-002
  type: reliability
  criteria: parameter、project、command profile、fleet row の順序は map iteration に依存しない。
  measurement: Referenced implementation-contract witnesses and commands pass.
- id: NFR-003
  type: reliability
  criteria: descriptor validation、create validation、default application、argv materialization は同じ server-owned profile
    revision を参照する。
  measurement: Referenced implementation-contract witnesses and commands pass.
- id: NFR-004
  type: reliability
  criteria: 一 server の timeout、auth failure、malformed response は他 server の catalog/create/session projection を停止しない。
  measurement: Referenced implementation-contract witnesses and commands pass.
- id: NFR-005
  type: usability
  criteria: server label、parameter label/state/error、retry、confirmation は keyboard、pointer、支援技術で同じ意味に到達できる。
  measurement: Referenced implementation-contract witnesses and commands pass.
- id: NFR-006
  type: maintainability
  criteria: fleet transport、config loading、clock/revision、session create seam を注入可能にし、外部 gateway ごとに fake、FakeVsReal、invariant
    contract test を持つ。
  measurement: Referenced implementation-contract witnesses and commands pass.
- id: NFR-007
  type: compatibility
  criteria: wire/config evolution は command_profiles と version=2 request の単一路だけを検証し、旧経路が拒否される。
  measurement: Referenced implementation-contract witnesses and commands pass.
- id: NFR-FLEET-001
  type: scalability
  criteria: Each host process owns at most one upstream lifecycle stream per enabled server; view subscriptions
    do not multiply upstream streams.
  measurement: Count upstream streams for S servers and W views; result is <= S.
- id: NFR-FLEET-002
  type: reliability
  criteria: REST fan-out remains at most four; each request times out at ten seconds; cancellation/removal/close
    settles within one second after timeout.
  measurement: Fake-clock lifecycle test.
- id: NFR-FLEET-003
  type: scalability
  criteria: One immutable O(N) snapshot value is shared by all subscribers; each mailbox holds at most one in-flight
    and one latest pending reference, coalesces intermediate revisions, retains O(N+W) publisher memory, permits
    at most O(NW) concurrent serialization, and severs an unwriteable transport as slow_subscriber for replay on
    resubscribe.
  measurement: Measure N sessions, W subscribers and D delivered revisions; work is O(NWD), backlog per subscriber
    <= 2 references.
- id: NFR-FLEET-004
  type: reliability
  criteria: Fleet revision and member generation are monotonic and stale or duplicate envelopes are idempotently
    ignored.
  measurement: Reducer property test.
- id: NFR-FLEET-005
  type: maintainability
  criteria: The SPA stores one SessionInfo per compound key and one view-local active SessionRef; no server-specific
    duplicate global store exists.
  measurement: Type/lint and reducer test.
- id: NFR-FLEET-006
  type: reliability
  criteria: Shutdown, process-restart server removal, supersede, unsubscribe and active-channel switch release timers,
    pending requests and bindings deterministically.
  measurement: Fake-clock leak test.
- id: NFR-FLEET-007
  type: compatibility
  criteria: The same conformance suite runs against Web and Workspace; each external lifecycle dependency ships
    fake, FakeVsReal and invariant contract tests.
  measurement: Run both adapter suites against one fixture.
- id: NFR-FLEET-008
  type: reliability
  criteria: createObservationDeadline is exactly ten seconds from accepting a validated successful POST identity;
    reconnect or observation retry never extends it and never repeats POST.
  measurement: Fake-clock create correlation test at 9.999s and 10s.
acceptance:
- id: AC-001
  given: The approved UX scenario prerequisites hold.
  when: UAC-001 is exercised through the public provider/palette boundary.
  then: The corresponding UX success or typed failure observation occurs without cross-server or stale execution.
  requirement_refs:
  - FR-001
- id: AC-002
  given: The approved UX scenario prerequisites hold.
  when: UAC-002 is exercised through the public provider/palette boundary.
  then: The corresponding UX success or typed failure observation occurs without cross-server or stale execution.
  requirement_refs:
  - FR-002
- id: AC-003
  given: The approved UX scenario prerequisites hold.
  when: UAC-003 is exercised through the public provider/palette boundary.
  then: The corresponding UX success or typed failure observation occurs without cross-server or stale execution.
  requirement_refs:
  - FR-003
- id: AC-004
  given: The approved UX scenario prerequisites hold.
  when: UAC-004 is exercised through the public provider/palette boundary.
  then: The corresponding UX success or typed failure observation occurs without cross-server or stale execution.
  requirement_refs:
  - FR-004
- id: AC-005
  given: The approved UX scenario prerequisites hold.
  when: UAC-005 is exercised through the public provider/palette boundary.
  then: The corresponding UX success or typed failure observation occurs without cross-server or stale execution.
  requirement_refs:
  - FR-005
- id: AC-006
  given: The approved UX scenario prerequisites hold.
  when: UAC-006 is exercised through the public provider/palette boundary.
  then: The corresponding UX success or typed failure observation occurs without cross-server or stale execution.
  requirement_refs:
  - FR-006
- id: AC-007
  given: The approved UX scenario prerequisites hold.
  when: UAC-007 is exercised through the public provider/palette boundary.
  then: The corresponding UX success or typed failure observation occurs without cross-server or stale execution.
  requirement_refs:
  - FR-007
- id: AC-008
  given: The approved UX scenario prerequisites hold.
  when: UAC-008 is exercised through the public provider/palette boundary.
  then: The corresponding UX success or typed failure observation occurs without cross-server or stale execution.
  requirement_refs:
  - FR-008
- id: AC-009
  given: The approved UX scenario prerequisites hold.
  when: UAC-009 is exercised through the public provider/palette boundary.
  then: The corresponding UX success or typed failure observation occurs without cross-server or stale execution.
  requirement_refs:
  - FR-009
- id: AC-010
  given: The approved UX scenario prerequisites hold.
  when: UAC-010 is exercised through the public provider/palette boundary.
  then: The corresponding UX success or typed failure observation occurs without cross-server or stale execution.
  requirement_refs:
  - FR-010
- id: AC-011
  given: Project/catalog/profile/parameter discovery contains one failed server or resource scope while another
    scope and the current palette context remain valid.
  when: UAC-011 retries the failed scope through the public provider/palette boundary.
  then: The UI names the failed scope and reason, exposes retry, keeps healthy scopes usable, and preserves the
    valid target selection, compatible parameter values, and focus until retry converges.
  requirement_refs:
  - FR-009
- id: AC-012
  given: Catalog refresh starts while the selected server/project/profile, parameter values, and focus are present.
  when: UAC-012 refresh completes successfully or reports a scoped failure.
  then: A still-valid target preserves selection, compatible values, and focus; an invalid target shows the reason
    and reselection path without silently accepting stale context or resetting unrelated valid state.
  requirement_refs:
  - FR-005
  - FR-009
- id: AC-FLEET-001
  given: Two Workspace views and enabled servers A/B
  when: B publishes a session revision
  then: Both views show the same A+B list and revision while keeping independent selection
  requirement_refs:
  - FR-FLEET-001
  - FR-FLEET-002
- id: AC-FLEET-002
  given: A view is active on A
  when: New Session creates on B
  then: B/session appears before confirmed and selection switches in the same window without another process/window/store
  requirement_refs:
  - FR-FLEET-003
  - FR-FLEET-004
  - FR-FLEET-009
- id: AC-FLEET-003
  given: A and B have the same display label and local session id
  when: The list and active context render
  then: Both rows show owning label plus stable serverId and route separately
  requirement_refs:
  - FR-008
  - FR-FLEET-005
  - FR-FLEET-013
- id: AC-FLEET-004
  given: B has a last-known snapshot and disconnects
  when: A continues and B reconnects
  then: A remains current; B stays explicitly stale then atomically replaces only B on recovery
  requirement_refs:
  - FR-FLEET-006
  - FR-FLEET-008
- id: AC-FLEET-005
  given: Workspace state is dirty on A
  when: The user selects B
  then: Active remains A pending confirmation; Cancel clears pending; Confirm revalidates B, discards old state,
    then commits B in the same window
  requirement_refs:
  - FR-FLEET-004
  - FR-FLEET-012
- id: AC-FLEET-006
  given: One subscriber stalls across many fleet revisions
  when: Its pending slot is occupied
  then: Intermediate revisions coalesce latest-wins or transport is severed as slow_subscriber; other views progress
    and resubscribe replays current full snapshot
  requirement_refs:
  - FR-FLEET-002
  - NFR-FLEET-003
---

# Requirements

## Functional requirements

The canonical EARS wording is projected above and in [design-plan.json](design-plan.json). Existing typed profile, arbitrary-N parameter form, server-owned materialization, stale validation and partial launch-catalog recovery remain mandatory. Fleet session lifecycle now adds one application-scoped authority, identical Web/Workspace observation, compound routing, guarded same-window selection, bounded create observation and bounded subscriber delivery.

The UX counterexamples remain owned by [ux.md](ux.md). In particular, session rows and active context always expose the owning server label and expose stable `serverId` when labels collide; local session ID, hover-only identity, server-gated lists, another window, or a renderer-local server connection cannot satisfy the requirement.

## Fixed behavior

`session.command_profiles`, one server-local `GET /api/session-config`, version-2 create, typed values and server-owned argv materialization remain unchanged. Session membership is separately supplied by the host-neutral FleetRuntime and never inferred from a create response or one selected server connection.
