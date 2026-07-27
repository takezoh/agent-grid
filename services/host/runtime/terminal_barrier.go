package runtime

// TerminalBarrier prevents a terminal marker from overtaking admitted output.
// A terminal marker is deliverable when every sequence through finalSequence
// was forwarded, or when the caller explicitly records a delivery gap.
type TerminalBarrier struct {
	forwarded uint64
}

func (b *TerminalBarrier) MarkForwarded(sequence uint64) {
	if sequence > b.forwarded {
		b.forwarded = sequence
	}
}

func (b *TerminalBarrier) Ready(finalSequence uint64) bool {
	return b.forwarded >= finalSequence
}

func (b *TerminalBarrier) DeliveryGap(finalSequence uint64) (bool, uint64) {
	if b.Ready(finalSequence) {
		return false, 0
	}
	return true, finalSequence - b.forwarded
}
