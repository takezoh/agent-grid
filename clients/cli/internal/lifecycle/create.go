// Package lifecycle creates and terminates sessions. It is the only place in
// this module that asks a server to change what exists.
package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/takezoh/agent-grid/clients/cli/internal/fleet"
	"github.com/takezoh/agent-grid/clients/cli/internal/gateway"
	"github.com/takezoh/agent-grid/clients/goclient/clientconfig"
)

// Profile is one launch profile as the target server publishes it.
//
// Revision is server-computed and server-validated. It is carried through
// rather than dropped because a creation that omits it is refused, and a
// creation that sends a stale one is refused with a distinct conflict — which
// is exactly the signal that the operator's idea of the profile no longer
// matches the server's.
type Profile struct {
	ID       string
	Label    string
	Revision string
}

// ErrTargetRequired is returned when a creation names no server.
var ErrTargetRequired = errors.New("session creation requires an explicit target server")

// ErrUnknownAgent is returned when the requested profile does not exist on the
// target server.
var ErrUnknownAgent = errors.New("no such launch profile on this server")

// SelectCreateTarget finds the server a creation names.
//
// There is deliberately no fallback of any kind. The client configuration
// carries a `default_execution` marker and every loader applies a
// first-enabled fallback when the marker is absent, so an implementation that
// "helpfully" resolves an empty qualifier has two ready-made ways to pick a
// server the operator did not name — and creating a session on the wrong
// machine is not an error the operator can see until they go looking for it.
//
// This function therefore never consults either. TestDefaultExecutionIsNeverReadAsACreateTarget
// drives a fleet whose single usable entry is marked for default execution and
// requires this to still refuse.
func SelectCreateTarget(snapshot fleet.Snapshot, qualifier string) (fleet.Member, error) {
	if strings.TrimSpace(qualifier) == "" {
		return fleet.Member{}, ErrTargetRequired
	}
	var matched []fleet.Member
	for _, member := range snapshot.Members {
		if member.DisplayName == qualifier || member.ServerID == qualifier {
			matched = append(matched, member)
		}
	}
	if len(matched) == 0 {
		return fleet.Member{}, fmt.Errorf("no configured server is named %q", qualifier)
	}
	var usable []fleet.Member
	for _, member := range matched {
		if member.Server.Admission() == clientconfig.AdmissionUsable {
			usable = append(usable, member)
		}
	}
	switch len(usable) {
	case 0:
		return fleet.Member{}, fmt.Errorf("every configured entry named %q is unusable", qualifier)
	case 1:
		return usable[0], nil
	default:
		return fleet.Member{}, fmt.Errorf("%q names more than one usable server", qualifier)
	}
}

// sessionConfigResponse is the shape GET /api/session-config returns. Only the
// two profile carriers are declared; the rest of the document is for the
// graphical clients' own forms.
type sessionConfigResponse struct {
	BaseCommandProfiles []profileWire `json:"base_command_profiles"`
	Projects            []struct {
		Path            string        `json:"path"`
		CommandProfiles []profileWire `json:"command_profiles"`
	} `json:"projects"`
}

type profileWire struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Revision string `json:"revision"`
}

// LoadProfiles reads the launch profile set from the target server, for one
// project directory.
//
// It is read at invocation time and never cached across invocations. The
// revision is a digest of the profile's own definition, so a cached one goes
// stale the moment the operator edits their settings — and a creation carrying
// a stale revision is refused with a conflict rather than silently launching
// the old command.
func LoadProfiles(ctx context.Context, client *gateway.Client, project string) ([]Profile, error) {
	var response sessionConfigResponse
	if err := client.GetJSON(ctx, "/api/session-config", &response); err != nil {
		return nil, err
	}
	// A project may override the base set. The project's own list wins when the
	// creation names that project, because that is the list the server will
	// resolve the creation against.
	wire := response.BaseCommandProfiles
	for _, candidate := range response.Projects {
		if candidate.Path == project && len(candidate.CommandProfiles) > 0 {
			wire = candidate.CommandProfiles
			break
		}
	}
	profiles := make([]Profile, 0, len(wire))
	for _, entry := range wire {
		profiles = append(profiles, Profile(entry))
	}
	return profiles, nil
}

