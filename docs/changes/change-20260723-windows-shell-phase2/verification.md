---
change: change-20260723-windows-shell-phase2
role: verification
id: verif-20260723-windows-shell-phase2
kind: verification
title: 20260723 Windows Shell Phase 2 Verification
status: draft
created: '2026-07-23'
summary: Tiered verification plan (T0 pure / T1 wired / T2 contract / T3 fidelity)
  derived from implementation_contracts. Fake + fidelity backstop + contract-test
  triple for every external-dependency boundary crossing.
change_id: change-20260723-windows-shell-phase2
verifications:
- id: verify-b1-envelope-strict
  contract: contract-b1-jsonlines-envelope-shape
  tier: T1
  method: vitest strict-schema fuzz + Playwright-for-Electron pipe round trip (openSession
    succeeds; unknown-field envelope is rejected without breaking the connection)
  criterion: 100% of unknown-field / unknown-op envelopes receive an {error} reply
    and the pipe connection remains open for the next valid line.
  requirement_refs:
  - FR-B1-01
  - FR-B1-02
  - FR-B1-03
  - NFR-boundary-1-envelope-strict
  command: null
- id: verify-registry-atomic-dedup
  contract: contract-b1-window-registry-dedup
  tier: T1
  method: 'Playwright-for-Electron: two rapid openSession(sess-X) calls (double-click
    simulation) and a racing deep-link injection; assert BrowserWindow count for sess-X
    stays at 1.'
  criterion: Across ≥100 fuzzed race iterations, window count for any session id stays
    ≤ 1.
  requirement_refs:
  - FR-MIG-01
  command: null
- id: verify-supervisor-partition
  contract: contract-b3-daemon-supervisor-state-machine
  tier: T0
  method: xUnit pure state-machine test enumerating every (state, event) pair including
    double-failure paths; fake Runner + fake HTTP probe injection. T3 adopt path against
    make run-dev is RunDevGatewayE2ETests.DaemonSupervisor_adopts_*.
  criterion: Every (state, event) branch reaches a terminal Connected or Degraded/failed
    state within a bounded probe count; no branch leaves state stuck in Spawning.
  requirement_refs:
  - FR-B3-01
  - FR-B3-02
  - FR-B3-03
  command: ./clients/windows-shell/scripts/e2e.sh --start-run-dev
- id: verify-wsl-detach-fidelity
  contract: contract-b3-wsl-detach-mechanism
  tier: T3
  method: 'WSL detach spike (setsid) via clients/windows-shell/scripts/wsl-detach-spike.sh —
    separate from run-dev e2e. See docs/wsl-detach-spike-result.md.'
  criterion: Process remains live and /api/sessions responds ≥5s after launcher returns.
  requirement_refs:
  - FR-B3-05
  command: bash clients/windows-shell/scripts/wsl-detach-spike.sh
- id: verify-token-fresh-and-explicit-failure
  contract: contract-b2-token-acquisition
  tier: T1
  method: 'xUnit FileTokenSourceTests; run-dev e2e uses NoAuthTokenSource (loopback
    -no-auth). Auth path FakeVsReal remains file-rotation unit + future token e2e.'
  criterion: Post-rotation connection succeeds with the new token in ≤1 attempt; unreadable
    UNC yields an explicit failure indicator every time.
  requirement_refs:
  - FR-B2-01
  - FR-B2-03
  command: powershell.exe -NoProfile -ExecutionPolicy Bypass -File clients/windows-shell/scripts/win-test.ps1
- id: verify-native-ws-ticket-fidelity
  contract: contract-b2-native-ws-auth-path
  tier: T3
  method: 'Shell.Core E2E against make run-dev — Mint_ticket_and_open_websocket
    (POST /api/ws-ticket + ClientWebSocket). See clients/windows-shell/docs/e2e.md.'
  criterion: SDK connect succeeds when the two-step flow is followed against the real
    gateway; always-on suite skips when AG_E2E_RUN_DEV is unset.
  requirement_refs:
  - FR-B2-02
  command: ./clients/windows-shell/scripts/e2e.sh --start-run-dev
