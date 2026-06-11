package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/takezoh/agent-reactor/client/config"
	"github.com/takezoh/agent-reactor/client/proto"
	psess "github.com/takezoh/agent-reactor/client/proto/sessions"
	"github.com/takezoh/agent-reactor/client/tools"
	"github.com/takezoh/agent-reactor/client/tui"
	"github.com/takezoh/agent-reactor/client/tui/glyphs"
	"github.com/takezoh/agent-reactor/platform/appid"
	platformconfig "github.com/takezoh/agent-reactor/platform/config"
	"github.com/takezoh/agent-reactor/platform/features"
	"github.com/takezoh/agent-reactor/platform/lib/git"
	"github.com/takezoh/agent-reactor/platform/lib/openurl"
	"github.com/takezoh/agent-reactor/platform/logger"
)

type tuiBootstrapOpts struct {
	Subscribe    bool
	AllowOffline bool
}

// tuiBootstrap loads config, applies theme, initialises glyphs, and dials the IPC socket.
// If AllowOffline is true and Dial fails, returns (cfg, nil, nil).
// If Subscribe is true and Dial succeeds, calls client.Subscribe().
// Caller must defer client.Close() when client is non-nil.
func tuiBootstrap(opts tuiBootstrapOpts) (*config.Config, *psess.Client, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, nil, err
	}
	initThemes(cfg.Theme)
	initGlyphs()
	sockPath := filepath.Join(cfg.ResolveDataDir(), appid.SocketFileName)

	raw, err := proto.Dial(sockPath)
	if err != nil {
		if opts.AllowOffline {
			return cfg, nil, nil
		}
		return nil, nil, fmt.Errorf("connect: %w", err)
	}
	client := psess.Wrap(raw)

	if opts.Subscribe {
		_ = client.Subscribe()
	}
	return cfg, client, nil
}

// initThemes loads user themes from ~/.agent-reactor/themes/ then selects the active
// theme from ROOST_THEME env (highest priority), the config value, or "default".
func initThemes(cfgTheme string) {
	if home, err := os.UserHomeDir(); err == nil {
		tui.LoadThemesFromDir(filepath.Join(home, appid.DotDir, "themes"))
	}
	name := cfgTheme
	if env := os.Getenv("ROOST_THEME"); env != "" {
		name = env
	}
	tui.ApplyTheme(name)
}

// initGlyphs loads the optional user glyph override and applies the
// ROOST_GLYPHS environment variable (default: "nerd").
func initGlyphs() {
	home, err := os.UserHomeDir()
	if err == nil {
		if err := glyphs.Load(filepath.Join(home, appid.DotDir, "glyphs.json")); err != nil {
			slog.Warn("glyphs: load error", "err", err)
		}
	}
	if name := os.Getenv("ROOST_GLYPHS"); name != "" {
		glyphs.Use(name)
	}
}

func runHeaderTUI() error {
	_, client, err := tuiBootstrap(tuiBootstrapOpts{Subscribe: true, AllowOffline: true})
	if err != nil {
		return err
	}
	if client != nil {
		defer client.Close()
	}
	model := tui.NewHeaderModel(client)
	if _, err := tea.NewProgram(model).Run(); err != nil {
		return fmt.Errorf("header: %w", err)
	}
	return nil
}

func runMainTUI() error {
	_, client, err := tuiBootstrap(tuiBootstrapOpts{Subscribe: true, AllowOffline: true})
	if err != nil {
		return err
	}
	if client != nil {
		defer client.Close()
	}
	model := tui.NewMainModel(client)
	if _, err := tea.NewProgram(model).Run(); err != nil {
		return fmt.Errorf("main: %w", err)
	}
	return nil
}

