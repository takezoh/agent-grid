package state

// Resume capsule domain (plans/cognitive-work-orchestration-ux.md §12.1,
// Phase 0). A capsule is a compact, automatically regenerated resume state
// for one session: objective, current state, next action, unresolved items.
// Generation is digest-driven: whenever the session's recent conversation
// turns change, a worker job rebuilds the capsule via the configured
// capsule command. Trust requirements (staleness display, provenance,
// one-action wrong-flag) are part of the domain, not UI polish.

import (
	"crypto/sha256"
	"encoding/hex"
	"hash/fnv"
	"strings"
	"time"
)

// ResumeCapsule is the per-session capsule record. It persists across
// daemon restarts inside the session snapshot and is cleared together
// with the session.
//
// The completion fields (Changed/Verified/Remaining/Attention) are the
// compressed completion report of plan §18, generated in the same LLM
// call as the resume fields so completion reporting never doubles
// generation cost.
type ResumeCapsule struct {
	Objective    string
	CurrentState string
	NextAction   string
	Unresolved   []string
	Changed      []string // completion report: what was concretely changed
	Verified     []string // completion report: what was verified (tests, checks)
	Remaining    []string // completion report: what remains to be done
	Attention    string   // completion report: single line when user attention is needed
	GeneratedAt  time.Time
	SourceDigest string // digest of the turns the capsule was generated from
	SourceTurns  int    // provenance: number of turns fed to the generator
	FlaggedWrong bool
	FlaggedAt    time.Time
	LastError    string // non-empty when the latest generation attempt failed
	LastErrorAt  time.Time
}

// CapsuleTurn is a driver-neutral conversation turn handed to the capsule
// generator through the CapsuleSource capability.
type CapsuleTurn struct {
	Role string
	Text string
}

// CapsuleSource is an optional driver capability. Drivers that can expose
// recent conversation turns for resume-capsule generation implement it;
// sessions whose driver does not implement it never get a capsule.
type CapsuleSource interface {
	CapsuleTurns(s DriverState) []CapsuleTurn
}

// CapsuleJobInput carries a fully assembled capsule prompt to the worker
// pool. Prompt construction is pure and happens in the reducer.
type CapsuleJobInput struct {
	SessionID    SessionID
	Prompt       string
	SourceDigest string
	SourceTurns  int
}

func (CapsuleJobInput) JobKind() string { return "resume_capsule" }

// CapsuleJobResult is the parsed generator output. A zero-value result
// (no digest) means the runner was not configured and the job is a no-op.
type CapsuleJobResult struct {
	Objective    string
	CurrentState string
	NextAction   string
	Unresolved   []string
	Changed      []string
	Verified     []string
	Remaining    []string
	Attention    string
	SourceDigest string
	SourceTurns  int
}

const (
	// capsuleCheckInterval gates the digest comparison to roughly one check
	// per session per this many ticks (≈seconds at the default tick rate).
	capsuleCheckInterval = 15
	// capsuleErrorBackoff suppresses regeneration retries after a failed
	// generation so a persistently failing command cannot hammer the LLM.
	capsuleErrorBackoff = 5 * time.Minute
	capsuleEntryTextCap = 2000
	capsuleTotalTextCap = 16000
)

