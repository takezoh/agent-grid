package runtime

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/takezoh/agent-grid/client/driver"
	"github.com/takezoh/agent-grid/client/state"
	"github.com/takezoh/agent-grid/platform/agentlaunch"
)

const (
	frameMessagingKindSent  = "message_sent"
	frameMessagingKindReply = "message_reply"
	frameMessagingKindRead  = "message_read"

	frameMessagingAuditAllow  = "allow"
	frameMessagingAuditReject = "reject"
)

type frameMessagingJournalRecord struct {
	Kind       string                `json:"kind"`
	Timestamp  string                `json:"timestamp"`
	Message    *FrameMessageSnapshot `json:"message,omitempty"`
	MessageID  string                `json:"message_id,omitempty"`
	MessageIDs []string              `json:"message_ids,omitempty"`
	Reply      *FrameReplySnapshot   `json:"reply,omitempty"`
}

type frameMessagingAuditRecord struct {
	Timestamp     string `json:"timestamp"`
	ToolName      string `json:"tool_name"`
	SessionID     string `json:"session_id"`
	SourceFrameID string `json:"source_frame_id"`
	TargetFrameID string `json:"target_frame_id,omitempty"`
	MessageID     string `json:"message_id,omitempty"`
	Decision      string `json:"decision"`
	Reason        string `json:"reason,omitempty"`
	BodyHash      string `json:"body_hash,omitempty"`
}

func (r *Runtime) appendJSONL(path string, record any) error {
	if r.appendFrameMessagingJSONL != nil {
		return r.appendFrameMessagingJSONL(path, record)
	}
	return appendJSONL(path, record)
}

func cloneRuntimeSessions(in map[state.SessionID]state.Session) map[state.SessionID]state.Session {
	out := make(map[state.SessionID]state.Session, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func isAgentFrame(frame state.SessionFrame) bool {
	argv, err := agentlaunch.SplitArgs(frame.Command)
	if err != nil || len(argv) == 0 {
		return frame.Command == driver.ClaudeDriverName || frame.Command == driver.CodexDriverName
	}
	return argv[0] == driver.ClaudeDriverName || argv[0] == driver.CodexDriverName
}

func liveFrameSet(frames []state.SessionFrame) map[state.FrameID]struct{} {
	out := make(map[state.FrameID]struct{}, len(frames))
	for _, frame := range frames {
		out[frame.ID] = struct{}{}
	}
	return out
}

func sanitizeFrameMessaging(msgs *state.SessionFrameMessaging, liveFrames map[state.FrameID]struct{}) *state.SessionFrameMessaging {
	if msgs == nil {
		return nil
	}
	out := state.CloneSessionFrameMessaging(msgs)
	if out == nil {
		return nil
	}
	filtered := out.Messages[:0]
	for _, msg := range out.Messages {
		if _, ok := liveFrames[msg.SourceFrameID]; !ok {
			continue
		}
		if _, ok := liveFrames[msg.TargetFrameID]; !ok {
			continue
		}
		if msg.Reply != nil {
			if _, ok := liveFrames[msg.Reply.SourceFrameID]; !ok {
				continue
			}
		}
		filtered = append(filtered, msg)
	}
	out.Messages = filtered
	out.Summary = computeFrameMessagingSummary(out.Messages)
	return out
}

func allocFrameMessageID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UTC().UnixNano())
}

func frameMessagingPreview(body string) string {
	return stateMessagePreview(body)
}

func frameMessagingBodyHash(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		if part == "" {
			continue
		}
		_, _ = h.Write([]byte(part))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (r *Runtime) frameMessagingDir(sessionID state.SessionID) string {
	if r.cfg.DataDir == "" {
		return ""
	}
	return filepath.Join(r.cfg.DataDir, "frame-messaging", string(sessionID))
}

func (r *Runtime) frameMessagingMessagesPath(sessionID state.SessionID) string {
	dir := r.frameMessagingDir(sessionID)
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "messages.jsonl")
}

func (r *Runtime) frameMessagingAuditPath(sessionID state.SessionID) string {
	dir := r.frameMessagingDir(sessionID)
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "audit.jsonl")
}

func appendJSONL(path string, record any) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	return enc.Encode(record)
}

