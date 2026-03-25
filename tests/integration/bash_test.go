package integration_test

import (
	"os"
	"path/filepath"
	"testing"
)

// TestBashInit verifies that `inari init` detects Bash from .sh files.
func TestBashInit(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "run.sh"), []byte("#!/bin/bash\n"), 0644); err != nil {
		t.Fatalf("failed to write run.sh: %v", err)
	}
	stdout, _ := runInari(t, dir, "init")
	assertContains(t, stdout, "Bash")
}

// TestBashIndex verifies that indexing the bash-simple fixture succeeds
// and creates a non-empty graph.db.
func TestBashIndex(t *testing.T) {
	dir := copyFixture(t, "bash-simple")

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

// TestBashSketch verifies that `inari sketch process_payment` works on the
// Bash fixture.
func TestBashSketch(t *testing.T) {
	dir := setupIndexedFixture(t, "bash-simple")

	stdout, _ := runInari(t, dir, "sketch", "process_payment")
	assertContains(t, stdout, "process_payment")
	assertContains(t, stdout, "function")
}

// TestBashSketchJsonOutput verifies JSON output on the Bash fixture.
func TestBashSketchJsonOutput(t *testing.T) {
	dir := setupIndexedFixture(t, "bash-simple")

	stdout, _ := runInari(t, dir, "sketch", "process_payment", "--json")
	envelope := parseJSON(t, stdout)

	cmd, _ := envelope["command"].(string)
	if cmd != "sketch" {
		t.Errorf("expected command=sketch, got %s", cmd)
	}
}

// TestBashFindFunctions verifies that key Bash functions are indexed.
func TestBashFindFunctions(t *testing.T) {
	dir := setupIndexedFixture(t, "bash-simple")

	symbols := []string{"process_payment", "create_order", "log_info"}
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

// TestBashRefs verifies that `inari refs process_payment` finds callers.
func TestBashRefs(t *testing.T) {
	dir := setupIndexedFixture(t, "bash-simple")

	stdout, _ := runInari(t, dir, "refs", "process_payment")
	assertContains(t, stdout, "controller.sh")
}

// TestBashMap verifies that `inari map` produces a valid overview.
func TestBashMap(t *testing.T) {
	dir := setupIndexedFixture(t, "bash-simple")

	stdout, _ := runInari(t, dir, "map")
	assertContains(t, stdout, "bash")
	assertContains(t, stdout, "process_payment")
}
