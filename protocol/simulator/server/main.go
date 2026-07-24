// Command simserver replays protocol/simulator recordings on a tiny HTTP+WS
// surface so generated SDKs can be driven without a live agent (FR-P1-07/08).
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8790", "listen address")
	recording := flag.String("recording", "", "path to .jsonl recording (required)")
	flag.Parse()
	if *recording == "" {
		// Default relative to this binary's source tree when run from repo root.
		*recording = filepath.Join("protocol", "simulator", "recordings", "approval-round-trip.jsonl")
	}
	events, err := loadRecording(*recording)
	if err != nil {
		log.Fatalf("load recording: %v", err)
	}
	s := &simServer{events: events}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /api/capabilities", s.handleCapabilities)
	mux.HandleFunc("GET /replay", s.handleReplay)
	log.Printf("simserver listening on %s (events=%d)", *addr, len(events))
	log.Fatal(http.ListenAndServe(*addr, mux))
}

type simEvent map[string]any

type simServer struct {
	mu     sync.Mutex
	events []simEvent
}

func loadRecording(path string) ([]simEvent, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []simEvent
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev simEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			return nil, fmt.Errorf("line %d: %w", len(out)+1, err)
		}
		out = append(out, ev)
	}
	return out, sc.Err()
}

func (s *simServer) handleCapabilities(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"protocolVersion": "1.0.0-phase01",
		"capabilities": []string{
			"approval.respond",
			"question.respond",
			"sessions.view_update",
			"surface.subscribe",
		},
		"axis": "bundled",
	})
}

// handleReplay returns the full recorded sequence as a JSON array. SDKs and
// CI assert byte-stable ordering against fixtures.
func (s *simServer) handleReplay(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Copy so concurrent readers cannot observe partial writes.
	cp := make([]simEvent, len(s.events))
	copy(cp, s.events)
	writeJSON(w, map[string]any{
		"protocolVersion": "1.0.0-phase01",
		"replayedAt":      time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
		"events":          cp,
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
