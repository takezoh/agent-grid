package harnesspolicy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type ProfileRegistry struct {
	Version  int                   `json:"version"`
	Profiles []VerificationProfile `json:"profiles"`
}

type VerificationProfile struct {
	Name     string                `json:"name"`
	Tier     string                `json:"tier"`
	Commands []VerificationCommand `json:"commands"`
}

type VerificationCommand struct {
	ID      string `json:"id"`
	Group   string `json:"group"`
	When    string `json:"when,omitempty"`
	Command string `json:"command"`
}

func LoadProfiles(path string) (ProfileRegistry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ProfileRegistry{}, err
	}
	var registry ProfileRegistry
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&registry); err != nil {
		return ProfileRegistry{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return ProfileRegistry{}, fmt.Errorf("trailing JSON value")
	}
	if registry.Version != 2 {
		return ProfileRegistry{}, fmt.Errorf("unsupported profile version %d", registry.Version)
	}
	seenProfiles := map[string]bool{}
	for _, profile := range registry.Profiles {
		if profile.Name == "" || profile.Tier == "" || len(profile.Commands) == 0 {
			return ProfileRegistry{}, fmt.Errorf("profile name, tier, and commands are required")
		}
		if seenProfiles[profile.Name] {
			return ProfileRegistry{}, fmt.Errorf("duplicate profile %q", profile.Name)
		}
		seenProfiles[profile.Name] = true
		seenCommands := map[string]bool{}
		for _, command := range profile.Commands {
			if command.ID == "" || command.Group == "" || command.Command == "" {
				return ProfileRegistry{}, fmt.Errorf("profile %s command id, group, and command are required", profile.Name)
			}
			if seenCommands[command.ID] {
				return ProfileRegistry{}, fmt.Errorf("profile %s duplicate command %q", profile.Name, command.ID)
			}
			if command.When != "" && command.When != "pull-request" {
				return ProfileRegistry{}, fmt.Errorf("profile %s command %s has unsupported when %q", profile.Name, command.ID, command.When)
			}
			seenCommands[command.ID] = true
		}
	}
	for _, required := range []string{"save", "pre-push", "pr", "nightly"} {
		if !seenProfiles[required] {
			return ProfileRegistry{}, fmt.Errorf("missing profile %q", required)
		}
	}
	return registry, nil
}

func (r ProfileRegistry) Find(name string) (VerificationProfile, bool) {
	for _, profile := range r.Profiles {
		if profile.Name == name {
			return profile, true
		}
	}
	return VerificationProfile{}, false
}
