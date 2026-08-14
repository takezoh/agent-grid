module github.com/takezoh/agent-grid/clients/web-host

go 1.26.1

require (
	github.com/coder/websocket v1.8.15
	github.com/takezoh/agent-grid/clients/goclient v0.0.0
)

// The Go projection of the canonical client configuration contract lives in
// this repository and is versioned with it. The web host is one consumer of
// that contract, exactly like the CLI, Workspace and Windows Shell — not its
// origin (adr-20260814-shared-go-client-config-module).
replace github.com/takezoh/agent-grid/clients/goclient => ../goclient
