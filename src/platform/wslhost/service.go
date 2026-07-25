package wslhost

import (
	"context"
	"errors"
	"sync"
	"time"
)

type ServiceCmd uint32

const (
	CmdStop        ServiceCmd = 1
	CmdInterrogate ServiceCmd = 4
	CmdShutdown    ServiceCmd = 5
)

type ServiceState uint32

const (
	StateStopped      ServiceState = 1
	StateStartPending ServiceState = 2
	StateStopPending  ServiceState = 3
	StateRunning      ServiceState = 4
)

type ServiceStatus struct {
	State         ServiceState
	AcceptsStop   bool
	CheckPoint    uint32
	WaitHintMS    uint32
	Win32ExitCode uint32
}

type ServiceControl struct{ Cmd ServiceCmd }

// SCM is the injectable Windows Service Control Manager seam.
type SCM interface {
	Publish(status ServiceStatus) error
	Controls() <-chan ServiceControl
}

// FakeSCM is the in-process SCM fake: control channel + status recorder.
type FakeSCM struct {
	mu       sync.Mutex
	Statuses []ServiceStatus
	ctrl     chan ServiceControl
}

func NewFakeSCM() *FakeSCM { return &FakeSCM{ctrl: make(chan ServiceControl, 8)} }

func (f *FakeSCM) Publish(status ServiceStatus) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Statuses = append(f.Statuses, status)
	return nil
}

func (f *FakeSCM) Controls() <-chan ServiceControl { return f.ctrl }
func (f *FakeSCM) Send(cmd ServiceCmd)             { f.ctrl <- ServiceControl{Cmd: cmd} }

func (f *FakeSCM) SnapshotStatuses() []ServiceStatus {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]ServiceStatus(nil), f.Statuses...)
}

func (f *FakeSCM) LastStatus() (ServiceStatus, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.Statuses) == 0 {
		return ServiceStatus{}, false
	}
	return f.Statuses[len(f.Statuses)-1], true
}

// StatusRecorder publishes stop-pending checkpoints to SCM.
type StatusRecorder struct{ SCM SCM }

func (r StatusRecorder) OnCheckpoint(cp StopCheckpoint, waitHintMS uint32, checkpoint uint32) {
	if r.SCM == nil {
		return
	}
	_ = r.SCM.Publish(ServiceStatus{State: StateStopPending, CheckPoint: checkpoint, WaitHintMS: waitHintMS})
	_ = cp
}

// ServiceHost binds lifecycle start, serialized stop, and SCM status.
type ServiceHost struct {
	Snap         Snapshot
	SCM          SCM
	Sup          *Supervisor
	Stop         *StopCoordinator
	Record       Recorder
	NotifyResume func(time.Time)
	RunLifecycle func(ctx context.Context)
}

func NewServiceHost(snap Snapshot, deps Deps, scm SCM, source StopSessionSource) (*ServiceHost, error) {
	sup, err := NewSupervisor(snap, deps)
	if err != nil {
		return nil, err
	}
	if source == nil {
		source = FuncStopSession(func() StopSessionState {
			return StopSessionState{OwnershipToken: sup.OwnershipToken(), Unit: sup.Unit(), Generation: snap.Generation}
		})
	}
	stop, err := NewStopCoordinator(snap, deps.Host, deps.WSL, source)
	if err != nil {
		return nil, err
	}
	if scm != nil {
		stop.Status = StatusRecorder{SCM: scm}
	}
	sup.SetRecoveryStop(func(ctx context.Context) StopOutcome {
		return stop.Run(ctx, StopPathRecovery)
	})
	host := &ServiceHost{Snap: snap, SCM: scm, Sup: sup, Stop: stop, Record: deps.Record}
	if notifier, ok := deps.Resume.(interface{ NotifyResume(time.Time) }); ok {
		host.NotifyResume = notifier.NotifyResume
	}
	return host, nil
}

