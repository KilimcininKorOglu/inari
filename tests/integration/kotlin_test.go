package integration_test

import (
	"os"
	"path/filepath"
	"testing"
)

// TestKotlinInit verifies that `inari init` detects Kotlin from a build.gradle.kts file.
func TestKotlinInit(t *testing.T) {
	dir := t.TempDir()

	kts := `plugins { kotlin("jvm") version "1.9.0" }`
	if err := os.WriteFile(filepath.Join(dir, "build.gradle.kts"), []byte(kts), 0644); err != nil {
		t.Fatalf("failed to write build.gradle.kts: %v", err)
	}

	stdout, _ := runInari(t, dir, "init")
	assertContains(t, stdout, "Kotlin")
}

// TestKotlinInitSettings verifies that `inari init` detects Kotlin from settings.gradle.kts.
func TestKotlinInitSettings(t *testing.T) {
	dir := t.TempDir()

	settings := `rootProject.name = "test"`
	if err := os.WriteFile(filepath.Join(dir, "settings.gradle.kts"), []byte(settings), 0644); err != nil {
		t.Fatalf("failed to write settings.gradle.kts: %v", err)
	}

	stdout, _ := runInari(t, dir, "init")
	assertContains(t, stdout, "Kotlin")
}

// TestKotlinIndex verifies that indexing the kotlin-simple fixture succeeds
// and creates a non-empty graph.db.
func TestKotlinIndex(t *testing.T) {
	dir := copyFixture(t, "kotlin-simple")

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

// TestKotlinSketch verifies that `inari sketch PaymentService` works on the
// Kotlin fixture and includes the class name in its output.
func TestKotlinSketch(t *testing.T) {
	dir := setupIndexedFixture(t, "kotlin-simple")

	stdout, _ := runInari(t, dir, "sketch", "PaymentService")
	assertContains(t, stdout, "PaymentService")
	assertContains(t, stdout, "class")
}

// TestKotlinSketchJsonOutput verifies that `inari sketch PaymentService --json`
// returns valid JSON with command="sketch" on the Kotlin fixture.
func TestKotlinSketchJsonOutput(t *testing.T) {
	dir := setupIndexedFixture(t, "kotlin-simple")

	stdout, _ := runInari(t, dir, "sketch", "PaymentService", "--json")
	envelope := parseJSON(t, stdout)

	cmd, _ := envelope["command"].(string)
	if cmd != "sketch" {
		t.Errorf("expected command=sketch, got %s", cmd)
	}
}

// TestKotlinFindClasses verifies that key Kotlin classes are indexed and
// discoverable via `inari find`.
func TestKotlinFindClasses(t *testing.T) {
	dir := setupIndexedFixture(t, "kotlin-simple")

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

// TestKotlinRefs verifies that `inari refs processPayment` finds callers
// in the Kotlin fixture.
func TestKotlinRefs(t *testing.T) {
	dir := setupIndexedFixture(t, "kotlin-simple")

	stdout, _ := runInari(t, dir, "refs", "processPayment")
	assertContains(t, stdout, "OrderController")
}

// TestKotlinMap verifies that `inari map` produces a valid overview for
// the Kotlin fixture.
func TestKotlinMap(t *testing.T) {
	dir := setupIndexedFixture(t, "kotlin-simple")

	stdout, _ := runInari(t, dir, "map")
	assertContains(t, stdout, "kotlin")
	assertContains(t, stdout, "PaymentService")
}

// TestKotlinDeps verifies that `inari deps PaymentService` returns
// expected dependencies like Logger import and method calls.
func TestKotlinDeps(t *testing.T) {
	dir := setupIndexedFixture(t, "kotlin-simple")

	stdout, _ := runInari(t, dir, "deps", "PaymentService")
	assertContains(t, stdout, "Logger")
}
