package clientconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Contract: contract-config-and-fleet-identity
// (adr-20260814-shared-go-client-config-module).

// TestServerIdMatchesCrossLanguageGoldenVector is
// ver-server-id-digest-golden-vector.
//
// Routing identity is derived from the gateway URL and from nothing else, so a
// loader that canonicalises differently gives one server two identities. The
// two halves of a fleet then disagree about which rows are the same row —
// silently, because both are internally consistent. This fails an
// implementation that derives a different identity than the other hosts.
func TestServerIdMatchesCrossLanguageGoldenVector(t *testing.T) {
	t.Parallel()

	var corpus struct {
		Vectors []struct {
			Raw       string `json:"raw"`
			Canonical string `json:"canonical"`
			ServerID  string `json:"server_id"`
		} `json:"vectors"`
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "config", "fixtures", "server-id-vectors.json"))
	if err != nil {
		t.Fatalf("read golden vectors: %v", err)
	}
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatalf("decode golden vectors: %v", err)
	}
	if len(corpus.Vectors) == 0 {
		t.Fatal("golden vector file carries no vectors")
	}

	for _, vector := range corpus.Vectors {
		t.Run(vector.Raw, func(t *testing.T) {
			canonical, err := canonicalURL(vector.Raw)
			if err != nil {
				t.Fatalf("canonicalise %q: %v", vector.Raw, err)
			}
			if canonical != vector.Canonical {
				t.Errorf("canonical URL = %q, want %q", canonical, vector.Canonical)
			}
			if got := ServerID(canonical); got != vector.ServerID {
				t.Errorf("server id = %q, want %q", got, vector.ServerID)
			}
		})
	}
}
