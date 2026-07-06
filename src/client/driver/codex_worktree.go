package driver

import (
	"github.com/takezoh/agent-grid/client/state"
	"github.com/takezoh/agent-grid/platform/agentlaunch"
	libcodex "github.com/takezoh/agent-grid/platform/lib/codex"
)

func (d CodexDriver) PrepareCreate(s state.DriverState, _ state.SessionID, project, command string, options state.LaunchOptions) (state.DriverState, state.CreateLaunch, error) {
	cs, ok := s.(CodexState)
	if !ok {
		cs = CodexState{}
	}
	if argv, err := agentlaunch.SplitArgs(command); err == nil {
		if cfg, err := libcodex.ParseCommand(argv); err == nil {
			cs.Model = cfg.Model
			cs.Effort = cfg.Effort
		}
	}
	launch, err := CommonPrepareCreate(&cs.CommonState, project, command, options, "--worktree")
	return cs, launch, err
}
