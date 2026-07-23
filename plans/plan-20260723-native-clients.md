# Agent Grid Native Clients Plan

- **作成日**: 2026-07-23
- **更新日**: 2026-07-23 (rev 4 — Windows のワークスペースホストを Electron に決定 (シェルはネイティブ維持)、詳細設計を plan-20260723-windows-shell-design.md に分離。rev 3 で Windows/iOS を先行 OS に決定。rev 2 でデスクトップを「フルネイティブ再構築」から「ネイティブシェル + 既存 Web ワークスペースのホスト」へ再定義)
- **ブランチ**: `claude/native-clients-plan-review-2sjfs5`
- **ステータス**: draft (設計レビュー段階)
- **影響範囲**: 新規デスクトップアプリシェル、`server/api` の契約層公開面、approval/question のサーバー側ドメイン新設、`clients/ui` の hosted モード、配布/署名/自動更新

## Related documents

| Document | Relation |
|---|---|
| [multi-host-gateway.md](./multi-host-gateway.md) | **Prerequisite for mobile phases.** Remote reachability, E2E encryption, pairing, and auth design are authoritative there. Open conflict to resolve: push-notification delivery requires something server-side to observe session events, while the gateway plan forbids the gateway from holding domain data (§ Phase R below). |
| [remote-control-mobile-session-deep-link.md](./remote-control-mobile-session-deep-link.md) | Prior investigation of notification → mobile deep-link flows; input to `deep-links.schema.json`. |
| [client-as-host.md](./client-as-host.md) | Client-machine-as-host follow-up; unaffected but shares the app-shell packaging. |
| [plan-20260723-repo-structure.md](./plan-20260723-repo-structure.md) | Repository layout anticipating `clients/`, `protocol/`, `contracts/`, the `client/`→`host/` rename, and reserved names for the multi-host gateway. |
| [ARCHITECTURE.md](../ARCHITECTURE.md) | The `server/*` gateway layer already exists as a stateless proxy explicitly designed for "future native clients". This plan adds clients to that face; it does not restructure the server. |
| ADR-0021 / ADR-0025 / ADR-0011 / ADR-0022 | Existing wire-type and reconnect decisions this plan extends or supersedes (§ Current-state gaps). |

## Purpose

Agent Grid's core product hypothesis is the **supervision loop**: as agents run autonomously for longer, the throughput bottleneck moves to human response latency at approval points, agent questions, and completion events. Everything in this plan is justified by shrinking notification-to-intervention time and context-recovery time — not by UI technology preferences.

Evolve Agent Grid from a browser-tab-hosted control surface into a server-backed platform with:

1. a **desktop app** that owns an ambient supervision panel and the session workspace, removing the browser from the local flow entirely;
2. **mobile supervision clients** that keep the same loop alive away from the desk;
3. one **platform-independent contract** for sessions, approvals, questions, commands, and events that every client — including the desktop app itself — consumes without privileged back doors.

The goal is not to maximize shared UI code, and equally not to maximize native rewrites. Native investment concentrates where design quality is the product (the ambient panel, window/focus behavior); the mature web workspace is promoted into the app rather than rebuilt.

## Product hypothesis (decided)

Two hypotheses were considered:

- **A. Remote supervision tool** — value is the notification → intervention loop. Investment order: contract layer → ambient panel + app shell → remote path + push → one mobile OS.
- **B. OS-integrated agent workspace as a new product** — overlays, UI Automation, voice, and local-AI experiments are themselves the product.

**This plan commits to A.** The success measures below are A-measures, and Phase 2's exit criterion is A-shaped. Evidence: the ambient-panel category (e.g. Vibe Island — a native macOS notch panel doing exactly monitor / approve / answer / jump-back in <50 MB) validates that the highest-value native footprint is small and supervision-shaped, not a workspace rebuild. B-class experiments (overlays, UI Automation, voice) are deferred to Phase 6 and gated on usage data from A.

