package runtime

import (
	"path/filepath"

	"github.com/takezoh/agent-grid/host/state"
)

// ResolveWorkspaceRoot returns the absolute workspace root for the active frame
// using the SSOT triple: worktree StartDir (when WorktreeName is set) >
// driver StartDir > session/frame project.
func ResolveWorkspaceRoot(headFrame state.SessionFrame, sess state.Session) string {
	startDir, worktreeName := workspaceDirsFromDriver(headFrame.Driver)

	var candidate string
	switch {
	case worktreeName != "" && startDir != "":
		candidate = startDir
	case startDir != "":
		candidate = startDir
	case headFrame.Project != "":
		candidate = headFrame.Project
	default:
		candidate = sess.Project
	}
	if candidate == "" {
		return ""
	}
	abs, err := filepath.Abs(candidate)
	if err != nil {
		return filepath.Clean(candidate)
	}
	return filepath.Clean(abs)
}

// workspaceDirSource mirrors driver.WorkspaceDirSource structurally so the
// runtime root needs no driver/ import (depguard rule runtime-no-driver);
// interfaces are defined where they are consumed.
type workspaceDirSource interface {
	WorkspaceStartDir() string
	WorkspaceWorktreeName() string
}

func workspaceDirsFromDriver(ds state.DriverState) (startDir, worktreeName string) {
	if src, ok := ds.(workspaceDirSource); ok {
		return src.WorkspaceStartDir(), src.WorkspaceWorktreeName()
	}
	return "", ""
}
