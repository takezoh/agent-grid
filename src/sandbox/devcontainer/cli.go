// Package devcontainer implements sandbox.Manager using direct docker commands.
// @devcontainers/cli is used only for image build (devcontainer build).
// Container lifecycle (create/start/exec/rm) is managed by roost directly.
package devcontainer

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// CLI wraps the @devcontainers/cli tool for image build only.
type CLI struct {
	binPath string
}

// NewCLI returns a CLI using binPath (PATH-resolved name or absolute path).
// Returns error when the binary cannot be found.
func NewCLI(binPath string) (*CLI, error) {
	if binPath == "" {
		binPath = "devcontainer"
	}
	if _, err := exec.LookPath(binPath); err != nil {
		return nil, fmt.Errorf("devcontainer CLI not found (%q): %w\n  install: npm install -g @devcontainers/cli", binPath, err)
	}
	return &CLI{binPath: binPath}, nil
}

// Build runs "devcontainer build" and returns the built image name.
// workspaceFolder is the --workspace-folder arg; configPath is the materialized devcontainer.json;
// imageName is the target image name. extraArgs are appended verbatim to the CLI invocation.
func (c *CLI) Build(ctx context.Context, workspaceFolder, configPath, imageName string, extraArgs []string) (string, error) {
	args := []string{
		"build",
		"--workspace-folder", workspaceFolder,
		"--config", configPath,
		"--image-name", imageName,
		"--log-format", "json",
	}
	args = append(args, extraArgs...)

	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, c.binPath, args...)
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		tail := lastNLines(stdout.String(), 10)
		return "", fmt.Errorf("devcontainer build: %w\n%s", err, tail)
	}
	return parseBuildOutput(&stdout, imageName)
}

// buildOutcomeLine is the final JSON line emitted by "devcontainer build --log-format json".
type buildOutcomeLine struct {
	Outcome   string `json:"outcome"`
	ImageName string `json:"imageName"`
	Message   string `json:"message"`
}

func parseBuildOutput(r *bytes.Buffer, fallback string) (string, error) {
	var last buildOutcomeLine
	found := false
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var candidate buildOutcomeLine
		if err := json.Unmarshal([]byte(line), &candidate); err != nil {
			continue
		}
		if candidate.Outcome != "" {
			last = candidate
			found = true
		}
	}
	if !found {
		// Some versions emit no structured outcome; return the expected image name.
		return fallback, nil
	}
	if last.Outcome != "success" {
		return "", fmt.Errorf("devcontainer build: outcome=%s: %s", last.Outcome, last.Message)
	}
	if last.ImageName != "" {
		return last.ImageName, nil
	}
	return fallback, nil
}

func lastNLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
