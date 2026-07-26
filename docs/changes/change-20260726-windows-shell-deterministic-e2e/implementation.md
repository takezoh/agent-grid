---
change: change-20260726-windows-shell-deterministic-e2e
role: implementation
---

<!-- lifecycle is owned by change.md -->

# Implementation

1. Remove redundant real-gateway supervisor and composition facts already
   covered through injected seams.
2. Add an `HttpListener`/WebSocket fake gateway inside the WinUI UI test
   fixture and point the isolated Shell config at it.
3. Emit two sessions plus approval/question requests and strengthen UIA
   assertions for sections, row layout, and status labels.
4. Document the Tier split.
