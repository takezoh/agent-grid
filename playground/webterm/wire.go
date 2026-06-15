package main

import "encoding/json"

// Wire format (PoC). Two message shapes share one WebSocket text channel:
//
//   server → client, OUTPUT  : JSON array  [time, "o", data]   (asciicast v2 event)
//   server → client, CONTROL : JSON object {"k":...,"code":...,"data":...}
//   client → server          : JSON object {"k":"i","d":...} | {"k":"r","cols":..,"rows":..}
//
// Output uses the asciicast v2 3-tuple so the stream is recorder/player
// compatible; arc-specific control events (OSC, prompt, title, …) are objects so
// the client distinguishes them with a single Array.isArray() check.

// outputFrame encodes an asciicast v2 output event: [time, "o", data].
func outputFrame(t float64, data []byte) []byte {
	b, _ := json.Marshal([]any{t, "o", string(data)})
	return b
}

// controlMsg is a server→client control event (OSC handled server-side, etc.).
type controlMsg struct {
	K    string `json:"k"`              // "osc" | "prompt" | "title" | "bell" | "exit"
	Code int    `json:"code,omitempty"` // OSC command number when applicable
	Data string `json:"data,omitempty"`
}

func controlFrame(kind string, code int, data string) []byte {
	b, _ := json.Marshal(controlMsg{K: kind, Code: code, Data: data})
	return b
}

// inbound is a client→server message (always a JSON object).
type inbound struct {
	K    string `json:"k"` // "i" input | "r" resize
	D    string `json:"d"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}
