package integration_test

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSwiftInit verifies that `inari init` detects Swift from project marker files.
func TestSwiftInit(t *testing.T) {
	dir := t.TempDir()
	content := `// swift-tools-version: 5.9
import PackageDescription
let package = Package(name: "test")
`
	if err := os.WriteFile(filepath.Join(dir, "Package.swift"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write Package.swift: %v", err)
	}
	stdout, _ := runInari(t, dir, "init")
	assertContains(t, stdout, "Swift")
}

// TestSwiftIndex verifies that indexing the swift-simple fixture succeeds
// and creates a non-empty graph.db.
func TestSwiftIndex(t *testing.T) {
	dir := copyFixture(t, "swift-simple")

	runInari(t, dir, "init")
	_, stderr := runInari(t, dir, "index", "--full")

	assertContains(t, stderr, "symbols")

	graphDB := filepath.Join(dir, ".inari", "graph.db")
	info, err := os.Stat(graphDB)
	if err != nil {
		t.Fatalf("graph.db should exist after indexing: %v", err)
	}
	if info.Size() == 0 {
		t.Error("graph.db should not be empty")
	}
}

// TestSwiftSketch verifies that `inari sketch PaymentService` works on the
// Swift fixture and includes the class name in its output.
func TestSwiftSketch(t *testing.T) {
	dir := setupIndexedFixture(t, "swift-simple")

	stdout, _ := runInari(t, dir, "sketch", "PaymentService")
	assertContains(t, stdout, "PaymentService")
	assertContains(t, stdout, "class")
}

// TestSwiftSketchJsonOutput verifies that `inari sketch PaymentService --json`
// returns valid JSON with command="sketch" on the Swift fixture.
func TestSwiftSketchJsonOutput(t *testing.T) {
	dir := setupIndexedFixture(t, "swift-simple")

	stdout, _ := runInari(t, dir, "sketch", "PaymentService", "--json")
	envelope := parseJSON(t, stdout)

	cmd, _ := envelope["command"].(string)
	if cmd != "sketch" {
		t.Errorf("expected command=sketch, got %s", cmd)
	}
}

// TestSwiftFindClasses verifies that key Swift classes are indexed and
// discoverable via `inari find`.
func TestSwiftFindClasses(t *testing.T) {
	dir := setupIndexedFixture(t, "swift-simple")

	symbols := []string{"PaymentService", "OrderController", "Logger"}
	for _, sym := range symbols {
		t.Run(sym, func(t *testing.T) {
			stdout, _ := runInari(t, dir, "find", sym, "--json")
			envelope := parseJSON(t, stdout)
			data, ok := envelope["data"].([]interface{})
			if !ok {
				t.Fatalf("expected data to be an array, got %T", envelope["data"])
			}
			if len(data) == 0 {
				t.Errorf("expected %s to be found in the index", sym)
			}
		})
	}
}

// TestSwiftRefs verifies that `inari refs processPayment` finds callers
// in the Swift fixture.
func TestSwiftRefs(t *testing.T) {
	dir := setupIndexedFixture(t, "swift-simple")

	stdout, _ := runInari(t, dir, "refs", "processPayment")
	assertContains(t, stdout, "OrderController.swift")
}

// TestSwiftMap verifies that `inari map` produces a valid overview for
// the Swift fixture.
func TestSwiftMap(t *testing.T) {
	dir := setupIndexedFixture(t, "swift-simple")

	stdout, _ := runInari(t, dir, "map")
	assertContains(t, stdout, "swift")
	assertContains(t, stdout, "PaymentService")
}

// TestSwiftDeps verifies that `inari deps PaymentService` returns
// expected dependencies.
func TestSwiftDeps(t *testing.T) {
	dir := setupIndexedFixture(t, "swift-simple")

	stdout, _ := runInari(t, dir, "deps", "PaymentService")
	assertContains(t, stdout, "PaymentServiceProtocol")
}

// TestSwiftProtocols verifies that Swift protocols are indexed as interfaces.
func TestSwiftProtocols(t *testing.T) {
	dir := setupIndexedFixture(t, "swift-simple")

	stdout, _ := runInari(t, dir, "find", "PaymentServiceProtocol", "--json")
	envelope := parseJSON(t, stdout)
	data, ok := envelope["data"].([]interface{})
	if !ok {
		t.Fatalf("expected data to be an array, got %T", envelope["data"])
	}
	if len(data) == 0 {
		t.Errorf("expected PaymentServiceProtocol to be found in the index")
	}
}
