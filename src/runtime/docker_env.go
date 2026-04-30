package runtime

import "path/filepath"

// ResolveDockerHost returns the DOCKER_HOST value to set, or "" to leave the
// daemon's environment untouched.
//
// Pure: callers inject env values and a stat callback so this is testable without
// touching the filesystem. Detection order:
//  1. If envDockerHost is already set, return "" (respect the caller's config).
//  2. If xdgRuntimeDir contains a docker.sock, return unix://<path>.
//  3. Otherwise return "" (fall back to docker's default /var/run/docker.sock).
func ResolveDockerHost(envDockerHost, xdgRuntimeDir string, socketExists func(string) bool) string {
	if envDockerHost != "" {
		return ""
	}
	if xdgRuntimeDir == "" {
		return ""
	}
	sock := filepath.Join(xdgRuntimeDir, "docker.sock")
	if !socketExists(sock) {
		return ""
	}
	return "unix://" + sock
}
