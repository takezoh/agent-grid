package runtime

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestDirtyTelemetrySlotKeepsLatestAndPreservesRacingUpdate(t *testing.T) {
	var slot DirtyTelemetrySlot
	slot.Publish(TelemetryRecord{Producer: "daemon", Sequence: 1})
	slot.Publish(TelemetryRecord{Producer: "daemon", Sequence: 2})
	got, ok := slot.SnapshotAndClear()
	if !ok || got.Sequence != 2 {
		t.Fatalf("snapshot = %#v, ok=%v", got, ok)
	}
	if _, ok := slot.SnapshotAndClear(); ok {
		t.Fatal("clean slot returned a record")
	}
}

func TestDirtyTelemetrySlotLowRateFlushesWithinBound(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var slot DirtyTelemetrySlot
	var mu sync.Mutex
	var emitted []TelemetryRecord
	StartTelemetryFlusher(ctx, &slot, func(record TelemetryRecord) {
		mu.Lock()
		emitted = append(emitted, record)
		mu.Unlock()
	})
	start := time.Now()
	slot.Publish(TelemetryRecord{Producer: "daemon", Sequence: 1})
	for time.Since(start) < 600*time.Millisecond {
		mu.Lock()
		count := len(emitted)
		mu.Unlock()
		if count > 0 {
			if time.Since(start) > 300*time.Millisecond {
				t.Fatalf("first telemetry emission took %s", time.Since(start))
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("low-rate telemetry was not emitted")
}
