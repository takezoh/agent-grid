// Package clientconfig is the Go projection of the canonical client
// configuration contract in clients/config.
//
// It is the only Go parser of servers.json: both Go hosts — the web host and
// the terminal CLI — import it, and TestClientConfigHasExactlyOneLoaderPerLanguage
// fails when a second one appears. The contract itself is not owned here; this
// package is one of three loaders (Go, TypeScript, C#) bound to it, and the
// fixture corpus under clients/config/fixtures is what makes the three
// comparable rather than merely similar
// (adr-20260814-shared-go-client-config-module).
//
// Admission is per entry. A malformed entry costs that entry and the entries it
// collides with, not the fleet: an operator's typo in a server they are not
// using must not take away every other server on every host
// (adr-20260814-server-display-name-naming-contract). Only an unreadable entry
// list stops the host, because that is the one case where no entry can be
// named.
//
// Nothing in this package writes. A configuration directory is the operator's,
// and a client that repairs what it reads makes the same document mean
// different things depending on which host opened it last. In particular the
// loader never renames, normalises, suffixes, disables or drops an entry to
// resolve a violation.
package clientconfig

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const ServersSchemaVersion = 2

// Rule identifiers, shared with clients/config/servers.schema.json
// x-contract-rules and with the expected-outcome corpus. A reported violation
// names one of these so an operator is told which rule an entry broke, not
// merely that something was wrong.
const (
	RuleDisplayNameRequired  = "display-name-required"
	RuleDisplayNameCharset   = "display-name-charset"
	RuleDisplayNameUnique    = "display-name-unique"
	RuleEntryFieldTypes      = "entry-field-types"
	RuleUnknownEntryField    = "unknown-entry-field-rejected"
	RuleURLScheme            = "url-http-or-https"
	RuleURLBareAuthority     = "url-carries-no-credentials-query-or-fragment"
	RuleDuplicateURL         = "duplicate-url-rejected"
	RuleAuthVariant          = "auth-variant-exact"
	RuleAtMostOneDefault     = "at-most-one-default-execution"
	RuleDefaultMustBeEnabled = "default-execution-must-be-enabled"
)

// Admission states. Every loaded entry is in exactly one of them, and the
// corpus records which one each fixture entry must reach.
const (
	// AdmissionUsable is a valid, enabled entry — one that is connected to.
	AdmissionUsable = "usable"
	// AdmissionDisabled is a valid entry the operator turned off. It is never
	// connected to, and it is not an error.
	AdmissionDisabled = "disabled"
	// AdmissionConfigurationInvalid is an entry that broke a rule, or that
	// collides with one that did. It is never probed and never connected to,
	// and it is reported so the operator can find it — without it, per-entry
	// admission would be worse than the whole-file stop it replaces, because
	// a server would simply be missing with no signal.
	AdmissionConfigurationInvalid = "configuration_invalid"
)

type Auth struct {
	Mode  string `json:"mode"`
	Token string `json:"token,omitempty"`
}

// Violation is why an entry is unusable: the rule it broke and enough detail
// to find it in the file. Position and the display name as written are on the
// Server itself.
type Violation struct {
	Rule   string
	Detail string
}

// Server is a loaded entry. ID and URL hold the derived identifier and the
// canonical URL, neither of which is read from the file. Both are empty on an
// entry whose URL could not be canonicalised.
type Server struct {
	// Index is the entry's position in the file's servers array, which is how
	// an entry with no usable name is still identifiable.
	Index            int
	ID               string
	DisplayName      string
	Enabled          bool
	DefaultExecution bool
	URL              string
	Auth             Auth
	// Violation is nil for an admissible entry.
	Violation *Violation
}

// Valid reports whether the entry may be used at all.
func (server Server) Valid() bool { return server.Violation == nil }

// Admission reports how this entry may be used. It is a named state rather
// than a bare bool because the vocabulary is shared with the TypeScript and
// C# loaders through the fixture corpus.
func (server Server) Admission() string {
	switch {
	case server.Violation != nil:
		return AdmissionConfigurationInvalid
	case server.Enabled:
		return AdmissionUsable
	default:
		return AdmissionDisabled
	}
}

