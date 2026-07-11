package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/takezoh/agent-grid/platform/codexconfig"
)

func runCodexTrustProject(args []string) error {
	fs := flag.NewFlagSet("codex-trust-project", flag.ContinueOnError)
	project := fs.String("project", "", "container project path to trust")
	configPath := fs.String("config", "", "Codex config.toml path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *project == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("codex-trust-project: resolve cwd: %w", err)
		}
		*project = cwd
	}
	path := *configPath
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("codex-trust-project: resolve home: %w", err)
		}
		path = codexconfig.ConfigPath(os.Getenv("CODEX_HOME"), home)
	}
	return codexconfig.EnsureProjectTrusted(path, *project)
}
