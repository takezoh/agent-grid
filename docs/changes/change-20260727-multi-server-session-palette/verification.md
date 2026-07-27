---
change: change-20260727-multi-server-session-palette
role: verification
---

<!-- lifecycle is owned by change.md -->

# Verification

## Success conditions

- All contract normal/adversarial witnesses pass and invalid/stale input produces no create side effect.
- Server identity remains compound and visible through row, selection, confirmation, routing, result, and active context.
- One failed server does not stop healthy scopes; superseded requests and streams are cancelled and cleaned up within the fixed bounds.
- Legacy config/create shapes are absent or rejected; only typed profile config and version-2 create remain.

## Tier gates

- T0/T1: `cd src && go test ./host/config/... ./host/state/... ./server/api/...`
- T1: `cd clients/ui && npm run test:unit`
- T1: `cd clients/workspace && npm test`
- T3 descriptor/create triple: httptest fake + `FakeVsRealSessionConfig/Create` + OpenAPI/no-create contract via `GOCACHE=/tmp/gocache-agent-grid make test-e2e`.
- T3 Electron triple: injected fetch/WS fake + `FakeVsRealFleetGateway` + token/target isolation contract.
- T3 browser triple: fake fetch/WS + `FakeVsRealBrowserProvider` + one-server parity contract via `cd clients/ui && npm run test:e2e:real`.

Exact contract verification IDs, criteria, witnesses, and commands are canonical in [design-plan.json](design-plan.json).

## Recorded evidence

- `cd src && go test ./host/config ./server/api`: pass
- `cd src && go vet ./...`: pass
- `cd clients/ui && npm run lint && npm run typecheck && npm run test:unit`: pass (118 files, 1769 tests)
- `cd clients/workspace && npm run typecheck && npm test`: pass (12 files, 80 tests)
- docs conformance lint: pass (341 indexed, no warnings)
- `go test ./...`: all changed packages pass; the unrelated `uihost/TestProxyDialRefusedMaps503WithRetryAfter` fails in this environment because dialing `127.0.0.1:1` times out and maps to 502 instead of producing connection-refused/503.
- Windows Shell .NET test was not executable in the current environment because `dotnet` is unavailable.
