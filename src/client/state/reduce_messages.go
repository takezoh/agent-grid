package state

type SessionMessagesParams struct {
	SessionID         string `json:"session_id"`
	LastReadMessageID string `json:"last_read_message_id,omitempty"`
}

type SessionMessagesReply struct {
	SessionID SessionID
}

func init() {
	RegisterEvent[SessionMessagesParams](EventListSessionMessages, reduceListSessionMessages)
	RegisterEvent[SessionMessagesParams](EventReadSessionMessages, reduceReadSessionMessages)
}

func reduceListSessionMessages(s State, connID ConnID, reqID string, p SessionMessagesParams) (State, []Effect) {
	sid := SessionID(p.SessionID)
	if sid == "" {
		return s, []Effect{errResp(connID, reqID, ErrCodeInvalidArgument, "session_id required")}
	}
	if _, ok := s.Sessions[sid]; !ok {
		return s, []Effect{errResp(connID, reqID, ErrCodeNotFound, "session not found")}
	}
	return s, []Effect{EffSendResponseSync{
		ConnID: connID,
		ReqID:  reqID,
		Body:   SessionMessagesReply{SessionID: sid},
	}}
}

func reduceReadSessionMessages(s State, connID ConnID, reqID string, p SessionMessagesParams) (State, []Effect) {
	sid := SessionID(p.SessionID)
	if sid == "" {
		return s, []Effect{errResp(connID, reqID, ErrCodeInvalidArgument, "session_id required")}
	}
	sess, ok := s.Sessions[sid]
	if !ok {
		return s, []Effect{errResp(connID, reqID, ErrCodeNotFound, "session not found")}
	}
	if sess.FrameMessaging == nil {
		return s, []Effect{okResp(connID, reqID, nil)}
	}
	nextFrameMessaging, changed := markSessionMessagesReadThroughID(sess.FrameMessaging, p.LastReadMessageID)
	if !changed {
		return s, []Effect{okResp(connID, reqID, nil)}
	}
	readIDs := make([]string, 0, len(nextFrameMessaging.Messages))
	for i := range nextFrameMessaging.Messages {
		if nextFrameMessaging.Messages[i].Read && !sess.FrameMessaging.Messages[i].Read {
			readIDs = append(readIDs, nextFrameMessaging.Messages[i].ID)
		}
	}
	s.Sessions = cloneSessions(s.Sessions)
	sess.FrameMessaging = nextFrameMessaging
	s.Sessions[sid] = sess
	effs := []Effect{
		okResp(connID, reqID, nil),
		EffPersistSnapshot{},
		EffBroadcastSessionsChanged{},
	}
	if len(readIDs) > 0 {
		effs = append(effs, EffFrameMessagingPersistRead{SessionID: sid, MessageIDs: readIDs})
	}
	return s, effs
}
