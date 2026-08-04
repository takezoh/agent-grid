# Cognitive Work Orchestration UX Plan

Status: Draft v2 — revised after value-hypothesis review (2026-08-04). Adds value hypothesis and falsification criteria (§1.1–1.4), the Classic ⇄ Cognitive dual-workspace model (§2.1), and a value-density-ordered MVP with a Phase 0 probe (§26–27).

## 1. Purpose

The primary purpose of this initiative is to reduce the cognitive load of a user who has many responsibilities, many roles, and several development efforts running in parallel as part of their main work.

The system must not merely make agents more capable. It must reduce the amount of work state the user has to remember, reconstruct, supervise, and manually coordinate.

The intended outcome is:

> The user can delegate, suspend, resume, review, and redirect work across multiple projects without continuously holding every project's context, unresolved decisions, dependencies, agent sessions, and next actions in their head.

The product should continuously organize:

- what the user is responsible for,
- what outcomes are currently active,
- what is progressing without user involvement,
- what is blocked,
- what requires a decision,
- what changed while the user was away,
- what should receive attention next,
- and what can safely remain hidden.

The system should make the user think only about decisions that genuinely require the user's judgment.

### 1.1 Value hypothesis

The initiative rests on one falsifiable hypothesis:

> A trustworthy, automatically maintained intermediate representation of work state (outcomes, decisions, resume capsules, verified completions) reduces the user's supervision and context-reconstruction time more than it costs to maintain and to verify.

The hypothesis must be validated against the cheap baseline the user already operates — disciplined markdown plans, resume notes, dev-skills workflows, and manual session management — not against "no system". Only the marginal value over that baseline justifies the investment.

### 1.2 Known value risks

1. **Trust-chain fragility.** Value flows through: raw work → LLM extraction → structured state → compressed surfaces → user decisions. Every link can inject error, and the failure mode is near-total rather than gradual: one materially wrong resume capsule or one bad accepted recommendation pushes the user back to reading raw logs, at which point the system is pure overhead. Mitigations are therefore first-class requirements, not polish: provenance on every record (§29.5), freshness/staleness display (§12.1), evidence-cited recommendations (§3.6), and a one-click fallback to the Classic Workspace (§2.1) so that reverting is a mode switch, not product abandonment.
2. **Baseline competition.** Each surface must beat the markdown-and-discipline baseline on measurable time saved. Surfaces that cannot demonstrate marginal value are cut (§26.4).
3. **Commoditization of execution machinery.** Runtime vendors are climbing this stack; background execution, notifications, and completion summaries are likely to be absorbed into commercial runtimes. The durable asset is the user-specific cognitive state — outcomes, decisions, constraints, attention history, provenance — not the orchestration machinery.

### 1.3 Investment policy: thin durable layer

- **Durable (build carefully, own the schema):** the cognitive/outcome data model, decision and constraint records with provenance, resume capsules, attention and completion history.
- **Disposable or borrowed (build thin, expect replacement):** execution supervision, runtime adapters, notification plumbing, event compression. Reuse existing Agent Grid machinery and runtime features wherever possible.
- **Budget follows value density:** resume continuity and trustworthy completion first; decision/attention management second; role framing, capture inference, and portfolio views last (§26.4 cut candidates).

### 1.4 Falsification criteria

Defined before implementation and evaluated after roughly two weeks of routine use per phase. The thresholds below were adopted as initial values by the user on 2026-08-04; they may be recalibrated once, explicitly, before a phase starts — never retroactively.

- **Resume:** if more than 30% of resumes still require opening raw session history, the resume-capsule hypothesis fails.
- **Capsule trust:** if more than ~10% of resume capsules are found materially wrong, trust is considered broken; capsule generation must be fixed before any further surface is built.
- **Completion trust:** if more than 30% of ordinary successful work still requires opening raw execution detail to confirm completion, the completion-report hypothesis fails.
- **Maintenance cost:** if keeping the structured state accurate requires recurring manual user input (weekly self-check), Phase 1 fails — state freshness must be automatic.

A failed criterion stops expansion: fix the failing layer or stop the initiative. Do not proceed to the next phase on top of a failed criterion.

## 2. Product definition

Agent Grid should evolve from an agent/session-oriented interface into a cognitive workspace for a multi-responsibility user.

The product is not primarily:

- an agent launcher,
- a prompt editor,
- a multi-agent visualization,
- a task tracker,
- a chat history browser,
- or a model router.

It is primarily:

> An attention, decision, continuity, and delegation interface that uses agents as interchangeable workers behind the scenes.

A useful framing is:

> A single-user cognitive system whose workers happen to be agents.

The user-facing hierarchy should therefore be:

1. Focus
2. Decisions
3. Outcomes
4. Commitments
5. Workstreams
6. Deliverables
7. Executions
8. Runtime threads and turns

Agents, models, prompts, thread IDs, token usage, and tool calls should normally appear only as expandable implementation details.

### 2.1 Dual workspace: Classic ⇄ Cognitive

Agent Grid keeps two coexisting top-level workspaces over the same backend:

- **Classic Workspace** — the current session/agent-oriented UI (sessions, terminal, files, diff, agent activity). It remains fully functional and is not degraded or rewritten by this initiative.
- **Cognitive Workspace** — the attention/decision/outcome surface described in this plan.

A persistent global switcher toggles between them with one action, and the selection is persisted per client.

Requirements:

