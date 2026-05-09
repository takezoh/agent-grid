package state

import (
	"encoding/json"
	"fmt"
)

type CreateSessionParams struct {
	Project string          `json:"project"`
	Command string          `json:"command"`
	Sandbox SandboxOverride `json:"sandbox,omitempty"`
	Options LaunchOptions   `json:"options,omitempty"`
}

type PushDriverParams struct {
	SessionID string        `json:"session_id"`
	Project   string        `json:"project"`
	Command   string        `json:"command"`
	Options   LaunchOptions `json:"options,omitempty"`
	Input     []byte        `json:"input,omitempty"`
}

type ForkSessionParams struct {
	SessionID string `json:"session_id"`
}

type StopSessionParams struct {
	SessionID string `json:"session_id"`
}

type PreviewSessionParams struct {
	SessionID string `json:"session_id"`
}

type SwitchSessionParams struct {
	SessionID string `json:"session_id"`
}

type PreviewProjectParams struct {
	Project string `json:"project"`
}

type FocusPaneParams struct {
	Pane string `json:"pane"`
}

func init() {
	RegisterEvent[CreateSessionParams](EventCreateSession, reduceCreateSession)
	RegisterEvent[PushDriverParams](EventPushDriver, reducePushDriver)
	RegisterEvent[ForkSessionParams](EventForkSession, reduceForkSession)
	RegisterEvent[StopSessionParams](EventStopSession, reduceStopSession)
	RegisterEvent[struct{}](EventListSessions, reduceListSessions)
	RegisterEvent[PreviewSessionParams](EventPreviewSession, reducePreviewSession)
	RegisterEvent[SwitchSessionParams](EventSwitchSession, reduceSwitchSession)
	RegisterEvent[PreviewProjectParams](EventPreviewProject, reducePreviewProject)
	RegisterEvent[FocusPaneParams](EventFocusPane, reduceFocusPane)
	RegisterEvent[json.RawMessage](EventLaunchTool, reduceLaunchTool)
}

func reduceCreateSession(s State, connID ConnID, reqID string, p CreateSessionParams) (State, []Effect) { //nolint:funlen
	if p.Project == "" {
		return s, []Effect{errResp(connID, reqID, ErrCodeInvalidArgument, "project arg required")}
	}
	command := resolveCreateCommand(s, p.Command)
	sessID := allocSessionID()
	drv := GetDriver(command)
	if drv == nil {
		return s, []Effect{errResp(connID, reqID, ErrCodeUnsupported, "no driver registered for command "+command)}
	}

	driverState, setupJob, err := prepareSessionDriver(s, drv, sessID, p.Project, command, p.Options)
	if err != nil {
		return s, []Effect{errResp(connID, reqID, ErrCodeInvalidArgument, err.Error())}
	}

	rootFrameID := allocFrameID()
	session := Session{
		ID:            sessID,
		Project:       p.Project,
		CreatedAt:     s.Now,
		ActiveFrameID: rootFrameID,
		Command:       command,
		Sandbox:       p.Sandbox,
		LaunchOptions: p.Options,
		Driver:        driverState,
		Frames: []SessionFrame{{
			ID:        rootFrameID,
			Project:   p.Project,
			Command:   command,
			CreatedAt: s.Now,
			Driver:    driverState,
		}},
	}
	if setupJob != nil {
		s.NextJobID++
		jobID := s.NextJobID
		s.Jobs = cloneJobs(s.Jobs)
		s.Jobs[jobID] = JobMeta{SessionID: sessID, FrameID: rootFrameID, StartedAt: s.Now}
		s.PendingCreates = clonePendingCreates(s.PendingCreates)
		s.PendingCreates[jobID] = PendingCreate{
			Session:    session,
			FrameID:    rootFrameID,
			ReplyConn:  connID,
			ReplyReqID: reqID,
		}
		return s, []Effect{
			EffStartJob{JobID: jobID, Input: setupJob},
		}
	}

	launch, err := drv.PrepareLaunch(driverState, LaunchModeCreate, p.Project, command, p.Options, isSandboxed(s, p.Project, p.Sandbox))
	if err != nil {
		return s, []Effect{errResp(connID, reqID, ErrCodeInvalidArgument, err.Error())}
	}
	launch.Project = p.Project
	launch.Sandbox = p.Sandbox
	session.Frames[0].LaunchOptions = launch.Options

	s.Sessions = cloneSessions(s.Sessions)
	s.Sessions[sessID] = session

	return s, []Effect{spawnEffect(sessID, rootFrameID, launch, connID, reqID)}
}

