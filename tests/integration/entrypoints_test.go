package integration_test

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestEntrypointsTypeScript verifies that `inarientrypoints` succeeds and
// lists entry points grouped by type, including controllers from the fixture.
func TestEntrypointsTypeScript(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	stdout, _ := runInari(t, dir, "entrypoints")
	assertContains(t, stdout, "Entrypoints")
	assertContains(t, stdout, "Controller")
}

// TestEntrypointsJson verifies that --json output is valid JSON with
// command="entrypoints" and data as an array.
func TestEntrypointsJson(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	stdout, _ := runInari(t, dir, "entrypoints", "--json")

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\noutput: %s", err, stdout)
	}

	cmd, ok := parsed["command"]
	if !ok {
		t.Fatal("JSON envelope missing 'command' field")
	}
	if cmd != "entrypoints" {
		t.Fatalf("expected command='entrypoints', got %q", cmd)
	}

	if parsed["data"] == nil {
		t.Fatal("JSON envelope must have a non-null 'data' field")
	}

	if _, isArr := parsed["data"].([]interface{}); !isArr {
		t.Fatal("data must be an array of groups")
	}
}

// TestEntrypointsNoIndexFails verifies that running entrypoints without an
// index produces a clear error.
func TestEntrypointsNoIndexFails(t *testing.T) {
	dir := t.TempDir()

	_, stderr := runInariExpectFail(t, dir, "entrypoints")
	lower := strings.ToLower(stderr)
	if !strings.Contains(lower, ".inari") && !strings.Contains(lower, "inari init") {
		t.Errorf("expected stderr to mention '.inari' or 'inari init', got: %s", stderr)
	}
}
