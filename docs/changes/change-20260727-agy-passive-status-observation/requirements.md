---
change: change-20260727-agy-passive-status-observation
role: requirements
---

<!-- lifecycle is owned by change.md -->

# Requirements

## Functional requirements

- **FR-001**: `agy` command MUST resolve to a dedicated Driver, not the generic
  fallback.
- **FR-002**: `PreInvocation` MUST project Running and a newer invocation MUST
  supersede delayed events from an older invocation.
- **FR-003**: only `Stop{fullyIdle:true}` MUST project Waiting. A non-idle or
  stale Stop MUST NOT clear Running.
- **FR-004**: process exit MUST remain the sole Stopped authority.
- **FR-005**: Agent Grid MUST install managed Antigravity plugins for the
  desktop/global and CLI plugin roots on both host and devcontainer paths.
- **FR-006**: the managed plugin MUST NOT register `PreToolUse`, because its
  mandatory decision changes the user's permission policy.
- **FR-007**: no tick, elapsed-time threshold, terminal silence, bare BEL, or
  uncharacterized OSC MUST change agy lifecycle.
- **FR-008**: the repository MUST contain reusable scripts and documented steps
  for raw PTY, hook, binary, and third-party application characterization.

## Known limits

- agy 1.1.7 cancels the Stop hook context before the hook command is executed
  when Esc interrupts an active response.
- The published hook contract has no passive notification for native permission
  prompt onset. `PreToolUse` is an intervention point, not an observer.
- These limits MUST remain explicit until a real upstream event or independently
  characterized terminal signal closes them.
