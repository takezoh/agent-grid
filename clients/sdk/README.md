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
- Go remains hand-written + stdlib-only (`services/host/proto`, `services/server/api/wire.go`).
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

## SessionTarget discriminator conformance

Repository flag-day requires every shipped SDK to reject collapsed/mixed
`SessionTarget` payloads (project XOR repository, non-empty identity fields).
Each language has a runnable harness that imports the **checked-in generated**
model — do not hand-edit `generated/` / `Generated/`.

| Language | Harness entry | Local command |
|---|---|---|
| TypeScript | `ts/session_target_discriminator.test.ts` | `cd clients/sdk && npm test` |
| C# | `csharp/harness/` (`dotnet` + System.Text.Json) | `dotnet run --project clients/sdk/csharp/harness` |
| Kotlin | `kotlin/harness/SessionTargetConformance.kt` | via runner (needs `kotlinc` + `java`) |
| Swift | `swift/harness/main.swift` | via runner (needs `swiftc`) |

**Orchestrated runner** (preferred):

```sh
# from repo root — runs TS always; C#/Kotlin/Swift only when toolchains exist
clients/sdk/scripts/run-session-target-conformance.sh

# CI / fail-closed: every language must execute
AG_SDK_CONFORMANCE_REQUIRE_ALL=1 clients/sdk/scripts/run-session-target-conformance.sh
```

The always-on Go gate `TestSessionTargetDiscriminatorCrossSDK`
(`services/gorules/repository_flagday_test.go`) asserts harness inventory,
always runs TypeScript, and conditionally runs native harnesses when present.
CI job `sdk-session-target` installs Node / .NET 8 / JDK+Kotlin / Swift and runs
the runner with `AG_SDK_CONFORMANCE_REQUIRE_ALL=1`.
