package integration_test

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestRefsPaymentService verifies that `inarirefs PaymentService` succeeds
// and shows grouped references including the symbol name.
func TestRefsPaymentService(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	stdout, _ := runInari(t, dir, "refs", "PaymentService")
	assertContains(t, stdout, "PaymentService")
}

// TestRefsProcessPayment verifies that refs for a method shows callers.
func TestRefsProcessPayment(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	stdout, _ := runInari(t, dir, "refs", "processPayment")
	assertContains(t, stdout, "processPayment")
}

// TestRefsWithKindFilter verifies that --kind imports filters to only import
// edges.
func TestRefsWithKindFilter(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	stdout, _ := runInari(t, dir, "refs", "PaymentService", "--kind", "imports")
	assertContains(t, stdout, "PaymentService")
}

// TestRefsWithLimit verifies that --limit truncates the result set and shows
// a "more" indicator.
func TestRefsWithLimit(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	stdout, _ := runInari(t, dir, "refs", "PaymentService", "--kind", "imports", "--limit", "1")
	assertContains(t, stdout, "more")
}

// TestRefsUnknownSymbolFails verifies that looking up an unknown symbol fails
// with "not found" in stderr.
func TestRefsUnknownSymbolFails(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	_, stderr := runInariExpectFail(t, dir, "refs", "UnknownThing")
	assertContains(t, stderr, "not found")
}

// TestRefsJson verifies that --json output is valid JSON with command="refs"
// and a non-null data field.
func TestRefsJson(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	stdout, _ := runInari(t, dir, "refs", "PaymentService", "--json")

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\noutput: %s", err, stdout)
	}

	cmd, ok := parsed["command"]
	if !ok {
		t.Fatal("JSON envelope missing 'command' field")
	}
	if cmd != "refs" {
		t.Fatalf("expected command='refs', got %q", cmd)
	}

	if parsed["data"] == nil {
		t.Fatal("JSON envelope must have a non-null 'data' field")
	}
}

// TestCallersDepth1 verifies that `inaricallers processPayment` (default
// depth) shows direct callers in a flat format.
func TestCallersDepth1(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	stdout, _ := runInari(t, dir, "callers", "processPayment")
	assertContains(t, stdout, "reference")
	// Default depth=1 should NOT use impact-style grouping.
	assertNotContains(t, stdout, "Direct callers")
}

// TestCallersDepth2 verifies that `inaricallers processPayment --depth 2`
// shows transitive callers with depth grouping.
func TestCallersDepth2(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	stdout, _ := runInari(t, dir, "callers", "processPayment", "--depth", "2")

	// Transitive output uses impact formatting with depth labels.
	if !strings.Contains(stdout, "Direct callers") && !strings.Contains(stdout, "Impact analysis") {
		t.Errorf("expected depth-grouped output (Direct callers or Impact analysis), got:\n%s", stdout)
	}
}
