package prompt_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/takezoh/agent-roost/orchestrator/prompt"
	"github.com/takezoh/agent-roost/platform/tracker"
)

func TestRender_interpolatesIssue(t *testing.T) {
	iss := tracker.Issue{Identifier: "PROJ-1", Title: "Fix login"}
	out, err := prompt.Render(
		"Issue: {{ issue.identifier }} — {{ issue.title }}",
		prompt.Vars{Issue: iss, Attempt: 1},
	)
	assert.NoError(t, err)
	assert.Equal(t, "Issue: PROJ-1 — Fix login", out)
}

func TestRender_interpolatesAttempt(t *testing.T) {
	out, err := prompt.Render("Attempt {{ attempt }}", prompt.Vars{Attempt: 3})
	assert.NoError(t, err)
	assert.Equal(t, "Attempt 3", out)
}

func TestRender_unknownVariableErrors(t *testing.T) {
	_, err := prompt.Render("{{ unknown_var }}", prompt.Vars{})
	assert.True(t, errors.Is(err, prompt.ErrTemplateRender), "want ErrTemplateRender, got %v", err)
}

func TestRender_unknownFilterErrors(t *testing.T) {
	iss := tracker.Issue{Title: "hello"}
	_, err := prompt.Render("{{ issue.title | nonexistent_filter }}", prompt.Vars{Issue: iss})
	assert.True(t, errors.Is(err, prompt.ErrTemplateRender), "want ErrTemplateRender, got %v", err)
}

func TestRender_emptyTemplateReturnsDefault(t *testing.T) {
	out, err := prompt.Render("", prompt.Vars{})
	assert.NoError(t, err)
	assert.NotEmpty(t, out)
}

func TestRender_allIssueFields(t *testing.T) {
	prio := 1
	iss := tracker.Issue{
		ID:          "id-1",
		Identifier:  "PROJ-2",
		Title:       "T",
		Description: "D",
		State:       "active",
		BranchName:  "feature/proj-2",
		URL:         "https://example.com",
		Labels:      []string{"bug"},
		Priority:    &prio,
	}
	tmpl := "{{ issue.id }}|{{ issue.identifier }}|{{ issue.title }}|{{ issue.state }}|{{ issue.branch_name }}|{{ issue.url }}|{{ issue.priority }}"
	out, err := prompt.Render(tmpl, prompt.Vars{Issue: iss})
	assert.NoError(t, err)
	assert.Equal(t, "id-1|PROJ-2|T|active|feature/proj-2|https://example.com|1", out)
}
