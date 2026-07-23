package runtime

import (
	"log/slog"
	"strings"
	"time"

	"github.com/takezoh/agent-grid/host/proto"
	"github.com/takezoh/agent-grid/host/state"
)

type frameListResult struct {
	resp proto.RespFrameList
	err  error
}

type frameReadResult struct {
	resp proto.RespFrameRead
	err  error
}

type frameSendResult struct {
	resp proto.RespFrameSend
	err  error
}

type frameReplyResult struct {
	resp proto.RespFrameReply
	err  error
}

type internalFrameListRequest struct {
	source state.FrameID
	reply  chan frameListResult
}

func (internalFrameListRequest) isInternalEvent() {}

type internalFrameReadRequest struct {
	source      state.FrameID
	peerFrameID state.FrameID
	reply       chan frameReadResult
}

func (internalFrameReadRequest) isInternalEvent() {}

type internalFrameSendRequest struct {
	source        state.FrameID
	targetFrameID state.FrameID
	topic         string
	body          string
	priority      string
	reply         chan frameSendResult
}

func (internalFrameSendRequest) isInternalEvent() {}

type internalFrameReplyRequest struct {
	source      state.FrameID
	messageID   string
	body        string
	finalAnswer string
	resolution  string
	confidence  string
	reply       chan frameReplyResult
}

func (internalFrameReplyRequest) isInternalEvent() {}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func messageToProto(msg state.FrameMessage) proto.SessionMessage {
	out := proto.SessionMessage{
		ID:                 msg.ID,
		SourceFrameID:      string(msg.SourceFrameID),
		TargetFrameID:      string(msg.TargetFrameID),
		Topic:              msg.Topic,
		Priority:           msg.Priority,
		Body:               msg.Body,
		BodyPreview:        frameMessagingPreview(msg.Body),
		CreatedAt:          msg.CreatedAt.UTC().Format(time.RFC3339),
		Read:               msg.Read,
		ReplyStatus:        msg.ReplyStatus,
		DeliveryStatus:     msg.DeliveryStatus,
		FinalAnswerPreview: "",
	}
	if out.ReplyStatus == "" {
		switch {
		case msg.Reply != nil && msg.Reply.Resolution != "":
			out.ReplyStatus = msg.Reply.Resolution
		case msg.Reply != nil:
			out.ReplyStatus = "replied"
		default:
			out.ReplyStatus = "pending"
		}
	}
	if msg.Reply != nil {
		out.FinalAnswerPreview = msg.Reply.FinalAnswerPreview
		out.Reply = &proto.SessionMessageReply{
			ID:                 msg.Reply.ID,
			SourceFrameID:      string(msg.Reply.SourceFrameID),
			Body:               msg.Reply.Body,
			BodyPreview:        frameMessagingPreview(msg.Reply.Body),
			FinalAnswer:        msg.Reply.FinalAnswer,
			CreatedAt:          msg.Reply.CreatedAt.UTC().Format(time.RFC3339),
			Resolution:         msg.Reply.Resolution,
			Confidence:         msg.Reply.Confidence,
			FinalAnswerPreview: msg.Reply.FinalAnswerPreview,
		}
	}
	return out
}

func (r *Runtime) frameSessionForSource(frameID state.FrameID) (state.SessionID, state.Session, error) {
	for sessionID, sess := range r.state.Sessions {
		for _, frame := range sess.Frames {
			if frame.ID != frameID {
				continue
			}
			if !isAgentFrame(frame) {
				return "", state.Session{}, &proto.ErrorBody{Code: proto.ErrUnsupported, Message: "frame messaging is limited to managed agent frames"}
			}
			return sessionID, sess, nil
		}
	}
	return "", state.Session{}, &proto.ErrorBody{Code: proto.ErrNotFound, Message: "source frame not found"}
}