## Desired outcomes

1. Agent Grid sessions, approvals, questions, commands, and events have one platform-independent contract.
2. The desktop app is the primary local surface: ambient panel + owned workspace windows. **The browser does not appear in the local flow.**
3. iOS is the first remote supervision client for notifications, approvals, questions, short interventions, and handoff; Android follows with the same role.
4. The browser remains first-class for remote, temporary, installation-free, and fallback access only.
5. Client implementations may diverge where platform conventions differ, without diverging in domain behavior.

## Product model

```text
Agent Grid Platform
├── Server / execution plane            (source of truth; separate process, survives all clients)
│   ├── session ownership, PTY and agent processes
│   ├── containers and worktrees
│   ├── orchestrator
│   ├── approvals and questions         ← new server-side domain (§ Current-state gaps)
│   └── event history
│
├── Protocol / domain contracts
│   ├── session state machine, command model, event model
│   ├── capability negotiation, version compatibility
│   ├── reconnect semantics
│   ├── notification policy
│   └── handoff and deep links
│
└── Clients
    ├── Desktop app (Agent Grid.app / .msix)
    ├── iOS · Android
    ├── Browser (remote / fallback only)
    └── CLI (unchanged; automation and scripting)
```

### Desktop app composition

```text
Agent Grid  (one app identity, one install)
├── Shell — fully native, resident; design investment concentrates here
│    ├── ambient panel (notch / tray flyout + top bar)
│    ├── notifications, deep links, global shortcuts, jump-back
│    └── daemon supervisor — launches / adopts / health-checks the bundled `server` binary
└── Workspace — on-demand window host for the existing SPA (hosted mode)
     └── terminal / files / diff / Markdown = existing web assets (xterm.js et al.)
     └── on Windows: an Electron app owned by the native shell (VS Code model);
        window discipline enforced in its window registry
```

The server is the source of truth. No GUI client owns long-running sessions. **Closing any client — including quitting the desktop app — must not terminate agent work.** The app supervises the daemon's lifecycle (login-item autostart, health, upgrade) but does not own it.

## Strategic decisions

### 1. The app is a client plus daemon supervisor, not a monolith

The desktop app talks to the daemon exclusively through the public contract — the same REST/WS face every other client uses. No privileged back door. This makes the app the first dogfooder of the contract layer, and keeps headless server operation (remote hosts, CLI-only, orchestrator) intact.

### 2. Desktop = native shell + hosted web workspace (the VS Code model)

Do not rebuild the workspace natively, and do not leave it in a browser tab. The shell — the ambient panel, notifications, deep links, shortcuts, daemon supervision — is native. The workspace content (terminal, files, diff, Markdown) is the existing SPA in a **hosted mode**, living in a dedicated window host: on Windows, an **Electron workspace app** launched and directed by the native shell (chosen over WebView2 islands for xterm.js/IME maturity, TS-owned multi-window management, and Playwright testability — see [plan-20260723-windows-shell-design.md](./plan-20260723-windows-shell-design.md)); on macOS, decided at Phase 4. VS Code and Cursor demonstrate that a web-rendered workspace reads as a first-class app; xterm.js in Electron is the industry-standard terminal for this class of product. The Electron footprint is confined to the on-demand workspace — the resident shell stays native.

Hosted mode is a real work item, not a wrapper: remove browser idioms (page navigation, browser scrollbars, web-page text selection), invert the window model (native tab/window per session; the SPA renders one session view per surface), route keyboard/IME/menu through the shell, and connect OS materials (Mica / vibrancy) and theme following.

### 3. The ambient panel is fully native

