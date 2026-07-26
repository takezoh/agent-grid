---
change: change-20260727-agy-passive-status-observation
role: implementation
---

<!-- lifecycle is owned by change.md -->

# Implementation

## Boundaries

- `host/lib/agenthook` owns managed plugin files and shell-safe hook commands.
- `host/event` owns transport plus the source hook's stdout response contract.
- `host/driver/AgyDriver` owns agy payload ordering and status projection.
- `CommonState.StatusEvidence` remains the shared resolver for all Drivers.
- `host/state` and clients continue to consume only the common projected status.

## agy event mapping

| Input | Projection |
|---|---|
| process start | Waiting baseline |
| `PreInvocation{invocationNum:N}` | Running; remember N |
| `PostInvocation` / `PostToolUse` | Running |
| `Stop{executionNum:N,fullyIdle:true}` with N not stale | Waiting |
| `Stop{fullyIdle:false}` or stale N | no clear |
| process exit | Stopped / Failed through common process handling |

The helper returns `{}` for passive lifecycle callbacks and
`{"decision":"stop"}` for Stop. `PreToolUse` is deliberately absent.

## Managed plugin layout

Agent Grid owns only directories carrying `.agent-grid-managed`:

```text
~/.gemini/config/plugins/agent-grid/
~/.gemini/antigravity-cli/plugins/agent-grid/
```

Each contains `plugin.json` and `hooks.json`. An existing directory without the
ownership marker is rejected rather than overwritten.

## Reproducible characterization workflow

The investigation scripts are intentionally generic and live under
`scripts/research/`.

### 1. Pin the executable and environment

```sh
command -v claude codex agy grok
claude --version
codex --version
agy --version
grok --version
sha256sum "$(command -v agy)"
```

Record CLI version, OS, terminal, `TERM`, tmux presence, focus state, sandbox
mode, and permission mode. A signal absence is meaningful only for the exact
environment exercised.

### 2. Capture exact PTY bytes

Do not use copied terminal text: it loses control sequences. Schedule input as
hex bytes and save the PTY stream:

```sh
scripts/research/capture-terminal-signals.py \
  --output /tmp/agy-esc.raw \
  --timeout 30 \
  --send 2:5265706c792065786163746c79204f4b2e0d \
  --send 8:1b \
  -- agy

scripts/research/analyze-terminal-signals.py /tmp/agy-esc.raw
xxd -g1 /tmp/agy-esc.raw
```

Repeat startup, normal completion, active Running, Esc interruption, permission
prompt, focus/unfocus, tmux, and a supported native terminal. Distinguish:

- OSC 0/2 title;
- OSC `9;4` progress;
- OSC 9/99/777 notification;
- bare BEL outside an OSC payload.

The BEL byte terminating an OSC sequence is not a standalone bell.

### 3. Capture hooks with contract-conformant responses

Use `record-agent-hook.py`, never a recorder that violates the upstream stdout
schema:

```json
{
  "agent-grid-research": {
    "PreInvocation": [{
      "type": "command",
      "command": "/workspace/agent-grid/scripts/research/record-agent-hook.py --contract agy --event PreInvocation --output /tmp/agy-hooks.jsonl"
    }],
    "PostInvocation": [{
      "type": "command",
      "command": "/workspace/agent-grid/scripts/research/record-agent-hook.py --contract agy --event PostInvocation --output /tmp/agy-hooks.jsonl"
    }],
    "Stop": [{
      "type": "command",
      "command": "/workspace/agent-grid/scripts/research/record-agent-hook.py --contract agy --event Stop --output /tmp/agy-hooks.jsonl"
    }]
  }
}
```

Place this temporarily at `<workspace>/.agents/hooks.json`. Do not add
`PreToolUse` to a passive experiment: agy requires a gating decision and the
experiment would alter the scenario being observed.

### 4. Distinguish “hook not called” from “recorder failed”

Use a minimal handler that reads no stdin, writes a marker before all other
work, returns the required JSON, and exits zero. Then trace process creation:

```sh
strace -f -e trace=execve -o /tmp/agy-hook-execve.log agy
rg 'record-agent-hook|Stop' /tmp/agy-hook-execve.log
```

For the measured Esc case, agy logged `context canceled` and no Stop handler
`execve` occurred. That proves cancellation before hook command execution;
changing the recorder cannot fix it.

### 5. Inspect published binaries without promoting strings to contracts

```sh
strings -a "$(command -v agy)" > /tmp/agy-strings.txt
rg -n 'OSC|fullyIdle|terminationReason|PreInvocation|Stop' /tmp/agy-strings.txt
```

A string establishes only that bytes exist in the artifact. It becomes a
Driver contract only after a reachable source path, official documentation, or
real execution capture establishes its semantics.

### 6. Compare third-party integrations

For a signed third-party distribution such as Vibe Island:

1. download only from its official update feed;
2. record URL, release version, date, and SHA-256;
3. extract without installing;
4. inspect hook helper and main application strings separately;
5. locate config paths, event names, normalized states, sockets, and OSC use;
6. distinguish writes used for tab identity from reads used for lifecycle;
7. corroborate public claims with the official changelog;
8. label conclusions from closed-source static analysis as implementation
   evidence, not a public compatibility contract.

The 1.0.42 artifact hashes and findings are retained in
`note-20260726-terminal-status-signals-claude-agy-grok`.

### 7. Evidence acceptance rules

- Accept: official event contract, source implementation and tests, exact PTY
  bytes from the stated scenario, hook JSON plus conformant response, or
  process trace proving execution order.
- Corroboration only: terminal title, progress OSC, transcript/cache files, and
  third-party static implementation details.
- Reject as lifecycle evidence: screen text parsing, undocumented private
  diagnostic logs, mere binary string presence, keypress alone, silence, and
  elapsed-time decay.
