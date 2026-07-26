#!/usr/bin/env python3
"""List OSC and standalone BEL records from an exact PTY byte capture."""

from __future__ import annotations

import argparse
import json
from pathlib import Path

ESC = 0x1B
BEL = 0x07


def scan(data: bytes) -> list[dict[str, object]]:
    records: list[dict[str, object]] = []
    index = 0
    while index < len(data):
        if data[index : index + 2] == b"\x1b]":
            start = index
            index += 2
            payload_start = index
            terminator = ""
            while index < len(data):
                if data[index] == BEL:
                    terminator = "BEL"
                    payload = data[payload_start:index]
                    index += 1
                    break
                if data[index : index + 2] == b"\x1b\\":
                    terminator = "ST"
                    payload = data[payload_start:index]
                    index += 2
                    break
                index += 1
            else:
                payload = data[payload_start:]
                terminator = "unterminated"
            command, separator, body = payload.partition(b";")
            records.append(
                {
                    "offset": start,
                    "kind": "osc",
                    "command": command.decode("ascii", "backslashreplace"),
                    "payload": body.decode("utf-8", "backslashreplace") if separator else "",
                    "terminator": terminator,
                }
            )
            continue
        if data[index] == BEL:
            records.append({"offset": index, "kind": "bare_bel"})
        index += 1
    return records


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("capture")
    args = parser.parse_args()
    print(json.dumps(scan(Path(args.capture).read_bytes()), indent=2, ensure_ascii=False))


if __name__ == "__main__":
    main()