func frameByID(sess state.Session, frameID state.FrameID) (state.SessionFrame, bool) {
	for _, frame := range sess.Frames {
		if frame.ID == frameID {
			return frame, true
		}
	}
	return state.SessionFrame{}, false
}

func findFrameSession(sessions map[state.SessionID]state.Session, frameID state.FrameID) (state.SessionID, state.SessionFrame, bool) {
	for sessionID, sess := range sessions {
		for _, frame := range sess.Frames {
			if frame.ID == frameID {
				return sessionID, frame, true
			}
		}
	}
	return "", state.SessionFrame{}, false
}

func (r *Runtime) commitFrameMessagingSession(sessionID state.SessionID, sess state.Session) {
	r.state.Sessions = cloneRuntimeSessions(r.state.Sessions)
	r.state.Sessions[sessionID] = sess
	r.publishState(r.state)
	r.broadcastSessionsChanged()
	if err := r.cfg.Persist.Save(r.snapshotSessions()); err != nil {
		slog.Error("frame messaging: persist snapshot failed", "session", sessionID, "err", err)
	}
}

func (r *Runtime) frameMessagingList(source state.FrameID) frameListResult {
	sessionID, sess, err := r.frameSessionForSource(source)
	if err != nil {
		return frameListResult{err: err}
	}
	frames := make([]proto.FrameRef, 0, len(sess.Frames))
	for _, frame := range sess.Frames {
		if !isAgentFrame(frame) {
			continue
		}
		frames = append(frames, proto.FrameRef{
			SessionID: string(sessionID),
			FrameID:   string(frame.ID),
			Command:   frame.Command,
			Project:   frame.Project,
			Sendable:  frame.ID != source,
		})
	}
	return frameListResult{resp: proto.RespFrameList{Frames: frames}}
}

func (r *Runtime) frameMessagingRead(source, peerFrameID state.FrameID) frameReadResult {
	sessionID, sess, err := r.frameSessionForSource(source)
	if err != nil {
		return frameReadResult{err: err}
	}
	if peerFrameID != "" {
		target, ok := frameByID(sess, peerFrameID)
		if !ok || !isAgentFrame(target) {
			return frameReadResult{err: &proto.ErrorBody{Code: proto.ErrNotFound, Message: "peer frame not found"}}
		}
	}
	if sess.FrameMessaging == nil {
		return frameReadResult{resp: proto.RespFrameRead{SessionID: string(sessionID), Messages: []proto.SessionMessage{}}}
	}

	next := state.CloneSessionFrameMessaging(sess.FrameMessaging)
	if next == nil {
		next = &state.SessionFrameMessaging{}
	}
	readIDs := make([]string, 0, len(next.Messages))
	readIndexes := make([]int, 0, len(next.Messages))
	var messages []proto.SessionMessage
	for i := range next.Messages {
		msg := next.Messages[i]
		if msg.SourceFrameID != source && msg.TargetFrameID != source {
			continue
		}
		if peerFrameID != "" && msg.SourceFrameID != peerFrameID && msg.TargetFrameID != peerFrameID {
			continue
		}
		if msg.TargetFrameID == source && !msg.Read {
			readIDs = append(readIDs, msg.ID)
			readIndexes = append(readIndexes, i)
		}
		messages = append(messages, messageToProto(msg))
	}
	if len(readIDs) > 0 {
		if err := r.appendFrameMessagingRecord(sessionID, frameMessagingJournalRecord{
			Kind:       frameMessagingKindRead,
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
			MessageIDs: readIDs,
		}); err != nil {
			return frameReadResult{err: &proto.ErrorBody{Code: proto.ErrInternal, Message: err.Error()}}
		}
		for _, idx := range readIndexes {
			next.Messages[idx].Read = true
		}
		next.Summary = computeFrameMessagingSummary(next.Messages)
		sess.FrameMessaging = next
		r.commitFrameMessagingSession(sessionID, sess)
		messages = messages[:0]
		for i := range next.Messages {
			msg := next.Messages[i]
			if msg.SourceFrameID != source && msg.TargetFrameID != source {
				continue
			}
			if peerFrameID != "" && msg.SourceFrameID != peerFrameID && msg.TargetFrameID != peerFrameID {
				continue
			}
			messages = append(messages, messageToProto(msg))
		}
	}
	_ = r.appendFrameMessagingAudit(sessionID, frameMessagingAuditRecord{
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		ToolName:      "agent_frames.read",
		SessionID:     string(sessionID),
		SourceFrameID: string(source),
		TargetFrameID: string(peerFrameID),
		Decision:      frameMessagingAuditAllow,
	})
	return frameReadResult{resp: proto.RespFrameRead{SessionID: string(sessionID), Messages: messages}}
}

