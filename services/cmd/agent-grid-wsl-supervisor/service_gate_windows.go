//go:build windows

package main

import (
	"errors"
	"fmt"

	"github.com/takezoh/agent-grid/services/platform/wslhost"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

type scmServiceGate struct{}

func newServiceGate() wslhost.ServiceGate { return scmServiceGate{} }

func (scmServiceGate) Stopped() (bool, error) {
	manager, err := mgr.Connect()
	if err != nil {
		return false, fmt.Errorf("connect service manager: %w", err)
	}
	defer manager.Disconnect()
	service, err := manager.OpenService(wslhost.WindowsServiceName)
	if err != nil {
		if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			return true, nil
		}
		return false, fmt.Errorf("open service %s: %w", wslhost.WindowsServiceName, err)
	}
	defer service.Close()
	status, err := service.Query()
	if err != nil {
		return false, fmt.Errorf("query service %s: %w", wslhost.WindowsServiceName, err)
	}
	return status.State == svc.Stopped, nil
}
