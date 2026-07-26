---
change: change-20260726-retire-nondriving-stream-t3
role: verification
---

<!-- lifecycle is owned by change.md -->

# Verification

- PASS: `go test ./host/runtime/subsystem/stream/... ./platform/agent/fakecodex/...`
- PASS: e2e-tag compile/skip selection for deterministic fakecodex fidelity.
- PASS: dev-docs lint and conformance.
