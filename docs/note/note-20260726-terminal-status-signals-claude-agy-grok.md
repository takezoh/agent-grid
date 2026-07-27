---
id: note-20260726-terminal-status-signals-claude-agy-grok
kind: note
title: Terminal status signals emitted by Claude, agy, and Grok
status: published
created: '2026-07-26'
tags: []
owners: []
relations:
- {type: references, target: adr-20260726-multi-source-driver-status-evidence}
source_paths: []
summary: Claude・agy・Grok が出力する OSC 0/9/9;4 と BEL を実体・配布ソース・PTY 生バイトで分類し、Agent Grid
  の lifecycle・action required・notification への変換契約を記録する。
updated: '2026-07-26'
---

## Summary

Claude Code 2.1.220、Antigravity CLI (`agy`) 1.1.7、Grok Build
0.2.112 を対象に、導入済み実体、配布ソース、PTY 生バイトを突き合わせた。
三者とも terminal presentation を lifecycle API と同一視できない。

- Claude は標準 TUI から OSC 0 title と OSC 9;4 progress を出す。
- agy は起動時の PTY 観測では title/notification を出さなかったが、
  hook から invocation、tool、permission、normal Stop を観測できる。
  バイナリには OSC 9;4 の文字列が存在するものの、実行中のどの調査区間でも
  発火せず、agy TUIのstatus契約としては採用できない。
- Grok は OSC 0 title、OSC 9;4 progress、OSC 9/99/777/BEL notification
  を別経路で出す。title は spinner と Action Required を同時に合成できる。

調査時点の Agent Grid は OSC command 9 を一律 notification として扱い、
OSC 9;4 progress を通知に誤分類していた。bare BEL も FrameTap の semantic
event として捕捉していなかった。本変更で両者を分離した。

## Notes

### Evidence baseline

| CLI | Observed version | Evidence |
|---|---:|---|
| Claude Code | 2.1.220 | installed native binary, startup PTY capture, official changelog/hooks docs |
| Antigravity CLI (`agy`) | 1.1.7 | official docs, installed ELF strings, real hook/PTY/transcript capture |
| Grok Build | 0.2.112 | installed ELF, startup PTY capture, xai-org/grok-build source |

PTY startup capture:

- Claude: `ESC ] 0 ; ✳ Claude Code BEL`
- Grok: `ESC ] 0 ; grok BEL`
- agy: no OSC 0/2/9 and no bare BEL during the captured startup interval

In OSC 0 and OSC 9;4, the final `BEL` terminates the OSC payload. It is not a
standalone audible/visual bell event. A parser must classify the complete OSC
before considering bare BEL.

### Claude Code

Claude emits two independent terminal surfaces.

1. OSC 0 title:
   - `✳` prefix: idle/waiting
   - Braille prefix: working
2. OSC 9;4 progress:
   - introduced as the terminal progress bar in 2.0.56
   - recent Claude changes explicitly keep it active across a complete turn and
     clear it on completion

Claude 2.1.141 also added custom hook JSON output `terminalSequence`, but this
is user/plugin-defined terminal presentation rather than Claude lifecycle.
It is outside the status mapping contract. If present, its bytes are
indistinguishable at the PTY boundary and therefore cannot raise the authority
of OSC evidence. Command number alone still cannot distinguish notification
from progress: OSC 9 and OSC 9;4 require payload classification.

変更前の Claude Driver は OSC 0/2 title と hook の双方から
`CommonState.Status` を直接上書きしていた。`CommonState.HandleTick` は
Running を期限切れにしないため、terminal title や終了 hook が欠落すると
`PreToolUse`/`UserPromptSubmit` の Running が残り得た。

#### Esc interruption capture

Claude Code 2.1.220 を `TERM=xterm-256color` の実 PTY（120×40）で起動し、
safe mode で hook/plugin の副作用を除外して title の生バイトと送信時刻を
採取した。prompt submit 後の遷移は次のとおりだった。

| Relative time | Input/output |
|---:|---|
| 6.542019 s | `ESC ] 0 ; ⠂ Claude Code BEL` |
| 7.205692 s | harness sends Esc |
| 7.275262 s | `ESC ] 0 ; ✳ Claude Code BEL` |