func (r *Runtime) frameMessagingSend(source, targetFrameID state.FrameID, topic, body, priority string) frameSendResult {
	sessionID, sess, err := r.frameSessionForSource(source)
	if err != nil {
		return frameSendResult{err: err}
	}
	audit := frameMessagingAuditRecord{
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		ToolName:      "agent_frames.send_message",
		SessionID:     string(sessionID),
		SourceFrameID: string(source),
		TargetFrameID: string(targetFrameID),
		BodyHash:      frameMessagingBodyHash(body),
	}
	if strings.TrimSpace(body) == "" {
		audit.Decision = frameMessagingAuditReject
		audit.Reason = "empty_body"
		_ = r.appendFrameMessagingAudit(sessionID, audit)
		return frameSendResult{err: &proto.ErrorBody{Code: proto.ErrInvalidArgument, Message: "body required"}}
	}
	target, ok := frameByID(sess, targetFrameID)
	if !ok {
		if otherSessionID, _, exists := findFrameSession(r.state.Sessions, targetFrameID); exists && otherSessionID != sessionID {
			audit.Decision = frameMessagingAuditReject
			audit.Reason = "cross_session_target"
			_ = r.appendFrameMessagingAudit(sessionID, audit)
			return frameSendResult{err: &proto.ErrorBody{Code: proto.ErrInvalidArgument, Message: "target must be in the same session"}}
		}
		audit.Decision = frameMessagingAuditReject
		audit.Reason = "target_not_found"
		_ = r.appendFrameMessagingAudit(sessionID, audit)
		return frameSendResult{err: &proto.ErrorBody{Code: proto.ErrNotFound, Message: "target frame not found"}}
	}
	if targetFrameID == source {
		audit.Decision = frameMessagingAuditReject
		audit.Reason = "self_target"
		_ = r.appendFrameMessagingAudit(sessionID, audit)
		return frameSendResult{err: &proto.ErrorBody{Code: proto.ErrInvalidArgument, Message: "self-target is not allowed"}}
	}
	if !isAgentFrame(target) {
		audit.Decision = frameMessagingAuditReject
		audit.Reason = "target_not_agent_frame"
		_ = r.appendFrameMessagingAudit(sessionID, audit)
		return frameSendResult{err: &proto.ErrorBody{Code: proto.ErrUnsupported, Message: "target must be a managed agent frame"}}
	}

	msg := state.FrameMessage{
		ID:             allocFrameMessageID("msg"),
		SourceFrameID:  source,
		TargetFrameID:  targetFrameID,
		Topic:          topic,
		Body:           body,
		Priority:       priority,
		CreatedAt:      time.Now().UTC(),
		ReplyStatus:    "pending",
		DeliveryStatus: "pending",
	}
	if sess.FrameMessaging == nil {
		sess.FrameMessaging = &state.SessionFrameMessaging{}
	} else {
		sess.FrameMessaging = state.CloneSessionFrameMessaging(sess.FrameMessaging)
	}
	sess.FrameMessaging.Messages = append(sess.FrameMessaging.Messages, msg)
	sess.FrameMessaging.Summary = computeFrameMessagingSummary(sess.FrameMessaging.Messages)
	if err := r.appendFrameMessagingRecord(sessionID, frameMessagingJournalRecord{
		Kind:      frameMessagingKindSent,
		Timestamp: msg.CreatedAt.Format(time.RFC3339),
		Message:   ptrFrameMessageSnapshot(frameMessageToSnapshot(msg)),
	}); err != nil {
		return frameSendResult{err: &proto.ErrorBody{Code: proto.ErrInternal, Message: err.Error()}}
	}
	audit.Decision = frameMessagingAuditAllow
	_ = r.appendFrameMessagingAudit(sessionID, audit)
	r.commitFrameMessagingSession(sessionID, sess)
	return frameSendResult{resp: proto.RespFrameSend{SessionID: string(sessionID), Message: messageToProto(msg)}}
}

