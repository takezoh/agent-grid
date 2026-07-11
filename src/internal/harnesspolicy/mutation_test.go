package harnesspolicy

import (
	"context"
	"strings"
	"testing"
)

func TestApplyMutantPinsNormalizedHashIdentityAndSpan(t *testing.T) {
	source := []byte("a\r\ncondition\r\nz")
	hash := NormalizedSourceHash(source)
	mutant := Mutant{Path: "x.go", Start: 3, End: 12, Operator: "conditional-negation", SourceHash: hash, Before: "condition", Replacement: "!condition"}
	mutant.ID = MutationIdentity(mutant.Path, mutant.Start, mutant.End, mutant.Operator, hash)
	got, err := ApplyMutant(source, mutant)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "a\r\n!condition\r\nz" {
		t.Fatalf("mutation = %q", got)
	}
	mutant.Start++
	if _, err := ApplyMutant(source, mutant); err == nil || !strings.Contains(err.Error(), "identity drift") {
		t.Fatalf("expected identity drift, got %v", err)
	}
}

type fakeMutationRunner struct{ outcomes map[string]MutationOutcome }

func (runner fakeMutationRunner) Run(_ context.Context, mutant Mutant) MutationOutcome {
	return runner.outcomes[mutant.ID]
}

func TestRunMutantsIsSortedAndTimeoutSurvives(t *testing.T) {
	mutants := []Mutant{{ID: "z"}, {ID: "a"}}
	outcomes := RunMutants(context.Background(), mutants, fakeMutationRunner{outcomes: map[string]MutationOutcome{
		"z": {Result: "timeout", ExitCode: 124}, "a": {Result: "killed", ExitCode: 1},
	}})
	if outcomes[0].ID != "a" || outcomes[0].Result != "killed" || outcomes[1].Result != "survived" {
		t.Fatalf("outcomes = %#v", outcomes)
	}
}

func TestValidateMutationResultsRejectsSurvivorScoreAndRunnerDrift(t *testing.T) {
	runner := []byte("runner\n")
	manifest := MutationManifest{Mutants: []Mutant{{ID: "a"}, {ID: "b"}}}
	baseline := MutationBaseline{MinimumScore: 1, RunnerHash: NormalizedSourceHash(runner), Mutants: []BaselineMutant{{ID: "a", MustKill: true}, {ID: "b", MustKill: true}}}
	outcomes := []MutationOutcome{{ID: "a", Result: "killed"}, {ID: "b", Result: "survived"}}
	err := ValidateMutationResults(manifest, baseline, []byte("changed runner"), outcomes)
	for _, want := range []string{"runner hash drift", "must-kill mutant survived", "score below baseline"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not contain %q", err, want)
		}
	}
}

func TestParseMutationManifestRejectsUnknownOrIncompleteOperatorSet(t *testing.T) {
	_, err := ParseMutationManifest([]byte(`{"version":1,"operator_set_version":"v1","seed":20260711,"mutants":[{"id":"x","path":"x","start":0,"end":1,"operator":"random","source_hash":"h","before":"x","replacement":"y","command":["test"],"working_dir":".","timeout_seconds":1}]}`))
	if err == nil || !strings.Contains(err.Error(), "unknown operator") {
		t.Fatalf("expected operator rejection, got %v", err)
	}
}

func TestValidateMutationBaselineUpdateRejectsLoweringAndUnapprovedChange(t *testing.T) {
	trusted := MutationBaseline{MinimumScore: 1, RunnerHash: "runner", Mutants: []BaselineMutant{{ID: "a", MustKill: true}}}
	lowered := trusted
	lowered.MinimumScore = 0.5
	if err := ValidateMutationBaselineUpdate(trusted, lowered, true); err == nil || !strings.Contains(err.Error(), "lowered") {
		t.Fatalf("expected score lowering rejection, got %v", err)
	}
	changed := trusted
	changed.RunnerHash = "candidate-runner"
	if err := ValidateMutationBaselineUpdate(trusted, changed, false); err == nil || !strings.Contains(err.Error(), "approval") {
		t.Fatalf("expected unapproved update rejection, got %v", err)
	}
	if err := ValidateMutationBaselineUpdate(trusted, changed, true); err != nil {
		t.Fatalf("approved non-weakening update rejected: %v", err)
	}
}
