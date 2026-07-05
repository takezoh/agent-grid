package stream

import (
	"encoding/json"
	"strings"

	"github.com/takezoh/agent-reactor/client/state"
)

type subsystemEmission struct {
	kind    state.SubsystemEventKind
	payload state.SubsystemPayload
}

type codexThreadMetadata struct {
	threadID  string
	title     string
	titleSet  bool
	preview   string
	prompt    string
	model     string
	modelSet  bool
	effort    string
	effortSet bool
}

func normalizeCodexThreadMetadata(raw json.RawMessage) codexThreadMetadata {
	var data map[string]any
	if json.Unmarshal(raw, &data) != nil {
		return codexThreadMetadata{}
	}
	thread, _ := data["thread"].(map[string]any)
	title, titleSet := metadataTitle(data, thread)
	meta := codexThreadMetadata{
		threadID: firstNonEmpty(stringValue(data["threadId"]), stringValue(thread["id"])),
		title:    title,
		titleSet: titleSet,
		preview:  firstNonEmpty(stringValue(thread["preview"]), stringValue(data["preview"])),
		prompt:   turnPromptFromData(data),
	}
	meta.title = collapseMetadataText(meta.title)
	meta.preview = collapseMetadataText(meta.preview)
	meta.prompt = collapseMetadataText(meta.prompt)
	return meta
}

func normalizeCodexThreadSettings(raw json.RawMessage) codexThreadMetadata {
	var data struct {
		ThreadID       string                     `json:"threadId"`
		ThreadSettings map[string]json.RawMessage `json:"threadSettings"`
	}
	if json.Unmarshal(raw, &data) != nil || data.ThreadSettings == nil {
		return codexThreadMetadata{}
	}
	model, modelSet := normalizeMetadataStringField(data.ThreadSettings["model"])
	effort, effortSet := normalizeMetadataEffortAliasFields(
		data.ThreadSettings["effort"],
		data.ThreadSettings["reasoning_effort"],
	)
	return codexThreadMetadata{
		threadID:  strings.TrimSpace(data.ThreadID),
		model:     model,
		modelSet:  modelSet,
		effort:    effort,
		effortSet: effortSet,
	}
}

func stringValue(v any) string {
	s, _ := v.(string)
	return s
}

func metadataTitle(data, thread map[string]any) (string, bool) {
	for _, source := range []struct {
		data map[string]any
		key  string
	}{
		{data: thread, key: "name"},
		{data: data, key: "threadName"},
		{data: data, key: "name"},
	} {
		if source.data == nil {
			continue
		}
		v, ok := source.data[source.key]
		if !ok {
			continue
		}
		return stringValue(v), true
	}
	return "", false
}

func collapseMetadataText(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

func normalizeMetadataStringField(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	if strings.TrimSpace(string(raw)) == "null" {
		return "", true
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return "", false
	}
	return strings.TrimSpace(value), true
}

func normalizeMetadataEffortAliasFields(primary, alias json.RawMessage) (string, bool) {
	effort, effortSet := normalizeMetadataEffortField(primary)
	if effortSet && strings.TrimSpace(effort) != "" {
		return effort, true
	}
	aliasEffort, aliasSet := normalizeMetadataEffortField(alias)
	if aliasSet && strings.TrimSpace(aliasEffort) != "" {
		return aliasEffort, true
	}
	if effortSet {
		return effort, true
	}
	return aliasEffort, aliasSet
}

func normalizeMetadataEffortField(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	if strings.TrimSpace(string(raw)) == "null" {
		return "", true
	}
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return "", false
	}
	switch effort := value.(type) {
	case string:
		return strings.TrimSpace(effort), true
	case map[string]any:
		if level, _ := effort["level"].(string); strings.TrimSpace(level) != "" {
			return strings.TrimSpace(level), true
		}
		return "", true
	default:
		return "", true
	}
}

func extractThreadID(raw json.RawMessage) string {
	var data map[string]any
	if json.Unmarshal(raw, &data) != nil {
		return ""
	}
	if s, _ := data["threadId"].(string); s != "" {
		return s
	}
	if thread, ok := data["thread"].(map[string]any); ok {
		if s, _ := thread["id"].(string); s != "" {
			return s
		}
	}
	return ""
}

func extractThreadPath(raw json.RawMessage) string {
	var data map[string]any
	if json.Unmarshal(raw, &data) != nil {
		return ""
	}
	if s, _ := data["path"].(string); s != "" {
		return s
	}
	if thread, ok := data["thread"].(map[string]any); ok {
		if s, _ := thread["path"].(string); s != "" {
			return s
		}
	}
	return ""
}

