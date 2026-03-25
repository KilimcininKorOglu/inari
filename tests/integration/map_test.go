package integration_test

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestMapTypeScript verifies that `inarimap` succeeds and shows project
// stats (files, symbols), entry points, and core symbols sections.
func TestMapTypeScript(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	stdout, _ := runInari(t, dir, "map")

	assertContains(t, stdout, "files")
	assertContains(t, stdout, "symbols")
	assertContains(t, stdout, "Entry points:")
	assertContains(t, stdout, "Core symbols")
}

// TestMapOutputIsCompact verifies that map output stays under 30 lines.
func TestMapOutputIsCompact(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	stdout, _ := runInari(t, dir, "map")

	lineCount := len(strings.Split(strings.TrimRight(stdout, "\n"), "\n"))
	if lineCount > 30 {
		t.Errorf("map output should be under 30 lines, got %d", lineCount)
	}
}

// TestMapJson verifies that --json output is valid JSON with command="map"
// and all expected nested fields.
func TestMapJson(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	stdout, _ := runInari(t, dir, "map", "--json")

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\noutput: %s", err, stdout)
	}

	cmd, ok := parsed["command"]
	if !ok {
		t.Fatal("JSON envelope missing 'command' field")
	}
	if cmd != "map" {
		t.Fatalf("expected command='map', got %q", cmd)
	}

	if parsed["data"] == nil {
		t.Fatal("JSON envelope must have a non-null 'data' field")
	}

	data, ok := parsed["data"].(map[string]interface{})
	if !ok {
		t.Fatal("data must be an object")
	}

	// Validate stats sub-object.
	stats, ok := data["stats"].(map[string]interface{})
	if !ok {
		t.Fatal("data.stats must be an object")
	}
	for _, key := range []string{"file_count", "symbol_count", "edge_count"} {
		val, exists := stats[key]
		if !exists {
			t.Fatalf("stats.%s must exist", key)
		}
		if _, isNum := val.(float64); !isNum {
			t.Fatalf("stats.%s must be a number, got %T", key, val)
		}
	}

	// Validate array fields.
	for _, key := range []string{"entrypoints", "core_symbols", "architecture"} {
		val, exists := data[key]
		if !exists {
			t.Fatalf("data.%s must exist", key)
		}
		if _, isArr := val.([]interface{}); !isArr {
			t.Fatalf("data.%s must be an array, got %T", key, val)
		}
	}
}

// TestMapLimit verifies that --limit controls the number of core symbols shown.
func TestMapLimit(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	stdout, _ := runInari(t, dir, "map", "--limit", "3")

	// Count lines in the core symbols section.
	inCore := false
	coreLines := 0
	for _, line := range strings.Split(stdout, "\n") {
		if strings.HasPrefix(line, "Core symbols") {
			inCore = true
			continue
		}
		if inCore {
			// End of section: empty line or a non-indented, non-empty line.
			if line == "" || (!strings.HasPrefix(line, "  ") && line != "") {
				break
			}
			if strings.HasPrefix(line, "  ") {
				coreLines++
			}
		}
	}

	if coreLines > 3 {
		t.Errorf("core symbols section should have at most 3 entries with --limit 3, got %d", coreLines)
	}
}

// TestMapNoIndexFails verifies that running map without an index produces a
// clear error.
func TestMapNoIndexFails(t *testing.T) {
	dir := t.TempDir()

	_, stderr := runInariExpectFail(t, dir, "map")
	lower := strings.ToLower(stderr)
	if !strings.Contains(lower, ".inari") && !strings.Contains(lower, "inari init") {
		t.Errorf("expected stderr to mention '.inari' or 'inari init', got: %s", stderr)
	}
}
