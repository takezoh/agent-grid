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

// nestedString reads a top-level string field from a raw JSON object. Empty
// string + ok=false when missing/wrong type.
func nestedString(raw json.RawMessage, field string) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return "", false
	}
	s, ok := m[field].(string)
	return s, ok
}

// extractTurnInput pulls the user text out of a turn/start params body. The
// codexclient.StartTurn helper serialises the raw stdin bytes into a
// "message" string field (see StartTurn in platform/agent/codexclient/client.go).
// Returns ("", false) when the shape doesn't match.
func extractTurnInput(raw json.RawMessage) (string, bool) {
	return nestedString(raw, "message")
}