The panel lives on screen 100% of the time; jank, focus-stealing, or off-OS material quality are uninstall reasons there. The behaviors that define it — non-activating click-through focus (`NSPanel .nonactivatingPanel` / `WS_EX_NOACTIVATE`), all-Spaces/fullscreen presence, notch geometry (`NSScreen.safeAreaInsets`), OS blur materials — are native APIs; Electron/Tauri reach them only through native plugins anyway, at 3–10× the memory. Panel scope: session states, approve/deny, question answering, jump-back. macOS: notch panel. Windows (no notch): tray flyout + top-center floating bar.

### 4. Window discipline — the panel must not become a tab factory

- Panel destinations are always **app-owned native windows**; the SPA inside is an implementation detail. No URL bar, no tabs-of-tabs.
- **Windows are reused, not spawned**: session → window is 1:1 (or a slot in the single workspace window). Activating the same session twice focuses the existing window.
- **Closing a window ≠ closing a session**; window layout is restored after app restart.
- `agent-grid://` deep links resolve **inside the app** to existing-window activation, never to the browser.
- Interaction hierarchy: (1) resolve in the panel without opening anything (approvals, questions — should be the majority); (2) jump back to the existing terminal/IDE/target app window; (3) only then open an app viewer window (diff review, plan reading).

### 5. One install; headless server as a separate channel — deferred until distribution

The eventual distributable is a single bundle (dmg / brew cask / winget / msix) containing the native shell, the `server` binary, and the web assets, with the headless server distribution as the only second artifact (remote hosts). **For now the operating posture is personal use**: local build scripts, unpackaged binaries, manual updates — installer, code signing, and auto-update are deferred wholesale until distribution begins. On Windows the daemon is **not ported**: it runs unchanged inside WSL (or on a remote Linux host) and the shell/workspace connect over loopback — pty, Unix sockets, and sandboxing stay Linux-only. Version skew between shell and daemon still makes `compatibility-policy.md` a day-one need.

### 6. Platform stacks

| Surface | Technology | Role |
|---|---|---|
| Desktop shell (Windows) | C# + WinUI 3 / Windows App SDK; `WS_EX_NOACTIVATE`, Mica/Acrylic, AppNotification | Panel, notifications, deep links, jump-back, daemon supervision |
| Workspace host (Windows) | Electron (TS), on-demand, shell-directed | Session windows with window discipline; hosts the SPA |
| Desktop shell (macOS) | SwiftUI + AppKit (NSPanel, NSVisualEffectView); workspace host decided at Phase 4 | Same roles, plus notch panel |
| Workspace content | Existing SPA (React/TS/xterm.js) in hosted mode | Terminal, files, diff, Markdown — shared with browser client |
| iOS | Swift + SwiftUI | Remote supervision: push, approvals, questions, short commands, handoff |
| Android | Kotlin + Jetpack Compose | Same role, Android-native patterns |
| Browser | Existing web stack | Remote / temporary / fallback only |
| CLI | Go | Automation; unchanged by this plan |

**First desktop OS: Windows (decided 2026-07-23).** The Phase 2 vertical slice targets the Windows shell: WinUI 3 / WPF + WebView2, with the ambient panel realized as a tray flyout + top-center floating bar (`WS_EX_NOACTIVATE`, Mica/Acrylic). The shell architecture (panel + hosted workspace + daemon supervisor) is OS-neutral, so the macOS shell (including the notch panel) ports it later rather than being designed separately. **First mobile OS: iOS (decided 2026-07-23).** macOS-vs-Android ordering for Phase 4 remains a data decision.

### 7. Local application control (UE / Blender / browser) is a server-side capability

Letting agents operate Unreal Engine, Blender, or a browser is an **execution-plane** concern: the agent runs on (or reaches) the machine hosting those apps and controls them via their APIs (UE Remote Control, Blender Python, Playwright/CDP — MCP servers exist for all three). This works today with the existing browser client and requires **no native client work**. It proceeds as a parallel track. The native shell adds only the thin UI band on top: jump-back/window activation (cheap, Phase 2) and speculative overlays/UI Automation (Phase 6, data-gated). Apps without APIs are the only case forcing UI Automation, and none of the three named targets is in that case.

