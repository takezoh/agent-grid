#!/usr/bin/env bash
# DEPRECATED for client e2e.
#
# Use the repo-standard WSL stack instead:
#   make run-dev    # → scripts/run-dev.sh (server + web, -no-auth on loopback)
#
# This file is kept only as a pointer so old docs/links do not 404.
# Detach survival (setsid) is covered by wsl-detach-spike.sh against a real server binary.
set -euo pipefail
echo "start-daemon.sh is not used for Windows Shell e2e." >&2
echo "In WSL run:  make run-dev" >&2
echo "Then on Windows: clients/windows-shell/scripts/dev-up.ps1" >&2
exit 2
