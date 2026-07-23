package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/takezoh/agent-grid/host/proto"
)

type mcpEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *mcpRPCError    `json:"error,omitempty"`
}

type mcpRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type mcpTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type mcpToolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type mcpTextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type mcpToolResult struct {
	Content           []mcpTextContent `json:"content"`
	StructuredContent any              `json:"structuredContent,omitempty"`
	IsError           bool             `json:"isError,omitempty"`
}

type mcpSendArgs struct {
	TargetFrameID string `json:"targetFrameId"`
	Topic         string `json:"topic,omitempty"`
	Body          string `json:"body"`
	Priority      string `json:"priority,omitempty"`
}

type mcpReadArgs struct {
	PeerFrameID string `json:"peerFrameId,omitempty"`
}

type mcpReplyArgs struct {
	MessageID   string `json:"messageId"`
	Body        string `json:"body,omitempty"`
	FinalAnswer string `json:"finalAnswer,omitempty"`
	Resolution  string `json:"resolution,omitempty"`
	Confidence  string `json:"confidence,omitempty"`
}

func runAgentFramesMCP(args []string) error {
	fs := flag.NewFlagSet("agent-frames-mcp", flag.ContinueOnError)
	sock := fs.String("sock", "", "container endpoint socket path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *sock == "" {
		return fmt.Errorf("usage: agent-frames-mcp --sock <path>")
	}
	token := os.Getenv("AG_SOCKET_TOKEN")
	if token == "" {
		return errors.New("AG_SOCKET_TOKEN is required")
	}
	client, err := proto.Dial(*sock)
	if err != nil {
		return err
	}
	defer client.Close()

	stream := mcpStdioStream{
		r: bufio.NewReader(os.Stdin),
		w: bufio.NewWriter(os.Stdout),
	}
	for {
		msg, err := stream.Read()
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) {
				return nil
			}
			return err
		}
		resp := handleMCPMessage(client, token, msg)
		if resp == nil {
			continue
		}
		if err := stream.Write(*resp); err != nil {
			return err
		}
	}
}

type mcpStdioStream struct {
	r *bufio.Reader
	w *bufio.Writer
}

func (s mcpStdioStream) Read() (mcpEnvelope, error) {
	length, err := readMCPContentLength(s.r)
	if err != nil {
		return mcpEnvelope{}, err
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(s.r, body); err != nil {
		return mcpEnvelope{}, err
	}
	var msg mcpEnvelope
	if err := json.Unmarshal(body, &msg); err != nil {
		return mcpEnvelope{}, err
	}
	return msg, nil
}

func (s mcpStdioStream) Write(msg mcpEnvelope) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(s.w, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	if _, err := s.w.Write(body); err != nil {
		return err
	}
	return s.w.Flush()
}

func readMCPContentLength(r *bufio.Reader) (int, error) {
	length := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return 0, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if length < 0 {
				return 0, errors.New("mcp stdio: missing Content-Length header")
			}
			return length, nil
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			return 0, fmt.Errorf("mcp stdio: malformed header %q", line)
		}
		if !strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || n < 0 {
			return 0, fmt.Errorf("mcp stdio: invalid Content-Length %q", value)
		}
		length = n
	}
}

func handleMCPMessage(client *proto.Client, token string, msg mcpEnvelope) *mcpEnvelope {
	switch msg.Method {
	case "initialize":
		return &mcpEnvelope{
			JSONRPC: "2.0",
			ID:      msg.ID,
			Result: map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "agent_frames", "version": "1.0.0"},
			},
		}
	case "notifications/initialized":
		return nil
	case "tools/list":
		return &mcpEnvelope{
			JSONRPC: "2.0",
			ID:      msg.ID,
			Result: map[string]any{
				"tools": []mcpTool{
					{
						Name:        "agent_frames.list",
						Description: "List same-session claude/codex frames visible to the caller.",
						InputSchema: map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{}},
					},
					{
						Name:        "agent_frames.read",
						Description: "Read durable inbox messages for the caller. Optionally filter to one peer frame.",
						InputSchema: map[string]any{
							"type":                 "object",
							"additionalProperties": false,
							"properties": map[string]any{
								"peerFrameId": map[string]any{"type": "string"},
							},
						},
					},
					{
						Name:        "agent_frames.send_message",
						Description: "Store a durable inbox message for a same-session claude/codex frame.",
						InputSchema: map[string]any{
							"type":                 "object",
							"additionalProperties": false,
							"required":             []string{"targetFrameId", "body"},
							"properties": map[string]any{
								"targetFrameId": map[string]any{"type": "string"},
								"topic":         map[string]any{"type": "string"},
								"body":          map[string]any{"type": "string"},
								"priority":      map[string]any{"type": "string"},
							},
						},
					},
					{
						Name:        "agent_frames.reply",
						Description: "Reply to a durable frame message as the target frame.",
						InputSchema: map[string]any{
							"type":                 "object",
							"additionalProperties": false,
							"required":             []string{"messageId"},
							"properties": map[string]any{
								"messageId":   map[string]any{"type": "string"},
								"body":        map[string]any{"type": "string"},
								"finalAnswer": map[string]any{"type": "string"},
								"resolution":  map[string]any{"type": "string"},
								"confidence":  map[string]any{"type": "string"},
							},
						},
					},
				},
			},
		}
	case "tools/call":
		return handleMCPToolCall(client, token, msg)
	default:
		return &mcpEnvelope{JSONRPC: "2.0", ID: msg.ID, Error: &mcpRPCError{Code: -32601, Message: "method not found"}}
	}
}

