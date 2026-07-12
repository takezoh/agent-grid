package driver

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/takezoh/agent-grid/client/state"
	libgrok "github.com/takezoh/agent-grid/platform/lib/grok"
	"github.com/takezoh/fishpath-go"
)

const (
	GrokDriverName   = libgrok.DriverName
	grokKeySessionID = "grok_session_id"
	grokKeyModel     = "grok_model"
	grokKeyEffort    = "grok_effort"
)

type GrokState struct {
	CommonState
	GrokSessionID string
	ForkParentID  string
	Model         string
	Effort        string
}

type GrokDriver struct{ eventLogDir string }

func NewGrokDriver(eventLogDir string) GrokDriver          { return GrokDriver{eventLogDir: eventLogDir} }
func (GrokDriver) Name() string                            { return GrokDriverName }
func (GrokDriver) DisplayName() string                     { return GrokDriverName }
func (GrokDriver) Status(s state.DriverState) state.Status { return s.(GrokState).Status }

func (GrokDriver) StartDir(s state.DriverState) string {
	if gs, ok := s.(GrokState); ok {
		return gs.StartDir
	}
	return ""
}
func (GrokDriver) WithStartDir(s state.DriverState, dir string) state.DriverState {
	if gs, ok := s.(GrokState); ok {
		gs.StartDir = dir
		return gs
	}
	return s
}

func (d GrokDriver) NewState(now time.Time) state.DriverState {
	return GrokState{CommonState: CommonState{Status: state.StatusIdle, StatusChangedAt: now}}
}

func (d GrokDriver) View(s state.DriverState) state.View {
	gs, _ := s.(GrokState)
	return state.View{Card: state.Card{Title: resolveCardTitle(gs.Title, gs.Summary), Tags: CommonTags(gs.CommonState), BorderTitle: CommandTag(GrokDriverName), BorderBadge: fishpath.Shorten(gs.StartDir, "")}, DisplayName: GrokDriverName, Model: gs.Model, Effort: gs.Effort, Status: gs.Status, StatusChangedAt: gs.StatusChangedAt, InfoExtras: grokInfo(gs)}
}

func grokInfo(gs GrokState) []state.InfoLine {
	values := []struct{ label, value string }{{"Grok Session", gs.GrokSessionID}, {"Working Dir", gs.StartDir}, {"Model", gs.Model}, {"Effort", gs.Effort}}
	out := make([]state.InfoLine, 0, len(values))
	for _, v := range values {
		if v.value != "" {
			out = append(out, state.InfoLine{Label: v.label, Value: v.value})
		}
	}
	return out
}

func (d GrokDriver) PrepareCreate(s state.DriverState, sessionID state.SessionID, project, command string, options state.LaunchOptions) (state.DriverState, state.CreateLaunch, error) {
	gs, _ := s.(GrokState)
	cfg, err := libgrok.ParseCommand(command)
	if err != nil {
		return s, state.CreateLaunch{}, err
	}
	gs.Model, gs.Effort = cfg.Model, cfg.Effort
	gs.GrokSessionID = grokUUID(sessionID)
	launch, err := CommonPrepareCreate(&gs.CommonState, project, command, options, "--worktree")
	if err != nil {
		return gs, launch, err
	}
	launch.Command, err = libgrok.BuildCommand(launch.Command, libgrok.LifecycleFresh, gs.GrokSessionID, gs.Model, gs.Effort)
	return gs, launch, err
}

func (d GrokDriver) PrepareLaunch(s state.DriverState, mode state.LaunchMode, project, command string, options state.LaunchOptions, _ bool) (state.LaunchPlan, error) {
	gs, _ := s.(GrokState)
	startDir := project
	if gs.StartDir != "" {
		startDir = gs.StartDir
	}
	req, command := resolveWorktreeRequest(command, options, "--worktree")
	if mode == state.LaunchModeColdStart && gs.GrokSessionID != "" {
		command, _ = stripGrokLifecycle(command)
		built, err := libgrok.BuildCommand(command, libgrok.LifecycleResume, gs.GrokSessionID, gs.Model, gs.Effort)
		if err != nil {
			return state.LaunchPlan{}, err
		}
		command = built
	}
	if mode == state.LaunchModeCreate && gs.ForkParentID != "" {
		command, _ = stripGrokLifecycle(command)
		built, err := libgrok.BuildForkCommand(command, gs.ForkParentID, gs.GrokSessionID, gs.Model, gs.Effort)
		if err != nil {
			return state.LaunchPlan{}, err
		}
		command = built
	}
	return state.LaunchPlan{Command: strings.TrimSpace(command), StartDir: startDir, Options: PreserveLaunchOptions(options, req.Enabled), Stdin: options.InitialInput}, nil
}

