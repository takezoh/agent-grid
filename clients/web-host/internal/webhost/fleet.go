package webhost

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/takezoh/agent-grid/clients/web-host/internal/clientconfig"
)

// Closed protocol-version token: protocol-N only. Anything else is <invalid>/<missing>.
var (
	safeProtocolVersionToken     = regexp.MustCompile(`^protocol-\d+$`)
	closedProtocolMismatchDetail = regexp.MustCompile(
		`^incompatible protocol: client protocol-\d+, server (?:protocol-\d+|<(?:missing|invalid)>)$`,
	)
)

func sanitizeProtocolVersionToken(actual string) string {
	if actual == "" {
		return "<missing>"
	}
	if safeProtocolVersionToken.MatchString(actual) {
		return actual
	}
	return "<invalid>"
}

// protocolMismatchDetail builds the closed protocol-mismatch wire/DOM detail.
func protocolMismatchDetail(actual string) string {
	return "incompatible protocol: client " + protocolVersion + ", server " + sanitizeProtocolVersionToken(actual)
}

const (
	healthPhaseTimeout       = 3 * time.Second
	healthConnectedFreshness = 5 * time.Second
	healthRetryBase          = 250 * time.Millisecond
	healthRetryCap           = 2 * time.Second
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

type healthPrior struct {
	Kind       string `json:"kind"`
	ObservedAt string `json:"observedAt"`
	Reason     string `json:"reason,omitempty"`
	Detail     string `json:"detail,omitempty"`
}

type healthObservation struct {
	Kind       string       `json:"kind"`
	ObservedAt string       `json:"observedAt,omitempty"`
	Reason     string       `json:"reason,omitempty"`
	Detail     string       `json:"detail,omitempty"`
	Prior      *healthPrior `json:"prior,omitempty"`
}

type healthServer struct {
	ServerID    string            `json:"serverId"`
	ServerLabel string            `json:"serverLabel"`
	Enabled     bool              `json:"enabled"`
	Observation healthObservation `json:"observation"`
}

type fleetHealthSnapshotV1 struct {
	SchemaVersion  int            `json:"schemaVersion"`
	HostInstanceID string         `json:"hostInstanceId"`
	Revision       uint64         `json:"revision"`
	Servers        []healthServer `json:"servers"`
}

type healthRecord struct {
	generation           uint64
	terminalIncompatible bool
	observation          healthObservation
}

type fleetRuntime struct {
	mu               sync.RWMutex
	snapshot         fleetSnapshot
	healthRevision   uint64
	hostInstanceID   string
	subscribers      map[chan fleetSnapshot]struct{}
	healthSubs       map[chan fleetHealthSnapshotV1]struct{}
	servers          []clientconfig.Server
	allServers       []clientconfig.Server
	health           map[string]*healthRecord
	startOnce        sync.Once
	stopped          atomic.Bool
	liveTimers       atomic.Int64
	liveFetches      atomic.Int64
	liveSockets      atomic.Int64
	startedWorkers   atomic.Int64
	httpClient       *http.Client
	now              func() time.Time
	dialWebSocket    func(ctx context.Context, endpoint string) (*websocket.Conn, error)
	memberGeneration map[string]uint64
	workerEpoch      map[string]uint64
	memberCancel     map[string]context.CancelFunc
	memberWake       map[string]chan struct{}
	rootCtx          context.Context
	rootCancel       context.CancelFunc
	workers          sync.WaitGroup
	connMu           sync.Mutex
	activeConns      map[*websocket.Conn]struct{}
}

func newFleetRuntime(config clientconfig.Snapshot) *fleetRuntime {
	rootCtx, rootCancel := context.WithCancel(context.Background())
	runtime := &fleetRuntime{
		subscribers:      make(map[chan fleetSnapshot]struct{}),
		healthSubs:       make(map[chan fleetHealthSnapshotV1]struct{}),
		health:           make(map[string]*healthRecord),
		hostInstanceID:   newHostInstanceID(),
		allServers:       append([]clientconfig.Server(nil), config.Servers...),
		httpClient:       &http.Client{Timeout: healthPhaseTimeout},
		now:              time.Now,
		memberGeneration: make(map[string]uint64),
		workerEpoch:      make(map[string]uint64),
		memberCancel:     make(map[string]context.CancelFunc),
		memberWake:       make(map[string]chan struct{}),
		rootCtx:          rootCtx,
		rootCancel:       rootCancel,
		activeConns:      make(map[*websocket.Conn]struct{}),
	}
	runtime.dialWebSocket = func(ctx context.Context, endpoint string) (*websocket.Conn, error) {
		connection, _, err := websocket.Dial(ctx, endpoint, nil)
		return connection, err
	}
	for _, server := range config.Servers {
		if server.Enabled {
			runtime.snapshot.Members = append(runtime.snapshot.Members, fleetMember{
				ServerID: server.ID, ServerLabel: server.DisplayName, State: "connecting",
				Sessions: []json.RawMessage{},
			})
			runtime.servers = append(runtime.servers, server)
			runtime.health[server.ID] = &healthRecord{
				observation: healthObservation{Kind: "checking", ObservedAt: runtime.isoNow()},
			}
		} else {
			runtime.health[server.ID] = &healthRecord{
				observation: healthObservation{Kind: "disabled"},
			}
		}
	}
	return runtime
}

func newHostInstanceID() string {
	var raw [16]byte
	_, _ = rand.Read(raw[:])
	return hex.EncodeToString(raw[:])
}

func (runtime *fleetRuntime) isoNow() string {
	return runtime.now().UTC().Format(time.RFC3339Nano)
}

func (runtime *fleetRuntime) subscribe() (<-chan fleetSnapshot, func()) {
	runtime.startOnce.Do(runtime.startWorkers)
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

func (runtime *fleetRuntime) subscribeHealth() (<-chan fleetHealthSnapshotV1, func()) {
	runtime.startOnce.Do(runtime.startWorkers)
	channel := make(chan fleetHealthSnapshotV1, 1)
	runtime.mu.Lock()
	runtime.healthSubs[channel] = struct{}{}
	channel <- runtime.healthSnapshotLocked()
	runtime.mu.Unlock()
	return channel, func() {
		runtime.mu.Lock()
		delete(runtime.healthSubs, channel)
		runtime.mu.Unlock()
	}
}

func (runtime *fleetRuntime) startWorkers() {
	for _, server := range runtime.servers {
		runtime.mu.Lock()
		epoch := runtime.workerEpoch[server.ID]
		runtime.mu.Unlock()
		runtime.startedWorkers.Add(1)
		runtime.workers.Add(1)
		go func(server clientconfig.Server, epoch uint64) {
			defer runtime.workers.Done()
			runtime.follow(server, epoch)
		}(server, epoch)
	}
}

// reconnect immediately retires the selected server's current session/health
// attempt, fences late callbacks by generation, resets capped retry / terminal
// protocol state, publishes checking evidence, and starts exactly one fresh
// worker. It does not mutate servers.json or restart other servers.
func (runtime *fleetRuntime) reconnect(serverID string) error {
	if runtime.stopped.Load() {
		return errors.New("fleet runtime is not ready")
	}
	var server clientconfig.Server
	found := false
	for _, candidate := range runtime.allServers {
		if candidate.ID == serverID {
			server = candidate
			found = true
			break
		}
	}
	if !found || !server.Enabled {
		return errUnknownOrDisabledServer
	}

	// Ensure the baseline fleet workers exist before replacing one of them.
	runtime.startOnce.Do(runtime.startWorkers)

	runtime.mu.Lock()
	if record := runtime.health[serverID]; record != nil {
		record.terminalIncompatible = false
	}
	if cancel := runtime.memberCancel[serverID]; cancel != nil {
		cancel()
		delete(runtime.memberCancel, serverID)
	}
	runtime.wakeMemberLocked(serverID)
	generation := runtime.memberGeneration[serverID] + 1
	runtime.memberGeneration[serverID] = generation
	epoch := runtime.workerEpoch[serverID] + 1
	runtime.workerEpoch[serverID] = epoch
	runtime.mu.Unlock()

	runtime.publishHealth(serverID, generation, healthObservation{
		Kind: "checking", ObservedAt: runtime.isoNow(),
	}, false)
	runtime.replace(server, generation, "reconnecting", "manual reconnect", nil)

	runtime.startedWorkers.Add(1)
	runtime.workers.Add(1)
	go func() {
		defer runtime.workers.Done()
		runtime.follow(server, epoch)
	}()
	return nil
}

var errUnknownOrDisabledServer = errors.New("unknown or disabled server")

// wakeMemberLocked closes the current wake channel for serverID and installs a
// fresh one. Caller must hold runtime.mu.
func (runtime *fleetRuntime) wakeMemberLocked(serverID string) {
	if ch := runtime.memberWake[serverID]; ch != nil {
		select {
		case <-ch:
		default:
			close(ch)
		}
	}
	runtime.memberWake[serverID] = make(chan struct{})
}

func (runtime *fleetRuntime) memberWakeChannel(serverID string) <-chan struct{} {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	ch := runtime.memberWake[serverID]
	if ch == nil {
		ch = make(chan struct{})
		runtime.memberWake[serverID] = ch
	}
	return ch
}

// stop cancels all workers, closes live sockets, and waits until live handles
// drain. Retired generations cannot publish after stopped is set.
func (runtime *fleetRuntime) stop() {
	runtime.stopped.Store(true)
	if runtime.rootCancel != nil {
		runtime.rootCancel()
	}
	runtime.connMu.Lock()
	for conn := range runtime.activeConns {
		_ = conn.CloseNow()
	}
	runtime.activeConns = make(map[*websocket.Conn]struct{})
	runtime.connMu.Unlock()

	done := make(chan struct{})
	go func() {
		runtime.workers.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
	}

	// Drop subscriber channels so late publishers cannot deliver.
	runtime.mu.Lock()
	for channel := range runtime.subscribers {
		delete(runtime.subscribers, channel)
	}
	for channel := range runtime.healthSubs {
		delete(runtime.healthSubs, channel)
	}
	runtime.mu.Unlock()

	// Wait briefly for in-flight defers to retire handle counters.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		timers, fetches, sockets := runtime.liveHandles()
		if timers == 0 && fetches == 0 && sockets == 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (runtime *fleetRuntime) liveHandles() (timers, fetches, sockets int64) {
	return runtime.liveTimers.Load(), runtime.liveFetches.Load(), runtime.liveSockets.Load()
}

func (runtime *fleetRuntime) trackConn(conn *websocket.Conn) {
	if conn == nil {
		return
	}
	runtime.connMu.Lock()
	runtime.activeConns[conn] = struct{}{}
	runtime.connMu.Unlock()
}

func (runtime *fleetRuntime) untrackConn(conn *websocket.Conn) {
	if conn == nil {
		return
	}
	runtime.connMu.Lock()
	delete(runtime.activeConns, conn)
	runtime.connMu.Unlock()
}

// interruptibleSleep sleeps unless the runtime is stopped / root context canceled
// or the optional server is woken by manual reconnect.
func (runtime *fleetRuntime) interruptibleSleep(d time.Duration, serverID string) {
	if runtime.stopped.Load() {
		return
	}
	runtime.liveTimers.Add(1)
	defer runtime.liveTimers.Add(-1)
	timer := time.NewTimer(d)
	defer timer.Stop()
	var wake <-chan struct{}
	if serverID != "" {
		wake = runtime.memberWakeChannel(serverID)
	}
	select {
	case <-timer.C:
	case <-runtime.rootCtx.Done():
	case <-wake:
	}
}

func (runtime *fleetRuntime) healthSnapshot() fleetHealthSnapshotV1 {
	runtime.mu.RLock()
	defer runtime.mu.RUnlock()
	return runtime.healthSnapshotLocked()
}

func (runtime *fleetRuntime) healthSnapshotLocked() fleetHealthSnapshotV1 {
	servers := make([]healthServer, 0, len(runtime.allServers))
	for _, server := range runtime.allServers {
		record := runtime.health[server.ID]
		observation := healthObservation{Kind: "disabled"}
		if record != nil {
			observation = record.observation
		}
		servers = append(servers, healthServer{
			ServerID: server.ID, ServerLabel: server.DisplayName, Enabled: server.Enabled,
			Observation: observation,
		})
	}
	return fleetHealthSnapshotV1{
		SchemaVersion:  1,
		HostInstanceID: runtime.hostInstanceID,
		Revision:       runtime.healthRevision,
		Servers:        servers,
	}
}

func (runtime *fleetRuntime) publishHealth(
	serverID string,
	generation uint64,
	observation healthObservation,
	terminalIncompatible bool,
) bool {
	if runtime.stopped.Load() {
		return false
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	record := runtime.health[serverID]
	if record == nil || generation < record.generation {
		return false
	}
	record.generation = generation
	record.observation = observation
	if terminalIncompatible {
		record.terminalIncompatible = true
	}
	runtime.healthRevision++
	snapshot := runtime.healthSnapshotLocked()
	for subscriber := range runtime.healthSubs {
		select {
		case subscriber <- snapshot:
		default:
			select {
			case <-subscriber:
			default:
			}
			subscriber <- snapshot
		}
	}
	return true
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

func (runtime *fleetRuntime) follow(server clientconfig.Server, epoch uint64) {
	var attempt int
	for !runtime.stopped.Load() {
		runtime.mu.Lock()
		if runtime.workerEpoch[server.ID] != epoch {
			runtime.mu.Unlock()
			return
		}
		generation := runtime.memberGeneration[server.ID] + 1
		runtime.memberGeneration[server.ID] = generation
		record := runtime.health[server.ID]
		terminal := record != nil && record.terminalIncompatible
		genCtx, genCancel := context.WithCancel(runtime.rootCtx)
		runtime.memberCancel[server.ID] = genCancel
		runtime.mu.Unlock()
		if terminal {
			genCancel()
			return
		}
		runtime.publishHealth(server.ID, generation, healthObservation{
			Kind: "checking", ObservedAt: runtime.isoNow(),
		}, false)
		err := runtime.followGeneration(server, generation, genCtx)
		genCancel()
		if runtime.stopped.Load() {
			return
		}
		runtime.mu.RLock()
		currentEpoch := runtime.workerEpoch[server.ID]
		record = runtime.health[server.ID]
		terminal = record != nil && record.terminalIncompatible
		prior := nestablePrior(record)
		wasConnected := record != nil && (record.observation.Kind == "connected" ||
			record.observation.Kind == "health_unavailable")
		runtime.mu.RUnlock()
		if currentEpoch != epoch {
			return
		}
		if terminal {
			return
		}
		reason, detail := classifyHealthError(err)
		if reason == "protocol_mismatch" {
			runtime.publishHealth(server.ID, generation, healthObservation{
				Kind: "incompatible_protocol", ObservedAt: runtime.isoNow(),
				Reason: reason, Detail: detail,
			}, true)
			runtime.replace(server, generation, "unavailable", detail, nil)
			return
		}
		if wasConnected {
			observation := healthObservation{
				Kind: "health_unavailable", ObservedAt: runtime.isoNow(), Reason: "refresh_failed",
			}
			if prior != nil {
				observation.Prior = prior
			}
			runtime.publishHealth(server.ID, generation, observation, false)
		} else {
			runtime.publishHealth(server.ID, generation, healthObservation{
				Kind: "unavailable", ObservedAt: runtime.isoNow(), Reason: reason, Detail: detail,
			}, false)
		}
		runtime.replace(server, generation, "reconnecting", reason, nil)
		delay := healthRetryBase
		if attempt > 0 {
			shift := attempt
			if shift > 4 {
				shift = 4
			}
			delay = healthRetryBase * time.Duration(1<<shift)
			if delay > healthRetryCap {
				delay = healthRetryCap
			}
		}
		attempt++
		runtime.interruptibleSleep(delay, server.ID)
	}
}

func nestablePrior(record *healthRecord) *healthPrior {
	if record == nil {
		return nil
	}
	switch record.observation.Kind {
	case "connected", "unavailable", "incompatible_protocol":
		return &healthPrior{
			Kind: record.observation.Kind, ObservedAt: record.observation.ObservedAt,
			Reason: record.observation.Reason, Detail: record.observation.Detail,
		}
	case "health_unavailable":
		return record.observation.Prior
	default:
		return nil
	}
}

func (runtime *fleetRuntime) followGeneration(
	server clientconfig.Server,
	generation uint64,
	ctx context.Context,
) error {
	version, err := runtime.probeCapabilities(ctx, server)
	if err != nil {
		return err
	}
	if version != protocolVersion {
		return errors.New(protocolMismatchDetail(version))
	}

	ticket, err := runtime.requestFleetTicket(ctx, server)
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

	helloCtx, helloCancel := context.WithTimeout(ctx, healthPhaseTimeout)
	defer helloCancel()
	runtime.liveSockets.Add(1)
	connection, err := runtime.dialWebSocket(helloCtx, target.String())
	if err != nil {
		runtime.liveSockets.Add(-1)
		return err
	}
	runtime.trackConn(connection)
	defer func() {
		runtime.untrackConn(connection)
		_ = connection.CloseNow()
		runtime.liveSockets.Add(-1)
	}()

	// Hello phase: wait up to 3s for the first qualifying frame, then keep
	// reading view-updates so fleet membership stays live. Each membership
	// snapshot renews Connected freshness (mirrors Workspace fleet-session-runtime).
	helloDeadline := runtime.now().Add(healthPhaseTimeout)
	connected := false
	var freshnessDeadline time.Time

	for {
		if runtime.stopped.Load() {
			return errors.New("runtime stopped")
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}

		var readCtx context.Context
		var readCancel context.CancelFunc
		if !connected {
			if runtime.now().After(helloDeadline) {
				return errors.New("hello timed out")
			}
			readCtx, readCancel = context.WithDeadline(ctx, helloDeadline)
		} else {
			// Bound the read by remaining Connected freshness so silence ends
			// the generation instead of hanging forever.
			readCtx, readCancel = context.WithDeadline(ctx, freshnessDeadline)
		}
		runtime.liveTimers.Add(1)
		_, data, readErr := connection.Read(readCtx)
		readCancel()
		runtime.liveTimers.Add(-1)
		if readErr != nil {
			if connected && (errors.Is(readErr, context.DeadlineExceeded) || errors.Is(readErr, context.Canceled)) {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return errors.New("connected freshness expired")
			}
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
			runtime.publishHealth(server.ID, generation, healthObservation{
				Kind: "connected", ObservedAt: runtime.isoNow(),
			}, false)
			connected = true
			freshnessDeadline = runtime.now().Add(healthConnectedFreshness)
		}
	}
}

func (runtime *fleetRuntime) probeCapabilities(ctx context.Context, server clientconfig.Server) (string, error) {
	target := strings.TrimRight(server.URL, "/") + "/api/capabilities"
	phaseCtx, cancel := context.WithTimeout(ctx, healthPhaseTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(phaseCtx, http.MethodGet, target, nil)
	if err != nil {
		return "", err
	}
	if server.Auth.Mode == "bearer" {
		request.Header.Set("Authorization", "Bearer "+server.Auth.Token)
	}
	runtime.liveFetches.Add(1)
	defer runtime.liveFetches.Add(-1)
	runtime.liveTimers.Add(1)
	response, err := runtime.httpClient.Do(request)
	runtime.liveTimers.Add(-1)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", errors.New(response.Status)
	}
	var body struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		return "", err
	}
	return body.ProtocolVersion, nil
}

func (runtime *fleetRuntime) requestFleetTicket(ctx context.Context, server clientconfig.Server) (string, error) {
	target := strings.TrimRight(server.URL, "/") + "/api/ws-ticket"
	phaseCtx, cancel := context.WithTimeout(ctx, healthPhaseTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(phaseCtx, http.MethodPost, target, bytes.NewReader(nil))
	if err != nil {
		return "", err
	}
	request.Header.Set(protocolVersionHeader, protocolVersion)
	if server.Auth.Mode == "bearer" {
		request.Header.Set("Authorization", "Bearer "+server.Auth.Token)
	}
	runtime.liveFetches.Add(1)
	defer runtime.liveFetches.Add(-1)
	runtime.liveTimers.Add(1)
	response, err := runtime.httpClient.Do(request)
	runtime.liveTimers.Add(-1)
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

func classifyHealthError(err error) (reason, detail string) {
	if err == nil {
		return "connection_failed", ""
	}
	message := err.Error()
	switch {
	case strings.Contains(strings.ToLower(message), "timeout") ||
		strings.Contains(strings.ToLower(message), "deadline"):
		return "connection_timeout", ""
	case strings.Contains(message, "incompatible protocol"):
		// Only pass through closed detail; never echo untrusted free-form text.
		if closedProtocolMismatchDetail.MatchString(message) {
			return "protocol_mismatch", message
		}
		return "protocol_mismatch", protocolMismatchDetail("")
	case strings.Contains(strings.ToLower(message), "websocket"):
		return "websocket_failed", ""
	case strings.Contains(strings.ToLower(message), "empty websocket ticket"):
		return "empty_ticket", ""
	case strings.Contains(message, "HTTP") || strings.HasPrefix(message, "5") || strings.HasPrefix(message, "4"):
		return "http_error", ""
	default:
		return "connection_failed", ""
	}
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

func serveFleetHealth(runtime *fleetRuntime, w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		http.Error(w, "forbidden origin", http.StatusForbidden)
		return
	}
	if r.Header.Get("Upgrade") == "" && r.URL.Query().Get("subscribe") != "1" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(runtime.healthSnapshot())
		return
	}
	connection, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer func() { _ = connection.CloseNow() }()
	snapshots, unsubscribe := runtime.subscribeHealth()
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

func serveFleetReconnect(
	runtime *fleetRuntime,
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	serverID := strings.TrimPrefix(r.URL.Path, "/host/fleet/servers/")
	serverID = strings.TrimSuffix(serverID, "/reconnect")
	serverID = strings.Trim(serverID, "/")
	if serverID == "" || strings.Contains(serverID, "/") {
		http.Error(w, "unknown or disabled server", http.StatusNotFound)
		return
	}
	if err := runtime.reconnect(serverID); err != nil {
		if errors.Is(err, errUnknownOrDisabledServer) {
			http.Error(w, "unknown or disabled server", http.StatusNotFound)
			return
		}
		http.Error(w, "reconnect failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

// DecodeFleetHealthSnapshotV1 validates the credential-free health wire schema.
func DecodeFleetHealthSnapshotV1(raw []byte) (fleetHealthSnapshotV1, error) {
	var snapshot fleetHealthSnapshotV1
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return snapshot, err
	}
	if snapshot.SchemaVersion != 1 {
		return snapshot, errors.New("unknown health schema version")
	}
	if snapshot.HostInstanceID == "" {
		return snapshot, errors.New("hostInstanceId required")
	}
	// Re-walk raw observations so unknown fields (e.g. stale-as-current) reject
	// with the same closed union rules as the TS codecs.
	var envelope struct {
		Servers []struct {
			Observation json.RawMessage `json:"observation"`
		} `json:"servers"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return snapshot, err
	}
	if len(envelope.Servers) != len(snapshot.Servers) {
		return snapshot, errors.New("servers required")
	}
	for i, server := range snapshot.Servers {
		if err := validateHealthObservationRaw(envelope.Servers[i].Observation, server.Observation); err != nil {
			return snapshot, err
		}
	}
	return snapshot, nil
}

func validateHealthObservationRaw(raw json.RawMessage, observation healthObservation) error {
	if len(raw) == 0 {
		return errors.New("invalid health observation")
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return err
	}
	if _, ok := object["stale"]; ok {
		return errors.New("independent stale flag is forbidden")
	}
	return validateHealthObservation(observation, object)
}

func validateHealthObservation(observation healthObservation, object map[string]json.RawMessage) error {
	switch observation.Kind {
	case "disabled":
		if observation.ObservedAt != "" || observation.Reason != "" || observation.Prior != nil || observation.Detail != "" {
			return errors.New("disabled observation must not carry freshness")
		}
		for key := range object {
			if key != "kind" {
				return errors.New("disabled observation must not carry freshness")
			}
		}
		return nil
	case "checking", "connected":
		if observation.ObservedAt == "" {
			return errors.New("observedAt required")
		}
		if observation.Reason != "" || observation.Prior != nil || observation.Detail != "" {
			return errors.New("exclusive observation violated")
		}
		for key := range object {
			if key != "kind" && key != "observedAt" {
				return errors.New("exclusive observation violated")
			}
		}
		return nil
	case "unavailable", "incompatible_protocol":
		if observation.ObservedAt == "" {
			return errors.New("observedAt required")
		}
		if observation.Prior != nil {
			return errors.New("current observation cannot nest prior")
		}
		if _, ok := object["prior"]; ok {
			return errors.New("current observation cannot nest prior")
		}
		return validateHealthReason(observation.Reason)
	case "health_unavailable":
		if observation.ObservedAt == "" {
			return errors.New("observedAt required")
		}
		if observation.Prior != nil {
			switch observation.Prior.Kind {
			case "connected", "unavailable", "incompatible_protocol":
			default:
				return errors.New("invalid health prior kind")
			}
			if observation.Prior.ObservedAt == "" {
				return errors.New("prior observedAt required")
			}
			if rawPrior, ok := object["prior"]; ok {
				var priorObject map[string]json.RawMessage
				if err := json.Unmarshal(rawPrior, &priorObject); err != nil {
					return err
				}
				if _, ok := priorObject["stale"]; ok {
					return errors.New("stale flag is not allowed on prior")
				}
				// Reject empty/invalid prior shapes (missing kind handled by Kind switch).
				if len(priorObject) == 0 {
					return errors.New("invalid health prior kind")
				}
			}
		}
		return validateHealthReason(observation.Reason)
	default:
		return errors.New("unknown health observation kind")
	}
}

func validateHealthReason(reason string) error {
	if reason == "" {
		return nil
	}
	switch reason {
	case "connection_timeout", "connection_failed", "http_error", "websocket_failed",
		"empty_ticket", "protocol_mismatch", "refresh_failed", "delivery_lost":
		return nil
	default:
		return errors.New("invalid health reason")
	}
}
