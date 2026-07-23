package driver

import (
	"path/filepath"
	"strings"
)

const toolLogSchemaVersion = 2

type fileEventKind string

const (
	fileEventRead         fileEventKind = "read"
	fileEventCreate       fileEventKind = "create"
	fileEventEdit         fileEventKind = "edit"
	fileEventDelete       fileEventKind = "delete"
	fileEventUnclassified fileEventKind = "unclassified"
)

func workspaceRelativePath(workspaceRoot, absPath string) string {
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

func classifyClaudeTool(toolName string, toolInput map[string]any, workspaceRoot string) (fileEventKind, string) {
	switch toolName {
	case "Read":
		if p := toolInputString(toolInput, "file_path"); p != "" {
			return fileEventRead, workspaceRelativePath(workspaceRoot, p)
		}
	case "Write":
		if p := toolInputString(toolInput, "file_path"); p != "" {
			return fileEventCreate, workspaceRelativePath(workspaceRoot, p)
		}
	case "Edit":
		if p := toolInputString(toolInput, "file_path"); p != "" {
			return fileEventEdit, workspaceRelativePath(workspaceRoot, p)
		}
	case "Bash":
		return fileEventUnclassified, ""
	}
	return fileEventUnclassified, ""
}

func classifyCodexTool(toolName string, toolInput map[string]any, workspaceRoot string) (fileEventKind, string) {
	switch toolName {
	case "read", "read_file", "Read":
		if p := firstNonEmpty(
			toolInputString(toolInput, "file_path"),
			toolInputString(toolInput, "path"),
		); p != "" {
			return fileEventRead, workspaceRelativePath(workspaceRoot, p)
		}
	case "apply_patch", "file_change":
		return classifyCodexPatch(toolInput, workspaceRoot)
	}
	return fileEventUnclassified, ""
}

func classifyCodexPatch(toolInput map[string]any, workspaceRoot string) (fileEventKind, string) {
	changes, ok := toolInput["changes"].([]any)
	if !ok || len(changes) == 0 {
		if p := firstNonEmpty(
			toolInputString(toolInput, "file_path"),
			toolInputString(toolInput, "path"),
		); p != "" {
			return fileEventEdit, workspaceRelativePath(workspaceRoot, p)
		}
		return fileEventUnclassified, ""
	}
	change, ok := changes[0].(map[string]any)
	if !ok {
		return fileEventUnclassified, ""
	}
	path := toolInputString(change, "path")
	if path == "" {
		return fileEventUnclassified, ""
	}
	kind := codexPatchChangeKind(change["kind"])
	if kind == fileEventUnclassified {
		return fileEventUnclassified, ""
	}
	return kind, workspaceRelativePath(workspaceRoot, path)
}

func codexPatchChangeKind(v any) fileEventKind {
	m, ok := v.(map[string]any)
	if !ok {
		return fileEventUnclassified
	}
	switch toolInputString(m, "type") {
	case "add":
		return fileEventCreate
	case "update":
		return fileEventEdit
	case "delete":
		return fileEventDelete
	default:
		return fileEventUnclassified
	}
}
