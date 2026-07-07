package fake

// FakeCLI mimics the `codex --remote [resume <id>]` binary for tests. It
// connects to a fake (or real) app-server, creates or resumes a thread on its
// own connection — reproducing the CLI-owns-the-thread invariant that the
// production Backend must adopt after — reads user prompts as newline-
// delimited lines from stdin, and forwards them as `turn/start` notifications.
// Every notification the server broadcasts back is echoed to stdout as
// `[EVENT] method=… threadId=… payload=…` so a pty-attached test can observe
// end-to-end wire behaviour without any coupling to the codex TUI's rendering.

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/takezoh/agent-grid/platform/agent/codexclient"
)

// CLIArgs configures a FakeCLI invocation.
type CLIArgs struct {
	Sock     string
	Resume   string // resume this thread id (fake app-server only accepts ids it minted)
	Cwd      string
	Stdin    io.Reader
	Stdout   io.Writer
	Stderr   io.Writer
	DialWait time.Duration // how long to keep trying DialUDS if the sock is not yet ready (0 → 3s default)
}

// RunCLI is the FakeCLI entry point. Blocks until stdin EOFs or the connection
// drops; returns the first error encountered. Callers that run this in a pty
// subprocess use ParseCLIArgs + RunCLI from a TestMain dispatcher.
func RunCLI(args CLIArgs) error {
	if args.Sock == "" {
		return errors.New("fake CLI: Sock is required")
	}
	if args.Stdin == nil {
		args.Stdin = os.Stdin
	}
	if args.Stdout == nil {
		args.Stdout = os.Stdout
	}
	if args.Stderr == nil {
		args.Stderr = os.Stderr
	}
	if args.DialWait == 0 {
		args.DialWait = 3 * time.Second
	}

	tr, err := codexclient.DialUDS(args.Sock, args.DialWait)
	if err != nil {
		return fmt.Errorf("fake CLI: dial %s: %w", args.Sock, err)
	}
	conn := codexclient.NewConn(tr, 30*time.Second)

	obs := &cliObserver{stdout: args.Stdout}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runErr := make(chan error, 1)
	go func() { runErr <- conn.Run(ctx, obs) }()

	if err := codexclient.Initialize(conn); err != nil {
		return fmt.Errorf("fake CLI: initialize: %w", err)
	}

	// Fresh vs resume: same as real codex CLI, backend passes threadID via
	// `resume` subcommand for cold-start recovery.
	var threadID string
	if args.Resume != "" {
		sess, err := codexclient.ResumeThread(conn, codexclient.ResumeOptions{ThreadID: args.Resume, Cwd: args.Cwd})
		if err != nil {
			return fmt.Errorf("fake CLI: resume %s: %w", args.Resume, err)
		}
		threadID = sess.ThreadID
	} else {
		sess, err := codexclient.StartThread(conn, args.Cwd, nil, codexclient.ThreadOptions{})
		if err != nil {
			return fmt.Errorf("fake CLI: thread/start: %w", err)
		}
		threadID = sess.ThreadID
	}
	fmt.Fprintf(args.Stdout, "[READY] threadId=%s\n", threadID)

	// Read prompts line-by-line from stdin; each non-empty line becomes a
	// turn/start notification. Empty line or EOF ends the session.
	scanner := bufio.NewScanner(args.Stdin)
	// Allow long single-line prompts.
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			break
		}
		if err := codexclient.StartTurn(conn, threadID, args.Cwd, []byte(line), codexclient.TurnOptions{}); err != nil {
			return fmt.Errorf("fake CLI: turn/start: %w", err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("fake CLI: read stdin: %w", err)
	}

	// Give the server a moment to flush any pending broadcast for the last
	// turn, then close.
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case err := <-runErr:
		if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, io.EOF) {
			return fmt.Errorf("fake CLI: read loop: %w", err)
		}
	case <-time.After(500 * time.Millisecond):
	}
	return nil
}

// cliObserver echoes every incoming notification / server request to stdout as
// a machine-parseable `[EVENT]` line so pty-attached tests can observe wire
// events without needing a second Backend instance.
type cliObserver struct {
	stdout io.Writer
}

func (o *cliObserver) OnNotification(method string, params json.RawMessage) {
	fmt.Fprintf(o.stdout, "[EVENT] method=%s params=%s\n", method, string(params))
}
func (o *cliObserver) OnServerRequest(_ codexclient.RequestID, method string, params json.RawMessage) {
	fmt.Fprintf(o.stdout, "[REQUEST] method=%s params=%s\n", method, string(params))
}

// MaybeRunCLIFromArgs consumes the process argv: if it looks like a FakeCLI
// invocation (`<binary> fake-cli …`), it runs the CLI and never returns
// (calls os.Exit). Any other argv shape returns without side effects, so a
// TestMain can call MaybeRunCLIFromArgs(os.Args) before m.Run() to accept
// SpawnCLI re-invocations of the same test binary.
func MaybeRunCLIFromArgs(argv []string) {
	if len(argv) < 2 || argv[1] != "fake-cli" {
		return
	}
	args, err := ParseCLIArgs(argv[2:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "fake-cli: %v\n", err)
		os.Exit(2)
	}
	if err := RunCLI(args); err != nil {
		fmt.Fprintf(os.Stderr, "fake-cli: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

// ParseCLIArgs parses argv (typically os.Args[2:] after the "fake-cli"
// dispatcher token) into a CLIArgs. Returns an error suitable for os.Exit(2).
func ParseCLIArgs(argv []string) (CLIArgs, error) {
	fs := flag.NewFlagSet("fake-cli", flag.ContinueOnError)
	remote := fs.String("remote", "", "UDS path (unix://…)")
	resume := fs.String("resume", "", "resume the given thread id instead of creating a fresh one")
	cwd := fs.String("cd", "", "working directory to advertise on thread/start / turn/start")
	if err := fs.Parse(argv); err != nil {
		return CLIArgs{}, err
	}
	if *remote == "" {
		return CLIArgs{}, errors.New("--remote unix://… is required")
	}
	sock := *remote
	if len(sock) > len("unix://") && sock[:len("unix://")] == "unix://" {
		sock = sock[len("unix://"):]
	}
	return CLIArgs{Sock: sock, Resume: *resume, Cwd: *cwd}, nil
}
