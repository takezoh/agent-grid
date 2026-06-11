package connector

import (
	"sync"

	"github.com/takezoh/agent-reactor/client/state"
)

var registerOnce sync.Once

func RegisterDefaults() {
	registerOnce.Do(func() {
		state.RegisterConnector(GitHubConnector{})
	})
}
