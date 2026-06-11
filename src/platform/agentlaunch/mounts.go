package agentlaunch

import "github.com/takezoh/agent-reactor/platform/pathmap"

// HostPath translates a container-absolute path to its host-side path using the
// launch's bind mounts, so the daemon can dial a UDS that an in-container
// process listens on. Returns ("", false) when no mount covers the path — the
// host-mode case, where there are no mounts and the listen path is already a
// host path.
func (w WrappedLaunch) HostPath(containerPath string) (string, bool) {
	if len(w.Mounts) == 0 {
		return "", false
	}
	pm := make(pathmap.Mounts, len(w.Mounts))
	for i, m := range w.Mounts {
		pm[i] = pathmap.Mount{Host: m.Host, Container: m.Container}
	}
	return pm.ToHost(containerPath)
}
