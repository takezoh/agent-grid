package fake

// TestMain doubles as the FakeCLI entrypoint. SpawnCLI re-invokes the test
// binary with `fake-cli` as os.Args[1]; that branch runs RunCLI and exits
// without ever entering the test runner. Any other invocation runs the
// package's tests normally.

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	MaybeRunCLIFromArgs(os.Args)
	os.Exit(m.Run())
}
