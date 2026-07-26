---
change: change-20260726-retire-nondriving-stream-t3
role: implementation
---

<!-- lifecycle is owned by change.md -->

# Implementation

Delete `routing_e2e_test.go`. Its approval test was retired by the originating
change; its remaining isolation test and placeholder shared the same invalid
assumption that `BindFrame` itself starts the returned launch plan.
