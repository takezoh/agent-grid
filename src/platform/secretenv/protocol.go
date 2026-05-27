// Package secretenv implements the host-side broker for secret env-file resolution.
// The container shim (a roost-bridge wrapper named "credproxy") sends an env-file
// path to the broker over a per-project Unix socket. The broker gates the path,
// resolves secrets via the configured hook, and returns the resolved env map.
package secretenv

// ContainerSockName is the socket file name placed under ContainerRunDir.
// roost-bridge dials this path to reach the host broker; provider.go mounts
// the host-side socket to this same container-side path. Both sides reference
// this constant so they stay in sync when ContainerRunDir changes.
const ContainerSockName = "secretenv.sock"

// Request is the message sent by the container shim to the broker.
type Request struct {
	EnvFilePath string `json:"env_file_path"`
}

// Response is the message returned by the broker.
// On success Env holds the resolved name→value map and Error is empty.
// On failure Env is nil and Error holds a human-readable message.
type Response struct {
	Env   map[string]string `json:"env,omitempty"`
	Error string            `json:"error,omitempty"`
}
