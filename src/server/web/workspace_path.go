package web

import (
	"errors"
	"path/filepath"
	"strings"
)

// ErrWorkspacePathNotFound is returned when a workspace-relative path is
// rejected or resolves outside the workspace root. Callers map it to HTTP 404.
var ErrWorkspacePathNotFound = errors.New("workspace path not found")

// GuardWorkspacePath resolves rel under root using per-segment joining,
// EvalSymlinks on the full path, and a descendant check against the
// EvalSymlinks-normalized root. Rejects absolute paths and ".." segments.
func GuardWorkspacePath(root, rel string) (string, error) {
	root = filepath.Clean(root)
	if root == "" {
		return "", ErrWorkspacePathNotFound
	}
	if filepath.IsAbs(rel) {
		return "", ErrWorkspacePathNotFound
	}
	rel = filepath.Clean(rel)
	if rel == "." {
		rel = ""
	}
	if strings.Contains(rel, "..") {
		return "", ErrWorkspacePathNotFound
	}

	current := root
	if rel != "" {
		for _, seg := range strings.Split(rel, string(filepath.Separator)) {
			if seg == "" || seg == ".." {
				return "", ErrWorkspacePathNotFound
			}
			current = filepath.Join(current, seg)
		}
	}

	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", ErrWorkspacePathNotFound
	}
	resolved, err := filepath.EvalSymlinks(current)
	if err != nil {
		return "", ErrWorkspacePathNotFound
	}
	if !workspaceDescendant(resolvedRoot, resolved) {
		return "", ErrWorkspacePathNotFound
	}
	return resolved, nil
}

func workspaceDescendant(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
