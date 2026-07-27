package harnesspolicy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

const (
	MutationSeed       = 20260711
	MutationOperatorV1 = "v1"
)

var mutationOperators = map[string]bool{
	"conditional-negation":      true,
	"route-target-substitution": true,
	"event-drop":                true,
	"codec-field-omission":      true,
}

type MutationManifest struct {
	Version            int      `json:"version"`
	OperatorSetVersion string   `json:"operator_set_version"`
	Seed               int      `json:"seed"`
	Mutants            []Mutant `json:"mutants"`
}

type Mutant struct {
	ID             string   `json:"id"`
	Path           string   `json:"path"`
	Start          int      `json:"start"`
	End            int      `json:"end"`
	Operator       string   `json:"operator"`
	SourceHash     string   `json:"source_hash"`
	Before         string   `json:"before"`
	Replacement    string   `json:"replacement"`
	Command        []string `json:"command"`
	WorkingDir     string   `json:"working_dir"`
	TimeoutSeconds int      `json:"timeout_seconds"`
}

type MutationBaseline struct {
	Version            int              `json:"version"`
	OperatorSetVersion string           `json:"operator_set_version"`
	Seed               int              `json:"seed"`
	MinimumScore       float64          `json:"minimum_score"`
	RunnerHash         string           `json:"runner_hash"`
	Mutants            []BaselineMutant `json:"mutants"`
}

type BaselineMutant struct {
	ID       string `json:"id"`
	MustKill bool   `json:"must_kill"`
}

func ParseMutationManifest(data []byte) (MutationManifest, error) {
	var manifest MutationManifest
	if err := decodeStrictJSON(data, &manifest); err != nil {
		return manifest, fmt.Errorf("parse mutation manifest: %w", err)
	}
	if manifest.Version != 1 || manifest.OperatorSetVersion != MutationOperatorV1 || manifest.Seed != MutationSeed {
		return manifest, fmt.Errorf("mutation manifest contract mismatch")
	}
	seenOperators := make(map[string]bool)
	seenIDs := make(map[string]bool)
	for index, mutant := range manifest.Mutants {
		if !mutationOperators[mutant.Operator] {
			return manifest, fmt.Errorf("mutant %d has unknown operator %q", index, mutant.Operator)
		}
		seenOperators[mutant.Operator] = true
		if mutant.ID == "" || seenIDs[mutant.ID] {
			return manifest, fmt.Errorf("mutant %d has empty or duplicate identity", index)
		}
		seenIDs[mutant.ID] = true
		if mutant.Path == "" || mutant.Start < 0 || mutant.End <= mutant.Start || mutant.Before == "" ||
			len(mutant.Command) == 0 || mutant.WorkingDir == "" || mutant.TimeoutSeconds < 1 || mutant.TimeoutSeconds > 300 {
			return manifest, fmt.Errorf("mutant %s has invalid execution metadata", mutant.ID)
		}
	}
	for operator := range mutationOperators {
		if !seenOperators[operator] {
			return manifest, fmt.Errorf("operator set is incomplete: missing %s", operator)
		}
	}
	return manifest, nil
}

func ParseMutationBaseline(data []byte) (MutationBaseline, error) {
	var baseline MutationBaseline
	if err := decodeStrictJSON(data, &baseline); err != nil {
		return baseline, fmt.Errorf("parse mutation baseline: %w", err)
	}
	if baseline.Version != 1 || baseline.OperatorSetVersion != MutationOperatorV1 || baseline.Seed != MutationSeed ||
		baseline.MinimumScore < 0 || baseline.MinimumScore > 1 || baseline.RunnerHash == "" {
		return baseline, fmt.Errorf("mutation baseline contract mismatch")
	}
	return baseline, nil
}

func NormalizedSourceHash(source []byte) string {
	normalized := bytes.ReplaceAll(source, []byte("\r\n"), []byte("\n"))
	sum := sha256.Sum256(normalized)
	return hex.EncodeToString(sum[:])
}

func MutationIdentity(path string, start, end int, operator, sourceHash string) string {
	return fmt.Sprintf("%s@%d:%d#%s#%s", path, start, end, operator, sourceHash)
}