- Both workspaces read the same underlying state. Cognitive entities (outcomes, decisions, capsules) are an additive layer; nothing captured in the Cognitive Workspace becomes invisible from Classic, and sessions remain first-class in Classic regardless of whether an outcome references them.
- Cross-navigation deep links in both directions: an Execution in the Cognitive Workspace opens its live session in Classic; a session in Classic shows a link to its outcome when one exists.
- The Classic Workspace is the guaranteed fallback. This is a deliberate mitigation of the trust-chain risk (§1.2): if the cognitive layer misleads, reverting costs one click, not the product.
- Time and task share spent in Classic versus Cognitive is itself a primary adoption and trust signal (§30). The Cognitive Workspace must earn usage; it is never forced.

## 3. Core UX principles

### 3.1 Reduce attention, not merely effort

The system succeeds when it reduces the number of things the user must remember, inspect, reconcile, and explicitly manage.

Executing more work is not sufficient if the user must then supervise more sessions, read more logs, or manually integrate more results.

### 3.2 Show decisions, not raw state

The default surface should answer:

- What requires my attention?
- Why does it require attention now?
- What is the recommended decision?
- What happens if I accept it?
- What can continue without me?

Raw execution state should be secondary.

### 3.3 Clarify by proposing, not interrogating

When user input is incomplete, the system should form a reasonable interpretation from existing context and present it as a reversible proposal.

Instead of asking several setup questions, it should say:

> I interpreted this as an update to the Windows host support outcome. I will revise the plan, preserve the current systemd exclusion, and add user-scope supervision constraints.

The user should only be interrupted when ambiguity materially changes scope, risk, external effects, cost, permissions, or irreversible decisions.

### 3.4 Preserve continuity across interruption

Every active workstream must be resumable without requiring the user to reread the full conversation, logs, or previous implementation history.

The system should maintain a compact, continuously updated resume state.

### 3.5 Hide resolved operational noise

The user should normally not see:

- transient command failures,
- automatic retries,
- discarded exploration branches,
- resolved merge conflicts,
- internal subagent coordination,
- intermediate summaries,
- or tool-level execution detail.

These should remain available for inspection, but not compete for attention.

### 3.6 Every decision request should include a recommendation

The system should not merely ask the user to choose among options. It should provide:

- a recommended option,
- the reason for the recommendation,
- trade-offs,
- reversibility,
- affected outcomes,
- and the consequence of delaying the decision.

A recommendation must cite the evidence it rests on (tests, documents, prior decisions, execution results). When the evidence basis is weak, the system must say so and present the options without a recommendation rather than fabricate one. The UI must not visually pressure acceptance: `Accept` is a primary action only when the recommendation is evidence-backed and reversible. Accepted recommendations that are later overturned are tracked as a trust metric (§30).

### 3.7 Automation requires strong undo and traceability

The system should be proactive, but every automatic interpretation, task creation, scope update, and delegation action must be inspectable and reversible.

## 4. User cognitive load model

The UX should explicitly target the following sources of cognitive load.

### 4.1 Context reconstruction

The user should not need to remember:

- what was previously decided,
- which repository or plan a short instruction refers to,
- why an implementation exists,
- which constraints still apply,
- or what changed since the last session.

### 4.2 Role switching

The same user may act as:

- product owner,
- engineering manager,
- architect,
- implementer,
- reviewer,
- operator,
- researcher,
- or stakeholder.

The system should identify the relevant role for each decision and frame information accordingly.

### 4.3 Parallel work tracking

The user should not need to remember which projects are:

- active,
- autonomous,
- waiting,
- blocked,
- at risk,
- or ready for review.

### 4.4 Agent supervision

The user should not have to manage individual agent sessions under normal conditions.

The system should consolidate progress, failures, evidence, and review results into outcome-oriented state.

### 4.5 Completion verification

The user should not need to infer whether work is truly complete from a long agent response.

Completion should be expressed through explicit deliverables, evidence, verification results, unresolved items, and required user attention.

## 5. Information architecture

The top level is the global workspace switcher (§2.1):

```text
[Cognitive ⇄ Classic]

Cognitive Workspace
├─ Home
│  ├─ Focus
│  ├─ Decisions
│  ├─ Workstreams
│  ├─ Inbox
│  └─ Activity
└─ Projects
   ├─ Project overview
   ├─ Outcomes
   ├─ Decisions
   ├─ Knowledge
   └─ Executions

Classic Workspace (existing session-oriented UI, unchanged)
├─ Sessions / Agent activity
├─ Files
├─ Diff
├─ Terminal
└─ Documents
```

The Classic Workspace keeps its current internal navigation; this plan does not restructure it. Cross-links between the two are defined in §2.1.

The primary daily surfaces should be limited to:

- Home
- Outcome Detail
- Decision Detail
- Resume View
- Execution Detail

Execution Detail is intentionally secondary.

## 6. Home UX

Home is not a conventional dashboard. It is the user's attention allocation surface.

It should answer within approximately 30 seconds:

- What requires my judgment?
- What is progressing automatically?
- What can I resume?
- What is blocked?
- What completed without needing review?

### 6.1 Recommended layout

```text
┌─────────────────────────────────────────────────────────┐
│ Agent Grid                            Tue, Aug 4          │
│ Role: All roles ▼     Scope: All projects ▼             │
├─────────────────────────────────────────────────────────┤
│ You have 3 decisions and 2 blocked items.               │
│ [Continue highest-priority work]                        │
├──────────────────────────────┬──────────────────────────┤
│ NEEDS YOUR ATTENTION         │ IN PROGRESS              │
│                              │                          │
│ Decide Windows MVP scope     │ context-fabric plan      │
│ agent-grid · Product Owner   │ Planning · no action     │
│ Blocking 2 work items        │                          │
│                              │ Air Sync Drive research  │
│ Review collision fix         │ Investigating            │
│ Main project · Reviewer      │                          │
├──────────────────────────────┴──────────────────────────┤
│ READY TO RESUME                                         │
│ agent-grid / Windows supervisor                         │
│ Next: verify user-scope service                         │
│ [Resume]                                                │
├─────────────────────────────────────────────────────────┤
│ RECENTLY COMPLETED                                      │
│ 4 items completed and verified · [View summary]         │
└─────────────────────────────────────────────────────────┘
```

