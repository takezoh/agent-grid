package harnesspolicy

import (
	"encoding/json"
	"fmt"
	"os"
)

type ProfileRegistry struct {
	Version  int                   `json:"version"`
	Profiles []VerificationProfile `json:"profiles"`
}

type VerificationProfile struct {
	Name     string   `json:"name"`
	Tier     string   `json:"tier"`
	Commands []string `json:"commands"`
}

func LoadProfiles(path string) (ProfileRegistry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ProfileRegistry{}, err
	}
	var registry ProfileRegistry
	if err := json.Unmarshal(raw, &registry); err != nil {
		return ProfileRegistry{}, err
	}
	if registry.Version != 1 {
		return ProfileRegistry{}, fmt.Errorf("unsupported profile version %d", registry.Version)
	}
	seen := map[string]bool{}
	for _, profile := range registry.Profiles {
		if profile.Name == "" || profile.Tier == "" || len(profile.Commands) == 0 {
			return ProfileRegistry{}, fmt.Errorf("profile name, tier, and commands are required")
		}
		if seen[profile.Name] {
			return ProfileRegistry{}, fmt.Errorf("duplicate profile %q", profile.Name)
		}
		seen[profile.Name] = true
	}
	for _, required := range []string{"save", "pre-push", "pr", "nightly"} {
		if !seen[required] {
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