- id: verify-hosted-mode-token-not-in-url
  contract: contract-b2-hosted-mode-token-injection
  tier: T1
  method: 'Playwright-for-Electron: navigate to a hosted-mode window; capture network
    requests; assert token absent from URL and headers; assert SPA still authenticates.'
  criterion: Token string never observed in URL, Referer, or any request/response
    header; SPA reaches connected state.
  requirement_refs:
  - FR-B2-04
  command: null
- id: verify-launch-order-independence
  contract: contract-b1-b2-launch-ordering
  tier: T1
  method: 'Playwright-for-Electron: force UNC token read to fail; drive openSession
    over the pipe; assert the resulting window shows the connection-error view, not
    the SPA.'
  criterion: Under injected boundary-2 failure, the Workspace window never renders
    the SPA.
  requirement_refs:
  - FR-B1-03
  - FR-B2-04
  command: null
- id: verify-jb-staged
  contract: contract-jump-back-staged-resolution
  tier: T2
  method: 'xUnit table-driven test with fake window-enumeration + fake HWND cache:
    exercise cache-hit, stale-hwnd, process-title-match-hit, and no-match paths; assert
    (activation vs ''not found'' vs conflicting-fallthrough) outcomes.'
  criterion: Every enumerated path yields exactly one of the four documented outcomes;
    no path activates a non-matching window.
  requirement_refs:
  - FR-JB-01
  - FR-JB-02
  - FR-JB-03
  command: null
- id: verify-engage-restore-partition
  contract: contract-engage-focus-return-mechanism
  tier: T3
  method: 'xUnit + manual Win32 fidelity checklist: exercise restore-live, restore-denied
    (foreground-lock timer), and target-destroyed branches with a fake IWin32InteropService;
    T3 manual checklist for real HWND behavior on target hardware.'
  criterion: In each branch, foreground state matches the specified outcome and engage-mode
    UI reaches a clean exit.
  requirement_refs:
  - FR-EF-01
  - FR-EF-02
  command: null
- id: verify-restart-continuity-fidelity
  contract: contract-b3-restart-continuity
  tier: T3
  method: 'FakeVsReal fidelity: 3 running sessions, force daemon restart via graceful-shutdown
    + re-spawn against a fake WSL daemon; assert all surfaces show the same 3 sessions
    within the measured threshold; separately test the ETag-mismatch and mid-flight
    race scenarios.'
  criterion: All surfaces reach Connected within measured threshold; session set matches
    pre-restart; no partial view observed.
  requirement_refs:
  - FR-B3-04
  - NFR-daemon-restart-reconnect-delay
  command: null
- id: verify-toast-outcome-partition
  contract: contract-toast-panel-watched-detection
  tier: T2
  method: 'xUnit with fake IWin32InteropService: enumerate all (panel-open, DND, locked,
    query-ok) tuples; assert the deterministic outcome per the documented partition;
    separately assert repeated trials on the same tuple produce the same outcome.'
  criterion: Every enumerated tuple yields exactly one documented outcome; query-failure
    branch always fires toast.
  requirement_refs:
  - FR-TOAST-01
  command: null
- id: verify-health-toast-budget
  contract: contract-daemon-health-toast-budget
  tier: T2
  method: 'xUnit + notification-history probe: drive DaemonSupervisor through a Healthy→Degraded→Healthy
    flap burst within the observation window; capture Windows notification count;
    assert 0.'
  criterion: 0 daemon-health notifications observed in a 5min observation window even
    under a 100-transition burst.
  requirement_refs:
  - FR-TOAST-02
  - NFR-daemon-healthy-toast-budget-window
  command: null
- id: verify-mig-single-window
  contract: contract-migration-window-per-session-invariant
  tier: T2
  method: 'Playwright-for-Electron: open session from all three entry points; assert
    window count 1 and no browser process; run ESLint on the workspace source tree
    and assert 0 violations of the no-restricted-syntax rule; press Ctrl+T/Ctrl+L
    in hosted window and assert no effect.'
  criterion: Window count = 1 across entry points; ESLint reports 0 rule violations;
    no default-browser process; accelerators inert.
  requirement_refs:
  - FR-MIG-01
  - FR-MIG-02
  - FR-MIG-03
  command: null
