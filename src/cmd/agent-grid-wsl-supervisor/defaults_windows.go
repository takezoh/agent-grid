//go:build windows

package main

import (
	"fmt"
	"os/user"
	"strings"
	"unicode/utf16"

	"github.com/takezoh/agent-grid/platform/wslhost"
	"golang.org/x/sys/windows/registry"
)

type windowsDefaultSource struct{}

func newDefaultSource() defaultSource { return windowsDefaultSource{} }

func (windowsDefaultSource) Defaults() (wslhost.Defaults, error) {
	current, err := user.Current()
	if err != nil {
		return wslhost.Defaults{}, fmt.Errorf("resolve registration identity: %w", err)
	}
	distribution, err := resolveDefaultWSLDistribution()
	if err != nil {
		return wslhost.Defaults{}, err
	}
	return wslhost.Defaults{
		WindowsIdentity: strings.TrimSpace(current.Username),
		Distribution:    distribution,
	}, nil
}

func resolveDefaultWSLDistribution() (string, error) {
	const lxssPath = `Software\Microsoft\Windows\CurrentVersion\Lxss`
	root, err := registry.OpenKey(registry.CURRENT_USER, lxssPath, registry.QUERY_VALUE)
	if err != nil {
		return "", fmt.Errorf("resolve default WSL distribution: open Lxss registry: %w", err)
	}
	defer root.Close()
	id, _, err := root.GetStringValue("DefaultDistribution")
	if err != nil {
		return "", fmt.Errorf("resolve default WSL distribution: read DefaultDistribution: %w", err)
	}
	distro, err := registry.OpenKey(root, id, registry.QUERY_VALUE)
	if err != nil {
		return "", fmt.Errorf("resolve default WSL distribution: open %s: %w", id, err)
	}
	defer distro.Close()
	name, _, err := distro.GetStringValue("DistributionName")
	if err != nil {
		return "", fmt.Errorf("resolve default WSL distribution: read DistributionName: %w", err)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("resolve default WSL distribution: empty DistributionName")
	}
	return name, nil
}

func decodeWSLOutput(raw []byte) string {
	if len(raw) >= 2 && (raw[1] == 0 || raw[0] == 0xff || raw[0] == 0xfe) {
		if len(raw)%2 != 0 {
			raw = raw[:len(raw)-1]
		}
		words := make([]uint16, 0, len(raw)/2)
		for i := 0; i+1 < len(raw); i += 2 {
			words = append(words, uint16(raw[i])|uint16(raw[i+1])<<8)
		}
		return strings.TrimPrefix(string(utf16.Decode(words)), "\ufeff")
	}
	return string(raw)
}