func stripGrokLifecycle(command string) (string, error) {
	args, err := splitCommandArgs(command)
	if err != nil {
		return "", err
	}
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--session-id", "-s", "--resume", "-r":
			i++
		case "--continue", "--fork-session", "--no-auto-update":
		default:
			out = append(out, args[i])
		}
	}
	return joinCommandArgs(out), nil
}

func (d GrokDriver) Step(prev state.DriverState, ctx state.FrameContext, ev state.DriverEvent) (state.DriverState, []state.Effect, state.View) {
	gs, ok := prev.(GrokState)
	if !ok {
		gs = d.NewState(time.Time{}).(GrokState)
	}
	if !ctx.IsRoot {
		return gs, nil, d.View(gs)
	}
	switch e := ev.(type) {
	case state.DEvTick:
		return gs, gs.HandleTick(e, false), d.View(gs)
	case state.DEvWorktreeResolved:
		gs.ApplyWorktreeResolved(e)
	case state.DEvCommandExited:
		gs.ApplyCommandExited(e)
	}
	return gs, nil, d.View(gs)
}

func (GrokDriver) Persist(s state.DriverState) map[string]string {
	gs, ok := s.(GrokState)
	if !ok {
		return nil
	}
	out := make(map[string]string)
	gs.PersistCommon(out)
	if gs.GrokSessionID != "" {
		out[grokKeySessionID] = gs.GrokSessionID
	}
	if gs.ForkParentID != "" {
		out["grok_fork_parent_id"] = gs.ForkParentID
	}
	if gs.Model != "" {
		out[grokKeyModel] = gs.Model
	}
	if gs.Effort != "" {
		out[grokKeyEffort] = gs.Effort
	}
	return out
}

func (d GrokDriver) Restore(bag map[string]string, now time.Time) state.DriverState {
	gs := d.NewState(now).(GrokState)
	gs.RestoreCommon(bag)
	gs.GrokSessionID = bag[grokKeySessionID]
	gs.ForkParentID = bag["grok_fork_parent_id"]
	gs.Model = bag[grokKeyModel]
	gs.Effort = bag[grokKeyEffort]
	return gs
}

func (d GrokDriver) RecoverableOnColdStart(s state.DriverState) bool {
	gs, ok := s.(GrokState)
	return ok && gs.GrokSessionID != ""
}
func (d GrokDriver) ForkCommand(s state.DriverState, base string) (string, bool) {
	gs, ok := s.(GrokState)
	if !ok || gs.GrokSessionID == "" {
		return "", false
	}
	command, err := libgrok.BuildCommand(base, libgrok.LifecycleFork, gs.GrokSessionID, gs.Model, gs.Effort)
	return command, err == nil
}
func (d GrokDriver) ForkChildState(parent state.DriverState, now time.Time) state.DriverState {
	child := d.NewState(now).(GrokState)
	if p, ok := parent.(GrokState); ok {
		child.Model, child.Effort = p.Model, p.Effort
		child.ForkParentID = p.GrokSessionID
	}
	return child
}
func (d GrokDriver) WithForkSessionID(s state.DriverState, id state.SessionID) state.DriverState {
	gs, ok := s.(GrokState)
	if !ok {
		return s
	}
	gs.GrokSessionID = grokUUID(id)
	return gs
}

func grokUUID(id state.SessionID) string {
	sum := sha256.Sum256([]byte(id))
	raw := fmt.Sprintf("%x", sum[:16])
	return raw[:8] + "-" + raw[8:12] + "-4" + raw[13:16] + "-a" + raw[17:20] + "-" + raw[20:32]
}