// capsuleDigest fingerprints the turns that would feed the generator.
func capsuleDigest(turns []CapsuleTurn) string {
	h := sha256.New()
	for _, t := range turns {
		h.Write([]byte(t.Role))
		h.Write([]byte{0})
		h.Write([]byte(t.Text))
		h.Write([]byte{0xff})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// maybeEnqueueResumeCapsules scans sessions on the tick and enqueues a
// capsule generation job for each session whose conversation advanced
// since its last capsule. Pure: all effects are returned.
func maybeEnqueueResumeCapsules(s State, tickN uint64) (State, []Effect) {
	if !s.ResumeCapsulesEnabled {
		return s, nil
	}
	var effs []Effect
	for sessID, sess := range s.Sessions {
		if !capsuleCheckDue(sessID, tickN) {
			continue
		}
		next, eff, ok := maybeEnqueueOneCapsule(s, sessID, sess)
		if !ok {
			continue
		}
		s = next
		effs = append(effs, eff)
	}
	return s, effs
}

// capsuleCheckDue staggers per-session digest checks across ticks so all
// sessions do not fire on the same tick.
func capsuleCheckDue(sessID SessionID, tickN uint64) bool {
	h := fnv.New32a()
	h.Write([]byte(sessID))
	return (tickN+uint64(h.Sum32()))%capsuleCheckInterval == 0
}

func maybeEnqueueOneCapsule(s State, sessID SessionID, sess Session) (State, Effect, bool) {
	if _, inFlight := s.CapsuleGenerating[sessID]; inFlight {
		return s, nil, false
	}
	frame, ok := rootFrame(sess)
	if !ok {
		return s, nil, false
	}
	drv := GetDriver(frame.Command)
	if drv == nil {
		return s, nil, false
	}
	src, ok := drv.(CapsuleSource)
	if !ok {
		return s, nil, false
	}
	// Skip mid-turn regeneration: wait until the agent is not actively
	// producing output so the capsule reflects a settled state.
	if drv.Status(frame.Driver) == StatusRunning {
		return s, nil, false
	}
	turns := src.CapsuleTurns(frame.Driver)
	if len(turns) == 0 {
		return s, nil, false
	}
	digest := capsuleDigest(turns)
	if prev := sess.ResumeCapsule; prev != nil {
		if prev.SourceDigest == digest {
			return s, nil, false
		}
		if prev.LastError != "" && s.Now.Sub(prev.LastErrorAt) < capsuleErrorBackoff {
			return s, nil, false
		}
	}
	var prev ResumeCapsule
	if sess.ResumeCapsule != nil {
		prev = *sess.ResumeCapsule
	}
	input := CapsuleJobInput{
		SessionID:    sessID,
		Prompt:       formatResumeCapsulePrompt(prev, turns, sess.Project),
		SourceDigest: digest,
		SourceTurns:  len(turns),
	}
	eff, next := postProcessEffect(s, sessID, frame.ID, EffStartJob{Input: input})
	s = next
	s.CapsuleGenerating = cloneMap(s.CapsuleGenerating)
	if s.CapsuleGenerating == nil {
		s.CapsuleGenerating = map[SessionID]string{}
	}
	s.CapsuleGenerating[sessID] = digest
	return s, eff, true
}

// formatResumeCapsulePrompt builds the generation prompt. The generator
// must return strict JSON so the runner can parse it into typed fields;
// non-JSON output is treated as a failed generation, never displayed.
func formatResumeCapsulePrompt(prev ResumeCapsule, turns []CapsuleTurn, project string) string {
	var b strings.Builder
	b.WriteString("You maintain a resume capsule for an AI coding session: a compact work-state ")
	b.WriteString("summary that lets the user resume after an interruption without rereading history. ")
	b.WriteString("From the previous capsule and the recent conversation turns below, produce the ")
	b.WriteString("updated capsule.\n\n")
	b.WriteString("Return ONLY a JSON object, no code fences, no prose, with exactly these keys:\n")
	b.WriteString(`{"objective": "...", "current_state": "...", "next_action": "...", "unresolved": ["..."],` + "\n")
	b.WriteString(` "changed": ["..."], "verified": ["..."], "remaining": ["..."], "attention": "..."}` + "\n\n")
	b.WriteString("objective: the overall goal the user is driving (1 sentence, stable across turns).\n")
	b.WriteString("current_state: what has been established or completed so far (1-2 sentences).\n")
	b.WriteString("next_action: the single next concrete step (1 sentence).\n")
	b.WriteString("unresolved: open questions or pending decisions (0-4 short items).\n")
	b.WriteString("changed: concrete changes completed so far — files, behaviors, documents (0-4 short items).\n")
	b.WriteString("verified: what has actually been verified — tests run, checks passed (0-3 short items; ")
	b.WriteString("only include evidence present in the conversation, never assumed).\n")
	b.WriteString("remaining: work that is still left to do (0-4 short items).\n")
	b.WriteString("attention: one sentence when the user's decision or review is needed, else an empty string.\n")
	b.WriteString("Base every field on the conversation only; never invent progress or verification.\n\n")
	if project != "" {
		b.WriteString("<project>\n" + project + "\n</project>\n\n")
	}
	if prev.GeneratedAt != (time.Time{}) {
		b.WriteString("<previous_capsule>\n")
		b.WriteString("objective: " + prev.Objective + "\n")
		b.WriteString("current_state: " + prev.CurrentState + "\n")
		b.WriteString("next_action: " + prev.NextAction + "\n")
		if len(prev.Unresolved) > 0 {
			b.WriteString("unresolved: " + strings.Join(prev.Unresolved, "; ") + "\n")
		}
		b.WriteString("</previous_capsule>\n\n")
	}
	b.WriteString("<recent_turns>\n")
	b.WriteString(renderCapsuleTurns(turns))
	b.WriteString("</recent_turns>\n")
	return b.String()
}

func renderCapsuleTurns(turns []CapsuleTurn) string {
	var blocks []string
	total := 0
	// Newest-last input; keep the newest turns when the total cap is hit.
	for i := len(turns) - 1; i >= 0; i-- {
		t := turns[i]
		text := capsuleTailClip(t.Text, capsuleEntryTextCap)
		block := "[" + t.Role + "]\n" + text + "\n"
		if total+len(block) > capsuleTotalTextCap && len(blocks) > 0 {
			break
		}
		total += len(block)
		blocks = append([]string{block}, blocks...)
	}
	return strings.Join(blocks, "\n")
}

func capsuleTailClip(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return "…" + string(r[len(r)-max:])
}
