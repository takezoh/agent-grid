package sandbox

import "testing"

func TestValidateLaunchSpec(t *testing.T) {
	cases := []struct {
		name    string
		spec    LaunchSpec
		wantErr bool
	}{
		{
			name: "argv only ok",
			spec: LaunchSpec{Argv: []string{"codex"}},
		},
		{
			name:    "command alone rejected",
			spec:    LaunchSpec{Command: "bash"},
			wantErr: true,
		},
		{
			name:    "argv and command exclusive",
			spec:    LaunchSpec{Argv: []string{"codex"}, Command: "bash"},
			wantErr: true,
		},
		{
			name:    "empty argv rejected",
			spec:    LaunchSpec{},
			wantErr: true,
		},
		{
			name: "precommands with argv ok",
			spec: LaunchSpec{Argv: []string{"codex"}, PreCommands: [][]string{{"true"}}},
		},
		{
			name: "preexec with argv ok",
			spec: LaunchSpec{Argv: []string{"codex"}, PreExec: "mise trust"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateLaunchSpec(tc.spec)
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