// ResolveProfile finds the requested profile.
//
// Resolving before the creating request is the whole point: an implementation
// that posted first and validated afterwards leaves a session behind whenever
// the profile name was wrong, and the operator has to notice and clean up a
// session they never meant to start.
func ResolveProfile(profiles []Profile, agent string) (Profile, error) {
	for _, profile := range profiles {
		if profile.ID == agent {
			return profile, nil
		}
	}
	available := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		available = append(available, profile.ID)
	}
	sort.Strings(available)
	if len(available) == 0 {
		return Profile{}, fmt.Errorf("%w: %q (this server publishes no launch profiles)",
			ErrUnknownAgent, agent)
	}
	return Profile{}, fmt.Errorf("%w: %q (available: %s)",
		ErrUnknownAgent, agent, strings.Join(available, ", "))
}

// Created is the session a creation produced.
type Created struct {
	ID      string `json:"id"`
	Project string `json:"project"`
	Command string `json:"command"`
}

// createRequest is the POST /api/sessions body, matching the Phase 0 capture.
type createRequest struct {
	Version         int            `json:"version"`
	Project         string         `json:"project"`
	ProfileID       string         `json:"profile_id"`
	ProfileRevision string         `json:"profile_revision"`
	Parameters      map[string]any `json:"parameters,omitempty"`
	Cols            int            `json:"cols,omitempty"`
	Rows            int            `json:"rows,omitempty"`
	Worktree        bool           `json:"worktree,omitempty"`
}

// Params is one creation request.
type Params struct {
	Project    string
	Agent      string
	Parameters map[string]string
	Cols, Rows int
	Worktree   bool
	// NoAttach suppresses the attach that normally follows a creation.
	NoAttach bool
	// Attach is called with the created session unless NoAttach is set. It is
	// injected so the three outcomes below can be driven without a terminal.
	Attach func(created Created) error
}

// Result is what a creation run observed.
//
// The three fields make four states distinguishable, and they have to be:
// a failed creation, a creation whose attach failed, a creation with no attach
// asked for, and a complete success are four different things an operator or a
// script must be able to tell apart. Collapsing the middle two into "failed"
// hides a live session; collapsing them into "succeeded" hides that nobody is
// looking at it.
type Result struct {
	// Created is nil only when the creation itself failed.
	Created *Created
	// CreateErr is set only when Created is nil.
	CreateErr error
	// AttachErr is set only when the creation succeeded and the attach that
	// followed did not.
	AttachErr error
	// Attached reports whether an attach was attempted at all.
	Attached bool
}

// Run resolves the profile, creates the session, and attaches to it.
func Run(ctx context.Context, client *gateway.Client, params Params) Result {
	profiles, err := LoadProfiles(ctx, client, params.Project)
	if err != nil {
		return Result{CreateErr: err}
	}
	profile, err := ResolveProfile(profiles, params.Agent)
	if err != nil {
		return Result{CreateErr: err}
	}

	parameters := make(map[string]any, len(params.Parameters))
	for name, value := range params.Parameters {
		parameters[name] = value
	}
	var created Created
	err = client.PostJSON(ctx, "/api/sessions", createRequest{
		Version:         2,
		Project:         params.Project,
		ProfileID:       profile.ID,
		ProfileRevision: profile.Revision,
		Parameters:      parameters,
		Cols:            params.Cols,
		Rows:            params.Rows,
		Worktree:        params.Worktree,
	}, &created)
	if err != nil {
		return Result{CreateErr: err}
	}
	if params.NoAttach || params.Attach == nil {
		return Result{Created: &created}
	}
	if attachErr := params.Attach(created); attachErr != nil {
		return Result{Created: &created, AttachErr: attachErr, Attached: true}
	}
	return Result{Created: &created, Attached: true}
}
