# Agent Grid Native Clients Plan

## Purpose

Evolve Agent Grid from a browser-centered control surface into a multi-client agent workspace whose server and protocol are the source of truth, while each operating system receives a client optimized for its native interaction model.

The goal is not to maximize shared UI code. The goal is to maximize the quality and speed of the product-development loop while preserving deep future OS integration.

## Desired outcomes

1. Agent Grid sessions, approvals, questions, commands, and events have one platform-independent contract.
2. Windows becomes the primary full-workspace client and the main environment for product experimentation.
3. iOS becomes the second-priority remote supervision client for notifications, approvals, questions, short interventions, and handoff.
4. Android provides the same mobile supervision role through a native Android experience.
5. macOS becomes a native desktop client with menu-bar, overlay, window, and terminal integration rather than a Windows or web port.
6. The browser remains a first-class client for remote, temporary, installation-free, and fallback access, but it is not treated as the canonical UI implementation.
7. Client implementations can diverge where platform conventions and OS capabilities differ, without diverging in domain behavior.

## Product model

Agent Grid should be modeled as a platform with several clients, not as a web application wrapped for desktop.

```text
Agent Grid Platform
├── Server / execution plane
│   ├── session ownership
│   ├── PTY and agent processes
│   ├── containers and worktrees
│   ├── orchestrator
│   ├── approvals and questions
│   └── event history
│
├── Protocol / domain contracts
│   ├── session state machine
│   ├── command model
│   ├── event model
│   ├── capability negotiation
│   ├── reconnect semantics
│   ├── notification policy
│   └── handoff and deep links
│
└── Clients
    ├── Windows
    ├── iOS
    ├── Android
    ├── macOS
    ├── Browser
    └── CLI
```

The server is the source of truth. No GUI client owns long-running sessions. Closing any client must not terminate agent work.

## Strategic decisions

### 1. Do not make the web UI the canonical client

The browser client remains useful, but a browser tab is a poor default home for a long-running supervision tool:

- it is mixed with unrelated tabs and windows;
- its lifetime is controlled by the browser rather than Agent Grid;
- it has weaker control over notifications, global shortcuts, window activation, taskbar or menu-bar presence, and session handoff;
- it encourages page-centric interaction instead of workspace- and operating-system-centric interaction.

The browser client should remain available for universal access, not define the limits of all other clients.

### 2. Do not optimize primarily for UI code reuse

AI-assisted implementation reduces the cost of producing platform-specific code, but it does not remove the cost of debugging abstraction leaks, bridge failures, lifecycle differences, or OS-specific workarounds.

Share behavior and contracts rather than forcing shared UI implementation.

### 3. Prefer native platform stacks

| Platform | Priority | Technology | Primary role |
|---|---:|---|---|
| Windows | 1 | C# + WinUI 3 + Windows App SDK | Main workspace, deep OS integration, product experimentation |
| iOS | 2 | Swift + SwiftUI | Remote supervision, notifications, approvals, questions, short commands |
| Android | 3 | Kotlin + Jetpack Compose | Remote supervision, notification actions, widgets, tablet/foldable expansion |
| macOS | 4 | SwiftUI + AppKit where needed | Desktop supervision, menu bar, overlays, terminal/app activation |
| Browser | continuous | Existing web stack | Remote, temporary, installation-free, and fallback access |
| CLI | continuous | Go | Automation, scripting, direct session control |

The ordering of Android and macOS may be revisited from actual usage data, but Windows and iOS remain fixed as priorities one and two.

### 4. Use embedded web surfaces selectively

Native clients may use WebView2 or WKWebView for capabilities where the web ecosystem is clearly stronger, such as terminal rendering, Markdown and Mermaid, complex diff views, generated HTML reports, and code or structured-data viewers.

The native application shell, navigation, approvals, notifications, windows, shortcuts, and OS integration remain native. The native client is the application; web views are contained implementation islands.

## Platform roles

### Windows client

Windows is the primary product surface and the main experimentation environment.

Initial responsibilities:

- session and project workspace;
- terminal, files, Markdown, diff, and orchestrator views;
- approvals and agent questions;
- native notifications and taskbar state;
- system-tray presence;
- global quick panel;
- deep links such as `agent-grid://session/...`;
- launching Windows Terminal, Explorer, editors, WSL, and related tools;
- multi-window and multi-monitor restoration;
- local server discovery and connection management.

Future OS integration candidates include non-activating overlays, Jump Lists, virtual-desktop-aware workspaces, Windows UI Automation, window capture, session-to-application associations, Widgets, richer notification actions, voice, clipboard, drag-and-drop, and local AI integration.

Use WinUI 3 as the default. Use WebView2 only for high-value web-rendered islands.

