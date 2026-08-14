package attach

// geometry.go holds the size rules.

// MinDimension and MaxDimension are the bounds the execution plane accepts.
//
// Clamping here rather than sending whatever the terminal reported is the
// difference between a resize that arrives and one that is discarded: the
// server drops an out-of-range hint with a log line and no reply, so a client
// that sends 0 or 5000 gets silence and a session still rendering at the old
// size.
const (
	MinDimension = 1
	MaxDimension = 2000
)

// clampGeometry brings a reported terminal size into the accepted range.
//
// A zero dimension is the interesting case. The plane treats 0×0 as "no hint"
// and treats exactly one zero as an asymmetric hint it rejects outright, so a
// terminal that reports one zero — which happens when a size query races a
// window that is being destroyed — must not be forwarded as-is.
func clampGeometry(cols, rows int) (int, int) {
	return clampDimension(cols), clampDimension(rows)
}

func clampDimension(value int) int {
	if value < MinDimension {
		return MinDimension
	}
	if value > MaxDimension {
		return MaxDimension
	}
	return value
}
