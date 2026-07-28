package webhost

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/takezoh/agent-grid/clients/web-host/internal/clientconfig"
)

type fleetMember struct {
	ServerID    string            `json:"serverId"`
	ServerLabel string            `json:"serverLabel"`
	Generation  uint64            `json:"generation"`
	State       string            `json:"state"`
	Stale       bool              `json:"stale,omitempty"`
	Reason      string            `json:"reason,omitempty"`
	Sessions    []json.RawMessage `json:"sessions"`
}

type fleetSession struct {
	Ref         sessionRef      `json:"ref"`
	ServerLabel string          `json:"serverLabel"`
	Session     json.RawMessage `json:"session"`
	Stale       bool            `json:"stale"`
}

type sessionRef struct {
	ServerID  string `json:"serverId"`
	SessionID string `json:"sessionId"`
}

type fleetSnapshot struct {
	Revision uint64         `json:"revision"`
	Members  []fleetMember  `json:"members"`
	Sessions []fleetSession `json:"sessions"`
}

type fleetRuntime struct {
	mu          sync.RWMutex
	snapshot    fleetSnapshot
	subscribers map[chan fleetSnapshot]struct{}
	servers     []clientconfig.Server
	startOnce   sync.Once
}

func newFleetRuntime(config clientconfig.Snapshot) *fleetRuntime {
	runtime := &fleetRuntime{subscribers: make(map[chan fleetSnapshot]struct{})}
	for _, server := range config.Servers {
		if !server.Enabled {
			continue
		}
		runtime.snapshot.Members = append(runtime.snapshot.Members, fleetMember{
			ServerID: server.ID, ServerLabel: server.DisplayName, State: "connecting",
			Sessions: []json.RawMessage{},
		})
		runtime.servers = append(runtime.servers, server)
	}
	return runtime
}

func (runtime *fleetRuntime) subscribe() (<-chan fleetSnapshot, func()) {
	runtime.startOnce.Do(func() {
		for _, server := range runtime.servers {
			go runtime.follow(server)
		}
	})
	channel := make(chan fleetSnapshot, 1)
	runtime.mu.Lock()
	runtime.subscribers[channel] = struct{}{}
	channel <- runtime.snapshot
	runtime.mu.Unlock()
	return channel, func() {
		runtime.mu.Lock()
		delete(runtime.subscribers, channel)
		runtime.mu.Unlock()
	}
}

func (runtime *fleetRuntime) replace(
	server clientconfig.Server,
	generation uint64,
	state, reason string,
	sessions []json.RawMessage,
) {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	for index := range runtime.snapshot.Members {
		current := runtime.snapshot.Members[index]
		if current.ServerID != server.ID || generation < current.Generation {
			continue
		}
		if state != "connected" {
			sessions = current.Sessions
		}
		members := append([]fleetMember(nil), runtime.snapshot.Members...)
		members[index] = fleetMember{
			ServerID: server.ID, ServerLabel: server.DisplayName, Generation: generation,
			State: state, Stale: state != "connected" && len(sessions) > 0,
			Reason: reason, Sessions: sessions,
		}
		runtime.snapshot.Members = members
		runtime.snapshot.Revision++
		runtime.snapshot.Sessions = projectFleetSessions(runtime.snapshot.Members)
		for subscriber := range runtime.subscribers {
			select {
			case subscriber <- runtime.snapshot:
			default:
				select {
				case <-subscriber:
				default:
				}
				subscriber <- runtime.snapshot
			}
		}
		return
	}
}

func projectFleetSessions(members []fleetMember) []fleetSession {
	result := make([]fleetSession, 0)
	labelCounts := make(map[string]int, len(members))
	for _, member := range members {
		labelCounts[member.ServerLabel]++
	}
	for _, member := range members {
		displayLabel := member.ServerLabel
		if labelCounts[displayLabel] > 1 {
			displayLabel += " · " + member.ServerID
		}
		for _, raw := range member.Sessions {
			var identity struct {
				ID string `json:"id"`
			}
			if json.Unmarshal(raw, &identity) != nil || identity.ID == "" {
				continue
			}
			result = append(result, fleetSession{
				Ref:         sessionRef{ServerID: member.ServerID, SessionID: identity.ID},
				ServerLabel: displayLabel, Session: raw, Stale: member.Stale,
			})
		}
	}
	return result
}

func (runtime *fleetRuntime) follow(server clientconfig.Server) {
	var generation uint64
	for {
		generation++
		err := runtime.followGeneration(server, generation)
		runtime.replace(server, generation, "reconnecting", errorText(err), nil)
		time.Sleep(time.Second)
	}
}

