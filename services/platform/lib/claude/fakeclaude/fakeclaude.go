// Package fakeclaude publishes reusable fakes for the Claude CLI wire
// protocol. It generates the stream-json lines that a real `claude -p
// --output-format stream-json` process emits, and the JSON hook payloads
// that Claude sends on stdin to registered hook commands.
//
// Two invariants keep the fakes honest:
//
//  1. Everything in this package is exercised end-to-end by the opt-in
//     tests in `claude_cli_e2e_test.go` and `hook_e2e_test.go` (build
//     tag `e2e`) against a real `claude` binary. When those tests fail,
//     the fake — not the assertion — is the thing to update.
//
//  2. This package depends only on `platform/lib/claude/streamjson` and
//     `platform/lib/claude/cli`. It never imports `client/*` or
//     `orchestrator/*` (enforced by depguard).
//
// The package is safe to import from tests in any layer.
package fakeclaude

import (
	"context"
	"io"
	"strings"
	"sync"
)

// LauncherFunc matches the claudeLauncher signature declared in
// cmd/claude-app-server/launch.go. It is redeclared here so tests in
// packages that cannot import main can construct a launcher.
type LauncherFunc func(
	ctx context.Context,
	cwd, resumeSessionID, appendSystemPrompt, prompt string,
	extraEnv []string,
) (io.ReadCloser, func() error, error)

// LaunchCall records the arguments of a single launcher invocation.
type LaunchCall struct {
	CWD                string
	ResumeSessionID    string
	AppendSystemPrompt string
	Prompt             string
	ExtraEnv           []string
}

// CallLog is a thread-safe record of launcher invocations. Tests use it to
// assert that the shim propagated --resume, TOOL_BRIDGE_SOCKET, and so on.
type CallLog struct {
	mu    sync.Mutex
	calls []LaunchCall
}

// Calls returns a defensive copy of all recorded invocations.
func (l *CallLog) Calls() []LaunchCall {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]LaunchCall, len(l.calls))
	copy(out, l.calls)
	return out
}

// Len returns the number of recorded invocations.
func (l *CallLog) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.calls)
}

// Last returns the most recent invocation, or the zero value when empty.
func (l *CallLog) Last() LaunchCall {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.calls) == 0 {
		return LaunchCall{}
	}
	return l.calls[len(l.calls)-1]
}

func (l *CallLog) append(c LaunchCall) {
	l.mu.Lock()
	l.calls = append(l.calls, c)
	l.mu.Unlock()
}

// NewLauncher returns a LauncherFunc that yields the next sequence on each
// call. The final sequence sticks for any invocation beyond len(sequences).
//
// Each element of `sequences` is a list of already-formatted stream-json
// lines; the launcher concatenates them with newlines and returns the
// buffer as the process stdout. wait() always returns nil.
//
// The returned CallLog captures every invocation for assertion. Under
// concurrent launcher invocations the sequence index advances atomically
// per call, so goroutine A cannot receive B's sequence.
func NewLauncher(sequences ...[]string) (LauncherFunc, *CallLog) {
	if len(sequences) == 0 {
		sequences = [][]string{nil}
	}
	var (
		mu  sync.Mutex
		idx int
	)
	return NewProgrammableLauncher(func(_ LaunchArgs) LaunchResponse {
		mu.Lock()
		i := idx
		if idx < len(sequences)-1 {
			idx++
		}
		mu.Unlock()
		return LaunchResponse{Lines: sequences[i]}
	})
}

// LaunchArgs bundles the launcher's inputs so NewProgrammableLauncher can
// dispatch on them without spelling out the full parameter list at every
// call site.
type LaunchArgs struct {
	Ctx                context.Context
	CWD                string
	ResumeSessionID    string
	AppendSystemPrompt string
	Prompt             string
	ExtraEnv           []string
}

// LaunchResponse controls what one launcher call returns.
//
//   - Lines: stream-json lines to concatenate onto stdout (nil = empty stream).
//   - Err:   startup error to surface (short-circuits Lines and Wait).
//   - Wait:  optional custom wait func; nil means an immediate nil-return.
type LaunchResponse struct {
	Lines []string
	Err   error
	Wait  func() error
}

// NewProgrammableLauncher lets a test compute the response from the actual
// call arguments (e.g. to inspect extraEnv, or to block until ctx.Done).
// The CallLog is populated the same way as NewLauncher.
func NewProgrammableLauncher(fn func(LaunchArgs) LaunchResponse) (LauncherFunc, *CallLog) {
	log := &CallLog{}
	launcher := func(
		ctx context.Context,
		cwd, resumeSessionID, appendSystemPrompt, prompt string,
		extraEnv []string,
	) (io.ReadCloser, func() error, error) {
		envCopy := append([]string(nil), extraEnv...)
		log.append(LaunchCall{
			CWD:                cwd,
			ResumeSessionID:    resumeSessionID,
			AppendSystemPrompt: appendSystemPrompt,
			Prompt:             prompt,
			ExtraEnv:           envCopy,
		})
		resp := fn(LaunchArgs{
			Ctx:                ctx,
			CWD:                cwd,
			ResumeSessionID:    resumeSessionID,
			AppendSystemPrompt: appendSystemPrompt,
			Prompt:             prompt,
			ExtraEnv:           envCopy,
		})
		if resp.Err != nil {
			return nil, nil, resp.Err
		}
		body := strings.Join(resp.Lines, "\n")
		if body != "" {
			body += "\n"
		}
		wait := resp.Wait
		if wait == nil {
			wait = func() error { return nil }
		}
		return io.NopCloser(strings.NewReader(body)), wait, nil
	}
	return launcher, log
}
