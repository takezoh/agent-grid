package state

import (
	"strings"
	"time"
)

// SessionFrameMessaging is the session-scoped durable messaging payload the
// web surface reads. Phase 1 keeps the state intentionally minimal: the broker
// task is the authority for populating Messages and Summary, while the web
// read-path only marks the session summary/messages as read.
type SessionFrameMessaging struct {
	Summary  FrameMessagingSummary `json:"summary"`
	Messages []FrameMessage        `json:"messages,omitempty"`
}

type FrameMessagingSummary struct {
	UnreadCount          int    `json:"unread_count"`
	LatestMessagePreview string `json:"latest_message_preview,omitempty"`
	LatestReplyPreview   string `json:"latest_reply_preview,omitempty"`
	PendingDeliveryCount int    `json:"pending_delivery_count"`
	LastDeliveryStatus   string `json:"last_delivery_status,omitempty"`
}

type FrameMessage struct {
	ID             string      `json:"id"`
	SourceFrameID  FrameID     `json:"source_frame_id"`
	TargetFrameID  FrameID     `json:"target_frame_id"`
	Topic          string      `json:"topic,omitempty"`
	Body           string      `json:"body,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
	Read           bool        `json:"read,omitempty"`
	ReplyStatus    string      `json:"reply_status,omitempty"`
	DeliveryStatus string      `json:"delivery_status,omitempty"`
	Reply          *FrameReply `json:"reply,omitempty"`
}

type FrameReply struct {
	ID                 string    `json:"id"`
	SourceFrameID      FrameID   `json:"source_frame_id"`
	Body               string    `json:"body,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	Resolution         string    `json:"resolution,omitempty"`
	FinalAnswerPreview string    `json:"final_answer_preview,omitempty"`
}

func cloneSessionFrameMessaging(in *SessionFrameMessaging) *SessionFrameMessaging {
	if in == nil {
		return nil
	}
	out := &SessionFrameMessaging{
		Summary:  in.Summary,
		Messages: make([]FrameMessage, len(in.Messages)),
	}
	for i, msg := range in.Messages {
		out.Messages[i] = msg
		if msg.Reply != nil {
			reply := *msg.Reply
			out.Messages[i].Reply = &reply
		}
	}
	return out
}

func markSessionMessagesReadThroughID(in *SessionFrameMessaging, lastReadMessageID string) (*SessionFrameMessaging, bool) {
	if in == nil || lastReadMessageID == "" {
		return in, false
	}
	boundary := -1
	for i, msg := range in.Messages {
		if msg.ID == lastReadMessageID {
			boundary = i
			break
		}
	}
	if boundary == -1 {
		return in, false
	}
	out := cloneSessionFrameMessaging(in)
	changed := false
	for i := 0; i <= boundary; i++ {
		if !out.Messages[i].Read {
			out.Messages[i].Read = true
			changed = true
		}
	}
	unreadCount := 0
	for i := range out.Messages {
		if !out.Messages[i].Read {
			unreadCount++
		}
	}
	if !changed && out.Summary.UnreadCount == unreadCount {
		return in, false
	}
	out.Summary.UnreadCount = unreadCount
	return out, true
}

func messagePreview(body string) string {
	body = strings.Join(strings.Fields(body), " ")
	if len(body) <= 120 {
		return body
	}
	return body[:117] + "..."
}
