package integration_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// inari init
// ---------------------------------------------------------------------------

// TestInitCreatesInari verifies that `inari init` creates a .inari/ directory
// containing a config.toml that mentions the detected language.
func TestInitCreatesInari(t *testing.T) {
	dir := copyFixture(t, "typescript-simple")

	stdout, _ := runInari(t, dir, "init")
	assertContains(t, stdout, "Initialised")

	inariDir := filepath.Join(dir, ".inari")
	if _, err := os.Stat(inariDir); os.IsNotExist(err) {
		t.Fatal(".inari/ directory should exist after init")
	}

	configPath := filepath.Join(inariDir, "config.toml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("config.toml should exist after init")
	}

	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("reading config.toml: %v", err)
	}
	assertContains(t, string(configBytes), "typescript")
}

// TestInitDetectsTypeScript verifies that auto-detection recognises
// TypeScript from the presence of tsconfig.json.
func TestInitDetectsTypeScript(t *testing.T) {
	// Create a minimal directory with only a tsconfig.json.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("writing tsconfig.json: %v", err)
	}

	stdout, _ := runInari(t, dir, "init")
	assertContains(t, stdout, "Initialised")

	configBytes, err := os.ReadFile(filepath.Join(dir, ".inari", "config.toml"))
	if err != nil {
		t.Fatalf("reading config.toml: %v", err)
	}
	assertContains(t, string(configBytes), "typescript")
}

// TestInitAlreadyExists verifies that running `inari init` twice produces an
// error on the second invocation because .inari/ already exists.
func TestInitAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("writing tsconfig.json: %v", err)
	}

	// First init succeeds.
	runInari(t, dir, "init")

	// Second init must fail.
	_, stderr := runInariExpectFail(t, dir, "init")
	assertContains(t, stderr, "already")
}

// ---------------------------------------------------------------------------
// inari index
// ---------------------------------------------------------------------------

// TestIndexFullTypeScript verifies that `inari index --full` creates graph.db
// and reports files/symbols in its output.
func TestIndexFullTypeScript(t *testing.T) {
	dir := copyFixture(t, "typescript-simple")

	runInari(t, dir, "init")
	_, stderr := runInari(t, dir, "index", "--full")

	// The Rust version prints stats to stderr.
	combined := stderr
	assertContains(t, combined, "files")
	assertContains(t, combined, "symbols")

	graphDB := filepath.Join(dir, ".inari", "graph.db")
	info, err := os.Stat(graphDB)
	if os.IsNotExist(err) {
		t.Fatal("graph.db should exist after indexing")
	}
	if info.Size() == 0 {
		t.Fatal("graph.db should not be empty")
	}
}

// TestIndexIncremental verifies that running `inari index` (without --full)
// after a full index reports an "up to date" message.
func TestIndexIncremental(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	// Run index without --full — should detect no changes and report up to date.
	stdout, stderr := runInari(t, dir, "index")
	combined := stdout + stderr
	assertContains(t, combined, "up to date")
}

// TestIndexFullJson verifies that `inari index --full --json` emits valid JSON.
func TestIndexFullJson(t *testing.T) {
	dir := copyFixture(t, "typescript-simple")
	runInari(t, dir, "init")

	stdout, _ := runInari(t, dir, "index", "--full", "--json")

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\noutput: %s", err, stdout)
	}
}
