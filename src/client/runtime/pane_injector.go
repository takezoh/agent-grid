package runtime

import (
	"fmt"

	"github.com/takezoh/agent-reactor/client/state"
)

// RuntimePaneInjector implements PromptInjector backed by the
// runtime's session pane map and PaneBackend.
type RuntimePaneInjector struct {
	panes   map[state.FrameID]string // frameID → pane target
	backend PaneBackend
}

// NewRuntimePaneInjector constructs an injector from the runtime's pane
// map and backend.
func NewRuntimePaneInjector(panes map[state.FrameID]string, backend PaneBackend) *RuntimePaneInjector {
	return &RuntimePaneInjector{panes: panes, backend: backend}
}

// ResolveFramePane returns the pane target registered for frameID, or
// ("", false) if unknown or empty.
func (inj *RuntimePaneInjector) ResolveFramePane(frameID state.FrameID) (string, bool) {
	target, ok := inj.panes[frameID]
	return target, ok && target != ""
}

// PastePrompt loads text into a named buffer then pastes it into target.
func (inj *RuntimePaneInjector) PastePrompt(target, text string) error {
	bufName := fmt.Sprintf("reactor-peer-%s", target)
	if err := inj.backend.LoadBuffer(bufName, text); err != nil {
		return err
	}
	return inj.backend.PasteBuffer(bufName, target)
}

// SubmitEnter sends the Enter key to target.
func (inj *RuntimePaneInjector) SubmitEnter(target string) error {
	return inj.backend.SendEnter(target)
}
