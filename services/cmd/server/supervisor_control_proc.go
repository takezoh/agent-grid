package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// readProcessIdentity loads executable, argv, and the Linux start token for pid
// from /proc. The start token is "pid:starttime" where starttime is field 22 of
// /proc/<pid>/stat — binding stop authority to a specific process incarnation
// so PID reuse cannot inherit signal rights.
func readProcessIdentity(pid int) (processIdentity, error) {
	if pid <= 0 {
		return processIdentity{}, fmt.Errorf("invalid pid %d", pid)
	}
	exe, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
	if err != nil {
		return processIdentity{}, fmt.Errorf("read exe: %w", err)
	}
	// Kernel appends " (deleted)" when the binary was unlinked; strip for match.
	exe = strings.TrimSuffix(exe, " (deleted)")

	cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return processIdentity{}, fmt.Errorf("read cmdline: %w", err)
	}
	argv := splitCmdline(cmdline)

	startToken, err := readStartToken(pid)
	if err != nil {
		return processIdentity{}, err
	}
	return processIdentity{
		PID:        pid,
		Executable: exe,
		Argv:       argv,
		StartToken: startToken,
	}, nil
}

func readStartToken(pid int) (string, error) {
	stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return "", fmt.Errorf("read stat: %w", err)
	}
	starttime, err := parseProcStatStarttime(string(stat))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d:%s", pid, starttime), nil
}

// parseProcStatStarttime extracts field 22 (starttime) from /proc/<pid>/stat.
// Field 2 (comm) is parenthesized and may contain spaces, so we locate the
// final ')' and index from the remaining fields.
func parseProcStatStarttime(stat string) (string, error) {
	rparen := strings.LastIndexByte(stat, ')')
	if rparen < 0 || rparen+2 >= len(stat) {
		return "", fmt.Errorf("malformed /proc stat")
	}
	fields := strings.Fields(stat[rparen+2:])
	// After comm: state(3) … starttime(22) → index 19 in the remaining fields.
	const starttimeIndex = 19
	if len(fields) <= starttimeIndex {
		return "", fmt.Errorf("malformed /proc stat: short fields")
	}
	if _, err := strconv.ParseUint(fields[starttimeIndex], 10, 64); err != nil {
		return "", fmt.Errorf("malformed starttime: %w", err)
	}
	return fields[starttimeIndex], nil
}

func splitCmdline(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	// cmdline is NUL-separated; trailing NUL yields a final empty element.
	parts := strings.Split(string(raw), "\x00")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func signalProcess(pid int, sig unix.Signal) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid %d", pid)
	}
	if err := unix.Kill(pid, sig); err != nil {
		return fmt.Errorf("kill %d: %w", pid, err)
	}
	return nil
}
