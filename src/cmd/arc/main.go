package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/takezoh/agent-reactor/client/config"
	"github.com/takezoh/agent-reactor/client/procio"
	"github.com/takezoh/agent-reactor/platform/appid"
	"github.com/takezoh/agent-reactor/platform/logger"
)

var (
	loadBootstrapConfig   = config.Load
	initLoggerWithDataDir = logger.InitWithDataDir
	closeLogger           = logger.Close
	redirectStderr        = logger.RedirectStderr
	parseDaemonArgsFn     = parseDaemonArgs
	runCoordinatorFn      = runCoordinator
	runHeaderTUIFn        = runHeaderTUI
	runMainTUIFn          = runMainTUI
	runSessionListFn      = runSessionList
	runLogViewerFn        = runLogViewer
	runPaletteFn          = runPalette
)

func main() {
	os.Exit(runMain(os.Args[1:], os.Stdout, os.Stderr))
}

func runMain(args []string, stdout, stderr io.Writer) (code int) {
	kind := classifyCommand(args)
	cfg, cfgErr := loadBootstrapConfig()
	// Resolve the coordinator's -data-dir flag once at the top so EVERY
	// downstream call to config.ResolveDataDir() (logger init here, plus
	// runCoordinator's own fresh loadConfig() in coordinator.go) returns
	// the flag-specified path. We do this by exporting ROOST_DATA_DIR,
	// which is the highest-precedence branch inside ResolveDataDir — that
	// route makes the flag win over a stale shell env (systemd inherits
	// the user's env on `systemctl --user start`, so an `export
	// ROOST_DATA_DIR=…` in a developer's rc would otherwise silently
	// override the unit's explicit ExecStart=… -data-dir).
	//
	// Parse runs regardless of cfgErr / cfg==nil so a malformed
	// settings.toml never hides a bad -data-dir flag from the operator.
	if kind == commandKindRoost {
		dataDir, err := parseDaemonArgsFn(args)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", appid.ClientBin, err)
			return 2
		}
		if dataDir != "" {
			_ = os.Setenv("ROOST_DATA_DIR", dataDir)
			if cfg != nil {
				cfg.DataDir = dataDir
			}
		}
	}
	loggerReady, loggerErr := initMainLogger(cfg, kind == commandKindRoost)
	if loggerReady {
		defer closeLogger()
	}
	defer func() {
		if rec := recover(); rec != nil {
			err := fmt.Errorf("panic: %v", rec)
			if loggerReady {
				slog.Error("panic recovered", "err", err)
			}
			code = finishMain(kind, err, loggerReady, loggerErr, stdout, stderr)
		}
	}()

	if loggerErr != nil {
		return finishMain(kind, loggerErr, false, loggerErr, stdout, stderr)
	}
	switch kind {
	case commandKindCLI:
		procio.UseTerminal()
	case commandKindDaemon, commandKindRoost:
		procio.UseLogFile(logger.LogFile())
		// Dup fd 2 to the log file so goroutine panics (which bypass the
		// main-goroutine recover() and write the stack trace straight to
		// stderr) land in the log instead of vanishing onto a terminal
		// that may or may not still be there. Without this, "daemon
		// disappeared without a trace" is genuinely tracelessly true.
		redirectStderr()
	default:
	}
	if cfgErr != nil {
		slog.Error("config load failed during logger bootstrap", "err", cfgErr)
	}

	err := runCommand(args, stdout)
	if err != nil {
		slog.Error("main failed", "err", err)
	}
	return finishMain(kind, err, true, nil, stdout, stderr)
}

func finishMain(kind commandKind, err error, loggerReady bool, loggerErr error, stdout, stderr io.Writer) int {
	if kind == commandKindRoost {
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", appid.ClientBin, err)
			return 1
		}
		fmt.Fprintf(stdout, "%s: exited\n", appid.ClientBin)
		return 0
	}
	if !loggerReady && loggerErr != nil {
		return 1
	}
	if err != nil {
		return 1
	}
	return 0
}

func initMainLogger(cfg *config.Config, rotate bool) (bool, error) {
	level := "info"
	dataDir := ""
	if cfg != nil {
		level = cfg.Log.Level
		dataDir = cfg.ResolveDataDir()
	}
	if rotate {
		logger.Rotate(dataDir)
	}
	if err := initLoggerWithDataDir(level, dataDir); err != nil {
		return false, err
	}
	return true, nil
}

func loadConfig() (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func resolveExe() string {
	exe, _ := os.Executable()
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return exe
	}
	return resolved
}
