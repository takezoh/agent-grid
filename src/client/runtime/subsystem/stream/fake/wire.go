package fake

import (
	"context"
	"encoding/json"

	"github.com/coder/websocket"
)

// wsServerTransport adapts a coder/websocket connection to the
// codexclient.Transport interface so codexclient.NewConn can run the same
// read/write/close loop that production uses on the client side.
type wsServerTransport struct {
	c *websocket.Conn
}

func (t *wsServerTransport) ReadMessage(ctx context.Context) ([]byte, error) {
	_, data, err := t.c.Read(ctx)
	return data, err
}

func (t *wsServerTransport) WriteMessage(ctx context.Context, data []byte) error {
	return t.c.Write(ctx, websocket.MessageText, data)
}

func (t *wsServerTransport) Close() error { return t.c.CloseNow() }

// nestedString reads a top-level string field from a raw JSON object.
// Returns empty string when missing/wrong type.
func nestedString(raw json.RawMessage, field string) string {
	if len(raw) == 0 {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	s, _ := m[field].(string)
	return s
}

// extractTurnInput pulls the user text out of a turn/start params body.
// Current Codex schema carries user text under input[0].text; legacy tests may
// still provide "message", so keep that as a fallback.
// Returns ("", false) when the shape doesn't match.
func extractTurnInput(raw json.RawMessage) (string, bool) {
	var payload struct {
		Input []struct {
			Type string  `json:"type"`
			Text *string `json:"text"`
		} `json:"input"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &payload); err == nil {
		for _, item := range payload.Input {
			if item.Type == "text" && item.Text != nil {
				return *item.Text, true
			}
		}
		if payload.Message != "" {
			return payload.Message, true
		}
	}
	return "", false
}