### iOS client

iOS is not a reduced Windows client. It is a focused remote supervision surface.

Initial responsibilities:

- running, waiting, failed, and completed session summaries;
- push notifications;
- approval allow/deny actions;
- responses to structured and free-form agent questions;
- short follow-up instructions;
- stop, pause, resume, and retry actions where supported;
- concise change and diff summaries;
- handoff to Windows, macOS, or browser clients;
- secure authentication and device registration.

Future candidates include Live Activities, notification actions, widgets, Siri and Shortcuts integration, richer iPad review, and continuity handoff.

### Android client

Android has the same product role as iOS but should use Android-native interaction patterns.

Initial responsibilities:

- session summaries;
- approval and question handling;
- short follow-up instructions;
- completion and failure notifications;
- concise diff summaries;
- stop, pause, resume, and retry actions;
- handoff to a desktop or browser client.

Future candidates include notification actions, foreground-service-backed connections where justified, home-screen widgets, Quick Settings tiles, tablet and foldable layouts, biometric confirmation, and Android App Links.

### macOS client

The macOS client should share Swift domain and networking code with iOS while providing a distinct desktop experience.

Initial responsibilities:

- full session workspace;
- menu-bar status and quick actions;
- native notifications;
- multiple windows;
- approval and question panels;
- terminal and editor activation;
- deep links and handoff.

Future candidates include non-activating `NSPanel` overlays, notch or top-of-screen ambient surfaces, Spaces-aware restoration, and precise terminal, editor, window, or tmux-session targeting.

Use SwiftUI for shared and standard views and AppKit bridges for menu-bar applications, special panels, focus behavior, and window activation. Prefer a native macOS target over Mac Catalyst when desktop-specific integration becomes important.

### Browser client

Retain and improve the browser client for remote hosts, temporary access, installation-restricted environments, mobile fallback, development and debugging, emergency access, and shareable web URLs where security policy permits.

The browser client may retain broad functionality, but new native client features are not required to be constrained by browser parity.

## Shared contracts and generated clients

Create a platform-independent contract layer before substantial native UI development.

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

Generate or maintain typed clients for C#, Swift, Kotlin, TypeScript, and Go where useful. Generated clients provide transport, typed messages, validation, and version compatibility, but no presentation behavior.

## Repository direction

```text
clients/
├── windows/
│   ├── AgentGrid.App/
│   ├── AgentGrid.Presentation/
│   ├── AgentGrid.Platform.Windows/
│   └── AgentGrid.WebIslands/
├── apple/
│   ├── Package.swift
│   ├── Sources/
│   │   ├── AgentGridCore/
│   │   ├── AgentGridProtocol/
│   │   └── AgentGridSharedUI/
│   ├── iOS/
│   └── macOS/
├── android/
│   ├── app/
│   ├── core/
│   ├── protocol/
│   └── feature/
├── web/
└── cli/

protocol/
contracts/
server/
orchestrator/
```

This is directional rather than an immediate migration requirement. Existing boundaries should be reviewed before files move.

## Development-cycle principles

The optimization target is the time from idea to validated behavior.

1. Keep client-server boundaries typed and observable.
2. Provide deterministic fixtures and recorded event streams for every client.
3. Make client UIs runnable without live agents through a simulation server.
4. Support the fastest available edit-run loop on every platform.
5. Keep OS-specific functionality behind small platform services, not broad cross-platform abstractions.
6. Prefer official platform APIs and samples over generic compatibility layers.
7. Capture screenshots, traces, logs, and event sequences as test artifacts.
8. Allow clients to evolve in parallel against the same contracts.
9. Do not require feature parity where a platform-specific experience produces a better outcome.
10. Measure interaction latency and supervision steps, not only implementation completeness.

## Delivery phases

### Phase 0: Validate the platform model

- clarify or rename any internal `client` layer that conflicts with user-facing clients;
- document server ownership and client responsibilities;
- define capability negotiation;
- decide authentication and trust boundaries;
- establish compatibility rules.

Exit when clients can reconnect to authoritative state, negotiate supported capabilities, and terminology clearly separates runtime layers from user-facing clients.

### Phase 1: Stabilize the protocol

- define typed session, command, event, approval, and question schemas;
- define reconnection and event replay;
- define deep links and handoff;
- generate C#, Swift, Kotlin, and TypeScript clients;
- create a simulator and recorded scenarios.

Exit when the same recorded scenarios drive all SDKs, compatibility tests run in CI, and clients do not depend on undocumented behavior.

### Phase 2: Windows native vertical slice

Build one complete path:

