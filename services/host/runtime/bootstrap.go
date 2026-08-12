package runtime

import (
	"log/slog"
	"time"

	"github.com/takezoh/agent-grid/services/host/state"
)

// Bootstrap helpers used at startup before the event loop starts.
// These mutate r.state directly — safe because no goroutine is reading
// state yet.

// LoadSnapshot reads sessions.json and registers each session in
// r.state. Driver state is restored via the registered Driver's
// Restore method.
//
// On cold start the frame backend is brand new — any dead frames from
// the previous daemon run have evaporated along with the old server.
// Frames whose driver was at status=stopped have nothing left to
// inspect (the tail output lived in the dead frame, which is gone)
// and cannot be respawned without overwriting the very state the
// user wanted to keep. Those frames are dropped from the current in-memory
// view, but their snapshots remain on disk: only an explicit UI stop may
// delete a durable session. The exception is a driver
// whose durable state outlives the frame (a ColdStartRecoverer such as
// codex, which resumes its thread): its stopped frames are kept and
// relaunched.
//
// Warm start keeps everything: stopped frames in a live frame backend
// still have their dead pty sessions attached for inspection.
func (r *Runtime) LoadSnapshot(coldStart bool) error {
	snaps, err := r.cfg.Persist.Load()
	if err != nil {
		return err
	}
	now := time.Now()
	dropped := 0
	for _, snap := range snaps {
		sess, ok := restoreSession(snap, coldStart, now)
		if !ok {
			if coldStart {
				dropped++
				slog.Info("bootstrap: retained unrecoverable snapshot outside current view",
					"id", snap.ID, "frames", len(snap.Frames))
			}
			continue
		}
		r.state.Sessions[sess.ID] = sess
	}
	r.restoreFrameMessagingStores()
	slog.Info("bootstrap: snapshot loaded", "count", len(snaps)-dropped, "dropped_stopped", dropped)
	return nil
}

func restoreSession(snap SessionSnapshot, coldStart bool, now time.Time) (state.Session, bool) {
	createdAt, _ := time.Parse(time.RFC3339, snap.CreatedAt)
	if createdAt.IsZero() {
		createdAt = now
	}
	sess := state.Session{ID: state.SessionID(snap.ID), Project: snap.Project, CreatedAt: createdAt}
	for _, fsnap := range snap.Frames {
		drv := state.GetDriver(fsnap.Command)
		if drv == nil {
			slog.Warn("bootstrap: no driver for command, skipping frame", "command", fsnap.Command)
			break
		}
		if coldStart && fsnap.DriverState["status"] == "running" {
			fsnap.DriverState["status"] = "waiting"
		}
		driverState := drv.Restore(fsnap.DriverState, now)
		// Cold start has no live frame to inherit the dead command's
		// tail output, so a stopped frame turns into a zombie (no
		// surface to display, no session to close). Drop it here instead
		// — unless the driver declares the frame should survive a cold
		// start. Codex does this because a runtime restart can persist a
		// stopped status before its app-server locator is available; deleting
		// that snapshot would discard the user's session entry.
		if coldStart && drv.Status(driverState) == state.StatusStopped && !coldStartRecoverable(drv, driverState) {
			slog.Info("bootstrap: dropping stopped frame on cold start (driver declared unrecoverable)",
				"session", snap.ID,
				"frame", fsnap.ID,
				"command", fsnap.Command,
				"thread_id", fsnap.DriverState["thread_id"],
				"session_id", fsnap.DriverState["session_id"],
				"rollout_path", fsnap.DriverState["rollout_path"],
				"transcript_path", fsnap.DriverState["transcript_path"],
			)
			continue
		}
		frameCreatedAt, _ := time.Parse(time.RFC3339, fsnap.CreatedAt)
		if frameCreatedAt.IsZero() {
			frameCreatedAt = createdAt
		}
		subsystemID := fsnap.SubsystemID
		if subsystemID == "" {
			subsystemID = fsnap.ID
		}
		targetID := fsnap.TargetID
		if targetID == "" {
			targetID = fsnap.ID
		}
		sess.Frames = append(sess.Frames, state.SessionFrame{
			ID:            state.FrameID(fsnap.ID),
			SubsystemID:   state.SubsystemID(subsystemID),
			TargetID:      state.TargetID(targetID),
			Project:       fsnap.Project,
			Command:       fsnap.Command,
			LaunchOptions: fsnap.LaunchOptions,
			CreatedAt:     frameCreatedAt,
			Driver:        driverState,
		})
	}
	if len(sess.Frames) == 0 {
		return state.Session{}, false
	}
	sess.Command = sess.Frames[0].Command
	sess.LaunchOptions = sess.Frames[0].LaunchOptions
	sess.Driver = sess.Frames[0].Driver
	if snap.HeadFrameID != "" {
		sess.HeadFrameID = state.FrameID(snap.HeadFrameID)
	} else {
		sess.HeadFrameID = sess.Frames[len(sess.Frames)-1].ID
	}
	mru := make([]state.FrameID, 0, len(snap.MRUFrameIDs))
	for _, id := range snap.MRUFrameIDs {
		mru = append(mru, state.FrameID(id))
	}
	sess.MRUFrameIDs = mru
	sess.FrameGeneration = snap.FrameGeneration
	sess.FrameMessaging = restoreFrameMessaging(snap.FrameMessaging, liveFrameSet(sess.Frames))
	sess.ResumeCapsule = restoreResumeCapsule(snap.ResumeCapsule)
	return sess, true
}