- id: verify-focus-invariant-scan
  contract: contract-cross-flow-focus-invariant
  tier: T2
  method: 'Roslyn analyzer or ripgrep-based lint step in CI: enumerate SetForegroundWindow
    / AllowSetForegroundWindow call sites; assert only the two allowed files contain
    them.'
  criterion: Lint step reports exactly 2 call sites; a synthetic third call site in
    a test fixture fails the lint.
  requirement_refs:
  - FR-FOCUS-INV
  command: null
- id: verify-deeplink-schema-fidelity
  contract: contract-deep-link-question-jump-kind-gap
  tier: T1
  method: 'xUnit table test against protocol/deep-links.schema.json fixtures: assert
    session/approval kinds route correctly; assert question/jump URIs return the documented
    rejection until the additive extension lands.'
  criterion: Session/approval URIs route to the right target; unknown kinds emit the
    documented rejection message with 100% consistency.
  requirement_refs:
  - FR-B1-01
  command: null
- id: verify-approve-rollback
  contract: contract-approve-submission-rollback
  tier: T0
  method: 'xUnit table-driven test on SupervisionState.Reduce: feed (optimistic-remove,
    network-error) and (optimistic-remove, 5xx) sequences; assert item reappears and
    agent-unblocked flag is false.'
  criterion: Every failure classification triggers rollback or explicit error; agent-unblocked
    flag is never set on a failed submission.
  requirement_refs:
  - FR-APPROVE-ROLLBACK
  command: null
- id: verify-rbo-outcome
  contract: contract-resolved-by-other-display
  tier: T0
  method: 'xUnit table-driven test on SupervisionState.Reduce: feed accepted / resolved-by-other
    / malformed responses; assert render state and non-duplication.'
  criterion: Every response kind yields the documented render state; 0 duplicate submissions
    observed across ≥100 fuzzed race iterations.
  requirement_refs:
  - FR-APPROVE-RESOLVED-BY-OTHER
  command: null
- id: verify-structural-toast-separation
  contract: contract-health-toast-structural-separation
  tier: T2
  method: Roslyn analyzer (or NetArchTest) rule asserting DaemonSupervisor namespace
    has no reference into ToastNotifier namespace; wired into CI as a build step.
  criterion: Analyzer reports 0 forbidden edges; a synthetic PR adding such an edge
    fails the check.
  requirement_refs:
  - FR-TOAST-02
  command: null
- id: verify-close-not-stop
  contract: contract-window-close-not-session-stop
  tier: T1
  method: 'Playwright-for-Electron: close a Workspace window mid-session; assert daemon
    session status stays running/waiting on Shell panel; rapid close-then-reopen assertion
    for boundary case.'
  criterion: 0 stop/end API calls observed via network trace when window close fires;
    session status never transitions to ended within observation window.
  requirement_refs:
  - FR-MIG-03
  command: null
- id: verify-state-schema-safe-fallback
  contract: contract-workspace-state-schema-evolution
  tier: T1
  method: vitest with fixture files at (current, older-known, newer-unknown, corrupt)
    schema_versions; assert registry behavior matches the documented outcome for each.
  criterion: Newer-unknown file yields empty-map identical to missing; older-known
    runs documented upgrade path and restores; corrupt yields empty-map.
  requirement_refs:
  - FR-WS-STATE-SCHEMA
  command: null
- id: verify-hosted-spa-browser-mode-unchanged
  contract: contract-hosted-mode-existing-spa-compat
  tier: T1
  method: Existing clients/ui Playwright suite executed against the browser-mode entry
    unchanged; a diff over CSP headers and WS-origin behavior asserts byte-identity
    across the hosted-mode branch landing.
  criterion: Browser-mode suite passes unmodified; CSP / WS-origin diff is empty.
  requirement_refs:
  - FR-HOSTED-SPA-COMPAT
  command: null
- id: verify-quit-vs-stop
  contract: contract-quit-vs-daemon-stop
  tier: T0
  method: 'xUnit: substitute DaemonSupervisor with a spy; invoke the Quit menu handler;
    assert Stop was never called. Separately assert the Stop-daemon menu handler DOES
    call Stop.'
  criterion: Quit handler produces 0 calls to DaemonSupervisor.Stop; Stop-daemon handler
    produces exactly 1 such call.
  requirement_refs:
  - FR-B3-05
  command: null