func runLogViewer() error {
	_, client, err := tuiBootstrap(tuiBootstrapOpts{Subscribe: true, AllowOffline: true})
	if err != nil {
		return err
	}
	if client != nil {
		defer client.Close()
	}
	tui.SetOpenProject(openurl.Open)
	model := tui.NewLogModel(logger.LogFilePath(), client)
	if _, err := tea.NewProgram(model).Run(); err != nil {
		return fmt.Errorf("log: %w", err)
	}
	return nil
}

func runSessionList() error {
	cfg, client, err := tuiBootstrap(tuiBootstrapOpts{Subscribe: true, AllowOffline: false})
	if err != nil {
		return err
	}
	defer client.Close()
	model := tui.NewModel(client, cfg)
	if _, err := tea.NewProgram(model).Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	return nil
}

func runPalette(args []string) error { //nolint:funlen
	slog.Info("palette start", "args", args)
	cfg, client, err := tuiBootstrap(tuiBootstrapOpts{Subscribe: false, AllowOffline: false})
	if err != nil {
		slog.Error("palette bootstrap failed", "err", err)
		return err
	}
	slog.Info("palette connected")
	defer client.Close()

	var toolName string
	var scopeProject bool
	prefill := make(map[string]string)
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "--tool="):
			toolName = strings.TrimPrefix(a, "--tool=")
		case strings.HasPrefix(a, "--arg="):
			kv := strings.TrimPrefix(a, "--arg=")
			if parts := strings.SplitN(kv, "=", 2); len(parts) == 2 {
				prefill[parts[0]] = parts[1]
			}
		case a == "--scope=project":
			scopeProject = true
		}
	}

	sessions, activeID, activeOccupant, _, _, err := client.ListSessions()
	if err != nil {
		slog.Warn("palette: ListSessions failed", "err", err)
	}

	if scopeProject && activeID == "" {
		slog.Info("palette: project scope falling back to standard (no active session)")
		scopeProject = false
	}

	mainHasDriver := activeID != "" && activeOccupant == proto.OccupantFrame
	mainHasForkable := false
	var activeProject string
	var activeProjectPath string

	if activeID != "" {
		for _, s := range sessions {
			if s.ID == activeID {
				activeProjectPath = s.Project
				if scopeProject {
					activeProject = tools.ProjectDisplayName(s.Project)
					prefill["project"] = s.Project
				}
				if mainHasDriver && scopeProject {
					mainHasForkable = s.RootDriverForkable
				}
				break
			}
		}
	}

	scope := tools.ScopeStandard
	if scopeProject {
		scope = tools.ScopeProject
	}

	feats := features.FromConfig(cfg.Features.Enabled, features.All())
	reg := tools.DefaultRegistry(feats, tools.PaletteContext{
		Scope:                 scope,
		MainHasDriverFrame:    mainHasDriver,
		MainHasForkableDriver: mainHasForkable,
		PushCommands:          cfg.Session.PushCommands,
		HasActiveProject:      activeProjectPath != "",
	})
	roots := make([]string, len(cfg.Projects.ProjectRoots))
	for i, r := range cfg.Projects.ProjectRoots {
		roots[i] = platformconfig.ExpandPath(r)
	}
	sbResolver := platformconfig.NewSandboxResolver(cfg.Sandbox)
	ctx := &tools.ToolContext{
		Client: client,
		Config: tools.ToolConfig{
			DefaultCommand: cfg.Session.DefaultCommand,
			Commands:       cfg.Session.Commands,
			Projects:       cfg.ListProjects(),
			ProjectRoots:   roots,
			ActiveProject:  activeProjectPath,
			Editor: tools.EditorConfig{
				Command:    cfg.Editor.Command,
				Extensions: cfg.Editor.Extensions,
			},
		},
		Args:               prefill,
		IsGitProject:       git.IsRepo,
		IsSandboxedProject: func(path string) bool { return sbResolver.Resolve(path).IsSandboxed() },
	}

	model := tui.NewPaletteModel(reg, ctx, toolName, activeProject)
	if _, err := tea.NewProgram(model).Run(); err != nil {
		return fmt.Errorf("palette: %w", err)
	}
	return nil
}
