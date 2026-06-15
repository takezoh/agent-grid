// Package web bridges a browser (xterm.js over WebSocket) to a server-side
// termvt session. It encodes terminal output as asciicast v2 frames and
// structured control events, and decodes client input/resize messages.
//
// It depends on platform/termvt and stdlib + coder/websocket only; session
// lifecycle (create/stop) is owned by the server runtime, not this package —
// the gateway only attaches to an already-running session.
package web

import (
	"encoding/json"

	"github.com/takezoh/agent-reactor/platform/termvt"
)

// outputFrame encodes an asciicast v2 output event: [time, "o", data].
func outputFrame(t float64, data []byte) []byte {
	b, _ := json.Marshal([]any{t, "o", string(data)})
	return b
}

// controlMsg is a server→client control event (OSC/title/bell/exit), distinct
// from output by being a JSON object rather than the asciicast array.
type controlMsg struct {
	K    string `json:"k"`
	Code int    `json:"code,omitempty"`
	Data string `json:"data,omitempty"`
}

func controlFrame(kind string, code int, data string) []byte {
	b, _ := json.Marshal(controlMsg{K: kind, Code: code, Data: data})
	return b
}

// encodeEvent renders a termvt.Event as a single WebSocket text frame.
func encodeEvent(elapsed float64, ev termvt.Event) []byte {
	switch ev.Kind {
	case termvt.EventOutput:
		return outputFrame(elapsed, ev.Data)
	case termvt.EventControl:
		return controlFrame(ev.Ctl.Kind, ev.Ctl.Code, ev.Ctl.Data)
	case termvt.EventExit:
		return controlFrame("exit", 0, "")
	default:
		return nil
	}
}

// inbound is a client→server message (always a JSON object).
type inbound struct {
	K    string `json:"k"` // "i" input | "r" resize
	D    string `json:"d"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}
