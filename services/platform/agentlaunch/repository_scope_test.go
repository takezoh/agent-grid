package agentlaunch

import "testing"

func TestRepositoryLaunchScopeContract(t *testing.T) {
	s := RepositoryLaunchScope{SessionsRoot: "/data/repository-sessions", ClonePath: "/data/repository-sessions/clones/p1"}
	if err := ValidateRepositoryLaunchScope(s); err != nil {
		t.Fatal(err)
	}
	host, err := RepositoryMountHostPath(s)
	if err != nil || host != "/data/repository-sessions" {
		t.Fatalf("mount=%q err=%v", host, err)
	}
	cwd, err := RepositoryFrameCWD(s)
	if err != nil || cwd != "/data/repository-sessions/clones/p1" {
		t.Fatalf("cwd=%q err=%v", cwd, err)
	}
	if err := ValidateRepositoryLaunchScope(RepositoryLaunchScope{SessionsRoot: "/s", ClonePath: "/other/c"}); err == nil {
		t.Fatal("escape must fail")
	}
	if err := ValidateRepositoryLaunchScope(RepositoryLaunchScope{SessionsRoot: "/s", ClonePath: "/s"}); err == nil {
		t.Fatal("root==clone must fail")
	}
}