## Current-state gaps (grounding in this repository)

Facts the phases below must not assume away:

1. **Approvals and questions do not exist in the server-side domain.** They appear only inside the codex app-server protocol (`platform/agent/codexschema`) and the orchestrator. Nothing in `host/state`, `host/proto`, or `server/api` models them. The contract layer's centerpiece therefore requires new domain work — driver → state → proto → gateway — before any client can render an approval. This is Phase 0's largest item.
2. **Terminology is further along than "rename the client layer" suggests.** ARCHITECTURE.md already separates the `server/*` gateway (stateless proxy, "future native clients" explicitly in scope) from the internal `client/*` daemon layer. Phase 0's job is a docs-level terminology pass (runtime layers vs user-facing clients), not a restructure.
3. **Reconnect semantics partially exist**: ADR-0025 (transcript REST backfill → WS tail), ADR-0011 (two-step WS close), ADR-0022 (subscribe retry in socket layer). "Define reconnection and event replay" means inventorying and extending these, not starting blank. The current WS is surface-subscription + `viewUpdate` broadcast (ADR-0023); recorded-scenario replay implies an event-model extension whose size must be scoped in Phase 1, not discovered there.
4. **Wire types are hand-written and stdlib-only** (ADR-0021; AGENTS.md wire-format rule). Generating C#/Swift/Kotlin/TS clients from schemas is a deliberate supersession of ADR-0021 and must be recorded as such; generated Go (if any) stays stdlib-only.
5. **Auth is localhost bearer-token + WS ticket** (`server/api/auth.go`, `ticket.go`). There is no remote reachability and no push delivery. Both belong to the multi-host-gateway plan's territory (§ Phase R).

## Shared contracts and generated clients

```text
protocol/
├── openapi.yaml
├── events.schema.json
├── commands.schema.json
├── capabilities.schema.json
├── deep-links.schema.json
└── notifications.schema.json

contracts/
├── session-state-machine.md
├── approval-contract.md
├── question-contract.md
├── reconnect-contract.md
├── command-acknowledgement.md
├── notification-policy.md
├── handoff-contract.md
└── compatibility-policy.md
```

Generate or maintain typed clients for C#, Swift, Kotlin, TypeScript, and Go where useful — transport, typed messages, validation, and version negotiation; no presentation behavior. Supersedes ADR-0021 for cross-language clients (record a new ADR); the browser client may migrate incrementally.

## Development-cycle principles

1. Keep client-server boundaries typed and observable.
2. Provide deterministic fixtures and recorded event streams for every client; make client UIs runnable without live agents through a simulation server.
3. Support the fastest available edit-run loop on every platform — note this favors the hosted-SPA workspace (Vite HMR) over native workspace rebuilds.
4. Keep OS-specific functionality behind small platform services, not broad cross-platform abstractions; prefer official platform APIs over compatibility layers.
5. Capture screenshots, traces, logs, and event sequences as test artifacts.
6. Do not require feature parity where a platform-specific experience produces a better outcome.
7. Measure interaction latency and supervision steps, not implementation completeness.

## Delivery phases

### Phase 0: Platform model and the approval/question domain

