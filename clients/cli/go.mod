// clients/cli is an independent Go module (adr-20260814-cli-independent-go-module).
//
// It is a client of the public wire contract and nothing else: it must not gain
// a require or replace edge to github.com/takezoh/agent-grid/services, and
// ver-no-services-module-edge fails the moment one appears. There is no root
// go.mod and no go.work tying the two together, which is what makes that check
// meaningful rather than a convention.
module github.com/takezoh/agent-grid/clients/cli

go 1.26.1

require golang.org/x/term v0.41.0

require (
	github.com/coder/websocket v1.8.15
	github.com/takezoh/agent-grid/clients/goclient v0.0.0
	golang.org/x/sys v0.42.0
)

replace github.com/takezoh/agent-grid/clients/goclient => ../goclient
