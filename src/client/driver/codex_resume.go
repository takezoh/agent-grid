package driver

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/takezoh/agent-reactor/client/state"
	_ "modernc.org/sqlite"
)

const codexStateDBName = "state_5.sqlite"

func (cs *CodexState) setRolloutPath(path string) {
	path = strings.TrimSpace(path)
	cs.RolloutPath = path
	cs.TranscriptPath = path
}

func (cs CodexState) resolvedRolloutPath() string {
	if rolloutPath := strings.TrimSpace(cs.RolloutPath); rolloutPath != "" {
		return rolloutPath
	}
	return strings.TrimSpace(cs.TranscriptPath)
}

func logCodexResumeSkip(project, threadID, rolloutPath, reason string) {
	slog.Debug("codex: coldstart without resume",
		"project", project,
		"thread", threadID,
		"rollout_path", rolloutPath,
		"reason", reason)
}

func (cs CodexState) coldStartResumePlan() (state.ResumeTarget, string, bool, error) {
	threadID := strings.TrimSpace(cs.ThreadID)
	rolloutPath := usableRolloutPath(cs.resolvedRolloutPath())
	sessionID := strings.TrimSpace(cs.SessionID)
	if threadID == "" && rolloutPath == "" && sessionID == "" {
		return state.ResumeTarget{}, "", false, nil
	}
	if !isAlphanumHyphen(threadID) {
		return state.ResumeTarget{}, "", false, fmt.Errorf("codex cold-start resume requires a valid thread_id, got %q", threadID)
	}
	return state.ResumeTarget{ThreadID: threadID, RolloutPath: rolloutPath}, resolveCodexSessionID(rolloutPath, sessionID), true, nil
}

func resolveCodexSessionID(rolloutPath, persistedSessionID string) string {
	if rolloutPath == "" || persistedSessionID != "" {
		return persistedSessionID
	}
	codexHome, err := codexHomeDir()
	if err != nil {
		slog.Debug("codex: session id lookup skipped",
			"rollout_path", rolloutPath, "err", err)
		return ""
	}
	sessionID, err := lookupCodexThreadByRollout(codexHome, rolloutPath)
	if err != nil {
		slog.Debug("codex: session id lookup skipped",
			"rollout_path", rolloutPath, "codex_home", codexHome, "err", err)
		return ""
	}
	return sessionID
}

func usableRolloutPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		return ""
	}
	return path
}

func codexHomeDir() (string, error) {
	if home := strings.TrimSpace(os.Getenv("CODEX_HOME")); home != "" {
		return home, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("codex cold-start resume could not determine CODEX_HOME: %w", err)
	}
	return filepath.Join(home, ".codex"), nil
}

func lookupCodexThreadByRollout(codexHome, rolloutPath string) (string, error) {
	dbPath := filepath.Join(codexHome, codexStateDBName)
	if _, err := os.Stat(dbPath); err != nil {
		return "", fmt.Errorf("codex cold-start resume local session source missing %s: %w", dbPath, err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return "", fmt.Errorf("codex cold-start resume failed to open local session source %s: %w", dbPath, err)
	}
	defer func() { _ = db.Close() }()
	rows, err := db.Query("SELECT id FROM threads WHERE rollout_path = ? LIMIT 2", rolloutPath)
	if err != nil {
		return "", fmt.Errorf("codex cold-start resume failed to query local session source %s: %w", dbPath, err)
	}
	defer func() { _ = rows.Close() }()
	var matches []string
	for rows.Next() {
		var sessionID string
		if err := rows.Scan(&sessionID); err != nil {
			return "", fmt.Errorf("codex cold-start resume got malformed sqlite row for rollout_path %s: %w", rolloutPath, err)
		}
		matches = append(matches, strings.TrimSpace(sessionID))
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("codex cold-start resume failed to read local session source %s: %w", dbPath, err)
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("codex cold-start resume local session source has no thread row for rollout_path %s", rolloutPath)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("codex cold-start resume local session source returned multiple rows for rollout_path %s", rolloutPath)
	}
	if matches[0] == "" {
		return "", fmt.Errorf("codex cold-start resume local session source returned empty session_id for rollout_path %s", rolloutPath)
	}
	return matches[0], nil
}