func (r *Runtime) appendFrameMessagingRecord(sessionID state.SessionID, record frameMessagingJournalRecord) error {
	return r.appendJSONL(r.frameMessagingMessagesPath(sessionID), record)
}

func (r *Runtime) appendFrameMessagingAudit(sessionID state.SessionID, record frameMessagingAuditRecord) error {
	return r.appendJSONL(r.frameMessagingAuditPath(sessionID), record)
}

func (r *Runtime) deleteFrameMessagingSession(sessionID state.SessionID) error {
	dir := r.frameMessagingDir(sessionID)
	if dir == "" {
		return nil
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("frame messaging: delete %s: %w", sessionID, err)
	}
	return nil
}

func (r *Runtime) restoreFrameMessagingStores() {
	for sessionID, sess := range r.state.Sessions {
		msgs, err := r.loadFrameMessagingFromJournal(sessionID, sess.FrameMessaging)
		if err != nil {
			slog.Warn("frame messaging: replay failed", "session", sessionID, "err", err)
			continue
		}
		if msgs == nil {
			continue
		}
		msgs = sanitizeFrameMessaging(msgs, liveFrameSet(sess.Frames))
		r.state.Sessions = cloneRuntimeSessions(r.state.Sessions)
		sess.FrameMessaging = msgs
		r.state.Sessions[sessionID] = sess
	}
	r.publishState(r.state)
}

func (r *Runtime) loadFrameMessagingFromJournal(sessionID state.SessionID, base *state.SessionFrameMessaging) (*state.SessionFrameMessaging, error) {
	path := r.frameMessagingMessagesPath(sessionID)
	if path == "" {
		return state.CloneSessionFrameMessaging(base), nil
	}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return state.CloneSessionFrameMessaging(base), nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	out := state.CloneSessionFrameMessaging(base)
	if out == nil {
		out = &state.SessionFrameMessaging{}
	}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 4096), 1<<20)
	for sc.Scan() {
		var rec frameMessagingJournalRecord
		if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
			return nil, err
		}
		applyFrameMessagingJournalRecord(out, rec)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	out.Summary = computeFrameMessagingSummary(out.Messages)
	return out, nil
}

func applyFrameMessagingJournalRecord(out *state.SessionFrameMessaging, rec frameMessagingJournalRecord) {
	switch rec.Kind {
	case frameMessagingKindSent:
		applyFrameMessagingSent(out, rec)
	case frameMessagingKindReply:
		applyFrameMessagingReply(out, rec)
	case frameMessagingKindRead:
		applyFrameMessagingRead(out, rec)
	}
}

func applyFrameMessagingSent(out *state.SessionFrameMessaging, rec frameMessagingJournalRecord) {
	if rec.Message == nil {
		return
	}
	msg := snapshotToFrameMessage(*rec.Message)
	if idx := findFrameMessageIndex(out.Messages, msg.ID); idx >= 0 {
		preserved := out.Messages[idx]
		if preserved.Read {
			msg.Read = true
		}
		if preserved.Reply != nil {
			msg.Reply = preserved.Reply
		}
		if preserved.ReplyStatus != "" {
			msg.ReplyStatus = preserved.ReplyStatus
		}
		if preserved.DeliveryStatus != "" {
			msg.DeliveryStatus = preserved.DeliveryStatus
		}
		out.Messages[idx] = msg
		return
	}
	out.Messages = append(out.Messages, msg)
}

func applyFrameMessagingReply(out *state.SessionFrameMessaging, rec frameMessagingJournalRecord) {
	if rec.Reply == nil || rec.MessageID == "" {
		return
	}
	for i := range out.Messages {
		if out.Messages[i].ID != rec.MessageID {
			continue
		}
		reply := snapshotToFrameReply(*rec.Reply)
		out.Messages[i].Reply = &reply
		if reply.Resolution != "" {
			out.Messages[i].ReplyStatus = reply.Resolution
		} else {
			out.Messages[i].ReplyStatus = "replied"
		}
		out.Messages[i].DeliveryStatus = "replied"
		break
	}
}

