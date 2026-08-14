package gorules_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Contract: contract-transport-close-taxonomy
// (adr-20260814-transport-close-reason-taxonomy).
//
// This is ver-close-reason-vocabulary-is-closed-and-registered. Four ad-hoc
// close reason strings accumulated in the gateway with nothing to stop a
// fifth, because no schema modelled them and no check enumerated them. The
// vocabulary is only closed if adding a reason outside it fails.
//
// It closes two doors, because there are exactly two ways in: declare a new
// constant (caught by the schema/Go set comparison) or bypass the helpers and
// pass a string straight to the socket (caught by the single-close-site rule).

const (
	transportCloseSchemaRel = "protocol/transport-close.schema.json"
	transportCloseGoRel     = "services/server/api/transport_close.go"
	gatewayAPIPkgRel        = "services/server/api"
)

func TestTransportCloseReasonVocabularyIsRegisteredAndExhaustive(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)

	published := publishedCloseReasonsFromSchema(t, root)
	declared := declaredCloseReasonsFromGo(t, root)

	if diff := missing(published, declared); len(diff) > 0 {
		t.Errorf("published in %s but not declared in %s: %v",
			transportCloseSchemaRel, transportCloseGoRel, diff)
	}
	if diff := missing(declared, published); len(diff) > 0 {
		t.Errorf("declared in %s but not published in %s — an ad-hoc reason the contract does not list: %v",
			transportCloseGoRel, transportCloseSchemaRel, diff)
	}

	assertSingleCloseSite(t, root)
	assertEveryReasonIsReachable(t, root, published)
}

// publishedCloseReasonsFromSchema reads the closed vocabulary out of the
// published schema, which is the contract's side of the comparison.
func publishedCloseReasonsFromSchema(t *testing.T, root string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, transportCloseSchemaRel))
	if err != nil {
		t.Fatalf("read %s: %v", transportCloseSchemaRel, err)
	}
	var schema struct {
		Defs struct {
			CloseReason struct {
				Enum []string `json:"enum"`
			} `json:"closeReason"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("decode %s: %v", transportCloseSchemaRel, err)
	}
	if len(schema.Defs.CloseReason.Enum) == 0 {
		t.Fatalf("%s declares no closeReason enum", transportCloseSchemaRel)
	}
	return schema.Defs.CloseReason.Enum
}

var closeReasonConstPattern = regexp.MustCompile(`(?m)^\s*Close\w+\s+CloseReason\s*=\s*"([a-z_]+)"`)

// declaredCloseReasonsFromGo reads the constants the gateway can actually
// send, which is the implementation's side of the comparison.
func declaredCloseReasonsFromGo(t *testing.T, root string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, transportCloseGoRel))
	if err != nil {
		t.Fatalf("read %s: %v", transportCloseGoRel, err)
	}
	matches := closeReasonConstPattern.FindAllStringSubmatch(string(raw), -1)
	if len(matches) == 0 {
		t.Fatalf("%s declares no CloseReason constants", transportCloseGoRel)
	}
	values := make([]string, 0, len(matches))
	for _, m := range matches {
		values = append(values, m[1])
	}
	return values
}

// assertSingleCloseSite pins the other way a reason could escape the
// vocabulary: calling Close on the connection directly with a free string,
// which is how all four of the pre-taxonomy reasons were written.
func assertSingleCloseSite(t *testing.T, root string) {
	t.Helper()
	offenders := []string{}
	for _, file := range nonTestGoFiles(t, filepath.Join(root, gatewayAPIPkgRel)) {
		if filepath.Base(file) == filepath.Base(transportCloseGoRel) {
			continue
		}
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		if strings.Contains(string(raw), "c.Close(websocket.Status") {
			rel, _ := filepath.Rel(root, file)
			offenders = append(offenders, rel)
		}
	}
	if len(offenders) > 0 {
		t.Errorf("close frames must be written by closeWith in %s so every reason comes from the published vocabulary; direct close sites: %v",
			transportCloseGoRel, offenders)
	}
}

// assertEveryReasonIsReachable makes the vocabulary exhaustive in the other
// direction: a published reason no code path can produce is a promise the
// contract cannot keep, and a client would wait for a value that never
// arrives.
//
// The two places that merely enumerate the vocabulary — the constant block and
// PublishedCloseReasons — are cut out first, or every reason would look
// reachable by virtue of being declared.
func assertEveryReasonIsReachable(t *testing.T, root string, published []string) {
	t.Helper()
	var body strings.Builder
	for _, file := range nonTestGoFiles(t, filepath.Join(root, gatewayAPIPkgRel)) {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		if filepath.Base(file) == filepath.Base(transportCloseGoRel) {
			body.WriteString(afterVocabularyDeclarations(t, string(raw)))
			continue
		}
		body.Write(raw)
	}
	text := body.String()
	for _, reason := range published {
		if !strings.Contains(text, goConstNameFor(reason)) {
			t.Errorf("published reason %q is declared but no code path in %s produces it", reason, gatewayAPIPkgRel)
		}
	}
}

// afterVocabularyDeclarations returns the part of transport_close.go that
// follows the PublishedCloseReasons slice — everything that uses the
// vocabulary rather than declaring it.
func afterVocabularyDeclarations(t *testing.T, src string) string {
	t.Helper()
	start := strings.Index(src, "var PublishedCloseReasons")
	if start < 0 {
		t.Fatalf("%s no longer declares PublishedCloseReasons", transportCloseGoRel)
	}
	end := strings.Index(src[start:], "\n}\n")
	if end < 0 {
		t.Fatalf("%s: PublishedCloseReasons block is unterminated", transportCloseGoRel)
	}
	return src[start+end:]
}

// goConstNameFor maps going_away → CloseGoingAway, matching the naming this
// file and transport_close.go both rely on.
func goConstNameFor(reason string) string {
	var name strings.Builder
	name.WriteString("Close")
	for part := range strings.SplitSeq(reason, "_") {
		name.WriteString(strings.ToUpper(part[:1]))
		name.WriteString(part[1:])
	}
	return name.String()
}

func nonTestGoFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %s: %v", dir, err)
	}
	files := []string{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		files = append(files, filepath.Join(dir, name))
	}
	return files
}

// missing returns the members of want that have no counterpart in got.
func missing(want, got []string) []string {
	index := make(map[string]struct{}, len(got))
	for _, g := range got {
		index[g] = struct{}{}
	}
	absent := []string{}
	for _, w := range want {
		if _, ok := index[w]; !ok {
			absent = append(absent, w)
		}
	}
	sort.Strings(absent)
	return absent
}
