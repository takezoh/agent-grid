package e2etest

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

var recordFixtures = flag.Bool("record", false, "rewrite committed e2e recordings")

var (
	uuidLike    = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	absPathLike = regexp.MustCompile(`^(/|~\/|[A-Za-z]:[\\/])`)
)

func ShouldRecordFixtures() bool {
	return *recordFixtures
}

func AssertJSONLFixture(t *testing.T, path string, entries []any) {
	t.Helper()
	got, err := MarshalNormalizedJSONL(entries)
	if err != nil {
		t.Fatalf("marshal normalized jsonl: %v", err)
	}
	if ShouldRecordFixtures() {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir fixture dir: %v", err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if diff := cmp.Diff(string(want), string(got)); diff != "" {
		t.Fatalf("fixture drift (-want +got):\n%s", diff)
	}
}

func MarshalNormalizedJSONL(entries []any) ([]byte, error) {
	var buf bytes.Buffer
	for _, entry := range entries {
		norm, err := NormalizeAny(entry)
		if err != nil {
			return nil, err
		}
		line, err := json.Marshal(norm)
		if err != nil {
			return nil, err
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	return buf.Bytes(), nil
}

func ReadJSONLFixture(t *testing.T, path string) []map[string]any {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer func() { _ = f.Close() }()

	var out []map[string]any
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64<<10), 8<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var item map[string]any
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			t.Fatalf("unmarshal fixture line: %v", err)
		}
		out = append(out, item)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan fixture: %v", err)
	}
	return out
}

func NormalizeJSON(raw []byte) (map[string]any, error) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	norm, err := NormalizeAny(v)
	if err != nil {
		return nil, err
	}
	m, ok := norm.(map[string]any)
	if !ok {
		return nil, nil
	}
	return m, nil
}

func NormalizeAny(v any) (any, error) {
	return normalizeValue("", v), nil
}

func Contract(v any) any {
	return contractValue("", v)
}

func normalizeValue(key string, v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			out[k] = normalizeValue(k, x[k])
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = normalizeValue(key, x[i])
		}
		return out
	case string:
		return normalizeString(key, x)
	default:
		return v
	}
}

func normalizeString(key, value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return value
	}
	lowerKey := strings.ToLower(key)
	switch {
	case looksLikeTimestamp(lowerKey, trimmed):
		return "<timestamp>"
	case looksLikeSecret(lowerKey):
		return "<secret>"
	case looksLikePath(lowerKey, trimmed):
		return "<path>"
	case looksLikeIdentifier(lowerKey, trimmed):
		return "<id>"
	default:
		return value
	}
}

func looksLikeTimestamp(key, value string) bool {
	if strings.Contains(key, "time") || strings.Contains(key, "date") || strings.HasSuffix(key, "_at") || strings.HasSuffix(key, "at") {
		if _, err := time.Parse(time.RFC3339Nano, value); err == nil {
			return true
		}
	}
	_, err := time.Parse(time.RFC3339Nano, value)
	return err == nil
}

func looksLikeSecret(key string) bool {
	return strings.Contains(key, "token") || strings.Contains(key, "secret") || strings.Contains(key, "apikey") || strings.Contains(key, "api_key")
}

func looksLikePath(key, value string) bool {
	if strings.Contains(key, "path") || strings.Contains(key, "cwd") || strings.Contains(key, "dir") || strings.Contains(key, "socket") || strings.Contains(key, "home") {
		return true
	}
	if absPathLike.MatchString(value) {
		return true
	}
	return strings.Contains(value, ".jsonl") || strings.Contains(value, ".sock")
}

func looksLikeIdentifier(key, value string) bool {
	if key == "id" || strings.HasSuffix(key, "id") || strings.HasSuffix(key, "_id") || strings.HasSuffix(key, "Id") {
		return true
	}
	return uuidLike.MatchString(value)
}

func contractValue(key string, v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			out[k] = contractValue(k, x[k])
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = contractValue(key, x[i])
		}
		return out
	case string:
		if isDiscriminatorKey(key) {
			return x
		}
		return "string"
	case float64:
		return "number"
	case bool:
		return "bool"
	case nil:
		return nil
	default:
		return "value"
	}
}

func isDiscriminatorKey(key string) bool {
	switch key {
	case "type", "subtype", "method", "hook_event_name":
		return true
	default:
		return false
	}
}
