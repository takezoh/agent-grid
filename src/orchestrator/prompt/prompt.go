// Package prompt renders WORKFLOW.md prompt templates per SPEC §5.4 / §12.
package prompt

import (
	"errors"
	"fmt"
	"sync"

	"github.com/osteele/liquid"
	"github.com/takezoh/agent-roost/platform/tracker"
)

const defaultPrompt = "You are working on an issue from Linear."

// ErrTemplateRender is returned when a template contains unknown variables or filters.
var ErrTemplateRender = errors.New("prompt: template render error")

// Vars holds the variables available in a WORKFLOW.md prompt template.
type Vars struct {
	Issue   tracker.Issue
	Attempt int
}

var (
	engineOnce sync.Once
	eng        *liquid.Engine
)

func engine() *liquid.Engine {
	engineOnce.Do(func() {
		eng = liquid.NewEngine()
		eng.StrictVariables()
	})
	return eng
}

// Render executes tmpl with vars. An empty template returns the default prompt.
// Unknown variables or filters return ErrTemplateRender.
func Render(tmpl string, vars Vars) (string, error) {
	if tmpl == "" {
		return defaultPrompt, nil
	}
	bindings := liquid.Bindings{
		"issue":   toIssueMap(vars.Issue),
		"attempt": vars.Attempt,
	}
	out, err := engine().ParseAndRenderString(tmpl, bindings)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrTemplateRender, err)
	}
	return out, nil
}

func toIssueMap(iss tracker.Issue) map[string]any {
	m := map[string]any{
		"id":          iss.ID,
		"identifier":  iss.Identifier,
		"title":       iss.Title,
		"description": iss.Description,
		"state":       iss.State,
		"branch_name": iss.BranchName,
		"url":         iss.URL,
		"labels":      iss.Labels,
	}
	if iss.Priority != nil {
		m["priority"] = *iss.Priority
	}
	return m
}