witnesses:
- id: witness-b1-openSession-normal
  contract: contract-b1-jsonlines-envelope-shape
  case: normal
  risk_tags: []
  requirement_refs:
  - FR-B1-01
  - FR-B1-02
  - FR-B1-03
  verification_refs:
  - verify-b1-envelope-strict
- id: witness-b1-malformed-adv
  contract: contract-b1-jsonlines-envelope-shape
  case: adversarial
  risk_tags:
  - malformed
  - boundary
  requirement_refs:
  - FR-B1-01
  - FR-B1-02
  - NFR-boundary-1-envelope-strict
  verification_refs:
  - verify-b1-envelope-strict
- id: witness-registry-single-open-normal
  contract: contract-b1-window-registry-dedup
  case: normal
  risk_tags: []
  requirement_refs:
  - FR-MIG-01
  verification_refs:
  - verify-registry-atomic-dedup
- id: witness-registry-double-open-adv
  contract: contract-b1-window-registry-dedup
  case: adversarial
  risk_tags:
  - concurrency
  - repeated_usage
  - stale
  requirement_refs:
  - FR-MIG-01
  verification_refs:
  - verify-registry-atomic-dedup
- id: witness-supervisor-adopt-normal
  contract: contract-b3-daemon-supervisor-state-machine
  case: normal
  risk_tags: []
  requirement_refs:
  - FR-B3-01
  - FR-B3-02
  verification_refs:
  - verify-supervisor-partition
- id: witness-supervisor-spawn-crash-adv
  contract: contract-b3-daemon-supervisor-state-machine
  case: adversarial
  risk_tags:
  - boundary
  - recovery
  - unsupported_environment
  - unknown
  - inconclusive
  - conflicting_evidence
  requirement_refs:
  - FR-B3-01
  - FR-B3-02
  - FR-B3-03
  verification_refs:
  - verify-supervisor-partition
- id: witness-wsl-detach-survives-normal
  contract: contract-b3-wsl-detach-mechanism
  case: normal
  risk_tags: []
  requirement_refs:
  - FR-B3-05
  verification_refs:
  - verify-wsl-detach-fidelity
- id: witness-wsl-detach-launcher-kill-adv
  contract: contract-b3-wsl-detach-mechanism
  case: adversarial
  risk_tags:
  - boundary
  - unsupported_environment
  - recovery
  - repeated_usage
  requirement_refs:
  - FR-B3-05
  verification_refs:
  - verify-wsl-detach-fidelity
- id: witness-token-fresh-normal
  contract: contract-b2-token-acquisition
  case: normal
  risk_tags: []
  requirement_refs:
  - FR-B2-01
  verification_refs:
  - verify-token-fresh-and-explicit-failure
- id: witness-token-rotation-adv
  contract: contract-b2-token-acquisition
  case: adversarial
  risk_tags:
  - stale
  - boundary
  - unsupported_environment
  requirement_refs:
  - FR-B2-01
  - FR-B2-03
  verification_refs:
  - verify-token-fresh-and-explicit-failure
- id: witness-native-ws-mint-normal
  contract: contract-b2-native-ws-auth-path
  case: normal
  risk_tags: []
  requirement_refs:
  - FR-B2-02
  verification_refs:
  - verify-native-ws-ticket-fidelity
- id: witness-native-ws-header-only-adv
  contract: contract-b2-native-ws-auth-path
  case: adversarial
  risk_tags:
  - boundary
  - unsupported_environment
  - stale
  requirement_refs:
  - FR-B2-02
  verification_refs:
  - verify-native-ws-ticket-fidelity
- id: witness-hosted-mode-preload-normal
  contract: contract-b2-hosted-mode-token-injection
  case: normal
  risk_tags: []
  requirement_refs:
  - FR-B2-04
  verification_refs:
  - verify-hosted-mode-token-not-in-url
