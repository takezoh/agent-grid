package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"

	"github.com/coder/websocket"

	"github.com/takezoh/agent-grid/host/proto"
	"github.com/takezoh/agent-grid/host/state"
)

// lifecycleSubSet tracks the sessions owned by one lifecycle WebSocket.
type lifecycleSubSet struct {
	mu  sync.Mutex
	ids map[string]struct{}
}

// lifecycleResponder serializes request responses for one WebSocket. Data
// calls run asynchronously, but websocket writes remain one-at-a-time.
type lifecycleResponder struct {
	mu *sync.Mutex
	c  *websocket.Conn
}

func writeLifecycleFrameLocked(ctx context.Context, c *websocket.Conn, mu *sync.Mutex, frame []byte) error {
	mu.Lock()
	defer mu.Unlock()
	return c.Write(ctx, websocket.MessageText, frame)
}

func (w *lifecycleResponder) ok(ctx context.Context, reqID string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	writeRespOKFrame(ctx, w.c, reqID)
}

func (w *lifecycleResponder) err(ctx context.Context, reqID, code, message string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	writeRespErrFrame(ctx, w.c, reqID, code, message)
}

func newLifecycleSubSet() *lifecycleSubSet { return &lifecycleSubSet{ids: make(map[string]struct{})} }

func (s *lifecycleSubSet) add(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, existed := s.ids[id]
	s.ids[id] = struct{}{}
	return !existed
}

func (s *lifecycleSubSet) remove(id string) { s.mu.Lock(); delete(s.ids, id); s.mu.Unlock() }

func (s *lifecycleSubSet) contains(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.ids[id]
	return ok
}

func (s *lifecycleSubSet) drain() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.ids))
	for id := range s.ids {
		out = append(out, id)
	}
	s.ids = make(map[string]struct{})
	return out
}

func readLifecycleInbound(ctx context.Context, sess Attacher, c *websocket.Conn, subs *lifecycleSubSet, subscriberID, ownerID, clientInstanceID string, writeMu *sync.Mutex) {
	responses := &lifecycleResponder{c: c, mu: writeMu}
	dataSlots := make(chan struct{}, 64)
	desiredSlot := make(chan inbound, 1)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-desiredSlot:
				handleLifecycleDesired(ctx, sess, responses, &msg, subs, subscriberID, ownerID)
			}
		}
	}()
	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			return
		}
		var msg inbound
		if json.Unmarshal(data, &msg) != nil {
			continue
		}
		msgCopy := msg
		switch msg.K {
		case "ld":
			select {
			case desiredSlot <- msgCopy:
			default:
				// A newer complete desired value supersedes an unadmitted one.
				// Resolve only the request envelope; no lifecycle outcome is
				// authored by the gateway for the displaced publication.
				select {
				case displaced := <-desiredSlot:
					responses.ok(ctx, displaced.ReqID)
				default:
				}
				select {
				case desiredSlot <- msgCopy:
				case <-ctx.Done():
					return
				}
			}
		case "s":
			go handleLifecycleSubscribe(ctx, sess, responses, &msgCopy, subs, subscriberID)
		case "u":
			go handleLifecycleUnsubscribe(ctx, sess, responses, &msgCopy, subs, subscriberID)
		case "i":
			if msg.SessionID == "" {
				continue
			}
			select {
			case dataSlots <- struct{}{}:
				go func(msg inbound) {
					defer func() { <-dataSlots }()
					if err := sess.WriteRaw(ctx, msg.SessionID, []byte(msg.D)); err != nil {
						slog.Warn("server/api: lifecycle write raw", "err", err, "sid", msg.SessionID)
					}
				}(msgCopy)
			default:
				slog.Warn("server/api: lifecycle data lane full", "kind", "write_raw", "sid", msg.SessionID)
			}
		case "r":
			if msg.SessionID == "" {
				continue
			}
			select {
			case dataSlots <- struct{}{}:
				go func(msg inbound) {
					defer func() { <-dataSlots }()
					tryResize(ctx, sess, msg.SessionID, msg.Cols, msg.Rows, "lifecycle resize")
				}(msgCopy)
			default:
				slog.Warn("server/api: lifecycle data lane full", "kind", "resize", "sid", msg.SessionID)
			}
		case "ar":
			go handleLifecycleApprovalRespond(ctx, sess, responses, &msgCopy, clientInstanceID)
		case "qr":
			go handleLifecycleQuestionRespond(ctx, sess, responses, &msgCopy, clientInstanceID)
		}
	}
}