したがって Esc から idle title までの観測遅延は約 69.57 ms であり、
2 分の expiry を待つ必要はない。[Claude Code の公式 hooks reference](https://code.claude.com/docs/en/hooks#stop)
は
`Stop` について “Does not run if the stoppage occurred due to a user
interrupt” と規定している。よって user interrupt は hook completion
では閉じず、後着の native terminal idle evidence が先行する hook Running
を閉じる契約で扱う。

この capture では OSC 9;4 は観測されなかったため、Esc が progress-clear
を必ず出すとは結論しない。確認済みの中断契約は OSC 0 idle title である。

### Antigravity CLI (`agy`)

`agy` の installed binary は OSC 9;4 の clear と states 1–4 の文字列を
含むが、起動時、通常Running、Esc中断、permission promptの実測では
OSC title/progress/notificationを出さなかった。
`WT_SESSION`と`TERM_PROGRAM=Windows_Terminal`を設定した再測定でも0件
だった。文字列の存在だけではagy TUIから到達可能なコードパスを証明
しないため、OSC 9;4はDriver status契約に含めない。

### Grok Build

The published `xai-org/grok-build` source makes the separation explicit:

- `TitleManager` emits OSC title through crossterm `SetTitle`.
- Default title items are Action Required, spinner, activity, session name,
  and grok. Items are composed independently, so `⚠ Action Required` and a
  Braille spinner may coexist.
- `progress.rs` emits OSC `9;4;1;-1` while busy and `9;4;0;0` when clear.
- `protocol.rs` emits OSC 9, OSC 99, OSC 777, or bare BEL for transient
  notifications depending on terminal type.
- Default notification events are turn complete and approval required.

変更前の Grok Driver は hook も OSC title も消費せず、Tick handler にも
activity-based status transition がなかった。この terminal evidence を
本変更で Driver の観測へ配線した。

### Agent Grid parsing defect

変更前の `services/host/state/reduce_osc.go` は every non-title OSC を
`EffRecordNotification` に送り、`EvFrameOsc` は OSC 9 subcommand を保持・
分類していなかった。Consequently:

- `OSC 9;<message>` is a notification;
- `OSC 9;4;<state>;<progress>` is progress state;
- the current reducer treats both as notification.

The protocol parser must introduce a semantic OSC variant before Driver status
fusion. The minimum variants are `window_title`, `notification`,
`progress`, and `unknown`. Bare BEL must be represented separately if Agent
Grid wants to expose transient notification evidence.

### Status implications

Agent Grid keeps the already accepted Codex/app-server canonical display
statuses. Per-CLI native title combinations are observations, not display
contracts. In particular, Grok’s native `spinner + Action Required` does not
change Agent Grid’s canonical rule that human input suppresses the running
spinner.

Elapsed time is not status evidence. Running is never cleared by a lease and
Pending is never cleared by debounce. Grok's blinking Action Required title
does not expose an explicit resolved edge: the hidden blink frame is
indistinguishable from an ordinary title. Until a deterministic resolution
event is characterized, that title remains notification evidence and is not
projected to canonical Pending.

The implementation stores hook, terminal title, and OSC 9;4 as separate
timestamped observations and projects them through a shared resolver. OSC
notifications and bare BEL increment the shared one-shot bell signal but never
change lifecycle.
Codex continues to ignore all OSC for status. See
`adr-20260726-multi-source-driver-status-evidence`.

### Coverage gaps by purpose

The target is not “observe everything through OSC.” The target is to combine
the most semantic source available with an independent native TUI edge:

- Claude hooks identify prompt/tool/permission/elicitation semantics; OSC title
  closes the documented user-interrupt hole. A crash or terminal kill is owned
  by process lifecycle. No standard-TUI lifecycle scenario is currently known
  to be invisible to all three sources. Permission allow has no dedicated
  result hook, but the next `PreToolUse`/continuation event proves that Pending
  has ended and recovers Running; `Stop` or native idle recovers Waiting.
- agy hooks identify invocation/tool/normal completion semantics.
  `PostToolUse(ask_permission)` で permission の解決は回収できる一方、
  policy-neutral な開始イベントはない。Esc 中断では
  Stop hook 自体が context canceled となり、OSC と transcript にも
  終了 edge がないことを実測した。See the follow-up investigation below.
- Grok exposes running/waiting through title/progress and attention through
  notifications. Approval-required is source-confirmed, but its blinking title
  and post-approval resolution sequence lack a measured, unambiguous edge.
  Pending is therefore not fabricated.
- OSC 9/99/777 and bare BEL answer “notify the user,” not “which lifecycle
  state is current.” Their absence or presence cannot close Running.

### Follow-up: semantic alternatives to OSC

Versions checked:

- Claude Code 2.1.220;
- Antigravity CLI (`agy`) 1.1.7;
- Grok Build 0.2.112 and published source commit
  `47348d13ec4508dcfe440e34c6d511bb02998fb2`.

#### Claude

[Claude's official hook reference](https://code.claude.com/docs/en/hooks)
defines stronger semantic events than the Notification compatibility path:

| Flow | Start | Resolution |
|---|---|---|
| permission | `PermissionRequest` | `PermissionDenied` for denial; no documented `PermissionGranted` event |
| MCP elicitation | `Elicitation` | `ElicitationResult` with accept/decline/cancel |
| normal turn | `UserPromptSubmit` / tool hooks | `Stop` |
| user interrupt | no `Stop` by contract | measured OSC 0 `✳` idle title |

Agent Grid's current Claude hook installer does not register
`PermissionRequest`, `PermissionDenied`, `Elicitation`, or
`ElicitationResult`; the shared payload and Driver do not classify them.
Elicitation can therefore be made exact without terminal inference.
Permission onset and denial can also be exact. The allowed-permission
resolution has no dedicated granted/result hook, so the Driver clears Pending
on the next explicit continuation (`PreToolUse`, `UserPromptSubmit`) or
completion (`Stop` or native idle). The existing
`TestClaudePendingTransitionsToRunningOnToolUse` pins this recovery contract;
the lack of a dedicated event is not an observable-status gap.

#### Antigravity CLI (`agy`)

[Antigravity's official hooks reference](https://antigravity.google/docs/hooks)
defines `PreInvocation`, `PostInvocation`, `PreToolUse`, `PostToolUse`, and
`Stop`. `Stop` carries `terminationReason`, `error`, and `fullyIdle`.
[The permissions reference](https://antigravity.google/docs/cli/permissions)
confirms that command, MCP, URL, and out-of-workspace operations can pause on a
real Ask prompt. [The interaction reference](https://antigravity.google/docs/cli/prompting)
defines Esc as immediate cancellation of an active turn.

An agy 1.1.7 real-PTY run installed a temporary user-defined workspace
`hooks.json` recorder for all five events, submitted a live prompt, then sent
Esc. This was a research recorder, not an Agent Grid-installed production
hook. The recorder received
`PreInvocation` and a `PostToolUse` payload, but no `Stop`. agy's own diagnostic
stream recorded:

```text
Cancelling in-progress response ...
clearing ResponsePending
failed to call custom stop hook ... command failed: context canceled
Interrupted · What should Antigravity CLI do instead?
```

To exclude a recorder-process bug, the command was replaced with a script that
reads no stdin, writes a start marker immediately, returns the required Stop
output `{"decision":"stop"}`, and exits zero.
Normal `PreInvocation` and `PostToolUse` handlers wrote their markers, while
the Esc `Stop` handler wrote none. An `strace -f -e execve` capture likewise
showed execs for those two handlers and no shell or recorder exec for `Stop`;
agy still reported `context canceled`. Thus the Stop command is rejected
before `execve`, rather than failing inside the recorder.

A separate normal-completion run used the same conformant Stop response. Its
order was `PreInvocation → PostInvocation → Stop`, and Stop carried
`terminationReason:"NO_TOOL_CALL"` and `fullyIdle:true`. This isolates the
failure to the Esc cancellation path rather than the hook registration or
output contract.

No OSC title/progress/notification sequence was present in that capture.
Therefore Stop is suitable for normal completion but is not an Esc-close
signal in agy 1.1.7. This is an upstream execution-order defect, not a missing
Agent Grid mapping.

The permission flow was then exercised with a prompt requiring
`run_command("pwd")`. An initial all-tool research recorder produced the
following sequence, but its PreToolUse `{}` response was not contract-conformant:

```text
PreInvocation
PreToolUse(run_command)
PostToolUse(run_command)
PreInvocation
PreToolUse(list_permissions)
PostToolUse(list_permissions)
PreInvocation
PreToolUse(ask_permission)     # Pending begins
PostToolUse(ask_permission)    # user interaction resolved
PreInvocation
PreToolUse(run_command)        # Running resumes
PostToolUse(run_command)
...
Stop{terminationReason:"NO_TOOL_CALL", fullyIdle:true}
```

That sequence does not establish that `ask_permission` is the universal
permission boundary. A second run used a contract-conformant matcher restricted
to `ask_permission`; the native `run_command` approval prompt appeared without
any matching PreToolUse event. It rendered `Requesting permission for:` and
`Do you want to proceed?`, while OSC remained absent.

The official `PreToolUse` output contract requires a `decision` (`allow`,
`deny`, `ask`, or `force_ask`); the research recorder's `{}` response was not
contract-conformant. Registering a conformant PreToolUse observer necessarily
selects a gating decision and therefore cannot be assumed policy-neutral.
A matcher for `run_command` returning `ask` can deliberately create/retain an
approval gate, but it changes command gating and is not an observational
Driver. The public hook contract currently provides no passive semantic onset
event for native permission prompts.

The writable Esc run confirmed the earlier result. `Stop` still failed with a
cancelled context. Its `transcript_full.jsonl` contained only completed
USER_INPUT, CONVERSATION_HISTORY, and PLANNER_RESPONSE steps; it contained no
interrupt, cancel, or termination record. The private diagnostic log did
contain `clearing ResponsePending`, but that implementation log is not a
published event contract and is not suitable as Driver input.

Before change `change-20260727-agy-passive-status-observation`, Agent Grid
registered agy as a generic Driver, installed no agy plugin, and consumed only
OSC 9;4. The change replaces that boundary with a dedicated `AgyDriver` and
managed passive plugins. It consumes `PreInvocation`, `PostInvocation`,
`PostToolUse`, and `Stop`, while deliberately excluding policy-changing
`PreToolUse`. Reusable capture and analysis commands are documented in that
change's implementation member and checked in under `scripts/research/`.

#### Grok Build

Published source and tests show that approval state is owned by the TUI's
`permission_queue`:

- empty→non-empty emits an `ApprovalRequired` notification;
- title `has_pending_permissions` is exactly
  `!permission_queue.is_empty()`;
- queue resolution removes `Action Required`;
- when focused the marker is static, but when unfocused it intentionally
  alternates visible/hidden every 15 ticks.

Grok also supports notification command hooks with `GROK_EVENT`,
`GROK_MESSAGE`, and `GROK_SESSION_ID`, but only for events such as approval
required and turn complete. There is no approval-resolved notification hook.
Thus the hook/OSC notification can detect Pending onset, while neither can
independently distinguish an unfocused hidden blink from resolution. The
internal ACP permission request/response is exact but belongs to the private
in-process TUI↔agent connection; starting `grok agent stdio` as a sidecar would
observe a different agent and remains invalid.

### Vibe Island 1.0.42 comparison

Vibe Island is closed source. This comparison therefore uses its official
changelog and static analysis of the officially distributed 1.0.42 DMG, rather
than treating implementation details as a published compatibility contract.
The inspected artifacts are reproducible by these SHA-256 values:

| Artifact | SHA-256 |
|---|---|
| official 1.0.42 DMG | `416306d0b85a09073f1406ef5a6145d59d871e28aeeb8708f8585028cedf199d` |
| `Contents/MacOS/vibe-island` | `f2fda8a0ccf112d19fc7545510d52689be634238700c25c74211627b496f3371` |
| Linux amd64 hook helper | `d9c8840b04804f9c4f5b2773794edc530a506e005e4855e57f4fc9e8e33c3551` |

The shipped application contains per-agent installers rather than one OSC
status parser:

| Driver | Observed Vibe Island input | Consequence for Agent Grid |
|---|---|---|
| Codex | managed `.codex/hooks.json`, Codex rollout/session files, and an app-server client containing `thread/status/changed`, `waitingOnApproval`, and `isAwaitingApproval` | Confirms app-server as the precise live lifecycle/approval source. Hooks and files serve identity, titles, restoration, or auxiliary data; Codex terminal OSC need not be a status authority. |
| Claude | managed `.claude/settings.json` hooks, a status-line bridge, process/session identity, and transcript discovery | Confirms semantic hooks plus independent liveness/identity observations. The status-line strings are usage/model-oriented; no evidence was found that native Claude OSC is Vibe Island's lifecycle authority. |
| Antigravity (`agy`) | a managed plugin at `.gemini/config/plugins/vibe-island/hooks.json` and `.gemini/antigravity-cli/plugins/vibe-island/hooks.json`; normalized fields include `fullyIdle`, `terminationReason`, and `invocationNum` | Confirms the official Antigravity hook/plugin route and the value of preserving its native completion fields. It does **not** demonstrate recovery from the measured Esc cancellation: no OSC status consumer or other exact cancellation signal was found in the app. |
| Grok Build | managed `.grok/hooks/vibe-island.json`; the public changelog claims live status, questions, and completion alerts | Confirms that Grok's semantic hook surface should be consumed in addition to its measured title/progress presentation. Static analysis did not expose a separate approval-resolved event, so later lifecycle/title evidence remains necessary to clear Pending. |
| OpenCode/Pi/Hermes and others | dedicated generated plugins translate native session, tool, permission, question, completion, and interruption callbacks into a shared event vocabulary | Strong evidence for per-driver semantic adapters followed by a shared reducer, rather than a universal terminal heuristic. |

The hook helper forwards a normalized envelope over a Unix socket or TCP using
keys such as `hook_event_name`, `session_id`, `tool_name`, and `tool_input`.
Its vocabulary includes `SessionStart`, `SessionEnd`, `UserPromptSubmit`,
`PreToolUse`, `PostToolUse`, `PostToolUseFailure`, `Stop`, `StopFailure`,
`PermissionRequest`, `Notification`, and subagent events. The application then
projects these into shared states whose embedded names include `processing`,
`thinking`, `running_tool`, `waiting_for_approval`, `question`, `compacting`,
and `ended`.

OSC-related strings describe writing OSC 2 titles and caching
`vibe-island-osc2-title-*` values for terminal-tab identification and precise
jump-back. No evidence was found that Vibe Island consumes OSC 0/2, OSC 9;4,
OSC 9/99/777, or bare BEL as its cross-agent lifecycle SSOT. This distinction
is directly useful: terminal presentation and terminal identity may corroborate
a Driver, but native semantic events should own status whenever available.

The useful design pattern is therefore:

1. install a passive, agent-native hook/plugin or connect to the agent's real
   app-server;
2. translate source-specific events into a normalized semantic envelope;
3. retain source provenance and process liveness independently;
4. let one reducer resolve status, including clearing Pending from a later
   continuation, completion, or idle event;
5. reserve terminal titles/OSC for a characterized gap or terminal identity,
   not as a universal parser.

This comparison does not close the remaining agy gaps. Vibe Island's 1.0.41
claim that Antigravity “surfaces when an agent needs your attention” proves a
user-facing result, but neither the public changelog nor the distributed
artifacts establish a policy-neutral permission-start event or an Esc-stop
event beyond the same plugin surface already measured above. Copying an
unidentified timeout or attention heuristic would reintroduce inference, not
observation.

{% transition from="draft" to="published" date="2026-07-26" %}
Claude, agy, Grok の実体・配布ソース・PTY 観測を突合済み
{% /transition %}
