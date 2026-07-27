---
change: change-20260727-client-server-protocol-version
role: verification
---

<!-- lifecycle is owned by change.md -->

# Verification

## Success conditions

- Exact equality is the only admitted state on every REST/WS carrier, and every rejection precedes operational side effects.
- Authentication/version precedence and version disclosure are deterministic for compound failures.
- Web, Workspace, and Windows preserve one typed mismatch meaning, stop operational/reconnect work, and retain healthy peers.
- Server-list output identifies the current server label and both version values or explicit evidence markers.
- A public surface or compatibility branch cannot be added without a deterministic compatibility-gate failure.

## Tier strategy

### T0 — pure contracts

- Token grammar boundary values: empty, duplicate, 1/64/65 bytes, control/non-ASCII, valid unequal, and exact equal.
- Closed fleet outcome reducer, generation fencing, label/evidence join, and presenter formatting.
- Manifest/schema validation and generated-projection equality.

Criterion: every input belongs to exactly one typed outcome; no malformed input reaches equality comparison and no mismatch becomes unavailable.

### T1 — wired components and gateway scenarios

- `services/server/api`: auth/version precedence, representative read/write route effect counters, ticket binding/replay/expiry, auth/no-auth `/ws`.
- `clients/ui`, Web host, Workspace, and Windows: shared recorded mismatch scenarios, header continuity, typed decoding, zero retry/operation count, and healthy-peer preservation.
- Playwright fake backend and protocol simulator implement the same exact-match rejection contract.

Commands:

- `cd services && go test ./server/api/...`
- `cd clients/ui && npm run test:unit && npm run test:e2e`
- `cd clients/workspace && npm run test:unit`
- `dotnet test clients/windows-shell/AgentGrid.Shell.sln`

Criterion: all host projections agree for identical fixtures and every mismatch path has zero operational or reconnect calls.

### T2 — contract and conformance

- Closed inventory covers all public routes, shipped hosts, decoders, retry decision sites, SDK projections, fakes, and simulator surfaces.
- Mutation fixtures add a range comparator, dual decoder, capability degradation, renamed fallback retry, and unregistered surface; each must make the gate fail.
- Cross-language generation remains deterministic and Go wire-format code remains stdlib-only.

Commands:

- `make lint`
- `cd services && go test ./...`
- `scripts/check-protocol-compatibility.sh`

Criterion: any inventory gap is inconclusive and therefore a failure; advisory word scans are never the only evidence.

### T3 — fidelity backstops

- `make test-e2e` runs fake-vs-real protocol fixtures when configured.
- `cd clients/ui && npm run test:e2e:real` proves a real server plus two-server partial failure.
- `make test-windows-shell-e2e` proves the native server-list mismatch cause when its environment is available.

Criterion: real implementations and fakes produce identical typed evidence; if `FakeVsReal*` differs, the fake is fixed.

## Negative assertions

- No SemVer/range comparison, legacy decoder, capability hide/disable, downgrade, retry fallback, user override, or mixed-version success exists.
- Invalid credentials never disclose server version.
- Mismatch never reaches handler effects, WS application frames, or reconnect/backoff.
- A mismatching server never erases or disables a matching peer.

