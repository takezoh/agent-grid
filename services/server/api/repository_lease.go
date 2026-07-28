package api

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

const (
	leaseTTL         = 60 * time.Second
	maxLeasesPerHost = 32
	leaseUsername    = "x-access-token"
)

type leaseRecord struct {
	ID, HostID, RepositoryID, Owner, Name, Token, TokenGeneration string
	ExpiresAt                                                     time.Time
	Consumed, Revoked                                             bool
}

// LeaseIssue is the credential-free admission receipt.
type LeaseIssue struct {
	LeaseID   string    `json:"lease_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

// LeaseFailure is a typed admission/consume rejection.
type LeaseFailure struct{ Code string }

func (f LeaseFailure) Error() string { return f.Code }

// ConsumedLease is one-shot authority for the clone broker.
type ConsumedLease struct {
	LeaseID, RepositoryID, Owner, Name, Username, Token string
}
type leaseStore struct {
	mu   sync.Mutex
	now  func() time.Time
	byID map[string]*leaseRecord
}

func newLeaseStore() *leaseStore {
	return &leaseStore{now: time.Now, byID: map[string]*leaseRecord{}}
}
func (s *leaseStore) Issue(hostID, repositoryID, owner, name, token, generation string) (LeaseIssue, *LeaseFailure) {
	if hostID == "" || repositoryID == "" || owner == "" || name == "" || token == "" {
		return LeaseIssue{}, &LeaseFailure{Code: "invalid_identity"}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictLocked()
	if s.unconsumedLocked(hostID) >= maxLeasesPerHost {
		return LeaseIssue{}, &LeaseFailure{Code: "lease_capacity_exceeded"}
	}
	id, exp := randomLeaseID(), s.now().Add(leaseTTL)
	s.byID[id] = &leaseRecord{ID: id, HostID: hostID, RepositoryID: repositoryID, Owner: owner, Name: name, Token: token, TokenGeneration: generation, ExpiresAt: exp}
	return LeaseIssue{LeaseID: id, ExpiresAt: exp}, nil
}
func (s *leaseStore) Consume(leaseID string) (ConsumedLease, *LeaseFailure) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.byID[leaseID]
	if !ok {
		s.evictLocked()
		return ConsumedLease{}, &LeaseFailure{Code: "lease_not_found"}
	}
	if rec.Revoked {
		return ConsumedLease{}, &LeaseFailure{Code: "lease_revoked"}
	}
	if !s.now().Before(rec.ExpiresAt) {
		rec.Token = ""
		delete(s.byID, leaseID)
		return ConsumedLease{}, &LeaseFailure{Code: "lease_expired"}
	}
	if rec.Consumed {
		return ConsumedLease{}, &LeaseFailure{Code: "lease_replay"}
	}
	s.evictLocked()
	rec.Consumed = true
	out := ConsumedLease{LeaseID: rec.ID, RepositoryID: rec.RepositoryID, Owner: rec.Owner, Name: rec.Name, Username: leaseUsername, Token: rec.Token}
	rec.Token = ""
	return out, nil
}
func (s *leaseStore) Revoke(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rec, ok := s.byID[id]; ok {
		rec.Token = ""
		delete(s.byID, id)
	}
}
func (s *leaseStore) RevokeHost(hostID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, rec := range s.byID {
		if rec.HostID == hostID {
			rec.Token = ""
			delete(s.byID, id)
		}
	}
}
func (s *leaseStore) RevokeGeneration(hostID, generation string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, rec := range s.byID {
		if rec.HostID == hostID && rec.TokenGeneration != generation {
			rec.Token = ""
			delete(s.byID, id)
		}
	}
}
func (s *leaseStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID = map[string]*leaseRecord{}
}
func (s *leaseStore) UnconsumedCount(hostID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictLocked()
	return s.unconsumedLocked(hostID)
}
func (s *leaseStore) unconsumedLocked(hostID string) int {
	n := 0
	for _, rec := range s.byID {
		if rec.HostID == hostID && !rec.Consumed && !rec.Revoked {
			n++
		}
	}
	return n
}
func (s *leaseStore) evictLocked() {
	now := s.now()
	for id, rec := range s.byID {
		if !now.Before(rec.ExpiresAt) {
			rec.Token = ""
			delete(s.byID, id)
		}
	}
}
func randomLeaseID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
