# Generated SDKs (quicktype models + hand-written transport)

Design:

- **Message SoT**: `protocol/*.schema.json` (JSON Schema 2020-12)
- **REST annex**: `protocol/openapi.yaml` (routes only — **not** a generator input)
  — `adr-20260724-protocol-message-schema-sot-rest-binding`
- **Models**: quicktype, pinned under this directory's npm lockfile
  — `adr-20260724-sdk-codegen-quicktype-typegen`
- **Transport**: hand-written per language (REST / WS / reconnect)

| Language | Models (generated) | Transport (hand-written) |
|---|---|---|
| TypeScript | `ts/generated/` | `ts/src/` |
| C# | `csharp/Generated/` | `csharp/Transport/` |
| Kotlin | `kotlin/generated/` | `kotlin/transport/` |
| Swift | `swift/Generated/` | `swift/Transport/` |

## Rules

- Do **not** hand-edit `generated/` / `Generated/` trees; re-run `scripts/generate-sdks.sh`.
- Go remains hand-written + stdlib-only (`src/host/proto`, `src/server/api/wire.go`).
- Compatibility CI fails on undeclared surface, missing pins, or missing shared simulator scenarios.

## Generate

```sh
# from repo root
scripts/generate-sdks.sh
# or
make generate-sdks
```

Requires Node/npm. First run creates/updates `clients/sdk/package-lock.json` and
`clients/sdk/.quicktype-version`. Two runs against the same `protocol/` commit
and the same pin must produce byte-identical model trees.

## Pin

| File | Role |
|---|---|
| `package.json` / `package-lock.json` | exact `quicktype` version |
| `.quicktype-version` | stamp written by the generator (CI asserts match) |
| `quicktype-emit.json` | per-language emit options (invariance-witnessed) |
