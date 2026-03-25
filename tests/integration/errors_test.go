package integration_test

import (
	"strings"
	"testing"
)

// TestNoIndexError verifies that running `inari sketch Foo` in a directory
// with no .inari/ produces a helpful error telling the user to run inari init.
func TestNoIndexError(t *testing.T) {
	dir := t.TempDir()

	_, stderr := runInariExpectFail(t, dir, "sketch", "Foo")
	lower := strings.ToLower(stderr)
	if !strings.Contains(lower, ".inari") && !strings.Contains(lower, "inari init") && !strings.Contains(lower, "no index found") {
		t.Errorf("expected stderr to mention '.inari', 'inari init', or 'no index found', got: %s", stderr)
	}
}

// TestSymbolNotFound verifies that querying an unknown symbol against a valid
// index fails with a "not found" error and suggests using `inari find`.
func TestSymbolNotFound(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	_, stderr := runInariExpectFail(t, dir, "sketch", "Unknown")
	assertContains(t, stderr, "not found")
	assertContains(t, stderr, "inari find")
}

// TestInitAlreadyInitialised verifies that running `inari init` a second time
// on an already-initialised project fails with an appropriate error message.
func TestInitAlreadyInitialised(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	// Second init should fail because .inari/config.toml already exists.
	_, stderr := runInariExpectFail(t, dir, "init")
	lower := strings.ToLower(stderr)
	if !strings.Contains(lower, "already") && !strings.Contains(lower, "exists") {
		t.Errorf("expected stderr to mention 'already' or 'exists', got: %s", stderr)
	}
}

// TestNoIndexRefsError verifies that `inari refs Foo` without an index fails.
func TestNoIndexRefsError(t *testing.T) {
	dir := t.TempDir()

	_, stderr := runInariExpectFail(t, dir, "refs", "Foo")
	lower := strings.ToLower(stderr)
	if !strings.Contains(lower, ".inari") && !strings.Contains(lower, "inari init") && !strings.Contains(lower, "no index found") {
		t.Errorf("expected stderr to mention '.inari', 'inari init', or 'no index found', got: %s", stderr)
	}
}

// TestNoIndexImpactError verifies that `inari impact Foo` without an index fails.
func TestNoIndexImpactError(t *testing.T) {
	dir := t.TempDir()

	_, stderr := runInariExpectFail(t, dir, "impact", "Foo")
	lower := strings.ToLower(stderr)
	if !strings.Contains(lower, ".inari") && !strings.Contains(lower, "inari init") && !strings.Contains(lower, "no index found") {
		t.Errorf("expected stderr to mention '.inari', 'inari init', or 'no index found', got: %s", stderr)
	}
}

// TestNoIndexFindError verifies that `inari find "payment"` without an index fails.
func TestNoIndexFindError(t *testing.T) {
	dir := t.TempDir()

	_, stderr := runInariExpectFail(t, dir, "find", "payment")
	lower := strings.ToLower(stderr)
	// The find command may produce a DB-level error when no .inari/ exists.
	// We just verify it fails with some error output on stderr.
	if len(strings.TrimSpace(lower)) == 0 {
		t.Error("expected non-empty stderr when running find without index")
	}
}