1. connect to a server;
2. display session states;
3. open one session;
4. receive an approval request;
5. approve or deny it;
6. receive completion;
7. open the related terminal, files, or diff;
8. restore the workspace after restart.

Include a WinUI 3 shell, tray and notifications, a global quick panel, deep-link handling, and one WebView2 island if required.

Exit when the client is meaningfully better than a dedicated browser window, closing it does not affect sessions, notification-to-session navigation is reliable, and a full approval round trip works without the browser.

### Phase 3: iOS supervision vertical slice

Implement authentication, device registration, session summary, push notification, approval handling, question response, short follow-up commands, and desktop/browser handoff.

Exit when a user can safely supervise a Windows-hosted session away from the desk and reconnect or notification races are handled correctly.

### Phase 4: Android supervision vertical slice

Implement the mobile supervision contract using Kotlin and Jetpack Compose while respecting Android lifecycle and notification behavior.

Exit when the same supervision outcomes are available as on iOS without relying on Apple-specific assumptions, and background behavior remains reliable under Android restrictions.

### Phase 5: macOS desktop client

Reuse Apple core and protocol packages while adding a full workspace, menu bar, multiple windows, terminal/editor activation, approval panels, and an optional ambient-overlay prototype.

Exit when the macOS client provides desktop-native value beyond the browser and platform specialization is not constrained by shared iOS code.

### Phase 6: Convergence and product learning

Use telemetry and observation to decide which views belong everywhere, which OS integrations reduce supervision cost, whether Android or macOS receives the next investment, which web surfaces remain useful, and whether iPad-optimized or Linux desktop clients are justified.

## Testing strategy

### Contract tests

- schema compatibility;
- state-machine transitions;
- command idempotency;
- event ordering and replay;
- approval expiry and conflict resolution;
- version negotiation.

### Shared client scenarios

- a new session starts;
- a session requests approval;
- a session asks a question;
- the network disconnects and reconnects;
- two clients act on the same request;
- a session completes while a client is suspended;
- the server restarts;
- the client receives an unsupported capability.

### Platform integration tests

- Windows notifications, tray, deep links, restoration, and external-app activation;
- iOS push, background transitions, notification actions, and handoff;
- Android notification actions, process recreation, background restrictions, and App Links;
- macOS menu bar, panel focus, Spaces, notifications, and terminal activation;
- browser reconnect and remote access.

## Risks and mitigations

- **Excessive parallel scope:** require a vertical slice and measurable outcome before expanding a client.
- **Protocol drift:** use generated clients, recorded scenarios, compatibility CI, and explicit version policy.
- **Duplicated UI effort:** accept intentional duplication; share semantics, design tokens, assets, copy, and fixtures.
- **Inconsistent behavior:** make domain transitions server-authoritative and test clients against the same scenarios.
- **Web islands become a hidden second app:** restrict them to isolated rendering capabilities with typed bridges and native navigation ownership.
- **OS experiments destabilize the core:** capability-gate integrations and keep them outside authoritative server domain logic.

## Success measures

Product measures:

- fewer steps from notification to intervention;
- lower time-to-approval and time-to-answer;
- fewer lost or forgotten sessions;
- higher successful remote-supervision rate;
- less dependence on opening a generic browser to find Agent Grid;
- higher unattended-work completion rate.

Engineering measures:

- time from contract change to validated behavior on the priority client;
- protocol compatibility pass rate;
- reconnect and replay reliability;
- client crash-free session rate;
- OS integrations added without changing server semantics;
- percentage of client development executable against deterministic simulations.

## Immediate next actions

1. Review terminology and rename the internal `client` layer if it conflicts with the new model.
2. Inventory current REST and WebSocket APIs and undocumented browser dependencies.
3. Define the session state machine, approval contract, reconnect contract, and capability model.
4. Create recorded scenarios and a lightweight simulation server.
5. Generate the first C# client and build the Windows vertical slice.
6. Validate that Windows provides value beyond a dedicated browser window.
7. Generate Swift and implement the iOS supervision vertical slice.
8. Follow with Kotlin/Compose Android implementation, then macOS specialization.

## Decision summary

- Agent Grid is a server-backed platform with multiple first-class clients.
- The browser is retained but is not the canonical UI implementation.
- Windows is the primary client and uses C# with WinUI 3.
- iOS is the second-priority client and uses Swift with SwiftUI.
- Android uses Kotlin with Jetpack Compose.
- macOS uses SwiftUI with AppKit bridges where necessary.
- Web technologies are embedded selectively where they provide clear rendering advantages.
- Protocols, state transitions, capabilities, and interaction contracts are shared; UI implementations are platform-native.
- Development-cycle speed and future OS integration take precedence over distribution size and maximum UI code reuse.
