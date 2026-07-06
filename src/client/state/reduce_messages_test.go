package state

import "testing"

func TestReadSessionMessagesMarksOnlyFetchedRange(t *testing.T) {
	s := New()
	s.Sessions["s1"] = Session{
		ID: "s1",
		FrameMessaging: &SessionFrameMessaging{
			Summary: FrameMessagingSummary{
				UnreadCount:          2,
				PendingDeliveryCount: 1,
			},
			Messages: []FrameMessage{
				{ID: "m1", Read: false},
				{ID: "m2", Read: false},
				{ID: "m3", Read: false},
			},
		},
	}

	next, effs := Reduce(s, EvEvent{
		ConnID: 1,
		ReqID:  "r1",
		Event:  EventReadSessionMessages,
		Payload: mustPayload(map[string]string{
			"session_id":           "s1",
			"last_read_message_id": "m2",
		}),
	})

	msgs := next.Sessions["s1"].FrameMessaging.Messages
	if !msgs[0].Read || !msgs[1].Read {
		t.Fatalf("messages through m2 must be marked read: %+v", msgs)
	}
	if msgs[2].Read {
		t.Fatalf("message after fetched range must stay unread: %+v", msgs)
	}
	if got := next.Sessions["s1"].FrameMessaging.Summary.UnreadCount; got != 1 {
		t.Fatalf("UnreadCount = %d, want 1", got)
	}
	if _, ok := findEff[EffPersistSnapshot](effs); !ok {
		t.Fatal("expected EffPersistSnapshot")
	}
	if _, ok := findEff[EffBroadcastSessionsChanged](effs); !ok {
		t.Fatal("expected EffBroadcastSessionsChanged")
	}
	mustOK(t, effs)
}

func TestReadSessionMessagesWithoutMatchingBoundaryIsNoOp(t *testing.T) {
	s := New()
	s.Sessions["s1"] = Session{
		ID: "s1",
		FrameMessaging: &SessionFrameMessaging{
			Summary: FrameMessagingSummary{UnreadCount: 2},
			Messages: []FrameMessage{
				{ID: "m1", Read: false},
				{ID: "m2", Read: false},
			},
		},
	}

	next, effs := Reduce(s, EvEvent{
		ConnID: 1,
		ReqID:  "r1",
		Event:  EventReadSessionMessages,
		Payload: mustPayload(map[string]string{
			"session_id":           "s1",
			"last_read_message_id": "missing",
		}),
	})

	msgs := next.Sessions["s1"].FrameMessaging.Messages
	if msgs[0].Read || msgs[1].Read {
		t.Fatalf("messages must stay unread on unknown boundary: %+v", msgs)
	}
	if got := next.Sessions["s1"].FrameMessaging.Summary.UnreadCount; got != 2 {
		t.Fatalf("UnreadCount = %d, want 2", got)
	}
	if _, ok := findEff[EffPersistSnapshot](effs); ok {
		t.Fatal("did not expect EffPersistSnapshot")
	}
	if _, ok := findEff[EffBroadcastSessionsChanged](effs); ok {
		t.Fatal("did not expect EffBroadcastSessionsChanged")
	}
	mustOK(t, effs)
}