func handleLifecycleDesired(ctx context.Context, sess Attacher, responses *lifecycleResponder, msg *inbound, subs *lifecycleSubSet, subscriberID, ownerID string) {
	if msg.Correlation == nil || msg.Correlation.ClientInstanceID == "" || msg.SessionID == "" || msg.Desired == nil {
		responses.err(ctx, msg.ReqID, "invalid_argument", "lifecycle desired requires correlation, sessionId and desired")
		return
	}
	if sender, ok := sess.(LifecycleDesiredSender); ok {
		if ownerID == "" {
			ownerID = subscriberID
		}
		cmd := proto.CmdLifecycleDesired{
			Correlation: *msg.Correlation,
			SessionID:   msg.SessionID,
			Cols:        uint16(msg.Cols), Rows: uint16(msg.Rows), Desired: *msg.Desired,
		}
		if *msg.Desired && (msg.Cols <= 0 || msg.Rows <= 0) {
			responses.err(ctx, msg.ReqID, "invalid_argument", "lifecycle desired requires non-zero cols and rows")
			return
		}
		if *msg.Desired {
			if reason := state.SizeHintRejectReason(msg.Cols, msg.Rows); reason != "" {
				responses.err(ctx, msg.ReqID, "invalid_argument", reason)
				return
			}
		}
		if err := sender.SendLifecycleDesired(ctx, cmd, ownerID); err != nil {
			if *msg.Desired {
				subs.remove(msg.SessionID)
			}
			code, message := unwrapProtoError(err)
			responses.err(ctx, msg.ReqID, code, message)
			return
		}
		if *msg.Desired {
			subs.add(msg.SessionID)
		} else {
			subs.remove(msg.SessionID)
		}
		responses.ok(ctx, msg.ReqID)
		return
	}
	if *msg.Desired {
		if msg.Cols <= 0 || msg.Rows <= 0 {
			responses.err(ctx, msg.ReqID, "invalid_argument", "lifecycle desired requires non-zero cols and rows")
			return
		}
		added := subs.add(msg.SessionID)
		if err := sess.SendSurfaceSubscribe(ctx, msg.SessionID, subscriberID, uint16(msg.Cols), uint16(msg.Rows)); err != nil {
			var protocolErr *proto.ErrorBody
			if added && errors.As(err, &protocolErr) {
				subs.remove(msg.SessionID)
			}
			code, message := unwrapProtoError(err)
			responses.err(ctx, msg.ReqID, code, message)
			return
		}
	} else {
		if err := sess.SendSurfaceUnsubscribe(ctx, msg.SessionID, subscriberID); err != nil {
			code, message := unwrapProtoError(err)
			responses.err(ctx, msg.ReqID, code, message)
			return
		}
		subs.remove(msg.SessionID)
	}
	responses.ok(ctx, msg.ReqID)
}

func handleLifecycleSubscribe(ctx context.Context, sess Attacher, responses *lifecycleResponder, msg *inbound, subs *lifecycleSubSet, subscriberID string) {
	if msg.SessionID == "" {
		responses.err(ctx, msg.ReqID, "invalid_argument", "sessionId required")
		return
	}
	if msg.Cols <= 0 || msg.Rows <= 0 {
		responses.err(ctx, msg.ReqID, "invalid_argument", "subscribe requires non-zero cols and rows")
		return
	}
	if reason := state.SizeHintRejectReason(msg.Cols, msg.Rows); reason != "" {
		responses.err(ctx, msg.ReqID, "invalid_argument", reason)
		return
	}
	added := subs.add(msg.SessionID)
	err := sess.SendSurfaceSubscribe(ctx, msg.SessionID, subscriberID, uint16(msg.Cols), uint16(msg.Rows))
	if err != nil {
		var protocolErr *proto.ErrorBody
		if added && errors.As(err, &protocolErr) {
			subs.remove(msg.SessionID)
		}
		code, message := unwrapProtoError(err)
		slog.Warn("server/api: lifecycle surface subscribe", "err", err, "sid", msg.SessionID)
		responses.err(ctx, msg.ReqID, code, message)
		return
	}
	responses.ok(ctx, msg.ReqID)
}

func handleLifecycleUnsubscribe(ctx context.Context, sess Attacher, responses *lifecycleResponder, msg *inbound, subs *lifecycleSubSet, subscriberID string) {
	if msg.SessionID == "" {
		responses.err(ctx, msg.ReqID, "invalid_argument", "sessionId required")
		return
	}
	if err := sess.SendSurfaceUnsubscribe(ctx, msg.SessionID, subscriberID); err != nil {
		code, message := unwrapProtoError(err)
		slog.Warn("server/api: lifecycle surface unsubscribe", "err", err, "sid", msg.SessionID)
		responses.err(ctx, msg.ReqID, code, message)
		return
	}
	subs.remove(msg.SessionID)
	responses.ok(ctx, msg.ReqID)
}
