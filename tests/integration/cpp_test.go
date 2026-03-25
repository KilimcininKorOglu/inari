package integration_test

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCppInit verifies that `inari init` detects C++ from project markers.
func TestCppInit(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "main.cpp"), []byte("int main() { return 0; }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	stdout, _ := runInari(t, dir, "init")
	assertContains(t, stdout, "C++")
}

// TestCppIndex verifies that indexing the cpp-simple fixture succeeds.
func TestCppIndex(t *testing.T) {
	dir := copyFixture(t, "cpp-simple")

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

// TestCppSketch verifies sketch on the C++ fixture.
func TestCppSketch(t *testing.T) {
	dir := setupIndexedFixture(t, "cpp-simple")

	stdout, _ := runInari(t, dir, "sketch", "PaymentService")
	assertContains(t, stdout, "PaymentService")
}

// TestCppSketchJsonOutput verifies JSON output on the C++ fixture.
func TestCppSketchJsonOutput(t *testing.T) {
	dir := setupIndexedFixture(t, "cpp-simple")

	stdout, _ := runInari(t, dir, "sketch", "PaymentService", "--json")
	envelope := parseJSON(t, stdout)

	cmd, _ := envelope["command"].(string)
	if cmd != "sketch" {
		t.Errorf("expected command=sketch, got %s", cmd)
	}
}

// TestCppFindClasses verifies that key C++ classes are indexed.
func TestCppFindClasses(t *testing.T) {
	dir := setupIndexedFixture(t, "cpp-simple")

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

// TestCppRefs verifies that `inari refs processPayment` finds callers.
func TestCppRefs(t *testing.T) {
	dir := setupIndexedFixture(t, "cpp-simple")

	stdout, _ := runInari(t, dir, "refs", "processPayment")
	assertContains(t, stdout, "controller.cpp")
}

// TestCppMap verifies that `inari map` produces a valid overview.
func TestCppMap(t *testing.T) {
	dir := setupIndexedFixture(t, "cpp-simple")

	stdout, _ := runInari(t, dir, "map")
	assertContains(t, stdout, "PaymentService")
}

// TestCppNamespaces verifies that C++ namespaces are indexed as modules.
func TestCppNamespaces(t *testing.T) {
	dir := setupIndexedFixture(t, "cpp-simple")

	stdout, _ := runInari(t, dir, "find", "payments", "--json")
	envelope := parseJSON(t, stdout)
	data, ok := envelope["data"].([]interface{})
	if !ok {
		t.Fatalf("expected data to be an array, got %T", envelope["data"])
	}
	if len(data) == 0 {
		t.Errorf("expected payments namespace to be found in the index")
	}
}
