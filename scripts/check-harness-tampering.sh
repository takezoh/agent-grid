#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
base_root=
head_root=$repo_root
request=
output=

while [ "$#" -gt 0 ]; do
  case "$1" in
    --base-root) base_root=$2; shift 2 ;;
    --head-root) head_root=$2; shift 2 ;;
    --request) request=$2; shift 2 ;;
    --output) output=$2; shift 2 ;;
    *) echo "usage: $0 --base-root PATH [--head-root PATH] [--request FILE] [--output FILE]" >&2; exit 2 ;;
  esac
done

[ -n "$base_root" ] || { echo "--base-root is required" >&2; exit 2; }
base_root=$(CDPATH= cd -- "$base_root" && pwd)
head_root=$(CDPATH= cd -- "$head_root" && pwd)
caller_root=$(pwd)
case "$request" in ""|/*) ;; *) request=$caller_root/$request ;; esac
case "$output" in ""|/*) ;; *) output=$caller_root/$output ;; esac

set -- --mode tampering --base-root "$base_root" --head-root "$head_root"
[ -z "$request" ] || set -- "$@" --request "$request"
[ -z "$output" ] || set -- "$@" --output "$output"

cd "$repo_root/src"
go run ./cmd/harness-check "$@"
