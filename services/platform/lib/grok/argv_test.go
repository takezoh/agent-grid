package grok

import "testing"

func TestBuildCommandLifecycle(t *testing.T) {
	tests := []struct {
		name, want string
		lifecycle  Lifecycle
		sessionID  string
	}{
		{"fresh", "grok --no-auto-update --session-id id", LifecycleFresh, "id"},
		{"continue", "grok --no-auto-update --continue", LifecycleContinue, ""},
		{"resume", "grok --no-auto-update --resume id", LifecycleResume, "id"},
		{"fork", "grok --no-auto-update --resume id --fork-session", LifecycleFork, "id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildCommand("grok", tt.lifecycle, tt.sessionID, "", "")
			if err != nil || got != tt.want {
				t.Fatalf("BuildCommand() = %q, %v; want %q, nil", got, err, tt.want)
			}
		})
	}
}

func TestBuildCommandRejectsConflictingSessionFlags(t *testing.T) {
	if _, err := BuildCommand("grok --resume old", LifecycleFresh, "new", "", ""); err == nil {
		t.Fatal("BuildCommand accepted conflicting lifecycle flag")
	}
}
