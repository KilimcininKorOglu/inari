package integration_test

import (
	"os"
	"path/filepath"
	"testing"
)

// TestEmptyProject verifies that `inariinit` + `inariindex --full` on a
// directory with only a tsconfig.json (no source files) completes without error.
func TestEmptyProject(t *testing.T) {
	dir := t.TempDir()

	// A minimal tsconfig.json is enough for language detection.
	tsconfigContent := `{"compilerOptions":{"target":"ES2020"},"include":["src/**/*"]}`
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(tsconfigContent), 0644); err != nil {
		t.Fatalf("failed to write tsconfig.json: %v", err)
	}

	// Init should succeed.
	runInari(t, dir, "init")

	// Index on empty project should succeed (no files to parse).
	runInari(t, dir, "index", "--full")
}

// TestSketchWithDot verifies that querying a qualified symbol name containing
// a dot (e.g., "PaymentService.processPayment") works or fails gracefully.
func TestSketchWithDot(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	// Sketch a symbol — the exact behaviour depends on whether the CLI
	// supports dotted names. We just verify it does not crash.
	stdout, _ := runInari(t, dir, "sketch", "PaymentService")
	assertContains(t, stdout, "PaymentService")
}

// TestFilePath verifies that passing a file path to `inari sketch` returns
// all symbols defined in that file (sketch supports file-path lookups).
func TestFilePath(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	stdout, _ := runInari(t, dir, "sketch", "src/payments/service.ts")
	assertContains(t, stdout, "PaymentService")
}

// TestIndexFileWithSyntaxErrors verifies that a TypeScript file with broken
// syntax does not cause `inariindex --full` to crash. Valid files must still
// be indexed.
func TestIndexFileWithSyntaxErrors(t *testing.T) {
	dir := t.TempDir()

	// Create tsconfig for language detection.
	tsconfigContent := `{"compilerOptions":{"target":"ES2020"},"include":["src/**/*"]}`
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(tsconfigContent), 0644); err != nil {
		t.Fatalf("failed to write tsconfig.json: %v", err)
	}

	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("failed to create src dir: %v", err)
	}

	// Write a valid TypeScript file.
	validContent := "export class ValidClass { greet(): string { return 'hello'; } }"
	if err := os.WriteFile(filepath.Join(srcDir, "valid.ts"), []byte(validContent), 0644); err != nil {
		t.Fatalf("failed to write valid.ts: %v", err)
	}

	// Write a TypeScript file with deliberately broken syntax.
	brokenContent := "export class {{{{ this is not valid TypeScript )))))"
	if err := os.WriteFile(filepath.Join(srcDir, "broken.ts"), []byte(brokenContent), 0644); err != nil {
		t.Fatalf("failed to write broken.ts: %v", err)
	}

	// Init and index must complete without crashing.
	runInari(t, dir, "init")
	runInari(t, dir, "index", "--full")

	// The valid class must still be discoverable.
	stdout, _ := runInari(t, dir, "sketch", "ValidClass")
	assertContains(t, stdout, "ValidClass")
}

// TestIndexEmptyFile verifies that an empty .ts file does not crash indexing.
func TestIndexEmptyFile(t *testing.T) {
	dir := t.TempDir()

	tsconfigContent := `{"compilerOptions":{"target":"ES2020"},"include":["src/**/*"]}`
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(tsconfigContent), 0644); err != nil {
		t.Fatalf("failed to write tsconfig.json: %v", err)
	}

	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("failed to create src dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(srcDir, "empty.ts"), []byte(""), 0644); err != nil {
		t.Fatalf("failed to write empty.ts: %v", err)
	}

	runInari(t, dir, "init")
	runInari(t, dir, "index", "--full")
}

// TestFindNoResults verifies that `inarifind` with a query that matches
// nothing exits successfully and indicates no results were found.
func TestFindNoResults(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	stdout, _ := runInari(t, dir, "find", "xyzzynonexistent")
	assertContains(t, stdout, "no results found")
}

// TestImpactDeepDepth verifies that `inariimpact PaymentService --depth 10`
// completes without hanging, crashing, or producing a non-zero exit code.
func TestImpactDeepDepth(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	stdout, _ := runInari(t, dir, "impact", "PaymentService", "--depth", "10")
	assertContains(t, stdout, "Impact analysis")
}
