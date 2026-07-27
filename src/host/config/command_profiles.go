package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	ParameterEnum    = "enum"
	ParameterBoolean = "boolean"
	ParameterString  = "string"
	ParameterNumber  = "number"
)

// CommandProfileConfig declares one safe, parameterized launch command.
// Argv entries are complete argv tokens. "{{name}}" references inside a
// token are replaced by validated parameter values before the complete
// result is shell-quoted at the API boundary.
type CommandProfileConfig struct {
	ID         string                   `toml:"id" json:"id"`
	Label      string                   `toml:"label" json:"label"`
	Disabled   bool                     `toml:"disabled" json:"-"`
	Executable string                   `toml:"executable" json:"executable"`
	Argv       []string                 `toml:"argv" json:"argv"`
	Parameters []CommandParameterConfig `toml:"parameters" json:"parameters"`
}

type CommandParameterConfig struct {
	Name     string   `toml:"name" json:"name"`
	Label    string   `toml:"label" json:"label"`
	Type     string   `toml:"type" json:"type"`
	Required bool     `toml:"required" json:"required"`
	Default  any      `toml:"default" json:"default,omitempty"`
	Values   []string `toml:"values" json:"values,omitempty"`
	Pattern  string   `toml:"pattern" json:"pattern,omitempty"`
	Min      *float64 `toml:"min" json:"min,omitempty"`
	Max      *float64 `toml:"max" json:"max,omitempty"`
}

type ResolvedCommandProfile struct {
	CommandProfileConfig
	Revision string `json:"revision"`
}

var (
	parameterNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)
	argvReferencePattern = regexp.MustCompile(`\{\{\s*([A-Za-z][A-Za-z0-9_-]*)\s*\}\}`)
)

func rejectRemovedCommandProfileFields(metadata toml.MetaData) error {
	for _, key := range metadata.Undecoded() {
		if strings.HasSuffix(key.String(), "command_profiles.projects") {
			return fmt.Errorf(
				"%s is unsupported; use <project>/.agent-grid/settings.toml", key,
			)
		}
	}
	return nil
}

func (s SessionConfig) ValidateCommandProfiles() error {
	return s.validateCommandProfiles(false)
}

func (s SessionConfig) validateCommandProfiles(allowDisabled bool) error {
	seen := make(map[string]struct{}, len(s.CommandProfiles))
	for i := range s.CommandProfiles {
		p := &s.CommandProfiles[i]
		if p.ID == "" {
			return fmt.Errorf("profile %d requires id", i)
		}
		if _, ok := seen[p.ID]; ok {
			return fmt.Errorf("duplicate profile id %q", p.ID)
		}
		seen[p.ID] = struct{}{}
		if p.Disabled {
			if !allowDisabled {
				return fmt.Errorf("profile %q: disabled is only valid in project settings", p.ID)
			}
			if p.Executable != "" || len(p.Argv) != 0 || len(p.Parameters) != 0 {
				return fmt.Errorf("profile %q: disabled profile must contain only id and disabled", p.ID)
			}
			continue
		}
		if p.Executable == "" {
			return fmt.Errorf("profile %q requires executable", p.ID)
		}
		if p.Label == "" {
			p.Label = p.ID
		}
		if err := validateParameters(p.Parameters); err != nil {
			return fmt.Errorf("profile %q: %w", p.ID, err)
		}
		if err := validateArgv(*p); err != nil {
			return fmt.Errorf("profile %q: %w", p.ID, err)
		}
	}
	return nil
}

func validateArgv(profile CommandProfileConfig) error {
	parameters := make(map[string]CommandParameterConfig, len(profile.Parameters))
	for _, parameter := range profile.Parameters {
		parameters[parameter.Name] = parameter
	}
	for _, token := range profile.Argv {
		references, err := argvReferences(token)
		if err != nil {
			return err
		}
		for _, name := range references {
			parameter, ok := parameters[name]
			if !ok {
				return fmt.Errorf("argv references unknown parameter %q", name)
			}
			if !parameter.Required && parameter.Default == nil {
				return fmt.Errorf(
					"argv parameter %q must be required or have a default", name,
				)
			}
		}
	}
	return nil
}

func argvReferences(token string) ([]string, error) {
	matches := argvReferencePattern.FindAllStringSubmatch(token, -1)
	remainder := argvReferencePattern.ReplaceAllString(token, "")
	if strings.Contains(remainder, "{{") || strings.Contains(remainder, "}}") {
		return nil, fmt.Errorf("malformed argv template %q", token)
	}
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		out = append(out, match[1])
	}
	return out, nil
}

func validateParameters(params []CommandParameterConfig) error {
	seen := make(map[string]struct{}, len(params))
	for i, p := range params {
		if p.Name == "" {
			return fmt.Errorf("parameter %d requires name", i)
		}
		if !parameterNamePattern.MatchString(p.Name) {
			return fmt.Errorf("parameter %q has invalid name", p.Name)
		}
		if _, ok := seen[p.Name]; ok {
			return fmt.Errorf("duplicate parameter %q", p.Name)
		}
		seen[p.Name] = struct{}{}
		switch p.Type {
		case ParameterEnum:
			if len(p.Values) == 0 {
				return fmt.Errorf("enum parameter %q requires values", p.Name)
			}
		case ParameterBoolean:
		case ParameterString:
			if p.Pattern != "" {
				if _, err := regexp.Compile(p.Pattern); err != nil {
					return fmt.Errorf("parameter %q pattern: %w", p.Name, err)
				}
			}
		case ParameterNumber:
			if p.Min != nil && p.Max != nil && *p.Min > *p.Max {
				return fmt.Errorf("parameter %q min exceeds max", p.Name)
			}
		default:
			return fmt.Errorf("parameter %q has unsupported type %q", p.Name, p.Type)
		}
		if p.Default != nil {
			if _, err := normalizeParameterValue(p, p.Default); err != nil {
				return fmt.Errorf("parameter %q default: %w", p.Name, err)
			}
		}
	}
	return nil
}

