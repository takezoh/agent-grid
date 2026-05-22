package main

import (
	"slices"
	"testing"
)

func TestClaudeArgs(t *testing.T) {
	t.Run("includes --verbose for stream-json (required by current claude)", func(t *testing.T) {
		got := claudeArgs("", "", "do the thing")
		want := []string{"-p", "--output-format", "stream-json", "--verbose", "do the thing"}
		if !slices.Equal(got, want) {
			t.Errorf("claudeArgs() = %v, want %v", got, want)
		}
	})

	t.Run("resume session inserts --resume before the prompt", func(t *testing.T) {
		got := claudeArgs("sess-123", "", "continue")
		want := []string{"-p", "--output-format", "stream-json", "--verbose", "--resume", "sess-123", "continue"}
		if !slices.Equal(got, want) {
			t.Errorf("claudeArgs() = %v, want %v", got, want)
		}
	})

	t.Run("append-system-prompt is inserted before resume and prompt", func(t *testing.T) {
		got := claudeArgs("sess-9", "tool rules", "go")
		want := []string{"-p", "--output-format", "stream-json", "--verbose",
			"--append-system-prompt", "tool rules", "--resume", "sess-9", "go"}
		if !slices.Equal(got, want) {
			t.Errorf("claudeArgs() = %v, want %v", got, want)
		}
	})
}