The header also carries the global workspace switcher (§2.1).

### 6.2 Needs Your Attention

This section should include only items that genuinely require user involvement:

- scope decisions,
- approval,
- priority conflicts,
- production or external impact,
- destructive operations,
- unresolved architectural choices,
- material risk,
- or failures that automatic recovery could not resolve.

It should not contain routine progress or completion notifications.

Each card should include:

- the decision or action required,
- the relevant project and role,
- why it matters now,
- what is blocked,
- the recommended action,
- and the estimated attention type.

Example:

```text
Windows host MVP scope

Recommendation:
Include Windows-side supervision, but exclude pre-login startup.

Why now:
Two implementation work items depend on this boundary.

[Accept recommendation] [Review alternatives] [Defer]
```

### 6.3 In Progress

This section should summarize autonomous work without exposing implementation noise.

Each item should show:

- outcome,
- current phase,
- whether action is required,
- and the next meaningful checkpoint.

Avoid artificial numeric progress percentages unless there is an objective basis.

Use a small set of semantic phases:

- Understanding
- Planning
- Executing
- Verifying
- Waiting
- Needs decision
- Completed

### 6.4 Ready to Resume

This section is central to cognitive load reduction.

Each card should include:

- the outcome,
- where work stopped,
- the last important decision,
- the next action,
- unresolved issues,
- and whether user attention is needed.

The Resume action should open a reconstructed resume view, not merely the original chat transcript.

### 6.5 Recently Completed

Completed work should remain visible temporarily so that it does not disappear unexpectedly.

Items should indicate:

- completed and verified,
- completed but review recommended,
- or completed with unresolved follow-up.

## 7. Global Capture UX

A single global input should always be available.

The user should not need to select a project, task type, role, repository, or agent before entering a thought or instruction.

Example input:

> Change the Windows support direction so that Windows supervises the WSL process.

The system should infer:

```yaml
project: agent-grid
outcome: Windows host support
input_type: direction_change
role: architect
priority: normal
actions:
  - update current decision
  - identify affected plans
  - create follow-up work items
```

After capture, the system should display a compact interpretation:

```text
Proposed: direction change for agent-grid / Windows host support.
Nothing has been applied yet.

Would change:
- Windows-side supervisor adopted
- WSL systemd remains out of scope

Affected:
- Windows host plan
- startup and recovery design
- service permission research

[Confirm and apply] [Adjust] [Discard]
```

The system should proceed automatically only when the action is low risk, reversible, **and does not modify a recorded decision or outcome scope** — for example, filing a note, attaching context, or creating a work item inside an already-approved scope. A direction change such as the example above modifies a recorded decision, so it is staged as a pending proposal with one-click confirmation rather than applied silently. This keeps capture behavior consistent with the mandatory confirmation categories in §8 and the scope safeguard in §29.3.

## 8. Intake and interpretation flow

The intake pipeline should be:

```text
User input
→ identify relevant project, outcome, role, and prior decisions
→ generate a proposed interpretation
→ detect material ambiguity and risk
→ create or update work state
→ delegate when appropriate
→ expose Undo and Review interpretation
```

Questions should be reserved for cases where the system cannot safely choose among materially different interpretations.

Mandatory confirmation categories should include:

- external messages or publication,
- destructive actions,
- production changes,
- spending or billing,
- elevated permissions,
- significant scope expansion,
- or irreversible decisions.

### 8.1 Undo scope

Undo of an intake interpretation is transactional over the state records it created or modified — decisions, constraints, work items, outcome updates are reverted as a unit, with the reverted versions retained in history.

Undo does not retroactively unwind executions. If delegated work has already started, undo stops the execution, preserves its findings as evidence, and reverts the state records; workspace changes made by the execution are handled through the normal review/discard path, not by the intake undo.

In the MVP, automatic delegation is deferred until an interpretation survives its confirmation window, so intake undo normally happens before any execution starts.

## 9. Outcome Detail UX

Outcome Detail is the main work-centered view.

It should foreground:

- purpose,
- success criteria,
- current state,
- next action,
- pending decisions,
- and workstreams.

Example structure:

```text
Windows hostを実運用可能にする
Active · High priority · Architect / Implementer

OUTCOME
Windows上でAgent Grid Serverを安定稼働させ、障害時に
ユーザー操作なしで復旧できるようにする。

DONE WHEN
✓ Windows restart startup path defined
○ WSL termination recovery verified
○ User-scope operation verified
○ Failure cause visible in logs

CURRENT STATE
Windows-side supervisor selected.
Plan update in progress.

NEXT
Verify user-scope service constraints.

DECISIONS
✓ Use Windows-side supervision
✓ Do not use WSL systemd
? Include pre-login startup in MVP

WORKSTREAMS
Architecture        Ready for review
Service prototype   In progress
Recovery tests      Not started
```

The primary actions should be:

- Continue
- Ask about this outcome
- Review decision
- Open details

## 10. Continue action

Continue should be the highest-value action in the product.

It must not simply reopen a chat.

The system should evaluate current state and choose the next valid operation.

