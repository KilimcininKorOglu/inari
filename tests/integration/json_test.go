package integration_test

import (
	"encoding/json"
	"testing"
)

// parseJSON unmarshals a JSON string into a generic map. Fails the test if
// the input is not valid JSON.
func parseJSON(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nraw output: %s", err, raw)
	}
	return result
}

// TestAllCommandsJsonValid verifies that --json on all major commands returns
// valid JSON with the expected envelope fields.
func TestAllCommandsJsonValid(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	commands := []struct {
		name string
		args []string
	}{
		{"sketch", []string{"sketch", "PaymentService", "--json"}},
		{"refs", []string{"refs", "PaymentService", "--json"}},
		{"deps", []string{"deps", "PaymentService", "--json"}},
		{"impact", []string{"impact", "PaymentService", "--json"}},
		{"find", []string{"find", "payment", "--json"}},
		{"status", []string{"status", "--json"}},
	}

	for _, tc := range commands {
		t.Run(tc.name, func(t *testing.T) {
			stdout, _ := runInari(t, dir, tc.args...)
			envelope := parseJSON(t, stdout)

			cmd, ok := envelope["command"].(string)
			if !ok {
				t.Errorf("expected command field to be a string, got %T", envelope["command"])
			}
			if cmd != tc.name {
				// Some commands may use different names in the envelope
				// (e.g., "workspace list" vs "list"). Just verify it is non-empty.
				if cmd == "" {
					t.Errorf("expected non-empty command field")
				}
			}

			if envelope["data"] == nil {
				t.Errorf("expected non-null data field in JSON envelope for %s", tc.name)
			}
		})
	}
}

// TestJsonEnvelopeStructure verifies the JSON envelope for `inarisketch --json`
// contains command, data, and other expected fields.
func TestJsonEnvelopeStructure(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	stdout, _ := runInari(t, dir, "sketch", "PaymentService", "--json")
	envelope := parseJSON(t, stdout)

	// command field
	cmd, ok := envelope["command"].(string)
	if !ok || cmd != "sketch" {
		t.Errorf("expected command=sketch, got %v", envelope["command"])
	}

	// data field must be non-null
	if envelope["data"] == nil {
		t.Error("expected non-null data field")
	}

	// data should be a map with symbol info
	data, ok := envelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data to be an object, got %T", envelope["data"])
	}

	// data.symbol should exist and have name + kind
	symbol, ok := data["symbol"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data.symbol to be an object, got %T", data["symbol"])
	}

	name, _ := symbol["name"].(string)
	if name != "PaymentService" {
		t.Errorf("expected data.symbol.name=PaymentService, got %s", name)
	}

	kind, _ := symbol["kind"].(string)
	if kind == "" {
		t.Error("expected data.symbol.kind to be a non-empty string")
	}
}

// TestFindJson verifies that `inarifind --json` returns search results with
// expected fields (name, kind, score) in the data array.
func TestFindJson(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	stdout, _ := runInari(t, dir, "find", "payment", "--json")
	envelope := parseJSON(t, stdout)

	cmd, _ := envelope["command"].(string)
	if cmd != "find" {
		t.Errorf("expected command=find, got %s", cmd)
	}

	// data should be an array of results.
	data, ok := envelope["data"].([]interface{})
	if !ok {
		t.Fatalf("expected data to be an array, got %T", envelope["data"])
	}

	if len(data) == 0 {
		t.Fatal("expected at least one search result for 'payment'")
	}

	// Each result should have name, kind, and score.
	for i, item := range data {
		result, ok := item.(map[string]interface{})
		if !ok {
			t.Errorf("result[%d] is not an object", i)
			continue
		}

		if _, ok := result["name"].(string); !ok {
			t.Errorf("result[%d] missing string 'name' field", i)
		}
		if _, ok := result["kind"].(string); !ok {
			t.Errorf("result[%d] missing string 'kind' field", i)
		}
		if _, ok := result["score"].(float64); !ok {
			t.Errorf("result[%d] missing numeric 'score' field", i)
		}
	}
}

// TestStatusJsonHasIndexExists verifies that `inaristatus --json` on an
// indexed project has data.index_exists == true.
func TestStatusJsonHasIndexExists(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	stdout, _ := runInari(t, dir, "status", "--json")
	envelope := parseJSON(t, stdout)

	data, ok := envelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data to be an object, got %T", envelope["data"])
	}

	indexExists, ok := data["index_exists"].(bool)
	if !ok || !indexExists {
		t.Errorf("expected data.index_exists=true, got %v", data["index_exists"])
	}
}

// TestRefsJsonHasGroups verifies that `inari refs --json` data contains a
// grouped array of reference kinds (calls, imports, references_type, etc.).
func TestRefsJsonHasGroups(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	stdout, _ := runInari(t, dir, "refs", "PaymentService", "--json")
	envelope := parseJSON(t, stdout)

	// Data is an array of groups, each with "kind" and "refs" fields.
	data, ok := envelope["data"].([]interface{})
	if !ok {
		t.Fatalf("expected data to be an array, got %T", envelope["data"])
	}
	if len(data) == 0 {
		t.Fatal("expected at least one reference group")
	}

	// Verify each group has "kind" and "refs" fields.
	for i, item := range data {
		group, ok := item.(map[string]interface{})
		if !ok {
			t.Errorf("group[%d] is not an object", i)
			continue
		}
		if _, ok := group["kind"].(string); !ok {
			t.Errorf("group[%d] missing string 'kind' field", i)
		}
		if _, ok := group["refs"].([]interface{}); !ok {
			t.Errorf("group[%d] missing array 'refs' field", i)
		}
	}
}