type Snapshot struct {
	Servers []Server
}

// UsableServers returns the entries a client may connect to. An invalid or
// disabled entry is never among them.
func (snapshot Snapshot) UsableServers() []Server {
	usable := make([]Server, 0, len(snapshot.Servers))
	for _, server := range snapshot.Servers {
		if server.Admission() == AdmissionUsable {
			usable = append(usable, server)
		}
	}
	return usable
}

// InvalidServers returns the entries a client must present as
// configuration-invalid rather than silently omit.
func (snapshot Snapshot) InvalidServers() []Server {
	invalid := make([]Server, 0)
	for _, server := range snapshot.Servers {
		if server.Violation != nil {
			invalid = append(invalid, server)
		}
	}
	return invalid
}

func (snapshot Snapshot) EnabledServer(id string) (Server, bool) {
	for _, server := range snapshot.Servers {
		if server.Admission() == AdmissionUsable && server.ID == id {
			return server, true
		}
	}
	return Server{}, false
}

// serversDocument is the file-level shape. Servers is a pointer so an absent
// list is distinguishable from an empty one: an absent list means no entry can
// be named and the host stops, while an empty list is simply an empty fleet.
type serversDocument struct {
	SchemaVersion int                `json:"schema_version"`
	Servers       *[]json.RawMessage `json:"servers"`
}

// serverDocument is the on-disk shape of one entry, kept separate from Server
// so the derived identifier can never be read from the file.
//
// LegacyID accepts the deprecated `id` field older clients wrote, which
// clients/config/servers.schema.json still permits; it is read and discarded.
// Every other unrecognised field is rejected rather than dropped: a silently
// ignored field is an operator's intent that the client never carried out and
// never reported.
type serverDocument struct {
	LegacyID         string `json:"id"`
	DisplayName      string `json:"display_name"`
	Enabled          bool   `json:"enabled"`
	DefaultExecution bool   `json:"default_execution"`
	URL              string `json:"url"`
	Auth             Auth   `json:"auth"`
}

// DefaultDirectory resolves the configuration directory this host shares with
// every other client host on the same machine.
func DefaultDirectory() (string, error) {
	return directoryFor(runtime.GOOS, os.Getenv, os.UserHomeDir)
}

// directoryFor is the platform mapping, taken as data so it can be checked
// against the graphical hosts on any machine rather than only on the platform
// that happens to be running the test.
//
// The Windows branch is the one this loader lacked. Without it a Windows CLI
// would read a different directory than the Shell and Workspace on the same
// machine and report a different fleet — the same operator, the same
// configuration, two answers.
func directoryFor(goos string, env func(string) string, homeDir func() (string, error)) (string, error) {
	if goos == "windows" {
		if appData := strings.TrimSpace(env("APPDATA")); appData != "" {
			return filepath.Join(appData, "agent-grid", "config"), nil
		}
		home, err := homeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		return filepath.Join(home, "AppData", "Roaming", "agent-grid", "config"), nil
	}
	if value := env("XDG_CONFIG_HOME"); value != "" {
		return filepath.Join(value, "agent-grid"), nil
	}
	home, err := homeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".config", "agent-grid"), nil
}

// Load reads servers.json. It returns an error only for the three file-level
// stops — unreadable bytes, a document that is not JSON or carries the wrong
// schema version, and an absent or non-array entry list — because those are
// exactly the cases where no entry can be named. Every other failure is
// admitted as a configuration-invalid entry in the returned snapshot.
func Load(directory string) (Snapshot, error) {
	data, err := os.ReadFile(filepath.Join(directory, "servers.json"))
	if err != nil {
		return Snapshot{}, fmt.Errorf("config_unavailable: servers.json: %w", err)
	}
	var document serversDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return Snapshot{}, fmt.Errorf("config_invalid: servers.json: %w", err)
	}
	if document.SchemaVersion != ServersSchemaVersion {
		return Snapshot{}, fmt.Errorf(
			"config_invalid: servers.json: unsupported schema_version %d; expected %d",
			document.SchemaVersion,
			ServersSchemaVersion,
		)
	}
	if document.Servers == nil {
		return Snapshot{}, errors.New("config_invalid: servers.json: servers must be an array")
	}
	return Snapshot{Servers: admitServers(*document.Servers)}, nil
}

