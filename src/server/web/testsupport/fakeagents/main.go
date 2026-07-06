package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	streamfake "github.com/takezoh/agent-reactor/client/runtime/subsystem/stream/fake"
	"github.com/takezoh/agent-reactor/platform/lib/claude/fakeclaude"
	claudehookpayload "github.com/takezoh/agent-reactor/platform/lib/claude/hookpayload"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "fakeagents: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	switch filepath.Base(os.Args[0]) {
	case "claude":
		return runClaude(args)
	case "codex":
		return runCodex(args)
	default:
		return fmt.Errorf("unsupported argv0 %q", os.Args[0])
	}
}

func runClaude(args []string) error {
	cfg := claudeConfig{
		sessionID: "fake-claude-session",
		model:     "claude-sonnet-4-5",
		effort:    "high",
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--resume":
			if i+1 < len(args) {
				cfg.sessionID = strings.TrimSpace(args[i+1])
				i++
			}
		case "--model":
			if i+1 < len(args) {
				cfg.model = strings.TrimSpace(args[i+1])
				i++
			}
		case "--effort":
			if i+1 < len(args) {
				cfg.effort = strings.TrimSpace(args[i+1])
				i++
			}
		}
	}
	if cfg.sessionID == "" {
		cfg.sessionID = "fake-claude-session"
	}
	if cfg.model == "" {
		cfg.model = "claude-sonnet-4-5"
	}
	if cfg.effort == "" {
		cfg.effort = "high"
	}

	transcriptPath, err := ensureClaudeTranscript()
	if err != nil {
		return err
	}
	if err := emitClaudeHook("SessionStart", claudeHookPayload{
		SessionID:      cfg.sessionID,
		HookEventName:  "SessionStart",
		TranscriptPath: transcriptPath,
		Source:         "startup",
		Model:          cfg.model,
		Effort:         cfg.effort,
	}); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "[READY] fake claude")
	return runClaudeInputLoop(cfg, transcriptPath)
}

func runClaudeInputLoop(cfg claudeConfig, transcriptPath string) error {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if err := emitClaudeHook("UserPromptSubmit", claudeHookPayload{
			SessionID:      cfg.sessionID,
			HookEventName:  "UserPromptSubmit",
			TranscriptPath: transcriptPath,
			Prompt:         line,
			Model:          cfg.model,
			Effort:         cfg.effort,
		}); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "prompt: %s\n", line)
		time.Sleep(100 * time.Millisecond)
		if err := emitClaudeHook("Stop", claudeHookPayload{
			SessionID:      cfg.sessionID,
			HookEventName:  "Stop",
			TranscriptPath: transcriptPath,
			Model:          cfg.model,
			Effort:         cfg.effort,
		}); err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, "[DONE] fake claude")
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("scan stdin: %w", err)
	}
	return waitForSignal()
}

type claudeConfig struct {
	sessionID string
	model     string
	effort    string
}

type claudeHookPayload struct {
	SessionID      string
	HookEventName  string
	TranscriptPath string
	Source         string
	Prompt         string
	Model          string
	Effort         string
}

func emitClaudeHook(eventName string, payload claudeHookPayload) error {
	cmds, err := hookCommands(filepath.Join(os.Getenv("HOME"), ".claude", "settings.json"), eventName)
	if err != nil {
		return err
	}
	hook := fakeclaude.HookPayload{
		SessionID:      payload.SessionID,
		HookEventName:  payload.HookEventName,
		TranscriptPath: payload.TranscriptPath,
		Source:         payload.Source,
		Prompt:         payload.Prompt,
		Model:          payload.Model,
		ModelSet:       payload.Model != "",
	}
	if payload.Effort != "" {
		hook.Effort = &claudehookpayload.Effort{Level: payload.Effort}
		hook.EffortSet = true
	}
	raw := fakeclaude.Marshal(hook)
	for _, command := range cmds {
		if err := runShellCommand(command, raw); err != nil {
			return fmt.Errorf("%s hook %q: %w", eventName, command, err)
		}
	}
	return nil
}

func ensureClaudeTranscript() (string, error) {
	path := filepath.Join(os.Getenv("HOME"), ".claude", "fake-transcript.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("mkdir transcript dir: %w", err)
	}
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		return "", fmt.Errorf("write transcript: %w", err)
	}
	return path, nil
}

func runCodex(args []string) error {
	if len(args) > 0 && args[0] == "app-server" {
		sock, err := codexListenSock(args[1:])
		if err != nil {
			return err
		}
		srv := streamfake.New(streamfake.Config{Sock: sock})
		if err := srv.Start(); err != nil {
			return err
		}
		defer srv.Stop()
		return waitForSignal()
	}
	cliArgs, err := parseCodexCLIArgs(args)
	if err != nil {
		return err
	}
	return streamfake.RunCLI(cliArgs)
}

func codexListenSock(args []string) (string, error) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--listen":
			if i+1 >= len(args) {
				return "", errors.New("codex app-server: --listen requires a value")
			}
			return trimUnixPrefix(args[i+1]), nil
		default:
			if strings.HasPrefix(args[i], "--listen=") {
				return trimUnixPrefix(strings.TrimPrefix(args[i], "--listen=")), nil
			}
		}
	}
	return "", errors.New("codex app-server: missing --listen")
}

func parseCodexCLIArgs(args []string) (streamfake.CLIArgs, error) {
	cfg := streamfake.CLIArgs{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "resume":
			if i+1 >= len(args) {
				return streamfake.CLIArgs{}, errors.New("codex resume requires thread id")
			}
			cfg.Resume = strings.TrimSpace(args[i+1])
			i++
		case "--remote":
			if i+1 >= len(args) {
				return streamfake.CLIArgs{}, errors.New("codex --remote requires socket")
			}
			cfg.Sock = trimUnixPrefix(args[i+1])
			i++
		case "-C", "--cd":
			if i+1 >= len(args) {
				return streamfake.CLIArgs{}, fmt.Errorf("%s requires a value", args[i])
			}
			cfg.Cwd = strings.TrimSpace(args[i+1])
			i++
		}
	}
	if cfg.Sock == "" {
		return streamfake.CLIArgs{}, errors.New("codex: missing --remote unix:// path")
	}
	return cfg, nil
}

func waitForSignal() error {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(ch)
	<-ch
	return nil
}

func hookCommands(path, eventName string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	hooks, _ := root["hooks"].(map[string]any)
	if hooks == nil {
		return nil, fmt.Errorf("%s missing hooks object", path)
	}
	entries, _ := hooks[eventName].([]any)
	var commands []string
	for _, entry := range entries {
		entryMap, _ := entry.(map[string]any)
		hookList, _ := entryMap["hooks"].([]any)
		for _, item := range hookList {
			hookMap, _ := item.(map[string]any)
			command, _ := hookMap["command"].(string)
			if strings.TrimSpace(command) != "" {
				commands = append(commands, command)
			}
		}
	}
	if len(commands) == 0 {
		return nil, fmt.Errorf("%s has no commands for %s", path, eventName)
	}
	return commands, nil
}

func runShellCommand(command string, stdin []byte) error {
	cmd := exec.Command("sh", "-lc", command)
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		<-done
		return ctx.Err()
	}
}

func trimUnixPrefix(sock string) string {
	return strings.TrimPrefix(strings.TrimSpace(sock), "unix://")
}