func applyFrameMessagingRead(out *state.SessionFrameMessaging, rec frameMessagingJournalRecord) {
	ids := rec.MessageIDs
	if len(ids) == 0 && rec.MessageID != "" {
		ids = []string{rec.MessageID}
	}
	if len(ids) == 0 {
		return
	}
	readSet := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id != "" {
			readSet[id] = struct{}{}
		}
	}
	for i := range out.Messages {
		if _, ok := readSet[out.Messages[i].ID]; ok {
			out.Messages[i].Read = true
		}
	}
}

func findFrameMessageIndex(messages []state.FrameMessage, id string) int {
	for i := range messages {
		if messages[i].ID == id {
			return i
		}
	}
	return -1
}

func snapshotToFrameMessage(in FrameMessageSnapshot) state.FrameMessage {
	createdAt, _ := time.Parse(time.RFC3339, in.CreatedAt)
	msg := state.FrameMessage{
		ID:             in.ID,
		SourceFrameID:  state.FrameID(in.SourceFrameID),
		TargetFrameID:  state.FrameID(in.TargetFrameID),
		Topic:          in.Topic,
		Body:           in.Body,
		Priority:       in.Priority,
		CreatedAt:      createdAt,
		Read:           in.Read,
		ReplyStatus:    in.ReplyStatus,
		DeliveryStatus: in.DeliveryStatus,
	}
	if in.Reply != nil {
		reply := snapshotToFrameReply(*in.Reply)
		msg.Reply = &reply
	}
	return msg
}

func snapshotToFrameReply(in FrameReplySnapshot) state.FrameReply {
	createdAt, _ := time.Parse(time.RFC3339, in.CreatedAt)
	return state.FrameReply{
		ID:                 in.ID,
		SourceFrameID:      state.FrameID(in.SourceFrameID),
		Body:               in.Body,
		FinalAnswer:        in.FinalAnswer,
		CreatedAt:          createdAt,
		Resolution:         in.Resolution,
		Confidence:         in.Confidence,
		FinalAnswerPreview: in.FinalAnswerPreview,
	}
}

func frameMessageToSnapshot(msg state.FrameMessage) FrameMessageSnapshot {
	snap := FrameMessageSnapshot{
		ID:             msg.ID,
		SourceFrameID:  string(msg.SourceFrameID),
		TargetFrameID:  string(msg.TargetFrameID),
		Topic:          msg.Topic,
		Body:           msg.Body,
		Priority:       msg.Priority,
		CreatedAt:      msg.CreatedAt.UTC().Format(time.RFC3339),
		Read:           msg.Read,
		ReplyStatus:    msg.ReplyStatus,
		DeliveryStatus: msg.DeliveryStatus,
	}
	if msg.Reply != nil {
		reply := frameReplyToSnapshot(*msg.Reply)
		snap.Reply = &reply
	}
	return snap
}

func frameReplyToSnapshot(reply state.FrameReply) FrameReplySnapshot {
	return FrameReplySnapshot{
		ID:                 reply.ID,
		SourceFrameID:      string(reply.SourceFrameID),
		Body:               reply.Body,
		FinalAnswer:        reply.FinalAnswer,
		CreatedAt:          reply.CreatedAt.UTC().Format(time.RFC3339),
		Resolution:         reply.Resolution,
		Confidence:         reply.Confidence,
		FinalAnswerPreview: reply.FinalAnswerPreview,
	}
}

func computeFrameMessagingSummary(messages []state.FrameMessage) state.FrameMessagingSummary {
	var summary state.FrameMessagingSummary
	for i := range messages {
		msg := messages[i]
		if !msg.Read {
			summary.UnreadCount++
		}
		if msg.Reply == nil {
			summary.PendingDeliveryCount++
		}
		if body := frameMessagingPreview(msg.Body); body != "" {
			summary.LatestMessagePreview = body
		}
		if msg.Reply != nil {
			if preview := frameMessagingPreview(firstNonEmpty(msg.Reply.FinalAnswer, msg.Reply.Body)); preview != "" {
				summary.LatestReplyPreview = preview
			}
		}
		if msg.ReplyStatus != "" {
			summary.LastDeliveryStatus = msg.ReplyStatus
		} else if msg.DeliveryStatus != "" {
			summary.LastDeliveryStatus = msg.DeliveryStatus
		}
	}
	return summary
}
