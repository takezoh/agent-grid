//go:build !windows

package main

import (
	"os"
	"strings"

	"github.com/takezoh/agent-grid/platform/wslhost"
)

type envDefaultSource struct{}

func newDefaultSource() defaultSource { return envDefaultSource{} }

func (envDefaultSource) Defaults() (wslhost.Defaults, error) {
	return wslhost.Defaults{
		WindowsIdentity: strings.TrimSpace(os.Getenv("AG_WSL_DEFAULT_IDENTITY")),
		Distribution:    strings.TrimSpace(os.Getenv("AG_WSL_DEFAULT_DISTRIBUTION")),
	}, nil
}