func resolveCreateCommand(s State, command string) string {
	if command == "" {
		command = s.DefaultCommand
	}
	if command == "" {
		command = "shell"
	}
	if expanded, ok := s.Aliases[command]; ok {
		command = expanded
	}
	return command
}

func prepareSessionDriver(s State, drv Driver, sessID SessionID, project, command string, options LaunchOptions) (DriverState, JobInput, error) {
	driverState := drv.NewState(s.Now)
	var setupJob JobInput
	if planner, ok := drv.(CreateSessionPlanner); ok {
		var plan CreatePlan
		var err error
		driverState, plan, err = planner.PrepareCreate(driverState, sessID, project, command, options)
		if err != nil {
			return nil, nil, err
		}
		setupJob = plan.SetupJob
	}
	return driverState, setupJob, nil
}

func reducePushDriver(s State, connID ConnID, reqID string, p PushDriverParams) (State, []Effect) {
	sid := SessionID(p.SessionID)
	if sid == "" {
		return s, []Effect{errResp(connID, reqID, ErrCodeInvalidArgument, "session_id required")}
	}
	if _, ok := s.Sessions[sid]; !ok {
		return s, []Effect{errResp(connID, reqID, ErrCodeNotFound, "session not found")}
	}
	newS, effs, err := pushDriverInternal(s, sid, p.Project, p.Command, p.Options, p.Input, connID, reqID)
	if err != nil {
		return s, []Effect{errResp(connID, reqID, ErrCodeInvalidArgument, err.Error())}
	}
	return newS, effs
}

// pushDriverInternal is the shared implementation for pushing a new driver frame
// onto a session. Used by reducePushDriver (IPC) and reduceDriverHook (EffPushDriver).
func pushDriverInternal(s State, sid SessionID, project, rawCommand string, options LaunchOptions, input []byte, connID ConnID, reqID string) (State, []Effect, error) { //nolint:funlen
	sess, ok := s.Sessions[sid]
	if !ok {
		return s, nil, fmt.Errorf("session not found")
	}
	if project == "" {
		project = sess.Project
	}
	options.InitialInput = input

	command := resolveCreateCommand(s, rawCommand)
	drv := GetDriver(command)
	if drv == nil {
		return s, nil, fmt.Errorf("no driver registered for command %s", command)
	}

	driverState, setupJob, err := prepareSessionDriver(s, drv, sid, project, command, options)
	if err != nil {
		return s, nil, err
	}

	// Inherit root frame's StartDir so the child frame starts in the same directory.
	if rootF, ok := rootFrame(sess); ok {
		rootDrv := GetDriver(rootF.Command)
		if rp, ok := rootDrv.(StartDirAware); ok {
			if parentDir := rp.StartDir(rootF.Driver); parentDir != "" {
				if wp, ok := drv.(StartDirAware); ok {
					driverState = wp.WithStartDir(driverState, parentDir)
				}
			}
		}
	}

	frame := SessionFrame{
		ID:        allocFrameID(),
		Project:   project,
		Command:   command,
		CreatedAt: s.Now,
		Driver:    driverState,
	}
	sess = pushMRU(sess, sess.ActiveFrameID)
	sess.ActiveFrameID = frame.ID
	sess.Frames = append(append([]SessionFrame(nil), sess.Frames...), frame)
	s.Sessions = cloneSessions(s.Sessions)
	s.Sessions[sid] = sess

	if setupJob != nil {
		s.NextJobID++
		jobID := s.NextJobID
		s.Jobs = cloneJobs(s.Jobs)
		s.Jobs[jobID] = JobMeta{SessionID: sid, FrameID: frame.ID, StartedAt: s.Now}
		s.PendingCreates = clonePendingCreates(s.PendingCreates)
		s.PendingCreates[jobID] = PendingCreate{Session: sess, FrameID: frame.ID, ReplyConn: connID, ReplyReqID: reqID}
		return s, []Effect{EffStartJob{JobID: jobID, Input: setupJob}}, nil
	}

	launch, err := drv.PrepareLaunch(driverState, LaunchModeCreate, project, command, options, isSandboxed(s, project, sess.Sandbox))
	if err != nil {
		return s, nil, err
	}
	launch.Project = project
	launch.Sandbox = sess.Sandbox
	sess.Frames[len(sess.Frames)-1].LaunchOptions = launch.Options
	s.Sessions[sid] = sess

	return s, []Effect{spawnEffect(sid, frame.ID, launch, connID, reqID)}, nil
}

