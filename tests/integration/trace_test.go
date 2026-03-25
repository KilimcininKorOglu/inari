package integration_test

import (
	"encoding/json"
	"testing"
)

// TestTraceProcessPayment verifies that `inaritrace processPayment` succeeds
// and shows entry paths.
func TestTraceProcessPayment(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	stdout, _ := runInari(t, dir, "trace", "processPayment")
	assertContains(t, stdout, "entry path")
}

// TestTraceUnknownSymbolFails verifies that tracing an unknown symbol fails
// with "not found" in stderr.
func TestTraceUnknownSymbolFails(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	_, stderr := runInariExpectFail(t, dir, "trace", "UnknownSymbol")
	assertContains(t, stderr, "not found")
}

// TestTraceNoCallers verifies that tracing a private method with no external
// callers shows "0 entry paths".
func TestTraceNoCallers(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	stdout, _ := runInari(t, dir, "trace", "validateAmount")
	assertContains(t, stdout, "0 entry paths")
}

// TestTraceJson verifies that --json output is valid JSON with command="trace"
// and a data.paths array.
func TestTraceJson(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	stdout, _ := runInari(t, dir, "trace", "processPayment", "--json")

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\noutput: %s", err, stdout)
	}

	cmd, ok := parsed["command"]
	if !ok {
		t.Fatal("JSON envelope missing 'command' field")
	}
	if cmd != "trace" {
		t.Fatalf("expected command='trace', got %q", cmd)
	}

	if parsed["data"] == nil {
		t.Fatal("JSON envelope must have a non-null 'data' field")
	}

	data, ok := parsed["data"].(map[string]interface{})
	if !ok {
		t.Fatal("data must be an object")
	}

	paths, ok := data["paths"]
	if !ok {
		t.Fatal("data must contain a 'paths' field")
	}

	if _, isArr := paths.([]interface{}); !isArr {
		t.Fatal("data.paths must be an array")
	}
}
