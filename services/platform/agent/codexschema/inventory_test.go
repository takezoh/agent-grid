package codexschema

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"testing"
)

type aggregateSchema struct {
	Definitions map[string]struct {
		OneOf []json.RawMessage `json:"oneOf"`
	} `json:"definitions"`
}

func TestMethodInventoryMatchesPinnedCodexSchema(t *testing.T) {
	raw, err := os.ReadFile("schema/codex_app_server_protocol.schemas.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema aggregateSchema
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	assertInventory(t, "ServerNotification", schema, ServerNotifications)
	assertInventory(t, "ServerRequest", schema, ServerRequests)
}

func assertInventory(t *testing.T, name string, schema aggregateSchema, inventory []string) {
	t.Helper()
	var methods []string
	for _, variant := range schema.Definitions[name].OneOf {
		var object struct {
			Properties json.RawMessage `json:"properties"`
		}
		if err := json.Unmarshal(variant, &object); err != nil {
			t.Fatal(err)
		}
		var properties map[string]struct {
			Enum []string `json:"enum"`
		}
		if len(object.Properties) == 0 || object.Properties[0] != '{' {
			continue
		}
		if err := json.Unmarshal(object.Properties, &properties); err != nil {
			t.Fatal(err)
		}
		methods = append(methods, properties["method"].Enum...)
	}
	sort.Strings(methods)
	got := append([]string(nil), inventory...)
	sort.Strings(got)
	if !reflect.DeepEqual(got, methods) {
		t.Fatalf("%s inventory drift\ngot:  %q\nwant: %q", name, got, methods)
	}
}
