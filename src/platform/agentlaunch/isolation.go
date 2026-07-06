package agentlaunch

import (
	"github.com/takezoh/agent-grid/platform/config"
	"github.com/takezoh/agent-grid/platform/sandbox"
)

// IsolationInputs are the resolved, side-effect-free inputs to the shared-vs-
// project decision. The launcher's shell gathers them (filesystem stat for the
// project's own devcontainer, scope resolution, ~-expansion); DecideIsolation is
// a pure function of them, so the precedence ladder can be exercised by table
// tests without any I/O.
type IsolationInputs struct {
	// HasOwnDevcontainer is true when <project>/.devcontainer/devcontainer.json
	// exists. It forces per-project isolation with auto-discovery and wins over
	// every scope setting.
	HasOwnDevcontainer bool
	// ProjectScope is the project-scope sandbox config (nil when none).
	ProjectScope *config.SandboxConfig
	// UserScope is the user-scope sandbox config.
	UserScope config.SandboxConfig
	// ProjectDevcontainerDir / UserDevcontainerDir are the ~-expanded
	// devcontainer.json directory overrides for each scope ("" = auto-discover).
	ProjectDevcontainerDir string
	UserDevcontainerDir    string
}

// DecideIsolation resolves the effective IsolationPlan from already-gathered
// inputs. Precedence (first match wins):
//
//  1. the project ships its own .devcontainer                         -> project, auto-discover
//  2. project scope forces project (isolation=project, or a dc path)  -> project
//  3. user scope opts into sharing (isolation=shared)                 -> shared
//  4. default                                                         -> project
func DecideIsolation(in IsolationInputs) sandbox.IsolationPlan {
	switch {
	case in.HasOwnDevcontainer:
		return sandbox.IsolationPlan{Kind: sandbox.IsolationProject}
	case in.ProjectScope != nil &&
		(in.ProjectScope.Isolation == "project" || in.ProjectDevcontainerDir != ""):
		return sandbox.IsolationPlan{
			Kind:            sandbox.IsolationProject,
			DevcontainerDir: in.ProjectDevcontainerDir,
		}
	case in.UserScope.Isolation == "shared":
		return sandbox.IsolationPlan{
			Kind:            sandbox.IsolationShared,
			DevcontainerDir: in.UserDevcontainerDir,
		}
	default:
		return sandbox.IsolationPlan{Kind: sandbox.IsolationProject}
	}
}