func ApplyMutant(source []byte, mutant Mutant) ([]byte, error) {
	if NormalizedSourceHash(source) != mutant.SourceHash {
		return nil, fmt.Errorf("%s: source hash drift", mutant.ID)
	}
	if MutationIdentity(mutant.Path, mutant.Start, mutant.End, mutant.Operator, mutant.SourceHash) != mutant.ID {
		return nil, fmt.Errorf("%s: identity drift", mutant.ID)
	}
	if mutant.End > len(source) || string(source[mutant.Start:mutant.End]) != mutant.Before {
		return nil, fmt.Errorf("%s: byte span drift", mutant.ID)
	}
	result := make([]byte, 0, len(source)-len(mutant.Before)+len(mutant.Replacement))
	result = append(result, source[:mutant.Start]...)
	result = append(result, mutant.Replacement...)
	result = append(result, source[mutant.End:]...)
	return result, nil
}

type MutationOutcome struct {
	ID       string `json:"id"`
	Result   string `json:"result"`
	ExitCode int    `json:"exit_code"`
	Duration int64  `json:"duration_ms"`
}

type MutationCommandRunner interface {
	Run(context.Context, Mutant) MutationOutcome
}

func RunMutants(ctx context.Context, mutants []Mutant, runner MutationCommandRunner) []MutationOutcome {
	ordered := append([]Mutant(nil), mutants...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
	results := make([]MutationOutcome, 0, len(ordered))
	for _, mutant := range ordered {
		outcome := runner.Run(ctx, mutant)
		outcome.ID = mutant.ID
		if outcome.Result == "timeout" || outcome.ExitCode == 0 || outcome.Result != "killed" {
			outcome.Result = "survived"
		}
		results = append(results, outcome)
	}
	return results
}

func ValidateMutationResults(manifest MutationManifest, baseline MutationBaseline, runnerSource []byte, outcomes []MutationOutcome) error {
	errs := make([]string, 0)
	if baseline.RunnerHash != NormalizedSourceHash(runnerSource) {
		errs = append(errs, "runner hash drift")
	}
	baselineByID := make(map[string]BaselineMutant)
	for _, mutant := range baseline.Mutants {
		baselineByID[mutant.ID] = mutant
	}
	outcomeByID := make(map[string]MutationOutcome)
	for _, outcome := range outcomes {
		outcomeByID[outcome.ID] = outcome
	}
	killed := 0
	for _, mutant := range manifest.Mutants {
		baselineMutant, ok := baselineByID[mutant.ID]
		if !ok {
			errs = append(errs, fmt.Sprintf("%s: missing from trusted baseline", mutant.ID))
			continue
		}
		outcome, ok := outcomeByID[mutant.ID]
		if !ok {
			errs = append(errs, fmt.Sprintf("%s: missing result", mutant.ID))
			continue
		}
		if outcome.Result == "killed" {
			killed++
		} else if baselineMutant.MustKill {
			errs = append(errs, fmt.Sprintf("%s: must-kill mutant survived", mutant.ID))
		}
	}
	if len(manifest.Mutants) == 0 || float64(killed)/float64(len(manifest.Mutants)) < baseline.MinimumScore {
		errs = append(errs, "mutation score below baseline")
	}
	sort.Strings(errs)
	if len(errs) > 0 {
		return fmt.Errorf("mutation policy violations:\n%s", strings.Join(errs, "\n"))
	}
	return nil
}

// ValidateMutationBaselineUpdate is the pure T2 half of the protected-file
// gate. Approval is supplied by the trusted merge-base gate, never by the
// candidate baseline itself.
func ValidateMutationBaselineUpdate(trusted, candidate MutationBaseline, approved bool) error {
	if candidate.MinimumScore < trusted.MinimumScore {
		return fmt.Errorf("mutation baseline score may not be lowered")
	}
	trustedMustKill := make(map[string]bool)
	for _, mutant := range trusted.Mutants {
		trustedMustKill[mutant.ID] = mutant.MustKill
	}
	for _, mutant := range candidate.Mutants {
		if trustedMustKill[mutant.ID] && !mutant.MustKill {
			return fmt.Errorf("mutation baseline must-kill may not be weakened: %s", mutant.ID)
		}
		delete(trustedMustKill, mutant.ID)
	}
	for id, mustKill := range trustedMustKill {
		if mustKill {
			return fmt.Errorf("mutation baseline must-kill may not be removed: %s", id)
		}
	}
	if !approved && mutationBaselineFingerprint(trusted) != mutationBaselineFingerprint(candidate) {
		return fmt.Errorf("mutation baseline update requires trusted approval")
	}
	return nil
}

func mutationBaselineFingerprint(baseline MutationBaseline) string {
	data, _ := json.Marshal(baseline)
	return NormalizedSourceHash(data)
}

func decodeStrictJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("trailing JSON value")
	}
	return nil
}