func extractThreadSessionID(raw json.RawMessage) string {
	var data map[string]any
	if json.Unmarshal(raw, &data) != nil {
		return ""
	}
	if s, _ := data["sessionId"].(string); s != "" {
		return s
	}
	if thread, ok := data["thread"].(map[string]any); ok {
		if s, _ := thread["sessionId"].(string); s != "" {
			return s
		}
	}
	return ""
}

func extractTurnID(raw json.RawMessage) string {
	var data map[string]any
	if json.Unmarshal(raw, &data) != nil {
		return ""
	}
	if s, _ := data["turnId"].(string); s != "" {
		return s
	}
	if turn, ok := data["turn"].(map[string]any); ok {
		if s, _ := turn["id"].(string); s != "" {
			return s
		}
	}
	return ""
}

func extractTurnPrompt(raw json.RawMessage) string {
	var data map[string]any
	if json.Unmarshal(raw, &data) != nil {
		return ""
	}
	return turnPromptFromData(data)
}

func turnPromptFromData(data map[string]any) string {
	turn, _ := data["turn"].(map[string]any)
	if turn == nil {
		return ""
	}
	items, _ := turn["items"].([]any)
	for _, itemRaw := range items {
		item, _ := itemRaw.(map[string]any)
		if item == nil || item["type"] != "userMessage" {
			continue
		}
		if text := userMessageText(item); text != "" {
			return text
		}
	}
	return ""
}

func userMessageText(item map[string]any) string {
	if text, _ := item["content"].(string); strings.TrimSpace(text) != "" {
		return strings.TrimSpace(text)
	}
	content, _ := item["content"].([]any)
	var parts []string
	for _, contentRaw := range content {
		if text, _ := contentRaw.(string); strings.TrimSpace(text) != "" {
			parts = append(parts, strings.TrimSpace(text))
			continue
		}
		c, _ := contentRaw.(map[string]any)
		if c == nil || c["type"] != "text" {
			continue
		}
		if text, _ := c["text"].(string); strings.TrimSpace(text) != "" {
			parts = append(parts, strings.TrimSpace(text))
		}
	}
	return strings.Join(parts, "\n")
}

func extractText(raw json.RawMessage) string {
	var data map[string]any
	if json.Unmarshal(raw, &data) != nil {
		return ""
	}
	for _, key := range []string{"text", "delta"} {
		if s, _ := data[key].(string); s != "" {
			return s
		}
	}
	if item, ok := data["item"].(map[string]any); ok {
		for _, key := range []string{"text", "content"} {
			if s, _ := item[key].(string); s != "" {
				return s
			}
		}
	}
	return ""
}

func nestedString(raw json.RawMessage, key string) string {
	var data map[string]any
	if json.Unmarshal(raw, &data) != nil {
		return ""
	}
	if s, _ := data[key].(string); s != "" {
		return s
	}
	if item, ok := data["item"].(map[string]any); ok {
		if s, _ := item[key].(string); s != "" {
			return s
		}
	}
	return ""
}

func extractThreadStatus(raw json.RawMessage) (string, bool, string) {
	var data map[string]any
	if json.Unmarshal(raw, &data) != nil {
		return "", false, ""
	}
	threadID, _ := data["threadId"].(string)
	status, _ := data["status"].(map[string]any)
	if thread, ok := data["thread"].(map[string]any); ok {
		if threadID == "" {
			threadID, _ = thread["id"].(string)
		}
		if status == nil {
			status, _ = thread["status"].(map[string]any)
		}
	}
	if status == nil {
		return "", false, threadID
	}
	statusType, _ := status["type"].(string)
	flags, _ := status["activeFlags"].([]any)
	waitingApproval := false
	for _, flag := range flags {
		s, _ := flag.(string)
		if s == "waitingOnApproval" {
			waitingApproval = true
			break
		}
	}
	return statusType, waitingApproval, threadID
}

