//go:build windows

package wslhost

import (
	"context"
	"time"

	"golang.org/x/sys/windows/svc"
)

const WindowsServiceName = "AgentGridWSLHost"

func RunWindowsService(serviceName string, host *ServiceHost) error {
	return svc.Run(serviceName, &windowsService{host: host})
}

type windowsService struct{ host *ServiceHost }

func (ws *windowsService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	const accepts = svc.AcceptStop | svc.AcceptShutdown | svc.AcceptPowerEvent
	changes <- svc.Status{State: svc.StartPending}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	scm := &bridgeSCM{changes: changes, accepts: accepts}
	ws.host.SCM = scm
	if ws.host.Stop != nil {
		ws.host.Stop.Status = StatusRecorder{SCM: scm}
	}
	if ws.host.RunLifecycle != nil {
		ws.host.RunLifecycle(ctx)
	} else if ws.host.Sup != nil {
		ws.host.Sup.Start(ctx)
	}
	changes <- svc.Status{State: svc.Running, Accepts: accepts}
	for c := range r {
		switch c.Cmd {
		case svc.Interrogate:
			changes <- c.CurrentStatus
		case svc.PowerEvent:
			if ws.host.NotifyResume != nil && isResumePowerEvent(c.EventType) {
				ws.host.NotifyResume(time.Now())
			}
		case svc.Stop, svc.Shutdown:
			hint := uint32(15_000)
			if ws.host.Stop != nil {
				hint = uint32(ws.host.Stop.GracefulBudget() / time.Millisecond)
			}
			changes <- svc.Status{State: svc.StopPending, WaitHint: hint}
			if ws.host.Stop == nil {
				changes <- svc.Status{State: svc.Stopped}
				return false, 0
			}
			out := ws.host.Stop.Run(ctx, StopPathSCM)
			ws.host.RecordStopOutcome(out)
			changes <- svc.Status{State: svc.Stopped}
			if out.Rejected && !out.Completed {
				return false, 1
			}
			return false, 0
		}
	}
	changes <- svc.Status{State: svc.Stopped}
	return false, 0
}

func isResumePowerEvent(eventType uint32) bool {
	const (
		pbtAPMResumeCritical  = 6
		pbtAPMResumeSuspend   = 7
		pbtAPMResumeAutomatic = 18
	)
	return eventType == pbtAPMResumeCritical ||
		eventType == pbtAPMResumeSuspend ||
		eventType == pbtAPMResumeAutomatic
}

type bridgeSCM struct {
	changes chan<- svc.Status
	accepts svc.Accepted
}

func (b *bridgeSCM) Publish(status ServiceStatus) error {
	st := svc.Status{State: mapState(status.State), CheckPoint: status.CheckPoint, WaitHint: status.WaitHintMS}
	if status.AcceptsStop || status.State == StateRunning {
		st.Accepts = b.accepts
	}
	b.changes <- st
	return nil
}

func (b *bridgeSCM) Controls() <-chan ServiceControl { return nil }

func mapState(s ServiceState) svc.State {
	switch s {
	case StateStopped:
		return svc.Stopped
	case StateStartPending:
		return svc.StartPending
	case StateStopPending:
		return svc.StopPending
	case StateRunning:
		return svc.Running
	default:
		return svc.Stopped
	}
}
