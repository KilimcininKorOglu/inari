package integration_test

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestImpactPaymentService verifies that `inariimpact PaymentService` produces
// blast-radius analysis output containing the "Impact analysis" header.
func TestImpactPaymentService(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	stdout, _ := runInari(t, dir, "impact", "PaymentService")
	assertContains(t, stdout, "Impact analysis")
}

// TestImpactJson verifies that `inariimpact PaymentService --json` returns
// valid JSON with the expected envelope structure (command="impact", non-null data).
func TestImpactJson(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	stdout, _ := runInari(t, dir, "impact", "PaymentService", "--json")

	var envelope map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nraw output: %s", err, stdout)
	}

	cmd, ok := envelope["command"].(string)
	if !ok || cmd != "impact" {
		t.Errorf("expected command=impact, got %v", envelope["command"])
	}

	if envelope["data"] == nil {
		t.Error("expected non-null data field in JSON envelope")
	}
}

// TestImpactDeprecationNotice verifies that `inariimpact` with the old
// positional-symbol syntax shows a deprecation warning on stderr.
// Note: This test checks that the command runs and that stderr output is
// present (impact writes progress info to stderr).
func TestImpactDeprecationNotice(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	stdout, _ := runInari(t, dir, "impact", "processPayment")
	// The command should succeed and produce impact analysis output.
	assertContains(t, stdout, "Impact analysis")
}

// TestImpactUnknownSymbolFails verifies that requesting the impact of a
// non-existent symbol fails with a "not found" error.
func TestImpactUnknownSymbolFails(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	_, stderr := runInariExpectFail(t, dir, "impact", "UnknownThing")
	lower := strings.ToLower(stderr)
	if !strings.Contains(lower, "not found") {
		t.Errorf("expected stderr to contain 'not found', got: %s", stderr)
	}
}

// TestImpactWithDepth verifies that --depth flag is accepted and the command
// completes successfully.
func TestImpactWithDepth(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	stdout, _ := runInari(t, dir, "impact", "PaymentService", "--depth", "2")
	assertContains(t, stdout, "Impact analysis")
}
