package web

import (
	"context"
	"testing"
)

func TestApplyInboundProto_IgnoresMalformedAndNonPositiveResize(t *testing.T) {
	t.Parallel()

	fake := newFakeAttacher()
	ctx := context.Background()

	for _, frame := range [][]byte{
		[]byte(`not-json`),
		[]byte(`{"k":"r","cols":0,"rows":24}`),
		[]byte(`{"k":"r","cols":80,"rows":0}`),
		[]byte(`{"k":"r","cols":-1,"rows":24}`),
		[]byte(`{"k":"r","cols":80,"rows":-1}`),
		[]byte(`{"k":"unknown"}`),
	} {
		applyInboundProto(ctx, fake, "s1", frame)
	}

	if got := len(fake.writeRawSnapshot()); got != 0 {
		t.Fatalf("WriteRaw calls = %d, want 0", got)
	}
	if got := len(fake.resizeSnapshot()); got != 0 {
		t.Fatalf("Resize calls = %d, want 0", got)
	}
}

func FuzzApplyInboundProto(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte(`{"k":"i","d":"hello"}`),
		[]byte(`{"k":"r","cols":80,"rows":24}`),
		[]byte(`{"k":"r","cols":0,"rows":24}`),
		[]byte(`{"k":"r","cols":-1,"rows":24}`),
		[]byte(`{"k":"x"}`),
		[]byte(`not-json`),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		fake := newFakeAttacher()
		applyInboundProto(context.Background(), fake, "s1", data)
		for _, call := range fake.resizeSnapshot() {
			if call.cols == 0 || call.rows == 0 {
				t.Fatalf("Resize accepted non-positive size: %+v", call)
			}
		}
	})
}
