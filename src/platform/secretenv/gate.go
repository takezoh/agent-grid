package secretenv

import (
	"fmt"
	"path/filepath"
)

// Gate enforces an allowlist of env-file path patterns.
// Uses filepath.Match glob syntax. Default-deny when Allow is empty.
//
// Pattern notes:
//   - '*' matches any sequence of non-separator characters within ONE path
//     segment. "/workspace/*.env" matches "/workspace/dev.env" but NOT
//     "/workspace/sub/dev.env".
//   - '**' is NOT treated as a recursive wildcard; it is equivalent to two
//     adjacent '*' within the same segment. To allow an entire directory tree,
//     list each level explicitly or use the path prefix pattern "/dir/*".
//   - Input paths are NOT cleaned before matching; callers are responsible for
//     passing canonical (filepath.Clean) paths so patterns are predictable.
type Gate struct {
	allow []string
}

// NewGate builds a Gate from a list of filepath.Match glob patterns.
func NewGate(allow []string) *Gate {
	patterns := make([]string, len(allow))
	copy(patterns, allow)
	return &Gate{allow: patterns}
}

// Check returns nil if path matches at least one allow pattern, or an error.
func (g *Gate) Check(path string) error {
	for _, pat := range g.allow {
		ok, err := filepath.Match(pat, path)
		if err != nil {
			return fmt.Errorf("secretenv gate: invalid pattern %q: %w", pat, err)
		}
		if ok {
			return nil
		}
	}
	return fmt.Errorf("secretenv gate: %q is not in the allowlist", path)
}
