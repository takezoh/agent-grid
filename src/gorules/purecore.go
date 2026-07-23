//go:build ruleguard

// Package gorules holds go-ruleguard rules run by gocritic's ruleguard checker
// (wired in src/.golangci.yml). These encode pure-functional-core invariants that
// forbidigo cannot express: the `go` statement is a GoStmt, invisible to forbidigo's
// CallExpr matching, and the wall-clock / direct-I/O bans must be scoped to the
// reducer files within orchestrator/scheduler (the shell files there legitimately
// own concurrency, timers, and I/O).
//
// Targeted cores:
//   - host/state         — wholly pure; every file is in scope.
//   - orchestrator/scheduler — reducer and shell share one package, so the shell
//     files (scheduler.go / effects_exec.go / clock.go / watch.go) are excluded.
//
// The scope predicate is repeated inline in each rule because ruleguard's Where()
// only accepts direct m.File()/m.Var filter expressions, not helper calls.
package gorules

import "github.com/quasilyte/go-ruleguard/dsl"

// noGoroutinesInPureCore forbids spawning goroutines in the functional core.
// Concurrency belongs to the imperative shell (runtime loop / scheduler.Run).
func noGoroutinesInPureCore(m dsl.Matcher) {
	m.Match(`go $f($*_)`, `go func($*_) { $*_ }($*_)`).
		Where(!m.File().Name.Matches(`_test\.go$`) &&
			(m.File().PkgPath.Matches(`/host/state$`) ||
				(m.File().PkgPath.Matches(`/orchestrator/scheduler$`) &&
					!m.File().Name.Matches(`^(scheduler|effects_exec|clock|watch)\.go$`)))).
		Report(`goroutines are forbidden in the pure functional core — concurrency belongs in the imperative shell (ARCHITECTURE.md / code-enforcement.md)`)
}

// noWallClockInPureCore forbids reading the wall clock in the functional core.
// Time enters Reduce as a value (the injected `now`), never read inside it.
func noWallClockInPureCore(m dsl.Matcher) {
	m.Match(`time.Now()`, `time.Since($_)`).
		Where(!m.File().Name.Matches(`_test\.go$`) &&
			(m.File().PkgPath.Matches(`/host/state$`) ||
				(m.File().PkgPath.Matches(`/orchestrator/scheduler$`) &&
					!m.File().Name.Matches(`^(scheduler|effects_exec|clock|watch)\.go$`)))).
		Report(`the pure functional core must not read the wall clock — time enters Reduce as a value (ARCHITECTURE.md)`)
}

// noDirectIOInPureCore forbids real I/O in the functional core. The only permitted
// synchronous I/O is bounded read-only filesystem stat (os.Stat / os.Lstat).
func noDirectIOInPureCore(m dsl.Matcher) {
	m.Match(
		`os.Open($*_)`, `os.OpenFile($*_)`, `os.Create($*_)`,
		`os.ReadFile($*_)`, `os.WriteFile($*_)`,
		`os.Remove($*_)`, `os.RemoveAll($*_)`, `os.Mkdir($*_)`, `os.MkdirAll($*_)`,
		`net.Dial($*_)`, `net.Listen($*_)`,
		`exec.Command($*_)`, `exec.CommandContext($*_)`,
	).
		Where(!m.File().Name.Matches(`_test\.go$`) &&
			(m.File().PkgPath.Matches(`/host/state$`) ||
				(m.File().PkgPath.Matches(`/orchestrator/scheduler$`) &&
					!m.File().Name.Matches(`^(scheduler|effects_exec|clock|watch)\.go$`)))).
		Report(`the pure functional core must not perform I/O — emit an Effect instead (only bounded read-only os.Stat is allowed; see ARCHITECTURE.md)`)
}

// noRealBinaryExecOutsideE2E confines direct real-binary exec to the shared
// platform/lib wrappers, fake packages that validate themselves against the
// real binary, and *_e2e_test.go files.
func noRealBinaryExecOutsideE2E(m dsl.Matcher) {
	m.Match(`exec.Command($name, $*_)`, `exec.CommandContext($_, $name, $*_)`, `exec.LookPath($name)`).
		Where((m["name"].Text.Matches(`^"(claude|codex|docker)"$`) ||
			m["name"].Text.Matches(`^(claude|codex|docker)(Bin|Path|Command)?$`)) &&
			!m.File().Name.Matches(`_e2e_test\.go$`) &&
			!m.File().PkgPath.Matches(`/platform/lib(/|$)`) &&
			!m.File().PkgPath.Matches(`/(fakeclaude|fakecodex|fakedocker)$`)).
		Report(`real binary exec is restricted to platform/lib wrappers, fake packages, and *_e2e_test.go fidelity tests`)
}

// noE2EEnvOutsideE2E confines AG_E2E_* env access to *_e2e_test.go files.
// Non-test helpers that legitimately bridge into those suites are excluded by
// path in .golangci.yml rather than per-line annotations.
func noE2EEnvOutsideE2E(m dsl.Matcher) {
	m.Match(`os.Getenv($name)`, `os.LookupEnv($name)`).
		Where(m["name"].Text.Matches(`^"AG_E2E_[A-Z0-9_]+"$`) &&
			!m.File().Name.Matches(`_e2e_test\.go$`)).
		Report(`AG_E2E_* env access is restricted to *_e2e_test.go fidelity suites`)
}