func snapshotResumeCapsule(in *state.ResumeCapsule) *ResumeCapsuleSnapshot {
	if in == nil {
		return nil
	}
	out := &ResumeCapsuleSnapshot{
		Objective:    in.Objective,
		CurrentState: in.CurrentState,
		NextAction:   in.NextAction,
		Unresolved:   in.Unresolved,
		Changed:      in.Changed,
		Verified:     in.Verified,
		Remaining:    in.Remaining,
		Attention:    in.Attention,
		GeneratedAt:  in.GeneratedAt.UTC().Format(time.RFC3339),
		SourceDigest: in.SourceDigest,
		SourceTurns:  in.SourceTurns,
		FlaggedWrong: in.FlaggedWrong,
		LastError:    in.LastError,
	}
	if !in.FlaggedAt.IsZero() {
		out.FlaggedAt = in.FlaggedAt.UTC().Format(time.RFC3339)
	}
	if !in.LastErrorAt.IsZero() {
		out.LastErrorAt = in.LastErrorAt.UTC().Format(time.RFC3339)
	}
	return out
}

func restoreResumeCapsule(in *ResumeCapsuleSnapshot) *state.ResumeCapsule {
	if in == nil {
		return nil
	}
	generatedAt, err := time.Parse(time.RFC3339, in.GeneratedAt)
	if err != nil || generatedAt.IsZero() {
		// A capsule without a valid generation timestamp cannot honour the
		// staleness contract; drop it rather than display it as current.
		return nil
	}
	out := &state.ResumeCapsule{
		Objective:    in.Objective,
		CurrentState: in.CurrentState,
		NextAction:   in.NextAction,
		Unresolved:   in.Unresolved,
		Changed:      in.Changed,
		Verified:     in.Verified,
		Remaining:    in.Remaining,
		Attention:    in.Attention,
		GeneratedAt:  generatedAt,
		SourceDigest: in.SourceDigest,
		SourceTurns:  in.SourceTurns,
		FlaggedWrong: in.FlaggedWrong,
		LastError:    in.LastError,
	}
	if t, err := time.Parse(time.RFC3339, in.FlaggedAt); err == nil {
		out.FlaggedAt = t
	}
	if t, err := time.Parse(time.RFC3339, in.LastErrorAt); err == nil {
		out.LastErrorAt = t
	}
	return out
}