func (runtime *fleetRuntime) followGeneration(
	server clientconfig.Server,
	generation uint64,
) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ticket, err := requestFleetTicket(ctx, server)
	if err != nil {
		return err
	}
	target, err := url.Parse(server.URL)
	if err != nil {
		return err
	}
	target.Scheme = map[string]string{"http": "ws", "https": "wss"}[target.Scheme]
	target.Path = joinTargetPath(target.Path, "/ws")
	query := target.Query()
	query.Set("ticket", ticket)
	query.Set("protocolVersion", protocolVersion)
	target.RawQuery = query.Encode()
	connection, _, err := websocket.Dial(ctx, target.String(), nil)
	if err != nil {
		return err
	}
	defer func() { _ = connection.CloseNow() }()
	for {
		_, data, readErr := connection.Read(ctx)
		if readErr != nil {
			return readErr
		}
		var frame struct {
			Kind     string            `json:"k"`
			Sessions []json.RawMessage `json:"sessions"`
		}
		if json.Unmarshal(data, &frame) != nil {
			continue
		}
		if (frame.Kind == "h" || frame.Kind == "v") && frame.Sessions != nil {
			runtime.replace(server, generation, "connected", "", frame.Sessions)
		}
	}
}

func requestFleetTicket(ctx context.Context, server clientconfig.Server) (string, error) {
	target := strings.TrimRight(server.URL, "/") + "/api/ws-ticket"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(nil))
	if err != nil {
		return "", err
	}
	request.Header.Set(protocolVersionHeader, protocolVersion)
	if server.Auth.Mode == "bearer" {
		request.Header.Set("Authorization", "Bearer "+server.Auth.Token)
	}
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", errors.New(response.Status)
	}
	var body struct {
		Ticket string `json:"ticket"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		return "", err
	}
	if body.Ticket == "" {
		return "", errors.New("empty websocket ticket")
	}
	return body.Ticket, nil
}

func errorText(err error) string {
	if err == nil {
		return "connection closed"
	}
	return err.Error()
}

func serveFleet(runtime *fleetRuntime, w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		http.Error(w, "forbidden origin", http.StatusForbidden)
		return
	}
	connection, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer func() { _ = connection.CloseNow() }()
	snapshots, unsubscribe := runtime.subscribe()
	defer unsubscribe()
	for {
		select {
		case <-r.Context().Done():
			return
		case snapshot := <-snapshots:
			data, marshalErr := json.Marshal(snapshot)
			if marshalErr != nil ||
				connection.Write(r.Context(), websocket.MessageText, data) != nil {
				return
			}
		}
	}
}

func (runtime *fleetRuntime) waitFor(ref sessionRef, timeout time.Duration) bool {
	snapshots, unsubscribe := runtime.subscribe()
	defer unsubscribe()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case snapshot := <-snapshots:
			for _, session := range snapshot.Sessions {
				if session.Ref == ref {
					return true
				}
			}
		case <-timer.C:
			return false
		}
	}
}

func serveFleetCreate(
	runtime *fleetRuntime,
	config clientconfig.Snapshot,
	w http.ResponseWriter,
	r *http.Request,
) {
	serverID := strings.TrimPrefix(r.URL.Path, "/host/fleet/servers/")
	serverID = strings.TrimSuffix(serverID, "/sessions")
	server, found := config.EnabledServer(serverID)
	if !found || serverID == "" {
		http.Error(w, "unknown or disabled server", http.StatusNotFound)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	target := strings.TrimRight(server.URL, "/") + "/api/sessions"
	request, err := http.NewRequestWithContext(r.Context(), http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		http.Error(w, "invalid upstream", http.StatusInternalServerError)
		return
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(protocolVersionHeader, protocolVersion)
	if server.Auth.Mode == "bearer" {
		request.Header.Set("Authorization", "Bearer "+server.Auth.Token)
	}
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	if err != nil {
		proxyErrorHandler(w, r, err)
		return
	}
	defer response.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		w.WriteHeader(response.StatusCode)
		_, _ = w.Write(responseBody)
		return
	}
	var result map[string]any
	if json.Unmarshal(responseBody, &result) != nil {
		http.Error(w, "invalid create response", http.StatusBadGateway)
		return
	}
	id, _ := result["id"].(string)
	confirmed := id != "" && runtime.waitFor(
		sessionRef{ServerID: serverID, SessionID: id},
		10*time.Second,
	)
	if confirmed {
		result["fleet_status"] = "confirmed"
	} else {
		result["fleet_status"] = "created_unconfirmed"
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}
