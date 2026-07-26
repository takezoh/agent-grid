#!/usr/bin/env python3
"""Append one JSON hook payload and return a contract-conformant response."""

from __future__ import annotations

import argparse
import fcntl
import json
import sys
from pathlib import Path


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--event", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--contract", choices=["passive", "agy"], default="passive")
    args = parser.parse_args()
    payload = json.load(sys.stdin)
    path = Path(args.output)
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("a", encoding="utf-8") as stream:
        fcntl.flock(stream, fcntl.LOCK_EX)
        stream.write(json.dumps({"event": args.event, "payload": payload}) + "\n")
        stream.flush()
        fcntl.flock(stream, fcntl.LOCK_UN)
    if args.contract == "agy" and args.event == "Stop":
        json.dump({"decision": "stop"}, sys.stdout)
    else:
        json.dump({}, sys.stdout)


if __name__ == "__main__":
    main()
