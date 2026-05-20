package agent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/takezoh/agent-roost/orchestrator/prompt"
	"github.com/takezoh/agent-roost/orchestrator/scheduler"
	"github.com/takezoh/agent-roost/platform/agent/codexclient"
	"github.com/takezoh/agent-roost/platform/tracker"
)

const spawnTimeout = 30 * time.Second

type launchResult struct {
	conn         *codexclient.Conn
	sessionReady <-chan sessionIDs
	turnDone     <-chan turnResult
	doneCh       <-chan struct{}
}

func (r *Runner) spawnWith(ctx context.Context, issue tracker.Issue, attempt int, emit func(Event)) (scheduler.LiveSession, error) {
	wsPath, err := r.prepareWorkspace(ctx, issue.Identifier)
	if err != nil {
		return scheduler.LiveSession{}, err
	}

	rendered, err := prompt.Render(r.PromptTemplate, prompt.Vars{Issue: issue, Attempt: attempt})
	if err != nil {
		return scheduler.LiveSession{}, err
	}

	workerCtx, cancel := context.WithCancel(ctx)
	lr, err := r.launchConn(workerCtx, wsPath)
	if err != nil {
		cancel()
		return scheduler.LiveSession{}, err
	}

	ids, err := initSession(lr.conn, wsPath, rendered, lr.sessionReady, lr.doneCh)
	if err != nil {
		cancel()
		<-lr.doneCh
		return scheduler.LiveSession{}, err
	}

	worker := &Worker{cancel: cancel, done: lr.doneCh}
	go r.runMonitor(issue.Identifier, ids, lr.turnDone, lr.doneCh, emit)

	emit(Event{
		Kind:      EventSessionStarted,
		SessionID: ids.sessionID(),
		ThreadID:  ids.threadID,
		TurnID:    ids.turnID,
		Timestamp: time.Now(),
	})

	return scheduler.LiveSession{
		SessionID: ids.sessionID(),
		ThreadID:  ids.threadID,
		TurnID:    ids.turnID,
		StartedAt: time.Now(),
		Worker:    worker,
	}, nil
}

func (r *Runner) prepareWorkspace(ctx context.Context, identifier string) (string, error) {
	wsPath, err := r.Workspace.Ensure(ctx, identifier)
	if err != nil {
		return "", fmt.Errorf("agent: workspace ensure: %w", err)
	}
	if err := r.Workspace.VerifyCWD(identifier, wsPath); err != nil {
		return "", fmt.Errorf("agent: verify cwd: %w", err)
	}
	if err := r.Workspace.BeforeRun(ctx, identifier); err != nil {
		return "", fmt.Errorf("agent: before run: %w", err)
	}
	return wsPath, nil
}

func (r *Runner) launchConn(ctx context.Context, wsPath string) (*launchResult, error) {
	stdout, stdin, err := r.proc(ctx, wsPath, r.Cfg.Codex.Command)
	if err != nil {
		return nil, err
	}

	readTimeout := time.Duration(r.Cfg.Codex.ReadTimeoutMS) * time.Millisecond
	tr := codexclient.StdioTransport(stdout, stdin)
	conn := codexclient.NewConn(tr, readTimeout)

	sessionReady := make(chan sessionIDs, 1)
	turnDone := make(chan turnResult, 1)
	h := &turnHandler{
		conn:         conn,
		sessionReady: sessionReady,
		turnDone:     turnDone,
	}

	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		_ = conn.Run(ctx, h)
	}()

	return &launchResult{
		conn:         conn,
		sessionReady: sessionReady,
		turnDone:     turnDone,
		doneCh:       doneCh,
	}, nil
}

func initSession(conn *codexclient.Conn, wsPath, rendered string, sessionReady <-chan sessionIDs, doneCh <-chan struct{}) (sessionIDs, error) {
	if err := codexclient.Initialize(conn); err != nil {
		return sessionIDs{}, fmt.Errorf("agent: initialize: %w", err)
	}
	if err := codexclient.StartTurn(conn, "", wsPath, []byte(rendered)); err != nil {
		return sessionIDs{}, fmt.Errorf("agent: start turn: %w", err)
	}

	timer := time.NewTimer(spawnTimeout)
	defer timer.Stop()

	select {
	case ids := <-sessionReady:
		return ids, nil
	case <-timer.C:
		return sessionIDs{}, errors.New("agent: timeout waiting for session start")
	case <-doneCh:
		return sessionIDs{}, errors.New("agent: codex exited before session started")
	}
}

func (r *Runner) runMonitor(identifier string, ids sessionIDs, turnDone <-chan turnResult, doneCh <-chan struct{}, emit func(Event)) {
	var result turnResult
	select {
	case result = <-turnDone:
	case <-doneCh:
		select {
		case result = <-turnDone:
		default:
			result = turnResult{failed: true, err: errors.New("codex process exited unexpectedly")}
		}
	}

	if result.failed {
		emit(Event{
			Kind:      EventTurnFailed,
			SessionID: ids.sessionID(),
			ThreadID:  ids.threadID,
			TurnID:    ids.turnID,
			Timestamp: time.Now(),
			Err:       result.err,
		})
	} else {
		emit(Event{
			Kind:      EventTurnCompleted,
			SessionID: ids.sessionID(),
			ThreadID:  ids.threadID,
			TurnID:    ids.turnID,
			Timestamp: time.Now(),
		})
	}
	r.Workspace.AfterRun(context.Background(), identifier)
}
