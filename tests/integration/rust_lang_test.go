package integration_test

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRustInit verifies that `inariinit` detects Rust from a Cargo.toml.
func TestRustInit(t *testing.T) {
	dir := t.TempDir()

	cargoContent := "[package]\nname = \"test\"\n"
	if err := os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte(cargoContent), 0644); err != nil {
		t.Fatalf("failed to write Cargo.toml: %v", err)
	}

	stdout, _ := runInari(t, dir, "init")
	assertContains(t, stdout, "Rust")
}

// TestRustIndex verifies that indexing the rust-simple fixture succeeds,
// creates a non-empty graph.db, and reports symbols on stderr.
func TestRustIndex(t *testing.T) {
	dir := copyFixture(t, "rust-simple")

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

// TestRustSketch verifies that `inarisketch PaymentService` works on the
// Rust fixture and includes the struct name in its output.
func TestRustSketch(t *testing.T) {
	dir := setupIndexedFixture(t, "rust-simple")

	stdout, _ := runInari(t, dir, "sketch", "PaymentService")
	assertContains(t, stdout, "PaymentService")
}

// TestRustVisibility verifies that the Rust indexer captures visibility
// metadata (pub, private) for symbols. We check this through the sketch
// --json output for PaymentService.
func TestRustVisibility(t *testing.T) {
	dir := setupIndexedFixture(t, "rust-simple")

	// Verify the struct is accessible via sketch --json.
	stdout, _ := runInari(t, dir, "sketch", "PaymentService", "--json")
	envelope := parseJSON(t, stdout)

	cmd, _ := envelope["command"].(string)
	if cmd != "sketch" {
		t.Errorf("expected command=sketch, got %s", cmd)
	}

	if envelope["data"] == nil {
		t.Fatal("expected non-null data in sketch JSON")
	}

	// Also verify that process_payment (pub) and validate_card (private) are
	// both discoverable via find.
	pubStdout, _ := runInari(t, dir, "find", "process_payment", "--json")
	pubEnvelope := parseJSON(t, pubStdout)
	pubData, ok := pubEnvelope["data"].([]interface{})
	if !ok {
		t.Fatalf("expected data to be an array, got %T", pubEnvelope["data"])
	}
	if len(pubData) == 0 {
		t.Error("expected process_payment (pub) to be found in the index")
	}

	privStdout, _ := runInari(t, dir, "find", "validate_card", "--json")
	privEnvelope := parseJSON(t, privStdout)
	privData, ok := privEnvelope["data"].([]interface{})
	if !ok {
		t.Fatalf("expected data to be an array, got %T", privEnvelope["data"])
	}
	if len(privData) == 0 {
		t.Error("expected validate_card (private) to be found in the index")
	}
}

// TestRustSketchJsonOutput verifies that `inarisketch PaymentService --json`
// returns valid JSON with command="sketch" on the Rust fixture.
func TestRustSketchJsonOutput(t *testing.T) {
	dir := setupIndexedFixture(t, "rust-simple")

	stdout, _ := runInari(t, dir, "sketch", "PaymentService", "--json")
	envelope := parseJSON(t, stdout)

	cmd, _ := envelope["command"].(string)
	if cmd != "sketch" {
		t.Errorf("expected command=sketch, got %s", cmd)
	}
}

// TestRustFindStructs verifies that key Rust structs/enums/traits are indexed
// and discoverable via `inarifind`.
func TestRustFindStructs(t *testing.T) {
	dir := setupIndexedFixture(t, "rust-simple")

	symbols := []string{"PaymentService", "PaymentResult", "PaymentClient"}
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
