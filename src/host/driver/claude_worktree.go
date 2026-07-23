package driver

import (
	"github.com/takezoh/agent-grid/host/state"
	claudecli "github.com/takezoh/agent-grid/platform/lib/claude/cli"
)

func (d ClaudeDriver) PrepareCreate(s state.DriverState, _ state.SessionID, project, command string, options state.LaunchOptions) (state.DriverState, state.CreateLaunch, error) {
	cs, ok := s.(ClaudeState)
	if !ok {
		cs = ClaudeState{}
	}
	if argv, err := splitCommandArgs(command); err == nil {
		if cfg, err := claudecli.ParseCommand(argv); err == nil {
			cs.Model = cfg.Model
			cs.Effort = cfg.Effort
		}
	}
	launch, err := CommonPrepareCreate(&cs.CommonState, project, command, options, "--worktree")
	return cs, launch, err
}
