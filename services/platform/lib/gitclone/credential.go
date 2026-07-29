// Package gitclone: canonical github.com HTTPS remotes, peer/nonce-bound
// one-use askpass broker, sanitized git process (no credential surfaces).
package gitclone

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const (
	EnvAskpassSock                                               = "AGENT_GRID_CLONE_ASKPASS_SOCK"
	EnvAskpassMode                                               = "AGENT_GRID_CLONE_ASKPASS_MODE"
	AskpassNonceFD                                               = 3
	CodeOK, CodeInvalidIdentity, CodeAuthorityUnavailable        = "ok", "invalid_identity", "authority_unavailable"
	CodeCloneFailed, CodeCancelled, CodePeerMismatch, CodeReplay = "clone_failed", "cancelled", "peer_mismatch", "replay"
	CodeCredentialSurfaceLeak                                    = "credential_surface_violation"
)

var ownerRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,38}$`)
var nameRE = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}$`)

type Credential struct{ Username, Token string }

func CanonicalHTTPSRemote(owner, name string) (string, error) {
	if !ownerRE.MatchString(owner) || strings.HasSuffix(owner, "-") || !nameRE.MatchString(name) || strings.Contains(name, "..") {
		return "", errors.New(CodeInvalidIdentity)
	}
	return fmt.Sprintf("https://github.com/%s/%s.git", owner, name), nil
}
func SanitizeDiagnostic(raw string, secrets ...string) string {
	out := raw
	for _, s := range secrets {
		if s != "" {
			out = strings.ReplaceAll(out, s, "***")
		}
	}
	if len(out) > 2048 {
		out = out[:2048] + "…(truncated)"
	}
	return out
}
func ContainsSentinel(surfaces []string, sentinel string) bool {
	if sentinel == "" {
		return false
	}
	for _, s := range surfaces {
		if strings.Contains(s, sentinel) {
			return true
		}
	}
	return false
}