func (h *ServiceHost) Serve(ctx context.Context) error {
	if h.SCM == nil {
		return errors.New("wslhost: SCM is required")
	}
	_ = h.SCM.Publish(ServiceStatus{State: StateStartPending, WaitHintMS: 30_000})
	if h.RunLifecycle != nil {
		h.RunLifecycle(ctx)
	} else if h.Sup != nil {
		h.Sup.Start(ctx)
	}
	_ = h.SCM.Publish(ServiceStatus{State: StateRunning, AcceptsStop: true})
	for {
		select {
		case <-ctx.Done():
			return h.finishStop(context.WithoutCancel(ctx))
		case ctl, ok := <-h.SCM.Controls():
			if !ok {
				return h.finishStop(ctx)
			}
			switch ctl.Cmd {
			case CmdStop, CmdShutdown:
				return h.finishStop(ctx)
			case CmdInterrogate:
				_ = h.SCM.Publish(ServiceStatus{State: StateRunning, AcceptsStop: true})
			}
		}
	}
}

func (h *ServiceHost) finishStop(ctx context.Context) error {
	if h.Stop == nil {
		return h.SCM.Publish(ServiceStatus{State: StateStopped})
	}
	_ = h.SCM.Publish(ServiceStatus{State: StateStopPending, CheckPoint: 1, WaitHintMS: uint32(h.Stop.GracefulBudget().Milliseconds())})
	out := h.Stop.Run(ctx, StopPathSCM)
	h.RecordStopOutcome(out)
	st := ServiceStatus{State: StateStopped}
	if out.Rejected && !out.Completed {
		st.Win32ExitCode = 1
	}
	return h.SCM.Publish(st)
}

// HandleControl processes a single control without the full Serve loop.
func (h *ServiceHost) HandleControl(ctx context.Context, cmd ServiceCmd) (StopOutcome, error) {
	if cmd != CmdStop && cmd != CmdShutdown {
		return StopOutcome{}, nil
	}
	if h.SCM != nil && h.Stop != nil {
		_ = h.SCM.Publish(ServiceStatus{State: StateStopPending, CheckPoint: 1, WaitHintMS: uint32(h.Stop.GracefulBudget().Milliseconds())})
	}
	if h.Stop == nil {
		return StopOutcome{}, errors.New("wslhost: stop coordinator missing")
	}
	out := h.Stop.Run(ctx, StopPathSCM)
	h.RecordStopOutcome(out)
	if h.SCM != nil {
		st := ServiceStatus{State: StateStopped}
		if out.Rejected && !out.Completed {
			st.Win32ExitCode = 1
		}
		_ = h.SCM.Publish(st)
	}
	return out, nil
}

// RecordStopOutcome preserves stop, budget, and escalation observability across
// both the portable ServiceHost loop and the Windows SCM adapter.
func (h *ServiceHost) RecordStopOutcome(out StopOutcome) {
	if h == nil || h.Record == nil || out.Duplicate {
		return
	}
	now := time.Now().UTC()
	h.Record.Record(LifecycleEvent{
		At: now, Kind: KindServiceStop, Generation: h.Snap.Generation,
		Class: out.Class, Reason: out.Reason,
	})
	if stopHasCheckpoint(out, CheckpointBudgetExpired) {
		h.Record.Record(LifecycleEvent{
			At: now, Kind: KindBudgetExpiry, Generation: h.Snap.Generation,
			Reason: out.Reason, DelayMS: out.BudgetMS,
		})
	}
	if out.Escalation != "" {
		h.Record.Record(LifecycleEvent{
			At: now, Kind: KindEscalation, Generation: h.Snap.Generation,
			Reason: out.Escalation, Setting: out.TerminateTarget,
		})
	}
}

func stopHasCheckpoint(out StopOutcome, want StopCheckpoint) bool {
	for _, checkpoint := range out.Checkpoints {
		if checkpoint == want {
			return true
		}
	}
	return false
}
