package state

import (
	"fmt"

	"github.com/takezoh/agent-grid/platform/termvt"
)

// ValidateSizeHint checks terminal size hints before int→uint16 narrowing at
// the web boundary. Both zero means unspecified (accepted; downstream uses
// 80×24). Exactly one zero is asymmetric and rejected. When both are
// non-zero they must be in 1..termvt.MaxDim.
func ValidateSizeHint(cols, rows int) error {
	if cols == 0 && rows == 0 {
		return nil
	}
	if (cols == 0) != (rows == 0) {
		return fmt.Errorf("asymmetric size hint: both cols and rows must be specified")
	}
	if cols < 1 || cols > termvt.MaxDim {
		return fmt.Errorf("cols must be 0 or 1..%d; got %d", termvt.MaxDim, cols)
	}
	if rows < 1 || rows > termvt.MaxDim {
		return fmt.Errorf("rows must be 0 or 1..%d; got %d", termvt.MaxDim, rows)
	}
	return nil
}

// SizeHintRejectReason returns a short reason token for structured warn logs
// when cols/rows fail ValidateSizeHint. Returns "" when the hint is valid.
func SizeHintRejectReason(cols, rows int) string {
	if ValidateSizeHint(cols, rows) == nil {
		return ""
	}
	if (cols == 0) != (rows == 0) {
		return "asymmetric size hint"
	}
	if cols < 1 || cols > termvt.MaxDim {
		return "cols out of range"
	}
	if rows < 1 || rows > termvt.MaxDim {
		return "rows out of range"
	}
	return "invalid size hint"
}
