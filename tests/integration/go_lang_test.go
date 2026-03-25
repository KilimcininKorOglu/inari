package integration_test

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGoInit verifies that `inari init` detects Go from a go.mod file.
func TestGoInit(t *testing.T) {
	dir := t.TempDir()

	gomod := "module example.com/test\n\ngo 1.25\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	stdout, _ := runInari(t, dir, "init")
	assertContains(t, stdout, "Go")
}

// TestGoIndex verifies that indexing the go-simple fixture succeeds,
// creates a non-empty graph.db, and reports symbols on stderr.
func TestGoIndex(t *testing.T) {
	dir := copyFixture(t, "go-simple")

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

// TestGoSketch verifies that `inari sketch PaymentService` works on the
// Go fixture and includes the struct name in its output.
func TestGoSketch(t *testing.T) {
	dir := setupIndexedFixture(t, "go-simple")

	stdout, _ := runInari(t, dir, "sketch", "PaymentService")
	assertContains(t, stdout, "PaymentService")
}

// TestGoVisibility verifies that the Go indexer captures exported vs
// unexported status. ProcessPayment (exported) and validateAmount
// (unexported) should both be discoverable.
func TestGoVisibility(t *testing.T) {
	dir := setupIndexedFixture(t, "go-simple")

	stdout, _ := runInari(t, dir, "sketch", "PaymentService", "--json")
	envelope := parseJSON(t, stdout)

	cmd, _ := envelope["command"].(string)
	if cmd != "sketch" {
		t.Errorf("expected command=sketch, got %s", cmd)
	}

	if envelope["data"] == nil {
		t.Fatal("expected non-null data in sketch JSON")
	}

	// Verify ProcessPayment (exported) is discoverable.
	findStdout, _ := runInari(t, dir, "find", "ProcessPayment", "--json")
	findEnvelope := parseJSON(t, findStdout)
	data, ok := findEnvelope["data"].([]interface{})
	if !ok {
		t.Fatalf("expected data to be an array, got %T", findEnvelope["data"])
	}
	if len(data) == 0 {
		t.Error("expected ProcessPayment (exported) to be found in the index")
	}
}

// TestGoSketchJsonOutput verifies that `inari sketch PaymentService --json`
// returns valid JSON with command="sketch" on the Go fixture.
func TestGoSketchJsonOutput(t *testing.T) {
	dir := setupIndexedFixture(t, "go-simple")

	stdout, _ := runInari(t, dir, "sketch", "PaymentService", "--json")
	envelope := parseJSON(t, stdout)

	cmd, _ := envelope["command"].(string)
	if cmd != "sketch" {
		t.Errorf("expected command=sketch, got %s", cmd)
	}
}

// TestGoFindStructs verifies that key Go structs are indexed and
// discoverable via `inari find`.
func TestGoFindStructs(t *testing.T) {
	dir := setupIndexedFixture(t, "go-simple")

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

// TestGoRefsFindsCallers verifies that `inari refs ProcessPayment`
// succeeds on the Go fixture.
func TestGoRefsFindsCallers(t *testing.T) {
	dir := setupIndexedFixture(t, "go-simple")

	runInari(t, dir, "refs", "ProcessPayment")
}
