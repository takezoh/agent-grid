package dockercli

import (
	"fmt"
	"os"
	"os/exec"

	platformconfig "github.com/takezoh/agent-grid/platform/config"
)

type SetupResult struct {
	DOCKERHOST       string
	UsingRootless    bool
	UsingDefaultSock bool
}

// Setup verifies docker is available and configures DOCKER_HOST for rootless
// setups when the caller has not set one explicitly.
func Setup(socketExists func(string) bool) (SetupResult, error) {
	if _, err := exec.LookPath("docker"); err != nil {
		return SetupResult{}, fmt.Errorf("sandbox: devcontainer mode requires docker in PATH: %w", err)
	}

	currentHost := os.Getenv("DOCKER_HOST")
	if host := platformconfig.ResolveDockerHost(
		currentHost,
		os.Getenv("XDG_RUNTIME_DIR"),
		socketExists,
	); host != "" {
		_ = os.Setenv("DOCKER_HOST", host)
		return SetupResult{DOCKERHOST: host, UsingRootless: true}, nil
	}
	return SetupResult{UsingDefaultSock: currentHost == ""}, nil
}
