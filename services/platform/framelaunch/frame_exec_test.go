package framelaunch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
)

func TestLoadFrameSpec_ErrorsOnEmpty(t *testing.T) {
	_, err := loadFrameSpec("")
	if err == nil {
		t.Fatal("expected error for empty AG_FRAME_SPEC")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error = %v, want empty mention", err)
	}
}

func TestLoadFrameSpec_ParsesFullSpec(t *testing.T) {
	raw := `{"pre_exec":"mise trust","login_shell":"/bin/zsh","pre_commands":[["a","b"],["c"]],"main_command":["codex","--remote"],"pre_command_timeout":"5s"}`
	got, err := loadFrameSpec(raw)
	if err != nil {
		t.Fatalf("loadFrameSpec: %v", err)
	}
	want := FrameSpec{
		PreExec:           "mise trust",
		LoginShell:        "/bin/zsh",
		PreCommands:       [][]string{{"a", "b"}, {"c"}},
		MainCommand:       []string{"codex", "--remote"},
		PreCommandTimeout: "5s",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestParseEnv0_HandlesEmptyEntries(t *testing.T) {
	got := parseEnv0([]byte("FOO=bar\x00\x00BAZ=qux\x00"))
	if got["FOO"] != "bar" || got["BAZ"] != "qux" {
		t.Fatalf("got %#v", got)
	}
}

func TestParseEnv0_HandlesEqualsInValue(t *testing.T) {
	got := parseEnv0([]byte("FOO=a=b=c\x00"))
	if got["FOO"] != "a=b=c" {
		t.Fatalf("got %q, want a=b=c", got["FOO"])
	}
}

func TestEncode_RoundTrip(t *testing.T) {
	spec := FrameSpec{
		PreExec:     "export FOO=1",
		PreCommands: [][]string{{"true"}},
		MainCommand: []string{"echo", "hi"},
	}
	raw, err := Encode(spec)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	got, err := loadFrameSpec(raw)
	if err != nil {
		t.Fatalf("loadFrameSpec: %v", err)
	}
	if !reflect.DeepEqual(got, spec) {
		t.Fatalf("got %#v, want %#v", got, spec)
	}
}

func TestEncode_EmptyPreExecOmittedFromJSON(t *testing.T) {
	raw, err := Encode(FrameSpec{MainCommand: []string{"true"}})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if strings.Contains(raw, "pre_exec") {
		t.Fatalf("empty PreExec should be omitted: %s", raw)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("json: %v", err)
	}
	if _, ok := m["pre_exec"]; ok {
		t.Fatal("pre_exec key present in JSON")
	}
}

func TestRun_MissingMainCommand(t *testing.T) {
	restore := swapExecReplacer(t, func(string, []string, []string) error {
		t.Fatal("execReplacer must not be called")
		return nil
	})
	defer restore()

	t.Setenv(EnvVar, mustEncode(t, FrameSpec{MainCommand: nil}))
	if err := Run(); err == nil {
		t.Fatal("expected error for empty MainCommand")
	}
}

func TestRun_AllPreCommandsSucceed_CallsExecReplacer(t *testing.T) {
	var gotArgv []string
	restore := swapExecReplacer(t, func(_ string, argv []string, _ []string) error {
		gotArgv = append([]string(nil), argv...)
		return nil
	})
	defer restore()

	trueBin, err := exec.LookPath("true")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvVar, mustEncode(t, FrameSpec{
		PreCommands: [][]string{{"true"}, {"true"}},
		MainCommand: []string{"true"},
	}))
	if err := Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !reflect.DeepEqual(gotArgv, []string{"true"}) {
		t.Fatalf("argv = %#v, want [true]", gotArgv)
	}
	// LookPath may return absolute path as argv0 to exec; we only check MainCommand slice.
	_ = trueBin
}

func TestRun_PreCommandFails_SkipsExecReplacer(t *testing.T) {
	called := false
	restore := swapExecReplacer(t, func(string, []string, []string) error {
		called = true
		return nil
	})
	defer restore()

	t.Setenv(EnvVar, mustEncode(t, FrameSpec{
		PreCommands: [][]string{{"false"}},
		MainCommand: []string{"true"},
	}))
	if err := Run(); err == nil {
		t.Fatal("expected preCommand failure")
	}
	if called {
		t.Fatal("execReplacer must not run after preCommand failure")
	}
}

func TestRun_PreCommandTimeout(t *testing.T) {
	called := false
	restore := swapExecReplacer(t, func(string, []string, []string) error {
		called = true
		return nil
	})
	defer restore()

	t.Setenv(EnvVar, mustEncode(t, FrameSpec{
		PreCommands:       [][]string{{"sleep", "30"}},
		MainCommand:       []string{"true"},
		PreCommandTimeout: "50ms",
	}))
	start := time.Now()
	err := Run()
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if time.Since(start) > 5*time.Second {
		t.Fatalf("timeout took too long: %v", time.Since(start))
	}
	if called {
		t.Fatal("execReplacer must not run after timeout")
	}
}

func TestRun_PreExecEnvIsApplied_BeforeExecReplacer(t *testing.T) {
	// Fake "login shell": ignore -lc args and emit a NUL-delimited env dump.
	// Real shells write the full environment; here we only need FOO=bar.
	shell := writeFakeShell(t, `#!/bin/sh
printf 'FOO=bar\0'
`)
	var sawFOO string
	restore := swapExecReplacer(t, func(_ string, _ []string, envv []string) error {
		for _, e := range envv {
			if strings.HasPrefix(e, "FOO=") {
				sawFOO = strings.TrimPrefix(e, "FOO=")
			}
		}
		// Also check process env (Run uses os.Setenv then os.Environ).
		sawFOO = os.Getenv("FOO")
		return nil
	})
	defer restore()

	t.Setenv(EnvVar, mustEncode(t, FrameSpec{
		PreExec:     "export FOO=bar",
		LoginShell:  shell,
		MainCommand: []string{"true"},
	}))
	if err := Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if sawFOO != "bar" {
		t.Fatalf("FOO = %q, want bar", sawFOO)
	}
}

func TestResolveLoginShell_ReadsEtcPasswd(t *testing.T) {
	u, err := user.Current()
	if err != nil {
		t.Fatalf("user.Current: %v", err)
	}
	prevRead := readPasswd
	prevUser := currentUser
	t.Cleanup(func() {
		readPasswd = prevRead
		currentUser = prevUser
	})
	currentUser = func() (*user.User, error) {
		return &user.User{Username: u.Username}, nil
	}
	readPasswd = func() ([]byte, error) {
		return []byte(fmt.Sprintf("%s:x:1000:1000::/home/%s:/usr/bin/fish\n", u.Username, u.Username)), nil
	}
	if got := resolveLoginShell(); got != "/usr/bin/fish" {
		t.Fatalf("resolveLoginShell = %q, want /usr/bin/fish", got)
	}
}

func TestResolveLoginShell_FallbackOnMissingUser(t *testing.T) {
	prevRead := readPasswd
	prevUser := currentUser
	t.Cleanup(func() {
		readPasswd = prevRead
		currentUser = prevUser
	})
	currentUser = func() (*user.User, error) {
		return &user.User{Username: "no-such-user-xyz"}, nil
	}
	readPasswd = func() ([]byte, error) {
		return []byte("root:x:0:0::/root:/bin/bash\n"), nil
	}
	if got := resolveLoginShell(); got != "/bin/sh" {
		t.Fatalf("resolveLoginShell = %q, want /bin/sh", got)
	}
}

func TestCapturePreExecEnv_UsesActualShell(t *testing.T) {
	env, err := capturePreExecEnv("/bin/sh", "export FRAME_EXEC_T2_FOO=bar", 5*time.Second)
	if err != nil {
		t.Fatalf("capturePreExecEnv: %v", err)
	}
	if env["FRAME_EXEC_T2_FOO"] != "bar" {
		t.Fatalf("FRAME_EXEC_T2_FOO = %q, want bar (full dump keys=%d)", env["FRAME_EXEC_T2_FOO"], len(env))
	}
}

func TestRun_RealPtyIsInheritedByMain(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "tty-ok")
	spec := FrameSpec{
		MainCommand: []string{"sh", "-c", "test -t 0 && touch " + marker},
	}
	raw, err := Encode(spec)
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestFrameExecHelper$")
	cmd.Env = append(os.Environ(),
		EnvVar+"="+raw,
		"FRAMELAUNCH_TEST_HELPER=1",
	)
	// Use a real exec path: re-exec the test binary as frame-exec via helper.
	// The helper calls Run() which syscall.Execs main; with pty, isatty(0) is true.
	ptmx, err := pty.Start(cmd)
	if err != nil {
		t.Fatalf("pty.Start: %v", err)
	}
	defer ptmx.Close()
	_ = cmd.Wait()

	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("main did not inherit a tty (marker missing): %v", err)
	}
}

