package driver

import (
	"context"
	"strings"
	"testing"

	"github.com/takezoh/agent-grid/services/host/state"
)

func TestParseCapsuleJSON(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantErr bool
		want    state.CapsuleJobResult
	}{
		{
			name: "plain json",
			in:   `{"objective":"ship it","current_state":"tests green","next_action":"commit","unresolved":["naming"]}`,
			want: state.CapsuleJobResult{
				Objective: "ship it", CurrentState: "tests green",
				NextAction: "commit", Unresolved: []string{"naming"},
			},
		},
		{
			name: "fenced json with prose",
			in:   "Here is the capsule:\n```json\n{\"objective\":\"ship it\",\"current_state\":\"wip\"}\n```\nDone.",
			want: state.CapsuleJobResult{Objective: "ship it", CurrentState: "wip"},
		},
		{
			name: "completion report fields",
			in: `{"objective":"o","current_state":"c","changed":["fixed a"],` +
				`"verified":["tests pass"],"remaining":["docs"],"attention":" review me "}`,
			want: state.CapsuleJobResult{
				Objective: "o", CurrentState: "c",
				Changed: []string{"fixed a"}, Verified: []string{"tests pass"},
				Remaining: []string{"docs"}, Attention: "review me",
			},
		},
		{
			name: "empty unresolved entries dropped",
			in:   `{"objective":"o","current_state":"c","unresolved":["", "  ", "real"]}`,
			want: state.CapsuleJobResult{Objective: "o", CurrentState: "c", Unresolved: []string{"real"}},
		},
		{name: "no json", in: "sorry, I cannot do that", wantErr: true},
		{name: "invalid json", in: "{objective: broken}", wantErr: true},
		{name: "empty fields", in: `{"objective":"","current_state":"  "}`, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseCapsuleJSON(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Objective != tc.want.Objective || got.CurrentState != tc.want.CurrentState ||
				got.NextAction != tc.want.NextAction || got.Attention != tc.want.Attention {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
			lists := [][2][]string{
				{got.Unresolved, tc.want.Unresolved},
				{got.Changed, tc.want.Changed},
				{got.Verified, tc.want.Verified},
				{got.Remaining, tc.want.Remaining},
			}
			for _, pair := range lists {
				if strings.Join(pair[0], "|") != strings.Join(pair[1], "|") {
					t.Errorf("list mismatch: got %v, want %v", pair[0], pair[1])
				}
			}
		})
	}
}

func TestCapsuleRunnerUnconfigured(t *testing.T) {
	runner := newCapsuleRunner("")
	got, err := runner(context.Background(), state.CapsuleJobInput{Prompt: "p", SourceDigest: "d"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.SourceDigest != "" {
		t.Errorf("unconfigured runner must return zero result, got %+v", got)
	}
}

func TestCapsuleRunnerParsesCommandOutput(t *testing.T) {
	runner := newCapsuleRunner(`echo '{"objective":"o","current_state":"c","next_action":"n"}'`)
	got, err := runner(context.Background(), state.CapsuleJobInput{
		Prompt: "prompt", SourceDigest: "digest-9", SourceTurns: 3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Objective != "o" || got.SourceDigest != "digest-9" || got.SourceTurns != 3 {
		t.Errorf("unexpected result: %+v", got)
	}
}

func TestCapsuleRunnerCommandFailure(t *testing.T) {
	runner := newCapsuleRunner("false")
	if _, err := runner(context.Background(), state.CapsuleJobInput{Prompt: "p"}); err == nil {
		t.Fatal("expected error from failing command")
	}
}

func TestDriversImplementCapsuleSource(t *testing.T) {
	turns := []SummaryTurn{{Role: "user", Text: "hi"}, {Role: "assistant", Text: "yo"}}
	cases := []struct {
		name string
		src  state.CapsuleSource
		ds   state.DriverState
	}{
		{"claude", ClaudeDriver{}, ClaudeState{RecentTurns: turns}},
		{"codex", CodexDriver{}, CodexState{RecentTurns: turns}},
		{"gemini", GeminiDriver{}, GeminiState{RecentTurns: turns}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.src.CapsuleTurns(tc.ds)
			if len(got) != 2 || got[0].Role != "user" || got[1].Text != "yo" {
				t.Errorf("unexpected turns: %+v", got)
			}
			if tc.src.CapsuleTurns(nil) != nil {
				t.Error("wrong state type must yield nil turns")
			}
		})
	}
}