// admitServers decides each entry's admission. Entry-local rules run first so
// that a relation between entries — a duplicate name, a duplicate URL, two
// default-execution markers — is only computed over entries that are otherwise
// sound.
func admitServers(raw []json.RawMessage) []Server {
	servers := make([]Server, 0, len(raw))
	for index, entry := range raw {
		servers = append(servers, admitEntry(index, entry))
	}
	markDuplicateURLs(servers)
	markDuplicateDisplayNames(servers)
	markDefaultExecution(servers)
	return servers
}

// admitEntry applies every rule decidable from one entry alone.
func admitEntry(index int, raw json.RawMessage) Server {
	server := Server{Index: index}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var document serverDocument
	if err := decoder.Decode(&document); err != nil {
		server.Violation = &Violation{Rule: decodeRule(err), Detail: err.Error()}
		server.DisplayName = salvageDisplayName(raw)
		return server
	}

	server.DisplayName = document.DisplayName
	server.Enabled = document.Enabled
	server.DefaultExecution = document.DefaultExecution
	server.URL = document.URL
	server.Auth = document.Auth

	if violation := checkDisplayName(document.DisplayName); violation != nil {
		server.Violation = violation
		return server
	}
	canonical, err := canonicalURL(document.URL)
	if err != nil {
		server.Violation = &Violation{Rule: urlRule(err), Detail: err.Error()}
		return server
	}
	server.URL = canonical
	server.ID = ServerID(canonical)

	if violation := checkAuth(document.Auth); violation != nil {
		server.Violation = violation
		return server
	}
	if document.DefaultExecution && !document.Enabled {
		server.Violation = &Violation{
			Rule:   RuleDefaultMustBeEnabled,
			Detail: "a default execution entry must be enabled",
		}
	}
	return server
}

// decodeRule separates a value of the wrong type from a field the contract
// does not define, so the operator is told which of the two they wrote.
func decodeRule(err error) string {
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		return RuleEntryFieldTypes
	}
	if strings.Contains(err.Error(), "unknown field") {
		return RuleUnknownEntryField
	}
	return RuleEntryFieldTypes
}

func urlRule(err error) string {
	if errors.Is(err, errURLScheme) {
		return RuleURLScheme
	}
	return RuleURLBareAuthority
}

// salvageDisplayName recovers the name as written from an entry that failed to
// decode, so a report can still identify it by more than its position.
func salvageDisplayName(raw json.RawMessage) string {
	var partial struct {
		DisplayName string `json:"display_name"`
	}
	_ = json.Unmarshal(raw, &partial)
	return partial.DisplayName
}

// displayNameKey is the uniqueness key: exact code points after trimming
// leading and trailing whitespace. No case folding and no Unicode
// normalisation, because those would require Go, TypeScript and C# to agree on
// a fold — a new cross-language divergence in the very code path this contract
// exists to make consistent.
func displayNameKey(name string) string { return strings.TrimSpace(name) }

func checkDisplayName(name string) *Violation {
	trimmed := displayNameKey(name)
	if trimmed == "" {
		return &Violation{Rule: RuleDisplayNameRequired, Detail: "display_name must not be blank"}
	}
	for _, r := range trimmed {
		if r <= 0x1F || r == 0x7F {
			return &Violation{
				Rule:   RuleDisplayNameCharset,
				Detail: fmt.Sprintf("display_name contains control character U+%04X", r),
			}
		}
	}
	if strings.HasPrefix(trimmed, "-") {
		return &Violation{
			Rule:   RuleDisplayNameCharset,
			Detail: "display_name must not begin with '-', which a command line reads as a flag",
		}
	}
	return nil
}

func checkAuth(auth Auth) *Violation {
	switch auth.Mode {
	case "none":
		if auth.Token != "" {
			return &Violation{Rule: RuleAuthVariant, Detail: "auth mode none must not carry a token"}
		}
	case "bearer":
		if strings.TrimSpace(auth.Token) == "" {
			return &Violation{Rule: RuleAuthVariant, Detail: "auth mode bearer needs a non-blank token"}
		}
	default:
		return &Violation{
			Rule:   RuleAuthVariant,
			Detail: fmt.Sprintf("unsupported auth mode %q", auth.Mode),
		}
	}
	return nil
}

