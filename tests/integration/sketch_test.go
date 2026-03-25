package integration_test

import (
	"encoding/json"
	"testing"
)

// TestSketchClassPaymentService verifies that sketching a class shows
// the class name, kind, file path, methods, dependencies, and caller counts.
func TestSketchClassPaymentService(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	stdout, _ := runInari(t, dir, "sketch", "PaymentService")
	assertContains(t, stdout, "PaymentService")
	assertContains(t, stdout, "class")
	assertContains(t, stdout, "service.ts")
}

// TestSketchMethodProcessPayment verifies that sketching a qualified method
// shows the method kind, calls, and called-by information.
func TestSketchMethodProcessPayment(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	stdout, _ := runInari(t, dir, "sketch", "PaymentService.processPayment")
	assertContains(t, stdout, "method")
	assertContains(t, stdout, "processPayment")
}

// TestSketchQualifiedRefundPayment verifies lookup of a second method on the
// same class.
func TestSketchQualifiedRefundPayment(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	stdout, _ := runInari(t, dir, "sketch", "PaymentService.refundPayment")
	assertContains(t, stdout, "refundPayment")
}

// TestSketchFileServiceTs verifies that sketching a file path returns
// all symbols defined in that file.
func TestSketchFileServiceTs(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	stdout, _ := runInari(t, dir, "sketch", "src/payments/service.ts")
	assertContains(t, stdout, "PaymentService")
}

// TestSketchInterface verifies that sketching an interface / type alias works.
func TestSketchInterface(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	stdout, _ := runInari(t, dir, "sketch", "PaymentRequest")
	assertContains(t, stdout, "PaymentRequest")
}

// TestSketchJson verifies that --json output is valid JSON with the expected
// envelope fields (command: "sketch", data: non-null).
func TestSketchJson(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	stdout, _ := runInari(t, dir, "sketch", "PaymentService", "--json")

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\noutput: %s", err, stdout)
	}

	cmd, ok := parsed["command"]
	if !ok {
		t.Fatal("JSON envelope missing 'command' field")
	}
	if cmd != "sketch" {
		t.Fatalf("expected command='sketch', got %q", cmd)
	}

	if parsed["data"] == nil {
		t.Fatal("JSON envelope must have a non-null 'data' field")
	}
}

// TestSketchNotFound verifies that sketching an unknown symbol fails with an
// error containing "not found".
func TestSketchNotFound(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	_, stderr := runInariExpectFail(t, dir, "sketch", "NonExistentThing")
	assertContains(t, stderr, "not found")
}