```text
Continue
→ inspect pending decisions and dependencies
→ if no user decision is required, start or resume the next work item
→ if a decision is required, open Decision Detail
→ if execution completed, open Completion Summary
```

This removes the requirement for the user to formulate the next prompt.

## 11. Decision Detail UX

Decision Detail should reduce decision-making cost.

Required structure:

1. Decision question
2. Recommended choice
3. Why the decision is needed now
4. Options
5. Trade-offs
6. Reversibility
7. Affected outcomes and workstreams
8. Consequences of deferral
9. Decision actions

Example:

```text
Should pre-login startup be included in the MVP?

Recommendation: Exclude it.

Reason:
The current outcome is stable personal development operation.
Pre-login startup substantially increases Windows service,
identity, and WSL session complexity without improving the core MVP.

A. Exclude from MVP — recommended
   + simpler user-scope operation
   + smaller verification surface
   - starts only after login
   Reversible: Yes

B. Include in MVP
   + supports unattended machine startup
   - increases permission and session complexity
   - expands implementation and test scope
   Reversible: Yes, but costly

Affected:
- Windows host plan
- supervisor implementation
- recovery tests

[Exclude from MVP] [Include] [Discuss]
```

After a decision, the system should automatically update:

- outcome scope,
- active constraints,
- affected work items,
- acceptance criteria,
- plans and documentation where appropriate,
- and the resume capsule.

## 12. Resume View

Resume View should reconstruct the work state for the user.

It should include:

- what the user was working on,
- the underlying goal,
- where work stopped,
- what changed since then,
- the current issue,
- the recommended next action,
- and the amount or type of attention required.

Example:

```text
Welcome back

You were working on:
agent-grid / Windows host support

Goal:
Run Agent Grid Server reliably on Windows.

Where you left off:
Windows-side supervision was selected.
WSL systemd was explicitly rejected.

Since then:
- the plan was updated
- user-scope constraints were investigated
- two implementation options were compared

Current issue:
Choose Task Scheduler or a user-session supervisor for the MVP.

Recommended next step:
Review one decision.

[Review decision] [Let agent continue] [Open full history]
```

### 12.1 Resume Capsule data

Each active outcome should maintain a compact resume capsule:

```yaml
resume:
  objective:
  current_state:
  last_decision:
  changes_since_last_attention:
  next_action:
  unresolved:
  blockers:
  user_attention_needed:
  updated_at:
```

The resume capsule should be continuously regenerated from structured state, not manually maintained as an independent source of truth.

Trust requirements (load-bearing for the value hypothesis, §1.2):

- Each capsule field carries a reference to the structured state or source event it was derived from, so a suspicious capsule can be verified without rereading history.
- Capsules and outcome states surface their freshness prominently. A capsule older than its outcome's latest activity is displayed as **stale**, never presented as current.
- A materially wrong capsule is treated as a product defect and tracked (§30), not accepted as noise. The user can flag a capsule as wrong in one action; flags feed the capsule-trust falsification criterion (§1.4).

### 12.2 Generation cost and consolidation with session summaries

Capsule generation costs one LLM call per settled turn. The existing session-card summary (`summarize_command`) is a second call per turn, over nearly the same input, answering nearly the same question — "what work is the user driving in this session". Running both duplicates cost and, more importantly, duplicates judgement: the card title and the capsule can disagree, and a visible disagreement pollutes the capsule-trust measurement (§1.4).

Decided (2026-08-04): consolidate on the capsule call.

- The capsule carries a short card title (~25 characters) produced in the same call as the resume and completion fields, so consolidation adds no generation cost — the same technique used for completion reports (§18).
- Session-card title resolution prefers a fresh capsule title and falls back to the existing summary field. The dependency is one-directional: the capsule layer is preferred, never depended on. Removing capsules after a failed §1.4 evaluation restores current behaviour by deleting the preference, which keeps the probe disposable (§1.3).
- The per-turn summary job is suppressed only for agent sessions whose capsule generation is healthy. Generic terminal sessions, which expose no conversation turns, keep using the summary command unchanged.
- Accepted trade-off: the card title updates when the turn settles rather than at prompt submission, so the previous title remains visible while a turn runs. The card title exists to identify a session, not to acknowledge input.

This is scheduled before the Phase 0 measurement window, not after: running the probe with duplicated generation would pay double cost for two weeks and would contaminate the capsule-error rate with card/capsule disagreements.

Cost controls that remain in force regardless of consolidation: digest-gated regeneration (unchanged history is never regenerated), no generation while the agent is running, prompt input capped independently of history length, error backoff, and opt-in configuration. If measured generation volume during Phase 0 is still too high, the next lever is lazy generation — regenerate when a capsule is viewed rather than on every settled turn — before any reduction of capsule content.

## 13. Workstream UX

Workstreams should group execution by purpose rather than by agent or session.

Example:

```text
Windows host support

Architecture
  ✓ Requirements consolidated
  ✓ Supervisor direction selected
  ○ Plan review

Implementation
  ● Service prototype
  ○ Restart recovery
  ○ Logging

Verification
  ○ WSL termination test
  ○ Windows restart test
  ○ Permission test
```

Use a compact status set:

- Ready
- In progress
- Waiting
- Needs attention
- Verifying
- Done
- Stopped

Avoid recreating a highly granular issue tracker state machine.

## 14. Role-aware presentation

The system should infer and display the role from which the user is being asked to act.

Example:

```text
You are being asked as: Product Owner
Decision type: MVP scope
```

Information should be framed according to role.

### Product Owner

Emphasize:

