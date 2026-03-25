package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestIncrementalUpToDate verifies that running `inariindex` a second time
// immediately after a full build reports "up to date" on stderr, since no
// files have changed.
func TestIncrementalUpToDate(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	// Run incremental index — nothing changed since the full index.
	_, stderr := runInari(t, dir, "index")
	lower := strings.ToLower(stderr)
	if !strings.Contains(lower, "up to date") {
		t.Errorf("expected stderr to contain 'up to date', got: %s", stderr)
	}
}

// TestIncrementalAfterEdit verifies that modifying an existing source file is
// detected by the incremental indexer. The modified file should appear as
// "Modified" in stderr output.
func TestIncrementalAfterEdit(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	// Overwrite the existing logger file with different content.
	loggerPath := filepath.Join(dir, "src", "utils", "logger.ts")
	newContent := `export class Logger {
  info(message: string): void {
    console.log(message);
  }

  error(message: string): void {
    console.error(message);
  }

  warn(message: string): void {
    console.warn(message);
  }
}
`
	if err := os.WriteFile(loggerPath, []byte(newContent), 0644); err != nil {
		t.Fatalf("failed to write modified logger.ts: %v", err)
	}

	// Run incremental index and verify the modified file is reported.
	_, stderr := runInari(t, dir, "index")
	assertContains(t, stderr, "Modified")
	assertContains(t, stderr, "logger.ts")
}

// TestIncrementalAfterDelete verifies that deleting a source file is detected
// by the incremental indexer. The deleted file should appear as "Deleted" in
// stderr output.
func TestIncrementalAfterDelete(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	// Delete an existing file from the fixture copy.
	loggerPath := filepath.Join(dir, "src", "utils", "logger.ts")
	if err := os.Remove(loggerPath); err != nil {
		t.Fatalf("failed to delete logger.ts: %v", err)
	}

	// Run incremental index and verify the deleted file is reported.
	_, stderr := runInari(t, dir, "index")
	assertContains(t, stderr, "Deleted")
	assertContains(t, stderr, "logger.ts")
}

// TestIncrementalDetectsAddedFile verifies that adding a new source file is
// detected by the incremental indexer and the new symbol becomes queryable.
func TestIncrementalDetectsAddedFile(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	// Create a new TypeScript file that did not exist in the original fixture.
	helperDir := filepath.Join(dir, "src", "utils")
	if err := os.MkdirAll(helperDir, 0755); err != nil {
		t.Fatalf("failed to create helper dir: %v", err)
	}
	helperContent := "export function helper(value: string): string {\n  return value.trim();\n}\n"
	if err := os.WriteFile(filepath.Join(helperDir, "helper.ts"), []byte(helperContent), 0644); err != nil {
		t.Fatalf("failed to write helper.ts: %v", err)
	}

	// Run incremental index and verify the added file appears.
	_, stderr := runInari(t, dir, "index")
	assertContains(t, stderr, "Added")
	assertContains(t, stderr, "helper.ts")

	// The new symbol must now be in the index.
	stdout, _ := runInari(t, dir, "sketch", "helper")
	assertContains(t, stdout, "helper")
}