// TestFrameExecHelper is invoked as a subprocess by TestRun_RealPtyIsInheritedByMain.
// It is not a real test; it runs frame-exec when FRAMELAUNCH_TEST_HELPER=1.
// TestRun_PathReassert_PrependsRuntimeList pins FR-001 / FR-002: after preExec
// runs (and rebuilds PATH), Run() prepends the runtime authoritative list to
// PATH before handing control to execReplacer.
func TestRun_PathReassert_PrependsRuntimeList(t *testing.T) {
	shell := writeFakeShell(t, `#!/bin/sh
# Simulate a mise-activate-style rc that shoves user tooling ahead of anything
# agent-grid put in front. The runtime shim dir is not mentioned here at all
# — case D's invariant is that framelaunch re-asserts it regardless.
printf 'PATH=/home/user/.mise/bin:/usr/bin:/bin\0'
`)
	runtimeList := []string{"/opt/agent-grid/run/hostexec-shims", "/opt/agent-grid/run/secretenv-shims"}
	restoreList := swapRuntimePathList(t, func() []string { return runtimeList })
	defer restoreList()

	t.Setenv("PATH", "/opt/agent-grid/run/hostexec-shims:/usr/bin:/bin")
	restore := swapExecReplacer(t, func(_ string, _ []string, _ []string) error {
		return nil
	})
	defer restore()

	t.Setenv(EnvVar, mustEncode(t, FrameSpec{
		PreExec:     "true",
		LoginShell:  shell,
		MainCommand: []string{"true"},
	}))
	if err := Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got := os.Getenv("PATH")
	want := "/opt/agent-grid/run/hostexec-shims:/opt/agent-grid/run/secretenv-shims:/home/user/.mise/bin:/usr/bin:/bin"
	if got != want {
		t.Fatalf("PATH after Run() =\n  %q\nwant:\n  %q", got, want)
	}
}

