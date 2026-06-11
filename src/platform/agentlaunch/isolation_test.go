package agentlaunch

import (
	"testing"

	"github.com/takezoh/agent-reactor/platform/config"
	"github.com/takezoh/agent-reactor/platform/sandbox"
)

func TestDecideIsolation(t *testing.T) {
	shared := config.SandboxConfig{Isolation: "shared"}
	cases := []struct {
		name       string
		in         IsolationInputs
		wantShared bool
		wantDCDir  string
	}{
		{
			name:       "default is project isolation",
			in:         IsolationInputs{UserScope: config.SandboxConfig{}},
			wantShared: false,
		},
		{
			name:       "user shared opts into sharing",
			in:         IsolationInputs{UserScope: shared, UserDevcontainerDir: "/u/dc"},
			wantShared: true,
			wantDCDir:  "/u/dc",
		},
		{
			name: "project's own devcontainer forces project even under user shared",
			in: IsolationInputs{
				HasOwnDevcontainer:  true,
				UserScope:           shared,
				UserDevcontainerDir: "/u/dc",
			},
			wantShared: false,
			wantDCDir:  "", // auto-discover the project's own .devcontainer
		},
		{
			// Pins precedence: the project's own .devcontainer must win even over a
			// project-scope override that would otherwise force project with a dir.
			// Guards against reordering case 1 below case 2 in DecideIsolation.
			name: "own devcontainer wins over project-scope forcing project",
			in: IsolationInputs{
				HasOwnDevcontainer:     true,
				ProjectScope:           &config.SandboxConfig{Isolation: "project"},
				ProjectDevcontainerDir: "/p/dc",
				UserScope:              shared,
			},
			wantShared: false,
			wantDCDir:  "", // own .devcontainer auto-discovered; project-scope dir ignored
		},
		{
			name: "project scope isolation=project wins over user shared",
			in: IsolationInputs{
				ProjectScope: &config.SandboxConfig{Isolation: "project"},
				UserScope:    shared,
			},
			wantShared: false,
		},
		{
			name: "project scope devcontainer path forces project (with dir)",
			in: IsolationInputs{
				ProjectScope:           &config.SandboxConfig{},
				ProjectDevcontainerDir: "/p/dc",
				UserScope:              shared,
			},
			wantShared: false,
			wantDCDir:  "/p/dc",
		},
		{
			name: "project scope present but neutral falls through to user shared",
			in: IsolationInputs{
				ProjectScope:        &config.SandboxConfig{},
				UserScope:           shared,
				UserDevcontainerDir: "/u/dc",
			},
			wantShared: true,
			wantDCDir:  "/u/dc",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan := DecideIsolation(tc.in)
			if plan.IsShared() != tc.wantShared {
				t.Errorf("IsShared() = %v, want %v (plan=%+v)", plan.IsShared(), tc.wantShared, plan)
			}
			if plan.DevcontainerDir != tc.wantDCDir {
				t.Errorf("DevcontainerDir = %q, want %q", plan.DevcontainerDir, tc.wantDCDir)
			}
		})
	}
}

// TestDecideIsolation_ContainerKeyMatchesPlan checks the producer→consumer
// round-trip: a shared decision yields the shared sentinel key, a project
// decision keys by the project path — the property the manager and overlay rely
// on for a single, drift-free instance identity.
func TestDecideIsolation_ContainerKeyMatchesPlan(t *testing.T) {
	const project = "/workspace/myapp"

	sharedPlan := DecideIsolation(IsolationInputs{UserScope: config.SandboxConfig{Isolation: "shared"}})
	if got := sharedPlan.ContainerKey(project); got != sandbox.SharedInstanceKey {
		t.Errorf("shared ContainerKey = %q, want %q", got, sandbox.SharedInstanceKey)
	}

	projectPlan := DecideIsolation(IsolationInputs{UserScope: config.SandboxConfig{}})
	if got := projectPlan.ContainerKey(project); got != project {
		t.Errorf("project ContainerKey = %q, want %q", got, project)
	}
}
