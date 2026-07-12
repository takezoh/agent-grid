package runtime

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"sync"

	"github.com/takezoh/agent-grid/client/proto"
	"github.com/takezoh/agent-grid/client/state"
)

// ipcConn is one accepted client connection. The reader goroutine
// decodes envelopes off the socket and forwards typed Commands to
// the runtime as state.Event values; the writer goroutine drains
// outbox and writes wire bytes to the socket.
type ipcConn struct {
	id                state.ConnID
	conn              net.Conn
	outboxInteractive chan []byte
	outboxBulk        chan []byte
	done              chan struct{}
	once              sync.Once
	writeMu           sync.Mutex
}

const ipcOutboxLaneSize = 32

func newIPCConn(id state.ConnID, conn net.Conn) *ipcConn {
	return &ipcConn{
		id:                id,
		conn:              conn,
		outboxInteractive: make(chan []byte, ipcOutboxLaneSize),
		outboxBulk:        make(chan []byte, ipcOutboxLaneSize),
		done:              make(chan struct{}),
	}
}

// shut closes the connection and signals the writer to exit.
// Idempotent.
func (cc *ipcConn) shut() {
	cc.once.Do(func() {
		close(cc.done)
		_ = cc.conn.Close()
	})
}

// === Listener / accept loop ===

// StartIPC opens the Unix socket and starts the accept loop. Should
// be called from main after Run is already running (so the accept
// loop can call Enqueue).
func (r *Runtime) StartIPC(sockPath string) error {
	_ = os.Remove(sockPath)
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		return fmt.Errorf("runtime: listen %s: %w", sockPath, err)
	}
	// Restrict socket to owner only — the client controls the frame backend
	// lifecycle, so unauthenticated local access = arbitrary command
	// execution.
	if err := os.Chmod(sockPath, 0o600); err != nil {
		_ = ln.Close()
		return fmt.Errorf("runtime: chmod %s: %w", sockPath, err)
	}
	r.listener = ln
	slog.Info("runtime: ipc listening", "sock", sockPath)
	go r.acceptLoop()
	return nil
}

func (r *Runtime) acceptLoop() {
	for {
		conn, err := r.listener.Accept()
		if err != nil {
			select {
			case <-r.done:
				return
			default:
				if errors.Is(err, net.ErrClosed) {
					return
				}
				slog.Error("runtime: accept failed", "err", err)
				continue
			}
		}
		if err := checkPeerCred(conn); err != nil {
			slog.Warn("runtime: rejecting connection", "err", err)
			_ = conn.Close()
			continue
		}
		_ = r.enqueueInternal(connOpen{conn: conn})
	}
}

func (r *Runtime) handleConnOpen(conn net.Conn) {
	r.nextConn++
	id := r.nextConn
	cc := newIPCConn(id, conn)
	r.conns[id] = cc
	go r.connWriter(cc)
	go r.connReader(cc)
	r.dispatch(state.EvConnOpened{ConnID: id})
}

// connReader decodes wire envelopes, translates Commands into
// state.Events, and enqueues them on the runtime event loop. On EOF
// or error, it enqueues EvConnClosed and exits.
func (r *Runtime) connReader(cc *ipcConn) {
	defer func() {
		_ = r.enqueueInternal(connClose{id: cc.id})
	}()
	dec := json.NewDecoder(cc.conn)
	for {
		select {
		case <-cc.done:
			return
		default:
		}
		var env proto.Envelope
		if err := dec.Decode(&env); err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
				slog.Debug("runtime: conn decode error", "conn", cc.id, "err", err)
			}
			return
		}
		if env.Type != proto.TypeCommand {
			continue
		}
		cmd, err := proto.DecodeCommand(env)
		if err != nil {
			slog.Warn("runtime: bad command", "conn", cc.id, "err", err)
			r.sendErrorImmediate(cc, env.ReqID, proto.ErrInvalidArgument, err.Error())
			continue
		}
		if r.handleDirectIPCCommand(cc, env.ReqID, cmd) {
			continue
		}
		ev := commandToStateEvent(cc.id, env.ReqID, cmd)
		if ev == nil {
			r.sendErrorImmediate(cc, env.ReqID, proto.ErrUnsupported, "unknown command")
			continue
		}
		r.Enqueue(ev)
	}
}

func (r *Runtime) handleDirectIPCCommand(cc *ipcConn, reqID string, cmd proto.Command) bool {
	var (
		resp proto.Response
		err  error
		ok   bool
	)
	switch c := cmd.(type) {
	case proto.CmdHookEvent:
		resp, err = r.directHookEvent(c.Token, c.Hook, c.Timestamp, c.Payload)
		ok = true
	case proto.CmdFrameList:
		resp, err = r.directFrameList(c.Token)
		ok = true
	case proto.CmdFrameRead:
		resp, err = r.directFrameRead(c.Token, state.FrameID(c.PeerFrameID))
		ok = true
	case proto.CmdFrameSend:
		resp, err = r.directFrameSend(c.Token, state.FrameID(c.TargetFrameID), c.Topic, c.Body, c.Priority)
		ok = true
	case proto.CmdFrameReply:
		resp, err = r.directFrameReply(c.Token, c.MessageID, c.Body, c.FinalAnswer, c.Resolution, c.Confidence)
		ok = true
	case proto.CmdFrameListByThread:
		resp, err = r.ListByThread(state.SessionID(c.SessionID), c.ThreadID)
		ok = true
	case proto.CmdFrameReadByThread:
		resp, err = r.ReadByThread(state.SessionID(c.SessionID), c.ThreadID, state.FrameID(c.PeerFrameID))
		ok = true
	case proto.CmdFrameSendByThread:
		resp, err = r.SendByThread(state.SessionID(c.SessionID), c.ThreadID, state.FrameID(c.TargetFrameID), c.Topic, c.Body, c.Priority)
		ok = true
	case proto.CmdFrameReplyByThread:
		resp, err = r.ReplyByThread(state.SessionID(c.SessionID), c.ThreadID, c.MessageID, c.Body, c.FinalAnswer, c.Resolution, c.Confidence)
		ok = true
	}
	if !ok {
		return false
	}
	if err != nil {
		var body *proto.ErrorBody
		if errors.As(err, &body) {
			r.sendErrorImmediate(cc, reqID, body.Code, body.Message)
			return true
		}
		r.sendErrorImmediate(cc, reqID, proto.ErrInternal, err.Error())
		return true
	}
	wire, encErr := proto.EncodeResponse(reqID, resp)
	if encErr != nil {
		r.sendErrorImmediate(cc, reqID, proto.ErrInternal, encErr.Error())
		return true
	}
	r.queueWire(cc, wire)
	return true
}

