package main

import (
	"fmt"
	"os"

	"github.com/takezoh/agent-grid/platform/sandbox/devcontainer/fakedocker"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(125)
	}
}

func run(args []string) error {
	cfgPath := os.Getenv(fakedocker.EnvConfigPath)
	recordPath := os.Getenv(fakedocker.EnvRecordPath)
	if cfgPath == "" {
		return fmt.Errorf("%s is not set", fakedocker.EnvConfigPath)
	}
	if recordPath == "" {
		return fmt.Errorf("%s is not set", fakedocker.EnvRecordPath)
	}

	cfg, err := fakedocker.LoadConfig(cfgPath)
	if err != nil {
		return err
	}
	records, err := fakedocker.ReadInvocations(recordPath)
	if err != nil {
		return err
	}

	subcommand := fakedocker.Classify(args)
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getwd: %w", err)
	}
	if err := fakedocker.AppendInvocation(recordPath, fakedocker.Invocation{
		Args:       append([]string(nil), args...),
		Dir:        wd,
		Subcommand: subcommand,
	}); err != nil {
		return err
	}

	resp, ok := fakedocker.ResponseFor(cfg, records, subcommand)
	if !ok {
		return fmt.Errorf("no fake response configured for %q", subcommand)
	}
	if _, err := os.Stdout.WriteString(resp.Stdout); err != nil {
		return fmt.Errorf("write stdout: %w", err)
	}
	if _, err := os.Stderr.WriteString(resp.Stderr); err != nil {
		return fmt.Errorf("write stderr: %w", err)
	}
	if resp.ExitCode != 0 {
		os.Exit(resp.ExitCode)
	}
	return nil
}