- user value,
- scope,
- priority,
- timeline,
- and dependency impact.

### Architect

Emphasize:

- constraints,
- alternatives,
- reversibility,
- technical debt,
- and system-wide impact.

### Reviewer

Emphasize:

- changed behavior,
- regression risk,
- evidence,
- acceptance criteria,
- and unresolved issues.

### Operator

Emphasize:

- current health,
- failure mode,
- recovery,
- observability,
- and runbook implications.

The user should not be required to manually switch roles before every interaction. Manual correction should be possible.

## 15. Cross-project and portfolio UX

The portfolio view should organize commitments by attention state rather than merely list projects.

Recommended groups:

- High attention
- Autonomous
- Waiting
- At risk
- Ready to resume
- Recently completed

Example:

```text
High attention
- Main project release quality
- agent-grid Windows support

Autonomous
- context-fabric planning
- Air Sync provider research

Waiting
- External PR review
- Microsoft 365 configuration

At risk
- UE collision regression
  Release dependency · unresolved for 2 days
```

### 15.1 Attention conflict detection

The system should detect conflicts where multiple outcomes require attention within the same time window.

Example:

```text
Attention conflict detected

Both require review today:
- Main project collision fix
- agent-grid architecture plan

Recommendation:
Review the collision fix first because it blocks the release.
Move the architecture review to tomorrow.

[Accept] [Change priority]
```

The initial implementation may rely on explicit priority, deadlines, dependency status, and user-marked importance rather than attempting complex autonomous scheduling.

## 16. Execution Detail UX

Agent execution should be expandable detail.

Default representation:

```text
Service prototype
In progress · No action required
```

Expanded representation:

```text
Runtime: Codex App Server
Workspace: agent-grid
Execution profile: implementation-standard

Current activity:
Reviewing existing Windows host startup code

Recent meaningful events:
- located startup entry point
- identified current shutdown handling
- preparing implementation patch

[Open live session] [Pause] [Stop]
```

`Open live session` switches to the Classic Workspace session view (§2.1).

Advanced details may include:

- model,
- reasoning effort,
- token usage,
- thread and turn IDs,
- commands,
- tool calls,
- raw transcript,
- environment,
- sandbox and approval policy.

These must not dominate ordinary workflows.

## 17. Notification and interruption policy

The system should classify events into four attention levels.

### 17.1 Immediate interruption

Use only for:

- imminent data loss,
- production or security impact,
- explicit approval requirement,
- urgent deadline,
- or a decision blocking critical work.

### 17.2 Attention Queue

Use for:

- ordinary architectural decisions,
- priority choices,
- medium-risk changes,
- external dependency changes,
- or decisions that have a safe deferral window.

### 17.3 Batched summary

Use for:

- successful completion,
- passing tests,
- documentation changes,
- minor implementation decisions,
- and automatically resolved failures.

### 17.4 Hidden by default

Hide:

- transient tool failures,
- automatic retries,
- subagent coordination,
- discarded approaches,
- duplicate research results,
- and resolved conflicts.

## 18. Completion UX

Completion reports should be compressed into four sections:

1. Changed
2. Verified
3. Remaining
4. Your attention

Example:

```text
Completed: Windows host plan update

Changed
- added Windows-side supervision direction
- documented systemd exclusion
- added user-scope constraints

Verified
- consistent with existing decisions
- required sections present
- commit created

Remaining
- choose final service mechanism
- implement recovery tests

Your attention
One decision remains
```

If fully verified and no review is needed:

```text
Completed and verified
No action required
```

Detailed diffs, test logs, agent messages, and evidence should remain expandable.

## 19. Chat UX

Chat should remain available but should not be the only durable representation of work.

Chat is appropriate for:

- consultation,
- ambiguous input,
- direction changes,
- feedback,
- and exceptions.

Important information from chat should be extracted into structured state:

```text
Chat message
→ Decision, constraint, outcome update, or work item
→ Persistent work state
```

Example:

> Distribution does not need to be considered in this phase.

Should become:

```yaml
constraint:
  scope: agent-grid/windows-host
  statement: Distribution is out of scope for the current phase.
  source: user
  status: active
```

The system must preserve a trace back to the source conversation.

## 20. Terminology

User-facing terminology should describe work rather than orchestration internals.

| Internal concept | User-facing wording |
|---|---|
| Task | Work item |
| Agent thread | Work session |
| Prompt | Instructions |
| Context | Background |
| Escalation | Needs your attention |
| Human-in-the-loop | Your decision is required |
| Artifact | Deliverable |
| Acceptance criteria | Done when |
| Agent failure | Could not complete automatically |
| Orchestration | Coordination |
| Subagent | Worker, or hidden |

Avoid phrases such as "spawn agent" in ordinary UX. Prefer "run the investigation in parallel" or "delegate implementation."

## 21. Conceptual architecture

```text
Cognitive Control Plane
├─ user roles
├─ responsibilities
├─ active commitments
├─ attention queue
├─ pending decisions
├─ interruption and resume state
└─ cross-project priorities
          │
          ▼
Outcome Control Plane
├─ outcomes
├─ workstreams
├─ dependencies
├─ deliverables
├─ acceptance criteria
└─ completion state
          │
          ▼
Execution Control Plane
├─ task decomposition
├─ skill selection
├─ runtime selection
├─ model profile selection
├─ permission policy
└─ execution supervision
          │
          ▼
Runtime Adapters
├─ Codex App Server
├─ Claude or other host agents
├─ local models
└─ future runtimes
```

The internal domain model must not depend directly on Codex thread and turn concepts.

