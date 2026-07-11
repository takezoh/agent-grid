package grok

import (
	"testing"

	"github.com/takezoh/agent-grid/platform/agentlaunch"
	"github.com/takezoh/agent-grid/platform/lib/grok/fake"
)

func TestGrokLifecycleContract_SelectorShape(t *testing.T) {
	for _, lifecycle := range []Lifecycle{LifecycleFresh, LifecycleContinue, LifecycleResume, LifecycleFork} {
		command, err := BuildCommand("grok", lifecycle, "01234567-89ab-4cde-af01-23456789abcd", "", "")
		if err != nil {
			t.Fatal(err)
		}
		argv, err := agentlaunch.SplitArgs(command)
		if err != nil {
			t.Fatal(err)
		}
		if err := fake.ValidateArgv(argv); err != nil {
			t.Fatalf("%q violates lifecycle contract: %v", command, err)
		}
	}
	command, err := BuildForkCommand("grok", "parent", "child", "", "")
	if err != nil {
		t.Fatal(err)
	}
	argv, err := agentlaunch.SplitArgs(command)
	if err != nil {
		t.Fatal(err)
	}
	if err := fake.ValidateArgv(argv); err != nil {
		t.Fatalf("%q violates fork lifecycle contract: %v", command, err)
	}
}
