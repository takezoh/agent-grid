package agentlaunch

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// shEsc mirrors devcontainer.shellEscape (POSIX single-quote escaping).
func shEsc(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// TestWrap_ArgvExecutesNestedQuotedCommandViaShell pins the regression where the
// codex app-server never launched: BuildLaunchCommand wraps the agent command in
// `sh -c 'exec <login-shell> -lc '\”pre; exec <agent>'\”'` (PreExec path), and
// Wrap used to re-tokenize that string with SplitArgs. SplitArgs is not a shell
// lexer — it cannot reverse the '\” escaping — so the agent command was split off
// into stray tokens that the shell ignored, and the app-server was never started.
//
// The fix runs the command string through a real shell (Argv = ["sh","-c",cmd]).
// This test executes Wrap's Argv with a real /bin/sh and asserts the embedded
// agent command actually runs. Even though SplitArgs now understands the
// quoting pattern used here, Wrap's contract remains to preserve the original
// shell text exactly instead of reconstructing argv from it.
func TestWrap_ArgvExecutesNestedQuotedCommandViaShell(t *testing.T) {
	dir := t.TempDir()
	sentinel := filepath.Join(dir, "launched")

	// Agent command carries a double-quoted value (like codex's
	// sandbox_mode="danger-full-access") and writes the sentinel when it runs.
	agentCmd := `env CODEX_X="a b" touch ` + sentinel
	inner := shEsc("true; exec " + agentCmd)
	cmd := "sh -c " + shEsc("exec sh -lc "+inner)

	mgr := &mockMgr{buildCmd: cmd}
	l := newLauncherForTest(t, mgr, "")
	wl, err := l.Wrap(context.Background(), "frame-1", LaunchPlan{Project: "/p", StartDir: "/p"})
	if err != nil {
		t.Fatalf("Wrap: %v", err)
	}

	// Wrap must hand Spawn a shell invocation, not a SplitArgs tokenization.
	want := []string{"sh", "-c", cmd}
	if len(wl.Argv) != 3 || wl.Argv[0] != want[0] || wl.Argv[1] != want[1] || wl.Argv[2] != want[2] {
		t.Fatalf("Argv = %#v, want %#v", wl.Argv, want)
	}

	// Executing Argv through a real shell must run the embedded agent command.
	if out, err := exec.Command(wl.Argv[0], wl.Argv[1:]...).CombinedOutput(); err != nil {
		t.Fatalf("exec Argv: %v (output: %s)", err, out)
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("agent command did not run via Wrap.Argv: sentinel missing: %v", err)
	}

}
