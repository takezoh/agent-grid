# Compatibility policy

**Change**: change-20260723-native-clients-phase01  
**Schema**: `protocol/capabilities.schema.json`

## Two-axis model

| Axis | When | Behavior |
|---|---|---|
| **bundled** | Native shell + co-shipped same-build daemon (`protocolVersion` equal) | Version-match only; no per-capability negotiation round-trip (NFR-04) |
| **remote** | Version skew or non-bundled client | Daemon advertises capability set; client degrades undeclared capabilities to disabled/hidden; never invokes them speculatively (FR-P1-04) |

## Evolution

Within a major version, protocol schema changes are **additive-only** (NFR-05). Removed/renamed required fields require a major bump.

## CI gate

The `compatibility` test-harness profile fails closed on:

1. Generated SDK invoking undeclared wire surface
2. Inconclusive undeclared-surface scans
3. New SDK targets that skip the shared recorded-scenario suite
