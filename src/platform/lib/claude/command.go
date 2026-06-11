package claude

import (
	"fmt"
	"os"
	"path/filepath"
)

// Run dispatches Claude subcommands.
func Run(args []string) error {
	if len(args) == 0 {
		printHelp()
		return fmt.Errorf("claude: missing subcommand")
	}
	switch args[0] {
	case "setup":
		return RunSetup()
	case "help", "-h", "--help":
		printHelp()
		return nil
	default:
		fmt.Fprintf(os.Stderr, "arc claude: unknown subcommand: %s\n", args[0])
		printHelp()
		return fmt.Errorf("claude: unknown subcommand: %s", args[0])
	}
}

func printHelp() {
	fmt.Print(`Usage: arc claude <command>

Commands:
  setup    Register arc hooks in ~/.claude/settings.json
  help     Show this help message
`)
}

// RunSetup registers arc hooks and MCP server in Claude's settings.
func RunSetup() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	roostPath, _ := os.Executable()
	if resolved, err := filepath.EvalSymlinks(roostPath); err == nil {
		roostPath = resolved
	}
	events, err := RegisterHooks(settingsPath, roostPath)
	if err != nil {
		return err
	}
	if len(events) == 0 {
		fmt.Println("Hooks already registered")
	} else {
		fmt.Printf("Registered events: %v\n", events)
		fmt.Printf("  Settings: %s\n", settingsPath)
	}
	added, err := RegisterMCPServer(settingsPath, roostPath)
	if err != nil {
		return err
	}
	if added {
		fmt.Printf("Registered MCP server: reactor-peers\n")
		fmt.Printf("  Settings: %s\n", settingsPath)
	} else {
		fmt.Println("MCP server reactor-peers already registered")
	}
	return nil
}
