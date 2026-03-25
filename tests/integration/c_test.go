package integration_test

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCLangInit verifies that `inari init` detects C from project markers.
func TestCLangInit(t *testing.T) {
	dir := t.TempDir()
	// Create Makefile and a .c file in src/
	if err := os.WriteFile(filepath.Join(dir, "CMakeLists.txt"), []byte("cmake_minimum_required(VERSION 3.10)\n"), 0644); err != nil {
		t.Fatalf("failed to write CMakeLists.txt: %v", err)
	}
	stdout, _ := runInari(t, dir, "init")
	assertContains(t, stdout, "C")
}

// TestCLangIndex verifies that indexing the c-simple fixture succeeds.
func TestCLangIndex(t *testing.T) {
	dir := copyFixture(t, "c-simple")

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

// TestCLangSketch verifies sketch on the C fixture.
func TestCLangSketch(t *testing.T) {
	dir := setupIndexedFixture(t, "c-simple")

	stdout, _ := runInari(t, dir, "sketch", "process_payment")
	assertContains(t, stdout, "process_payment")
	assertContains(t, stdout, "function")
}

// TestCLangSketchJsonOutput verifies JSON output on the C fixture.
func TestCLangSketchJsonOutput(t *testing.T) {
	dir := setupIndexedFixture(t, "c-simple")

	stdout, _ := runInari(t, dir, "sketch", "process_payment", "--json")
	envelope := parseJSON(t, stdout)

	cmd, _ := envelope["command"].(string)
	if cmd != "sketch" {
		t.Errorf("expected command=sketch, got %s", cmd)
	}
}

// TestCLangFindFunctions verifies that key C functions are indexed.
func TestCLangFindFunctions(t *testing.T) {
	dir := setupIndexedFixture(t, "c-simple")

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

// TestCLangRefs verifies that `inari refs process_payment` finds callers.
func TestCLangRefs(t *testing.T) {
	dir := setupIndexedFixture(t, "c-simple")

	stdout, _ := runInari(t, dir, "refs", "process_payment")
	assertContains(t, stdout, "controller.c")
}

// TestCLangMap verifies that `inari map` produces a valid overview.
func TestCLangMap(t *testing.T) {
	dir := setupIndexedFixture(t, "c-simple")

	stdout, _ := runInari(t, dir, "map")
	assertContains(t, stdout, "process_payment")
}

// TestCLangStructs verifies that C structs are indexed.
func TestCLangStructs(t *testing.T) {
	dir := setupIndexedFixture(t, "c-simple")

	stdout, _ := runInari(t, dir, "find", "PaymentService", "--json")
	envelope := parseJSON(t, stdout)
	data, ok := envelope["data"].([]interface{})
	if !ok {
		t.Fatalf("expected data to be an array, got %T", envelope["data"])
	}
	if len(data) == 0 {
		t.Errorf("expected PaymentService struct to be found in the index")
	}
}
