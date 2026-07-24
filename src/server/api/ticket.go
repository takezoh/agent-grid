package api

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

// ticketTTL bounds how long a minted WebSocket ticket stays valid. Tickets are
// single-use; the TTL only caps the window during which an unused one is
// honoured.
const ticketTTL = 30 * time.Second

// ticketEntry binds a single-use WS ticket to the ephemeral client-instance-id
// minted with it (FR-P0-12 / adr-20260724-approval-answerer-identity-per-ws-instance).
type ticketEntry struct {
	expiry           time.Time
	clientInstanceID string
}

// ticketStore mints and validates short-lived, single-use tickets that let a
// browser authenticate a WebSocket connection without putting the bearer token
// in the URL. A ticket is minted over the header-authenticated API and consumed
// (removed) on the first /ws connection, so a ticket that leaks into a log is
// useless after one use or ticketTTL, whichever comes first.
//
// Phase 0 also mints a per-WS-connection ephemeral client-instance-id bound to
// the ticket for the WS lifetime. The id is returned on consume and never
// reused across connections.
type ticketStore struct {
	now func() time.Time // injectable clock for tests
	mu  sync.Mutex
	m   map[string]ticketEntry // ticket → entry
}

func newTicketStore() *ticketStore {
	return &ticketStore{now: time.Now, m: make(map[string]ticketEntry)}
}

func randomOpaque24() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// mint returns a fresh single-use ticket and a bound client-instance-id,
// both valid for ticketTTL until consume.
func (s *ticketStore) mint() (ticket string, clientInstanceID string) {
	tok := randomOpaque24()
	ci := randomOpaque24()

	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictExpiredLocked()
	s.m[tok] = ticketEntry{
		expiry:           s.now().Add(ticketTTL),
		clientInstanceID: ci,
	}
	return tok, ci
}

// consume validates and removes a ticket, returning the bound
// client-instance-id exactly once per minted, unexpired ticket.
// ok is false for empty, unknown, already-used, or expired tickets.
func (s *ticketStore) consume(tok string) (clientInstanceID string, ok bool) {
	if tok == "" {
		return "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, found := s.m[tok]
	if !found {
		return "", false
	}
	delete(s.m, tok)
	if !s.now().Before(entry.expiry) {
		return "", false
	}
	return entry.clientInstanceID, true
}

// evictExpiredLocked drops expired tickets so the map cannot grow without bound
// from minted-but-never-used tickets. The caller holds s.mu.
func (s *ticketStore) evictExpiredLocked() {
	now := s.now()
	for tok, entry := range s.m {
		if !now.Before(entry.expiry) {
			delete(s.m, tok)
		}
	}
}
