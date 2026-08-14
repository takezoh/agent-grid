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
# A user-local SDK install is not on PATH by default; clients/windows-shell/
# scripts/e2e.sh reaches for the same kind of fallback on the Windows side.
dotnet_bin=""
if command -v dotnet >/dev/null 2>&1; then
  dotnet_bin=dotnet
elif [[ -x "$HOME/.dotnet/dotnet" ]]; then
  dotnet_bin="$HOME/.dotnet/dotnet"
fi
if [[ -n "$dotnet_bin" ]]; then
  # -m:1 disables MSBuild's out-of-proc nodes, which talk over a named pipe. In
  # a sandbox that denies socket(2) the build otherwise dies with
  # SocketException (13) before compiling anything, and it costs only build
  # parallelism anywhere else.
  dotnet_args=(-m:1)
  # SourceLink's git task reads .gitmodules directly. Some checkouts — a WSL
  # bind mount, a container overlay — expose it as something other than a
  # regular file, and the task then fails the whole build with an access error.
  # The condition is detected rather than assumed, so a normal checkout keeps
  # its source-control metadata.
  if [[ -e .gitmodules && ! -f .gitmodules ]]; then
    echo "note: .gitmodules is not a regular file; skipping SourceLink's git queries" >&2
    dotnet_args+=(-p:EnableSourceControlManagerQueries=false)
  fi
  "$dotnet_bin" test clients/windows-shell/AgentGrid.Shell.Core.Tests \
    --nologo "${dotnet_args[@]}" --filter 'FullyQualifiedName~ConfigCorpusTests'
else
  echo "SKIPPED: no .NET SDK on PATH or at ~/.dotnet; the C# corpus reader did not run." >&2
  echo "SKIPPED: run 'dotnet test clients/windows-shell/AgentGrid.Shell.Core.Tests --filter FullyQualifiedName~ConfigCorpusTests' on a machine with the .NET SDK." >&2
fi
