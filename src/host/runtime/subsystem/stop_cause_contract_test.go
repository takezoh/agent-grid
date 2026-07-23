package subsystem_test

import (
	"testing"

	"github.com/takezoh/agent-grid/host/runtime/subsystem"
	"github.com/takezoh/agent-grid/host/runtime/subsystem/cli"
	"github.com/takezoh/agent-grid/host/runtime/subsystem/stream"
)

// TestContract_AllSubsystemsRequireTypedStopCause is intentionally a
// compile-time boundary test: adding an implementation that retains the old
// untyped Stop signature fails this package before runtime behavior can drift.
func TestContract_AllSubsystemsRequireTypedStopCause(t *testing.T) {
	var _ subsystem.Subsystem = (*cli.Backend)(nil)
	var _ subsystem.Subsystem = (*stream.Backend)(nil)
}
