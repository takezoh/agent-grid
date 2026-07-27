package framelaunch

import "strings"

// ComputeFinalPathDecision records what computeFinalPath did, for observability
// (see framelaunch.path_reassert slog record in Run()).
type ComputeFinalPathDecision struct {
	// Branch is one of: "merged" (normal case-D prepend + dedup), "no_op"
	// (should not occur outside preExec branch — no reassert happened),
	// "toggle_disabled" (rollback toggle skipped merge).
	Branch string
	// PrefixCount is len(runtimeList) as observed at merge time.
	PrefixCount int
	// DroppedCount is the number of PATH segments dedupPath eliminated.
	DroppedCount int
	// HeadChanged is true when the first segment of PATH differs from the first
	// segment of the input base (capturedPath or origPath).
	HeadChanged bool
}

// dedupPath deduplicates a colon-separated PATH string. The first occurrence
// of each segment wins; empty segments are dropped. Byte-exact segment
// comparison — a trailing slash makes two segments distinct (matching bash's
// PATH resolver and platform/sandbox/devcontainer/spec.go:deduplicateColonList).
//
// Empty input yields empty output; single-segment input yields the same
// segment; a leading, trailing, or interior colon (indicating an empty
// segment, which bash treats as CWD) is dropped, per case-D contract.
func dedupPath(s string) (string, int) {
	if s == "" {
		return "", 0
	}
	segs := strings.Split(s, ":")
	seen := make(map[string]struct{}, len(segs))
	out := make([]string, 0, len(segs))
	dropped := 0
	for _, seg := range segs {
		if seg == "" {
			dropped++
			continue
		}
		if _, ok := seen[seg]; ok {
			dropped++
			continue
		}
		seen[seg] = struct{}{}
		out = append(out, seg)
	}
	return strings.Join(out, ":"), dropped
}

// computeFinalPath composes the PATH for a PreExec-branched frame launch.
// It always prepends the runtimeList entries in their given order, then
// deduplicates. The base is capturedPath if non-empty, else origPath — this
// preserves preExec's PATH work while ensuring runtime shim dirs win.
//
// Case-D structural invariant: this function does NOT read runtimeList from
// origPath (no extraction predicate) — the caller supplies the SSOT list.
//
// Guarantees:
//   - never returns an empty string when runtimeList is non-empty
//   - never returns a string beginning with an empty segment
//   - order-preserving deduplication (first occurrence wins)
//   - pure: does not touch os.Setenv or any process env
func computeFinalPath(runtimeList []string, capturedPath, origPath string) (string, ComputeFinalPathDecision) {
	base := capturedPath
	if base == "" {
		base = origPath
	}
	var merged string
	switch {
	case base == "":
		merged = strings.Join(runtimeList, ":")
	case len(runtimeList) == 0:
		merged = base
	default:
		merged = strings.Join(runtimeList, ":") + ":" + base
	}
	final, dropped := dedupPath(merged)
	decision := ComputeFinalPathDecision{
		Branch:       "merged",
		PrefixCount:  len(runtimeList),
		DroppedCount: dropped,
		HeadChanged:  headSegment(final) != headSegment(base),
	}
	return final, decision
}

func headSegment(s string) string {
	if i := strings.IndexByte(s, ':'); i >= 0 {
		return s[:i]
	}
	return s
}