func threadStatusEvents(raw json.RawMessage, currentThreadID, prevStatus string, prevWaitingApproval bool) ([]subsystemEmission, string, bool) {
	statusType, waitingApproval, threadID := extractThreadStatus(raw)
	if threadID == "" {
		threadID = currentThreadID
	}
	if currentThreadID != "" && threadID != "" && threadID != currentThreadID {
		return nil, prevStatus, prevWaitingApproval
	}
	if statusType == "" {
		return nil, prevStatus, prevWaitingApproval
	}
	var out []subsystemEmission
	switch statusType {
	case "active":
		if prevStatus != "active" {
			out = append(out, subsystemEmission{kind: state.SubsystemTurnStarted, payload: state.SubsystemPayload{SessionID: threadID, TargetID: threadID}})
		}
		if waitingApproval && !prevWaitingApproval {
			out = append(out, subsystemEmission{kind: state.SubsystemApprovalRequested, payload: state.SubsystemPayload{
				SessionID: threadID,
				TargetID:  threadID,
				Approval:  &state.SubsystemApproval{Kind: "command"},
			}})
		}
		if !waitingApproval && prevWaitingApproval {
			out = append(out, subsystemEmission{kind: state.SubsystemApprovalResolved, payload: state.SubsystemPayload{
				SessionID: threadID,
				TargetID:  threadID,
				Approval:  &state.SubsystemApproval{Kind: "command", Resolved: true},
			}})
		}
	case "idle":
		if prevWaitingApproval {
			out = append(out, subsystemEmission{kind: state.SubsystemApprovalResolved, payload: state.SubsystemPayload{
				SessionID: threadID,
				TargetID:  threadID,
				Approval:  &state.SubsystemApproval{Kind: "command", Resolved: true},
			}})
		}
		if prevStatus != "idle" {
			out = append(out, subsystemEmission{kind: state.SubsystemTurnCompleted, payload: state.SubsystemPayload{SessionID: threadID, TargetID: threadID}})
		}
	}
	return out, statusType, waitingApproval
}

func itemType(raw json.RawMessage) string { return nestedString(raw, "type") }

func itemLifecycleEvents(method string, raw json.RawMessage, currentThreadID string) []subsystemEmission {
	threadID := firstNonEmpty(extractThreadID(raw), currentThreadID)
	switch method {
	case "item/started":
		switch itemType(raw) {
		case "commandExecution":
			tool := commandTool(raw)
			return []subsystemEmission{{kind: state.SubsystemToolStarted, payload: state.SubsystemPayload{SessionID: threadID, TargetID: threadID, Tool: &tool}}}
		case "fileChange":
			tool := fileChangeTool(raw)
			return []subsystemEmission{{kind: state.SubsystemToolStarted, payload: state.SubsystemPayload{SessionID: threadID, TargetID: threadID, Tool: &tool}}}
		}
	case "item/completed":
		switch itemType(raw) {
		case "commandExecution":
			tool := commandTool(raw)
			tool.Error = nestedString(raw, "error")
			return []subsystemEmission{{kind: state.SubsystemToolCompleted, payload: state.SubsystemPayload{SessionID: threadID, TargetID: threadID, Tool: &tool}}}
		case "fileChange":
			tool := fileChangeTool(raw)
			return []subsystemEmission{{kind: state.SubsystemToolCompleted, payload: state.SubsystemPayload{SessionID: threadID, TargetID: threadID, Tool: &tool}}}
		}
	}
	return nil
}

func commandTool(raw json.RawMessage) state.SubsystemTool {
	return state.SubsystemTool{
		ID:      nestedString(raw, "itemId"),
		Name:    "command",
		Command: nestedString(raw, "command"),
		Path:    nestedString(raw, "cwd"),
	}
}

func fileChangeTool(raw json.RawMessage) state.SubsystemTool {
	return state.SubsystemTool{
		ID:   nestedString(raw, "itemId"),
		Name: "file_change",
		Path: nestedString(raw, "path"),
	}
}

func summarizePlan(raw json.RawMessage) string {
	var data map[string]any
	if json.Unmarshal(raw, &data) != nil {
		return ""
	}
	if s, _ := data["summary"].(string); s != "" {
		return s
	}
	if items, ok := data["items"].([]any); ok {
		parts := make([]string, 0, len(items))
		for _, item := range items {
			m, _ := item.(map[string]any)
			step, _ := m["step"].(string)
			status, _ := m["status"].(string)
			if step != "" {
				parts = append(parts, strings.TrimSpace(step+" "+status))
			}
		}
		return strings.Join(parts, " | ")
	}
	return ""
}

func summarizeDiff(raw json.RawMessage) string {
	paths := diffPaths(raw)
	if len(paths) == 0 {
		return ""
	}
	return strings.Join(paths, ", ")
}

func diffPaths(raw json.RawMessage) []string {
	var data map[string]any
	if json.Unmarshal(raw, &data) != nil {
		return nil
	}
	list, ok := data["paths"].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		if s, _ := item.(string); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func approvalFromParams(method string, raw json.RawMessage, auto bool) state.SubsystemApproval {
	kind := "command"
	if strings.Contains(method, "fileChange") {
		kind = "file_change"
	}
	return state.SubsystemApproval{
		ID:          nestedString(raw, "itemId"),
		Kind:        kind,
		Command:     nestedString(raw, "command"),
		Path:        nestedString(raw, "path"),
		Reason:      nestedString(raw, "reason"),
		AutoApprove: auto,
	}
}

func appendHistory(history *[]state.SubsystemTurn, role, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	*history = append(*history, state.SubsystemTurn{Role: role, Text: text})
	if len(*history) > 6 {
		*history = (*history)[len(*history)-6:]
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