func restoreFrameMessaging(in *SessionFrameMessagingSnapshot, liveFrames map[state.FrameID]struct{}) *state.SessionFrameMessaging {
	if in == nil {
		return nil
	}
	out := &state.SessionFrameMessaging{
		Summary: state.FrameMessagingSummary{
			UnreadCount:          in.Summary.UnreadCount,
			LatestMessagePreview: in.Summary.LatestMessagePreview,
			LatestReplyPreview:   in.Summary.LatestReplyPreview,
			PendingDeliveryCount: in.Summary.PendingDeliveryCount,
			LastDeliveryStatus:   in.Summary.LastDeliveryStatus,
		},
		Messages: make([]state.FrameMessage, 0, len(in.Messages)),
	}
	for _, msg := range in.Messages {
		createdAt, _ := time.Parse(time.RFC3339, msg.CreatedAt)
		frameMsg := state.FrameMessage{
			ID:             msg.ID,
			SourceFrameID:  state.FrameID(msg.SourceFrameID),
			TargetFrameID:  state.FrameID(msg.TargetFrameID),
			Topic:          msg.Topic,
			Body:           msg.Body,
			Priority:       msg.Priority,
			CreatedAt:      createdAt,
			Read:           msg.Read,
			ReplyStatus:    msg.ReplyStatus,
			DeliveryStatus: msg.DeliveryStatus,
		}
		if msg.Reply != nil {
			replyCreatedAt, _ := time.Parse(time.RFC3339, msg.Reply.CreatedAt)
			frameMsg.Reply = &state.FrameReply{
				ID:                 msg.Reply.ID,
				SourceFrameID:      state.FrameID(msg.Reply.SourceFrameID),
				Body:               msg.Reply.Body,
				FinalAnswer:        msg.Reply.FinalAnswer,
				CreatedAt:          replyCreatedAt,
				Resolution:         msg.Reply.Resolution,
				Confidence:         msg.Reply.Confidence,
				FinalAnswerPreview: msg.Reply.FinalAnswerPreview,
			}
		}
		out.Messages = append(out.Messages, frameMsg)
	}
	return sanitizeFrameMessaging(out, liveFrames)
}

// PtyBackend pairs one pty session per frame so a main-frame-owner concept
// does not exist; cross-daemon-restart frame recovery is out of scope per
// ADR 0004 decision 2 (daemon and frames share a lifetime).

func (r *Runtime) RecoverWarmStartSessions() {
	now := time.Now()
	changed := false
	for sessID, sess := range r.state.Sessions {
		for i, frame := range sess.Frames {
			drv := state.GetDriver(frame.Command)
			if drv == nil {
				continue
			}
			recoverer, ok := drv.(state.WarmStartRecoverer)
			if !ok {
				continue
			}
			next, effs := recoverer.WarmStartRecover(frame.Driver, now)
			sess.Frames[i].Driver = next
			r.state.Sessions[sessID] = sess
			for _, eff := range effs {
				r.execute(r.bootstrapSessionEffect(sessID, frame.ID, now, eff))
			}
			changed = true
		}
	}
	if changed {
		if err := r.cfg.Persist.Save(r.snapshotSessions()); err != nil {
			slog.Error("bootstrap: persist after warm start recovery failed", "err", err)
		}
	}
}

func (r *Runtime) bootstrapSessionEffect(sessID state.SessionID, frameID state.FrameID, now time.Time, eff state.Effect) state.Effect {
	switch e := eff.(type) {
	case state.EffStartJob:
		r.state.NextJobID++
		jobID := r.state.NextJobID
		r.state.Jobs[jobID] = state.JobMeta{
			SessionID: sessID,
			FrameID:   frameID,
			StartedAt: now,
		}
		e.JobID = jobID
		return e
	case state.EffEventLogAppend:
		if e.FrameID == "" {
			e.FrameID = frameID
		}
		return e
	case state.EffWatchFile:
		if e.FrameID == "" {
			e.FrameID = frameID
		}
		return e
	case state.EffUnwatchFile:
		if e.FrameID == "" {
			e.FrameID = frameID
		}
		return e
	default:
		return eff
	}
}

// Daemon shutdown closes the IPC socket and tears down frames via
// PtyBackend without any layout fix-up.

// SetAliases sets the command alias map on state. Called once at
// startup from main.go with the config's [session] aliases.
func (r *Runtime) SetAliases(aliases map[string]string) {
	r.state.Aliases = aliases
}

// SetDefaultCommand sets the fallback command for sessions created
// without an explicit command. Called once at startup from main.go.
func (r *Runtime) SetDefaultCommand(cmd string) {
	r.state.DefaultCommand = cmd
}

// SetSyncCallbacks installs the optional frame backend sync callbacks.
// Kept as a stable hook for future use.
func (r *Runtime) SetSyncCallbacks(active, status func(string)) {
	// Reserved.
}
