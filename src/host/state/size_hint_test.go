package state

import (
	"strings"
	"testing"

	"github.com/takezoh/agent-grid/platform/termvt"
)

func TestValidateSizeHint(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		cols    int
		rows    int
		wantErr string
	}{
		{name: "both zero passes", cols: 0, rows: 0},
		{name: "valid hint passes", cols: 203, rows: 47},
		{name: "negative cols", cols: -1, rows: 24, wantErr: "cols must be"},
		{name: "wrap around cols", cols: 65536, rows: 24, wantErr: "cols must be"},
		{name: "above maxDim cols", cols: 99999, rows: 47, wantErr: "cols must be"},
		{name: "above maxDim rows", cols: 80, rows: termvt.MaxDim + 1, wantErr: "rows must be"},
		{name: "asymmetric cols only", cols: 120, rows: 0, wantErr: "asymmetric"},
		{name: "asymmetric rows only", cols: 0, rows: 40, wantErr: "asymmetric"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateSizeHint(tc.cols, tc.rows)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateSizeHint(%d,%d) = %v, want nil", tc.cols, tc.rows, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("ValidateSizeHint(%d,%d) = nil, want error containing %q", tc.cols, tc.rows, tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("ValidateSizeHint(%d,%d) = %q, want substring %q", tc.cols, tc.rows, err.Error(), tc.wantErr)
			}
		})
	}
}

func TestSizeHintRejectReason(t *testing.T) {
	t.Parallel()

	if got := SizeHintRejectReason(99999, 47); got != "cols out of range" {
		t.Fatalf("SizeHintRejectReason(99999,47) = %q, want cols out of range", got)
	}
	if got := SizeHintRejectReason(120, 40); got != "" {
		t.Fatalf("SizeHintRejectReason(120,40) = %q, want empty", got)
	}
}