func reduceForkSession(s State, connID ConnID, reqID string, p ForkSessionParams) (State, []Effect) {
	sid := SessionID(p.SessionID)
	if sid == "" {
		return s, []Effect{errResp(connID, reqID, ErrCodeInvalidArgument, "session_id required")}
	}
	sess, ok := s.Sessions[sid]
	if !ok {
		return s, []Effect{errResp(connID, reqID, ErrCodeNotFound, "session not found")}
	}
	rootF, ok := rootFrame(sess)
	if !ok {
		return s, []Effect{errResp(connID, reqID, ErrCodeInvalidArgument, "session has no root frame")}
	}
	rootDrv, forkDrv, forkCommand, ok, errEff := resolveForkDrv(s, connID, reqID, rootF)
	if !ok {
		return s, errEff
	}
	return buildForkSession(s, connID, reqID, sess, sid, rootF, rootDrv, forkDrv, forkCommand)
}

// resolveForkDrv looks up and validates drivers for a fork operation.
// Returns (rootDrv, forkDrv, forkCommand, true) on success,
// or (nil, nil, "", false) with errEff set on failure.
func resolveForkDrv(s State, connID ConnID, reqID string, rootF SessionFrame) (Driver, Driver, string, bool, []Effect) {
	rootDrv := GetDriver(rootF.Command)
	if rootDrv == nil {
		return nil, nil, "", false, []Effect{errResp(connID, reqID, ErrCodeUnsupported, "no driver for command "+rootF.Command)}
	}
	forkable, ok := rootDrv.(Forkable)
	if !ok {
		return nil, nil, "", false, []Effect{errResp(connID, reqID, ErrCodeUnsupported, rootDrv.Name()+" driver does not support fork")}
	}
	forkCmd, ok := forkable.ForkCommand(rootF.Driver, rootF.Command)
	if !ok {
		return nil, nil, "", false, []Effect{errResp(connID, reqID, ErrCodeUnsupported, "fork not available (session ID not yet established)")}
	}
	forkCommand := resolveCreateCommand(s, forkCmd)
	forkDrv := GetDriver(forkCommand)
	if forkDrv == nil {
		return nil, nil, "", false, []Effect{errResp(connID, reqID, ErrCodeUnsupported, "no driver for fork command "+forkCommand)}
	}
	return rootDrv, forkDrv, forkCommand, true, nil
}

// buildForkSession constructs and enqueues (or directly spawns) the forked session.
// Worktree creation is deliberately skipped: the fork shares the original's working directory.
func buildForkSession(s State, connID ConnID, reqID string, sess Session, sid SessionID, rootF SessionFrame, rootDrv, forkDrv Driver, forkCommand string) (State, []Effect) {
	opts := LaunchOptions{Worktree: WorktreeOption{Enabled: false}}
	driverState, setupJob, err := prepareSessionDriver(s, forkDrv, sid, sess.Project, forkCommand, opts)
	if err != nil {
		return s, []Effect{errResp(connID, reqID, ErrCodeInvalidArgument, err.Error())}
	}
	if rp, ok := rootDrv.(StartDirAware); ok {
		if dir := rp.StartDir(rootF.Driver); dir != "" {
			if wp, ok := forkDrv.(StartDirAware); ok {
				driverState = wp.WithStartDir(driverState, dir)
			}
		}
	}

	return spawnForkSession(s, connID, reqID, sess, forkDrv, driverState, forkCommand, opts, setupJob)
}

