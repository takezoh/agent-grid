//go:build !windows

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/takezoh/agent-grid/services/platform/wslhost"
)

type envServiceGate struct{}

func newServiceGate() wslhost.ServiceGate { return envServiceGate{} }

func (envServiceGate) Stopped() (bool, error) {
	switch strings.TrimSpace(os.Getenv("AG_WSL_SERVICE_STOPPED")) {
	case "1", "true", "TRUE", "yes":
		return true, nil
	case "", "0", "false", "FALSE", "no":
		return false, nil
	default:
		return false, fmt.Errorf("invalid AG_WSL_SERVICE_STOPPED")
	}
}
