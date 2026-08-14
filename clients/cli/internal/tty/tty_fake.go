package tty

import (
	"errors"
	"io"
	"sync"
)

// tty_fake.go is the fake half of the local-terminal boundary's registered
// triple. It exists so the attach runtime's exit paths can be driven
// deterministically: without it, "the terminal is restored on a signal exit"
// would be testable only by hand, on a real terminal, once.
//
// Everything a real terminal does asynchronously is a method here, so a test
// says when a resize happens instead of arranging for one.

// Fake is an in-memory Terminal plus the controls a test drives it with.
type Fake struct {
	mu sync.Mutex

	// RawCalls counts entries into raw mode, and RestoreCalls counts restores.
	// Counting rather than flagging is what lets a test assert that restore ran
	// exactly once on the paths where both a defer and a signal handler fire.
	RawCalls     int
	RestoreCalls int
	// EffectiveRestores counts restores that actually changed the terminal.
	// Idempotence means this is at most one however many times Restore is
	// called.
	EffectiveRestores int
	// Discards counts input-queue discards, and DiscardedBeforeRestore records
	// whether the discard happened while still in raw mode — which is the
	// ordering FR-010 requires and the one an implementation gets wrong by
	// restoring first.
	Discards               int
	DiscardedBeforeRestore bool
	Closed                 bool
	MakeRawErr             error
	DiscardErr             error
	SizeErr                error
	cols, rows             int
	raw                    bool
	restored               bool
	input                  *io.PipeReader
	inputWriter            *io.PipeWriter
	output                 writeCollector
	changes                chan struct{}
}

// NewFake builds a fake terminal of the given size.
func NewFake(cols, rows int) *Fake {
	reader, writer := io.Pipe()
	return &Fake{
		cols: cols, rows: rows,
		input: reader, inputWriter: writer,
		changes: make(chan struct{}, 8),
	}
}

// MakeRaw records an entry into raw mode.
func (fake *Fake) MakeRaw() error {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.MakeRawErr != nil {
		return fake.MakeRawErr
	}
	fake.RawCalls++
	fake.raw = true
	// Cleared so a suspend/resume cycle can restore a second time, exactly as
	// the real terminal allows.
	fake.restored = false
	return nil
}

// Restore records a restore and applies it at most once.
func (fake *Fake) Restore() error {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	fake.RestoreCalls++
	if fake.restored || !fake.raw {
		fake.restored = true
		return nil
	}
	fake.EffectiveRestores++
	fake.restored = true
	fake.raw = false
	return nil
}

// Size reports the fake's current geometry.
func (fake *Fake) Size() (int, int, error) {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.SizeErr != nil {
		return 0, 0, fake.SizeErr
	}
	return fake.cols, fake.rows, nil
}

// SizeChanges yields once per Resize call.
func (fake *Fake) SizeChanges() <-chan struct{} { return fake.changes }

// DiscardInputQueue records a queue discard and whether it preceded restore.
func (fake *Fake) DiscardInputQueue() error {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	fake.Discards++
	if fake.raw && !fake.restored {
		fake.DiscardedBeforeRestore = true
	}
	return fake.DiscardErr
}

// Reader is the fake's input stream, fed by Type.
func (fake *Fake) Reader() io.Reader { return fake.input }

// Writer collects everything written to the terminal.
func (fake *Fake) Writer() io.Writer { return &fake.output }

// Close releases the fake and restores it.
func (fake *Fake) Close() error {
	fake.mu.Lock()
	fake.Closed = true
	fake.mu.Unlock()
	_ = fake.inputWriter.Close()
	return fake.Restore()
}

// --- controls ---------------------------------------------------------------

// Type delivers bytes as if the user typed them.
func (fake *Fake) Type(data []byte) error {
	_, err := fake.inputWriter.Write(data)
	return err
}

// EndInput closes the input stream, as end-of-input does on a real terminal.
func (fake *Fake) EndInput() { _ = fake.inputWriter.Close() }

// FailInput ends the input stream with an error, as a terminal that goes away
// mid-session does.
func (fake *Fake) FailInput(err error) { _ = fake.inputWriter.CloseWithError(err) }

// Resize changes the reported size and delivers a size-change edge, in that
// order — a consumer that re-queries on the edge must see the new value.
func (fake *Fake) Resize(cols, rows int) {
	fake.mu.Lock()
	fake.cols, fake.rows = cols, rows
	fake.mu.Unlock()
	fake.changes <- struct{}{}
}

// Output is everything written to the terminal.
func (fake *Fake) Output() []byte { return fake.output.Bytes() }

// IsRaw reports whether the fake is currently in raw mode.
func (fake *Fake) IsRaw() bool {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	return fake.raw
}

// ErrFakeTerminalGone is a convenient failure for FailInput.
var ErrFakeTerminalGone = errors.New("the terminal went away")

// writeCollector is a synchronised byte sink. The attach runtime writes output
// from one goroutine while a test reads from another.
type writeCollector struct {
	mu   sync.Mutex
	data []byte
}

func (collector *writeCollector) Write(p []byte) (int, error) {
	collector.mu.Lock()
	defer collector.mu.Unlock()
	collector.data = append(collector.data, p...)
	return len(p), nil
}

func (collector *writeCollector) Bytes() []byte {
	collector.mu.Lock()
	defer collector.mu.Unlock()
	return append([]byte(nil), collector.data...)
}
