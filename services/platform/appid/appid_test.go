package appid

import "testing"

func TestShimPathDerivation(t *testing.T) {
	if got, want := HostExecShimsPath, ContainerRunDir+"/hostexec-shims"; got != want {
		t.Errorf("HostExecShimsPath = %q, want %q", got, want)
	}
	if got, want := SecretEnvShimsPath, ContainerRunDir+"/secretenv-shims"; got != want {
		t.Errorf("SecretEnvShimsPath = %q, want %q", got, want)
	}
	if got, want := HostExecShimsPath, ContainerRunDir+"/"+HostExecShimsDir; got != want {
		t.Errorf("HostExecShimsPath not derived from HostExecShimsDir: got %q, want %q", got, want)
	}
	if got, want := SecretEnvShimsPath, ContainerRunDir+"/"+SecretEnvShimsDir; got != want {
		t.Errorf("SecretEnvShimsPath not derived from SecretEnvShimsDir: got %q, want %q", got, want)
	}
}

func TestRuntimeAuthoritativePathList_OrderAndContent(t *testing.T) {
	got := RuntimeAuthoritativePathList()
	want := []string{HostExecShimsPath, SecretEnvShimsPath}
	if len(got) != len(want) {
		t.Fatalf("length = %d, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("[%d] = %q, want %q", i, got[i], w)
		}
	}
}

func TestRuntimeAuthoritativePathList_NonEmpty(t *testing.T) {
	got := RuntimeAuthoritativePathList()
	if len(got) == 0 {
		t.Fatal("RuntimeAuthoritativePathList returned empty slice; case-D invariant requires non-empty")
	}
	for i, p := range got {
		if p == "" {
			t.Errorf("[%d] is empty string", i)
		}
	}
}

// TestRuntimeAuthoritativePathList_FreshSlice pins the invariant that mutating
// the returned slice does not affect subsequent callers. Without this guard,
// a caller in framelaunch could accidentally corrupt the shim list for every
// other frame launch in the process.
func TestRuntimeAuthoritativePathList_FreshSlice(t *testing.T) {
	first := RuntimeAuthoritativePathList()
	first[0] = "/tmp/evil"
	second := RuntimeAuthoritativePathList()
	if second[0] == "/tmp/evil" {
		t.Fatalf("RuntimeAuthoritativePathList returned aliased slice; second[0] = %q (mutation from first call leaked)", second[0])
	}
	if second[0] != HostExecShimsPath {
		t.Errorf("second[0] = %q, want %q (untouched by first-call mutation)", second[0], HostExecShimsPath)
	}
}
