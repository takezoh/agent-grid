package runtime

import (
	"sort"
	"time"

	"github.com/takezoh/agent-grid/services/host/proto"
	"github.com/takezoh/agent-grid/services/host/state"
)

// buildResumeCapsules materializes every session's resume capsule into
// the wire format, sorted by session creation time for stable output.
// Sessions without a capsule are omitted.
func (r *Runtime) buildResumeCapsules() proto.Response {
	sorted := make([]state.Session, 0, len(r.state.Sessions))
	for _, sess := range r.state.Sessions {
		if sess.ResumeCapsule != nil {
			sorted = append(sorted, sess)
		}
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].CreatedAt.Before(sorted[j].CreatedAt)
	})
	infos := make([]proto.ResumeCapsuleInfo, 0, len(sorted))
	for _, sess := range sorted {
		capsule := sess.ResumeCapsule
		info := proto.ResumeCapsuleInfo{
			SessionID:    string(sess.ID),
			Project:      sess.Project,
			Workspace:    r.workspaceResolver.Resolve(sess.Project),
			Objective:    capsule.Objective,
			CurrentState: capsule.CurrentState,
			NextAction:   capsule.NextAction,
			Unresolved:   capsule.Unresolved,
			GeneratedAt:  capsule.GeneratedAt.UTC().Format(time.RFC3339),
			SourceTurns:  capsule.SourceTurns,
			FlaggedWrong: capsule.FlaggedWrong,
			LastError:    capsule.LastError,
		}
		if frame, ok := sessionRootFrame(sess); ok {
			if drv := state.GetDriver(frame.Command); drv != nil {
				info.Title = wireCardTitle(drv.View(frame.Driver).Card.Title)
			}
		}
		infos = append(infos, info)
	}
	return proto.RespResumeCapsules{ResumeCapsules: infos}
}
