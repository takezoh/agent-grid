package runtime

import (
	"fmt"
	"sort"
	"strconv"
	"sync"

	"github.com/takezoh/agent-reactor/platform/agentlaunch"
	"github.com/takezoh/agent-reactor/platform/termvt"
)

// PtyBackend implements the TmuxBackend role interfaces over platform/termvt,
// driving pty-backed sessions without tmux (ADR 0004). The data plane
// (lifecycle, IO, inspection, liveness) is implemented for real; the
// presentation plane (WindowLayout layout ops, TmuxControl) is stubbed because
// a pty multiplexer has no server-side equivalent — layout composition moves
// client-side in the tmux-removal phase.
//
// Targets are synthetic pane ids ("%1", "%2", …) that PtyBackend allocates and
// uses as the termvt.Manager session id, so the live session is always resolved
// via mgr.Get(target) — the Manager is the single owner of the id→Session map.
// The unchanged runtime/reducer/driver address panes by these ids exactly as
// they addressed tmux pane ids.
type PtyBackend struct {
	mgr *termvt.Manager

	mu      sync.Mutex
	buffers map[string]string // named tmux-style paste buffers
	env     map[string]string // session-level env (tmux session env stand-in)
	paneSeq int               // last allocated pane number
	winSeq  int               // last allocated window index
}

// NewPtyBackend returns a PtyBackend with its own termvt.Manager.
func NewPtyBackend() *PtyBackend {
	return &PtyBackend{
		mgr:     termvt.NewManager(),
		buffers: map[string]string{},
		env:     map[string]string{},
	}
}

// === PaneLifecycle ===

// SpawnWindow starts command in a new pty and returns synthetic window/pane ids.
// startDir is currently unused: termvt.Spec has no working-directory field.
// TODO(B1): thread startDir once termvt.Spec gains a Dir field.
func (p *PtyBackend) SpawnWindow(name, command, startDir string, env map[string]string) (string, string, error) {
	argv, err := agentlaunch.SplitArgs(command)
	if err != nil {
		return "", "", err
	}
	if len(argv) == 0 {
		return "", "", fmt.Errorf("runtime: empty command for window %q", name)
	}

	// mu protects only the id counters; the ids it yields are unique, so release
	// it before Create rather than holding it across the fork/exec in
	// pty.StartWithSize (which would serialise every other backend op behind a
	// spawn). The Manager has its own mutex and rejects duplicate ids.
	p.mu.Lock()
	p.paneSeq++
	p.winSeq++
	paneID := newPaneID(p.paneSeq)
	winIdx := newWindowIndex(p.winSeq)
	p.mu.Unlock()

	if _, err := p.mgr.Create(paneID, termvt.Spec{Argv: argv, Env: envSlice(env)}); err != nil {
		return "", "", err
	}
	return winIdx, paneID, nil
}

// KillPaneWindow closes the session for target and forgets it.
func (p *PtyBackend) KillPaneWindow(target string) error {
	return p.mgr.Remove(target)
}

