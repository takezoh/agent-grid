package view

import (
	"fmt"
	"io"
	"strings"

	"github.com/takezoh/agent-grid/clients/cli/internal/fleet"
	"github.com/takezoh/agent-grid/clients/cli/internal/gateway"
)

// table.go renders the session listing for a human reader.
//
// Two rules constrain the layout more than taste does. Nothing is truncated —
// FR-018 makes the display name the server's card title verbatim, and a column
// that elides it would make two sessions indistinguishable at exactly the
// moment the operator needs to tell them apart. And nothing depends on whether
// stdout is a terminal — no colour, no terminal-width fitting — because FR-025
// requires identical content either way.

// Row is one session as the listing presents it. The field set is the default
// column set (DP-LS-DEFAULT-COLUMNS), and the machine-readable projection
// carries exactly the same breadth.
type Row struct {
	Ref      gateway.SessionRef
	Cwd      string
	Worktree string
	State    State
	Signal   bool
	Model    string
}

// Rows projects an observed fleet into session rows, in fleet order and then in
// the order the server published.
//
// Nothing is filtered. A session whose lifecycle state says it has ended is a
// row like any other: the server publishes it deliberately now that ended
// sessions are retained, and a client that hid them would put the operator back
// to guessing whether a session ended or was never there.
func Rows(snapshot fleet.Snapshot) []Row {
	rows := make([]Row, 0)
	for _, member := range snapshot.Members {
		for _, session := range member.Sessions {
			rows = append(rows, Row{
				Ref:      gateway.NewSessionRef(member.Server, session),
				Cwd:      session.Project,
				Worktree: session.WorkspaceRoot,
				State:    Collapse(session.Status),
				Signal:   HasSignal(session.Status),
				Model:    session.View.Model,
			})
		}
	}
	return rows
}

// serverColumn is how a server is named in a row. The joining rule belongs to
// the identity owner, so this only decides whether the name is ambiguous.
func serverColumn(row Row, ambiguous map[string]bool) string {
	return row.Ref.ServerLabel(ambiguous[row.Ref.ServerName()])
}

// ambiguousServerNames reports which display names appear more than once.
func ambiguousServerNames(snapshot fleet.Snapshot) map[string]bool {
	counts := make(map[string]int, len(snapshot.Members))
	for _, member := range snapshot.Members {
		counts[member.DisplayName]++
	}
	ambiguous := make(map[string]bool, len(counts))
	for name, count := range counts {
		if count > 1 {
			ambiguous[name] = true
		}
	}
	return ambiguous
}

// WriteTable renders the listing.
//
// Servers that did not answer, and entries that were never contacted, are
// printed below the sessions rather than omitted: an empty listing that hides
// the reason it is empty is the failure AC-partial-outage exists to prevent.
func WriteTable(out io.Writer, snapshot fleet.Snapshot) {
	rows := Rows(snapshot)
	ambiguous := ambiguousServerNames(snapshot)

	if len(rows) > 0 {
		header := []string{"SESSION", "NAME", "SERVER", "CWD", "STATE", "MODEL"}
		table := make([][]string, 0, len(rows)+1)
		table = append(table, header)
		for _, row := range rows {
			table = append(table, []string{
				row.Ref.SessionID(),
				row.Ref.DisplayName(),
				serverColumn(row, ambiguous),
				workingDirectory(row),
				stateColumn(row),
				row.Model,
			})
		}
		writeAligned(out, table)
	}

	writeServerConditions(out, snapshot, len(rows) > 0)
}

// workingDirectory is the project directory, with the worktree root shown
// beside it when the session runs in one. `project` is the field the shipping
// SPA presents as the session's directory; `workspace_root` is the separate
// fact that it is running in a worktree.
func workingDirectory(row Row) string {
	if row.Worktree != "" && row.Worktree != row.Cwd {
		return row.Cwd + " (worktree " + row.Worktree + ")"
	}
	return row.Cwd
}

// stateColumn is the shared label, with the transient signal marked beside it
// rather than replacing it — the matrix allows a bell to coexist with both
// working and action-required.
func stateColumn(row Row) string {
	label := Label(row.State)
	if row.Signal {
		return label + " *"
	}
	return label
}

// writeServerConditions prints one line per entry that has no sessions to show,
// naming why. Three vocabularies meet here and none of them borrows a word from
// another: an unreachable server, an entry that was never contacted, and a
// healthy server that simply has nothing running.
func writeServerConditions(out io.Writer, snapshot fleet.Snapshot, printedRows bool) {
	var lines []string
	for _, member := range snapshot.Members {
		switch member.Observation.Kind {
		case fleet.KindConnected:
			if len(member.Sessions) == 0 {
				lines = append(lines, fmt.Sprintf("%s: no sessions", member.DisplayName))
			}
		case fleet.KindConfigurationInvalid:
			// Position, display name as written, and the violated rule — the
			// three things FR-031 requires, and the three things a loader that
			// "helpfully" renamed or dropped the entry would have destroyed.
			line := fmt.Sprintf("%s: configuration invalid (entry %d, rule %s)",
				member.DisplayName, member.Index, member.Observation.Rule)
			if member.Observation.Detail != "" {
				line += ": " + member.Observation.Detail
			}
			lines = append(lines, line)
		case fleet.KindDisabled:
			lines = append(lines, fmt.Sprintf("%s: disabled", member.DisplayName))
		default:
			line := fmt.Sprintf("%s: %s", member.DisplayName, member.Observation.Kind)
			if member.Observation.Reason != "" {
				line += " (" + member.Observation.Reason + ")"
			}
			if member.Observation.Detail != "" {
				line += ": " + member.Observation.Detail
			}
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		return
	}
	if printedRows {
		_, _ = fmt.Fprintln(out)
	}
	for _, line := range lines {
		_, _ = fmt.Fprintln(out, line)
	}
}

// writeAligned pads every column to its widest cell. Padding is by rune count
// rather than byte count so a card title in Japanese does not shear the columns
// to its right.
func writeAligned(out io.Writer, table [][]string) {
	if len(table) == 0 {
		return
	}
	widths := make([]int, len(table[0]))
	for _, row := range table {
		for column, cell := range row {
			if width := len([]rune(cell)); width > widths[column] {
				widths[column] = width
			}
		}
	}
	for _, row := range table {
		var line strings.Builder
		for column, cell := range row {
			line.WriteString(cell)
			if column == len(row)-1 {
				continue
			}
			line.WriteString(strings.Repeat(" ", widths[column]-len([]rune(cell))+2))
		}
		_, _ = fmt.Fprintln(out, strings.TrimRight(line.String(), " "))
	}
}