Placement hypothesis in the existing Agent Grid architecture (to be validated during design):

- The Cognitive and Outcome control planes are a new domain layered above the existing gateway (`services/server`), persisting cognitive state separately from pty/session state. Per the thin-durable-layer policy (§1.3), this data model is the durable asset.
- The Execution Control Plane starts as a thin coordinator that reuses the existing session daemon and `claude-app-server` (the Codex app-server shim) as its first runtime adapters. No new orchestration machinery is built where existing machinery or runtime-vendor features suffice.
- `clients/ui` hosts both workspaces behind the global switcher (§2.1). The existing session-oriented surfaces are not rewritten.

Recommended internal concepts:

- Outcome
- Decision
- Workstream
- WorkItem
- Deliverable
- Execution
- Evidence
- Review
- Completion
- ResumeCapsule
- AttentionItem

Codex thread, turn, and item identifiers should be stored by a runtime adapter.

## 22. Relationship to dev-skills

The system should not replace dev-skills.

Responsibilities should be separated as follows.

### dev-skills

Defines reusable execution knowledge:

- how to investigate a repository,
- how to create a plan,
- how to implement and verify a change,
- how to review a pull request,
- how to diagnose an Unreal Engine issue,
- or how to update documentation.

### Cognitive and outcome orchestration

Defines:

- why the work exists,
- what outcome it supports,
- which constraints apply,
- what should happen next,
- what the user must decide,
- and whether completion is trustworthy.

### Runtime

Executes work through Codex App Server or another host.

The initial system should improve dev-skills and orchestration in parallel, but should avoid encoding dynamic project state inside reusable skills.

## 23. Multi-agent policy

Multi-agent execution is an implementation strategy, not the primary UX concept.

### Prefer a single worker when

- work is tightly coupled,
- exploration and implementation share context,
- one repository is involved,
- the deliverable is small or medium,
- or decomposing the work would create coordination overhead.

### Use delegated or parallel workers when

- research areas are independent,
- multiple repositories can be investigated separately,
- read-heavy exploration can be isolated,
- alternative designs should be compared independently,
- implementation and independent review should be separated,
- or different permissions and workspaces are required.

### Avoid multi-agent execution when

- requirements are still changing through conversation,
- workers would edit the same files,
- success criteria are unclear,
- the work is small,
- or the orchestration cost exceeds the work itself.

The user-facing UI should normally show one workstream, regardless of how many workers operate behind it.

## 24. Verification model

Completion verification should use three layers.

### Mechanical verification

Examples:

- file exists,
- test command succeeds,
- lint and type checks pass,
- expected commit exists,
- required sections are present,
- prohibited files were not changed.

### Specification verification

Examples:

- acceptance criteria are satisfied,
- constraints are respected,
- required evidence is attached,
- known limitations are explicit.

### Semantic verification

Examples:

- the result materially supports the outcome,
- the design decision is coherent,
- important risks were not missed,
- the result matches user intent.

LLM review should primarily supplement semantic verification rather than replace mechanical evidence.

## 25. Minimum data model

```yaml
UserContext:
  active_roles:
  responsibilities:
  attention_preferences:

Project:
  name:
  status:
  priority:

Outcome:
  purpose:
  success_criteria:
  current_state:
  next_action:
  priority:
  role:
  project_id:
  depends_on:
  resume_capsule:
  last_attended_at:
  updated_at:

Decision:
  question:
  recommendation:
  recommendation_evidence:
  options:
  tradeoffs:
  reversibility:
  affected_outcome_ids:
  source:
  status:

Constraint:
  scope:
  statement:
  source:
  status:

Workstream:
  outcome_id:
  purpose:
  status:

WorkItem:
  objective:
  status:
  execution_policy:
  user_attention_required:
  outcome_id:
  workstream_id:
  depends_on:

Execution:
  runtime:
  workspace:
  runtime_session_id:
  current_activity:
  evidence:

Completion:
  changes:
  verification:
  remaining:
  attention_required:

AttentionItem:
  type:
  urgency:
  reason:
  recommendation:
  affected_ids:
  source:
  status:
```

Every extracted or inferred record (`Decision`, `Constraint`, outcome updates, `AttentionItem`) carries `source` provenance back to the user message, document, plan, or execution that produced it (§29.5). `last_attended_at` supports the "changes since last attention" delta in Resume View (§12); `depends_on` supports conflict and blocking detection (§15.1). `Commitment`, `Deliverable`, `Evidence`, and `Review` (§21) are intentionally omitted from the minimum model; they are added only when a phase needs them.

## 26. MVP scope

The first implementation should contain only the surfaces and flows necessary to validate cognitive load reduction, ordered by value density (§1.3): resume continuity and completion trust first, decision/attention management second, inference and framing last.

### 26.1 MVP screens

1. Workspace switcher (Cognitive ⇄ Classic) — the Classic Workspace is the unchanged existing UI
2. Home
3. Outcome Detail
4. Decision Detail
5. Resume View
6. Execution Detail (links to Classic for live sessions)

### 26.2 MVP capabilities

Core (validates the hypothesis; §1.4 criteria apply):

- resume capsule with freshness and provenance,
- compressed completion reports with evidence attachment,
- outcome state and success criteria,
- decision records with evidence-cited recommendations,
- attention queue,
- Undo for automatic intake interpretation (§8.1 scope).

Supporting:

- global natural-language capture (proposal-staged for scope-changing input, §7),
- project and outcome inference,
- workstream state,
- one Codex App Server worker execution,
- expandable execution detail,
- Classic ⇄ Cognitive cross-navigation deep links.

