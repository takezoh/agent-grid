---
id: adr-20260624-0019-react-ts-vite-zustand-stack
kind: adr
title: 'ADR 0019 — A1-β frontend stack: React 18 + TypeScript + Vite + Zustand + xterm.js'
status: accepted
created: '2026-06-24'
updated: '2026-07-04'
tags:
- adr
- legacy-import
owners: []
relations:
- {type: references, target: adr-20260624-0021-frontend-wire-types-hand-written}
source_paths:
- src/client/web/
decision_makers:
- unknown
summary: A1-β replaces client/web's vanilla JS UI with a typed SPA. Master Plan (plans-cheerful-thompson.md
  §A1-β) named the stack as React + TypeScript + Vite + Zustand + xterm.js. We document
  the choice and pin the exact tools
---

<!-- migrated_from: docs/adr/0019-react-ts-vite-zustand-stack.md -->

# ADR 0019 — A1-β frontend stack: React 18 + TypeScript + Vite + Zustand + xterm.js

Status: Accepted

## Context

A1-β replaces `client/web`'s vanilla JS UI with a typed SPA. Master Plan
(`plans-cheerful-thompson.md` §A1-β) named the stack as React + TypeScript +
Vite + Zustand + xterm.js. We document the choice and pin the exact tools so
PR-1 consumers (and PR-3 reviewers) can reproduce the build with `npm ci`.

Alternative state libraries (Jotai, Redux Toolkit), terminal wrappers
(react-xtermjs), and bundlers (esbuild, webpack) were considered. The wire
contract from A1-α is frozen, so the frontend exists to render it — not to
re-invent the data model.

## Decision

Adopt the following exact-pin stack for `src/client/web/`:

- **React 18.x** with `react-jsx` runtime (no need for compatibility shims)
- **TypeScript 5.x** with `strict` + `noUncheckedIndexedAccess` + `verbatimModuleSyntax`
- **Vite 5.x** with **`@vitejs/plugin-react-swc`** for Fast Refresh (SWC is
  the lightest plugin that still supports React 18 effects correctly)
- **Zustand 4.x** as the single state library (Master Plan choice)
- **xterm.js 5.x** wrapped directly via `useEffect` (no
  `react-xtermjs` wrapper — one fewer dependency to track)
- **vitest 1.x** + **happy-dom** + **@testing-library/react** for tests
- **npm** as the package manager (no pnpm/yarn — minimum operational surface)
- **single chunk output** from Vite (`build.rollupOptions.output.manualChunks`
  disabled) to keep CSP `script-src 'self'` simple

Dev workflow: production-style build is the default (`npm run build`) so the
browser always sees the same CSP it ships with. Vite's HMR dev server is
available for fast iteration but is not used in `make run-dev`.

## Consequences

- The wire layer stays in raw TypeScript; no codegen between Go proto and TS
  (see [ADR 0021](../adr/adr-20260624-0021-frontend-wire-types-hand-written.md)).
- Bundle target: ~250 KB gzip including xterm.js. Single chunk keeps the CSP
  surface minimal.
- `vite-plugin-react-swc` has fewer config knobs than the Babel-based
  `@vitejs/plugin-react`, which is fine for a small SPA.
- `happy-dom` is faster than `jsdom`; xterm.js needs a `Worker` mock plus
  Canvas mock provided in the vitest setup file.
- `react-xtermjs` is a popular wrapper but adds a maintenance dependency; the
  direct `useEffect` wrapper is ~40 lines and stays in our control.

## Alternatives

- **Jotai** — atomic state library. Smaller API, but Zustand's selector model
  fits the "single source of truth at the store" goal more naturally.
- **Redux Toolkit** — overkill for the size of this app and pulls in
  `immer`/redux machinery that we don't need.
- **react-xtermjs wrapper** — saves ~40 lines but adds a versioned dependency
  that may diverge from xterm.js itself.
- **pnpm / yarn** — both work but introduce a second package-manager toolchain
  the team must keep up to date.
- **Multi-chunk bundle** — would marginally improve initial load but adds CSP
  surface (`script-src` may need to allow hashes per chunk). Not worth it
  until the bundle exceeds 500 KB.

## Related requirements

- FR-β02, FR-β04, FR-β07, FR-β10, FR-β14