func (r *Runtime) directFrameList(token string) (proto.Response, error) {
	source, err := r.frameSourceForToken(token)
	if err != nil {
		return nil, err
	}
	return r.List(source)
}

func (r *Runtime) directFrameRead(token string, peer state.FrameID) (proto.Response, error) {
	source, err := r.frameSourceForToken(token)
	if err != nil {
		return nil, err
	}
	return r.Read(source, peer)
}

func (r *Runtime) directFrameSend(token string, target state.FrameID, topic, body, priority string) (proto.Response, error) {
	source, err := r.frameSourceForToken(token)
	if err != nil {
		return nil, err
	}
	return r.Send(source, target, topic, body, priority)
}

func (r *Runtime) directFrameReply(token, messageID, body, finalAnswer, resolution, confidence string) (proto.Response, error) {
	source, err := r.frameSourceForToken(token)
	if err != nil {
		return nil, err
	}
	return r.Reply(source, messageID, body, finalAnswer, resolution, confidence)
}

func (r *Runtime) frameSourceForToken(token string) (state.FrameID, error) {
	if token == "" {
		return "", &proto.ErrorBody{Code: proto.ErrInvalidArgument, Message: "invalid token"}
	}
	source, ok := r.frameReg.Lookup(token)
	if !ok {
		return "", &proto.ErrorBody{Code: proto.ErrInvalidArgument, Message: "invalid token"}
	}
	return source, nil
}

// connWriter drains the outbox lanes with interactive priority and writes
// wire bytes to the socket.
func (r *Runtime) connWriter(cc *ipcConn) {
	for {
		select {
		case <-cc.done:
			return
		case payload := <-cc.outboxInteractive:
			if err := r.writeWire(cc, payload); err != nil {
				return
			}
			continue
		default:
		}
		select {
		case <-cc.done:
			return
		case payload := <-cc.outboxInteractive:
			if err := r.writeWire(cc, payload); err != nil {
				return
			}
		case payload := <-cc.outboxBulk:
			if err := r.writeWire(cc, payload); err != nil {
				return
			}
		}
	}
}

func (r *Runtime) handleConnClose(id state.ConnID) {
	if cc, ok := r.conns[id]; ok {
		cc.shut()
		delete(r.conns, id)
	}
	r.dispatch(state.EvConnClosed{ConnID: id})
}

// sendErrorImmediate writes an error response on a connection
// without going through the reducer (used for malformed envelopes
// caught in connReader, before the event loop ever sees them).
func (r *Runtime) sendErrorImmediate(cc *ipcConn, reqID string, code proto.ErrCode, msg string) {
	wire, err := proto.EncodeError(reqID, code, msg, nil)
	if err != nil {
		return
	}
	r.queueWire(cc, wire)
}

// queueWire enqueues raw wire bytes on a conn's bulk outbox lane.
func (r *Runtime) queueWire(cc *ipcConn, wire []byte) {
	r.queueWireLane(cc, wire, false, SubscriptionKey{})
}

func (r *Runtime) queueWireLane(cc *ipcConn, wire []byte, interactive bool, sub SubscriptionKey) {
	lane := cc.outboxBulk
	if interactive {
		lane = cc.outboxInteractive
	}
	select {
	case lane <- wire:
	case <-cc.done:
	default:
		if interactive && sub.SessionID != "" && r.terminalRelay != nil {
			r.terminalRelay.SeverOwned(sub.ConnID, sub.SessionID, sub.SubscriberID)
			return
		}
		slog.Warn("runtime: conn outbox full, dropping", "conn", cc.id, "interactive", interactive)
	}
}

func (r *Runtime) writeWire(cc *ipcConn, wire []byte) error {
	cc.writeMu.Lock()
	defer cc.writeMu.Unlock()

	select {
	case <-cc.done:
		return net.ErrClosed
	default:
	}

	w := bufio.NewWriter(cc.conn)
	if _, err := w.Write(wire); err != nil {
		return err
	}
	if err := w.WriteByte('\n'); err != nil {
		return err
	}
	return w.Flush()
}

// shutdownIPC closes the listener and every active connection. Called
// from Run on shutdown.
func (r *Runtime) shutdownIPC() {
	if r.listener != nil {
		_ = r.listener.Close()
	}
	for id, cc := range r.conns {
		cc.shut()
		delete(r.conns, id)
	}
	r.shutdownContainerEndpoints()
}

// === Loop integration ===