- id: witness-hosted-mode-token-leak-adv
  contract: contract-b2-hosted-mode-token-injection
  case: adversarial
  risk_tags:
  - boundary
  - malformed
  - stale
  requirement_refs:
  - FR-B2-04
  verification_refs:
  - verify-hosted-mode-token-not-in-url
- id: witness-launch-order-normal
  contract: contract-b1-b2-launch-ordering
  case: normal
  risk_tags: []
  requirement_refs:
  - FR-B1-03
  - FR-B2-04
  verification_refs:
  - verify-launch-order-independence
- id: witness-launch-order-partial-adv
  contract: contract-b1-b2-launch-ordering
  case: adversarial
  risk_tags:
  - partial_data
  - boundary
  requirement_refs:
  - FR-B1-03
  - FR-B2-04
  verification_refs:
  - verify-launch-order-independence
- id: witness-jb-hwnd-hit-normal
  contract: contract-jump-back-staged-resolution
  case: normal
  risk_tags: []
  requirement_refs:
  - FR-JB-01
  verification_refs:
  - verify-jb-staged
- id: witness-jb-stale-hwnd-adv
  contract: contract-jump-back-staged-resolution
  case: adversarial
  risk_tags:
  - stale
  - conflicting_evidence
  - boundary
  - unknown
  - inconclusive
  requirement_refs:
  - FR-JB-02
  - FR-JB-03
  verification_refs:
  - verify-jb-staged
- id: witness-engage-restore-normal
  contract: contract-engage-focus-return-mechanism
  case: normal
  risk_tags: []
  requirement_refs:
  - FR-EF-01
  verification_refs:
  - verify-engage-restore-partition
- id: witness-engage-target-gone-adv
  contract: contract-engage-focus-return-mechanism
  case: adversarial
  risk_tags:
  - boundary
  - unsupported_environment
  - recovery
  - unknown
  - inconclusive
  requirement_refs:
  - FR-EF-02
  verification_refs:
  - verify-engage-restore-partition
- id: witness-restart-3-sessions-normal
  contract: contract-b3-restart-continuity
  case: normal
  risk_tags: []
  requirement_refs:
  - FR-B3-04
  verification_refs:
  - verify-restart-continuity-fidelity
- id: witness-restart-mid-backfill-race-adv
  contract: contract-b3-restart-continuity
  case: adversarial
  risk_tags:
  - concurrency
  - recovery
  - boundary
  - scale
  - repeated_usage
  - unknown
  - inconclusive
  requirement_refs:
  - FR-B3-04
  - NFR-daemon-restart-reconnect-delay
  verification_refs:
  - verify-restart-continuity-fidelity
- id: witness-toast-fires-normal
  contract: contract-toast-panel-watched-detection
  case: normal
  risk_tags: []
  requirement_refs:
  - FR-TOAST-01
  verification_refs:
  - verify-toast-outcome-partition
- id: witness-toast-query-failed-adv
  contract: contract-toast-panel-watched-detection
  case: adversarial
  risk_tags:
  - conflicting_evidence
  - boundary
  - unknown
  - inconclusive
  - stale
  requirement_refs:
  - FR-TOAST-01
  verification_refs:
  - verify-toast-outcome-partition
- id: witness-health-toast-quiet-normal
  contract: contract-daemon-health-toast-budget
  case: normal
  risk_tags: []
  requirement_refs:
  - FR-TOAST-02
  verification_refs:
  - verify-health-toast-budget
- id: witness-health-toast-flap-adv
  contract: contract-daemon-health-toast-budget
  case: adversarial
  risk_tags:
  - scale
  - repeated_usage
  requirement_refs:
  - FR-TOAST-02
  - NFR-daemon-healthy-toast-budget-window
  verification_refs:
  - verify-health-toast-budget
- id: witness-mig-repeat-open-normal
  contract: contract-migration-window-per-session-invariant
  case: normal
  risk_tags: []
  requirement_refs:
  - FR-MIG-01
  - FR-MIG-02
  verification_refs:
  - verify-mig-single-window
- id: witness-mig-lint-regression-adv
  contract: contract-migration-window-per-session-invariant
  case: adversarial
  risk_tags:
  - repeated_usage
  - boundary
  - stale
  - recovery
  requirement_refs:
  - FR-MIG-01
  - FR-MIG-03
  verification_refs:
  - verify-mig-single-window