func (r *Runtime) frameMessagingReply(source state.FrameID, messageID, body, finalAnswer, resolution, confidence string) frameReplyResult {
	sessionID, sess, err := r.frameSessionForSource(source)
	if err != nil {
		return frameReplyResult{err: err}
	}
	audit := frameMessagingAuditRecord{
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		ToolName:      "agent_frames.reply",
		SessionID:     string(sessionID),
		SourceFrameID: string(source),
		MessageID:     messageID,
		BodyHash:      frameMessagingBodyHash(body, finalAnswer),
	}
	if sess.FrameMessaging == nil {
		audit.Decision = frameMessagingAuditReject
		audit.Reason = "message_not_found"
		_ = r.appendFrameMessagingAudit(sessionID, audit)
		return frameReplyResult{err: &proto.ErrorBody{Code: proto.ErrNotFound, Message: "message not found"}}
	}
	next := state.CloneSessionFrameMessaging(sess.FrameMessaging)
	found := -1
	for i := range next.Messages {
		if next.Messages[i].ID == messageID {
			found = i
			break
		}
	}
	if found == -1 {
		audit.Decision = frameMessagingAuditReject
		audit.Reason = "message_not_found"
		_ = r.appendFrameMessagingAudit(sessionID, audit)
		return frameReplyResult{err: &proto.ErrorBody{Code: proto.ErrNotFound, Message: "message not found"}}
	}
	msg := next.Messages[found]
	audit.TargetFrameID = string(msg.SourceFrameID)
	if msg.TargetFrameID != source {
		audit.Decision = frameMessagingAuditReject
		audit.Reason = "not_message_target"
		_ = r.appendFrameMessagingAudit(sessionID, audit)
		return frameReplyResult{err: &proto.ErrorBody{Code: proto.ErrInvalidArgument, Message: "only the target frame may reply"}}
	}
	if err := r.rejectInvalidFrameReply(sessionID, msg, body, finalAnswer, resolution, &audit); err != nil {
		return frameReplyResult{err: err}
	}
	reply := state.FrameReply{
		ID:                 allocFrameMessageID("reply"),
		SourceFrameID:      source,
		Body:               body,
		FinalAnswer:        finalAnswer,
		CreatedAt:          time.Now().UTC(),
		Resolution:         resolution,
		Confidence:         firstNonEmpty(confidence, "high"),
		FinalAnswerPreview: frameMessagingPreview(firstNonEmpty(finalAnswer, body)),
	}
	next.Messages[found].Reply = &reply
	next.Messages[found].ReplyStatus = firstNonEmpty(resolution, "replied")
	next.Messages[found].DeliveryStatus = "replied"
	next.Summary = computeFrameMessagingSummary(next.Messages)
	sess.FrameMessaging = next
	if err := r.appendFrameMessagingRecord(sessionID, frameMessagingJournalRecord{
		Kind:      frameMessagingKindReply,
		Timestamp: reply.CreatedAt.Format(time.RFC3339),
		MessageID: messageID,
		Reply:     ptrFrameReplySnapshot(frameReplyToSnapshot(reply)),
	}); err != nil {
		return frameReplyResult{err: &proto.ErrorBody{Code: proto.ErrInternal, Message: err.Error()}}
	}
	audit.Decision = frameMessagingAuditAllow
	_ = r.appendFrameMessagingAudit(sessionID, audit)
	r.commitFrameMessagingSession(sessionID, sess)
	return frameReplyResult{
		resp: proto.RespFrameReply{
			SessionID: string(sessionID),
			MessageID: messageID,
			Reply:     *messageToProto(next.Messages[found]).Reply,
		},
	}
}