func (s SessionConfig) ResolveProfiles(project string) ([]ResolvedCommandProfile, error) {
	projectConfig, err := LoadProject(project)
	if err != nil {
		return nil, err
	}
	merged := append([]CommandProfileConfig(nil), s.CommandProfiles...)
	index := make(map[string]int, len(merged))
	for i, profile := range merged {
		index[profile.ID] = i
	}
	for _, profile := range projectConfig.Session.CommandProfiles {
		if profile.Disabled {
			if i, ok := index[profile.ID]; ok {
				merged = append(merged[:i], merged[i+1:]...)
				index = profileIndexes(merged)
			}
			continue
		}
		if i, ok := index[profile.ID]; ok {
			merged[i] = profile
			continue
		}
		index[profile.ID] = len(merged)
		merged = append(merged, profile)
	}
	out := make([]ResolvedCommandProfile, 0, len(merged))
	for _, profile := range merged {
		out = append(out, resolvedProfile(profile))
	}
	return out, nil
}

func profileIndexes(profiles []CommandProfileConfig) map[string]int {
	index := make(map[string]int, len(profiles))
	for i, profile := range profiles {
		index[profile.ID] = i
	}
	return index
}

func resolvedProfile(p CommandProfileConfig) ResolvedCommandProfile {
	encoded, _ := json.Marshal(p)
	sum := sha256.Sum256(encoded)
	return ResolvedCommandProfile{
		CommandProfileConfig: p,
		Revision:             hex.EncodeToString(sum[:]),
	}
}

func MaterializeResolvedProfile(
	profiles []ResolvedCommandProfile, id, revision string,
	values map[string]any,
) ([]string, error) {
	for _, profile := range profiles {
		if profile.ID != id {
			continue
		}
		if profile.Revision != revision {
			return nil, fmt.Errorf("stale profile revision")
		}
		return materializeProfile(profile.CommandProfileConfig, values)
	}
	return nil, fmt.Errorf("profile %q is not available", id)
}

func materializeProfile(profile CommandProfileConfig, values map[string]any) ([]string, error) {
	normalized := make(map[string]string, len(profile.Parameters))
	known := make(map[string]struct{}, len(profile.Parameters))
	for _, parameter := range profile.Parameters {
		known[parameter.Name] = struct{}{}
		value, ok := values[parameter.Name]
		if !ok {
			value = parameter.Default
		}
		if value == nil {
			if parameter.Required {
				return nil, fmt.Errorf("parameter %q is required", parameter.Name)
			}
			continue
		}
		text, err := normalizeParameterValue(parameter, value)
		if err != nil {
			return nil, fmt.Errorf("parameter %q: %w", parameter.Name, err)
		}
		normalized[parameter.Name] = text
	}
	for name := range values {
		if _, ok := known[name]; !ok {
			return nil, fmt.Errorf("unknown parameter %q", name)
		}
	}
	argv := []string{profile.Executable}
	for _, token := range profile.Argv {
		references, err := argvReferences(token)
		if err != nil {
			return nil, err
		}
		for _, name := range references {
			if _, ok := normalized[name]; !ok {
				return nil, fmt.Errorf("argv parameter %q has no value", name)
			}
		}
		rendered := argvReferencePattern.ReplaceAllStringFunc(token, func(reference string) string {
			match := argvReferencePattern.FindStringSubmatch(reference)
			return normalized[match[1]]
		})
		argv = append(argv, rendered)
	}
	return argv, nil
}

func normalizeParameterValue(parameter CommandParameterConfig, value any) (string, error) {
	switch parameter.Type {
	case ParameterEnum:
		text, ok := value.(string)
		if !ok {
			return "", fmt.Errorf("must be a string")
		}
		for _, candidate := range parameter.Values {
			if text == candidate {
				return text, nil
			}
		}
		return "", fmt.Errorf("must be one of %v", parameter.Values)
	case ParameterBoolean:
		boolean, ok := value.(bool)
		if !ok {
			return "", fmt.Errorf("must be a boolean")
		}
		return strconv.FormatBool(boolean), nil
	case ParameterString:
		text, ok := value.(string)
		if !ok {
			return "", fmt.Errorf("must be a string")
		}
		if parameter.Pattern != "" {
			matched, _ := regexp.MatchString(parameter.Pattern, text)
			if !matched {
				return "", fmt.Errorf("does not match configured pattern")
			}
		}
		return text, nil
	case ParameterNumber:
		number, ok := numericValue(value)
		if !ok {
			return "", fmt.Errorf("must be a number")
		}
		if parameter.Min != nil && number < *parameter.Min {
			return "", fmt.Errorf("must be at least %v", *parameter.Min)
		}
		if parameter.Max != nil && number > *parameter.Max {
			return "", fmt.Errorf("must be at most %v", *parameter.Max)
		}
		return strconv.FormatFloat(number, 'f', -1, 64), nil
	default:
		return "", fmt.Errorf("unsupported type %q", parameter.Type)
	}
}

func numericValue(value any) (float64, bool) {
	switch number := value.(type) {
	case float64:
		return number, true
	case int64:
		return float64(number), true
	case int:
		return float64(number), true
	default:
		return 0, false
	}
}

func SortedProfileIDs(profiles []ResolvedCommandProfile) []string {
	ids := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		ids = append(ids, profile.ID)
	}
	sort.Strings(ids)
	return ids
}
