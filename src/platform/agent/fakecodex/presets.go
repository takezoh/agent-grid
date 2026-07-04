package fakecodex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// DefaultTurnHandler completes the turn immediately with the text "done".
// Used when Config.Handler is nil.
func DefaultTurnHandler(_ context.Context, _ TurnRequest, _ TurnEmitter) (string, error) {
	return "done", nil
}

// FailingTurnHandler returns a TurnHandler that always fails with the given message.
func FailingTurnHandler(msg string) TurnHandler {
	return func(_ context.Context, _ TurnRequest, _ TurnEmitter) (string, error) {
		return "", errors.New(msg)
	}
}

// TextTurnHandler streams a single agentMessage delta then completes.
func TextTurnHandler(delta, completedText string) TurnHandler {
	return func(_ context.Context, _ TurnRequest, e TurnEmitter) (string, error) {
		if err := e.AgentDelta(delta); err != nil {
			return "", fmt.Errorf("emit delta: %w", err)
		}
		return completedText, nil
	}
}

// ToolCallHandler returns a TurnHandler that issues a single item/tool/call
// request. The reply body is captured into Server.ToolReplies().
func ToolCallHandler(toolName string, arguments any, completedText string) TurnHandler {
	return func(_ context.Context, _ TurnRequest, e TurnEmitter) (string, error) {
		if _, err := e.ToolCallRequest(toolName, marshalOrRaw(arguments), "call-1"); err != nil {
			return "", fmt.Errorf("tool call: %w", err)
		}
		return completedText, nil
	}
}

// ItemPairHandler emits an item/started + item/completed pair — mirrors the
// stream-json tool_use / tool_result pair the claude-app-server shim relays.
func ItemPairHandler(started, completed map[string]any, completedText string) TurnHandler {
	return func(_ context.Context, _ TurnRequest, e TurnEmitter) (string, error) {
		if err := e.ItemStarted(started); err != nil {
			return "", err
		}
		if err := e.ItemCompleted(completed); err != nil {
			return "", err
		}
		return completedText, nil
	}
}

func marshalOrRaw(v any) json.RawMessage {
	switch x := v.(type) {
	case json.RawMessage:
		return x
	case []byte:
		return x
	case string:
		return json.RawMessage(x)
	}
	b, err := json.Marshal(v)
	if err != nil {
		panic("fakecodex: marshal args: " + err.Error())
	}
	return b
}
