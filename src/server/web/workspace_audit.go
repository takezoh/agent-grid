package web

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"

	"github.com/takezoh/agent-grid/client/proto"
	"github.com/takezoh/agent-grid/client/runtime"
	"github.com/takezoh/agent-grid/client/state"
)

type workspaceOperatorAuditor interface {
	EmitOperatorWrite(sessionID, relPath string) error
}

type workspaceOperatorAuditorNoop struct{}

func (workspaceOperatorAuditorNoop) EmitOperatorWrite(string, string) error { return nil }

var (
	workspaceOperatorAuditorMu   sync.RWMutex
	workspaceOperatorAuditorHook workspaceOperatorAuditor = workspaceOperatorAuditorNoop{}
)

func emitOperatorWriteAudit(sessionID, relPath string) error {
	workspaceOperatorAuditorMu.RLock()
	hook := workspaceOperatorAuditorHook
	workspaceOperatorAuditorMu.RUnlock()
	return hook.EmitOperatorWrite(sessionID, relPath)
}

// SetWorkspaceOperatorAuditor swaps the operator audit hook (tests and wiring).
func SetWorkspaceOperatorAuditor(a workspaceOperatorAuditor) {
	workspaceOperatorAuditorMu.Lock()
	defer workspaceOperatorAuditorMu.Unlock()
	if a == nil {
		workspaceOperatorAuditorHook = workspaceOperatorAuditorNoop{}
		return
	}
	workspaceOperatorAuditorHook = a
}

type fileToolLogOperatorAuditor struct {
	log       runtime.ToolLogBackend
	namespace string
	project   string
}

// NewFileToolLogOperatorAuditor returns a production auditor that appends
// operator write records to the shared tool-log backend. namespace and project
// identify the JSONL file (<dataDir>/<namespace>/tool-logs/<project>.jsonl).
func NewFileToolLogOperatorAuditor(log runtime.ToolLogBackend, namespace, project string) *fileToolLogOperatorAuditor {
	return &fileToolLogOperatorAuditor{
		log:       log,
		namespace: namespace,
		project:   project,
	}
}

func (a *fileToolLogOperatorAuditor) EmitOperatorWrite(sessionID, relPath string) error {
	line, err := runtime.FormatOperatorWriteLine(sessionID, relPath)
	if err != nil {
		return err
	}
	return a.log.Append(a.namespace, a.project, line)
}

// NewFileToolLogOperatorAuditorForSession builds an auditor from session metadata.
func NewFileToolLogOperatorAuditorForSession(
	log runtime.ToolLogBackend,
	command, rootDriver, project string,
) *fileToolLogOperatorAuditor {
	return NewFileToolLogOperatorAuditor(
		log,
		toolLogNamespaceFromCommand(command, rootDriver),
		toolLogProjectSlug(project),
	)
}

type daemonToolLogOperatorAuditor struct {
	daemon *DaemonClient
	log    runtime.ToolLogBackend
}

func newDaemonToolLogOperatorAuditor(d *DaemonClient, log runtime.ToolLogBackend) *daemonToolLogOperatorAuditor {
	return &daemonToolLogOperatorAuditor{daemon: d, log: log}
}

func (a *daemonToolLogOperatorAuditor) EmitOperatorWrite(sessionID, relPath string) error {
	meta, err := a.lookupSession(sessionID)
	if err != nil {
		return err
	}
	auditor := NewFileToolLogOperatorAuditorForSession(a.log, meta.Command, meta.RootDriver, meta.Project)
	return auditor.EmitOperatorWrite(sessionID, relPath)
}

func (a *daemonToolLogOperatorAuditor) lookupSession(sessionID string) (proto.SessionInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), daemonRPCTimeout)
	defer cancel()
	resp, err := a.daemon.SendCommand(ctx, proto.CmdEvent{
		Event:   state.EventListSessions,
		Payload: json.RawMessage("{}"),
	})
	if err != nil {
		return proto.SessionInfo{}, err
	}
	rs, ok := resp.(proto.RespSessions)
	if !ok {
		return proto.SessionInfo{}, errors.New("unexpected response type")
	}
	for i := range rs.Sessions {
		if rs.Sessions[i].ID == sessionID {
			return rs.Sessions[i], nil
		}
	}
	return proto.SessionInfo{}, errSessionNotFound
}

// InitWorkspaceOperatorAuditor wires production operator-write audit emission
// using the shared on-disk tool log under dataDir. Safe to call once at gateway
// startup; tests may override via SetWorkspaceOperatorAuditor.
func InitWorkspaceOperatorAuditor(d *DaemonClient, dataDir string) {
	SetWorkspaceOperatorAuditor(newDaemonToolLogOperatorAuditor(d, runtime.NewFileToolLog(dataDir)))
}

// toolLogNamespaceFromCommand maps a session command/root_driver to the
// tool-log namespace slug (claude, codex, etc.).
func toolLogNamespaceFromCommand(command, rootDriver string) string {
	if ns := strings.TrimSpace(rootDriver); ns != "" {
		return ns
	}
	cmd := strings.ToLower(strings.TrimSpace(command))
	if strings.Contains(cmd, "codex") {
		return "codex"
	}
	if strings.Contains(cmd, "gemini") {
		return "gemini"
	}
	return "claude"
}

// toolLogProjectSlug converts an absolute project path to the tool-log
// filename slug (mirrors driver.projectDir).
func toolLogProjectSlug(project string) string {
	project = strings.TrimSpace(project)
	if project == "" {
		return ""
	}
	return strings.NewReplacer("/", "-", ".", "-").Replace(project)
}
