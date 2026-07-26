---
change: change-20260726-unified-driver-session-status
role: implementation
---

<!-- lifecycle is owned by change.md -->

# Implementation contract

## Ownership and boundaries

- `host/state` is the sole owner of valid composite facets, pending request projection, and `(serverEpoch, signalSequence)`.
- The Codex router owns held-request terminalization; atomic held-entry removal is the single linearization point.
- Driver normalizers accept their authoritative sources. Codex uses app-server only; Claude/Gemini retain existing OSC title observations; Shell retains OSC 133 prompt observations.
- The server schema owns a breaking composite-only wire. The legacy scalar, legacy event fallback, and Codex OSC status-inference paths are removed in the same revision.
- A checked-in language-neutral fixture owns semantic visual keys, allowed combinations, animation roles, and accessible labels.

## Fixed semantics

- Valid base lifecycle is stopped, idle, or active; failure is a distinct tagged outcome. Zero/active+failure invalid combinations fail fast.
- A non-empty pending human-input ID set derives `!` and suppresses spinner. Signals are orthogonal and may coexist with `!`.
- Client signal markers are in-process, per session and server epoch. Reconnect does not replay; client/server restart begins a new consumption scope.
- Codex method authority is a build-time inventory tied to the supported version. Unknown, malformed, and inventory-unavailable are distinct.
- Unknown diagnostics use a fixed-size rate-limited ring; held entries are bounded by outstanding server requests and drained on connection loss.

## Driver inventory

| Driver | Authoritative producer | Supported observations | Unsupported outcome |
|---|---|---|---|
| Codex | app-server JSON-RPC | lifecycle, approvals, requestUserInput, elicitation, tool proposal, resolved | diagnostic, no inferred state |
| Claude | structured runtime/driver events | lifecycle, input, failure exposed by adapter | explicit unknown for absent detail |
| Gemini | structured runtime/driver events | lifecycle, input, failure exposed by adapter | explicit unknown for absent detail |
| Grok | structured runtime/driver events | lifecycle, input, failure exposed by adapter | explicit unknown for absent detail |
| Shell | process lifecycle + OSC 133 prompt | active/idle/stopped limited profile | attention/failure detail unknown |

OSC 0/2, BEL, and OSC notification are forbidden status producers for the Codex row only.

## Delivery order

1. Implement composite status reducer and signal identity.
2. Implement Codex exhaustive router and held-request lifecycle.
3. Implement all-driver normalizers and remove Codex OSC inference.
4. Replace the server wire with the composite-only shape, delete legacy scalar/event/OSC status producers and consumers, regenerate every SDK, and update Web, Workspace, and Windows Shell in the same revision.
5. Land the shared display fixture and all three renderers behind one atomic generation, compile, zero-legacy-search, gateway-contract, and client-parity gate. Mixed-version rollout and runtime fallback are unsupported.