// TestRun_PathReassert_LookPathResolvesShim pins FR-008: with a shim file
// present in the first runtime-list directory, exec.LookPath resolves it via
// shim, not via any preExec-inserted alternative earlier in PATH.
func TestRun_PathReassert_LookPathResolvesShim(t *testing.T) {
	shimDir := t.TempDir()
	shimPath := filepath.Join(shimDir, "gh")
	if err := os.WriteFile(shimPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// The preExec rebuilds PATH so /usr/bin comes first — if we did NOT
	// re-assert, LookPath("gh") would find /usr/bin/gh (or fail). With case
	// D re-assert, the shim dir is first and shim wins.
	shell := writeFakeShell(t, `#!/bin/sh
printf 'PATH=/usr/bin:/bin\0'
`)
	restoreList := swapRuntimePathList(t, func() []string { return []string{shimDir} })
	defer restoreList()

	t.Setenv("PATH", shimDir+":/usr/bin")
	restore := swapExecReplacer(t, func(_ string, _ []string, _ []string) error {
		return nil
	})
	defer restore()

	t.Setenv(EnvVar, mustEncode(t, FrameSpec{
		PreExec:     "true",
		LoginShell:  shell,
		MainCommand: []string{"true"},
	}))
	if err := Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	resolved, err := exec.LookPath("gh")
	if err != nil {
		t.Fatalf("LookPath(gh): %v", err)
	}
	if resolved != shimPath {
		t.Fatalf("LookPath(gh) = %q, want %q (shim not resolving first)", resolved, shimPath)
	}
}

// TestRun_EmptyPreExec_LeavesPathUntouched pins FR-007: when PreExec is empty,
// Run() does not modify PATH at all.
func TestRun_EmptyPreExec_LeavesPathUntouched(t *testing.T) {
	restore := swapExecReplacer(t, func(_ string, _ []string, _ []string) error {
		return nil
	})
	defer restore()
	restoreList := swapRuntimePathList(t, func() []string {
		return []string{"/opt/should-not-appear"}
	})
	defer restoreList()

	orig := "/usr/local/bin:/usr/bin:/bin"
	t.Setenv("PATH", orig)
	t.Setenv(EnvVar, mustEncode(t, FrameSpec{
		// No PreExec — the entire re-assert branch must be skipped.
		MainCommand: []string{"true"},
	}))
	if err := Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := os.Getenv("PATH"); got != orig {
		t.Fatalf("PATH mutated with empty PreExec: got %q, want %q", got, orig)
	}
}

// TestRun_PathReassertSlog verifies the observability contract:
// exactly one framelaunch.path_reassert slog record per PreExec-branched
// invocation, with all required fields present.
func TestRun_PathReassertSlog(t *testing.T) {
	shell := writeFakeShell(t, `#!/bin/sh
printf 'PATH=/usr/bin:/bin\0'
`)
	restoreList := swapRuntimePathList(t, func() []string { return []string{"/opt/agent-grid/run/hostexec-shims"} })
	defer restoreList()
	restore := swapExecReplacer(t, func(_ string, _ []string, _ []string) error {
		return nil
	})
	defer restore()
	buf, restoreLogger := captureSlog(t)
	defer restoreLogger()

	t.Setenv("PATH", "/opt/agent-grid/run/hostexec-shims:/usr/bin:/bin")
	t.Setenv(EnvVar, mustEncode(t, FrameSpec{
		PreExec:     "true",
		LoginShell:  shell,
		MainCommand: []string{"true"},
	}))
	if err := Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	logOut := buf.String()
	if got := strings.Count(logOut, "framelaunch.path_reassert"); got != 1 {
		t.Fatalf("slog record count = %d, want 1; full log:\n%s", got, logOut)
	}
	for _, field := range []string{"orig_path_len=", "runtime_prefix_count=1", "dedup_dropped_count=", "post_reassert_changed_head=", "skip_branch="} {
		if !strings.Contains(logOut, field) {
			t.Errorf("slog record missing field marker %q; full log:\n%s", field, logOut)
		}
	}
}

// TestRun_MigrationToggle_SkipsMergeButLogsSkip pins the rollback toggle:
// when AG_FRAMELAUNCH_DISABLE_PATH_REASSERT is truthy, the merge is skipped
// (PATH unchanged from preExec's output) but the slog still fires with
// skip_branch="toggle_disabled" so operators can see the fix was gated off.
func TestRun_MigrationToggle_SkipsMergeButLogsSkip(t *testing.T) {
	shell := writeFakeShell(t, `#!/bin/sh
printf 'PATH=/usr/bin:/bin\0'
`)
	restoreList := swapRuntimePathList(t, func() []string { return []string{"/opt/agent-grid/run/hostexec-shims"} })
	defer restoreList()
	restore := swapExecReplacer(t, func(_ string, _ []string, _ []string) error {
		return nil
	})
	defer restore()
	buf, restoreLogger := captureSlog(t)
	defer restoreLogger()

	t.Setenv("PATH", "/original-orig")
	t.Setenv(TogglePathReassertDisableEnv, "1")
	t.Setenv(EnvVar, mustEncode(t, FrameSpec{
		PreExec:     "true",
		LoginShell:  shell,
		MainCommand: []string{"true"},
	}))
	if err := Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := os.Getenv("PATH"); got != "/usr/bin:/bin" {
		t.Fatalf("PATH = %q, want %q (preExec output preserved, merge skipped)", got, "/usr/bin:/bin")
	}
	logOut := buf.String()
	if !strings.Contains(logOut, `skip_branch=toggle_disabled`) {
		t.Errorf("slog record missing skip_branch=toggle_disabled; full log:\n%s", logOut)
	}
	if got := strings.Count(logOut, "framelaunch.path_reassert"); got != 1 {
		t.Errorf("slog record count = %d, want 1 (still one record even when toggle-disabled)", got)
	}
}

// swapRuntimePathList swaps the T1 seam. Callers must call the returned
// closure via defer or t.Cleanup.
func swapRuntimePathList(t *testing.T, fn func() []string) func() {
	t.Helper()
	prev := runtimePathListForTest
	runtimePathListForTest = fn
	return func() { runtimePathListForTest = prev }
}

// captureSlog replaces the default slog logger with one writing to a buffer,
// returning the buffer and a restore closure.
func captureSlog(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()
	buf := &bytes.Buffer{}
	handler := slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	prev := slog.Default()
	slog.SetDefault(slog.New(handler))
	return buf, func() { slog.SetDefault(prev) }
}

func TestFrameExecHelper(t *testing.T) {
	if os.Getenv("FRAMELAUNCH_TEST_HELPER") != "1" {
		return
	}
	// Swap execReplacer is not needed — we want real Exec into main.
	if err := Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// On success Run does not return (process replaced). If the mock path is hit, fail.
	os.Exit(0)
}

func swapExecReplacer(t *testing.T, fn func(argv0 string, argv []string, envv []string) error) func() {
	t.Helper()
	prev := execReplacer
	execReplacer = fn
	return func() { execReplacer = prev }
}

func mustEncode(t *testing.T, spec FrameSpec) string {
	t.Helper()
	raw, err := Encode(spec)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func writeFakeShell(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-shell")
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
