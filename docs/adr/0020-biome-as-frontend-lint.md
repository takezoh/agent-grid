# ADR 0020 — Biome as the single frontend lint/format tool

Status: Accepted

## Context

The Go side uses `golangci-lint` as a single binary covering vet, depguard,
funlen, staticcheck, and friends. Bringing in two toolchains for the
frontend — `eslint` + `typescript-eslint` for lint plus `prettier` for
format — would double the configuration surface and the npm dependency
graph.

Biome (formerly Rome) provides linting + formatting in one binary, ships
with TypeScript support out of the box, and has a small config footprint
(`biome.json`).

## Decision

Use **Biome** as the sole linter and formatter under `src/client/web/`.
Configure `biome.json` with:

- `recommended` rule set as baseline
- `formatter.indentStyle: 'space'`, `indentWidth: 2`, `lineWidth: 100`
- `linter.rules.complexity.noUselessTypeConstraint: 'error'` (TS-aware)
- `organizeImports.enabled: true`

`npm run lint` invokes `biome check src/`. CI runs the same. Pre-commit
hooks are out of scope for the β PR.

## Consequences

- One npm dependency (`@biomejs/biome`) instead of 4–5 for eslint + plugins
  + prettier.
- Faster lint runs (~50 ms for a small SPA vs ~500 ms for eslint).
- Biome's rule coverage is narrower than eslint's, but the `recommended`
  set covers the common TS bugs we care about.
- Editor integration: Biome ships VS Code and JetBrains plugins.
- If a rule we need is missing, falling back to eslint later is a
  package.json swap, not a code change.

## Alternatives

- **eslint + typescript-eslint + prettier** — the standard React stack.
  More rules, more configuration, more dependencies. Rejected to mirror
  the Go side's single-binary model.
- **dprint** — comparable to Biome for formatting but lacks the linter.
- **No linter, only `tsc`** — under-protects; TypeScript catches type
  errors but not stylistic / antipattern issues.

## Related requirements

- FR-β04, FR-β14
