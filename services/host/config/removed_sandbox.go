package config

import (
	"log/slog"
	"strings"

	"github.com/BurntSushi/toml"
)

// warnRemovedSandboxTable logs once per settings file that still carries a
// [sandbox] table. Agents always launch directly on the host now, so the whole
// table (mode, isolation, devcontainer, proxy) is inert.
//
// This is deliberately a warning and not an error: the daemon must never refuse
// to start because a user's settings.toml carries a key we stopped reading, and
// we must not rewrite the user's file for them. Undecoded() surfaces the key
// because the struct no longer declares it.
func warnRemovedSandboxTable(metadata toml.MetaData, path string) {
	for _, key := range metadata.Undecoded() {
		if key.String() == "sandbox" || strings.HasPrefix(key.String(), "sandbox.") {
			slog.Warn("config: [sandbox] has been removed; agents always launch on the host. "+
				"The table is ignored — delete it to silence this warning.",
				"path", path)
			return
		}
	}
}
