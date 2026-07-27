package agentlaunch

import "testing"

func TestWrappedLaunchHostPath(t *testing.T) {
	t.Run("no mounts returns false (host mode)", func(t *testing.T) {
		if _, ok := (WrappedLaunch{}).HostPath(ContainerRunDir + "/codex-x.sock"); ok {
			t.Fatal("HostPath with no mounts should return false")
		}
	})

	t.Run("translates a path under a mount to its host path", func(t *testing.T) {
		w := WrappedLaunch{Mounts: []Mount{{Host: "/h/run/abc", Container: ContainerRunDir}}}
		got, ok := w.HostPath(ContainerRunDir + "/codex-x.sock")
		if !ok {
			t.Fatal("HostPath should resolve a path under the mount")
		}
		if want := "/h/run/abc/codex-x.sock"; got != want {
			t.Errorf("HostPath = %q, want %q", got, want)
		}
	})

	t.Run("path outside every mount returns false", func(t *testing.T) {
		w := WrappedLaunch{Mounts: []Mount{{Host: "/h/run/abc", Container: ContainerRunDir}}}
		if _, ok := w.HostPath("/var/other.sock"); ok {
			t.Fatal("HostPath outside mounts should return false")
		}
	})
}
