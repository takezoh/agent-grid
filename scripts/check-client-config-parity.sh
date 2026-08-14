#!/usr/bin/env bash
# ver-expected-outcome-corpus-agrees-in-three-languages
# (contract-client-config-parity / adr-20260814-shared-go-client-config-module).
#
# clients/config declares servers.json a shared contract for every client host.
# Three hand-written loaders read it — Go, TypeScript, C# — and before the
# expected-outcome corpus nothing could tell three loaders that agree from
# three loaders that each merely throw. This runs all three readers of that
# corpus.
#
# The C# reader needs the .NET SDK, which the Linux PR runner does not carry.
# When it is absent the run is reported as SKIPPED rather than passing quietly:
# a parity check that silently drops a language is worse than no parity check,
# because it reads as agreement.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

echo "== Go loader =="
(cd clients/goclient && GOWORK=off go test ./clientconfig/ -run 'TestExpectedOutcomeCorpus' -count=1)

echo "== TypeScript loader =="
(cd clients/workspace && npx vitest run tests/config-corpus.test.ts)

echo "== C# loader =="
if command -v dotnet >/dev/null 2>&1; then
  dotnet test clients/windows-shell/AgentGrid.Shell.Core.Tests \
    --nologo --filter 'FullyQualifiedName~ConfigCorpusTests'
else
  echo "SKIPPED: dotnet is not installed; the C# corpus reader did not run." >&2
  echo "SKIPPED: run 'dotnet test clients/windows-shell/AgentGrid.Shell.Core.Tests --filter FullyQualifiedName~ConfigCorpusTests' on a machine with the .NET SDK." >&2
fi
