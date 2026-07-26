#!/usr/bin/env python3
"""Capture exact PTY bytes from an interactive agent without interpreting them.

Example:
  capture-terminal-signals.py --output /tmp/claude.raw \
    --timeout 20 --send 2:68656c6c6f0d --send 8:1b -- claude

Each --send value is DELAY_SECONDS:HEX_BYTES measured from process start.
The raw file is suitable for analyze-terminal-signals.py or xxd.
"""

from __future__ import annotations

import argparse
import os
import pty
import selectors
import signal
import sys
import time


def parse_send(value: str) -> tuple[float, bytes]:
    delay, separator, raw = value.partition(":")
    if not separator:
        raise argparse.ArgumentTypeError("send must be DELAY_SECONDS:HEX_BYTES")
    try:
        return float(delay), bytes.fromhex(raw)
    except ValueError as error:
        raise argparse.ArgumentTypeError(str(error)) from error


def arguments() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", required=True)
    parser.add_argument("--timeout", type=float, default=30)
    parser.add_argument("--send", action="append", type=parse_send, default=[])
    parser.add_argument("command", nargs=argparse.REMAINDER)
    args = parser.parse_args()
    if args.command[:1] == ["--"]:
        args.command = args.command[1:]
    if not args.command:
        parser.error("command is required after --")
    return args


def main() -> int:
    args = arguments()
    pid, master = pty.fork()
    if pid == 0:
        os.execvp(args.command[0], args.command)

    started = time.monotonic()
    pending = sorted(args.send)
    selector = selectors.DefaultSelector()
    selector.register(master, selectors.EVENT_READ)
    status = 0
    with open(args.output, "wb") as capture:
        try:
            while time.monotonic() - started < args.timeout:
                elapsed = time.monotonic() - started
                while pending and pending[0][0] <= elapsed:
                    _, payload = pending.pop(0)
                    os.write(master, payload)
                for _, _ in selector.select(timeout=0.05):
                    try:
                        chunk = os.read(master, 65536)
                    except OSError:
                        chunk = b""
                    if not chunk:
                        _, raw_status = os.waitpid(pid, 0)
                        return os.waitstatus_to_exitcode(raw_status)
                    capture.write(chunk)
                    capture.flush()
                waited, raw_status = os.waitpid(pid, os.WNOHANG)
                if waited:
                    return os.waitstatus_to_exitcode(raw_status)
            os.kill(pid, signal.SIGTERM)
            time.sleep(0.2)
            waited, raw_status = os.waitpid(pid, os.WNOHANG)
            if not waited:
                os.kill(pid, signal.SIGKILL)
                _, raw_status = os.waitpid(pid, 0)
            status = os.waitstatus_to_exitcode(raw_status)
        finally:
            selector.close()
            os.close(master)
    return status


if __name__ == "__main__":
    sys.exit(main())
