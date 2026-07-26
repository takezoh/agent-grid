---
change: change-20260726-codex-dot-spinner
role: implementation
---

<!-- lifecycle is owned by change.md -->

# Implementation

- Replace Web SVG ring/arc geometry with eight circles and CSS opacity classes.
- Replace WinUI ellipse/path ring with eight Canvas ellipses; retain the
  Composition rotation on the containing element.
- Add a DOM geometry contract rejecting the old ring and arc.