// RespawnPane tears the dead pane down and re-creates a session under the same
// target. It does NOT carry over the session-env store or the original spawn
// env — respawn launches a fresh process with the default environment — and the
// new session starts at the default terminal size until the next ResizeWindow.
func (p *PtyBackend) RespawnPane(target, command string) error {
	argv, err := agentlaunch.SplitArgs(command)
	if err != nil {
		return err
	}
	if len(argv) == 0 {
		return fmt.Errorf("runtime: empty respawn command for %q", target)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	// Tear down the old session if present. If teardown fails, abort: do not
	// stack a new session on top of a pane we could not cleanly remove.
	if _, known := p.mgr.Get(target); known {
		if err := p.mgr.Remove(target); err != nil {
			return fmt.Errorf("runtime: respawn %q: %w", target, err)
		}
	}

	if _, err := p.mgr.Create(target, termvt.Spec{Argv: argv}); err != nil {
		return err
	}
	return nil
}

// PaneAlive reports whether the session is still running (known to the Manager
// and not yet reaped, i.e. ExitCode reports not-exited).
func (p *PtyBackend) PaneAlive(target string) (bool, error) {
	sess, ok := p.mgr.Get(target)
	if !ok {
		return false, nil
	}
	_, exited := sess.ExitCode()
	return !exited, nil
}

// PaneExitStatus reports the exit code once the process has been reaped.
func (p *PtyBackend) PaneExitStatus(target string) (bool, int, error) {
	sess, ok := p.mgr.Get(target)
	if !ok {
		return false, -1, fmt.Errorf("runtime: unknown pane %q", target)
	}
	code, exited := sess.ExitCode()
	if !exited {
		return false, -1, nil
	}
	return true, code, nil
}

// === PaneIO ===

// SendKeys writes text followed by a carriage return to the pane.
func (p *PtyBackend) SendKeys(target, text string) error {
	return p.write(target, []byte(text+"\r"))
}

// SendEnter writes a single carriage return to the pane.
func (p *PtyBackend) SendEnter(target string) error {
	return p.write(target, []byte("\r"))
}

// SendKey writes a named key (or the literal key when unknown) to the pane.
func (p *PtyBackend) SendKey(target, key string) error {
	return p.write(target, []byte(keyBytes(key)))
}

// LoadBuffer stores text under name in the in-memory buffer map.
func (p *PtyBackend) LoadBuffer(name, text string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.buffers[name] = text
	return nil
}

// PasteBuffer writes a stored buffer to target then deletes it.
func (p *PtyBackend) PasteBuffer(name, target string) error {
	p.mu.Lock()
	text, ok := p.buffers[name]
	if ok {
		delete(p.buffers, name)
	}
	p.mu.Unlock()
	if !ok {
		return fmt.Errorf("runtime: unknown buffer %q", name)
	}
	return p.write(target, []byte(text))
}

// PipePane is a no-op: output taps are served by termvt.Subscribe in a separate
// task, not by re-piping pane output through a shell command.
// TODO(B1): wire the output tap via Session.Subscribe in the pty_tap task, and
// honor the empty-command contract (tmux pipe-pane with no command stops the
// running tap) once the tap is live.
func (p *PtyBackend) PipePane(target, command string) error { return nil }

// === PaneInspect ===

// PaneID echoes the synthetic pane id back when the pane is known.
func (p *PtyBackend) PaneID(target string) (string, error) {
	if _, ok := p.mgr.Get(target); !ok {
		return "", fmt.Errorf("runtime: unknown pane %q", target)
	}
	return target, nil
}

// PaneSize returns the session's current terminal dimensions.
func (p *PtyBackend) PaneSize(target string) (int, int, error) {
	sess, ok := p.mgr.Get(target)
	if !ok {
		return 0, 0, fmt.Errorf("runtime: unknown pane %q", target)
	}
	cols, rows := sess.Size()
	return cols, rows, nil
}

// CapturePane returns the trailing nLines of the pane's rendered screen with SGR
// escapes stripped.
func (p *PtyBackend) CapturePane(target string, nLines int) (string, error) {
	sess, ok := p.mgr.Get(target)
	if !ok {
		return "", fmt.Errorf("runtime: unknown pane %q", target)
	}
	return termvt.CaptureTail(sess, nLines), nil
}

// === SessionEnv ===
//
// The session-env store is in-process only: it lives in p.env and dies with the
// process. It is NOT a persistence layer — values do not survive a daemon
// restart and are not injected into spawned children. Cross-restart pane
// recovery is out of scope for B1 (ADR 0004) and belongs to a later phase.

// SetEnv writes a session-level env var into the in-process store.
func (p *PtyBackend) SetEnv(key, value string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.env[key] = value
	return nil
}

// UnsetEnv removes a session-level env var from the in-process store.
func (p *PtyBackend) UnsetEnv(key string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.env, key)
	return nil
}