### 26.3 Explicit MVP exclusions

- visual DAG editor,
- complex autonomous scheduling,
- many simultaneous write-capable agents,
- advanced learned model routing,
- organization-wide multi-user workflow,
- exhaustive issue tracker replacement,
- high-granularity progress percentages,
- fully autonomous priority assignment,
- and automatic external communication.

### 26.4 Cut candidates

The following are in the plan but are the first to be dropped if their marginal value over the baseline (§1.2) is not demonstrated. They are built last:

- role-aware framing (§14) — wrong-role framing can be worse than none,
- capture inference beyond simple project selection — typing a project name is cheap,
- portfolio/attention-conflict views (§15) — a good list may suffice for a single user,
- meaningful-event compression beyond what completion reports need.

## 27. MVP phases

Phases are gated by the falsification criteria in §1.4: a phase starts only if the previous phase's criteria passed.

### Phase 0: Value probe — resume capsule and completion trust

Decided (2026-08-04): Phase 0 proceeds as a lightweight direct implementation — no requirements/design pipeline for the probe itself. Its scope is **agent-grid development work only**: capsules are generated from this repository's own development sessions, keeping the session source and the measurement inside one project. Multi-project coverage is deferred to a later phase. Measurement stays lightweight: weekly self-report plus the one-action wrong-capsule flag; no dedicated telemetry is built for Phase 0.

Implement on top of the existing Classic Workspace, with no new navigation surfaces beyond a minimal panel and the workspace-switcher shell:

- resume capsules generated automatically from existing session activity,
- compressed completion reports with evidence for finished sessions,
- freshness/staleness display and one-action wrong-capsule flagging,
- consolidation of the per-turn session summary into the capsule call, with fallback preserved (§12.2),
- Classic remains the default workspace.

Primary validation (§1.4): resume without rereading history; capsule material-error rate; completion review without opening raw execution.

This phase deliberately builds no orchestration and no new data-entry surfaces. If capsule trust or resume time fails here, the failing layer is fixed or the initiative stops — the remaining phases all depend on this result.

### Phase 1: Structured cognitive state

Implement:

- outcomes,
- decisions,
- attention items,
- Home,
- Outcome Detail,
- Global Capture (proposal-staged, §7).

Execution may initially remain manual or use one existing agent session.

State freshness is a hard requirement of this phase: structured state (outcome `current_state`, capsules, attention items) must be updated automatically from session and execution activity. A steady state where the user types state updates by hand replaces "remembering" with "data entry" and fails the maintenance-cost criterion (§1.4). Stale state must be visibly marked (§12.1); a stale capsule presented as current is worse than no capsule.

Primary validation:

- Can the user understand current state quickly?
- Can interrupted work be resumed without rereading history?
- Does state stay fresh without manual maintenance?

### Phase 2: Codex App Server execution

Implement, as a thin adapter per §1.3 (reuse the existing session daemon and `claude-app-server`; do not build new orchestration where runtime features suffice):

- runtime adapter,
- execution creation and status,
- meaningful event compression,
- completion reports,
- evidence and verification display,
- pause, stop, and open-session controls (open-session switches to Classic).

Primary validation:

- Can the user delegate without managing the thread directly?

### Phase 3: Decision and attention management

Implement:

- recommendation-based Decision Detail,
- interruption policy,
- cross-project attention groups,
- role-aware framing,
- conflict detection using explicit priorities and dependencies.

Primary validation:

- Does the system reduce unnecessary interruptions and decision setup cost?

### Phase 4: Selective delegation and review

Implement:

- independent read-heavy worker delegation,
- independent reviewer execution,
- multi-runtime adapters,
- runtime-neutral execution profiles.

Primary validation:

- Does parallel work increase throughput without increasing user supervision?

### Phase 5: Evidence-based routing

Use execution history to refine:

- model profile choice,
- skill selection,
- when to delegate,
- when to request review,
- and when to ask the user.

Do not implement advanced routing before sufficient execution data exists.

## 28. Primary user flows

### 28.1 New request

```text
User enters natural language
→ system associates project, outcome, role, and context
→ system shows compact interpretation
→ system creates or updates work state
→ system starts one worker when safe
→ system verifies output
→ system compresses completion
→ system updates resume and attention state
```

### 28.2 Consultation becomes work

```text
User discusses a design
→ decisions and constraints are extracted
→ affected outcome is updated
→ implementation or plan changes are identified
→ reversible work begins or is proposed
→ progress is represented as workstream state
```

### 28.3 Interruption and resume

```text
User leaves work
→ resume capsule is updated
→ autonomous execution may continue
→ changes are accumulated
→ user returns
→ Resume View shows delta, current state, and next action
```

### 28.4 Worker becomes blocked

```text
Execution failure
→ automatic retry
→ alternative skill or approach
→ optional alternative execution profile
→ if still blocked, compress findings
→ create one attention item with recommendation
```

User-facing result:

```text
Could not complete automatically

Problem:
The selected Windows service approach cannot reliably access the
interactive user's WSL distribution.

Tried:
- direct wsl.exe launch
- scheduled task bridge
- user-session helper

Recommendation:
Limit the MVP to post-login startup.

[Accept recommendation] [Review findings]
```

## 29. UX safeguards

### 29.1 Do not over-notify

The system should optimize for fewer meaningful interruptions rather than maximum transparency by default.

### 29.2 Do not fabricate certainty

Avoid unsupported confidence percentages.

Prefer explicit evidence:

- tests passed,
- official documentation confirmed,
- one assumption remains,
- independent review completed.

### 29.3 Do not silently broaden scope

