package runtime

import (
	"strings"

	"github.com/takezoh/agent-grid/host/proto"
	cstream "github.com/takezoh/agent-grid/host/runtime/subsystem/stream"
	"github.com/takezoh/agent-grid/host/state"
)

type internalFrameListByThreadRequest struct {
	sessionID state.SessionID
	threadID  string
	reply     chan frameListResult
}

func (internalFrameListByThreadRequest) isInternalEvent() {}

type internalFrameReadByThreadRequest struct {
	sessionID   state.SessionID
	threadID    string
	peerFrameID state.FrameID
	reply       chan frameReadResult
}

func (internalFrameReadByThreadRequest) isInternalEvent() {}

type internalFrameSendByThreadRequest struct {
	sessionID     state.SessionID
	threadID      string
	targetFrameID state.FrameID
	topic         string
	body          string
	priority      string
	reply         chan frameSendResult
}

func (internalFrameSendByThreadRequest) isInternalEvent() {}

type internalFrameReplyByThreadRequest struct {
	sessionID   state.SessionID
	threadID    string
	messageID   string
	body        string
	finalAnswer string
	resolution  string
	confidence  string
	reply       chan frameReplyResult
}

func (internalFrameReplyByThreadRequest) isInternalEvent() {}

func (r *Runtime) frameIDForThread(sessionID state.SessionID, threadID string) (state.FrameID, error) {
	if strings.TrimSpace(string(sessionID)) == "" {
		return "", &proto.ErrorBody{Code: proto.ErrInvalidArgument, Message: "session id required"}
	}
	threadID = strings.TrimSpace(threadID)
	if threadID == "" {
		return "", &proto.ErrorBody{Code: proto.ErrInvalidArgument, Message: "thread id required"}
	}
	factory, ok := r.subsystemFactories[state.LaunchSubsystemStream].(*cstream.Factory)
	if !ok {
		return "", &proto.ErrorBody{Code: proto.ErrInternal, Message: "stream subsystem unavailable"}
	}
	frameID, ok := factory.FindFrameByThread(sessionID, threadID)
	if !ok || frameID == "" {
		return "", &proto.ErrorBody{Code: proto.ErrNotFound, Message: "thread not bound to a frame"}
	}
	return frameID, nil
}

func (r *Runtime) frameMessagingListByThread(sessionID state.SessionID, threadID string) frameListResult {
	frameID, err := r.frameIDForThread(sessionID, threadID)
	if err != nil {
		return frameListResult{err: err}
	}
	return r.frameMessagingList(frameID)
}

func (r *Runtime) frameMessagingReadByThread(sessionID state.SessionID, threadID string, peerFrameID state.FrameID) frameReadResult {
	frameID, err := r.frameIDForThread(sessionID, threadID)
	if err != nil {
		return frameReadResult{err: err}
	}
	return r.frameMessagingRead(frameID, peerFrameID)
}

func (r *Runtime) frameMessagingSendByThread(sessionID state.SessionID, threadID string, targetFrameID state.FrameID, topic, body, priority string) frameSendResult {
	frameID, err := r.frameIDForThread(sessionID, threadID)
	if err != nil {
		return frameSendResult{err: err}
	}
	return r.frameMessagingSend(frameID, targetFrameID, topic, body, priority)
}

func (r *Runtime) frameMessagingReplyByThread(sessionID state.SessionID, threadID, messageID, body, finalAnswer, resolution, confidence string) frameReplyResult {
	frameID, err := r.frameIDForThread(sessionID, threadID)
	if err != nil {
		return frameReplyResult{err: err}
	}
	return r.frameMessagingReply(frameID, messageID, body, finalAnswer, resolution, confidence)
}

func (r *Runtime) ListByThread(sessionID state.SessionID, threadID string) (proto.RespFrameList, error) {
	reply := make(chan frameListResult, 1)
	if !r.enqueueInternal(internalFrameListByThreadRequest{sessionID: sessionID, threadID: threadID, reply: reply}) {
		return proto.RespFrameList{}, &proto.ErrorBody{Code: proto.ErrInternal, Message: "frame messaging request dropped"}
	}
	res := <-reply
	return res.resp, res.err
}

func (r *Runtime) ReadByThread(sessionID state.SessionID, threadID string, peer state.FrameID) (proto.RespFrameRead, error) {
	reply := make(chan frameReadResult, 1)
	if !r.enqueueInternal(internalFrameReadByThreadRequest{sessionID: sessionID, threadID: threadID, peerFrameID: peer, reply: reply}) {
		return proto.RespFrameRead{}, &proto.ErrorBody{Code: proto.ErrInternal, Message: "frame messaging request dropped"}
	}
	res := <-reply
	return res.resp, res.err
}

func (r *Runtime) SendByThread(sessionID state.SessionID, threadID string, target state.FrameID, topic, body, priority string) (proto.RespFrameSend, error) {
	reply := make(chan frameSendResult, 1)
	if !r.enqueueInternal(internalFrameSendByThreadRequest{
		sessionID:     sessionID,
		threadID:      threadID,
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

func (r *Runtime) ReplyByThread(sessionID state.SessionID, threadID, messageID, body, finalAnswer, resolution, confidence string) (proto.RespFrameReply, error) {
	reply := make(chan frameReplyResult, 1)
	if !r.enqueueInternal(internalFrameReplyByThreadRequest{
		sessionID:   sessionID,
		threadID:    threadID,
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
