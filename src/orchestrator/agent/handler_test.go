package agent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/takezoh/agent-roost/orchestrator/lineargql"
	"github.com/takezoh/agent-roost/platform/agent/codexclient"
	"github.com/takezoh/agent-roost/platform/agent/codexschema"
	"github.com/takezoh/agent-roost/platform/tracker"
)

// toolCallServer is a fake codex app-server that:
//  1. Replies to initialize.
//  2. On turn/start: emits thread/started + turn/started.
//  3. Sends one item/tool/call request to the orchestrator and captures the reply.
//  4. After capturing the reply, emits turn/completed to end the session.
type toolCallServer struct {
	srv      *codexclient.Server
	toolName string
	args     json.RawMessage
	reply    json.RawMessage
	replyErr string
}

func (s *toolCallServer) OnServerRequest(id int64, method string, _ json.RawMessage) {
	if method == codexschema.MethodInitialize {
		_ = s.srv.Conn().Reply(id, map[string]any{})
	}
}

func (s *toolCallServer) OnNotification(method string, _ json.RawMessage) {
	if method != codexschema.MethodTurnStart {
		return
	}
	_ = s.srv.EmitThreadStarted(testThreadID, "/ws")
	_ = s.srv.EmitTurnStarted(testThreadID, testTurnID)

	go func() {
		raw, err := s.srv.Conn().Request(codexschema.MethodItemToolCall, map[string]any{
			"tool":      s.toolName,
			"arguments": s.args,
			"callId":    "call-1",
			"threadId":  testThreadID,
			"turnId":    testTurnID,
		})
		if err != nil {
			s.replyErr = err.Error()
		} else {
			s.reply = raw
		}
		_ = s.srv.EmitTurnCompleted(testThreadID, testTurnID, "done")
	}()
}

// makeToolCallProc wires runner ↔ toolCallServer via io.Pipe, same pattern as
// makeFakeProc in runner_test.go.
func makeToolCallProc(ts *toolCallServer) procFunc {
	return func(ctx context.Context, _ string, _ map[string]string, _ string) (io.ReadCloser, io.WriteCloser, func(), error) {
		pr1, pw1 := io.Pipe()
		pr2, pw2 := io.Pipe()
		serverConn := codexclient.NewConn(codexclient.StdioTransport(pr2, pw1), 2*time.Second)
		ts.srv = codexclient.NewServer(serverConn)
		go func() {
			defer pw2.Close()
			_ = serverConn.Run(ctx, ts)
		}()
		go func() {
			<-ctx.Done()
			_ = pw1.Close()
		}()
		return pr1, pw2, func() {}, nil
	}
}

func makeLinearServer(t *testing.T, respBody string) *lineargql.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(respBody)) //nolint:errcheck
	}))
	t.Cleanup(srv.Close)
	return lineargql.New(srv.URL, "test-token")
}

func makeRunnerWithLinear(t *testing.T, lc *lineargql.Client, proc procFunc) *Runner {
	t.Helper()
	r := makeRunner(t, "", proc)
	r.LinearClient = lc
	return r
}

func testIssue() tracker.Issue {
	return tracker.Issue{Identifier: "PROJ-H1", Title: "handler test issue"}
}

func spawnAndWaitForToolReply(t *testing.T, r *Runner, ts *toolCallServer) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := r.spawnWith(ctx, testIssue(), 1, func(Event) {})
	require.NoError(t, err)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && ts.reply == nil && ts.replyErr == "" {
		time.Sleep(20 * time.Millisecond)
	}
}

func TestHandleToolCall_linearGraphql_success(t *testing.T) {
	lc := makeLinearServer(t, `{"data":{"viewer":{"id":"u1"}}}`)
	args, _ := json.Marshal(map[string]any{"query": "{ viewer { id } }", "variables": nil})
	ts := &toolCallServer{toolName: "linear_graphql", args: args}

	r := makeRunnerWithLinear(t, lc, makeToolCallProc(ts))
	spawnAndWaitForToolReply(t, r, ts)

	require.Empty(t, ts.replyErr)
	require.NotEmpty(t, ts.reply)
	var result lineargql.Result
	require.NoError(t, json.Unmarshal(ts.reply, &result))
	assert.True(t, result.Success)
	assert.JSONEq(t, `{"viewer":{"id":"u1"}}`, string(result.Data))
}

func TestHandleToolCall_linearGraphql_graphqlErrors(t *testing.T) {
	lc := makeLinearServer(t, `{"errors":[{"message":"not found"},{"message":"forbidden"}]}`)
	args, _ := json.Marshal(map[string]any{"query": "{ issue { id } }", "variables": nil})
	ts := &toolCallServer{toolName: "linear_graphql", args: args}

	r := makeRunnerWithLinear(t, lc, makeToolCallProc(ts))
	spawnAndWaitForToolReply(t, r, ts)

	require.Empty(t, ts.replyErr)
	require.NotEmpty(t, ts.reply)
	var result lineargql.Result
	require.NoError(t, json.Unmarshal(ts.reply, &result))
	assert.False(t, result.Success)
	assert.Contains(t, string(result.Errors), "not found")
	assert.Contains(t, string(result.Errors), "forbidden")
}

func TestHandleToolCall_unknownTool_replyError(t *testing.T) {
	lc := makeLinearServer(t, `{"data":{}}`)
	args, _ := json.Marshal(map[string]any{"query": "x"})
	ts := &toolCallServer{toolName: "nonexistent_tool", args: args}

	r := makeRunnerWithLinear(t, lc, makeToolCallProc(ts))
	spawnAndWaitForToolReply(t, r, ts)

	assert.NotEmpty(t, ts.replyErr, "unknown tool should return a JSON-RPC error")
}

func TestHandleToolCall_linearDisabled_replyError(t *testing.T) {
	args, _ := json.Marshal(map[string]any{"query": "{ viewer { id } }"})
	ts := &toolCallServer{toolName: "linear_graphql", args: args}

	// runner with no LinearClient — tool is disabled
	r := makeRunner(t, "", makeToolCallProc(ts))
	spawnAndWaitForToolReply(t, r, ts)

	assert.NotEmpty(t, ts.replyErr, "disabled linear_graphql should return a JSON-RPC error")
}