func spawnForkSession(s State, connID ConnID, reqID string, sess Session, forkDrv Driver, driverState DriverState, forkCommand string, opts LaunchOptions, setupJob JobInput) (State, []Effect) {
	newSessID := allocSessionID()
	rootFrameID := allocFrameID()
	newSess := makeForkSession(s, sess, newSessID, rootFrameID, forkCommand, opts, driverState)
	if setupJob != nil {
		s.NextJobID++
		jobID := s.NextJobID
		s.Jobs = cloneJobs(s.Jobs)
		s.Jobs[jobID] = JobMeta{SessionID: newSessID, FrameID: rootFrameID, StartedAt: s.Now}
		s.PendingCreates = clonePendingCreates(s.PendingCreates)
		s.PendingCreates[jobID] = PendingCreate{Session: newSess, FrameID: rootFrameID, ReplyConn: connID, ReplyReqID: reqID}
		s.Sessions = cloneSessions(s.Sessions)
		s.Sessions[newSessID] = newSess
		return s, []Effect{EffStartJob{JobID: jobID, Input: setupJob}}
	}
	launch, err := forkDrv.PrepareLaunch(driverState, LaunchModeCreate, sess.Project, forkCommand, opts, isSandboxed(s, sess.Project, sess.Sandbox))
	if err != nil {
		return s, []Effect{errResp(connID, reqID, ErrCodeInvalidArgument, err.Error())}
	}
	launch.Project = sess.Project
	launch.Sandbox = sess.Sandbox
	newSess.Frames[0].LaunchOptions = launch.Options
	s.Sessions = cloneSessions(s.Sessions)
	s.Sessions[newSessID] = newSess
	return s, []Effect{spawnEffect(newSessID, rootFrameID, launch, connID, reqID)}
}

// makeForkSession initialises a new Session value for a fork operation.
func makeForkSession(s State, src Session, newSessID SessionID, rootFrameID FrameID, forkCommand string, opts LaunchOptions, driverState DriverState) Session {
	return Session{
		ID:            newSessID,
		Project:       src.Project,
		CreatedAt:     s.Now,
		ActiveFrameID: rootFrameID,
		Command:       forkCommand,
		Sandbox:       src.Sandbox,
		LaunchOptions: opts,
		Driver:        driverState,
		Frames: []SessionFrame{{
			ID:        rootFrameID,
			Project:   src.Project,
			Command:   forkCommand,
			CreatedAt: s.Now,
			Driver:    driverState,
		}},
	}
}

// worktreePathReferenced reports whether any session other than exceptSession
// has a frame whose ManagedWorktreePath matches path. Used by reduceStopSession
// to avoid deleting a worktree that is still referenced by another session.
func worktreePathReferenced(s State, path string, exceptSession SessionID) bool {
	for sid, sess := range s.Sessions {
		if sid == exceptSession {
			continue
		}
		for _, f := range sess.Frames {
			drv := GetDriver(f.Command)
			if provider, ok := drv.(ManagedWorktreeProvider); ok {
				if provider.ManagedWorktreePath(f.Driver) == path {
					return true
				}
			}
		}
	}
	return false
}

func reduceStopSession(s State, connID ConnID, reqID string, p StopSessionParams) (State, []Effect) {
	sid := SessionID(p.SessionID)
	sess, ok := s.Sessions[sid]
	if !ok {
		return s, []Effect{errResp(connID, reqID, ErrCodeNotFound, "session not found")}
	}
	_, removed := truncateFrames(sess, 0)
	s.Sessions = cloneSessions(s.Sessions)
	delete(s.Sessions, sid)
	var deactivate []Effect
	if s.ActiveSession == sid {
		s.ActiveSession = ""
		if s.ActiveOccupant == OccupantFrame {
			s.ActiveOccupant = OccupantMain
			deactivate = []Effect{EffDeactivateSession{}}
		}
	}
	// place broadcast first — TUI updates before tmux kill completes
	effs := []Effect{EffBroadcastSessionsChanged{}}
	effs = append(effs, deactivate...)
	for _, frame := range removed {
		effs = append(effs,
			EffKillSessionWindow{FrameID: frame.ID},
			EffUnregisterPane{FrameID: frame.ID},
			EffUnwatchFile{FrameID: frame.ID},
		)
	}
	if path := sessionManagedWorktreePath(removed); path != "" {
		if !worktreePathReferenced(s, path, sid) {
			effs = append(effs, EffRemoveManagedWorktree{Path: path})
		}
	}
	effs = append(effs, okResp(connID, reqID, nil), EffPersistSnapshot{})
	return s, effs
}

func sessionManagedWorktreePath(frames []SessionFrame) string {
	for _, frame := range frames {
		drv := GetDriver(frame.Command)
		if provider, ok := drv.(ManagedWorktreeProvider); ok {
			if path := provider.ManagedWorktreePath(frame.Driver); path != "" {
				return path
			}
		}
	}
	return ""
}
