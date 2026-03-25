package integration_test

import (
	"encoding/json"
	"testing"
)

// TestDepsPaymentService verifies that `inari deps PaymentService` succeeds
// and lists its direct dependencies.
func TestDepsPaymentService(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	stdout, _ := runInari(t, dir, "deps", "PaymentService")
	assertContains(t, stdout, "PaymentService")
}

// TestDepsDepth2 verifies that `inari deps PaymentService --depth 2` shows
// transitive dependencies and labels the output accordingly.
func TestDepsDepth2(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	stdout, _ := runInari(t, dir, "deps", "PaymentService", "--depth", "2")
	assertContains(t, stdout, "PaymentService")
	assertContains(t, stdout, "transitive")
}

// TestDepsFileLevel verifies that passing a file path triggers file-level
// dependency listing.
func TestDepsFileLevel(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	stdout, _ := runInari(t, dir, "deps", "src/payments/service.ts")
	assertContains(t, stdout, "service.ts")
}

// TestDepsUnknownSymbolFails verifies that an unknown symbol fails with
// "not found" in stderr.
func TestDepsUnknownSymbolFails(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	_, stderr := runInariExpectFail(t, dir, "deps", "UnknownThing")
	assertContains(t, stderr, "not found")
}

// TestDepsJson verifies that --json output is valid JSON with command="deps"
// and a non-null data field.
func TestDepsJson(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	stdout, _ := runInari(t, dir, "deps", "PaymentService", "--json")

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\noutput: %s", err, stdout)
	}

	cmd, ok := parsed["command"]
	if !ok {
		t.Fatal("JSON envelope missing 'command' field")
	}
	if cmd != "deps" {
		t.Fatalf("expected command='deps', got %q", cmd)
	}

	if parsed["data"] == nil {
		t.Fatal("JSON envelope must have a non-null 'data' field")
	}
}
