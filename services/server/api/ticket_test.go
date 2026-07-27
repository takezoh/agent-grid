package api

import (
	"testing"
	"time"
)

func TestTicketSingleUse(t *testing.T) {
	s := newTicketStore()
	tok, ci := s.mint()
	if tok == "" || ci == "" {
		t.Fatal("mint returned empty ticket or client-instance-id")
	}
	got, ok := s.consume(tok)
	if !ok {
		t.Fatal("first consume should succeed")
	}
	if got != ci {
		t.Fatalf("client-instance-id = %q, want %q", got, ci)
	}
	if _, ok := s.consume(tok); ok {
		t.Fatal("second consume must fail: ticket is single-use")
	}
}

func TestTicketUnknownAndEmpty(t *testing.T) {
	s := newTicketStore()
	if _, ok := s.consume(""); ok {
		t.Fatal("empty ticket must be rejected")
	}
	if _, ok := s.consume("never-minted"); ok {
		t.Fatal("unknown ticket must be rejected")
	}
}

func TestTicketDistinct(t *testing.T) {
	s := newTicketStore()
	a, cia := s.mint()
	b, cib := s.mint()
	if a == b {
		t.Fatal("mint must return distinct tickets")
	}
	if cia == cib {
		t.Fatal("mint must return distinct client-instance-ids")
	}
}

func TestTicketClientInstanceIDNotReused(t *testing.T) {
	s := newTicketStore()
	_, id1 := s.mint()
	_, id2 := s.mint()
	if id1 == id2 {
		t.Fatal("client-instance-id must not be reused across mint calls")
	}
	// After consume + re-mint, ids must still differ from prior ones.
	tok3, id3 := s.mint()
	if _, ok := s.consume(tok3); !ok {
		t.Fatal("consume failed")
	}
	_, id4 := s.mint()
	if id3 == id4 || id1 == id3 || id2 == id4 {
		t.Fatal("client-instance-id reused across reconnect mint")
	}
}

func TestTicketExpires(t *testing.T) {
	now := time.Unix(1_000, 0)
	s := newTicketStore()
	s.now = func() time.Time { return now }

	tok, _ := s.mint()
	now = now.Add(ticketTTL + time.Second) // advance past expiry
	if _, ok := s.consume(tok); ok {
		t.Fatal("expired ticket must be rejected")
	}
}

func TestTicketEvictsExpired(t *testing.T) {
	now := time.Unix(1_000, 0)
	s := newTicketStore()
	s.now = func() time.Time { return now }

	stale, _ := s.mint()
	now = now.Add(ticketTTL + time.Second)
	s.mint() // minting evicts expired entries

	s.mu.Lock()
	_, present := s.m[stale]
	count := len(s.m)
	s.mu.Unlock()
	if present {
		t.Fatal("expired ticket should have been evicted on mint")
	}
	if count != 1 {
		t.Fatalf("store should hold only the fresh ticket, got %d", count)
	}
}