- id: witness-focus-invariant-clean-normal
  contract: contract-cross-flow-focus-invariant
  case: normal
  risk_tags: []
  requirement_refs:
  - FR-FOCUS-INV
  verification_refs:
  - verify-focus-invariant-scan
- id: witness-focus-invariant-new-site-adv
  contract: contract-cross-flow-focus-invariant
  case: adversarial
  risk_tags:
  - boundary
  - repeated_usage
  - recovery
  requirement_refs:
  - FR-FOCUS-INV
  verification_refs:
  - verify-focus-invariant-scan
- id: witness-deeplink-approved-normal
  contract: contract-deep-link-question-jump-kind-gap
  case: normal
  risk_tags: []
  requirement_refs:
  - FR-B1-01
  verification_refs:
  - verify-deeplink-schema-fidelity
- id: witness-deeplink-unknown-kind-adv
  contract: contract-deep-link-question-jump-kind-gap
  case: adversarial
  risk_tags:
  - boundary
  - unsupported_environment
  - stale
  - recovery
  requirement_refs:
  - FR-B1-01
  verification_refs:
  - verify-deeplink-schema-fidelity
- id: witness-approve-success-normal
  contract: contract-approve-submission-rollback
  case: normal
  risk_tags: []
  requirement_refs:
  - FR-APPROVE-ROLLBACK
  verification_refs:
  - verify-approve-rollback
- id: witness-approve-network-error-adv
  contract: contract-approve-submission-rollback
  case: adversarial
  risk_tags:
  - recovery
  - partial_data
  - unknown
  - inconclusive
  requirement_refs:
  - FR-APPROVE-ROLLBACK
  verification_refs:
  - verify-approve-rollback
- id: witness-rbo-normal-win
  contract: contract-resolved-by-other-display
  case: normal
  risk_tags: []
  requirement_refs:
  - FR-APPROVE-RESOLVED-BY-OTHER
  verification_refs:
  - verify-rbo-outcome
- id: witness-rbo-race-loss-adv
  contract: contract-resolved-by-other-display
  case: adversarial
  risk_tags:
  - conflicting_evidence
  - concurrency
  - unknown
  - inconclusive
  - boundary
  requirement_refs:
  - FR-APPROVE-RESOLVED-BY-OTHER
  verification_refs:
  - verify-rbo-outcome
- id: witness-structural-clean-normal
  contract: contract-health-toast-structural-separation
  case: normal
  risk_tags: []
  requirement_refs:
  - FR-TOAST-02
  verification_refs:
  - verify-structural-toast-separation
- id: witness-structural-edge-added-adv
  contract: contract-health-toast-structural-separation
  case: adversarial
  risk_tags:
  - boundary
  - repeated_usage
  - recovery
  requirement_refs:
  - FR-TOAST-02
  verification_refs:
  - verify-structural-toast-separation
- id: witness-close-normal
  contract: contract-window-close-not-session-stop
  case: normal
  risk_tags: []
  requirement_refs:
  - FR-MIG-03
  verification_refs:
  - verify-close-not-stop
- id: witness-close-reopen-race-adv
  contract: contract-window-close-not-session-stop
  case: adversarial
  risk_tags:
  - boundary
  - concurrency
  requirement_refs:
  - FR-MIG-03
  verification_refs:
  - verify-close-not-stop
- id: witness-state-schema-current-normal
  contract: contract-workspace-state-schema-evolution
  case: normal
  risk_tags: []
  requirement_refs:
  - FR-WS-STATE-SCHEMA
  verification_refs:
  - verify-state-schema-safe-fallback
- id: witness-state-schema-newer-unknown-adv
  contract: contract-workspace-state-schema-evolution
  case: adversarial
  risk_tags:
  - stale
  - recovery
  - unknown
  - inconclusive
  - boundary
  - unsupported_environment
  requirement_refs:
  - FR-WS-STATE-SCHEMA
  verification_refs:
  - verify-state-schema-safe-fallback