Automatic work may fill in implementation detail, but significant scope changes must create a decision or clear reversible proposal.

### 29.4 Do not make the user manage orchestration

The system should not expose routing choices unless the automatic choice is unavailable, unusually expensive, risky, or explicitly requested.

### 29.5 Preserve source provenance

Every extracted decision, constraint, and outcome update should retain a source reference to the user message, document, issue, plan, or execution that produced it.

### 29.6 Prefer reversible automation

Automatic actions should be biased toward operations that can be undone, paused, or revised.

## 30. Success metrics

The primary metric should not be number of agent runs or generated artifacts.

The north-star metric is:

> Time per week that the user spends managing AI work rather than making necessary domain decisions.

This cannot be measured automatically for a single user; it is tracked by a lightweight weekly self-report. The operational primary metrics — automatically measurable and tied to the falsification criteria in §1.4 — are:

- time to resume an interrupted outcome,
- rate of resumes that required opening raw history,
- share of ordinary successful work reviewed without opening raw execution,
- resume capsules flagged as materially wrong,
- user corrections to automatic intake interpretation,
- accepted recommendations later overturned,
- share of active time spent in Classic versus Cognitive workspace (adoption and trust signal, §2.1).

Supporting measures:

- time to understand current work state from Home,
- time to resume an interrupted outcome,
- number of repeated background explanations,
- number of user questions generated by agents,
- number of raw execution sessions the user must inspect,
- number of unnecessary notifications,
- number of forgotten or abandoned commitments,
- number of duplicate work items,
- number of manual integrations across agent results,
- number of active sessions directly managed by the user,
- percentage of completions with usable evidence,
- percentage of decisions with a recommendation,
- percentage of autonomous work that completes without user interruption,
- and user corrections to automatic intake interpretation.

Initial target UX criteria:

- understand Home state within 30 seconds,
- resume an interrupted outcome within 1 minute,
- complete routine verification review within a few seconds,
- avoid opening raw agent execution for ordinary successful work,
- and ensure every attention item states why user attention is required.

## 31. Open design questions

The following should be resolved during implementation planning:

1. How should outcomes be inferred and merged without creating duplicates?
2. What is the source of truth for cross-project priority?
3. How long should completed items remain visible on Home?
4. Which automatic actions require an explicit preview?
5. How should stale decisions and constraints be detected?
6. How should resume capsules handle conflicting execution results?
7. How should the system distinguish a project, outcome, responsibility, and recurring operational commitment?
8. How should attention preferences vary by role, project, and time window?
9. Which Codex App Server events should be promoted into meaningful execution events?
10. How should independent review be triggered without doubling cost for low-risk work?
11. How should the UX represent work that spans GitHub, Google Drive, Notion, Slack, and other external systems?
12. How should private company work and personal projects be isolated while still appearing in a unified attention view?
13. How should capsule correctness be measured cheaply (one-action flagging is the start, but silent wrongness must also be caught)?
14. What minimal telemetry is needed for the §30 operational metrics without building heavy instrumentation?
15. How should the cognitive state store relate to the existing dev-docs system (change packages, tasks, issues)? Outcome/Decision/WorkItem overlap conceptually with dev-docs documents; the options are extending dev-docs as the substrate versus an independent store with provenance links. Decided 2026-08-04: resolve at Phase 1 design, not before.

Resolved in this revision:

- Coexistence with the existing UI → dual-workspace switcher (§2.1).
- Undo transactionality → scoped in §8.1.
- Stale-state handling in the UX → freshness requirements in §12.1 and Phase 1.
- Auto-apply boundary for Global Capture → proposal staging for decision/scope changes (§7).

User decisions (2026-08-04):

- Phase 0 proceeds as a lightweight direct implementation (no requirements/design pipeline for the probe).
- §1.4 falsification thresholds adopted as initially proposed (30% raw-history resumes / ~10% capsule error / 2-week windows).
- Phase 0 scope is agent-grid development only; multi-project coverage deferred.
- dev-docs relationship (Q15) is deliberately deferred to Phase 1 design.
- Resume-capsule generation supersedes the per-turn session summary for agent sessions, with one-directional fallback to the existing summary (§12.2). Done before the Phase 0 measurement window.

Still open for the user (not blocking Phase 0): client scope for the workspace switcher (web-only first vs. Windows Shell/Electron), user-facing UI language (mockups mix English and Japanese), cross-project priority source of truth (Q2), company/personal isolation (Q12).

## 32. Final design position

Agent Grid should not be designed around the statement:

> Codex agent 12 is running.

It should be designed around the statement:

> One decision requires your attention. Four other workstreams are progressing without you.

The core product value is not multi-agent execution itself. The core value is that a user with many responsibilities, roles, and parallel development efforts can safely stop remembering and manually coordinating the full state of their work.

Codex App Server, dev-skills, model routing, and subagents are supporting mechanisms. The durable product boundary is:

```text
Attention
→ Decision
→ Outcome
→ Delegation
→ Evidence
→ Continuity
```

Investment follows the thin-durable-layer policy (§1.3): the user's cognitive state — outcomes, decisions, constraints, provenance, attention history — is the durable asset. Execution machinery is kept thin and replaceable, on the assumption that runtime products will keep absorbing it.

The Classic Workspace remains the permanent fallback (§2.1). The Cognitive Workspace must continuously earn its usage against that fallback and against the markdown-and-discipline baseline (§1.2); the falsification criteria in §1.4 decide whether each phase has earned the next one. Implementation experiments should target measurable cognitive load reduction, never orchestration complexity for its own sake.