func (r *Runtime) rejectInvalidFrameReply(sessionID state.SessionID, msg state.FrameMessage, body, finalAnswer, resolution string, audit *frameMessagingAuditRecord) error {
	if msg.Reply != nil {
		audit.Decision = frameMessagingAuditReject
		audit.Reason = "reply_already_exists"
		_ = r.appendFrameMessagingAudit(sessionID, *audit)
		return &proto.ErrorBody{Code: proto.ErrInvalidArgument, Message: "message already has a reply"}
	}
	if strings.TrimSpace(body) == "" && strings.TrimSpace(finalAnswer) == "" && strings.TrimSpace(resolution) == "" {
		audit.Decision = frameMessagingAuditReject
		audit.Reason = "empty_reply"
		_ = r.appendFrameMessagingAudit(sessionID, *audit)
		return &proto.ErrorBody{Code: proto.ErrInvalidArgument, Message: "reply body, final answer, or resolution required"}
	}
	return nil
}

func ptrFrameMessageSnapshot(in FrameMessageSnapshot) *FrameMessageSnapshot { return &in }

func ptrFrameReplySnapshot(in FrameReplySnapshot) *FrameReplySnapshot { return &in }

func (r *Runtime) List(source state.FrameID) (proto.RespFrameList, error) {
	reply := make(chan frameListResult, 1)
	if !r.enqueueInternal(internalFrameListRequest{source: source, reply: reply}) {
		return proto.RespFrameList{}, &proto.ErrorBody{Code: proto.ErrInternal, Message: "frame messaging request dropped"}
	}
	res := <-reply
	return res.resp, res.err
}

func (r *Runtime) Read(source, peer state.FrameID) (proto.RespFrameRead, error) {
	reply := make(chan frameReadResult, 1)
	if !r.enqueueInternal(internalFrameReadRequest{source: source, peerFrameID: peer, reply: reply}) {
		return proto.RespFrameRead{}, &proto.ErrorBody{Code: proto.ErrInternal, Message: "frame messaging request dropped"}
	}
	res := <-reply
	return res.resp, res.err
}

func (r *Runtime) Send(source, target state.FrameID, topic, body, priority string) (proto.RespFrameSend, error) {
	reply := make(chan frameSendResult, 1)
	if !r.enqueueInternal(internalFrameSendRequest{
		source:        source,
		targetFrameID: target,
		topic:         topic,
		body:          body,
		priority:      priority,
		reply:         reply,
	}) {
		return proto.RespFrameSend{}, &proto.ErrorBody{Code: proto.ErrInternal, Message: "frame messaging request dropped"}
	}
	res := <-reply
	return res.resp, res.err
}

func (r *Runtime) Reply(source state.FrameID, messageID, body, finalAnswer, resolution, confidence string) (proto.RespFrameReply, error) {
	reply := make(chan frameReplyResult, 1)
	if !r.enqueueInternal(internalFrameReplyRequest{
		source:      source,
		messageID:   messageID,
		body:        body,
		finalAnswer: finalAnswer,
		resolution:  resolution,
		confidence:  confidence,
		reply:       reply,
	}) {
		return proto.RespFrameReply{}, &proto.ErrorBody{Code: proto.ErrInternal, Message: "frame messaging request dropped"}
	}
	res := <-reply
	return res.resp, res.err
}
