package driver

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/takezoh/agent-grid/host/state"
	"github.com/takezoh/agent-grid/platform/agent/codexschema"
	"github.com/takezoh/agent-grid/platform/e2etest"
)

var updateReplayGolden = flag.Bool("update", false, "rewrite driver replay golden fixtures")

func TestCodexRecordedReplayGolden(t *testing.T) {
	recordingPath := filepath.Join("..", "..", "platform", "agent", "fakecodex", "testdata", "recordings", "default-turn.jsonl")
	items := e2etest.ReadJSONLFixture(t, recordingPath)

	driver, current, now := newCodex(t)
	ctx := state.FrameContext{
		ID:        "frame-1",
		IsRoot:    true,
		Project:   "/tmp/work",
		Command:   "codex",
		CreatedAt: now,
	}

	snapshots := make([]codexReplaySnapshot, 0, len(items))
	for i, item := range items {
		ev := codexReplayEvent(t, item, now.Add(time.Duration(i)*time.Second))
		next, _, view := driver.Step(current, ctx, ev)
		current = next.(CodexState)
		snapshots = append(snapshots, snapshotCodexView(item["method"].(string), view))
	}

	got, err := json.MarshalIndent(snapshots, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	goldenPath := filepath.Join("testdata", "codex_replay_golden.json")
	if *updateReplayGolden {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("mkdir golden dir: %v", err)
		}
		if err := os.WriteFile(goldenPath, append(got, '\n'), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if diff := cmp.Diff(string(want), string(append(got, '\n'))); diff != "" {
		t.Fatalf("golden drift (-want +got):\n%s", diff)
	}
}

type codexReplaySnapshot struct {
	Method     string   `json:"method"`
	Status     string   `json:"status"`
	Title      string   `json:"title,omitempty"`
	Model      string   `json:"model,omitempty"`
	Effort     string   `json:"effort,omitempty"`
	LogTabIDs  []string `json:"log_tab_ids,omitempty"`
	InfoLabels []string `json:"info_labels,omitempty"`
}

func snapshotCodexView(method string, view state.View) codexReplaySnapshot {
	snapshot := codexReplaySnapshot{
		Method: method,
		Status: view.Status.String(),
		Title:  view.Card.Title,
		Model:  view.Model,
		Effort: view.Effort,
	}
	for _, tab := range view.LogTabs {
		snapshot.LogTabIDs = append(snapshot.LogTabIDs, tab.Label+":"+string(tab.Kind))
	}
	for _, line := range view.InfoExtras {
		snapshot.InfoLabels = append(snapshot.InfoLabels, line.Label)
	}
	return snapshot
}

func codexReplayEvent(t *testing.T, item map[string]any, ts time.Time) state.DEvSubsystem {
	t.Helper()
	method, _ := item["method"].(string)
	params, _ := item["params"].(map[string]any)
	payload := state.SubsystemPayload{
		SessionID:      replayThreadID(params),
		TargetID:       replayThreadID(params),
		TranscriptPath: replayThreadPath(params),
	}
	switch method {
	case codexschema.MethodThreadStarted:
		return state.DEvSubsystem{Source: state.SubsystemStream, Kind: state.SubsystemSessionReady, Timestamp: ts, Payload: payload}
	case codexschema.MethodTurnStarted:
		return state.DEvSubsystem{Source: state.SubsystemStream, Kind: state.SubsystemTurnStarted, Timestamp: ts, Payload: payload}
	case codexschema.MethodThreadSettingsUpdated:
		payload.Model, payload.ModelSet = replayModel(params)
		payload.Effort, payload.EffortSet = replayEffort(params)
		return state.DEvSubsystem{Source: state.SubsystemStream, Kind: state.SubsystemMetadataUpdated, Timestamp: ts, Payload: payload}
	case codexschema.MethodTurnCompleted:
		return state.DEvSubsystem{Source: state.SubsystemStream, Kind: state.SubsystemTurnCompleted, Timestamp: ts, Payload: payload}
	default:
		t.Fatalf("unsupported recorded method: %s", method)
		return state.DEvSubsystem{}
	}
}

func replayThreadID(params map[string]any) string {
	if threadID, ok := params["threadId"].(string); ok && threadID != "" {
		return threadID
	}
	thread, _ := params["thread"].(map[string]any)
	if threadID, ok := thread["id"].(string); ok {
		return threadID
	}
	return ""
}

func replayThreadPath(params map[string]any) string {
	thread, _ := params["thread"].(map[string]any)
	if path, ok := thread["path"].(string); ok {
		return path
	}
	if path, ok := params["path"].(string); ok {
		return path
	}
	return ""
}

func replayModel(params map[string]any) (string, bool) {
	settings, _ := params["threadSettings"].(map[string]any)
	model, ok := settings["model"].(string)
	return model, ok
}

func replayEffort(params map[string]any) (string, bool) {
	settings, _ := params["threadSettings"].(map[string]any)
	for _, key := range []string{"effort", "reasoning_effort"} {
		raw, ok := settings[key].(map[string]any)
		if !ok {
			continue
		}
		level, ok := raw["level"].(string)
		if ok {
			return level, true
		}
	}
	return "", false
}
