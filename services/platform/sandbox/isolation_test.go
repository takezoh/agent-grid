package sandbox

import "testing"

func TestIsolationPlan_ContainerKey(t *testing.T) {
	cases := []struct {
		name        string
		plan        IsolationPlan
		projectPath string
		want        string
	}{
		{"project keys by path", IsolationPlan{Kind: IsolationProject}, "/workspace/myapp", "/workspace/myapp"},
		{"project with empty path", IsolationPlan{Kind: IsolationProject}, "", ""},
		{"shared collapses to sentinel", IsolationPlan{Kind: IsolationShared}, "/workspace/fintech", SharedInstanceKey},
		{"shared ignores path", IsolationPlan{Kind: IsolationShared}, "", SharedInstanceKey},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.plan.ContainerKey(tc.projectPath); got != tc.want {
				t.Errorf("ContainerKey(%q) = %q, want %q", tc.projectPath, got, tc.want)
			}
		})
	}
}

// TestIsolationPlan_OverlayProjectEqualsContainerKey pins the invariant whose
// violation broke gh/ssh inside shared containers: the proxy socket directory
// (keyed by OverlayProject) must resolve to the same key as the run-dir bind
// (keyed by ContainerKey). Encoding OverlayProject as ContainerKey makes the
// divergence impossible, but the test guards against a future refactor that
// re-splits them.
func TestIsolationPlan_OverlayProjectEqualsContainerKey(t *testing.T) {
	projects := []string{"/workspace/fintech", "", "/a/b/c"}
	for _, kind := range []IsolationKind{IsolationProject, IsolationShared} {
		plan := IsolationPlan{Kind: kind}
		for _, p := range projects {
			if plan.OverlayProject(p) != plan.ContainerKey(p) {
				t.Errorf("kind=%d project=%q: OverlayProject=%q != ContainerKey=%q",
					kind, p, plan.OverlayProject(p), plan.ContainerKey(p))
			}
		}
	}
}

func TestIsolationPlan_WorkspaceFallbackProject(t *testing.T) {
	if got := (IsolationPlan{Kind: IsolationProject}).WorkspaceFallbackProject("/p"); got != "/p" {
		t.Errorf("project fallback = %q, want /p", got)
	}
	if got := (IsolationPlan{Kind: IsolationShared}).WorkspaceFallbackProject("/p"); got != "" {
		t.Errorf("shared fallback = %q, want empty", got)
	}
}

func TestIsolationPlan_FrameWorkspaceMount(t *testing.T) {
	host, container := (IsolationPlan{Kind: IsolationProject}).FrameWorkspaceMount("/p", "/ws")
	if host != "/p" || container != "/ws" {
		t.Errorf("project mount = (%q,%q), want (/p,/ws)", host, container)
	}
	host, container = (IsolationPlan{Kind: IsolationShared}).FrameWorkspaceMount("/p", "/ws")
	if host != "" || container != "" {
		t.Errorf("shared mount = (%q,%q), want empty", host, container)
	}
}

func TestIsolationPlan_IsShared(t *testing.T) {
	if !(IsolationPlan{Kind: IsolationShared}).IsShared() {
		t.Error("shared plan must report IsShared")
	}
	if (IsolationPlan{Kind: IsolationProject}).IsShared() {
		t.Error("project plan must not report IsShared")
	}
	// Zero value is project isolation.
	if (IsolationPlan{}).IsShared() {
		t.Error("zero-value plan must default to project isolation")
	}
}