- **Implement approvals and questions as server-side domain objects** (gap #1): surfacing from drivers through `state`/`proto` to the gateway, with expiry and conflict-resolution semantics (two clients answering the same request).
- Terminology pass separating runtime layers from user-facing clients (gap #2).
- Define capability negotiation and the compatibility policy skeleton.
- Decide authentication and trust boundaries **jointly with multi-host-gateway.md** — mobile approve = remote code-execution authorization, so this is safety-critical, not a checkbox.

Exit: a fake agent can raise an approval, two clients see it, one answers, both observe the resolution; reconnect returns authoritative state.

### Phase 1: Stabilize the protocol

- Typed session, command, event, approval, and question schemas; inventory existing REST/WS + reconnect ADRs (gap #3) and scope the event-replay extension explicitly.
- Deep links and handoff (input: remote-control-mobile-session-deep-link.md).
- Generate C#, Swift, Kotlin, TypeScript clients; ADR superseding ADR-0021 (gap #4).
- Simulation server + recorded scenarios.

Exit: the same recorded scenarios drive all SDKs; compatibility tests run in CI; clients depend on no undocumented behavior.

### Phase 2: Desktop app vertical slice (Windows)

Detailed design: [plan-20260723-windows-shell-design.md](./plan-20260723-windows-shell-design.md). Build the app on Windows:

1. personal-use deployment: local build + placement scripts, unpackaged; daemon runs in WSL (no Windows port); installer/signing/auto-update deferred;
2. daemon supervision: launch (via `wsl.exe`) / adopt, health, graceful restart; quitting the app leaves sessions running;
3. ambient panel (tray flyout + top bar): session states, approve/deny, question answering, jump-back to terminal/IDE/WSL;
4. Electron workspace app hosting the SPA in hosted mode, with the window discipline (reuse, restore, in-app deep-link resolution) enforced in its window registry;
5. native notifications → panel or window activation.

Exit: **the local flow never touches a browser**; a full approval round trip happens in the panel; notification-to-session navigation is reliable; the app quits and relaunches without losing sessions or window layout.

### Phase R: Remote reachability and push delivery (prerequisite for mobile)

Mobile supervision "away from the desk" requires infrastructure no other phase builds:

- a remote access path — this is multi-host-gateway.md's tunnel/relay design; adopt it rather than inventing a parallel one;
- push delivery (APNs/FCM): requires an always-on sender, Apple/Google developer accounts, and a resolution of the conflict with the gateway plan's "gateway holds no domain data / E2E" principle (candidates: host-originated pushes, opaque/encrypted payloads with client-side materialization, user-run relay). Decide who operates this and what it may learn.

Exit: a session event on a desk-bound host produces a push on a phone outside the LAN, with the privacy posture documented.

### Phase 3: iOS supervision vertical slice

Authentication + device registration (per Phase 0/R decisions), session summaries, push, approval handling, question response, short follow-up commands, desktop/browser handoff.

Exit: a user safely supervises a desk-hosted session away from the desk; reconnect and notification races are handled.

### Phase 4: Second platform by data

Either the macOS shell (reusing the shell architecture; notch panel; Apple core/protocol packages shared with iOS) or Android (Kotlin/Compose, same supervision contract), chosen from Phase 2–3 usage.

### Phase 5: Remaining platform

The one not chosen in Phase 4.

### Phase 6: Convergence and B-hypothesis experiments

Telemetry/observation decides: which views belong everywhere, which OS integrations reduce supervision cost, and whether B-class experiments (non-activating overlays over UE/Blender viewports, UI Automation, voice, widgets, Live Activities) earn investment. Each is capability-gated and stays out of authoritative server domain logic.

## Testing strategy

### Contract tests

Schema compatibility; state-machine transitions; command idempotency; event ordering and replay; approval expiry and two-client conflict resolution; version negotiation (bundled vs remote daemon skew).

### Shared client scenarios

New session starts; approval requested; question asked; disconnect/reconnect; two clients act on the same request; session completes while a client is suspended/quit; server restarts; unsupported capability received.

### Shell and app tests

- window discipline: reuse on re-activation, restore after restart, deep link → existing window;
- daemon lifecycle: adopt running daemon, graceful swap on update, quit-app-sessions-survive;
- panel: non-activating focus, all-Spaces presence, approve round trip;
- packaging: signed artifact installs and auto-updates.

### Platform integration tests

Windows notifications/tray/deep links/external-app activation; macOS notch panel/menu bar/Spaces/terminal activation; iOS push/background/notification actions/handoff; Android notification actions/process recreation/background restrictions; browser reconnect and remote access.

## Risks and mitigations

- **Approval/question domain underestimated** — it is new server-side work, not schema-writing; Phase 0 exit criterion forces it early.
- **Push/remote infra unowned** — Phase R exists precisely so Phase 3 cannot start on an unbuilt foundation; conflict with the E2E gateway principle is named and must be resolved on paper first.
- **Hosted workspace feels like "a website in a frame"** — hosted mode is a scoped work item (browser-idiom removal, window-model inversion, input routing, OS materials); exit review includes a design pass.
- **Panel becomes a tab factory** — window discipline is a contract (tested), not a convention.
- **Distribution fixed costs** — signing (Developer ID + notarization, Windows code signing), auto-update infra. Deferred entirely while the posture is personal use; re-opened as a scoped work item when distribution begins.
- **Excessive parallel scope** — one vertical slice with a measurable outcome before any new client; Phase 4/5 ordering is data-driven, not calendar-driven.
- **Protocol drift** — generated clients, recorded scenarios, compatibility CI, explicit version policy.
- **Sustained multi-stack maintenance** (C#/Swift/Kotlin + Go/TS) — mitigated by keeping native surface small (shell + panel), sharing the workspace SPA, and gating each new platform on usage.

## Success measures

Product (A-hypothesis measures):

- zero browser involvement in the local flow;
- fewer steps and lower latency from notification to intervention (time-to-approval, time-to-answer);
- fewer lost or forgotten sessions; higher unattended-work completion rate;
- successful remote-supervision rate once Phase 3 lands.

Measurement is initially self-observed and log-derived (single-operator tool); any telemetry beyond local logs is opt-in and decided explicitly in Phase 6.

Engineering:

- time from contract change to validated behavior on the desktop app;
- protocol compatibility pass rate; reconnect and replay reliability;
- crash-free session rate; percentage of client development executable against the simulator;
- OS integrations added without changing server semantics.

## Immediate next actions

1. ~~Record the first-OS decision in this document.~~ Decided: Windows desktop first, iOS mobile first (2026-07-23).
2. Phase 0: design and implement the approval/question server-side domain; write `approval-contract.md` / `question-contract.md` from it.
3. Inventory current REST/WS APIs and reconnect ADRs; scope the event-replay extension.
4. Reconcile auth/push with multi-host-gateway.md (joint decision note; resolve the E2E-vs-push conflict on paper).
5. Build the simulator and recorded scenarios.
6. Phase 2: panel-first vertical slice on Windows; hosted-mode SPA work in `clients/ui` behind a mode flag.
7. In parallel: connect UE/Blender/browser MCP servers against the existing stack to validate local-app-control value with zero client work.

## Decision summary

- Agent Grid is a server-backed platform; the desktop **app** is its primary local face: fully native resident shell (ambient panel, notifications, deep links, daemon supervision) + an on-demand workspace host for the existing SPA (VS Code model; Electron on Windows). The workspace is not rebuilt natively, and Electron never becomes the resident shell.
- The app is a client + daemon supervisor over the public contract — never a monolith, never a privileged client. Quitting it never kills sessions.
- One install bundles shell, daemon, and web assets; the headless server is a separate channel for remote hosts only.
- Window discipline: reuse, restore, in-app deep links; the browser exits the local flow and remains for remote/fallback.
- Product hypothesis A (supervision loop) governs priorities and measures; B-class OS-integration experiments are Phase 6, data-gated.
- Windows is the first desktop shell; iOS is the first mobile client. macOS-vs-Android ordering is decided from usage data.
- Mobile requires Phase R (remote path + push, reconciled with multi-host-gateway.md) before Phase 3.
- Contracts are shared and generated (superseding ADR-0021); approvals/questions are new server-side domain work and come first.
- Local app control (UE/Blender/browser) is server-side MCP capability, independent of any client rebuild.