// markDuplicateURLs invalidates every member of a collision group. No winner is
// chosen: a duplicate is a property of a pair, so blaming one member requires
// choosing which, and every choice rule silently redirects an operator's
// existing reference to a different machine.
func markDuplicateURLs(servers []Server) {
	markCollisions(servers, func(server Server) (string, bool) {
		return server.ID, server.ID != ""
	}, RuleDuplicateURL, "two entries resolve to the same canonical url")
}

// markDuplicateDisplayNames applies uniqueness among enabled entries only. A
// disabled entry is never addressed, so it cannot make another name ambiguous.
func markDuplicateDisplayNames(servers []Server) {
	markCollisions(servers, func(server Server) (string, bool) {
		return displayNameKey(server.DisplayName), server.Enabled
	}, RuleDisplayNameUnique, "two enabled entries share a display name")
}

func markCollisions(servers []Server, key func(Server) (string, bool), rule, detail string) {
	groups := map[string][]int{}
	for index, server := range servers {
		if server.Violation != nil {
			continue
		}
		value, ok := key(server)
		if !ok || value == "" {
			continue
		}
		groups[value] = append(groups[value], index)
	}
	for _, members := range groups {
		if len(members) < 2 {
			continue
		}
		for _, index := range members {
			servers[index].Violation = &Violation{Rule: rule, Detail: detail}
		}
	}
}

// markDefaultExecution invalidates the whole marked group when more than one
// entry claims the default, then applies the legacy fallback: a document with
// no marker uses its first usable entry, without being rewritten.
func markDefaultExecution(servers []Server) {
	marked := []int{}
	for index, server := range servers {
		if server.Violation == nil && server.DefaultExecution {
			marked = append(marked, index)
		}
	}
	if len(marked) > 1 {
		for _, index := range marked {
			servers[index].Violation = &Violation{
				Rule:   RuleAtMostOneDefault,
				Detail: "more than one entry is marked as the default execution server",
			}
		}
		// Every marker is now unusable, so no marker stands and the legacy
		// fallback below applies. Leaving the fleet with no execution target
		// would make an ambiguous marker cost more than an absent one.
		marked = nil
	}
	if len(marked) == 1 {
		return
	}
	for index, server := range servers {
		if server.Admission() == AdmissionUsable {
			servers[index].DefaultExecution = true
			return
		}
	}
}

var errURLScheme = errors.New("scheme must be http or https")

// canonicalURL folds a configured gateway URL into the one form every client
// host derives identity from.
//
// Credentials, a query and a fragment are rejected rather than stripped. All
// three are things an operator wrote on purpose; silently discarding them
// leaves the operator believing the client is doing something it is not — most
// sharply for a credential, where the belief is that authentication is
// configured. Rejecting names the entry instead.
func canonicalURL(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errURLScheme
	}
	if parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("credentials, query, fragment, or empty host are not allowed")
	}
	parsed.Host = canonicalHost(parsed.Scheme, parsed.Hostname(), parsed.Port())
	parsed.Path = strings.TrimRight(parsed.EscapedPath(), "/")
	return parsed.String(), nil
}

// canonicalHost lower-cases the host and drops a default port, which the
// TypeScript and C# URL parsers have always done and this one did not. Without
// it the same gateway written two equivalent ways yields two identities, and a
// fleet assembled from a Go host and a graphical host disagrees about which
// rows are the same row.
func canonicalHost(scheme, hostname, port string) string {
	host := strings.ToLower(hostname)
	if strings.Contains(host, ":") {
		// An IPv6 literal loses its brackets through Hostname().
		host = "[" + host + "]"
	}
	if port == "" || isDefaultPort(scheme, port) {
		return host
	}
	return host + ":" + port
}

func isDefaultPort(scheme, port string) bool {
	return (scheme == "http" && port == "80") || (scheme == "https" && port == "443")
}

func ServerID(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return "srv_" + hex.EncodeToString(sum[:])
}