func handleMCPToolCall(client *proto.Client, token string, msg mcpEnvelope) *mcpEnvelope {
	var params mcpToolCallParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return &mcpEnvelope{JSONRPC: "2.0", ID: msg.ID, Error: &mcpRPCError{Code: -32602, Message: "invalid params"}}
	}

	var (
		resp proto.Response
		err  error
	)
	switch params.Name {
	case "agent_frames.list":
		var args struct{}
		if err := decodeStrictJSONMCP(params.Arguments, &args); err != nil {
			return invalidToolArgs(msg.ID, err)
		}
		resp, err = client.Send(context.Background(), proto.CmdFrameList{Token: token})
	case "agent_frames.read":
		var args mcpReadArgs
		if err := decodeStrictJSONMCP(params.Arguments, &args); err != nil {
			return invalidToolArgs(msg.ID, err)
		}
		resp, err = client.Send(context.Background(), proto.CmdFrameRead{Token: token, PeerFrameID: args.PeerFrameID})
	case "agent_frames.send_message":
		var args mcpSendArgs
		if err := decodeStrictJSONMCP(params.Arguments, &args); err != nil {
			return invalidToolArgs(msg.ID, err)
		}
		resp, err = client.Send(context.Background(), proto.CmdFrameSend{
			Token:         token,
			TargetFrameID: args.TargetFrameID,
			Topic:         args.Topic,
			Body:          args.Body,
			Priority:      args.Priority,
		})
	case "agent_frames.reply":
		var args mcpReplyArgs
		if err := decodeStrictJSONMCP(params.Arguments, &args); err != nil {
			return invalidToolArgs(msg.ID, err)
		}
		resp, err = client.Send(context.Background(), proto.CmdFrameReply{
			Token:       token,
			MessageID:   args.MessageID,
			Body:        args.Body,
			FinalAnswer: args.FinalAnswer,
			Resolution:  args.Resolution,
			Confidence:  args.Confidence,
		})
	default:
		return &mcpEnvelope{JSONRPC: "2.0", ID: msg.ID, Error: &mcpRPCError{Code: -32601, Message: "tool not found"}}
	}
	if err != nil {
		return toolErrorEnvelope(msg.ID, err)
	}
	return toolSuccessEnvelope(msg.ID, resp)
}

func decodeStrictJSONMCP(raw json.RawMessage, dst any) error {
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func invalidToolArgs(id json.RawMessage, err error) *mcpEnvelope {
	return &mcpEnvelope{JSONRPC: "2.0", ID: id, Error: &mcpRPCError{Code: -32602, Message: err.Error()}}
}

func toolErrorEnvelope(id json.RawMessage, err error) *mcpEnvelope {
	return &mcpEnvelope{
		JSONRPC: "2.0",
		ID:      id,
		Result: mcpToolResult{
			Content: []mcpTextContent{{Type: "text", Text: err.Error()}},
			IsError: true,
		},
	}
}

func toolSuccessEnvelope(id json.RawMessage, resp proto.Response) *mcpEnvelope {
	structured, text := formatMCPToolResult(resp)
	return &mcpEnvelope{
		JSONRPC: "2.0",
		ID:      id,
		Result: mcpToolResult{
			Content:           []mcpTextContent{{Type: "text", Text: text}},
			StructuredContent: structured,
		},
	}
}

func formatMCPToolResult(resp proto.Response) (any, string) {
	switch v := resp.(type) {
	case proto.RespFrameList:
		return v, fmt.Sprintf("listed %d frame(s)", len(v.Frames))
	case proto.RespFrameRead:
		return v, fmt.Sprintf("read %d message(s)", len(v.Messages))
	case proto.RespFrameSend:
		return v, fmt.Sprintf("sent message %s", v.Message.ID)
	case proto.RespFrameReply:
		return v, fmt.Sprintf("recorded reply %s", v.Reply.ID)
	default:
		return resp, "ok"
	}
}
