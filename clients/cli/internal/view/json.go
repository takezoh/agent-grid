package view

import (
	"encoding/json"
	"io"

	"github.com/takezoh/agent-grid/clients/cli/internal/fleet"
)

// json.go is the machine-readable projection of the same observation the table
// renders.
//
// Two design rules come from the experience contract rather than from taste.
// Server identity and session identity are separate fields, never one
// concatenated string, so a caller can feed either back without parsing
// (FR-019). And the field set has the same breadth as the default columns, so
// automation is never pushed back to scraping the table for a value the
// structured output withheld.

// jsonSchemaVersion is this projection's own version. It is not the wire
// protocol version: this shape is the CLI's output contract with whatever is
// reading it, and the two move independently.
const jsonSchemaVersion = 1

type jsonDocument struct {
	SchemaVersion int          `json:"schema_version"`
	Servers       []jsonServer `json:"servers"`
	Sessions      []jsonRow    `json:"sessions"`
}

// jsonServer carries the per-entry facts. Admission and reachability are
// separate fields on purpose: an entry that was never contacted has no
// reachability at all, and merging the two would force a value meaning "we did
// not look" into a vocabulary about what we saw.
type jsonServer struct {
	Index       int    `json:"index"`
	ServerID    string `json:"server_id"`
	DisplayName string `json:"display_name"`
	Admission   string `json:"admission"`
	// Reachability is empty for an entry that was never contacted.
	Reachability string `json:"reachability,omitempty"`
	Reason       string `json:"reason,omitempty"`
	Rule         string `json:"rule,omitempty"`
	Detail       string `json:"detail,omitempty"`
	SessionCount int    `json:"session_count"`
}

// jsonRow is one session. Its fields are the default columns, one per key.
type jsonRow struct {
	ServerID    string `json:"server_id"`
	ServerName  string `json:"server_name"`
	SessionID   string `json:"session_id"`
	DisplayName string `json:"display_name"`
	Cwd         string `json:"cwd"`
	Worktree    string `json:"worktree,omitempty"`
	State       string `json:"state"`
	StateLabel  string `json:"state_label"`
	Signal      bool   `json:"signal,omitempty"`
	Model       string `json:"model,omitempty"`
}

// admissionOf separates the three answers to "why is this entry not showing
// sessions" into the three vocabularies the contract requires.
func admissionOf(member fleet.Member) (admission, reachability string) {
	switch member.Observation.Kind {
	case fleet.KindConfigurationInvalid:
		return fleet.KindConfigurationInvalid, ""
	case fleet.KindDisabled:
		return fleet.KindDisabled, ""
	default:
		// The entry was usable, so it was contacted; what happened next is a
		// reachability observation and belongs in its own field.
		return "usable", member.Observation.Kind
	}
}

// WriteJSON emits the machine-readable listing.
func WriteJSON(out io.Writer, snapshot fleet.Snapshot) error {
	document := jsonDocument{
		SchemaVersion: jsonSchemaVersion,
		Servers:       make([]jsonServer, 0, len(snapshot.Members)),
		Sessions:      make([]jsonRow, 0),
	}
	for _, member := range snapshot.Members {
		admission, reachability := admissionOf(member)
		document.Servers = append(document.Servers, jsonServer{
			Index:        member.Index,
			ServerID:     member.ServerID,
			DisplayName:  member.DisplayName,
			Admission:    admission,
			Reachability: reachability,
			Reason:       member.Observation.Reason,
			Rule:         member.Observation.Rule,
			Detail:       member.Observation.Detail,
			SessionCount: len(member.Sessions),
		})
	}
	for _, row := range Rows(snapshot) {
		document.Sessions = append(document.Sessions, jsonRow{
			ServerID:    row.Ref.ServerID(),
			ServerName:  row.Ref.ServerName(),
			SessionID:   row.Ref.SessionID(),
			DisplayName: row.Ref.DisplayName(),
			Cwd:         row.Cwd,
			Worktree:    row.Worktree,
			State:       string(row.State),
			StateLabel:  Label(row.State),
			Signal:      row.Signal,
			Model:       row.Model,
		})
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(document)
}
