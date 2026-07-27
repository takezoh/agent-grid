package framelaunch

import (
	"strings"
	"testing"
)

func TestDedupPath(t *testing.T) {
	cases := []struct {
		name        string
		in          string
		want        string
		wantDropped int
	}{
		{"empty", "", "", 0},
		{"single", "/a", "/a", 0},
		{"no dupe", "/a:/b:/c", "/a:/b:/c", 0},
		{"first wins", "/a:/b:/a", "/a:/b", 1},
		{"empty segment dropped", "/a::/b", "/a:/b", 1},
		{"leading colon dropped", ":/a:/b", "/a:/b", 1},
		{"trailing colon dropped", "/a:/b:", "/a:/b", 1},
		{"trailing slash distinct", "/a:/a/", "/a:/a/", 0},
		{"runtime prefix retained, dupe trimmed", "/opt/agent-grid/run/hostexec-shims:/usr/bin:/opt/agent-grid/run/hostexec-shims", "/opt/agent-grid/run/hostexec-shims:/usr/bin", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, dropped := dedupPath(tc.in)
			if got != tc.want {
				t.Errorf("dedupPath(%q) = %q, want %q", tc.in, got, tc.want)
			}
			if dropped != tc.wantDropped {
				t.Errorf("dedupPath(%q) dropped = %d, want %d", tc.in, dropped, tc.wantDropped)
			}
		})
	}
}

func TestComputeFinalPath(t *testing.T) {
	runtimeList := []string{"/opt/agent-grid/run/hostexec-shims", "/opt/agent-grid/run/secretenv-shims"}
	cases := []struct {
		name         string
		runtimeList  []string
		captured     string
		orig         string
		wantHead     string
		wantContains string
		wantEmpty    bool
	}{
		{
			name:         "normal — captured PATH prepended with runtime list",
			runtimeList:  runtimeList,
			captured:     "/home/ubuntu/.grok/bin:/usr/bin:/bin",
			orig:         "/opt/agent-grid/run/hostexec-shims:/usr/bin",
			wantHead:     "/opt/agent-grid/run/hostexec-shims",
			wantContains: "/home/ubuntu/.grok/bin",
		},
		{
			name:        "captured empty — falls back to orig",
			runtimeList: runtimeList,
			captured:    "",
			orig:        "/usr/bin:/bin",
			wantHead:    "/opt/agent-grid/run/hostexec-shims",
		},
		{
			name:        "both empty — output is runtime list only",
			runtimeList: runtimeList,
			captured:    "",
			orig:        "",
			wantHead:    "/opt/agent-grid/run/hostexec-shims",
		},
		{
			name:        "empty runtime list + non-empty base — base returned as-is",
			runtimeList: nil,
			captured:    "/usr/bin",
			orig:        "",
			wantHead:    "/usr/bin",
		},
		{
			name:        "runtime list dupes captured entries — dedup keeps runtime-first ordering",
			runtimeList: runtimeList,
			captured:    "/usr/bin:/opt/agent-grid/run/hostexec-shims:/bin",
			orig:        "",
			wantHead:    "/opt/agent-grid/run/hostexec-shims",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, decision := computeFinalPath(tc.runtimeList, tc.captured, tc.orig)
			if tc.wantEmpty && got != "" {
				t.Fatalf("got %q, want empty", got)
			}
			if !tc.wantEmpty && got == "" {
				t.Fatalf("got empty PATH; want non-empty starting with %q", tc.wantHead)
			}
			if strings.HasPrefix(got, ":") {
				t.Fatalf("got PATH starts with empty segment: %q", got)
			}
			if tc.wantHead != "" {
				head := headSegment(got)
				if head != tc.wantHead {
					t.Errorf("head = %q, want %q; full = %q", head, tc.wantHead, got)
				}
			}
			if tc.wantContains != "" && !strings.Contains(got, tc.wantContains) {
				t.Errorf("PATH %q does not contain %q", got, tc.wantContains)
			}
			if decision.Branch != "merged" {
				t.Errorf("Branch = %q, want %q", decision.Branch, "merged")
			}
			if decision.PrefixCount != len(tc.runtimeList) {
				t.Errorf("PrefixCount = %d, want %d", decision.PrefixCount, len(tc.runtimeList))
			}
		})
	}
}

// TestComputeFinalPath_NeverEmptyLeadWhenRuntimeNonEmpty pins FR-004: with
// non-empty runtime list, output is never empty and never starts with an
// empty segment, regardless of captured/orig combinations.
func TestComputeFinalPath_NeverEmptyLeadWhenRuntimeNonEmpty(t *testing.T) {
	rt := []string{"/a", "/b"}
	for _, pair := range [][2]string{
		{"", ""},
		{"", "/x"},
		{"/y", ""},
		{"/y", "/x"},
		{":", ""},
		{"", ":"},
	} {
		got, _ := computeFinalPath(rt, pair[0], pair[1])
		if got == "" {
			t.Errorf("captured=%q orig=%q → empty PATH (FR-004 violated)", pair[0], pair[1])
			continue
		}
		if strings.HasPrefix(got, ":") {
			t.Errorf("captured=%q orig=%q → leading empty segment: %q", pair[0], pair[1], got)
		}
	}
}