// ShowEnvironment returns the session env as KEY=VALUE lines sorted by key.
func (p *PtyBackend) ShowEnvironment() (string, error) {
	p.mu.Lock()
	pairs := make([][2]string, 0, len(p.env))
	for k, v := range p.env {
		pairs = append(pairs, [2]string{k, v})
	}
	p.mu.Unlock()

	sort.Slice(pairs, func(i, j int) bool { return pairs[i][0] < pairs[j][0] })
	var b []byte
	for _, kv := range pairs {
		b = append(b, kv[0]...)
		b = append(b, '=')
		b = append(b, kv[1]...)
		b = append(b, '\n')
	}
	return string(b), nil
}

// === WindowLayout ===

// ResizeWindow resizes the session's pty/grid. The other layout ops are stubbed.
func (p *PtyBackend) ResizeWindow(target string, width, height int) error {
	sess, ok := p.mgr.Get(target)
	if !ok {
		return fmt.Errorf("runtime: unknown pane %q", target)
	}
	return sess.Resize(width, height)
}

// The following WindowLayout ops have no pty equivalent — layout composition
// moves client-side in the tmux-removal phase (ADR 0004).
func (p *PtyBackend) SwapPane(srcPane, dstPane string) error    { return nil }
func (p *PtyBackend) BreakPane(srcPane, dstWindow string) error { return nil }
func (p *PtyBackend) BreakPaneToNewWindow(srcPane, name string) (string, error) {
	return "", nil
}
func (p *PtyBackend) JoinPane(srcPane, dstPane string, before bool, sizePct int) error {
	return nil
}
func (p *PtyBackend) SelectPane(target string) error { return nil }
func (p *PtyBackend) RunChain(ops ...[]string) error { return nil }

// === TmuxControl (all stubbed — no server-side equivalent) ===

func (p *PtyBackend) SetStatusLine(line string) error              { return nil }
func (p *PtyBackend) DetachClient() error                          { return nil }
func (p *PtyBackend) KillSession() error                           { return nil }
func (p *PtyBackend) DisplayPopup(width, height, cmd string) error { return nil }

// === helpers ===

func (p *PtyBackend) write(target string, b []byte) error {
	sess, ok := p.mgr.Get(target)
	if !ok {
		return fmt.Errorf("runtime: unknown pane %q", target)
	}
	return sess.WriteInput(b)
}

// newPaneID formats a synthetic tmux-style pane id ("%1", "%2", …).
func newPaneID(n int) string { return "%" + strconv.Itoa(n) }

// newWindowIndex formats a synthetic tmux-style window index ("1", "2", …).
func newWindowIndex(n int) string { return strconv.Itoa(n) }

// envSlice converts a KEY→VALUE map into the KEY=VALUE slice termvt.Spec wants.
// A nil/empty map yields nil so the session inherits os.Environ().
func envSlice(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	return out
}

// keyBytes maps the named keys the runtime sends to their byte sequence.
// Control chords ("C-c") and meta chords ("M-x") are decoded generically;
// remaining unknown keys pass through literally.
// TODO(B1): extend coverage to the full tmux key-name table as drivers need it.
func keyBytes(key string) string {
	switch key {
	case "Escape":
		return "\x1b"
	case "Enter":
		return "\r"
	case "Up":
		return "\x1b[A"
	case "Down":
		return "\x1b[B"
	case "Right":
		return "\x1b[C"
	case "Left":
		return "\x1b[D"
	case "Tab":
		return "\t"
	case "BSpace":
		return "\x7f"
	case "Space":
		return " "
	}
	if b, ok := chordBytes(key); ok {
		return b
	}
	return key
}

// chordBytes decodes a single-character control or meta chord. "C-<ch>" maps to
// the control byte (ch & 0x1f), so "C-c" → 0x03 (SIGINT). "M-<ch>" maps to ESC
// followed by ch. It reports ok=false for anything that is not a recognised
// single-character chord so the caller can fall back to literal passthrough.
func chordBytes(key string) (string, bool) {
	if len(key) != 3 || key[1] != '-' {
		return "", false
	}
	ch := key[2]
	switch key[0] {
	case 'C':
		return string([]byte{ch & 0x1f}), true
	case 'M':
		return string([]byte{0x1b, ch}), true
	default:
		return "", false
	}
}
