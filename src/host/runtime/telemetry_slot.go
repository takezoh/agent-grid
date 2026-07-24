package runtime

import (
	"context"
	"sync"
	"time"

	"github.com/takezoh/agent-grid/host/proto"
)

// TelemetryRecord is metadata-only evidence. It deliberately carries no
// terminal bytes or bearer material.
type TelemetryRecord struct {
	Correlation proto.PublicCorrelation
	Producer    string
	Watermark   uint64
	Sequence    uint64
	Digest      string
	DropCount   uint64
	Unknown     bool
}

// DirtyTelemetrySlot is a capacity-one latest-value slot. Publish and
// SnapshotAndClear are one critical section, so an update racing with a
// snapshot remains dirty for the next flush instead of being lost.
type DirtyTelemetrySlot struct {
	mu     sync.Mutex
	latest TelemetryRecord
	dirty  bool
	drops  uint64
}

func (s *DirtyTelemetrySlot) Publish(record TelemetryRecord) {
	s.mu.Lock()
	if s.dirty {
		s.drops++
	}
	record.DropCount = s.drops
	s.latest = record
	s.dirty = true
	s.mu.Unlock()
}

func (s *DirtyTelemetrySlot) SnapshotAndClear() (TelemetryRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.dirty {
		return TelemetryRecord{}, false
	}
	record := s.latest
	s.dirty = false
	return record, true
}

// StartTelemetryFlusher emits at most one latest record every 250ms. The
// first record is emitted on the first tick, bounding low-rate liveness while
// coalescing sustained bursts.
func StartTelemetryFlusher(ctx context.Context, slot *DirtyTelemetrySlot, emit func(TelemetryRecord)) {
	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if record, ok := slot.SnapshotAndClear(); ok {
					emit(record)
				}
			}
		}
	}()
}
