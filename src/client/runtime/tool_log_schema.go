package runtime

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"time"
)

const ToolLogSchemaVersion = 2

// FileEventKind is the normalized workspace file-event classification.
type FileEventKind string

const (
	FileEventRead         FileEventKind = "read"
	FileEventCreate       FileEventKind = "create"
	FileEventEdit         FileEventKind = "edit"
	FileEventDelete       FileEventKind = "delete"
	FileEventUnclassified FileEventKind = "unclassified"
)

// ToolLogRecord is the schema_version=2 JSONL record written by tool-log
// writers and consumed by ToolLogReader.
type ToolLogRecord struct {
	SchemaVersion         int            `json:"schema_version"`
	TS                    time.Time      `json:"ts"`
	RoostSessionID        string         `json:"roost_session_id,omitempty"`
	ClaudeSessionID       string         `json:"claude_session_id,omitempty"`
	ToolUseID             string         `json:"tool_use_id,omitempty"`
	ToolName              string         `json:"tool_name,omitempty"`
	Kind                  string         `json:"kind,omitempty"`
	PermissionMode        string         `json:"permission_mode,omitempty"`
	DurationMs            int64          `json:"duration_ms,omitempty"`
	ToolInput             map[string]any `json:"tool_input,omitempty"`
	Error                 string         `json:"error,omitempty"`
	TurnID                string         `json:"turn_id,omitempty"`
	FileEventKind         FileEventKind  `json:"file_event_kind,omitempty"`
	WorkspaceRelativePath string         `json:"workspace_relative_path,omitempty"`
	TurnComplete          bool           `json:"turn_complete,omitempty"`
	TurnFailure           bool           `json:"turn_failure,omitempty"`
}

// ParseToolLogRecord unmarshals one JSONL line. Returns ok=false for empty
// or invalid JSON without treating it as legacy.
func ParseToolLogRecord(line string) (ToolLogRecord, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return ToolLogRecord{}, false
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return ToolLogRecord{}, false
	}
	var rec ToolLogRecord
	if err := json.Unmarshal([]byte(line), &rec); err != nil {
		return ToolLogRecord{}, false
	}
	if v, ok := raw["schema_version"]; ok {
		switch n := v.(type) {
		case float64:
			rec.SchemaVersion = int(n)
		case int:
			rec.SchemaVersion = n
		}
	}
	return rec, true
}

// IsLegacy returns true when schema_version is missing or < 2.
func (r ToolLogRecord) IsLegacy() bool {
	return r.SchemaVersion < ToolLogSchemaVersion
}

// IsClassified returns true when the entry carries a structured
// workspace-relative path suitable for activity emission.
func (r ToolLogRecord) IsClassified() bool {
	if r.TurnComplete {
		return false
	}
	if r.FileEventKind == "" || r.FileEventKind == FileEventUnclassified {
		return false
	}
	return strings.TrimSpace(r.WorkspaceRelativePath) != ""
}

// WorkspaceRelativePath returns path relative to workspaceRoot, or "" when
// absPath is empty, outside the root, or cannot be relativized.
func WorkspaceRelativePath(workspaceRoot, absPath string) string {
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	absPath = strings.TrimSpace(absPath)
	if workspaceRoot == "" || absPath == "" {
		return ""
	}
	root := filepath.Clean(workspaceRoot)
	path := filepath.Clean(absPath)
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
		path = filepath.Clean(path)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return ""
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ""
	}
	return filepath.ToSlash(rel)
}

// ClassifyClaudeTool maps Claude PostToolUse payloads to normalized kind
// and workspace-relative path. Returns unclassified when no structured path
// is available (e.g. Bash rm without file_path).
func ClassifyClaudeTool(toolName string, toolInput map[string]any, workspaceRoot string) (FileEventKind, string) {
	switch toolName {
	case "Read":
		if p := toolInputPath(toolInput); p != "" {
			return FileEventRead, WorkspaceRelativePath(workspaceRoot, p)
		}
	case "Write":
		if p := toolInputPath(toolInput); p != "" {
			return FileEventCreate, WorkspaceRelativePath(workspaceRoot, p)
		}
	case "Edit":
		if p := toolInputPath(toolInput); p != "" {
			return FileEventEdit, WorkspaceRelativePath(workspaceRoot, p)
		}
	case "Bash":
		return FileEventUnclassified, ""
	}
	return FileEventUnclassified, ""
}

// ClassifyCodexTool maps Codex apply_patch/file_change and read tool shapes.
func ClassifyCodexTool(toolName string, toolInput map[string]any, workspaceRoot string) (FileEventKind, string) {
	switch toolName {
	case "read", "read_file", "Read":
		if p := firstNonEmptyStr(
			toolInputPath(toolInput),
			stringField(toolInput, "path"),
		); p != "" {
			return FileEventRead, WorkspaceRelativePath(workspaceRoot, p)
		}
	case "apply_patch", "file_change":
		return classifyPatchChanges(toolInput, workspaceRoot)
	}
	return FileEventUnclassified, ""
}

func classifyPatchChanges(toolInput map[string]any, workspaceRoot string) (FileEventKind, string) {
	changes, ok := toolInput["changes"].([]any)
	if !ok || len(changes) == 0 {
		if p := firstNonEmptyStr(toolInputPath(toolInput), stringField(toolInput, "path")); p != "" {
			return FileEventEdit, WorkspaceRelativePath(workspaceRoot, p)
		}
		return FileEventUnclassified, ""
	}
	change, ok := changes[0].(map[string]any)
	if !ok {
		return FileEventUnclassified, ""
	}
	path := stringField(change, "path")
	if path == "" {
		return FileEventUnclassified, ""
	}
	kind := patchChangeKind(change["kind"])
	if kind == FileEventUnclassified {
		return FileEventUnclassified, ""
	}
	return kind, WorkspaceRelativePath(workspaceRoot, path)
}

func patchChangeKind(v any) FileEventKind {
	m, ok := v.(map[string]any)
	if !ok {
		return FileEventUnclassified
	}
	switch stringField(m, "type") {
	case "add":
		return FileEventCreate
	case "update":
		return FileEventEdit
	case "delete":
		return FileEventDelete
	default:
		return FileEventUnclassified
	}
}

func toolInputPath(in map[string]any) string {
	return stringField(in, "file_path")
}

func stringField(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func firstNonEmptyStr(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