- id: witness-hosted-spa-browser-mode-passes-normal
  contract: contract-hosted-mode-existing-spa-compat
  case: normal
  risk_tags: []
  requirement_refs:
  - FR-HOSTED-SPA-COMPAT
  verification_refs:
  - verify-hosted-spa-browser-mode-unchanged
- id: witness-hosted-spa-shared-code-regression-adv
  contract: contract-hosted-mode-existing-spa-compat
  case: adversarial
  risk_tags:
  - stale
  - recovery
  - boundary
  - repeated_usage
  requirement_refs:
  - FR-HOSTED-SPA-COMPAT
  verification_refs:
  - verify-hosted-spa-browser-mode-unchanged
- id: witness-quit-preserves-daemon-normal
  contract: contract-quit-vs-daemon-stop
  case: normal
  risk_tags: []
  requirement_refs:
  - FR-B3-05
  verification_refs:
  - verify-quit-vs-stop
- id: witness-quit-regression-adv
  contract: contract-quit-vs-daemon-stop
  case: adversarial
  risk_tags:
  - boundary
  - repeated_usage
  - recovery
  requirement_refs:
  - FR-B3-05
  verification_refs:
  - verify-quit-vs-stop
---

## Tier distribution

- **T0 pure** (xUnit against pure reducers / analyzers): DaemonSupervisor state machine,
  SupervisionState optimistic-rollback + resolved-by-other, Quit vs Stop-daemon menu split.
- **T1 wired** (test-harness + faked external boundary): boundary-1 envelope strict-schema,
  window-registry atomic dedup + close-not-stop + schema fallback, boundary-2 token fresh-read,
  hosted-mode token-not-in-URL, launch-order independence, deep-link schema fidelity,
  hosted-mode browser-mode regression baseline.
- **T2 contract** (Roslyn analyzer / lint-enforced structural): cross-flow focus invariant scan,
  health/toast structural separation, migration window-per-session invariant with ESLint,
  jump-back staged resolution with fake enumeration, toast panel-watched outcome partition,
  daemon-health toast budget.
- **T3 fidelity** (opt-in, `make test-e2e`): WSL detach mechanism survival, native WS
  ticket-flow SDK-vs-real-gateway, engage focus restore manual checklist, restart continuity
  end-to-end with 3 sessions.

## Verification-to-contract map

`verifications[]` in the frontmatter above binds each verify-* id to its owning contract,
tier, method, and machine-checkable criterion. `witnesses[]` reciprocates: every FR is covered
by at least one normal witness AND one adversarial witness, and every observable is exercised
by both cases.

## Commands

- Go unit + integration (T0/T1): `cd src && go test ./...` (host + server + orchestrator + platform
  packages already covered).
- Go opt-in fidelity (T3): `make test-e2e` (WSL detach spike case included once the ADR-selected
  mechanism lands).
- Workspace Playwright (T1/T2): `cd clients/ui && npm run test:web` for the hosted-mode
  browser-mode baseline; `cd clients/workspace && npx playwright test` for window-discipline
  fixtures.
- C# xUnit (T0/T1/T2): `dotnet test` under `clients/windows-shell/AgentGrid.Shell.Core.Tests`
  and `AgentGrid.Shell.Tests`.
- Static analysis (T2): Roslyn analyzer + NetArchTest under
  `clients/windows-shell/AgentGrid.Shell.Analyzers/`; ESLint no-restricted-syntax under
  `clients/workspace/eslint.config.js`.

## Fake + fidelity + contract triple discipline

Every external-dependency boundary lists all three in the plan's `test_seams`:

| Boundary | Fake | Fidelity backstop | Contract test |
|----------|------|-------------------|---------------|
| Shell↔Workspace pipe | fake named-pipe server (xUnit + vitest) | Playwright-for-Electron | closed-schema fuzz |
| Shell↔WSL daemon REST/WS | fake gateway (httptest) | FakeVsReal against real server binary | native SDK-vs-real-gateway ticket fidelity |
| Shell↔WSL lifecycle | fake Runner | opt-in T3 real WSL launcher-kill | supervisor outcome partition (xUnit) |
| Workspace SPA hosted-mode | fake-backend.ts extended with hosted-flag | Playwright-for-Electron network trace | preload contextBridge token contract |

